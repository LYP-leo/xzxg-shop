# API 接口文档 v2

更新时间：2026-05-22

## 版本定位

v2 在 v1 的基础上补齐三端角色、分页、商家后台、管理员后台、质量测评、链路追踪、动态配置和更完整的 Agent 工具化能力。此版本是从“可演示最小闭环”走向“可调试、可评测、可管理”的过渡版本。

当前正式联调请使用 `API接口文档_v3.md`。v2 用于说明 v3 之前的接口演进。

## 通用约定

Base URL：

```text
http://127.0.0.1:8080/api/v1
```

分页参数：

```text
page=1&page_size=10
```

分页响应：

```json
{
  "items": [],
  "page": 1,
  "page_size": 10,
  "total": 0
}
```

角色：

- `user`：普通用户。
- `merchant`：商家。
- `admin`：管理员。

## 认证与账号

### POST /auth/login

权限：公开

用途：账号密码登录。

### POST /auth/register

权限：公开

用途：注册普通用户账号。商家和管理员账号由管理员创建或初始化数据提供。

### GET /auth/me

权限：登录用户。

## 商品与商家

### GET /categories/tree

权限：公开

用途：查询类目树。

### GET /merchants

权限：公开

用途：分页查询商家列表。

### GET /products

权限：公开

用途：分页查询商品列表，支持关键词、类目、商家筛选。

### GET /products/{product_id}

权限：公开

用途：商品详情。

### GET /products/{product_id}/skus

权限：公开

用途：查询商品 SKU。

## 购物车与订单

### GET /cart

权限：用户

用途：查询购物车。

### POST /cart/items

权限：用户

用途：添加商品到购物车。

### POST /orders

权限：用户

用途：创建订单。

### GET /orders

权限：用户

用途：分页查询当前用户订单。

### GET /merchant/orders

权限：商家

用途：分页查询本商家订单。

### PATCH /merchant/orders/{order_id}/status

权限：商家

用途：修改订单发货或完成状态。

## 管理员后台

### GET /admin/accounts

权限：管理员

用途：分页查询账号。

### GET /admin/products

权限：管理员

用途：分页查询全站商品。

### GET /admin/orders

权限：管理员

用途：分页查询全站订单。

### GET /admin/traces

权限：管理员

用途：查询最近 Agent 请求链路。

### GET /admin/traces/{run_id}

权限：管理员

用途：查询单次 Agent run 的完整 trace，包括意图、工具调用、LLM 输入输出、耗时和错误。

## Agent 与工具

### GET /agent/sessions

权限：用户

用途：查询历史会话列表。

### POST /agent/sessions/{session_id}/messages:stream

权限：用户

用途：发送消息并通过 SSE 返回回复。

v2 增加：

- `client_message_id` 幂等键。
- Agent trace。
- 导购和非导购 route。
- 商品搜索、购物车、订单等工具调用。

## 质量测评

### POST /admin/evaluations/runs

权限：管理员

用途：发起测评任务。

测评类型：

- `agent_e2e`
- `intent_eval`
- `rag_eval`

### GET /admin/evaluations/reports

权限：管理员

用途：分页查询测评报告。

### GET /admin/evaluations/reports/{report_id}

权限：管理员

用途：查询测评报告详情。

## 动态配置

### GET /admin/configs

权限：管理员

用途：查询当前动态配置。

说明：

- v2 仍存在 Nacos 与数据库混合管理 Prompt 的阶段性状态。
- 后续 v3 要求 Prompt 统一落库，Nacos 只保留运行配置。

## 风控

### GET /admin/risk/rules

权限：管理员

用途：查询风控规则。

### POST /admin/risk/rules

权限：管理员

用途：创建风控规则。

说明：

- 用户输入命中风控词时应拦截。
- 风险用户应拦截。
- 风险商品和商家不得进入召回结果。
