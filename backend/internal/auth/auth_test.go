package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingAuthStore struct {
	credentials Credentials
	findErr     error
	recordErr   error
	email       string
	loggedUser  users.User
}

func (store *recordingAuthStore) FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	store.email = email
	if store.findErr != nil {
		return Credentials{}, store.findErr
	}
	return store.credentials, nil
}

func (store *recordingAuthStore) RecordLogin(ctx context.Context, user users.User) error {
	store.loggedUser = user
	return store.recordErr
}

func TestServiceLoginIssuesToken(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	userID := uuid.New()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := &recordingAuthStore{
		credentials: Credentials{
			User: users.User{
				ID:                 userID,
				Name:               "Ada Lovelace",
				Email:              "ada@example.com",
				AccountType:        users.AccountTypeIndividual,
				VerificationStatus: users.VerificationStatusUnverified,
			},
			PasswordHash: string(passwordHash),
		},
	}
	tokenManager := NewTokenManagerWithClock("test-secret", time.Hour, func() time.Time { return now })
	service := NewService(store, tokenManager)

	result, err := service.Login(context.Background(), LoginInput{
		Email:    " ADA@Example.COM ",
		Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if store.email != "ada@example.com" {
		t.Fatalf("email = %q, want normalized email", store.email)
	}

	if result.AccessToken == "" {
		t.Fatal("expected access token")
	}

	if result.TokenType != TokenTypeBearer {
		t.Fatalf("token type = %q, want %q", result.TokenType, TokenTypeBearer)
	}

	if !result.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires at = %s, want %s", result.ExpiresAt, now.Add(time.Hour))
	}

	authenticatedUserID, err := service.Authenticate(result.AccessToken)
	if err != nil {
		t.Fatalf("authenticate token: %v", err)
	}

	if authenticatedUserID != userID {
		t.Fatalf("authenticated user = %s, want %s", authenticatedUserID, userID)
	}

	if store.loggedUser.ID != userID {
		t.Fatalf("logged user = %s, want %s", store.loggedUser.ID, userID)
	}
}

func TestServiceLoginRejectsInvalidCredentials(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name  string
		input LoginInput
		store *recordingAuthStore
	}{
		{
			name:  "invalid email",
			input: LoginInput{Email: "not-email", Password: "strong-password"},
			store: &recordingAuthStore{},
		},
		{
			name:  "missing password",
			input: LoginInput{Email: "ada@example.com"},
			store: &recordingAuthStore{},
		},
		{
			name:  "unknown email",
			input: LoginInput{Email: "ada@example.com", Password: "strong-password"},
			store: &recordingAuthStore{findErr: ErrInvalidCredentials},
		},
		{
			name:  "wrong password",
			input: LoginInput{Email: "ada@example.com", Password: "wrong-password"},
			store: &recordingAuthStore{credentials: Credentials{PasswordHash: string(passwordHash)}},
		},
	}

	service := NewService(nil, NewTokenManager("test-secret", time.Hour))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service.store = test.store
			_, err := service.Login(context.Background(), test.input)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidCredentials)
			}
		})
	}
}

func TestServiceLoginReturnsAuditError(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("strong-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	store := &recordingAuthStore{
		credentials: Credentials{
			User:         users.User{ID: uuid.New(), Email: "ada@example.com"},
			PasswordHash: string(passwordHash),
		},
		recordErr: errors.New("audit failed"),
	}
	service := NewService(store, NewTokenManager("test-secret", time.Hour))

	_, err = service.Login(context.Background(), LoginInput{
		Email:    "ada@example.com",
		Password: "strong-password",
	})
	if err == nil {
		t.Fatal("expected audit error")
	}
}
