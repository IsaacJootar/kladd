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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	EventClaimApproved = "claim.approved"
	EventClaimExpired  = "claim.expired"
	EventClaimRevoked  = "claim.revoked"

	StatusPending   = "pending"
	StatusDelivered = "delivered"
	StatusFailed    = "failed"

	EndpointStatusActive   = "active"
	EndpointStatusDisabled = "disabled"
)

var (
	ErrInvalidOrganization   = errors.New("organization name is required")
	ErrInvalidOrganizationID = errors.New("organization_id is required")
	ErrInvalidEndpointURL    = errors.New("webhook endpoint url must be http or https")
	ErrEndpointNotFound      = errors.New("webhook endpoint not found")
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
	PayloadJSON    string
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

type DeliveryLog struct {
	ID             uuid.UUID  `json:"id"`
	EventType      string     `json:"event_type"`
	AggregateID    uuid.UUID  `json:"aggregate_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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

type PendingDelivery struct {
	ID             uuid.UUID
	EventType      string
	OrganizationID uuid.UUID
	PayloadJSON    string
	Signature      string
	EndpointURL    string
	Attempts       int
}

type DeliveryAttempt struct {
	DeliveryID    uuid.UUID
	Status        string
	ResponseCode  int
	ErrorMessage  string
	AttemptedAt   time.Time
	NextAttemptAt *time.Time
}

type DeliverySummary struct {
	Attempted int `json:"attempted"`
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
}

type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type EndpointStore interface {
	ConfigureEndpoint(ctx context.Context, record ConfigureEndpointRecord) (Endpoint, error)
	GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (Endpoint, error)
	ListDeliveriesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]DeliveryLog, error)
}

type DeliveryStore interface {
	ListPendingDeliveries(ctx context.Context, dueAt time.Time, limit int) ([]PendingDelivery, error)
	RecordDeliveryAttempt(ctx context.Context, attempt DeliveryAttempt) error
}

type Sender interface {
	Send(ctx context.Context, delivery PendingDelivery) (int, error)
}

type EndpointService struct {
	store EndpointStore
	now   func() time.Time
}

type DeliveryService struct {
	store  DeliveryStore
	sender Sender
	now    func() time.Time
	limit  int
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

func NewDeliveryService(store DeliveryStore, sender Sender) DeliveryService {
	return DeliveryService{
		store:  store,
		sender: sender,
		now:    time.Now,
		limit:  25,
	}
}

func NewDeliveryServiceWithClock(store DeliveryStore, sender Sender, now func() time.Time) DeliveryService {
	return DeliveryService{
		store:  store,
		sender: sender,
		now:    now,
		limit:  25,
	}
}

func (service EndpointService) ConfigureEndpoint(ctx context.Context, input ConfigureEndpointInput) (Endpoint, error) {
	record, err := service.prepareConfigureRecord(input)
	if err != nil {
		return Endpoint{}, err
	}

	return service.store.ConfigureEndpoint(ctx, record)
}

func (service EndpointService) GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (Endpoint, error) {
	if organizationID == uuid.Nil {
		return Endpoint{}, ErrInvalidOrganizationID
	}

	return service.store.GetEndpointForOrganization(ctx, organizationID)
}

func (service EndpointService) ListDeliveriesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]DeliveryLog, error) {
	if organizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	return service.store.ListDeliveriesForOrganization(ctx, organizationID)
}

func (service DeliveryService) DeliverPending(ctx context.Context) (DeliverySummary, error) {
	now := service.now().UTC()
	deliveries, err := service.store.ListPendingDeliveries(ctx, now, service.limit)
	if err != nil {
		return DeliverySummary{}, err
	}

	summary := DeliverySummary{}
	for _, delivery := range deliveries {
		summary.Attempted++

		statusCode, err := service.sender.Send(ctx, delivery)
		attempt := DeliveryAttempt{
			DeliveryID:   delivery.ID,
			ResponseCode: statusCode,
			AttemptedAt:  now,
		}

		if err == nil && statusCode >= 200 && statusCode < 300 {
			attempt.Status = StatusDelivered
			summary.Delivered++
		} else {
			attempt.Status = StatusPending
			attempt.ErrorMessage = deliveryErrorMessage(statusCode, err)
			nextAttemptAt := now.Add(retryDelay(delivery.Attempts + 1))
			attempt.NextAttemptAt = &nextAttemptAt
			summary.Failed++
		}

		if err := service.store.RecordDeliveryAttempt(ctx, attempt); err != nil {
			return summary, err
		}
	}

	return summary, nil
}

func EnqueueClaimEvent(ctx context.Context, tx txExecutor, signingSecret string, event ClaimEvent) error {
	delivery, err := BuildClaimDelivery(signingSecret, event)
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
		delivery.PayloadJSON,
		delivery.Signature,
		delivery.Status,
		delivery.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue webhook delivery: %w", err)
	}

	return nil
}

type HTTPSender struct {
	client *http.Client
}

func NewHTTPSender(client *http.Client) HTTPSender {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return HTTPSender{client: client}
}

func (sender HTTPSender) Send(ctx context.Context, delivery PendingDelivery) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.EndpointURL, strings.NewReader(delivery.PayloadJSON))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Kladd-Event", delivery.EventType)
	request.Header.Set("X-Kladd-Signature", delivery.Signature)
	request.Header.Set("X-Kladd-Delivery", delivery.ID.String())

	response, err := sender.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode, nil
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
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return Delivery{}, err
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
		PayloadJSON:    string(payloadBytes),
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

func deliveryErrorMessage(statusCode int, err error) string {
	if err != nil {
		return err.Error()
	}
	if statusCode > 0 {
		return fmt.Sprintf("webhook endpoint returned status %d", statusCode)
	}
	return "webhook delivery failed"
}

func retryDelay(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 1 * time.Minute
	case attempts == 2:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
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
