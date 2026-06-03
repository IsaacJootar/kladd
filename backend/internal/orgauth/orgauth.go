package orgauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

var (
	ErrMissingAPIKey       = errors.New("organization api key is required")
	ErrInvalidAPIKey       = errors.New("organization api key is invalid")
	ErrInvalidOrganization = errors.New("organization name is required")
	ErrInvalidKeyName      = errors.New("api key name is required")
)

type Store interface {
	AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error)
	IssueAPIKey(ctx context.Context, record IssueRecord) (IssuedAPIKey, error)
}

type Service struct {
	store Store
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
