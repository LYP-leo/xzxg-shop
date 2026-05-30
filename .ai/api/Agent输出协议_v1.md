# Agent 输出协议 v1

## 目标

Agent 与前端之间只通过 SSE 事件和结构化 block 通信。前端不解析自然语言中的标签、不从文本里提取商品/订单/优惠信息。

核心原则：

1. 流式正文走 `text_delta`。
2. 可交互、可点击、可展示为卡片的数据走 `block_delta`。
3. 推荐追问走 `followups`。
4. 本轮生命周期由 `message_start` 和 `message_end` 标记。
5. 错误统一走 `error`。

## 请求

### POST /api/v1/agent/sessions/{session_id}/messages:stream

请求头：

```http
Accept: text/event-stream
Content-Type: application/json
Authorization: Bearer <token>
```

请求体：

```json
{
  "client_message_id": "cli_xxx",
  "content": "帮我推荐一双通勤跑鞋",
  "attachments": [
    {
      "attachment_id": "att_xxx",
      "type": "image",
      "url": "https://..."
    }
  ]
}
```

幂等要求：

- `client_message_id` 是客户端生成的消息幂等键，同一账号、同一会话内必须唯一。
- 客户端超时重试、断网重发、页面恢复重放时，必须复用同一个 `client_message_id`。
- 后端按 `(account_id, session_id, client_message_id)` 做唯一约束；重复提交不会重新创建消息，也不会再次执行 Agent。
- 如果重复提交命中已有 run，SSE 会返回已有 `run_id` 和 `duplicate_message` 事件，客户端应停止本次发送态，并通过会话详情或 run trace 刷新已有结果。
- 新建消息时如果客户端未传 `client_message_id`，后端会生成服务端幂等键；但这种请求无法跨网络重试去重，正式客户端必须传。

## SSE 包格式

后端每次发送：

```text
data: {"type":"text_delta","run_id":"run_xxx","delta":"你好"}

```

前端解析规则：

1. 按 `\n\n` 切分 SSE chunk。
2. 找 `data:` 行。
3. `JSON.parse(data)` 得到 `AgentSseEvent`。
4. 按 `type` 分发。

当前前端实现位置：

- `frontend/src/api/agent.ts`
- `frontend/src/pages/AgentSessionPage.tsx`
- `frontend/src/types/agent.ts`

## 事件类型

### message_start

用途：标记本轮 run 已创建，前端把乐观消息绑定真实 `run_id/user_message_id`。

```json
{
  "type": "message_start",
  "run_id": "run_xxx",
  "session_id": "sess_xxx",
  "user_message_id": "msg_xxx",
  "trace_id": "trace_xxx"
}
```

前端处理：

- 设置当前 active run。
- 将最后一个 optimistic turn 的 `runId` 替换为后端返回值。

### status

用途：展示短暂处理状态。

```json
{
  "type": "status",
  "run_id": "run_xxx",
  "stage": "intent",
  "text": "正在理解你的需求"
}
```

建议 `stage` 枚举：

- `intent`
- `query_rewrite`
- `retrieval`
- `tool`
- `answer`
- `followup`
- `done`

前端处理：

- 只展示 `text`。
- 不把 `status.text` 拼进最终回答。

### text_delta

用途：流式输出主回答正文。

```json
{
  "type": "text_delta",
  "run_id": "run_xxx",
  "delta": "这双鞋更适合通勤，"
}
```

前端处理：

- 找到 `run_id` 对应 turn。
- `turn.text += delta`。
- 不解析 markdown 外的隐藏结构。

### block_delta

用途：输出结构化展示块。

```json
{
  "type": "block_delta",
  "run_id": "run_xxx",
  "block": {
    "type": "product_card",
    "product": {}
  }
}
```

前端处理：

- 追加到 `turn.blocks`。
- 按 `block.type` 渲染。
- 未识别的 block 不能让页面崩溃，应展示通用 warning 或忽略并记录日志。

### followups

用途：输出 2-3 个推荐追问按钮。

```json
{
  "type": "followups",
  "run_id": "run_xxx",
  "questions": ["500以内跑鞋怎么选", "通勤跑鞋要不要防水"]
}
```

前端处理：

- 覆盖当前 turn 的 `followups`。
- 用户点击后作为新用户消息发送，不拼接旧消息。

### message_end

用途：标记本轮结束。

```json
{
  "type": "message_end",
  "run_id": "run_xxx"
}
```

前端处理：

- 将 turn 状态改为 `completed`。
- 清空 active run。

### error

用途：标记本轮失败或被取消。

```json
{
  "type": "error",
  "run_id": "run_xxx",
  "code": "canceled",
  "message": "已停止生成"
}
```

前端处理：

- 若有 `run_id`，更新对应 turn。
- 若没有 `run_id`，更新当前最后一个 turn。
- 展示 `message`，不要展示堆栈。

## Block 类型

### markdown

用于非流式补充文本。不建议主回答大量使用，主回答优先走 `text_delta`。

```json
{
  "type": "markdown",
  "content": "补充说明"
}
```

### product_card

用于单个商品卡。

> 兼容说明：ReAct 工具化链路上线后，Runtime 不再主动追加完整 `product_card`。主 Agent 默认输出 `product_refs`，前端拿到商品 ID 后自行调用商品详情接口渲染卡片。`product_card` 保留给旧链路和兼容层。

```json
{
  "type": "product_card",
  "product": {
    "productId": "p_001",
    "skuId": "sku_001",
    "merchantId": "m_001",
    "merchantName": "小猪数码旗舰店",
    "name": "X Phone 12",
    "brand": "X",
    "categoryId": "c_phone",
    "imageUrl": "https://...",
    "price": "2999.00",
    "marketPrice": "3299.00",
    "stockStatus": "in_stock",
    "tags": ["拍照", "预算内"],
    "sellingPoints": ["高速对焦"],
    "recommendReason": "抓拍和对焦能力适合日常拍照。",
    "riskNotes": []
  }
}
```

前端动作：

- 点击卡片：打开商品详情。
- 点击加购：调用购物车接口。

### product_refs

用于声明本轮回答引用了哪些商品。前端需要展示商品卡时，按 `product_ids` 调用商品详情接口或批量详情接口。

```json
{
  "type": "product_refs",
  "product_ids": ["p_001", "p_002"]
}
```

前端动作：

- 记录本轮回答关联商品。
- 调用商品详情接口渲染卡片。
- 不从自然语言中解析商品名称、价格或库存。

### 文本内 `<item>` 挂品标签

主 Agent 的自然语言 `text_delta` 中允许出现轻量挂品标签：

```text
这双更适合你现在的通勤和轻运动场景。<item>p_001</item>
```

约定：

- `<item>` 内只能放商品 ID，即后端商品接口中的 `productId` / 工具 observation 中的 `product_id`。
- 禁止在 `<item>` 内放商品名、品牌名、品类名、推荐语或自然语言挂品指令。
- 合法示例：`<item>p_001</item>`、`<item>p_beauty_001</item>`。
- 非法示例：`<item>X Phone 12</item>`、`<item>耐克跑鞋</item>`、`<item>推荐防水冲锋衣</item>`。
- 前端如果选择解析 `<item>`，只按商品 ID 读取，不要从文本周边推断商品名、价格或库存。
- 推荐优先使用结构化 `product_refs` block；`<item>` 只作为文本中挂品位置提示或兼容参考。

前端建议处理：

1. 扫描 `text_delta` 累积文本中的 `<item>{productId}</item>`。
2. 提取 `productId` 后去重。
3. 按 `productId` 调商品详情接口或批量详情接口渲染卡片。
4. 展示给用户的正文中可以隐藏 `<item>` 标签本身，只保留自然语言。
5. 如果同一轮同时收到 `product_refs` 和 `<item>`，以 `product_refs.product_ids` 为准，`<item>` 只补充排序或挂载位置。

### citation_refs

用于声明本轮回答引用了哪些知识片段。前端可选择折叠展示引用来源。

```json
{
  "type": "citation_refs",
  "chunk_ids": ["chunk_001", "chunk_002"]
}
```

### comparison_table

用于商品/品牌对比。

```json
{
  "type": "comparison_table",
  "columns": ["商品", "价格", "适合人群"],
  "rows": [
    {
      "productId": "p_001",
      "values": ["X Phone 12", "2999.00", "预算内拍照"]
    }
  ]
}
```

### cart_state

用于购物车动作后的最新状态。

```json
{
  "type": "cart_state",
  "cart": {
    "items": [],
    "summary": {
      "selectedCount": 1,
      "totalAmount": "2999.00",
      "discountAmount": "30.00",
      "payAmount": "2969.00"
    }
  }
}
```

前端动作：

- 刷新购物车角标。
- 可展示“去购物车/去结算”按钮。

### order_summary

用于 checkout 后展示订单摘要。

```json
{
  "type": "order_summary",
  "orders": [
    {
      "order_id": "ord_xxx",
      "order_no": "NO20260523120000",
      "merchant_id": "m_001",
      "merchant_name": "小猪数码旗舰店",
      "status": "pending_payment",
      "total_amount": "2999.00",
      "discount_amount": "30.00",
      "pay_amount": "2969.00",
      "payment_deadline_at": "2026-05-23T12:30:00+08:00",
      "items": []
    }
  ]
}
```

前端动作：

- 展示订单号、状态、应付金额、支付截止时间。
- `pending_payment` 状态展示“去支付”按钮。

### citation

用于 RAG 证据。

```json
{
  "type": "citation",
  "citation": {
    "chunkId": "ck_xxx",
    "title": "售后政策",
    "snippet": "7天无理由适用于...",
    "source": "doc_xxx"
  }
}
```

前端动作：

- 用折叠面板展示。
- 不作为主回答正文。

### warning

用于能力边界、缺参、降级。

```json
{
  "type": "warning",
  "code": "need_clarification",
  "message": "我还不能确定要加入购物车的是哪件商品。"
}
```

## 计划扩展 Block

为了支持后端 v3 的优惠、评价、非导购链路，建议新增以下 block。后端接入前，前端先按类型设计渲染兜底。

### discount_preview

```json
{
  "type": "discount_preview",
  "discount": {
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
}
```

### coupon_list

```json
{
  "type": "coupon_list",
  "coupons": [
    {
      "coupon_id": "coupon_platform_001",
      "name": "平台新人满 200 减 20",
      "threshold_amount": "200.00",
      "discount_amount": "20.00",
      "status": "active"
    }
  ]
}
```

### review_summary

```json
{
  "type": "review_summary",
  "product_id": "p_001",
  "rating_avg": "4.8",
  "rating_count": 120,
  "highlights": ["做工好", "物流快"],
  "risks": ["尺码偏小"]
}
```

### navigation_action

```json
{
  "type": "navigation_action",
  "action": {
    "label": "查看我的订单",
    "route": "orders",
    "params": {}
  }
}
```

用于非导购场景，例如订单、优惠、售后入口跳转。

## 前端解析状态机

每条用户消息对应一个 turn：

```ts
type AgentTurn = {
  userMessageId: string;
  userContent: string;
  runId?: string;
  status: 'idle' | 'streaming' | 'completed' | 'failed' | 'canceled';
  statusText?: string;
  text: string;
  blocks: AgentBlock[];
  followups: string[];
};
```

处理顺序：

1. 用户点击发送：前端创建 optimistic turn，`status=streaming`。
2. 收到 `message_start`：绑定真实 `run_id/user_message_id`。
3. 收到 `status`：更新 `statusText`。
4. 收到 `text_delta`：追加 `text`，清空 `statusText`。
5. 收到 `block_delta`：追加 `blocks`。
6. 收到 `followups`：设置追问按钮。
7. 收到 `message_end`：`status=completed`，清空 active run。
8. 收到 `error`：`status=failed`，展示错误信息。

## 后端输出约束

1. `run_id` 必须贯穿同一轮全部事件。
2. `message_start` 必须是第一条事件。
3. 正常结束必须发送 `message_end`。
4. 主回答文本只能通过 `text_delta` 输出。
5. 商品、订单、优惠、评价、引用证据必须通过 `block_delta` 输出。
6. 前端不需要解析 `<buyer>` 或其它 XML/内部标签；`<item>` 是唯一允许解析的文本内挂品标签，且标签内容必须是商品 ID。
7. `block_delta` 可以在 `text_delta` 之前、中间或之后发送；前端按收到顺序追加展示。
8. 错误后不再发送 `message_end`，前端以 `error` 为终态。

## 兼容策略

当前已实现事件：

- `message_start`
- `status`
- `text_delta`
- `block_delta`
- `followups`
- `message_end`
- `error`

当前已实现 block：

- `product_card`
- `comparison_table`
- `cart_state`
- `order_summary`
- `citation`
- `warning`
- `markdown` 类型在前端已有定义，但后端当前较少使用。

新增 block 必须先更新：

1. `backend/src/domain/types.go` 的 `AgentBlock` 字段。
2. `frontend/src/types/agent.ts` 的 `AgentBlock` union。
3. `frontend/src/components/chat/ChatMessageList.tsx` 的渲染分支。
4. 本协议文档。
