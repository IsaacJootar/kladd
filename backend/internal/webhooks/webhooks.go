package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	EventClaimApproved = "claim.approved"
	EventClaimRevoked  = "claim.revoked"

	StatusPending = "pending"
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

type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
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
	if event.EventType != EventClaimApproved && event.EventType != EventClaimRevoked {
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
