CREATE TABLE organization_api_keys (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    name TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT organization_api_keys_hash_not_empty CHECK (LENGTH(TRIM(key_hash)) > 0),
    CONSTRAINT organization_api_keys_prefix_not_empty CHECK (LENGTH(TRIM(key_prefix)) > 0),
    CONSTRAINT organization_api_keys_name_not_empty CHECK (LENGTH(TRIM(name)) > 0)
);

CREATE INDEX organization_api_keys_org_created_idx ON organization_api_keys (organization_id, created_at DESC);
CREATE INDEX organization_api_keys_active_idx ON organization_api_keys (organization_id, revoked_at);
