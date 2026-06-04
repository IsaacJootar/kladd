package claimrequests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/webhooks"
	"github.com/google/uuid"
)

type PostgresStore struct {
	db                   *sql.DB
	webhookSigningSecret string
}

func NewPostgresStore(db *sql.DB, webhookSigningSecrets ...string) PostgresStore {
	signingSecret := ""
	if len(webhookSigningSecrets) > 0 {
		signingSecret = webhookSigningSecrets[0]
	}

	return PostgresStore{
		db:                   db,
		webhookSigningSecret: signingSecret,
	}
}

func (store PostgresStore) Create(ctx context.Context, record CreateRecord) (ClaimRequest, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimRequest{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	organization, err := upsertOrganization(ctx, tx, record)
	if err != nil {
		return ClaimRequest{}, err
	}
	record.OrganizationID = organization.ID

	request, err := insertClaimRequest(ctx, tx, record)
	if err != nil {
		return ClaimRequest{}, err
	}

	if err := insertClaimRequestCreatedAudit(ctx, tx, request); err != nil {
		return ClaimRequest{}, err
	}

	if err := tx.Commit(); err != nil {
		return ClaimRequest{}, err
	}

	return request, nil
}

func (store PostgresStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]ClaimRequest, error) {
	rows, err := store.db.QueryContext(ctx, claimRequestSelectQuery()+`
WHERE cr.user_id = $1
ORDER BY cr.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []ClaimRequest{}
	for rows.Next() {
		request, err := scanClaimRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

func (store PostgresStore) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]ClaimRequest, error) {
	rows, err := store.db.QueryContext(ctx, claimRequestSelectQuery()+`
WHERE cr.organization_id = $1
ORDER BY cr.created_at DESC`,
		organizationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := []ClaimRequest{}
	for rows.Next() {
		request, err := scanClaimRequest(rows)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

func (store PostgresStore) GetForUser(ctx context.Context, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error) {
	request, err := scanClaimRequest(store.db.QueryRowContext(ctx, claimRequestSelectQuery()+`
WHERE cr.user_id = $1 AND cr.id = $2`,
		userID,
		requestID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return ClaimRequest{}, ErrClaimRequestNotFound
		}
		return ClaimRequest{}, err
	}

	return request, nil
}

func (store PostgresStore) Approve(ctx context.Context, record ApproveRecord) (ApprovalResult, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ApprovalResult{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	request, err := lockClaimRequestForApproval(ctx, tx, record.UserID, record.RequestID)
	if err != nil {
		return ApprovalResult{}, err
	}
	if request.Status != StatusPendingApproval {
		return ApprovalResult{}, ErrClaimRequestNotOpen
	}
	if !request.ExpiresAt.After(record.ApprovedAt) {
		return ApprovalResult{}, ErrClaimRequestExpired
	}

	if err := insertClaim(ctx, tx, record, request.ExpiresAt); err != nil {
		return ApprovalResult{}, err
	}

	if err := insertConsent(ctx, tx, record, request.Organization.ID); err != nil {
		return ApprovalResult{}, err
	}

	approvedRequest, err := updateClaimRequestApproved(ctx, tx, record.RequestID)
	if err != nil {
		return ApprovalResult{}, err
	}

	if err := insertClaimRequestApprovedAudit(ctx, tx, record, approvedRequest); err != nil {
		return ApprovalResult{}, err
	}

	if err := webhooks.EnqueueClaimEvent(ctx, tx, store.webhookSigningSecret, webhooks.ClaimEvent{
		EventType:      webhooks.EventClaimApproved,
		ClaimID:        record.ClaimID,
		ClaimRequestID: approvedRequest.ID,
		OrganizationID: approvedRequest.Organization.ID,
		Status:         ClaimStatusActive,
		ExpiresAt:      approvedRequest.ExpiresAt,
		OccurredAt:     record.ApprovedAt,
	}); err != nil {
		return ApprovalResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return ApprovalResult{}, err
	}

	return ApprovalResult{
		ConsentID:    record.ConsentID,
		ClaimID:      record.ClaimID,
		ClaimRequest: approvedRequest,
		ApprovedAt:   record.ApprovedAt,
	}, nil
}

func (store PostgresStore) Deny(ctx context.Context, record DenyRecord) (ClaimRequest, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ClaimRequest{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	request, err := lockClaimRequestForApproval(ctx, tx, record.UserID, record.RequestID)
	if err != nil {
		return ClaimRequest{}, err
	}
	if request.Status != StatusPendingApproval {
		return ClaimRequest{}, ErrClaimRequestNotOpen
	}
	if !request.ExpiresAt.After(record.DeniedAt) {
		return ClaimRequest{}, ErrClaimRequestExpired
	}

	deniedRequest, err := updateClaimRequestDenied(ctx, tx, record.RequestID)
	if err != nil {
		return ClaimRequest{}, err
	}

	if err := insertClaimRequestDeniedAudit(ctx, tx, record, deniedRequest); err != nil {
		return ClaimRequest{}, err
	}

	if err := tx.Commit(); err != nil {
		return ClaimRequest{}, err
	}

	return deniedRequest, nil
}

func upsertOrganization(ctx context.Context, tx *sql.Tx, record CreateRecord) (Organization, error) {
	var organization Organization
	err := tx.QueryRowContext(ctx, `
INSERT INTO organizations (
    id,
    name,
    organization_type
) VALUES ($1, $2, $3)
ON CONFLICT (name) DO UPDATE
SET organization_type = EXCLUDED.organization_type
RETURNING id, name, organization_type, verification_status`,
		record.OrganizationID,
		record.OrganizationName,
		record.OrganizationType,
	).Scan(
		&organization.ID,
		&organization.Name,
		&organization.OrganizationType,
		&organization.VerificationStatus,
	)
	if err != nil {
		return Organization{}, err
	}

	return organization, nil
}

func insertClaimRequest(ctx context.Context, tx *sql.Tx, record CreateRecord) (ClaimRequest, error) {
	scope, err := json.Marshal(record.Scope)
	if err != nil {
		return ClaimRequest{}, err
	}

	request, err := scanClaimRequest(tx.QueryRowContext(ctx, `
WITH inserted AS (
    INSERT INTO claim_requests (
        id,
        organization_id,
        user_id,
        purpose,
        scope_json,
        status,
        expires_at
    ) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
    RETURNING id, user_id, purpose, scope_json, status, expires_at, created_at, organization_id
)
SELECT
    inserted.id,
    inserted.user_id,
    inserted.purpose,
    inserted.scope_json,
    inserted.status,
    inserted.expires_at,
    inserted.created_at,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM inserted
JOIN organizations org ON org.id = inserted.organization_id`,
		record.ID,
		record.OrganizationID,
		record.UserID,
		record.Purpose,
		string(scope),
		record.Status,
		record.ExpiresAt,
	))
	if err != nil {
		return ClaimRequest{}, err
	}

	return request, nil
}

func insertClaimRequestCreatedAudit(ctx context.Context, tx *sql.Tx, request ClaimRequest) error {
	metadata, err := json.Marshal(map[string]any{
		"claim_request_id": request.ID.String(),
		"user_id":          request.UserID.String(),
		"requested_truths": request.RequestedTruths,
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"organization",
		request.Organization.ID,
		"claim_request.created",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim request created audit: %w", err)
	}

	return nil
}

func lockClaimRequestForApproval(ctx context.Context, tx *sql.Tx, userID uuid.UUID, requestID uuid.UUID) (ClaimRequest, error) {
	request, err := scanClaimRequest(tx.QueryRowContext(ctx, claimRequestSelectQuery()+`
WHERE cr.user_id = $1 AND cr.id = $2
FOR UPDATE OF cr`,
		userID,
		requestID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return ClaimRequest{}, ErrClaimRequestNotFound
		}
		return ClaimRequest{}, err
	}

	return request, nil
}

func insertClaim(ctx context.Context, tx *sql.Tx, record ApproveRecord, expiresAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO claims (
    id,
    claim_request_id,
    status,
    issued_at,
    expires_at
) VALUES ($1, $2, $3, $4, $5)`,
		record.ClaimID,
		record.RequestID,
		ClaimStatusActive,
		record.ApprovedAt,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert claim: %w", err)
	}

	return nil
}

func insertConsent(ctx context.Context, tx *sql.Tx, record ApproveRecord, organizationID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO consents (
    id,
    claim_request_id,
    claim_id,
    user_id,
    organization_id,
    approved,
    approval_method,
    approved_at,
    ip_address,
    user_agent,
    session_id
) VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9, $10)`,
		record.ConsentID,
		record.RequestID,
		record.ClaimID,
		record.UserID,
		organizationID,
		approvalMethodPIN,
		record.ApprovedAt,
		nullString(record.IPAddress),
		nullString(record.UserAgent),
		nullString(record.SessionID),
	)
	if err != nil {
		return fmt.Errorf("insert consent: %w", err)
	}

	return nil
}

func updateClaimRequestApproved(ctx context.Context, tx *sql.Tx, requestID uuid.UUID) (ClaimRequest, error) {
	request, err := scanClaimRequest(tx.QueryRowContext(ctx, `
WITH updated AS (
    UPDATE claim_requests
    SET status = $2
    WHERE id = $1
    RETURNING id, user_id, purpose, scope_json, status, expires_at, created_at, organization_id
)
SELECT
    updated.id,
    updated.user_id,
    updated.purpose,
    updated.scope_json,
    updated.status,
    updated.expires_at,
    updated.created_at,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM updated
JOIN organizations org ON org.id = updated.organization_id`,
		requestID,
		StatusApprovedWithPIN,
	))
	if err != nil {
		return ClaimRequest{}, err
	}

	return request, nil
}

func updateClaimRequestDenied(ctx context.Context, tx *sql.Tx, requestID uuid.UUID) (ClaimRequest, error) {
	request, err := scanClaimRequest(tx.QueryRowContext(ctx, `
WITH updated AS (
    UPDATE claim_requests
    SET status = $2
    WHERE id = $1
    RETURNING id, user_id, purpose, scope_json, status, expires_at, created_at, organization_id
)
SELECT
    updated.id,
    updated.user_id,
    updated.purpose,
    updated.scope_json,
    updated.status,
    updated.expires_at,
    updated.created_at,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM updated
JOIN organizations org ON org.id = updated.organization_id`,
		requestID,
		StatusDenied,
	))
	if err != nil {
		return ClaimRequest{}, err
	}

	return request, nil
}

func insertClaimRequestApprovedAudit(ctx context.Context, tx *sql.Tx, record ApproveRecord, request ClaimRequest) error {
	metadata, err := json.Marshal(map[string]any{
		"claim_request_id": request.ID.String(),
		"claim_id":         record.ClaimID.String(),
		"consent_id":       record.ConsentID.String(),
		"method":           approvalMethodPIN,
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"user",
		request.UserID,
		"claim_request.approved",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim request approved audit: %w", err)
	}

	return nil
}

func insertClaimRequestDeniedAudit(ctx context.Context, tx *sql.Tx, record DenyRecord, request ClaimRequest) error {
	metadata, err := json.Marshal(map[string]any{
		"claim_request_id": request.ID.String(),
		"denied_at":        record.DeniedAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"user",
		request.UserID,
		"claim_request.denied",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim request denied audit: %w", err)
	}

	return nil
}

func claimRequestSelectQuery() string {
	return `
SELECT
    cr.id,
    cr.user_id,
    cr.purpose,
    cr.scope_json,
    cr.status,
    cr.expires_at,
    cr.created_at,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM claim_requests cr
JOIN organizations org ON org.id = cr.organization_id
`
}

type claimRequestScanner interface {
	Scan(dest ...any) error
}

func scanClaimRequest(scanner claimRequestScanner) (ClaimRequest, error) {
	var request ClaimRequest
	var scopeBytes []byte
	err := scanner.Scan(
		&request.ID,
		&request.UserID,
		&request.Purpose,
		&scopeBytes,
		&request.Status,
		&request.ExpiresAt,
		&request.CreatedAt,
		&request.Organization.ID,
		&request.Organization.Name,
		&request.Organization.OrganizationType,
		&request.Organization.VerificationStatus,
	)
	if err != nil {
		return ClaimRequest{}, err
	}

	var scope Scope
	if err := json.Unmarshal(scopeBytes, &scope); err != nil {
		return ClaimRequest{}, err
	}
	request.RequestedTruths = scope.RequestedTruths

	return request, nil
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: value, Valid: true}
}
