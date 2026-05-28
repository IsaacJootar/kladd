# Kladd Backend

Go API scaffold for Kladd.

## Current Scope

- HTTP server entrypoint
- environment-based config
- health endpoint at `GET /healthz`

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
