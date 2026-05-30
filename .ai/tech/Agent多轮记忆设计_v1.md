# Agent 多轮记忆设计 v1

## 1. 背景

当前 Agent 会话链路已经具备 `chat_sessions`、`user_messages`、`agent_runs`、`agent_trace_events` 和 `chat_sessions.summary` 等基础表结构，但主链路执行时仍主要依赖当前轮 `UserMessage.Content`。

因此同一个 `session_id` 下，后续问题无法稳定理解前文语境，例如：

- “刚才那个加入购物车”
- “换个便宜点的”
- “它和第二个比呢”
- “还是按我之前说的通勤场景推荐”

本设计目标是把多轮上下文能力接入现有 Agent Runtime，同时控制范围，避免一开始就引入不可控的长期记忆。

## 2. 目标

### 2.1 功能目标

- 支持同一会话内的指代消解、偏好延续和对比延续。
- 支持最近对话窗口注入 Planner、Guide Intent、ReAct 和 Final Answer。
- 保存助手最终回答，形成完整 user / assistant 对话历史。
- 利用 `chat_sessions.summary` 保存会话级摘要，降低长会话 token 压力。
- 为后续长期用户偏好记忆预留表和接口边界。

### 2.2 非目标

- 不在一期实现跨会话长期画像。
- 不把历史回答中的价格、库存、优惠等内容当作当前事实。
- 不让模型基于历史内容伪造商品 ID、购物车项 ID、订单 ID。
- 不改变现有 SSE 协议的基本形态。

## 3. 当前系统现状

已有数据结构：

- `chat_sessions`
  - `summary`
  - `message_count`
  - `last_message_at`
- `user_messages`
  - 用户每轮输入
  - 附件 JSON
- `agent_runs`
  - 每条用户消息对应一次 Agent 执行
- `agent_trace_events`
  - LLM 调用、工具调用和运行状态追踪

当前缺口：

- 没有稳定保存助手最终回答的业务表。
- `Runtime.Stream` 只接收当前用户消息，没有读取最近对话。
- ReAct 工具循环只在单轮内保留中间 observation。
- `chat_sessions.summary` 目前主要用于列表摘要，没有进入 Agent prompt。

## 4. 总体方案

分三期推进：

1. 一期：短期多轮上下文。
2. 二期：会话摘要。
3. 三期：长期用户偏好记忆。

优先顺序是先做到会话内可用，再做摘要压缩，最后做跨会话偏好。这样能快速验证“刚才那个”“继续看”“便宜点”等核心体验，也能避免把短期上下文和长期记忆混在一起。

## 5. 一期：短期多轮上下文

### 5.1 新增助手消息表

新增 `assistant_messages` 表，保存每次 Agent 最终回答。

建议字段：

```sql
CREATE TABLE IF NOT EXISTS assistant_messages (
  assistant_message_id VARCHAR(64) PRIMARY KEY,
  run_id VARCHAR(64) NOT NULL,
  session_id VARCHAR(64) NOT NULL,
  message_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  content MEDIUMTEXT NOT NULL,
  product_ids_json JSON NOT NULL,
  citation_ids_json JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_assistant_messages_account_session (account_id, session_id),
  INDEX idx_assistant_messages_run_id (run_id),
  INDEX idx_assistant_messages_message_id (message_id)
);
```

说明：

- `content` 保存最终给用户看的文本。
- `product_ids_json` 保存本轮回答中挂出的商品 ID。
- `citation_ids_json` 保存本轮回答引用的知识片段 ID。
- 与 `agent_trace_events` 区分：trace 面向排障，assistant message 面向业务对话历史。

### 5.2 Store 接口

在 `store.Store` 增加：

```go
SaveAssistantMessage(ctx context.Context, input domain.AssistantMessageInput) (domain.AssistantMessage, error)
ListRecentConversationTurns(ctx context.Context, accountID string, sessionID string, limit int) []domain.ConversationTurn
GetLastAssistantProducts(ctx context.Context, accountID string, sessionID string, limit int) []string
```

建议领域模型：

```go
type AssistantMessage struct {
    AssistantMessageID string
    RunID              string
    SessionID          string
    MessageID          string
    AccountID          string
    Content            string
    ProductIDs         []string
    CitationIDs        []string
    CreatedAt          time.Time
}

type ConversationTurn struct {
    UserMessage      UserMessage
    AssistantMessage *AssistantMessage
}
```

### 5.3 保存时机

保存位置应在最终回答已经聚合完成之后：

- ReAct `<final>...</final>` 流式结束后保存。
- fallback final 输出结束后保存。
- 如果生成失败或用户取消，不保存完整助手消息；可选保存状态为 `canceled` 的短文本，但一期不建议做。

当前可优先接入：

- `runReactAgent`
- `emitReactFinalText`
- `streamReactFinal`

保存逻辑需要包含：

- 最终文本。
- 本轮挂品 `product_ids`。
- 本轮引用 `citation_ids`。
- `run_id / session_id / message_id / account_id`。

### 5.4 上下文构建

每次 `Runtime.Stream` 开始时读取最近 6 到 10 轮：

```text
当前会话历史：
用户：我想买一个通勤用静音鼠标
助手：推荐 Quiet Mouse S，静音微动、无线、续航长...
用户：有没有便宜点的？
助手：...
```

注入原则：

- 历史用于理解指代、偏好、比较对象和场景。
- 当前事实仍要通过工具确认，尤其是价格、库存、优惠、售后、订单状态。
- 历史中的商品 ID 可以作为指代候选，但加购、结算前仍需满足现有工具规则。

建议新增 helper：

```go
func (r *Runtime) conversationMemory(ctx context.Context, run domain.AgentRun, current domain.UserMessage) conversationMemory
func formatConversationMemory(memory conversationMemory) string
```

### 5.5 Prompt 注入点

一期至少注入四处：

1. Route Planner  
用于判断“继续推荐”“便宜点”“第二个”这类 query 是否仍属于导购。

2. Guide Intent Classifier  
用于把“它和第二个比呢”归到 `compare_decide`。

3. ReAct System/User Messages  
用于工具选择和指代消解。

4. Final Answer  
用于保持回答连续性，不重复问已知偏好。

建议 prompt 片段：

```text
会话记忆：
{{conversation_memory}}

使用规则：
- 会话记忆只用于理解当前 query 的指代、偏好和上下文。
- 涉及价格、库存、优惠、售后、订单状态时，必须以工具结果为准。
- 不要把历史回答中未被本轮工具确认的信息当作当前事实。
```

## 6. 二期：会话摘要

短期窗口超过 token 后，需要结合 `chat_sessions.summary`。

### 6.1 摘要内容

摘要只保留可复用决策信息：

- 用户偏好：预算、场景、人群、品牌、颜色、尺码、禁忌。
- 已讨论商品：商品 ID、名称、用户态度。
- 当前任务状态：在比较、待加购、需要补充预算、已推荐但未决策。

不建议写入：

- 完整闲聊内容。
- 已过期的价格、库存、活动。
- 单次临时情绪或无稳定意义的描述。

### 6.2 更新策略

可选两种：

1. 同步更新  
每轮回答结束后，如果 `message_count % N == 0`，调用小模型更新 summary。

2. 异步更新  
回答结束后启动后台 goroutine，带超时更新 summary。失败不影响主链路。

推荐先用异步更新，避免影响首屏和流式完成时间。

### 6.3 Prompt 组合

长期会话 prompt 采用：

```text
会话摘要：
{{session_summary}}

最近对话：
{{recent_turns}}
```

优先级：

1. 当前用户 query。
2. 本轮工具结果。
3. 最近对话。
4. 会话摘要。

## 7. 三期：长期用户偏好记忆

三期再引入跨会话 `user_memories`。

建议表：

```sql
CREATE TABLE IF NOT EXISTS user_memories (
  memory_id VARCHAR(64) PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL,
  memory_type VARCHAR(32) NOT NULL,
  content TEXT NOT NULL,
  confidence DECIMAL(4,3) NOT NULL DEFAULT 0.800,
  source_session_id VARCHAR(64) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_user_memories_account_type (account_id, memory_type)
);
```

只保存稳定偏好，例如：

- “偏好静音办公鼠标”
- “常买 42 码跑鞋”
- “护肤品偏好温和修护，避免强刺激酸类”

不保存临时诉求，例如：

- “今天想买防晒”
- “刚才那件不喜欢”
- “这次预算 300 元以内”

## 8. 风险与约束

### 8.1 历史事实过期

价格、库存、活动可能变化。解决方式：

- Prompt 明确历史不作为当前事实。
- 工具结果优先级高于历史。
- 商品推荐和加购前仍调用 `search_products` 或购物车工具确认。

### 8.2 Token 增长

解决方式：

- 一期限制最近 6 到 10 轮。
- 二期引入 summary。
- 历史内容截断单轮最长字符数。

### 8.3 指代错误

解决方式：

- 保存上一轮挂品 ID。
- 对“第一个、第二个、刚才那个”优先绑定最近助手消息中的 `product_ids_json`。
- 多候选不明确时让模型澄清，不猜。

### 8.4 并发与重复提交

当前已有 `client_message_id` 幂等约束。一期需保证：

- 重复请求不重复保存助手消息。
- `assistant_messages.run_id` 可考虑加唯一索引。
- 保存失败不应导致流式回答失败，但应打日志和 trace。

## 9. 验收用例

### 9.1 指代加购

用户：

```text
推荐一个通勤用静音鼠标
```

助手推荐 `p_mouse_001`。

用户：

```text
刚才那个加购物车
```

期望：

- 识别“刚才那个”为上一轮挂品。
- 调用加购工具。
- 不重新泛搜无关商品。

### 9.2 继续筛选

用户：

```text
推荐一双跑步鞋，预算 500
```

用户：

```text
换个更适合雨天的
```

期望：

- 保留“跑步鞋、预算 500”。
- 新增“雨天、防滑、防水”等约束。

### 9.3 多商品比较

用户：

```text
这两个手机哪个适合拍娃？
```

助手给出两个商品。

用户：

```text
那第二个续航怎么样？
```

期望：

- “第二个”绑定上一轮第二个挂品。
- 回答前检索或使用工具结果确认商品信息。

### 9.4 历史事实不直接复用

用户上一轮看到某商品价格。

用户：

```text
它现在还有货吗？
```

期望：

- 不直接引用历史库存。
- 通过商品工具确认当前库存。

## 10. 推荐落地顺序

1. 新增 `assistant_messages` 表和 domain 类型。
2. 实现助手最终回答保存。
3. 实现最近对话读取接口。
4. 在 planner、intent、ReAct、final 注入最近对话。
5. 增加指代解析辅助：上一轮挂品、序号映射。
6. 加入会话摘要异步更新。
7. 评估后再做长期 `user_memories`。

## 11. 一期最小改造范围

一期最小可交付版本只需要：

- `assistant_messages` 表。
- `SaveAssistantMessage`。
- `ListRecentConversationTurns`。
- 最近 6 轮对话注入 prompt。
- 覆盖“刚才那个”“第二个”“便宜点”“继续”四类测试。

这样可以最快让多轮体验可用，同时不阻塞后续摘要和长期记忆扩展。
