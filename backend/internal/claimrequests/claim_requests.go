package claimrequests

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPendingApproval = "pending_approval"

	defaultOrganizationType = "organization"
)

var (
	ErrInvalidUser          = errors.New("user_id is required")
	ErrInvalidOrganization  = errors.New("organization name is required")
	ErrInvalidPurpose       = errors.New("purpose is required")
	ErrInvalidScope         = errors.New("requested truths are required")
	ErrInvalidDuration      = errors.New("duration must be at least 1 day")
	ErrClaimRequestNotFound = errors.New("claim request not found")
)

type CreateInput struct {
	UserID           uuid.UUID
	OrganizationName string
	OrganizationType string
	Purpose          string
	RequestedTruths  []string
	DurationDays     int
}

type Organization struct {
	ID                 uuid.UUID `json:"id"`
	Name               string    `json:"name"`
	OrganizationType   string    `json:"organization_type"`
	VerificationStatus string    `json:"verification_status"`
}

type ClaimRequest struct {
	ID              uuid.UUID    `json:"id"`
	Organization    Organization `json:"organization"`
	UserID          uuid.UUID    `json:"user_id"`
	Purpose         string       `json:"purpose"`
	RequestedTruths []string     `json:"requested_truths"`
	Status          string       `json:"status"`
	ExpiresAt       time.Time    `json:"expires_at"`
	CreatedAt       time.Time    `json:"created_at"`
}

type Scope struct {
	RequestedTruths []string `json:"requested_truths"`
}

type CreateRecord struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	OrganizationName string
	OrganizationType string
	UserID           uuid.UUID
	Purpose          string
	Scope            Scope
	Status           string
	ExpiresAt        time.Time
}

type Store interface {
	Create(ctx context.Context, record CreateRecord) (ClaimRequest, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]ClaimRequest, error)
	GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error)
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (service Service) Create(ctx context.Context, input CreateInput) (ClaimRequest, error) {
	record, err := prepareCreateRecord(input)
	if err != nil {
		return ClaimRequest{}, err
	}

	return service.store.Create(ctx, record)
}

func (service Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]ClaimRequest, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidUser
	}

	return service.store.ListForUser(ctx, userID)
}

func (service Service) GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error) {
	if userID == uuid.Nil {
		return ClaimRequest{}, ErrInvalidUser
	}
	if requestID == uuid.Nil {
		return ClaimRequest{}, ErrClaimRequestNotFound
	}

	return service.store.GetForUser(ctx, userID, requestID)
}

func prepareCreateRecord(input CreateInput) (CreateRecord, error) {
	if input.UserID == uuid.Nil {
		return CreateRecord{}, ErrInvalidUser
	}

	organizationName := strings.TrimSpace(input.OrganizationName)
	if organizationName == "" {
		return CreateRecord{}, ErrInvalidOrganization
	}

	organizationType := strings.TrimSpace(input.OrganizationType)
	if organizationType == "" {
		organizationType = defaultOrganizationType
	}

	purpose := strings.TrimSpace(input.Purpose)
	if purpose == "" {
		return CreateRecord{}, ErrInvalidPurpose
	}

	requestedTruths := cleanRequestedTruths(input.RequestedTruths)
	if len(requestedTruths) == 0 {
		return CreateRecord{}, ErrInvalidScope
	}

	if input.DurationDays < 1 {
		return CreateRecord{}, ErrInvalidDuration
	}

	return CreateRecord{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		OrganizationName: organizationName,
		OrganizationType: organizationType,
		UserID:           input.UserID,
		Purpose:          purpose,
		Scope: Scope{
			RequestedTruths: requestedTruths,
		},
		Status:    StatusPendingApproval,
		ExpiresAt: time.Now().UTC().Add(time.Duration(input.DurationDays) * 24 * time.Hour),
	}, nil
}

func cleanRequestedTruths(values []string) []string {
	seen := map[string]bool{}
	cleaned := []string{}
	for _, value := range values {
		truth := strings.TrimSpace(value)
		if truth == "" || seen[truth] {
			continue
		}

		seen[truth] = true
		cleaned = append(cleaned, truth)
	}

	return cleaned
}
