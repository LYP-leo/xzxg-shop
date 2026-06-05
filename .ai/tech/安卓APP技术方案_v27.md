# 安卓 APP 技术方案 v27

## 1. 背景

本文针对 [安卓APP_问题_v26.md](./安卓APP_问题_v26.md) 设计下一版 Android 原生 APP 改造方案。

本轮问题集中在聊天主界面三处体验缺口：

1. 用户发送图片后，右侧用户消息没有展示图片，只显示文字或“已选择一个附件”。
2. 附件上传过程中只有右下角按钮变成“...”，缺少明确的上传中反馈。
3. 思考过程仍未达到参考图要求，需要展示“分析用户需求 / 查询买手团经验 / 总结答案”三个模块，完成后默认收起为“已完成思考”。

改造范围限定在 Android 原生 APP，优先修改：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java`

除非验证发现后端 SSE 字段缺失，否则不改后端。

## 2. 当前实现核对

### 2.1 用户附件消息展示

当前发送入口：

```java
private void sendCurrentInput()
private void sendMessage(String text, JSONArray attachments)
```

现状流程：

```text
选择图片
  -> pendingAttachmentsBySession 保存本地 Uri
点击发送
  -> 上传附件
  -> sendMessage(text, uploaded)
  -> visibleText = 文本 或 attachmentSummary(attachments)
  -> addBubble(visibleText, true)
  -> chatStore.saveMessage(..., visibleText, ...)
```

问题：

- `sendMessage()` 只把附件转成一段文本，没有把附件 JSON 交给用户消息气泡渲染。
- 有文本 + 图片时，`visibleText = text`，图片完全丢失在本地 UI 中。
- 只有图片时，`visibleText = "[已选择 1 个附件]"`，用户看不到刚上传的图片。
- 本地历史消息表 `messages` 没有专门保存用户附件 JSON，切换会话或重启后无法恢复图片预览。

### 2.2 上传中反馈

当前上传中反馈：

```java
setActionButtonText("…");
actionButton.setEnabled(false);
```

问题：

- 用户只能看到发送按钮变化，不知道具体哪张图片正在上传。
- 上传耗时较长时，右侧消息区没有占位气泡，体验像“卡住”。
- 上传失败时未保留可感知的失败消息状态。

### 2.3 思考过程展示

当前已有 `ThinkingViewState`、`ThinkingStageState`、`handleThinkingDelta()`、`renderThinkingView()`。

现状问题：

- `thinking_delta` 只有收到事件时才创建视图；若后端先发 `status` 或部分字段不完整，用户仍只看到“正在思考...”。
- 三个模块没有固定占位，未开始的模块不会显示，视觉上不像参考图的完整时间线。
- “查询买手团经验”当前只在 `items` 非空时展示横向卡片，若后端返回的是普通文本或不同字段，容易展示为空。
- 完成收起时应只显示“已完成思考”，点击后展开完整内容；该行为需要稳定应用于实时消息和历史消息。

## 3. v27 总体目标

1. 用户发送图片后，右侧用户消息必须展示图片缩略图；有文本时同时展示文本和图片。
2. 纯图片发送时，不再只显示“已选择一个附件”，而是显示图片气泡。
3. 上传过程中在聊天区展示右侧上传中气泡，包含图片缩略图骨架、进度动画和上传状态文本。
4. 上传失败时，上传中气泡改为失败状态，并提示可重新发送或保留原待发送附件。
5. 思考过程固定展示三个模块：
   - 分析用户需求
   - 查询买手团经验
   - 总结答案
6. 思考过程中展示后端返回的详细内容；完成后默认收起为“已完成思考”，点击后可展开。
7. 历史消息恢复时，用户图片消息和已完成思考区都能正确展示。

## 4. 用户附件消息展示方案

### 4.1 新增本地消息附件字段

在 `LocalChatStore` 的 `messages` 表新增列：

```sql
attachments_json TEXT NOT NULL DEFAULT '[]'
```

升级逻辑：

```java
addColumnIfMissing(db, "messages", "attachments_json", "TEXT NOT NULL DEFAULT '[]'");
```

`MessageItem` 增加字段：

```java
public final String attachmentsJson;
```

新增或扩展保存方法：

```java
saveMessage(
    String localSessionId,
    String role,
    String content,
    String attachmentsJson,
    String blocksJson,
    String followupsJson,
    String segmentsJson,
    String status
)
```

兼容要求：

- 旧调用默认 `attachments_json = "[]"`。
- 旧数据库升级后，历史纯文本消息不受影响。
- 查询 `messages()` 时把 `attachments_json` 一并取出。

### 4.2 用户消息渲染入口改造

把当前：

```java
addBubble(visibleText, true);
chatStore.saveMessage(localSessionId, "user", visibleText, "pending");
```

改为：

```java
addUserMessageBubble(text, attachments);
chatStore.saveMessage(
    localSessionId,
    "user",
    text,
    attachments.toString(),
    "[]",
    "[]",
    "[]",
    "pending"
);
```

`sendMessage()` 中保留传给后端的文本：

- 有文本：后端 `content = text`
- 纯附件：后端 `content = ""` 或当前后端要求的占位文本

不要再把 `"[已选择 1 个附件]"` 当作真正用户消息内容传入 UI 和本地存储。

### 4.3 右侧用户消息 UI

新增方法：

```java
private View addUserMessageBubble(String text, JSONArray attachments)
private LinearLayout userMessageBubbleContainer()
private View userImageAttachmentView(JSONObject attachment)
private View userFileAttachmentView(JSONObject attachment)
```

布局规则：

- 右侧气泡保持当前用户消息黑色文本风格。
- 有文本时，文本显示在气泡顶部。
- 图片附件显示在文本下方，使用圆角缩略图。
- 单张图片：宽度约 `160dp`，高度按 `4:3` 或图片实际比例限制，最大高度 `220dp`。
- 多张图片：使用 2 列网格或横向缩略图列表，每张 `96dp` 左右。
- 文件附件：显示文件图标、文件名、大小。
- 图片加载优先使用附件返回的 `url`；如果上传前本地 Uri 仍可用，上传中状态使用本地 Uri。

附件 JSON 字段兼容：

```text
url
name
type
mime_type
attachment_id
object_key
size
```

判断图片：

```text
type == "image"
或 mime_type 以 image/ 开头
或 url/name 后缀为 jpg/jpeg/png/webp/gif
```

### 4.4 历史消息恢复

`renderStoredMessagesIfAny()` 中用户消息从：

```java
addBubble(message.content, "user".equals(message.role));
```

改为：

```java
if ("user".equals(message.role)) {
    addUserMessageBubble(message.content, jsonArray(message.attachmentsJson));
}
```

如果旧消息没有附件字段，仍走纯文本气泡。

## 5. 上传过程动画方案

### 5.1 上传状态模型

新增上传占位状态：

```java
private static class UploadingMessageViewState {
    LinearLayout container;
    TextView statusText;
    final Map<String, View> attachmentViews = new HashMap<>();
    boolean failed;
}
```

发送时流程改为：

```text
点击发送
  -> 复制 attachmentSnapshot
  -> 创建右侧上传中气泡
  -> 清空输入框
  -> 后台逐个上传
  -> 每个附件上传完成后更新对应缩略图状态
  -> 全部成功：把上传中气泡替换为正式用户消息气泡
  -> 失败：气泡切到失败样式并提示错误
```

### 5.2 上传中气泡 UI

新增方法：

```java
private UploadingMessageViewState addUploadingUserMessageBubble(String text, List<PendingAttachment> attachments)
private void updateUploadingAttachmentState(UploadingMessageViewState state, int index, String status)
private void finishUploadingUserMessageBubble(UploadingMessageViewState state, String text, JSONArray uploaded)
private void failUploadingUserMessageBubble(UploadingMessageViewState state, Throwable error)
```

视觉要求：

- 右侧展示一条临时用户消息，位置与最终用户消息一致。
- 文本存在时先显示文本。
- 图片缩略图使用本地 Uri，覆盖一层半透明蒙层。
- 蒙层内显示一个轻量动画：
  - 可用 `ProgressBar` indeterminate 圆形进度。
  - 或自定义三个点透明度动画。
- 状态文案：
  - 上传前：`准备上传`
  - 上传中：`正在上传 1/2`
  - 成功：`上传完成`
  - 失败：`上传失败：xxx`

实现优先级：

1. 先使用 Android 原生 `ProgressBar`，稳定可靠。
2. 若现有视觉不协调，再封装 `PulsingDotsView` 自定义动画。

### 5.3 失败处理

上传失败时：

- 取消本次后端消息发送。
- 不清空当前会话 `pendingAttachments`，或将 snapshot 放回当前会话附件列表。
- 上传中气泡改为失败状态，保留图片缩略图。
- `actionButton` 恢复可点击。
- Toast 仍提示简短错误。

失败气泡可提供一个“重试”按钮，但 v27 可先不做按钮，保留附件在输入区即可重新发送。

### 5.4 与正式消息替换

上传成功后不应重复出现两条用户消息。

建议实现：

```text
上传中气泡 container 已在 chatList 中
  -> finishUploadingUserMessageBubble 内 removeView(container)
  -> 在同一 index 插入正式 addUserMessageBubble(...)
```

若实现同 index 插入复杂，可接受先删除再 append 到底部，因为上传期间聊天底部通常仍是当前消息；但必须避免同时存在“上传中”和正式消息两条。

## 6. 思考过程详细展示方案

### 6.1 固定三阶段模型

`ThinkingViewState` 初始化时就创建三个阶段：

```text
user_need        -> 分析用户需求
buyer_experience -> 查询买手团经验
answer_summary   -> 总结答案
```

不要等后端事件到了才显示阶段。`ensureThinkingView()` 创建后立即初始化：

```java
ensureThinkingStage(thinking, "user_need");
ensureThinkingStage(thinking, "buyer_experience");
ensureThinkingStage(thinking, "answer_summary");
```

每个阶段状态：

```text
pending
running
completed
failed
```

图标规则：

- pending：空心圆或浅灰点
- running：小型进度动画或高亮圆点
- completed：灰色圆形对勾
- failed：红色感叹号

### 6.2 后端字段映射

继续兼容当前 `thinking_delta` 事件，字段优先级如下：

```text
stage: user_need / buyer_experience / answer_summary
title: 阶段标题
delta: 文本增量
text/content/summary: 完整文本或摘要
items: 买手团经验卡片列表
status: running / completed / failed
```

`thinkingStageKey()` 继续兼容旧值：

```text
intent / query_rewrite -> user_need
retrieval / tool       -> buyer_experience
answer / done          -> answer_summary
```

内容更新规则：

- `分析用户需求`：展示 `delta`、`text`、`content`、`summary` 中可用内容。
- `查询买手团经验`：
  - 有 `items`：渲染横向卡片。
  - 同时有文本：卡片上方或下方显示摘要文本。
  - 没有 `items` 但有文本：直接显示文本，不隐藏 body。
- `总结答案`：收到该阶段完成或流结束时，显示固定文案 `总结答案完成`。

### 6.3 UI 布局对齐参考图

思考区位于 AI 正文之前，独立占一块左侧内容区域。

Header：

```text
✦ 正在思考 ︿
✦ 已完成思考 ﹀
```

要求：

- 文字颜色使用灰色，字号约 `15sp`。
- 点击 header 切换展开/收起。
- 进行中默认展开。
- 完成后如果用户没有手动展开/收起，自动收起。
- 用户手动展开后，不要在后续 delta 中强制收起，直到完成；完成时仍可按需求收起，但需要保留可展开。

Detail 时间线：

```text
✓ 分析用户需求完成
  用户需求分析文本...

✓ 查询买手团经验完成
  横向经验卡片或文本...

✦ 总结答案完成
```

细节：

- 左侧竖线连接三个阶段。
- 阶段标题后根据状态拼接“中 / 完成”。
- 内容文本使用浅灰色，行距略大。
- 买手团经验卡片横向滚动，卡片内至少显示标题和摘要。
- 不能只显示“正在 xxx”，必须展示内容区。

### 6.4 生命周期与完成条件

创建时机：

- 收到第一个 `status` 且内容类似“正在思考”时，也应创建三阶段思考区。
- 收到第一个 `thinking_delta` 时创建。
- 开始流式请求后可先展示三阶段 pending 状态，替代纯 `addLoadingBubble()`，但若后端很快返回正文，也允许短暂 loading。

完成时机：

```text
收到 answer_summary completed
或收到 final/done 事件
或 finishStream()
或 finishCanceledStream()
```

`finishStream()` 必须调用：

```java
completeThinkingView();
```

完成时：

- 所有未失败阶段标记为 completed。
- `answer_summary.text = "总结答案完成"`。
- 如果 `thinking.userToggled == false`，设置 `expanded = false`。
- 保存 thinking segment 到 `activeAssistantSegments`。

### 6.5 历史消息恢复

当前已有：

```java
segment.put("type", "thinking");
segment.put("thinking", value);
```

v27 要求：

- `thinkingToJson()` 保存三个阶段，即使某个阶段只有 pending/completed 状态也要保存。
- `renderStoredThinking()` 默认 `expanded = false`。
- 历史中 header 显示“已完成思考 ﹀”。
- 点击后展开三个阶段完整内容。

## 7. 需要修改的方法清单

### 7.1 `LocalChatStore.java`

修改：

- `onCreate()`
- `onUpgrade()`
- `saveMessage(...)`
- `messages(...)`
- `MessageItem`

新增：

- `attachments_json` 列。
- 带 `attachmentsJson` 参数的保存重载。

### 7.2 `MainActivity.java` 附件消息

修改：

- `sendCurrentInput()`
- `sendMessage(String text, JSONArray attachments)`
- `renderStoredMessagesIfAny()`

新增：

- `addUserMessageBubble(...)`
- `userImageAttachmentView(...)`
- `userFileAttachmentView(...)`
- `isImageAttachment(...)`
- `attachmentDisplayName(...)`
- `attachmentImageUrl(...)`

### 7.3 `MainActivity.java` 上传动画

修改：

- `sendCurrentInput()` 上传线程 UI 流程。

新增：

- `UploadingMessageViewState`
- `addUploadingUserMessageBubble(...)`
- `updateUploadingAttachmentState(...)`
- `finishUploadingUserMessageBubble(...)`
- `failUploadingUserMessageBubble(...)`

### 7.4 `MainActivity.java` 思考过程

修改：

- `handleSse(...)`
- `handleThinkingDelta(...)`
- `ensureThinkingView(...)`
- `renderThinkingView(...)`
- `ensureThinkingStageViews(...)`
- `updateThinkingStageViews(...)`
- `completeThinkingView(...)`
- `thinkingToJson(...)`
- `renderStoredThinking(...)`

新增：

- `ensureThinkingStage(...)`
- `initializeThinkingStages(...)`
- `thinkingStageStatusIcon(...)`
- `thinkingStageTextFromEvent(...)`
- `thinkingExperienceCards(...)`

## 8. 验证方案

### 8.1 编译验证

在临时目录构建：

```bash
gradle assembleDebug
```

要求：

- 构建成功。
- 不引入新的 Android SDK 或第三方依赖。

### 8.2 虚拟机手工验证

启动后端 8080 和 Web 前端 5173 后，在虚拟机安装并运行 Android APP。

验证用例：

1. 只选择一张图片并发送：
   - 发送前输入区显示待发送图片。
   - 上传中聊天区出现右侧上传动画气泡。
   - 上传成功后右侧消息显示图片缩略图，不显示“已选择 1 个附件”。

2. 输入文字 + 选择图片并发送：
   - 上传中气泡同时显示文字和图片缩略图。
   - 上传成功后右侧正式消息同时显示文字和图片。
   - 后端仍能收到文本和 attachments。

3. 上传失败：
   - 右侧上传中气泡变为失败状态。
   - 输入区或会话附件状态可重新发送。
   - 不产生空白用户消息。

4. 思考过程：
   - AI 正文前出现“正在思考”区域。
   - 展开态展示三阶段。
   - `分析用户需求` 展示后端分析内容。
   - `查询买手团经验` 展示后端经验内容或卡片。
   - `总结答案` 显示“总结答案完成”。
   - 完成后自动收起为“已完成思考”。
   - 点击“已完成思考”可展开完整内容。

5. 历史恢复：
   - 切换会话回来后，用户图片消息仍能显示。
   - 已完成思考区默认收起，点击可展开。

### 8.3 日志验证

安装并启动后查看：

```bash
adb logcat -d AndroidRuntime:E '*:S'
```

要求：

- 无 `FATAL EXCEPTION`。
- 无 `ClassCastException`、`NullPointerException`、`SQLiteException`。

## 9. 风险与约束

1. 图片 URL 可能是相对路径或需要鉴权。若 `ImageView.setImageURI(Uri.parse(url))` 无法加载远程图片，需要使用现有后端可访问完整 URL，或实现简单异步下载 Bitmap。v27 优先兼容本地 Uri 上传中预览和后端返回绝对 URL。
2. 本地数据库新增字段必须兼容旧数据，避免用户升级后历史会话崩溃。
3. 上传中占位气泡和正式气泡替换要避免重复保存消息。
4. 思考区不能阻塞正文渲染；若后端不返回 `thinking_delta`，APP 仍要正常显示最终答案。
5. 后端 `thinking_delta` 字段若与方案不一致，Android 侧应做宽松字段读取，不因字段缺失导致空白。

## 10. 实施顺序

1. 先改 `LocalChatStore`，支持用户消息保存 `attachments_json`。
2. 实现用户附件消息气泡和历史恢复。
3. 改造上传流程，加入上传中气泡和成功替换。
4. 改造思考区为固定三阶段时间线。
5. 编译、安装、运行，按验证用例检查。
