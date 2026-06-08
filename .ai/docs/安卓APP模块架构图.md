# 安卓 APP 模块架构图

更新时间：2026-06-08

代码范围：`/Users/leo/xzxg-shop/android-native`

## 1. 当前架构定位

`android-native` 是小猪小狗电商 AI 导购系统的 Java 原生 Android 客户端。当前已经从早期的“所有页面集中在 `MainActivity`”拆分为“多 Activity + 按模块分包”的结构。

当前 Android 端的核心特点：

- 入口层由 `app.MainActivity` 负责转发到聊天主页。
- 业务页面已经按模块拆成独立 Activity，例如聊天、商品、购物车、结算、订单、账号、设置等。
- 公共能力按职责进入 `network`、`storage`、`ui`、`navigation`、`voice` 等共享包。
- 页面仍然采用 Java 手写 Android View，不使用 XML layout、Fragment、Jetpack Compose 或 Navigation Component。
- 网络层仍由 `ApiClient` 统一封装 REST、附件上传和 Agent SSE。
- 本地存储仍由 `SessionStore` 和 `LocalChatStore` 负责。

## 2. Android 工程结构

```mermaid
flowchart TB
    Repo[xzxg-shop]
    Repo --> Native[android-native]
    Native --> App[app Android module]
    App --> Manifest[AndroidManifest.xml]
    App --> Java[Java source<br/>com.xzxg.shop.*]
    App --> Res[res<br/>styles / anim / mipmap]

    Java --> AppPkg[app]
    Java --> BasePkg[base]
    Java --> ChatPkg[chat]
    Java --> ProductPkg[product]
    Java --> CartPkg[cart]
    Java --> AccountPkg[account]
    Java --> SettingsPkg[settings]
    Java --> OrderPkg[order]
    Java --> CouponPkg[coupon]
    Java --> NetworkPkg[network]
    Java --> StoragePkg[storage]
    Java --> UiPkg[ui]
    Java --> NavPkg[navigation]
    Java --> VoicePkg[voice]
```

## 3. 分包结构

当前 Java 源码目录：

```text
android-native/app/src/main/java/com/xzxg/shop
├── account
│   ├── AccountActivity.java
│   ├── AccountSessionHelper.java
│   ├── AccountUi.java
│   ├── AvatarPreviewActivity.java
│   ├── EditProfileActivity.java
│   └── LoginActivity.java
├── app
│   ├── MainActivity.java
│   └── ShopApplication.java
├── base
│   └── BaseShopActivity.java
├── cart
│   ├── CartActivity.java
│   ├── CheckoutActivity.java
│   └── CheckoutDraftStore.java
├── chat
│   ├── AgentMessageRenderer.java
│   ├── AgentStreamController.java
│   ├── ChatActivity.java
│   ├── ChatAttachmentController.java
│   ├── ChatHistoryDrawer.java
│   └── ChatMarkdownRenderer.java
├── coupon
│   └── CouponActivity.java
├── navigation
│   ├── NavigationHelper.java
│   └── Routes.java
├── network
│   └── ApiClient.java
├── order
│   └── OrderListActivity.java
├── product
│   ├── ProductDetailActivity.java
│   └── ProductListActivity.java
├── settings
│   ├── AboutActivity.java
│   ├── AdvancedSettingsActivity.java
│   ├── HelpActivity.java
│   └── SettingsActivity.java
├── storage
│   ├── LocalChatStore.java
│   └── SessionStore.java
├── ui
│   ├── BottomSheetHelper.java
│   ├── CircleImageView.java
│   ├── ImageLoader.java
│   ├── MarkdownRenderer.java
│   ├── ShopUi.java
│   └── TopBarHelper.java
└── voice
    ├── PcmRecorder.java
    ├── SpeechRealtimeClient.java
    └── VoiceInputController.java
```

## 4. 模块职责

| 包 | 主要职责 | 关键类 |
| --- | --- | --- |
| `app` | 应用入口、全局单例对象 | `MainActivity`、`ShopApplication` |
| `base` | Activity 公共基类、系统栏、Toast、公共依赖入口 | `BaseShopActivity` |
| `chat` | AI 导购聊天、SSE、思考过程、附件、历史会话、Agent 内容渲染 | `ChatActivity`、`AgentStreamController`、`AgentMessageRenderer` |
| `product` | 商品列表、搜索、分类、活动页、商品详情、SKU、评价、加购 | `ProductListActivity`、`ProductDetailActivity` |
| `cart` | 购物车、勾选、数量修改、删除、结算确认、下单草稿 | `CartActivity`、`CheckoutActivity`、`CheckoutDraftStore` |
| `order` | 订单列表、支付、取消、确认收货、评价 | `OrderListActivity` |
| `coupon` | 可领优惠券、我的优惠券、领取优惠券 | `CouponActivity` |
| `account` | 登录注册、账号管理、个人资料、头像预览和编辑 | `LoginActivity`、`AccountActivity`、`EditProfileActivity` |
| `settings` | 我的、帮助、关于、高级设置、退出登录 | `SettingsActivity`、`AdvancedSettingsActivity` |
| `network` | REST、SSE、附件上传、接口异常封装 | `ApiClient` |
| `storage` | 登录态、本地聊天会话和消息缓存 | `SessionStore`、`LocalChatStore` |
| `ui` | 通用 UI 组件、图片加载、Markdown 渲染、底部弹窗、顶部栏 | `ShopUi`、`TopBarHelper`、`BottomSheetHelper` |
| `navigation` | 路由常量和跨 Activity 跳转适配 | `Routes`、`NavigationHelper` |
| `voice` | 语音输入、讯飞实时语音、Android 系统语音兜底 | `VoiceInputController`、`SpeechRealtimeClient`、`PcmRecorder` |

## 5. Activity 架构图

```mermaid
flowchart TB
    Android[Android Launcher] --> Main[app.MainActivity]
    Main --> Chat[chat.ChatActivity<br/>AI 导购首页]

    Chat --> ProductList[product.ProductListActivity]
    Chat --> Cart[cart.CartActivity]
    Chat --> Orders[order.OrderListActivity]
    Chat --> Settings[settings.SettingsActivity]
    Chat --> Account[account.AccountActivity]
    Chat --> Login[account.LoginActivity]
    Chat --> Coupon[coupon.CouponActivity]

    ProductList --> ProductDetail[product.ProductDetailActivity]
    ProductList --> Cart
    ProductDetail --> Login

    Cart --> Checkout[cart.CheckoutActivity]
    Cart --> ProductList
    Checkout --> Orders

    Settings --> Avatar[account.AvatarPreviewActivity]
    Settings --> EditProfile[account.EditProfileActivity]
    Settings --> Account
    Settings --> Help[settings.HelpActivity]
    Settings --> About[settings.AboutActivity]
    Settings --> Advanced[settings.AdvancedSettingsActivity]

    Account --> EditProfile
    Account --> Login
    EditProfile --> Login
    Advanced --> Login
```

## 6. 运行时分层

```mermaid
flowchart TB
    subgraph UI[Presentation Layer]
        ChatActivity[ChatActivity]
        ProductActivities[ProductListActivity / ProductDetailActivity]
        CartActivities[CartActivity / CheckoutActivity]
        AccountActivities[Account / Login / Profile]
        SettingsActivities[Settings / Help / About]
        OrderActivity[OrderListActivity]
    end

    subgraph Shared[Shared App Services]
        Base[BaseShopActivity]
        App[ShopApplication]
        Nav[NavigationHelper / Routes]
        Ui[ShopUi / TopBarHelper / BottomSheetHelper]
        Voice[VoiceInputController]
    end

    subgraph Data[Data Adapters]
        API[ApiClient]
        Session[SessionStore]
        DB[LocalChatStore]
        Image[ImageLoader]
    end

    Backend[Go Backend<br/>Auth / Agent / Product / Cart / Order / Coupon / Files / Speech]

    UI --> Base
    Base --> App
    App --> API
    App --> Session
    App --> DB
    App --> Image
    UI --> Nav
    UI --> Ui
    ChatActivity --> Voice
    API --> Session
    API --> Backend
    Voice --> Backend
    DB --> UI
```

说明：

- `BaseShopActivity` 暴露 `sessionStore()`、`localChatStore()`、`api()`、`imageLoader()` 等公共入口，业务 Activity 不直接创建这些对象。
- `ShopApplication` 持有进程级单例，避免各页面重复初始化网络、存储和图片加载器。
- `NavigationHelper` 使用分包后的完整 Activity 路径，例如 `.chat.ChatActivity`、`.product.ProductListActivity`。
- `ApiClient` 仍是统一网络适配器，业务模块通过它访问后端。

## 7. Manifest 注册

`AndroidManifest.xml` 已按分包后的类名注册：

```text
application: .app.ShopApplication
launcher:    .app.MainActivity

activities:
.chat.ChatActivity
.product.ProductListActivity
.product.ProductDetailActivity
.cart.CartActivity
.cart.CheckoutActivity
.order.OrderListActivity
.coupon.CouponActivity
.account.LoginActivity
.account.AccountActivity
.account.EditProfileActivity
.account.AvatarPreviewActivity
.settings.SettingsActivity
.settings.AdvancedSettingsActivity
.settings.HelpActivity
.settings.AboutActivity
```

权限：

- `INTERNET`：访问后端 API、图片、SSE 和语音服务。
- `RECORD_AUDIO`：实时语音输入和系统语音识别。

## 8. 启动流程

```mermaid
sequenceDiagram
    participant Android as Android System
    participant Main as app.MainActivity
    participant Chat as chat.ChatActivity
    participant Base as BaseShopActivity
    participant App as ShopApplication
    participant API as ApiClient
    participant Store as LocalChatStore

    Android->>Main: 启动 launcher Activity
    Main->>Chat: startActivity(ChatActivity)
    Main->>Main: finish + no animation
    Chat->>Base: onCreate / configureShopSystemBars
    Chat->>App: 获取 SessionStore / LocalChatStore / ApiClient / ImageLoader
    Chat->>Store: 创建新的本地会话
    Chat->>Chat: 渲染聊天首页
    Chat->>API: 异步加载首页推荐和校验登录态
```

启动后默认进入 `ChatActivity`，这是用户端的主工作台。`MainActivity` 只保留入口转发职责。

## 9. AI 导购聊天流程

```mermaid
sequenceDiagram
    participant User as 用户
    participant Chat as ChatActivity
    participant Attach as ChatAttachmentController
    participant Stream as AgentStreamController
    participant API as ApiClient
    participant DB as LocalChatStore
    participant BE as Backend Agent

    User->>Chat: 输入文字/选择附件/语音输入
    Chat->>Attach: 管理待发送附件
    Chat->>DB: 保存用户消息 pending

    alt 未登录
        Chat->>Chat: 本地提示请先登录
        Chat->>DB: 保存本地 assistant 提示
    else 已登录
        opt 有附件
            Chat->>API: uploadAttachment
            API->>BE: POST /files
            BE-->>API: attachment JSON
        end
        Chat->>Stream: start
        Stream->>API: createSession 如需要
        API->>BE: POST /agent/sessions
        Stream->>API: streamMessage
        API->>BE: messages:stream
        BE-->>API: SSE status / thinking / text / block / followups
        API-->>Stream: callback
        Stream-->>Chat: onEvent
        Chat->>Chat: 渲染思考过程、正文、商品卡、追问
        Chat->>DB: 保存 assistant segments
    end
```

关键类分工：

- `ChatActivity`：页面状态、输入栏、消息列表、思考面板、历史回放、跨页面导航。
- `AgentStreamController`：Agent 会话创建、SSE 流启动、取消、流式生命周期回调。
- `AgentMessageRenderer`：商品卡、购物车状态、订单摘要、优惠券、评价摘要、政策卡片等结构化内容。
- `ChatMarkdownRenderer` / `MarkdownRenderer`：Markdown、表格、文本片段渲染。
- `ChatAttachmentController`：图片和文件附件选择、预览、上传状态管理。
- `ChatHistoryDrawer`：历史会话分页、搜索、远端同步状态和滚动位置。

## 10. 思考过程渲染流程

```mermaid
flowchart TB
    SSE[SSE Event] --> Dispatch[ChatActivity.handleSse]
    Dispatch --> Status[status / thinking_status]
    Dispatch --> Thinking[thinking_delta]
    Dispatch --> Text[text_delta]
    Dispatch --> Block[content_delta / block_delta]

    Status --> Panel[ensureThinkingView]
    Thinking --> Panel
    Panel --> Step1[分析用户需求]
    Panel --> Step2[查询买手团经验]
    Panel --> Step3[总结答案]

    Text --> Begin[beginAssistantAnswer]
    Block --> Begin
    Begin --> Collapse[自动收起思考面板<br/>带高度和透明度动画]
    Collapse --> Answer[渲染正式回答]
```

当前思考面板支持：

- 后端流式输出 `分析用户需求`、`查询买手团经验`、`总结答案`。
- 用户手动展开/收起，带动画。
- AI 正式回答开始后自动收起，自动收起也带动画。
- 历史消息回放时从 `segments_json` 恢复思考过程。

## 11. 商品和购物车流程

```mermaid
flowchart TB
    ChatCard[聊天商品卡] --> Detail[ProductDetailActivity]
    ChatCard --> AddCart[ApiClient.addCartItem]
    ProductList[ProductListActivity] --> Search[关键词搜索]
    ProductList --> Category[分类筛选]
    ProductList --> Promotion[活动 Tab]
    ProductList --> Detail
    ProductList --> FloatingCart[购物车悬浮入口]
    Detail --> AddCart
    FloatingCart --> Cart[CartActivity]
    Cart --> Select[勾选 / 全选]
    Cart --> Qty[修改数量]
    Cart --> Delete[删除商品]
    Cart --> Checkout[CheckoutActivity]
    Checkout --> Discount[discountPreview]
    Checkout --> Submit[checkout]
    Submit --> Orders[OrderListActivity]
```

分工：

- `ProductListActivity`：商品列表、分页加载、关键词搜索、分类选择、活动页、购物车悬浮数量角标。
- `ProductDetailActivity`：商品主图、价格、SKU、卖点、参数、评价、加入购物车。
- `CartActivity`：购物车列表、选择、数量变更、删除、合计金额、进入结算。
- `CheckoutActivity`：商品清单、金额明细、优惠试算、提交订单。
- `CheckoutDraftStore`：结算页短期内存草稿，避免通过 Intent 传递大 JSON。

## 12. 订单、优惠券和账号流程

```mermaid
flowchart TB
    Orders[OrderListActivity] --> Pay[支付]
    Orders --> Cancel[取消订单]
    Orders --> Confirm[确认收货]
    Orders --> Review[商品评价]

    Coupon[CouponActivity] --> Available[可领优惠券]
    Coupon --> Mine[我的优惠券]
    Available --> Claim[领取]

    Login[LoginActivity] --> Register[注册]
    Login --> SignIn[登录]
    Account[AccountActivity] --> Contact[手机号/邮箱]
    Account --> Logout[退出登录]
    Account --> Delete[删除账号]
    Settings[SettingsActivity] --> Profile[EditProfileActivity]
    Profile --> Avatar[头像裁剪/上传]
```

说明：

- 订单模块只放 `OrderListActivity`，订单状态操作都通过 `ApiClient` 调后端。
- 优惠券从账号或聊天路由进入 `CouponActivity`。
- 账号资料相关页面统一在 `account` 包内，设置页只做入口和偏好设置。

## 13. 语音输入流程

```mermaid
flowchart TB
    Mic[点击麦克风] --> Permission{RECORD_AUDIO 权限}
    Permission -- 未授权 --> Request[请求权限]
    Permission -- 已授权 --> Mode{SpeechMode}

    Mode -- AUTO / XUNFEI_REALTIME --> Realtime[VoiceInputController.startRealtimeSpeech]
    Realtime --> Recorder[PcmRecorder<br/>AudioRecord 16k PCM]
    Realtime --> WS[SpeechRealtimeClient<br/>WebSocket]
    Recorder --> WS
    WS --> Partial[partial 文本]
    WS --> Final[final 文本]
    Partial --> Input[更新输入框]
    Final --> Input

    Mode -- ANDROID_INLINE --> AndroidSpeech[SpeechRecognizer]
    Mode -- ANDROID_ACTIVITY --> RecognizerIntent[系统语音 Activity]
    AndroidSpeech --> Input
    RecognizerIntent --> Input
```

`voice` 包只负责语音识别能力，识别结果回填输入框仍由 `ChatActivity` 控制。

## 14. 本地存储

```mermaid
erDiagram
    sessions {
        TEXT local_session_id PK
        TEXT server_session_id
        TEXT title
        TEXT summary
        TEXT sync_state
        INTEGER created_at
        INTEGER updated_at
        INTEGER pinned_at
        INTEGER deleted_at
    }

    messages {
        TEXT local_message_id PK
        TEXT local_session_id
        TEXT role
        TEXT content
        TEXT attachments_json
        TEXT blocks_json
        TEXT followups_json
        TEXT segments_json
        TEXT status
        INTEGER created_at
    }

    sessions ||--o{ messages : local_session_id
```

`SessionStore` 保存：

- `token`
- `role`
- `nickname`
- `avatar_url`
- `account_id`
- `username`
- `phone`
- `email`
- 测试后端地址
- 侧栏是否显示“返回聊天”

`LocalChatStore` 保存：

- 本地会话和远端会话绑定关系。
- 用户消息、assistant 消息、附件、结构化块、追问、思考过程。
- 历史会话搜索、分页、置顶、重命名、软删除和同步状态。

## 15. 网络能力映射

```mermaid
flowchart TB
    Api[network.ApiClient]
    Api --> Auth[认证<br/>login / register / me / logout]
    Api --> Account[账号<br/>profile / contact / avatar / delete]
    Api --> Product[商品<br/>categories / products / skus / reviews]
    Api --> Cart[购物车<br/>cart / add / update / delete / discount]
    Api --> Order[订单<br/>checkout / pay / cancel / confirm / review]
    Api --> Coupon[优惠券和活动<br/>promotions / available / mine / claim]
    Api --> Agent[Agent 会话<br/>sessions / detail / update / pin / delete]
    Api --> Stream[Agent SSE<br/>messages:stream / cancel run]
    Api --> Files[附件<br/>multipart upload]
```

实现特点：

- REST 使用 `HttpURLConnection`。
- SSE 使用 `HttpURLConnection` 读取 `data:` 行并解析 JSON。
- 附件上传支持 byte array 和 `InputStream`。
- 语音 WebSocket 由 `SpeechRealtimeClient` 负责，不放在 `ApiClient` 内。

## 16. 当前架构优点和后续建议

已完成的改善：

- `MainActivity` 不再承载所有页面，入口职责被压缩到启动转发。
- 商品、购物车、结算、订单、账号、设置等页面都有独立 Activity。
- 共享能力已经按模块分包，文件定位更清晰。
- Manifest 和 `NavigationHelper` 已适配分包后的 Activity 路径。

仍需注意：

- `ChatActivity` 仍然较大，后续可以继续拆成 `ChatComposerView`、`ThinkingRenderer`、`ChatSessionController` 等更小单元。
- 当前仍是手写 View 架构，没有 ViewModel/Repository，异步状态主要靠 Activity 字段维护。
- `ApiClient` 仍聚合所有后端接口，后续可以按 `AuthApi`、`ProductApi`、`CartApi`、`AgentApi` 继续拆。
- 业务线程主要使用 `new Thread` + `runOnUiThread`，后续可统一封装任务执行器。

## 17. 验证记录

分包完成后已执行：

```text
bash ./gradlew :app:assembleDebug
```

结果：编译通过。

已安装并通过新入口启动：

```text
com.xzxg.shop/.app.MainActivity
```

日志检查未发现：

- `FATAL EXCEPTION`
- `ClassNotFoundException`
- `ActivityNotFoundException`
