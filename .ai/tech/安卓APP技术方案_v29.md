# 安卓 APP 技术方案 v29

## 1. 背景

本文针对 [安卓APP_问题_v28.md](./安卓APP_问题_v28.md) 设计 Android 原生 APP 下一版改造方案。

本轮问题集中在三类：

1. 键盘顶起页面仍不稳定：第一次弹出有效，收起后第二次弹出失效；滚动位置在中部时也应整体避让键盘；键盘弹出与页面上移之间存在可见时间差。
2. 思考过程 UI 仍不符合目标：历史记录里的思考步骤被渲染成多个独立对话；思考完成后应自动收起，参考 `详细信息_收起.jpg` 和 `详细信息_展开.jpg` 重新设计；展开/收起入口应清晰可点。
3. 侧栏不应展示“优惠券”和“活动”两个入口。

v29 方案只覆盖上述问题，不改动其他业务页面、上传逻辑、商品/订单/购物车功能。

## 2. 当前实现核对

### 2.1 键盘问题

当前聊天页已经有 `chatViewport`：

```java
root FrameLayout
  chatViewport LinearLayout vertical
    topBar
    chatMessageLayer
    composerBar
```

并在 `applyChatImeInset()` 中通过：

```java
params.bottomMargin = imeBottom;
```

把聊天页底边推到键盘上沿。

但仍有三个问题：

1. 只在 `imeWillShow && lastImeBottom == 0` 时记录状态，第二次弹出键盘时如果 `lastImeBottom` 没有可靠回到 0，就会跳过关键处理。
2. `scrollBottom()` 被 `wasChatAtBottomBeforeIme` 限制，导致用户在中部阅读时键盘虽然应该避让，但视觉上容易被误判为“没有顶起”。
3. 只监听 `OnApplyWindowInsetsListener`，而 IME 动画期间初始 inset 到达有延迟，因此页面上移和键盘弹出之间存在时间差。

### 2.2 思考过程问题

当前后端新协议已经采用：

```json
{
  "type": "thinking_delta",
  "run_id": "run_xxx",
  "step": {
    "id": "intent",
    "title": "分析用户需求",
    "status": "done",
    "summary": "..."
  }
}
```

Android 当前实时渲染入口为：

```java
updateThinkingPanel(event.optJSONObject("step"));
```

历史记录保存为多个 segment：

```json
{ "type": "thinking", "thought": { ... } }
{ "type": "thinking", "thought": { ... } }
{ "type": "thinking", "thought": { ... } }
```

历史渲染时每个 `thinking` segment 都调用一次：

```java
renderHistoricalThinkingSegment(...)
```

所以三个步骤被显示成三个独立气泡，而不是一个统一的“已完成思考”组件。

### 2.3 侧栏问题

当前 `showDrawer()` 的导航列表包含：

```java
drawerNavButton("券", "优惠券", "coupons", ...)
drawerNavButton("促", "活动", "promotions", ...)
```

v29 要求删除这两个侧栏入口。注意：只删除侧栏入口，不删除 `renderCoupons()` / `renderPromotions()` 方法，避免影响后端 block 导航或其他页面跳转。

## 3. v29 总体目标

1. 键盘每次弹出都稳定把聊天页整体顶到键盘上方，不依赖“当前是否在底部”。
2. 键盘收起后再次弹出仍然生效，`lastImeBottom` 状态必须可靠复位。
3. 键盘动画期间页面同步移动，消除明显时间差。
4. 用户在聊天列表中部点击输入框时，聊天页底边同样贴到键盘上沿，但不强制把聊天内容滚到底。
5. 实时思考过程使用一个统一组件，输出过程中保持展开，不允许收起。
6. 主回答开始输出时，思考组件自动收起，只显示“已完成思考”和展开提示。
7. 思考完成后，用户可点击“已完成思考”或展开/收起图标切换详情。
8. 历史记录中的多个 thinking segment 聚合成一个思考组件展示，不能显示为多个独立对话。
9. 删除侧栏“优惠券”和“活动”入口。

## 4. 键盘适配方案

### 4.1 引入 IME 动画监听

保留 `OnApplyWindowInsetsListener` 作为最终状态兜底，同时新增 `WindowInsetsAnimation.Callback`：

```java
private void bindImeInsets() {
    root.setOnApplyWindowInsetsListener((view, insets) -> {
        int imeBottom = currentImeBottom(insets);
        applyChatImeInset(imeBottom, false);
        return insets;
    });
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
        root.setWindowInsetsAnimationCallback(new WindowInsetsAnimation.Callback(DISPATCH_MODE_CONTINUE_ON_SUBTREE) {
            @Override
            public WindowInsets onProgress(WindowInsets insets, List<WindowInsetsAnimation> runningAnimations) {
                applyChatImeInset(currentImeBottom(insets), true);
                return insets;
            }
        });
    }
    root.requestApplyInsets();
}
```

新增：

```java
private int currentImeBottom(WindowInsets insets)
```

只读取 `WindowInsets.Type.ime()`；低版本返回 0，保持原有 `adjustResize` 兜底。

### 4.2 `applyChatImeInset()` 改为无条件布局避让

当前逻辑把滚动和避让耦合。v29 拆开：

```java
private void applyChatImeInset(int imeBottom, boolean fromAnimation)
```

职责：

- 无论聊天列表是否在底部，只要 `imeBottom` 变化，就更新 `chatViewport.bottomMargin`。
- `bottomMargin` 从大于 0 变为 0 时，明确复位 `lastImeBottom = 0`。
- `imeBottom > 0` 时，`chatViewport` 必须立即 `requestLayout()`，不等待下一轮滚动。
- 只在“键盘弹出前已经贴底”时才滚到底；中部阅读时不滚动。

伪代码：

```java
private void applyChatImeInset(int imeBottom, boolean fromAnimation) {
    if (chatViewport == null) return;

    boolean wasHidden = lastImeBottom <= 0;
    boolean willShow = imeBottom > 0;
    if (willShow && wasHidden) {
        wasChatAtBottomBeforeIme = isChatScrolledToBottom();
    }

    FrameLayout.LayoutParams params = (FrameLayout.LayoutParams) chatViewport.getLayoutParams();
    if (params.bottomMargin != imeBottom || params.height != MATCH_PARENT) {
        params.height = MATCH_PARENT;
        params.bottomMargin = imeBottom;
        chatViewport.setLayoutParams(params);
        chatViewport.requestLayout();
    }

    lastImeBottom = imeBottom;

    if (willShow && wasChatAtBottomBeforeIme && !fromAnimation) {
        chatViewport.post(this::scrollBottom);
    }
}
```

关键点：

- 避让永远执行，不受 `wasChatAtBottomBeforeIme` 限制。
- `wasChatAtBottomBeforeIme` 只决定是否自动滚到底。
- 第二次弹出失效时，优先检查 `lastImeBottom` 是否在收起后回到 0。

### 4.3 输入框聚焦时主动请求 IME inset

输入框 focus 逻辑补充：

```java
input.setOnFocusChangeListener((v, hasFocus) -> {
    if (hasFocus) {
        hideAttachmentPanel();
        wasChatAtBottomBeforeIme = isChatScrolledToBottom();
        root.requestApplyInsets();
    }
});
```

不要在 focus 中无条件 `scrollBottom()`。

### 4.4 清理页面时解绑动画回调

`baseScreen()` 中除了清理 listener，还要清理 animation callback：

```java
if (root != null) {
    root.setOnApplyWindowInsetsListener(null);
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
        root.setWindowInsetsAnimationCallback(null);
    }
}
lastImeBottom = 0;
wasChatAtBottomBeforeIme = true;
```

## 5. 思考过程 UI 方案

### 5.1 单一组件模型

无论实时还是历史，思考过程都必须渲染成一个组件：

```text
ThinkingPanel
  Header: 图标 + 已完成思考/正在思考 + 展开/收起图标
  Body: steps 列表
```

不允许每个 step 创建一个独立聊天气泡。

新增 UI 状态：

```java
private boolean activeThinkingCompleted;
private boolean activeThinkingUserToggled;
```

已有：

```java
private LinearLayout activeThinkingPanel;
private LinearLayout activeThinkingList;
private TextView activeThinkingTitle;
private boolean activeThinkingExpanded;
private final Map<String, JSONObject> activeThinkingSteps;
```

### 5.2 输出过程中的展开规则

`updateThinkingPanel(step)`：

- 收到首个 step 时创建 panel。
- 如果还未开始正文输出，则强制 `activeThinkingExpanded = true`。
- 输出过程中不响应收起点击；点击时可以 toast “思考完成后可收起”，也可以直接忽略。
- step 内容仍持续写入 `activeThinkingSteps`。

实现判断：

```java
private boolean isAnswerStarted() {
    return activeAssistantFullMarkdown != null && activeAssistantFullMarkdown.length() > 0
        || activeAssistant != null
        || activeAssistantMessageBubble != null;
}
```

### 5.3 正文开始时自动收起

在 `appendAssistant(String delta)` 或 `appendAssistantText(String delta)` 的正文首次到达处执行：

```java
collapseThinkingWhenAnswerStarts();
```

规则：

- 只自动收起一次。
- 自动收起后 header 显示：

```text
已完成思考  展开
```

- 如果用户后续点击展开，再展示完整 steps。

新增：

```java
private boolean thinkingAutoCollapsed;
```

### 5.4 Header 交互设计

参考图片：

收起态：

```text
✦ 已完成思考  ˅
```

展开态：

```text
✦ 已完成思考  ˄
  ✓ 分析用户需求完成
    summary...
  ✓ 查询买手团经验完成
    横向经验卡片...
  ✦ 总结答案完成
```

实现要求：

- Header 整行可点。
- 右侧图标明确表示当前操作：收起态显示向下/展开，展开态显示向上/收起。
- 正在思考时显示“正在思考”，且展开固定为 true。
- 完成后显示“已完成思考”。
- 切换展开/收起时保持当前 scrollY，不触发 `scrollBottom()`。

`activeThinkingTitle.setOnClickListener` 改为：

```java
if (!activeThinkingCompleted && streaming) return;
int before = chatScroll.getScrollY();
activeThinkingExpanded = !activeThinkingExpanded;
activeThinkingUserToggled = true;
renderThinkingPanel(false);
chatScroll.post(() -> chatScroll.setScrollY(before));
```

### 5.5 历史思考聚合

当前历史渲染循环遇到每个：

```json
{ "type": "thinking", "thought": {...} }
```

都会调用 `renderHistoricalThinkingSegment()`，导致多个独立气泡。

v29 改为在 `renderAgentSegmentsInternal()` 先聚合连续或全部 thinking segments：

```java
JSONArray collectedThoughts = new JSONArray();
for segment in segments:
    if type == "thinking":
        collect thought
    else:
        flushHistoricalThinkingPanel(collectedThoughts)
        render normal segment
flushHistoricalThinkingPanel(collectedThoughts)
```

`flushHistoricalThinkingPanel()`：

- 创建一个历史 ThinkingPanel。
- Header 默认收起，显示“已完成思考  展开”。
- 点击 header 展开/收起。
- Body 使用 `thinkingStepView(thought)` 复用实时 step 样式。
- 如果遇到旧格式 `{ "thinking": { "stages": [...] } }`，转换为 step 后加入同一个 panel。

### 5.6 保存格式兼容

后端已经支持 `AgentSegment.Thought`：

```go
type AgentSegment struct {
    Type    string       `json:"type"`
    Text    string       `json:"text,omitempty"`
    Block   *AgentBlock  `json:"block,omitempty"`
    Thought *ThoughtStep `json:"thought,omitempty"`
}
```

Android 保存时继续使用：

```json
{ "type": "thinking", "thought": {...} }
```

但历史渲染时必须把多个 `thought` 合并为一个 UI。

## 6. 侧栏清理方案

`showDrawer()` 中删除：

```java
navGroup.addView(drawerNavButton("券", "优惠券", "coupons", v -> renderCoupons()));
navGroup.addView(drawerNavButton("促", "活动", "promotions", v -> renderPromotions()));
```

保留：

- `renderCoupons()`
- `renderPromotions()`
- `navigateRoute("coupons")`
- `navigateRoute("promotions")`

原因：Agent 的 `navigation_action` 或其他页面仍可能跳转到这些功能。

## 7. 修改文件清单

只需要修改：

```text
android-native/app/src/main/java/com/xzxg/shop/MainActivity.java
```

本轮不需要修改后端协议；后端已经发送 `thinking_delta.step`。

## 8. 验证方案

### 8.1 键盘验证

在模拟器中验证：

1. 进入聊天页，滚到底部，点击输入框。
2. 记录输入栏 bounds，应该贴键盘顶部。
3. 收起键盘，再次点击输入框。
4. 第二次仍应贴键盘顶部。
5. 手动把聊天列表滚到中部，点击输入框。
6. 页面整体底边仍贴键盘顶部，但聊天内容不强制跳到底部。
7. 观察键盘动画期间页面是否同步移动，无明显延迟。

用 UI dump 检查：

```bash
adb shell uiautomator dump --compressed /sdcard/ime.xml
adb pull /sdcard/ime.xml /private/tmp/ime.xml
rg "输入问题|AI导购|已完成思考" /private/tmp/ime.xml
```

成功标准：

- 每次键盘弹出，composer bounds bottom 等于键盘上沿。
- 收起后 composer 恢复屏幕底部。
- 中部滚动时不会自动滚到底。
- 无明显二次弹出失效。

### 8.2 思考过程验证

实时会话：

1. 发送一条能触发 thinking 的消息。
2. 思考输出时 panel 展开，且不能被收起。
3. 正文开始输出后 panel 自动收起。
4. 点击“已完成思考”或图标后展开。
5. 再次点击后收起，且 scrollY 不跳动。

历史会话：

1. 重新进入历史会话。
2. 思考过程只显示为一个“已完成思考”组件。
3. 展开后能看到全部步骤。
4. 不出现三个独立 thinking 对话气泡。

### 8.3 侧栏验证

打开侧栏：

- 应看到：AI导购、商品、购物车、订单。
- 不应看到：优惠券、活动。

### 8.4 构建验证

```bash
cd android-native
gradle assembleDebug
```

如共享盘 Gradle 哈希报错，复制到 `/private/tmp` 后构建验证。

## 9. 风险与注意事项

1. 键盘动画 callback 仅 Android R 及以上可用，低版本仍依赖 `adjustResize` 和 final insets。
2. 思考过程输出时禁止收起是产品约束；完成后才允许用户控制展开状态。
3. 历史 thinking 聚合要兼容旧 `thinking.stages` 和新 `thought` 两种格式，避免旧会话丢失思考内容。
4. 删除侧栏入口不等于删除优惠券/活动功能；不要误删页面方法和 API 调用。
