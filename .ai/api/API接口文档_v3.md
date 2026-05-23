# API 接口文档 v3

## 说明

本文档记录后端需求 v3 开始补齐的接口。鉴权仍使用 `Authorization: Bearer <token>`。

## 会话模块

### GET /api/v1/agent/sessions

权限：用户

用途：查询当前用户的历史会话列表。

响应：

```json
{
  "items": [
    {
      "session_id": "sess_xxx",
      "account_id": "acct_user_001",
      "title": "跑鞋推荐",
      "summary": "用户咨询：跑鞋推荐",
      "message_count": 3,
      "last_message_at": "2026-05-23T12:00:00+08:00",
      "created_at": "2026-05-23T11:58:00+08:00",
      "updated_at": "2026-05-23T12:00:00+08:00"
    }
  ]
}
```

### PATCH /api/v1/agent/sessions/{session_id}

权限：用户

用途：更新会话标题或摘要。前端可用于重命名；Agent 对话压缩后也可写入摘要。

请求：

```json
{
  "title": "露营装备清单",
  "summary": "用户在选露营装备，预算偏入门。"
}
```

响应：更新后的 session。

## 订单模块

### POST /api/v1/orders:checkout

权限：用户

用途：从购物车选中项创建待支付订单。当前版本会按商家拆单，并创建虚拟支付单。

响应：

```json
{
  "items": [
    {
      "order_id": "ord_xxx",
      "order_no": "NO20260523120000...",
      "status": "pending_payment",
      "total_amount": "299.00",
      "discount_amount": "0.00",
      "pay_amount": "299.00",
      "payment_deadline_at": "2026-05-23T12:30:00+08:00",
      "items": []
    }
  ]
}
```

### GET /api/v1/orders/{order_id}

权限：用户

用途：查询当前用户订单详情。查询时会懒执行待支付订单超时关闭。

### POST /api/v1/orders/{order_id}:pay

权限：用户

用途：虚拟支付待支付订单。

请求：

```json
{
  "method": "mock_balance"
}
```

响应：

```json
{
  "order": {
    "order_id": "ord_xxx",
    "status": "pending_ship",
    "paid_at": "2026-05-23T12:03:00+08:00"
  },
  "payment": {
    "payment_id": "pay_xxx",
    "order_id": "ord_xxx",
    "status": "success",
    "method": "mock_balance",
    "transaction_no": "txn_xxx"
  }
}
```

错误：

- `409 pay_failed`：订单不可支付，可能已支付、取消或超时关闭。

### POST /api/v1/orders/{order_id}:cancel

权限：用户

用途：取消待支付订单。

请求：

```json
{
  "reason": "暂时不买了"
}
```

响应：更新后的订单，状态为 `canceled`。

错误：

- `409 cancel_failed`：只有待支付订单可取消。

### POST /api/v1/orders/{order_id}:confirm-receipt

权限：用户

用途：确认收货。只有 `shipped` 订单可确认。

响应：更新后的订单，状态为 `completed`。

错误：

- `409 confirm_failed`：订单不可确认收货。

## 订单状态

当前订单状态：

- `pending_payment`：待支付。
- `pending_ship`：已支付，待商家发货。
- `shipped`：已发货。
- `completed`：已完成。
- `closed_timeout`：支付超时自动关闭。
- `canceled`：用户取消。
- `refund_requested`：退款/售后申请中，预留。
- `refunded`：已退款，预留。

## 后续待补

- Agent tools 接口/trace 文档。

## 优惠模块

### GET /api/v1/promotions

权限：公开

用途：查询当前平台和商家促销规则。

### GET /api/v1/coupons/available

权限：用户

用途：查询当前可领取优惠券。

### GET /api/v1/coupons/mine

权限：用户

用途：查询当前用户已领取优惠券。

### POST /api/v1/coupons/{coupon_id}:claim

权限：用户

用途：领取优惠券。

错误：

- `409 claim_failed`：已领取、已过期或已领完。

### GET /api/v1/cart/discount-preview

权限：用户

用途：按购物车选中项计算当前可用促销和优惠券折扣。

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

### GET /api/v1/merchant/promotions

权限：商家

用途：查询当前商家的促销规则。

### POST /api/v1/merchant/promotions

权限：商家

用途：创建商家促销。后端会强制 `scope=merchant`，并绑定当前商家。

请求：

```json
{
  "name": "店铺满 500 减 50",
  "type": "full_reduction",
  "threshold_amount": "500.00",
  "discount_amount": "50.00",
  "stackable": true,
  "start_at": "2026-05-23T00:00:00+08:00",
  "end_at": "2026-06-23T00:00:00+08:00",
  "status": "active"
}
```

### PATCH /api/v1/merchant/promotions/{promotion_id}

权限：商家

用途：更新当前商家的促销状态。

请求：

```json
{"status": "inactive"}
```

### GET /api/v1/admin/promotions

权限：管理员

用途：查询全部促销。

### POST /api/v1/admin/promotions

权限：管理员

用途：创建平台或指定商家促销。

### PATCH /api/v1/admin/promotions/{promotion_id}

权限：管理员

用途：更新任意促销状态。

## 评价模块

### GET /api/v1/products/{product_id}/reviews

权限：公开

用途：查询商品可见评价。

### POST /api/v1/orders/{order_id}/items/{order_item_id}:review

权限：用户

用途：对已完成订单项创建评价。同一订单项只能评价一次。

请求：

```json
{
  "rating": 5,
  "content": "做工不错，发货也快。",
  "tags": ["做工好", "物流快"]
}
```

错误：

- `409 review_failed`：订单未完成、订单项不属于当前用户或重复评价。

### GET /api/v1/merchant/reviews

权限：商家

用途：查询当前商家商品评价。

### POST /api/v1/merchant/reviews/{review_id}:reply

权限：商家

用途：回复当前商家商品评价。

请求：

```json
{"reply": "感谢反馈，我们会继续优化。"}
```

### GET /api/v1/admin/reviews

权限：管理员

用途：查询全部评价。

### PATCH /api/v1/admin/reviews/{review_id}

权限：管理员

用途：更新评价状态。

请求：

```json
{"status": "hidden"}
```

状态：

- `visible`
- `hidden`
- `deleted`
