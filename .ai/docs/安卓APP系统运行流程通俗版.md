# 安卓 APP 系统运行流程通俗版

更新时间：2026-06-08

参考文档：[安卓APP模块架构图.md](./安卓APP模块架构图.md)

## 一、总览：这个 APP 是怎么跑起来的

可以把这个 Android APP 理解成一个“商场导购手机端”。

用户打开 APP 后，默认先进入 AI 导购聊天页。用户可以直接问问题，比如“我想买一台适合拍照的手机”，AI 会一边思考、一边从后端拿商品、优惠、订单、知识库等信息，然后把结果实时显示出来。用户看到推荐商品后，可以点进商品详情、加入购物车、结算下单、查看订单、确认收货和评价。

从运行角度看，整个 APP 主要由三类角色配合完成：

```text
用户看到的页面
    负责展示聊天、商品、购物车、订单、账号等界面

APP 内部公共能力
    负责保存登录态、保存聊天记录、发网络请求、加载图片、跳转页面、语音识别

后端服务
    负责账号、商品、购物车、订单、优惠券、AI Agent、附件上传、语音识别等真实业务
```

一句话概括整体运行流程：

```text
用户操作页面
  -> 页面调用公共能力
  -> 公共能力请求后端或本地数据库
  -> 后端/本地返回数据
  -> 页面刷新 UI
  -> 用户继续下一步操作
```

## 二、系统整体运行流程

### 1. 启动 APP

用户点击手机桌面图标后，Android 系统先启动 `app.MainActivity`。

不过现在 `MainActivity` 不再负责具体页面，它只做一件事：把用户转到真正的首页 `chat.ChatActivity`。

可以理解成：

```text
MainActivity = 商场大门
ChatActivity = 用户真正进入后的导购大厅
```

启动流程是：

```text
用户点击 APP
  -> Android 启动 app.MainActivity
  -> MainActivity 立刻打开 chat.ChatActivity
  -> MainActivity 自己关闭
  -> 用户看到 AI 导购首页
```

### 2. 初始化公共能力

进入 `ChatActivity` 后，页面会通过 `BaseShopActivity` 拿到几个公共能力：

| 公共能力 | 通俗解释 |
| --- | --- |
| `SessionStore` | 记住用户有没有登录、昵称、头像、token、后端地址 |
| `LocalChatStore` | 在手机本地保存聊天会话和聊天消息 |
| `ApiClient` | 统一负责和后端打交道 |
| `ImageLoader` | 负责加载商品图、头像图 |

这些对象不需要每个页面都重复创建，而是由 `ShopApplication` 统一管理。

可以理解成：

```text
ShopApplication = APP 的总管家
BaseShopActivity = 每个页面找总管家的统一入口
```

### 3. 用户在不同页面之间流转

现在 APP 已经拆成了多个 Activity，每个核心页面都有自己的 Activity。

比如：

```text
AI 导购页 -> ChatActivity
商品列表页 -> ProductListActivity
商品详情页 -> ProductDetailActivity
购物车页 -> CartActivity
结算页 -> CheckoutActivity
订单页 -> OrderListActivity
登录页 -> LoginActivity
账号页 -> AccountActivity
设置页 -> SettingsActivity
```

页面跳转时，有两种常见方式：

1. 页面直接使用 `Intent` 打开另一个 Activity。
2. 通过 `NavigationHelper` 根据路由名称打开对应 Activity。

比如用户在聊天页点击“商品”，大致流程是：

```text
用户点击商品入口
  -> ChatActivity 发起跳转
  -> 打开 ProductListActivity
  -> 商品页调用 ApiClient 请求商品列表
  -> 后端返回商品数据
  -> 商品页展示商品卡片
```

### 4. 本地数据和后端数据如何配合

APP 里有两类数据来源：

```text
本地数据
    登录态、聊天历史、本地会话缓存

后端数据
    商品、购物车、订单、优惠券、AI 回复、附件、语音识别
```

本地数据的作用是让页面打开更快，也能保留历史记录。后端数据的作用是保证商品、价格、库存、订单、AI 回答都是真实的。

比如聊天记录：

```text
用户发送消息
  -> 先保存到手机本地数据库
  -> 再请求后端 Agent
  -> AI 回复流式返回
  -> 回复内容继续保存到本地数据库
  -> 下次打开会话时，从本地数据库恢复
```

### 5. 一条完整购物链路

从用户角度看，一条完整链路是：

```text
打开 APP
  -> 登录
  -> 问 AI 想买什么
  -> 查看 AI 推荐
  -> 进入商品详情
  -> 加入购物车
  -> 去购物车
  -> 勾选商品
  -> 结算
  -> 提交订单
  -> 支付/确认收货
  -> 评价商品
```

从系统角度看，同一条链路是：

```text
ChatActivity
  -> ApiClient 请求 Agent SSE
  -> AgentMessageRenderer 渲染商品卡
  -> ProductDetailActivity 展示详情
  -> ApiClient.addCartItem 加入购物车
  -> CartActivity 管理购物车
  -> CheckoutActivity 确认订单
  -> ApiClient.checkout 创建订单
  -> OrderListActivity 管理订单状态
```


## 三、每个模块的运行流程

### 1. `app` 模块：APP 入口和全局管家

包含：

- `MainActivity`
- `ShopApplication`

它的运行流程：

```text
Android 系统启动 APP
  -> MainActivity 被打开
  -> MainActivity 跳转到 ChatActivity
  -> ShopApplication 在 APP 进程里保存公共对象
```

通俗理解：

- `MainActivity` 是入口，不做复杂业务。
- `ShopApplication` 是全局管家，统一保存网络客户端、本地数据库、登录态和图片加载器。

### 2. `base` 模块：所有页面的公共底座

包含：

- `BaseShopActivity`

它的运行流程：

```text
某个页面启动
  -> 继承 BaseShopActivity
  -> 设置状态栏/导航栏/键盘适配
  -> 通过 BaseShopActivity 获取 api、sessionStore、localChatStore、imageLoader
```

通俗理解：

每个页面都需要一些通用能力，比如发请求、看登录态、弹 Toast、适配系统栏。`BaseShopActivity` 把这些公共能力统一放好，避免每个页面重复写。

### 3. `chat` 模块：AI 导购聊天主流程

包含：

- `ChatActivity`
- `AgentStreamController`
- `AgentMessageRenderer`
- `ChatAttachmentController`
- `ChatHistoryDrawer`
- `ChatMarkdownRenderer`

它的运行流程：

```text
用户进入聊天页
  -> ChatActivity 渲染欢迎语、推荐问题、输入框
  -> 用户输入文字/语音/附件
  -> ChatActivity 保存用户消息到本地
  -> 已登录时交给 AgentStreamController 请求后端 Agent
  -> 后端通过 SSE 流式返回思考过程和正式回答
  -> ChatActivity 实时更新聊天列表
  -> AgentMessageRenderer 渲染商品卡、订单卡、优惠券卡等结构化内容
  -> 回复结束后保存 assistant 消息到本地
```

几个关键角色：

| 类 | 通俗解释 |
| --- | --- |
| `ChatActivity` | 聊天大厅，负责输入、消息列表、侧栏、整体页面状态 |
| `AgentStreamController` | 专门管 AI 流式请求，负责开始、取消、接收 SSE |
| `AgentMessageRenderer` | 把 AI 返回的商品卡、表格、订单卡等画出来 |
| `ChatAttachmentController` | 管图片和文件附件 |
| `ChatHistoryDrawer` | 管左侧历史会话列表 |
| `ChatMarkdownRenderer` | 管 Markdown 文本和表格显示 |

AI 思考过程的运行方式：

```text
后端返回 thinking/status 事件
  -> APP 展开思考面板
  -> 显示“分析用户需求”
  -> 显示“查询买手团经验”
  -> 显示“总结答案”
  -> 正式回答开始后，思考面板自动收起
  -> 用户可以手动展开查看
```

### 4. `product` 模块：商品列表和商品详情

包含：

- `ProductListActivity`
- `ProductDetailActivity`

商品列表运行流程：

```text
用户进入商品页
  -> ProductListActivity 渲染搜索框、分类、商品列表
  -> 调用 ApiClient.productsPage 请求商品
  -> 后端返回商品列表
  -> 页面展示商品卡片
  -> 用户可以搜索、筛选、加载更多、切换活动 Tab
```

商品详情运行流程：

```text
用户点击商品详情
  -> ProductDetailActivity 接收 productId
  -> 请求商品详情和 SKU
  -> 请求商品评价
  -> 展示图片、价格、卖点、参数、评价
  -> 用户点击加入购物车
  -> 调用 ApiClient.addCartItem
```

通俗理解：

`ProductListActivity` 像商场货架，负责让用户找商品。`ProductDetailActivity` 像商品说明页，负责让用户看清楚并决定要不要加入购物车。

### 5. `cart` 模块：购物车和结算

包含：

- `CartActivity`
- `CheckoutActivity`
- `CheckoutDraftStore`

购物车运行流程：

```text
用户进入购物车
  -> CartActivity 调用 ApiClient.cart
  -> 后端返回购物车商品和汇总金额
  -> 页面展示商品、数量、勾选框、合计金额
  -> 用户修改数量/删除/勾选
  -> CartActivity 调用 updateCartItem 或 deleteCartItem
  -> 后端返回新的购物车
  -> 页面刷新
```

结算运行流程：

```text
用户点击结算
  -> CartActivity 检查是否有已选商品
  -> 把当前购物车快照放入 CheckoutDraftStore
  -> 打开 CheckoutActivity
  -> CheckoutActivity 展示商品清单
  -> 调用 discountPreview 计算优惠
  -> 展示商品总额、优惠金额、应付金额
  -> 用户点击提交订单
  -> 调用 ApiClient.checkout
  -> 下单成功后跳转订单页
```

为什么需要 `CheckoutDraftStore`：

结算页需要用到购物车快照。如果直接通过 Intent 传一大段 JSON，容易不稳定。`CheckoutDraftStore` 就像一个临时柜台，购物车先把数据放进去，结算页再凭编号取出来。

### 6. `order` 模块：订单管理

包含：

- `OrderListActivity`

它的运行流程：

```text
用户进入订单页
  -> OrderListActivity 调用 ApiClient.orders
  -> 后端返回订单列表
  -> 页面展示订单状态、金额、商品
  -> 用户点击支付/取消/确认收货/评价
  -> 页面调用对应后端接口
  -> 操作成功后刷新订单列表
```

订单页支持：

- 支付订单
- 取消订单
- 确认收货
- 商品评价

通俗理解：

`OrderListActivity` 是“我的订单”页面，负责用户购买之后的后续动作。

### 7. `coupon` 模块：优惠券

包含：

- `CouponActivity`

它的运行流程：

```text
用户进入优惠券页
  -> CouponActivity 同时请求可领优惠券和我的优惠券
  -> 页面分区展示
  -> 用户点击领取
  -> 调用 ApiClient.claimCoupon
  -> 领取成功后刷新列表
```

通俗理解：

优惠券模块负责“有哪些券可以领、我已经有哪些券”。结算时后端会根据购物车和优惠规则计算最终优惠。

### 8. `account` 模块：登录、账号、头像、资料

包含：

- `LoginActivity`
- `AccountActivity`
- `EditProfileActivity`
- `AvatarPreviewActivity`
- `AccountSessionHelper`
- `AccountUi`

登录运行流程：

```text
用户进入登录页
  -> 输入账号密码
  -> LoginActivity 调用 ApiClient.login
  -> 后端返回 token 和账号信息
  -> AccountSessionHelper 保存账号信息到 SessionStore
  -> 登录成功后回到聊天页
```

注册运行流程：

```text
用户输入注册信息
  -> LoginActivity 调用 ApiClient.register
  -> 后端创建账号
  -> 注册成功后保存登录态
  -> 回到聊天页
```

账号管理运行流程：

```text
用户进入账号管理
  -> AccountActivity 展示账号、手机号、邮箱等信息
  -> 用户修改联系方式
  -> 调用 ApiClient.updateContact
  -> 成功后更新本地 SessionStore
```

个人资料运行流程：

```text
用户进入个人资料
  -> EditProfileActivity 展示昵称和头像
  -> 用户选择头像
  -> 本地裁剪成正方形
  -> 上传头像
  -> 保存昵称和头像 URL
```

通俗理解：

`account` 模块管理“我是谁”。登录态一旦保存，购物车、订单、AI 会话这些需要身份的功能才能正常使用。

### 9. `settings` 模块：我的、帮助、关于、高级设置

包含：

- `SettingsActivity`
- `AdvancedSettingsActivity`
- `HelpActivity`
- `AboutActivity`

设置页运行流程：

```text
用户进入我的/设置页
  -> SettingsActivity 展示头像、昵称、账号入口
  -> 用户可以进入个人资料、账号管理、帮助、关于、高级设置
  -> 用户也可以退出登录
```

高级设置运行流程：

```text
用户进入高级设置
  -> AdvancedSettingsActivity 展示测试后端地址
  -> 用户修改后端地址
  -> 保存到 SessionStore
  -> refreshApiClient 让后续请求使用新地址
```

通俗理解：

`settings` 模块管理“APP 怎么用”和“调试配置”。普通用户主要使用“我的、帮助、关于”，开发测试时会用到高级设置。

### 10. `network` 模块：和后端通信

包含：

- `ApiClient`

它的运行流程：

```text
业务页面需要数据
  -> 调用 ApiClient 某个方法
  -> ApiClient 读取 SessionStore 里的 token 和 API 地址
  -> 拼接 HTTP 请求
  -> 发送到后端
  -> 解析 JSON
  -> 返回给业务页面
```

`ApiClient` 管的接口包括：

- 登录注册
- 账号资料
- 商品和分类
- 购物车
- 订单
- 优惠券和活动
- Agent 会话
- SSE 流式回答
- 附件上传

通俗理解：

`ApiClient` 就像 APP 的“外卖跑腿员”。页面想要数据，都让它去后端取。

### 11. `storage` 模块：本地存储

包含：

- `SessionStore`
- `LocalChatStore`

`SessionStore` 运行流程：

```text
登录成功
  -> 保存 token、昵称、头像等
APP 下次启动
  -> 从本地读取这些信息
  -> 判断用户是否登录
请求后端
  -> ApiClient 从 SessionStore 取 token
  -> 放到 Authorization 请求头
```

`LocalChatStore` 运行流程：

```text
用户发送消息
  -> 保存到 messages 表
AI 回复完成
  -> 保存 assistant 消息、思考过程、商品卡、追问
用户打开历史会话
  -> 从 sessions 和 messages 表读取
  -> 恢复聊天界面
```

通俗理解：

`SessionStore` 记住用户身份，`LocalChatStore` 记住聊天历史。

### 12. `ui` 模块：通用界面工具

包含：

- `ShopUi`
- `TopBarHelper`
- `BottomSheetHelper`
- `ImageLoader`
- `MarkdownRenderer`
- `CircleImageView`

它的运行流程：

```text
业务页面需要一个按钮/卡片/标题栏/弹窗/图片/Markdown
  -> 调用 ui 模块的工具方法
  -> 生成统一风格的 View
  -> 加到当前页面里
```

几个常用能力：

| 类 | 通俗解释 |
| --- | --- |
| `ShopUi` | 统一创建按钮、卡片、文字、价格样式 |
| `TopBarHelper` | 统一创建顶部栏和返回按钮 |
| `BottomSheetHelper` | 统一创建底部弹窗 |
| `ImageLoader` | 加载网络图片并缓存 |
| `MarkdownRenderer` | 把 Markdown 文本显示成富文本 |
| `CircleImageView` | 显示圆形头像 |

通俗理解：

`ui` 模块是 APP 的“装修队”。每个页面需要好看的控件，都从这里拿。

### 13. `navigation` 模块：页面跳转

包含：

- `Routes`
- `NavigationHelper`

它的运行流程：

```text
某个模块想跳转到一个业务页面
  -> 传入 route，比如 products、cart、orders
  -> NavigationHelper 把 route 翻译成 Activity 类名
  -> 创建 Intent
  -> startActivity 打开页面
```

比如：

```text
products -> .product.ProductListActivity
cart -> .cart.CartActivity
orders -> .order.OrderListActivity
account -> .account.AccountActivity
```

通俗理解：

`navigation` 模块像“商场导览牌”。你告诉它要去哪里，它负责带你到正确页面。

### 14. `voice` 模块：语音输入

包含：

- `VoiceInputController`
- `SpeechRealtimeClient`
- `PcmRecorder`

它的运行流程：

```text
用户点击麦克风
  -> ChatActivity 检查录音权限
  -> VoiceInputController 决定使用哪种语音模式
  -> 如果走实时识别，PcmRecorder 录音
  -> SpeechRealtimeClient 通过 WebSocket 发音频
  -> 后端返回识别文字
  -> ChatActivity 把文字填入输入框
```

如果实时语音不可用，会走 Android 系统语音识别兜底。

通俗理解：

`voice` 模块负责把用户说的话变成文字，真正发送给 AI 的仍然是文字内容。

## 四、几个核心业务流程串起来看

### 1. 用户问 AI 买什么

```text
用户输入问题
  -> ChatActivity 保存用户消息
  -> AgentStreamController 请求后端 AI
  -> ApiClient 建立 SSE 流
  -> 后端实时返回思考过程和回答
  -> ChatActivity 更新聊天页面
  -> AgentMessageRenderer 渲染商品卡
  -> LocalChatStore 保存回复
```

### 2. 用户把 AI 推荐商品加入购物车

```text
用户点击商品卡的加入购物车
  -> ChatActivity 调用 ApiClient.addCartItem
  -> 后端更新购物车
  -> APP 显示加入成功
  -> 用户可以进入 CartActivity 查看购物车
```

### 3. 用户从商品页购买

```text
用户进入商品页
  -> ProductListActivity 加载商品列表
  -> 用户进入 ProductDetailActivity
  -> 查看详情和评价
  -> 点击加入购物车
  -> 进入 CartActivity
  -> 勾选商品并结算
  -> CheckoutActivity 提交订单
  -> OrderListActivity 查看订单
```

### 4. 用户查看历史导购记录

```text
用户打开聊天侧栏
  -> ChatHistoryDrawer 加载本地历史
  -> 登录后同步远端历史
  -> 用户点击某条历史
  -> ChatActivity 从 LocalChatStore 恢复消息
  -> 页面重新显示文字、商品卡、追问和思考过程
```

### 5. 用户用语音提问

```text
用户点击麦克风
  -> voice 模块录音并识别
  -> 识别结果回到输入框
  -> 用户点击发送
  -> 后续流程和文字提问一致
```

## 五、总结：拆分后的运行逻辑更清楚了

现在的 Android APP 已经从“一个大页面包办所有事情”，变成了“不同模块各司其职”。

可以用一个商场来理解：

```text
app
    负责开门和准备总管家

chat
    是导购大厅，负责 AI 咨询和推荐

product
    是商品货架和商品详情

cart
    是购物车和收银台前的确认页

order
    是订单服务台

coupon
    是优惠券中心

account
    是会员中心

settings
    是 APP 设置和帮助中心

network
    是和后端沟通的电话线

storage
    是本地记事本和档案柜

ui
    是统一装修和组件库

navigation
    是导览牌

voice
    是语音输入助手
```

整体上，用户每做一个动作，页面模块负责接住这个动作；需要数据时，通过 `ApiClient` 找后端；需要保存时，通过 `SessionStore` 或 `LocalChatStore` 存到本地；需要跳页面时，通过 Intent 或 `NavigationHelper` 打开对应 Activity。

这种结构的好处是：

- 找代码更容易：商品问题看 `product`，购物车问题看 `cart`，账号问题看 `account`。
- 页面职责更清楚：每个 Activity 负责一个主要页面。
- 公共能力更统一：网络、存储、UI、语音、导航都有固定位置。
- 后续继续优化更方便：例如以后可以继续拆 `ChatActivity`、拆 `ApiClient`，而不会影响所有页面。

一句话总结：

```text
用户看到的是一个完整的 AI 导购购物 APP；
代码内部则是多个模块配合完成“问导购 -> 看商品 -> 加购物车 -> 下单 -> 订单评价”的完整闭环。
```
