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

## Agent Models

The Agent runtime uses DashScope's OpenAI-compatible API when `DASHSCOPE_API_KEY` is configured. Keep API keys in local environment variables or secret management, never in source code.

```bash
export DASHSCOPE_API_KEY='your-new-key'
export AI_BASE_URL='https://dashscope.aliyuncs.com/compatible-mode/v1'
export AI_SMALL_MODEL='qwen3.5-flash'
export AI_LARGE_MODEL='qwen3.6-plus'
go run ./cmd/api
```

Current main chain:

```text
user message
  -> small-model planner
  -> product / knowledge retrieval
  -> small or large answer model
  -> small-model follow-up generation
  -> SSE events for frontend rendering
```

If `DASHSCOPE_API_KEY` is not set, the runtime falls back to a local rule-based answer so local development can still run.

## Current APIs

- `GET /api/v1/health`
- `POST /api/v1/agent/sessions`
- `POST /api/v1/agent/sessions/{session_id}/messages:stream`
- `POST /api/v1/agent/runs/{run_id}:cancel`
- `GET /api/v1/agent/runs/{run_id}/trace`

The API uses MySQL for sessions, runs, products, SKUs, cart items, and knowledge chunks. On startup it applies `migrations/001_mysql_schema.sql`, which creates the required tables and inserts initial demo catalog data if missing.

Agent trace events are stored in `agent_trace_events` and include planner, retrieval, answer, follow-up, and run completion events. Use `run_id` from the SSE `message_start` event to query the trace endpoint.
