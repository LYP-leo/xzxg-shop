# AI Guide Backend Protocol

This document defines the backend contracts needed by the Android pig guide and TTS features.

## Scope

The Android frontend owns:

- Rendering the draggable pig guide floating widget.
- Calling the guide suggestion API when entering commerce pages.
- Showing returned questions in guide bubbles.
- Sending a selected question to the existing AI chat page.
- Calling the TTS API after an assistant reply is complete and playing the returned audio.

The backend owns:

- Generating page-aware guide questions.
- Keeping model prompts, ranking, fallback logic, and cache behavior server-side.
- Calling Xunfei TTS with server-side credentials.
- Returning synthesized audio to the Android client.

## Guide Suggestions API

### Endpoint

```http
POST /api/v1/agent/guide-suggestions
Authorization: Bearer <token>
Content-Type: application/json
```

### Request

```json
{
  "page": "products",
  "context": {
    "keyword": "手机",
    "category_id": "digital",
    "category_name": "数码电子",
    "cart_item_count": 3,
    "visible_product_ids": ["p1", "p2", "p3"]
  },
  "limit": 3
}
```

`page` values:

- `products`
- `cart`
- `orders`
- `product_detail`

`limit` should default to `3` when omitted. The backend may cap it to `3` for mobile display.

`context` is page-specific. Unknown fields should be ignored so the Android client can add fields later.

Recommended context fields:

| Page | Fields |
| --- | --- |
| `products` | `keyword`, `category_id`, `category_name`, `cart_item_count`, `visible_product_ids` |
| `cart` | `cart_item_count`, `selected_item_count`, `total_amount`, `selected_amount`, `product_ids` |
| `orders` | `order_count`, `pending_payment_count`, `pending_receipt_count`, `pending_review_count`, `latest_order_ids` |
| `product_detail` | `product_id`, `product_name`, `category_id`, `category_name`, `price`, `stock_status` |

### Response

```json
{
  "items": [
    {
      "id": "budget_recommend",
      "question": "预算 3000 内有什么手机推荐？",
      "reason": "当前在手机商品列表页，可引导用户按预算筛选"
    },
    {
      "id": "compare_visible",
      "question": "帮我比较当前这些商品",
      "reason": "用户正在浏览商品列表"
    }
  ]
}
```

The Android client only requires `question`. `id` and `reason` are for analytics, logging, and debugging.

### Failure Behavior

The backend should prefer fallback suggestions over hard failures. If model generation fails, return rule-based results:

```json
{
  "items": [
    {
      "id": "fallback_cart_check",
      "question": "帮我分析购物车哪些值得买",
      "reason": "fallback"
    }
  ]
}
```

Return an error only when the request is invalid or the user is unauthorized.

Suggested errors:

```json
{
  "code": "invalid_page",
  "message": "Unsupported guide page"
}
```

## Guide Suggestion Generation Logic

Use a hybrid strategy:

1. Build rule-based candidate questions.
2. Enrich candidates with page context.
3. Optionally ask the LLM to rewrite and rank candidates.
4. Validate length, safety, and page fit.
5. Return fallback questions if the LLM fails or produces invalid output.

### Rule-Based Candidates

`products`:

- `帮我按预算推荐几款{category_name}`
- `帮我比较当前这些商品`
- `有什么高性价比商品推荐？`

If `keyword` exists:

- `帮我找和“{keyword}”相关的好物`

`cart`:

- `帮我分析购物车哪些值得买`
- `购物车里哪些可以删减？`
- `现在适合直接下单吗？`

If `cart_item_count > 0`:

- `帮我分析这 {cart_item_count} 件商品`

`orders`:

- `帮我总结订单状态`
- `哪些订单需要尽快处理？`
- `待评价订单怎么写评价？`

If `pending_payment_count > 0`:

- `哪些订单还没支付？`

`product_detail`:

- `帮我分析这个商品适合我吗`
- `这个商品有什么优缺点？`
- `同类商品里它值得买吗？`

If `product_name` exists:

- `{product_name} 值得买吗？`

### Optional LLM Prompt

```text
你是电商 App 的小猪 AI 向导。请根据用户当前页面和上下文，生成 1-3 个用户可能想问 AI 导购的问题。

要求：
- 问题必须短，适合显示在移动端气泡里。
- 每个问题不超过 28 个中文字符。
- 问题要能直接发送给 AI 导购。
- 不要生成解释文字。
- 不要承诺平台没有的数据能力。
- 优先贴合当前页面。

页面：{{page}}
上下文：{{context_json}}
候选问题：{{rule_candidates_json}}

返回 JSON：
{
  "items": [
    { "id": "...", "question": "...", "reason": "..." }
  ]
}
```

### Validation

Before returning model output:

- Keep 1 to 3 items.
- Drop questions longer than 28 Chinese characters when possible.
- Drop empty questions.
- Drop duplicated or near-duplicated questions.
- Drop questions that reference missing data, such as coupons when no coupon context exists.
- Ensure each question can be sent directly to the existing AI chat endpoint.

### Cache

Cache suggestions for 30 to 120 seconds.

Recommended cache key:

```text
user_id + page + keyword + category_id + cart_updated_at + order_updated_at + product_id
```

If exact update timestamps are unavailable, use lightweight fingerprints:

```text
cart item ids + quantities + selected states
latest order ids + statuses
visible product ids
```

## Xunfei TTS API

Xunfei credentials must stay on the server. Do not put `API Secret` in the Android APK.

### Backend Configuration

```env
XUNFEI_TTS_ENABLED=true
XUNFEI_TTS_APP_ID=xxx
XUNFEI_TTS_API_KEY=xxx
XUNFEI_TTS_API_SECRET=xxx
XUNFEI_TTS_VOICE=xiaoyan
XUNFEI_TTS_SPEED=50
XUNFEI_TTS_VOLUME=50
XUNFEI_TTS_PITCH=50
```

### Endpoint

```http
POST /api/v1/speech/tts
Authorization: Bearer <token>
Content-Type: application/json
Accept: audio/mpeg
```

### Request

```json
{
  "text": "这是一段要朗读的 AI 回复",
  "voice": "xiaoyan"
}
```

`voice` is optional. The backend should use `XUNFEI_TTS_VOICE` when it is omitted.

### Success Response

Return audio bytes directly:

```http
HTTP/1.1 200 OK
Content-Type: audio/mpeg
Cache-Control: no-store
```

If the Xunfei response format is PCM or WAV in the first implementation, return the matching content type:

```http
Content-Type: audio/wav
```

The Android frontend will choose playback logic by response content type.

### Error Response

```json
{
  "code": "tts_unavailable",
  "message": "TTS 服务未启用"
}
```

Recommended status codes:

- `400`: empty text or text too long.
- `401`: missing or invalid token.
- `429`: rate limit exceeded.
- `503`: TTS disabled, unconfigured, or upstream unavailable.

### TTS Server Logic

1. Authenticate the user.
2. Trim text and reject empty input.
3. Limit text length, for example 500 to 1000 Chinese characters.
4. Build the Xunfei WebSocket authenticated URL using server-side credentials.
5. Send the synthesis request.
6. Collect base64 audio frames from Xunfei.
7. Decode and concatenate audio bytes.
8. Return audio bytes to Android.

### Rate Limits

Recommended limits:

- Per user: 10 to 20 requests per minute.
- Per request: 500 to 1000 characters.
- Optional daily quota for cost control.

## Android Integration Contract

The Android client will:

1. Call `/agent/guide-suggestions` after rendering `products`, `cart`, or `orders`.
2. Render returned `question` values in pig guide bubbles.
3. On bubble click, navigate to the AI chat page and send the question.
4. After an assistant response completes, call `/speech/tts` with the visible assistant text.
5. If TTS returns `404`, `503`, or another failure, skip playback without blocking chat.

