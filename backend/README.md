# Kladd Backend

Go API scaffold for Kladd.

## Current Scope

- HTTP server entrypoint
- environment-based config
- health endpoint at `GET /healthz`
- initial PostgreSQL migration for `users` and `audit_logs`
- Security PIN validation, hashing, comparison, and lockout helpers

No claim, consent, evidence, identity anchor, or truth release logic is implemented in this module.

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
