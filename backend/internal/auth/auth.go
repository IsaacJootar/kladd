package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/users"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	AccessToken string     `json:"access_token"`
	TokenType   string     `json:"token_type"`
	ExpiresAt   time.Time  `json:"expires_at"`
	User        users.User `json:"user"`
}

type Credentials struct {
	User         users.User
	PasswordHash string
}

type Store interface {
	FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
	RecordLogin(ctx context.Context, user users.User) error
}

type Service struct {
	store        Store
	tokenManager TokenManager
}

func NewService(store Store, tokenManager TokenManager) Service {
	return Service{
		store:        store,
		tokenManager: tokenManager,
	}
}

func (service Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	if input.Password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}

	credentials, err := service.store.FindCredentialsByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(input.Password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	accessToken, expiresAt, err := service.tokenManager.Issue(credentials.User.ID)
	if err != nil {
		return LoginResult{}, err
	}

	if err := service.store.RecordLogin(ctx, credentials.User); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken: accessToken,
		TokenType:   TokenTypeBearer,
		ExpiresAt:   expiresAt,
		User:        credentials.User,
	}, nil
}

func (service Service) Authenticate(tokenString string) (uuid.UUID, error) {
	return service.tokenManager.Verify(tokenString)
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", err
	}

	return normalized, nil
}
