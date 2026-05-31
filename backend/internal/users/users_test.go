package users

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type recordingStore struct {
	record CreateRecord
	user   User
	err    error
}

func (store *recordingStore) Create(ctx context.Context, record CreateRecord) (User, error) {
	store.record = record
	if store.err != nil {
		return User{}, store.err
	}
	store.user = User{
		ID:                 record.ID,
		Name:               record.Name,
		Email:              record.Email,
		Phone:              record.Phone,
		AccountType:        record.AccountType,
		VerificationStatus: record.VerificationStatus,
	}
	return store.user, nil
}

func (store *recordingStore) Get(ctx context.Context, id uuid.UUID) (User, error) {
	if store.err != nil {
		return User{}, store.err
	}
	return store.user, nil
}

func TestServiceCreatePreparesUserRecord(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)

	user, err := service.Create(context.Background(), CreateInput{
		Name:        " Ada Lovelace ",
		Email:       "ADA@Example.COM ",
		Phone:       " 08030000000 ",
		Password:    "strong-password",
		AccountType: AccountTypeIndividual,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if user.Email != "ada@example.com" {
		t.Fatalf("email = %q, want normalized email", user.Email)
	}

	if store.record.PasswordHash == "strong-password" {
		t.Fatal("password stored as raw text")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(store.record.PasswordHash), []byte("strong-password")); err != nil {
		t.Fatalf("password hash does not match password: %v", err)
	}

	if user.VerificationStatus != VerificationStatusUnverified {
		t.Fatalf("verification status = %q, want %q", user.VerificationStatus, VerificationStatusUnverified)
	}
}

func TestServiceCreateDefaultsAccountType(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)

	_, err := service.Create(context.Background(), CreateInput{
		Name:     "Ada Lovelace",
		Email:    "ada@example.com",
		Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if store.record.AccountType != AccountTypeIndividual {
		t.Fatalf("account type = %q, want %q", store.record.AccountType, AccountTypeIndividual)
	}
}

func TestServiceCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		err   error
	}{
		{
			name: "missing name",
			input: CreateInput{
				Email:    "ada@example.com",
				Password: "strong-password",
			},
			err: ErrInvalidName,
		},
		{
			name: "invalid email",
			input: CreateInput{
				Name:     "Ada Lovelace",
				Email:    "not-email",
				Password: "strong-password",
			},
			err: ErrInvalidEmail,
		},
		{
			name: "short password",
			input: CreateInput{
				Name:     "Ada Lovelace",
				Email:    "ada@example.com",
				Password: "short",
			},
			err: ErrInvalidPassword,
		},
		{
			name: "invalid account type",
			input: CreateInput{
				Name:        "Ada Lovelace",
				Email:       "ada@example.com",
				Password:    "strong-password",
				AccountType: "admin",
			},
			err: ErrInvalidAccountType,
		},
	}

	service := NewService(&recordingStore{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
		})
	}
}

func TestServiceGetReturnsUser(t *testing.T) {
	userID := uuid.New()
	store := &recordingStore{
		user: User{
			ID:                 userID,
			Name:               "Ada Lovelace",
			Email:              "ada@example.com",
			AccountType:        AccountTypeIndividual,
			VerificationStatus: VerificationStatusUnverified,
		},
	}
	service := NewService(store)

	user, err := service.Get(context.Background(), userID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}

	if user.ID != userID {
		t.Fatalf("user id = %s, want %s", user.ID, userID)
	}
}
