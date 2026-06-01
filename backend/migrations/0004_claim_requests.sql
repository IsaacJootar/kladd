CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    organization_type TEXT NOT NULL,
    verification_status TEXT NOT NULL DEFAULT 'unverified',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT organizations_name_not_empty CHECK (LENGTH(TRIM(name)) > 0),
    CONSTRAINT organizations_type_not_empty CHECK (LENGTH(TRIM(organization_type)) > 0),
    CONSTRAINT organizations_verification_status_known CHECK (verification_status IN ('unverified', 'pending', 'verified', 'rejected'))
);

CREATE INDEX organizations_verification_status_idx ON organizations (verification_status);

CREATE TABLE claim_requests (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL,
    scope_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT claim_requests_purpose_not_empty CHECK (LENGTH(TRIM(purpose)) > 0),
    CONSTRAINT claim_requests_status_known CHECK (status IN ('pending_approval', 'approved', 'denied', 'expired', 'cancelled')),
    CONSTRAINT claim_requests_expires_after_created CHECK (expires_at > created_at)
);

CREATE INDEX claim_requests_user_created_idx ON claim_requests (user_id, created_at DESC);
CREATE INDEX claim_requests_org_created_idx ON claim_requests (organization_id, created_at DESC);
CREATE INDEX claim_requests_status_idx ON claim_requests (status);
