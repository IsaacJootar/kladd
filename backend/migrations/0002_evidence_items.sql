CREATE TABLE evidence_items (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT evidence_items_category_not_empty CHECK (LENGTH(TRIM(category)) > 0),
    CONSTRAINT evidence_items_status_known CHECK (status IN ('uploaded', 'pending_verification', 'verified', 'rejected', 'expired'))
);

CREATE INDEX evidence_items_user_uploaded_idx ON evidence_items (user_id, uploaded_at DESC);
CREATE INDEX evidence_items_status_idx ON evidence_items (status);
