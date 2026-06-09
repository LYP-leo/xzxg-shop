# 安卓 APP 技术方案 v34

## 1. 背景与目标

当前安卓 APP 已经趋于稳定，本次新增功能应坚持“只做增量、不改主链路”的原则，避免影响已稳定的聊天、附件、语音转文字、商品卡片、购物车和历史会话能力。

本版本解决两个新增需求：

1. 聊天页面左下角“+”附件面板新增“拍照”入口。
2. 聊天页面新增 TTS 开关，开启后自动朗读 AI 回复正文。

本方案参考问题文档：`.ai/tech/安卓APP_问题_v33.md`。

## 2. 非目标

本次不重构聊天页面整体结构。

本次不改变现有“相册”“文件”的选择和上传逻辑。

本次不替换现有讯飞语音转文字能力。

本次不把火山引擎 API Key 写入安卓 APK。

本次不做完整语音对话模式，只做“AI 文本回复朗读”。

## 3. 问题到模块映射

| 需求 | 安卓模块 | 后端模块 | 配置模块 |
| --- | --- | --- | --- |
| 附件面板新增拍照 | `ChatActivity`、`ChatAttachmentController` | 不需要新增 | 不需要新增 |
| 拍照后自动选取图片 | `ChatActivity`、系统相机 Intent、`ContentResolver` | 不需要新增 | 不需要新增 |
| TTS 开关 | `ChatActivity`、本地偏好存储 | 可选返回能力开关 | Nacos 非密配置 |
| AI 回复自动朗读 | 新增 `tts` 控制器、文本清洗、播放器 | TTS 代理或临时凭证接口 | Nacos 火山 TTS 配置 |
| 剔除 markdown 和思考内容 | 新增 `TtsTextSanitizer`、复用聊天流解析结果 | 不需要新增 | 不需要新增 |

## 4. 总体设计

### 4.1 拍照附件

拍照能力复用现有附件上传链路。用户拍照后，APP 将照片的 `content://` URI 转成现有 `PendingAttachment`，后续发送消息、上传附件、展示附件缩略信息都沿用已有逻辑。

附件面板顺序调整为：

1. 拍照
2. 相册
3. 文件

### 4.2 TTS 朗读

TTS 作为聊天页面的独立增强能力存在。它只消费 AI 已经生成的可见正文，不参与 AI 回复生成，也不影响购物车卡片、商品卡片、订单卡片和工具调用结果。

推荐先接入“语音合成大模型”，不接入“端到端实时语音大模型”。

官方文档：

- 端到端实时语音大模型：https://www.volcengine.com/docs/6561/1594356?lang=zh
- 语音合成大模型：https://www.volcengine.com/docs/6561/1329505?lang=zh

## 5. 拍照功能方案

### 5.1 UI 调整

在 `ChatActivity.renderAttachmentPanel()` 中，把附件入口从当前两个扩展为三个：

```java
attachmentPanelView.addView(attachmentEntry("拍照", "相机", v -> takePhoto()), ...);
attachmentPanelView.addView(attachmentEntry("相册", "图片", v -> pickImage()), ...);
attachmentPanelView.addView(attachmentEntry("文件", "文档", v -> pickFile()), ...);
```

为降低视觉变化，入口样式继续复用现有 `attachmentEntry()`。

### 5.2 请求码

新增拍照请求码：

```java
private static final int REQUEST_TAKE_PHOTO = 3104;
```

不要复用相册或文件请求码，避免 `onActivityResult()` 分支误判。

### 5.3 拍照 URI

建议使用 `MediaStore` 预创建图片 URI，再通过 `MediaStore.EXTRA_OUTPUT` 交给系统相机写入。

关键字段：

- `pendingCameraUri`
- `pendingCameraDisplayName`

流程：

1. 点击“拍照”。
2. APP 调用 `createCameraImageUri()` 创建 `image/jpeg` 的 `content://` URI。
3. APP 启动 `MediaStore.ACTION_IMAGE_CAPTURE`。
4. 通过 `putExtra(MediaStore.EXTRA_OUTPUT, pendingCameraUri)` 指定输出位置。
5. 拍照成功后，`onActivityResult()` 使用 `pendingCameraUri` 生成 `PendingAttachment`。
6. 拍照取消或失败时，删除预创建但未使用的 URI。

注意：很多系统相机在使用 `EXTRA_OUTPUT` 后不会通过 `data.getData()` 返回图片，所以拍照分支不能沿用现有“`data == null` 就返回”的判断。

### 5.4 权限策略

使用系统相机 Intent 拍照时，通常不需要 APP 自己申请 `CAMERA` 权限，因为拍照动作由外部相机应用完成。

建议在 `AndroidManifest.xml` 增加非强制相机特性声明：

```xml
<uses-feature
    android:name="android.hardware.camera"
    android:required="false" />
```

不要为了本次需求引入 CameraX 或自定义相机页。自定义相机页会扩大权限、生命周期和机型兼容成本，不符合当前“稳定增量”的目标。

### 5.5 附件接入

拍照成功后调用现有附件控制器：

```java
ChatAttachmentController.PendingAttachment attachment =
        attachmentController.fromUri(getContentResolver(), pendingCameraUri, true);
currentPendingAttachments.add(attachment);
renderPendingAttachments();
```

`forceImage = true`，确保拍照结果按图片附件处理。

### 5.6 失败处理

需要覆盖以下场景：

- 设备没有可用相机应用：Toast 提示“无法打开相机”。
- 用户取消拍照：不新增附件，并清理预创建 URI。
- 相机写入失败：Toast 提示“照片保存失败”。
- URI 读取失败：不影响聊天页面继续使用。

## 6. TTS 选型

### 6.1 两个方案对比

| 方案 | 适配度 | 优点 | 风险 |
| --- | --- | --- | --- |
| 端到端实时语音大模型 | 低 | 适合实时语音对话 | 会重叠 ASR、LLM、TTS 链路，容易冲击当前聊天主流程 |
| 语音合成大模型 | 高 | 适合把现有 AI 文本回复转成语音 | 只需要处理文本清洗、音频播放和密钥配置 |

### 6.2 推荐方案

推荐使用“语音合成大模型”。

原因：

1. 当前 APP 已经有稳定的文本聊天和讯飞语音转文字能力。
2. 本次需求是朗读 AI 回复，不是重做实时语音对话。
3. 语音合成可以作为聊天流的消费者，不改变 AI 生成、工具调用、商品推荐和购物车逻辑。
4. 可以明确控制只朗读正文，不朗读思考内容和卡片内容。
5. 后端可以通过 Nacos 管理火山配置，避免密钥进入 APK。

端到端实时语音大模型适合后续单独做“语音导购模式”时评估，不建议塞进当前聊天页的 TTS 开关。

## 7. TTS 安卓端方案

### 7.1 页面入口

在聊天页面右上角新增喇叭按钮。

状态：

- 关闭：显示静音或未启用态。
- 开启：显示朗读启用态。
- 不可用：灰置，并可在点击时提示“语音朗读暂不可用”。

开关状态建议持久化到本地：

```text
chat_tts_enabled = true / false
```

可使用现有本地设置存储能力。如果当前项目没有统一设置仓库，则先使用 `SharedPreferences`，避免引入额外依赖。

### 7.2 新增类建议

```text
android-native/app/src/main/java/com/xzxg/shop/tts/TtsController.java
android-native/app/src/main/java/com/xzxg/shop/tts/TtsTextSanitizer.java
android-native/app/src/main/java/com/xzxg/shop/tts/VolcengineTtsClient.java
android-native/app/src/main/java/com/xzxg/shop/tts/TtsPlaybackController.java
```

职责拆分：

| 类 | 职责 |
| --- | --- |
| `TtsController` | 管理开关、队列、取消、生命周期 |
| `TtsTextSanitizer` | 剔除 markdown、代码块、表格、链接格式和多余空白 |
| `VolcengineTtsClient` | 调用后端 TTS 代理或临时凭证后的火山接口 |
| `TtsPlaybackController` | 播放音频、停止播放、释放资源 |

### 7.3 朗读触发点

只在 AI 回复完成后触发朗读，优先做 MVP：

1. 用户发送消息。
2. AI 通过现有流式链路生成回复。
3. 回复结束后，取最终可见正文。
4. 清洗 markdown。
5. 调用 TTS。
6. 播放音频。

暂不建议第一版边生成边朗读。边生成边朗读需要处理断句、队列、回滚、卡片插入和中断，容易引入体验问题。

### 7.4 朗读内容范围

需要朗读：

- AI 回复正文。

不朗读：

- 思考内容。
- markdown 标记本身。
- 商品卡片。
- 加购卡片。
- 订单卡片。
- 工具调用 JSON。
- 代码块。
- 历史页面中已存在的旧消息，除非用户后续明确要求“点按朗读历史消息”。

### 7.5 markdown 清洗规则

`TtsTextSanitizer` 需要至少覆盖：

```text
# 标题         -> 标题文字
**加粗**       -> 加粗
*斜体*         -> 斜体
`代码`         -> 代码
[文本](链接)   -> 文本
> 引用         -> 引用内容
- 列表         -> 列表内容
1. 列表        -> 列表内容
```代码块```   -> 删除或只保留必要说明
| 表格 |        -> 删除表格分隔符，尽量转成普通句子
```

同时压缩多余空白，避免朗读停顿异常。

### 7.6 生命周期

以下情况必须停止当前朗读：

- 用户关闭 TTS 开关。
- 用户发送新消息。
- 用户离开聊天页面。
- 当前会话切换。
- APP 进入后台。
- 新一条 AI 回复开始播放。

`ChatActivity.onStop()` 或 `onDestroy()` 中应释放播放器资源，避免音频泄露。

## 8. 后端与 Nacos 配置方案

### 8.1 密钥策略

不允许安卓端直接保存火山引擎 API Key、Access Key、Secret Key 或长期 Token。

推荐后端读取 Nacos 配置，然后提供以下二选一能力：

1. 后端代理 TTS 合成请求，安卓只拿音频流。
2. 后端下发短期临时凭证，安卓再直连火山 TTS。

优先推荐后端代理。它对客户端最简单，也能统一限流、鉴权、日志和密钥轮换。

### 8.2 Nacos 配置示例

建议新增或扩展一个 voice 配置。实际字段名需要以火山引擎控制台和官方文档为准，下面是项目侧建议的配置结构：

```yaml
voice:
  tts:
    enabled: true
    provider: volcengine
    mode: speech_synthesis
    max_text_chars: 800
    timeout_ms: 15000
    volcengine:
      endpoint: ""
      app_id: ""
      access_token: ""
      resource_id: ""
      cluster: ""
      voice_type: ""
      encoding: mp3
      sample_rate: 24000
      speed_ratio: 1.0
      volume_ratio: 1.0
      pitch_ratio: 1.0
```

说明：

- `enabled` 控制服务端是否开放 TTS。
- `provider` 预留多供应商扩展。
- `mode` 固定为 `speech_synthesis`，避免和未来实时语音对话混淆。
- `max_text_chars` 用于限制单次合成长度，防止超长回复导致延迟过高。
- `endpoint`、`resource_id`、`cluster`、`voice_type` 等字段以火山最终接入要求映射。

### 8.3 后端接口建议

如果采用后端代理，建议接口：

```http
GET /api/v1/voice/tts/config
```

返回非敏感能力配置：

```json
{
  "enabled": true,
  "provider": "volcengine",
  "maxTextChars": 800
}
```

合成接口：

```http
POST /api/v1/voice/tts:synthesize
Content-Type: application/json

{
  "text": "需要朗读的正文",
  "voice": "default",
  "format": "mp3"
}
```

返回：

- `audio/mpeg` 音频流；或
- 一个短期可访问的音频 URL。

安卓端不需要知道火山密钥。

## 9. 实施步骤

### 9.1 第一阶段：拍照附件

1. `ChatActivity` 新增“拍照”入口。
2. 新增 `REQUEST_TAKE_PHOTO`。
3. 新增 `pendingCameraUri`。
4. 实现 `takePhoto()` 和 `createCameraImageUri()`。
5. 调整 `onActivityResult()`，让拍照分支不依赖 `data.getData()`。
6. 拍照成功后复用 `ChatAttachmentController.fromUri(..., true)`。
7. 补齐取消和失败清理。

### 9.2 第二阶段：TTS 基础开关

1. 聊天页右上角新增喇叭按钮。
2. 增加本地开关状态。
3. 开关关闭时不触发任何 TTS 请求。
4. 开关打开但后端未启用时提示不可用。

### 9.3 第三阶段：TTS 文本处理与播放

1. 新增 `TtsTextSanitizer`。
2. 接入 AI 回复完成事件。
3. 只抽取可见正文。
4. 清洗 markdown。
5. 调用 TTS 合成接口。
6. 播放音频。
7. 做好停止和资源释放。

### 9.4 第四阶段：后端与 Nacos

1. 后端新增 TTS 配置读取。
2. Nacos 增加 `voice.tts` 配置。
3. 后端新增 TTS config 接口。
4. 后端新增 TTS synthesize 代理接口。
5. 接入火山“语音合成大模型”。
6. 加入日志、限流和错误码。

## 10. 验收标准

### 10.1 拍照

1. 点击聊天页“+”，面板顺序为“拍照、相册、文件”。
2. 点击“拍照”能打开系统相机。
3. 拍照完成后，照片自动出现在待发送附件中。
4. 发送消息后，照片按图片附件上传。
5. 取消拍照不会新增空附件。
6. 没有相机应用时页面不崩溃，并给出提示。

### 10.2 TTS

1. 聊天页右上角有 TTS 开关。
2. 默认关闭，不影响现有聊天。
3. 开启后，AI 回复完成时自动朗读正文。
4. 不朗读思考内容。
5. 不朗读商品卡片、加购卡片、订单卡片。
6. markdown 标题、加粗、列表等格式不会被当作符号朗读。
7. 用户关闭开关时，当前朗读立即停止。
8. 用户发送新消息时，上一条朗读停止。
9. 后端 TTS 未配置或失败时，聊天功能不受影响。

## 11. 风险与规避

| 风险 | 影响 | 规避 |
| --- | --- | --- |
| API Key 进入 APK | 密钥泄露 | 只放 Nacos，由后端代理或下发短期凭证 |
| 边生成边朗读体验不稳定 | 断句错误、重复朗读 | 第一版只在回复完成后朗读 |
| markdown 清洗不完整 | 朗读符号影响体验 | 独立 `TtsTextSanitizer`，持续补规则 |
| 超长回复合成慢 | 用户等待时间长 | `max_text_chars` 限制，必要时分段 |
| 播放器生命周期泄露 | 后台仍播放或资源占用 | 页面停止、会话切换、开关关闭时统一 stop/release |
| 系统相机返回 data 为空 | 拍照附件丢失 | 使用 `EXTRA_OUTPUT` 的 `pendingCameraUri` 作为结果 |
| 拍照取消留下空图片 | 媒体库脏数据 | 取消或失败时删除预创建 URI |

## 12. 推荐结论

本次 v34 建议按以下优先级落地：

1. 先实现拍照附件，因为它完全复用现有附件链路，风险最低。
2. TTS 先采用“语音合成大模型”，不要引入端到端实时语音大模型。
3. TTS 第一版只朗读 AI 最终正文，不做流式边生成边朗读。
4. 火山配置全部放在 Nacos 和后端，安卓端只调用项目后端接口。
5. 所有新增功能都以独立模块接入，不改现有聊天主流程。
