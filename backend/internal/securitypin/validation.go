package securitypin

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPINNotSet   = errors.New("security pin is not set")
	ErrInvalidPIN  = errors.New("security pin is incorrect")
	ErrPINLocked   = errors.New("security pin is temporarily locked")
	ErrInvalidUser = errors.New("user_id is required")
)

type ValidationState struct {
	PINHash        string
	FailedAttempts int
	LockedUntil    *time.Time
}

type ValidationStore interface {
	GetValidationState(ctx context.Context, userID uuid.UUID) (ValidationState, error)
	RecordValidationFailure(ctx context.Context, userID uuid.UUID, decision LockoutDecision) error
	RecordValidationSuccess(ctx context.Context, userID uuid.UUID) error
}

type ValidationService struct {
	store ValidationStore
	now   func() time.Time
}

func NewValidationService(store ValidationStore) ValidationService {
	return ValidationService{
		store: store,
		now:   time.Now,
	}
}

func NewValidationServiceWithClock(store ValidationStore, now func() time.Time) ValidationService {
	return ValidationService{
		store: store,
		now:   now,
	}
}

func (service ValidationService) Validate(ctx context.Context, userID uuid.UUID, pin string) error {
	if userID == uuid.Nil {
		return ErrInvalidUser
	}

	state, err := service.store.GetValidationState(ctx, userID)
	if err != nil {
		return err
	}
	if state.PINHash == "" {
		return ErrPINNotSet
	}

	now := service.now()
	if IsLocked(state.LockedUntil, now) {
		return ErrPINLocked
	}

	if !Compare(state.PINHash, pin) {
		decision := NextFailure(state.FailedAttempts, now)
		if err := service.store.RecordValidationFailure(ctx, userID, decision); err != nil {
			return err
		}
		if decision.LockedUntil != nil {
			return ErrPINLocked
		}
		return ErrInvalidPIN
	}

	return service.store.RecordValidationSuccess(ctx, userID)
}
