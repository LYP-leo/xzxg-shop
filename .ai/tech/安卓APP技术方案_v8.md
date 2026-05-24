# 安卓 APP 技术方案 v8

## 1. 背景

本文针对 [安卓APP_问题_v7.md](./安卓APP_问题_v7.md) 设计下一版 Android 原生 APP 改造方案。

v7 已经补齐了商品图片、商品详情、聊天商品卡片、流式 Markdown、购物车确认等主流程。但当前仍有三个明显问题：

- 从历史会话返回聊天页时，商品卡片和追问提示会丢失。
- Android 聊天渲染没有完整迁移 Web 端的 `text + blocks + followups` 结构，也没有处理 `<tool_call>` 等模型内部标签。
- 商品页筛选器过于扁平，详情页返回位置丢失，长内容栏目缺少“显示更多”。

v8 的目标是把聊天记录从“只保存文字”升级为“保存完整回合”，并把商品页筛选和详情体验做成接近电商 APP 的层级交互。

## 2. 设计结论

v8 按以下方向实现：

```text
聊天
  本地保存完整 AgentTurn
  历史恢复 text + blocks + followups
  Android 端补齐 Web 端 block 渲染
  tool_call 等内部标签不直接裸露

商品页
  顶部只展示一级筛选入口：商品类别
  点击后弹出分类选择窗口
  返回详情前保留列表滚动位置和筛选条件

商品详情
  长栏目默认折叠
  点击“显示更多”展开完整内容
```

实现方式仍保持单 Activity + Java 原生 View，不引入 Compose。

## 3. 聊天历史完整恢复

### 3.1 当前问题

当前 `LocalChatStore.saveMessage(...)` 每次只保存：

```text
role
content
status
blocks_json = []
```

因此历史恢复时只能调用：

```java
addBubble(message.content, ...)
```

这会导致以下内容消失：

- `block_delta` 商品卡片
- `comparison_table`
- `citation`
- `cart_state`
- `order_summary`
- `followups` 追问提示
- 未来可能新增的结构化块

Web 端不是这样做的。Web 端 `AgentTurn` 的结构是：

```ts
{
  userContent,
  text,
  blocks,
  followups,
  statusText
}
```

Android v8 要迁移这个结构。

### 3.2 新本地数据模型

`messages` 表继续保留，但 assistant 消息必须保存完整结构：

```text
content       assistant 文本
blocks_json   AgentBlock 数组 JSON
followups_json 追问数组 JSON
status        completed / failed / canceled / local_only
```

因为旧表没有 `followups_json`，升级数据库版本：

```java
super(context, "xzxg_chat.db", null, 2);
```

迁移规则：

```sql
ALTER TABLE messages ADD COLUMN followups_json TEXT NOT NULL DEFAULT '[]';
```

不要 drop 旧表，否则用户已有历史会话会丢。

### 3.3 Android 内存态

新增当前流式回合缓存：

```java
private final JSONArray activeAssistantBlocks = new JSONArray();
private JSONArray activeFollowups = new JSONArray();
```

事件处理规则：

```text
text_delta
  -> 追加 activeAssistantMarkdown
  -> 实时渲染 Markdown

block_delta
  -> activeAssistantBlocks.put(block)
  -> 立即渲染对应 block

followups
  -> activeFollowups = questions
  -> 立即渲染追问 chips

message_end
  -> saveAssistantTurn(text, blocks, followups, completed)
```

### 3.4 历史恢复规则

`LocalChatStore.MessageItem` 增加：

```java
String blocksJson;
String followupsJson;
```

恢复历史时：

```text
user
  -> addBubble(content, true)

assistant
  -> addBubble(content, false)
  -> renderBlocks(JSONArray blocks_json)
  -> renderFollowups(JSONArray followups_json)

system
  -> addChatSystemLine(content)
```

这样从侧栏历史回到聊天页时，商品卡片和“您还可以继续追问...”不会消失。

## 4. 迁移 Web 端聊天渲染能力

### 4.1 Web 端已有能力

Web 端 `ChatMessageList.tsx` 已支持：

- `markdown`
- `product_card`
- `citation`
- `comparison_table`
- `cart_state`
- `order_summary`
- `warning`
- `followups`

Android v8 要一一对应，不再只处理部分 block。

### 4.2 Android block 渲染清单

新增统一入口：

```java
private void renderAgentBlock(JSONObject block, LinearLayout parent)
private void renderAgentBlocks(JSONArray blocks, LinearLayout parent)
```

支持规则：

```text
markdown
  -> MarkdownRenderer 渲染 content

product_card
  -> 商品卡片：图片、名称、价格、详情、加购

citation
  -> 可展开引用卡：标题 + snippet + source

comparison_table
  -> 手机端纵向对比卡，不做横向大表格

cart_state
  -> 购物车摘要：商品名、数量、价格、应付金额

order_summary
  -> 订单摘要：商家、金额、状态

warning
  -> 浅黄色提示卡
```

### 4.3 followups 渲染

Android 不应再把 followups 简单拼成一行系统文字：

```text
你还可以继续追问：[...]
```

v8 改成和 Web 一致的 chips：

```text
您还可以继续追问：
[预算范围大概是多少？] [主要用途是什么？] [品牌有偏好吗？]
```

点击 chip：

```java
sendMessage(question)
```

保存历史时 `followups_json` 一起保存。

## 5. tool_call 等内部标签处理

### 5.1 当前问题

用户看到类似：

```xml
<tool_call>
...
</tool_call>
```

这类标签不是面向用户的回答内容，直接裸露会破坏聊天体验。

### 5.2 渲染原则

Android 端不直接展示裸露的工具调用标签。

处理优先级：

```text
1. 如果标签能解析为工具调用块：渲染为可折叠“工具调用”调试卡
2. 如果只是模型内部标签：从用户可见 Markdown 中移除
3. 如果标签不闭合：流式阶段先隐藏，等待后续 delta；最终仍不展示原始标签
```

### 5.3 默认行为

正式聊天默认隐藏工具调用细节，只显示自然语言结果。

测试版可以保留可折叠调试卡：

```text
工具调用
  工具名
  参数摘要
  状态
```

正式版不展示调试卡。

### 5.4 实现方案

新增：

```java
private static class RenderedMarkdown {
    String visibleMarkdown;
    JSONArray toolCalls;
}

private RenderedMarkdown sanitizeAgentMarkdown(String raw)
```

处理范围：

- `<tool_call>...</tool_call>`
- `<tool_response>...</tool_response>`
- `<think>...</think>`，如果后端模型意外输出
- 未来可扩展到其他内部标签

`appendAssistant(delta)` 中不直接把 raw markdown 送给 Markwon，而是：

```java
RenderedMarkdown rendered = sanitizeAgentMarkdown(activeAssistantMarkdown.toString());
MarkdownRenderer.setMarkdown(activeAssistant, rendered.visibleMarkdown);
```

保存本地时保存两份：

```text
content = visibleMarkdown
raw_content 可选，v8 暂不新增，避免扩大数据库迁移
```

如果后续要做调试回放，再新增 `raw_content`。

## 6. 商品筛选器重设计

### 6.1 当前问题

当前分类筛选是横向 chips：

```text
全部 手机 鼠标 洁面 ...
```

问题：

- 分类多时横向拥挤。
- 一级/二级关系不清楚。
- 不像电商 APP 的筛选交互。

### 6.2 新结构

商品页顶部只保留搜索和一级筛选入口：

```text
搜索商品                         搜索

[商品类别  全部 >]
```

一级筛选入口只有一个：

```text
商品类别
```

点击后打开分类选择窗口。

### 6.3 分类选择窗口

采用底部弹窗或全屏弹窗。考虑当前 Java 原生 View 实现，优先底部弹窗：

```text
选择商品类别

左侧：一级分类
  全部
  数码电子
  美妆护肤
  食品生活

右侧：二级分类
  全部
  手机
  鼠标
  洁面

底部：
  重置    确定
```

交互规则：

- 点击一级分类，右侧刷新对应二级分类。
- 点击二级分类，只改变临时选择，不立刻刷新商品列表。
- 点击“确定”后关闭弹窗并刷新商品列表。
- 点击“重置”选择全部。
- 取消弹窗不改变当前筛选条件。

### 6.4 状态字段

新增：

```java
private JSONArray cachedCategoryTree = new JSONArray();
private String selectedCategoryId = "";
private String selectedCategoryName = "全部";
private String pendingCategoryId = "";
private String pendingCategoryName = "全部";
```

商品页筛选按钮文案：

```text
商品类别：全部
商品类别：手机
```

## 7. 商品详情返回位置保持

### 7.1 当前问题

从商品列表中间进入详情，再按返回，会重新 `renderProducts()`，列表回到顶部。

### 7.2 新规则

进入详情前保存商品页状态：

```java
private int productsScrollY;
private String lastProductKeyword;
private String lastCategoryId;
private String lastCategoryName;
```

点击详情前：

```java
productsScrollY = productsScroll.getScrollY();
```

返回时：

```java
renderProducts();
productsScroll.post(() -> productsScroll.scrollTo(0, productsScrollY));
```

如果商品列表数据重新加载，滚动恢复必须在 `renderProductItems(...)` 完成后执行。

### 7.3 实现细节

`pageBody()` 当前只返回 `LinearLayout page`，拿不到外层 `ScrollView`。

v8 需要为商品页单独持有：

```java
private ScrollView productsScroll;
private LinearLayout productsList;
```

或者新增：

```java
private static class PageBody {
    ScrollView scroll;
    LinearLayout content;
}
```

推荐先用商品页专属字段，改动更小。

## 8. 商品详情长内容“显示更多”

### 8.1 当前问题

详情页中的栏目，例如“推荐理由”，内容过长时会撑得很长，影响浏览。

### 8.2 新规则

以下栏目默认折叠：

- 推荐理由
- 商品卖点
- 适合人群
- 不适合人群
- 风险提示
- 商品参数，如果条目超过 6 条
- 商品描述，如果后端后续返回

默认展示：

```text
文本：最多 3 行
列表：最多 3 条
参数：最多 6 条
```

如果超过限制，显示无边框按钮：

```text
显示更多
```

点击后：

```text
展示完整内容
按钮变成：收起
```

### 8.3 UI 规则

“显示更多”是无边框文字按钮：

```text
颜色：深灰或主题蓝
字号：14sp
背景：透明
高度：36dp
```

不要使用大黑按钮，避免喧宾夺主。

### 8.4 实现方案

新增 helper：

```java
private void addExpandableTextPanel(LinearLayout page, String title, String text, int collapsedLines)
private void addExpandableArrayPanel(LinearLayout page, String title, JSONArray items, int collapsedCount)
private void addExpandableAttributesPanel(LinearLayout page, JSONArray items, int collapsedCount)
```

内部维护展开状态：

```java
final boolean[] expanded = {false};
```

点击按钮后重新填充当前 panel。

## 9. 需要修改的文件

### 9.1 LocalChatStore.java

- 数据库版本升级到 2。
- `onUpgrade` 不再直接 drop 表。
- `messages` 增加 `followups_json`。
- 新增 `saveAssistantTurn(localSessionId, content, blocksJson, followupsJson, status)`。
- `MessageItem` 增加 `blocksJson`、`followupsJson`。

### 9.2 MainActivity.java

- 保存 `activeAssistantBlocks` 和 `activeFollowups`。
- 历史恢复时按 `content + blocks + followups` 渲染。
- 迁移 Web 端所有 block 类型。
- 增加 `sanitizeAgentMarkdown(...)`。
- followups 改为 chips。
- 商品筛选改为“商品类别”入口 + 底部分类选择窗口。
- 商品详情返回恢复滚动位置。
- 商品详情长栏目改为可展开 panel。

### 9.3 ApiClient.java

原则上 v7 已有分类和商品详情接口。v8 只需要保证分类树缓存使用：

```java
categoriesTree()
```

如果已有接口稳定，不需要改。

## 10. 验收标准

### 10.1 聊天历史

- 发送一轮带商品推荐的聊天后，侧栏进入其他会话，再返回该会话，商品卡片仍存在。
- 追问 chips 仍存在，并且点击后可以继续发送。
- `citation`、`comparison_table`、`cart_state`、`order_summary` 历史恢复后仍能展示。

### 10.2 特殊标签

- 用户不可见区域不再裸露 `<tool_call>` / `</tool_call>`。
- 如果模型输出 `<think>`，用户也不会看到原始标签。
- 普通 Markdown 仍正常渲染。

### 10.3 商品筛选

- 商品页顶部只有“商品类别”一级筛选入口。
- 点击后出现分类选择窗口。
- 可以选择一级/二级分类。
- 点击确定后商品列表按分类刷新。
- 点击取消不改变当前分类。

### 10.4 商品详情返回

- 在商品列表中滚动到中部，点击商品详情。
- 按返回键后回到商品列表原来的滚动位置。
- 搜索词和分类筛选条件不丢失。

### 10.5 商品详情长内容

- 推荐理由等长内容默认折叠。
- 超长栏目显示“显示更多”无边框按钮。
- 点击后展开完整内容，再点击“收起”可折叠。

## 11. 测试计划

### 11.1 构建

```bash
gradle --no-daemon -Dorg.gradle.vfs.watch=false assembleDebug
```

仍使用 `/private/tmp/xzxg-android-build` 作为构建目录。

### 11.2 模拟器验证

必须用模拟器验证：

- 商品推荐聊天，历史恢复后商品卡和追问仍存在。
- 构造包含 `<tool_call>...</tool_call>` 的文本，确认不会裸露。
- 商品页分类弹窗。
- 商品详情返回滚动位置。
- 商品详情长内容显示更多。
- logcat 无 `AndroidRuntime` / `FATAL EXCEPTION`。

### 11.3 回归

保留 v7 已验证能力：

- 商品图片展示。
- 商品详情展示。
- 购物车下单确认。
- 流式 Markdown 每个 delta 立即渲染。
- 一级页返回键打开侧栏。

## 12. 实施顺序

1. 升级 `LocalChatStore`，保证历史数据不丢。
2. 改聊天事件缓存与保存逻辑：text、blocks、followups 一起保存。
3. 补齐 Android block 渲染和 followups chips。
4. 增加 Markdown 内部标签净化。
5. 重做商品筛选为底部分类选择窗口。
6. 保存和恢复商品页滚动位置。
7. 商品详情长内容折叠/展开。
8. 构建并模拟器回归。
