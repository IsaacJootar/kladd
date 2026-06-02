package securitypin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingResetStore struct {
	passwordHash string
	userID       uuid.UUID
	pinHash      string
	resetAt      time.Time
	err          error
}

func (store *recordingResetStore) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	if store.err != nil {
		return "", store.err
	}
	return store.passwordHash, nil
}

func (store *recordingResetStore) ResetPIN(ctx context.Context, userID uuid.UUID, pinHash string, resetAt time.Time) (SetupResult, error) {
	store.userID = userID
	store.pinHash = pinHash
	store.resetAt = resetAt
	return SetupResult{
		UserID: userID,
		Set:    true,
		SetAt:  resetAt,
	}, nil
}

func TestResetServiceRequiresPasswordBeforeResettingPIN(t *testing.T) {
	userID := uuid.New()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("account-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := &recordingResetStore{passwordHash: string(passwordHash)}
	service := NewResetServiceWithClock(store, func() time.Time { return now })

	result, err := service.Reset(context.Background(), ResetInput{
		UserID:   userID,
		Password: "account-password",
		PIN:      "7391",
	})
	if err != nil {
		t.Fatalf("reset pin: %v", err)
	}

	if result.UserID != userID {
		t.Fatalf("user id = %s, want %s", result.UserID, userID)
	}
	if !result.Set {
		t.Fatal("expected pin to be marked as set")
	}
	if store.pinHash == "7391" {
		t.Fatal("security pin stored as raw text")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.pinHash), []byte("7391")); err != nil {
		t.Fatalf("pin hash does not match pin: %v", err)
	}
	if !store.resetAt.Equal(now) {
		t.Fatalf("reset at = %s, want %s", store.resetAt, now)
	}
}

func TestResetServiceRejectsIncorrectPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("account-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := &recordingResetStore{passwordHash: string(passwordHash)}
	service := NewResetService(store)

	_, err = service.Reset(context.Background(), ResetInput{
		UserID:   uuid.New(),
		Password: "wrong-password",
		PIN:      "7391",
	})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidPassword)
	}
	if store.pinHash != "" {
		t.Fatal("pin was reset after incorrect password")
	}
}

func TestResetServiceValidatesNewPIN(t *testing.T) {
	service := NewResetService(&recordingResetStore{})

	_, err := service.Reset(context.Background(), ResetInput{
		UserID:   uuid.New(),
		Password: "account-password",
		PIN:      "12a4",
	})
	if !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidFormat)
	}
}

func TestResetServiceRequiresUser(t *testing.T) {
	service := NewResetService(&recordingResetStore{})

	_, err := service.Reset(context.Background(), ResetInput{
		Password: "account-password",
		PIN:      "7391",
	})
	if !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidUser)
	}
}
