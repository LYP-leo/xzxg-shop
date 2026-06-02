# 安卓 APP 技术方案 v24

## 1. 背景

本文针对 [安卓APP_问题_v23.md](./安卓APP_问题_v23.md) 设计下一版 Android 原生 APP 语音转文字方案。

本轮目标只有一个：实现稳定可用的语音转文字，并解决当前 Android 系统语音识别方案在模拟器、无系统语音服务设备、网络异常设备上不可控的问题。

问题文档提供了讯飞实时语音转写大模型信息：

- 服务名称：实时语音转写大模型
- 服务接口类型：WebSocket
- 接口地址：`wss://office-api-ast-dx.iflyaisol.com/`
- 推荐完整接口：`wss://office-api-ast-dx.iflyaisol.com/ast/communicate/v1?{请求参数}`
- 文档：
  - `https://www.xfyun.cn/doc/spark/asr_llm/rtasr_llm.html`
  - `https://www.xfyun.cn/doc/asr/rtasr/Android.html`

根据讯飞文档，实时语音转写大模型采用 WebSocket 实时通信，音频要求为 `16kHz`、`16bit`、单声道，`pcm` 格式；建议每 `40ms` 发送 `1280` 字节音频流。接口鉴权使用签名机制，请求参数包含 `appId`、`accessKeyId`、`utc`、`signature`、`audio_encode`、`lang`、`samplerate` 等。

注意：问题文档中已经出现明文 `APIKey` / `APISecret`。v24 实施前应把这组密钥视为已泄露密钥处理，至少在生产环境中轮换，并且不得继续把密钥写入 Android APK、Gradle、资源文件或日志。

## 2. 当前实现核对

当前 Android 端已有基础语音入口，主要代码在 `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`：

- `enterVoiceMode()`
- `startSpeechRecognizerActivity()`
- `startSpeechRecognition()`
- `sendRecognizedSpeech()`
- `speechErrorText()`
- `onActivityResult()` 处理 `REQUEST_SPEECH_INPUT`

当前依赖：

```text
Android SpeechRecognizer
Android RecognizerIntent
RECORD_AUDIO 权限
```

当前链路：

```text
输入框为空点击麦克风
  -> SpeechRecognizer.isRecognitionAvailable()
  -> 可用：startListening()
  -> 不可用：RecognizerIntent.ACTION_RECOGNIZE_SPEECH
  -> onResults()/onActivityResult()
  -> sendMessage(text)
```

当前方案的问题：

- `SpeechRecognizer` 本质依赖设备内置 `RecognitionService`，不是 APP 自带 ASR 能力。
- 模拟器、精简 ROM、无 Google/厂商语音服务的设备可能直接不可用。
- `ERROR_NETWORK` / `ERROR_NETWORK_TIMEOUT` 属于系统识别服务失败，APP 端无法修复。
- `RecognizerIntent` 只是调起系统语音 UI，仍依赖设备是否有可处理的 Activity。
- 设备差异会导致同一份代码表现不一致，无法作为稳定演示和交付方案。

因此 v24 主线不能继续只修 `SpeechRecognizer`，必须新增项目可控的 ASR 通道。

## 3. 可行方案调研结论

### 3.1 Android 系统语音识别

方案：

- `SpeechRecognizer`：APP 内联识别，支持 partial/final 回调。
- `RecognizerIntent`：打开系统语音识别界面，由系统返回结果。

优点：

- 集成成本最低。
- 不新增三方 SDK。
- 当前代码已有基础实现。

缺点：

- 不可控，依赖系统语音服务。
- 模拟器和部分国产/精简设备容易不可用。
- 不适合作为 v24 的唯一方案。

结论：继续保留为降级方案，不再作为主方案。

### 3.2 讯飞 Android SDK

讯飞实时语音转写 Android SDK 可直接集成到客户端。

优点：

- 官方 Android SDK，客户端接入路径清晰。
- 相比系统语音识别更可控。
- 可以获得较完整的实时转写能力。

缺点：

- APPID、APIKey、APISecret 放入 APK 后可被逆向提取。
- SDK 依赖和初始化会增加客户端复杂度。
- 多端复用差，后续 iOS/Web/小程序需要分别接入。
- 当前仓库只有 Android 工程，没有后端代码，直接接 SDK 虽然最快，但安全边界较差。

结论：可作为短期演示备选，不建议作为生产主线。

### 3.3 讯飞实时语音转写大模型 WebSocket

问题文档明确给出的是 WebSocket 服务。根据讯飞文档，实时转写大模型适合流式上传音频并实时返回识别结果。客户端需要发送符合服务要求的音频流，服务端通过 WebSocket 返回转写事件。

关键实现要点：

- WebSocket 接口按文档使用 `/ast/communicate/v1`，完整地址由后端配置拼接，例如 `wss://office-api-ast-dx.iflyaisol.com/ast/communicate/v1?{请求参数}`。
- 握手参数至少包含：
  - `appId`：讯飞应用 ID。
  - `accessKeyId`：讯飞 APIKey。
  - `utc`：当前时间，按文档格式生成。
  - `signature`：用 APISecret 对排序后的参数 baseString 做 `HmacSHA1`，再 `Base64`。
  - `audio_encode=pcm_s16le`。
  - `lang=autodialect`，支持中英和方言混合识别。
  - `samplerate=16000`。
- 参数需要 URL encode；签名参数本身不参与 baseString 排序。
- 音频建议使用 PCM：`16kHz`、`16bit`、单声道。
- Android 端使用 `AudioRecord` 采集 PCM，比 `MediaRecorder` 更适合实时 WebSocket 分片。
- 常见发送节奏按 `40ms` 一帧，即 `1280 bytes` 左右的 PCM 数据。
- 握手成功后持续发送 binary message，内容为音频二进制数据。
- 音频发送完成后发送结束 JSON：`{"end": true, "sessionId": "<sid>"}`。
- 讯飞结果中 `data.cn.st.type=1` 表示中间结果，`type=0` 表示确定性结果；`data.ls=true` 表示最后一帧。
- 连接鉴权需要 `APPID`、`APIKey`、`APISecret` 等凭证参与签名，凭证必须放后端。

优点：

- 不依赖设备系统语音识别服务。
- 支持实时转写，用户体验比“录完再传”更好。
- 后端集中管理密钥、鉴权、限流、日志和供应商切换。

缺点：

- 需要后端新增 WebSocket 代理或 HTTP/SSE 转发接口。
- Android 端要处理录音帧、连接生命周期、partial/final 文本更新。
- 比短音频上传方案复杂。

结论：这是 v24 推荐主方案。

### 3.4 短音频上传到后端 STT

Android 用 `MediaRecorder` 录制 `m4a/aac`，停止后上传本项目后端，由后端调用讯飞或其他 STT。

优点：

- Android 实现简单。
- 后端仍可保护密钥。
- 适合“按一下开始，再按一下结束”的短语音输入。

缺点：

- 不是实时转写，用户说完后才知道结果。
- 无法实时显示 partial 文本。
- 讯飞本轮给出的重点服务是实时 WebSocket，短音频上传不是最贴合的问题输入。

结论：作为 v24 降级实现，后端实时代理来不及完成时可先落地。

### 3.5 离线开源 STT

可选：

- Vosk Android
- whisper.cpp Android / JNI

优点：

- 不依赖网络。
- 不依赖系统语音服务。
- 不暴露云端密钥。

缺点：

- 中文模型体积、准确率、延迟都需要单独评估。
- whisper.cpp 需要 NDK/JNI、模型管理和性能优化。
- 中低端设备耗电和发热风险高。

结论：不作为 v24 主线，放到 v25+ 的离线增强路线。

## 4. v24 总体方案

v24 采用“后端代理讯飞实时 WebSocket + Android AudioRecord 实时上传 + 系统识别降级”的方案。

推荐链路：

```text
Android 麦克风
  -> AudioRecord 采集 16k/16bit/mono PCM
  -> WebSocket 连接本项目后端 /api/v1/speech/realtime
  -> 后端持有讯飞密钥并连接讯飞实时转写大模型 WebSocket
  -> 后端转发 partial/final 文本事件
  -> Android 展示临时识别文本
  -> final 文本直接 sendMessage(text)
```

降级链路：

```text
后端实时 STT 不可用
  -> Android SpeechRecognizer
  -> RecognizerIntent
  -> 提示用户使用键盘或输入法语音
```

语音识别模式：

```java
private enum SpeechMode {
    AUTO,
    XUNFEI_REALTIME,
    ANDROID_INLINE,
    ANDROID_ACTIVITY
}
```

默认使用 `AUTO`：

1. 优先连接后端实时 STT。
2. 后端返回 `speech_not_enabled` / `404` / `501` 时降级系统内联识别。
3. 系统内联识别不可用时降级 `RecognizerIntent`。
4. 仍不可用时提示“当前设备不支持语音输入”。

## 5. 后端接口设计

当前仓库没有后端代码，但 Android 默认后端地址已经是 `/api/v1`。因此 v24 需要后端新增一个实时语音代理接口。

### 5.1 Android 到项目后端

新增：

```http
GET /api/v1/speech/realtime?language=zh-CN&format=pcm&sample_rate=16000
Upgrade: websocket
Authorization: Bearer <token>
```

Android 发送二进制消息：

```text
PCM frame
  sample_rate: 16000
  sample_size: 16bit
  channels: 1
  frame_interval: 40ms
  frame_bytes: 1280
```

Android 发送控制消息：

```json
{"type":"start","language":"zh-CN","format":"pcm","sample_rate":16000}
{"type":"end"}
{"type":"cancel"}
```

后端收到 `end` 后负责向讯飞发送：

```json
{"end": true, "sessionId": "<xunfei_sid>"}
```

后端返回事件：

```json
{"type":"ready"}
{"type":"partial","text":"我想买","seq":1}
{"type":"partial","text":"我想买粉底液","seq":2}
{"type":"final","text":"我想买一支适合油皮的粉底液","seq":3}
{"type":"error","code":"speech_recognition_failed","message":"语音识别失败，请重试"}
{"type":"closed"}
```

Android 只在 `final.text` 非空时调用 `sendMessage(text)`。`partial.text` 只用于输入框 hint 或临时状态，不发送，避免重复消息。

### 5.2 项目后端到讯飞

后端负责：

1. 从环境变量读取讯飞配置。
2. 构造讯飞 WebSocket 鉴权参数和签名。
3. 连接讯飞实时转写大模型 WebSocket。
4. 把 Android PCM frame 转发给讯飞。
5. 把讯飞 partial/final 转写结果归一化为本项目事件。
6. 超时、取消、异常时关闭两侧 WebSocket。

建议后端配置：

```text
SPEECH_PROVIDER=xunfei_realtime
XUNFEI_APP_ID=...
XUNFEI_API_KEY=...
XUNFEI_API_SECRET=...
XUNFEI_RTASR_BASE_URL=wss://office-api-ast-dx.iflyaisol.com
XUNFEI_RTASR_PATH=/ast/communicate/v1
XUNFEI_RTASR_AUDIO_ENCODE=pcm_s16le
XUNFEI_RTASR_LANG=autodialect
SPEECH_LANGUAGE=zh-CN
SPEECH_SAMPLE_RATE=16000
SPEECH_MAX_DURATION_SECONDS=60
SPEECH_FRAME_MS=40
```

签名生成规则：

```text
1. 组织请求参数，排除 signature。
2. 参数名升序排序。
3. 对 key/value 分别 URL encode。
4. 拼接为 key=value&key=value 形式，得到 baseString。
5. 使用 APISecret 对 baseString 做 HmacSHA1。
6. 对 HmacSHA1 结果做 Base64，得到 signature。
7. 将 signature URL encode 后放入 WebSocket URL。
```

建议请求参数：

```text
appId=<XUNFEI_APP_ID>
accessKeyId=<XUNFEI_API_KEY>
uuid=<account_id_or_request_id>
utc=<current_time>
audio_encode=pcm_s16le
lang=autodialect
samplerate=16000
signature=<generated_signature>
```

密钥要求：

- 不把 `APIKey` / `APISecret` 写入 Android 代码、Gradle、资源文件、日志或崩溃上报。
- 问题文档里出现过的密钥应尽快在讯飞控制台轮换。
- 后端日志只打印 request id、耗时、错误码，不打印签名明文、音频内容和完整鉴权 URL。
- 如果短期没有后端，只允许 debug 包通过本机配置注入密钥直连讯飞；release 包必须禁用直连。

### 5.3 后端错误码

统一错误：

| code | 场景 | Android 行为 |
| --- | --- | --- |
| `speech_not_enabled` | 后端未配置 STT | AUTO 降级系统识别 |
| `speech_auth_failed` | 讯飞鉴权失败 | toast，显式模式不降级 |
| `speech_network_error` | 后端到讯飞网络失败 | toast，恢复输入态 |
| `speech_timeout` | 长时间无识别结果 | toast “未检测到语音” |
| `speech_no_text` | final 为空 | toast “没有识别到内容” |
| `speech_too_long` | 超过最大录音时长 | 自动结束并尝试发送已有 final |
| `speech_recognition_failed` | 其他识别失败 | toast “语音识别失败，请重试” |

讯飞错误码映射建议：

| 讯飞错误码 | 含义 | 本项目 code |
| --- | --- | --- |
| `35001` / `100002` | 鉴权或签名失败 | `speech_auth_failed` |
| `35002` / `35022` | 用量不足或超限 | `speech_quota_exceeded` |
| `35004` / `35005` | appId 不存在或被禁用 | `speech_auth_failed` |
| `35006` / `37002` | 并发路数已满 | `speech_too_many_connections` |
| `35014` / `35030` | 时间戳或签名重复问题 | `speech_auth_failed` |
| `37005` | 长时间未传音频 | `speech_timeout` |
| `37007` | 单次音频时长到上限 | `speech_too_long` |
| `100001` | 音频上传过快 | `speech_upload_too_fast` |

## 6. Android 端实现设计

### 6.1 依赖选择

当前 Android 工程只有基础依赖。v24 推荐加入 OkHttp WebSocket：

```gradle
implementation 'com.squareup.okhttp3:okhttp:4.12.0'
```

原因：

- Android 原生没有易用 WebSocket 客户端。
- OkHttp 稳定、体积可控、API 简单。
- 后续聊天 SSE/HTTP 也可逐步复用，但 v24 只用于语音 WebSocket。

如果不想引入依赖，也可以使用 `org.java-websocket`，但 OkHttp 更适合当前 Android 项目。

### 6.2 新增类

新增文件：

```text
android-native/app/src/main/java/com/xzxg/shop/SpeechRealtimeClient.java
android-native/app/src/main/java/com/xzxg/shop/PcmRecorder.java
```

职责：

```text
MainActivity
  管 UI 状态、权限、按钮、sendMessage()

SpeechRealtimeClient
  连接 /speech/realtime WebSocket
  发送 start/end/cancel
  发送 PCM frame
  解析 ready/partial/final/error

PcmRecorder
  AudioRecord 初始化
  16k/16bit/mono 采样
  后台线程每 40ms 输出 PCM frame
  stop/release
```

### 6.3 录音参数

使用 `AudioRecord`：

```java
int sampleRate = 16000;
int channelConfig = AudioFormat.CHANNEL_IN_MONO;
int encoding = AudioFormat.ENCODING_PCM_16BIT;
int frameBytes = sampleRate * 2 * 40 / 1000; // 1280
int minBuffer = AudioRecord.getMinBufferSize(sampleRate, channelConfig, encoding);
int bufferSize = Math.max(minBuffer, frameBytes * 4);
```

采集规则：

- `RECORD_AUDIO` 权限通过后才能初始化。
- 每次读取尽量按 `1280 bytes` 发送。
- 不足一帧可以缓存到下一次，不建议频繁发送很小包。
- 录音最长 60 秒，达到上限自动发送 `end`。
- 录音开始后 800ms 内停止，提示“说话时间太短”并发送 `cancel`。

### 6.4 MainActivity 状态机

新增字段：

```java
private SpeechMode speechMode = SpeechMode.AUTO;
private SpeechRealtimeClient speechRealtimeClient;
private PcmRecorder pcmRecorder;
private boolean realtimeVoiceActive;
private boolean realtimeVoiceFinalSent;
private String realtimePartialText = "";
private long realtimeVoiceStartedAt;
private Handler voiceHandler = new Handler(Looper.getMainLooper());
```

状态：

```text
idle
  -> connecting
  -> listening
  -> final_received
  -> idle

idle
  -> connecting/listening
  -> error
  -> fallback_or_idle
```

按钮规则：

| 状态 | 右侧按钮 | 点击行为 |
| --- | --- | --- |
| 输入框有文字 | 发送 | `sendMessage(inputText)` |
| idle 且输入框为空 | 麦克风 | `enterVoiceMode()` |
| connecting/listening | 停止 | `finishRealtimeVoice(true)` |
| final 发送中 | 禁用 | 防止重复 |
| chat streaming | 停止生成 | 保持现有逻辑 |

提示：

| 场景 | 文案 |
| --- | --- |
| 连接中 | `正在连接语音识别...` |
| 录音中无 partial | `正在聆听...` |
| 有 partial | 输入框 hint 显示 partial 文本 |
| final 为空 | `没有识别到内容` |
| 录音太短 | `说话时间太短` |
| 权限拒绝 | `需要麦克风权限才能使用语音输入` |
| 服务不可用 | `当前语音识别不可用，已切换系统语音` |

### 6.5 入口流程

```java
private void enterVoiceMode() {
    if (!hasRecordAudioPermission()) {
        requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, REQUEST_RECORD_AUDIO);
        return;
    }
    SpeechMode mode = currentSpeechMode();
    if (mode == SpeechMode.XUNFEI_REALTIME || mode == SpeechMode.AUTO) {
        startRealtimeSpeech();
        return;
    }
    if (mode == SpeechMode.ANDROID_INLINE) {
        startSpeechRecognition();
        return;
    }
    startSpeechRecognizerActivity();
}
```

实时识别：

```java
private void startRealtimeSpeech() {
    realtimeVoiceActive = true;
    realtimeVoiceFinalSent = false;
    realtimePartialText = "";
    realtimeVoiceStartedAt = System.currentTimeMillis();
    input.setText("");
    input.setHint("正在连接语音识别...");
    hideKeyboard();
    updateInputActionButtonState();

    speechRealtimeClient = new SpeechRealtimeClient(sessionStore, new SpeechRealtimeClient.Listener() {
        public void onReady() {
            runOnUiThread(() -> input.setHint("正在聆听..."));
            pcmRecorder.start(frame -> speechRealtimeClient.sendAudio(frame));
        }
        public void onPartial(String text) {
            runOnUiThread(() -> {
                realtimePartialText = text == null ? "" : text.trim();
                if (!realtimePartialText.isEmpty()) {
                    input.setHint(realtimePartialText);
                }
            });
        }
        public void onFinal(String text) {
            runOnUiThread(() -> sendRealtimeSpeechFinal(text));
        }
        public void onError(String code, String message) {
            runOnUiThread(() -> handleRealtimeSpeechError(code, message));
        }
    });
    speechRealtimeClient.connect();
}
```

结束识别：

```java
private void finishRealtimeVoice(boolean userStop) {
    long duration = System.currentTimeMillis() - realtimeVoiceStartedAt;
    if (duration < 800 && userStop) {
        toastLine("说话时间太短");
        cancelRealtimeVoice();
        return;
    }
    if (pcmRecorder != null) {
        pcmRecorder.stop();
    }
    if (speechRealtimeClient != null) {
        speechRealtimeClient.sendEnd();
    }
    input.setHint("正在识别语音...");
    updateInputActionButtonState();
}
```

final 发送：

```java
private void sendRealtimeSpeechFinal(String text) {
    if (realtimeVoiceFinalSent) {
        return;
    }
    String value = text == null ? "" : text.trim();
    if (value.isEmpty()) {
        value = realtimePartialText == null ? "" : realtimePartialText.trim();
    }
    if (value.isEmpty()) {
        toastLine("没有识别到内容");
        cleanupRealtimeVoice();
        return;
    }
    realtimeVoiceFinalSent = true;
    cleanupRealtimeVoice();
    sendMessage(value);
}
```

### 6.6 WebSocket URL 构造

Android 不直接连接讯飞，连接本项目后端：

```text
http://82.156.207.98:8080/api/v1
  -> ws://82.156.207.98:8080/api/v1/speech/realtime

https://example.com/api/v1
  -> wss://example.com/api/v1/speech/realtime
```

`SpeechRealtimeClient` 从 `sessionStore.apiBase()` 转换：

```java
String base = sessionStore.apiBase();
String wsBase = base.startsWith("https://")
        ? "wss://" + base.substring("https://".length())
        : "ws://" + base.substring("http://".length());
String url = wsBase + "/speech/realtime?language=zh-CN&format=pcm&sample_rate=16000";
```

请求头：

```text
Authorization: Bearer <session token>
X-Client: android-native
```

### 6.7 系统识别兼容保留

保留现有 `SpeechRecognizer` 和 `RecognizerIntent`，但做两点修正：

1. Android 11+ manifest 增加 queries：

```xml
<queries>
    <intent>
        <action android:name="android.speech.RecognitionService" />
    </intent>
    <intent>
        <action android:name="android.speech.action.RECOGNIZE_SPEECH" />
    </intent>
</queries>
```

2. `SpeechRecognizer.isRecognitionAvailable(this)` 为 false 时不要直接判死，继续检查 `RecognizerIntent` 是否可 resolve。

```java
private boolean canResolveRecognizerIntent() {
    Intent intent = new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);
    return intent.resolveActivity(getPackageManager()) != null;
}
```

## 7. 后端代理实现细节

### 7.1 为什么必须后端代理

讯飞实时转写需要密钥参与鉴权。若 Android 直接连接讯飞：

- APK 可被反编译拿到密钥。
- 请求签名逻辑也会暴露。
- 无法集中做限流和费用控制。
- 密钥轮换需要重新发版。

后端代理后：

- Android 只有本项目 token。
- 讯飞密钥只在服务端环境变量。
- 后端可以按用户、设备、IP 做限流。
- 后续可无感切换 OpenAI、阿里云、百度等 STT provider。

### 7.2 连接生命周期

```text
Android connect /speech/realtime
  -> 后端校验登录 token
  -> 后端检查 STT 配置
  -> 后端连接讯飞 WebSocket
  -> 后端返回 ready
  -> Android 开始发送 PCM frame
  -> Android 发送 end
  -> 后端转发结束帧
  -> 讯飞返回 final
  -> 后端返回 final
  -> 双方关闭连接
```

异常处理：

- Android 断开：后端关闭讯飞连接。
- 讯飞断开：后端返回 `error` 后关闭 Android 连接。
- Android 长时间不发音频：后端超时关闭。
- 超过最大录音时长：后端主动结束并返回当前结果或错误。

### 7.3 转写事件归一化

讯飞返回结构不要直接透传给 Android。后端统一转换为：

```json
{
  "type": "partial",
  "text": "我想买",
  "seq": 12,
  "provider": "xunfei_realtime"
}
```

```json
{
  "type": "final",
  "text": "我想买一支适合油皮的粉底液",
  "seq": 18,
  "provider": "xunfei_realtime",
  "duration_ms": 4200
}
```

Android 只消费本项目协议，不感知识别供应商细节。

讯飞文本解析规则：

```text
1. 只处理 msg_type=result 且 res_type=asr 的消息。
2. 从 data.cn.st.rt[].ws[].cw[].w 取词文本并拼接。
3. 忽略或按需处理 wp=s 的顺滑词、wp=p 的标点、wp=g 的分段标识。
4. data.cn.st.type=1 -> partial。
5. data.cn.st.type=0 -> stable/final segment。
6. data.ls=true -> 本轮转写结束，可向 Android 发 final。
```

后端需要维护会话内文本累积：

```text
partial_text: 当前中间结果，仅用于 Android 展示
stable_segments: 已确定句段
final_text: stable_segments + 最后一段确定结果
```

### 7.4 限流与安全

后端限制：

- 单次最长 60 秒。
- 单用户同时只能 1 路实时语音连接。
- 未登录不允许使用。
- 连接建立后 10 秒内没有音频帧则断开。
- 每分钟连接次数限制，例如每用户 10 次。
- 服务端不落盘音频，除非 debug 配置显式开启。

## 8. 短期备选：Android 直连接讯飞

如果后端短期完全无法改动，为了演示可以做临时 Android 直连方案：

```text
Android AudioRecord
  -> Android 本地生成讯飞鉴权 URL
  -> 直连 wss://office-api-ast-dx.iflyaisol.com/ast/communicate/v1
  -> 解析讯飞结果
  -> sendMessage(finalText)
```

但必须明确：

- 只允许 debug / demo 包使用。
- 密钥通过本地 `local.properties` 或构建环境注入，不提交 Git。
- release 包必须关闭。
- 一旦演示结束，立即在讯飞控制台轮换密钥。

不建议把问题文档里的明文密钥继续复制到代码或 Gradle。

## 9. 实施计划

### Phase 1：后端实时语音代理

1. 新增 `/api/v1/speech/realtime` WebSocket。
2. 校验用户 token。
3. 读取讯飞环境变量。
4. 连接讯飞实时转写大模型 WebSocket。
5. 转发 Android PCM frame。
6. 把讯飞结果归一化为 `ready/partial/final/error`。
7. 增加时长、并发、空闲超时限制。

### Phase 2：Android WebSocket 客户端

1. `app/build.gradle` 增加 OkHttp。
2. 新增 `SpeechRealtimeClient`。
3. 从 `sessionStore.apiBase()` 构造 ws/wss URL。
4. 加 Authorization header。
5. 支持 `connect/sendAudio/sendEnd/cancel/close`。
6. 解析 `ready/partial/final/error`。

### Phase 3：Android PCM 采集

1. 新增 `PcmRecorder`。
2. 使用 `AudioRecord` 采集 16k/16bit/mono。
3. 按 40ms frame 输出 `byte[]`。
4. 后台线程读取音频，UI 线程只处理状态。
5. stop 时释放 `AudioRecord`。

### Phase 4：MainActivity 状态机接入

1. `enterVoiceMode()` 优先走实时语音。
2. 录音中按钮变为停止。
3. partial 文本只更新 hint，不发送。
4. final 文本直接 `sendMessage(text)`。
5. 防止 final、partial fallback、系统识别重复发送。
6. 页面销毁时释放 WebSocket、AudioRecord、SpeechRecognizer。

### Phase 5：降级与设置

1. 新增 `SpeechMode`，默认 `AUTO`。
2. 高级设置可选：
   - 自动
   - 讯飞实时转写
   - 系统内联识别
   - 系统语音界面
3. AUTO 模式实时接口不可用时降级系统识别。
4. 显式模式失败时不自动切换，便于调试。

### Phase 6：验收

1. 真机测试实时转写。
2. 模拟器测试实时转写。
3. 后端未配置讯飞时测试降级。
4. 断网测试错误提示。
5. 权限拒绝测试。
6. 录音太短、无文本、超时测试。
7. 连续点击麦克风测试重复请求保护。

## 10. 验收标准

### 10.1 基础功能

- 输入框为空时右侧显示麦克风。
- 点击麦克风后开始录音。
- 说话过程中可以看到 partial 识别文本或“正在聆听...”状态。
- 点击停止或识别结束后，final 文本直接作为用户消息发送。
- 用户不需要再点一次发送。

### 10.2 讯飞实时转写

- 在无系统语音服务的模拟器上，只要后端实时 STT 可用，也能完成语音转文字。
- Android APK 不包含讯飞 `APIKey` / `APISecret`。
- 录音音频按 16k/16bit/mono PCM 发送。
- 单次录音超过上限自动结束。
- 后端关闭或讯飞异常时 APP 不崩溃。

### 10.3 降级

- `/speech/realtime` 返回 `speech_not_enabled` 时，AUTO 模式降级系统识别。
- `SpeechRecognizer` 不可用时，尝试 `RecognizerIntent`。
- 系统语音也不可用时，提示“当前设备不支持语音输入”。
- 显式选择“讯飞实时转写”时失败不自动切换，方便定位问题。

### 10.4 状态一致性

- 录音中再次点击右侧按钮会停止录音并等待 final。
- final 只发送一次。
- partial 不会发送为聊天消息，除非 final 为空且用户已停止时作为兜底。
- 失败后输入框 hint 恢复为“输入问题或直接发送...”。
- 离开页面或 Activity 销毁时释放录音和 WebSocket。

## 11. 风险与取舍

### 11.1 为什么不只继续修 Android 系统语音

系统语音识别不是 APP 自带能力，而是对设备语音服务的调用。设备没有服务、服务不可联网、模拟器缺少组件时，APP 无法靠代码修复。因此它只能作为降级，不应作为 v24 主线。

### 11.2 为什么不把讯飞密钥放 Android

Android 包可以被反编译。把讯飞 `APIKey` / `APISecret` 放客户端，会造成密钥泄露、费用滥用、签名逻辑暴露和轮换困难。v24 主线必须把密钥放后端。

### 11.3 为什么用 AudioRecord 而不是 MediaRecorder

讯飞实时转写是 WebSocket 流式接口，需要持续发送小块音频。`AudioRecord` 可以直接拿到 PCM frame，适合实时转写；`MediaRecorder` 更适合录完一个文件再上传，不适合作为实时 WebSocket 主链路。

### 11.4 为什么仍保留短音频上传方案

如果后端团队短期无法完成 WebSocket 双向代理，短音频上传是更快的中间方案：

```text
Android MediaRecorder 录 m4a
  -> POST /api/v1/speech:recognize
  -> 后端调用 STT
  -> 返回 text
```

它不如实时转写体验好，但仍比系统语音识别可控，并且不暴露密钥。

## 12. v24 优先级

必做：

1. 后端 `/api/v1/speech/realtime` 代理讯飞 WebSocket。
2. Android `SpeechRealtimeClient`。
3. Android `PcmRecorder`。
4. MainActivity 语音状态机接入。
5. AUTO 降级到现有系统识别。
6. 密钥移到后端环境变量，问题文档中的明文密钥生产前轮换。

可选：

1. 高级设置增加语音识别模式选择。
2. 识别中显示实时 partial 文本。
3. 短音频上传 `/api/v1/speech:recognize` 作为备用。

后续 v25+：

1. Vosk / whisper.cpp 离线识别。
2. 连续语音对话。
3. TTS 播报联动。
4. 多语言识别和自动语言检测。
