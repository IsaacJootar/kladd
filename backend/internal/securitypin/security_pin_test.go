package securitypin

import (
	"testing"
	"time"
)

func TestValidateAcceptsFourToSixDigits(t *testing.T) {
	validPins := []string{"1234", "48291", "123456"}

	for _, pin := range validPins {
		t.Run(pin, func(t *testing.T) {
			if err := Validate(pin); err != nil {
				t.Fatalf("expected valid pin, got %v", err)
			}
		})
	}
}

func TestValidateRejectsInvalidPins(t *testing.T) {
	invalidPins := []string{"", "123", "1234567", "12a4", "12 4"}

	for _, pin := range invalidPins {
		t.Run(pin, func(t *testing.T) {
			if err := Validate(pin); err == nil {
				t.Fatal("expected invalid pin error")
			}
		})
	}
}

func TestHashAndCompare(t *testing.T) {
	hash, err := Hash("4829")
	if err != nil {
		t.Fatalf("hash pin: %v", err)
	}

	if hash == "4829" {
		t.Fatal("security pin was stored as raw text")
	}

	if !Compare(hash, "4829") {
		t.Fatal("expected hash to match pin")
	}

	if Compare(hash, "1111") {
		t.Fatal("expected hash mismatch")
	}
}

func TestHashRejectsInvalidPin(t *testing.T) {
	if _, err := Hash("12a4"); err == nil {
		t.Fatal("expected invalid pin error")
	}
}

func TestNextFailureLocksAtFiveAttempts(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

	decision := NextFailure(4, now)

	if decision.FailedAttempts != MaxFailedAttempts {
		t.Fatalf("failed attempts = %d, want %d", decision.FailedAttempts, MaxFailedAttempts)
	}

	if decision.LockedUntil == nil {
		t.Fatal("expected lockout after fifth failed attempt")
	}

	if want := now.Add(LockoutDuration); !decision.LockedUntil.Equal(want) {
		t.Fatalf("locked until = %s, want %s", decision.LockedUntil, want)
	}
}

func TestNextFailureDoesNotLockBeforeFiveAttempts(t *testing.T) {
	decision := NextFailure(3, time.Now())

	if decision.FailedAttempts != 4 {
		t.Fatalf("failed attempts = %d, want 4", decision.FailedAttempts)
	}

	if decision.LockedUntil != nil {
		t.Fatal("did not expect lockout before fifth failed attempt")
	}
}

func TestIsLocked(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	if !IsLocked(&future, now) {
		t.Fatal("expected future lock time to be locked")
	}

	if IsLocked(&past, now) {
		t.Fatal("expected past lock time to be unlocked")
	}

	if IsLocked(nil, now) {
		t.Fatal("expected nil lock time to be unlocked")
	}
}

func TestResetAfterSuccess(t *testing.T) {
	decision := ResetAfterSuccess()

	if decision.FailedAttempts != 0 {
		t.Fatalf("failed attempts = %d, want 0", decision.FailedAttempts)
	}

	if decision.LockedUntil != nil {
		t.Fatal("expected lockout to be cleared")
	}
}
