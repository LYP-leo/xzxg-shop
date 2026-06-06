# 安卓 APP 代码检查 v1

检查对象：`/Volumes/shared/xzxg-shop/android-native`

检查时间：2026-06-06

## 结论摘要

当前 Android APP 已经具备完整业务闭环：登录、AI 导购会话、附件、语音、商品、购物车、订单、优惠券、本地会话缓存等都已实现。整体实现偏“单 Activity 快速交付”形态，能跑通功能，但后续稳定性风险主要集中在：

1. 安全配置仍是内测/调试状态，release 包默认使用明文 HTTP。
2. `MainActivity` 承担过多职责，异步请求与全局 UI 状态耦合，存在页面切换后的串数据风险。
3. 语音录音、商品分页、附件上传、会话同步等异步链路缺少统一生命周期取消和状态校验。
4. 图片和 SQLite 存储实现较轻，长列表和高频聊天场景下有内存、性能、重复/丢失数据风险。
5. 本次构建验证未进入编译阶段，Gradle 在共享卷上报 `java.io.IOException: Operation not supported`，需要迁移到支持 Gradle 文件锁/哈希能力的本地目录或调整 Gradle 缓存目录后再做编译级确认。

## 已执行验证

执行命令：

```bash
cd /Volumes/shared/xzxg-shop/android-native
gradle assembleDebug
```

结果：失败在 Gradle 初始化阶段，尚未开始 Java/Android 编译。

错误摘要：

```text
Could not create service of type FileHasher
java.io.IOException: Operation not supported
```

判断：这更像是当前 `/Volumes/shared` 文件系统能力与 Gradle 文件哈希/锁机制不兼容，不代表源码一定无法编译。建议复制到普通本地磁盘目录，或把 `GRADLE_USER_HOME`、项目目录放到 `/private/tmp`/用户目录后再跑 `./gradlew assembleDebug`。

## 高优先级问题

### 1. release 包仍默认明文 HTTP，且全局允许明文流量

位置：

- `android-native/app/build.gradle:24`
- `android-native/app/build.gradle:28`
- `android-native/app/src/main/AndroidManifest.xml:15`
- `android-native/app/src/main/AndroidManifest.xml:16`

现象：

- debug 和 release 的 `DEFAULT_API_BASE` 都是 `http://82.156.207.98:8080/api/v1`。
- Manifest 配置了 `android:usesCleartextTraffic="true"`。
- `android:allowBackup="true"` 会允许系统备份 SharedPreferences 和本地 SQLite，里面包含 token、账号资料和聊天记录。

影响：

- 登录 token、聊天内容、订单/购物车信息、语音识别请求都可能走明文通道。
- release 包如果分发给真实用户，会直接形成中间人和隐私泄露风险。
- 用户换机/云备份场景可能备份敏感数据。

建议：

- release 使用 HTTPS 域名，并关闭全局 `usesCleartextTraffic`。
- 如必须内测 HTTP，使用 debug-only `networkSecurityConfig` 定向放行测试域名/IP。
- release 设置 `allowBackup="false"`，或增加 backup rules 排除 token、聊天 SQLite、附件缓存等敏感数据。

### 2. 商品分页加载使用全局 `currentProductState`，旧请求可能污染新页面

位置：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:5678`

现象：

`loadMoreProducts` 在线程启动后，后台和 UI 回调都直接读取/修改全局 `currentProductState`：

- 请求参数使用 `currentProductState.nextPage`。
- 回调里把 `page.items` append 到 `currentProductState.items`。

如果用户在请求未返回时快速切换关键词、分类或页面，`currentProductState` 已经指向新的列表状态，旧请求返回后会把旧分类/旧关键词数据追加到新列表。

影响：

- 商品列表出现串分类、重复、分页错乱。
- `loadingProducts` 全局锁也会导致 A 列表请求阻塞 B 列表加载。

建议：

- 在线程启动前捕获局部不可变状态：`ProductListState requestState = currentProductState`、`String requestKey = productCacheKey(keyword, categoryId)`、`int requestPage = requestState.nextPage`。
- UI 回调时校验当前 key 是否仍等于 requestKey，不一致则丢弃旧响应。
- `loadingProducts` 改成按列表 key 存储，或至少只在当前 key 匹配时清理。

### 3. 远程会话创建未校验 `session_id`，可能把空 ID 传入流式接口

位置：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:2287`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:2288`
- `android-native/app/src/main/java/com/xzxg/shop/ApiClient.java:448`

现象：

`api.createSession` 后直接 `optString("session_id", "")`，没有判断为空就调用 `stream(ownerLocalSessionId, createdServerSessionId, ...)`。`ApiClient.streamMessage` 会拼接 `/agent/sessions/` + `sessionId` + `/messages:stream`。

影响：

- 后端字段名变化、返回异常空对象、代理截断响应时，APP 会发出 `/agent/sessions//messages:stream`，错误定位会变成“发送失败/流式中断”，而不是“创建会话响应无效”。
- 本地会话可能被标记为已绑定/同步，实际远端会话不可用。

建议：

- 创建后立即校验 `session_id` 非空，不合法时抛出明确错误并不要 `bindServerSession`。
- `ApiClient.streamMessage` 对 `sessionId` 做前置参数校验。

### 4. 语音录音启动缺少 AudioRecord 状态校验

位置：

- `android-native/app/src/main/java/com/xzxg/shop/PcmRecorder.java:27`
- `android-native/app/src/main/java/com/xzxg/shop/PcmRecorder.java:33`
- `android-native/app/src/main/java/com/xzxg/shop/PcmRecorder.java:41`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:1117`

现象：

`AudioRecord.getMinBufferSize` 可能返回 `ERROR` 或 `ERROR_BAD_VALUE`。当前代码没有判断，随后直接创建 `AudioRecord` 并 `startRecording`。另外 `realtimeRecorderStarted = true` 在 `pcmRecorder.start` 前设置。

影响：

- 部分设备、蓝牙输入、权限状态异常、采样率不支持时可能崩溃或进入错误状态。
- UI 会认为录音已开始，超时逻辑不会按“未开始录音”处理。

建议：

- 判断 `minBuffer > 0`。
- 创建后检查 `recorder.getState() == AudioRecord.STATE_INITIALIZED`。
- `startRecording` 后检查 `getRecordingState()`。
- `realtimeRecorderStarted = true` 应在 `pcmRecorder.start` 成功后设置。

## 中优先级问题

### 5. 附件上传先一次性读入内存，容易 OOM

位置：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:780`

现象：

发送附件时 `readAttachmentBytes(attachment.uri)` 把文件全部读入 byte array，再交给 `api.uploadAttachment`。虽然业务限制单个附件 10MB，但多附件连续上传、图片解码、商品图缓存并存时仍会制造内存峰值。

影响：

- 中低端 Android 设备可能在多图发送、长商品列表、头像裁剪之后出现 OOM。
- 上传失败后已有上传成功的附件没有服务器侧回滚，可能产生孤儿文件。

建议：

- 上传 API 改为基于 `InputStream` 流式写 multipart。
- 多附件上传增加总大小限制和数量限制。
- 附件上传成功但消息发送失败时，至少在本地保存“已上传未发送”状态，方便重试或清理。

### 6. 图片缓存无上限、无尺寸采样，长列表有内存风险

位置：

- `android-native/app/src/main/java/com/xzxg/shop/ImageLoader.java:16`
- `android-native/app/src/main/java/com/xzxg/shop/ImageLoader.java:42`
- `android-native/app/src/main/java/com/xzxg/shop/ImageLoader.java:46`

现象：

图片缓存是 `ConcurrentHashMap<String, Bitmap>`，没有 LRU 上限。网络图片用 `BitmapFactory.decodeStream` 原尺寸解码。

影响：

- 商品列表、商品详情、头像、Markdown 图片积累后会长期占用内存。
- 大图原尺寸解码可能瞬间 OOM。

建议：

- 使用 `LruCache<String, Bitmap>`，按字节大小限制。
- 根据目标 View 尺寸使用 `inSampleSize` 采样解码。
- 长期建议换 Glide/Coil/Picasso 这类成熟图片库。

### 7. 本地消息 ID 有碰撞和静默插入失败风险

位置：

- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:20`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:124`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:128`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:138`

现象：

本地消息主键是 `"local_msg_" + now + "_" + Math.abs(content.hashCode())`。同一毫秒内发送相同内容、或高频保存相同空内容/附件消息，可能产生相同主键。`insert` 返回值没有检查。

影响：

- 消息可能插入失败但 UI 已显示，重启后丢失。
- 附件-only 消息内容为空，hash 一样，碰撞概率更高。

建议：

- 本地消息 ID 改为 UUID。
- `insert` 返回 -1 时记录错误并反馈状态。
- 对“保存用户消息 + 更新会话”使用事务，避免半成功。

### 8. SQLite 查询缺少索引，聊天历史多后会卡顿

位置：

- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:19`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java:20`

现象：

表结构没有为常用查询建索引。代码里频繁按 `server_session_id`、`local_session_id`、`updated_at`、`pinned_at`、`deleted_at`、`created_at` 查询/排序。

影响：

- 会话和消息数量增长后，打开侧栏、搜索历史、加载消息会越来越慢。
- 单 Activity UI 线程触发部分本地查询时，可能造成可感知卡顿。

建议：

- 增加索引：
  - `sessions(server_session_id)`
  - `sessions(deleted_at, pinned_at, updated_at)`
  - `messages(local_session_id, created_at)`
  - 视搜索需求考虑 FTS 表替代 `LIKE '%keyword%'`。

### 9. 会话远程同步只同步标题/summary，置顶和删除状态可能被误标为 synced

位置：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:2184`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:2216`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:8047`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:8101`

现象：

`enqueueSessionSync`/`drainSessionSyncQueue` 只调用 `api.updateSession(title, summary)` 并把本地状态标为 `synced`。但本地 `pinSession`、`deleteSession` 也会把 `sync_state` 置为 `pending_sync`，它们真实远程同步走另一条 fire-and-forget 线程。

影响：

- 如果远程置顶/删除失败，本地可能已经刷新为成功，后续同步状态也可能不准确。
- 删除失败后侧栏本地隐藏，但远端仍存在；重新拉远程列表时可能恢复或产生混乱。

建议：

- 将标题、置顶、删除拆成明确 sync job 类型。
- 每个远程动作失败时保留可重试状态，不要被标题同步覆盖为 `synced`。
- 删除建议实现 tombstone 队列，远程成功后再最终清理或保持已删除状态。

### 10. 后台线程中直接调用 `handleApiError` 的路径存在 UI 线程风险

位置：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:8053`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:8082`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:8108`

现象：

置顶、重命名、删除会话的远程失败分支在 `new Thread` 内直接调用 `handleApiError(error)`。如果 `handleApiError` 内部触发 Toast、页面跳转或登录过期处理，就会从非 UI 线程操作 UI。

影响：

- 可能出现 `CalledFromWrongThreadException`。
- 登录过期、Toast 展示等行为不稳定。

建议：

- 所有后台线程 catch 分支统一 `runOnUiThread(() -> handleApiError(error))`。
- 建议封装 `runApiTask(success, failure)`，统一线程切换和 auth 过期处理。

## 低优先级/规范问题

### 11. `MainActivity` 过大，模块边界不清晰

位置：`android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`

现象：

单文件约 9000 行，包含导航、聊天、语音、附件、商品、购物车、订单、优惠券、账号、Markdown、图片保存、系统 inset 等大量职责。

影响：

- 任一功能改动都可能影响全局状态。
- 生命周期取消、页面状态恢复、异步请求去重很难统一。
- 后续多人协作和回归测试成本高。

建议：

- 先不必一次性重构为复杂架构，可按风险逐步拆：
  - `ChatController`/`ChatRenderer`
  - `ProductController`
  - `OrderCartController`
  - `VoiceInputController`
  - `HistoryDrawerController`
- 最少先抽统一的后台任务执行器和 UI 状态 token 校验。

### 12. Debug 日志开关常量为 true，可能泄露 agent block 内容

位置：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java:93`
- `android-native/app/src/main/java/com/xzxg/shop/SpeechRealtimeClient.java:68`

现象：

`DEBUG_AGENT_BLOCKS = true`，并且语音 WebSocket message 会完整打印。导购对话、思考块、识别文本可能进入 logcat。

影响：

- release/内测包日志可能包含用户隐私、商品偏好、账号相关信息。

建议：

- 日志开关绑定 `BuildConfig.DEBUG`。
- 语音/agent 日志只打印事件类型、长度、run_id，不打印全文。

### 13. 网络层仍是手写 `HttpURLConnection`，缺少统一拦截、重试、取消和 JSON schema 校验

位置：`android-native/app/src/main/java/com/xzxg/shop/ApiClient.java`

现象：

普通 REST 使用 `HttpURLConnection`，语音使用 OkHttp WebSocket。网络错误解析、鉴权过期、重试、取消分散在各调用点。

影响：

- 同类错误在不同页面表现不一致。
- 部分 API 请求在 Activity 销毁后仍可能继续执行，只是回调时对象已变化。

建议：

- 统一到 OkHttp Client。
- 封装请求任务返回 cancellable handle。
- 对关键响应字段做必填校验，例如 `session_id`、`items`、`file.file_id`。

## 建议修复顺序

1. 先修安全配置：release HTTPS、关闭明文、处理 backup。
2. 修直接运行 bug：商品分页状态串扰、空 `session_id` 校验、录音状态校验、后台线程 UI 调用。
3. 补稳定性：图片 LRU、附件流式上传、本地消息 UUID 和 SQLite 索引。
4. 做架构整理：后台任务统一封装、按业务拆分 `MainActivity`。
5. 重新跑构建和真机回归：登录、发送文本、发送附件、流式回复、语音输入、商品分页、购物车结算、历史会话同步。

## 回归测试清单

建议在修复后至少覆盖：

- 冷启动、登录态有效/过期、退出登录后再次登录。
- 新会话第一条消息、已有会话继续发送、快速切换会话时流式响应不串屏。
- 快速切换商品关键词/分类并滚动分页，确认旧请求不会污染新列表。
- 上传 0 字节、超 10MB、多张图片、上传中切页面/返回。
- 语音识别：无权限、权限拒绝、服务不可用、录音 1 秒内取消、60 秒自动结束。
- 长聊天历史 500+ 条、商品图 200+ 张，观察内存和侧栏搜索耗时。
- release 构建包抓包确认无明文 API 请求。
