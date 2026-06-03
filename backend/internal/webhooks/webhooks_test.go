package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingExecutor struct {
	query string
	args  []any
	err   error
}

type endpointRecordingStore struct {
	record   ConfigureEndpointRecord
	endpoint Endpoint
	err      error
}

func (executor *recordingExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	executor.query = query
	executor.args = args
	return nil, executor.err
}

func (store *endpointRecordingStore) ConfigureEndpoint(ctx context.Context, record ConfigureEndpointRecord) (Endpoint, error) {
	store.record = record
	if store.err != nil {
		return Endpoint{}, store.err
	}
	if store.endpoint.ID != uuid.Nil {
		return store.endpoint, nil
	}

	return Endpoint{
		ID: record.ID,
		Organization: Organization{
			ID:               record.OrganizationID,
			Name:             record.OrganizationName,
			OrganizationType: record.OrganizationType,
		},
		URL:       record.URL,
		Status:    record.Status,
		CreatedAt: record.ConfiguredAt,
		UpdatedAt: record.ConfiguredAt,
	}, nil
}

func TestEndpointServiceConfigureEndpointPreparesRecord(t *testing.T) {
	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	store := &endpointRecordingStore{}
	service := NewEndpointServiceWithClock(store, func() time.Time {
		return now
	})

	endpoint, err := service.ConfigureEndpoint(context.Background(), ConfigureEndpointInput{
		OrganizationName: " Acme Bank ",
		OrganizationType: "bank",
		URL:              " https://example.com/kladd/webhooks ",
	})
	if err != nil {
		t.Fatalf("configure endpoint: %v", err)
	}

	if store.record.OrganizationName != "Acme Bank" {
		t.Fatalf("organization name = %q, want Acme Bank", store.record.OrganizationName)
	}
	if store.record.OrganizationType != "bank" {
		t.Fatalf("organization type = %q, want bank", store.record.OrganizationType)
	}
	if store.record.URL != "https://example.com/kladd/webhooks" {
		t.Fatalf("url = %q, want normalized url", store.record.URL)
	}
	if store.record.Status != EndpointStatusActive {
		t.Fatalf("status = %q, want %q", store.record.Status, EndpointStatusActive)
	}
	if !store.record.ConfiguredAt.Equal(now) {
		t.Fatalf("configured at = %s, want %s", store.record.ConfiguredAt, now)
	}
	if endpoint.URL != "https://example.com/kladd/webhooks" {
		t.Fatalf("endpoint url = %q", endpoint.URL)
	}
}

func TestEndpointServiceConfigureEndpointValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input ConfigureEndpointInput
		err   error
	}{
		{
			name: "missing organization",
			input: ConfigureEndpointInput{
				URL: "https://example.com/webhooks",
			},
			err: ErrInvalidOrganization,
		},
		{
			name: "missing url",
			input: ConfigureEndpointInput{
				OrganizationName: "Acme Bank",
			},
			err: ErrInvalidEndpointURL,
		},
		{
			name: "unsupported scheme",
			input: ConfigureEndpointInput{
				OrganizationName: "Acme Bank",
				URL:              "ftp://example.com/webhooks",
			},
			err: ErrInvalidEndpointURL,
		},
	}

	service := NewEndpointService(&endpointRecordingStore{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.ConfigureEndpoint(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("err = %v, want %v", err, test.err)
			}
		})
	}
}

func TestBuildClaimDeliveryCreatesSignedSafePayload(t *testing.T) {
	claimID := uuid.New()
	requestID := uuid.New()
	orgID := uuid.New()
	occurredAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)

	delivery, err := BuildClaimDelivery("secret", ClaimEvent{
		EventType:      EventClaimApproved,
		ClaimID:        claimID,
		ClaimRequestID: requestID,
		OrganizationID: orgID,
		Status:         "active",
		ExpiresAt:      expiresAt,
		OccurredAt:     occurredAt,
	})
	if err != nil {
		t.Fatalf("build delivery: %v", err)
	}

	if delivery.EventType != EventClaimApproved {
		t.Fatalf("event type = %q, want %q", delivery.EventType, EventClaimApproved)
	}
	if delivery.Status != StatusPending {
		t.Fatalf("status = %q, want %q", delivery.Status, StatusPending)
	}
	if !strings.HasPrefix(delivery.Signature, "sha256=") {
		t.Fatalf("signature = %q, want sha256 prefix", delivery.Signature)
	}
	if delivery.Payload["verification_path"] != "/verify/"+claimID.String() {
		t.Fatalf("verification path = %q", delivery.Payload["verification_path"])
	}

	body := strings.ToLower(strings.ReplaceAll(strings.Join(payloadValues(delivery.Payload), " "), "_", " "))
	for _, forbidden := range []string{"raw document", "file path", "security pin", "security pin hash", "truth value", "bvn", "nin", "passport number", "tax id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("payload exposed forbidden value %q", forbidden)
		}
	}
}

func TestBuildClaimDeliveryRequiresSigningSecret(t *testing.T) {
	_, err := BuildClaimDelivery("", ClaimEvent{
		EventType:      EventClaimApproved,
		ClaimID:        uuid.New(),
		ClaimRequestID: uuid.New(),
		OrganizationID: uuid.New(),
		Status:         "active",
		ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		OccurredAt:     time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected missing signing secret error")
	}
}

func TestBuildClaimDeliveryAcceptsExpiredEvent(t *testing.T) {
	delivery, err := BuildClaimDelivery("secret", ClaimEvent{
		EventType:      EventClaimExpired,
		ClaimID:        uuid.New(),
		ClaimRequestID: uuid.New(),
		OrganizationID: uuid.New(),
		Status:         "expired",
		ExpiresAt:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		OccurredAt:     time.Date(2026, 6, 1, 12, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("build expired delivery: %v", err)
	}

	if delivery.EventType != EventClaimExpired {
		t.Fatalf("event type = %q, want %q", delivery.EventType, EventClaimExpired)
	}
	if delivery.Payload["status"] != "expired" {
		t.Fatalf("status = %q, want expired", delivery.Payload["status"])
	}
}

func TestEnqueueClaimEventWritesPendingDelivery(t *testing.T) {
	executor := &recordingExecutor{}
	claimID := uuid.New()
	orgID := uuid.New()
	occurredAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	err := EnqueueClaimEvent(context.Background(), executor, "secret", ClaimEvent{
		EventType:      EventClaimRevoked,
		ClaimID:        claimID,
		ClaimRequestID: uuid.New(),
		OrganizationID: orgID,
		Status:         "revoked",
		ExpiresAt:      time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		OccurredAt:     occurredAt,
	})
	if err != nil {
		t.Fatalf("enqueue event: %v", err)
	}

	if !strings.Contains(executor.query, "INSERT INTO webhook_deliveries") {
		t.Fatalf("query = %s", executor.query)
	}
	if executor.args[1] != EventClaimRevoked {
		t.Fatalf("event type arg = %v, want %s", executor.args[1], EventClaimRevoked)
	}
	if executor.args[2] != claimID {
		t.Fatalf("aggregate id arg = %v, want %s", executor.args[2], claimID)
	}
	if executor.args[3] != orgID {
		t.Fatalf("organization id arg = %v, want %s", executor.args[3], orgID)
	}
	if executor.args[6] != StatusPending {
		t.Fatalf("status arg = %v, want %s", executor.args[6], StatusPending)
	}
}

func payloadValues(payload map[string]any) []string {
	values := make([]string, 0, len(payload))
	for _, value := range payload {
		values = append(values, fmt.Sprint(value))
	}
	return values
}
