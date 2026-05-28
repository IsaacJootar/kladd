# KLADD — Claim Exchange Specification

## Purpose
Defines how claims move between users, Kladd, and organizations.

## Exchange Modes

### 1. User-Initiated Exchange
Form → approval request → Security PIN → claim → form receives verified truth.

### 2. Institution-Initiated Exchange
Organization creates request → user receives approval request → user reviews and enters Security PIN → claim released.

### 3. API Exchange
Organization sends request by API. User approval is required unless a valid active consent exists. Claim delivered by API or webhook.

### 4. QR Exchange
User shows QR code. Organization scans. Temporary active claim is retrieved.

### 5. PIN Exchange
User generates short exchange PIN. Organization enters PIN into verification portal.

Security PIN and Exchange PIN are different:
- Security PIN approves claim release.
- Exchange PIN retrieves a temporary claim.

## Claim Lifecycle
request_created → pending_approval → approved_with_security_pin → active → expired/revoked

## Rule
Expired or revoked claims must no longer expose truth details.
