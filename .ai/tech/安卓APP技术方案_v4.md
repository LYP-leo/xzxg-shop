# 安卓 APP 技术方案 v4

## 1. 背景

本文针对 [安卓APP_问题_v3.md](./安卓APP_问题_v3.md) 中提出的全面屏与顶部栏问题，设计下一版 Android 原生 APP 改造方案。

当前 v3 实现解决了状态栏颜色、底部安全区、Markdown、商品/购物车/订单等问题，但聊天页顶部栏仍存在两个关键问题：

- 聊天内容上滑后，消息会穿过顶部区域，导致顶部栏像透明的一样，视觉上无法覆盖聊天内容。
- 顶部栏高度过高，状态栏和业务顶部栏之间留白太大，和参考图 [bug_03_correct.jpg](./assets/bug_03_correct.jpg) 不一致。

v4 的目标是只针对聊天页顶部栏和全面屏布局做结构性修正，避免继续用 padding 修补。

## 2. 设计结论

聊天页不应继续使用：

```text
LinearLayout.VERTICAL
  顶部栏
  ScrollView
  输入栏
```

这种结构会导致顶部栏是普通布局的一部分，消息区和顶部栏之间很难形成稳定的覆盖关系。

v4 改成：

```text
FrameLayout root
  ScrollView messageLayer
  LinearLayout topOverlay
  LinearLayout bottomComposerOverlay
  drawerLayer
```

其中：

- `messageLayer` 是聊天消息滚动层。
- `topOverlay` 是固定顶部浮层，不参与消息滚动。
- `bottomComposerOverlay` 是固定底部输入浮层，不参与消息滚动。
- 消息列表通过 `paddingTop` 和 `paddingBottom` 给顶部栏和输入栏预留空间。

## 3. 顶部栏问题分析

### 3.1 当前错误表现

参考 [bug_03_error.jpg](./assets/bug_03_error.jpg)：

- 消息气泡滚到顶部时，能从顶部菜单区域背后透出来。
- 菜单按钮和用户状态没有一个明确的白色背景承载。
- 顶部区域看起来像透明覆盖层，而不是稳定导航栏。

### 3.2 正确目标

参考 [bug_03_correct.jpg](./assets/bug_03_correct.jpg)：

- 状态栏和顶部栏合并成一个固定区域。
- 顶部栏背景是实色白色或接近页面背景色，不透明。
- 消息内容上滑时会被顶部栏遮住，不会穿透显示。
- 顶部栏高度紧凑，菜单按钮、标题/状态文案在状态栏下方居中排布。

## 4. 聊天页整体结构

### 4.1 新布局结构

`renderChatHome()` 改为：

```text
baseScreen()

chatRoot = FrameLayout
  messageScroll
    chatList
  topOverlay
  bottomComposerOverlay
```

不要再把顶部栏和输入栏直接加到 `content` 的线性布局中。

### 4.2 根布局规则

`baseScreen()` 仍然创建全屏 `FrameLayout root`：

```java
root = new FrameLayout(this);
root.setBackgroundColor(BG_COLOR);
setContentView(root);
```

聊天页内部再创建：

```java
FrameLayout chatRoot = new FrameLayout(this);
root.addView(chatRoot, MATCH_PARENT);
```

其他页面可以继续使用当前 `content = LinearLayout.VERTICAL` 方案。v4 优先修聊天页。

## 5. 顶部浮层方案

### 5.1 顶部栏定位

顶部栏使用 `FrameLayout.LayoutParams` 固定在屏幕顶部：

```java
FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(
    MATCH_PARENT,
    statusBarHeight() + dp(56),
    Gravity.TOP
);
```

高度规则：

```text
topOverlayHeight = statusBarHeight + 56dp
```

不要再使用：

```text
statusBarHeight + 12dp + 48dp + 8dp
```

这种叠加会导致顶部栏过高。

### 5.2 顶部栏内部布局

顶部栏结构：

```text
topOverlay
  toolbarRow
    left: 菜单按钮
    center: 可选标题/当前 Agent 名称
    right: 用户入口
```

布局规则：

- `topOverlay` 背景必须是不透明颜色，建议 `#F8F9FB` 或 `#FFFFFF`。
- `toolbarRow` 高度固定 `56dp`。
- `toolbarRow` 顶部 margin 等于 `statusBarHeight()`。
- 菜单按钮尺寸 `44dp x 44dp`。
- 用户入口高度 `44dp`，右侧对齐。
- 按钮仍保持透明背景，但其父级顶部栏不透明。

### 5.3 顶部栏背景

必须设置：

```java
topOverlay.setBackgroundColor(BG_COLOR);
topOverlay.setClickable(true);
```

`setClickable(true)` 的目的：

- 防止顶部栏区域的点击事件穿透到底下的聊天消息。
- 明确顶部栏是覆盖层，而不是普通透明容器。

### 5.4 顶部栏阴影

第一阶段不加明显阴影，避免像卡片。

可选：

```java
topOverlay.setElevation(dp(2));
```

如果加阴影，只能很轻，作用是分隔顶部栏和滚动内容。

## 6. 消息列表避让顶部栏

### 6.1 问题

顶部栏固定后，如果消息列表没有 padding，第一条消息会被顶部栏盖住。

### 6.2 方案

消息列表设置顶部 padding：

```java
int topOverlayHeight = statusBarHeight() + dp(56);
chatList.setPadding(
    dp(22),
    topOverlayHeight + dp(16),
    dp(22),
    bottomComposerHeight + dp(16)
);
```

注意：

- 这里是消息列表的内容 padding。
- 不是给顶部栏增加高度。
- 消息滚动到顶部时，内容进入顶部栏后方，但由于顶部栏不透明，用户看不到穿透。

## 7. 底部输入栏浮层

### 7.1 定位

底部输入栏同样固定在底部：

```java
FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(
    MATCH_PARENT,
    composerHeight,
    Gravity.BOTTOM
);
```

高度：

```text
composerHeight = navBarHeight + 92dp
```

其中：

- 输入栏胶囊高度：`64dp`
- 顶部阴影空间：`10dp`
- 底部安全区和留白：`navBarHeight + 18dp`

### 7.2 消息列表避让底部

消息列表底部 padding 使用同一个 `composerHeight`：

```java
chatList.setPadding(..., composerHeight + dp(16));
```

这样最后一条消息不会被底部输入栏挡住。

## 8. 全面屏策略

### 8.1 不使用真正沉浸式隐藏状态栏

v4 不隐藏状态栏，不使用：

```java
SYSTEM_UI_FLAG_FULLSCREEN
WindowInsetsController.hide(...)
```

原因：

- 用户仍需要看到时间、电量、网络。
- 参考图也是状态栏可见，只是顶部栏与状态栏融合。

### 8.2 状态栏颜色

继续使用：

```java
window.setStatusBarColor(BG_COLOR);
window.setNavigationBarColor(BG_COLOR);
window.getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
```

### 8.3 顶部栏高度标准

聊天页顶部栏总高度：

```text
statusBarHeight + 56dp
```

二级页面顶部栏仍可保留较高的 `PageHeader`，因为它们有标题和副标题；但聊天页顶部栏必须紧凑。

## 9. 与二级页面的区别

### 9.1 聊天页

聊天页顶部栏是固定浮层：

```text
FrameLayout overlay
```

特点：

- 永远固定顶部。
- 背景不透明。
- 高度紧凑。
- 覆盖聊天内容。

### 9.2 商品/购物车/订单/我的/设置页

这些页面可以继续使用：

```text
LinearLayout.VERTICAL
  PageHeader
  ScrollView
```

原因：

- 这些页面不是连续对话滚动场景。
- 当前问题只发生在聊天消息上滑穿透顶部栏。

后续如果要统一，也可以再把所有页面改成固定顶部栏，但本轮不扩大范围。

## 10. 实现步骤

### 第一步：拆分聊天页布局

新增或调整方法：

```java
private void renderChatHome()
private View createChatTopOverlay()
private View createChatMessageLayer(int topOverlayHeight, int composerHeight)
private View createComposerOverlay()
```

`renderChatHome()` 中：

```java
baseScreen();
FrameLayout chatRoot = new FrameLayout(this);
root.addView(chatRoot, MATCH_PARENT);

int topHeight = statusBarHeight() + dp(56);
int composerHeight = navBarHeight() + dp(92);

chatRoot.addView(createChatMessageLayer(topHeight, composerHeight));
chatRoot.addView(createChatTopOverlay(), topParams);
chatRoot.addView(createComposerOverlay(), bottomParams);
```

### 第二步：顶部栏固定化

将当前：

```java
content.addView(top);
```

改为：

```java
chatRoot.addView(topOverlay, topOverlayParams);
```

并确保：

```java
topOverlay.setBackgroundColor(BG_COLOR);
topOverlay.setClickable(true);
```

### 第三步：消息列表 padding

`chatList` 的 padding 改为：

```java
chatList.setPadding(
    dp(22),
    topHeight + dp(16),
    dp(22),
    composerHeight + dp(16)
);
```

欢迎语的顶部 margin 不能再使用过大的 `180dp`。

建议：

```text
welcomeTopMargin = 120dp
```

或根据屏幕高度动态居中，但不能把顶部栏撑高。

### 第四步：底部输入栏固定化

`renderComposer()` 不再直接：

```java
content.addView(outer)
```

而是返回 `View`：

```java
private View createComposerOverlay()
```

由 `renderChatHome()` 通过 `FrameLayout.LayoutParams(Gravity.BOTTOM)` 加入。

## 11. 验收标准

### 11.1 顶部栏遮挡

- 聊天消息上滑到顶部时，不能从菜单按钮、用户入口或状态栏区域透出来。
- 顶部栏区域始终有不透明背景。
- 点击顶部栏空白区域不会触发下面的消息区域。

### 11.2 顶部栏高度

- 顶部栏总高度约为 `statusBarHeight + 56dp`。
- 菜单按钮距离状态栏不过度留白。
- 顶部栏视觉接近 [bug_03_correct.jpg](./assets/bug_03_correct.jpg)，而不是 [bug_03_error.jpg](./assets/bug_03_error.jpg)。

### 11.3 消息可读性

- 第一条消息不会被顶部栏盖住。
- 最后一条消息不会被底部输入栏盖住。
- 长回复滚动时，顶部栏始终固定。

### 11.4 其他功能不回退

- Markdown 渲染仍可用。
- 短消息气泡宽度仍自适应。
- 历史会话规则不变。
- 商品、购物车、订单页面不受影响。

## 12. 模拟器测试要求

实现后必须在模拟器做以下检查：

```text
1. 进入聊天页。
2. 发送一条消息，让页面出现用户气泡。
3. 获得一段较长 AI 回复，或者用本地历史构造长回复。
4. 上滑聊天内容到顶部。
5. 检查消息是否被顶部栏不透明遮住。
6. 检查顶部栏高度是否明显低于 v3 错误版本。
7. 打开侧栏、商品、购物车、订单，确认其它页面未崩溃。
```

截图建议：

```text
/private/tmp/xzxg-android-v4-chat-top-overlay.png
/private/tmp/xzxg-android-v4-chat-scrolled-top.png
/private/tmp/xzxg-android-v4-chat-bottom-composer.png
```

崩溃检查：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb logcat -d -v time \
  | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|System.err" \
  | tail -120
```

## 13. 风险与边界

- 本轮只修聊天页顶部栏，不重做二级页面 PageHeader。
- 如果仍使用 `LinearLayout.VERTICAL`，这个问题会反复出现，因此必须改成 `FrameLayout overlay`。
- 顶部栏高度必须用固定设计值控制，不能继续靠多层 padding 叠加。
- 参考图中的电话、语音等按钮不是本轮目标；本轮只解决顶部栏覆盖和高度问题。
