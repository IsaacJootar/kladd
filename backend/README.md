# Kladd Backend

Go API scaffold for Kladd.

## Current Scope

- HTTP server entrypoint
- environment-based config
- health endpoint at `GET /healthz`
- user registration endpoint at `POST /api/users`
- user login endpoint at `POST /api/auth/login`
- current account endpoint at `GET /api/account/me`
- Security PIN setup endpoint at `POST /api/account/security-pin`
- evidence metadata endpoints at `GET /api/evidence-items` and `POST /api/evidence-items`
- truth registry endpoint at `GET /api/truth-definitions`
- PostgreSQL migrations for `users`, `audit_logs`, `evidence_items`, and `truth_definitions`
- PostgreSQL connection package
- migration runner command
- Security PIN validation, hashing, comparison, and lockout helpers

No claim, consent approval, identity anchor, Security PIN reset, refresh token, evidence download, truth derivation, or truth release logic is implemented in this module.

## User Registration

`POST /api/users` creates an account and returns only non-sensitive user fields.

```json
{
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "phone": "08030000000",
  "password": "strong-password",
  "account_type": "individual"
}
```

Passwords are hashed before storage. Responses do not include passwords or password hashes.

## Login

`POST /api/auth/login` verifies email and password, records a login audit event, and returns a short-lived JWT access token.

```json
{
  "email": "ada@example.com",
  "password": "strong-password"
}
```

Login responses include only safe user fields and token metadata.

## Current Account

`GET /api/account/me` returns safe profile fields for the authenticated user.

Requests require `Authorization: Bearer <access_token>`. Responses do not include passwords, password hashes, Security PIN values, Security PIN hashes, raw documents, or sensitive identity anchors.

## Security PIN Setup

`POST /api/account/security-pin` stores a hashed Security PIN for the authenticated user.

```json
{
  "security_pin": "4829"
}
```

Security PINs must be 4-6 digits. Requests require `Authorization: Bearer <access_token>`. Responses do not include the PIN or PIN hash.

## Evidence Items

`GET /api/evidence-items` returns metadata-only records for the authenticated user.

`POST /api/evidence-items` accepts `multipart/form-data`:

- `category`
- `display_name`
- `file`

Files are stored with local storage for the MVP. API responses do not include internal file paths, download URLs, raw document contents, sensitive identity anchors, Security PIN values, or hashes.

## Truth Definitions

`GET /api/truth-definitions` returns supported truth registry metadata for authenticated users.

Responses include keys, categories, return types, sensitivity, validity durations, derivation rule names, and required evidence categories. Responses do not include derived truth values, raw documents, sensitive identity anchors, Security PIN values, or hashes.

## Environment

```powershell
$env:KLADD_HTTP_ADDR = ":8080"
$env:KLADD_DATABASE_URL = "postgres://kladd:kladd_local_password@localhost:5432/kladd?sslmode=disable"
$env:KLADD_JWT_SECRET = "local-dev-change-me"
$env:KLADD_WEBHOOK_SIGNING_SECRET = "local-dev-webhook-secret"
$env:KLADD_STORAGE_DIR = "storage"
```

See `.env.example` for the local defaults.

## Run

```powershell
go run ./cmd/api
```

Optional:

```powershell
$env:KLADD_HTTP_ADDR = ":8080"
go run ./cmd/api
```

## Migrations

Initial SQL migrations live in `migrations/`.

Current migrations add:

- `users`
- `audit_logs`
- `evidence_items`
- `truth_definitions`

These migrations do not create identity anchor, claim, or consent tables yet.

Run migrations from the backend directory after PostgreSQL is available:

```powershell
go run ./cmd/migrate
```

To use a different migrations directory:

```powershell
go run ./cmd/migrate -dir migrations
```

## Organization API Keys

Create a local organization API key from the backend directory:

```powershell
go run ./cmd/orgkey -organization "Acme Bank" -type bank -name "Local setup"
```

The command prints the raw API key once. Kladd stores only the key hash. Use the raw value in `X-Kladd-API-Key` when calling organization endpoints.

## Claim Expiry Sweep

Run the expiry sweep from the backend directory:

```powershell
go run ./cmd/expireclaims
```

The command marks due active claims as expired, records audit events, and queues signed `claim.expired` webhook deliveries. Expired claim responses continue to hide proof details.
