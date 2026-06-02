CREATE TABLE claim_exchange_pins (
    id UUID PRIMARY KEY,
    claim_id UUID NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    pin_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT claim_exchange_pins_hash_not_empty CHECK (LENGTH(TRIM(pin_hash)) > 0),
    CONSTRAINT claim_exchange_pins_expires_after_created CHECK (expires_at > created_at)
);

CREATE INDEX claim_exchange_pins_claim_idx ON claim_exchange_pins (claim_id);
CREATE INDEX claim_exchange_pins_expires_at_idx ON claim_exchange_pins (expires_at);
