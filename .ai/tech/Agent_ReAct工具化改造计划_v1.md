# Agent ReAct 工具化改造计划 v1

## 背景

当前 `backend/src/agent/runtime.go` 里的主链路仍然是“代码先做意图识别、商品/知识库预检索、硬编码工具动作，再把候选内容塞给模型生成回答”。这会导致几个问题：

- 主 Agent 不能自主决定是否查商品、查知识库或操作购物车。
- 商品卡、对比表、引用片段由 Runtime 固定追加，前端和 Agent 输出协议耦合较重。
- 购物车、下单、问候、非导购兜底等动作分散在 `executeDeterministicIntent`，不符合“非导购意图/工具调用统一处理”的目标。
- `streamAnswer` 仍然只是普通流式回答，不是 ReAct。

目标是把链路改成：意图识别只决定路由和 prompt，主 Agent 在 ReAct 循环中按需调用工具，最终输出文本和结构化引用。

## runtime.go TODO 对应关系

| 位置 | 当前问题 | 改造方向 |
| --- | --- | --- |
| `Stream` 预检索 `SearchProducts/SearchKnowledge` | Runtime 先查商品和知识库，再塞给模型 | 改成工具，由主 Agent 自己决定是否调用 |
| `executeDeterministicIntent` | 硬编码购物车、下单、非导购等动作 | 删除主链路提前拦截，把动作迁移到 ReAct tools 或非导购 prompt |
| 固定输出 `product_card` | Runtime 直接把完整商品卡发给前端 | Agent final 只输出商品 ID 引用，前端按 ID 查详情 |
| 固定输出对比表/引用片段 | Runtime 根据 intent 追加 block | 由主 Agent 自主决定 final blocks |
| `streamAnswer` | 只是普通流式 LLM 回答 | 改成 ReAct 循环：模型决策、工具执行、观察结果、最终回答 |

## 目标链路

```text
用户消息
  -> message_start
  -> 两阶段意图识别
       1. route: guide / non_guide / fast_product
       2. guide 内部 P1-P6 细分
  -> 根据 route/intent 选择主 Agent prompt
  -> ReAct 循环
       -> 模型输出 tool_call 或 final
       -> 后端执行 tool_call
       -> 工具结果作为 observation 回传模型
       -> 最多循环 N 轮
  -> text_delta 流式输出最终回答
  -> block_delta 输出结构化引用
  -> followups
  -> message_end
```

## ReAct 协议

第一版不引入外部 Agent 框架，先实现轻量 JSON Action ReAct。原因：

- 当前 LLM client 走 OpenAI-compatible `/chat/completions`，还没有 function calling 封装。
- JSON Action 容易调试、容易落 trace，也不依赖具体模型 function-call 能力。
- 可以先把业务链路跑通，后续再替换为标准 tool calling。

### 模型输出格式

模型每一步只能输出以下两种 JSON 之一。

工具调用：

```json
{
  "type": "tool_call",
  "tool": "search_products",
  "arguments": {
    "query": "500元以内 跑鞋",
    "limit": 5
  }
}
```

最终回答：

```json
{
  "type": "final",
  "text": "我建议优先看这两款...",
  "blocks": [
    {
      "type": "product_refs",
      "product_ids": ["p_001", "p_002"]
    }
  ]
}
```

### 后端执行规则

- 单次 run 最多执行 6 个 ReAct step。
- 连续 2 次 JSON 解析失败时，进入兜底回答。
- 工具调用失败不直接中断，把错误作为 observation 交给模型，让模型解释或改问。
- 每个 step 都写入 `agent_trace_events`，字段包含 `tool`、`arguments`、`result_count`、`duration_ms`、`status`。
- 最终只允许输出白名单 block 类型，避免模型伪造前端不能解析的结构。

## 工具清单

### `search_products`

用途：商品搜索、导购推荐、加购前确认候选商品。

参数：

```json
{
  "query": "string",
  "limit": 5
}
```

返回精简字段：

```json
{
  "items": [
    {
      "product_id": "p_001",
      "sku_id": "sku_001",
      "name": "X Phone 12",
      "brand": "X",
      "price": "2999.00",
      "merchant_name": "小猪数码旗舰店",
      "selling_points": ["高速对焦", "儿童抓拍模式"],
      "risk_notes": []
    }
  ]
}
```

### `search_knowledge`

用途：查知识库资料、商品说明、平台规则、导购依据。

参数：

```json
{
  "query": "string",
  "limit": 3
}
```

返回精简字段：

```json
{
  "items": [
    {
      "chunk_id": "k_001",
      "title": "手机选购指南",
      "snippet": "选购手机时应关注...",
      "source": "guide"
    }
  ]
}
```

### `get_cart`

用途：购物车查询、修改数量、删除商品、结算前确认。

参数：

```json
{}
```

返回：当前用户购物车摘要和商品项。

### `add_cart_item`

用途：把明确商品加入购物车。

参数：

```json
{
  "product_id": "p_001",
  "sku_id": "sku_001",
  "quantity": 1
}
```

规则：

- 如果没有明确商品 ID，Agent 应先调用 `search_products` 或向用户澄清。
- 工具执行成功后返回最新购物车。

### `update_cart_item`

用途：修改购物车数量或选中状态。

参数：

```json
{
  "cart_item_id": "cart_001",
  "quantity": 2,
  "selected": true
}
```

规则：

- 如果用户说“第一个/第二个”，Agent 应先调用 `get_cart` 找到对应 `cart_item_id`。

### `delete_cart_item`

用途：删除购物车商品。

参数：

```json
{
  "cart_item_id": "cart_001"
}
```

### `checkout`

用途：创建待支付订单。

参数：

```json
{}
```

规则：

- 执行前建议先 `get_cart`，确认有选中商品。
- 创建订单后返回订单摘要，不直接支付。

## 前端输出协议调整

保留现有 SSE 事件类型：

- `message_start`
- `status`
- `text_delta`
- `block_delta`
- `followups`
- `message_end`
- `error`

新增推荐 block：

```json
{
  "type": "product_refs",
  "product_ids": ["p_001", "p_002"]
}
```

含义：

- Agent 只告诉前端“本次回答引用了哪些商品”。
- 前端需要展示商品卡时，自己调用商品详情接口。
- 后端可以后续补一个批量接口：`GET /api/v1/products:batch?ids=p_001,p_002`。

保留兼容旧 block：

- `cart`
- `order_summary`
- `warning`
- `citation`

但 Runtime 不再主动根据 intent 固定追加 `product_card`、`comparison`、`citation`。

## 代码改造拆分

### 1. 新增 `backend/src/agent/tools.go`

内容：

- `ToolCall`
- `ToolResult`
- `ToolExecutor`
- `executeTool(ctx, run, call)`
- 参数解析和校验
- 工具结果精简格式化

工具实现只依赖 `store.Store`，不直接访问 MySQL。

### 2. 新增 `backend/src/agent/react.go`

内容：

- `reactStep`
- `reactFinal`
- `runReactAgent`
- JSON 提取和白名单校验
- 最大 step 控制
- 工具 observation 拼接
- trace 记录

### 3. 改造 `backend/src/agent/runtime.go`

改动：

- 删除 `Stream` 中的 `products/chunks` 预检索。
- 删除主链路里的 `executeDeterministicIntent` 提前拦截。
- 删除 Runtime 固定追加 `product_card`、`comparison`、`citation`。
- `streamAnswer` 改为调用 `runReactAgent`。
- `followups` 不再依赖提前检索的 products，改为基于 query + final blocks 生成。

### 4. Prompt 配置改造

需要新增或更新 Nacos 配置：

- `agent.prompt.main_agent`
- `agent.prompt.main_agent.guide`
- `agent.prompt.main_agent.non_guide`
- `agent.prompt.main_agent.fast_product`
- `agent.prompt.tool_protocol`

核心要求：

- 明确只能输出 JSON Action。
- 明确什么时候必须调用工具。
- 明确不要编造商品、价格、库存、订单状态。
- 明确最终回答里商品只通过 `product_refs` 引用。

### 5. 测试和质量验证

后端测试：

```bash
cd backend
go test ./...
```

手动验证：

| 用例 | 预期 |
| --- | --- |
| `你好` | 不查商品，直接正常问候，不返回默认假数据 |
| `推荐一双500以内跑鞋` | 调用 `search_products`，最终输出商品 ID 引用 |
| `对比这两款手机` | 如无上下文则澄清；有上下文则基于工具结果对比 |
| `把刚才推荐的第一个加入购物车` | 调用 `add_cart_item`，返回购物车 block |
| `删除购物车第二个商品` | 调用 `get_cart` + `delete_cart_item` |
| `确认下单` | 调用 `get_cart` + `checkout` |
| `我拍照找同款` | 仍然明确提示 VLM 未配置，不伪造识别 |

质量脚本：

```bash
node quality/evals/run_agent_e2e.mjs
```

## 风险和处理

### JSON 解析失败

风险：模型输出自然语言或 Markdown 包裹 JSON。

处理：

- 使用 `extractJSONObject` 兼容包裹文本。
- 连续失败后降级到普通回答。
- prompt 强约束“只能输出 JSON”。

### 工具循环过多

风险：模型反复搜索不结束。

处理：

- 最大 6 步。
- 搜索工具最多连续调用 2 次。
- 超限后要求模型基于已有 observation 给最终答复。

### 加购缺少商品 ID

风险：用户说“加到购物车”，但当前没有明确商品。

处理：

- Agent 先 `search_products`。
- 如果仍不明确，final 中要求用户确认，不执行加购。

### 前端展示兼容

风险：前端现在可能依赖 `product_card`。

处理：

- 第一版保留旧 block 解析兼容。
- 新协议新增 `product_refs`。
- 如前端暂时未改，可后端临时把 `product_refs` 转为旧 `product_card`，但这只是兼容层，不放在主 Agent 决策里。

## 推荐分支

```bash
git checkout -b feature/agent-react-tools
```

## 里程碑

### M1：工具化最小闭环

- 新增 tools executor。
- 新增 ReAct loop。
- 支持 `search_products`、`search_knowledge`、`get_cart`。
- 商品推荐类问题能由 Agent 主动查商品后回答。

### M2：购物车和订单动作迁移

- 支持 `add_cart_item`、`update_cart_item`、`delete_cart_item`、`checkout`。
- 删除 `executeDeterministicIntent` 主链路调用。
- 加购、删除、下单都通过 ReAct tool trace 可追踪。

### M3：输出协议收敛

- Runtime 不再固定追加商品卡/对比表/引用。
- final blocks 输出 `product_refs`。
- 更新 `.ai/api/Agent输出协议_v1.md` 或新增 v2。

### M4：Prompt 和 Nacos

- 将主 Agent prompt 和 tool protocol 拆到 Nacos。
- 本地默认 prompt 与 Nacos 配置保持一致。
- 关闭 thinking 保持现有策略。

### M5：验证

- `go test ./...`
- `quality/evals/run_agent_e2e.mjs`
- 手动测试真实流式输出、工具 trace、购物车/订单状态。

