package claims

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/IsaacJootar/kladd/backend/internal/claimrequests"
	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
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
