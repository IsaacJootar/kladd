CREATE TABLE organization_webhook_endpoints (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT organization_webhook_endpoints_url_not_empty CHECK (LENGTH(TRIM(url)) > 0),
    CONSTRAINT organization_webhook_endpoints_status_known CHECK (status IN ('active', 'disabled'))
);

CREATE INDEX organization_webhook_endpoints_status_idx ON organization_webhook_endpoints (status);
