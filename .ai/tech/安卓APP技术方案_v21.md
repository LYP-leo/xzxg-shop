# 安卓 APP 技术方案 v21

## 1. 背景

本文针对 [安卓APP_问题_v20.md](./安卓APP_问题_v20.md) 设计下一版 Android 原生 APP 修复方案。

本轮只处理聊天页渲染问题：

- 从历史记录进入对话时，商品卡片可能消失、位置错误或多个卡片挤在一起。
- 商品卡片后面出现多余句号。
- Markdown 表格单元格内的 `**加粗**` 等格式没有被渲染。
- 标题需要增加颜色层级。
- 后端会用 `<form></form>` 包裹表格，Android 需要在表格片段完整后再渲染表格；普通文本仍要实时渲染。

本方案只设计 Android 端改造。后端协议以 [Agent输出协议_v1.md](../api/Agent输出协议_v1.md) 和 `backend/README.md` 为准。

## 2. 协议边界

### 2.1 SSE 主协议

后端聊天接口：

```http
POST /api/v1/agent/sessions/{session_id}/messages:stream
Accept: text/event-stream
Content-Type: application/json
Authorization: Bearer <token>
```

Android 端只按事件类型分发：

| SSE 事件 | Android 动作 |
| --- | --- |
| `message_start` | 记录 `run_id` |
| `status` | 更新临时 loading 文案，不保存进最终回答 |
| `text_delta` | 进入流式文本解析器 |
| `content_delta` | 兼容实际后端包装；`part.type=text` 不重复渲染，非 text block 按结构化 block 处理 |
| `block_delta` | 按 `block.type` 渲染结构化内容 |
| `followups` | 更新推荐追问 |
| `message_end` | flush 未完成片段，保存本轮消息 |
| `error` | 展示错误或取消状态 |

正文主链路仍然是 `text_delta`。如果同一段文字同时出现在 `content_delta.part.type=text` 和 `text_delta`，Android 必须只渲染 `text_delta`，避免重复。

### 2.2 商品卡协议

商品卡只能来自结构化 block，不从自然语言中猜测商品。

协议主链路：

```json
{
  "type": "product_refs",
  "product_ids": ["p_001", "p_002"]
}
```

Android 收到后按 `product_ids` 调商品详情接口并渲染卡片。

兼容旧链路：

```json
{
  "type": "product_card",
  "product": {
    "productId": "p_001",
    "name": "X Phone 12",
    "price": "2999.00"
  }
}
```

Android 可直接渲染 `product`。

注意：

- `product_refs` 字段名是 `product_ids`，不是 `ids`、`items`、`products`。
- `product_card` 字段名是 `product`。
- `content_delta.part.product_card` 不是协议主链路，但当前 Android 可以作为兼容包装处理，内部仍按 `product_card.product` 规则渲染。
- `<item>p_xxx</item>` 只作为文本引用清理和兜底引用，不作为商品卡主链路。

## 3. 总体方案

```text
SSE
  text_delta
    -> FormAwareMarkdownStream
       TEXT: 普通文本实时 Markdown 渲染
       FORM_TABLE: <form> 后创建原文占位，缓冲表格源码

  block_delta/content_delta 非 text block
    -> flush 当前 TEXT
    -> 同一个 assistant 气泡内插入商品卡/引用/结构化表格
    -> 保存 segment

  message_end
    -> flush TEXT
    -> 未闭合 FORM_TABLE 降级为原文显示
    -> 保存本轮 assistant turn

历史恢复
  每条 assistant message 独立 RenderContext
  按 segments 顺序恢复 text/table/block/followups
```

核心原则：

1. 普通文本实时渲染，不能因为表格缓冲而延迟整轮回答。
2. `<form>` 只影响标签内部的表格片段。
3. 商品卡必须渲染在当前 assistant 对话框内部，不允许落到对话框下面。
4. 历史恢复必须使用每条消息自己的上下文，不能复用流式全局变量。

## 4. 商品卡片修复方案

### 4.1 流式渲染中的商品卡位置

当前流式状态需要保留一个“本轮 assistant 气泡容器”：

```java
LinearLayout activeAssistantMessageBubble;
TextView activeAssistantTextView;
StringBuilder activeAssistantMarkdown;
JSONArray activeAssistantSegments;
Set<String> activeRenderedProductIds;
```

处理规则：

1. 普通文本进入 `activeAssistantTextView`。
2. 收到商品 block 前，先 `flushActiveTextSegment(false)`，把当前文本保存为 text segment，但不拆表格。
3. 调用 `ensureActiveAssistantMessageBubble(chatList)` 获取当前 assistant 气泡。
4. 商品卡 `bubble.addView(chatProductCard(product))`，直接插入同一个白色 assistant 气泡内部。
5. 保存 block segment：

```json
{"type":"block","block":{"type":"product_refs","product_ids":["p_001"]}}
```

或：

```json
{"type":"block","block":{"type":"product_card","product":{}}}
```

### 4.2 `product_refs` 渲染

`product_refs` 需要稳定 slot，避免异步返回顺序导致卡片乱序：

```text
product_refs block
  slot p_001: loading -> card/error
  slot p_002: loading -> card/error
```

实现要求：

- 先按 `product_ids` 顺序创建 slot。
- 每个 slot 异步请求商品详情。
- 商品详情成功时只替换自己的 slot。
- 请求失败时在 slot 内展示“商品信息加载失败”，不能只 toast。
- 去重只限定在当前 assistant turn 内，同一商品 ID 在本轮只展示一次。
- 进入新会话、新一轮回答、历史消息恢复时，重新创建独立去重集合。

### 4.3 `product_card` 渲染

`product_card` 直接使用 `product` 对象渲染：

```java
JSONObject product = block.optJSONObject("product");
```

字段兼容：

| 展示字段 | 优先字段 |
| --- | --- |
| 商品 ID | `productId`、`product_id`、`id` |
| 名称 | `name` |
| 图片 | `imageUrl`、`image_url`、`imageUrls[0]` |
| 价格 | `price` |
| 卖点 | `recommendReason`、`recommend_reason`、`sellingPoints` |

商品卡点击和加购沿用现有商品详情、购物车接口。

### 4.4 历史恢复中的商品卡

历史恢复不能调用流式全局 `ensureActiveAssistantMessageBubble()` 直接复用 `activeAssistantMessageBubble`，否则多条历史消息会共享同一个容器。

新增局部上下文：

```java
class MessageRenderContext {
    LinearLayout bubble;
    Set<String> renderedProductIds;
}
```

历史消息恢复流程：

```text
renderAssistantMessage(message):
  ctx = new MessageRenderContext()
  for segment in message.segments:
    text -> renderHistoricalTextSegment(ctx, text)
    table -> renderHistoricalTableSegment(ctx, markdown)
    block -> renderHistoricalBlockSegment(ctx, block)
  followups -> 渲染在该消息后
```

禁止历史恢复期间复用：

- `activeAssistantMessageBubble`
- `activeAssistant`
- `activeRenderedProductIds`

这些字段只属于当前正在流式输出的一轮。

## 5. 商品卡后多余句号

多余句号通常来自文本中的 `<item>` 占位符：

```markdown
推荐 **X Phone 12** <item>p_001</item>。
```

清理 `<item>` 后会留下孤立 `。`。处理方案：

1. 在 `sanitizeAgentMarkdown()` 的 `extractItemRefs()` 中清理 `<item>` 前后的孤立标点。
2. 只清理紧邻 `<item>...</item>` 的标点，不处理普通句子的标点。
3. 清理后做空白归一化。

建议正则：

```java
Pattern.compile("\\s*[。\\.，,；;：:、]?\\s*<item>([^<]+)</item>\\s*[。\\.，,；;：:、]?\\s*")
```

替换为一个空格，再做：

```java
.replaceAll("[ \\t]+\\n", "\n")
.replaceAll("\\n[ \\t]+", "\n")
.replaceAll("[ \\t]{2,}", " ")
```

## 6. 表格 Markdown 渲染修复

### 6.1 根因

当前 `markdownTableView()` 对表格单元格使用：

```java
cell.setText(cells.get(i));
```

所以 `**加粗**`、链接、行内代码等 Markdown 会原样显示。

### 6.2 单元格改走 MarkdownRenderer

改为：

```java
MarkdownRenderer.setMarkdown(
    cell,
    sanitizeAgentMarkdown(cells.get(i)).visibleMarkdown,
    13
);
```

要求：

- 表头保持加粗。
- 单元格支持 `**加粗**`、`*斜体*`、行内代码、链接。
- 单元格仍使用固定列宽和横向滚动，不撑宽整个 assistant 气泡。
- 表格背景、边框、padding 保持可读。

### 6.3 行高一致

表格每一行内所有列必须等高。实现 `EqualHeightTableRow extends LinearLayout`：

```text
第一次 measure 得到每个 child 高度
取 maxHeight
第二次以 maxHeight EXACTLY 重新 measure 每个 child
row measuredHeight = maxHeight
```

单元格 `LayoutParams.height` 使用 `MATCH_PARENT`，这样长文本列变高后，同一行其它列背景也撑满。

### 6.4 表格横向滚动范围

表格只允许自身横向滚动：

```text
assistant bubble
  text
  HorizontalScrollView
    table
  text
```

禁止：

- 把整个 assistant 气泡设置成超宽。
- 让表格撑破聊天页宽度。
- 把表格直接放进普通 TextView 后交给 Markwon 自动撑宽。

## 7. 标题颜色方案

`MarkdownRenderer` 继续集中处理标题样式，不在各调用点重复实现。

方案：

1. 保留 Markwon 渲染 Markdown。
2. 对原始 Markdown 中 `#`、`##`、`###` 行做后处理 span。
3. 给不同层级设置颜色和字号倍率。

建议颜色：

| 标题 | 颜色 | 字号倍率 |
| --- | --- | --- |
| `#` | `#0F4C81` | `1.20` |
| `##` | `#1F6AA5` | `1.08` |
| `###` | `#2E7D6B` | `1.00` |

注意：

- 不要用整段大号字体破坏聊天密度。
- 表格单元格内如果出现标题语法，应按普通单元格文本降级处理，避免单元格行高异常。

## 8. `<form></form>` 表格流式方案

### 8.1 目标行为

用户要求：

```text
接收到 <form> 标签：
  新建一个 plain text 原文占位框
  在占位框中实时输出标签内部收到的原始内容

接收到 </form> 标签：
  删除 plain text 原文占位框
  只渲染 <form> 与 </form> 之间的表格

普通文本：
  不受影响，继续实时渲染
```

这意味着不能等整轮 `message_end`，也不能在表格每一行收到时提前渲染最终表格。

### 8.2 状态机

新增流式解析状态：

```java
enum StreamMode {
    TEXT,
    FORM_TABLE
}

StreamMode streamMode;
StringBuilder pendingTagBuffer;
StringBuilder activeFormRawMarkdown;
LinearLayout activeFormPlaceholder;
TextView activeFormRawText;
```

`TEXT` 状态：

- 普通 delta 追加到当前文本 TextView。
- 需要保留 `<form>` 的可能部分匹配，例如本次 delta 末尾是 `<fo`。
- 检测到完整 `<form>`：
  - `<form>` 前的文本进入普通实时渲染。
  - flush 当前 text segment。
  - 创建 `plain text` 占位框。
  - 切换到 `FORM_TABLE`。

`FORM_TABLE` 状态：

- 标签内部内容只追加到 `activeFormRawMarkdown`。
- 同步更新 `activeFormRawText`，显示原始 Markdown。
- 需要保留 `</form>` 的可能部分匹配，例如本次 delta 末尾是 `</fo`。
- 检测到完整 `</form>`：
  - 用完整 `activeFormRawMarkdown` 渲染表格。
  - 删除原文占位框。
  - 保存 table segment。
  - 切回 `TEXT`，继续处理 `</form>` 后面的文本。

### 8.3 占位框样式

占位框不是最终表格，只是表格源码流式过程中的临时态：

```text
assistant bubble
  TextView: 普通文本
  LinearLayout code-like box
    TextView: plain text
    TextView: 原始表格 Markdown
```

样式：

- 背景 `#F6F8FA`
- 圆角 8-10dp
- header 文案 `plain text`
- 内容使用 monospace 字体
- 允许自动换行，不需要横向滚动

### 8.4 完成后替换为表格

收到 `</form>` 后：

1. 从父容器移除 `activeFormPlaceholder`。
2. 解析 `activeFormRawMarkdown`。
3. 如果包含合法 Markdown 表格，调用 `addMarkdownTableToBubble()`。
4. 如果不是合法表格，降级为普通 Markdown 文本，避免内容丢失。
5. 保存 segment：

```json
{"type":"table","markdown":"| 商品 | 价格 |\n| --- | --- |\n| X Phone | ¥2999 |"}
```

注意：

- table segment 不保存 `<form>` 和 `</form>`。
- `activeAssistantFullMarkdown` 中可以保存原始表格 Markdown，不能保存占位框 UI 文案。
- 如果 `message_end` 时 `<form>` 未闭合，不强制渲染表格；保留原文占位或降级为普通 code-like 文本，并保存为 text segment。

### 8.5 与普通文本实时渲染的关系

示例输入：

```text
这是推荐理由：
<form>
| 商品 | 价格 |
| --- | --- |
| X Phone | ¥2999 |
</form>
表格之后继续说明。
```

Android 渲染顺序：

```text
实时显示：这是推荐理由：
收到 <form>：显示 plain text 源码框
form 内持续更新源码框
收到 </form>：源码框替换为最终表格
实时显示：表格之后继续说明。
```

## 9. 历史消息 segments 设计

当前历史恢复必须优先使用 `segments_json`，不要只用 `content` 重跑流式逻辑。

建议 segment 类型：

```json
{"type":"text","text":"普通 Markdown 文本"}
{"type":"table","markdown":"| A | B |\n| --- | --- |\n| 1 | 2 |"}
{"type":"block","block":{"type":"product_refs","product_ids":["p_001"]}}
```

恢复规则：

- `text`：用 `splitMarkdownParts()` 拆普通文本和普通 Markdown 表格。
- `table`：直接渲染为表格。
- `block.product_refs`：按商品详情恢复卡片。
- `block.product_card`：直接恢复卡片。
- 未知 block：忽略并记录日志，不能让页面崩溃。

如果旧消息没有 `segments_json`：

- 使用 `content` 作为普通 Markdown 渲染。
- `<item>` 只清理占位符和孤立标点。
- 不从自然语言推断商品卡。

## 10. 实施步骤

1. 新增 `FormAwareMarkdownStream` 或在 `MainActivity` 中实现等价状态机。
2. 改造 `appendAssistant(delta)`：
   - `TEXT` 实时渲染。
   - `<form>` 进入 `FORM_TABLE`。
   - `</form>` 后立即替换为最终表格。
3. 商品 block 渲染改为显式传入当前 `MessageRenderContext`：
   - 流式用 active context。
   - 历史用局部 context。
4. `product_refs` 引入稳定 slot。
5. `extractItemRefs()` 清理 `<item>` 周围孤立标点。
6. `markdownTableView()` 单元格改用 `MarkdownRenderer.setMarkdown(cell, ..., 13)`。
7. 实现 `EqualHeightTableRow`。
8. `MarkdownRenderer` 保持标题颜色后处理。
9. 构建并在模拟器验证流式、历史恢复、商品卡和表格。

## 11. 测试计划

### 11.1 单轮流式

测试输入：

```text
推荐几款拍照手机，并输出一个对比表。
```

验收：

- 普通文本实时出现。
- 商品卡出现在 assistant 气泡内部。
- 商品卡后没有孤立句号。
- 表格最终渲染为表格，不显示 `<form>` 标签。

### 11.2 `<form>` 表格

使用后端真实流或调试后端输出：

```text
普通说明 <form>
| 商品 | 价格 | 推荐 |
| --- | --- | --- |
| **X Phone 12** | ¥2999 | **拍照好** |
</form> 后续说明
```

验收：

- 收到 `<form>` 后出现 `plain text` 原文占位框。
- `</form>` 前不渲染最终表格。
- 收到 `</form>` 后原文占位框消失，出现最终表格。
- `**X Phone 12**` 和 `**拍照好**` 在表格内加粗显示。
- 表格每行各列高度一致。
- “后续说明”继续实时渲染。

### 11.3 历史恢复

步骤：

1. 发送一次包含商品卡和表格的对话。
2. 退出当前会话。
3. 从侧栏历史进入该会话。

验收：

- 商品卡仍在对应 assistant 气泡内部。
- 商品卡没有挤在一起。
- 表格仍正常显示。
- 追问按钮仍显示在对应回答后。

### 11.4 日志和异常

检查 logcat：

```text
AndroidRuntime
FATAL EXCEPTION
发送失败
AgentBlock
```

验收：

- 无崩溃。
- 无 `发送失败：null`。
- 商品 block 日志字段与协议一致。

## 12. 风险与降级

### 12.1 后端未输出 `<form>`

如果真实后端没有输出 `<form>`，Android 仍按普通 Markdown 渲染，不影响正常对话。

### 12.2 `<form>` 未闭合

如果收到 `message_end` 仍未见 `</form>`：

- 不渲染最终表格。
- 保留已经显示的原文占位，或降级为普通代码样式文本。
- 保存为 text segment，避免历史恢复丢内容。

### 12.3 表格内容不是合法 Markdown 表格

如果 `<form>` 内不是合法表格：

- 删除 `<form>` 标签。
- 按普通 Markdown 文本显示内容。
- 不丢弃用户可见内容。

### 12.4 商品详情接口失败

商品卡 slot 显示失败态：

```text
商品信息加载失败
```

并允许用户继续阅读文本，不阻塞整轮回答。

## 13. 不做事项

- 不从自然语言商品名推断商品卡。
- 不新增后端接口。
- 不修改 `product_refs.product_ids` 字段名。
- 不把 `<form>` 当 HTML 表单渲染。
- 不等待整轮回答结束后才渲染已闭合的 `<form>` 表格。
- 不把商品卡渲染到 assistant 气泡外面。

