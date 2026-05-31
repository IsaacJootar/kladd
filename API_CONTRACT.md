# KLADD — API Contract

## Authentication
- JWT access tokens
- API keys for organizations

## Account Endpoints
- POST /api/users
- POST /api/account/security-pin
- POST /api/account/security-pin/reset

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
