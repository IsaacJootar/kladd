CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    payload_json JSONB NOT NULL,
    signature TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT webhook_deliveries_event_type_known CHECK (event_type IN ('claim.approved', 'claim.revoked')),
    CONSTRAINT webhook_deliveries_status_known CHECK (status IN ('pending', 'delivered', 'failed')),
    CONSTRAINT webhook_deliveries_attempts_not_negative CHECK (attempts >= 0),
    CONSTRAINT webhook_deliveries_signature_not_empty CHECK (LENGTH(TRIM(signature)) > 0)
);

CREATE INDEX webhook_deliveries_status_next_attempt_idx ON webhook_deliveries (status, next_attempt_at);
CREATE INDEX webhook_deliveries_organization_created_idx ON webhook_deliveries (organization_id, created_at DESC);
CREATE INDEX webhook_deliveries_aggregate_idx ON webhook_deliveries (aggregate_id);
