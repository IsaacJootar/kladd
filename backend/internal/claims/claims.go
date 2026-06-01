package claims

import (
	"context"
	"errors"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

const (
	StatusActive  = "active"
	StatusExpired = "expired"
	StatusRevoked = "revoked"
)

var (
	ErrInvalidUser   = errors.New("user_id is required")
	ErrClaimNotFound = errors.New("claim not found")
)

type Claim struct {
	ID             uuid.UUID                  `json:"id"`
	ClaimRequestID uuid.UUID                  `json:"claim_request_id"`
	Organization   claimrequests.Organization `json:"organization"`
	Purpose        string                     `json:"purpose"`
	ApprovedTruths []string                   `json:"approved_truths,omitempty"`
	Status         string                     `json:"status"`
	IssuedAt       time.Time                  `json:"issued_at"`
	ExpiresAt      time.Time                  `json:"expires_at"`
	RevokedAt      *time.Time                 `json:"revoked_at,omitempty"`
	DetailsVisible bool                       `json:"details_visible"`
}

type Store interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error)
	GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) Service {
	return Service{
		store: store,
		now:   time.Now,
	}
}

func NewServiceWithClock(store Store, now func() time.Time) Service {
	return Service{
		store: store,
		now:   now,
	}
}

func (service Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidUser
	}

	claimList, err := service.store.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return service.sanitizeClaims(claimList), nil
}

func (service Service) GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	if userID == uuid.Nil {
		return Claim{}, ErrInvalidUser
	}
	if claimID == uuid.Nil {
		return Claim{}, ErrClaimNotFound
	}

	claim, err := service.store.GetForUser(ctx, userID, claimID)
	if err != nil {
		return Claim{}, err
	}

	return service.sanitizeClaim(claim), nil
}

func (service Service) sanitizeClaims(claimList []Claim) []Claim {
	sanitized := make([]Claim, 0, len(claimList))
	for _, claim := range claimList {
		sanitized = append(sanitized, service.sanitizeClaim(claim))
	}

	return sanitized
}

func (service Service) sanitizeClaim(claim Claim) Claim {
	effectiveStatus := claim.Status
	if effectiveStatus == StatusActive && !claim.ExpiresAt.After(service.now()) {
		effectiveStatus = StatusExpired
	}

	claim.Status = effectiveStatus
	claim.DetailsVisible = effectiveStatus == StatusActive
	if !claim.DetailsVisible {
		claim.ApprovedTruths = nil
	}

	return claim
}
