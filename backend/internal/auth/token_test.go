package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenManagerIssueAndVerify(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	manager := NewTokenManagerWithClock("test-secret", time.Hour, func() time.Time { return now })
	userID := uuid.New()

	token, expiresAt, err := manager.Issue(userID)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if !expiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires at = %s, want %s", expiresAt, now.Add(time.Hour))
	}

	verifiedUserID, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	if verifiedUserID != userID {
		t.Fatalf("verified user = %s, want %s", verifiedUserID, userID)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	issuedAt := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	now := issuedAt
	manager := NewTokenManagerWithClock("test-secret", time.Hour, func() time.Time { return now })

	token, _, err := manager.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	now = issuedAt.Add(2 * time.Hour)
	if _, err := manager.Verify(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestTokenManagerRejectsWrongSecret(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	manager := NewTokenManagerWithClock("test-secret", time.Hour, func() time.Time { return now })
	token, _, err := manager.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	otherManager := NewTokenManagerWithClock("other-secret", time.Hour, func() time.Time { return now })
	if _, err := otherManager.Verify(token); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}
