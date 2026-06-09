# AI 小猪向导后端接口协议

本文档定义安卓端“小猪向导”和科大讯飞 TTS 功能需要后端提供的接口、生成逻辑和前后端边界。

## 职责边界

安卓前端负责：

- 渲染可拖动的小猪向导悬浮窗。
- 进入商城、购物车、订单等页面时调用向导建议问题接口。
- 将后端返回的问题展示成小猪气泡。
- 用户点击问题后，跳转到现有 AI 聊天页并发送该问题。
- AI 回复完成后调用 TTS 接口，并播放返回的音频。

后端负责：

- 根据页面和上下文生成小猪向导建议问题。
- 在服务端维护提示词、排序、兜底规则和缓存逻辑。
- 使用服务端保存的科大讯飞密钥调用 TTS。
- 将合成后的音频返回给安卓端。

## 向导建议问题接口

### 接口

```http
POST /api/v1/agent/guide-suggestions
Authorization: Bearer <token>
Content-Type: application/json
```

### 请求体

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

`page` 可选值：

- `products`：商品列表页。
- `cart`：购物车页。
- `orders`：订单页。
- `product_detail`：商品详情页，当前安卓端可以先不接，后端可预留。

`limit` 不传时默认 `3`。考虑移动端气泡空间，后端建议最多返回 3 条。

`context` 为页面上下文。后端应忽略未知字段，方便安卓端后续逐步补充更多上下文。

推荐上下文字段：

| 页面 | 字段 |
| --- | --- |
| `products` | `keyword`、`category_id`、`category_name`、`cart_item_count`、`visible_product_ids` |
| `cart` | `cart_item_count`、`selected_item_count`、`total_amount`、`selected_amount`、`product_ids` |
| `orders` | `order_count`、`pending_payment_count`、`pending_receipt_count`、`pending_review_count`、`latest_order_ids` |
| `product_detail` | `product_id`、`product_name`、`category_id`、`category_name`、`price`、`stock_status` |

### 响应体

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

安卓端只依赖 `question` 字段。`id` 和 `reason` 主要用于埋点、日志、问题排查和后端调试。

### 失败和兜底

后端应优先返回兜底问题，而不是直接失败。大模型调用失败、解析失败或结果不合规时，返回规则生成的问题：

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

只有请求参数错误、未登录或权限问题才建议返回错误。

示例错误：

```json
{
  "code": "bad_page",
  "message": "不支持的向导页面"
}
```

## 建议问题生成逻辑

当前使用“规则候选 + 上下文增强 + 小模型改写排序 + 结果校验 + 规则兜底”的混合方案。Prompt 统一放在数据库 Prompt 管理体系中，key 为 `agent.prompt.guide_suggestions`，可在管理员 Prompt 页面维护。

推荐流程：

1. 根据页面类型生成规则候选问题。
2. 使用页面上下文增强候选问题。
3. 调用小模型，对候选问题做改写和排序。
4. 校验长度、安全性、重复度和页面相关性。
5. 如果模型失败或输出不合规，返回规则兜底问题。

### 规则候选

`products` 商品列表页：

- `{category_name}怎么选？`
- `比较当前商品`
- `高性价比推荐`

如果有 `keyword`：

- `帮我选{keyword}`

`cart` 购物车页：

- `购物车哪些值得买？`
- `看看能省多少`
- `现在适合直接下单吗？`

如果 `cart_item_count > 0`：

- `帮我分析这 {cart_item_count} 件商品`

`orders` 订单页：

- `总结订单状态`
- `哪些订单待办？`
- `待评价怎么写？`

如果 `pending_payment_count > 0`：

- `哪些订单还没支付？`

`product_detail` 商品详情页：

- `这个商品适合我吗？`
- `这个商品有什么优缺点？`
- `同类商品里它值得买吗？`

如果有 `product_name`：

- `{product_name} 值得买吗？`

### 小模型提示词

```text
你是电商 App 的小猪 AI 浮窗向导。请根据当前页面、上下文和候选问题，生成 1-3 个最适合展示给用户点击的问题。

要求：
- 只输出 JSON，不要解释。
- 每个 question 不超过 10 个中文字，越短越好。
- question 要能直接发送给 AI 导购。
- 贴合当前页面和上下文，不要承诺平台没有的数据能力。
- 可以改写候选问题，也可以从候选中挑选排序。
- 不要输出商品 ID、内部字段名、工具名或策略说明。

返回 JSON：
{
  "items": [
    {"id": "短英文id", "question": "十字以内", "reason": "中文，说明为什么适合"}
  ]
}
```

### 结果校验

模型输出返回给前端前，后端需要做校验：

- 保留 1 到 3 条。
- 剔除超过 10 个中文字的问题。
- 剔除空问题。
- 剔除重复或高度相似的问题。
- 剔除引用缺失数据的问题，例如没有优惠券上下文时不要问“有哪些优惠券可用”。
- 确保每个问题都可以直接发送给现有 AI 聊天接口。

### 缓存建议

建议缓存 30 到 120 秒，避免用户在页面内频繁触发模型调用。

推荐缓存键：

```text
user_id + page + keyword + category_id + cart_updated_at + order_updated_at + product_id
```

如果暂时没有更新时间戳，可以使用轻量指纹：

```text
购物车商品 id + 数量 + 选中状态
最近订单 id + 订单状态
当前可见商品 id
```

## 科大讯飞 TTS 接口

科大讯飞密钥必须保存在服务端。不要把 `API Secret` 放到安卓 APK 里，否则反编译后会泄露。

### 后端配置

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

### 接口

```http
POST /api/v1/speech/tts
Authorization: Bearer <token>
Content-Type: application/json
Accept: audio/mpeg
```

### 请求体

```json
{
  "text": "这是一段要朗读的 AI 回复",
  "voice": "xiaoyan"
}
```

`voice` 可选。不传时后端使用 `XUNFEI_TTS_VOICE` 默认配置。

### 成功响应

建议直接返回音频字节：

```http
HTTP/1.1 200 OK
Content-Type: audio/mpeg
Cache-Control: no-store
```

如果第一版接的是 PCM 或 WAV，也可以返回对应类型：

```http
Content-Type: audio/wav
```

安卓端会根据 `Content-Type` 选择播放方式。

### 错误响应

```json
{
  "code": "tts_not_enabled",
  "message": "TTS 服务未启用"
}
```

推荐状态码：

- `400`：文本为空或文本过长。
- `401`：缺少登录态或 token 无效。
- `429`：触发限流。
- `501`：TTS 未启用或未配置。
- `502`：上游讯飞服务不可用或网络异常。

### TTS 服务端流程

1. 校验用户登录态。
2. 清理文本，空文本直接拒绝。
3. 限制文本长度，例如 500 到 1000 个中文字符。
4. 使用服务端密钥生成科大讯飞 WebSocket 鉴权 URL。
5. 发送合成请求。
6. 收集讯飞返回的 base64 音频帧。
7. 解码并拼接音频字节。
8. 将音频字节返回给安卓端。

### 限流建议

建议加这些限制：

- 单用户每分钟 10 到 20 次。
- 单次请求 500 到 1000 字。
- 可选每日额度，用于成本控制。

## 安卓接入约定

安卓端后续按以下方式接入：

1. 渲染 `products`、`cart` 或 `orders` 页面后调用 `/agent/guide-suggestions`。
2. 将返回的 `question` 展示在小猪向导气泡里。
3. 用户点击气泡后，跳转到 AI 聊天页并发送该问题。
4. AI 回复完成后，使用可见回复文本调用 `/speech/tts`。
5. 如果 TTS 返回 `404`、`501`、`502` 或其他错误，安卓端跳过播放，不阻塞聊天流程。
