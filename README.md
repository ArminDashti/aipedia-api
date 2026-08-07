# aipedia-api

Go (Gin) + PostgreSQL API for AIPedia.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness and database ping |

## Run

```bash
# Postgres (Docker) — host port 5433
docker compose up -d

cp .env.example .env
go mod tidy
go run ./cmd/server
```

Default listen: `:8091`.
