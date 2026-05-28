CREATE TABLE users (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    phone TEXT,
    password_hash TEXT NOT NULL,
    security_pin_hash TEXT,
    security_pin_set_at TIMESTAMPTZ,
    pin_failed_attempts INTEGER NOT NULL DEFAULT 0,
    pin_locked_until TIMESTAMPTZ,
    account_type TEXT NOT NULL,
    verification_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_pin_failed_attempts_non_negative CHECK (pin_failed_attempts >= 0),
    CONSTRAINT users_account_type_known CHECK (account_type IN ('individual', 'business', 'organization', 'admin')),
    CONSTRAINT users_verification_status_known CHECK (verification_status IN ('unverified', 'pending', 'verified', 'rejected'))
);

CREATE INDEX users_email_idx ON users (email);
CREATE INDEX users_verification_status_idx ON users (verification_status);
CREATE INDEX users_pin_locked_until_idx ON users (pin_locked_until);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id UUID,
    event_type TEXT NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT audit_logs_actor_type_known CHECK (actor_type IN ('user', 'organization', 'admin', 'system'))
);

CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_type, actor_id);
CREATE INDEX audit_logs_event_type_idx ON audit_logs (event_type);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);
