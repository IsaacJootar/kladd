package webhooks

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
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

func (store PostgresStore) GetEndpointForOrganization(ctx context.Context, organizationID uuid.UUID) (Endpoint, error) {
	var endpoint Endpoint
	err := store.db.QueryRowContext(ctx, `
SELECT
    endpoint.id,
    endpoint.url,
    endpoint.status,
    endpoint.created_at,
    endpoint.updated_at,
    org.id,
    org.name,
    org.organization_type,
    org.verification_status
FROM organization_webhook_endpoints endpoint
JOIN organizations org ON org.id = endpoint.organization_id
WHERE endpoint.organization_id = $1`,
		organizationID,
	).Scan(
		&endpoint.ID,
		&endpoint.URL,
		&endpoint.Status,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
		&endpoint.Organization.ID,
		&endpoint.Organization.Name,
		&endpoint.Organization.OrganizationType,
		&endpoint.Organization.VerificationStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Endpoint{}, ErrEndpointNotFound
		}
		return Endpoint{}, err
	}

	return endpoint, nil
}

func (store PostgresStore) ListDeliveriesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]DeliveryLog, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT
    id,
    event_type,
    aggregate_id,
    organization_id,
    status,
    attempts,
    next_attempt_at,
    delivered_at,
    created_at,
    updated_at
FROM webhook_deliveries
WHERE organization_id = $1
ORDER BY created_at DESC`,
		organizationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := []DeliveryLog{}
	for rows.Next() {
		var delivery DeliveryLog
		if err := rows.Scan(
			&delivery.ID,
			&delivery.EventType,
			&delivery.AggregateID,
			&delivery.OrganizationID,
			&delivery.Status,
			&delivery.Attempts,
			&delivery.NextAttemptAt,
			&delivery.DeliveredAt,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (store PostgresStore) ListPendingDeliveries(ctx context.Context, dueAt time.Time, limit int) ([]PendingDelivery, error) {
	if limit < 1 {
		limit = 25
	}

	rows, err := store.db.QueryContext(ctx, `
SELECT
    delivery.id,
    delivery.event_type,
    delivery.organization_id,
    delivery.payload_json::text,
    delivery.signature,
    endpoint.url,
    delivery.attempts
FROM webhook_deliveries delivery
JOIN organization_webhook_endpoints endpoint ON endpoint.organization_id = delivery.organization_id
WHERE delivery.status = $1
    AND endpoint.status = $2
    AND (delivery.next_attempt_at IS NULL OR delivery.next_attempt_at <= $3)
ORDER BY delivery.created_at ASC
LIMIT $4`,
		StatusPending,
		EndpointStatusActive,
		dueAt,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := []PendingDelivery{}
	for rows.Next() {
		var delivery PendingDelivery
		if err := rows.Scan(
			&delivery.ID,
			&delivery.EventType,
			&delivery.OrganizationID,
			&delivery.PayloadJSON,
			&delivery.Signature,
			&delivery.EndpointURL,
			&delivery.Attempts,
		); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (store PostgresStore) RecordDeliveryAttempt(ctx context.Context, attempt DeliveryAttempt) error {
	_, err := store.db.ExecContext(ctx, `
UPDATE webhook_deliveries
SET
    status = $2,
    attempts = attempts + 1,
    next_attempt_at = $3,
    delivered_at = CASE WHEN $2 = $4 THEN $5 ELSE delivered_at END,
    updated_at = $5
WHERE id = $1`,
		attempt.DeliveryID,
		attempt.Status,
		attempt.NextAttemptAt,
		StatusDelivered,
		attempt.AttemptedAt,
	)
	return err
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
