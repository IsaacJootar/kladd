package claims

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

type recordingStore struct {
	claim          Claim
	claims         []Claim
	err            error
	userID         uuid.UUID
	organizationID uuid.UUID
	claimID        uuid.UUID
	statusID       uuid.UUID
	retrieved      time.Time
	revokedAt      time.Time
	pinHash        string
	pinExpiry      time.Time
	pinCreated     time.Time
	expiredAt      time.Time
}

func (store *recordingStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error) {
	store.userID = userID
	if store.err != nil {
		return nil, store.err
	}
	return store.claims, nil
}

func (store *recordingStore) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]Claim, error) {
	store.organizationID = organizationID
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

func (store *recordingStore) GetStatus(ctx context.Context, claimID uuid.UUID, retrievedAt time.Time) (Claim, error) {
	store.statusID = claimID
	store.retrieved = retrievedAt
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

func (store *recordingStore) CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, pinHash string, expiresAt time.Time, createdAt time.Time) (ExchangePIN, error) {
	store.userID = userID
	store.claimID = claimID
	store.pinHash = pinHash
	store.pinExpiry = expiresAt
	store.pinCreated = createdAt
	if store.err != nil {
		return ExchangePIN{}, store.err
	}

	return ExchangePIN{
		ClaimID:   claimID,
		ExpiresAt: expiresAt,
	}, nil
}

func (store *recordingStore) ResolveExchangePIN(ctx context.Context, pinHash string, retrievedAt time.Time) (Claim, error) {
	store.pinHash = pinHash
	store.retrieved = retrievedAt
	if store.err != nil {
		return Claim{}, store.err
	}

	return store.claim, nil
}

func (store *recordingStore) ExpireDue(ctx context.Context, expiredAt time.Time) ([]Claim, error) {
	store.expiredAt = expiredAt
	if store.err != nil {
		return nil, store.err
	}
	return store.claims, nil
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

func TestServiceListForOrganizationHidesDetailsForExpiredClaims(t *testing.T) {
	organizationID := uuid.New()
	store := &recordingStore{
		claims: []Claim{
			{
				ID:             uuid.New(),
				ApprovedTruths: []string{"identity_verified"},
				Status:         StatusActive,
				ExpiresAt:      time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claims, err := service.ListForOrganization(context.Background(), organizationID)
	if err != nil {
		t.Fatalf("list organization claims: %v", err)
	}

	if store.organizationID != organizationID {
		t.Fatalf("organization id = %s, want %s", store.organizationID, organizationID)
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

func TestServiceListForOrganizationValidatesOrganization(t *testing.T) {
	service := NewService(&recordingStore{})

	_, err := service.ListForOrganization(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidOrganizationID) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidOrganizationID)
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

func TestServiceGetStatusShowsDetailsForActiveClaim(t *testing.T) {
	claimID := uuid.New()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{
		claim: Claim{
			ID:             claimID,
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusActive,
			ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		},
	}
	service := NewServiceWithClock(store, func() time.Time {
		return now
	})

	claim, err := service.GetStatus(context.Background(), claimID)
	if err != nil {
		t.Fatalf("get claim status: %v", err)
	}

	if store.statusID != claimID {
		t.Fatalf("claim id = %s, want %s", store.statusID, claimID)
	}
	if !store.retrieved.Equal(now) {
		t.Fatalf("retrieved at = %s, want %s", store.retrieved, now)
	}
	if !claim.DetailsVisible {
		t.Fatal("active claim details should be visible")
	}
	if len(claim.ApprovedTruths) != 1 {
		t.Fatal("active claim should include approved truths")
	}
}

func TestServiceGetStatusHidesDetailsForRevokedClaim(t *testing.T) {
	service := NewServiceWithClock(&recordingStore{
		claim: Claim{
			ID:             uuid.New(),
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusRevoked,
			ExpiresAt:      time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
		},
	}, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claim, err := service.GetStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("get claim status: %v", err)
	}

	if claim.DetailsVisible {
		t.Fatal("revoked claim details should not be visible")
	}
	if len(claim.ApprovedTruths) != 0 {
		t.Fatal("revoked claim exposed approved truths")
	}
}

func TestServiceGetStatusHidesDetailsForExpiredClaim(t *testing.T) {
	service := NewServiceWithClock(&recordingStore{
		claim: Claim{
			ID:             uuid.New(),
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusActive,
			ExpiresAt:      time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
		},
	}, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	claim, err := service.GetStatus(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("get claim status: %v", err)
	}

	if claim.Status != StatusExpired {
		t.Fatalf("status = %q, want %q", claim.Status, StatusExpired)
	}
	if claim.DetailsVisible {
		t.Fatal("expired claim details should not be visible")
	}
	if len(claim.ApprovedTruths) != 0 {
		t.Fatal("expired claim exposed approved truths")
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

func TestServiceCreateExchangePINGeneratesTemporaryPIN(t *testing.T) {
	userID := uuid.New()
	claimID := uuid.New()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{}
	service := NewServiceWithClock(store, func() time.Time {
		return now
	})

	exchangePIN, err := service.CreateExchangePIN(context.Background(), userID, claimID)
	if err != nil {
		t.Fatalf("create exchange pin: %v", err)
	}

	if store.userID != userID {
		t.Fatalf("user id = %s, want %s", store.userID, userID)
	}
	if store.claimID != claimID {
		t.Fatalf("claim id = %s, want %s", store.claimID, claimID)
	}
	if len(exchangePIN.ExchangePIN) != 8 {
		t.Fatalf("pin length = %d, want 8", len(exchangePIN.ExchangePIN))
	}
	if !validExchangePIN(exchangePIN.ExchangePIN) {
		t.Fatalf("generated invalid exchange pin %q", exchangePIN.ExchangePIN)
	}
	if store.pinHash == exchangePIN.ExchangePIN {
		t.Fatal("store received raw exchange pin instead of hash")
	}
	if !store.pinCreated.Equal(now) {
		t.Fatalf("created at = %s, want %s", store.pinCreated, now)
	}
	if !store.pinExpiry.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expires at = %s, want %s", store.pinExpiry, now.Add(15*time.Minute))
	}
}

func TestServiceResolveExchangePINUsesStatusSanitization(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{
		claim: Claim{
			ID:             uuid.New(),
			ApprovedTruths: []string{"identity_verified"},
			Status:         StatusActive,
			ExpiresAt:      time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
		},
	}
	service := NewServiceWithClock(store, func() time.Time {
		return now
	})

	claim, err := service.ResolveExchangePIN(context.Background(), "123456")
	if err != nil {
		t.Fatalf("resolve exchange pin: %v", err)
	}

	if store.pinHash == "123456" {
		t.Fatal("store received raw exchange pin instead of hash")
	}
	if !store.retrieved.Equal(now) {
		t.Fatalf("retrieved at = %s, want %s", store.retrieved, now)
	}
	if claim.Status != StatusExpired {
		t.Fatalf("status = %q, want %q", claim.Status, StatusExpired)
	}
	if claim.DetailsVisible {
		t.Fatal("expired claim details should not be visible")
	}
	if len(claim.ApprovedTruths) != 0 {
		t.Fatal("expired claim exposed approved truths")
	}
}

func TestServiceResolveExchangePINRejectsInvalidPIN(t *testing.T) {
	service := NewServiceWithClock(&recordingStore{}, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	_, err := service.ResolveExchangePIN(context.Background(), "12ab")
	if !errors.Is(err, ErrInvalidExchangePIN) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidExchangePIN)
	}
}

func TestServiceExpireDueHidesExpiredClaimDetails(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &recordingStore{
		claims: []Claim{
			{
				ID:             uuid.New(),
				ApprovedTruths: []string{"identity_verified"},
				Status:         StatusExpired,
				ExpiresAt:      time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewServiceWithClock(store, func() time.Time {
		return now
	})

	expiredClaims, err := service.ExpireDue(context.Background())
	if err != nil {
		t.Fatalf("expire due claims: %v", err)
	}

	if !store.expiredAt.Equal(now) {
		t.Fatalf("expired at = %s, want %s", store.expiredAt, now)
	}
	if len(expiredClaims) != 1 {
		t.Fatalf("expired claims = %d, want 1", len(expiredClaims))
	}
	if expiredClaims[0].DetailsVisible {
		t.Fatal("expired claim details should not be visible")
	}
	if len(expiredClaims[0].ApprovedTruths) != 0 {
		t.Fatal("expired claim exposed approved truths")
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
