# 安卓 APP 技术方案 v28

## 1. 背景

本文针对 [安卓APP_问题_v27.md](./安卓APP_问题_v27.md) 设计下一版 Android 原生 APP 改造方案。

本轮问题集中在两个方向：

1. 思考过程仍未真正展示后端返回的“分析用户需求 / 查询买手团经验”详情，只停在“分析用户需求”阶段后直接进入回答。
2. 键盘弹出后的页面避让方式错误。正确效果应参考用户提供的截图：键盘收起时聊天页正常占满底部；键盘出现时，聊天页底边整体贴到键盘上沿，底部输入栏和聊天内容一起被顶上去，即使聊天列表已经滚到底，也能保持最新内容在键盘上方可见。

本方案必须基于已有后端/Agent 协议文档实现，不再靠猜测事件字段。

## 2. 已阅读的后端/Agent 文档结论

本仓库当前可见目录只有 Android 工程和 `.ai/tech` 文档，没有可直接读取的后端源码目录。已重点阅读以下后端/Agent 协议文档：

- [安卓APP技术方案_v25.md](./安卓APP技术方案_v25.md)
- [技术方案_v3.md](./技术方案_v3.md)
- [Agent框架设计_v1.md](./Agent框架设计_v1.md)
- [编码规范_v1.md](./编码规范_v1.md)

结论如下。

### 2.1 基础 SSE 事件

[技术方案_v3.md](./技术方案_v3.md) 定义 Android 消费后端 SSE 基础事件：

```text
message_start
status
text_delta
block_delta
followups
message_end
error
```

其中：

- `status` 只适合展示“正在检索商品 / 正在生成回答”等短提示。
- `text_delta` 是最终回答正文增量。
- `block_delta` 是商品卡、引用、对比表等结构化 UI。

### 2.2 旧 Agent 设计中的 thinking_status

[Agent框架设计_v1.md](./Agent框架设计_v1.md) 早期建议过：

```text
thinking_status
```

但文档也明确写明：`thinking_status` 只返回短状态，不暴露完整推理过程。因此它不能作为本轮“三段式详细思考内容”的主要数据源。

### 2.3 v25 新增的正式思考详情协议

[安卓APP技术方案_v25.md](./安卓APP技术方案_v25.md) 明确定义了本轮应使用的可展示思考详情事件：

```json
{
  "type": "thinking_delta",
  "run_id": "run_xxx",
  "stage": "user_need",
  "status": "running",
  "title": "分析用户需求",
  "delta": "用户正在寻找适合溪流路亚的装备，关注轻量、灵敏和新手可上手。",
  "items": []
}
```

字段定义：

- `stage`：阶段枚举。
- `status`：`running`、`completed`、`failed`。
- `title`：展示标题。
- `delta`：追加文本。
- `items`：可选结构化卡片，用于买手团经验。

阶段枚举：

```text
user_need        -> 分析用户需求
buyer_experience -> 查询买手团经验
answer_summary   -> 总结答案
```

### 2.4 后端输出点

v25 文档定义的后端输出点：

```text
intent / query rewrite 完成后：
thinking_delta stage=user_need status=running
thinking_delta stage=user_need status=completed

买手经验 / RAG 片段 / 商品经验库检索后：
thinking_delta stage=buyer_experience status=running
thinking_delta stage=buyer_experience status=completed items=[...]

final answer 开始生成或完成前：
thinking_delta stage=answer_summary status=completed delta=总结答案完成
```

如果没有命中买手经验，后端仍应发送 `buyer_experience completed`，内容可为：

```text
未找到强相关买手经验，已改用商品信息和用户需求进行推荐。
```

## 3. 当前实现问题核对

### 3.1 思考过程没有真实内容

当前 Android 代码已经创建了三阶段 UI，但存在几个关键问题：

1. `stream()` 一开始就调用 `startThinkingTimeline()`，导致 UI 先显示一个空的“分析用户需求中”。如果后端没有马上发 `thinking_delta`，用户看到的就是空壳。
2. 对 `status` 做了过度兼容，把短状态猜测映射成思考内容。这与文档不一致，且会造成“分析用户需求”一直 running。
3. 没有对 `thinking_delta` 做足够强的诊断日志，无法确认后端到底有没有发送 `user_need` / `buyer_experience`。
4. `status` 不能替代 `thinking_delta`，否则只能展示短提示，无法满足“详细思考过程”。
5. 点击“已完成思考”调用 `renderThinkingView()` 后会触发 `scrollBottom()`，导致页面跳到底部。
6. 阶段左侧使用 `│` 竖线，用户反馈显示有问题，应删除。
7. 展开符号使用 `⌃/⌄`，视觉不佳，应换成更清晰的轻量符号。

### 3.2 键盘顶起方式错误

当前代码：

```java
private void bindImeInsets() {
    root.setOnApplyWindowInsetsListener((view, insets) -> {
        int imeBottom = insets.getInsets(WindowInsets.Type.ime()).bottom;
        int systemBottom = insets.getInsets(WindowInsets.Type.systemBars()).bottom;
        int bottomInset = imeBottom > 0 ? imeBottom : systemBottom;
        content.setPadding(0, 0, 0, bottomInset);
        if (imeBottom > 0) {
            scrollBottom();
        }
        return insets;
    });
}
```

同时 Manifest 和运行时已经设置：

```xml
android:windowSoftInputMode="adjustResize"
```

```java
getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
```

这会形成“双重避让”和“错误滚动”：

- 系统已经 resize Activity 可用高度。
- Android 代码又给 `content` 加了 `imeBottom` padding。
- 还强制 `scrollBottom()`，表现像列表被手动滑动。
- 当聊天列表已经到底部时，继续上滑失效，输入区/内容恢复也异常。

当前后续代码又改成：

```java
int targetHeight = view.getHeight() - imeBottom;
content.setLayoutParams(params);
```

这仍然不够准确：

- 它直接修改整个 `content` 的固定高度，容易和系统 `adjustResize` 重复计算。
- 如果系统已经缩小了 root 高度，再减一次 `imeBottom`，页面会被过度压缩。
- 如果系统没有触发 resize，直接改高度虽然能避让输入栏，但没有维护“弹出前是否在底部”的状态，聊天列表到底时仍可能看不到最新内容。
- 它没有定义键盘隐藏时如何恢复容器高度和滚动锚点。

用户最新截图表达的目标不是“输入框不被盖住”，而是：

```text
键盘收起：
  顶栏
  聊天列表
  输入栏贴屏幕底部

键盘弹出：
  顶栏
  聊天列表高度变小
  输入栏贴键盘顶部
  聊天列表如果原本在底部，仍保持底部内容可见
```

因此 v28 需要把键盘适配定义为“聊天 viewport 随 IME inset 改变底边位置”，而不是简单 padding 或强制滚动。

## 4. v28 总体目标

1. Android 端严格以 `thinking_delta` 作为详细思考过程的数据源。
2. `status` / `thinking_status` 只作为短 loading 或诊断信息，不再伪造成详细思考内容。
3. 如果后端没有发送 `thinking_delta`，Android 应明确记录日志并降级显示短状态，而不是展示空的三阶段内容。
4. 收到 `user_need` 和 `buyer_experience` 后，必须把 `delta/items` 实时展示到对应模块。
5. 主回答开始前或回答开始时，若已收到思考详情，思考区稳定保持在 AI 正文上方。
6. 点击“已完成思考”展开/收起时，页面滚动位置保持不变。
7. 删除三阶段之间的小竖线，只保留适当间距。
8. 重新设计展开符号。
9. 键盘弹出时，聊天页根容器的底边必须贴到键盘上沿；顶栏、聊天列表、输入栏作为同一个聊天 viewport 重新布局。若键盘弹出前聊天列表在底部，弹出后仍自动保持底部锚定。

## 5. 思考过程协议处理方案

### 5.1 事件处理原则

`handleSse()` 中改为严格区分：

```text
message_start
  -> 记录 run_id，不创建空思考详情

thinking_delta
  -> 创建/更新三阶段思考详情组件

status / thinking_status
  -> 只更新短 loading 文案
  -> 不写入三阶段 detail

text_delta / content_delta / block_delta
  -> 主回答正常渲染

message_end
  -> finishStream()
  -> 如果已有 thinking 组件，则 completeThinkingView()
```

也就是说：

- 不再在 `stream()` 一开始无条件创建空的 `startThinkingTimeline()`。
- 只有收到首个 `thinking_delta` 后，才创建详细思考过程组件。
- 如果等待期间只有 `status`，显示短 loading，例如“正在分析用户需求...”，但不展示三阶段详情。

### 5.2 `thinking_delta` 字段读取

新增解析方法：

```java
private ThinkingEvent parseThinkingEvent(JSONObject event)
```

支持标准字段：

```text
type
run_id
stage
status
title
delta
items
```

只做必要兼容：

- `event.optJSONObject("thinking")` 作为嵌套 payload。
- `text/content/summary` 作为 `delta` fallback。

但不再用普通 `status.text` 去猜 `stage`。

### 5.3 思考事件诊断日志

为避免继续“自以为是”，新增调试日志：

```java
private void logThinkingEvent(JSONObject event, String reason)
```

日志内容：

```text
type
run_id
stage
status
title
delta 是否为空
items.length
原始 JSON
```

触发点：

- 收到 `thinking_delta`。
- 收到无法识别 stage 的 `thinking_delta`。
- 主回答 `text_delta` 已开始但尚未收到任何 `thinking_delta`。

示例日志：

```text
ThinkingEvent received stage=user_need status=completed delta_len=38 items=0
ThinkingEvent missing before text_delta, fallback to loading status only
```

### 5.4 后端缺失事件时的前端行为

如果没有收到 `thinking_delta`：

- 不展示三阶段空壳。
- 保留短 loading/status。
- 主回答到达后移除 loading，直接展示回答。
- 在日志中记录：`No thinking_delta before answer text_delta`。

这样能明确区分“前端渲染失败”和“后端没有按协议发送思考详情”。

## 6. 思考过程 UI 方案

### 6.1 展开态布局

收到首个 `thinking_delta` 后创建组件：

```text
✦ 正在思考  ▴

✓ 分析用户需求完成
  用户需求分析内容...

✓ 查询买手团经验完成
  买手经验摘要 / 横向卡片...

● 总结答案中
```

要求：

- 三阶段固定显示，但只有收到内容后展示内容。
- 阶段之间不使用小竖线。
- 阶段之间通过 margin 增加间距，例如 `bottomMargin = dp(14)`。
- `分析用户需求` 展示 `delta` 文本。
- `查询买手团经验`：
  - 有 `items`：显示横向卡片。
  - 有 `delta`：显示文本。
  - 两者都有：先文本，后卡片。
- `总结答案` 只显示“总结答案完成”。

### 6.2 收起态布局

完成后默认收起：

```text
✦ 已完成思考  ▾
```

要求：

- 只展示一行 header。
- 不展示三阶段 detail。
- 点击后展开 detail，再次点击收起。
- 点击不改变聊天列表滚动位置。

### 6.3 展开符号设计

替换当前 `⌃/⌄`。

建议使用：

```text
展开态：▴
收起态：▾
```

原因：

- 视觉更稳定。
- 字形更小，不像数学符号。
- 与“已完成思考”按钮搭配更自然。

也可使用纯 ASCII：

```text
展开态：^
收起态：v
```

但优先使用 `▴/▾`。

### 6.4 保持滚动位置

当前问题来自 `renderThinkingView()` 内部无条件 `scrollBottom()`。

改造：

```java
private void renderThinkingView(ThinkingViewState thinking, boolean preserveScroll)
```

点击 header 时：

```java
int before = chatScroll.getScrollY();
thinking.expanded = !thinking.expanded;
renderThinkingView(thinking, true);
chatScroll.post(() -> chatScroll.setScrollY(before));
```

流式更新时：

```java
renderThinkingView(thinking, false);
scrollBottom();
```

规则：

- 用户点击展开/收起：保持原滚动位置。
- 后端流式追加内容：可以按当前聊天生成体验滚到底部。
- 历史消息点击展开：必须保持位置。

### 6.5 阶段状态和完成逻辑

阶段状态：

```text
pending
running
completed
failed
```

收到某阶段 `completed`：

- 设置对应 stage completed。
- 不自动把后续阶段标记 completed。

收到 `answer_summary completed` 或 `message_end`：

- 如果已经存在 thinking 组件：
  - 补齐 `answer_summary.text = "总结答案完成"`。
  - 未失败阶段标记 completed。
  - 如果用户未手动操作，自动收起。
  - 保存 thinking segment。

注意：

- 如果从未收到任何 `thinking_delta`，`message_end` 不创建空 thinking segment。

## 7. 后端协议依赖与排查要求

### 7.1 Android 端必须确认收到的事件

实现后验证时必须抓取 Android logcat，确认是否出现：

```text
ThinkingEvent received stage=user_need
ThinkingEvent received stage=buyer_experience
ThinkingEvent received stage=answer_summary
```

如果没有，则说明后端没有按 v25 协议输出详细思考事件，前端不能凭空展示详细内容。

### 7.2 需要后端满足的输出

后端应在主回答 `text_delta` 前尽量发送：

```json
{"type":"thinking_delta","stage":"user_need","status":"completed","title":"分析用户需求","delta":"...","items":[]}
{"type":"thinking_delta","stage":"buyer_experience","status":"completed","title":"查询买手团经验","delta":"...","items":[...]}
{"type":"thinking_delta","stage":"answer_summary","status":"completed","title":"总结答案","delta":"总结答案完成","items":[]}
```

如果业务上必须边检索边回答，也可以在 `text_delta` 到达后补发，但 Android 仍应把组件固定在主回答上方。

## 8. 键盘顶起页面方案

### 8.1 目标效果

键盘适配必须严格对齐用户截图表达的效果。

键盘收起时：

```text
屏幕可用区域
  topBar
  chatScroll
  composerBar 贴屏幕底部
```

键盘弹出时：

```text
屏幕可用区域中键盘以上部分
  topBar
  chatScroll 高度变小
  composerBar 贴键盘上沿

键盘区域
```

也就是说，键盘出现时不是把某条消息“滑上去”，而是把整个聊天页面的底边改到键盘上沿。聊天列表如果原本已经在底部，弹出后仍然应该保持底部锚定，最新消息和输入栏都在键盘上方。

### 8.2 布局边界改造

新增一个聊天页专用 viewport 容器，不再直接把 `content` 当作全局可变高度对象：

```text
root FrameLayout
  chatViewport LinearLayout vertical
    topBar 固定高度
    chatScroll height=0 weight=1
    composerOuter wrap_content
```

要求：

- `chatViewport` 只在聊天页存在。
- `chatViewport` 的初始 `FrameLayout.LayoutParams` 为 `MATCH_PARENT x MATCH_PARENT`。
- 键盘显示时只调整 `chatViewport` 的底部约束，不修改全局 `content` 高度。
- `topBar`、`chatScroll`、`composerOuter` 必须在同一个 `chatViewport` 内，这样键盘出现时三者作为整体重新布局。
- `composerOuter` 不使用绝对定位，不使用 overlay，不使用额外底部 padding 占位。

推荐字段：

```java
private LinearLayout chatViewport;
private int lastImeBottom;
private boolean wasChatAtBottomBeforeIme;
```

### 8.3 IME inset 处理

保留窗口模式：

```xml
android:windowSoftInputMode="adjustResize"
```

```java
getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
```

但不能再做以下操作：

```java
content.setPadding(0, 0, 0, imeBottom);
content.setLayoutParams(height = rootHeight - imeBottom);
scrollBottom(); // 每次 WindowInsets 变化时无条件调用
```

新的 `bindImeInsets()` 只负责调整 `chatViewport` 的底部边界：

```java
private void bindImeInsets() {
    root.setOnApplyWindowInsetsListener((view, insets) -> {
        int imeBottom = 0;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            imeBottom = insets.getInsets(WindowInsets.Type.ime()).bottom;
        }
        applyChatImeInset(imeBottom);
        return insets;
    });
}
```

`applyChatImeInset()` 的职责：

```java
private void applyChatImeInset(int imeBottom) {
    if (chatViewport == null) {
        return;
    }
    boolean imeWillShow = imeBottom > 0;
    boolean imeWasHidden = lastImeBottom == 0;
    if (imeWillShow && imeWasHidden) {
        wasChatAtBottomBeforeIme = isChatScrolledToBottom();
    }

    FrameLayout.LayoutParams params = (FrameLayout.LayoutParams) chatViewport.getLayoutParams();
    params.height = FrameLayout.LayoutParams.MATCH_PARENT;
    params.bottomMargin = imeBottom;
    chatViewport.setLayoutParams(params);
    lastImeBottom = imeBottom;

    if (imeWillShow && wasChatAtBottomBeforeIme) {
        chatViewport.post(this::scrollBottom);
    }
}
```

关键点：

- `bottomMargin = imeBottom` 是“把聊天 viewport 的底边推到键盘上沿”，不是给消息列表塞 padding。
- 不改变 `content` 的高度，避免和 `adjustResize` 叠加。
- 只有键盘弹出前聊天列表本来在底部时，才在布局完成后 `scrollBottom()`，保持最新内容可见。
- 如果用户正在查看历史消息，键盘弹出时不要强制滚到底。
- 键盘收起时 `bottomMargin` 恢复为 0，聊天页回到截图 1 的底部布局。

### 8.4 底部锚定判断

新增：

```java
private boolean isChatScrolledToBottom() {
    if (chatScroll == null || chatScroll.getChildCount() == 0) {
        return true;
    }
    View child = chatScroll.getChildAt(0);
    int distance = child.getBottom() - (chatScroll.getScrollY() + chatScroll.getHeight());
    return distance <= dp(24);
}
```

使用 24dp 容差，避免最后几像素或动画过程导致误判。

### 8.5 输入框聚焦滚动

输入框获得焦点时可以延迟一次滚动，但必须遵守底部锚定规则：

```java
input.setOnFocusChangeListener((v, hasFocus) -> {
    if (hasFocus) {
        hideAttachmentPanel();
        wasChatAtBottomBeforeIme = isChatScrolledToBottom();
        input.postDelayed(() -> {
            if (input.hasFocus() && wasChatAtBottomBeforeIme) {
                scrollBottom();
            }
        }, 220);
    }
});
```

该滚动只用于保持“原本就在底部”的会话继续贴底，不承担键盘避让。键盘避让只由 `chatViewport.bottomMargin` 完成。

### 8.6 退出聊天页时恢复

离开聊天页或重建 `baseScreen()` 时必须清理聊天页 IME 状态：

```java
lastImeBottom = 0;
wasChatAtBottomBeforeIme = true;
root.setOnApplyWindowInsetsListener(null);
```

要求：

- 其他页面继续使用现有键盘模式恢复逻辑。
- 只在聊天页绑定 `bindImeInsets()`。
- 不把聊天页的 `bottomMargin` 状态泄漏到登录页、设置页、商品详情页。

## 9. 需要修改的方法清单

### 9.1 `MainActivity.java` 思考过程

修改：

- `stream(...)`
  - 删除无条件 `startThinkingTimeline()`。
- `handleSse(...)`
  - `thinking_delta` 才进入详细思考组件。
  - `status/thinking_status` 只更新短 loading。
  - `text_delta` 到达前如没有 thinking，记录诊断日志。
- `handleThinkingDelta(...)`
  - 严格解析 `thinking_delta`。
  - 不从普通 `status.text` 猜 stage。
- `renderThinkingView(...)`
  - 增加 preserve scroll 参数。
  - 移除无条件 `scrollBottom()`。
- `ensureThinkingStageViews(...)`
  - 删除小竖线。
  - 增加阶段间距。
- `completeThinkingView(...)`
  - 仅当已有 thinking 组件时完成并保存。

新增：

- `parseThinkingEvent(...)`
- `logThinkingEvent(...)`
- `renderThinkingView(thinking, preserveScroll)`
- `restoreScrollAfterThinkingToggle(...)`
- `hasReceivedThinkingDelta` 字段

删除或降级：

- `thinkingEventFromStatus(...)`
- `thinkingStageFromStatus(...)`
- `thinkingStatusDetail(...)`

这些方法只会继续制造“猜测协议”的问题，不应再作为详细思考内容来源。

### 9.2 `MainActivity.java` 键盘

修改：

- `renderChatHome()`
  - 创建聊天页专用 `chatViewport`。
  - `topBar`、`chatScroll`、`composerOuter` 全部加入 `chatViewport`。
  - `root` 只承载 `chatViewport`，不要让聊天页复用会被全局改高度的 `content`。
- `bindImeInsets()`
  - 删除 `content.setPadding(...)`。
  - 删除 `content.setLayoutParams(height = rootHeight - imeBottom)`。
  - 删除 `imeBottom > 0 -> scrollBottom()` 的无条件滚动。
  - 改为调用 `applyChatImeInset(imeBottom)`。
- `applyChatImeInset(int imeBottom)`
  - 新增。
  - 调整 `chatViewport` 的 `FrameLayout.LayoutParams.bottomMargin`。
  - 当键盘从隐藏变为显示时，记录弹出前是否在底部。
  - 如果弹出前在底部，则布局完成后滚到底部。
- `isChatScrolledToBottom()`
  - 新增。
  - 使用 `child.getBottom() - (scrollY + height)` 判断是否贴底。
- 输入框 focus 逻辑：
  - 只在弹出前已经贴底时延迟滚到底部一次。
  - 不承担键盘避让职责。
- `baseScreen()` 或离开聊天页逻辑：
  - 清理 `lastImeBottom`。
  - 清理 `wasChatAtBottomBeforeIme`。
  - 移除旧的 `OnApplyWindowInsetsListener`，避免影响其他页面。

新增字段：

```java
private LinearLayout chatViewport;
private int lastImeBottom;
private boolean wasChatAtBottomBeforeIme = true;
```

## 10. 验证方案

### 10.1 编译

```bash
gradle assembleDebug
```

要求：

- 构建成功。
- 不引入新依赖。

### 10.2 思考过程验证

运行后发送一条会触发导购/买手经验的问题。

必须检查 logcat：

```bash
adb logcat -d | rg "ThinkingEvent|thinking_delta"
```

成功标准：

- 能看到 `stage=user_need`。
- 能看到 `stage=buyer_experience`。
- 能看到 `stage=answer_summary`。
- UI 展示“分析用户需求”的后端 `delta`。
- UI 展示“查询买手团经验”的 `delta` 或 `items`。
- 主回答出现后，思考区仍在回答上方。
- 完成后默认收起为“已完成思考”。
- 点击展开后页面不跳到底部。
- 阶段之间没有小竖线。
- 展开符号已替换。

如果 logcat 没有 `thinking_delta`：

- 记录验证结论：后端未按 v25 文档输出思考详情。
- Android 只能显示短 loading，不能伪造详细内容。

### 10.3 键盘验证

验证步骤：

1. 聊天列表滚到底部，记录键盘收起时的布局，应接近用户截图 1：输入栏贴屏幕底部。
2. 点击输入框弹出键盘。
3. 观察键盘以上的整个聊天页面是否被顶上去，应接近用户截图 2：输入栏贴键盘顶部，最新内容仍在键盘上方。
4. 收起键盘。
5. 观察页面是否恢复截图 1 的底部布局。
6. 手动向上滚动查看历史消息，不在底部时再次点击输入框。
7. 观察键盘弹出后是否保持当前历史阅读位置，不强制跳到底部。

成功标准：

- 键盘收起时：输入栏贴屏幕底部，聊天页没有残留底部空白。
- 键盘弹出时：输入栏贴键盘顶部，聊天页底边与键盘上沿相接。
- 顶栏、聊天区、输入栏作为同一个 `chatViewport` 重新布局。
- 不出现 `content` 被二次压缩导致的异常空白。
- 不出现额外 IME padding。
- 不出现键盘收起后页面停在异常上移位置。
- 聊天列表到底时，键盘弹出后仍保持最新内容可见。
- 聊天列表不在底部时，键盘弹出不强制跳到底部。

### 10.4 崩溃日志

```bash
adb logcat -d AndroidRuntime:E '*:S'
```

要求：

- 无 `FATAL EXCEPTION`。
- 无 `SQLiteException`。
- 无 `NullPointerException`。
- 无 `ClassCastException`。

## 11. 风险与边界

1. 如果后端没有发送 `thinking_delta`，Android 无法展示真实“分析用户需求 / 查询买手团经验”详情。此时应通过日志暴露协议缺失，而不是在前端编造内容。
2. `status` / `thinking_status` 只能作为短状态，不应存入 thinking segment。
3. 个别系统键盘或全面屏模式下 `adjustResize` 可能不稳定，因此 v28 不只依赖系统 resize，而是用 `chatViewport.bottomMargin = imeBottom` 明确控制聊天页底边。实现时仍禁止给 `chatScroll` 或 `content` 增加 IME padding。
4. 点击“已完成思考”保持滚动位置时，如果展开内容高度很大，可能造成当前视口下方内容被挤出屏幕，这是预期；用户可手动滚动查看。
