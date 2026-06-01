ALTER TABLE claim_requests
DROP CONSTRAINT claim_requests_status_known;

ALTER TABLE claim_requests
ADD CONSTRAINT claim_requests_status_known
CHECK (status IN ('pending_approval', 'approved_with_security_pin', 'denied', 'expired', 'cancelled'));

CREATE TABLE claims (
    id UUID PRIMARY KEY,
    claim_request_id UUID NOT NULL UNIQUE REFERENCES claim_requests(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT claims_status_known CHECK (status IN ('active', 'expired', 'revoked')),
    CONSTRAINT claims_expires_after_issued CHECK (expires_at > issued_at)
);

CREATE INDEX claims_status_idx ON claims (status);
CREATE INDEX claims_expires_at_idx ON claims (expires_at);

CREATE TABLE consents (
    id UUID PRIMARY KEY,
    claim_request_id UUID NOT NULL REFERENCES claim_requests(id) ON DELETE CASCADE,
    claim_id UUID NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    approved BOOLEAN NOT NULL,
    approval_method TEXT NOT NULL,
    approved_at TIMESTAMPTZ,
    denied_at TIMESTAMPTZ,
    ip_address TEXT,
    user_agent TEXT,
    session_id TEXT,
    CONSTRAINT consents_approval_method_known CHECK (approval_method IN ('security_pin')),
    CONSTRAINT consents_approval_time_consistent CHECK (
        (approved = TRUE AND approved_at IS NOT NULL AND denied_at IS NULL)
        OR
        (approved = FALSE AND denied_at IS NOT NULL AND approved_at IS NULL)
    )
);

CREATE INDEX consents_claim_request_idx ON consents (claim_request_id);
CREATE INDEX consents_claim_idx ON consents (claim_id);
CREATE INDEX consents_user_idx ON consents (user_id);
