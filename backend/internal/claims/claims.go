package claims

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
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
	ErrInvalidUser           = errors.New("user_id is required")
	ErrInvalidOrganizationID = errors.New("organization_id is required")
	ErrClaimNotFound         = errors.New("claim not found")
	ErrClaimNotActive        = errors.New("claim is not active")
	ErrInvalidExchangePIN    = errors.New("exchange pin must be 6-8 digits")
	ErrExchangePINNotFound   = errors.New("exchange pin not found")
	ErrExchangePINGeneration = errors.New("unable to generate exchange pin")
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

type ExchangePIN struct {
	ClaimID     uuid.UUID `json:"claim_id"`
	ExchangePIN string    `json:"exchange_pin"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Store interface {
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error)
	ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]Claim, error)
	GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error)
	GetStatus(ctx context.Context, claimID uuid.UUID, retrievedAt time.Time) (Claim, error)
	Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, revokedAt time.Time) (Claim, error)
	CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, pinHash string, expiresAt time.Time, createdAt time.Time) (ExchangePIN, error)
	ResolveExchangePIN(ctx context.Context, pinHash string, retrievedAt time.Time) (Claim, error)
	ExpireDue(ctx context.Context, expiredAt time.Time) ([]Claim, error)
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

func (service Service) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]Claim, error) {
	if organizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	claimList, err := service.store.ListForOrganization(ctx, organizationID)
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

func (service Service) GetStatus(ctx context.Context, claimID uuid.UUID) (Claim, error) {
	if claimID == uuid.Nil {
		return Claim{}, ErrClaimNotFound
	}

	claim, err := service.store.GetStatus(ctx, claimID, service.now())
	if err != nil {
		return Claim{}, err
	}

	return service.sanitizeClaim(claim), nil
}

func (service Service) Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	if userID == uuid.Nil {
		return Claim{}, ErrInvalidUser
	}
	if claimID == uuid.Nil {
		return Claim{}, ErrClaimNotFound
	}

	claim, err := service.store.Revoke(ctx, userID, claimID, service.now())
	if err != nil {
		return Claim{}, err
	}

	return service.sanitizeClaim(claim), nil
}

func (service Service) CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (ExchangePIN, error) {
	if userID == uuid.Nil {
		return ExchangePIN{}, ErrInvalidUser
	}
	if claimID == uuid.Nil {
		return ExchangePIN{}, ErrClaimNotFound
	}

	pin, err := generateExchangePIN()
	if err != nil {
		return ExchangePIN{}, ErrExchangePINGeneration
	}

	now := service.now()
	exchangePIN, err := service.store.CreateExchangePIN(
		ctx,
		userID,
		claimID,
		hashExchangePIN(pin),
		now.Add(15*time.Minute),
		now,
	)
	if err != nil {
		return ExchangePIN{}, err
	}

	exchangePIN.ExchangePIN = pin
	return exchangePIN, nil
}

func (service Service) ResolveExchangePIN(ctx context.Context, pin string) (Claim, error) {
	if !validExchangePIN(pin) {
		return Claim{}, ErrInvalidExchangePIN
	}

	claim, err := service.store.ResolveExchangePIN(ctx, hashExchangePIN(pin), service.now())
	if err != nil {
		return Claim{}, err
	}

	return service.sanitizeClaim(claim), nil
}

func (service Service) ExpireDue(ctx context.Context) ([]Claim, error) {
	expiredClaims, err := service.store.ExpireDue(ctx, service.now())
	if err != nil {
		return nil, err
	}

	return service.sanitizeClaims(expiredClaims), nil
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

func generateExchangePIN() (string, error) {
	const digits = 100000000
	value, err := rand.Int(rand.Reader, big.NewInt(digits))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%08d", value.Int64()), nil
}

func validExchangePIN(pin string) bool {
	if len(pin) < 6 || len(pin) > 8 {
		return false
	}

	for _, char := range pin {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

func hashExchangePIN(pin string) string {
	sum := sha256.Sum256([]byte("kladd-exchange-pin:" + pin))
	return hex.EncodeToString(sum[:])
}
