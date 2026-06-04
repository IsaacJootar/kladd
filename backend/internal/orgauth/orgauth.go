package orgauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/auth"
	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMissingAPIKey       = errors.New("organization api key is required")
	ErrInvalidAPIKey       = errors.New("organization api key is invalid")
	ErrInvalidOrganization = errors.New("organization name is required")
	ErrInvalidKeyName      = errors.New("api key name is required")
	ErrInvalidEmail        = errors.New("valid organization email is required")
	ErrInvalidPassword     = errors.New("organization password must be at least 8 characters")
	ErrEmailTaken          = errors.New("organization email is already registered")
	ErrInvalidCredentials  = errors.New("invalid organization email or password")
)

type Store interface {
	AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error)
	IssueAPIKey(ctx context.Context, record IssueRecord) (IssuedAPIKey, error)
	RegisterAccount(ctx context.Context, record RegisterRecord) (Account, error)
	FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
	RecordLogin(ctx context.Context, account Account) error
}

type Service struct {
	store        Store
	tokenManager auth.TokenManager
}

type RegisterInput struct {
	Name             string
	Email            string
	Password         string
	OrganizationType string
}

type RegisterRecord struct {
	ID                 uuid.UUID
	Name               string
	Email              string
	PasswordHash       string
	OrganizationType   string
	VerificationStatus string
	CreatedAt          time.Time
}

type Account struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	Email              string    `json:"email"`
	OrganizationType   string    `json:"organization_type"`
	VerificationStatus string    `json:"verification_status"`
	CreatedAt          time.Time `json:"created_at"`
}

type Credentials struct {
	Account      Account
	PasswordHash string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Organization Account   `json:"organization"`
}

type IssueInput struct {
	OrganizationName string
	OrganizationType string
	KeyName          string
}

type IssueRecord struct {
	OrganizationID   uuid.UUID
	OrganizationName string
	OrganizationType string
	KeyID            uuid.UUID
	KeyName          string
	KeyHash          string
	KeyPrefix        string
	CreatedAt        time.Time
}

type IssuedAPIKey struct {
	Organization claimrequests.Organization `json:"organization"`
	KeyID        uuid.UUID                  `json:"key_id"`
	KeyName      string                     `json:"key_name"`
	KeyPrefix    string                     `json:"key_prefix"`
	APIKey       string                     `json:"api_key,omitempty"`
	CreatedAt    time.Time                  `json:"created_at"`
}

func NewService(store Store) Service {
	return Service{store: store}
}

func NewServiceWithTokenManager(store Store, tokenManager auth.TokenManager) Service {
	return Service{store: store, tokenManager: tokenManager}
}

func (service Service) Authenticate(ctx context.Context, apiKey string) (claimrequests.Organization, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return claimrequests.Organization{}, ErrMissingAPIKey
	}

	return service.store.AuthenticateAPIKey(ctx, HashAPIKey(key))
}

func (service Service) IssueAPIKey(ctx context.Context, input IssueInput) (IssuedAPIKey, error) {
	record, apiKey, err := prepareIssueRecord(input)
	if err != nil {
		return IssuedAPIKey{}, err
	}

	issued, err := service.store.IssueAPIKey(ctx, record)
	if err != nil {
		return IssuedAPIKey{}, err
	}

	issued.APIKey = apiKey
	return issued, nil
}

func (service Service) RegisterAccount(ctx context.Context, input RegisterInput) (Account, error) {
	record, err := prepareRegisterRecord(input)
	if err != nil {
		return Account{}, err
	}

	return service.store.RegisterAccount(ctx, record)
}

func (service Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || input.Password == "" {
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

	accessToken, expiresAt, err := service.tokenManager.Issue(credentials.Account.ID)
	if err != nil {
		return LoginResult{}, err
	}

	if err := service.store.RecordLogin(ctx, credentials.Account); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:  accessToken,
		TokenType:    auth.TokenTypeBearer,
		ExpiresAt:    expiresAt,
		Organization: credentials.Account,
	}, nil
}

func (service Service) AuthenticateToken(tokenString string) (uuid.UUID, error) {
	return service.tokenManager.Verify(tokenString)
}

func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte("kladd-organization-api-key:" + apiKey))
	return hex.EncodeToString(sum[:])
}

func prepareIssueRecord(input IssueInput) (IssueRecord, string, error) {
	organizationName := strings.TrimSpace(input.OrganizationName)
	if organizationName == "" {
		return IssueRecord{}, "", ErrInvalidOrganization
	}

	organizationType := strings.TrimSpace(input.OrganizationType)
	if organizationType == "" {
		organizationType = "organization"
	}

	keyName := strings.TrimSpace(input.KeyName)
	if keyName == "" {
		return IssueRecord{}, "", ErrInvalidKeyName
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		return IssueRecord{}, "", err
	}

	return IssueRecord{
		OrganizationID:   uuid.New(),
		OrganizationName: organizationName,
		OrganizationType: organizationType,
		KeyID:            uuid.New(),
		KeyName:          keyName,
		KeyHash:          HashAPIKey(apiKey),
		KeyPrefix:        apiKeyPrefix(apiKey),
		CreatedAt:        time.Now().UTC(),
	}, apiKey, nil
}

func prepareRegisterRecord(input RegisterInput) (RegisterRecord, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return RegisterRecord{}, ErrInvalidOrganization
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return RegisterRecord{}, ErrInvalidEmail
	}

	if len(input.Password) < 8 {
		return RegisterRecord{}, ErrInvalidPassword
	}

	organizationType := strings.TrimSpace(input.OrganizationType)
	if organizationType == "" {
		organizationType = "organization"
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return RegisterRecord{}, err
	}

	return RegisterRecord{
		ID:                 uuid.New(),
		Name:               name,
		Email:              email,
		PasswordHash:       string(passwordHash),
		OrganizationType:   organizationType,
		VerificationStatus: "unverified",
		CreatedAt:          time.Now().UTC(),
	}, nil
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", err
	}

	return normalized, nil
}

func generateAPIKey() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return "kladd_org_" + hex.EncodeToString(randomBytes), nil
}

func apiKeyPrefix(apiKey string) string {
	if len(apiKey) <= 18 {
		return apiKey
	}

	return apiKey[:18]
}
