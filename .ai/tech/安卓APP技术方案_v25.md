# 安卓 APP 技术方案 v25

## 1. 背景

本文针对 [安卓APP_问题_v24.md](./安卓APP_问题_v24.md) 设计下一版 Android 原生 APP 改造方案。

本轮问题分为三类：

1. 语音输入转换成功后不能自动发送，应先把识别文字填入聊天输入框，允许用户像普通输入一样编辑后再发送。
2. Agent 思考过程不能只展示一个简单的“正在 xxx”状态，需要展示更详细的三段式过程，并在完成后默认收起为“已完成思考”。
3. 需要在模拟器里验证 `AI 帮加购`、`AI 帮下单`、图片查询的响应时间和相似商品召回效果。

参考图：

- 展开态：`./assets/详细信息_展开.jpg`
- 收起态：`./assets/详细信息_收起.jpg`

## 2. 当前实现核对

### 2.1 语音输入

当前 Android 端已实现讯飞实时语音通道和系统语音降级，核心代码在 `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`：

- `startRealtimeSpeech()`
- `finishRealtimeVoice()`
- `sendRealtimeSpeechFinal()`
- `startSpeechRecognition()`
- `sendRecognizedSpeech()`
- `onActivityResult()` 中的 `REQUEST_SPEECH_INPUT`

当前行为：

```text
语音识别 final 文本
  -> sendRealtimeSpeechFinal(text) / sendRecognizedSpeech(text)
  -> sendMessage(text)
  -> 立即生成用户气泡并调用后端 Agent
```

这与 v24 问题文档要求不一致。新行为应为：

```text
语音识别 final 文本
  -> 填入 input EditText
  -> 恢复键盘输入态
  -> 用户可编辑
  -> 用户点击发送后再 sendMessage(inputText)
```

### 2.2 思考过程展示

当前 Android 端 SSE 处理在 `handleSse()`：

- `message_start`：记录 `run_id`
- `status`：调用 `updateLoadingStatus(event.text)`
- `text_delta`：追加主回答正文
- `content_delta` / `block_delta`：渲染结构化块
- `message_end`：结束流式输出

当前 `status` 只更新一个 loading 文本：

```text
正在思考...
正在处理...
```

v25 需要把状态升级为可展示内容的思考详情组件，并支持完成后默认收起。

### 2.3 后端协议现状

`.ai/api/Agent输出协议_v1.md` 已定义 `status` 事件：

```json
{
  "type": "status",
  "run_id": "run_xxx",
  "stage": "intent",
  "text": "正在理解你的需求"
}
```

但 v24 问题要求的“分析用户需求”“查询买手团经验”需要承载更长的结构化内容，不适合继续只依赖 `status.text`。v25 需要扩展 SSE 协议。

## 3. v25 总体方案

v25 采用“前后端协议小扩展 + Android 原生组件渲染 + 模拟器验收闭环”的方案。

核心目标：

1. 语音识别只负责把文本输入到输入框，不自动触发 Agent。
2. 后端通过 SSE 输出思考详情事件，Android 渲染三段式思考过程。
3. 思考完成后默认收起，只展示“已完成思考”，点击后可展开查看详情。
4. 在模拟器中验证加购、下单、图片查询，并记录响应时间和结果质量。

## 4. 语音输入改造

### 4.1 行为定义

语音输入完成后：

- 将最终识别文本写入聊天输入框。
- 保留文本，不调用 `sendMessage()`。
- 输入框恢复可编辑、可聚焦状态。
- 光标移动到文本末尾。
- action button 变为发送态。
- 如果识别文本为空，保持现有提示“没有识别到内容”。
- 用户再次点击麦克风时，若输入框已有文本，不应清空用户已编辑内容，除非用户明确删除。

### 4.2 Android 方法设计

新增统一入口：

```java
private void fillRecognizedSpeechToInput(String text) {
    String value = text == null ? "" : text.trim();
    if (value.isEmpty()) {
        toastLine("没有识别到内容");
        finishVoiceMode();
        return;
    }
    voiceResultSent = true;
    cleanupVoiceInputState();
    if (input != null) {
        input.setText(value);
        input.setSelection(value.length());
        input.setHint("输入问题或直接发送...");
        input.setFocusableInTouchMode(true);
        input.setFocusable(true);
        input.requestFocus();
    }
    showKeyboard();
    updateInputActionButtonState();
}
```

替换以下直接发送点：

- `sendRealtimeSpeechFinal(text)`：改为 `fillRecognizedSpeechToInput(value)`。
- `sendRecognizedSpeech(text)`：改为 `fillRecognizedSpeechToInput(value)`。
- `onActivityResult(REQUEST_SPEECH_INPUT)`：改为 `fillRecognizedSpeechToInput(value)`。
- `SpeechRecognizer.ERROR_NO_MATCH / ERROR_SPEECH_TIMEOUT` 使用 partial 文本时，也走同一方法。

注意：`cleanupRealtimeVoice()` 当前会清理 `input.hint` 和语音状态，可继续复用，但不能在之后调用 `sendMessage()`。

### 4.3 发送按钮状态

`updateInputActionButtonState()` 应以输入框文本为最高优先级：

```text
streaming -> 停止
input 非空 -> 发送
voiceMode 且正在结束 -> 识别中
voiceMode -> 停止录音
默认 -> 麦克风
```

这样语音识别完成后，输入框非空，按钮自然变为发送。

## 5. 思考详情协议设计

### 5.1 新增 SSE 事件：thinking_delta

新增事件类型 `thinking_delta`，用于输出可展示的思考过程摘要。该事件不是模型私有 chain-of-thought，而是后端可公开给用户的业务分析摘要。

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

字段：

- `stage`：阶段枚举。
- `status`：`running`、`completed`、`failed`。
- `title`：展示标题。
- `delta`：追加文本。
- `items`：可选结构化卡片，用于买手团经验。

阶段枚举：

```text
user_need       -> 分析用户需求
buyer_experience -> 查询买手团经验
answer_summary  -> 总结答案
```

### 5.2 三个模块内容

#### 分析用户需求

展示后端返回的用户需求分析相关内容。

推荐来源：

- intent 识别结果。
- query rewrite 结果。
- 用户约束解析结果。
- 商品类目、预算、使用场景、负向约束。

示例：

```json
{
  "type": "thinking_delta",
  "stage": "user_need",
  "status": "completed",
  "title": "分析用户需求",
  "delta": "用户需要溪流路亚装备，重点关注轻量、短竿、灵敏、适合新手和溪流环境。",
  "items": []
}
```

#### 查询买手团经验

展示后端返回的买手团经验相关内容。

推荐 `items` 结构：

```json
{
  "type": "thinking_delta",
  "stage": "buyer_experience",
  "status": "completed",
  "title": "查询买手团经验",
  "delta": "",
  "items": [
    {
      "title": "路亚新手装备选购",
      "summary": "新手优先选择轻量竿、1000 型纺车轮和 0.6 号 PE 主线，降低抛投和控饵难度。",
      "source": "buyer_note",
      "score": 0.82
    }
  ]
}
```

Android 展示为横向经验卡片，样式参考展开图中的卡片区。若 `items` 为空，则展示 `delta` 文本。

#### 总结答案

只展示固定文案：

```text
总结答案完成
```

后端可以发送：

```json
{
  "type": "thinking_delta",
  "stage": "answer_summary",
  "status": "completed",
  "title": "总结答案",
  "delta": "总结答案完成",
  "items": []
}
```

Android 不展示额外推理文本，只显示“总结答案完成”。

### 5.3 兼容旧 status

`status` 继续保留，用于旧客户端和普通短状态。Android v25 的处理规则：

- 收到 `thinking_delta` 后，以 `thinking_delta` 渲染思考组件。
- 没有 `thinking_delta` 时，保留旧 `status.text` loading 文本。
- `status.stage=done` 不能直接替代“已完成思考”，只作为兼容状态。

## 6. Android 思考组件设计

### 6.1 组件结构

新增运行时状态：

```java
private ThinkingViewState activeThinking;

private static class ThinkingViewState {
    LinearLayout container;
    TextView header;
    LinearLayout detail;
    boolean expanded = true;
    boolean completed = false;
    JSONObject stages = new JSONObject();
}
```

每个阶段维护：

```java
private static class ThinkingStageState {
    String stage;
    String title;
    StringBuilder text = new StringBuilder();
    JSONArray items = new JSONArray();
    String status = "running";
    LinearLayout row;
    TextView titleView;
    TextView bodyView;
    LinearLayout itemList;
}
```

### 6.2 展开态样式

展开态顺序：

```text
已完成思考/正在思考  ^
  分析用户需求完成
    用户需求分析内容
  查询买手团经验完成
    买手团经验卡片或文本
  总结答案完成
```

视觉策略：

- 容器在 assistant 主回答前渲染，不放进主回答气泡内部。
- 左侧使用细竖线串联步骤。
- 每个完成步骤用圆形 check 标记。
- 标题为灰黑色，内容为浅灰文本。
- 买手团经验使用横向滚动小卡片，卡片内显示标题和摘要。
- 不把思考详情写入最终回答 markdown。

### 6.3 收起态样式

当所有阶段完成并收到 `message_end` 或 `thinking_delta(stage=answer_summary,status=completed)` 后：

- 默认收起。
- 只显示一行：

```text
✦ 已完成思考 ˅
```

点击后展开；再次点击收起。

如果流式过程中用户点击收起：

- 允许收起。
- 后续 `thinking_delta` 继续更新内部状态。
- 完成后保持用户当前展开/收起选择，不强制打开。

### 6.4 与主回答的布局关系

推荐顺序：

```text
用户气泡
思考组件
Assistant 主回答气泡
商品卡 / 表格 / followups
```

当前 `addLoadingBubble()` 只创建一个 `TextView`。v25 改为：

- `addLoadingBubble()` 保留给兼容 status。
- 新增 `ensureThinkingView()` 创建思考组件。
- 收到首个 `thinking_delta` 时移除普通 loading bubble，插入思考组件。
- 主回答 `text_delta` 到达时照常创建 assistant bubble，不覆盖思考组件。

### 6.5 历史消息持久化

当前 `chatStore.saveAssistantTurn()` 保存：

- markdown
- blocks
- followups
- segments
- status

v25 建议扩展保存 thinking JSON：

```text
assistant_turns.thinking_json
```

若暂不改数据库，可先把 thinking 作为一个特殊 segment 存入 `activeAssistantSegments`：

```json
{
  "type": "thinking",
  "stages": {}
}
```

历史渲染时：

- completed 状态默认收起。
- canceled/failed 状态显示已完成的阶段，标题为“思考已停止”或“思考失败”。

## 7. 后端输出改造

### 7.1 用户需求分析输出点

在 Agent run 的 intent / query rewrite 完成后发送：

```text
thinking_delta stage=user_need status=running
thinking_delta stage=user_need status=completed
```

内容必须是面向用户的摘要，不输出模型内部逐步推理。

### 7.2 买手团经验输出点

在检索买手经验、RAG 片段、商品经验库后发送：

```text
thinking_delta stage=buyer_experience status=running
thinking_delta stage=buyer_experience status=completed items=[...]
```

如果当前问题没有命中买手经验：

- 仍展示“查询买手团经验完成”。
- 内容可为“未找到强相关买手经验，已改用商品信息和用户需求进行推荐。”

### 7.3 总结答案输出点

在 final answer 开始生成或完成前发送：

```text
thinking_delta stage=answer_summary status=completed delta=总结答案完成
```

如果主回答已经开始流式输出，也可以发送该事件，但 Android 应把思考组件稳定放在主回答上方。

## 8. AI 帮加购与 AI 帮下单验收方案

### 8.1 前置条件

模拟器测试前确认：

- 后端服务可访问。
- Android 高级设置中的 API Base URL 指向当前后端。
- 已登录普通用户账号。
- 商品列表存在有库存商品。
- 购物车初始状态可控，建议测试前清空或记录当前数量。

### 8.2 AI 帮加购测试

测试输入：

```text
帮我找一款适合通勤的耳机，并帮我加入购物车
```

验收标准：

- Agent 能推荐具体商品。
- SSE 中能看到工具调用或结构化块返回购物车状态。
- Android 展示加购成功提示或购物车状态卡。
- 进入购物车页后目标商品存在。
- 购物车角标数量增加。

失败排查：

- 如果 Agent 只推荐不加购，检查意图策略是否允许 cart 工具。
- 如果后端返回加购成功但 Android 未展示，检查 `cart_state` block 渲染。
- 如果 Android 购物车页无商品，检查 token、用户 ID、购物车接口返回。

### 8.3 AI 帮下单测试

测试输入：

```text
把购物车里选中的商品帮我下单
```

验收标准：

- Agent 调用 checkout 能力。
- 后端返回订单摘要。
- Android 展示订单 summary block。
- 订单页能看到新订单。
- 购物车中已下单商品不再作为选中待结算项。

失败排查：

- 如果没有选中项，先在购物车页选中商品再测。
- 如果库存不足，换有库存商品。
- 如果重复下单失败，应展示后端错误，不能静默失败。

## 9. 图片查询验收方案

### 9.1 测试入口

使用聊天输入框图片按钮选择一张商品相关图片，发送图片查询需求：

```text
帮我找和这张图类似的商品
```

### 9.2 响应时间指标

Android 端记录四个时间点：

```text
t0 用户点击发送
t1 附件上传完成
t2 收到首个 SSE 事件
t3 收到首个商品卡或 product_refs
t4 message_end
```

建议日志：

```text
ImageSearchPerf upload_ms=...
ImageSearchPerf first_event_ms=...
ImageSearchPerf first_product_ms=...
ImageSearchPerf total_ms=...
```

验收目标：

- 上传图片在本地网络下通常小于 2s。
- 首个 SSE 事件小于 3s。
- 首个相似商品结果小于 8s。
- 总耗时小于 15s。

若超过目标，需要区分：

- 图片上传慢：压缩图片或限制上传尺寸。
- embedding 慢：检查 DashScope / 后端向量接口耗时。
- Milvus 慢：检查 collection、索引、topK。
- Android 渲染慢：检查图片加载和主线程工作量。

### 9.3 相似商品效果指标

检查两个方面：

1. 是否返回同品类商品。
2. 是否返回视觉或语义上类似的商品。

建议测试集：

- 一张耳机图。
- 一张手机图。
- 一张美妆图。
- 一张服饰图。
- 一张食品图。

每次记录：

```text
query_image
top1 商品是否同品类
top3 同品类数量
top5 同品类数量
是否出现明显无关商品
首个商品耗时
总耗时
```

验收标准：

- top1 应为同品类或强相关商品。
- top3 至少 2 个同品类。
- top5 不应出现明显无关的大类跳转。
- 若图片是数据集已有商品，top1 或 top3 应召回原商品或高度相似商品。

## 10. 实施顺序

### 阶段 1：语音结果入输入框

1. 新增 `fillRecognizedSpeechToInput()`。
2. 替换实时语音、系统语音、RecognizerIntent 的自动发送逻辑。
3. 调整 action button 状态。
4. 手测语音识别后编辑再发送。

### 阶段 2：思考详情协议

1. 后端新增 `thinking_delta` SSE 事件。
2. intent/query rewrite 后输出 `user_need`。
3. 买手经验检索后输出 `buyer_experience`。
4. final 生成完成前输出 `answer_summary`。
5. 保留旧 `status` 事件兼容。

### 阶段 3：Android 思考组件

1. 新增 active thinking 状态。
2. 收到 `thinking_delta` 时创建组件。
3. 实现三段式展开渲染。
4. 完成后默认收起为“已完成思考”。
5. 历史消息支持 thinking 持久化或 segment 降级保存。

### 阶段 4：模拟器验收

1. 构建安装 Android debug 包。
2. 登录测试账号。
3. 执行 AI 帮加购。
4. 执行 AI 帮下单。
5. 执行图片查询并记录耗时和 topK 结果。
6. 汇总失败项并进入修复。

## 11. 风险与边界

- `thinking_delta` 只能展示业务摘要，不能输出模型内部完整推理链。
- 语音识别 final 文本进入输入框后，用户可能再次编辑为空；发送按钮应按普通输入规则处理。
- 如果后端暂时不能提供买手经验 `items`，Android 需要支持纯文本 fallback。
- 思考组件不能阻塞主回答流式展示；主回答到达时应立即渲染。
- 图片查询耗时受外部 embedding 服务影响，需要后端日志和 Android 日志一起定位。
- AI 帮下单涉及真实购物车状态，测试账号和测试数据应隔离，避免污染演示数据。

## 12. 验收清单

- 语音识别成功后，文本进入输入框且不会自动发送。
- 语音文本可编辑，编辑后点击发送使用编辑后的内容。
- 思考过程展示“分析用户需求”“查询买手团经验”“总结答案”三段。
- 思考完成后默认收起，只显示“已完成思考”。
- 点击“已完成思考”可以展开详情，再次点击可收起。
- 主回答、商品卡、followups 正常显示，不被思考组件覆盖。
- AI 帮加购成功后购物车数量和购物车页内容正确。
- AI 帮下单成功后订单页出现新订单。
- 图片查询记录响应时间，topK 结果满足同品类和相似商品要求。
