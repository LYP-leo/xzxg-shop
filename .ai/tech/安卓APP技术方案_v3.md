# 安卓 APP 技术方案 v3

## 1. 背景

本文针对 [安卓APP_问题_v2.md](./安卓APP_问题_v2.md) 中提出的问题，设计下一版 Android 原生 APP 改造方案。

v2 已完成侧栏、基础页面层级和 Toast 覆盖，但仍存在以下关键问题：

- 全面屏适配不完整，状态栏和顶部栏割裂。
- 聊天气泡固定宽度，短消息也显示得很长。
- AI 回复没有渲染 Markdown。
- 底部输入栏阴影被屏幕底部裁切。
- 历史会话排序规则错误，点击历史会话会改变顺序。
- 商品、购物车、订单只展示粗糙 JSON，没有形成可用功能。

v3 的目标是把 APP 从“能展示页面”推进到“基础购物流程可用”，同时修正聊天页的基础体验问题。

## 2. 改造范围

本轮只改 Android 原生顾客端：

```text
android-native/app/src/main/java/com/xzxg/shop
```

优先涉及：

- `MainActivity.java`
- `ApiClient.java`
- `LocalChatStore.java`

必要时新增工具类：

- `MarkdownRenderer.java`
- `MoneyText.java`
- `ProductViewFactory.java`
- `CartViewFactory.java`

不改动后端接口语义；只按现有后端接口接入。

## 3. 全面屏与状态栏方案

### 3.1 当前问题

现在状态栏区域和 APP 顶部栏分离，看起来像系统栏压在页面上方。全面屏手机上，页面顶部空间不自然。

### 3.2 目标效果

- 状态栏背景色与页面顶部背景一致。
- 页面内容不会被状态栏遮挡。
- 首页、侧栏、商品、购物车、订单、我的、设置页面都使用同一套顶部安全区规则。

### 3.3 实现方案

在 `MainActivity.onCreate()` 或 `baseScreen()` 中统一设置：

```java
Window window = getWindow();
window.setStatusBarColor(BG_COLOR);
window.setNavigationBarColor(BG_COLOR);
window.getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
```

页面顶部 padding 不再写死为 `30dp/32dp`，改为：

```text
topPadding = statusBarHeight + 14dp
```

新增方法：

```java
private int statusBarHeight()
private int navBarHeight()
```

使用规则：

- 顶部栏：`paddingTop = statusBarHeight + 12dp`
- 底部输入区：`paddingBottom = navBarHeight + 14dp`
- 侧栏：`paddingTop = statusBarHeight + 18dp`

## 4. 聊天气泡宽度方案

### 4.1 当前问题

聊天气泡使用固定宽度：

```java
width = screenWidth * 0.78
```

导致短消息，例如 `你是谁`，气泡也占据很长一段宽度。

### 4.2 目标效果

- 短消息气泡只包裹文字。
- 长消息最多占屏幕宽度的 78%。
- 用户消息右对齐，AI 消息左对齐。
- Markdown 渲染后的内容也遵守最大宽度。

### 4.3 实现方案

气泡布局改为：

```java
TextView bubble = new TextView(this);
bubble.setMaxWidth((int) (screenWidth * 0.78f));
LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
    ViewGroup.LayoutParams.WRAP_CONTENT,
    ViewGroup.LayoutParams.WRAP_CONTENT
);
params.gravity = right ? Gravity.RIGHT : Gravity.LEFT;
```

不要再给气泡设置固定 `LayoutParams.width`。

细节规则：

- 最小宽度不强制设置。
- 气泡左右 padding 保持 `14dp`。
- 中文短句保持单行。
- 超过最大宽度后自然换行。
- 系统提示仍居中展示，不使用聊天气泡样式。

## 5. Markdown 渲染方案

### 5.1 当前问题

AI 回复中的 Markdown 会原样显示，例如：

```text
**重点**
- 列表项
1. 编号
```

这会让 AI 聊天体验很粗糙。

### 5.2 设计选择

为了避免后续反复返工，v3 不再手写轻量 Markdown 子集，而是接入成熟 Markdown 渲染库，目标是支持完整 Markdown 文档格式。

支持范围：

| Markdown 能力 | v3 要求 |
| --- | --- |
| 标题 | 支持 `#` 到 `######` |
| 段落与换行 | 支持普通段落、软换行、空行分段 |
| 加粗/斜体/删除线 | 支持 |
| 有序/无序列表 | 支持多级嵌套 |
| 引用块 | 支持 |
| 行内代码 | 支持 |
| 代码块 | 支持 fenced code block，至少做到等宽字体和块级背景 |
| 链接 | 支持点击或至少以可识别样式展示 |
| 图片 | 支持图片节点；如果图片加载能力不足，先显示可点击链接或占位，不丢内容 |
| 表格 | 支持 GitHub Flavored Markdown 表格 |
| 分割线 | 支持 |
| 任务列表 | 支持 GitHub Flavored Markdown checkbox |

推荐库：

```gradle
implementation "io.noties.markwon:core:<version>"
implementation "io.noties.markwon:ext-tables:<version>"
implementation "io.noties.markwon:ext-tasklist:<version>"
implementation "io.noties.markwon:image:<version>"
implementation "io.noties.markwon:syntax-highlight:<version>"
```

版本号实现时选择当前 Gradle 可解析的稳定版本，并写入 `android-native/app/build.gradle`。如果网络或依赖解析失败，不能退回手写子集，应先解决依赖安装问题。

### 5.3 实现方案

新增：

```java
MarkdownRenderer.java
```

对外方法：

```java
public static void setMarkdown(TextView target, String markdown)
```

使用位置：

- `appendAssistant(delta)`：流式阶段可先追加纯文本。
- `finishStream()`：完成后用完整 Markdown 渲染库把整段 AI 回复重新渲染。
- `renderStoredMessagesIfAny()`：恢复历史消息时，对 assistant 消息渲染 Markdown。

注意：

- 用户消息不渲染 Markdown，按普通文本展示。
- AI 消息保存到 SQLite 时仍保存原始 Markdown 文本，避免丢失语义。
- Markdown 渲染后的消息气泡仍必须遵守最大宽度规则。
- 表格和代码块可能横向较宽，气泡内需要允许横向滚动或降级成可读的等宽换行展示，不能撑破页面。

## 6. 底部输入栏阴影方案

### 6.1 当前问题

底部输入栏使用 elevation 后，阴影下半部分被屏幕底部或父容器裁切。

### 6.2 目标效果

- 输入栏底部阴影完整可见。
- 不遮挡系统手势条。
- 键盘弹出时输入栏仍在键盘上方。

### 6.3 实现方案

底部输入区改成独立容器：

```text
composerOuter
  paddingLeft/right = 28dp
  paddingTop = 10dp
  paddingBottom = navBarHeight + 18dp
  clipToPadding = false
  clipChildren = false
  composerBar
```

根布局和内容布局均设置：

```java
root.setClipChildren(false);
root.setClipToPadding(false);
content.setClipChildren(false);
content.setClipToPadding(false);
```

输入栏高度仍为 `64dp`，但外层底部留出额外空间给阴影。

## 7. 历史会话排序方案

### 7.1 当前问题

现在点击历史会话时调用了 `touchSession()`，导致只是查看历史，也会把这条会话移动到最前。这不符合聊天软件习惯。

用户期望：

- 点击历史会话只是打开，不改变排序。
- 只有在该会话里新发送消息时，才把它移动到历史列表最前。
- 当前会话未发送消息时不显示在历史列表里。
- 当前会话发送过消息后显示在历史列表里。
- 空会话不显示在历史列表里。

### 7.2 正确规则

定义两个时间：

```text
viewed_at: 最近查看时间，不影响历史排序
updated_at: 最近产生新消息时间，影响历史排序
```

当前表只有 `updated_at`，v3 先不新增数据库字段。实现规则如下：

| 行为 | 是否更新 `updated_at` |
| --- | --- |
| APP 启动创建空会话 | 否，只写 created_at |
| 新建会话 | 否 |
| 点击历史会话 | 否 |
| 发送用户消息 | 是 |
| 收到 AI 回复完成 | 是 |
| 修改标题 | 否，除非由新消息触发 |

### 7.3 代码调整

`LocalChatStore`：

```java
void saveMessage(...) // 保持更新 updated_at
List<SessionSummary> recentSessions()
boolean hasMessages(String localSessionId)
List<MessageItem> messages(String localSessionId)
```

移除或禁止在点击历史时调用：

```java
touchSession(localSessionId)
```

历史查询改为不再强制排除当前会话，只排除空会话：

```sql
WHERE EXISTS (
  SELECT 1 FROM messages
  WHERE messages.local_session_id = sessions.local_session_id
)
ORDER BY updated_at DESC
LIMIT 50
```

如果 UI 需要区分当前会话，可在 `SessionSummary` 中用 `localSessionId.equals(currentLocalSessionId)` 标记，并给当前会话添加轻微选中态或“当前”标识。

### 7.4 当前会话展示规则

侧栏历史区域按照“是否有消息”决定是否展示当前会话：

- 当前会话没有任何消息：不展示。
- 当前会话已经发送过消息：展示，并参与正常排序。
- 当前会话是否排在第一，只由 `updated_at` 决定。
- 点击历史会话只打开，不更新 `updated_at`。
- 在任意会话中新发送消息后，该会话更新 `updated_at`，因此移动到历史列表最前。

例如：

```text
当前会话 A：未发送消息
历史会话：B、C、D
```

当 A 发送消息后：

```text
当前会话 A：已发送消息
历史会话：A、B、C、D
```

如果点击历史会话 C，只是进入 C，不改变排序：

```text
当前会话 C
历史会话：A、B、C、D
```

如果随后在 C 中发送新消息：

```text
当前会话 C
历史会话：C、A、B、D
```

## 8. 商品页方案

### 8.1 后端接口

```http
GET /api/v1/products?keyword=&category_id=
GET /api/v1/products/{product_id}/skus
POST /api/v1/cart/items
```

商品字段：

```text
productId
skuId
merchantId
merchantName
name
brand
categoryId
imageUrl
price
marketPrice
stockStatus
tags
sellingPoints
recommendReason
riskNotes
```

### 8.2 页面结构

```text
PageHeader
  商品
  搜索商品、查看详情、加入购物车

搜索栏

商品列表
  商品图
  商品名
  品牌 / 商家
  价格
  卖点
  加入购物车
```

### 8.3 交互规则

- 进入商品页自动请求 `GET /products`。
- 搜索框输入后点击搜索，使用 `keyword` 查询。
- 商品卡片点击后展开详情区：
  - `recommendReason`
  - `sellingPoints`
  - `riskNotes`
  - SKU 列表
- 点击“加入购物车”：
  - 未登录：Toast 提示并跳转我的页。
  - 已登录：调用 `POST /cart/items`。
  - 成功后 Toast 提示，并可跳转购物车。

### 8.4 SKU 规则

- 如果商品返回 `skuId`，直接使用。
- 如果没有 `skuId`，先请求 `/products/{productId}/skus`。
- 如果 SKU 为空，使用空 `sku_id` 调用加入购物车。
- 如果 SKU 多个，第一阶段默认选择第一个有库存 SKU；后续再做 SKU 弹窗。

## 9. 购物车页方案

### 9.1 后端接口

```http
GET /api/v1/cart
PATCH /api/v1/cart/items/{cart_item_id}
DELETE /api/v1/cart/items/{cart_item_id}
POST /api/v1/orders:checkout
```

购物车字段：

```text
items[]
  cartItemId
  productId
  skuId
  name
  imageUrl
  price
  quantity
  selected
  stockStatus
  merchantId
  merchantName

summary
  selectedCount
  totalAmount
  discountAmount
  payAmount
```

### 9.2 页面结构

```text
PageHeader
  购物车
  调整数量、选择商品并结算

购物车列表
  勾选
  商品图
  名称 / 商家
  单价
  数量 - / +
  删除

底部结算栏
  已选数量
  应付金额
  结算按钮
```

### 9.3 交互规则

- 进入购物车页：
  - 未登录：展示登录引导。
  - 已登录：请求 `GET /cart`。
- 勾选商品：
  - 调用 `PATCH /cart/items/{id}`，body: `{"selected": true/false}`。
  - 成功后用返回的 cart 刷新整页。
- 修改数量：
  - `-` 最小到 1。
  - `+` 加 1。
  - 调用 `PATCH /cart/items/{id}`，body: `{"quantity": n}`。
- 删除：
  - 调用 `DELETE /cart/items/{id}`。
  - 成功后刷新整页。
- 结算：
  - 如果 `selectedCount == 0`，Toast 提示。
  - 否则调用 `POST /orders:checkout`。
  - 成功后进入订单页。

## 10. 订单页方案

### 10.1 后端接口

```http
GET /api/v1/orders
```

订单字段：

```text
order_id
merchant_name
status
total_amount
items[]
created_at
```

### 10.2 页面结构

```text
PageHeader
  订单
  查看顾客订单状态

订单列表
  订单号
  状态
  商家
  总金额
  商品摘要
  创建时间
```

### 10.3 状态展示

| 后端状态 | APP 文案 |
| --- | --- |
| `pending_pay` | 待支付 |
| `paid` | 已支付 |
| `pending_ship` | 待发货 |
| `shipped` | 已发货 |
| `completed` | 已完成 |
| `cancelled` | 已取消 |

未知状态直接展示原始值。

## 11. ApiClient 调整

新增方法：

```java
JSONArray products(String keyword, String categoryId)
JSONArray productSkus(String productId)
JSONObject addCartItem(String productId, String skuId, int quantity)
JSONObject updateCartItem(String cartItemId, Integer quantity, Boolean selected)
JSONObject deleteCartItem(String cartItemId)
JSONArray checkout()
```

注意字段命名：

- 加购物车 body 使用后端要求的蛇形字段：

```json
{
  "product_id": "p_001",
  "sku_id": "sku_001",
  "quantity": 1
}
```

- Android 解析返回值时按后端 JSON 字段：
  - 商品：驼峰字段。
  - 购物车：驼峰字段。
  - 订单：下划线字段。

## 12. 页面状态与错误处理

所有列表页统一三种状态：

```text
loading
empty
content
error
```

展示规则：

- loading：显示轻量加载文案，不弹 Toast。
- empty：页面中展示空状态。
- error：页面中展示错误卡片，并提供“重试”按钮。
- 用户主动操作失败：Toast + 保持原页面状态。

## 13. 图片加载方案

商品、购物车、订单中会出现 `imageUrl`。

当前不引入图片库，v3 分两步：

第一步：

- 如果 `imageUrl` 为空，展示浅灰占位图块。
- 如果 `imageUrl` 非空，先展示文字占位，不阻塞业务流程。

第二步：

- 增加原生 `BitmapFactory + HttpURLConnection` 简单异步图片加载。
- 增加内存缓存，避免列表滚动重复下载。

本轮优先保证功能可用，图片精细加载可作为实现时的次优先级。

## 14. 验收标准

### 14.1 全面屏

- 状态栏背景和页面顶部背景一致。
- 顶部按钮和标题不贴状态栏。
- 底部输入栏不贴手势条。

### 14.2 聊天气泡

- 发送 `你是谁` 时，用户气泡宽度只包裹文本。
- 长文本气泡最大不超过屏幕 78%。
- AI Markdown 回复能正确显示标题、加粗、斜体、删除线、链接、引用、列表、嵌套列表、代码块、表格、任务列表、分割线、段落换行。

### 14.3 输入栏

- 底部阴影完整可见。
- 键盘收起时不被系统手势条遮挡。

### 14.4 历史会话

- 点击历史会话不改变历史排序。
- 只有发送新消息后，该会话才移动到最新。
- 当前会话未发送消息时不出现在历史列表。
- 当前会话发送消息后出现在历史列表。
- 空会话不出现在历史列表。

### 14.5 商品

- 商品页能加载后端商品列表。
- 商品搜索可用。
- 商品卡片展示名称、价格、卖点、商家。
- 登录后可加入购物车。

### 14.6 购物车

- 购物车页能加载后端购物车。
- 可勾选/取消勾选。
- 可修改数量。
- 可删除商品。
- 可结算生成订单。

### 14.7 订单

- 订单页能加载后端订单。
- 订单展示状态、金额、商家、商品摘要。
- 购物车结算成功后能进入订单页看到新订单。

## 15. 模拟器测试要求

实现后必须在模拟器验证以下流程：

```text
1. 启动 APP，检查状态栏融合。
2. 发送短消息“你是谁”，检查气泡宽度。
3. 触发或模拟完整 Markdown 回复，检查标题、列表、表格、代码块、引用、链接。
4. 打开侧栏，切换历史会话，确认排序不变。
5. 进入商品页，加载商品。
6. 登录账号。
7. 商品加入购物车。
8. 进入购物车，修改数量、勾选、删除。
9. 购物车结算。
10. 进入订单页，确认订单展示。
```

构建：

```bash
cd android-native
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools \
GRADLE_USER_HOME=/private/tmp/xzxg-gradle-home \
gradle --project-cache-dir /private/tmp/xzxg-gradle-project-cache assembleDebug
```

安装：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb install -r app/build/outputs/apk/debug/app-debug.apk
```

启动：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb shell am start -n com.xzxg.shop/.MainActivity
```

截图建议：

```text
/private/tmp/xzxg-android-v3-home-safe-area.png
/private/tmp/xzxg-android-v3-short-bubble.png
/private/tmp/xzxg-android-v3-markdown.png
/private/tmp/xzxg-android-v3-history-order.png
/private/tmp/xzxg-android-v3-products.png
/private/tmp/xzxg-android-v3-cart.png
/private/tmp/xzxg-android-v3-orders.png
```

崩溃检查：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb logcat -d -v time \
  | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|System.err" \
  | tail -120
```

## 16. 实施顺序

1. 修复全面屏和底部安全区。
2. 修复聊天气泡宽度。
3. 接入完整 Markdown 渲染库。
4. 修正历史会话排序规则，移除点击历史时的 `touchSession()`。
5. 扩展 `ApiClient` 的商品、购物车、订单方法。
6. 重做商品页。
7. 重做购物车页。
8. 重做订单页。
9. 模拟器完整回归。

## 17. 风险与边界

- 当前 APP 仍是 Java 动态 View，继续扩展会让 `MainActivity` 变重；实现时应尽量拆出小型 View 工厂方法。
- Markdown 渲染必须支持完整 Markdown 文档格式；如果表格、图片、代码块存在布局压力，应做横向滚动或可读降级，不能直接丢内容。
- 图片加载不是本轮最关键目标，不能因为图片加载阻塞购物流程。
- 购物车和订单接口需要登录态；测试时必须先登录。
- 如果后端返回字段为空，APP 必须降级展示，不能崩溃。
