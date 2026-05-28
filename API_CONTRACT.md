# KLADD — API Contract

## Authentication
- JWT access tokens
- API keys for organizations

## Core Endpoints
- POST /api/claim-requests
- GET /api/claim-requests/{id}
- POST /api/claim-requests/{id}/approve
- POST /api/claim-requests/{id}/deny
- GET /api/claims/{id}
- GET /api/claims/{id}/status
- POST /api/claims/{id}/revoke
- POST /api/account/security-pin
- POST /api/account/security-pin/reset

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
