package webhooks

import (
	"context"
	"database/sql"
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

func (executor *recordingExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	executor.query = query
	executor.args = args
	return nil, executor.err
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
