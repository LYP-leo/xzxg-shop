# 安卓 APP 技术方案 v17

## 1. 背景

本文针对 [安卓APP_问题_v16.md](./安卓APP_问题_v16.md) 设计下一版 Android 原生 APP 修复方案。

本轮问题覆盖四个区域：

- 商品页：键盘不应把底部 tab bar 顶上来；加入购物车需要可见反馈；需要商品页悬浮购物车数量入口；商品详情返回列表时不能先闪到顶部。
- 购物车页：商品图和数量控件间距不足；结算区文案应区分“商品总额”和“应付”；优惠明细多了一层底框。
- 设置页：新增“高级设置”，并把“测试后端”迁移进去。
- 侧栏：搜索键盘不应把底部用户栏顶上来；“返回聊天”按钮改为高级设置中的可选项。

## 2. 总体设计

```text
键盘策略:
  聊天页:
    保持输入框跟随键盘
  商品页 / 侧栏:
    键盘覆盖底部栏
    页面不 resize

商品页:
  加入购物车:
    按钮进入 loading/成功态
    刷新悬浮购物车数量
  悬浮购物车:
    右下角展示购物车图标
    红点显示购物车商品总数量
    >99 显示 99+
  详情返回:
    直接恢复进入前滚动位置
    不显示顶部闪烁

购物车页:
  商品图和数量控件上下分离
  结算主面板显示“商品总额”
  优惠明细内显示“应付”
  优惠明细不再作为嵌套 panel

设置:
  设置页新增“高级设置”
  测试后端迁移到高级设置页
  侧栏“返回聊天”按钮由高级设置开关控制
```

## 3. 键盘与底部栏策略

### 3.1 当前问题

`AndroidManifest.xml` 当前配置：

```xml
android:windowSoftInputMode="adjustResize"
```

`MainActivity.configureSystemBars()` 也设置：

```java
window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
```

聊天页还额外调用 `bindImeInsets()`：

```java
content.setPadding(0, 0, 0, imeBottom);
```

因此，商品页搜索框和侧栏搜索框获得焦点时，Activity 被键盘 resize，底部 `productBottomBar()` 和侧栏 `bottomUserBar()` 都会被顶到键盘上方。最新需求要求这些底部栏被键盘覆盖，而不是跟随上移。

### 3.2 页面级 soft input 模式

新增两个 helper：

```java
private void useKeyboardResize() {
    getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
}

private void useKeyboardOverlay() {
    getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_NOTHING);
}
```

页面策略：

- `renderChatHome()`：调用 `useKeyboardResize()`，保留聊天输入框跟随键盘。
- `renderProducts()` / `renderProductsTab()`：调用 `useKeyboardOverlay()`。
- `showDrawer()`：打开侧栏时调用 `useKeyboardOverlay()`；关闭侧栏后根据当前页面恢复对应模式。
- `renderCart()` / 设置类页面：无输入法底部栏冲突，可默认使用 `useKeyboardOverlay()`；如登录/表单页需要自然 resize，再单独调用 resize。

### 3.3 Insets 监听清理

`bindImeInsets()` 只应在聊天页生效。`baseScreen()` 建议清理旧 listener：

```java
root.setOnApplyWindowInsetsListener(null);
content.setPadding(0, 0, 0, 0);
```

避免从聊天页切到商品页后仍残留 IME padding。

## 4. 商品页改造

### 4.1 加入购物车按钮反馈

当前商品卡片和详情页都直接调用：

```java
add.setOnClickListener(v -> addProductToCart(item));
```

`addProductToCart()` 成功后只有 toast，按钮本身没有视觉反馈。v17 改为传入按钮：

```java
add.setOnClickListener(v -> addProductToCart(item, add));
```

按钮状态：

```text
点击前:
  文案: 加入购物车
  enabled: true

请求中:
  文案: 加入中...
  enabled: false
  透明度降低或背景变浅

成功:
  文案: 已加入
  enabled: true
  300-600ms 后恢复“加入购物车”
  刷新购物车悬浮数量

失败:
  文案恢复
  toast 展示错误
```

实现建议：

```java
private void addProductToCart(JSONObject item, Button sourceButton)
```

如果调用方没有按钮，例如后续从聊天卡片复用，可以提供重载：

```java
private void addProductToCart(JSONObject item) {
    addProductToCart(item, null);
}
```

### 4.2 商品页悬浮购物车入口

在商品页右下角新增悬浮按钮：

```text
位置:
  root 右下角
  right = 18dp
  bottom = 76dp
  避开底部商品/活动 tab bar

样式:
  圆形或 52dp icon button
  图标: 购物车符号
  右上角红色 badge
  badge 文案:
    0: 可隐藏红点，或显示空红点
    1-99: 显示数字
    >99: 显示 99+

点击:
  renderCart()
```

数量来源：

- 调用 `api.cart()` 获取购物车。
- 计算所有 `items[].quantity` 总和，而不是只用 `summary.selectedCount`。
- 未登录时隐藏悬浮购物车，或点击跳转登录。建议未登录隐藏，避免商品页视觉噪音。

新增字段：

```java
private FrameLayout productCartFab;
private TextView productCartBadge;
private int cartItemCountCache;
```

新增方法：

```java
private void addProductCartFab()
private void refreshProductCartBadge()
private int cartItemCount(JSONObject cart)
private String cartBadgeText(int count)
```

触发刷新：

- 进入商品列表 tab 后刷新。
- 进入商品活动 tab 后刷新。
- 加入购物车成功后刷新。
- 从购物车返回商品页后刷新。

### 4.3 商品详情返回不闪顶部

当前商品卡片进入详情前会记录：

```java
productsScrollY = productsScroll == null ? 0 : productsScroll.getScrollY();
currentProductState.scrollY = productsScrollY;
renderProductDetail(item.optString("productId"), () -> renderProducts(true));
```

返回时 `renderProducts(true)` 会重建页面，先展示新 ScrollView 顶部，再在 `post()` 中执行：

```java
productsScroll.post(() -> productsScroll.scrollTo(0, currentProductState.scrollY));
```

因此用户看到“先顶部一闪，再回到原位置”。

v17 方案：

1. 返回列表时先隐藏商品内容容器。
2. 从缓存渲染商品列表。
3. 在第一帧布局完成后先 `scrollTo(savedY)`。
4. 再显示内容。

伪代码：

```java
private void renderProductsTab(String tab, boolean restoreScroll) {
    ...
    if (restoreScroll) {
        content.setAlpha(0f);
    }
    renderProductListTab(restoreScroll);
    ...
}

private void restoreProductScrollIfNeeded() {
    if (!restoreProductsScroll || productsScroll == null || currentProductState == null) return;
    int savedY = currentProductState.scrollY;
    productsScroll.post(() -> {
        productsScroll.scrollTo(0, savedY);
        content.setAlpha(1f);
        restoreProductsScroll = false;
    });
}
```

注意：

- 只有返回详情页时隐藏内容；正常进入商品页不隐藏。
- 如果商品缓存不存在，需要加载数据，则显示 loading，不做 alpha 隐藏。
- 如果 `savedY <= 0`，无需隐藏。

更彻底的替代方案是保留商品列表 View，不在详情返回时重建，但对当前单 Activity 手写 UI 结构改动较大，本版建议采用“先 restore 后 reveal”的小改动。

## 5. 购物车页改造

### 5.1 商品图和数量控件间距

当前 `cartItemView()` 结构：

```text
card
  row: checkbox + image + info
  controls: - quantity + delete
```

截图中图片和数量减号视觉重叠，主要是控件行上边距不足，且按钮圆形背景较大。调整：

```java
LinearLayout.LayoutParams controlsParams = new LinearLayout.LayoutParams(-1, dp(46));
controlsParams.topMargin = dp(14);
card.addView(controls, controlsParams);
```

同时将 controls 右侧对齐但与图片行保持垂直间距：

- controls topMargin 从 0 增加到 12-16dp。
- `minus/plus` 按钮尺寸可从 `46x42` 调整为 `44x40`。
- 如果仍拥挤，将 controls 放入单独右对齐容器，避免上浮到图片区域。

### 5.2 结算区文案

当前结算主面板：

```java
checkoutBar.addView(muted("应付：¥" + summary.optString("payAmount", "0")));
```

需求：

- 结算按钮上方应显示“商品总额：”
- “应付”是商品总额减优惠金额后的结果，应在优惠明细中展示

修改为：

```java
checkoutBar.addView(muted("商品总额：¥" + summary.optString("totalAmount", "0")));
```

优惠明细中保留：

```text
商品总额
优惠金额
应付
优惠规则明细
```

如果主面板和优惠明细都展示商品总额，可接受；如果需要减少重复，主面板显示商品总额，优惠明细显示优惠金额和应付。

### 5.3 删除多余底框

截图红框中的多余底框来自当前结构：

```java
LinearLayout checkoutBar = panel();
...
loadDiscountPreview(checkoutBar);

private View discountPreviewCard(JSONObject discount) {
    LinearLayout card = panel();
    ...
    return card;
}
```

也就是在 `checkoutBar` 这个白色圆角 panel 里又嵌套了一个白色圆角 panel。外层和内层底边形成了重复底框。

v17 改为优惠明细内联渲染，不再返回 `panel()`：

```java
private void renderDiscountPreviewRows(LinearLayout parent, JSONObject discount)
```

结构：

```text
checkoutBar panel
  已选 N 件
  商品总额：¥...
  结算按钮
  分隔线
  优惠明细
  优惠金额：¥...
  应付：¥...
  优惠规则行...
```

或将优惠明细作为和 checkoutBar 同级的独立 panel，但不能嵌套在 checkoutBar 内。本版建议内联，能直接消除截图中的重复底框。

## 6. 设置与高级设置

### 6.1 设置页新增入口

当前设置页 group：

```java
group.addView(settingsRow("?", "帮助", ...));
group.addView(settingsRow("i", "关于", ...));
group.addView(settingsRow("→", "退出登录", ...));
```

v17 改为：

```java
group.addView(settingsRow("?", "帮助", ...));
group.addView(settingsRow("i", "关于", ...));
group.addView(settingsRow("⚙", "高级设置", v -> renderAdvancedSettingsPage(), ...));
group.addView(settingsRow("→", "退出登录", ...));
```

### 6.2 高级设置页内容

新增页面：

```java
private void renderAdvancedSettingsPage()
```

页面结构：

```text
顶部栏: 高级设置，左返回到设置页

功能设置:
  [开关] 侧栏显示“返回聊天”

开发设置:
  测试后端地址输入框
  保存测试地址按钮
```

“测试后端”从设置页迁移到高级设置页：

- 设置首页不再直接显示后端地址输入框。
- `BuildConfig.SHOW_TEST_SERVER_SETTINGS == false` 时，开发设置区隐藏。
- 保存逻辑继续复用 `saveApiBaseFromInput()`。

### 6.3 持久化侧栏返回聊天开关

`SessionStore` 新增：

```java
public boolean showDrawerReturnChat() {
    return prefs.getBoolean("show_drawer_return_chat", true);
}

public void saveShowDrawerReturnChat(boolean value) {
    prefs.edit().putBoolean("show_drawer_return_chat", value).apply();
}
```

默认值建议为 `true`，保持当前行为不变；用户在高级设置中关闭后，侧栏不再显示“返回聊天”按钮。

## 7. 侧栏改造

### 7.1 键盘覆盖底部用户栏

`showDrawer()` 打开时调用：

```java
useKeyboardOverlay();
```

关闭侧栏时恢复当前页面键盘策略：

```java
private void restoreKeyboardModeForActivePage() {
    if ("chat".equals(activePage)) {
        useKeyboardResize();
    } else {
        useKeyboardOverlay();
    }
}
```

在 `closeDrawer()` / `closeDrawerAnimated()` 的结束路径调用。

### 7.2 返回聊天按钮可隐藏

当前侧栏无条件创建：

```java
Button returnChat = primaryButton("返回聊天");
historyFrame.addView(returnChat, returnParams);
```

改为：

```java
if (sessionStore.showDrawerReturnChat()) {
    Button returnChat = primaryButton("返回聊天");
    historyFrame.addView(returnChat, returnParams);
}
```

如果隐藏按钮，需要调整 `historyList` bottom padding：

```java
int historyBottomPadding = sessionStore.showDrawerReturnChat() ? dp(72) : dp(16);
historyList.setPadding(0, dp(12), 0, historyBottomPadding);
```

避免底部留出不必要空白。

## 8. 实施步骤

1. 新增键盘模式 helper，并改造聊天页、商品页、侧栏打开/关闭路径。
2. 商品页：
   - `addProductToCart()` 支持按钮 loading/成功态。
   - 新增悬浮购物车按钮和数量刷新。
   - 商品详情返回时使用“restore 后 reveal”策略消除顶部闪动。
3. 购物车页：
   - 调整 cart item controls topMargin。
   - 主结算面板改成“商品总额”。
   - 优惠明细改为内联 rows 或同级 panel，删除嵌套 panel。
4. 设置页：
   - 新增高级设置入口和页面。
   - 迁移测试后端设置。
   - 新增侧栏返回聊天开关持久化。
5. 侧栏：
   - 键盘覆盖底部用户栏。
   - 按设置决定是否展示“返回聊天”按钮。
6. 构建 Android 并在模拟器做回归。

## 9. 回归测试清单

### 9.1 商品页

1. 点击商品搜索栏，键盘弹出后底部“商品列表 / 活动”tab bar 被键盘覆盖，不被顶到键盘上方。
2. 点击“加入购物车”，按钮出现“加入中...”和“已加入”状态。
3. 加入成功后右下角悬浮购物车红点数量增加。
4. 购物车数量超过 99 时 badge 显示 `99+`。
5. 商品列表滚动到中部，进入商品详情，再返回，直接回到原滚动位置，不出现顶部闪烁。

### 9.2 购物车页

1. 商品图片与数量加减控件不重叠，至少有 12dp 视觉间距。
2. 结算按钮上方显示“商品总额：¥...”。
3. 优惠明细中显示“应付：¥...”。
4. 页面底部不再出现截图中的多余内层底框。

### 9.3 设置页

1. 设置页“关于”下方出现“高级设置”。
2. 点击进入高级设置页，左上返回回到设置页。
3. 测试后端输入框只出现在高级设置页，不再出现在设置首页。
4. 保存测试地址后 API client 仍更新生效。

### 9.4 侧栏

1. 点击历史搜索栏，键盘弹出后底部用户栏被键盘覆盖，不被顶上来。
2. 高级设置关闭“侧栏显示返回聊天”后，侧栏不显示“返回聊天”按钮。
3. 重新开启后，侧栏恢复显示“返回聊天”按钮。

## 10. 风险与边界

- 全局从 `adjustResize` 切换到页面级键盘模式时，必须确保聊天页仍然调用 `useKeyboardResize()`，否则聊天输入框可能被键盘遮挡。
- 商品页悬浮购物车数量依赖 `GET /cart`，未登录或接口失败时不要阻塞商品页渲染。
- 购物车 badge 建议统计所有购物车项数量，而不是 selectedCount；否则未选中的购物车商品不会被计入红点。
- 商品详情返回的无闪动修复依赖商品列表缓存。如果缓存被清空，需要退化为 loading 状态，而不是展示空白页。
- 高级设置页新增开关默认值应为 true，避免改变老用户当前侧栏行为。
