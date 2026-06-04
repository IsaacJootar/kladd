ALTER TABLE organizations
ADD COLUMN email TEXT,
ADD COLUMN password_hash TEXT,
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
ADD CONSTRAINT organizations_email_not_empty CHECK (
    email IS NULL OR LENGTH(TRIM(email)) > 0
),
ADD CONSTRAINT organizations_password_hash_not_empty CHECK (
    password_hash IS NULL OR LENGTH(TRIM(password_hash)) > 0
),
ADD CONSTRAINT organizations_credentials_complete CHECK (
    (email IS NULL AND password_hash IS NULL)
    OR (email IS NOT NULL AND password_hash IS NOT NULL)
);

CREATE UNIQUE INDEX organizations_email_unique_idx ON organizations (email) WHERE email IS NOT NULL;
