package securitypin

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinLength         = 4
	MaxLength         = 6
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

var (
	ErrInvalidFormat = errors.New("security pin must be 4-6 digits")
)

type LockoutDecision struct {
	FailedAttempts int
	LockedUntil    *time.Time
}

func Validate(pin string) error {
	length := len(pin)
	if length < MinLength || length > MaxLength {
		return ErrInvalidFormat
	}

	for _, char := range pin {
		if char < '0' || char > '9' {
			return ErrInvalidFormat
		}
	}

	return nil
}

func Hash(pin string) (string, error) {
	if err := Validate(pin); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func Compare(hash, pin string) bool {
	if hash == "" {
		return false
	}

	if err := Validate(pin); err != nil {
		return false
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)) == nil
}

func NextFailure(failedAttempts int, now time.Time) LockoutDecision {
	nextAttempts := failedAttempts + 1
	decision := LockoutDecision{
		FailedAttempts: nextAttempts,
	}

	if nextAttempts >= MaxFailedAttempts {
		lockedUntil := now.Add(LockoutDuration)
		decision.LockedUntil = &lockedUntil
	}

	return decision
}

func IsLocked(lockedUntil *time.Time, now time.Time) bool {
	return lockedUntil != nil && lockedUntil.After(now)
}

func ResetAfterSuccess() LockoutDecision {
	return LockoutDecision{
		FailedAttempts: 0,
		LockedUntil:    nil,
	}
}
