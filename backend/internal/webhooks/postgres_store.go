package webhooks

import (
	"context"
	"database/sql"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) PostgresStore {
	return PostgresStore{db: db}
}

func (store PostgresStore) ConfigureEndpoint(ctx context.Context, record ConfigureEndpointRecord) (Endpoint, error) {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Endpoint{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	organization, err := upsertEndpointOrganization(ctx, tx, record)
	if err != nil {
		return Endpoint{}, err
	}

	endpoint, err := upsertWebhookEndpoint(ctx, tx, organization, record)
	if err != nil {
		return Endpoint{}, err
	}

	if err := tx.Commit(); err != nil {
		return Endpoint{}, err
	}

	return endpoint, nil
}

func upsertEndpointOrganization(ctx context.Context, tx *sql.Tx, record ConfigureEndpointRecord) (Organization, error) {
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

func upsertWebhookEndpoint(ctx context.Context, tx *sql.Tx, organization Organization, record ConfigureEndpointRecord) (Endpoint, error) {
	var endpoint Endpoint
	err := tx.QueryRowContext(ctx, `
INSERT INTO organization_webhook_endpoints (
    id,
    organization_id,
    url,
    status,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (organization_id) DO UPDATE
SET
    url = EXCLUDED.url,
    status = EXCLUDED.status,
    updated_at = EXCLUDED.updated_at
RETURNING id, url, status, created_at, updated_at`,
		record.ID,
		organization.ID,
		record.URL,
		record.Status,
		record.ConfiguredAt,
	).Scan(
		&endpoint.ID,
		&endpoint.URL,
		&endpoint.Status,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
	)
	if err != nil {
		return Endpoint{}, err
	}

	endpoint.Organization = organization
	return endpoint, nil
}
