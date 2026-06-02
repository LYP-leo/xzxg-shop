# Agent 输出协议与挂品流式化改造设计 v1

## 1. 背景

当前 Agent 最终回答存在三个问题：

1. Prompt 中仍要求输出 `<buyer>` 标签，但前端和客户端没有消费该标签，实际只会变成正文噪音或被过滤。
2. 管理员请求追踪能看到模型原始输出和工具结果，但看不到每次 `llm_call` 的实际输入 messages/prompt，排查 Prompt 是否生效、是否被后续指令覆盖不够直接。
3. 当前前端渲染结构是“正文 text 在上，所有 blocks 在下”。当 Agent 一次返回多个 `product_refs` 或商品 block 时，前端会在消息底部集中展示多张商品卡，无法做到“介绍一个商品 -> 挂一个卡片 -> 再介绍下一个商品”的导购阅读体验。

本设计目标是把 Agent 输出从“文本 + 底部结构化块”升级为“按生成顺序排列的内容流”，同时保持旧协议可兼容。

## 2. 目标

### 2.1 功能目标

- 移除 `<buyer>` 标签及相关 Prompt 要求。
- 管理员链路追踪展示每次大模型调用的实际输入 prompt/messages。
- 支持商品卡片按文本位置穿插展示。
- 保持 `<item>product_id</item>` 作为模型侧轻量挂品位置指令，但前端不直接解析该标签。
- 后端负责把 `<item>` 转换为结构化商品卡片事件。
- React 前端和 Android/Kotlin 客户端都能按统一协议渲染。

### 2.2 非目标

- 不让前端从自然语言里解析商品 ID。
- 不让模型直接输出完整商品卡 JSON。
- 不取消现有 `text_delta` / `block_delta`，需要保留一段兼容期。
- 不在本阶段引入复杂富文本编辑协议。

## 3. 当前协议问题

当前前端数据结构：

```ts
type AgentTurn = {
  text: string;
  blocks: AgentBlock[];
};
```

当前渲染顺序：

```tsx
<MarkdownText content={turn.text} />
{turn.blocks.map(block => <AgentBlockView block={block} />)}
```

因此即使后端在流式过程中发送了 `block_delta`，页面结构仍然天然倾向于“文本在上，卡片在下”。这和导购场景不匹配，尤其是多商品推荐时，用户需要在商品说明附近看到对应商品卡。

## 4. 协议设计

### 4.1 新增内容片段模型

新增统一内容片段 `AgentContentPart`：

```ts
type AgentContentPart =
  | { type: 'text'; content: string }
  | { type: 'product_card'; product: ProductCard }
  | { type: 'citation'; citation: Citation }
  | { type: 'comparison_table'; columns: string[]; rows: Array<{ productId: string; values: string[] }> }
  | { type: 'cart_state'; cart: Cart }
  | { type: 'order_summary'; orders: Order[] }
  | { type: 'action'; message?: string; action: { name: string; target: string; label?: string } }
  | { type: 'warning'; code: string; message: string };
```

`AgentTurn` 新增字段：

```ts
type AgentTurn = {
  text: string;                 // v1 兼容字段
  blocks: AgentBlock[];         // v1 兼容字段
  contentParts: AgentContentPart[];
};
```

### 4.2 新增 SSE 事件

新增：

```json
{
  "type": "content_delta",
  "run_id": "run_001",
  "part": {
    "type": "product_card",
    "product": {
      "product_id": "p_beauty_003",
      "name": "SK-II护肤精华露...",
      "price": "1690.00"
    }
  }
}
```

文本也走同一事件：

```json
{
  "type": "content_delta",
  "run_id": "run_001",
  "part": {
    "type": "text",
    "content": "## 这款适合什么人\n\n"
  }
}
```

### 4.3 兼容策略

后端在过渡期可以同时发送：

- `content_delta`：新前端和 Android 使用。
- `text_delta`：旧前端兼容。
- `block_delta`：旧前端兼容。

推荐灰度策略：

1. 后端先支持 `content_delta`，同时保留旧事件。
2. React 前端优先渲染 `contentParts`，没有 `contentParts` 时回退 `text + blocks`。
3. Android 客户端直接按 `content_delta` 接入。
4. 确认稳定后，旧 `block_delta` 仅保留给非正文流式块或历史兼容。

## 5. 后端实现设计

### 5.1 去掉 buyer 标签

Prompt 层：

- 删除所有 intent prompt 中的 `<buyer>` 示例。
- 删除“末尾先输出 `<buyer>`”类要求。
- 删除“`<buyer>` 与 `<item>` 必须一起出现”类要求。

过滤层：

- `streamTextFilter` 增加 `<buyer>...</buyer>` 过滤。
- 防止历史 Prompt、Nacos 未更新或模型惯性输出造成前端污染。

输出硬约束改为：

```markdown
- 禁止输出 <buyer> 标签。
- 如果输出 <item>...</item>，<item> 内只能写工具返回的 product_id。
- <item> 是商品卡片插入位置指令，不是用户可见文本。
- 不要在回答末尾集中输出一串 <item>；每个 <item> 应靠近对应商品说明。
```

### 5.2 item 标签到商品卡片

模型最终输出示例：

```markdown
# 美妆护肤怎么选

## 先看肤质和功效

如果你是 25+、有暗沉或毛孔问题，可以优先看这款：

<item>p_beauty_003</item>

它的重点是 **PITERA™** 和角质调理，适合想改善粗糙、暗沉的人群。

如果你更关注干皮保湿：

<item>p_beauty_019</item>

这款更偏 **舒缓保湿**，适合干性和换季敏感肌。

<further>你更关注保湿、抗初老还是控油？</further>
```

后端处理：

1. 普通文本增量：输出 `content_delta.text`，并兼容输出 `text_delta`。
2. 遇到完整 `<item>p_xxx</item>`：
   - 校验 `p_xxx` 是否在 `final_allowed_product_ids` 白名单内。
   - 校验是否已挂过，默认同一商品只挂一次。
   - 查询商品卡片详情。
   - 输出 `content_delta.product_card`。
   - 兼容期可同时输出 `block_delta.product_card`，但旧前端会底部重复堆卡，建议只对声明支持 v1 的客户端发送。
3. 标签本身不进入 `text_delta`。

### 5.3 product_refs 的角色调整

`product_refs` 继续保留，但只作为最终摘要或兼容字段：

- 用于 `message_end.final_output.blocks`。
- 用于历史记录和评测。
- 不再作为用户端主要挂品展示来源。

用户端主要展示应依赖 `content_delta.product_card`。

## 6. 管理员 Trace 设计

### 6.1 记录范围

每次 `llm_call` trace 增加：

```json
{
  "model": "qwen-turbo",
  "temperature": 0.4,
  "prompt_chars": 12345,
  "prompt_hash": "sha256:xxxx",
  "messages": [
    { "role": "system", "content": "..." },
    { "role": "user", "content": "..." },
    { "role": "assistant", "content": "..." }
  ],
  "raw_output": "..."
}
```

### 6.2 脱敏规则

写入 trace 前执行脱敏：

- `Authorization: Bearer ...`
- `api_key`
- `token`
- `password`
- 手机号、邮箱等用户敏感信息可先做简单正则脱敏。

### 6.3 配置开关

新增动态配置：

```json
{
  "trace.llm.prompt_capture": "full",
  "trace.llm.prompt_max_chars": "20000"
}
```

取值建议：

| 值 | 含义 |
| --- | --- |
| `off` | 不记录 prompt |
| `truncated` | 截断记录 |
| `full` | 完整记录，开发和比赛演示环境使用 |

### 6.4 管理员页面展示

在请求追踪详情中，每个 `llm_call` 节点新增：

- 输入 Prompt 折叠区。
- System/User/Assistant messages 分角色展示。
- 一键复制完整 messages JSON。
- Prompt 长度、hash、模型、temperature。
- 原始输出继续保留。

默认不要在列表页展开完整 prompt，避免页面卡顿。

## 7. Prompt 改造设计

### 7.1 通用硬约束

所有导购 intent prompt 共用：

```markdown
【输出硬约束】
- 禁止输出 <buyer> 标签。
- 如果输出 <item>...</item> 挂品标签，<item> 内只能写已由工具返回的 product_id，例如 <item>p_001</item>。
- <item> 是商品卡片插入位置指令，不是正文内容。
- 不要在末尾集中输出多个 <item>。
- 每个 <item> 应出现在对应商品说明附近；推荐写法是先给一段选择逻辑，再输出 <item>，随后说明该商品特点和适合人群。
- 重点词、品牌词、系列词、属性词必须用 Markdown 加粗，例如 **耐克**、**防水**。
- 禁止输出 special_word、special word、（special_word）等内部标识。
```

### 7.2 category_shop_no_brand 示例

```markdown
# [一级标题：回应品类选购需求]

## [二级标题：这个品类先看哪些决策因子]

[说明 2-3 个关键决策因子]

## [二级标题：推荐方向一]

[先解释为什么这个方向适合用户]

<item>p_001</item>

[围绕 p_001 说明特点、适合人群、风险和入手建议]

## [二级标题：推荐方向二]

<item>p_002</item>

[围绕 p_002 说明差异和取舍]

<further>追问内容，40字左右</further>
```

### 7.3 scene_solution 示例

```markdown
# [一级标题：回应场景解决方案]

## [二级标题：场景目标和风险]

[说明地点、季节、人群、预算和容易遗漏的点]

| 清单品类 | 为什么需要 | 怎么选 |
| --- | --- | --- |
| [品类] | [场景价值] | [选购要点] |

<inventory>清单摘要</inventory>

## [二级标题：核心品类一]

<item>p_001</item>

[说明该商品在当前场景中的作用和注意事项]

## [二级标题：核心品类二]

<item>p_002</item>

[说明与上一件商品的搭配或互补关系]

<further>追问内容，40字左右</further>
```

## 8. React 前端改造清单

### 8.1 类型

修改 `frontend/src/types/agent.ts`：

- 新增 `AgentContentPart`。
- `AgentTurn` 新增 `contentParts`。
- `AgentSseEvent` 新增 `content_delta`。

### 8.2 SSE 处理

修改 `frontend/src/pages/AgentSessionPage.tsx`：

- 收到 `content_delta.text`：
  - append 到 `turn.contentParts`。
  - 可同步 append 到 `turn.text` 以兼容复制/历史摘要。
- 收到 `content_delta.product_card`：
  - append 到 `turn.contentParts`。
- 收到旧 `block_delta`：
  - 如果已经启用 `contentParts`，可只用于非正文结构块或放入兼容区。

### 8.3 渲染

修改 `frontend/src/components/chat/ChatMessageList.tsx`：

当前：

```tsx
<MarkdownText content={turn.text} />
<BlockList blocks={turn.blocks} />
```

改为：

```tsx
{turn.contentParts.length ? (
  turn.contentParts.map(renderContentPart)
) : (
  <>
    <MarkdownText content={turn.text} />
    <BlockList blocks={turn.blocks} />
  </>
)}
```

`renderContentPart`：

- `text` -> `MarkdownText`
- `product_card` -> `ProductCard`
- `citation` -> `CitationView`
- `comparison_table` -> `ComparisonTable`
- `action` -> `ActionBlock`

### 8.4 样式

- 商品卡片作为消息正文的一部分展示，宽度应与消息内容对齐。
- 多张卡片之间保留 8-12px 间距。
- 不要再在消息底部集中展示 product card。

## 9. Android/Kotlin 客户端改造清单

### 9.1 数据模型

新增：

```kotlin
sealed interface AgentContentPart {
    data class Text(val content: String) : AgentContentPart
    data class ProductCard(val product: ProductCardDto) : AgentContentPart
    data class Citation(val citation: CitationDto) : AgentContentPart
    data class Warning(val code: String, val message: String) : AgentContentPart
}
```

`AgentTurn` 增加：

```kotlin
val contentParts: List<AgentContentPart>
```

### 9.2 SSE 解析

- 支持 `content_delta`。
- 保留 `text_delta` 和 `block_delta` 兼容。
- 对未知 part type 做兜底展示，不应崩溃。

### 9.3 UI 渲染

- 使用 `LazyColumn` 或消息内部纵向布局按 `contentParts` 顺序渲染。
- Text part 使用 Markdown 渲染或基础富文本。
- ProductCard part 使用商品卡组件。
- 点击商品卡进入商品详情。
- 加购按钮复用现有购物车接口。

## 10. API 文档更新点

需要更新 `.ai/api/Agent输出协议_v1.md` 或新增 `Agent输出协议_v2.md`：

1. 新增 `content_delta` 事件。
2. 新增 `AgentContentPart` 类型。
3. 标明 `<buyer>` 已废弃。
4. 标明 `<item>` 只作为后端解析指令，客户端不解析。
5. 给出完整 SSE 示例：
   - `message_start`
   - `content_delta.text`
   - `content_delta.product_card`
   - `content_delta.text`
   - `content_delta.product_card`
   - `followups`
   - `message_end`

## 11. 实施顺序

### Phase 1：后端 Prompt 与过滤

- 清理 Nacos 和默认 Prompt 中的 `<buyer>`。
- `streamTextFilter` 过滤 `<buyer>...</buyer>`。
- 更新 Prompt 硬约束，要求 `<item>` 就近插入，不要末尾集中输出。

### Phase 2：Trace Prompt 可视化

- LLM 调用前记录 messages。
- trace metadata 增加 `prompt_hash/prompt_chars/messages`。
- 管理员页面增加输入 Prompt 折叠区。

### Phase 3：content_delta 协议

- 后端新增 `content_delta` SSE。
- `streamTextFilter` 解析 `<item>` 时输出 `product_card` part。
- 保留旧事件兼容。

### Phase 4：React 前端改造

- 新增 `contentParts`。
- 按顺序渲染 text/product_card/citation/action。
- 保留旧 `text + blocks` 回退。

### Phase 5：Android/Kotlin 接入

- 按 v2 协议实现 SSE。
- 按 `contentParts` 顺序渲染。
- 完成商品卡点击和加购联动。

## 12. 验收标准

1. 任意 Agent 回答中不再出现 `<buyer>` 标签。
2. 管理员 trace 中每次 `llm_call` 都能查看实际输入 messages。
3. 多商品推荐时，商品卡片出现在对应文字说明附近，而不是统一堆在底部。
4. `<item>` 内只能接受 `final_allowed_product_ids` 中的商品 ID。
5. 同一商品默认不会重复挂多张卡。
6. React 前端和 Android 客户端均不需要解析自然语言标签。
7. 旧 `text_delta/block_delta` 客户端在兼容期内仍可正常展示。
