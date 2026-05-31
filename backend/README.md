# Kladd Backend

Go API scaffold for Kladd.

## Current Scope

- HTTP server entrypoint
- environment-based config
- health endpoint at `GET /healthz`
- user registration endpoint at `POST /api/users`
- initial PostgreSQL migration for `users` and `audit_logs`
- PostgreSQL connection package
- migration runner command
- Security PIN validation, hashing, comparison, and lockout helpers

No login session, claim, consent, evidence, identity anchor, or truth release logic is implemented in this module.

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

## Environment

```powershell
$env:KLADD_HTTP_ADDR = ":8080"
$env:KLADD_DATABASE_URL = "postgres://kladd:kladd_local_password@localhost:5432/kladd?sslmode=disable"
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

Module 2 adds:

- `users`
- `audit_logs`

These migrations do not create evidence, identity anchor, claim, or consent tables yet.

Run migrations from the backend directory after PostgreSQL is available:

```powershell
go run ./cmd/migrate
```

To use a different migrations directory:

```powershell
go run ./cmd/migrate -dir migrations
```
