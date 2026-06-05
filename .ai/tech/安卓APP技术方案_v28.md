# 安卓 APP 技术方案 v28

## 1. 背景

本文针对 [安卓APP_问题_v27.md](./安卓APP_问题_v27.md) 设计下一版 Android 原生 APP 改造方案。

本轮问题集中在两个方向：

1. 思考过程仍未真正展示后端返回的“分析用户需求 / 查询买手团经验”详情，只停在“分析用户需求”阶段后直接进入回答。
2. 键盘弹出后的页面避让方式错误，表现像聊天列表被手动上滑，而不是键盘把键盘以上的整个页面顶上去。

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

这会形成“双重避让”：

- 系统已经 resize Activity 可用高度。
- Android 代码又给 `content` 加了 `imeBottom` padding。
- 还强制 `scrollBottom()`，表现像列表被手动滑动。
- 当聊天列表已经到底部时，继续上滑失效，输入区/内容恢复也异常。

v5 技术方案已经明确：聊天页应回到普通纵向布局，依赖系统 `adjustResize`，不要再手动计算键盘高度移动页面。

## 4. v28 总体目标

1. Android 端严格以 `thinking_delta` 作为详细思考过程的数据源。
2. `status` / `thinking_status` 只作为短 loading 或诊断信息，不再伪造成详细思考内容。
3. 如果后端没有发送 `thinking_delta`，Android 应明确记录日志并降级显示短状态，而不是展示空的三阶段内容。
4. 收到 `user_need` 和 `buyer_experience` 后，必须把 `delta/items` 实时展示到对应模块。
5. 主回答开始前或回答开始时，若已收到思考详情，思考区稳定保持在 AI 正文上方。
6. 点击“已完成思考”展开/收起时，页面滚动位置保持不变。
7. 删除三阶段之间的小竖线，只保留适当间距。
8. 重新设计展开符号。
9. 键盘弹出时，键盘以上的整个页面被系统 resize 顶上去，不再通过 `content` padding 或强制滚动模拟。

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

### 8.1 回到系统 resize

保留：

```xml
android:windowSoftInputMode="adjustResize"
```

保留：

```java
getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
```

删除聊天页 `bindImeInsets()` 中对 `content` 的手动键盘 padding：

```java
content.setPadding(0, 0, 0, bottomInset);
scrollBottom();
```

改为：

```java
private void bindImeInsets() {
    root.setOnApplyWindowInsetsListener((view, insets) -> {
        return insets;
    });
}
```

或者聊天页不再调用 `bindImeInsets()`。

### 8.2 布局结构保持普通纵向布局

聊天页应保持：

```text
content LinearLayout vertical
  topBar 固定高度
  chatMessageLayer height=0 weight=1
  composerBar wrap_content
```

键盘弹出时：

- 系统缩小 Activity 可用高度。
- `content` 整体重新 layout。
- `composerBar` 自然贴到键盘上方。
- `chatMessageLayer` 高度自然变小。
- 不需要模拟滑动。

### 8.3 只在输入框聚焦时适度滚动

可以保留：

```java
input.setOnFocusChangeListener(...)
```

但不要在 WindowInsets 每次变化时 `scrollBottom()`。

建议：

```java
input.postDelayed(() -> {
    if (input.hasFocus()) {
        scrollBottom();
    }
}, 180);
```

该滚动只用于让最新消息可见，不承担键盘避让职责。

### 8.4 退出聊天页时恢复窗口模式

现有其他页面可能使用：

```java
useKeyboardNothing()
restoreKeyboardModeForActivePage()
```

v28 要求：

- 聊天页进入时 `useKeyboardResize()`。
- 离开聊天页或打开抽屉/其他页面时保持现有恢复逻辑。
- 不再叠加 IME padding。

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

- `bindImeInsets()`
  - 删除 `content.setPadding(...)`。
  - 删除 `imeBottom > 0 -> scrollBottom()`。
- `renderChatHome()`
  - 保留普通纵向布局。
  - 保留 `useKeyboardResize()`。
- 输入框 focus 逻辑：
  - 只延迟滚到底部一次，不承担避让职责。

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

1. 聊天列表滚到底部。
2. 点击输入框弹出键盘。
3. 观察键盘以上的整个页面是否被顶上去。
4. 收起键盘。
5. 观察页面是否恢复原状。

成功标准：

- 输入栏贴住键盘顶部。
- 顶栏、聊天区、输入栏作为一个整体被系统 resize。
- 不出现额外底部 padding。
- 不出现键盘收起后页面停在异常上移位置。
- 聊天列表到底时，键盘弹出仍能正确顶起页面。

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
3. 删除 IME padding 后，若个别系统键盘 resize 不稳定，应先确认 `adjustResize` 是否被全屏/透明系统栏破坏，不要重新回到手动 padding。
4. 点击“已完成思考”保持滚动位置时，如果展开内容高度很大，可能造成当前视口下方内容被挤出屏幕，这是预期；用户可手动滚动查看。

