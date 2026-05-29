# 安卓 APP 技术方案 v20

## 1. 背景

本文针对 [安卓APP_问题_v19.md](./安卓APP_问题_v19.md) 设计下一版 Android 原生 APP 修复方案。

本轮问题不是重新设计全部聊天系统，而是修复 v19 落地后的几个明确缺陷：

- 聊天页：
  - 商品卡片依旧没有渲染出来。
  - 表格、代码等结构化片段渲染时机仍然偏晚，应在该片段流式完成后立即渲染，而不是等整轮回答结束。
  - 表格视觉不够好，底部文字与边框距离过近。
  - 聊天内容不能长按复制；AI 回复复制结果应是原始 Markdown。
  - “提示 已记录引用来源”没有用户价值，应移除或改为可理解的引用展示。
- 侧栏：
  - 历史聊天只加载固定数量，不能滚动加载更早历史。期望初始 20 条，滑到底部再加载 20 条。
- 应用图标：
  - `pig.svg` 显示不全，宽度被裁剪。需要完整显示宽度，高度多余区域透明，并保持垂直居中。

## 2. 关键判断

### 2.1 商品卡不显示的优先排查方向

后端协议仍以 [Agent输出协议_v1.md](../api/Agent输出协议_v1.md) 为准：

- 商品卡数据必须通过 `block_delta` 输出。
- 主链路优先输出：

```json
{"type":"product_refs","product_ids":["p_001"]}
```

- 兼容旧链路：

```json
{"type":"product_card","product":{...}}
```

如果 Android 端没有看到商品卡，不能只改 UI，需要先确认 SSE 里到底有没有 `product_refs` 或 `product_card`：

1. 在 Android `handleSse()` 中记录 `block_delta` 原始 JSON。
2. 在后端 `streamAgentRun()` 或 runtime emit 处确认最终是否发出 `product_refs`。
3. 如果后端仅在文本中描述商品，而没有 block，则 Android 不应从文本猜商品卡；应修后端 Agent 输出。
4. 如果后端发了 block，但 Android 没渲染，则修 Android block 分发、segments 保存和商品详情请求。

v20 的商品卡修复策略必须分两层：

- Android：保证任何合法 `product_refs/product_card` 都能立即渲染。
- 后端：若没有发合法 block，需要补齐 Agent 输出或运行时转换。

### 2.2 表格渲染时机

用户明确要求：

```text
不是等整条对话结束后统一渲染。
而是表格内容流式输出完成后，立即渲染表格。
代码等结构化内容同理。
```

因此 v20 需要引入“流式片段提交”机制：

- 普通文本：继续实时渲染。
- 表格：检测到完整表格边界后立即提交表格 segment 并渲染。
- 代码块：检测到闭合 ``` 后立即提交代码 segment 并渲染。
- 公式：检测到闭合 `$$` / `\]` / `\)` 后立即提交公式 segment 并渲染。
- `message_end` 只 flush 未完成片段，不负责重渲染已完成片段。

### 2.3 引用提示

`citation_refs` 只有 `chunk_ids`，没有 title/snippet/source。直接展示“提示 已记录引用来源”对用户没有意义。

处理原则：

- `citation`：有完整 `citation` 对象时，展示可折叠引用卡。
- `citation_refs`：默认不展示 UI，只记录到本轮 turn 的元数据。
- 如果未来后端提供 chunk 详情查询接口，再将 `citation_refs.chunk_ids` 转成可读引用列表。

## 3. 总体方案

```text
SSE:
  message_start -> 初始化 turn 状态
  text_delta -> 进入流式片段解析器
  block_delta -> 立即按当前位置渲染 block，并保存 segment
  followups -> 追问区更新
  message_end -> flush 未闭合片段，保存 turn

片段解析器:
  普通文本实时渲染
  表格/代码/公式独立缓冲
  片段闭合时立即替换成最终组件

商品卡:
  Android 端强制支持 product_refs/product_card
  后端端到端确认 product_refs 是否真实输出
  商品详情请求失败在原位显示失败卡

历史侧栏:
  本地/远端历史分页统一 PageState
  初始加载 20 条
  滑到底部加载下一页 20 条

复制:
  长按用户/AI 气泡弹出复制
  AI 复制原始 Markdown
  表格/代码/商品 block 复制可读 Markdown

图标:
  pig.svg 放入更宽 viewport
  透明背景
  preserve aspect ratio，完整显示宽度
  垂直居中
```

## 4. 聊天页方案

### 4.1 SSE 调试与商品卡链路确认

新增调试日志，仅在 debug 构建或本地开关下启用：

```java
private static final boolean DEBUG_AGENT_BLOCKS = true;
```

在 `handleSse()` 中：

```java
if ("block_delta".equals(type)) {
    Log.d("AgentBlock", event.optJSONObject("block").toString());
}
```

后端也要确认 emit：

```go
case "block_delta":
    logger.Debug("agent block", "block", event.Block)
```

验收时必须能回答：

- 本轮商品推荐问题是否收到 `block_delta`？
- block type 是否为 `product_refs`？
- `product_ids` 是否非空？
- Android 是否调用了 `GET /products/{product_id}`？
- 商品详情接口是否返回 200？

### 4.2 Android 商品卡兜底修复

`renderAgentBlock()` 对商品块只接受协议字段：

```java
if ("product_refs".equals(type)) {
    JSONArray ids = block.optJSONArray("product_ids");
    renderProductRefs(ids, parent);
    return;
}

if ("product_card".equals(type)) {
    JSONObject product = block.optJSONObject("product");
    renderProductCard(product, parent);
    return;
}
```

`renderProductRefs()` 改为为每个商品 ID 建固定 slot：

```text
product_refs holder
  slot p_001: loading -> card/error
  slot p_002: loading -> card/error
```

要求：

- holder 立即插入当前 block 位置。
- 商品详情异步回来后只更新对应 slot。
- 不因返回顺序改变卡片顺序。
- 去重只在当前 assistant turn 内生效，不能跨会话全局去重。
- 商品详情失败时 slot 中展示“商品信息加载失败”，而不是只 toast。

### 4.3 后端商品 block 输出修复条件

如果排查发现后端没有发 `product_refs`，则后端需要修复：

- `search_products` 工具返回 `product_ids` 后，最终 Agent run 必须在 `block_delta` 中输出：

```json
{
  "type": "product_refs",
  "product_ids": ["p_digital_001"]
}
```

- Runtime 只允许已通过相关性过滤的商品进入 `product_refs`。
- 禁止只在自然语言中写商品名称而不发 block。
- `<item>` 只作兼容，不作为主链路。

## 5. 流式片段即时渲染方案

### 5.1 引入 StreamSegmentParser

新增解析器状态：

```java
enum SegmentKind {
    TEXT,
    TABLE,
    CODE_BLOCK,
    BLOCK_MATH,
    INLINE_MATH
}

class StreamSegmentParser {
    SegmentKind currentKind;
    StringBuilder textBuffer;
    StringBuilder segmentBuffer;
    TextView activeTextView;
}
```

核心规则：

- `TEXT` 状态下，普通文本持续更新当前 TextView。
- 一旦识别到表格/代码/公式开始，先 flush 当前文本。
- 结构化片段进入 `segmentBuffer`。
- 一旦片段闭合，立即调用对应 renderer，并清空 buffer。

### 5.2 Markdown 表格边界

表格开始：

```text
当前行包含 |
下一行是 separator 行：| --- | --- |
```

表格结束：

```text
后续第一行不再是 table row
或收到空行
或收到 message_end
```

结束时立即：

```text
flush table segment
renderMarkdownTable()
保存 segment 到 activeAssistantSegments
后续文本进入新 TextView
```

这能满足“表格内容流式输出完成之后直接渲染”。

### 5.3 代码块边界

代码块开始：

```text
``` 或 ```java / ```json / ```text
```

代码块结束：

```text
下一组三个反引号
```

闭合后立即渲染代码块：

- 等宽字体。
- 浅色背景。
- 右上角复制按钮或长按复制。
- 横向滚动。

### 5.4 公式边界

支持：

- 块公式：`$$...$$`、`\[...\]`
- 行内公式：`\(...\)`

Android 当前若没有真正 LaTeX renderer，先按等宽/弱样式展示完整公式，不在未闭合时反复 Markdown 渲染。

### 5.5 message_end 行为

`message_end` 只做：

1. flush 当前未完成结构。
2. 保存 `segments_json`。
3. 渲染 followups。
4. 清理状态。

禁止：

- 清空整条 AI 回复再重渲染。
- 等整轮对话结束后才渲染已闭合的表格/代码。

## 6. 表格美化方案

### 6.1 渲染组件

Markdown 表格不再直接用普通 `TextView` 显示源码。改为解析成：

```java
TableLayout / LinearLayout rows
```

结构：

```text
HorizontalScrollView
  LinearLayout table
    header row
    body row
```

### 6.2 视觉规范

- 外层圆角：8dp。
- 边框颜色：`#E5E7EB`。
- header 背景：`#F3F4F6`。
- body 背景：白色。
- 单元格 padding：
  - 左右 12dp。
  - 上下 10dp。
  - 最后一行 bottom padding 也必须是 10dp，不能贴边。
- 行分隔线：1px。
- 单元格最小宽度：110-140dp，根据列数决定。
- 文字：
  - header 加粗。
  - body 13-14sp。
  - 支持多行，不能因为长内容撑爆布局。

### 6.3 表格复制

长按表格区域复制原始 Markdown 表格，不复制 UI 渲染后的拼接文本。

## 7. 聊天内容复制方案

### 7.1 气泡长按行为

所有聊天气泡和结构化片段都添加：

```java
setOnLongClickListener()
```

弹出菜单：

```text
复制
```

后续可扩展：

```text
复制为纯文本
复制为 Markdown
```

### 7.2 用户消息复制

用户消息复制原始输入文本：

```text
message.content
```

### 7.3 AI 回复复制

AI 回复复制原始 Markdown：

```text
segment.type=text -> segment.text
segment.type=table -> 原始 markdown table
segment.type=code -> 原始 fenced code
segment.type=block product_refs/product_card -> 可读 Markdown 摘要
```

商品卡复制格式：

```markdown
### X Phone 12

- 品牌：X
- 商家：小猪数码旗舰店
- 价格：¥2999.00
- 推荐理由：抓拍和对焦能力适合日常拍照。
```

### 7.4 Android 实现

使用系统剪贴板：

```java
ClipboardManager clipboard = (ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
clipboard.setPrimaryClip(ClipData.newPlainText("chat", markdown));
toastLine("已复制");
```

## 8. 引用提示修复

### 8.1 `citation_refs`

当前“提示 已记录引用来源”应删除。

处理：

```java
if ("citation_refs".equals(type)) {
    storeCitationRefs(block.optJSONArray("chunk_ids"));
    return; // 不渲染 UI
}
```

### 8.2 `citation`

只有完整 citation 才展示：

```json
{
  "type": "citation",
  "citation": {
    "chunkId": "ck_xxx",
    "title": "售后政策",
    "snippet": "...",
    "source": "doc_xxx"
  }
}
```

展示为折叠的“引用来源”区域，不影响主回答阅读。

## 9. 侧栏历史分页方案

### 9.1 数据源

本地 SQLite 已保存会话和消息；远程接口文档支持：

```text
GET /agent/sessions?page=1&page_size=20
```

Android 需要统一分页状态：

```java
class HistoryPageState {
    int page = 1;
    int pageSize = 20;
    boolean loading;
    boolean hasMore = true;
    String query = "";
}
```

### 9.2 初始加载

打开侧栏：

```text
清空列表
page=1
加载 20 条
```

展示：

- loading 行。
- 空状态。
- 错误重试。

### 9.3 滚动到底加载更多

监听 `ScrollView`：

```java
if (scrollY + height >= childHeight - dp(80)) {
    loadNextHistoryPage();
}
```

每次加载：

```text
page += 1
append 新数据
不足 20 条 -> hasMore=false
```

注意：

- 删除某条历史后，不应靠“删除后露出更久远记录”来补位。
- 删除后如果当前可见数量少于 20 且 hasMore=true，可以自动补拉下一页。

### 9.4 搜索分页

如果搜索接口支持分页，则按 `q/page/page_size` 加载。

如果当前后端搜索接口不支持分页：

- 本地搜索可分页。
- 远程搜索暂保持单页，并在方案里标记后端需扩展。

## 10. 图标裁剪修复

### 10.1 问题原因

`pig.svg` 原始 viewBox 是 `128x128`，图形横向内容接近边界。直接放进 adaptive icon 前景时，系统会再套安全区，导致左右被裁剪。

### 10.2 资源策略

生成新的前景 vector：

```text
viewportWidth = 160
viewportHeight = 128
```

把原始 pig 图形放在横向居中位置：

```text
translateX = 16
translateY = 0
scale = 1
```

这样：

- 原始宽度 128 完整显示。
- 左右各 16 透明边距。
- 高度仍 128。
- 视觉垂直居中。

如果仍被系统 launcher 安全区裁剪，则改为：

```text
viewportWidth = 192
viewportHeight = 160
scale = 0.82
translateX / translateY 居中
```

### 10.3 adaptive icon

背景保持透明：

```xml
<background android:drawable="@android:color/transparent" />
```

前景使用完整宽度 pig vector。

同时 fallback mipmap icon 也要使用同一缩放策略，避免旧系统仍裁剪。

## 11. 验收清单

### 11.1 商品卡

- 发起能触发商品推荐的问题。
- 后端日志能看到 `block_delta product_refs`。
- Android 日志能看到 `product_refs.product_ids`。
- Android 调用 `GET /products/{product_id}` 成功。
- 商品卡在聊天中对应位置出现。
- 商品卡图片能加载，不重复拼 `/api/v1`。

### 11.2 流式片段

- 普通文本仍实时显示。
- 表格一结束，立即渲染为表格，不等整轮回答结束。
- 代码块一闭合，立即渲染为代码块。
- `message_end` 不会导致整条回答闪烁重排。

### 11.3 表格

- 表格横向滚动只发生在表格区域。
- 气泡宽度不被撑大。
- 最后一行文字与底部边框间距正常。
- 表格边框、header、行间距视觉统一。

### 11.4 复制

- 长按用户消息可以复制。
- 长按 AI 文本可以复制原始 Markdown。
- 长按表格复制原始 Markdown 表格。
- 长按商品卡复制商品 Markdown 摘要。

### 11.5 引用提示

- 不再展示“提示 已记录引用来源”。
- 如果收到完整 `citation`，展示可读引用来源。
- 如果只收到 `citation_refs`，不展示无意义 UI。

### 11.6 侧栏历史

- 首次打开侧栏加载 20 条。
- 滑到底部自动加载更早 20 条。
- 删除一条历史后，如果还有更多历史，可以继续加载补足。
- 搜索态不破坏分页状态。

### 11.7 图标

- 桌面图标完整显示猪图形宽度。
- 左右不裁剪。
- 上下多余部分透明。
- 垂直居中。

### 11.8 构建与模拟器

需要执行：

```bash
cd android-native
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

模拟器验证：

```text
安装 APK
登录
发送商品推荐问题
检查商品卡、表格、复制、引用、侧栏分页、图标
读取 logcat，确认无 AndroidRuntime / FATAL EXCEPTION
```
