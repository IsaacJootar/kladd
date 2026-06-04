package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingStore struct {
	records        []Record
	userID         uuid.UUID
	organizationID uuid.UUID
	limit          int
	err            error
}

func (store *recordingStore) ListForUser(ctx context.Context, userID uuid.UUID, limit int) ([]Record, error) {
	store.userID = userID
	store.limit = limit
	if store.err != nil {
		return nil, store.err
	}
	return store.records, nil
}

func (store *recordingStore) ListForOrganization(ctx context.Context, organizationID uuid.UUID, limit int) ([]Record, error) {
	store.organizationID = organizationID
	store.limit = limit
	if store.err != nil {
		return nil, store.err
	}
	return store.records, nil
}

func TestServiceListsSafeUserEvents(t *testing.T) {
	userID := uuid.New()
	store := &recordingStore{
		records: []Record{
			{
				ID:        uuid.New(),
				EventType: "evidence.created",
				Metadata: map[string]any{
					"category":  "degree_certificate",
					"file_path": "storage/private/raw.pdf",
				},
				CreatedAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:        uuid.New(),
				EventType: "claim_request.approved",
				Metadata: map[string]any{
					"claim_id": "hidden",
				},
				CreatedAt: time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	events, err := service.ListForUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	if store.userID != userID {
		t.Fatalf("user id = %s, want %s", store.userID, userID)
	}
	if store.limit != 20 {
		t.Fatalf("limit = %d, want 20", store.limit)
	}
	if events[0].Title != "Record added" {
		t.Fatalf("title = %q, want Record added", events[0].Title)
	}
	if !strings.Contains(events[0].Description, "degree certificate") {
		t.Fatalf("description = %q, want friendly category", events[0].Description)
	}
	if strings.Contains(events[0].Description, "file_path") || strings.Contains(events[0].Description, "storage/private") {
		t.Fatal("event description exposed storage details")
	}

	payload, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	for _, forbidden := range []string{"metadata", "file_path", "password", "security_pin", "security_pin_hash", "raw_document"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("events response exposed forbidden value %q", forbidden)
		}
	}
}

func TestServiceListsSafeOrganizationEvents(t *testing.T) {
	organizationID := uuid.New()
	store := &recordingStore{
		records: []Record{
			{
				ID:        uuid.New(),
				EventType: "claim_request.created",
				Metadata: map[string]any{
					"requested_truths": []any{"identity_verified"},
					"raw_document":     "hidden.pdf",
				},
				CreatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
			},
			{
				ID:        uuid.New(),
				EventType: "webhook.endpoint_configured",
				Metadata: map[string]any{
					"url":       "https://example.com/kladd/webhooks",
					"signature": "sha256=hidden",
				},
				CreatedAt: time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	events, err := service.ListForOrganization(context.Background(), organizationID)
	if err != nil {
		t.Fatalf("list organization events: %v", err)
	}

	if store.organizationID != organizationID {
		t.Fatalf("organization id = %s, want %s", store.organizationID, organizationID)
	}
	if store.limit != 20 {
		t.Fatalf("limit = %d, want 20", store.limit)
	}
	if events[1].Title != "Webhook endpoint saved" {
		t.Fatalf("title = %q, want Webhook endpoint saved", events[1].Title)
	}

	payload, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}
	for _, forbidden := range []string{"metadata", "raw_document", "signature", "security_pin", "api_key", "key_hash", "truth_value"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("events response exposed forbidden value %q", forbidden)
		}
	}
}

func TestServiceHandlesMixedAuditMetadataValues(t *testing.T) {
	service := NewService(&recordingStore{
		records: []Record{
			{
				ID:        uuid.New(),
				EventType: "security_pin.validation_succeeded",
				Metadata: map[string]any{
					"failed_attempts": float64(0),
					"locked":          false,
				},
				CreatedAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
			},
		},
	})

	events, err := service.ListForUser(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	if events[0].Title != "Security PIN confirmed" {
		t.Fatalf("title = %q, want Security PIN confirmed", events[0].Title)
	}
}

func TestServiceRequiresUser(t *testing.T) {
	service := NewService(&recordingStore{})

	_, err := service.ListForUser(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidUser)
	}
}

func TestServiceRequiresOrganization(t *testing.T) {
	service := NewService(&recordingStore{})

	_, err := service.ListForOrganization(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidOrganizationID) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidOrganizationID)
	}
}

func TestServiceReturnsStoreErrors(t *testing.T) {
	storeErr := errors.New("db down")
	service := NewService(&recordingStore{err: storeErr})

	_, err := service.ListForUser(context.Background(), uuid.New())
	if !errors.Is(err, storeErr) {
		t.Fatalf("error = %v, want %v", err, storeErr)
	}
}
