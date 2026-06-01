package securitypin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type validationRecordingStore struct {
	state           ValidationState
	err             error
	failureDecision LockoutDecision
	successUserID   uuid.UUID
	failureUserID   uuid.UUID
}

func (store *validationRecordingStore) GetValidationState(ctx context.Context, userID uuid.UUID) (ValidationState, error) {
	if store.err != nil {
		return ValidationState{}, store.err
	}
	return store.state, nil
}

func (store *validationRecordingStore) RecordValidationFailure(ctx context.Context, userID uuid.UUID, decision LockoutDecision) error {
	store.failureUserID = userID
	store.failureDecision = decision
	return nil
}

func (store *validationRecordingStore) RecordValidationSuccess(ctx context.Context, userID uuid.UUID) error {
	store.successUserID = userID
	return nil
}

func TestValidationServiceValidatesCorrectPIN(t *testing.T) {
	pinHash, err := Hash("4829")
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}
	userID := uuid.New()
	store := &validationRecordingStore{
		state: ValidationState{PINHash: pinHash, FailedAttempts: 2},
	}
	service := NewValidationServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	if err := service.Validate(context.Background(), userID, "4829"); err != nil {
		t.Fatalf("validate pin: %v", err)
	}
	if store.successUserID != userID {
		t.Fatalf("success user = %s, want %s", store.successUserID, userID)
	}
}

func TestValidationServiceRecordsFailure(t *testing.T) {
	pinHash, err := Hash("4829")
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}
	userID := uuid.New()
	store := &validationRecordingStore{
		state: ValidationState{PINHash: pinHash, FailedAttempts: 1},
	}
	service := NewValidationServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	err = service.Validate(context.Background(), userID, "1111")
	if !errors.Is(err, ErrInvalidPIN) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidPIN)
	}
	if store.failureUserID != userID {
		t.Fatalf("failure user = %s, want %s", store.failureUserID, userID)
	}
	if store.failureDecision.FailedAttempts != 2 {
		t.Fatalf("failed attempts = %d, want 2", store.failureDecision.FailedAttempts)
	}
}

func TestValidationServiceLocksAfterRepeatedFailures(t *testing.T) {
	pinHash, err := Hash("4829")
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}
	store := &validationRecordingStore{
		state: ValidationState{PINHash: pinHash, FailedAttempts: MaxFailedAttempts - 1},
	}
	service := NewValidationServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	err = service.Validate(context.Background(), uuid.New(), "1111")
	if !errors.Is(err, ErrPINLocked) {
		t.Fatalf("error = %v, want %v", err, ErrPINLocked)
	}
	if store.failureDecision.LockedUntil == nil {
		t.Fatal("expected lockout time after repeated failures")
	}
}

func TestValidationServiceRejectsLockedPIN(t *testing.T) {
	pinHash, err := Hash("4829")
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}
	lockedUntil := time.Date(2026, 6, 1, 12, 10, 0, 0, time.UTC)
	store := &validationRecordingStore{
		state: ValidationState{PINHash: pinHash, LockedUntil: &lockedUntil},
	}
	service := NewValidationServiceWithClock(store, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})

	err = service.Validate(context.Background(), uuid.New(), "4829")
	if !errors.Is(err, ErrPINLocked) {
		t.Fatalf("error = %v, want %v", err, ErrPINLocked)
	}
}

func TestValidationServiceRequiresPINSetup(t *testing.T) {
	service := NewValidationService(&validationRecordingStore{})

	err := service.Validate(context.Background(), uuid.New(), "4829")
	if !errors.Is(err, ErrPINNotSet) {
		t.Fatalf("error = %v, want %v", err, ErrPINNotSet)
	}
}
