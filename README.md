# xzxg-shop
Shopping system by piggy and doggy

## Project Layout

```text
backend/      Go API and Agent runtime
frontend/     React client
deployments/  Local middleware and deployment assets
.ai/          Product and technical docs
```

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
