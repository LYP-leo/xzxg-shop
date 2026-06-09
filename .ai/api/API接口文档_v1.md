# API 接口文档 v1

更新时间：2026-05-18

## 版本定位

v1 是项目早期接口基线，目标是先跑通“账号登录 - 商品浏览 - 购物车 - 订单 - 基础 Agent 对话”的最小闭环。此版本尚未完整覆盖质量测评、链路追踪、Prompt 管理、风控、图片检索和生产化配置。

当前正式联调请使用 `API接口文档_v3.md`。v1 仅用于追溯接口演进。

## 通用约定

Base URL：

```text
http://127.0.0.1:8080/api/v1
```

鉴权：

```http
Authorization: Bearer <token>
```

错误响应：

```json
{
  "code": "bad_request",
  "message": "请求不合法"
}
```

## 认证

### POST /auth/login

权限：公开

用途：账号密码登录，返回 token 和账号信息。

请求：

```json
{
  "username": "user",
  "password": "user123456"
}
```

响应：

```json
{
  "token": "tok_xxx",
  "account": {
    "account_id": "acct_xxx",
    "username": "user",
    "display_name": "用户",
    "role": "user"
  }
}
```

### GET /auth/me

权限：登录用户

用途：查询当前登录账号。

## 商品

### GET /categories/tree

权限：公开

用途：查询类目树。

### GET /products

权限：公开

用途：商品列表和关键词搜索。

Query：

- `keyword`：商品关键词，可为空。
- `category_id`：类目 ID，可为空。

响应：

```json
{
  "items": [
    {
      "productId": "p_001",
      "name": "商品名称",
      "brand": "品牌",
      "categoryId": "c_001",
      "imageUrl": "/api/v1/assets/xxx.jpg",
      "price": "199.00",
      "stockStatus": "in_stock"
    }
  ]
}
```

### GET /products/{product_id}

权限：公开

用途：查询商品详情。

## 购物车

### GET /cart

权限：用户

用途：查询当前用户购物车。

### POST /cart/items

权限：用户

用途：添加商品 SKU 到购物车。

请求：

```json
{
  "product_id": "p_001",
  "sku_id": "sku_001",
  "quantity": 1
}
```

### PATCH /cart/items/{item_id}

权限：用户

用途：修改购物车商品数量。

### DELETE /cart/items/{item_id}

权限：用户

用途：删除购物车商品。

## 订单

### GET /orders

权限：用户

用途：查询当前用户订单列表。

### POST /orders

权限：用户

用途：从购物车创建订单。

## Agent

### POST /agent/sessions

权限：用户

用途：创建导购会话。

### POST /agent/sessions/{session_id}/messages:stream

权限：用户

用途：发送用户消息并通过 SSE 返回 Agent 回复。

说明：

- v1 阶段主要返回纯文本流。
- v1 尚未要求结构化 `content_delta`、商品卡片插入位置、trace 事件和幂等键。
- 当前输出协议请看 `Agent输出协议_v1.md`。

