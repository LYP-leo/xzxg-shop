# API 接口文档 v3

更新时间：2026-05-24

## 通用约定

Base URL：

```text
http://127.0.0.1:8080/api/v1
```

鉴权：

```http
Authorization: Bearer <token>
```

分页接口统一参数：

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

错误响应：

```json
{
  "code": "bad_request",
  "message": "请求 JSON 不合法",
  "request_id": "req_xxx"
}
```

角色：

- 公开：不需要登录。
- 用户：`role=user`。
- 商家：`role=merchant`。
- 管理员：`role=admin`。

## 健康检查

### GET /health

权限：公开

响应：

```json
{"status": "ok"}
```

## 认证

### POST /auth/login

权限：公开

请求：

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

响应：

```json
{
  "token": "tok_xxx",
  "account": {
    "account_id": "acct_admin_001",
    "username": "admin",
    "display_name": "平台管理员",
    "role": "admin",
    "merchant_id": "",
    "created_at": "2026-05-24T10:00:00+08:00"
  }
}
```

### POST /auth/register

权限：公开

用途：注册普通用户账号。公开注册只创建 `role=user`，不允许通过该接口创建商家或管理员账号。

请求：

```json
{
  "username": "new_user",
  "password": "newuser123456",
  "display_name": "新用户"
}
```

字段约束：

- `username`：3-32 个字符，只允许字母、数字、下划线和短横线。
- `password`：8-64 个字符。
- `display_name`：可选，最长 32 个字符；为空时默认使用 `username`。

响应：`201 Created`

```json
{
  "token": "tok_xxx",
  "account": {
    "account_id": "acct_xxx",
    "username": "new_user",
    "display_name": "新用户",
    "role": "user",
    "merchant_id": "",
    "status": "active",
    "created_at": "2026-05-24T10:00:00+08:00"
  }
}
```

错误：

- `400 invalid_register_input`：账号、密码或昵称格式不合法。
- `409 username_exists`：账号已存在。

### GET /auth/me

权限：登录用户

响应：当前账号对象。

## 商品与类目

### GET /categories/tree

权限：公开

用途：查询类目树。

响应：

```json
{
  "items": [
    {
      "categoryId": "c_dataset_digital",
      "parentId": "",
      "name": "数码电子",
      "children": []
    }
  ]
}
```

### GET /merchants

权限：公开

用途：查询商家列表。

Query：`page`、`page_size`

### GET /products

权限：公开

用途：商品列表和搜索。

Query：

- `keyword`：商品关键词，可为空。
- `category_id`：类目 ID，可为空。
- `page`：页码，默认 1。
- `page_size`：每页数量，默认 10，最大 100。

响应：

```json
{
  "items": [
    {
      "productId": "p_digital_001",
      "skuId": "",
      "merchantId": "m_001",
      "merchantName": "小猪数码",
      "name": "商品名称",
      "brand": "品牌",
      "categoryId": "c_dataset_digital",
      "imageUrl": "/api/v1/assets/ecommerce_agent_dataset/...",
      "price": "2999.00",
      "marketPrice": "3299.00",
      "stockStatus": "in_stock",
      "tags": ["热卖"],
      "sellingPoints": ["卖点"],
      "recommendReason": "推荐理由",
      "riskNotes": []
    }
  ]
}
```

### GET /products/{product_id}

权限：公开

用途：查询商品详情。

响应字段在商品卡片基础上增加：

- `imageUrls`
- `stockQuantity`
- `attributes`
- `suitableFor`
- `notSuitableFor`
- `description`

### GET /products/{product_id}/skus

权限：公开

用途：查询商品 SKU。

Query：`page`、`page_size`

### GET /products/{product_id}/reviews

权限：公开

用途：查询商品可见评价。

Query：`page`、`page_size`

## 购物车

### GET /cart

权限：用户

用途：查询当前用户购物车。

响应：

```json
{
  "items": [
    {
      "cartItemId": "cart_xxx",
      "productId": "p_digital_001",
      "skuId": "sku_xxx",
      "name": "商品名称",
      "imageUrl": "...",
      "price": "2999.00",
      "quantity": 1,
      "selected": true,
      "stockStatus": "in_stock",
      "merchantId": "m_001",
      "merchantName": "小猪数码"
    }
  ],
  "summary": {
    "selectedCount": 1,
    "totalAmount": "2999.00",
    "discountAmount": "0.00",
    "payAmount": "2999.00"
  }
}
```

### POST /cart/items

权限：用户

用途：加入购物车。

请求：

```json
{
  "product_id": "p_digital_001",
  "sku_id": "sku_xxx",
  "quantity": 1
}
```

响应：更新后的购物车。

### PATCH /cart/items/{cart_item_id}

权限：用户

用途：修改购物车数量或选中状态。

请求：

```json
{
  "quantity": 2,
  "selected": true
}
```

响应：更新后的购物车。

### DELETE /cart/items/{cart_item_id}

权限：用户

用途：删除购物车项。

响应：更新后的购物车。

### GET /cart/discount-preview

权限：用户

用途：按购物车选中项计算促销和优惠券折扣。

响应：

```json
{
  "total_amount": "2999.00",
  "discount_amount": "130.00",
  "pay_amount": "2869.00",
  "lines": [
    {
      "type": "promotion",
      "id": "promo_platform_001",
      "name": "平台满 300 减 30",
      "amount": "30.00"
    }
  ]
}
```

## 订单

### GET /orders

权限：用户

用途：查询当前用户订单列表。

Query：`page`、`page_size`

### POST /orders:checkout

权限：用户

用途：从购物车选中项创建待支付订单。当前版本按商家拆单。

响应：

```json
{
  "items": [
    {
      "order_id": "ord_xxx",
      "order_no": "NO20260524120000...",
      "status": "pending_payment",
      "total_amount": "299.00",
      "discount_amount": "0.00",
      "pay_amount": "299.00",
      "payment_deadline_at": "2026-05-24T12:30:00+08:00",
      "items": []
    }
  ]
}
```

### GET /orders/{order_id}

权限：用户

用途：查询当前用户订单详情。查询时会懒执行待支付订单超时关闭。

### POST /orders/{order_id}:pay

权限：用户

用途：虚拟支付待支付订单。

请求：

```json
{"method": "mock_balance"}
```

响应：

```json
{
  "order": {"order_id": "ord_xxx", "status": "pending_ship"},
  "payment": {"payment_id": "pay_xxx", "status": "success"}
}
```

### POST /orders/{order_id}:cancel

权限：用户

用途：取消待支付订单。

请求：

```json
{"reason": "暂时不买了"}
```

### POST /orders/{order_id}:confirm-receipt

权限：用户

用途：确认收货。只有 `shipped` 订单可确认。

### POST /orders/{order_id}/items/{order_item_id}:review

权限：用户

用途：对已完成订单项评价。每个订单项只能评价一次。

请求：

```json
{
  "rating": 5,
  "content": "发货快，商品符合预期",
  "tags": ["物流快", "质量好"]
}
```

订单状态：

- `pending_payment`：待支付。
- `pending_ship`：已支付，待商家发货。
- `shipped`：已发货。
- `completed`：已完成。
- `closed_timeout`：支付超时自动关闭。
- `canceled`：用户取消。
- `refund_requested`：退款/售后申请中，预留。
- `refunded`：已退款，预留。

## 优惠券与促销

### GET /promotions

权限：公开

用途：查询平台和商家促销规则。

Query：`page`、`page_size`

### GET /coupons/available

权限：登录用户

用途：查询当前可领取优惠券。未登录时按空用户查询。

Query：`page`、`page_size`

### GET /coupons/mine

权限：用户

用途：查询当前用户已领取优惠券。

Query：`page`、`page_size`

### POST /coupons/{coupon_id}:claim

权限：用户

用途：领取优惠券。

错误：

- `409 claim_failed`：已领取、已过期或已领完。

## Agent 会话

Agent SSE 输出协议详见：

[Agent输出协议_v1.md](/Users/keii/xzxg-shop/.ai/api/Agent输出协议_v1.md)

前端挂品解析约定：

- 前端优先消费 SSE `block_delta` 中的 `product_card`、`product_ids` 等结构化块。
- 如果主回答文本里出现 `<item>...</item>`，标签内容必须是商品 ID，例如 `<item>p_digital_001</item>`。
- `<item>` 内禁止放商品名、品牌名或自然语言推荐语。
- 前端可用该商品 ID 再调用 `GET /products/{product_id}` 获取详情或渲染商品卡。

### GET /agent/sessions

权限：用户

用途：查询当前用户历史会话列表。

Query：`page`、`page_size`

### POST /agent/sessions

权限：用户

用途：创建 Agent 会话。

请求：

```json
{"title": "AI 导购"}
```

### GET /agent/sessions/{session_id}

权限：用户

用途：查询会话详情，包含消息和关联 run。

### PATCH /agent/sessions/{session_id}

权限：用户

用途：更新会话标题或摘要。

请求：

```json
{
  "title": "露营装备清单",
  "summary": "用户在选露营装备，预算偏入门。"
}
```

### POST /agent/sessions/{session_id}/messages:stream

权限：用户

用途：发送用户消息并通过 SSE 流式返回 Agent 输出。

请求头：

```http
Accept: text/event-stream
Content-Type: application/json
Authorization: Bearer <token>
```

请求：

```json
{
  "client_message_id": "cli_xxx",
  "content": "帮我推荐苹果电脑",
  "attachments": []
}
```

### POST /agent/runs/{run_id}:cancel

权限：用户

用途：取消当前用户自己的 Agent run。

### GET /agent/runs/{run_id}/trace

权限：用户

用途：查询当前用户自己的 Agent trace。

## 商家端

### POST /merchant/products

权限：商家

用途：创建当前商家的商品。

请求：

```json
{
  "name": "商品名称",
  "brand": "品牌",
  "category_id": "c_dataset_digital",
  "image_url": "/api/v1/assets/ecommerce_agent_dataset/...",
  "price": "2999.00",
  "market_price": "3299.00",
  "stock_quantity": 100,
  "stock_status": "in_stock",
  "tags": ["新品"],
  "selling_points": ["核心卖点"],
  "recommend_reason": "推荐理由",
  "risk_notes": [],
  "description": "商品详情"
}
```

说明：后端会强制 `merchant_id` 为当前商家。

### PATCH /merchant/products/{product_id}

权限：商家

用途：更新当前商家的商品。

### DELETE /merchant/products/{product_id}

权限：商家

用途：软删除商品，状态置为 `deleted`。

### GET /merchant/documents

权限：商家

用途：查询当前商家上传的知识资料。

Query：`page`、`page_size`

### POST /merchant/documents

权限：商家

用途：上传商家知识资料，后端会切分为知识 chunk 供 RAG 检索。

请求：

```json
{
  "title": "售后政策",
  "doc_type": "after_sales",
  "content": "七天无理由退货规则..."
}
```

### GET /merchant/orders

权限：商家

用途：查询当前商家的订单。

Query：`page`、`page_size`

### PATCH /merchant/orders/{order_id}

权限：商家

用途：更新当前商家的订单状态。

请求：

```json
{"status": "shipped"}
```

### GET /merchant/promotions

权限：商家

用途：查询当前商家的促销规则。

Query：`page`、`page_size`

### POST /merchant/promotions

权限：商家

用途：创建商家促销。后端会强制 `scope=merchant` 并绑定当前商家。

请求：

```json
{
  "name": "店铺满 500 减 50",
  "type": "full_reduction",
  "threshold_amount": "500.00",
  "discount_amount": "50.00",
  "stackable": true,
  "start_at": "2026-05-24T00:00:00+08:00",
  "end_at": "2026-06-24T00:00:00+08:00",
  "status": "active"
}
```

### PATCH /merchant/promotions/{promotion_id}

权限：商家

用途：更新商家促销状态。

请求：

```json
{"status": "inactive"}
```

### GET /merchant/reviews

权限：商家

用途：查询当前商家的商品评价。

Query：`page`、`page_size`

### POST /merchant/reviews/{review_id}:reply

权限：商家

用途：回复当前商家的评价。

请求：

```json
{"reply": "感谢反馈，我们会继续优化。"}
```

## 管理员端

以下接口均需管理员权限。

### GET /admin/accounts

用途：分页查询账号。

Query：`page`、`page_size`

### PATCH /admin/accounts/{account_id}

用途：启用或禁用账号。

请求：

```json
{"status": "inactive"}
```

### GET /admin/products

用途：分页查询所有商品。

### PATCH /admin/products/{product_id}

用途：更新商品状态。

请求：

```json
{"status": "inactive"}
```

状态允许：`active`、`inactive`、`deleted`。

### GET /admin/orders

用途：分页查询所有订单。

### GET /admin/documents

用途：分页查询所有知识资料。

### GET /admin/promotions

用途：分页查询促销。

### POST /admin/promotions

用途：创建平台或指定商家促销。

### PATCH /admin/promotions/{promotion_id}

用途：更新促销状态。

### GET /admin/reviews

用途：分页查询所有评价。

### PATCH /admin/reviews/{review_id}

用途：更新评价状态。

请求：

```json
{"status": "hidden"}
```

状态允许：`visible`、`hidden`、`deleted`。

## 管理员配置与 Prompt

### GET /admin/configs

权限：管理员

用途：分页查询非 Prompt 动态配置。`agent.prompt.*` 已移到 Prompt 管理接口。

### PATCH /admin/configs/{config_key}

权限：管理员

用途：更新动态配置。后端按 key 自动写入对应 Nacos dataId：

- `retrieval.*` -> `xzxg-shop-rag-config.json`
- `vector.*`、`milvus.*`、`embedding.*` -> `xzxg-shop-infra-config.json`
- `agent.prompt.*` -> `xzxg-shop-agent-prompts.json`
- 其他 -> `xzxg-shop-app-config.json`

请求：

```json
{
  "value": "80",
  "value_type": "int",
  "description": "向量召回候选数",
  "is_secret": false
}
```

### GET /admin/prompts

权限：管理员

用途：分页查询 Prompt 最新版本和最近发布记录。

响应：

```json
{
  "items": [
    {
      "prompt_id": "prm_xxx",
      "prompt_key": "agent.prompt.answer_base",
      "title": "主 Agent 基础 Prompt",
      "content": "...",
      "status": "active",
      "version": 1,
      "description": "主导购 Agent 系统提示词"
    }
  ],
  "page": 1,
  "page_size": 10,
  "total": 14,
  "publish_records": []
}
```

### PATCH /admin/prompts/{prompt_key}

权限：管理员

用途：保存 Prompt 草稿。不会影响运行时，必须发布后才写入 Nacos。

请求：

```json
{
  "title": "主 Agent 基础 Prompt",
  "description": "主导购 Agent 系统提示词",
  "content": "..."
}
```

### POST /admin/prompts/{prompt_key}/publish

权限：管理员

用途：发布该 Prompt 最新版本。后端会将旧 active 归档、新版本置为 active，并同步到 `xzxg-shop-agent-prompts.json`。

## 管理员调试与测评

### GET /admin/vector/status

权限：管理员

用途：查看 Milvus 向量索引状态。

### GET /admin/agent/runs

权限：管理员

用途：分页查询最近 Agent run。

### GET /admin/agent/runs/{run_id}/trace

权限：管理员

用途：查询指定 run 的全链路 trace，包括意图识别、工具调用、知识片段、模型原始输出等。

### GET /admin/evals

权限：管理员

用途：查询质量测评数据集、工具套件和最近报告。

### GET /admin/evals/reports/{run_id}/{report_name}

权限：管理员

用途：查询测评报告详情。

示例：

```text
GET /api/v1/admin/evals/reports/rag_recall_eval_20260524T023738/rag_recall_eval
```

### POST /eval/intent

权限：管理员

用途：单条意图识别调试。

请求：

```json
{"query": "帮我推荐一台华为电脑"}
```

### POST /eval/rag

权限：管理员

用途：单条 RAG 召回调试。

请求：

```json
{
  "query": "华为 电脑",
  "top_k": 5
}
```

响应：

```json
{
  "keyword": "华为 电脑",
  "items": [
    {
      "chunkId": "k_xxx",
      "title": "商品资料",
      "snippet": "资料片段",
      "source": "product:p_digital_001"
    }
  ]
}
```

## 静态资源

### GET /assets/ecommerce_agent_dataset/{path}

权限：公开

用途：访问导入数据集中的商品图片等静态资源。
