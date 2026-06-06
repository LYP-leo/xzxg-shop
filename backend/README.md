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
  -> risk check / photo-search fallback
  -> multi-turn memory retrieval
  -> small-model planner
  -> ReAct tool loop using large answer model for guide/non-guide
  -> tools / skills / product retrieval / knowledge retrieval / cart / order
  -> <final>...</final> single-call streaming answer
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
agent.prompt.route
agent.prompt.guide_intent
agent.prompt.main_template
agent.prompt.tool_call_protocol
agent.prompt.final_output_rules
agent.prompt.intent_tool_policy
agent.prompt.followups
agent.prompt.intent.*
retrieval.keyword.top_n
retrieval.vector.top_n
retrieval.rerank.weight.*
risk.blocked_terms
http.cors.allowed_origins
http.trusted_proxy_cidrs
files.max_upload_bytes
minio.endpoint
minio.access_key
minio.secret_key
milvus.token
```

Secret config values such as `ai.api_key` are masked in list responses. The Agent runtime refreshes dynamic config on demand with a default 15-second cache. The main Agent prompt is assembled from `agent.prompt.main_template`, runtime-selected tools/skills, `agent.prompt.final_output_rules`, and the active `agent.prompt.intent.*` content.

Nacos connection env:

```text
NACOS_ADDR=http://127.0.0.1:8848
NACOS_NAMESPACE=
NACOS_GROUP=XZXG_SHOP
NACOS_DATA_ID=xzxg-shop-app-config.json
```

## Current APIs

- `GET /api/v1/health`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `POST /api/v1/agent/sessions`
- `POST /api/v1/agent/sessions/{session_id}/messages:stream`
- `POST /api/v1/agent/runs/{run_id}:cancel`
- `GET /api/v1/agent/runs/{run_id}/trace`
- `GET /api/v1/products?page=1&page_size=10&keyword=&category_id=`
- `GET /api/v1/merchants?page=1&page_size=10`
- `POST /api/v1/files`
- `GET /api/v1/files/{file_id}`
- `GET /api/v1/merchant/documents?page=1&page_size=10`
- `POST /api/v1/merchant/documents`
- `POST /api/v1/merchant/unstructured-ingestions`
- `GET /api/v1/admin/documents?page=1&page_size=10`
- `POST /api/v1/admin/unstructured-ingestions`

The API uses MySQL for sessions, runs, products, SKUs, cart items, files, and knowledge chunks. Local development applies `migrations/001_mysql_schema.sql` on startup by default. In production, set `APP_ENV=production` and run migrations as a separate job; the API will not run migrations unless `RUN_MIGRATIONS=true`.

Production startup also requires explicit non-default secrets and endpoints. `root:root`, `minioadmin`, wildcard CORS, empty model keys, and the local Milvus token are rejected when `APP_ENV=production`.

Vector bootstrap is enabled by default only outside production. In production, run it as an explicit job with `BOOTSTRAP_VECTOR_INDEX=true` to avoid repeated embedding work during rolling deploys.

Authentication uses bcrypt for new password hashes and random bearer tokens stored as SHA-256 digests. Legacy SHA-256 password hashes are accepted only to migrate the hash on successful login.

File downloads require authentication and are restricted to the file owner or an admin account. Upload MIME type is determined by server-side content detection rather than the client-provided `Content-Type`.

Unstructured ingestion accepts text, HTML, JSON text, or an HTTP/HTTPS `source_url`, normalizes it into clean text, writes a knowledge document, splits chunks through the existing RAG splitter, and deduplicates by content hash per merchant unless `force_reindex=true`.

Example ingestion request:

```json
{
  "merchant_id": "m_001",
  "title": "运动鞋选购笔记",
  "source_type": "html",
  "source_url": "https://example.com/post/123",
  "html": "<article>跑步鞋要关注缓震、足弓支撑和尺码...</article>",
  "metadata": {
    "platform": "external_web",
    "category": "运动鞋"
  }
}
```

Agent trace events are stored in `agent_trace_events` and include planner, retrieval, answer, follow-up, and run completion events. Use `run_id` from the SSE `message_start` event to query the trace endpoint.

## Idempotency and Transactions

- `POST /api/v1/agent/sessions/{session_id}/messages:stream` requires a client-generated `client_message_id`.
- The store enforces `(account_id, session_id, client_message_id)` uniqueness for user messages and `(account_id, message_id)` uniqueness for runs.
- Duplicate message submissions return the existing run snapshot and do not execute the Agent again.
- Cart add uses atomic upsert on `(account_id, product_id, sku_id)`.
- Checkout locks selected cart rows in a transaction, reserves product/SKU stock, creates orders/payments/items, and clears selected cart rows atomically.
