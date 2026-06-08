package claimrequests

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/truths"
	"github.com/google/uuid"
)

const (
	StatusPendingApproval = "pending_approval"
	StatusApprovedWithPIN = "approved_with_security_pin"
	StatusDenied          = "denied"

	ClaimStatusActive = "active"

	defaultOrganizationType = "organization"
	approvalMethodPIN       = "security_pin"
)

var (
	ErrInvalidUser           = errors.New("user_id is required")
	ErrInvalidOrganization   = errors.New("organization name is required")
	ErrInvalidOrganizationID = errors.New("organization_id is required")
	ErrInvalidPurpose        = errors.New("purpose is required")
	ErrInvalidScope          = errors.New("requested truths are required")
	ErrInvalidDuration       = errors.New("duration must be at least 1 day")
	ErrUnknownTruth          = errors.New("requested truth is not supported")
	ErrInvalidSecurityPIN    = errors.New("security pin is required")
	ErrClaimRequestNotFound  = errors.New("claim request not found")
	ErrClaimRequestExpired   = errors.New("claim request has expired")
	ErrClaimRequestNotOpen   = errors.New("claim request is not pending approval")
	ErrPINValidatorMissing   = errors.New("security pin validator is required")
)

type CreateInput struct {
	UserID           uuid.UUID
	OrganizationName string
	OrganizationType string
	Purpose          string
	RequestedTruths  []string
	DurationDays     int
}

type ApproveInput struct {
	UserID      uuid.UUID
	RequestID   uuid.UUID
	SecurityPIN string
	IPAddress   string
	UserAgent   string
	SessionID   string
}

type DenyInput struct {
	UserID    uuid.UUID
	RequestID uuid.UUID
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

type ApprovalResult struct {
	ConsentID    uuid.UUID    `json:"consent_id"`
	ClaimID      uuid.UUID    `json:"claim_id"`
	ClaimRequest ClaimRequest `json:"claim_request"`
	ApprovedAt   time.Time    `json:"approved_at"`
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

type ApproveRecord struct {
	ConsentID  uuid.UUID
	ClaimID    uuid.UUID
	RequestID  uuid.UUID
	UserID     uuid.UUID
	ApprovedAt time.Time
	IPAddress  string
	UserAgent  string
	SessionID  string
}

type DenyRecord struct {
	RequestID uuid.UUID
	UserID    uuid.UUID
	DeniedAt  time.Time
}

type Store interface {
	Create(ctx context.Context, record CreateRecord) (ClaimRequest, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]ClaimRequest, error)
	ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]ClaimRequest, error)
	GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error)
	Approve(ctx context.Context, record ApproveRecord) (ApprovalResult, error)
	Deny(ctx context.Context, record DenyRecord) (ClaimRequest, error)
}

type SecurityPINValidator interface {
	Validate(ctx context.Context, userID uuid.UUID, pin string) error
}

type TruthRegistry interface {
	ListDefinitions(ctx context.Context) ([]truths.Definition, error)
}

type Service struct {
	store         Store
	pinValidator  SecurityPINValidator
	truthRegistry TruthRegistry
}

func NewService(store Store, validators ...SecurityPINValidator) Service {
	var validator SecurityPINValidator
	if len(validators) > 0 {
		validator = validators[0]
	}

	return Service{
		store:        store,
		pinValidator: validator,
	}
}

func NewServiceWithTruthRegistry(store Store, validator SecurityPINValidator, truthRegistry TruthRegistry) Service {
	return Service{
		store:         store,
		pinValidator:  validator,
		truthRegistry: truthRegistry,
	}
}

func (service Service) Create(ctx context.Context, input CreateInput) (ClaimRequest, error) {
	record, err := prepareCreateRecord(input)
	if err != nil {
		return ClaimRequest{}, err
	}
	if err := service.validateRequestedTruths(ctx, record.Scope.RequestedTruths); err != nil {
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

func (service Service) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]ClaimRequest, error) {
	if organizationID == uuid.Nil {
		return nil, ErrInvalidOrganizationID
	}

	return service.store.ListForOrganization(ctx, organizationID)
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

func (service Service) Approve(ctx context.Context, input ApproveInput) (ApprovalResult, error) {
	if input.UserID == uuid.Nil {
		return ApprovalResult{}, ErrInvalidUser
	}
	if input.RequestID == uuid.Nil {
		return ApprovalResult{}, ErrClaimRequestNotFound
	}
	if strings.TrimSpace(input.SecurityPIN) == "" {
		return ApprovalResult{}, ErrInvalidSecurityPIN
	}
	if service.pinValidator == nil {
		return ApprovalResult{}, ErrPINValidatorMissing
	}

	request, err := service.store.GetForUser(ctx, input.UserID, input.RequestID)
	if err != nil {
		return ApprovalResult{}, err
	}
	if request.Status != StatusPendingApproval {
		return ApprovalResult{}, ErrClaimRequestNotOpen
	}
	if !request.ExpiresAt.After(time.Now().UTC()) {
		return ApprovalResult{}, ErrClaimRequestExpired
	}

	if err := service.pinValidator.Validate(ctx, input.UserID, input.SecurityPIN); err != nil {
		return ApprovalResult{}, err
	}

	return service.store.Approve(ctx, ApproveRecord{
		ConsentID:  uuid.New(),
		ClaimID:    uuid.New(),
		RequestID:  input.RequestID,
		UserID:     input.UserID,
		ApprovedAt: time.Now().UTC(),
		IPAddress:  strings.TrimSpace(input.IPAddress),
		UserAgent:  strings.TrimSpace(input.UserAgent),
		SessionID:  strings.TrimSpace(input.SessionID),
	})
}

func (service Service) Deny(ctx context.Context, input DenyInput) (ClaimRequest, error) {
	if input.UserID == uuid.Nil {
		return ClaimRequest{}, ErrInvalidUser
	}
	if input.RequestID == uuid.Nil {
		return ClaimRequest{}, ErrClaimRequestNotFound
	}

	request, err := service.store.GetForUser(ctx, input.UserID, input.RequestID)
	if err != nil {
		return ClaimRequest{}, err
	}
	if request.Status != StatusPendingApproval {
		return ClaimRequest{}, ErrClaimRequestNotOpen
	}
	if !request.ExpiresAt.After(time.Now().UTC()) {
		return ClaimRequest{}, ErrClaimRequestExpired
	}

	return service.store.Deny(ctx, DenyRecord{
		RequestID: input.RequestID,
		UserID:    input.UserID,
		DeniedAt:  time.Now().UTC(),
	})
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

func (service Service) validateRequestedTruths(ctx context.Context, requestedTruths []string) error {
	if service.truthRegistry == nil {
		return nil
	}

	definitions, err := service.truthRegistry.ListDefinitions(ctx)
	if err != nil {
		return err
	}

	supported := map[string]bool{}
	for _, definition := range definitions {
		supported[definition.TruthKey] = true
	}

	for _, requestedTruth := range requestedTruths {
		if !supported[requestedTruth] {
			return ErrUnknownTruth
		}
	}

	return nil
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
