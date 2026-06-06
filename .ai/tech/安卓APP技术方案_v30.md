# 安卓 APP 技术方案 v30

## 1. 背景

本文针对 [安卓APP_问题_v29.md](./安卓APP_问题_v29.md) 设计 Android 原生 APP 下一版改造方案。

本轮问题覆盖面比 v29 更大，包含侧栏安全区、聊天欢迎态与流式滚动、设置弹窗、购物车结算链路、评价展示与提交、商品搜索/分类视觉优化。

本轮修改原则：

1. 高版本 Android 的安全区和 system bars 问题只在高版本分支内处理，低版本继续依赖现有 `decorFitsSystemWindows=true` / `adjustResize` 行为。
2. 高版本和低版本的页面结构、操作路径、按钮含义保持一致；差异只允许存在于 inset、动画兼容和系统窗口适配层。
3. 优先复用当前 `MainActivity.java` 的原生 View 工具方法、`ApiClient` 接口和既有页面状态字段，不引入 Compose、不重写架构。
4. 所有新增固定底栏、弹窗、悬浮层都必须走 `applyContentInsets()` / `appliedBottomInset`，避免再次出现状态栏或导航栏遮挡。

## 2. 当前实现核对

### 2.1 侧栏

`showDrawer()` 中侧栏容器当前写死：

```java
drawer.setPadding(dp(14), dp(10), dp(14), dp(10));
drawer.addView(top, new LinearLayout.LayoutParams(-1, dp(62)));
```

在 Android 15+ edge-to-edge 模式下，侧栏是加在 `root` 上的浮层，不经过 `content` 的顶部 padding，因此搜索栏和新建会话按钮会进入系统状态栏区域。

### 2.2 聊天欢迎态

`createChatMessageLayer()` 创建了：

```java
welcome.setText("");
welcomeParams.topMargin = dp(96);
```

随后 `applyHomeCopy()` 使用后端 `welcome_text`，为空时回退到：

```java
"这里展示新会话欢迎语"
```

建议问题来自后端 `home.suggestions`，但当前有两个限制：

1. 未登录时直接不渲染建议按钮。
2. 建议数量和随机策略完全依赖后端，没有本地兜底。

### 2.3 流式输出期间滚动

当前大量渲染入口在追加内容后直接调用 `scrollBottom()`，例如 `updateThinkingPanel()`、`appendAssistant()` 相关分支、`renderAgentBlock()`、`renderItemRefs()` 等。`scrollBottom()` 当前无条件：

```java
chatScroll.post(() -> chatScroll.fullScroll(View.FOCUS_DOWN));
```

因此用户在 AI 输出期间手动向上滚动，下一段 token 或商品卡片渲染后又会被强制拉回底部。

### 2.4 思考组件

v29 已将实时思考组件改成一个统一 panel，但仍使用：

```java
activeThinkingPanel.setBackground(rounded(Color.WHITE, dp(16)));
```

视觉上仍像一个独立白色对话框。本轮要求弱化这个边界，让它更像聊天流中的状态说明。

### 2.5 商品卡片后的标点

商品引用通过 `renderItemRefs()` 在文本段落之后渲染商品卡。当前 markdown 切分可能会把模型输出中紧跟商品引用后的 `，`、`。`、`、`、`,`、`.` 留在可见文本中，导致卡片后出现孤立标点。

### 2.6 设置页弹窗

手机号/邮箱弹窗使用自定义 `AlertDialog dialog = new AlertDialog.Builder(this).create()`，然后 `dialog.setView(box)`。窗口宽度、背景被手动处理，但未设置 `Gravity.BOTTOM` 和底部弹窗样式。

删除账号、退出登录使用系统 `AlertDialog.Builder(...).show()`，在当前主题/设备上表现为底部弹出风格。手机号/邮箱因此出现从左上角出现、样式不一致的问题。

### 2.7 购物车

`renderCartContent()` 把结算区作为普通 `panel()` 加在商品列表末尾：

```java
checkoutBar.addView(strong("已选 ... 件"));
checkoutBar.addView(muted("商品总额：¥..."));
loadDiscountPreview(checkoutBar);
checkoutBar.addView(checkout);
page.addView(checkoutBar);
```

商品多时用户必须滚到底部才能看到已选件数、金额和结算按钮。优惠明细也挤在购物车列表底部，不适合作为固定操作区。

### 2.8 评价功能

后端已提供：

1. `GET /api/v1/products/{product_id}/reviews`
   - 公开接口，只返回 `visible` 评价。
   - 返回分页结构 `items`。
2. `POST /api/v1/orders/{order_id}/items/{order_item_id}:review`
   - 需要用户登录。
   - 只允许已完成订单项评价，且不可重复评价。
   - 入参为 `rating`、`content`、`tags`。

评价数据结构包含：

```go
review_id, order_id, order_item_id, product_id, sku_id,
account_id, username, rating, content, tags, status,
merchant_reply, merchant_replied_at, created_at, updated_at
```

Android 当前商品详情只展示前三条纯文本评价；订单页评价只用两个 `EditText` 输入评分和内容，缺少星级控件、标签、商品上下文和底部弹窗风格。

### 2.9 商品搜索与分类

商品页当前搜索区是下划线输入框 + “搜索”文字按钮，分类选择是普通 `AlertDialog` 两列列表。问题文档在“搜索功能的部分太丑了”处截断，因此本方案先覆盖已明确部分：搜索框、搜索按钮、当前分类展示、分类选择器的视觉与交互优化。

## 3. v30 总体目标

1. 侧栏顶部搜索栏和新建会话按钮在 Android 15+ 不被状态栏遮挡。
2. 新会话欢迎语改为按时间段生成：`早上好/中午好/晚上好/夜深了，{用户名}`。
3. 欢迎语下方随机展示 3 个问题按钮；点击按钮立即发送对应消息。
4. 思考组件取消独立白色气泡观感，融入聊天背景。
5. AI 流式输出时，用户向上滚动后页面停在用户当前位置，底部内容继续生成。
6. 当用户保持在底部或重新滚到底部时，流式输出继续自动贴底。
7. 去掉商品卡片前后残留的孤立标点。
8. 手机号、邮箱设置弹窗与删除账号、退出登录弹窗的出现位置和视觉风格统一。
9. 购物车底部固定展示全选、已选件数、合计金额、结算按钮。
10. 结算详情页承接优惠明细、优惠券、应付金额和确认下单。
11. 商品详情评价区和订单评价流程完成视觉升级。
12. 商品页搜索和分类筛选改成更紧凑、现代、可扫描的控件。

## 4. 高低版本兼容方案

### 4.1 安全区读取

继续使用现有字段：

```java
private boolean manualWindowInsets; // Build.VERSION.SDK_INT >= 35
private int appliedTopInset;
private int appliedBottomInset;
```

新增两个只读方法：

```java
private int currentTopSafeInset()
private int currentBottomSafeInset()
```

规则：

- `manualWindowInsets == true`：返回 `stableTopInset(appliedTopInset)` 和 `Math.max(0, appliedBottomInset)`。
- `manualWindowInsets == false`：返回 0，不改变低版本当前布局。

### 4.2 浮层统一适配

所有直接加到 `root` 的浮层，包括 `drawerLayer`、图片预览、购物车固定底栏、底部弹窗遮罩，都必须手动使用 `currentTopSafeInset()` / `currentBottomSafeInset()`。

不要给普通内容页重复加 top inset，避免低版本和高版本出现双重 padding。

## 5. 侧栏方案

### 5.1 顶部 padding

`showDrawer()` 中侧栏容器 padding 改为：

```java
int topSafe = currentTopSafeInset();
drawer.setPadding(dp(14), dp(10) + topSafe, dp(14), dp(10));
```

侧栏宽度、历史列表、新建会话逻辑不变。

### 5.2 抽屉动画保持一致

只调整 drawer 内部 padding，不修改 `drawerLayer` 的 alpha 动画、点击关闭、历史搜索和新建会话逻辑。低版本 `topSafe=0`，视觉不变。

## 6. 聊天欢迎态方案

### 6.1 欢迎语生成

新增：

```java
private String greetingText()
private String displayNickname()
```

时间段建议：

- 05:00 - 10:59：早上好
- 11:00 - 13:59：中午好
- 14:00 - 18:59：晚上好
- 19:00 - 23:59：晚上好
- 00:00 - 04:59：夜深了

用户名来源：

1. `sessionStore.nickname()`
2. 为空时使用 `用户名`

最终格式：

```text
早上好，张三
```

`applyHomeCopy()` 不再使用后端 `welcome_text` 覆盖主欢迎语；后端文案只可作为 hint 或后续运营位，避免再次回到占位文本。

### 6.2 随机问题池

新增本地问题池，覆盖电商主要类目和导购场景。实际实现至少保留 100 条本地候选，避免用户频繁新建会话时问题重复：

```java
private static final String[] DEFAULT_HOME_PROMPTS = {
    "城市通勤自行车推荐",
    "为我推荐一些护肤品",
    "预算三千以内的手机怎么选",
    "适合送父母的按摩仪推荐",
    "干皮秋冬保湿水怎么选",
    "帮我找适合办公室的咖啡机",
    "儿童安全座椅怎么挑",
    "入门跑步鞋推荐",
    "油皮夏天底妆怎么选",
    "敏感肌可以用的面霜推荐",
    "学生党平价防晒推荐",
    "通勤用双肩包怎么选",
    "适合小户型的空气炸锅推荐",
    "租房党投影仪怎么选",
    "两千以内洗地机推荐",
    "扫地机器人避坑指南",
    "新手露营装备清单",
    "适合办公室久坐的腰靠推荐",
    "送女朋友生日礼物推荐",
    "送男朋友实用礼物推荐",
    "父亲节礼物怎么选",
    "母亲节护肤礼盒推荐",
    "宝宝湿巾和纸尿裤怎么选",
    "新生儿奶瓶推荐",
    "孕妇可用护肤品推荐",
    "儿童学习桌椅怎么挑",
    "家用净水器怎么选",
    "厨房小家电哪些值得买",
    "适合一人食的电饭煲推荐",
    "家用咖啡豆和咖啡机搭配",
    "降噪耳机通勤推荐",
    "运动耳机怎么选",
    "平板电脑学习记笔记推荐",
    "轻薄本办公怎么选",
    "游戏本预算六千推荐",
    "拍照好的手机推荐",
    "老人用手机怎么选",
    "儿童电话手表推荐",
    "智能手表健康监测怎么选",
    "家庭 NAS 入门推荐",
    "路由器大户型怎么选",
    "显示器办公护眼推荐",
    "机械键盘新手推荐",
    "人体工学椅怎么选",
    "护眼台灯学生党推荐",
    "卧室香薰和加湿器推荐",
    "床垫软硬怎么选",
    "四件套材质怎么挑",
    "猫砂和猫粮怎么选",
    "狗狗自动喂食器推荐",
    "车载吸尘器推荐",
    "行车记录仪怎么选",
    "电动车头盔推荐",
    "长途旅行行李箱怎么选",
    "防晒衣和遮阳伞怎么选",
    "户外徒步鞋推荐",
    "骑行头盔和手套推荐",
    "健身新手哑铃怎么选",
    "家用跑步机避坑",
    "瑜伽垫厚度怎么选",
    "游泳装备新手推荐",
    "男士控油洗面奶推荐",
    "男士剃须刀怎么选",
    "头发干枯毛躁护发推荐",
    "防脱洗发水怎么选",
    "口红色号日常通勤推荐",
    "粉底液色号怎么选",
    "新手化妆刷套装推荐",
    "香水入门怎么选",
    "美白精华怎么避坑",
    "抗老精华适合多少岁用",
    "眼霜黑眼圈推荐",
    "身体乳秋冬推荐",
    "牙刷电动还是手动好",
    "冲牙器新手推荐",
    "益生菌和维生素怎么选",
    "低糖零食推荐",
    "早餐麦片怎么选",
    "办公室咖啡和茶包推荐",
    "年货礼盒推荐",
    "端午礼盒怎么选",
    "中秋月饼礼盒推荐",
    "搬家清洁用品清单",
    "浴室收纳怎么做",
    "厨房锅具套装推荐",
    "不粘锅和铁锅怎么选",
    "保温杯材质怎么选",
    "雨天通勤鞋推荐",
    "冬季保暖内衣推荐",
    "羽绒服充绒量怎么选",
    "夏季凉感床品推荐",
    "通勤衬衫不易皱推荐",
    "牛仔裤版型怎么选",
    "女生面试穿搭推荐",
    "男生日常通勤鞋推荐",
    "儿童书包护脊推荐",
    "学习机和平板怎么选",
    "蓝牙音箱户外推荐",
    "家庭影院音响怎么选",
    "相机新手入门推荐"
};
```

渲染时合并后端 `home.suggestions` 与本地池：

1. 后端建议先进入候选池。
2. 本地池补足候选。
3. 去重后随机洗牌。
4. 取 3 条展示。

未登录也展示这 3 个问题按钮。点击后调用现有 `sendMessage(text)`；如果发送链路要求登录，由 `sendMessage()` 或后续 API 处理登录提示，欢迎态不做特殊分叉。

### 6.3 按钮样式

建议按钮改为纵向或自动换行的轻量 chip：

- 宽度 `WRAP_CONTENT`，最大不超过屏宽减边距。
- 高度 42-46dp。
- 背景 `Color.WHITE` 或 `#F1F3F6`，圆角 18dp。
- 文本 14sp，颜色 `#374151`。
- 三个按钮在中心欢迎语下方垂直排列或两行流式排列，避免横向滚动隐藏内容。

## 7. 流式输出滚动方案

### 7.1 引入用户滚动状态

新增字段：

```java
private boolean chatAutoScrollEnabled = true;
private boolean userDetachedFromBottom;
private long lastUserScrollAt;
```

`chatScroll.setOnTouchListener()` 中：

- `ACTION_DOWN`：记录 `lastUserScrollAt`，不要立即隐藏键盘后强制滚动。
- `ACTION_UP` / `ACTION_CANCEL`：根据 `isChatScrolledToBottom()` 更新 `chatAutoScrollEnabled`。

新增 `OnScrollChangeListener`：

```java
chatScroll.setOnScrollChangeListener((v, sx, sy, osx, osy) -> {
    boolean atBottom = isChatScrolledToBottom();
    if (!atBottom && sy < osy) {
        chatAutoScrollEnabled = false;
        userDetachedFromBottom = true;
    } else if (atBottom) {
        chatAutoScrollEnabled = true;
        userDetachedFromBottom = false;
    }
});
```

### 7.2 替换无条件滚动

保留 `scrollBottom()` 作为强制滚到底部方法，用于：

- 用户发送新消息后。
- 打开历史会话时初始定位。
- 创建新会话或切换会话时。

新增：

```java
private void scrollBottomIfAllowed()
```

规则：

- `chatAutoScrollEnabled == true` 时才调用 `scrollBottom()`。
- 如果用户已离底，不滚动。

流式输出路径中的 `scrollBottom()` 改成 `scrollBottomIfAllowed()`：

- `updateThinkingPanel()`
- `appendAssistant()` 相关渲染结束
- `renderAgentBlock()`
- `renderItemRefs()`
- `renderFollowups()`
- 商品卡异步补全回调

用户发送消息时，在 `sendMessage()` 前后明确：

```java
chatAutoScrollEnabled = true;
userDetachedFromBottom = false;
scrollBottom();
```

### 7.3 底部提示

可选增加一个小型“新内容”浮动按钮。当用户离底且 AI 仍在输出时显示，点击后：

```java
chatAutoScrollEnabled = true;
scrollBottom();
```

本轮可以先不做该按钮，先保证“不强制拉回底部”。

## 8. 思考组件视觉方案

### 8.1 实时思考 panel

将 `ensureThinkingPanel()` 中的背景从白色改为透明或与页面背景一致：

```java
activeThinkingPanel.setBackgroundColor(Color.TRANSPARENT);
```

内边距改小：

```java
activeThinkingPanel.setPadding(dp(4), dp(6), dp(4), dp(6));
```

标题颜色保持灰色；步骤列表使用淡灰色左边线或小圆点，不使用白色卡片。

### 8.2 历史思考 panel

`renderHistoricalThinkingPanel()` 同样取消白色背景，宽度仍与助手消息宽度一致，但视觉上像一段系统状态：

- 标题：`✦ 已完成思考 展开`
- 展开内容：每个步骤为一行小图标 + 标题 + 摘要
- 不加阴影、不加白色卡片

## 9. 商品卡片标点清理方案

### 9.1 文本段尾部清理

新增工具方法：

```java
private String trimDanglingProductPunctuation(String text)
```

清理规则：

- 只清理文本末尾的孤立标点：`，。、、,.;；:：`
- 不清理问号、感叹号，避免改变正常语气。
- 不清理中文句子内部标点。

在渲染商品引用前，对即将提交到气泡的文本末尾执行一次清理。

### 9.2 segment 级别兜底

如果商品引用后模型又输出一个仅包含标点的 text segment，则直接丢弃该 segment。新增状态：

```java
private boolean lastRenderedSegmentWasProductRef;
```

当 `renderItemRefs()` 成功渲染至少一张商品卡后置为 true；下一段 text 如果 `trim()` 后只包含上述标点，则跳过。

历史消息渲染也使用同一规则，保证旧消息展示一致。

## 10. 设置弹窗统一方案

### 10.1 抽象底部弹窗

新增通用方法：

```java
private AlertDialog showBottomSheetDialog(View content)
```

窗口配置：

```java
window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
window.setGravity(Gravity.BOTTOM);
window.setDimAmount(0.32f);
params.width = MATCH_PARENT;
params.height = WRAP_CONTENT;
window.setAttributes(params);
window.getDecorView().setPadding(dp(12), 0, dp(12), dp(12) + currentBottomSafeInset());
```

内容容器统一：

- 白色背景
- 顶部圆角 22dp
- 左右 padding 20dp
- 底部 padding 16dp + bottom safe inset

### 10.2 手机号/邮箱设置

`editContact(boolean phone)` 改为使用 `showBottomSheetDialog(box)`，保留当前输入校验、保存禁用态、错误文案和 `updateContact()`。

按钮样式与退出/删除类弹窗统一：

- 主按钮黑底白字。
- 次按钮浅灰背景黑字。
- 两按钮固定高度 48dp。

### 10.3 删除账号/退出登录

为了真正统一风格，建议把 `confirmLogout()`、`confirmDeleteAccount()` 也迁移到同一个 `showConfirmBottomSheet(...)`：

```java
private void showConfirmBottomSheet(String title, String message, String dangerText, Runnable onConfirm)
```

这样手机号、邮箱、退出登录、删除账号都从底部出现，视觉一致。若担心改动范围，第一阶段只迁移手机号/邮箱到相同 bottom gravity；第二阶段再统一确认弹窗。

## 11. 购物车固定底栏与结算页方案

### 11.1 页面结构调整

`renderCart()` 不再只使用 `pageBody()` 承载全部内容，改成：

```java
addPageHeader(...)
FrameLayout cartFrame = new FrameLayout(this)
ScrollView cartScroll
LinearLayout cartList
LinearLayout fixedCheckoutBar
```

或者保持 `content` 纵向：

```java
content.addView(cartScroll, new LinearLayout.LayoutParams(-1, 0, 1));
content.addView(cartCheckoutBar, new LinearLayout.LayoutParams(-1, dp(64) + currentBottomSafeInset()));
```

低版本 bottom inset 为 0，高版本自动避开导航栏。

### 11.2 固定 bar 内容

参考 `assets/cart_page.jpg`，底栏从左到右：

1. 全选 `CheckBox`
2. 文案：`全选` / `已选 xx 件`
3. 合计：`合计：¥payAmount`
4. 结算按钮：`结算`

样式：

- 背景白色。
- 顶部分割线 `#E5E7EB`。
- 高度主体 64dp，底部追加 safe inset。
- 结算按钮橙色或黑色二选一。为保持当前 APP 黑白风格，建议黑色；若要贴近参考图，可使用橙色 `#F97316`。

### 11.3 全选逻辑

后端当前只有单项 `PATCH /cart/items/{id}`，没有全选接口。因此全选实现：

- 如果当前存在未选中项，逐个调用 `api.updateCartItem(cartItemId, null, true)`。
- 如果全部已选中，逐个调用 `api.updateCartItem(cartItemId, null, false)`。
- 完成后重新 `api.cart()` 并刷新列表和底栏。

为避免多次闪烁：

- 全选过程中禁用底栏按钮。
- 所有请求在线程内串行执行。
- 最后一次性 `renderCartContent(...)`。

### 11.4 购物车列表底部 padding

固定底栏会覆盖列表底部，`cartList` 底部 padding 至少：

```java
dp(84) + currentBottomSafeInset()
```

### 11.5 结算详情页

新增页面：

```java
private void renderCheckoutPage(JSONObject cart, JSONObject discount)
```

入口由底栏 `结算` 点击触发：

1. 校验 `selectedCount > 0`。
2. 异步加载 `api.discountPreview()`。
3. 进入结算页。

结算页内容：

- 顶部：`确认订单`
- 商品摘要：已选商品列表，显示图片、名称、数量、单价。
- 金额明细：
  - 商品总额
  - 优惠金额
  - 应付金额
- 优惠明细：
  - `discount.lines` 每一项的名称和金额。
- 优惠券信息：
  - 如果后端 `discountPreview` 已含 coupon line，按 line 展示。
  - 如需完整“我的优惠券”，可复用 `api.myCoupons()` 只读展示，不在本轮做选择券能力。
- 底部固定确认栏：
  - `应付 ¥xx`
  - `提交订单`

### 11.6 提交订单

结算页 `提交订单` 调用现有：

```java
api.checkout()
```

成功后保持现有行为：

```java
toastLine("订单已创建，请尽快支付");
renderOrders();
```

删除原 `confirmCheckout()` AlertDialog，避免“结算按钮 -> 弹窗 -> 下单”的旧链路与新结算页冲突。

## 12. 评价功能优化方案

### 12.1 商品详情评价区

`loadProductReviews()` / `renderProductReviews()` 改造为：

1. 顶部汇总区：
   - 平均评分。
   - 总评价数。
   - 星级展示。
2. 标签聚合：
   - 从 `review.tags` 聚合 Top 5 标签。
   - chip 展示，例如 `保湿好 12`。
3. 评价卡片：
   - 用户名：`username` 为空时显示 `匿名用户`。
   - 星级：使用 5 个星符号或小 TextView。
   - 内容：最多 3 行，长文本可展开。
   - 标签：浅灰 chip。
   - 时间：格式化 `created_at`。
   - 商家回复：有 `merchant_reply` 时用浅灰块显示。
4. 空态：
   - `暂无评价`
   - 副文案：`购买并完成订单后可以发表第一条评价。`

当前 `ApiClient.productReviews()` 只返回 `JSONArray items`，汇总可在客户端计算，不需要后端新增接口。

### 12.2 订单页评价入口

订单卡片中对 `completed` 状态显示“评价商品”。优化：

- 如果订单多商品，选择商品弹窗改为底部 sheet，每项显示商品名、数量、缩略图。
- 如果订单项已有评价标识，后端当前订单 items 未必返回 review 状态；本轮先保留按钮，提交重复评价时展示后端错误。后续可扩展后端订单项返回 `reviewed`。

### 12.3 评价提交底部表单

`showReviewForm()` 改为底部 sheet：

- 顶部商品摘要。
- 星级选择控件：5 个可点击星，默认 5 星。
- 快捷标签 chip：
  - `质量不错`
  - `物流快`
  - `包装好`
  - `性价比高`
  - `和描述一致`
  - `会回购`
- 多行评价输入框，最少 5 个字建议，但只强制非空以匹配后端。
- 提交按钮 loading 态。

提交时把已选标签写入：

```java
JSONArray tags = selectedReviewTags();
api.reviewOrderItem(orderId, orderItemId, rating, content, tags);
```

### 12.4 错误处理

后端重复评价或订单未完成会返回 conflict。Android 文案统一：

```text
评价失败：该商品暂不可评价，可能已评价或订单未完成
```

保留原始错误到 logcat，不直接把后端技术信息完整暴露给用户。

## 13. 商品搜索与分类优化方案

### 13.1 搜索框

替换当前下划线输入框为圆角搜索容器：

```text
[ 搜索图标  搜索商品、品牌、功效            清除 ]
```

细节：

- 背景 `#F3F4F6`
- 圆角 14dp
- 高度 48dp
- 搜索按钮只在键盘 IME_ACTION_SEARCH 或输入变化 debounce 后触发，不再单独放一个蓝色“搜索”文字按钮。
- 有输入时显示清除按钮 `×`。

### 13.2 分类展示

搜索框下方显示当前筛选 chip：

- `全部分类`
- 或当前 `lastCategoryName`

旁边可放 `筛选` 按钮，点击打开分类 sheet。

### 13.3 分类选择器

`openCategoryDialog()` 从普通 AlertDialog 改成底部 sheet：

- 高度约屏幕 70%。
- 左侧一级分类，右侧二级分类。
- 顶部标题 `选择商品类别`，右上角 `重置`。
- 底部固定操作：`取消` / `确定`。
- 选中态使用黑色文字 + 浅灰背景；确定后才写入 `lastCategoryId`，保持当前 pending 机制。

### 13.4 搜索行为

保留现有分页缓存：

```java
productCacheKey(keyword, categoryId)
productListCache
```

输入框行为：

- IME 搜索：立即更新 `lastProductKeyword`，重置滚动和缓存，`loadProducts(...)`。
- 清除按钮：清空关键词并立即加载全部。
- 切换分类：保留当前关键词。

## 14. 实施顺序

### 第一阶段：高版本遮挡与聊天体验

1. 增加 `currentTopSafeInset()` / `currentBottomSafeInset()`。
2. 修复侧栏顶部 padding。
3. 新会话欢迎语和 3 个随机问题按钮。
4. 流式输出滚动从无条件 `scrollBottom()` 改为 `scrollBottomIfAllowed()`。
5. 思考 panel 背景透明化。
6. 商品卡片标点清理。

### 第二阶段：设置与购物车

1. 抽象底部弹窗方法。
2. 手机号/邮箱弹窗迁移到底部 sheet。
3. 退出登录/删除账号确认弹窗统一到底部 sheet。
4. 购物车固定底栏。
5. 结算详情页。
6. 结算成功跳转订单页。

### 第三阶段：评价与商品筛选

1. 商品详情评价区美化与汇总。
2. 订单评价选择商品 sheet。
3. 星级 + 标签 + 多行输入评价表单。
4. 商品搜索框改造。
5. 分类选择器改造为底部 sheet。

## 15. 回归测试清单

### 15.1 Android 15+ 模拟器

1. 打开聊天页，状态栏不透内容。
2. 打开侧栏，搜索栏和新建会话按钮不被状态栏遮挡。
3. 新建会话，欢迎语为时间段 + 用户名。
4. 欢迎语下方出现 3 个建议按钮，点击后直接发送。
5. AI 输出中向上滚动，页面停住；继续输出不强制回底。
6. 滚到底部后，后续输出自动贴底。
7. 商品卡片前后没有孤立逗号、句号、顿号。
8. 打开手机号/邮箱设置，从底部弹出，样式一致。
9. 购物车商品多时，底部结算 bar 始终可见。
10. 点击结算进入结算页，优惠明细从购物车底部迁移到结算页。
11. 提交订单后进入订单页。
12. 商品详情评价区显示星级、标签、回复。
13. 订单页评价表单可提交评价。
14. 商品搜索和分类筛选可正常触发加载。

### 15.2 低版本模拟器或设备

1. 页面顶部不出现额外空白。
2. 侧栏顶部位置与旧版本一致。
3. 键盘、导航栏、购物车底栏没有双重 inset。
4. 所有页面操作路径与高版本一致。

### 15.3 构建验证

在共享盘如 Gradle 文件哈希报 `Operation not supported`，使用 `/private/tmp` 构建副本验证：

```bash
cp -R android-native /private/tmp/xzxg-android-v30-build
echo "sdk.dir=/opt/homebrew/share/android-commandlinetools" > /private/tmp/xzxg-android-v30-build/local.properties
cd /private/tmp/xzxg-android-v30-build
gradle :app:assembleDebug
```

安装运行：

```bash
adb install -r app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n com.xzxg.shop/.MainActivity
```

## 16. 风险与边界

1. 问题文档第 6 条在“搜索功能的部分太丑了，”处截断，本方案只覆盖已明确的搜索和分类视觉优化；若还有未写出的商品页问题，需要后续补充。
2. 后端购物车没有批量全选接口，全选需要逐项 PATCH；购物车项很多时会慢。可后续新增 `/cart/items:select-all` 优化。
3. 后端订单项未明确返回是否已评价，重复评价只能依赖提交接口返回 conflict。可后续让订单接口返回 `reviewed`。
4. 客户端计算评价平均分和标签聚合只基于当前页评价；如果后端分页只返回部分评价，汇总不是全量统计。可后续由后端返回 summary。
5. 随机建议问题每次新建会话会变化；如果需要一次登录周期内稳定，可用当天日期 + accountId 做 deterministic shuffle。
