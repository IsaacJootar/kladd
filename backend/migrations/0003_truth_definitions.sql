CREATE TABLE truth_definitions (
    id UUID PRIMARY KEY,
    truth_key TEXT NOT NULL UNIQUE,
    category TEXT NOT NULL,
    return_type TEXT NOT NULL,
    sensitivity TEXT NOT NULL,
    validity_days INTEGER NOT NULL,
    derivation_rule TEXT NOT NULL,
    required_evidence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT truth_definitions_truth_key_not_empty CHECK (LENGTH(TRIM(truth_key)) > 0),
    CONSTRAINT truth_definitions_category_not_empty CHECK (LENGTH(TRIM(category)) > 0),
    CONSTRAINT truth_definitions_return_type_known CHECK (return_type IN ('boolean', 'enum', 'range', 'status', 'masked_value', 'date')),
    CONSTRAINT truth_definitions_sensitivity_known CHECK (sensitivity IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT truth_definitions_validity_positive CHECK (validity_days > 0),
    CONSTRAINT truth_definitions_derivation_rule_not_empty CHECK (LENGTH(TRIM(derivation_rule)) > 0)
);

CREATE INDEX truth_definitions_category_idx ON truth_definitions (category);
CREATE INDEX truth_definitions_sensitivity_idx ON truth_definitions (sensitivity);

INSERT INTO truth_definitions (
    id,
    truth_key,
    category,
    return_type,
    sensitivity,
    validity_days,
    derivation_rule,
    required_evidence_json
) VALUES
    ('8b5dff61-ff73-4905-8016-b977430f9dd5', 'identity_verified', 'identity', 'boolean', 'high', 365, 'verified_government_identity_evidence', '["passport"]'::jsonb),
    ('c1ef0bd9-d34a-4bf1-9d33-4558a5a1708b', 'age_over_18', 'age', 'boolean', 'low', 365, 'verified_date_of_birth_evidence', '["passport"]'::jsonb),
    ('d2bfe4b4-7404-4c99-a663-63e9a8d4f478', 'address_verified', 'address', 'boolean', 'medium', 180, 'verified_address_evidence', '["utility_bill"]'::jsonb),
    ('5bf7021b-a064-486d-b8e6-0274aa1d29f2', 'degree_verified', 'education', 'boolean', 'medium', 365, 'verified_education_evidence', '["degree_certificate"]'::jsonb),
    ('8970ea3a-0619-46a8-bf61-0556e84bc20a', 'business_registered', 'business', 'boolean', 'medium', 365, 'verified_business_registration_evidence', '["business_registration"]'::jsonb),
    ('fc6bdb04-f352-4c59-982f-924a6ea4055c', 'license_active', 'licensing', 'status', 'medium', 90, 'verified_license_evidence', '["license"]'::jsonb)
ON CONFLICT (truth_key) DO NOTHING;
