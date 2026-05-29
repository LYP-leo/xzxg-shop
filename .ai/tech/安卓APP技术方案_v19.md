# 安卓 APP 技术方案 v19

## 1. 背景

本文针对 [安卓APP_问题_v18.md](./安卓APP_问题_v18.md) 设计下一版 Android 原生 APP 修复方案。

本轮问题集中在三处：

- 聊天页：
  - 商品卡片没有渲染出来。
  - 商品表格会把整个会话气泡撑宽。
  - 表格、公式等结构化片段在流式未完整时被提前渲染，导致源码态和渲染态反复切换、页面闪烁。
- 侧栏：
  - 置顶、更改标题、删除会话完成后不应回到聊天页。
  - 操作完成后侧栏必须展示最新列表。
- 应用图标：
  - launcher icon 改为 `.ai/tech/assets/pig.svg`，背景透明。

## 2. 现状判断

### 2.0 后端 API 文档约束

本方案以以下文档为准：

- [API接口文档_v3.md](../api/API接口文档_v3.md)
- [Agent输出协议_v1.md](../api/Agent输出协议_v1.md)

关键约束：

- Base URL 已包含 `/api/v1`，Android `ApiClient` 内部接口路径应使用相对路径，例如 `/products/{product_id}`，不要重复拼接 `/api/v1`。
- 商品列表、商品详情、类目、商家、评价、SKU 是公开接口。
- 购物车、订单、Agent 会话是用户接口，必须带 `Authorization: Bearer <token>`。
- SSE 主回答正文只来自 `text_delta`。
- 可点击、可交互、可展示成卡片的数据必须来自 `block_delta`。
- 前端不从自然语言中解析商品名、价格、库存；`<item>` 只是协议明确允许的商品 ID 挂载位置兼容标签。
- `product_refs` 的协议字段是 `product_ids`；不存在 `ids`、`items` 等别名。
- 商品响应字段是 camelCase，购物车加购请求字段是 snake_case。

### 2.1 聊天渲染闪烁的根因

当前 `appendAssistant()` 在每个 `text_delta` 到达时都会调用：

```java
MarkdownRenderer.setMarkdown(activeAssistant, rendered.visibleMarkdown);
```

这会导致：

1. Markdown 表格尚未流式输出完整时，前端已经尝试渲染，表格结构不稳定。
2. 公式、表格这类需要完整边界的内容，会从“源码态”到“渲染态”反复切换，视觉上出现闪烁。
3. `product_refs` 异步加载商品详情时，如果没有稳定占位容器，商品卡可能丢失或插入位置错误。

v19 需要改为：

```text
普通文本:
  继续实时 Markdown 渲染

敏感结构:
  表格、公式、代码块等需要完整边界的片段先缓冲
  片段边界确认后再一次性渲染该片段

流式结束:
  flush 仍在缓冲中的敏感结构
```

也就是说，不等待整条回答全部流式完成；只延迟渲染容易闪烁的局部结构。

### 2.2 表格撑宽气泡的根因

当前表格处理逻辑倾向于把整段 Markdown 气泡设置更宽或提高 `minWidth`。这会把“包含表格的整段 assistant 气泡”撑宽，违背“只有表格本身横向滚动”的要求。

v19 需要把 Markdown 内容拆成普通文本段和表格段：

- 普通文本：保持原气泡宽度。
- 表格段：在普通气泡内部单独放一个 `HorizontalScrollView`。
- 横向滚动只发生在表格容器，不改变整条会话气泡宽度。

### 2.3 侧栏刷新问题

当前置顶、重命名、删除操作里存在：

```java
closeDrawer();
showDrawer();
```

或者关闭弹窗后回到聊天页再重新打开侧栏才能看到最新结果。v19 需要把侧栏内容刷新改成原地刷新：

```text
操作弹窗关闭
  -> 本地数据库立即更新
  -> 远端接口后台同步
  -> drawerLayer 保持打开
  -> drawer 内容区域重新渲染
```

### 2.4 图标资源现状

当前 Manifest 已指向：

```xml
android:icon="@mipmap/ic_launcher"
android:roundIcon="@mipmap/ic_launcher_round"
```

v19 只需要替换 icon 资源来源：

- 前景图形使用 `.ai/tech/assets/pig.svg` 转换后的 vector 或 PNG。
- 背景使用透明色。
- adaptive icon 和低版本 fallback 都保持可用。

## 3. 总体设计

```text
聊天流式展示:
  普通文本 text_delta -> 实时 Markdown 渲染
  表格/公式/代码块 -> 缓冲到完整边界后局部渲染
  block_delta -> 按当前位置稳定插入占位并渲染

商品卡:
  block_delta product_refs/product_card 写入 segments
  product_refs 到达时按当前位置创建 holder
  product_refs 使用稳定占位容器异步填充商品卡

表格:
  Markdown 文本先拆分 table / non-table
  table 使用独立 HorizontalScrollView
  bubble 宽度不因 table 变宽

侧栏:
  抽屉保持打开
  操作后调用 refreshDrawerContent()
  列表读取最新 SQLite 状态

图标:
  pig.svg -> Android drawable
  adaptive icon 背景透明
  roundIcon 与 icon 使用同一透明背景小猪图形
```

## 4. 聊天页修复方案

### 4.1 新增流式片段解析器

为流式中的 assistant 回复新增片段解析状态：

```java
private TextView activeAssistantText;
private StringBuilder activeTextMarkdown;
private StringBuilder pendingSensitiveMarkdown;
private SensitiveKind pendingSensitiveKind; // NONE / TABLE / FORMULA / CODE
private JSONArray activeAssistantSegments;
private JSONArray activeAssistantBlocks;
```

核心原则：

- 普通文本仍使用 `MarkdownRenderer.setMarkdown()` 实时渲染。
- 表格、公式、代码块等敏感结构不在未闭合时交给 MarkdownRenderer。
- 敏感结构在边界完整后作为独立 segment 渲染，避免源码态和渲染态反复跳变。
- `<item>productId</item>` 是 Agent 协议明确允许的兼容挂品标签；只允许读取标签内商品 ID，不展示协议标签，不解析其它自然语言内容。

### 4.2 流式过程分段渲染

`text_delta`：

```text
逐字符或逐行追加到流式解析器

如果当前处于普通文本:
  追加到 activeTextMarkdown
  实时 MarkdownRenderer.setMarkdown(activeAssistantText, activeTextMarkdown)

如果检测到表格/公式/代码块开始:
  flush 普通文本 segment
  创建 pendingSensitiveMarkdown
  后续 delta 进入缓冲区，不实时富渲染

如果敏感结构边界完整:
  将 pendingSensitiveMarkdown 作为 table/formula/code segment 渲染
  清空 pendingSensitiveMarkdown
  后续内容回到普通文本实时渲染
```

`block_delta`：

```text
flush 当前普通文本 segment
如果存在完整敏感结构，也先 flush
保存 block segment
保存到 activeAssistantBlocks
在当前位置插入稳定 holder 并渲染 block
```

注意：如果后端仍会输出 `<item>{productId}</item>`，需要在流式记录阶段把它转成 Android 内部 segment：

```json
{"type":"block","block":{"type":"product_refs","product_ids":["p_001"]}}
```

该 `source`、`origin` 等调试信息不能写入后端协议 block；如需记录来源，只能放在 Android 内部状态中。展示文本中必须移除 `<item>` 标签。

### 4.3 敏感结构边界识别

需要识别并缓冲的结构：

- Markdown 表格：
  - 起始条件：连续两行中第二行匹配 `| --- | --- |` 这类分隔行。
  - 结束条件：下一行不再包含表格列分隔，或流式结束。
- LaTeX 块公式：
  - 起始条件：`$$` 或 `\[`。
  - 结束条件：匹配到对应 `$$` 或 `\]`。
- 行内公式：
  - 起始条件：单个 `$` 或 `\(`。
  - 结束条件：匹配到对应 `$` 或 `\)`。
  - 为避免普通价格符号误判，`$` 公式只在左右满足公式上下文时启用。
- 代码块：
  - 起始条件：三个反引号。
  - 结束条件：下一组三个反引号。

边界未完整时：

```text
不把 pendingSensitiveMarkdown 交给 MarkdownRenderer
可以在当前位置显示一个轻量占位或源码态临时文本
但不得反复替换已渲染内容
```

### 4.4 流式结束时 flush 剩余缓冲

`finishStream()` 和 `finishCanceledStream()` 不再重渲染整条回答，只做收尾：

1. 如果存在未闭合敏感结构：
   - 按最佳努力渲染为普通文本或对应结构。
   - 不影响前面已经实时渲染好的普通文本。
2. `flushActiveTextSegment()`。
3. 保存完整 `content / blocks_json / followups_json / segments_json`。
4. 渲染 followups。
5. 清理流式状态。

这样用户仍能看到正常文本实时出现，同时表格、公式等局部结构不会在未完整时闪烁。

### 4.5 商品卡片渲染方案

根据后端文档：

- [Agent输出协议_v1.md](../api/Agent输出协议_v1.md) 规定主链路优先输出 `product_refs`。
- `product_card` 保留给旧链路和兼容层。
- `product_refs.product_ids` 中是商品 ID，前端必须调用商品详情接口获取商品数据后再渲染卡片。
- 不允许从自然语言中解析商品名、价格、库存。

因此 Android 商品卡片渲染只支持协议定义的两类 block：

```json
{"type":"product_card","product":{...}}
{"type":"product_refs","product_ids":["p_001"]}
```

不再设计 `ids`、`items` 这类非协议字段兼容，避免掩盖后端格式错误。

#### 4.5.1 字段格式

商品卡字段以后端 `ProductCard` / `ProductDetail` JSON 为准，均为 camelCase：

```json
{
  "productId": "p_001",
  "skuId": "sku_001",
  "merchantId": "m_001",
  "merchantName": "小猪数码旗舰店",
  "name": "X Phone 12",
  "brand": "X",
  "categoryId": "c_phone",
  "imageUrl": "/api/v1/assets/...",
  "price": "2999.00",
  "marketPrice": "3299.00",
  "stockStatus": "in_stock",
  "tags": ["拍照", "预算内"],
  "sellingPoints": ["高速对焦"],
  "recommendReason": "抓拍和对焦能力适合日常拍照。",
  "riskNotes": []
}
```

在 Base URL 为 `/api/v1` 的前提下，`GET /products/{product_id}` 返回 `ProductDetail`，在上述字段基础上增加：

```json
{
  "imageUrls": [],
  "stockQuantity": 10,
  "attributes": [],
  "suitableFor": [],
  "notSuitableFor": [],
  "description": ""
}
```

聊天商品卡只使用 `ProductCard` 基础字段；详情页再使用 `ProductDetail` 扩展字段。

图片地址处理：

- `imageUrl` / `imageUrls` 可能是完整 URL，也可能是后端返回的站内路径，例如 `/api/v1/assets/ecommerce_agent_dataset/...`。
- 如果是站内路径，Android 只能拼接协议、主机和端口，不能再额外拼一次 `/api/v1`。
- 示例：Base URL 为 `http://10.0.2.2:8080/api/v1`，`imageUrl=/api/v1/assets/a.jpg` 时，最终图片地址应为 `http://10.0.2.2:8080/api/v1/assets/a.jpg`，不是 `http://10.0.2.2:8080/api/v1/api/v1/assets/a.jpg`。

#### 4.5.2 `product_refs` 处理

收到：

```json
{"type":"product_refs","product_ids":["p_001","p_002"]}
```

处理顺序：

1. 在当前 block 位置立即插入 `LinearLayout holder`，保证异步请求返回后卡片不会跑到回答末尾。
2. 按 `product_ids` 数组顺序逐个调用商品详情接口：

```text
GET /products/{product_id}
```

3. 每个商品详情返回后，将卡片插入对应的子 holder，保持原数组顺序。
4. 如果某个商品详情失败，仅在该商品位置展示轻量错误行，例如“商品信息加载失败”，不能影响其他商品卡。
5. 去重范围限定为当前 assistant turn。同一轮里 `product_refs` 和 `<item>` 指向同一商品时只展示一次；不同会话、不同轮次不共享去重状态。

#### 4.5.3 `product_card` 兼容处理

收到：

```json
{"type":"product_card","product":{...}}
```

处理策略：

- 直接按 `product` 对象渲染卡片，不再二次请求详情。
- 如果字段缺失：
  - `productId` 为空：卡片仍可展示基础信息，但详情和加购按钮禁用。
  - `imageUrl` 为空：展示占位图。
  - `price` 为空：展示“价格待确认”。
  - `stockStatus != in_stock`：加购按钮禁用或展示“暂不可加购”。

#### 4.5.4 卡片 UI 字段映射

聊天商品卡采用紧凑结构：

```text
图片 imageUrl
名称 name
品牌与商家 brand · merchantName
价格 ¥price
划线价 marketPrice，可为空
标签 tags，最多 3 个
卖点 sellingPoints，最多 2-3 条
推荐理由 recommendReason
风险提示 riskNotes，有内容时折叠或弱提示
按钮：详情 / 加入购物车
```

按钮行为：

- 详情：使用 `productId` 进入商品详情页，详情页调用 `GET /products/{product_id}`。
- 加入购物车：
  - 请求接口是 `POST /cart/items`。
  - 请求字段必须是 snake_case：`product_id`、`sku_id`、`quantity`。
  - 如果 `skuId` 为空，先调用 `GET /products/{product_id}/skus`，选择可售 SKU 后再加购；没有可售 SKU 时禁用加购。

#### 4.5.5 `<item>` 标签处理

`<item>` 只作为文本内挂品位置提示：

```text
这款适合日常拍照。<item>p_001</item>
```

处理规则：

- 只读取标签内商品 ID。
- 隐藏标签本身。
- 生成等价的局部 `product_refs` segment：

```json
{"type":"product_refs","product_ids":["p_001"]}
```

- 如果同一轮同时有结构化 `product_refs`，以 `product_refs.product_ids` 为准；`<item>` 只补充挂载位置，不参与自然语言推断。

### 4.6 Agent block 渲染矩阵

`renderAgentBlock()` 必须按 [Agent输出协议_v1.md](../api/Agent输出协议_v1.md) 的 block 类型分发，不从自然语言推断结构化数据。

当前协议已实现或已定义的 block：

| block.type | 字段 | Android 处理 |
| --- | --- | --- |
| `markdown` | `content` | 作为非流式补充 Markdown 渲染。主回答仍以 `text_delta` 为准。 |
| `product_refs` | `product_ids` | 按商品 ID 调 `GET /products/{product_id}`，渲染商品卡。 |
| `product_card` | `product` | 旧链路兼容，直接渲染 `product` 对象。 |
| `comparison_table` | `columns`、`rows[].productId`、`rows[].values` | 渲染横向滚动对比表。 |
| `citation_refs` | `chunk_ids` | 只记录引用 ID；若没有引用详情接口，不强行渲染正文内容。 |
| `citation` | `citation.chunkId/title/snippet/source` | 折叠或弱展示 RAG 证据，不拼入主回答正文。 |
| `cart_state` | `cart.items`、`cart.summary.selectedCount/totalAmount/discountAmount/payAmount` | 更新购物车角标，可展示“去购物车/去结算”。 |
| `order_summary` | `orders[]`，字段为 `order_id/order_no/status/pay_amount/...` | 展示订单摘要，待支付订单显示支付入口。 |
| `warning` | `code`、`message` | 展示轻量提示，不视为崩溃。 |
| `discount_preview` | `discount.total_amount/discount_amount/pay_amount/lines` | 计划扩展；若后端输出则展示折扣明细。 |
| `coupon_list` | `coupons[]` | 计划扩展；若后端输出则展示优惠券列表。 |
| `review_summary` | `product_id/rating_avg/rating_count/highlights/risks` | 计划扩展；若后端输出则展示评价摘要。 |
| `navigation_action` | `action.label/route/params` | 计划扩展；若后端输出则渲染跳转按钮。 |

兜底要求：

- 未识别的 `block.type` 不能导致页面崩溃。
- 可以展示“暂不支持的内容类型”，也可以忽略并记录日志。
- 任何 block 都必须按 SSE 收到顺序进入 `segments_json`，历史恢复时仍按同一顺序渲染。

### 4.7 历史渲染与实时渲染共用同一入口

新增统一入口：

```java
private void renderAssistantTurn(
    LinearLayout parent,
    String content,
    String blocksJson,
    String followupsJson,
    String segmentsJson
)
```

实时结束、历史恢复、远程会话详情加载都走这个入口。

优先级：

1. `segments_json` 非空：按 segments 顺序渲染。
2. `segments_json` 为空但 `blocks_json` 非空：兼容旧数据，正文后追加 blocks。
3. 都为空：只渲染正文。

## 5. 表格横向滚动方案

### 5.1 Markdown 表格拆分

新增：

```java
private List<MarkdownPart> splitMarkdownParts(String markdown);
```

规则：

- 连续满足 Markdown 表格结构的行归为 `type=table`。
- 其他行归为 `type=text`。
- 表格前后保留合理间距。

示例：

```text
普通说明文字

| 商品 | 价格 | 优点 |
| --- | --- | --- |
| A | 2999 | 续航强 |

总结文字
```

渲染为：

```text
TextView 普通说明文字
HorizontalScrollView(table)
TextView 总结文字
```

### 5.2 表格容器宽度约束

表格 UI：

```text
Bubble 宽度:
  maxWidth = 屏幕宽度 * 0.78

HorizontalScrollView:
  width = MATCH_PARENT
  horizontalScrollBarEnabled = true

TableLayout:
  width = WRAP_CONTENT
  minWidth = bubble 内容宽度
```

禁止：

- 给整个 bubble 设置超大 `minWidth`。
- 让 `TextView` 自己承载完整 Markdown 表格并撑开父容器。

### 5.3 comparison_table 结构化表格

`comparisonCard()` 同样调整：

- 外层卡片/气泡宽度保持普通 assistant 最大宽度。
- 只有 rows/columns 表格区域放入 `HorizontalScrollView`。
- 标题、说明、总结不进入横向滚动区域。

## 6. 侧栏操作刷新方案

### 6.1 拆分侧栏渲染

把 `showDrawer()` 拆为：

```java
private void showDrawer();
private void renderDrawerContent();
private void refreshDrawerContent();
```

职责：

- `showDrawer()`：只负责创建并打开 `drawerLayer`。
- `renderDrawerContent()`：从 SQLite 查询最新历史和用户信息，绘制侧栏内容。
- `refreshDrawerContent()`：在 `drawerLayer` 已打开时清空内容区并调用 `renderDrawerContent()`。

### 6.2 操作完成后保持侧栏打开

置顶：

```text
chatStore.pinSession()
refreshDrawerContent()
后台 api.pinSession()
失败时 toast，并可回滚本地状态
```

重命名：

```text
保存按钮点击
  -> chatStore.renameSession()
  -> dialog dismiss
  -> refreshDrawerContent()
  -> 后台 api.updateSession()
```

删除：

```text
确认删除
  -> chatStore.deleteSession()
  -> 如果删除的是当前会话，currentSession 重置为新会话
  -> refreshDrawerContent()
  -> 后台 api.deleteSession()
```

关键要求：

- 不调用 `closeDrawer()`。
- 不回到聊天页。
- 列表中置顶状态、标题、删除结果必须立即体现。

### 6.3 搜索态刷新

如果侧栏历史搜索框有查询词：

- 置顶/重命名/删除后继续保留搜索词。
- 刷新结果应基于当前搜索词重新查询。
- 删除后如果结果为空，展示空状态，不自动清空搜索词。

## 7. 图标替换方案

### 7.1 资源转换

输入文件：

```text
/Volumes/shared/xzxg-shop/.ai/tech/assets/pig.svg
```

输出：

```text
android-native/app/src/main/res/drawable/ic_launcher_foreground.xml
android-native/app/src/main/res/drawable/ic_launcher_background.xml
android-native/app/src/main/res/mipmap-anydpi-v26/ic_launcher.xml
android-native/app/src/main/res/mipmap-anydpi-v26/ic_launcher_round.xml
android-native/app/src/main/res/mipmap/ic_launcher.xml
android-native/app/src/main/res/mipmap/ic_launcher_round.xml
```

实现要求：

- `ic_launcher_background.xml` 使用透明色。
- `ic_launcher_foreground.xml` 使用 pig.svg 转换后的 vector path。
- adaptive icon：

```xml
<adaptive-icon>
    <background android:drawable="@android:color/transparent" />
    <foreground android:drawable="@drawable/ic_launcher_foreground" />
</adaptive-icon>
```

- fallback `@mipmap/ic_launcher` / `@mipmap/ic_launcher_round` 仍可被旧系统加载。

### 7.2 透明背景校验

构建后在模拟器确认：

- 桌面图标背景不再显示纯色圆底。
- 图标主体为 `pig.svg` 图形。
- 安装后 Manifest 不报资源缺失。

## 8. 后端影响

本轮实现必须区分“API 文档 v3 已声明能力”和“v18 代码实现扩展能力”。

API 文档 v3 已声明：

- `GET /products/{product_id}`：公开商品详情。
- `GET /products/{product_id}/skus`：公开 SKU 列表。
- `POST /cart/items`：用户加购，字段为 `product_id` / `sku_id` / `quantity`。
- `GET /agent/sessions`
- `POST /agent/sessions`
- `GET /agent/sessions/{session_id}`
- `PATCH /agent/sessions/{session_id}`
- `POST /agent/sessions/{session_id}/messages:stream`
- `POST /agent/runs/{run_id}:cancel`
- `GET /agent/runs/{run_id}/trace`

API 文档 v3 没有声明，但当前 v18 后端代码可能已经扩展：

- `DELETE /api/v1/agent/sessions/{session_id}`
- `POST /api/v1/agent/sessions/{session_id}:pin`
- 会话详情返回 `runs.content / blocks / followups / segments`

因此 v19 方案要求：

1. Android 调用文档已声明接口时，严格按 API 文档字段和路径实现。
2. 侧栏置顶/删除如果依赖 v18 扩展接口，后端需要同步更新 API 文档；否则 Android 只能先做本地置顶/本地删除，不能把未声明接口当成稳定契约。
3. 远程历史恢复如果依赖 `runs.content / blocks / followups / segments`，后端也需要把这些字段补进 API 文档；否则 Android 只能保证本地历史恢复，不能承诺远程历史完整恢复商品卡。
4. 如果 `product_refs` 商品详情接口返回失败，优先检查 Android 是否重复拼接 `/api/v1`、是否错误携带鉴权、是否字段名用了 snake_case；再判断是否需要后端修复。

## 9. 验收清单

### 9.1 聊天页

- 发送能触发商品推荐的问题，回答结束后商品卡片正常显示。
- 商品卡片位置与 Agent 输出顺序一致。
- 商品表格只在表格区域横向滚动，整个会话气泡不变宽。
- 普通文本在流式过程中继续实时渲染。
- 表格、公式、代码块等敏感结构在片段完整后再局部渲染，不等待整条回答完成。
- 表格、公式、商品卡不再反复闪烁。
- 从历史会话进入后商品卡和表格仍正常恢复。

### 9.2 侧栏

- 长按历史会话后置顶，弹窗关闭后仍停留侧栏，列表立即显示“置顶 · 标题”。
- 更改会话标题后仍停留侧栏，标题立即变更。
- 删除会话后仍停留侧栏，该会话立即消失。
- 搜索历史时执行上述操作，搜索结果同步刷新。

### 9.3 图标

- 安装调试 APK 后桌面图标显示 `pig.svg`。
- 图标背景透明。
- Android 8+ adaptive icon 和旧版本 fallback 均无资源错误。

### 9.4 自动与手动验证

需要执行：

```bash
cd android-native
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

后端回归：

```bash
cd backend
GOTOOLCHAIN=local go1.24.0 test ./...
```

模拟器验证：

```text
安装新版 APK
登录测试账号
发送商品推荐问题
检查商品卡、表格横向滚动、历史恢复
长按历史会话验证置顶/重命名/删除
回到桌面检查图标
读取 logcat，确认无 AndroidRuntime / FATAL EXCEPTION
```
