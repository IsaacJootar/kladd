# Kladd Backend

Go API scaffold for Kladd.

## Current Scope

- HTTP server entrypoint
- environment-based config
- health endpoint at `GET /healthz`
- initial PostgreSQL migration for `users` and `audit_logs`
- PostgreSQL connection package
- migration runner command
- Security PIN validation, hashing, comparison, and lockout helpers

No claim, consent, evidence, identity anchor, or truth release logic is implemented in this module.

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
