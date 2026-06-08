package orgauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func (store PostgresStore) GetOrganization(ctx context.Context, id uuid.UUID) (claimrequests.Organization, error) {
	var organization claimrequests.Organization
	err := store.db.QueryRowContext(ctx, `
SELECT
    id,
    name,
    organization_type,
    verification_status
FROM organizations
WHERE id = $1`,
		id,
	).Scan(
		&organization.ID,
		&organization.Name,
		&organization.OrganizationType,
		&organization.VerificationStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return claimrequests.Organization{}, ErrInvalidOrganization
		}
		return claimrequests.Organization{}, err
	}

	return organization, nil
}

func (store PostgresStore) RegisterAccount(ctx context.Context, record RegisterRecord) (Account, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	account, err := insertOrganizationAccount(ctx, tx, record)
	if err != nil {
		if isUniqueViolation(err) {
			return Account{}, ErrEmailTaken
		}
		return Account{}, err
	}

	if err := insertOrganizationCreatedAudit(ctx, tx, account); err != nil {
		return Account{}, err
	}

	if err := tx.Commit(); err != nil {
		return Account{}, err
	}

	return account, nil
}

func (store PostgresStore) FindCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	var credentials Credentials
	err := store.db.QueryRowContext(ctx, `
SELECT
    id,
    name,
    email,
    password_hash,
    organization_type,
    verification_status,
    created_at
FROM organizations
WHERE email = $1
    AND password_hash IS NOT NULL`,
		email,
	).Scan(
		&credentials.Account.ID,
		&credentials.Account.Name,
		&credentials.Account.Email,
		&credentials.PasswordHash,
		&credentials.Account.OrganizationType,
		&credentials.Account.VerificationStatus,
		&credentials.Account.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Credentials{}, ErrInvalidCredentials
		}
		return Credentials{}, err
	}

	return credentials, nil
}

func (store PostgresStore) RecordLogin(ctx context.Context, account Account) error {
	metadata, err := json.Marshal(map[string]string{
		"method": "password",
	})
	if err != nil {
		return err
	}

	_, err = store.db.ExecContext(ctx, `
INSERT INTO audit_logs (
    id,
    actor_type,
    actor_id,
    event_type,
    metadata_json
) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		uuid.New(),
		"organization",
		account.ID,
		"organization.login",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert organization login audit: %w", err)
	}

	return nil
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

func insertOrganizationAccount(ctx context.Context, tx *sql.Tx, record RegisterRecord) (Account, error) {
	var account Account
	err := tx.QueryRowContext(ctx, `
INSERT INTO organizations (
    id,
    name,
    email,
    password_hash,
    organization_type,
    verification_status,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING id, name, email, organization_type, verification_status, created_at`,
		record.ID,
		record.Name,
		record.Email,
		record.PasswordHash,
		record.OrganizationType,
		record.VerificationStatus,
		record.CreatedAt,
	).Scan(
		&account.ID,
		&account.Name,
		&account.Email,
		&account.OrganizationType,
		&account.VerificationStatus,
		&account.CreatedAt,
	)
	if err != nil {
		return Account{}, err
	}

	return account, nil
}

func insertOrganizationCreatedAudit(ctx context.Context, tx *sql.Tx, account Account) error {
	metadata, err := json.Marshal(map[string]string{
		"email":             account.Email,
		"organization_type": account.OrganizationType,
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
		account.ID,
		"organization.created",
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert organization created audit: %w", err)
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
