# 安卓 APP 技术方案 v33

## 1. 背景与目标

本方案针对 `安卓APP_问题_v32.md` 中提出的问题设计，代码基线为当前已拆分后的安卓工程：

- 订单页：`android-native/app/src/main/java/com/xzxg/shop/order/OrderListActivity.java`
- 商品页：`android-native/app/src/main/java/com/xzxg/shop/product/ProductListActivity.java`
- 商品详情页：`android-native/app/src/main/java/com/xzxg/shop/product/ProductDetailActivity.java`
- 聊天页：`android-native/app/src/main/java/com/xzxg/shop/chat/ChatActivity.java`
- 基础 Activity：`android-native/app/src/main/java/com/xzxg/shop/base/BaseShopActivity.java`

目标：

- 订单列表中所有时间统一显示为 `yyyy-MM-dd HH:mm:ss`，不再直接展示接口返回的 ISO 时间字符串。
- 订单在不同状态下补充展示付款时间、发货时间、完成时间。
- 将商品页已有的“购物车”悬浮入口抽成可复用组件，并接入商品详情页、订单页。
- 聊天页 AI 回复商品卡片中，点击“加入购物车”成功后，按钮变为“进入购物车”，再次点击直接跳转购物车页。

非目标：

- 不改变订单创建、支付、发货、确认收货、评价等业务流程。
- 不改变购物车页面和结算页面的主流程。
- 不重做页面导航结构，只复用现有多 Activity 架构。
- 不引入新的三方 UI 框架。

## 2. 问题到模块映射

| 问题 | 当前主要位置 | 根因判断 | 方案入口 |
| --- | --- | --- | --- |
| 创建时间展示为 `2026-06-09T13:59:42+08:00` | `OrderListActivity.orderCard()` | 前端直接 `item.optString("created_at")` 展示原始接口字段 | 新增统一时间格式化工具，订单页展示前格式化 |
| 支付/发货/完成后缺少对应时间 | `OrderListActivity.orderCard()` | 订单卡片只展示 `created_at`，未读取生命周期时间字段 | 增加订单时间行渲染逻辑，按字段存在与状态展示 |
| 购物车悬浮入口只在商品页展示 | `ProductListActivity.addProductCartFab()` | FAB 逻辑写死在商品列表 Activity 内，无法在其他页面复用 | 抽出 `CartFabHelper` 或基类方法，接入商品详情页和订单页 |
| 聊天商品卡片加入购物车后仍回到“加入购物车” | `ChatActivity.addProductToCart()`、`AgentMessageRenderer.productCard()` | 成功后按钮被 `postDelayed()` 恢复为初始文案 | 成功后把按钮切换为“进入购物车”，点击跳转购物车 |

## 3. 总体设计

本轮采用“小范围抽公共能力”的方式：

- 订单时间格式化抽到 `ShopUi` 或新增 `DateTimeFormatter` 工具类，避免订单页内散落字符串处理。
- 购物车悬浮入口抽到 `ui/CartFabHelper.java`，负责创建 UI、读取购物车数量、更新角标、跳转购物车。
- 各页面只负责提供根 `FrameLayout`、底部避让距离和生命周期触发点。
- 聊天页商品卡片不增加购物车悬浮入口，本轮只修正商品卡片按钮状态和跳转行为。
- 接口字段采用兼容读取，优先使用后端标准 snake_case 字段，同时兼容常见 camelCase 字段，降低前后端字段命名切换风险。

## 4. 订单时间展示方案

### 4.1 接口字段约定

订单列表接口仍使用现有 `GET /orders`，返回结构保持：

```json
{
  "items": [
    {
      "order_id": "order_xxx",
      "status": "paid",
      "merchant_name": "商家",
      "total_amount": "128.00",
      "created_at": "2026-06-09T13:59:42+08:00",
      "paid_at": "2026-06-09T14:01:10+08:00",
      "shipped_at": null,
      "completed_at": null,
      "items": []
    }
  ]
}
```

前端建议读取字段：

- 创建时间：`created_at`，兼容 `createdAt`
- 付款时间：`paid_at`，兼容 `paidAt`、`paid_time`、`paidTime`
- 发货时间：`shipped_at`，兼容 `shippedAt`、`shipped_time`、`shippedTime`
- 完成时间：`completed_at`，兼容 `completedAt`、`completed_time`、`completedTime`、`received_at`、`receivedAt`

后端字段建议统一为 snake_case：

- `created_at`
- `paid_at`
- `shipped_at`
- `completed_at`

### 4.2 前端格式化规则

新增统一格式化方法，建议位置二选一：

- 简单方案：放入 `ShopUi.formatDateTime(String raw)`。
- 更清晰方案：新增 `ui/DateTimeFormatter.java`，只处理时间字符串。

建议采用新增工具类：

```java
package com.xzxg.shop.ui;

public final class DateTimeFormatter {
    private DateTimeFormatter() {}

    public static String formatOrderTime(String raw) {
        // 输入为空返回空字符串。
        // 优先解析 ISO_OFFSET_DATE_TIME，例如 2026-06-09T13:59:42+08:00。
        // 解析成功后输出 yyyy-MM-dd HH:mm:ss。
        // 解析失败时做轻量兜底：去掉 T，去掉末尾时区，保留前 19 位。
    }
}
```

兼容规则：

- `null`、空字符串、`"null"` 返回空字符串，不展示该时间行。
- `2026-06-09T13:59:42+08:00` 展示为 `2026-06-09 13:59:42`。
- `2026-06-09T13:59:42Z` 展示为本地时区或原始 UTC 转换后的 `yyyy-MM-dd HH:mm:ss`，建议统一按设备默认时区展示。
- `2026-06-09 13:59:42` 已是目标格式时原样返回。
- 解析失败时不让页面崩溃，展示原始字符串或轻量清洗后的字符串。

### 4.3 订单卡片渲染

`OrderListActivity.orderCard(JSONObject item)` 当前只渲染：

```java
card.addView(ShopUi.muted(this, "创建时间：" + item.optString("created_at", "")));
```

改为：

```java
addOrderTimeLine(card, "创建时间", readOrderTime(item, "created_at", "createdAt"));
addOrderTimeLine(card, "付款时间", readOrderTime(item, "paid_at", "paidAt", "paid_time", "paidTime"));
addOrderTimeLine(card, "发货时间", readOrderTime(item, "shipped_at", "shippedAt", "shipped_time", "shippedTime"));
addOrderTimeLine(card, "完成时间", readOrderTime(item, "completed_at", "completedAt", "completed_time", "completedTime", "received_at", "receivedAt"));
```

`addOrderTimeLine()` 只在格式化后非空时添加 TextView。

状态与展示关系：

| 订单状态 | 应展示时间 |
| --- | --- |
| `pending_payment` / `pending_pay` | 创建时间 |
| `paid` / `pending_ship` | 创建时间、付款时间 |
| `shipped` | 创建时间、付款时间、发货时间 |
| `completed` | 创建时间、付款时间、发货时间、完成时间 |
| `cancelled` / `canceled` / `closed_timeout` | 创建时间；如接口提供付款时间也可展示 |

实现上不需要强制根据状态隐藏字段：只要后端返回了对应时间，前端就展示。这样可以兼容退款、取消前已支付等边界状态。

### 4.4 验收标准

- 订单创建时间不再出现 `T` 和 `+08:00`。
- 支付成功刷新订单后，已支付订单展示“付款时间”。
- 发货后的订单展示“发货时间”。
- 确认收货后的订单展示“完成时间”。
- 任一时间字段为空时，不出现 `null`、空冒号或多余空行。

## 5. 购物车悬浮入口复用方案

### 5.1 当前实现问题

`ProductListActivity` 内已有 `addProductCartFab()`、`refreshProductCartBadge()`、`updateProductCartBadge()`、`cartItemCount()`、`openLegacyCartPage()` 等逻辑。

问题是这些逻辑绑定了商品页字段：

- `root`
- `productCartFab`
- `productCartBadge`
- `cartItemCountCache`

如果复制到商品详情页、订单页，会造成 UI 和接口刷新逻辑重复，后续角标样式或避让距离也难统一。

### 5.2 抽取组件

新增：

`android-native/app/src/main/java/com/xzxg/shop/ui/CartFabHelper.java`

职责：

- 创建白色圆形“购物车”悬浮按钮。
- 创建红色数量角标。
- 登录态为空时不展示。
- 调用 `ApiClient.cart()` 获取购物车数量。
- 点击后跳转 `CartActivity`。
- 对外暴露 `attach()`、`refresh()`、`detach()`。

建议接口：

```java
public final class CartFabHelper {
    public interface Host {
        Activity activity();
        SessionStore sessionStore();
        ApiClient api();
        void showToastLine(String text);
    }

    public static CartFabHelper attach(
            Host host,
            FrameLayout root,
            int bottomMarginDp
    ) {
        // 创建并添加 FAB，返回实例。
    }

    public void refresh() {
        // 后台线程读取 cart，主线程更新 badge。
    }

    public void detach() {
        // 从 root 移除，避免重复添加。
    }
}
```

如果不想引入 `Host` 接口，也可以让 `BaseShopActivity` 提供受保护方法：

```java
protected CartFabHelper attachCartFab(FrameLayout root, int bottomMarginDp) {
    return CartFabHelper.attach(this, root, bottomMarginDp);
}
```

更推荐在 `BaseShopActivity` 里统一封装，因为现有页面都继承 `BaseShopActivity`，并且已经能访问 `sessionStore()`、`api()`、`showToastLine()`。

### 5.3 UI 规则

沿用当前商品页样式，但组件化后统一维护：

- 背景：白色圆形。
- 边框：`#DCE0E6`，1dp。
- 文字：`购物车`，13sp，黑色加粗。
- 角标：红底白字，超过 99 显示 `99+`。
- 尺寸：
  - 按钮 56dp。
  - 外层容器 72dp，保留角标空间。
  - 角标 30dp x 20dp。
- 默认位置：右下角。
- 默认右边距：18dp。
- 底部边距由页面传入，避免挡住底部输入框、底部 tab 或订单操作区。

各页面建议底部边距：

| 页面 | 根布局 | 建议 bottomMarginDp | 说明 |
| --- | --- | --- | --- |
| 商品页 | `ProductListActivity.root` | 68 | 避让底部商品 tab |
| 商品详情页 | `ProductDetailActivity.root` | 24 | 页面底部无固定输入栏 |
| 订单页 | `OrderListActivity` 改为 `FrameLayout root` | 24 | 页面底部无固定输入栏 |

### 5.4 页面接入方式

#### 商品页

将 `ProductListActivity.addProductCartFab()` 替换为：

```java
cartFabHelper = attachCartFab(root, 68);
```

商品加入购物车成功后：

```java
if (cartFabHelper != null) {
    cartFabHelper.refresh();
}
```

原有商品页私有字段和方法可以删除：

- `productCartFab`
- `productCartBadge`
- `cartItemCountCache`
- `addProductCartFab()`
- `refreshProductCartBadge()`
- `updateProductCartBadge()`
- `cartItemCount()`
- `openLegacyCartPage()`

#### 商品详情页

`ProductDetailActivity.render()` 已经使用 `FrameLayout root`，在 `setContentView(root)` 和 `bindRootSystemBarPadding(content)` 之后调用：

```java
cartFabHelper = attachCartFab(root, 24);
```

商品详情页“加入购物车”成功后刷新角标。

#### 订单页

`OrderListActivity` 当前根布局是 `LinearLayout content`，需要轻量改成：

```java
FrameLayout root = new FrameLayout(this);
content = new LinearLayout(this);
root.addView(content, new FrameLayout.LayoutParams(-1, -1));
setContentView(root);
bindRootSystemBarPadding(content);
cartFabHelper = attachCartFab(root, 24);
```

这不是页面结构重构，只是为了让 FAB 能覆盖在内容上方。

### 5.5 生命周期与刷新

建议在 `BaseShopActivity` 或各 Activity 中处理：

- `onResume()`：调用 `cartFabHelper.refresh()`，确保从购物车页返回后角标更新。
- 加入购物车成功：立即 `refresh()`。
- 未登录：不展示 FAB。
- 登录状态变化后：页面重建或 `onResume()` 时重新 attach。

如果组件内部开启后台线程，需要注意：

- Activity finishing 或 root 已 detach 后，主线程更新前检查 `activity.isFinishing()`。
- `badge` 为 `null` 时直接返回。
- 网络失败时静默，不弹错误，避免每次进页面都打扰用户。

## 6. 聊天页商品卡片按钮方案

### 6.1 当前实现问题

AI 回复里的商品卡片由 `AgentMessageRenderer.productCard()` 渲染，卡片下方有“详情”和“加入购物车”两个按钮：

```java
Button detail = ShopUi.secondaryButton(context, "详情");
Button add = ShopUi.primaryButton(context, "加入购物车");
```

点击“详情”会调用 `ProductActions.openDetail()`，当前会进入 `ProductDetailActivity`。按照本方案第 5 节，商品详情页会展示购物车悬浮入口，因此详情链路满足 v32 要求。

点击“加入购物车”会调用 `ChatActivity.addProductToCart(JSONObject item, Button sourceButton)`。当前成功后的逻辑是：

```java
sourceButton.setText("已加入");
sourceButton.postDelayed(() -> sourceButton.setText("加入购物车"), 600);
```

这不符合 v32 要求。成功后按钮不应该恢复为“加入购物车”，而应该变为“进入购物车”，点击后跳转购物车页。

### 6.2 按钮状态设计

聊天商品卡片“加入购物车”按钮状态如下：

| 状态 | 文案 | enabled | 点击行为 |
| --- | --- | --- | --- |
| 初始 | 加入购物车 | true | 调用加入购物车接口 |
| 请求中 | 加入中... | false | 不响应点击 |
| 成功 | 进入购物车 | true | 跳转 `CartActivity` |
| 失败 | 加入购物车 | true | 允许重新加入 |

成功后不再使用 `postDelayed()` 恢复文案。

### 6.3 前端实现

修改 `ChatActivity.addProductToCart(JSONObject item, Button sourceButton)` 成功分支：

```java
runOnUiThread(() -> {
    toastLine("已加入购物车");
    if (sourceButton != null) {
        sourceButton.setEnabled(true);
        sourceButton.setAlpha(1f);
        sourceButton.setText("进入购物车");
        sourceButton.setOnClickListener(v -> startActivity(new Intent(this, CartActivity.class)));
    }
});
```

失败分支仍恢复为：

```java
sourceButton.setText("加入购物车");
sourceButton.setOnClickListener(v -> addProductToCart(item, sourceButton));
```

为了避免把点击事件逻辑散在 `ChatActivity` 和 `AgentMessageRenderer` 两处，也可以把 `ProductActions` 扩展为：

```java
void openCart();
```

但本轮更小的改法是在 `ChatActivity.addProductToCart()` 成功后直接覆盖 `sourceButton` 的点击行为。

### 6.4 商品详情链路

`AgentMessageRenderer` 的“详情”按钮保持现状：

```java
productActions.openDetail(productField(item, "productId", "product_id", "id", ""));
```

`ChatActivity.openProductDetailActivity()` 继续打开 `ProductDetailActivity`。商品详情页接入 `CartFabHelper` 后，用户从聊天商品卡片点“详情”进入详情页时，也能看到购物车悬浮入口。

## 7. 后端配合事项

订单时间展示需要后端确认 `GET /orders` 是否返回以下字段：

- `created_at`
- `paid_at`
- `shipped_at`
- `completed_at`

如果当前接口没有 `paid_at`、`shipped_at`、`completed_at`，后端需要补充：

- 支付成功时写入 `paid_at`。
- 发货成功时写入 `shipped_at`。
- 确认收货成功时写入 `completed_at`。

时间格式建议：

- 后端返回 ISO 8601 字符串，例如 `2026-06-09T13:59:42+08:00`。
- 前端负责展示格式转换为 `2026-06-09 13:59:42`。

购物车悬浮入口不需要新增后端接口，继续使用现有：

- `GET /cart`

前端根据 `items[].quantity` 求和得到角标数量。

聊天页商品卡片按钮不需要新增后端接口，仍使用现有：

- `POST /cart/items`

加入成功后由前端切换按钮状态。

## 8. 实施步骤

1. 新增订单时间格式化工具。
2. 修改 `OrderListActivity.orderCard()`，使用 `addOrderTimeLine()` 渲染创建、付款、发货、完成时间。
3. 从 `ProductListActivity` 抽出 `CartFabHelper`。
4. 在 `BaseShopActivity` 增加 `attachCartFab()` 便捷方法。
5. 商品页替换为复用组件，并在加入购物车成功后刷新角标。
6. 商品详情页接入购物车 FAB，并在加入购物车成功后刷新角标。
7. 订单页根布局轻量改为 `FrameLayout + content`，接入购物车 FAB。
8. 修改聊天页 `addProductToCart()` 成功后的按钮状态：从“加入购物车”变为“进入购物车”，点击进入购物车页。
9. 编译并真机回归。

## 9. 测试计划

### 9.1 编译检查

在 `android-native` 目录执行：

```powershell
.\gradlew.bat :app:compileDebugJavaWithJavac
```

必要时执行：

```powershell
.\gradlew.bat :app:assembleDebug
```

### 9.2 真机验证

订单页：

- 新建订单后进入订单页，创建时间展示为 `yyyy-MM-dd HH:mm:ss`。
- 支付订单后刷新，出现付款时间。
- 后端或测试数据置为已发货后，出现发货时间。
- 确认收货后，出现完成时间。
- 空字段不展示，不出现 `null`。

购物车悬浮入口：

- 商品页、商品详情页、订单页登录后均展示购物车 FAB。
- 未登录时不展示购物车 FAB。
- 点击 FAB 跳转购物车页。
- 加入购物车后角标数量刷新。
- 从购物车页删除商品后返回，角标刷新。

聊天商品卡片：

- 点击“详情”进入商品详情页。
- 商品详情页展示购物车悬浮入口。
- 点击聊天商品卡片“加入购物车”后，按钮进入请求中状态。
- 加入成功后按钮显示“进入购物车”。
- 再次点击“进入购物车”跳转购物车页。
- 加入失败后按钮恢复为“加入购物车”，允许重试。

### 9.3 回归范围

- 商品列表搜索、分类筛选、分页加载。
- 商品详情加入购物车。
- 聊天页发送消息、商品卡片详情、商品卡片加入购物车、语音按钮、小猪向导、侧边栏历史。
- 订单支付、取消、确认收货、评价。

## 10. 风险与处理

| 风险 | 影响 | 处理 |
| --- | --- | --- |
| 后端没有返回生命周期时间字段 | 前端只能展示创建时间 | 前端兼容空字段；后端补齐 `paid_at`、`shipped_at`、`completed_at` |
| 时间字符串格式不统一 | 订单时间展示异常 | 格式化工具多格式兼容，解析失败时兜底展示清洗后的原值 |
| FAB 层级遮挡页面控件 | 商品详情页或订单页操作受影响 | 页面传入不同 bottomMargin，根布局使用 `FrameLayout` 承载浮层 |
| 多页面重复刷新购物车数量 | 轻微网络开销 | 只在 attach、onResume、加入购物车成功后刷新；失败静默 |
| 未登录展示购物车入口 | 点击后体验不一致 | 组件内部检查 token，未登录不 attach |
| 聊天商品卡片按钮被复用渲染覆盖状态 | 历史消息重新渲染后按钮可能回到初始状态 | 本轮只要求当前点击后的按钮状态；如后续需要历史持久化，可记录本地已加购 productId 集合 |

## 11. 验收结论

本方案完成后，订单时间展示与订单生命周期字段会统一；购物车悬浮入口会从商品页私有实现变成可复用能力，并覆盖商品详情页、订单页；聊天页商品卡片加入购物车成功后会明确引导用户进入购物车。整体改动保持在现有多 Activity 架构内，不引入新的导航或 UI 框架。
