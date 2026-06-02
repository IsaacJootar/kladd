package securitypin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidPassword = errors.New("password is incorrect")
)

type ResetInput struct {
	UserID   uuid.UUID
	Password string
	PIN      string
}

type ResetStore interface {
	GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error)
	ResetPIN(ctx context.Context, userID uuid.UUID, pinHash string, resetAt time.Time) (SetupResult, error)
}

type ResetService struct {
	store ResetStore
	now   func() time.Time
}

func NewResetService(store ResetStore) ResetService {
	return ResetService{
		store: store,
		now:   time.Now,
	}
}

func NewResetServiceWithClock(store ResetStore, now func() time.Time) ResetService {
	return ResetService{
		store: store,
		now:   now,
	}
}

func (service ResetService) Reset(ctx context.Context, input ResetInput) (SetupResult, error) {
	if input.UserID == uuid.Nil {
		return SetupResult{}, ErrInvalidUser
	}
	if input.Password == "" {
		return SetupResult{}, ErrInvalidPassword
	}

	pinHash, err := Hash(input.PIN)
	if err != nil {
		return SetupResult{}, err
	}

	passwordHash, err := service.store.GetPasswordHash(ctx, input.UserID)
	if err != nil {
		return SetupResult{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		return SetupResult{}, ErrInvalidPassword
	}

	return service.store.ResetPIN(ctx, input.UserID, pinHash, service.now())
}
