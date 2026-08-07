# aipedia-api

Go (Gin) + PostgreSQL API for AIPedia.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness and database ping |

## Run (local)

```bash
docker compose up -d
cp .env.example .env
go mod tidy
go run ./cmd/server
```

Default listen: `:8091`.

## Irancell-T3

Use [`docker-compose.t3.yml`](docker-compose.t3.yml) with images `aipedia-api:latest` and `aipedia-webui:latest` on `t3-net`. Public UI: https://aipedia.xaigrok.ir/
