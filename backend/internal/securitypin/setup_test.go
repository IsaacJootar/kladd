package securitypin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingSetupStore struct {
	userID  uuid.UUID
	pinHash string
	setAt   time.Time
	result  SetupResult
	err     error
}

func (store *recordingSetupStore) SetPIN(ctx context.Context, userID uuid.UUID, pinHash string, setAt time.Time) (SetupResult, error) {
	store.userID = userID
	store.pinHash = pinHash
	store.setAt = setAt
	if store.err != nil {
		return SetupResult{}, store.err
	}

	store.result = SetupResult{
		UserID: userID,
		Set:    true,
		SetAt:  setAt,
	}
	return store.result, nil
}

func TestSetupServiceHashesSecurityPIN(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	store := &recordingSetupStore{}
	service := NewSetupServiceWithClock(store, func() time.Time { return now })

	result, err := service.Setup(context.Background(), SetupInput{
		UserID: userID,
		PIN:    "4829",
	})
	if err != nil {
		t.Fatalf("setup pin: %v", err)
	}

	if result.UserID != userID {
		t.Fatalf("user id = %s, want %s", result.UserID, userID)
	}

	if !result.Set {
		t.Fatal("expected security pin to be marked as set")
	}

	if store.pinHash == "4829" {
		t.Fatal("security pin stored as raw text")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(store.pinHash), []byte("4829")); err != nil {
		t.Fatalf("pin hash does not match pin: %v", err)
	}

	if !store.setAt.Equal(now) {
		t.Fatalf("set at = %s, want %s", store.setAt, now)
	}
}

func TestSetupServiceValidatesPIN(t *testing.T) {
	service := NewSetupService(&recordingSetupStore{})

	_, err := service.Setup(context.Background(), SetupInput{
		UserID: uuid.New(),
		PIN:    "12a4",
	})
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidFormat)
	}
}

func TestSetupServiceReturnsStoreErrors(t *testing.T) {
	service := NewSetupService(&recordingSetupStore{err: ErrUserNotFound})

	_, err := service.Setup(context.Background(), SetupInput{
		UserID: uuid.New(),
		PIN:    "4829",
	})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrUserNotFound)
	}
}
