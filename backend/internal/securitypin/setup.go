package securitypin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type SetupInput struct {
	UserID uuid.UUID
	PIN    string
}

type SetupResult struct {
	UserID uuid.UUID `json:"user_id"`
	Set    bool      `json:"security_pin_set"`
	SetAt  time.Time `json:"security_pin_set_at"`
}

type SetupStore interface {
	SetPIN(ctx context.Context, userID uuid.UUID, pinHash string, setAt time.Time) (SetupResult, error)
}

type SetupService struct {
	store SetupStore
	now   func() time.Time
}

func NewSetupService(store SetupStore) SetupService {
	return SetupService{
		store: store,
		now:   time.Now,
	}
}

func NewSetupServiceWithClock(store SetupStore, now func() time.Time) SetupService {
	return SetupService{
		store: store,
		now:   now,
	}
}

func (service SetupService) Setup(ctx context.Context, input SetupInput) (SetupResult, error) {
	pinHash, err := Hash(input.PIN)
	if err != nil {
		return SetupResult{}, err
	}

	return service.store.SetPIN(ctx, input.UserID, pinHash, service.now())
}
