# KLADD — Security & Access Rules

## Core Principles
- minimum exposure
- no unrestricted document transfer
- auditability
- time-bound access
- revocable claims
- Security PIN approval for every claim release

## Sensitive Data Rules
Critical identity anchors include BVN, NIN, passport number, and Tax IDs.

They must:
- be encrypted
- be isolated
- never exposed directly by default

## Security PIN Rules
- 4–6 digits
- stored hashed only
- never logged
- required for every claim approval
- rate limited
- lock approval after repeated failed attempts
- reset requires re-authentication

## Claim Rules
- claims are temporary
- expired claims hide truths
- revoked claims hide truths
- all retrievals logged
- no claim is issued without consent approval

## Admin Rules
- role-based permissions
- restricted sensitive access
- audit all admin actions

## API Security
- signed webhooks
- rate limiting
- token expiration
- request validation
