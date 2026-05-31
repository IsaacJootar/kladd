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

## WSL PostgreSQL Fallback

If Docker Desktop is unavailable, PostgreSQL can run directly inside Ubuntu/WSL.

Install PostgreSQL in Ubuntu:

```powershell
wsl -d Ubuntu -u root -- bash -lc "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y postgresql postgresql-client"
```

Start PostgreSQL:

```powershell
wsl -d Ubuntu -u root -- service postgresql start
```

Create the local Kladd role and database:

```powershell
wsl -d Ubuntu -u postgres -- createuser kladd
wsl -d Ubuntu -u postgres -- psql -c "ALTER ROLE kladd WITH LOGIN PASSWORD 'kladd_local_password';"
wsl -d Ubuntu -u postgres -- createdb -O kladd kladd
```

Confirm tables after migration:

```powershell
wsl -d Ubuntu -- env PGPASSWORD=kladd_local_password psql -h 127.0.0.1 -U kladd -d kladd -tAc "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;"
```
