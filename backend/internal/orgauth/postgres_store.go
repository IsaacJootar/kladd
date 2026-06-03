package orgauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) IssueAPIKey(ctx context.Context, record IssueRecord) (IssuedAPIKey, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return IssuedAPIKey{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	organization, err := upsertOrganization(ctx, tx, record)
	if err != nil {
		return IssuedAPIKey{}, err
	}

	issued, err := insertAPIKey(ctx, tx, organization, record)
	if err != nil {
		return IssuedAPIKey{}, err
	}

	if err := tx.Commit(); err != nil {
		return IssuedAPIKey{}, err
	}

	return issued, nil
}

func (store PostgresStore) AuthenticateAPIKey(ctx context.Context, keyHash string) (claimrequests.Organization, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return claimrequests.Organization{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var organization claimrequests.Organization
	err = tx.QueryRowContext(ctx, `
SELECT
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM organization_api_keys key
JOIN organizations org ON org.id = key.organization_id
WHERE key.key_hash = $1
    AND key.revoked_at IS NULL`,
		keyHash,
	).Scan(
		&organization.ID,
		&organization.Name,
		&organization.OrganizationType,
		&organization.VerificationStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return claimrequests.Organization{}, ErrInvalidAPIKey
		}
		return claimrequests.Organization{}, err
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE organization_api_keys
SET last_used_at = $2
WHERE key_hash = $1`,
		keyHash,
		time.Now().UTC(),
	); err != nil {
		return claimrequests.Organization{}, fmt.Errorf("update organization api key last used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return claimrequests.Organization{}, err
	}

	return organization, nil
}

func upsertOrganization(ctx context.Context, tx *sql.Tx, record IssueRecord) (claimrequests.Organization, error) {
	var organization claimrequests.Organization
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
		return claimrequests.Organization{}, err
	}

	return organization, nil
}

func insertAPIKey(ctx context.Context, tx *sql.Tx, organization claimrequests.Organization, record IssueRecord) (IssuedAPIKey, error) {
	var issued IssuedAPIKey
	err := tx.QueryRowContext(ctx, `
INSERT INTO organization_api_keys (
    id,
    organization_id,
    key_hash,
    key_prefix,
    name,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, key_prefix, created_at`,
		record.KeyID,
		organization.ID,
		record.KeyHash,
		record.KeyPrefix,
		record.KeyName,
		record.CreatedAt,
	).Scan(
		&issued.KeyID,
		&issued.KeyName,
		&issued.KeyPrefix,
		&issued.CreatedAt,
	)
	if err != nil {
		return IssuedAPIKey{}, err
	}

	issued.Organization = organization
	return issued, nil
}
