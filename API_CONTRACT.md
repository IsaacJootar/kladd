# KLADD — API Contract

## Authentication
- JWT access tokens
- API keys for organizations

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
- GET /api/claim-requests/{id}
- POST /api/claim-requests/{id}/approve
- POST /api/claim-requests/{id}/deny
- GET /api/claims/{id}
- GET /api/claims/{id}/status
- POST /api/claims/{id}/revoke

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

## Webhook Events
- claim.approved
- claim.denied
- claim.expired
- claim.revoked

## Verification URL
GET /verify/{claim_id}

## API Principles
- no raw document exposure
- short-lived access
- signed responses
- auditable access
- no claim issued without Security PIN approval
