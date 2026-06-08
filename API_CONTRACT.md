# KLADD — API Contract

## Authentication
- JWT access tokens
- JWT access tokens for organizations
- API keys for organization integrations

## Account Endpoints
- POST /api/users
- POST /api/auth/login
- GET /api/account/me
- POST /api/account/security-pin
- POST /api/account/security-pin/reset
- GET /api/audit-logs

## Evidence Endpoints
- GET /api/evidence-items
- POST /api/evidence-items

## Truth Registry Endpoints
- GET /api/truth-definitions

## Core Endpoints
- POST /api/claim-requests
- POST /api/organizations
- POST /api/organization/auth/login
- GET /api/organization/me
- GET /api/organization/audit-logs
- POST /api/organization/claim-requests
- GET /api/organization/claim-requests
- GET /api/organization/claims
- GET /api/organization/webhook-endpoint
- POST /api/organization/webhook-endpoint
- GET /api/organization/webhook-deliveries
- GET /api/claim-requests/{id}
- POST /api/claim-requests/{id}/approve
- POST /api/claim-requests/{id}/deny
- GET /api/claims/{id}
- GET /api/claims/{id}/status
- POST /api/claims/{id}/revoke
- POST /api/claims/{id}/exchange-pin
- POST /api/exchange-pins/resolve

## Create User Request Body
```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "phone": "08030000000",
  "password": "strong-password",
  "account_type": "individual"
}
```

Create user responses must not include passwords, password hashes, raw documents, or sensitive identity anchors.

## Login Request Body
```json
{
  "email": "ada@example.com",
  "password": "strong-password"
}
```

Login responses return a short-lived JWT access token and safe user fields only. They must not include passwords, password hashes, raw documents, or sensitive identity anchors.

## Create Organization Request Body
POST /api/organizations

```json
{
  "name": "Acme Bank",
  "email": "admin@example.com",
  "password": "strong-password",
  "organization_type": "bank"
}
```

Create organization responses return safe organization account fields only. They must not include passwords, password hashes, API keys, API key hashes, raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, or truth values.

## Organization Login Request Body
POST /api/organization/auth/login

```json
{
  "email": "admin@example.com",
  "password": "strong-password"
}
```

Organization login responses return a short-lived JWT access token and safe organization fields only. They must not include passwords, password hashes, API keys, API key hashes, raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, or truth values.

## Current Account
GET /api/account/me

Requires `Authorization: Bearer <access_token>`.

Current account responses return safe user fields only. They must not include passwords, password hashes, Security PIN values, Security PIN hashes, raw documents, or sensitive identity anchors.

## Set Security PIN Request Body
```json
{
  "security_pin": "4829"
}
```

Security PIN setup requires `Authorization: Bearer <access_token>`. Responses must not include the PIN or PIN hash.

## Reset Security PIN Request Body
```json
{
  "password": "strong-password",
  "security_pin": "7391"
}
```

Security PIN reset requires `Authorization: Bearer <access_token>` and account password re-authentication. Responses must not include the password, password hash, PIN, or PIN hash.

## Create Evidence Item Request
POST /api/evidence-items

Requires `Authorization: Bearer <access_token>`.

Request content type: `multipart/form-data`

Fields:
- `category`
- `display_name`
- `file`

Evidence responses return metadata only. They must not include internal `file_path`, download URLs, raw document contents, sensitive identity anchors, Security PIN values, or hashes.

## Truth Definitions
GET /api/truth-definitions

Requires `Authorization: Bearer <access_token>`.

Truth definition responses return registry metadata only. They must not include derived truth values, raw documents, unrestricted evidence, sensitive identity anchors, Security PIN values, or hashes.

## Approve Request Body
```json
{
  "security_pin": "4829"
}
```

## Organization Claim Request
GET /api/organization/me

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns safe organization profile fields only:
- id
- name
- organization_type
- verification_status

Responses must not include API keys, API key hashes, raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, or truth values.

GET /api/organization/audit-logs

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns sanitized activity history for the authenticated organization. Responses include event type, safe title, safe description, and timestamp only. They must not include raw metadata, raw documents, webhook payloads, webhook signatures, sensitive identity anchors, Security PIN values, Security PIN hashes, exchange PIN hashes, API keys, API key hashes, or truth values.

POST /api/organization/claim-requests

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Local MVP API keys can be created with:

```powershell
go run ./cmd/orgkey -organization "Acme Bank" -type bank -name "Local setup"
```

```json
{
  "user_email": "ada@example.com",
  "purpose": "Account opening",
  "requested_truths": ["identity_verified"],
  "duration_days": 30
}
```

This creates a pending claim request for the target user. It must not issue a claim or release truths. The user must still approve with their Security PIN before any claim becomes active.

GET /api/organization/claim-requests

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns claim requests created by the authenticated organization. Responses include request metadata, purpose, requested proof types, status, and expiry only. They must not include raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, API key hashes, or truth values.

GET /api/organization/claims

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns claims belonging to the authenticated organization. Active claims may show approved proof types. Expired or revoked claims must hide proof details. Responses must not include raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, API key hashes, exchange PIN hashes, or truth values.

## Webhook Events
- claim.approved
- claim.denied
- claim.expired
- claim.revoked

Webhook delivery foundation records signed webhook payloads in an internal outbox table. Current MVP payloads include safe claim metadata only:

- event_type
- claim_id
- claim_request_id
- organization_id
- status
- expires_at
- occurred_at
- verification_path

Webhook payloads must not include raw documents, truth values, sensitive identity anchors, Security PIN values, Security PIN hashes, or exchange PIN hashes.

Local MVP webhook endpoints can be configured with:

```powershell
go run ./cmd/webhookendpoint -organization "Acme Bank" -type bank -url "https://example.com/kladd/webhooks"
```

Deliver pending webhook events with:

```powershell
go run ./cmd/deliverwebhooks
```

The delivery command sends already-signed safe payloads to active endpoints, marks successful deliveries, and schedules failed attempts for retry.

## Organization Webhook Endpoint
GET /api/organization/webhook-endpoint

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns the authenticated organization's configured webhook endpoint metadata only:
- id
- organization
- url
- status
- created_at
- updated_at

Responses must not include webhook signatures, payload bodies, API keys, API key hashes, raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, exchange PIN hashes, or truth values.

POST /api/organization/webhook-endpoint

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

```json
{
  "url": "https://example.com/kladd/webhooks"
}
```

Creates or updates the authenticated organization's active webhook endpoint. The endpoint URL must use `http` or `https`.

GET /api/organization/webhook-deliveries

Requires `Authorization: Bearer <organization_access_token>` for dashboard access. `X-Kladd-API-Key` remains supported for organization integrations.

Returns safe webhook delivery history for the authenticated organization. Responses include event type, aggregate id, status, attempts, delivery timing, and retry timing only. They must not include webhook signatures, payload bodies, raw documents, sensitive identity anchors, Security PIN values, Security PIN hashes, exchange PIN hashes, API key hashes, or truth values.

## Verification URL
GET /verify/{claim_id}

## Exchange PIN
POST /api/claims/{id}/exchange-pin

Requires `Authorization: Bearer <access_token>`.

Creates a short temporary exchange PIN for an active claim only. Responses include the one-time visible PIN and expiry, but never include PIN hashes, raw documents, sensitive identity anchors, Security PIN values, or Security PIN hashes.

POST /api/exchange-pins/resolve

```json
{
  "exchange_pin": "12345678"
}
```

Resolves a temporary exchange PIN to the existing safe claim verification response. Expired PINs, expired claims, revoked claims, and inactive claims must not expose truth details.

## API Principles
- no raw document exposure
- short-lived access
- signed responses
- auditable access
- no claim issued without Security PIN approval
