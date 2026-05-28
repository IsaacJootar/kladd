# Kladd Local Services

Local development services for Kladd:

- PostgreSQL on `localhost:5432`
- Redis on `localhost:6379`
- MinIO S3-compatible API on `localhost:9100`
- MinIO console on `http://localhost:9101`

MinIO uses host ports `9100` and `9101` so the frontend can keep running on `http://localhost:9000`.

## Start

```powershell
docker compose -f infra/docker-compose.yml up -d
```

## Stop

```powershell
docker compose -f infra/docker-compose.yml down
```

## Database Migration

After PostgreSQL is running:

```powershell
cd backend
go run ./cmd/migrate
```
