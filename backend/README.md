# Backend

Go API foundation for the xzxg-shop Agent demo.

## Run

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
```

## Current APIs

- `GET /api/v1/health`
- `POST /api/v1/agent/sessions`
- `POST /api/v1/agent/sessions/{session_id}/messages:stream`
- `POST /api/v1/agent/runs/{run_id}:cancel`

The first version uses in-memory sessions, runs, mock products, and mock knowledge chunks. It is intentionally shaped so MySQL, Redis, Qdrant, and LLM providers can replace the in-memory pieces without changing the frontend contract.
