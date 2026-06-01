package claimrequests

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingStore struct {
	record        CreateRecord
	approveRecord ApproveRecord
	request       ClaimRequest
	requests      []ClaimRequest
	approval      ApprovalResult
	err           error
}

func (store *recordingStore) Create(ctx context.Context, record CreateRecord) (ClaimRequest, error) {
	store.record = record
	if store.err != nil {
		return ClaimRequest{}, store.err
	}

	store.request = ClaimRequest{
		ID:              record.ID,
		Organization:    Organization{ID: record.OrganizationID, Name: record.OrganizationName, OrganizationType: record.OrganizationType},
		UserID:          record.UserID,
		Purpose:         record.Purpose,
		RequestedTruths: record.Scope.RequestedTruths,
		Status:          record.Status,
		ExpiresAt:       record.ExpiresAt,
	}
	return store.request, nil
}

func (store *recordingStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]ClaimRequest, error) {
	if store.err != nil {
		return nil, store.err
	}
	return store.requests, nil
}

func (store *recordingStore) GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error) {
	if store.err != nil {
		return ClaimRequest{}, store.err
	}
	return store.request, nil
}

func (store *recordingStore) Approve(ctx context.Context, record ApproveRecord) (ApprovalResult, error) {
	store.approveRecord = record
	if store.err != nil {
		return ApprovalResult{}, store.err
	}
	if store.approval.ClaimID != uuid.Nil {
		return store.approval, nil
	}

	return ApprovalResult{
		ConsentID: record.ConsentID,
		ClaimID:   record.ClaimID,
		ClaimRequest: ClaimRequest{
			ID:              record.RequestID,
			UserID:          record.UserID,
			Status:          StatusApprovedWithPIN,
			RequestedTruths: []string{"identity_verified"},
			ExpiresAt:       time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		},
		ApprovedAt: record.ApprovedAt,
	}, nil
}

type recordingPINValidator struct {
	userID uuid.UUID
	pin    string
	err    error
}

func (validator *recordingPINValidator) Validate(ctx context.Context, userID uuid.UUID, pin string) error {
	validator.userID = userID
	validator.pin = pin
	return validator.err
}

func TestServiceCreatePreparesPendingRequest(t *testing.T) {
	userID := uuid.New()
	store := &recordingStore{}
	service := NewService(store)

	request, err := service.Create(context.Background(), CreateInput{
		UserID:           userID,
		OrganizationName: " Acme Bank ",
		OrganizationType: "bank",
		Purpose:          " Employment onboarding ",
		RequestedTruths:  []string{" identity_verified ", "degree_verified", "identity_verified", ""},
		DurationDays:     30,
	})
	if err != nil {
		t.Fatalf("create claim request: %v", err)
	}

	if request.UserID != userID {
		t.Fatalf("user id = %s, want %s", request.UserID, userID)
	}
	if request.Status != StatusPendingApproval {
		t.Fatalf("status = %q, want %q", request.Status, StatusPendingApproval)
	}
	if request.Organization.Name != "Acme Bank" {
		t.Fatalf("organization = %q, want Acme Bank", request.Organization.Name)
	}
	if store.record.Purpose != "Employment onboarding" {
		t.Fatalf("purpose = %q, want trimmed purpose", store.record.Purpose)
	}
	if got := strings.Join(store.record.Scope.RequestedTruths, ","); got != "identity_verified,degree_verified" {
		t.Fatalf("requested truths = %q, want deduplicated truths", got)
	}
	if store.record.ExpiresAt.IsZero() {
		t.Fatal("expected expiration to be set")
	}
}

func TestServiceCreateValidatesInput(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name  string
		input CreateInput
		err   error
	}{
		{
			name: "missing user",
			input: CreateInput{
				OrganizationName: "Acme Bank",
				Purpose:          "Employment onboarding",
				RequestedTruths:  []string{"identity_verified"},
				DurationDays:     30,
			},
			err: ErrInvalidUser,
		},
		{
			name: "missing organization",
			input: CreateInput{
				UserID:          userID,
				Purpose:         "Employment onboarding",
				RequestedTruths: []string{"identity_verified"},
				DurationDays:    30,
			},
			err: ErrInvalidOrganization,
		},
		{
			name: "missing purpose",
			input: CreateInput{
				UserID:           userID,
				OrganizationName: "Acme Bank",
				RequestedTruths:  []string{"identity_verified"},
				DurationDays:     30,
			},
			err: ErrInvalidPurpose,
		},
		{
			name: "missing scope",
			input: CreateInput{
				UserID:           userID,
				OrganizationName: "Acme Bank",
				Purpose:          "Employment onboarding",
				DurationDays:     30,
			},
			err: ErrInvalidScope,
		},
		{
			name: "invalid duration",
			input: CreateInput{
				UserID:           userID,
				OrganizationName: "Acme Bank",
				Purpose:          "Employment onboarding",
				RequestedTruths:  []string{"identity_verified"},
			},
			err: ErrInvalidDuration,
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

func TestServiceApproveValidatesPINBeforeApproving(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	pinValidator := &recordingPINValidator{}
	store := &recordingStore{
		request: ClaimRequest{
			ID:              requestID,
			Organization:    Organization{ID: uuid.New(), Name: "Acme Bank"},
			UserID:          userID,
			RequestedTruths: []string{"identity_verified"},
			Status:          StatusPendingApproval,
			ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
		},
	}
	service := NewService(store, pinValidator)

	result, err := service.Approve(context.Background(), ApproveInput{
		UserID:      userID,
		RequestID:   requestID,
		SecurityPIN: "4829",
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
		SessionID:   "session-1",
	})
	if err != nil {
		t.Fatalf("approve request: %v", err)
	}

	if pinValidator.userID != userID {
		t.Fatalf("pin validator user = %s, want %s", pinValidator.userID, userID)
	}
	if pinValidator.pin != "4829" {
		t.Fatalf("pin = %q, want 4829", pinValidator.pin)
	}
	if store.approveRecord.RequestID != requestID {
		t.Fatalf("approve request id = %s, want %s", store.approveRecord.RequestID, requestID)
	}
	if store.approveRecord.ConsentID == uuid.Nil {
		t.Fatal("expected consent id")
	}
	if store.approveRecord.ClaimID == uuid.Nil {
		t.Fatal("expected claim id")
	}
	if result.ClaimRequest.Status != StatusApprovedWithPIN {
		t.Fatalf("status = %q, want %q", result.ClaimRequest.Status, StatusApprovedWithPIN)
	}
}

func TestServiceApproveStopsWhenPINValidationFails(t *testing.T) {
	userID := uuid.New()
	requestID := uuid.New()
	pinErr := errors.New("bad pin")
	pinValidator := &recordingPINValidator{err: pinErr}
	store := &recordingStore{
		request: ClaimRequest{
			ID:        requestID,
			UserID:    userID,
			Status:    StatusPendingApproval,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		},
	}
	service := NewService(store, pinValidator)

	_, err := service.Approve(context.Background(), ApproveInput{
		UserID:      userID,
		RequestID:   requestID,
		SecurityPIN: "1111",
	})
	if !errors.Is(err, pinErr) {
		t.Fatalf("error = %v, want %v", err, pinErr)
	}
	if store.approveRecord.RequestID != uuid.Nil {
		t.Fatal("request was approved after failed pin validation")
	}
}

func TestServiceApproveRequiresPendingRequest(t *testing.T) {
	service := NewService(&recordingStore{
		request: ClaimRequest{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Status:    StatusApprovedWithPIN,
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		},
	}, &recordingPINValidator{})

	_, err := service.Approve(context.Background(), ApproveInput{
		UserID:      uuid.New(),
		RequestID:   uuid.New(),
		SecurityPIN: "4829",
	})
	if !errors.Is(err, ErrClaimRequestNotOpen) {
		t.Fatalf("error = %v, want %v", err, ErrClaimRequestNotOpen)
	}
}

func TestServiceApproveRejectsExpiredRequest(t *testing.T) {
	service := NewService(&recordingStore{
		request: ClaimRequest{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Status:    StatusPendingApproval,
			ExpiresAt: time.Now().UTC().Add(-time.Hour),
		},
	}, &recordingPINValidator{})

	_, err := service.Approve(context.Background(), ApproveInput{
		UserID:      uuid.New(),
		RequestID:   uuid.New(),
		SecurityPIN: "4829",
	})
	if !errors.Is(err, ErrClaimRequestExpired) {
		t.Fatalf("error = %v, want %v", err, ErrClaimRequestExpired)
	}
}

func TestClaimRequestJSONDoesNotExposeTruthValuesOrDocuments(t *testing.T) {
	payload, err := json.Marshal(ClaimRequest{
		ID:              uuid.New(),
		Organization:    Organization{ID: uuid.New(), Name: "Acme Bank", OrganizationType: "bank"},
		UserID:          uuid.New(),
		Purpose:         "Employment onboarding",
		RequestedTruths: []string{"identity_verified"},
		Status:          StatusPendingApproval,
	})
	if err != nil {
		t.Fatalf("marshal claim request: %v", err)
	}

	body := string(payload)
	for _, forbidden := range []string{"truth_value", "raw_document", "file_path", "bvn", "nin", "passport_number", "tax_id", "security_pin"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("claim request response exposed forbidden field %q", forbidden)
		}
	}
}
