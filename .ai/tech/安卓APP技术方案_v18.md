# 安卓 APP 技术方案 v18

## 1. 背景

本文针对 [安卓APP_问题_v17.md](./安卓APP_问题_v17.md) 设计下一版 Android 原生 APP 修复方案。

本轮问题覆盖四类能力：

- 聊天页：商品卡片要按 Agent 输出协议出现在回答中的对应位置；从历史记录进入时不能丢失商品推荐；表格间距和横向滚动体验需要优化。
- 侧栏：长按历史会话后支持置顶、更改标题、删除会话。
- 登录态：后端 token 有过期时间，APP 需要在 token 失效时自动退出登录状态。
- 应用图标：新增符合“小猪小狗导购”主题的 launcher icon。

## 2. 现状判断

### 2.1 Agent 输出协议

[Agent输出协议_v1.md](../api/Agent输出协议_v1.md) 明确要求：

- 主回答流式正文走 `text_delta`。
- 可展示为卡片的数据走 `block_delta`。
- `product_card` 是完整商品卡。
- `product_refs` 只给商品 ID，前端按 `product_ids` 请求 `GET /products/{product_id}` 后渲染商品卡。
- 文本内 `<item>{productId}</item>` 只能作为轻量挂品位置提示，推荐优先使用 `product_refs`。
- `comparison_table` 用结构化 columns/rows 渲染对比表。

当前 Android 端已经有 `renderAgentBlock()`，能处理 `product_card`、`product_refs`、`comparison_table` 等 block，但存在两个结构性问题：

1. `text_delta` 被合并成一个 `TextView` 气泡，`block_delta` 到达时只能追加到 `chatList` 末尾，无法插入到回答正文中间。
2. `renderStoredMessagesIfAny()` 恢复历史时先渲染 assistant 文本气泡，再整体渲染 `blocks_json`，因此历史消息里的商品卡仍然只能出现在回答后面。

### 2.2 历史会话恢复

本地 SQLite `messages` 表已有：

```text
content
blocks_json
followups_json
```

因此本地已保存的 assistant turn 可以恢复商品卡和追问。

但从远程历史进入时，`loadRemoteSessionDetail()` 当前只读取远程 `messages[].content`，并用空 blocks 保存：

```text
blocks_json = []
followups_json = []
```

这会导致“从历史记录进入的时候，商品推荐没有了”。如果远程接口没有返回 assistant run 的最终内容和 blocks，Android 端无法仅凭用户消息重建商品卡。

### 2.3 Token 有效期

后端实现里 `CreateAuthToken()` 写入：

```go
expires_at = time.Now().Add(24*time.Hour)
```

`GetAccountByToken()` 查询条件包含：

```sql
WHERE t.token = ? AND t.expires_at > ? AND a.status = 'active'
```

因此登录 token 有 24 小时有效期。过期后所有登录接口会返回 401，APP 需要清理本地登录态并回到未登录状态。

### 2.4 侧栏历史会话接口

当前后端已有：

- `GET /agent/sessions`
- `GET /agent/sessions/search`
- `GET /agent/sessions/{session_id}`
- `PATCH /agent/sessions/{session_id}`：可更新 `title` / `summary`
- `POST /agent/sessions/{session_id}:summarize`

尚缺：

- 删除远程会话接口。
- 远程会话置顶字段与接口。

Android 本地 `sessions` 表也尚缺 `pinned_at` / `deleted_at` 这类字段。

## 3. 总体设计

```text
聊天回答:
  将一条 assistant turn 拆成有序片段
  text_delta 形成 text segment
  block_delta 形成 block segment
  渲染时按 segment 顺序展示

历史恢复:
  本地保存 segment 顺序
  远程会话详情返回 assistant runs 的 content + blocks + followups
  Android 恢复时仍按 segment 顺序渲染

表格:
  comparison_table 使用横向 ScrollView
  markdown 表格使用独立横向容器或更宽 TextView
  表格上下增加间距

侧栏历史:
  长按历史项弹出操作菜单
  置顶/取消置顶
  更改标题
  删除会话
  本地即时生效，远程登录态同步

登录态:
  启动时调用 /auth/me 校验 token
  任意接口 401 统一触发登录过期处理
  清理 token、账号资料和需要登录的缓存 UI

应用图标:
  使用 adaptive icon
  前景为小猪小狗导购主题图形
  补齐 mipmap-anydpi-v26 与 fallback png/vector
```

## 4. 聊天页结构化渲染方案

### 4.1 引入有序渲染片段

新增内存结构：

```java
class ChatRenderSegment {
    String type;       // text / block
    String text;       // type=text
    JSONObject block;  // type=block
}
```

一条 assistant turn 保存为有序 `segments_json`：

```json
[
  {"type":"text","text":"我建议先看这款："},
  {"type":"block","block":{"type":"product_refs","product_ids":["p_001"]}},
  {"type":"text","text":"如果你更重视续航，可以再对比下面这款。"},
  {"type":"block","block":{"type":"product_refs","product_ids":["p_002"]}}
]
```

迁移 `LocalChatStore`：

- 数据库版本从 `2` 升到 `3`。
- `messages` 表新增 `segments_json TEXT NOT NULL DEFAULT '[]'`。
- 旧数据兼容规则：
  - user 消息：`segments_json` 为空，按 content 渲染。
  - assistant 旧消息：如果 `segments_json=[]`，按旧逻辑渲染 content 后再渲染 blocks。
  - 新 assistant 消息：优先按 `segments_json` 渲染。

新增方法：

```java
saveAssistantTurn(localSessionId, content, blocksJson, followupsJson, segmentsJson, status)
saveRemoteMessageSnapshot(..., segmentsJson, ...)
```

### 4.2 流式过程中的 segment 构建

当前 `appendAssistant()` 会持续更新同一个 `TextView`。v18 保留这个行为，但把每段连续文本作为一个 segment：

- 收到第一个 `text_delta`：
  - 如果没有当前文本段，创建一个 assistant 文本气泡。
  - 新建 `currentTextSegment`。
- 连续收到 `text_delta`：
  - 追加到当前文本气泡。
  - 追加到 `currentTextSegment.text`。
- 收到 `block_delta`：
  - 先 flush 当前文本段到 `activeAssistantSegments`。
  - 渲染 block。
  - 将 block segment 追加到 `activeAssistantSegments`。
  - 清空 `activeAssistant`，下一段 text_delta 会新建新的文本气泡。

关键点：

```text
text_delta A
block_delta product_refs
text_delta B

渲染顺序:
  文本气泡 A
  商品卡
  文本气泡 B
```

不再把整轮回答强制放在一个气泡里。

### 4.3 `<item>` 标签按位置渲染

协议允许 `<item>p_001</item>` 作为文本内挂品位置提示。v18 需要把它也纳入 segment 化处理：

1. `appendAssistant()` 对新增 delta 后的当前文本段做增量扫描。
2. 当发现完整 `<item>{productId}</item>`：
   - 标签前文本保留在当前文本气泡。
   - flush 文本 segment。
   - 生成一个 block segment：

```json
{"type":"product_refs","product_ids":["p_001"],"source":"item_tag"}
```

   - 标签本身不展示给用户。
   - 标签后的文本进入新的文本气泡。

3. 如果同一商品 ID 后续又通过 `product_refs` 出现：
   - 使用 `activeRenderedProductIds` 去重。
   - 保留第一次出现的位置。

### 4.4 `product_refs` 的位置语义

`product_refs` block 到达时直接在当前位置渲染，不再延迟到回答末尾。

注意：

- `product_refs.product_ids` 里的多个商品按数组顺序渲染。
- 每个商品详情请求异步完成，插入位置必须稳定。
- 做法：先在当前位置插入一个占位容器 `LinearLayout productRefsContainer`，异步详情返回后往该容器里填卡片，而不是直接 `parent.addView()` 到末尾。

```text
chatList:
  text A
  productRefsContainer
  text B

异步加载完成:
  productRefsContainer.addView(productCard)
```

### 4.5 历史消息渲染

`renderStoredMessagesIfAny()` 改为：

1. user 消息：仍按普通右侧气泡渲染。
2. assistant 消息：
   - 如果 `segments_json` 非空：按 segment 顺序渲染。
   - 如果 `segments_json` 为空：按旧逻辑兼容渲染 `content + blocks_json`。
3. followups 仍在该 assistant turn 最后渲染。

这能解决：

- 新消息商品卡出现在回答对应位置。
- 本地历史进入后商品卡仍在原位置。

## 5. 远程历史商品推荐恢复方案

### 5.1 后端补齐会话详情

当前 `GET /agent/sessions/{session_id}` 返回用户消息以及 runs，但 run 里缺少足够的最终回答结构化信息。v18 后端需要补齐：

```json
{
  "messages": [
    {
      "role": "user",
      "content": "推荐一款拍照手机",
      "created_at": "..."
    },
    {
      "role": "assistant",
      "content": "这款更适合你。",
      "blocks": [
        {"type":"product_refs","product_ids":["p_digital_001"]}
      ],
      "followups": ["再便宜一点", "对比续航"],
      "segments": [
        {"type":"text","text":"这款更适合你。"},
        {"type":"block","block":{"type":"product_refs","product_ids":["p_digital_001"]}}
      ],
      "created_at": "..."
    }
  ]
}
```

实现路径：

- 如果后端已有 run content / trace blocks 存储，直接在详情接口组装 assistant message。
- 如果后端没有持久化 assistant blocks，则新增持久化：
  - `agent_runs.content`：最终可见 markdown。
  - `agent_runs.blocks_json`：最终 blocks。
  - `agent_runs.followups_json`：最终 followups。
  - `agent_runs.segments_json`：最终有序 segments。
- `streamAgentRun()` 在 `message_end` 前把最终内容、blocks、followups、segments 写入 run。
- `GetSessionDetail()` 返回用户消息时，同时展开该 user message 下 completed/canceled run 的 assistant 消息。

### 5.2 Android 远程恢复逻辑

`loadRemoteSessionDetail()` 改为识别 `role`：

- `role=user`：保存用户消息。
- `role=assistant`：保存 content、blocks、followups、segments。
- 如果后端返回旧格式：从 `message.runs[]` 中取最新 completed run 组装 assistant 快照。

保存时必须保留：

```text
content
blocks_json
followups_json
segments_json
created_at
```

加载完成后刷新当前聊天页，按第 4 节渲染。

## 6. 表格渲染方案

### 6.1 comparison_table

`comparisonCard()` 改为横向滚动结构：

```text
外层 card:
  padding top/bottom 增大
  标题与表格间距 12dp
  HorizontalScrollView:
    tableContainer minWidth = screenWidth * 1.15
```

样式要求：

- 表格整体宽度比当前略宽，最小宽度不小于屏幕宽度的 `1.15` 倍。
- 每列最小宽度 `120dp`，商品名列 `150dp`。
- 单元格 padding 增加到 `10dp horizontal / 9dp vertical`。
- 行间分割线使用浅灰色。
- 表格上下与正文之间至少 `12dp`。

### 6.2 Markdown 表格

Markwon 的 table 插件默认在 TextView 内排版，横向滚动能力弱。v18 使用预处理策略：

1. 在 `MarkdownRenderer` 增加 `containsMarkdownTable(markdown)`。
2. `addBubbleTo()` 渲染 assistant markdown 时，如果包含表格：
   - 使用 `HorizontalScrollView` 包裹 TextView。
   - TextView 设置 `minWidth = screenWidth * 1.15`。
   - 气泡宽度允许达到 `0.92 * screenWidth`。
3. 表格前后自动补空行，避免“字和表格贴太近”。

预处理规则：

```text
普通段落
| A | B |
|---|---|
| 1 | 2 |

转换为:
普通段落

| A | B |
|---|---|
| 1 | 2 |

```

## 7. 侧栏历史长按操作

### 7.1 本地模型

`sessions` 表新增：

```text
pinned_at INTEGER DEFAULT 0
deleted_at INTEGER DEFAULT 0
```

排序规则：

```sql
WHERE deleted_at = 0
ORDER BY
  CASE WHEN pinned_at > 0 THEN 0 ELSE 1 END,
  pinned_at DESC,
  updated_at DESC
```

本地新增方法：

```java
pinSession(localSessionId, boolean pinned)
renameSession(localSessionId, title)
deleteSession(localSessionId)
```

删除采用软删除：

- `deleted_at = now`
- 不立即删 messages，避免误删和同步冲突。
- 如果删除的是当前会话，删除后创建新会话并回到聊天首页。

### 7.2 长按菜单

`historyButton()` 增加：

```java
row.setOnLongClickListener(v -> {
    showHistoryActionSheet(item);
    return true;
});
```

菜单项：

- 未置顶：`置顶会话`
- 已置顶：`取消置顶`
- `更改会话标题`
- `删除会话`

交互：

- 置顶：立即更新本地列表并刷新侧栏。
- 更改标题：弹窗输入框，默认当前标题；保存后刷新侧栏。
- 删除：二次确认；确认后软删除并刷新侧栏。

### 7.3 远程接口补齐

登录态下同步到后端：

```http
PATCH /api/v1/agent/sessions/{session_id}
{
  "title": "新标题"
}
```

新增后端接口：

```http
POST /api/v1/agent/sessions/{session_id}:pin
{
  "pinned": true
}
```

```http
DELETE /api/v1/agent/sessions/{session_id}
```

后端数据表 `chat_sessions` 新增：

```sql
pinned_at DATETIME NULL
deleted_at DATETIME NULL
```

列表和搜索接口过滤：

```sql
deleted_at IS NULL
```

排序：

```sql
ORDER BY
  CASE WHEN pinned_at IS NULL THEN 1 ELSE 0 END,
  pinned_at DESC,
  COALESCE(last_message_at, updated_at, created_at) DESC
```

如果远程同步失败：

- 本地操作仍保留。
- `sync_state` 标记为 `pending_sync`。
- 下次打开侧栏或启动 APP 时重试。

## 8. Token 失效自动退出

### 8.1 启动校验

`MainActivity.onCreate()` 初始化后，如果本地 token 非空：

```text
GET /auth/me
```

结果：

- 200：刷新账号资料，继续已登录态。
- 401：调用 `handleAuthExpired()`。
- 网络错误：不立刻退出，保留登录态并提示弱错误，避免离线时误退出。

### 8.2 统一 401 处理

`ApiClient.readJSON()` 在非 2xx 时解析 HTTP code。新增自定义异常：

```java
class ApiException extends RuntimeException {
    int statusCode;
    String code;
    String message;
}
```

所有后台调用捕获到 `statusCode == 401` 时统一：

```java
runOnUiThread(() -> handleAuthExpired());
```

`handleAuthExpired()`：

```text
1. 取消正在进行的流式请求。
2. sessionStore.clearAuth()
3. 清理购物车 badge / 登录用户资料缓存。
4. 如果当前页面依赖登录态：
   - 设置、账号管理、购物车、订单、优惠券：跳转登录页或未登录提示页。
   - 聊天页：保留本地聊天，但提示“登录已过期，请重新登录”。
5. toast：登录已过期，请重新登录。
```

### 8.3 流式接口 401

`streamMessage()` 当前在非 2xx 时只把 error body 包成 RuntimeException。v18 改为：

- 如果 `conn.getResponseCode() == 401`，回调 `onAuthExpired()` 或抛 `ApiException(401, ...)`。
- 不把 401 文案保存成 assistant 消息。
- 用户消息仍保存在本地，重新登录后可继续发送。

## 9. 应用图标方案

### 9.1 设计方向

主题：“小猪小狗导购”。

视觉元素：

- 圆形浅色底。
- 左侧小猪鼻子/耳朵，右侧小狗耳朵/鼻尖，用简化线面图形。
- 中间放一个小购物袋或放大镜，表达导购/购物。
- 主色建议：
  - 猪粉：`#FFB8C8`
  - 小狗暖黄：`#F6C56B`
  - 导购蓝绿：`#2DD4BF`
  - 深色描边：`#111827`

### 9.2 Android 资源

新增资源：

```text
res/drawable/ic_launcher_foreground.xml
res/drawable/ic_launcher_background.xml
res/mipmap-anydpi-v26/ic_launcher.xml
res/mipmap-anydpi-v26/ic_launcher_round.xml
```

`AndroidManifest.xml` 更新：

```xml
android:icon="@mipmap/ic_launcher"
android:roundIcon="@mipmap/ic_launcher_round"
```

兼容低版本：

- 如果只用 vector foreground，minSdk 24 可用。
- 如需 PNG fallback，可用 `mipmap-mdpi` 到 `mipmap-xxxhdpi` 生成静态图标。

## 10. 实施顺序

1. 数据层迁移：Android `segments_json/pinned_at/deleted_at`，后端 `pinned_at/deleted_at` 和 run 最终结果字段。
2. 后端接口：会话详情返回 assistant blocks/segments，新增 pin/delete，会话列表排序过滤。
3. Android 聊天渲染：segment 构建、流式插入、历史恢复、product_refs 占位容器。
4. 表格渲染：comparison_table 横向滚动，Markdown 表格容器和间距。
5. 侧栏长按菜单：置顶、改名、删除，本地即时反馈和远程同步。
6. Token 过期处理：启动 `/auth/me` 校验、统一 401、流式 401。
7. 应用图标资源和 Manifest 配置。
8. 模拟器回归测试。

## 11. 验收清单

### 聊天页

- Agent 输出 `text_delta -> product_refs -> text_delta` 时，商品卡显示在两段文字中间。
- Agent 输出 `product_card` 时，商品卡显示在收到 block 的当前位置。
- Agent 文本包含 `<item>p_xxx</item>` 时，标签不显示，商品卡出现在标签位置。
- 同一商品通过 `<item>` 和 `product_refs` 重复出现时只渲染一次。
- 从本地历史进入，会话中的商品推荐仍存在，且顺序不变。
- 从远程历史进入，会话中的商品推荐仍存在，且顺序不变。
- 对比表上下间距明显增大。
- 对比表宽度更宽，可以横向拖动。

### 侧栏

- 长按历史会话弹出菜单。
- 置顶后该会话固定在历史列表顶部。
- 取消置顶后恢复按更新时间排序。
- 更改标题后侧栏立即显示新标题，重新打开 APP 后仍保留。
- 删除会话后侧栏不再显示；删除当前会话时回到新聊天。
- 登录态下上述操作能同步远程；断网失败不影响本地操作。

### 登录态

- token 未过期时启动 APP 保持登录。
- token 过期后启动 APP 自动退出登录态。
- token 过期后调用购物车/订单/设置等接口，APP 自动退出并提示重新登录。
- 流式发送遇到 401 时不产生错误 assistant 消息。

### 应用图标

- 桌面图标显示为“小猪小狗导购”主题。
- 圆形图标和普通图标都正常。
- debug/release 安装包均能显示新图标。

## 12. 风险与边界

- 如果后端不持久化 assistant blocks/segments，远程历史无法完整恢复商品推荐；必须后端配合补齐。
- `product_refs` 异步加载商品详情时要使用占位容器，否则商品卡可能因网络返回顺序错位。
- 表格横向滚动与聊天页纵向滚动会有手势竞争，需要 `HorizontalScrollView` 只消费横向滑动。
- 删除会话建议先软删除，避免误删后无法恢复，也便于处理离线同步失败。
- 启动 token 校验遇到网络错误不能直接退出，否则会造成离线误登出。
