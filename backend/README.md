# Backend

Go API foundation for the xzxg-shop Agent demo.

## Run

Start MySQL first. The default local DSN matches the Docker container used in this repo:

```text
root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local
```

```bash
cd backend
go run ./cmd/api
```

Default address:

```text
http://localhost:8080
```

Override:

```bash
API_ADDR=:8081 go run ./cmd/api
MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local' go run ./cmd/api
```

## Current APIs

- `GET /api/v1/health`
- `POST /api/v1/agent/sessions`
- `POST /api/v1/agent/sessions/{session_id}/messages:stream`
- `POST /api/v1/agent/runs/{run_id}:cancel`

The API uses MySQL for sessions, runs, products, SKUs, cart items, and knowledge chunks. On startup it applies `migrations/001_mysql_schema.sql`, which creates the required tables and inserts initial demo catalog data if missing.
