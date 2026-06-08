# 安卓 APP 技术方案 v32

## 1. 背景与目标

本方案针对 `安卓APP_问题_v31.md` 中提出的拆分后问题设计。当前用户给出的 `/Volumes/shared/xzxg-shop/.ai/tech/安卓APP_问题_v31.md` 在本机未找到，实际读取的问题文档为 `/Users/leo/xzxg-shop/.ai/tech/安卓APP_问题_v31.md`。代码基线以拆分后的安卓工程 `/Users/leo/xzxg-shop/android-native` 为准。

目标：

- 修复聊天页思考过程、流式保存、风控提示、语音识别问题。
- 修复商品页悬浮购物车、分类弹窗、搜索栏、筛选行样式问题。
- 修复结算页商品清单和应付金额样式问题。
- 统一账号弹窗和全局底部弹窗动画。
- 统一所有顶部返回栏的返回按钮和页面标题对齐规则。
- 优化拆分后页面动画卡顿，尤其是侧栏动画。

非目标：

- 不调整后端接口协议，除非前端需要兼容已有字段。
- 不重新拆分 Activity 架构，本轮只在现有多 Activity 结构上修复行为和 UI。
- 不改变购物车、结算、订单、登录等业务流程。

## 2. 问题到模块映射

| 问题 | 当前主要位置 | 根因判断 | 方案入口 |
| --- | --- | --- | --- |
| 思考过程不实时展示 | `ChatActivity.handleSse()`、`ensureThinkingView()`、`completeThinkingView()` | `thinking_status` 只更新 loading，不创建思考面板；完成时强制展开；思考容器左 padding 与聊天气泡不一致 | 增强思考状态渲染，抽出统一思考视图，完成后默认收起 |
| 切历史或切页面时未保存已输出内容 | `ChatActivity`、`AgentStreamController`、`LocalChatStore` | 当前流式状态和 UI 当前会话绑定，切换后事件被忽略，且只在 `finishStream()` 或取消时保存 | 引入可 upsert 的 assistant draft，流式内容按会话保存，不依赖当前页面 |
| 风控提示重复 | `ChatActivity.handleSse()`、`failStream()`、`AgentMessageRenderer` | SSE `error` 和提示 block/系统提示可能同时展示相同文案 | 风控错误仅保留提示卡/Toast，不保存为对话气泡 |
| 语音识别不可用 | `ChatActivity`、`VoiceInputController`、`SpeechRealtimeClient` | `AUTO` 模式优先实时语音，实时服务不可用时没有回退到 Android 识别 | 增加实时失败自动 fallback，修正生命周期清理 |
| 商品页购物车角标被裁剪 | `ProductListActivity.addProductCartFab()` | FAB 只有 56dp，badge 使用负 margin 越界 | 放大容器并关闭裁剪，badge 放在容器内可视区域 |
| 商品页分类弹窗动画不自然 | `ProductListActivity.openCategoryDialog()`、`BottomSheetHelper` | 底部弹窗只设置 `Gravity.BOTTOM`，没有统一进入/退出动画 | 给 `BottomSheetHelper` 增加底部滑入滑出动画 |
| 搜索图标过小 | `ProductListActivity.renderProductListTab()` | 搜索图标使用 `ShopUi.muted()` 默认 13sp | 独立设置图标尺寸和容器宽度 |
| 商品类别和全部分居两侧 | `ProductListActivity.textFilterRow()` | 左右 TextView 各占 1 份，右侧右对齐 | 改为左侧紧邻布局，二者基线和垂直中心对齐 |
| 商品清单文字样式 | `CheckoutActivity.renderItemPanel()` | 商品名、数量、价格合并到一个灰色 TextView | 拆成名称、数量、价格三个 TextView |
| 应付金额红色 | `CheckoutActivity.renderAmountPanel()`、`renderBottomBar()` | `ShopUi.strong()` 默认黑色 | 增加金额红色样式 |
| 账号弹窗动画不统一 | `AccountActivity.editContact()`、`AccountUi.showConfirmBottomSheet()`、`BottomSheetHelper` | 所有弹窗都依赖默认 AlertDialog 动画 | 统一使用底部弹窗动画 |
| 顶部 bar 对齐 | 多个 Activity 的 `createBackTopBar()`、`AccountUi.createBackTopBar()` | 每个 Activity 手写 toolbar，返回按钮、标题 padding 和 font padding 不完全一致 | 创建统一 `TopBarHelper` 或并入 `ShopUi` |
| 页面动画卡顿 | `ChatActivity.showDrawer()`、`closeDrawerAnimated()`、商品/历史渲染 | 侧栏动画期间同步渲染历史、远程加载、旧 `TranslateAnimation` 触发布局和绘制压力 | 改用 property animation、延迟重任务、复用/分页渲染 |

## 3. 总体设计

本轮采用“先稳定聊天状态，再统一 UI 基础组件，最后做动画性能”的顺序。核心原则是把页面是否可见与业务状态保存解耦：

- 聊天流式输出属于会话数据，不属于当前正在展示的 Activity 视图。
- 思考过程、文本、卡片、followups 都需要能在流式未结束时保存为本地 draft。
- 顶部栏、底部弹窗、金额样式不再在各 Activity 中重复实现。
- 动画只做 `translationX/translationY/alpha`，避免动画期间大量重建视图。

## 4. 聊天页方案

### 4.1 思考过程实时展示

当前问题：

- `thinking_status` 只调用 `rememberThinkingStatus()` 和 `updateLoadingStatus()`，没有创建思考面板。
- 只有收到 `thinking_delta` 或文本 delta 缺失兜底时才会创建 `ThinkingViewState`。
- `completeThinkingView()` 把 `activeThinking.expanded = true`，导致完成后默认展开。
- `ensureThinkingView()` 给容器设置了 `dp(24)` 左 padding，而聊天列表本身已有 `dp(16)` padding，导致思考区向右偏。
- `renderStoredThinking()` 也强制 `expanded = true`，历史渲染与实时渲染状态不统一。

设计改造：

1. 在 `thinking_status` 到达时立即创建思考面板。
   - 新增 `handleThinkingStatus(String statusText)`。
   - 第一次收到状态时调用 `ensureThinkingView(chatList, true)`。
   - 根据状态文本映射到三个固定阶段：
     - 分析用户需求：`user_need`
     - 查询买手团经验：`buyer_experience`
     - 总结答案：`answer_summary`
   - 无法精确映射时追加到 `pendingThinkingStatuses`，并展示在当前 running 阶段。

2. `thinking_delta` 继续作为更精细的数据源。
   - 如果 `thinking_delta` 后到达，以 delta 内容覆盖或补充 status 生成的兜底内容。
   - 保留已有 `thinkingStageKey()` 兼容逻辑。

3. 完成后默认收起。
   - `completeThinkingView()` 改为：
     - `activeThinking.completed = true`
     - `markKnownThinkingStagesCompleted(activeThinking)`
     - 如果 `!activeThinking.userToggled`，则 `activeThinking.expanded = false`
     - 如果用户手动展开或收起过，则尊重用户选择。
   - `renderStoredThinking()` 对 completed 的历史思考默认 `expanded = false`。

4. 对齐聊天框左边界。
   - 思考容器不再额外设置 24dp 左 padding。
   - 使用与 assistant 气泡相同的外层 layout：
     - `chatList` 已经有左右 16dp padding。
     - thinking container 内部只保留垂直 padding。
     - 如需文本缩进，只缩进 stage marker，不缩进整个面板。

5. 抽出统一思考视图。
   - 建议新增 `ThinkingViews.java`，从 `ChatActivity` 内部迁出 `ThinkingViewState` 和渲染函数。
   - 这样实时渲染和历史渲染使用同一套布局，避免“刚渲染完异常、从历史进入正常”的分叉。

验收：

- 用户发送消息后，在 AI 正文出现前能看到“分析用户需求、查询买手团经验、总结答案”实时变化。
- 正文开始输出前，思考面板已经出现在聊天列表中。
- 思考完成后默认收起，点击后可展开。
- 实时页面和历史页面中的思考面板左边界一致，且与 AI 回复气泡左边界一致。

### 4.2 流式未结束时保存已输出内容

当前问题：

- `AgentStreamController.shouldIgnoreEvent(ownerLocalSessionId, currentLocalSessionId)` 会在切换历史后忽略原会话的后续事件。
- `ChatActivity` 的流式缓冲区与当前 UI 绑定，例如 `activeAssistantMarkdown`、`activeAssistantSegments`、`activeThinking`。
- 只有 `finishStream()`、`finishCanceledStream()` 会落库；普通切页/切历史没有保存入口。
- `LocalChatStore.saveAssistantTurn()` 每次都是插入新消息，不能覆盖同一轮未完成输出。

设计改造：

1. 在 `AgentStreamController` 暴露稳定 draft id。
   - 使用现有 `activeRequestId` 作为本轮 assistant draft id。
   - 新增 `activeRequestId()` getter。
   - 本地消息 id 约定为：`assistant_draft_` + `activeRequestId`。

2. 在 `LocalChatStore` 增加 upsert 方法。
   - 新增：
     - `upsertAssistantDraft(String localMessageId, String localSessionId, String content, String blocksJson, String followupsJson, String segmentsJson, String status)`
     - `markAssistantDraftCompleted(String localMessageId, ...)` 可复用 upsert 实现。
   - 不需要改表结构，因为 `messages.local_message_id` 已经是主键。
   - `status` 使用：
     - `streaming`：正在输出
     - `partial`：切页/切会话时保存的未完成输出
     - `completed`：完整结束
     - `canceled`：用户主动停止

3. 增加 `persistActiveAssistantDraft(String status)`。
   - 执行前先 `flushPendingAssistantForm(true)` 和 `flushActiveTextSegment(...)`，保证屏幕上已经出现的文本也进入 `activeAssistantSegments`。
   - 汇总：
     - `visibleMarkdown`
     - `activeAssistantBlocks`
     - `activeFollowups`
     - `activeAssistantSegments`
   - 只要任一内容非空，就调用 `upsertAssistantDraft(...)`。
   - 对本地会话调用 `chatStore.touchSession(ownerLocalSessionId)`，让历史列表能看到最新输出。
   - 对已有 server session 调用 `enqueueSessionSync(...)`，summary 使用已输出的 visible markdown。

4. 导航前保存。
   - 在以下入口调用 `persistActiveAssistantDraft("partial")`：
     - 历史会话 row 点击前。
     - 新建会话前。
     - 进入商品、购物车、订单、设置等 Activity 前。
     - `onPause()`，防止系统切后台。
   - 如果切换后不继续后台生成，调用 `streamController.requestStop(api)` 或 `cancelActiveCall()`，避免后续事件进入无 UI 状态。

5. 更稳的长期方案：流式处理与 UI 可见性解耦。
   - `handleSse(ownerLocalSessionId, event)` 不再因为 owner 不是当前 localSessionId 就丢弃事件。
   - 改为判断 `boolean visible = ownerLocalSessionId.equals(localSessionId) && chatList != null`。
   - visible 时更新 UI 和 draft。
   - invisible 时只更新缓冲和 draft，不访问 `chatList`。
   - 如果实现成本需要控制，第一轮先做“导航前保存并取消后台流”，第二轮再做“后台继续保存”。

验收：

- AI 回复流式输出到一半时，点击历史会话，再回到原会话，能看到已输出内容。
- AI 回复流式输出到一半时，进入商品页或购物车页，再回到聊天页，能看到已输出内容。
- 多次切换不会生成多条重复 assistant 消息。
- 主动停止后仍保存已输出内容，状态为 `canceled`。

### 4.3 风控提示只保留提示框

当前问题：

- 风控类 SSE error 可能先进入 `failStream()`，把文案作为聊天系统行或 loading 气泡展示。
- 另一路提示卡也会展示相同文案，造成重复。

设计改造：

1. 新增风控识别函数：
   - `isRiskControlError(JSONObject event)`
   - 判断字段：
     - `code` 包含 `risk`、`policy`、`blocked`、`forbidden`
     - 或 `message` 包含 `风控`、`平台风控限制`、`账号命中`

2. `handleSse()` 对 error 单独处理：
   - 风控错误调用 `showRiskControlNotice(message)`。
   - 不调用 `failStream(message)`。
   - 不调用 `chatStore.saveAssistantTurn(...)`。

3. `showRiskControlNotice()` 行为：
   - 移除 loading 气泡。
   - 用现有提示框或 warning card 展示一次。
   - 同步停止当前 stream 状态。
   - 对用户已输入消息保持保存，不删除。

4. 去重保护：
   - 新增 `lastRiskNoticeText` 和 `lastRiskNoticeAt`，2 秒内相同风控文案只展示一次。

验收：

- 命中风控时只出现一个提示框或提示卡。
- 聊天对话中不再出现一条重复的“当前账号命中平台风控限制...”AI/系统气泡。
- 风控后输入框恢复可用。

### 4.4 语音转文字修复

当前问题：

- `AndroidManifest.xml` 已声明 `RECORD_AUDIO`。
- `ChatActivity.enterVoiceMode()` 在 `AUTO` 模式下优先走 `startRealtimeSpeech()`，实时语音服务失败后只 toast，不自动回退到 Android 识别。
- 如果后端 `/speech/realtime` 未启用、WebSocket 不通或 token 不满足实时识别要求，用户感知就是语音不可用。

设计改造：

1. `AUTO` 模式增加 fallback。
   - `realtimeSpeechListener().onError(code, message)` 中：
     - 如果 mode 是 `AUTO`，且错误为 `speech_not_enabled`、`speech_network_error`、连接超时、启动失败，则清理实时录音状态后调用 `startAndroidInlineSpeech()`。
     - 如果 Android inline 不可用，再调用 `startSpeechRecognizerActivity()`。
   - 只有用户主动停止、权限拒绝、录音太短时不 fallback。

2. 修正清理顺序。
   - 实时失败时先停止 `PcmRecorder`，再 close `SpeechRealtimeClient`。
   - 清理后恢复输入框 hint、focus、actionButton 状态。
   - 避免 `voiceMode=false` 后又被 fallback 分支误判。

3. 增加可观察日志。
   - 记录语音模式、权限结果、实时连接失败 code、fallback 目标。
   - 日志只打 `Log.w/Log.d`，不改变 UI 文案。

验收：

- 未开启后端实时语音时，点击麦克风仍可弹出系统语音识别或使用 Android inline 识别。
- 授权麦克风后无需再次点击即可开始识别。
- 识别结果填入输入框，光标在末尾，键盘恢复。

## 5. 商品页方案

### 5.1 悬浮购物车按钮与角标

当前问题：

- `ProductListActivity.addProductCartFab()` 中 FAB 容器是 56dp。
- badge 使用 `topMargin = -4dp`、`rightMargin = -6dp`，超出父容器后被裁剪。
- 黑色圆形加“购物车”文字可读性一般，和页面内容边界不够清晰。

设计改造：

1. 容器调整。
   - 外层 `FrameLayout fabWrap` 尺寸 72dp x 72dp。
   - `fabWrap.setClipChildren(false)`、`root.setClipChildren(false)`。
   - 内层按钮 56dp x 56dp，放在 center。
   - badge 放在 `Gravity.RIGHT | Gravity.TOP`，不使用负 margin。

2. 视觉重设。
   - 按钮底色改白色。
   - 增加 1dp 浅灰描边和 8dp elevation。
   - 文案保留“购物车”或改为购物车符号 + 小字，要求不影响现有点击行为。
   - badge 使用红色，最小 20dp 高，宽度 20-32dp，自适应 `99+`。

3. 布局位置。
   - 外层 wrap 右下角定位，right 16dp，bottom 76dp。
   - 角标始终在 wrap 内，避免裁剪。

验收：

- 购物车数量为 1、9、10、99+ 时角标完整显示。
- FAB 在商品列表和活动 tab 都可点击进入购物车。
- 底部导航不遮挡 FAB。

### 5.2 商品类别弹窗动画

当前问题：

- `BottomSheetHelper.show()` 仅设置 `Gravity.BOTTOM`，使用系统默认 AlertDialog 动画。
- 商品分类、账号编辑、退出登录、删除账号使用同一个 helper，但动画没有明确统一。

设计改造：

1. 新增动画资源：
   - `res/anim/bottom_sheet_slide_in.xml`
   - `res/anim/bottom_sheet_slide_out.xml`
   - `res/values/styles.xml` 增加 `XzxgBottomSheetAnimation`

2. `BottomSheetHelper.show()` 中设置：
   - `window.setWindowAnimations(R.style.XzxgBottomSheetAnimation)`
   - `window.setGravity(Gravity.BOTTOM)`
   - `params.width = MATCH_PARENT`
   - `params.height = WRAP_CONTENT`

3. 所有底部弹窗保持统一：
   - `ProductListActivity.openCategoryDialog()`
   - `AccountActivity.editContact()`
   - `AccountUi.showConfirmBottomSheet()`
   - `OrderListActivity` 评价弹窗如同样走 bottom sheet，也切到 helper。

验收：

- 商品分类弹窗从屏幕底部滑入，关闭时向底部滑出。
- 手机号、邮箱、删除账号、退出登录弹窗动画一致。
- 弹窗背景 dim、圆角、宽度一致。

### 5.3 搜索栏图标和筛选行

搜索栏：

- 将搜索图标 TextView 从 `ShopUi.muted()` 改为独立 TextView。
- 设置 `textSize = 22sp`，宽度 36dp 或 40dp。
- 图标垂直居中，颜色维持 `Color.rgb(107, 114, 128)`。

筛选行：

- 替换 `textFilterRow(String label, String value)` 的左右等分布局。
- 新布局：
  - 外层横向 LinearLayout，`Gravity.CENTER_VERTICAL | Gravity.LEFT`。
  - label 使用 muted 样式，宽度 wrap_content。
  - value 使用 strong 样式，左 margin 8dp，宽度 wrap_content。
  - 右侧不再占满并右对齐。
- 文案保留“商品类别”和当前类别，例如“商品类别  全部  >”。

验收：

- “商品类别”和“全部”紧邻排列，不分居页面两侧。
- 两者垂直中心和文字基线视觉对齐。
- 搜索图标明显大于原来，搜索输入行为不变。

## 6. 结算页方案

### 6.1 商品清单样式

当前问题：

- `CheckoutActivity.renderItemPanel()` 使用一个灰色 `ShopUi.muted()` 文本展示：`商品名 x数量  ¥价格`。
- 无法分别设置商品名、数量、价格样式。

设计改造：

1. row 内图片右侧改为横向/纵向组合：
   - 商品名：`TextView name`
     - 黑色 `Color.rgb(17, 24, 39)`
     - `Typeface.DEFAULT_BOLD`
     - 15sp 或 16sp
     - 单行或最多 2 行
   - 数量：`TextView quantity`
     - 保持当前 muted 颜色
     - 文案 `x` + quantity
   - 价格：`TextView price`
     - 红色 `Color.rgb(220, 38, 38)`
     - bold
     - 15sp 或 16sp

2. 建议布局：
   - 图片 48dp。
   - 中间 `name + quantity` 占满剩余空间。
   - price 靠右，宽度 wrap_content。

### 6.2 应付金额红色

位置：

- `CheckoutActivity.renderAmountPanel()`
- `CheckoutActivity.renderBottomBar()`

设计：

- 新增 `ShopUi.priceStrong(Context, String)` 或 `ShopUi.redStrong(Context, String)`。
- `金额明细` 中的“应付：¥xx”使用红色加粗。
- 底部提交栏 “应付 ¥xx” 中金额整体使用红色加粗。如果要只改数字，可以用 `SpannableString` 仅给数字部分设置红色和 bold。

验收：

- 商品清单中商品名黑色加粗、数量颜色不变、价格红色加粗。
- 金额明细和底部 bar 的应付金额为红色。
- 结算提交行为不变。

## 7. 账号弹窗和全局 BottomSheet 方案

当前代码：

- `AccountActivity.editContact()` 使用 `BottomSheetHelper.show(this, box, true)`。
- `AccountUi.showConfirmBottomSheet()` 也使用同一个 helper。
- helper 没有统一动画资源。

设计：

1. `BottomSheetHelper` 成为唯一底部弹窗入口。
2. 增加：
   - `show(Activity activity, View content, boolean cancelable)`
   - `showConfirm(...)`
   - 如需无动画，额外提供 `showWithoutAnimation(...)`，默认不使用。
3. `BottomSheetHelper.show()` 统一：
   - 背景透明。
   - dimAmount 0.35。
   - bottom gravity。
   - 横向 12dp 外边距。
   - 底部安全区 padding。
   - bottom slide animation。

验收：

- 手机号、邮箱、删除账号、退出登录弹窗出现和消失动画一致。
- 商品类别弹窗与账号弹窗动画一致。
- 点击遮罩关闭行为保持不变。

## 8. 顶部 bar 统一方案

当前问题：

- `ProductListActivity`、`CartActivity`、`CheckoutActivity`、`ProductDetailActivity`、`HelpActivity`、`AboutActivity`、`AdvancedSettingsActivity`、`OrderListActivity` 都有重复 `createBackTopBar()`。
- `AccountUi.createBackTopBar()` 也有一套实现。
- 返回按钮使用字符 `‹`，title 有额外 padding，部分 TextView includeFontPadding 不一致。

设计：

1. 新增统一组件：`TopBarHelper.java`。

建议接口：

```java
public final class TopBarHelper {
    public static View backBar(Activity activity, int backgroundColor, String title, Runnable onBack);
    public static View mainBar(Activity activity, int backgroundColor, String title, View.OnClickListener menuClick);
}
```

2. 统一布局规则：
   - toolbar 高度由调用方继续设置 56dp。
   - toolbar horizontal，`Gravity.CENTER_VERTICAL`。
   - 左侧返回按钮触控区 44dp x 44dp。
   - 返回按钮 `includeFontPadding(false)`，`Gravity.CENTER`。
   - title 高度 match parent，`Gravity.CENTER_VERTICAL`。
   - title 左 margin 8dp，不再设置不一致的内部 padding。
   - 右侧 spacer 44dp，保持标题区域和返回按钮视觉平衡。

3. 替换范围：
   - `AccountUi.createBackTopBar()`
   - `ProductListActivity.createTopBar()`
   - `CartActivity.createBackTopBar()`
   - `CheckoutActivity.createBackTopBar()`
   - `ProductDetailActivity.createBackTopBar()`
   - `OrderListActivity.createBackTopBar()`
   - `HelpActivity.createBackTopBar()`
   - `AboutActivity.createBackTopBar()`
   - `AdvancedSettingsActivity.createBackTopBar()`

验收：

- 所有带返回按钮页面，返回按钮和标题文字垂直中心一致。
- 标题不再相对返回按钮偏上或偏下。
- 返回行为不变。

## 9. 性能与动画优化方案

### 9.1 侧栏卡顿原因

当前 `ChatActivity` 拆分后仍承担聊天主页和历史侧栏：

- 打开侧栏时会立即构建整个 drawer view。
- 同步渲染本地历史列表首屏。
- 同时触发远程历史加载。
- 侧栏使用旧的 `TranslateAnimation`，动画和 view 构建/列表渲染容易抢主线程。
- 历史搜索和远程刷新会 `removeAllViews()` 重建列表，列表多时掉帧明显。

### 9.2 优化设计

1. 侧栏动画改为 property animation。
   - 初始 `drawer.setTranslationX(-panelWidth)`。
   - `drawer.animate().translationX(0).setDuration(220)`。
   - 遮罩 `drawerLayer.animate().alpha(1f).setDuration(160)`。
   - 动画期间 `drawer.setLayerType(View.LAYER_TYPE_HARDWARE, null)`，结束后恢复。

2. 延迟重任务。
   - 先显示空 drawer skeleton 或本地缓存首屏。
   - `drawer.postDelayed(..., 80)` 后再加载远程历史。
   - 搜索 debounce 已有，保留。

3. 历史列表增量更新。
   - 首屏不要每次全量 `removeAllViews()`，仅在 query 改变时重建。
   - append 时保持 `existingIds` 去重。
   - 本轮不强制引入 RecyclerView；如果历史量继续增大，下一轮迁移为 RecyclerView。

4. 页面切换减少重复 setContentView。
   - 商品页 tab 切换尽量复用 root/content，先做低风险优化：
     - 缓存商品列表数据已存在，继续保留。
     - 避免切 tab 后马上刷新购物车角标两次。
     - 图片加载继续走 `ImageLoader` 缓存。

5. 动画统一原则。
   - drawer 使用 translationX。
   - bottom sheet 使用 window animation。
   - 页面普通跳转使用 Android 默认 Activity 动画，不额外叠加复杂动画。

验收：

- 侧栏打开动画不明显掉帧。
- 打开侧栏时首屏交互可用，远程历史稍后刷新。
- 搜索历史时输入不卡顿。

## 10. 实施顺序

### 第 1 轮：聊天状态修复

修改文件：

- `ChatActivity.java`
- `AgentStreamController.java`
- `LocalChatStore.java`
- 可选新增 `ThinkingViews.java`

工作：

1. `thinking_status` 实时创建并更新思考面板。
2. 完成思考后默认收起。
3. 修复思考面板左对齐。
4. 增加 assistant draft upsert。
5. 导航前、切历史前、`onPause()` 保存 partial。
6. 风控 error 只展示提示，不落对话气泡。

### 第 2 轮：语音识别修复

修改文件：

- `ChatActivity.java`
- `VoiceInputController.java`
- `SpeechRealtimeClient.java`

工作：

1. AUTO 模式实时失败 fallback 到 Android inline。
2. Android inline 不可用时 fallback 到系统语音 Activity。
3. 修复失败、取消、超时后的状态清理。
4. 增加日志便于验证。

### 第 3 轮：商品页和结算页 UI

修改文件：

- `ProductListActivity.java`
- `CheckoutActivity.java`
- `ShopUi.java`

工作：

1. 重做商品页购物车 FAB 和 badge。
2. 搜索图标放大。
3. 商品类别筛选行紧邻布局。
4. 结算商品清单拆分 TextView。
5. 应付金额改红色。

### 第 4 轮：底部弹窗与顶部 bar 统一

修改文件：

- `BottomSheetHelper.java`
- `AccountActivity.java`
- `AccountUi.java`
- `ProductListActivity.java`
- 新增 `TopBarHelper.java`
- 各页面 Activity 的顶部栏调用点
- `res/anim/*.xml`
- `res/values/styles.xml`

工作：

1. BottomSheet 统一从底部滑入滑出。
2. 所有账号和商品类别弹窗切到统一 helper。
3. 新增统一顶部栏组件。
4. 替换重复 `createBackTopBar()`。

### 第 5 轮：动画性能优化与回归

修改文件：

- `ChatActivity.java`
- `ChatHistoryDrawer.java`
- 可能涉及 `ShopUi.java`

工作：

1. 侧栏动画改为 property animation。
2. 远程历史加载延后到动画后。
3. 历史列表避免不必要全量重建。
4. 全量手工回归。

## 11. 回归测试清单

聊天：

- 发送普通导购问题，确认思考过程实时展示且在正文前出现。
- 思考完成后默认收起，点击可展开。
- 流式输出中切到另一个历史，再回来，已输出内容存在。
- 流式输出中进入商品页，再回聊天，已输出内容存在。
- 主动停止生成，已输出内容保存为 canceled。
- 命中风控，只出现一条提示框或提示卡。
- 语音输入在实时服务不可用时可 fallback 到 Android 语音识别。

商品：

- 购物车数量 1、10、99+ 均完整显示。
- FAB 点击进入购物车。
- 商品类别弹窗从底部滑入。
- 搜索图标变大。
- “商品类别”和“全部”紧邻且对齐。

结算：

- 商品清单中商品名黑色加粗，数量保持灰色，价格红色加粗。
- 金额明细和底部提交栏应付金额红色。
- 提交订单流程不变。

账号：

- 手机号、邮箱、删除账号、退出登录弹窗动画一致。
- 弹窗确认和取消行为不变。

顶部栏：

- 商品、购物车、确认订单、订单、设置、账号管理、商品详情、帮助、关于、高级设置页面返回按钮和标题垂直对齐。
- 返回行为不变。

性能：

- 侧栏打开和关闭动画流畅。
- 侧栏打开后远程历史刷新不阻塞动画。
- 商品页滚动和加载更多不卡顿。

## 12. 构建验证

每轮完成后执行：

```bash
cd /Users/leo/xzxg-shop/android-native
./gradlew :app:assembleDebug
```

最终完成后建议连接设备做手工验证：

```bash
/Users/leo/Library/Android/sdk/platform-tools/adb install -r app/build/outputs/apk/debug/app-debug.apk
```

需要重点观察 log：

- `ThinkingEvent`
- `SpeechRealtimeClient`
- `VoiceInputController`
- `AgentStreamController`

## 13. 风险与控制

- assistant draft upsert 如果实现不当会造成重复消息。控制方式是用稳定 `local_message_id = assistant_draft_ + requestId`，完成时覆盖同一条消息。
- 后台继续流式保存会增加复杂度。第一版可以先在切页前保存 partial 并取消后台流，后续再升级为不可见会话继续接收并保存。
- 顶部栏统一会触及多个 Activity，但只替换 UI 组件，不改业务逻辑。
- BottomSheet 动画资源需要确认低版本 Android 兼容，必要时保留无动画 fallback。
