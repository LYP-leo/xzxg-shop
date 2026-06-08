# 安卓 APP MainActivity 多 Activity 拆分计划 v1

日期：2026-06-07

目标代码：`/Users/leo/xzxg-shop/android-native`

关联文档：

- `.ai/docs/安卓APP模块架构图.md`
- `.ai/tech/安卓APP_代码检查_v1.md`
- `.ai/tech/编码规范_v1.md`

## 1. 背景

当前原生 Android 端采用单 `MainActivity` 手写 View 的实现方式，所有页面和大量业务流程集中在：

```text
android-native/app/src/main/java/com/xzxg/shop/MainActivity.java
```

当前 `MainActivity.java` 承担了以下职责：

- APP 启动、系统栏和安全区适配。
- 页面导航、顶部栏、返回键、左侧抽屉。
- AI 导购聊天首页、输入栏、附件、语音、Agent SSE 流式渲染。
- 本地会话历史、远程会话同步、置顶、重命名、删除。
- 商品列表、分类、搜索、商品详情、评价展示。
- 购物车、确认订单、优惠预览、提交订单。
- 订单列表、支付、取消、确认收货、评价。
- 优惠券、促销活动。
- 登录、注册、个人资料、头像、账号管理、设置、帮助、关于。
- 大量 UI 工具方法，如按钮、卡片、底部弹窗、Markdown 表格、复制、图片预览。

这种结构已经影响可维护性：

1. 页面边界不清晰，一个页面改动容易牵动其它页面。
2. 全局状态字段过多，异步请求返回时容易依赖已变化的页面状态。
3. UI 工具方法和业务流程混在一起，难以复用和测试。
4. 后续新增商家端、管理员端或更复杂买家端页面时，`MainActivity` 会继续膨胀。

本计划的目标是将当前 `MainActivity` 合理拆分为多个 Activity，并在拆分前抽出共享基础设施，降低一次性大重构风险。

## 2. 设计原则

### 2.1 先抽共享能力，再拆页面

如果直接把代码剪到多个 Activity，会产生大量复制：

- `dp`、`rounded`、`primaryButton`、`panel` 等 UI 工具会被复制。
- `configureSystemBars`、`applyContentInsets` 等安全区逻辑会被复制。
- `toastLine`、底部弹窗、图片预览、复制逻辑会被复制。
- `SessionStore`、`ApiClient`、`ImageLoader` 初始化会分散。

因此应先抽出共享层，再逐步迁移页面。

### 2.2 ChatActivity 优先保守拆分

AI 导购聊天链路最复杂，包含：

- 本地会话。
- 附件上传。
- 语音输入。
- SSE 读取和取消。
- 思考过程。
- Markdown、表格、商品卡、服务 block。
- 历史回放。

聊天页应先从 `MainActivity` 改名或迁移为 `ChatActivity`，保持功能稳定；之后再从 ChatActivity 内继续抽 `ChatStreamController`、`AgentMessageRenderer` 等类。不要在第一阶段同时把聊天页拆成多个 Activity。

### 2.3 页面 Activity 只负责页面组合

目标状态中，每个 Activity 应主要负责：

- 读取 Intent 参数。
- 初始化页面布局。
- 调用共享 Repository/API。
- 渲染成功、空态、错误态。
- 处理当前页面交互。

公共能力应下沉到：

- `BaseShopActivity`
- `ShopUi`
- `ShopNavigation`
- `BottomSheetHelper`
- `InsetsHelper`
- `AppServices`
- 按业务拆分的 renderer/controller/helper。

### 2.4 分阶段可回滚

每个阶段都应能独立编译、安装和手动冒烟。不要一次性移动 9000 行。

推荐策略：

1. 先新增类，不删除旧逻辑。
2. 新页面迁移完成后，让旧入口跳转到新 Activity。
3. 验证通过后，再删除 `MainActivity` 中对应旧方法。
4. 每个阶段都控制在可审查的变更范围内。

### 2.5 行为不变优先于架构纯度

本次拆分是架构重构，不是产品改版。拆分前后必须保持用户可见行为、后端请求、数据保存格式和页面跳转语义一致。

明确禁止在本次拆分中顺手改动：

1. 不改接口 URL、请求方法、请求体字段、响应解析字段和错误文案。
2. 不改默认账号、默认后端地址、debug 后端地址保存规则。
3. 不改登录成功后的跳转行为。
4. 不改未登录访问购物车、订单、优惠券等页面的展示方式。
5. 不改聊天 SSE 事件消费规则和本地 `segments_json`、`blocks_json`、`followups_json` 保存格式。
6. 不改附件大小限制、上传顺序、上传失败后的输入恢复逻辑。
7. 不改语音识别默认策略、超时、错误提示和“识别结果填入输入框而不是自动发送”的行为。
8. 不改商品列表缓存、分页、分类、详情返回滚动恢复语义。
9. 不改购物车结算时使用的购物车快照和优惠预览语义。
10. 不改订单支付、取消、确认收货、评价的触发条件。
11. 不改 `LocalChatStore` 表结构和现有数据库升级路径，除非单独开数据库迁移方案。
12. 不把安全配置、UI 重设计、接口升级、Activity Result API 升级混入本次拆分。

如果为了拆分必须改变某个行为，应先在本文档新增“行为变更说明”和“回滚方案”，并得到确认后再实现。默认情况下，任何行为差异都视为拆分失败。

## 3. 当前行为基线

拆分前必须以现有 `MainActivity` 行为为准，不能按“更合理”的新行为重写。

| 场景 | 当前行为 | 拆分后要求 |
| --- | --- | --- |
| APP 启动 | 进入聊天首页，创建新的本地会话，异步加载首页文案，异步校验 token | 保持一致 |
| 未登录发送聊天 | 本地保存用户消息，插入“请先登录”assistant 提示，不请求后端 Agent | 保持一致 |
| 登录成功 | 保存 token/account，异步同步远端会话，创建新本地会话，回聊天首页 | 保持一致 |
| 注册成功 | 保存 token/account，异步同步远端会话，创建新本地会话，回聊天首页 | 保持一致 |
| token 过期且在聊天页 | 清理登录态，取消活跃流，聊天页插入系统提示 | 保持一致 |
| token 过期且在购物车/订单/设置/账号页 | 清理登录态并进入登录页 | 保持一致 |
| 未登录打开购物车 | 页面内显示“请先登录”，不自动跳登录页 | 保持一致 |
| 未登录打开订单 | 页面内显示“请先登录”，不自动跳登录页 | 保持一致 |
| 未登录打开优惠券 | 页面内显示“请先登录”，不自动跳登录页 | 保持一致 |
| 未登录打开设置/账号/编辑资料 | 进入登录页 | 保持一致 |
| 未登录加入购物车 | toast “请先登录后再加入购物车”，进入登录页 | 保持一致 |
| 商品详情返回 | 从商品列表进详情返回商品列表并恢复滚动；从聊天商品卡进详情返回聊天 | 保持一致 |
| 购物车进入确认订单 | 使用当前 `activeCartSnapshot`，请求优惠预览，失败时仍进入确认订单 | 保持一致 |
| 提交订单成功 | toast 后进入订单页 | 保持一致 |
| 退出登录/删除账号 | 清理登录态，创建新本地会话，回聊天首页 | 保持一致 |
| 语音识别成功 | 将识别文本填入输入框并弹出键盘，不自动发送 | 保持一致 |
| 用户离开聊天页时 SSE 仍在进行 | 不因普通页面跳转主动取消流式请求；返回聊天应能看到已保存/继续的结果 | 保持一致 |

## 4. 目标 Activity 划分

### 4.1 一级页面 Activity

| 目标 Activity | 职责 | 当前来源 |
| --- | --- | --- |
| `ChatActivity` | AI 导购聊天首页、输入栏、SSE、附件、语音、本地消息回放 | `renderChatHome`、`sendMessage`、`handleSse`、语音和附件方法 |
| `ProductListActivity` | 商品列表、搜索、分类筛选、活动 Tab、购物车浮标 | `renderProducts`、`renderProductListTab`、`renderProductPromotionsTab` |
| `CartActivity` | 购物车商品、选择、数量、删除、底部结算栏 | `renderCart`、`loadCart`、`renderCartContent` |
| `OrderListActivity` | 订单列表、支付、取消、确认收货、评价入口 | `renderOrders`、`orderCard`、`addOrderActions` |
| `CouponActivity` | 可领取优惠券、我的优惠券、领取 | `renderCoupons` |
| `SettingsActivity` | 设置首页、头像入口、账号管理入口、帮助/关于/高级设置入口 | `renderSettings` |

### 4.2 二级页面 Activity

| 目标 Activity | 职责 | 当前来源 |
| --- | --- | --- |
| `LoginActivity` | 登录、注册、debug 后端地址入口 | `renderLoginPage`、`login`、`register` |
| `ProductDetailActivity` | 商品详情、SKU、评价、加入购物车 | `renderProductDetail`、`renderProductDetailContent` |
| `CheckoutActivity` | 确认订单、优惠明细、提交订单 | `renderCheckoutPage`、`checkoutCart` |
| `AccountActivity` | 账号管理、手机号、邮箱、退出登录、删除账号 | `renderAccountPage`、`editContact` |
| `EditProfileActivity` | 头像选择/裁剪、昵称编辑、保存资料 | `renderEditProfilePage`、`pickAvatar`、`saveProfileChanges` |
| `AvatarPreviewActivity` | 头像预览、长按保存、进入编辑资料 | `renderAvatarPreviewPage` |
| `AdvancedSettingsActivity` | 侧栏设置、debug API Base | `renderAdvancedSettingsPage` |
| `HelpActivity` | 帮助说明 | `renderHelpPage` |
| `AboutActivity` | 关于页面 | `renderAboutPage` |

### 4.3 可不拆成 Activity 的 UI

这些更适合保留为 Dialog、BottomSheet 或局部组件：

| 功能 | 建议形态 | 原因 |
| --- | --- | --- |
| 左侧历史抽屉 | `ChatHistoryDrawer` 或 `HistoryActivity` 二选一 | 目前与 Chat 会话强耦合。第一阶段建议保留为 ChatActivity 内组件，后续如要完整页面化再拆。 |
| 分类选择 | BottomSheet helper | 临时选择器，不需要独立页面。 |
| 订单商品选择评价 | BottomSheet helper | 临时选择器。 |
| 评价表单 | BottomSheet 或 `ReviewDialogController` | 和订单详情上下文绑定，独立 Activity 成本偏高。 |
| 图片预览 | `ImagePreviewDialog` 或 `ImagePreviewActivity` | 当前只是全屏 overlay。若需要系统返回栈、保存权限、缩放手势，可后续独立。 |

## 5. 目标包结构

当前所有 Java 类都在：

```text
com.xzxg.shop
```

拆分后建议先保持同一个 package，避免大量 import 和可见性调整；第二轮再按子包整理。如果本轮直接引入子包，建议如下：

```text
com.xzxg.shop
  AppServices.java
  ShopApplication.java

com.xzxg.shop.base
  BaseShopActivity.java
  InsetsHelper.java
  ShopUi.java
  BottomSheetHelper.java
  NavigationHelper.java
  CopyHelper.java

com.xzxg.shop.data
  ApiClient.java
  SessionStore.java
  LocalChatStore.java

com.xzxg.shop.media
  ImageLoader.java
  MarkdownRenderer.java
  SpeechRealtimeClient.java
  PcmRecorder.java

com.xzxg.shop.chat
  ChatActivity.java
  ChatHistoryDrawer.java
  ChatComposerController.java
  ChatAttachmentController.java
  VoiceInputController.java
  AgentStreamController.java
  AgentMessageRenderer.java
  MarkdownTableRenderer.java
  AgentBlockRenderer.java

com.xzxg.shop.product
  ProductListActivity.java
  ProductDetailActivity.java
  ProductUiRenderer.java

com.xzxg.shop.cart
  CartActivity.java
  CheckoutActivity.java

com.xzxg.shop.order
  OrderListActivity.java
  ReviewSheetController.java

com.xzxg.shop.account
  LoginActivity.java
  SettingsActivity.java
  AccountActivity.java
  EditProfileActivity.java
  AvatarPreviewActivity.java
  AdvancedSettingsActivity.java
  HelpActivity.java
  AboutActivity.java
```

实际落地建议：

- 第一阶段不要急着拆包，先新增 `BaseShopActivity`、`ShopUi`、`NavigationHelper`。
- 页面 Activity 拆出来后，再做包结构迁移。
- 如果文件数量快速变多，再按上述包结构整理。

## 6. 共享基础设施设计

### 6.1 `ShopApplication`

新增 Application 类，用于提供进程级共享对象。

职责：

- 初始化 `SessionStore`。
- 初始化 `ApiClient`。
- 初始化 `ImageLoader`。
- 提供 `refreshApiClient()`，当 debug API Base 修改后重建 `ApiClient`。

示例接口：

```java
public class ShopApplication extends Application {
    private SessionStore sessionStore;
    private ApiClient apiClient;
    private ImageLoader imageLoader;

    public SessionStore sessionStore();
    public ApiClient api();
    public ImageLoader imageLoader();
    public void refreshApiClient();
}
```

Manifest 增加：

```xml
<application
    android:name=".ShopApplication"
    ...>
</application>
```

注意：

- `LocalChatStore` 可以先按 Activity 创建，也可以由 Application 持有单例。考虑 SQLiteOpenHelper 自身线程安全和生命周期，本轮建议 Application 持有一个全局实例，避免多个 Activity 同时创建不同 helper 对象。
- `ApiClient` 依赖 `SessionStore`，debug 后端地址修改后必须重建或确保每次读 `sessionStore.apiBase()`。当前 `ApiClient` 每次请求已动态读取 `apiBase`，但保存后重建更直观。

### 6.2 `BaseShopActivity`

所有页面 Activity 继承 `BaseShopActivity`。

职责：

- 持有 `SessionStore`、`ApiClient`、`LocalChatStore`、`ImageLoader`。
- 统一 `configureSystemBars`。
- 统一 root/content 创建。
- 统一安全区处理。
- 统一 toast。
- 统一登录过期处理。
- 提供基础 UI 容器方法。

建议接口：

```java
abstract class BaseShopActivity extends Activity {
    protected SessionStore sessionStore;
    protected ApiClient api;
    protected LocalChatStore chatStore;
    protected ImageLoader imageLoader;
    protected FrameLayout root;
    protected LinearLayout content;

    protected void setupBaseScreen();
    protected LinearLayout pageBody();
    protected View createTopBar(String title);
    protected View createBackTopBar(String title);
    protected void toastLine(String text);
    protected boolean handleApiError(Throwable error);
    protected void handleAuthExpired();
    protected int dp(int value);
}
```

### 6.3 `ShopUi`

抽出纯 UI 工厂方法，减少 Activity 间复制。

候选方法：

- `rounded`
- `primaryButton`
- `secondaryButton`
- `textOnlyButton`
- `transparentIconButton`
- `strong`
- `muted`
- `title`
- `card`
- `panel`
- `inputField`
- `textFilterRow`
- `statusText`
- `ratingStars`
- `chipText`

注意：

- 这些方法依赖 `Context` 和 `dp`，可设计为静态方法：`ShopUi.primaryButton(Context, text)`。
- 如果静态方法参数过多，也可以让 `BaseShopActivity` 继续包装一层。

### 6.4 `NavigationHelper`

统一多 Activity 跳转，避免字符串 route 分散。

职责：

- 启动登录页。
- 启动聊天页。
- 启动商品列表页。
- 启动商品详情页并传 `product_id`。
- 启动购物车、确认订单、订单、优惠券、活动 Tab。
- 处理 Agent `navigation_action` route。

建议常量：

```java
public final class Routes {
    public static final String EXTRA_PRODUCT_ID = "product_id";
    public static final String EXTRA_BACK_TARGET = "back_target";
    public static final String EXTRA_PRODUCT_TAB = "product_tab";
    public static final String EXTRA_LOCAL_SESSION_ID = "local_session_id";
    public static final String EXTRA_SERVER_SESSION_ID = "server_session_id";
}
```

### 6.5 `BottomSheetHelper`

抽出：

- `makeBottomSheetBox`
- `showBottomSheetDialog`
- `showConfirmBottomSheet`

这样 Account、Product、Order 等页面都能复用底部弹窗，不再依赖 MainActivity。

### 6.6 `ActivityResult` 迁移策略

当前项目使用传统 `startActivityForResult` / `onActivityResult`，虽然可继续用，但多 Activity 后需要明确归属：

| requestCode | 当前用途 | 拆分后归属 |
| --- | --- | --- |
| `REQUEST_PICK_IMAGE` | 聊天附件图片 | `ChatActivity` 或 `ChatAttachmentController` |
| `REQUEST_PICK_FILE` | 聊天附件文件 | `ChatActivity` 或 `ChatAttachmentController` |
| `REQUEST_RECORD_AUDIO` | 录音权限 | `ChatActivity` 或 `VoiceInputController` |
| `REQUEST_PICK_AVATAR` | 头像选择 | `EditProfileActivity` |
| `REQUEST_SPEECH_INPUT` | 系统语音识别 Activity | `ChatActivity` 或 `VoiceInputController` |

第一轮可继续使用 requestCode；后续再迁移到 Activity Result API。

## 7. 页面拆分方案

### 7.1 Launcher 策略

有两种选择：

#### 方案 A：保留 `MainActivity` 作为壳

`MainActivity` 只负责启动 `ChatActivity`，然后 finish。

优点：

- Manifest 入口不用大改。
- ADB 启动命令和已有文档 `/.MainActivity` 仍可用。
- 回滚容易。

缺点：

- 多一层无业务壳 Activity。

#### 方案 B：直接让 `ChatActivity` 成为 launcher

优点：

- 命名更清晰。
- 删除 MainActivity 更彻底。

缺点：

- 需要同步所有启动命令和文档。
- 变更范围稍大。

建议本轮采用方案 A。完成稳定后，再决定是否将 launcher 切到 `ChatActivity` 并删除 `MainActivity`。

### 7.2 `ChatActivity`

迁移内容：

- `renderChatHome`
- `createChatMessageLayer`
- `createComposerBar`
- 附件选择/上传
- 语音输入
- `sendCurrentInput`
- `sendMessage`
- `ensureServerSessionThenStream`
- `stream`
- `handleSse`
- `finishStream` / `failStream` / `stopActiveStream`
- 思考过程渲染
- Markdown/table/block/followups 渲染
- `renderStoredMessagesIfAny`
- 左侧历史抽屉第一阶段保留在 ChatActivity

ChatActivity 的 Intent 参数：

| extra | 用途 |
| --- | --- |
| `local_session_id` | 打开指定本地会话 |
| `server_session_id` | 指定远端会话 ID |
| `new_session` | 是否强制创建新本地会话 |

拆分注意：

- 当前 `localSessionId`、`serverSessionId` 是 Chat 的核心状态，必须归属 ChatActivity。
- 其它页面不应直接修改 ChatActivity 的字段；需要通过 Intent 返回聊天页。
- 发送消息产生的本地会话应仍由 `LocalChatStore` 持久化。

### 7.3 `LoginActivity`

迁移内容：

- 登录 UI
- 注册 UI
- debug 后端地址输入
- `login`
- `register`
- `saveAccountSession`

返回结果策略：

- 登录成功后启动 `ChatActivity`，附带 `new_session=true`，并清理登录页栈。
- 不在本次拆分中新增 `return_route` 返回原页面能力，因为当前 `MainActivity` 登录/注册成功后统一创建新本地会话并回聊天首页。新增返回原页面会改变现有行为，应作为后续独立需求处理。

### 7.4 `ProductListActivity`

迁移内容：

- 商品列表页。
- 搜索框。
- 分类底部弹窗。
- 商品分页缓存。
- 活动 Tab。
- 购物车浮标。
- 加入购物车。

Intent 参数：

| extra | 用途 |
| --- | --- |
| `tab` | `list` 或 `activity` |
| `keyword` | 可选初始搜索词 |
| `category_id` | 可选分类 |
| `category_name` | 可选分类名 |

跨 Activity 状态：

- 当前 `productListCache` 是 MainActivity 字段。拆分后它属于 ProductListActivity。
- 从详情返回列表时，需要保持列表滚动和缓存。第一轮可依赖 Android 返回栈，让 ProductListActivity 不 finish，打开 ProductDetailActivity 后返回即可恢复原 Activity 实例。

### 7.5 `ProductDetailActivity`

迁移内容：

- 商品详情加载。
- SKU 加载。
- 商品评价加载。
- 加入购物车。
- 图片预览。

Intent 参数：

| extra | 用途 |
| --- | --- |
| `product_id` | 商品 ID，必填 |

返回策略：

- 系统返回直接回 ProductListActivity 或 ChatActivity。
- 从 Chat 商品卡进入详情时，返回 ChatActivity。
- 从 ProductListActivity 进入详情时，返回 ProductListActivity。
- 可通过普通 Android back stack 自然处理，不必自定义 `backAction`。

### 7.6 `CartActivity` 和 `CheckoutActivity`

`CartActivity` 迁移：

- 加载购物车。
- 渲染购物车项。
- 全选、选择、数量、删除。
- 底部结算栏。
- 进入确认订单。

`CheckoutActivity` 迁移：

- 接收购物车快照。
- 加载优惠预览。
- 展示确认订单。
- 提交订单。

建议：

- 当前 `MainActivity` 从购物车进入确认订单时使用 `activeCartSnapshot`，只额外请求 `api.discountPreview()`。拆分后必须保持这个语义，不能在 `CheckoutActivity` 启动后重新请求 cart 来替代快照，否则可能改变用户点击结算瞬间看到的商品清单。
- 不建议通过 Intent 直接传完整 cart JSON，可能触发 Binder 大小限制。建议新增进程内 `CheckoutDraftStore`，用 `checkout_draft_id` 在 `CartActivity` 和 `CheckoutActivity` 间交接当前 cart snapshot。
- 如果进程被系统杀死导致 `CheckoutDraftStore` 丢失，`CheckoutActivity` 应显示“订单信息已失效，请返回购物车重新结算”，而不是自动重新请求并继续结算。自动重取 cart 会改变当前行为。

### 7.7 `OrderListActivity`

迁移内容：

- 订单列表。
- 订单卡片。
- 支付、取消、确认收货。
- 评价入口和评价 BottomSheet。

建议：

- 评价表单先保留为 `ReviewSheetController`，不要拆成独立 Activity。
- 评价成功后刷新 OrderListActivity。

### 7.8 `CouponActivity`

迁移内容：

- 可领取优惠券。
- 我的优惠券。
- 领取优惠券。

该页依赖少，适合在第一批页面拆分中处理。

### 7.9 设置和账号 Activity

建议拆分顺序：

1. `SettingsActivity`
2. `AccountActivity`
3. `EditProfileActivity`
4. `AvatarPreviewActivity`
5. `AdvancedSettingsActivity`
6. `HelpActivity`
7. `AboutActivity`

注意：

- 头像选择结果只属于 `EditProfileActivity`。
- 修改头像/昵称后，返回 SettingsActivity 时应刷新 `SessionStore` 最新数据。
- 退出登录或删除账号后，应清理任务栈并回到 ChatActivity。

## 8. Shared Renderer/Controller 拆分

页面 Activity 拆出来以后，仍有一些大块逻辑不应继续堆在 ChatActivity 中。

### 8.1 `AgentMessageRenderer`

负责：

- assistant 文本气泡。
- 历史消息回放。
- Markdown 渲染。
- 表格渲染。
- 商品引用渲染。
- 结构化 block 渲染。
- followups。
- 长按复制。

从当前方法迁出：

- `appendAssistantText`
- `renderAgentBlock`
- `renderAgentSegments`
- `renderHistoricalBlock`
- `chatProductCard`
- `comparisonCard`
- `cartStateCard`
- `orderSummaryCard`
- `discountPreviewCard`
- `couponListCard`
- `navigationActionCard`
- `addMarkdownTableToBubble`
- `splitMarkdownParts`

### 8.2 `AgentStreamController`

负责：

- 创建远端会话。
- 发起 SSE。
- 取消 run。
- 维护 active run 状态。
- 将 SSE event 转给 renderer。
- 保存 assistant turn。

从当前方法迁出：

- `ensureServerSessionThenStream`
- `stream`
- `handleSse`
- `finishStream`
- `finishDuplicateStream`
- `failStream`
- `stopActiveStream`
- `finishCanceledStream`

### 8.3 `ChatAttachmentController`

负责：

- 附件 pending 列表。
- 图片/文件选择。
- 上传。
- 上传中气泡状态。
- 用户附件气泡。

从当前方法迁出：

- `toggleAttachmentPanel`
- `renderAttachmentPanel`
- `renderAttachmentBuffer`
- `attachmentFromUri`
- `sendCurrentInput` 中的附件上传部分。

### 8.4 `VoiceInputController`

负责：

- 权限检查结果处理。
- 实时 WebSocket 语音。
- Android SpeechRecognizer fallback。
- 录音生命周期。

从当前方法迁出：

- `enterVoiceMode`
- `startRealtimeSpeech`
- `startAndroidInlineSpeech`
- `startSpeechRecognizerActivity`
- `finishRealtimeVoice`
- `cleanupRealtimeVoice`
- `sendRealtimeSpeechFinal`
- `handleRealtimeSpeechError`
- `startSpeechRecognition`

### 8.5 `ChatHistoryDrawer`

负责：

- 抽屉 UI。
- 本地历史分页。
- 远端历史搜索。
- 置顶、重命名、删除。
- 点击历史会话通知 ChatActivity 切换会话。

第一阶段可以仍放在 ChatActivity，第二阶段再拆成独立类。

## 9. AndroidManifest 目标变更

最终 Manifest 应显式声明各 Activity。

示例：

```xml
<activity android:name=".chat.ChatActivity" android:windowSoftInputMode="adjustResize" />
<activity android:name=".account.LoginActivity" android:windowSoftInputMode="adjustResize" />
<activity android:name=".product.ProductListActivity" android:windowSoftInputMode="adjustNothing" />
<activity android:name=".product.ProductDetailActivity" android:windowSoftInputMode="adjustNothing" />
<activity android:name=".cart.CartActivity" android:windowSoftInputMode="adjustNothing" />
<activity android:name=".cart.CheckoutActivity" android:windowSoftInputMode="adjustNothing" />
<activity android:name=".order.OrderListActivity" android:windowSoftInputMode="adjustNothing" />
<activity android:name=".account.SettingsActivity" android:windowSoftInputMode="adjustNothing" />
```

如果保留 MainActivity 作为 launcher：

```xml
<activity
    android:name=".MainActivity"
    android:exported="true">
    <intent-filter>
        <action android:name="android.intent.action.MAIN" />
        <category android:name="android.intent.category.LAUNCHER" />
    </intent-filter>
</activity>
```

`MainActivity.onCreate`：

```java
startActivity(new Intent(this, ChatActivity.class));
finish();
```

## 10. 详细执行轮次

本次拆分建议共 8 轮完成：第 0 轮建立行为基线，第 1-7 轮逐步拆分代码。每一轮都必须独立编译、可安装、可回滚；如果某轮无法做到行为等价，应停止进入下一轮。

| 轮次 | 主题 | 主要产出 | 是否拆业务页面 |
| --- | --- | --- | --- |
| 第 0 轮 | 行为基线和安全网 | 冒烟清单、当前构建状态、`MainActivity` 方法归属表 | 否 |
| 第 1 轮 | 抽共享基础设施 | `ShopApplication`、`BaseShopActivity`、`ShopUi`、导航 helper | 否 |
| 第 2 轮 | 拆低耦合页面 | 登录、帮助、关于、高级设置、优惠券等 Activity | 是 |
| 第 3 轮 | 拆商品域 | 商品列表、活动 Tab、商品详情 Activity | 是 |
| 第 4 轮 | 拆购物车和订单域 | 购物车、确认订单、订单列表 Activity | 是 |
| 第 5 轮 | 拆设置和账号域 | 设置、账号、编辑资料、头像预览 Activity | 是 |
| 第 6 轮 | 迁移聊天主页面 | `ChatActivity` 成为真实首页，`MainActivity` 变 launcher 壳 | 是 |
| 第 7 轮 | 抽聊天控制器并清理旧代码 | 聊天 renderer/controller、旧 `MainActivity` 逻辑删除 | 是 |

### 第 0 轮：行为基线和安全网

目标：先把“不能变”的行为记录下来，避免拆分时凭感觉重写。

具体工作：

1. 执行一次当前构建：
   - `cd android-native && ./gradlew :app:assembleDebug`
2. 用当前 `MainActivity` 完整跑一遍核心流程：
   - 冷启动进入聊天首页。
   - 未登录发送聊天，确认只保存本地消息并提示登录。
   - 登录、注册成功后回聊天页并创建新本地会话。
   - 登录后发送 AI 导购问题，观察 SSE、思考过程、Markdown、商品卡、followups。
   - 停止生成。
   - 选择附件、上传附件、上传失败后恢复输入。
   - 语音识别成功后填入输入框，不自动发送。
   - 商品列表搜索、分类、分页、活动 Tab。
   - 商品详情、评价、加入购物车。
   - 未登录进入购物车、订单、优惠券，确认仍是页面内空态。
   - 购物车选择、数量、删除、结算。
   - 确认订单、优惠预览、提交订单。
   - 订单支付、取消、确认收货、评价。
   - 设置页、账号页、修改头像、修改昵称、手机号、邮箱。
   - debug 后端地址保存后，请求使用新地址。
   - 退出登录、删除账号后清理登录态并回聊天页。
3. 为 `MainActivity` 做方法归属标记，建议按下面类别整理：
   - `chat`：聊天首页、输入栏、SSE、历史抽屉、附件、语音。
   - `auth`：登录、注册、token 过期。
   - `product`：商品列表、活动、详情、评价、SKU。
   - `cart_order`：购物车、确认订单、订单列表、评价弹窗。
   - `account_settings`：设置、账号、编辑资料、头像预览、高级设置。
   - `shared_ui`：toast、button、panel、title、safe area、bottom sheet。
   - `navigation`：`showPage`、`backStack`、Agent route。
4. 记录每个页面入口对应的旧方法名，后续迁移时逐项勾掉。

本轮不做：

- 不新增 Activity。
- 不移动业务方法。
- 不改 manifest。
- 不改接口、UI 文案、登录跳转和数据库。

验收标准：

- 当前版本可编译或已记录明确的环境问题。
- 已形成 `MainActivity` 方法归属表。
- 已形成手工冒烟路径，后续每轮按同一套路径回归。

### 第 1 轮：抽共享基础设施，不拆业务页面

目标：让后续 Activity 能复用同一套服务、UI、导航和生命周期处理，同时 `MainActivity` 仍然承载所有页面。

新增文件建议：

- `app/src/main/java/com/xzxg/shop/ShopApplication.java`
- `app/src/main/java/com/xzxg/shop/BaseShopActivity.java`
- `app/src/main/java/com/xzxg/shop/ShopUi.java`
- `app/src/main/java/com/xzxg/shop/NavigationHelper.java`
- `app/src/main/java/com/xzxg/shop/BottomSheetHelper.java`
- `app/src/main/java/com/xzxg/shop/Routes.java`

具体工作：

1. 在 `AndroidManifest.xml` 为 `<application>` 增加 `android:name=".ShopApplication"`。
2. `ShopApplication` 只负责持有共享对象：
   - `SessionStore`
   - `ApiClient`
   - `LocalChatStore`
   - `MarkdownRenderer`
   - `ImageLoader`
3. 新增 `BaseShopActivity`，迁移通用能力：
   - `sessionStore()`
   - `api()`
   - `localChatStore()`
   - `imageLoader()`
   - `markdownRenderer()`
   - 统一 toast。
   - 统一系统栏、安全区、loading/error 空态。
   - 统一 401 处理入口，但保持当前不同页面的行为差异。
4. 新增 `ShopUi`，先只迁移无业务状态的 UI 工具：
   - `dp`
   - `rounded`
   - `primaryButton`
   - `secondaryButton`
   - `textOnlyButton`
   - `strong`
   - `muted`
   - `title`
   - `panel`
   - 常用 `LinearLayout` / `TextView` 创建方法。
5. 新增 `BottomSheetHelper`，只迁移通用弹层框架，不迁移手机号、邮箱等业务逻辑。
6. 新增 `Routes` 和 `NavigationHelper`，先封装 route 字符串和 `Intent` 创建方法。
7. 将 `MainActivity extends Activity` 改为 `MainActivity extends BaseShopActivity`。
8. `MainActivity` 内原 helper 方法可以保留同名薄封装，内部委托到 `ShopUi`，这样业务代码改动最小。

本轮不做：

- 不创建业务 Activity。
- 不删除 `MainActivity` 里的页面渲染方法。
- 不改任何页面入口。
- 不升级 Activity Result API。

验收标准：

- `./gradlew :app:assembleDebug` 通过。
- 启动、聊天、商品、购物车、订单、设置、账号主流程行为不变。
- `MainActivity` 行数开始下降，但所有页面仍由 `MainActivity` 渲染。

### 第 2 轮：拆低耦合页面 Activity

目标：先拆风险最低的页面，验证多 Activity 的基础设施可用。

新增 Activity：

- `LoginActivity`
- `HelpActivity`
- `AboutActivity`
- `AdvancedSettingsActivity`
- `CouponActivity`

建议迁移顺序：

1. 先拆 `HelpActivity` 和 `AboutActivity`：
   - 页面展示基本静态内容。
   - 从 `MainActivity` 原入口改为 `startActivity(...)`。
   - 返回使用系统返回键，不保留自定义 `backStack`。
2. 再拆 `AdvancedSettingsActivity`：
   - 迁移 debug API Base 展示、保存、重置。
   - 保存后仍更新 `SessionStore`，并确保后续 `ApiClient` 请求读到新地址。
   - 未登录/登录过期处理保持旧逻辑。
3. 再拆 `CouponActivity`：
   - 迁移优惠券列表、领取、刷新。
   - 未登录时仍展示“请先登录”空态，不自动打开登录页。
   - 401 中途过期时按旧逻辑进入登录页。
4. 最后拆 `LoginActivity`：
   - 迁移登录、注册表单。
   - 登录/注册成功后保存 token/account。
   - 成功后仍同步远端会话、创建新本地会话、进入聊天页。
   - 不新增登录成功返回原页面能力。

需要改的入口：

- `MainActivity` 中进入帮助、关于、高级设置、优惠券、登录/注册的地方，改为调用 `NavigationHelper`。
- Agent route 里遇到 `coupons` / `coupon` 时启动 `CouponActivity`。
- 未登录加入购物车仍 toast 后进入 `LoginActivity`。

本轮不做：

- 不拆商品、购物车、订单、账号。
- 不让 `LoginActivity` 支持 `return_route`。
- 不修改登录成功后的目的地。

验收标准：

- 新 Activity 都已注册到 manifest。
- 原入口点击后进入新页面。
- 返回键符合原行为。
- 未登录优惠券空态不变。
- 登录/注册成功仍进入聊天首页。
- debug 后端地址保存后，后续请求使用新地址。

### 第 3 轮：拆商品域

目标：把商品列表、活动 Tab、商品详情从 `MainActivity` 中拆出，同时保持列表缓存、滚动恢复和聊天商品卡返回行为。

新增文件建议：

- `ProductListActivity`
- `ProductDetailActivity`
- `ProductUiRenderer`

具体工作：

1. 新建 `ProductListActivity`：
   - 迁移商品首页容器。
   - 迁移搜索框。
   - 迁移分类筛选。
   - 迁移商品列表 Tab。
   - 迁移活动/促销 Tab。
   - 迁移分页加载、loading、error、empty。
   - 迁移购物车浮标数量展示。
2. 新建 `ProductDetailActivity`：
   - 通过 Intent 接收 `product_id`。
   - 接收来源标识，例如 `from=chat` 或 `from=products`，只用于必要的返回语义，不改变 UI。
   - 迁移商品详情头图、价格、库存、规格、详情文案。
   - 迁移评价列表。
   - 迁移加入购物车。
3. 新建 `ProductUiRenderer`：
   - 只放商品卡片、价格、标签、评分等无页面状态的渲染方法。
   - 聊天里的商品卡可以继续复用同一 renderer，但不要让 renderer 持有 Activity 状态。
4. 修改导航：
   - 底部/侧栏商品入口启动 `ProductListActivity`。
   - Agent route `products` 启动 `ProductListActivity`。
   - Agent route `promotions` / `promotion` / `activity` 启动 `ProductListActivity(tab=activity)`。
   - 聊天商品卡启动 `ProductDetailActivity(from=chat)`。
   - 商品列表商品卡启动 `ProductDetailActivity(from=products)`。
5. 返回处理：
   - `ProductListActivity` 打开详情时不 `finish` 自己，让系统返回栈保留列表滚动。
   - `ChatActivity` 或当前仍在 `MainActivity` 的聊天页打开详情时也不 `finish` 聊天页。

本轮不做：

- 不拆购物车和订单。
- 不改变商品 API 请求参数和分页规则。
- 不改变商品卡样式和字段展示。
- 不改变未登录加入购物车逻辑。

验收标准：

- 商品搜索、分类、分页结果和旧版本一致。
- 活动 Tab route 正确。
- 从商品列表进详情再返回，列表滚动和已加载数据不丢。
- 从聊天商品卡进详情再返回，回到原聊天页。
- 未登录加入购物车仍 toast 后进入登录页。
- 登录后加入购物车成功，购物车浮标数量更新。

### 第 4 轮：拆购物车、确认订单和订单列表

目标：把交易闭环拆出，但严格保持购物车快照、优惠预览、订单状态流转不变。

新增文件建议：

- `CartActivity`
- `CheckoutActivity`
- `CheckoutDraftStore`
- `OrderListActivity`
- `ReviewSheetController`

具体工作：

1. 新建 `CartActivity`：
   - 迁移购物车加载。
   - 迁移未登录空态，仍页面内显示，不自动跳登录。
   - 迁移商品选择、全选、数量加减、删除。
   - 迁移底部结算栏、合计金额、优惠提示。
   - 点击结算时生成当前购物车快照。
2. 新建 `CheckoutDraftStore`：
   - 进程内保存结算点击瞬间的 cart snapshot。
   - 返回 `checkout_draft_id` 给 `CheckoutActivity`。
   - 不通过 Intent 传大 JSON。
3. 新建 `CheckoutActivity`：
   - 通过 `checkout_draft_id` 读取 cart snapshot。
   - 只额外请求优惠预览。
   - 优惠预览失败时仍按旧逻辑展示确认订单。
   - 提交订单成功后 toast，并进入 `OrderListActivity`。
   - 如果 snapshot 丢失，提示“订单信息已失效，请返回购物车重新结算”，不自动重新请求购物车继续结算。
4. 新建 `OrderListActivity`：
   - 迁移订单列表。
   - 迁移未登录空态，仍页面内显示，不自动跳登录。
   - 迁移订单状态标签、商品摘要、金额。
   - 迁移支付、取消、确认收货。
5. 新建 `ReviewSheetController`：
   - 迁移评价弹窗。
   - 迁移评分、评价内容、提交、成功后刷新订单。
6. 修改导航：
   - 购物车入口启动 `CartActivity`。
   - 订单入口启动 `OrderListActivity`。
   - Agent route `cart` 启动 `CartActivity`。
   - Agent route `orders` 启动 `OrderListActivity`。

本轮不做：

- 不把购物车未登录空态改为登录页。
- 不把订单未登录空态改为登录页。
- 不在确认订单中重新请求 cart 替代 snapshot。
- 不改变订单支付、取消、确认收货、评价的触发条件。

验收标准：

- 未登录购物车显示旧空态。
- 未登录订单显示旧空态。
- 购物车选择、全选、数量、删除、合计金额正确。
- 结算页商品和金额来自点击结算瞬间的 cart snapshot。
- 优惠预览成功/失败表现和旧版本一致。
- 提交订单成功后进入订单列表。
- 支付、取消、确认收货、评价后订单刷新。

### 第 5 轮：拆设置和账号域

目标：把用户设置、账号资料和头像相关页面拆出，保持登录态清理和资料刷新行为不变。

新增 Activity：

- `SettingsActivity`
- `AccountActivity`
- `EditProfileActivity`
- `AvatarPreviewActivity`

具体工作：

1. 新建 `SettingsActivity`：
   - 迁移设置首页。
   - 迁移账号入口、帮助、关于、高级设置入口。
   - 迁移退出登录。
   - 迁移删除账号。
   - 未登录进入设置时仍直接进入登录页并 finish 当前页。
2. 新建 `AccountActivity`：
   - 迁移账号信息展示。
   - 迁移头像、昵称、手机号、邮箱入口。
   - 未登录进入账号页仍直接进入登录页并 finish 当前页。
3. 新建 `EditProfileActivity`：
   - 迁移昵称编辑。
   - 迁移手机号 BottomSheet。
   - 迁移邮箱 BottomSheet。
   - 迁移头像选择、裁剪、上传。
   - 保持原 requestCode 逻辑，暂不升级 Activity Result API。
4. 新建 `AvatarPreviewActivity`：
   - 迁移头像预览。
   - 保持从账号/设置进入时的返回行为。
5. 修改导航：
   - 设置入口启动 `SettingsActivity`。
   - 账号入口启动 `AccountActivity`。
   - 编辑资料入口启动 `EditProfileActivity`。
   - 头像预览入口启动 `AvatarPreviewActivity`。
6. 退出登录和删除账号：
   - 清理 `SessionStore`。
   - 创建新本地会话。
   - 清任务栈进入聊天页。

本轮不做：

- 不改头像压缩、上传字段和错误提示。
- 不改手机号、邮箱校验规则。
- 不改退出登录和删除账号后的落点。
- 不把 requestCode 迁移为 Activity Result API。

验收标准：

- 设置页未登录仍进入登录页。
- 账号页未登录仍进入登录页。
- 修改头像、昵称后页面刷新。
- 手机号、邮箱校验和保存行为不变。
- 退出登录后回聊天首页，并且购物车/订单等需要登录能力失效。
- 删除账号后清登录态并回聊天首页。

### 第 6 轮：迁移 ChatActivity，让 MainActivity 变 launcher 壳

目标：把最后也是最大的聊天页面从 `MainActivity` 迁到 `ChatActivity`。这一轮先做“整体搬迁”，不要急着拆 renderer/controller，降低变量数量。

新增文件：

- `ChatActivity`

具体工作：

1. 新建 `ChatActivity extends BaseShopActivity`。
2. 将 `MainActivity` 中仍未拆走的聊天相关字段整体迁到 `ChatActivity`：
   - `localSessionId`
   - `serverSessionId`
   - `streaming`
   - `activeRunId`
   - `activeStreamCall`
   - active assistant 渲染状态。
   - 附件状态。
   - 语音状态。
   - 历史抽屉状态。
3. 将聊天相关方法整体迁到 `ChatActivity`：
   - 聊天首页渲染。
   - 发送消息。
   - SSE 处理。
   - 停止生成。
   - Markdown/思考过程/商品卡/followups 渲染。
   - 附件选择、预览、上传。
   - 语音识别。
   - 历史抽屉、本地历史、远端历史同步。
   - token 过期时聊天页插入系统提示。
4. `MainActivity` 改为 launcher 壳：
   - `onCreate` 只启动 `ChatActivity`。
   - `finish()` 自己。
5. manifest 保持 `MainActivity` 为 launcher，先不直接把 launcher 切到 `ChatActivity`，减少安装升级风险。
6. 所有页面导航统一从 `ChatActivity` 或 helper 发起。
7. 普通页面跳转时不要取消 active stream。

本轮不做：

- 不抽 `AgentStreamController`。
- 不抽 `AgentMessageRenderer`。
- 不改本地消息表结构。
- 不改 SSE 事件解析和保存字段。
- 不改变 APP 启动首屏。

验收标准：

- 冷启动仍进入聊天页。
- 未登录发送聊天仍只保存本地消息并提示登录。
- 登录后新会话创建、远端历史同步和 SSE 正常。
- 思考过程、Markdown、表格、商品卡、followups 正常。
- 停止生成正常。
- 附件选择、预览、上传、失败恢复正常。
- 语音识别成功后填入输入框，不自动发送。
- 历史抽屉搜索、切换、置顶、重命名、删除正常。
- 从聊天打开商品、购物车、订单、优惠券后返回聊天正常。

### 第 7 轮：抽聊天控制器，删除旧 MainActivity 页面逻辑

目标：在 `ChatActivity` 已经稳定的基础上，再把聊天内部复杂逻辑拆成可维护的 renderer/controller，并最终清理旧 `MainActivity`。

新增文件建议：

- `AgentMessageRenderer`
- `AgentStreamController`
- `ChatAttachmentController`
- `VoiceInputController`
- `ChatHistoryDrawer`

具体工作：

1. 抽 `AgentMessageRenderer`：
   - assistant 消息渲染。
   - thinking 展开/收起。
   - Markdown 渲染。
   - 商品卡渲染。
   - followups 渲染。
   - 不持有网络请求状态。
2. 抽 `AgentStreamController`：
   - 发起 Agent SSE。
   - 维护 `activeRunId`、`activeStreamCall`、`streaming`。
   - 分发 delta、thinking、product cards、navigation_action、done、error。
   - 停止生成仍调用 run cancel。
   - 不改变本地保存格式。
3. 抽 `ChatAttachmentController`：
   - 文件/图片选择。
   - 附件预览。
   - 上传顺序。
   - 失败后的输入恢复。
4. 抽 `VoiceInputController`：
   - Android `SpeechRecognizer`。
   - 实时语音客户端。
   - `PcmRecorder`。
   - onDestroy 释放策略保持旧行为。
5. 抽 `ChatHistoryDrawer`：
   - 本地历史加载。
   - 远端历史同步。
   - 搜索、分页、置顶、重命名、删除。
   - 点击历史后恢复对应会话。
6. 清理 `MainActivity`：
   - 如果继续保留 launcher 壳，文件控制在 100 行以内。
   - 或在确认安装升级无问题后，将 launcher 直接切到 `ChatActivity`，删除 `MainActivity`。
7. 清理旧字段、旧方法、旧 imports、废弃 requestCode。

本轮不做：

- 不改变聊天 UI。
- 不改变 SSE 生命周期策略。
- 不改变历史会话同步策略。
- 不改变语音默认策略。
- 不引入新的线程模型或事件总线。

验收标准：

- `MainActivity.java` 小于 100 行，或已删除并由 `ChatActivity` 作为 launcher。
- 所有原 `showPage(...)` 入口已迁移为明确的 Activity 导航。
- Agent route 不依赖 `MainActivity` 内部方法。
- 全量冒烟清单通过。
- `./gradlew :app:assembleDebug` 通过。

## 11. 页面导航规则

### 11.1 未登录处理规则

不能用一个统一规则替代现有逻辑。拆分后必须逐页复刻当前未登录行为：

| 页面/入口 | 当前未登录行为 | 拆分后要求 |
| --- | --- | --- |
| 购物车页 | 页面内显示“请先登录，登录后可查看购物车并结算。” | `CartActivity` 内展示同等空态，不自动跳转 |
| 订单页 | 页面内显示“请先登录，登录后可查看订单。” | `OrderListActivity` 内展示同等空态，不自动跳转 |
| 优惠券页 | 页面内显示“请先登录，登录后可领取和查看优惠券。” | `CouponActivity` 内展示同等空态，不自动跳转 |
| 设置页 | 直接进入登录页 | `SettingsActivity` 未登录时启动 `LoginActivity` 并 finish |
| 账号页 | 直接进入登录页 | `AccountActivity` 未登录时启动 `LoginActivity` 并 finish |
| 编辑资料页 | 直接进入登录页 | `EditProfileActivity` 未登录时启动 `LoginActivity` 并 finish |
| 头像预览页 | 当前从设置页进入，设置页已要求登录 | 保持登录前置 |
| 加入购物车按钮 | toast 后进入登录页 | 保持一致 |

通用要求：

1. 不把购物车、订单、优惠券改成自动跳登录页。
2. 不新增登录成功回原页面能力。
3. 登录成功后仍统一创建新本地会话并进入聊天页。

### 11.2 登录过期规则

所有 Activity 复用 `BaseShopActivity.handleApiError`：

1. 捕获 401。
2. 清空 SessionStore。
3. 如果当前是 ChatActivity，插入系统提示。
4. 如果当前是强登录页面，启动 LoginActivity 并 finish 当前页面。
5. 取消当前页面活跃请求或忽略旧回调。

这里的“强登录页面”按当前 `handleAuthExpired` 逻辑定义，不等同于未登录入口逻辑：

- `cart`
- `orders`
- `settings`
- `account`
- `advanced_settings`

也就是说，未登录主动打开购物车目前展示空态；但如果购物车请求中途收到 401，则进入登录页。拆分后也要保持这个差异。

### 11.3 Agent navigation_action 规则

Agent 返回 route 时通过 `NavigationHelper.navigateRoute` 处理：

| route | 目标 Activity |
| --- | --- |
| `orders` | `OrderListActivity` |
| `cart` | `CartActivity` |
| `products` | `ProductListActivity` |
| `coupons` / `coupon` | `CouponActivity` |
| `promotions` / `promotion` / `activity` | `ProductListActivity(tab=activity)` |

未知 route 仍 toast “暂不支持该跳转”。

## 12. 状态归属调整

拆分后应避免把所有状态继续留在一个全局类里。

| 当前字段 | 目标归属 |
| --- | --- |
| `localSessionId` / `serverSessionId` | `ChatActivity` |
| `streaming` / `activeRunId` / `activeStreamCall` | `AgentStreamController` |
| `activeAssistant*` / `activeThinking` / `activeFollowups` | `AgentMessageRenderer` 或 `ChatActivity` 渲染状态 |
| `pendingAttachmentsBySession` | `ChatAttachmentController` |
| `speechRecognizer` / `speechRealtimeClient` / `pcmRecorder` | `VoiceInputController` |
| `drawerHistory*` | `ChatHistoryDrawer` |
| `lastProductKeyword` / `lastCategoryId` / `productListCache` | `ProductListActivity` |
| `cartCheckoutBar` / `activeCartSnapshot` | `CartActivity` 或 `CheckoutActivity` |
| `pendingAvatarBytes` / `pendingAvatarPreview` | `EditProfileActivity` |
| `activeToast` | `BaseShopActivity` |
| `appliedTopInset` / `appliedBottomInset` | `BaseShopActivity` |

## 13. 兼容和风险点

### 13.1 多 Activity 后的返回栈

当前单 Activity 用 `backStack` 字段模拟部分返回逻辑。拆成 Activity 后，应优先使用 Android 系统返回栈。

风险：

- 从 Chat 商品卡进入详情，再返回时 ChatActivity 是否仍保留流式状态。
- 从 ProductList 进入详情，再返回时列表滚动是否保留。
- 登录成功后返回哪个页面。

对策：

- 第一版登录成功统一回 ChatActivity。
- ProductList -> ProductDetail 使用普通 startActivity，不 finish ProductList。
- ChatActivity 启动其它页面时不 finish，除非退出登录或删除账号。
- 从 ChatActivity 的商品卡进入 ProductDetailActivity 时，ChatActivity 也不能 finish；返回后应回到原聊天会话。
- 不新增“登录后回原页面”能力，避免改变现有登录成功统一回聊天的行为。

### 13.2 共享 ApiClient 和 token 状态

风险：

- 多个 Activity 同时持有旧 ApiClient。
- 修改 debug API Base 后旧页面仍使用旧地址。

对策：

- `ApiClient` 每次请求读取 `sessionStore.apiBase()`，保持动态性。
- `ShopApplication.refreshApiClient()` 只作为显式刷新辅助。
- BaseActivity 在 `onResume` 可重新读取 `((ShopApplication)getApplication()).api()`。

### 13.3 SSE 和页面离开

风险：

- ChatActivity 发起 SSE 后用户跳转商品页，ChatActivity 仍在后台。
- 如果 ChatActivity 被系统销毁，SSE 需要取消或恢复策略。

第一版策略：

- 用户离开 ChatActivity 但 Activity 未销毁时，允许 SSE 继续，返回后看到继续生成。
- 普通 Activity 跳转不得主动取消 active stream。
- `onDestroy` 中不要新增与现有行为不同的 SSE 取消策略；是否在 Activity 销毁时取消 SSE 应作为后续独立生命周期治理需求处理。
- 语音和录音仍按现有 `onDestroy` 行为释放。
- 后续再设计后台生成/通知/恢复。

### 13.4 ActivityResult 分散

风险：

- requestCode 冲突。
- 头像选择结果被 ChatActivity 处理或反之。

对策：

- requestCode 按 Activity 私有常量定义。
- 每个 Activity 只处理自己发起的结果。
- 后续迁移 Activity Result API。

### 13.5 UI 工具迁移导致视觉不一致

风险：

- 按钮、卡片、字体、间距在不同 Activity 间出现差异。

对策：

- 第 1 轮先抽 `ShopUi`。
- 所有 Activity 只能通过 `ShopUi` 或 BaseActivity 包装方法创建基础组件。
- 禁止迁移时复制一份局部 UI 工具。

### 13.6 行为等价对照方式

每迁移一个页面，都应做“旧方法 vs 新 Activity”的对照，而不是只看新页面能不能打开。

要求：

1. 迁移前记录旧方法的入口、默认文案、按钮文案、toast 文案、API 调用顺序和失败展示。
2. 迁移后用同一账号、同一后端地址、同一操作路径对照。
3. 如果发现新 Activity 更“合理”但和旧行为不同，应先回退到旧行为。
4. 行为优化必须单独立项，不能混在拆分提交里。

## 14. 不变性验收矩阵

| 模块 | 不变性要求 | 验证方式 |
| --- | --- | --- |
| 启动 | 启动后首屏仍为聊天页，欢迎语和推荐问题逻辑不变 | 冷启动截图/手测 |
| 登录 | 登录/注册成功仍回聊天页并创建新本地会话 | 手测 + 查看本地历史 |
| 未登录购物车/订单/优惠券 | 显示原空态，不自动跳登录 | 手测 |
| Agent SSE | 同样事件顺序下渲染结果、保存字段一致 | 使用同一问题对比 UI 和 SQLite |
| 停止生成 | 仍调用 run cancel，保存已生成内容并提示“已停止生成” | 手测 |
| 附件 | 选择、预览、上传、失败恢复逻辑不变 | 图片/文件各一例 |
| 语音 | 默认实时语音，失败提示和识别后填入输入框不变 | 手测 |
| 历史会话 | 本地先显示、远端后刷新，搜索/分页/置顶/重命名/删除不变 | 手测 |
| 商品列表 | 搜索、分类、分页、返回滚动不变 | 手测 |
| 商品详情 | 展示字段、评价、加入购物车不变 | 手测 |
| 购物车 | 选择、全选、数量、删除、底部金额不变 | 手测 |
| 确认订单 | 使用结算点击时的 cart snapshot，只额外请求优惠预览 | 手测 + 日志 |
| 订单 | 支付、取消、确认收货、评价入口条件不变 | 手测 |
| 账号 | 昵称、头像、手机号、邮箱、退出、删除行为不变 | 手测 |
| 设置 | debug API Base、侧栏返回聊天开关不变 | 手测 |

## 15. 验收清单

### 15.1 构建验收

```bash
cd android-native
./gradlew :app:assembleDebug
```

### 15.2 页面验收

- 启动进入聊天页。
- 左上角入口可进入登录或抽屉。
- 抽屉导航到商品、购物车、订单、设置正常。
- 系统返回键符合预期。
- 旋转屏幕当前可不作为强验收项，因为现有实现没有完整状态恢复；但不得崩溃。

### 15.3 聊天验收

- 未登录发送消息，保存本地并提示登录。
- 登录后发送消息，创建远程会话并流式展示。
- 可停止生成。
- 可长按复制 assistant 内容。
- Markdown、表格、商品卡展示正常。
- 追问按钮可填入输入框。
- 历史会话可搜索、打开、置顶、重命名、删除。
- 远程历史无本地消息时可拉取详情并回放。

### 15.4 商品交易验收

- 商品列表加载、搜索、分类、分页。
- 活动 Tab 加载。
- 商品详情展示图片、SKU、评价。
- 商品加入购物车。
- 购物车选择、全选、数量、删除。
- 确认订单、优惠展示、提交订单。
- 订单支付、取消、确认收货、评价。

### 15.5 账号验收

- 登录、注册。
- 修改昵称。
- 上传头像。
- 修改手机号、邮箱。
- 退出登录。
- 删除账号。
- debug 后端地址保存。

## 16. 拆分后文件行数预估

当前 `android-native` 主要 Java 文件共约 10776 行，其中 `MainActivity.java` 约 8961 行。拆分后总行数预计不会明显减少，可能因为 Activity 生命周期、Intent 参数、Manifest 注册和 helper 边界略有增加；目标不是压缩总代码量，而是把单个超大文件拆成可维护的小文件。

预估原则：

1. 单个 Activity 尽量控制在 900 行以内。
2. `ChatActivity`、`ProductDetailActivity` 这类复杂页面允许短期超过 900 行，但不建议超过 1400 行。
3. 超过 1200 行的文件必须优先考虑继续拆 renderer、controller 或 store。
4. 纯工具类、store、helper 应控制在 80-550 行，不承载页面业务流程。

### 16.1 入口和共享基础设施

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `MainActivity.java` | 30-80 | 只保留 launcher 壳；如果最终让 `ChatActivity` 直接做 launcher，可删除 |
| `ShopApplication.java` | 80-140 | 持有 `SessionStore`、`ApiClient`、`LocalChatStore`、`ImageLoader` 等共享对象 |
| `BaseShopActivity.java` | 200-320 | 系统栏、安全区、toast、通用错误处理、共享服务访问 |
| `ShopUi.java` | 350-550 | 按钮、卡片、标题、空态、loading、通用 view 工厂 |
| `NavigationHelper.java` | 120-220 | Activity 跳转、Agent route 到 Activity 的映射 |
| `Routes.java` | 40-80 | route 常量、Intent extra key 常量 |
| `BottomSheetHelper.java` | 150-250 | 通用 BottomSheet 容器、输入弹层基础能力 |

### 16.2 登录和低耦合页面

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `LoginActivity.java` | 380-560 | 登录、注册、保存登录态、登录成功回聊天 |
| `HelpActivity.java` | 80-140 | 帮助页静态内容 |
| `AboutActivity.java` | 80-140 | 关于页静态内容 |
| `AdvancedSettingsActivity.java` | 200-320 | debug 后端地址、保存、重置 |
| `CouponActivity.java` | 250-420 | 优惠券列表、领取、未登录空态 |

### 16.3 商品域

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `ProductListActivity.java` | 650-900 | 商品列表、搜索、分类、分页、活动 Tab、购物车浮标 |
| `ProductDetailActivity.java` | 700-1000 | 商品详情、图片、SKU、评价、加入购物车 |
| `ProductUiRenderer.java` | 300-500 | 商品卡、价格、标签、评分等复用渲染 |

### 16.4 购物车和订单域

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `CartActivity.java` | 550-800 | 购物车列表、选择、全选、数量、删除、结算栏 |
| `CheckoutActivity.java` | 350-550 | 确认订单、优惠预览、提交订单 |
| `CheckoutDraftStore.java` | 60-120 | 进程内保存结算点击瞬间的购物车快照 |
| `OrderListActivity.java` | 600-900 | 订单列表、支付、取消、确认收货 |
| `ReviewSheetController.java` | 180-320 | 订单评价弹窗、评分、提交 |

### 16.5 设置和账号域

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `SettingsActivity.java` | 320-500 | 设置首页、账号入口、帮助/关于/高级设置入口、退出、删除账号 |
| `AccountActivity.java` | 300-480 | 账号信息展示、头像/昵称/手机号/邮箱入口 |
| `EditProfileActivity.java` | 600-900 | 昵称、手机号、邮箱、头像选择裁剪上传 |
| `AvatarPreviewActivity.java` | 130-220 | 头像预览 |

### 16.6 聊天域

| 文件 | 预估行数 | 说明 |
| --- | ---: | --- |
| `ChatActivity.java` | 900-1400 | 聊天页面组合、输入栏、页面状态、协调各 controller |
| `AgentMessageRenderer.java` | 700-1100 | assistant 消息、thinking、Markdown、商品卡、followups |
| `AgentStreamController.java` | 500-800 | Agent SSE、run cancel、流式事件分发、本地保存触发 |
| `ChatAttachmentController.java` | 350-600 | 附件选择、预览、上传、失败恢复 |
| `VoiceInputController.java` | 350-600 | 语音识别、实时语音、录音资源释放 |
| `ChatHistoryDrawer.java` | 500-800 | 历史抽屉、本地/远端会话、搜索、分页、置顶、重命名、删除 |

### 16.7 现有支撑类

这些文件原则上不需要大改，除非拆分过程中发现有明确共享边界问题。

| 文件 | 当前行数 | 拆分后预估 | 说明 |
| --- | ---: | ---: | --- |
| `ApiClient.java` | 734 | 730-850 | 暂不拆 API client，避免接口行为变化 |
| `LocalChatStore.java` | 443 | 440-520 | 暂不改表结构和升级路径 |
| `SessionStore.java` | 102 | 100-140 | 可能补少量共享访问方法 |
| `ImageLoader.java` | 115 | 110-150 | 保持图片加载缓存逻辑 |
| `MarkdownRenderer.java` | 127 | 120-170 | 继续作为 Markdown 渲染适配 |
| `PcmRecorder.java` | 106 | 100-140 | 保持录音实现 |
| `SpeechRealtimeClient.java` | 188 | 190-260 | 保持实时语音客户端 |

### 16.8 总体规模预估

| 类别 | 预估行数 |
| --- | ---: |
| 入口和共享基础设施 | 970-1640 |
| 登录和低耦合页面 | 990-1580 |
| 商品域 | 1650-2400 |
| 购物车和订单域 | 1740-2690 |
| 设置和账号域 | 1350-2100 |
| 聊天域 | 3300-5300 |
| 现有支撑类 | 1790-2230 |
| 合计 | 11790-17940 |

合理目标区间是 12000-15000 行。如果超过 16000 行，通常说明拆分时复制了过多 UI 或业务逻辑，应优先检查 `ShopUi`、renderer 和 controller 是否真正复用。

## 17. 推荐提交切分

建议拆成多个 PR 或多个小提交。第 0 轮通常不产生业务代码提交，只产出行为基线和检查记录；第 1-7 轮可以对应下面 8 个提交，其中最后一轮可按风险再拆成两个提交。

1. `refactor(android): add shared base activity and ui helpers`
2. `refactor(android): move login and simple settings pages to activities`
3. `refactor(android): split product list and detail activities`
4. `refactor(android): split cart checkout and order activities`
5. `refactor(android): split account profile activities`
6. `refactor(android): introduce chat activity`
7. `refactor(android): extract chat stream and renderer controllers`
8. `chore(android): make chat activity launcher and remove legacy main page code`

每个提交都应可编译。不要把页面拆分和安全配置、UI 重设计、接口改造混在同一提交。

## 18. 最小可行第一步

如果要马上开始编码，建议第一步只做：

1. 新增 `BaseShopActivity`。
2. 新增 `ShopApplication`。
3. 新增 `ShopUi`。
4. 让 `MainActivity` 继承 `BaseShopActivity`。
5. 将最无风险的工具方法移动出去：
   - `dp`
   - `rounded`
   - `toastLine`
   - `primaryButton`
   - `secondaryButton`
   - `textOnlyButton`
   - `strong`
   - `muted`
   - `title`
   - `panel`
6. 不新增任何业务 Activity。

这一步完成后，后续每拆一个页面，都能复用同一套 UI 和服务初始化逻辑，风险最小。
