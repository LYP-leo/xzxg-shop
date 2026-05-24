# 安卓 APP 技术方案 v7

## 1. 背景

本文针对 [安卓APP_问题_v6.md](./安卓APP_问题_v6.md) 重新设计 Android 原生 APP 下一版改造方案。

v6 已经解决了全局顶栏、侧栏安全区、按钮间距等基础布局问题，但当前仍有以下体验缺口：

- 聊天页没有渲染后端流式返回的商品卡片。
- 流式 Markdown 只在结束时渲染，输出过程中仍是纯文本。
- 商品页、购物车页仍使用“图”占位，没有真实图片。
- 商品页缺少详情页和类别筛选。
- 购物车下单缺少确认环节。
- 我的页资料编辑仍像临时表单，不像正式账号资料页。
- Android 返回键没有按页面层级工作。

v7 的目标不是继续微调样式，而是把“可用的购物导购 APP”需要的关键交互补齐。

## 2. 总体设计结论

v7 按以下方向实现：

```text
聊天页
  实时 Markdown 渲染
  实时结构化商品卡片渲染
  顶栏登录状态展示优化

商品页
  搜索 + 分类筛选
  商品图片
  商品详情页
  加入购物车

购物车页
  商品图片
  数量/选择
  下单确认弹窗

我的页
  账号资料卡
  编辑资料表单
  安全设置区

全局导航
  一级页面返回键打开侧栏
  二级页面返回键返回上一界面
```

为了控制风险，继续使用当前单 Activity + Java 原生 View 的实现方式，不在这一版引入 Compose，也不大规模重构成多 Activity。

## 3. 聊天页面

### 3.1 商品卡片渲染

当前后端 SSE 已经可能返回：

```text
block_delta
```

但 APP 只是展示：

```text
已收到结构化内容，商品卡片渲染将在下一阶段完善。
```

v7 要把 `block_delta` 真正渲染成商品卡片。

后端结构化商品块来自 `domain.AgentBlock`，常见场景包括：

- 商品推荐
- 商品对比
- 导购回答中附带商品列表

Android 端处理规则：

```java
if ("block_delta".equals(type)) {
    JSONObject block = event.optJSONObject("block");
    renderAgentBlock(block);
}
```

如果后端事件不是 `block` 字段，而是直接把结构放在事件根节点，则兼容：

```java
JSONObject block = event.optJSONObject("block");
if (block == null) {
    block = event;
}
```

### 3.2 商品卡片 UI

聊天内商品卡片不能复用商品列表的大卡片，否则聊天流会显得太重。采用横向紧凑卡片：

```text
商品图片 72dp x 72dp
商品名 15sp，加粗，最多 2 行
品牌/商家 12sp
价格 16sp，加粗
推荐理由 12sp，最多 2 行
按钮：详情、加入购物车
```

卡片宽度：

```text
最大宽度 = 屏幕宽度 * 0.82
```

如果一个 `block_delta` 中包含多个商品：

- 1 个商品：直接渲染一张卡片。
- 2-3 个商品：连续纵向卡片。
- 超过 3 个：先展示前 3 个，再展示“查看全部相关商品”按钮，点击跳到商品页并带上当前关键词。

### 3.3 商品对比块

如果结构化块是对比类，渲染为：

```text
对比标题
商品 A / 商品 B / 商品 C
关键差异
适合人群
```

实现上不要在聊天页做复杂表格，因为手机屏幕横向空间不足。后端如果返回表格 Markdown，可以继续由 MarkdownRenderer 渲染；结构化商品对比优先使用纵向对比卡片。

### 3.4 流式 Markdown 实时渲染

当前逻辑：

```java
appendAssistant(delta) -> activeAssistant.setText(rawMarkdown)
finishStream() -> MarkdownRenderer.setMarkdown(...)
```

问题是用户在流式输出过程中看到的是 Markdown 源码，而不是渲染结果。

v7 改为：

```java
appendAssistant(delta) {
    activeAssistantMarkdown.append(delta);
    MarkdownRenderer.setMarkdown(activeAssistant, activeAssistantMarkdown.toString());
}
```

也就是说，流式输出了什么，就实时渲染什么，即使 Markdown 片段尚未闭合。

本项目不做 80ms、100ms 这类时间节流。后端每推送一个 `delta`，Android 端就立即追加到 `activeAssistantMarkdown`，并立刻调用一次 `MarkdownRenderer.setMarkdown(...)`。如果后端按单字推送，就做到“每展示一个字就渲染一次”。

`message_end` 时仍然强制完整渲染一次，确保最终 Markdown 状态和本地保存内容一致。

渲染策略：

- 普通文字、标题、列表、粗体、代码块、引用、表格、任务列表、HTML、图片均继续交给 Markwon。
- 未闭合的代码块、表格、链接按 Markwon 当前可解析状态展示，不做自定义补全。
- 如果 Markwon 渲染异常，降级为纯文本，不中断流式输出。

### 3.5 回复等待加载效果

当前用户发送消息后，到后端返回第一个 `text_delta` / `block_delta` 之间，聊天区没有明确反馈，用户会以为没有发出去。

v7 增加“助手加载气泡”：

```text
用户消息入列
  -> 立即新增一条助手 loading 气泡
  -> 显示三个跳动点 / “正在思考”
  -> 收到第一个 text_delta 时，把 loading 气泡替换为真实助手消息
  -> 收到第一个 block_delta 时，移除 loading 气泡并渲染商品卡片
  -> 发生错误时，把 loading 气泡改为错误提示
```

加载气泡样式：

```text
左侧助手气泡
浅色背景
内容为 “正在思考...” 或三个动态点
字号 15sp
```

实现上新增：

```java
private TextView loadingAssistant;
```

发送消息时：

```java
loadingAssistant = addLoadingBubble();
```

收到第一段回复时：

```java
removeLoadingBubbleIfNeeded();
activeAssistant = addBubble("", false);
```

### 3.6 发送按钮箭头居中

当前发送按钮是 `Button` + 文字符号 `➤`，垂直居中受字体基线影响。

v7 采用两种可选实现，优先第一种：

```text
方案 A：用 TextView/ImageButton 绘制发送图标，gravity = CENTER
方案 B：继续 Button，但取消默认 minHeight/minWidth/includeFontPadding，并设置 Gravity.CENTER
```

推荐落地：

```java
TextView actionButton = new TextView(this);
actionButton.setText("➤");
actionButton.setGravity(Gravity.CENTER);
actionButton.setIncludeFontPadding(false);
```

按钮容器固定为：

```text
40dp x 40dp
```

### 3.7 顶栏登录状态

当前未登录显示“未登录”，已登录显示“已登录”。v7 改成：

```text
未登录：右上角显示“登录”
已登录：右上角显示头像 + 用户昵称
```

未登录：

- 文案：`登录`
- 点击进入我的页登录表单。

已登录：

- 头像：32dp 圆形，优先使用 `avatar_url`，没有则显示昵称首字。
- 名称：最多 6 个中文字符，超出省略。
- 点击进入我的页。

## 4. 侧栏页面

### 4.1 历史字体

当前聊天历史字体大于一级导航字体。v7 统一：

```text
一级导航字体：15sp
历史会话字体：15sp
```

历史会话只展示标题，不展示时间、消息数量、状态等副信息。

历史会话标题最多两行，超出省略；不要用大标题样式。

## 5. 商品页面

### 5.1 图片展示

后端商品列表已有字段：

```json
{
  "imageUrl": "..."
}
```

v7 Android 端新增统一图片组件：

```java
private View productImage(String imageUrl, int sizeDp)
```

规则：

- `http/https` 图片：异步下载并展示。
- `/api/v1/assets/...` 相对路径：拼接 `apiBase` 的协议、host、port 后再加载。
- 失败时展示浅灰占位 + “图”。
- 图片使用 `CENTER_CROP`，圆角 10-12dp。
- 图片下载结果做内存缓存，避免列表滚动或刷新重复请求。

本项目当前没有 Glide/Coil 依赖。为了避免引入 Kotlin/AndroidX 复杂度，v7 先实现一个简单的 Java 图片加载器：

```text
ImageLoader
  Map<String, Bitmap> memoryCache
  ExecutorService
  load(ImageView target, String url)
```

### 5.2 搜索栏与列表间距

当前搜索栏和商品列表挨得太近。v7 规则：

```text
搜索区底部 margin：12dp
筛选区底部 margin：10dp
商品卡片间距：10dp
```

### 5.3 分类筛选

后端已有：

```text
GET /api/v1/categories/tree
GET /api/v1/products?keyword=&category_id=
```

v7 商品页结构：

```text
搜索行
分类横向筛选 chips
商品列表
```

分类筛选规则：

- 默认 chip：`全部`
- 一级分类和二级分类都可展示，优先展示二级分类；如果分类较少，则一级和二级一起展示。
- 当前选中的分类使用黑底白字。
- 切换分类后重新请求商品列表。
- 搜索关键词和分类可以同时生效。

Android 新增 API：

```java
JSONArray categoriesTree()
JSONArray products(String keyword, String categoryId)
```

### 5.4 商品详情入口

商品卡片新增两个按钮：

```text
详情
加入购物车
```

列表卡片推荐布局：

```text
图片 88dp
商品名
品牌/商家/库存状态
价格
卖点
底部按钮：详情、加入购物车
```

### 5.5 商品详情页

新增页面：

```java
renderProductDetail(String productId)
```

接口：

```text
GET /api/v1/products/{productId}
GET /api/v1/products/{productId}/skus
```

详情页展示：

- 顶部商品大图，优先 `imageUrls[0]`，没有则 `imageUrl`。
- 商品名、品牌、商家。
- 价格、市场价、库存状态。
- 卖点。
- 推荐理由。
- 适合人群。
- 不适合人群。
- 风险提示。
- 商品参数 attributes。
- SKU 列表。
- 底部按钮：加入购物车。

页面层级：

- 从商品页进入详情页，返回键回商品页。
- 从聊天商品卡进入详情页，返回键回聊天页。

## 6. 购物车页面

### 6.1 图片展示

购物车接口已有 `imageUrl` 字段。购物车商品行改为真实图片：

```text
选择框 40dp
图片 64dp
商品信息
数量控制
```

图片加载复用商品页 `productImage()`。

### 6.2 下单确认

当前点击“结算”后直接下单，缺少确认。v7 改成：

```text
点击结算
  -> 弹出确认对话框
  -> 展示商品数量、应付金额、提示
  -> 用户点击“确认下单”
  -> 调用 /orders:checkout
```

确认弹窗内容：

```text
确认下单
已选择 X 件商品
应付金额：¥Y
订单提交后将在订单页查看处理状态。

[取消] [确认下单]
```

如果 `selectedCount == 0`，不弹窗，直接 toast：

```text
请先选择要结算的商品
```

## 7. 我的页面

### 7.1 未登录态

未登录态保留登录/注册，但布局要分区：

```text
账号登录
  账号
  密码
  登录按钮

注册账号
  注册顾客账号
  注册商家账号

说明
  当前验证码为 mock...
```

每个区域使用独立 panel，避免所有控件堆在一个页面里。

### 7.2 已登录态资料页

已登录后不直接把所有输入框堆在页面上，改成：

```text
用户资料卡
  头像
  昵称
  角色
  账号

编辑资料
  昵称
  头像 URL
  保存资料

账号安全
  原密码
  新密码
  确认新密码
  修改密码

退出登录
```

### 7.3 表单校验

保存资料：

- 昵称不能为空。
- 昵称最多 24 个字符。
- 头像 URL 可为空；不为空时必须是 `http://` 或 `https://`。

修改密码：

- 原密码不能为空。
- 新密码至少 6 位。
- 新密码和确认新密码必须一致。

### 7.4 后端字段

当前 `ApiClient.updateProfile(String nickname)` 只提交：

```json
{
  "nickname": "...",
  "avatar_url": ""
}
```

v7 改成：

```java
updateProfile(String nickname, String avatarUrl)
```

同时 `SessionStore` 需要保存头像 URL，供聊天顶栏和我的页资料卡使用。

## 8. 返回键逻辑

### 8.1 页面分层

定义一级页面：

```text
AI导购 chat
商品 products
购物车 cart
订单 orders
我的 profile
```

在一级页面按 Android 返回键：

```text
打开侧栏
```

如果侧栏已经打开：

```text
关闭侧栏
```

### 8.2 二级页面

二级页面包括：

```text
商品详情 product_detail
设置 settings
未来的订单详情 order_detail
未来的编辑页 edit_profile
```

在二级页面按返回键：

```text
返回上一界面
```

### 8.3 实现方式

当前是单 Activity，v7 增加轻量页面栈：

```java
private final Deque<Runnable> backStack = new ArrayDeque<>();
```

进入二级页面时传入返回动作：

```java
openProductDetail(productId, () -> renderProducts(lastKeyword, lastCategoryId));
```

或者保存页面状态：

```java
private String previousPage;
private String lastProductKeyword;
private String lastCategoryId;
```

推荐做法：

- 一级页面切换时清空 `backStack`。
- 二级页面打开时压入返回动作。
- `onBackPressedDispatcher` 或 `onBackPressed()` 中统一处理。

处理顺序：

```text
1. 如果侧栏打开：关闭侧栏
2. 如果当前是二级页面且 backStack 非空：执行上一界面
3. 如果当前是一级页面：打开侧栏
4. 兜底：回到聊天页
```

## 9. 需要新增或修改的代码

### 9.1 MainActivity.java

新增/修改：

- `renderAgentBlock(JSONObject block)`
- `chatProductCard(JSONObject product)`
- `appendAssistant(String delta)` 实时 Markdown 渲染
- `createTopBar(...)` 支持登录/头像/昵称
- `drawer historyButton(...)` 字号统一 15sp
- `renderProducts(...)` 支持搜索 + 分类
- `renderProductDetail(String productId)`
- `productCard(...)` 增加图片和详情按钮
- `cartItemView(...)` 增加图片
- `confirmCheckout(...)`
- `renderProfile()` 重新分区设计
- 返回键处理逻辑

### 9.2 ApiClient.java

新增/修改：

- `JSONArray categoriesTree()`
- `JSONObject productDetail(String productId)`
- `JSONObject updateProfile(String nickname, String avatarUrl)`
- `String absolutizeAssetUrl(String url)` 或交给 ImageLoader 处理

### 9.3 SessionStore.java

新增：

- 保存 `avatarUrl`
- 读取 `avatarUrl`
- 登录、注册、更新资料后同步头像

### 9.4 新增 ImageLoader.java

职责：

- 异步加载网络图片。
- 支持相对资源路径拼接。
- 内存缓存。
- 失败占位。

## 10. 验收标准

### 10.1 聊天页

- 发送消息后，Markdown 在流式输出过程中实时渲染。
- 用户发送消息后，后端首段回复返回前，聊天区展示助手加载气泡。
- 后端每返回一个 `delta`，聊天区立即追加并重新渲染，不做时间节流。
- 后端返回商品结构化块时，聊天页展示商品卡片，而不是提示“下一阶段完善”。
- 商品卡片能点击详情。
- 商品卡片能加入购物车。
- 发送按钮箭头视觉居中。
- 未登录右上角显示“登录”。
- 已登录右上角显示头像 + 昵称。

### 10.2 侧栏

- 历史聊天标题字号与一级导航一致。
- 历史聊天只展示标题，不展示副信息。
- 历史聊天不再显得比导航更重。

### 10.3 商品页

- 商品图片正常展示。
- 搜索栏和商品列表之间有明确间距。
- 可以按分类筛选商品。
- 商品卡片有“详情”按钮。
- 商品详情页展示图片、价格、卖点、参数、SKU。

### 10.4 购物车页

- 购物车商品图片正常展示。
- 点击结算先弹确认框。
- 确认后才真正创建订单。

### 10.5 我的页

- 未登录态分成登录、注册、说明区域。
- 已登录态分成资料卡、编辑资料、安全设置、退出登录区域。
- 资料编辑有基本校验。

### 10.6 返回键

- 在 `AI导购 / 商品 / 购物车 / 订单 / 我的` 按返回键打开侧栏。
- 侧栏打开时按返回键关闭侧栏。
- 商品详情页按返回键回到来源页面。
- 设置页等二级页面按返回键回到上一界面。

## 11. 测试计划

### 11.1 构建测试

```bash
gradle --no-daemon -Dorg.gradle.vfs.watch=false assembleDebug
```

因为 `/Volumes/shared` 下 Gradle 文件哈希可能失败，仍然使用 `/private/tmp/xzxg-android-build` 复制目录构建。

### 11.2 模拟器测试

在模拟器中验证：

- 未登录聊天顶栏。
- 登录后聊天顶栏。
- 流式 Markdown。
- 聊天商品卡片。
- 商品列表图片。
- 商品详情页返回。
- 分类筛选。
- 购物车图片。
- 下单确认弹窗。
- 我的页编辑资料。
- 一级页面返回键打开侧栏。
- 二级页面返回键回上一页。

### 11.3 后端联调

后端建议以局域网可访问地址启动：

```bash
API_ADDR=192.168.3.100:8080 go run ./cmd/api
```

APP 测试版后端地址设置为：

```text
http://192.168.3.100:8080/api/v1
```

如果模拟器访问本机，也可以使用：

```text
http://10.0.2.2:8080/api/v1
```

## 12. 实施顺序

1. 先做图片加载器和 URL 归一化，因为商品页、购物车、聊天商品卡都依赖它。
2. 改商品页：图片、间距、分类筛选、详情入口。
3. 做商品详情页和返回栈。
4. 改购物车图片和下单确认。
5. 改聊天页：实时 Markdown、商品 block 渲染、顶栏登录状态、发送按钮居中。
6. 改侧栏历史字号。
7. 重做我的页信息结构和校验。
8. 模拟器完整回归。
