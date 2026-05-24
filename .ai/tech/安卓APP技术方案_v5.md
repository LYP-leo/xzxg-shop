# 安卓 APP 技术方案 v5

## 1. 背景

本文针对 [安卓APP_问题_v4.md](./安卓APP_问题_v4.md) 中提出的新问题，设计下一版 Android 原生 APP 改造方案。

本版不再继续做全面屏适配。当前问题的根源是 v4 为了处理状态栏、导航栏和键盘，叠加了 `statusBarHeight()`、`navBarHeight()`、固定浮层和手动键盘避让，导致输入栏位置被过度计算。

v5 的目标是回到更稳定的普通 Android 布局：

- 输入栏默认放在屏幕最底部。
- 键盘弹出时，输入栏刚好贴在键盘上方。
- 顶栏只保留容纳信息所需的高度。
- 整体 UI 字体、图标、控件尺寸稍微缩小。

## 2. 设计结论

聊天页从 v4 的三层浮层布局：

```text
FrameLayout
  ScrollView messageLayer
  topOverlay
  bottomComposerOverlay
```

改回更稳定的普通纵向布局：

```text
LinearLayout root
  topBar        固定高度
  ScrollView    消息区域，占满剩余空间
  composerBar   输入栏，位于底部
```

并依赖系统键盘调整：

```xml
android:windowSoftInputMode="adjustResize"
```

不要再手动计算键盘高度并调用：

```java
composerOverlay.setTranslationY(...)
```

这是本版必须删除的逻辑。

## 3. 输入框方案

### 3.1 当前错误

参考 [bug_04_error.jpg](./assets/bug_04_error.jpg)：

- 键盘弹出后，输入栏被抬得过高。
- 输入栏和键盘之间出现大面积空白。
- 消息内容被输入栏遮挡，整体层级看起来混乱。

### 3.2 正确目标

参考 [bug_04_correct.jpg](./assets/bug_04_correct.jpg)：

- 键盘弹出时，输入栏刚好贴在键盘顶部。
- 输入栏和键盘之间没有大块空白。
- 输入栏仍然位于聊天内容底部。
- 聊天内容滚动区域被压缩，不被输入栏遮挡。

### 3.3 布局策略

输入栏不再使用 `FrameLayout.LayoutParams(Gravity.BOTTOM)` 固定浮层。

改为普通 `LinearLayout` 的最后一个子 View：

```java
root = new LinearLayout(this);
root.setOrientation(LinearLayout.VERTICAL);

root.addView(topBar, new LinearLayout.LayoutParams(MATCH_PARENT, dp(52)));
root.addView(chatScroll, new LinearLayout.LayoutParams(MATCH_PARENT, 0, 1));
root.addView(composerBar, new LinearLayout.LayoutParams(MATCH_PARENT, dp(72)));
```

键盘弹出后，Android 会通过 `adjustResize` 缩小 Activity 可用高度，`composerBar` 会自然贴到键盘上方。

### 3.4 输入栏默认位置

默认状态下不考虑全面屏和底部导航栏安全区：

```java
composerBar.setPadding(dp(14), dp(8), dp(14), dp(8));
```

不要再使用：

```java
navBarHeight() + dp(18)
```

也不要再额外增加底部空白。

### 3.5 输入框尺寸

输入栏整体高度：

```text
72dp
```

输入胶囊高度：

```text
52dp
```

内部控件：

```text
左侧加号按钮：40dp x 40dp
右侧发送/语音按钮：42dp x 42dp
输入文字：15sp
圆角：26dp
```

和 v4 相比，整体缩小一档。

## 4. 顶栏方案

### 4.1 放弃全面屏顶栏计算

顶栏不再使用：

```java
statusBarHeight() + dp(56)
```

也不再把业务顶栏放到状态栏下面做沉浸式融合。

### 4.2 新顶栏高度

顶栏高度固定：

```text
56dp
```

顶栏只负责承载：

- 左侧菜单按钮。
- 中间或左侧标题 `AI导购`。
- 右侧登录状态。

如果状态栏由系统占用，顶栏自然位于状态栏下方；如果不同机型表现略有差异，本版不再额外适配。

### 4.3 顶栏控件尺寸

```text
菜单按钮：40dp x 40dp，图标 22sp
标题：18sp，加粗
登录状态：15sp
左右 padding：16dp
```

顶栏背景使用页面背景色或白色，不透明：

```java
topBar.setBackgroundColor(BG_COLOR);
```

## 5. 消息区方案

消息区使用普通 `ScrollView`，在顶栏和输入栏中间：

```java
chatScroll.setFillViewport(true);
root.addView(chatScroll, new LinearLayout.LayoutParams(-1, 0, 1));
```

消息列表 padding：

```java
chatList.setPadding(dp(16), dp(12), dp(16), dp(12));
```

不要再给消息列表加：

```java
topOverlayHeight + dp(16)
composerHeight + dp(16)
```

原因是顶栏和输入栏已经是普通布局的一部分，不需要通过 padding 预留浮层空间。

欢迎语顶部间距建议从 v4 的 `120dp` 降到：

```text
96dp
```

避免首页空白过大。

## 6. 整体 UI 缩小规则

本版整体缩小约 8%-12%，但不改变信息层级。

### 6.1 字号

```text
顶栏标题：18sp
消息正文：15sp
输入框正文：15sp
按钮图标：22sp-24sp
弱提示文字：13sp
商品/订单卡片标题：15sp-16sp
```

### 6.2 间距

```text
页面左右边距：16dp
消息气泡内边距：12dp x 9dp
消息气泡上下间距：6dp
卡片圆角：12dp
输入栏外边距：14dp
```

### 6.3 气泡宽度

消息气泡最大宽度从 `0.78 * screenWidth` 降为：

```text
0.74 * screenWidth
```

这样视觉上更接近聊天软件，不会显得粗大。

## 7. 需要删除或禁用的 v4 逻辑

以下 v4 逻辑必须删除或不再用于聊天页：

```java
bindKeyboardAvoidance(...)
composerOverlay.setTranslationY(...)
statusBarHeight() + dp(56)
navBarHeight() + dp(92)
FrameLayout topOverlay
FrameLayout bottomComposerOverlay
chatList padding = topOverlayHeight + composerHeight
```

`statusBarHeight()` 和 `navBarHeight()` 可以保留给其他页面暂时使用，但聊天页不再依赖它们。

## 8. 实现步骤

### 第一步：重构 `renderChatHome()`

将聊天页根布局改成普通纵向结构：

```java
private void renderChatHome() {
    closeDrawer();
    activePage = "chat";
    baseScreen();

    content.addView(createCompactTopBar(), new LinearLayout.LayoutParams(-1, dp(56)));
    content.addView(createChatMessageLayer(), new LinearLayout.LayoutParams(-1, 0, 1));
    content.addView(createComposerBar(), new LinearLayout.LayoutParams(-1, dp(72)));
}
```

### 第二步：删除聊天页浮层

删除聊天页中的：

```java
FrameLayout chatRoot
createChatTopOverlay()
createComposerOverlay()
bindKeyboardAvoidance()
```

替换为：

```java
createCompactTopBar()
createChatMessageLayer()
createComposerBar()
```

### 第三步：调整输入栏

`createComposerBar()` 返回普通底部栏，不设置 `Gravity.BOTTOM`，不设置 translation。

### 第四步：调整 AndroidManifest

保留：

```xml
android:windowSoftInputMode="adjustResize"
```

如模拟器上仍不生效，再在 Activity 启动时补充：

```java
getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
```

但不要再手动计算键盘高度。

### 第五步：整体缩小控件

集中调整：

- 顶栏按钮和文字。
- 输入栏高度和按钮。
- 消息气泡字号、padding、最大宽度。
- 商品/购物车/订单页面的卡片间距和标题字号。

## 9. 验收标准

### 9.1 输入栏

- 未打开键盘时，输入栏贴近屏幕底部。
- 打开键盘时，输入栏刚好贴在键盘上方。
- 输入栏和键盘之间不能出现大块空白。
- 输入栏不能遮住消息内容。

### 9.2 顶栏

- 顶栏高度明显小于 v4。
- 顶栏只占用容纳菜单、标题、登录状态所需的高度。
- 顶栏背景不透明。

### 9.3 UI 尺寸

- 字体和图标比 v4 略小。
- 输入栏、气泡、卡片不再显得笨重。
- 仍然保持可点击区域足够大，不牺牲基本可用性。

## 10. 模拟器测试要求

实现后必须测试以下场景：

```text
1. 打开聊天首页，确认输入栏默认贴底。
2. 点击输入框，确认输入栏贴在键盘上方。
3. 输入文字，确认发送按钮切换正常。
4. 发送一条消息，确认消息区没有被输入栏遮挡。
5. 关闭键盘，确认输入栏回到底部。
6. 打开侧栏，确认侧栏仍可打开和关闭。
7. 进入商品、购物车、订单页面，确认页面没有崩溃。
```

建议截图：

```text
/private/tmp/xzxg-android-v5-chat-default.png
/private/tmp/xzxg-android-v5-keyboard-attached.png
/private/tmp/xzxg-android-v5-message-after-send.png
/private/tmp/xzxg-android-v5-drawer.png
```

崩溃检查：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb logcat -d -v time \
  | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|System.err" \
  | tail -120
```

## 11. 风险与边界

- 本版主动放弃全面屏适配，不再追求状态栏/导航栏融合效果。
- 如果不同系统键盘对 `adjustResize` 行为不同，以“输入栏贴键盘”为最高优先级。
- 本轮先解决聊天主界面；二级页面只做字号和控件略微缩小，不重做信息架构。
- 后续如果重新做全面屏，必须基于系统 WindowInsets 正规方案，不再手动叠加 `statusBarHeight()` 和 `navBarHeight()`。
