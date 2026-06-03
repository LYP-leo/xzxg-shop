# 安卓 APP 技术方案 v23

## 1. 背景

本文针对 [安卓APP_问题_v22.md](./安卓APP_问题_v22.md) 设计下一版 Android 原生 APP 修复方案。

本轮聚焦聊天页面三个问题：

- 从对话历史返回对话时，商品卡片仍可能集中显示在 assistant 对话框底部，没有严格出现在 `<item>...</item>` 标签所在位置。
- 需要重新核对后端输出协议，补齐 Android 端尚未消费或消费不完整的标签和结构化事件。
- 语音输入仍不稳定，需要给出更完整的语音转文字实现方案和降级路径。

## 2. 后端协议核对结论

已核对后端文档和当前代码：

- 后端文档 `Agent输出协议与挂品流式化改造设计_v1.md` 明确：`<item>product_id</item>` 是模型侧挂品位置指令，后端应转换为结构化商品卡片事件；客户端不应把 `<item>` 当普通可见文本。
- 后端当前已实现 `content_delta`：
  - `content_delta.part.type = text`
  - `content_delta.part.type = product_card`
- 后端仍保留旧兼容事件：
  - `text_delta`
  - `block_delta`
- 后端最终 blocks 仍可能包含：
  - `product_refs`
  - `citation_refs`
  - `comparison_table`
  - `warning`
  - `cart_state`
  - `order_summary`
  - `discount_preview`
  - `coupon_list`
  - `navigation_action`
  - `review_summary`
- `<form>...</form>` 仍用于包裹 Markdown 表格。
- `<buyer>` 已废弃，应继续过滤，不进入 UI。
- `<tool_call>`、`<tool_response>`、`<think>` 属于内部标签，应继续过滤，不进入 UI。

Android 端当前问题不在于完全不识别商品，而在于渲染模型仍是“文本整体清洗 + itemIds 汇总 + 最后渲染商品卡”，丢失了 `<item>` 的原始位置。

## 3. 当前 Android 问题定位

### 3.1 `<item>` 位置丢失

当前逻辑：

```java
RenderedMarkdown rendered = sanitizeAgentMarkdown(markdown);
MarkdownRenderer.setMarkdown(activeAssistant, rendered.visibleMarkdown);
renderItemRefs(rendered.itemIds, chatList);
```

`sanitizeAgentMarkdown()` 内部通过 `extractItemRefs()` 提取商品 ID：

```java
value = extractItemRefs(value, itemIds);
```

`extractItemRefs()` 会把所有 `<item>p_xxx</item>` 从正文删除，并只返回一个 `itemIds` 数组。这样会导致：

1. 商品 ID 与正文位置脱钩。
2. `renderItemRefs()` 只能把商品卡 append 到当前气泡末尾。
3. 历史恢复时，如果消息包含多个商品和多段文字，商品卡会集中到底部。
4. 异步商品详情加载完成后也只能替换末尾 holder，无法回到原始 `<item>` 位置。

### 3.2 `content_delta.text` 未作为主协议消费

当前 `handleSse()` 已识别 `content_delta`，但遇到 `part.type = text` 时直接 `return`，依赖旧 `text_delta`：

```java
if ("text".equals(part.optString("type"))) {
    return;
}
```

这在兼容期可运行，但与后端新协议方向不一致。若未来后端减少 `text_delta`，Android 会丢正文。更关键的是，新协议的目标是按 `content_delta` 顺序渲染 `text/product_card/text/product_card`，Android 端继续混用 `text_delta + block_delta` 会天然回到“商品块后置”的老结构。

### 3.3 历史消息缺少统一 content parts

当前历史渲染入口：

```java
renderAgentSegments(jsonArray(message.segmentsJson), message.content, message.blocksJson, chatList);
```

虽然已经引入 `HistoricalMessageRenderContext`，避免商品卡插入错误气泡，但仍没有形成“文本片段 + 商品片段”的统一顺序模型。历史恢复的正确做法应是：

```text
segments/content_parts
  text
  product_card
  text
  product_card
  table
```

而不是：

```text
visibleMarkdown
itemIds/product_refs
blocks
```

## 4. 总体方案

本轮核心改造是新增 Android 端统一内容片段模型：

```text
AgentContentPart
  text
  product_card
  product_ref
  table
  comparison_table
  citation_refs
  warning
  cart_state
  order_summary
  discount_preview
  coupon_list
  navigation_action
  review_summary
```

渲染链路统一为：

```text
实时 SSE:
  content_delta.text / content_delta.product_card
    -> append AgentContentPart
    -> appendContentPartToActiveBubble()

历史恢复:
  segments_json 优先
  blocks_json + content fallback
    -> normalizeHistoricalContentParts()
    -> renderContentPartsToHistoricalBubble()
```

`<item>` 兼容层只负责把原始 Markdown 切成顺序片段：

```text
"推荐 A\n<item>p_1</item>\n说明 A\n<item>p_2</item>\n说明 B"
  -> text("推荐 A")
  -> product_ref("p_1")
  -> text("说明 A")
  -> product_ref("p_2")
  -> text("说明 B")
```

商品卡永远插入在 `product_ref/product_card` 片段所在位置。

## 5. 商品卡片按 `<item>` 位置渲染方案

### 5.1 新增内容片段类

在 `MainActivity` 内新增轻量内部类：

```java
private static class AgentContentPart {
    String type;
    String text;
    String productId;
    JSONObject block;
}
```

类型约定：

| type | 来源 | 渲染 |
| --- | --- | --- |
| `text` | Markdown 正文 | `MarkdownRenderer` |
| `product_ref` | `<item>p_xxx</item>` / `product_refs` | 按 ID 拉取商品详情并渲染卡 |
| `product_card` | `content_delta.product_card` / `block_delta.product_card` | 直接渲染商品卡 |
| `table` | `<form>` 或 Markdown 表格 | 表格组件 |
| `comparison_table` | block | 对比表 |
| `citation_refs` | block | 引用摘要或暂不展示 |
| `warning` | block | warningCard |
| `cart_state` | block | cartStateCard |
| `order_summary` | block | orderSummaryCard |
| `discount_preview` | block | discountPreviewCard |
| `coupon_list` | block | couponListCard |
| `navigation_action` | block | navigationActionCard |
| `review_summary` | block | review summary view |

### 5.2 替换 `extractItemRefs()` 的使用方式

保留 `sanitizeAgentMarkdown()` 过滤内部标签的能力，但商品标签不能再只汇总为数组。

新增：

```java
private List<AgentContentPart> splitMarkdownByItemTags(String markdown)
```

规则：

1. 使用正则识别完整 `<item>...</item>`。
2. 标签前的正文作为 `text` part。
3. 标签内 product_id trim 后作为 `product_ref` part。
4. 标签后的正文继续拆分。
5. 不完整 `<item>` 片段在流式中进入 pending，不提前渲染。
6. 非法 product_id 不渲染商品卡，可过滤为普通空内容，避免显示内部标签。

不要再在实时渲染里调用：

```java
renderItemRefs(rendered.itemIds, chatList);
```

改为：

```java
appendMarkdownPartsToActiveBubble(splitMarkdownByItemTags(deltaOrFinalMarkdown));
```

### 5.3 实时流式策略

优先使用后端新协议：

```text
content_delta.text
content_delta.product_card
content_delta.text
```

处理规则：

- `content_delta.text`：
  - 追加到 active content part 流。
  - 按 `<form>` 和 `<item>` 规则拆分。
  - 文本实时渲染。
- `content_delta.product_card`：
  - 先 flush 当前文本 part。
  - 在当前气泡内立即插入商品卡。
  - 记录到 `activeAssistantSegments`，便于历史恢复。
- `text_delta`：
  - 仅作为旧协议回退。
  - 如果本轮已收到任意 `content_delta.text`，忽略 `text_delta`，避免正文重复。
- `block_delta.product_card/product_refs`：
  - 如果本轮已收到 `content_delta.product_card`，仅写兼容 blocks，不重复渲染。
  - 若未收到 content_delta，则按旧协议渲染，但仍通过 content part 管线插入当前位置。

新增字段：

```java
private boolean activeUsesContentDelta;
private boolean activeRenderedContentText;
private JSONArray activeContentParts = new JSONArray();
```

### 5.4 历史恢复策略

历史恢复优先级：

1. `segments_json` 中的 `text/product_card/block` 顺序。
2. 如果未来后端提供 `content_parts_json`，优先使用它。
3. 如果只有 `content` 和 `blocks_json`：
   - 先用 `splitMarkdownByItemTags(content)` 得到 text/product_ref 顺序。
   - 再只渲染非商品类 blocks。
   - `product_refs` 仅作为兜底：当 content 中没有任何 `<item>` / product_card 时才渲染，避免底部重复堆卡。

历史消息渲染上下文扩展：

```java
private static class HistoricalMessageRenderContext {
    final LinearLayout parent;
    final Set<String> renderedProductIds = new HashSet<>();
    LinearLayout bubble;
}
```

商品去重只在“单条 assistant 消息”内生效，不能用全局 `activeRenderedProductIds`。

### 5.5 异步商品详情加载

`product_ref` 渲染时必须先在当前位置创建 holder：

```java
LinearLayout holder = new LinearLayout(this);
bubble.addView(holder, currentPositionLayoutParams);
```

随后：

- 命中 `productDetailCache`：立即替换 holder 内容。
- 未命中：显示“正在加载商品信息...”，异步请求 `GET /api/v1/products/{product_id}`。
- 请求成功：只替换该 holder。
- 请求失败：只在该 holder 显示 warning，不影响其他商品卡。

严禁异步完成后调用 `bubble.addView(chatProductCard(...))` 追加到底部。

## 6. 后端标签/功能 Android 覆盖清单

### 6.1 必须支持

| 后端输出 | Android 当前状态 | v23 方案 |
| --- | --- | --- |
| `content_delta.text` | 已识别但忽略 | 改为主文本流 |
| `content_delta.product_card` | 已按 block 渲染 | 改为 content part 顺序渲染 |
| `text_delta` | 支持 | 仅旧协议回退 |
| `block_delta.product_card` | 支持 | 兼容回退，避免重复 |
| `block_delta.product_refs` | 支持 | 只作为无 item/content_delta 时兜底 |
| `<item>p_xxx</item>` | 提取但丢位置 | 拆成 `text/product_ref/text` |
| `<form>...</form>` | 支持 | 保持现有表格中间态 |
| `<buyer>` | 应过滤 | 保持过滤或新增显式过滤 |
| `<tool_call>`/`<tool_response>`/`<think>` | 已过滤 | 保持 |

### 6.2 应补齐或明确降级

| 后端输出 | v23 处理 |
| --- | --- |
| `citation_refs` | 默认不显示；后续可按 chunk id 拉取引用摘要。当前避免展示“暂不支持”污染聊天。 |
| `citation` | 若有 citation 对象，渲染 citationCard。 |
| `navigation_action` | 渲染行动按钮，点击跳转商品/购物车/订单。 |
| `review_summary` | 不再只显示“评价摘要：rating”，改为包含评分、数量、摘要文本的轻量卡片。 |
| `comparison_table` | 继续使用 comparisonCard。 |
| `cart_state` | 继续使用 cartStateCard。 |
| `order_summary` | 继续使用 orderSummaryCard。 |
| `discount_preview` | 继续使用 discountPreviewCard。 |
| `coupon_list` | 继续使用 couponListCard。 |
| 未知 block type | 只在 debug 模式显示；正式 UI 不直接展示“暂不支持的内容类型”。 |

## 7. 语音输入方案调研与设计

### 7.1 方案 A：Android `SpeechRecognizer`

当前已接入方向，但在模拟器或部分设备上可能不可用，原因包括：

- 系统没有语音识别服务。
- Google/厂商语音服务不可用。
- 网络识别失败。
- 权限或麦克风占用。

继续保留作为首选方案：

```java
SpeechRecognizer.createSpeechRecognizer(this)
recognizer.startListening(RecognizerIntent)
```

优点：

- 不新增 SDK。
- Android 原生能力，集成成本低。
- 支持 partial results。

缺点：

- 依赖系统语音服务。
- 模拟器稳定性一般。
- 离线能力取决于设备。

### 7.2 方案 B：`RecognizerIntent` 系统语音界面

当 `SpeechRecognizer.isRecognitionAvailable(this)` 为 false，或连续错误时，降级到：

```java
Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
```

优点：

- 交给系统语音 UI 处理。
- 兼容一部分不支持后台识别但支持系统语音输入的设备。

缺点：

- 会跳出 APP 内联体验。
- 无法实时显示 partial result。

### 7.3 方案 C：输入法语音按钮

输入框保持普通 `EditText`，允许用户使用系统输入法自带语音输入。

优点：

- 零开发成本。
- 用户熟悉。

缺点：

- 不是 APP 自带语音按钮。
- 无法控制识别完成后自动发送。

v23 中应作为兜底说明，不作为主实现。

### 7.4 方案 D：云端语音识别 SDK

可选供应商：

- 讯飞开放平台语音听写。
- 阿里云智能语音交互。
- 腾讯云 ASR。
- 百度智能云语音识别。

推荐条件：

- 如果必须在比赛/演示环境稳定可用，且模拟器/设备系统语音不可控，则接入云端 SDK。
- 后端可新增短音频识别接口，Android 只负责录音上传，避免把云端 key 放到客户端。

建议架构：

```text
Android AudioRecord/MediaRecorder
  -> wav/aac 临时文件
  -> POST /api/v1/speech:recognize
  -> 后端调用云 ASR
  -> 返回 text
  -> Android sendMessage(text)
```

安全要求：

- 云服务 app key/secret 只放后端。
- Android 不保存长期音频。
- 请求失败给出 toast 并恢复输入态。

### 7.5 v23 推荐实现顺序

1. 保留并修正 `SpeechRecognizer` 内联识别。
2. 增加 `RecognizerIntent` 降级。
3. 在高级设置中增加“语音识别模式”：
   - 自动
   - 系统内联
   - 系统语音界面
   - 云端识别（后端接口存在时启用）
4. 若后端没有 `/api/v1/speech:recognize`，Android 云端识别入口置灰并提示“当前后端未启用云端语音识别”。

## 8. 数据持久化设计

### 8.1 segments_json 扩展

现有 `segments_json` 可继续使用，但需要写入更精确的顺序片段：

```json
[
  {"type":"text","text":"这款适合补妆："},
  {"type":"product_card","product":{...}},
  {"type":"text","text":"它的优势是..."}
]
```

如果只有 `<item>` ID，没有完整 product：

```json
[
  {"type":"text","text":"这款适合补妆："},
  {"type":"product_ref","product_id":"p_beauty_001"},
  {"type":"text","text":"它的优势是..."}
]
```

### 8.2 兼容旧数据

旧数据可能只有：

```json
content = "..."
blocks_json = [{"type":"product_refs","product_ids":[...]}]
segments_json = []
```

兼容规则：

1. 如果 `segments_json` 为空，先拆 `content`。
2. 如果 content 无 `<item>`，再渲染 `product_refs`。
3. 如果 content 有 `<item>`，跳过底部 `product_refs`，避免重复。

## 9. 实施步骤

### Phase 1：内容片段模型

- 新增 `AgentContentPart`。
- 新增 `splitMarkdownByItemTags()`。
- 新增 `appendContentPartToBubble()`。
- 新增 `renderProductRefIntoHolder()`。

### Phase 2：实时 SSE 重构

- `content_delta.text` 改为主渲染入口。
- `content_delta.product_card` 按顺序插入。
- `text_delta/block_delta` 降级为兼容入口。
- 增加 `activeUsesContentDelta`，避免重复渲染。

### Phase 3：历史恢复重构

- `renderAgentSegments()` 改为先 normalize content parts。
- `product_refs` 只作为兜底。
- 异步商品详情只替换当前位置 holder。

### Phase 4：后端功能覆盖补齐

- 增强 `citation_refs`、`review_summary`、`navigation_action` 的 Android 展示策略。
- 未知 block 在正式 UI 静默降级，debug 模式才显示 warning。

### Phase 5：语音输入增强

- 修正现有 `SpeechRecognizer` 状态机。
- 增加 `RecognizerIntent` fallback。
- 预留后端云端 ASR 接口检查和入口置灰。

### Phase 6：虚拟机验证

必须在模拟器中验证：

1. 新会话 `cosmetics` 能实时出现商品卡片。
2. 从历史点击同一会话，商品卡片仍出现在对应文字附近。
3. 多商品回答不再集中堆到底部。
4. 商品详情异步加载完成后不改变商品卡位置。
5. 没有重复商品卡。
6. `logcat` 无 `FATAL EXCEPTION`、`AndroidRuntime` APP 崩溃。
7. 语音按钮在模拟器无语音服务时能降级或明确提示。

## 10. 验收标准

- 商品卡片严格按 `<item>` 或 `content_delta.product_card` 的顺序渲染。
- 从历史返回对话时，商品卡不会集中显示在 assistant 气泡底部。
- 旧历史数据仍可显示商品，不丢卡。
- `content_delta.text` 可独立驱动正文渲染；即使后端未来减少 `text_delta`，Android 仍可显示正文。
- `product_refs` 不再造成重复底部挂卡。
- `<buyer>`、`<think>`、`<tool_call>`、`<tool_response>` 不出现在用户 UI。
- 语音输入至少支持系统内联识别和系统语音界面 fallback；不可用时有明确提示。

