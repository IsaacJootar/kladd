package users

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccountTypeIndividual = "individual"
	AccountTypeBusiness   = "business"

	VerificationStatusUnverified = "unverified"
)

var (
	ErrInvalidName        = errors.New("name is required")
	ErrInvalidEmail       = errors.New("valid email is required")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
	ErrInvalidAccountType = errors.New("account_type must be individual or business")
	ErrEmailTaken         = errors.New("email is already registered")
	ErrUserNotFound       = errors.New("user not found")
)

type CreateInput struct {
	Name        string
	Email       string
	Phone       string
	Password    string
	AccountType string
}

type CreateRecord struct {
	ID                 uuid.UUID
	Name               string
	Email              string
	Phone              string
	PasswordHash       string
	AccountType        string
	VerificationStatus string
}

type User struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Email              string    `json:"email"`
	Phone              string    `json:"phone,omitempty"`
	AccountType        string    `json:"account_type"`
	VerificationStatus string    `json:"verification_status"`
	CreatedAt          time.Time `json:"created_at"`
}

type Store interface {
	Create(ctx context.Context, record CreateRecord) (User, error)
	Get(ctx context.Context, id uuid.UUID) (User, error)
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (service Service) Create(ctx context.Context, input CreateInput) (User, error) {
	record, err := prepareCreateRecord(input)
	if err != nil {
		return User{}, err
	}

	return service.store.Create(ctx, record)
}

func (service Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return service.store.Get(ctx, id)
}

func prepareCreateRecord(input CreateInput) (CreateRecord, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateRecord{}, ErrInvalidName
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return CreateRecord{}, ErrInvalidEmail
	}

	if len(input.Password) < 8 {
		return CreateRecord{}, ErrInvalidPassword
	}

	accountType := strings.TrimSpace(input.AccountType)
	if accountType == "" {
		accountType = AccountTypeIndividual
	}
	if accountType != AccountTypeIndividual && accountType != AccountTypeBusiness {
		return CreateRecord{}, ErrInvalidAccountType
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return CreateRecord{}, err
	}

	return CreateRecord{
		ID:                 uuid.New(),
		Name:               name,
		Email:              email,
		Phone:              strings.TrimSpace(input.Phone),
		PasswordHash:       string(passwordHash),
		AccountType:        accountType,
		VerificationStatus: VerificationStatusUnverified,
	}, nil
}
