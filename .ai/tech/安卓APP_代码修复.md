# 安卓 APP 代码修复进度

记录对象：`/Volumes/shared/xzxg-shop/android-native`

开始时间：2026-06-06

说明：后端当前使用 HTTP 协议，按要求不修改 HTTP/API Base/cleartext 相关配置。

## 2026-06-06 第一轮修复

### 已修复

1. 商品分页异步串数据风险
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - `loadMoreProducts` 在线程启动前捕获 `requestKey`、`requestState`、`requestPage`。
     - 请求返回后校验 `currentProductState == requestState`，不匹配则丢弃旧响应。
     - 将单个全局 `loadingProducts` 扩展为 `loadingProductKeys`，避免不同商品筛选条件之间互相污染加载状态。
   - 对应检查项：代码检查 v1 高优先级问题 2。

2. 远程会话创建和流式发送的空 `session_id` 防护
   - 文件：
     - `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
     - `android-native/app/src/main/java/com/xzxg/shop/ApiClient.java`
   - 修复内容：
     - `createSession` 返回后立即校验 `session_id` 非空，异常时不绑定本地会话、不进入流式发送。
     - `ApiClient.streamMessage` 增加 `sessionId` 前置校验，避免拼出 `/agent/sessions//messages:stream`。
   - 对应检查项：代码检查 v1 高优先级问题 3。

3. 实时语音录音启动状态校验
   - 文件：
     - `android-native/app/src/main/java/com/xzxg/shop/PcmRecorder.java`
     - `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 校验 `AudioRecord.getMinBufferSize(...) > 0`。
     - 创建后校验 `AudioRecord.STATE_INITIALIZED`。
     - `startRecording` 失败时释放 recorder。
     - 启动后校验 `RECORDSTATE_RECORDING`。
     - `realtimeRecorderStarted = true` 移到 `pcmRecorder.start(...)` 成功之后。
   - 对应检查项：代码检查 v1 高优先级问题 4。

4. 后台线程错误处理显式回到 UI 线程
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - `syncRemoteSessions`、会话置顶、重命名、删除的后台 catch 分支改为 `runOnUiThread(() -> handleApiError(error))`。
   - 对应检查项：代码检查 v1 中优先级问题 10。

5. 本地消息保存改为 UUID + 事务
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java`
   - 修复内容：
     - 数据库版本从 4 升级到 5。
     - 本地消息 ID 从 `时间戳 + content.hashCode()` 改为 UUID。
     - `saveMessageWithAttachments` 使用事务包住“插入消息 + 更新会话”。
     - 检查 `insert` 返回值，失败时抛出明确异常。
   - 对应检查项：代码检查 v1 中优先级问题 7。

6. SQLite 常用查询索引
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java`
   - 修复内容：
     - 新增 `idx_sessions_server_session_id`。
     - 新增 `idx_sessions_history_order`。
     - 新增 `idx_messages_session_created`。
     - `onCreate` 和 `onUpgrade(oldVersion < 5)` 都会创建索引。
   - 对应检查项：代码检查 v1 中优先级问题 8。

7. 图片加载内存上限和采样解码
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/ImageLoader.java`
   - 修复内容：
     - `ConcurrentHashMap<String, Bitmap>` 改为按 Bitmap 字节大小计费的 `LruCache`。
     - 网络图片先读取字节，再按目标 ImageView 尺寸采样解码。
     - 目标宽高在调用 `load` 时捕获，后台线程不直接读取 View 状态。
   - 对应检查项：代码检查 v1 中优先级问题 6。

8. 隐私日志降噪
   - 文件：
     - `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
     - `android-native/app/src/main/java/com/xzxg/shop/SpeechRealtimeClient.java`
   - 修复内容：
     - `DEBUG_AGENT_BLOCKS` 绑定 `BuildConfig.DEBUG`。
     - 语音 WebSocket 不再打印完整 message 内容，只打印消息长度。
   - 对应检查项：代码检查 v1 低优先级问题 12。

### 按要求暂不修复

1. release/default API Base 仍使用 HTTP。
2. `android:usesCleartextTraffic="true"` 保持不变。
3. 后端协议、接口路径和服务地址保持不变。

这些对应代码检查 v1 高优先级问题 1 的 HTTP 部分，因当前明确要求“后端使用 HTTP 协议，不做任何改动”，本轮不处理。

### 验证结果

原目录 `/Volumes/shared/xzxg-shop/android-native` 直接跑 Gradle 仍可能受共享卷文件系统影响。为验证代码编译，已复制到临时目录：

```bash
/private/tmp/xzxg-android-codefix-build
```

执行：

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

结果：构建成功。

构建警告：

- Java source/target 8 在当前 JDK 25 下提示未来废弃。
- `MainActivity.java` 使用或覆盖了已过时 API。
- Gradle deprecated features 提示未来 Gradle 10 不兼容。

这些是构建警告，不阻断 APK 产物生成。

### 尚未处理/后续建议

1. 附件上传仍是一次性读入内存。
   - 对应代码检查 v1 中优先级问题 5。
   - 建议下一轮改造 `ApiClient.uploadAttachment` 支持 `InputStream` 流式 multipart，并加总大小/数量限制。

2. 会话同步仍缺少明确 job 类型。
   - 对应代码检查 v1 中优先级问题 9。
   - 建议把标题、置顶、删除拆成不同同步任务，避免远端失败时本地状态误判。

3. `MainActivity` 仍然过大。
   - 对应代码检查 v1 低优先级问题 11。
   - 建议后续按 Chat/Product/CartOrder/Voice/History 逐步拆分，不建议一次性大重构。

4. release 的 `allowBackup` 风险尚未处理。
   - 这不改变 HTTP 协议，但会影响用户数据备份策略。
   - 如果确认可以改 Android 备份策略，建议下一轮单独处理。

## 2026-06-06 第二轮修复

### 修改原则

1. 不修改后端 HTTP 协议、服务地址、接口路径。
2. 不改变已有 UI 流程和业务语义。
3. 每个独立批次修改后都执行 debug 构建验证，确认原有功能至少保持编译可用。

### 已修复

1. 附件上传内存峰值优化
   - 文件：
     - `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
     - `android-native/app/src/main/java/com/xzxg/shop/ApiClient.java`
   - 修复内容：
     - 保留原 `uploadAttachment(String, String, String, byte[])` 方法，避免影响既有调用。
     - 新增 `uploadAttachment(String, String, String, long, InputStream)` 流式上传重载。
     - 附件 metadata 中 `size > 0` 时，发送路径直接从 `ContentResolver.openInputStream(uri)` 写入 multipart，不再先整体读成 byte array。
     - 附件 `size <= 0` 时保留原 `readAttachmentBytes` 路径，避免未知大小内容提供器出现兼容性问题。
     - 仍保留单附件 10MB 限制、空内容校验、原 multipart 字段和返回结构转换逻辑。
   - 对应检查项：代码检查 v1 中优先级问题 5。
   - 兼容性说明：
     - 后端接口仍是 `POST /files`，multipart 字段仍包含 `type` 和 `file`。
     - 已知大小附件走新路径；未知大小附件走旧路径，因此原功能保留。

2. 会话手动操作的远程同步状态补齐
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 会话置顶远程成功后标记本地 `sync_state = synced`，失败标记 `failed`。
     - 会话重命名远程成功后标记本地 `sync_state = synced`，失败标记 `failed`。
     - 会话删除远程成功后标记本地 `sync_state = synced`，失败标记 `failed`。
     - 失败时仍保留原有 `handleApiError` 处理，登录过期等行为不变。
   - 对应检查项：代码检查 v1 中优先级问题 9。
   - 兼容性说明：
     - 不改变原来的 API 调用顺序。
     - 不改变侧栏刷新、删除后隐藏、重命名弹窗等 UI 行为。
     - 只补本地同步状态，便于后续重试/排查。

### 本轮验证

附件上传批次修改后执行：

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

验证目录：

```bash
/private/tmp/xzxg-android-codefix-build
```

结果：构建成功。

会话同步状态批次修改后再次执行同一构建命令。

结果：构建成功。

仍存在的构建警告：

- Java source/target 8 在当前 JDK 25 下提示未来废弃。
- `MainActivity.java` 使用或覆盖了已过时 API。
- Gradle deprecated features 提示未来 Gradle 10 不兼容。

这些警告与本轮修改无直接关系，且不阻断 debug APK 构建。

### 仍未处理

1. `MainActivity` 体积过大。
   - 暂不做大拆分，避免影响现有功能。

2. `allowBackup` 风险。
   - 该项会改变系统备份行为。按“不影响原有功能”的原则，本轮未改。

3. `ApiClient` 仍以 `HttpURLConnection` 为主。
   - 全量迁移 OkHttp 会扩大改动面，本轮未处理。

## 2026-06-06 侧栏历史定位修复

### 修改原则

1. 不修改后端 HTTP 协议、接口路径、Nacos 或服务配置。
2. 不改变历史会话原有排序规则，只改变打开侧栏后的初始滚动位置。
3. 保留搜索、远端刷新、向下加载更老历史的既有能力。

### 已修复

1. 打开侧栏时当前会话定位到历史可视区域顶部
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 新增当前会话历史窗口加载逻辑：空搜索打开侧栏时，从本地历史按原排序向下分页读取，直到包含当前会话，并额外保留当前会话之后的一段旧历史。
     - 渲染后根据历史行 `history:<localSessionId>` 标签，把当前会话行滚动到 `ScrollView` 顶部。
     - 当前会话上方仍保留更新会话；继续向下滚动仍加载更老会话，使滚动条可处于历史列表中间位置。

2. 侧栏远端刷新后保持当前会话定位
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 空搜索远端第一页返回后，先写入本地，再重新按当前会话窗口渲染并定位，避免远端刷新把滚动位置重置到历史顶部。
     - 非空搜索仍沿用远端搜索结果渲染方式，避免影响搜索功能。

3. 历史分页追加去重
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 追加历史行前读取已渲染的 `history:` 标签，避免定位窗口和后续分页之间出现重复历史项。

### 兼容性说明

- 不改会话排序 SQL：置顶、更新时间排序保持原规则。
- 不改历史行点击、长按置顶、重命名、删除逻辑。
- 不改后端协议，仍使用现有 HTTP API。

### 本轮验证

1. 直接在共享卷 `/Volumes/shared/xzxg-shop/android-native` 执行 `gradle assembleDebug`：
   - 结果：失败。
   - 原因：Gradle `FileHasher` 在共享卷文件系统上报 `Operation not supported`，属于构建环境文件系统限制，不是代码编译错误。

2. 复制 Android 工程到 `/private/tmp/xzxg-android-sidebar-anchor-build` 后执行：

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

结果：构建成功。

仍存在的构建警告：

- Java source/target 8 在当前 JDK 25 下提示未来废弃。
- `MainActivity.java` 使用或覆盖了已过时 API。
- Gradle deprecated features 提示未来 Gradle 10 不兼容。

这些警告与本次侧栏历史定位修改无直接关系，且不阻断 debug APK 构建。

## 2026-06-06 侧栏历史状态恢复修正

### 需求调整

上一版实现为“打开侧栏时将当前会话定位到历史列表可视区域顶部”。本次按新需求改为：

- 打开侧栏时，恢复上一次退出侧栏时的状态。
- 重点恢复聊天历史记录的滚动位置。

### 已修复

1. 关闭侧栏前保存历史列表状态
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 在普通关闭和动画关闭侧栏前保存历史 `ScrollView` 的 `scrollY`。
     - 保存当前已加载历史 offset，便于下次打开时先加载足够多的历史项，再恢复滚动位置。
     - 保存当前历史搜索词，若上次关闭时处于搜索态，下次打开侧栏时恢复同一搜索词和位置。

2. 打开侧栏时恢复上次退出状态
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 移除“按当前会话定位到顶部”的触发逻辑。
     - 若存在已保存状态，则按保存的搜索词和 offset 重新加载历史，再将滚动位置恢复到保存的 `scrollY`。
     - 远端第一页刷新返回后，如果仍匹配保存状态，继续恢复保存的滚动位置，避免刷新把位置冲回顶部。

3. 保留分页和去重
   - 文件：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
   - 修复内容：
     - 保留追加历史项时的 `history:` 标签去重逻辑，避免恢复加载和后续分页产生重复行。

### 兼容性说明

- 不改后端 HTTP 协议、接口路径、Nacos 或服务配置。
- 不改历史排序规则。
- 不改历史行点击、新建会话、置顶、重命名、删除等业务行为。

### 本轮验证

复制 Android 工程到 `/private/tmp/xzxg-android-sidebar-anchor-build` 后执行：

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

结果：构建成功。

仍存在的构建警告：

- Java source/target 8 在当前 JDK 25 下提示未来废弃。
- `MainActivity.java` 使用或覆盖了已过时 API。
- Gradle deprecated features 提示未来 Gradle 10 不兼容。

这些警告与本次侧栏历史状态恢复修改无直接关系，且不阻断 debug APK 构建。
