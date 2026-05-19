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

## Local Middleware

```bash
docker compose -f deployments/docker-compose.yml up -d
```
