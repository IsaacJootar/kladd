package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	EventClaimApproved = "claim.approved"
	EventClaimExpired  = "claim.expired"
	EventClaimRevoked  = "claim.revoked"

	StatusPending = "pending"

	EndpointStatusActive   = "active"
	EndpointStatusDisabled = "disabled"
)

var (
	ErrInvalidOrganization = errors.New("organization name is required")
	ErrInvalidEndpointURL  = errors.New("webhook endpoint url must be http or https")
)

type ClaimEvent struct {
	EventType      string
	ClaimID        uuid.UUID
	ClaimRequestID uuid.UUID
	OrganizationID uuid.UUID
	Status         string
	ExpiresAt      time.Time
	OccurredAt     time.Time
}

type Delivery struct {
	ID             uuid.UUID
	EventType      string
	AggregateID    uuid.UUID
	OrganizationID uuid.UUID
	Payload        map[string]any
	Signature      string
	Status         string
	CreatedAt      time.Time
}

type Organization struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	OrganizationType   string    `json:"organization_type"`
	VerificationStatus string    `json:"verification_status"`
}

type Endpoint struct {
	ID           uuid.UUID    `json:"id"`
	Organization Organization `json:"organization"`
	URL          string       `json:"url"`
	Status       string       `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type ConfigureEndpointInput struct {
	OrganizationName string
	OrganizationType string
	URL              string
}

type ConfigureEndpointRecord struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	OrganizationName string
	OrganizationType string
	URL              string
	Status           string
	ConfiguredAt     time.Time
}

type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type EndpointStore interface {
	ConfigureEndpoint(ctx context.Context, record ConfigureEndpointRecord) (Endpoint, error)
}

type EndpointService struct {
	store EndpointStore
	now   func() time.Time
}

func NewEndpointService(store EndpointStore) EndpointService {
	return EndpointService{
		store: store,
		now:   time.Now,
	}
}

func NewEndpointServiceWithClock(store EndpointStore, now func() time.Time) EndpointService {
	return EndpointService{
		store: store,
		now:   now,
	}
}

func (service EndpointService) ConfigureEndpoint(ctx context.Context, input ConfigureEndpointInput) (Endpoint, error) {
	record, err := service.prepareConfigureRecord(input)
	if err != nil {
		return Endpoint{}, err
	}

	return service.store.ConfigureEndpoint(ctx, record)
}

func EnqueueClaimEvent(ctx context.Context, tx txExecutor, signingSecret string, event ClaimEvent) error {
	delivery, err := BuildClaimDelivery(signingSecret, event)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(delivery.Payload)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO webhook_deliveries (
    id,
    event_type,
    aggregate_id,
    organization_id,
    payload_json,
    signature,
    status,
    created_at,
    next_attempt_at
) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $8)`,
		delivery.ID,
		delivery.EventType,
		delivery.AggregateID,
		delivery.OrganizationID,
		string(payload),
		delivery.Signature,
		delivery.Status,
		delivery.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue webhook delivery: %w", err)
	}

	return nil
}

func BuildClaimDelivery(signingSecret string, event ClaimEvent) (Delivery, error) {
	if event.EventType != EventClaimApproved && event.EventType != EventClaimExpired && event.EventType != EventClaimRevoked {
		return Delivery{}, fmt.Errorf("unsupported webhook event type %q", event.EventType)
	}
	if event.ClaimID == uuid.Nil || event.ClaimRequestID == uuid.Nil || event.OrganizationID == uuid.Nil {
		return Delivery{}, fmt.Errorf("claim webhook ids are required")
	}
	if event.OccurredAt.IsZero() {
		return Delivery{}, fmt.Errorf("claim webhook occurred_at is required")
	}

	payload := map[string]any{
		"event_type":        event.EventType,
		"claim_id":          event.ClaimID.String(),
		"claim_request_id":  event.ClaimRequestID.String(),
		"organization_id":   event.OrganizationID.String(),
		"status":            event.Status,
		"expires_at":        event.ExpiresAt.Format(time.RFC3339),
		"occurred_at":       event.OccurredAt.Format(time.RFC3339),
		"verification_path": "/verify/" + event.ClaimID.String(),
	}
	signature, err := SignPayload(signingSecret, payload)
	if err != nil {
		return Delivery{}, err
	}

	return Delivery{
		ID:             uuid.New(),
		EventType:      event.EventType,
		AggregateID:    event.ClaimID,
		OrganizationID: event.OrganizationID,
		Payload:        payload,
		Signature:      signature,
		Status:         StatusPending,
		CreatedAt:      event.OccurredAt,
	}, nil
}

func SignPayload(signingSecret string, payload map[string]any) (string, error) {
	if signingSecret == "" {
		return "", fmt.Errorf("webhook signing secret is required")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, []byte(signingSecret))
	if _, err := mac.Write(payloadBytes); err != nil {
		return "", err
	}

	return "sha256=" + hex.EncodeToString(mac.Sum(nil)), nil
}

func (service EndpointService) prepareConfigureRecord(input ConfigureEndpointInput) (ConfigureEndpointRecord, error) {
	organizationName := strings.TrimSpace(input.OrganizationName)
	if organizationName == "" {
		return ConfigureEndpointRecord{}, ErrInvalidOrganization
	}

	organizationType := strings.TrimSpace(input.OrganizationType)
	if organizationType == "" {
		organizationType = "organization"
	}

	endpointURL, err := normalizeEndpointURL(input.URL)
	if err != nil {
		return ConfigureEndpointRecord{}, err
	}

	return ConfigureEndpointRecord{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		OrganizationName: organizationName,
		OrganizationType: organizationType,
		URL:              endpointURL,
		Status:           EndpointStatusActive,
		ConfiguredAt:     service.now().UTC(),
	}, nil
}

func normalizeEndpointURL(value string) (string, error) {
	cleaned := strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(cleaned)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidEndpointURL
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", ErrInvalidEndpointURL
	}

	return parsed.String(), nil
}
