# xzxg-shop
Shopping system by piggy and doggy

## Project Layout

```text
backend/        Go API, Agent runtime, RAG, MySQL store
frontend/       React web client for user / merchant / admin
android-native/ Java native Android client
quality/        Eval datasets, scripts, and reports
deployments/    Local middleware and deployment assets
.ai/            Product, API, and technical docs
```

## Current Scope

- Three roles: customer, merchant, and administrator.
- Customer: AI shopping agent, products, cart, orders, coupons, profile.
- Merchant: product management, order management, promotions, reviews, documents.
- Administrator: platform data management, prompt/config management, Agent trace, vector status, quality reports.
- Agent runtime: two-stage intent routing, ReAct tool loop, `<final>...</final>` streaming final protocol, multi-turn memory, risk blocking, tool/skill policy, full trace.
- Retrieval: MySQL keyword retrieval plus Milvus vector retrieval for products, knowledge, and product images. Product image search currently supports JPEG, PNG, GIF, and WebP with a local 64-dimension color-histogram embedding; production semantic image search should use a multimodal embedding model.
- Production hardening already in place for core flows: auth, rate limit, request logging, body limit, message/run idempotency, cart upsert, checkout transaction and stock reservation.

## Backend Quick Start

```bash
cd backend
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/api/v1/health
```

Agent model configuration:

```bash
export DASHSCOPE_API_KEY='your-new-key'
export AI_SMALL_MODEL='qwen3.5-flash'
export AI_LARGE_MODEL='qwen3.6-plus'
export AI_BASE_URL='https://dashscope.aliyuncs.com/compatible-mode/v1'
```

The backend will call DashScope only when `DASHSCOPE_API_KEY` is present; otherwise it keeps a local fallback for development.

## Local Middleware

```bash
docker compose -f deployments/docker-compose.yml up -d
```

Middleware stack:

- MySQL: business data, Agent sessions/runs/traces, prompts.
- Nacos: dynamic config and prompt config.
- Milvus + etcd + MinIO: vector retrieval.
- Redis: reserved middleware for cache/rate-limit extension.

## Quality Evals

```bash
node quality/evals/run_intent_eval.mjs quality/data/eval/intent_cases.jsonl
node quality/evals/run_rag_recall_eval.mjs quality/data/eval/rag_recall_cases.jsonl
node quality/evals/run_agent_e2e.mjs quality/data/eval/agent_e2e_20_scenarios.jsonl
```

Reports are written to `quality/reports`.
