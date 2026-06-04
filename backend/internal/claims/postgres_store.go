package claims

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
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

func (store PostgresStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]Claim, error) {
	rows, err := store.db.QueryContext(ctx, claimSelectQuery()+`
WHERE cr.user_id = $1
ORDER BY c.issued_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimList := []Claim{}
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claimList = append(claimList, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return claimList, nil
}

func (store PostgresStore) ListForOrganization(ctx context.Context, organizationID uuid.UUID) ([]Claim, error) {
	rows, err := store.db.QueryContext(ctx, claimSelectQuery()+`
WHERE cr.organization_id = $1
ORDER BY c.issued_at DESC`,
		organizationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	claimList := []Claim{}
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claimList = append(claimList, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return claimList, nil
}

func (store PostgresStore) GetForUser(ctx context.Context, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	claim, err := scanClaim(store.db.QueryRowContext(ctx, claimSelectQuery()+`
WHERE cr.user_id = $1 AND c.id = $2`,
		userID,
		claimID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrClaimNotFound
		}
		return Claim{}, err
	}

	return claim, nil
}

func (store PostgresStore) GetStatus(ctx context.Context, claimID uuid.UUID, retrievedAt time.Time) (Claim, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	claim, err := scanClaim(tx.QueryRowContext(ctx, claimSelectQuery()+`
WHERE c.id = $1`,
		claimID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrClaimNotFound
		}
		return Claim{}, err
	}

	if err := insertClaimStatusRetrievedAudit(ctx, tx, claim, retrievedAt); err != nil {
		return Claim{}, err
	}

	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}

	return claim, nil
}

func (store PostgresStore) Revoke(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, revokedAt time.Time) (Claim, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	claim, err := lockClaimForRevoke(ctx, tx, userID, claimID)
	if err != nil {
		return Claim{}, err
	}
	if claim.Status != StatusActive {
		return Claim{}, ErrClaimNotActive
	}
	if !claim.ExpiresAt.After(revokedAt) {
		return Claim{}, ErrClaimNotActive
	}

	claim, err = updateClaimRevoked(ctx, tx, userID, claimID, revokedAt)
	if err != nil {
		return Claim{}, err
	}

	if err := insertClaimRevokedAudit(ctx, tx, userID, claim); err != nil {
		return Claim{}, err
	}

	if err := webhooks.EnqueueClaimEvent(ctx, tx, store.webhookSigningSecret, webhooks.ClaimEvent{
		EventType:      webhooks.EventClaimRevoked,
		ClaimID:        claim.ID,
		ClaimRequestID: claim.ClaimRequestID,
		OrganizationID: claim.Organization.ID,
		Status:         StatusRevoked,
		ExpiresAt:      claim.ExpiresAt,
		OccurredAt:     revokedAt,
	}); err != nil {
		return Claim{}, err
	}

	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}

	return claim, nil
}

func (store PostgresStore) CreateExchangePIN(ctx context.Context, userID uuid.UUID, claimID uuid.UUID, pinHash string, expiresAt time.Time, createdAt time.Time) (ExchangePIN, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return ExchangePIN{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	claim, err := lockClaimForExchangePIN(ctx, tx, userID, claimID)
	if err != nil {
		return ExchangePIN{}, err
	}
	if claim.Status != StatusActive {
		return ExchangePIN{}, ErrClaimNotActive
	}
	if !claim.ExpiresAt.After(createdAt) {
		return ExchangePIN{}, ErrClaimNotActive
	}
	if expiresAt.After(claim.ExpiresAt) {
		expiresAt = claim.ExpiresAt
	}

	exchangePIN := ExchangePIN{
		ClaimID:   claim.ID,
		ExpiresAt: expiresAt,
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO claim_exchange_pins (
    id,
    claim_id,
    pin_hash,
    expires_at,
    created_at
) VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(),
		claim.ID,
		pinHash,
		expiresAt,
		createdAt,
	); err != nil {
		return ExchangePIN{}, err
	}

	if err := insertExchangePINCreatedAudit(ctx, tx, userID, claim, expiresAt); err != nil {
		return ExchangePIN{}, err
	}

	if err := tx.Commit(); err != nil {
		return ExchangePIN{}, err
	}

	return exchangePIN, nil
}

func (store PostgresStore) ResolveExchangePIN(ctx context.Context, pinHash string, retrievedAt time.Time) (Claim, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	claim, err := scanClaim(tx.QueryRowContext(ctx, claimSelectQuery()+`
JOIN claim_exchange_pins ep ON ep.claim_id = c.id
WHERE ep.pin_hash = $1
    AND ep.expires_at > $2
    AND c.status = $3
    AND c.expires_at > $2`,
		pinHash,
		retrievedAt,
		StatusActive,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrExchangePINNotFound
		}
		return Claim{}, err
	}

	if err := insertExchangePINResolvedAudit(ctx, tx, claim, retrievedAt); err != nil {
		return Claim{}, err
	}

	if err := tx.Commit(); err != nil {
		return Claim{}, err
	}

	return claim, nil
}

func (store PostgresStore) ExpireDue(ctx context.Context, expiredAt time.Time) ([]Claim, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	expiredClaims, err := updateDueClaimsExpired(ctx, tx, expiredAt)
	if err != nil {
		return nil, err
	}

	for _, claim := range expiredClaims {
		if err := insertClaimExpiredAudit(ctx, tx, claim, expiredAt); err != nil {
			return nil, err
		}

		if err := webhooks.EnqueueClaimEvent(ctx, tx, store.webhookSigningSecret, webhooks.ClaimEvent{
			EventType:      webhooks.EventClaimExpired,
			ClaimID:        claim.ID,
			ClaimRequestID: claim.ClaimRequestID,
			OrganizationID: claim.Organization.ID,
			Status:         StatusExpired,
			ExpiresAt:      claim.ExpiresAt,
			OccurredAt:     expiredAt,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return expiredClaims, nil
}

func claimSelectQuery() string {
	return `
SELECT
    c.id,
    c.claim_request_id,
    c.status,
    c.issued_at,
    c.expires_at,
    c.revoked_at,
    cr.purpose,
    cr.scope_json,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM claims c
JOIN claim_requests cr ON cr.id = c.claim_request_id
JOIN organizations org ON org.id = cr.organization_id
`
}

func updateDueClaimsExpired(ctx context.Context, tx *sql.Tx, expiredAt time.Time) ([]Claim, error) {
	rows, err := tx.QueryContext(ctx, `
WITH updated AS (
    UPDATE claims
    SET status = $2
    WHERE status = $3
        AND expires_at <= $1
    RETURNING
        id,
        claim_request_id,
        status,
        issued_at,
        expires_at,
        revoked_at
)
SELECT
    updated.id,
    updated.claim_request_id,
    updated.status,
    updated.issued_at,
    updated.expires_at,
    updated.revoked_at,
    cr.purpose,
    cr.scope_json,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM updated
JOIN claim_requests cr ON cr.id = updated.claim_request_id
JOIN organizations org ON org.id = cr.organization_id
ORDER BY updated.expires_at ASC`,
		expiredAt,
		StatusExpired,
		StatusActive,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expiredClaims := []Claim{}
	for rows.Next() {
		claim, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		expiredClaims = append(expiredClaims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return expiredClaims, nil
}

func lockClaimForExchangePIN(ctx context.Context, tx *sql.Tx, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	claim, err := scanClaim(tx.QueryRowContext(ctx, claimSelectQuery()+`
WHERE cr.user_id = $1 AND c.id = $2
FOR UPDATE OF c`,
		userID,
		claimID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrClaimNotFound
		}
		return Claim{}, err
	}

	return claim, nil
}

func lockClaimForRevoke(ctx context.Context, tx *sql.Tx, userID uuid.UUID, claimID uuid.UUID) (Claim, error) {
	claim, err := scanClaim(tx.QueryRowContext(ctx, claimSelectQuery()+`
WHERE cr.user_id = $1 AND c.id = $2
FOR UPDATE OF c`,
		userID,
		claimID,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrClaimNotFound
		}
		return Claim{}, err
	}

	return claim, nil
}

func updateClaimRevoked(ctx context.Context, tx *sql.Tx, userID uuid.UUID, claimID uuid.UUID, revokedAt time.Time) (Claim, error) {
	claim, err := scanClaim(tx.QueryRowContext(ctx, `
WITH updated AS (
    UPDATE claims
    SET
        status = $3,
        revoked_at = $4
    FROM claim_requests cr
    WHERE claims.claim_request_id = cr.id
        AND cr.user_id = $1
        AND claims.id = $2
    RETURNING
        claims.id,
        claims.claim_request_id,
        claims.status,
        claims.issued_at,
        claims.expires_at,
        claims.revoked_at
)
SELECT
    updated.id,
    updated.claim_request_id,
    updated.status,
    updated.issued_at,
    updated.expires_at,
    updated.revoked_at,
    cr.purpose,
    cr.scope_json,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM updated
JOIN claim_requests cr ON cr.id = updated.claim_request_id
JOIN organizations org ON org.id = cr.organization_id`,
		userID,
		claimID,
		StatusRevoked,
		revokedAt,
	))
	if err != nil {
		if err == sql.ErrNoRows {
			return Claim{}, ErrClaimNotFound
		}
		return Claim{}, err
	}

	return claim, nil
}

func insertClaimRevokedAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID, claim Claim) error {
	metadata, err := json.Marshal(map[string]string{
		"claim_id":         claim.ID.String(),
		"claim_request_id": claim.ClaimRequestID.String(),
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
		userID,
		"claim.revoked",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim revoked audit: %w", err)
	}

	return nil
}

func insertClaimExpiredAudit(ctx context.Context, tx *sql.Tx, claim Claim, expiredAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{
		"claim_id":         claim.ID.String(),
		"claim_request_id": claim.ClaimRequestID.String(),
		"expired_at":       expiredAt.Format(time.RFC3339),
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
		"system",
		nil,
		"claim.expired",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim expired audit: %w", err)
	}

	return nil
}

func insertClaimStatusRetrievedAudit(ctx context.Context, tx *sql.Tx, claim Claim, retrievedAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{
		"claim_id":         claim.ID.String(),
		"claim_request_id": claim.ClaimRequestID.String(),
		"status":           claim.Status,
		"retrieved_at":     retrievedAt.Format(time.RFC3339),
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
		"system",
		nil,
		"claim.status_retrieved",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert claim status retrieved audit: %w", err)
	}

	return nil
}

func insertExchangePINCreatedAudit(ctx context.Context, tx *sql.Tx, userID uuid.UUID, claim Claim, expiresAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{
		"claim_id":         claim.ID.String(),
		"claim_request_id": claim.ClaimRequestID.String(),
		"expires_at":       expiresAt.Format(time.RFC3339),
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
		userID,
		"claim.exchange_pin_created",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert exchange pin created audit: %w", err)
	}

	return nil
}

func insertExchangePINResolvedAudit(ctx context.Context, tx *sql.Tx, claim Claim, retrievedAt time.Time) error {
	metadata, err := json.Marshal(map[string]string{
		"claim_id":         claim.ID.String(),
		"claim_request_id": claim.ClaimRequestID.String(),
		"retrieved_at":     retrievedAt.Format(time.RFC3339),
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
		"system",
		nil,
		"claim.exchange_pin_resolved",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert exchange pin resolved audit: %w", err)
	}

	return nil
}

type claimScanner interface {
	Scan(dest ...any) error
}

func scanClaim(scanner claimScanner) (Claim, error) {
	var claim Claim
	var scopeBytes []byte
	var revokedAt sql.NullTime
	err := scanner.Scan(
		&claim.ID,
		&claim.ClaimRequestID,
		&claim.Status,
		&claim.IssuedAt,
		&claim.ExpiresAt,
		&revokedAt,
		&claim.Purpose,
		&scopeBytes,
		&claim.Organization.ID,
		&claim.Organization.Name,
		&claim.Organization.OrganizationType,
		&claim.Organization.VerificationStatus,
	)
	if err != nil {
		return Claim{}, err
	}

	if revokedAt.Valid {
		claim.RevokedAt = &revokedAt.Time
	}

	var scope claimrequests.Scope
	if err := json.Unmarshal(scopeBytes, &scope); err != nil {
		return Claim{}, err
	}
	claim.ApprovedTruths = scope.RequestedTruths

	return claim, nil
}
