# KLADD — Database Schema

## Core Tables

### users
- id
- name
- email
- phone
- password_hash
- security_pin_hash
- security_pin_set_at
- pin_failed_attempts
- pin_locked_until
- account_type
- verification_status
- created_at

### organizations
- id
- name
- organization_type
- verification_status

### evidence_items
- id
- user_id
- category
- file_path
- status
- metadata_json
- uploaded_at

### identity_anchors
- id
- user_id
- anchor_type
- encrypted_value
- verification_status

### truth_definitions
- id
- truth_key
- category
- return_type
- sensitivity
- validity_days
- derivation_rule
- required_evidence_json
- created_at

### claim_requests
- id
- organization_id
- user_id
- purpose
- scope_json
- status
- expires_at

### consents
- id
- claim_request_id
- claim_id
- user_id
- organization_id
- approved
- approval_method
- approved_at
- denied_at
- ip_address
- user_agent
- session_id

### claims
- id
- claim_request_id
- status
- issued_at
- expires_at
- revoked_at

### claim_exchange_pins
- id
- claim_id
- pin_hash
- expires_at
- created_at

### claim_truths
- id
- claim_id
- truth_key
- truth_value

### claim_access_logs
- id
- claim_id
- accessed_by
- access_time
- ip_address

### audit_logs
- id
- actor_type
- actor_id
- event_type
- metadata_json
- created_at
