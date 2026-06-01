package claims

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

type recordingStore struct {
	claim     Claim
	claims    []Claim
	err       error
	userID    uuid.UUID
	claimID   uuid.UUID
	revokedAt time.Time
}

func (store *recordingStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.claims, nil
}

func (store *recordingStore) GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	if store.err != nil {
		return Claim{}, store.err
	}
	return store.claim, nil
}

func (store *recordingStore) Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, revokedAt time.Time) (Claim, error) {
	store.userID = userID
	store.claimID = claimID
	store.revokedAt = revokedAt
	if store.err != nil {
		return Claim{}, store.err
	}

	claim := store.claim
	claim.Status = StatusRevoked
	claim.RevokedAt = &revokedAt
	return claim, nil
}

func TestServiceListHidesDetailsForExpiredClaims(t *testing.T) {
	service := NewServiceWithClock(&recordingStore{
		claims: []Claim{
			{
				ID:             uuid.New(),
				ApprovedTruths: []string{"identity_verified"},
				Status:         StatusActive,
				ExpiresAt:      time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
			},
		},
	}, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claims, err := service.ListForUser(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}

	if claims[0].Status != StatusExpired {
		t.Fatalf("status = %q, want %q", claims[0].Status, StatusExpired)
	}
	if claims[0].DetailsVisible {
		t.Fatal("expired claim details should not be visible")
	}
	if len(claims[0].ApprovedTruths) != 0 {
		t.Fatal("expired claim exposed approved truths")
	}
}

func TestServiceGetShowsDetailsForActiveClaim(t *testing.T) {
	service := NewServiceWithClock(&recordingStore{
		claim: Claim{
			ID:             uuid.New(),
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusActive,
			ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		},
	}, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claim, err := service.GetForUser(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}

	if !claim.DetailsVisible {
		t.Fatal("active claim details should be visible")
	}
	if len(claim.ApprovedTruths) != 1 {
		t.Fatal("active claim should include approved truths")
	}
}

func TestServiceRevokeHidesClaimDetails(t *testing.T) {
	userID := uuid.New()
	claimID := uuid.New()
	store := &recordingStore{
		claim: Claim{
			ID:             claimID,
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusActive,
			ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		},
	}
	service := NewServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claim, err := service.Revoke(context.Background(), userID, claimID)
	if err != nil {
		t.Fatalf("revoke claim: %v", err)
	}

	if store.userID != userID {
		t.Fatalf("user id = %s, want %s", store.userID, userID)
	}
	if store.claimID != claimID {
		t.Fatalf("claim id = %s, want %s", store.claimID, claimID)
	}
	if store.revokedAt.IsZero() {
		t.Fatal("expected revoked time")
	}
	if claim.Status != StatusRevoked {
		t.Fatalf("status = %q, want %q", claim.Status, StatusRevoked)
	}
	if claim.DetailsVisible {
		t.Fatal("revoked claim details should not be visible")
	}
	if len(claim.ApprovedTruths) != 0 {
		t.Fatal("revoked claim exposed approved truths")
	}
}

func TestClaimJSONDoesNotExposeForbiddenFields(t *testing.T) {
	payload, err := json.Marshal(Claim{
		ID:             uuid.New(),
		ClaimRequestID: uuid.New(),
		Organization:   claimrequests.Organization{ID: uuid.New(), Name: "Acme Bank"},
		Purpose:        "Employment onboarding",
		Status:         StatusActive,
		ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal claim: %v", err)
	}

	body := string(payload)
	for _, forbidden := range []string{"raw_document", "file_path", "security_pin", "security_pin_hash", "bvn", "nin", "passport_number", "tax_id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("claim response exposed forbidden field %q", forbidden)
		}
	}
}
