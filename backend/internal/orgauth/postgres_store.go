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
