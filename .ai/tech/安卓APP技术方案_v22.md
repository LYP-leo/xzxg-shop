# 安卓 APP 技术方案 v22

## 1. 背景

本文针对 [安卓APP_问题_v21.md](./安卓APP_问题_v21.md) 设计下一版 Android 原生 APP 修复方案。

本轮聚焦三个区域：

- 聊天页面：当前还不支持语音转文字输入，需要补齐语音识别能力；输入框有文字时右侧按钮必须是发送按钮；表格流式中间态代码框不显示 `plain text` 等语言标识。
- 侧栏：替换 AI 导购、商品、购物车、订单 四个入口左侧不相关图标。
- 账号管理：头像统一圆形展示，美化手机号/邮箱设置弹窗，并对手机号和邮箱输入做校验。

本方案只涉及 Android 原生端；后端接口沿用现有账号资料接口和聊天接口，不新增后端能力。

## 2. 当前问题定位

### 2.1 聊天页语音输入

当前 APP 还无法完成“语音转文字输入”。用户点击麦克风后，预期是系统开始录音识别，识别出的文字直接作为用户消息发送；但现状无法稳定完成这条链路，因此本轮必须把语音转文字作为核心功能补齐，而不是只调整按钮图标。

当前 `MainActivity` 已存在语音相关字段和方法：

- `SpeechRecognizer`
- `RecognizerIntent`
- `voiceMode`
- `startVoiceInput()`
- `stopVoiceInput()`
- `ensureSpeechRecognizer()`
- `requestAudioPermissionAndStartVoice()`

但交互仍需要收敛：

1. 必须真正接入系统语音识别，完成语音转文字。
2. 识别出的文字必须直接发送为用户消息。
3. 右侧动作按钮的状态必须以输入框文本优先。
4. 输入框有内容时，无论是否支持语音，都只能显示发送态。
5. 点击麦克风识别到文本后应直接发送，不再回填等待用户再次点击。
6. 权限缺失、识别不可用、网络错误、无匹配内容需要给出明确 toast，并恢复输入态。

### 2.2 表格流式中间态

表格流式中间态由 `startActiveFormCodeBox()` 创建，目前会额外添加一个 TextView：

```java
language.setText("plain text");
activeAssistantFormCodeBox.addView(language, ...);
```

用户要求代码框上方不展示任何语言标识。因此代码框只保留原文内容区域，不再创建语言标签。

### 2.3 侧栏图标

当前侧栏四个导航项使用：

```java
drawerNavButton("✦", "AI导购", ...)
drawerNavButton("▣", "商品", ...)
drawerNavButton("□", "购物车", ...)
drawerNavButton("≡", "订单", ...)
```

图标语义弱，尤其商品/购物车/订单不够直观。由于项目当前未引入图标库，本版继续使用稳定 Unicode 符号，避免新增依赖和资源维护成本。

### 2.4 账号管理头像和联系人弹窗

头像目前有多处展示：

- 设置页大头像：`accountAvatarView(dp(118), 36)`
- 个人资料页头像：`accountAvatarView(dp(56), 18)`
- 编辑资料页头像预览：`avatarInitial` / `avatarImage`
- 头像预览页：`ImageView`
- 底部用户栏头像：`accountAvatarView(...)`

本轮要求所有用户头像都显示为圆形。文本首字母头像已经使用圆角背景，但远程图片头像需要统一裁剪为圆形，不能只设置圆角背景。

手机号/邮箱弹窗当前由 `editContact(boolean phone)` 使用默认 `AlertDialog + EditText` 实现：

```java
new AlertDialog.Builder(this)
    .setTitle(...)
    .setView(edit)
    .setPositiveButton("保存", ...)
```

问题：

- 默认样式和 APP 设计不一致。
- 输入框没有实时错误提示。
- 保存按钮不会根据输入合法性禁用。
- 手机号和邮箱没有格式校验。

## 3. 总体方案

```text
聊天输入栏
  TextWatcher / voiceMode / streaming
    -> updateActionButtonState()
       有文字: 发送按钮
       无文字 + 非语音: 麦克风按钮
       语音中: 键盘/停止语音按钮

语音识别
  麦克风按钮
    -> 权限检查
    -> startListening()
    -> onResults(text)
       -> sendMessage(text)

表格中间态
  <form> start
    -> 创建无标题代码框
    -> 流式展示原文
  </form>
    -> 移除代码框
    -> 渲染表格

侧栏导航
  替换 Unicode 图标

账号管理
  圆形头像 ImageView
  自定义联系人弹窗
    -> 实时校验
    -> 保存按钮禁用/启用
    -> updateContact()
```

## 4. 聊天页面改造

### 4.1 动作按钮状态规则

新增统一方法：

```java
private void updateInputActionButtonState() {
    String text = input == null ? "" : input.getText().toString().trim();
    if (streaming) {
        setActionButtonText("■");
        return;
    }
    if (!text.isEmpty()) {
        setActionButtonText("➤");
        return;
    }
    setActionButtonText(voiceMode ? "⌨" : "🎙");
}
```

所有现有直接调用：

```java
setActionButtonText(voiceMode ? "⌨" : (input.getText().toString().trim().isEmpty() ? "🎙" : "➤"));
```

统一替换为 `updateInputActionButtonState()`。

按钮点击规则：

```text
streaming -> stop stream
input text not empty -> sendCurrentInput()
voiceMode -> stopVoiceInput()
otherwise -> requestAudioPermissionAndStartVoice()
```

这样保证“输入框中有文字时，右侧按钮一定是发送按钮”。

### 4.2 语音输入发送规则

`RecognitionListener.onResults()`：

1. 取第一条识别结果。
2. trim 后为空则 toast “没有识别到内容”。
3. 非空则直接调用 `sendMessage(text)`。
4. 调用前清理输入框焦点和键盘，避免语音发送后输入栏残留状态。

`onPartialResults()` 只用于临时状态显示，不直接发送，避免部分识别和最终识别重复发送。若当前代码已经在 `onEndOfSpeech()` 使用 `lastPartialSpeech` 兜底发送，需要改为只在最终 `onResults()` 缺失时兜底一次，并用布尔值 `voiceResultSent` 防重复。

建议字段：

```java
private boolean voiceResultSent;
private String lastPartialSpeech = "";
```

发送函数：

```java
private void sendRecognizedSpeech(String text) {
    String value = text == null ? "" : text.trim();
    if (value.isEmpty() || voiceResultSent) return;
    voiceResultSent = true;
    stopVoiceInput();
    sendMessage(value);
}
```

### 4.3 权限和错误处理

麦克风入口：

- 如果 `SpeechRecognizer.isRecognitionAvailable(this)` 为 false，toast “当前设备不支持语音输入”。
- 如果缺少 `RECORD_AUDIO` 权限，请求权限；拒绝后 toast “需要麦克风权限才能使用语音输入”。
- `ERROR_NO_MATCH`：toast “没有识别到内容”。
- `ERROR_SPEECH_TIMEOUT`：toast “未检测到语音”。
- `ERROR_NETWORK` / `ERROR_NETWORK_TIMEOUT`：toast “语音识别网络异常，请稍后重试”。
- 其他错误：toast “语音识别失败，请重试”。

所有错误分支都必须：

```java
voiceMode = false;
updateInputActionButtonState();
```

### 4.4 表格中间态代码框去语言标识

修改 `startActiveFormCodeBox()`：

- 删除 `language` TextView 创建和添加。
- `activeAssistantFormCodeText` 顶部 padding 从 `dp(6)` 调整为 `0` 或 `dp(2)`。
- 代码框仍保留浅灰背景、等宽字体、原文流式更新。

目标结构：

```text
activeAssistantFormCodeBox
  activeAssistantFormCodeText
```

不要出现：

```text
plain text
markdown
code
```

## 5. 侧栏图标改造

### 5.1 图标映射

使用语义更明确的 Unicode 图标：

| 页面 | 当前 | 新图标 | 理由 |
| --- | --- | --- | --- |
| AI导购 | `✦` | `💬` | 聊天/对话语义 |
| 商品 | `▣` | `🏷` | 商品/标签/价格语义 |
| 购物车 | `□` | `🛒` | 购物车直观语义 |
| 订单 | `≡` | `📦` | 包裹/订单履约语义 |

如果目标设备字体对 emoji 显示不稳定，则使用无彩色备选：

| 页面 | 备选 |
| --- | --- |
| AI导购 | `◎` |
| 商品 | `◇` |
| 购物车 | `⌑` |
| 订单 | `▤` |

优先采用 emoji 方案；若模拟器出现缺字方框，再切换备选方案。

### 5.2 样式要求

`drawerNavButton()` 内的 icon TextView：

- 固定宽高，避免 emoji 字形导致行高抖动。
- `Gravity.CENTER`。
- 字号建议 20 到 22。
- 文本仍左对齐，按钮高度不变。

## 6. 账号管理头像圆形化

### 6.1 新增圆形 ImageView

新增内部类：

```java
private static class CircleImageView extends AppCompatImageView 或 ImageView {
    Paint paint;
    Path clipPath;
    @Override protected void onDraw(Canvas canvas) {
        canvas.save();
        canvas.clipPath(circlePath);
        super.onDraw(canvas);
        canvas.restore();
    }
}
```

由于当前项目是原生 Activity，不依赖 AppCompat，建议继承 `ImageView`：

```java
private static class CircleImageView extends ImageView {
    private final Path path = new Path();
    @Override
    protected void onSizeChanged(int w, int h, int oldw, int oldh) {
        path.reset();
        path.addCircle(w / 2f, h / 2f, Math.min(w, h) / 2f, Path.Direction.CW);
    }
    @Override
    protected void onDraw(Canvas canvas) {
        int save = canvas.save();
        canvas.clipPath(path);
        super.onDraw(canvas);
        canvas.restoreToCount(save);
    }
}
```

需要新增 import：

```java
import android.graphics.Canvas;
import android.graphics.Path;
```

### 6.2 替换头像图片展示

`accountAvatarView(sizePx, textSp)`：

- 有头像 URL：返回 `CircleImageView`，`ScaleType.CENTER_CROP`。
- 无头像 URL：返回 TextView，但背景半径必须是 `sizePx / 2`。

编辑资料页：

- `avatarImage` 改为 `CircleImageView`。
- 预览图片仍 `CENTER_CROP`。
- `avatarInitial` 保持圆形背景。

头像预览页：

- 如果是用户头像预览，也使用圆形裁剪或在预览页中明确展示圆形头像。

历史会话左侧图标不是用户头像，不纳入本轮“用户头像”范围。

## 7. 手机号和邮箱弹窗改造

### 7.1 自定义弹窗布局

替换 `editContact(boolean phone)` 的默认 AlertDialog view：

```text
LinearLayout container
  title
  subtitle
  EditText
  error TextView
  actions row
    取消
    保存
```

样式：

- 容器 padding：`dp(20)`
- 输入框使用现有 `inputField()` 或新建圆角浅灰背景输入框。
- error TextView 默认 `GONE`，错误时红色显示。
- 保存按钮使用 `primaryButton("保存")`。
- 取消按钮使用 `secondaryButton("取消")` 或 text button。
- Dialog 背景设为白色圆角，避免系统默认边角和项目风格不一致。

实现方式：

1. `AlertDialog dialog = new AlertDialog.Builder(this).create();`
2. `dialog.setView(container);`
3. `dialog.setOnShowListener(...)` 内设置 window 背景和宽度。

### 7.2 手机号校验

手机号输入规则：

- 允许空值：表示清空手机号。
- 非空时必须是中国大陆手机号：`^1[3-9]\\d{9}$`。
- 自动 trim，并移除中间空格、短横线：

```java
String normalized = raw.replaceAll("[\\s-]", "");
```

错误文案：

- 空值：不报错。
- 非法：`请输入 11 位有效手机号`。

### 7.3 邮箱校验

邮箱输入规则：

- 允许空值：表示清空邮箱。
- 非空时使用 Android 标准模式：

```java
Patterns.EMAIL_ADDRESS.matcher(value).matches()
```

需要 import：

```java
import android.util.Patterns;
```

错误文案：

- 非法：`请输入有效邮箱地址`。

### 7.4 保存按钮状态

新增：

```java
private String contactValidationError(boolean phone, String value)
```

TextWatcher 每次输入后：

```java
String error = contactValidationError(phone, edit.getText().toString());
errorView.setText(error);
errorView.setVisibility(error.isEmpty() ? View.GONE : View.VISIBLE);
save.setEnabled(error.isEmpty());
save.setAlpha(error.isEmpty() ? 1f : 0.45f);
```

点击保存时再次校验，避免绕过 TextWatcher。

### 7.5 调用接口

保存逻辑保持：

```java
updateContact(newPhone, newEmail)
```

手机号弹窗只改手机号，邮箱保持当前值；邮箱弹窗只改邮箱，手机号保持当前值。

成功后：

- 更新 `SessionStore`。
- 关闭弹窗。
- 刷新账号管理页。

失败后：

- 弹窗不关闭。
- 按现有 `handleApiError()` 判断登录过期。
- 非登录过期显示 toast：`保存失败，请重试`。

## 8. 实施步骤

1. 聊天输入栏
   - 抽取 `updateInputActionButtonState()`。
   - 替换所有直接 `setActionButtonText(...)` 的分散逻辑。
   - 梳理语音识别结果发送，避免 partial/final 重复发送。

2. 表格中间态
   - 修改 `startActiveFormCodeBox()`，删除 `plain text` 标签。
   - 保持 `<form>` 缓冲和 `</form>` 后渲染表格逻辑不变。

3. 侧栏导航
   - 替换四个 `drawerNavButton()` 的 icon 参数。
   - 调整 icon TextView 固定尺寸和居中样式。

4. 头像圆形化
   - 新增 `CircleImageView`。
   - 替换所有用户头像 ImageView。
   - 保证首字母头像背景圆形。

5. 联系人弹窗
   - 重写 `editContact(boolean phone)` 为自定义弹窗。
   - 新增手机号/邮箱校验函数。
   - 保存前再次校验。

## 9. 验证方案

### 9.1 编译验证

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

### 9.2 模拟器验证

聊天页：

- 输入框为空时，右侧显示麦克风按钮。
- 输入框输入文字后，右侧立即变为发送按钮。
- 有文字时点击右侧按钮发送文本，不进入语音模式。
- 空输入时点击麦克风，授权后开始识别。
- 识别到语音后直接发送为用户消息。
- 表格流式中间态代码框不显示 `plain text`。

侧栏：

- 四个入口图标分别与聊天、商品、购物车、订单语义匹配。
- 图标和文字垂直居中，行高不抖动。

账号管理：

- 设置页、账号管理页、个人资料页、编辑资料页、头像预览页中的用户头像均为圆形。
- 手机号弹窗样式与 APP 一致。
- 邮箱弹窗样式与 APP 一致。
- 手机号输入非法值时显示错误并禁用保存。
- 邮箱输入非法值时显示错误并禁用保存。
- 空手机号/空邮箱允许保存。

### 9.3 日志验证

安装运行后检查：

```bash
adb logcat -d -t 300 | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|Exception"
```

不应出现 APP 崩溃或语音识别相关未捕获异常。

## 10. 风险和边界

- Android 系统语音识别依赖设备服务，模拟器可能因为系统镜像或网络环境导致 `ERROR_NETWORK`，这属于设备能力问题。APP 需要优雅提示，不应崩溃。
- Unicode emoji 图标在不同系统字体下可能显示为彩色或黑白；若出现缺字方框，应切换到备选字符。
- 圆形图片裁剪使用 `clipPath`，头像尺寸较小，性能影响可以忽略。
- 手机号校验按中国大陆手机号设计；如果后续需要国际号码，应改成国家区号选择器，不在本轮范围内。
