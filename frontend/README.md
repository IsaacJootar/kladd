# Kladd Frontend

Next.js frontend for Kladd.

## Current Scope

- account registration screen
- login screen
- current account display
- Security PIN setup screen
- evidence record cards and upload form
- local API proxy to the Go backend

The frontend does not display passwords, password hashes, Security PIN hashes, raw documents, or sensitive identity anchors.

## Run

```powershell
npm run dev -- --webpack -p 9000
```

Open:

```text
http://localhost:9000
```

The frontend proxies browser requests from `/api/kladd/*` to the Go backend at:

```text
http://localhost:8080
```

To use a different backend URL:

```powershell
$env:KLADD_API_BASE_URL = "http://localhost:8080"
```
