package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	record         ConfigureEndpointRecord
	endpoint       Endpoint
	err            error
	organizationID uuid.UUID
}

type deliveryRecordingStore struct {
	deliveries []PendingDelivery
	attempts   []DeliveryAttempt
	dueAt      time.Time
	limit      int
	err        error
}

type recordingSender struct {
	statusCode int
	err        error
	sent       []PendingDelivery
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

func (store *endpointRecordingStore) GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (Endpoint, error) {
	store.organizationID = organizationID
	if store.err != nil {
		return Endpoint{}, store.err
	}
	return store.endpoint, nil
}

func (store *deliveryRecordingStore) ListPendingDeliveries(ctx context.Context, dueAt time.Time, limit int) ([]PendingDelivery, error) {
	store.dueAt = dueAt
	store.limit = limit
	if store.err != nil {
		return nil, store.err
	}
	return store.deliveries, nil
}

func (store *deliveryRecordingStore) RecordDeliveryAttempt(ctx context.Context, attempt DeliveryAttempt) error {
	store.attempts = append(store.attempts, attempt)
	return store.err
}

func (sender *recordingSender) Send(ctx context.Context, delivery PendingDelivery) (int, error) {
	sender.sent = append(sender.sent, delivery)
	return sender.statusCode, sender.err
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

func TestEndpointServiceGetEndpointForOrganizationUsesOrganizationBoundary(t *testing.T) {
	organizationID := uuid.New()
	store := &endpointRecordingStore{
		endpoint: Endpoint{
			ID: uuid.New(),
			Organization: Organization{
				ID:               organizationID,
				Name:             "Acme Bank",
				OrganizationType: "bank",
			},
			URL:    "https://example.com/kladd/webhooks",
			Status: EndpointStatusActive,
		},
	}
	service := NewEndpointService(store)

	endpoint, err := service.GetEndpointForOrganization(context.Background(), organizationID)
	if err != nil {
		t.Fatalf("get endpoint: %v", err)
	}

	if store.organizationID != organizationID {
		t.Fatalf("organization id = %s, want %s", store.organizationID, organizationID)
	}
	if endpoint.URL != "https://example.com/kladd/webhooks" {
		t.Fatalf("url = %q, want configured endpoint", endpoint.URL)
	}
}

func TestEndpointServiceGetEndpointForOrganizationValidatesOrganization(t *testing.T) {
	service := NewEndpointService(&endpointRecordingStore{})

	_, err := service.GetEndpointForOrganization(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidOrganizationID) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidOrganizationID)
	}
}

func TestDeliveryServiceDeliversPendingWebhook(t *testing.T) {
	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	deliveryID := uuid.New()
	store := &deliveryRecordingStore{
		deliveries: []PendingDelivery{
			{
				ID:          deliveryID,
				EventType:   EventClaimApproved,
				PayloadJSON: `{"event_type":"claim.approved"}`,
				Signature:   "sha256=test",
				EndpointURL: "https://example.com/webhooks",
			},
		},
	}
	sender := &recordingSender{statusCode: http.StatusOK}
	service := NewDeliveryServiceWithClock(store, sender, func() time.Time {
		return now
	})

	summary, err := service.DeliverPending(context.Background())
	if err != nil {
		t.Fatalf("deliver pending: %v", err)
	}

	if !store.dueAt.Equal(now) {
		t.Fatalf("due at = %s, want %s", store.dueAt, now)
	}
	if store.limit != 25 {
		t.Fatalf("limit = %d, want 25", store.limit)
	}
	if summary.Attempted != 1 || summary.Delivered != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(sender.sent))
	}
	if len(store.attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(store.attempts))
	}
	if store.attempts[0].Status != StatusDelivered {
		t.Fatalf("attempt status = %q, want %q", store.attempts[0].Status, StatusDelivered)
	}
	if store.attempts[0].NextAttemptAt != nil {
		t.Fatal("delivered attempt should not schedule retry")
	}
}

func TestDeliveryServiceSchedulesRetryForFailedWebhook(t *testing.T) {
	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	store := &deliveryRecordingStore{
		deliveries: []PendingDelivery{
			{
				ID:          uuid.New(),
				EventType:   EventClaimApproved,
				PayloadJSON: `{"event_type":"claim.approved"}`,
				Signature:   "sha256=test",
				EndpointURL: "https://example.com/webhooks",
				Attempts:    1,
			},
		},
	}
	sender := &recordingSender{statusCode: http.StatusInternalServerError}
	service := NewDeliveryServiceWithClock(store, sender, func() time.Time {
		return now
	})

	summary, err := service.DeliverPending(context.Background())
	if err != nil {
		t.Fatalf("deliver pending: %v", err)
	}

	if summary.Attempted != 1 || summary.Delivered != 0 || summary.Failed != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if store.attempts[0].Status != StatusPending {
		t.Fatalf("attempt status = %q, want %q", store.attempts[0].Status, StatusPending)
	}
	if store.attempts[0].NextAttemptAt == nil {
		t.Fatal("failed attempt should schedule retry")
	}
	if !store.attempts[0].NextAttemptAt.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("next attempt = %s, want %s", *store.attempts[0].NextAttemptAt, now.Add(5*time.Minute))
	}
}

func TestHTTPSenderPostsSignedPayload(t *testing.T) {
	var eventHeader string
	var signatureHeader string
	var deliveryHeader string
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventHeader = r.Header.Get("X-Kladd-Event")
		signatureHeader = r.Header.Get("X-Kladd-Signature")
		deliveryHeader = r.Header.Get("X-Kladd-Delivery")
		bodyBytes, _ := io.ReadAll(r.Body)
		body = string(bodyBytes)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	deliveryID := uuid.New()
	sender := NewHTTPSender(server.Client())
	statusCode, err := sender.Send(context.Background(), PendingDelivery{
		ID:          deliveryID,
		EventType:   EventClaimApproved,
		PayloadJSON: `{"event_type":"claim.approved"}`,
		Signature:   "sha256=test",
		EndpointURL: server.URL,
	})
	if err != nil {
		t.Fatalf("send webhook: %v", err)
	}

	if statusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", statusCode, http.StatusAccepted)
	}
	if eventHeader != EventClaimApproved {
		t.Fatalf("event header = %q, want %q", eventHeader, EventClaimApproved)
	}
	if signatureHeader != "sha256=test" {
		t.Fatalf("signature header = %q", signatureHeader)
	}
	if deliveryHeader != deliveryID.String() {
		t.Fatalf("delivery header = %q", deliveryHeader)
	}
	if body != `{"event_type":"claim.approved"}` {
		t.Fatalf("body = %q", body)
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
