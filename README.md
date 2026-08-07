# aipedia-api

Go (Gin) + PostgreSQL API for AIPedia.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness and database ping |
| GET | `/api/categories` | Root categories, or children via `?parent=ai` |
| GET | `/api/categories/*path` | Category by path (`ai`, `ai/tools`) |
| GET | `/api/categories/*path/children` | Child categories |
| GET | `/api/categories/*path/entries` | Leaf entries (`?q=` optional) |
| GET | `/api/entries?q=` | Global entry search |

## Run (local)

```bash
docker compose up -d
cp .env.example .env
go mod tidy
go run ./cmd/import-bookmarks
go run ./cmd/server
```

`BOOKMARKS_DIR` defaults to `../bookmarks` (sibling clone). Re-run import after bookmark edits.

Default listen: `:8091` (set `ADDR` in `.env` if that port is taken locally).

Local Postgres maps to host port **5435** (`5433` is often used by other stacks).

## Irancell-T3

Use [`docker-compose.t3.yml`](docker-compose.t3.yml) with images `aipedia-api:latest` and `aipedia-webui:latest` on `t3-net`. Public UI: https://aipedia.xaigrok.ir/
