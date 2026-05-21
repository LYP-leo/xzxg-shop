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

## Dynamic Config

Runtime configuration is stored in Nacos and can be changed through the admin APIs without restarting the service. Environment variables are still supported as startup defaults and in-memory fallback values when Nacos is unavailable.

Admin APIs:

```text
GET /api/v1/admin/configs
PATCH /api/v1/admin/configs/{config_key}
```

Initial config keys:

```text
ai.enabled
ai.base_url
ai.api_key
ai.small_model
ai.large_model
ai.enable_thinking
agent.followups_enabled
agent.config_refresh_seconds
agent.prompt.planner
agent.prompt.answer_base
agent.prompt.followups
agent.prompt.intent.*
```

Secret config values such as `ai.api_key` are masked in list responses. The Agent runtime refreshes dynamic config on demand with a default 15-second cache.

Nacos connection env:

```text
NACOS_ADDR=http://127.0.0.1:8848
NACOS_NAMESPACE=
NACOS_GROUP=XZXG_SHOP
NACOS_DATA_ID=xzxg-shop-app-config.json
```

## Current APIs

- `GET /api/v1/health`
- `POST /api/v1/agent/sessions`
- `POST /api/v1/agent/sessions/{session_id}/messages:stream`
- `POST /api/v1/agent/runs/{run_id}:cancel`
- `GET /api/v1/agent/runs/{run_id}/trace`

The API uses MySQL for sessions, runs, products, SKUs, cart items, and knowledge chunks. On startup it applies `migrations/001_mysql_schema.sql`, which creates the required tables and inserts initial demo catalog data if missing.

Agent trace events are stored in `agent_trace_events` and include planner, retrieval, answer, follow-up, and run completion events. Use `run_id` from the SSE `message_start` event to query the trace endpoint.
