# 安卓 APP 技术方案 v26

## 1. 背景

本文针对 [安卓APP_问题_v25.md](./安卓APP_问题_v25.md) 设计下一版 Android 原生 APP 改造方案。

本轮问题集中在聊天页附件发送、附件选择入口、软键盘遮挡和会话切换后的附件状态隔离。改造范围应保持在 Android APP 内，除非验证发现后端 `/attachments` 接口本身存在明确错误，否则不改后端接口。

## 2. 当前实现核对

### 2.1 附件发送链路

当前核心代码：

- `MainActivity.sendCurrentInput()`
- `MainActivity.readAttachmentBytes()`
- `ApiClient.uploadAttachment()`
- `ApiClient.streamMessage()`

现状流程：

```text
点击发送
  -> 读取全局 pendingAttachments
  -> 逐个 readAttachmentBytes(uri)
  -> ApiClient.uploadAttachment(...)
  -> 上传完成后 sendMessage(text, uploaded)
```

当前用户遇到的问题：

- 图片 + 文字发送时提示 `附件上传失败：Broken pipe`。
- 只选择图片、不输入文字时，发送体验不符合预期，应允许直接发送图片消息。

### 2.2 附件选择入口

当前核心代码：

- `toggleAttachmentPanel()`
- `renderAttachmentPanel()`
- `pickImage()`
- `pickFile()`
- `onActivityResult()`

现状：

- 点击 `+` 后展示“相册”“文件”。
- “相册”当前使用 `Intent.ACTION_OPEN_DOCUMENT` + `image/*`，系统通常进入文件管理器，而不是相册应用。
- “文件”也使用 `Intent.ACTION_OPEN_DOCUMENT`，部分设备会记住上次 Camera 路径。
- 点击页面其他区域不会自动收起附件面板。

### 2.3 键盘与聊天窗口

当前已设置：

- `AndroidManifest.xml`：`android:windowSoftInputMode="adjustResize"`
- `MainActivity.useKeyboardResize()`
- `MainActivity.bindImeInsets()`

但当前 `bindImeInsets()` 直接给 `content` 设置 `paddingBottom = imeBottom`，在自定义根布局和系统栏组合下仍可能出现聊天列表底部被键盘覆盖，且键盘显示时点击键盘外不会自动隐藏键盘。

### 2.4 待发送附件状态

当前只有一个全局字段：

```java
private final List<PendingAttachment> pendingAttachments = new ArrayList<>();
```

因此选中图片后切换到另一个聊天，另一个聊天仍会展示刚才选中的图片。v26 需要改为“每个对话独立保存待发送附件”。

## 3. v26 总体目标

1. 修复图片 + 文字发送时的 `Broken pipe` 附件上传错误。
2. 支持只上传图片、不输入文字，也能发送一条图片消息。
3. 点击 `+` 展示附件面板后，点击页面其他区域自动收起。
4. “相册”入口进入系统图片选择体验，不再默认进入文件管理器。
5. “文件”入口进入文件管理器，并尽量从根目录或通用文档入口开始，而不是 Camera 目录。
6. 输入框聚焦后，聊天窗口底部贴住键盘顶部，最新消息始终可见。
7. 键盘显示时点击聊天区域或其他非输入区域，自动收起键盘。
8. 待发送附件按会话隔离：切换聊天时只显示当前聊天自己的待发送附件。

## 4. 附件上传 Broken pipe 修复方案

### 4.1 根因判断

`Broken pipe` 通常发生在客户端还在写 multipart body 时，服务端或网络连接已经关闭。当前 `ApiClient.uploadAttachment()` 存在几个风险点：

- 未显式设置超时时间、固定长度或分块流模式，部分设备上 `HttpURLConnection` 可能缓冲策略不稳定。
- multipart 写入和响应读取缺少错误流诊断，失败时只能看到底层异常。
- 文件名未做完整 multipart header 转义，只移除了少量字符。
- 发送前没有确认附件字节是否为空、mime/type 是否有效。
- 上传失败后直接中断整次发送，没有针对短连接异常做一次重试。

### 4.2 ApiClient 改造

在 `ApiClient.uploadAttachment()` 中做最小增强：

1. 构造 multipart body 前先校验：

```text
data != null
data.length > 0
data.length <= 10MB
```

2. 将 multipart 写入改为可计算长度的 `ByteArrayOutputStream`：

```text
ByteArrayOutputStream body
  -> writeFormField(...)
  -> writeFileHeader(...)
  -> write data
  -> write closing boundary
```

然后对连接设置：

```java
conn.setFixedLengthStreamingMode(bodyBytes.length);
conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=" + boundary);
conn.setRequestProperty("Connection", "close");
```

固定长度可以避免部分设备边写边协商时被服务端提前断开；`Connection: close` 避免复用旧连接导致的断管。

3. 上传写入顺序保持简单：

```text
openRaw("/attachments", "POST")
  -> set headers
  -> getOutputStream()
  -> write bodyBytes
  -> flush
  -> readJSON(conn)
```

4. 对 `SocketException`、`ProtocolException`、包含 `Broken pipe` 的 `IOException` 做一次重试：

```text
第一次失败
  -> disconnect
  -> 重新创建连接
  -> 重新上传同一 body
```

只重试一次，避免用户重复等待。

5. 错误提示使用后端错误体：

若 `readJSON(conn)` 发现 HTTP 4xx/5xx，应读取 `conn.getErrorStream()`，把明确的业务错误展示出来；底层异常仍提示“附件上传失败：xxx”。

### 4.3 MainActivity 上传流程改造

`sendCurrentInput()` 中读取当前会话附件列表的快照：

```java
List<PendingAttachment> snapshot = new ArrayList<>(currentPendingAttachments());
```

上传线程只处理这个快照，上传成功后只清理当前会话对应的附件。这样可以避免上传过程中用户切换会话后误清空其他会话附件。

上传前补充校验：

- 附件 URI 可读取。
- 附件 size 若已知且大于 10MB，提前提示。
- 读取到的字节为空时提示“附件内容为空，无法发送”。

## 5. 支持只发送图片消息

### 5.1 行为定义

当输入框为空但当前会话存在待发送附件时：

```text
点击发送按钮
  -> 不进入语音模式
  -> 上传附件
  -> 发送 content 为空字符串、attachments 非空的消息
  -> 用户消息气泡展示附件摘要或图片预览
```

当输入框为空且没有附件时，才进入语音模式。

### 5.2 方法调整

把 `sendCurrentInput()` 的条件从全局附件改为当前会话附件：

```java
String text = input == null ? "" : input.getText().toString().trim();
List<PendingAttachment> attachments = currentPendingAttachments();
if (text.isEmpty() && attachments.isEmpty()) {
    enterVoiceMode();
    return;
}
```

`updateInputActionButtonState()` 也应把附件作为发送态条件：

```text
streaming -> 停止
input 非空 -> 发送
当前会话附件非空 -> 发送
voiceMode -> 停止录音/识别中
默认 -> 麦克风
```

这样只选择图片后，按钮会从麦克风变为发送。

## 6. 附件面板交互方案

### 6.1 点击其他区域收起

新增统一方法：

```java
private void hideAttachmentPanel() {
    if (attachmentPanelView != null) {
        attachmentPanelView.setVisibility(View.GONE);
    }
}
```

触发时机：

- 点击聊天列表空白区域。
- 点击已有聊天消息区域。
- 点击输入框。
- 点击顶部栏、侧边栏入口或切换会话。
- 选择“相册”或“文件”后立即收起。
- 发送消息后收起。

实现方式：

- 在聊天消息层外层容器设置 `OnTouchListener`，收到 `ACTION_DOWN` 时调用 `hideAttachmentPanel()` 和必要的键盘隐藏逻辑。
- 输入栏内部点击不应被外层吞掉，外层 listener 返回 `false`。
- `addButton` 自身点击只切换面板，不触发外层收起。

### 6.2 相册入口

`pickImage()` 不再使用 `ACTION_OPEN_DOCUMENT` 作为首选。按系统版本选择：

```text
Android 13+:
  Intent(MediaStore.ACTION_PICK_IMAGES)

Android 12 及以下:
  Intent(Intent.ACTION_PICK, MediaStore.Images.Media.EXTERNAL_CONTENT_URI)
  type = "image/*"

兜底:
  Intent.ACTION_GET_CONTENT
  type = "image/*"
```

注意：

- 相册返回的 URI 不一定支持 `takePersistableUriPermission()`，`onActivityResult()` 中保留 try-catch 即可。
- 选择后仍通过 `attachmentFromUri(uri, true)` 生成 `PendingAttachment`。
- 图片只需要读权限，不需要写权限。

### 6.3 文件入口默认目录

`pickFile()` 继续使用 `ACTION_OPEN_DOCUMENT`，但去掉任何可能引导到图片目录的行为，保持通用文件入口：

```java
Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
intent.addCategory(Intent.CATEGORY_OPENABLE);
intent.setType("*/*");
intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
    "application/pdf",
    "application/msword",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    "text/*",
    "image/*"
});
```

默认根目录无法在所有 Android 文件管理器中强制指定。可尝试：

```java
intent.putExtra("android.provider.extra.SHOW_ADVANCED", true);
```

不建议写死 `DocumentsContract.EXTRA_INITIAL_URI` 指向 Camera 或某个设备私有路径。验收标准应是：不主动带入 Camera 目录；系统若记忆上次目录，则属于文件管理器行为，APP 不再提供 Camera 初始 URI。

## 7. 键盘 Insets 与自动收起方案

### 7.1 聊天窗口随键盘上移

保留 `adjustResize`，但 `bindImeInsets()` 改为只给聊天输入区所在底部容器或内容根设置稳定的底部 inset，不重复叠加系统栏：

```text
imeVisible = imeBottom > 0
bottomInset = imeVisible ? imeBottom : systemBarsBottom
content.setPadding(0, 0, 0, bottomInset)
```

同时在键盘出现或高度变化后滚动到底部：

```java
chatScroll.post(() -> chatScroll.fullScroll(View.FOCUS_DOWN));
```

如果当前聊天列表使用的是 `ScrollView` 或 `NestedScrollView`，滚动目标应是承载消息的滚动容器，而不是整个 `content`。

验收行为：

- 输入框获取焦点后，输入栏底部贴住键盘顶部。
- 最新一条消息或附件预览不会被键盘遮住。
- 用户不需要手动下滑才能看到底部内容。

### 7.2 点击键盘外自动收起

新增：

```java
private void hideKeyboard() {
    InputMethodManager imm = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
    if (input != null && imm != null) {
        imm.hideSoftInputFromWindow(input.getWindowToken(), 0);
        input.clearFocus();
    }
}
```

触发时机：

- 点击聊天消息层。
- 点击顶部栏。
- 打开附件面板。
- 切换会话。

不触发时机：

- 点击输入框本身。
- 正在语音输入时点击停止按钮。
- 点击附件预览的删除按钮。

## 8. 待发送附件按会话隔离

### 8.1 数据结构

把全局列表改为按会话 ID 存储：

```java
private final Map<String, List<PendingAttachment>> pendingAttachmentsBySession = new HashMap<>();
```

新增辅助方法：

```java
private String currentAttachmentSessionKey() {
    return currentSessionId == null || currentSessionId.isEmpty() ? "local_default" : currentSessionId;
}

private List<PendingAttachment> currentPendingAttachments() {
    String key = currentAttachmentSessionKey();
    List<PendingAttachment> list = pendingAttachmentsBySession.get(key);
    if (list == null) {
        list = new ArrayList<>();
        pendingAttachmentsBySession.put(key, list);
    }
    return list;
}
```

如果当前项目中存在本地临时会话 ID 和服务端会话 ID 映射，应优先使用当前 UI 正在展示的会话 ID，保证“用户看到哪个会话，附件就挂在哪个会话”。

### 8.2 渲染与删除

以下位置全部改为使用 `currentPendingAttachments()`：

- `sendCurrentInput()`
- `renderAttachmentBuffer()`
- `attachmentPreview()` 删除按钮
- `onActivityResult()` 添加附件
- `updateInputActionButtonState()`

切换会话后必须调用：

```java
hideAttachmentPanel();
hideKeyboard();
renderAttachmentBuffer();
updateInputActionButtonState();
```

这样切到另一个聊天时不会显示之前聊天选择的图片，切回原聊天时附件仍保留。

### 8.3 发送成功后的清理

上传并发送成功后，只清理当前会话的附件：

```java
currentPendingAttachments().clear();
renderAttachmentBuffer();
```

如果发送失败，附件保留，方便用户重新发送。

## 9. 实施顺序

1. 修改附件状态结构，把 `pendingAttachments` 迁移为 `pendingAttachmentsBySession` 和辅助方法。
2. 调整 `sendCurrentInput()` 与 `updateInputActionButtonState()`，支持纯附件发送。
3. 加固 `ApiClient.uploadAttachment()`，固定长度 multipart、关闭连接复用、补充一次 Broken pipe 重试。
4. 修改 `pickImage()` 使用相册优先的 Intent；修改 `pickFile()` 为通用文件选择入口。
5. 增加 `hideAttachmentPanel()`、`hideKeyboard()`，接入聊天区域、输入框、会话切换等事件。
6. 调整 IME insets，确保聊天底部与键盘顶部衔接，并在键盘变化后滚动到底部。
7. 模拟器回归验证并检查 logcat。

## 10. 验收方案

### 10.1 本地构建

由于共享盘上 Gradle 文件哈希可能报 `Operation not supported`，继续使用临时目录构建：

```text
rm -rf /private/tmp/xzxg-android-v26-build
cp -R android-native /private/tmp/xzxg-android-v26-build
写入 local.properties: sdk.dir=/opt/homebrew/share/android-commandlinetools
gradle assembleDebug
```

### 10.2 模拟器测试

后端保持运行在 `127.0.0.1:8080`，模拟器内通过 `10.0.2.2:8080` 访问。

测试用例：

1. 图片 + 文字发送：
   - 选择一张图片。
   - 输入文字。
   - 点击发送。
   - 预期：不再出现 `Broken pipe`；用户消息正常出现；后端返回正常。

2. 纯图片发送：
   - 选择一张图片。
   - 输入框保持为空。
   - 点击发送。
   - 预期：按钮为发送态；消息可发送；不会进入语音模式。

3. 附件面板收起：
   - 点击 `+` 展示“相册”“文件”。
   - 点击聊天区域、输入框、顶部栏。
   - 预期：面板自动收起。

4. 相册入口：
   - 点击 `+` -> “相册”。
   - 预期：进入系统图片/相册选择体验，不进入普通文件管理器。

5. 文件入口：
   - 点击 `+` -> “文件”。
   - 预期：进入文件管理器通用入口；APP 不主动打开 Camera 目录。

6. 键盘遮挡：
   - 点击输入框。
   - 预期：聊天窗口整体上移，输入栏底部贴住键盘顶部，最新消息可见。

7. 点击键盘外收起：
   - 键盘显示时点击聊天区域。
   - 预期：键盘自动收起。

8. 会话附件隔离：
   - 会话 A 选择图片但不发送。
   - 切换到会话 B。
   - 预期：B 不显示 A 的图片。
   - 切回 A。
   - 预期：A 的图片仍显示。

### 10.3 日志检查

运行测试时同步检查：

```text
adb logcat
```

重点确认：

- 无 `AndroidRuntime`。
- 无 `FATAL EXCEPTION`。
- 无 `com.xzxg.shop` 崩溃。
- 附件上传失败时能看到明确错误，而不是只有底层 `Broken pipe`。

## 11. 不纳入本轮的内容

- 不改聊天回答展示样式。
- 不改 Agent 思考过程展示。
- 不改商品卡片、订单、购物车页面。
- 不改后端接口协议，除非验证确认 `/attachments` 服务端实现导致断管。
- 不增加多图批量选择能力，本轮只保证现有单选入口稳定可用。
