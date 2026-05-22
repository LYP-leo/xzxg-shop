# 安卓 APP 技术方案 v6

## 1. 背景

本文针对 [安卓APP_问题_v5.md](./安卓APP_问题_v5.md) 中提出的问题，设计下一版 Android 原生 APP 改造方案。

v5 已经把聊天页从全面屏浮层改成普通布局，但只有聊天页完成了统一。当前仍有三个明显问题：

- 商品、购物车、订单、我的、设置等二级页面顶栏仍使用 `statusBarHeight()`，高度明显大于聊天页。
- 我的页登录/注册按钮之间没有统一间隔。
- 侧栏仍使用 `statusBarHeight()` 和 `navBarHeight()`，导致顶部搜索栏和底部用户栏没有按普通页面逻辑贴边布局。

v6 的目标是把“取消全面屏适配”扩大到整个 APP，而不是只作用于聊天页。

## 2. 设计结论

全 APP 统一采用普通屏幕布局：

```text
页面根布局
  56dp 顶栏
  内容区
  可选底部区域
```

不再在业务 UI 中使用：

```java
statusBarHeight()
navBarHeight()
```

这两个方法可以暂时保留，但除非后续重新设计全面屏，否则页面布局、侧栏布局、输入栏布局都不再调用它们。

## 3. 顶栏统一方案

### 3.1 当前问题

聊天页顶栏已经是：

```text
56dp
```

但二级页面 `addPageHeader()` 仍然是：

```java
header.setPadding(dp(24), statusBarHeight() + dp(12), dp(22), dp(18));
```

这会导致二级页面顶栏比聊天页高很多，页面层级不统一。

### 3.2 新规则

所有页面顶栏统一为：

```text
高度：56dp
左右 padding：16dp
菜单按钮：40dp x 40dp
标题：18sp
副标题：默认不展示，或者降级为内容区说明文字
```

也就是说，二级页面不要再使用大号标题 + 副标题撑高顶栏。

### 3.3 实现方案

新增统一方法：

```java
private View createPageTopBar(String title) {
    LinearLayout toolbar = new LinearLayout(this);
    toolbar.setGravity(Gravity.CENTER_VERTICAL);
    toolbar.setPadding(dp(16), 0, dp(16), 0);
    toolbar.setBackgroundColor(BG_COLOR);

    Button menu = transparentIconButton("☰");
    menu.setTextSize(22);
    toolbar.addView(menu, new LinearLayout.LayoutParams(dp(40), dp(40)));

    TextView titleView = new TextView(this);
    titleView.setText(title);
    titleView.setTextSize(18);
    titleView.setTypeface(Typeface.DEFAULT_BOLD);
    toolbar.addView(titleView, new LinearLayout.LayoutParams(0, -1, 1));

    return toolbar;
}
```

聊天页的 `createCompactTopBar()` 可以改名或复用这个方法，避免两套顶栏代码。

`addPageHeader(title, subtitle)` 改成：

```java
content.addView(createPageTopBar(title), new LinearLayout.LayoutParams(-1, dp(56)));
if (!subtitle.isEmpty()) {
    pageBody 顶部添加 muted(subtitle)
}
```

为了降低改动风险，第一阶段可以保留 `addPageHeader(title, subtitle)` 方法名，但内部实现必须使用统一 56dp 顶栏。

## 4. 我的页按钮间距

### 4.1 当前问题

未登录态的按钮顺序是：

```text
登录
注册顾客账号
注册商家账号
```

但 `注册顾客账号` 和 `注册商家账号` 之间没有稳定 margin，看起来粘在一起。

已登录态的：

```text
修改密码
退出登录
```

也存在同类风险。

### 4.2 新规则

所有表单按钮必须通过统一 helper 加入页面：

```java
private void addFormButton(LinearLayout page, Button button) {
    LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(48));
    params.setMargins(0, 0, 0, dp(10));
    page.addView(button, params);
}
```

输入框沿用当前 `inputField()` 的下边距。

我的页按钮高度统一为：

```text
48dp
```

按钮之间间距统一为：

```text
10dp
```

## 5. 侧栏统一方案

### 5.1 当前问题

侧栏仍然使用：

```java
drawer.setPadding(dp(20), statusBarHeight() + dp(18), dp(18), navBarHeight() + dp(14));
```

这会带来两个问题：

- 搜索栏无法顶到侧栏最上方。
- 底部用户信息和设置按钮被 `navBarHeight()` 推高，和普通页面逻辑不一致。

### 5.2 新侧栏结构

侧栏改成普通三段式：

```text
drawer
  topArea      搜索栏 + 新建按钮，固定高度 62dp
  middleArea   导航 + 历史，weight = 1
  bottomArea   用户信息 + 设置按钮，固定高度 68dp
```

侧栏整体 padding：

```java
drawer.setPadding(dp(14), dp(10), dp(14), dp(10));
```

不要再使用 `statusBarHeight()` 和 `navBarHeight()`。

### 5.3 搜索栏

搜索栏区域顶到侧栏最上方，只保留普通视觉边距：

```text
topArea 高度：62dp
搜索框高度：44dp
搜索框字号：15sp
新建按钮：44dp x 44dp
```

### 5.4 返回聊天按钮

当前返回聊天按钮是白底，视觉权重不够。

改成黑底白字：

```java
Button returnChat = primaryButton("返回聊天");
```

尺寸不再固定为 `180dp x 56dp`，改为包裹内容：

```java
FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(
    ViewGroup.LayoutParams.WRAP_CONTENT,
    dp(44),
    Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL
);
```

按钮左右 padding：

```text
24dp
```

高度：

```text
44dp
```

这样按钮足够明显，但不会显得笨重。

### 5.5 底部用户栏

底部用户栏固定在侧栏最下面：

```text
高度：64dp-68dp
头像：42dp
用户名：16sp
设置按钮：44dp
```

不再添加 `navBarHeight()`。

## 6. 全局 UI 统一清单

除了用户明确指出的问题，v6 需要顺手统一以下残留点。

### 6.1 顶栏

以下页面必须使用同一套 56dp 顶栏：

- 聊天
- 我的
- 设置
- 商品
- 购物车
- 订单
- 通用列表页

### 6.2 页面内容区

页面内容区统一：

```text
左右 padding：16dp
顶部 padding：10dp
底部 padding：16dp
```

当前 `pageBody()` 基本接近，只需要确认不再额外叠加全面屏空白。

### 6.3 按钮

普通表单按钮：

```text
高度：48dp
圆角：22dp
字号：15sp
底部间距：10dp
```

顶部栏图标按钮：

```text
40dp x 40dp
图标：22sp
```

侧栏底部设置按钮：

```text
44dp x 44dp
图标：24sp
```

### 6.4 卡片

卡片继续使用 v5 的缩小规则：

```text
圆角：12dp
padding：14dp x 12dp
底部间距：12dp
```

不要在页面中出现过大的 `dp(18)`、`dp(20)`、`dp(24)` 作为主视觉 padding，除非是独立按钮内部横向 padding。

## 7. 实现步骤

### 第一步：统一顶栏方法

将 `createCompactTopBar()` 改为通用方法，或新增：

```java
private View createTopBar(String title, String rightText)
```

聊天页调用：

```java
content.addView(createTopBar("AI导购", loginText), new LinearLayout.LayoutParams(-1, dp(56)));
```

二级页面 `addPageHeader()` 内部也调用同一个方法。

### 第二步：改造 `addPageHeader()`

删除：

```java
statusBarHeight() + dp(12)
```

改为：

```java
content.addView(createTopBar(title, ""), new LinearLayout.LayoutParams(-1, dp(56)));
```

副标题如果仍需展示，放到内容区第一行，而不是顶栏内。

### 第三步：改造我的页按钮

新增：

```java
private void addFormButton(LinearLayout page, Button button)
```

替换我的页中所有：

```java
page.addView(button)
page.addView(button, new LinearLayout.LayoutParams(-1, dp(52)))
```

### 第四步：改造侧栏 padding 和结构

删除侧栏中的：

```java
statusBarHeight()
navBarHeight()
```

改为普通 padding：

```java
drawer.setPadding(dp(14), dp(10), dp(14), dp(10));
```

并调整：

- 搜索栏高度为 `44dp`。
- 新建按钮为 `44dp`。
- 返回聊天按钮改成黑底白字。
- bottomUserBar 高度和内部控件缩小。

### 第五步：全局检查残留

实现后必须用 `rg` 检查：

```bash
rg -n "statusBarHeight|navBarHeight|dp\\(24\\)|dp\\(20\\)|setTextSize\\(28\\)" android-native/app/src/main/java/com/xzxg/shop/MainActivity.java
```

如果 `statusBarHeight()` / `navBarHeight()` 仍在业务布局中出现，需要逐个说明原因；v6 目标是页面和侧栏不再使用它们。

## 8. 验收标准

### 8.1 顶栏

- 聊天、我的、设置、商品、购物车、订单页面顶栏高度一致。
- 二级页面顶栏不再明显高于聊天页。
- 顶栏不再使用全面屏状态栏高度计算。

### 8.2 我的页

- 登录、注册顾客账号、注册商家账号之间都有清晰间隔。
- 已登录态的保存、修改密码、退出登录按钮也有统一间隔。

### 8.3 侧栏

- 搜索栏位于侧栏顶部，只保留普通边距。
- 用户信息和设置按钮位于侧栏底部。
- 返回聊天按钮黑底白字，视觉上明显。
- 侧栏不再因为状态栏/导航栏高度出现过大上下空白。

### 8.4 全局一致性

- 页面主间距、卡片、按钮、顶栏字号一致。
- 不出现某个页面明显比其它页面“更大、更空、更厚”的情况。

## 9. 模拟器测试要求

实现后必须截图检查：

```text
/private/tmp/xzxg-android-v6-chat.png
/private/tmp/xzxg-android-v6-profile.png
/private/tmp/xzxg-android-v6-products.png
/private/tmp/xzxg-android-v6-cart.png
/private/tmp/xzxg-android-v6-orders.png
/private/tmp/xzxg-android-v6-drawer.png
```

测试路径：

```text
1. 打开聊天页，确认顶栏高度。
2. 打开侧栏，确认搜索栏、返回聊天按钮、底部用户栏。
3. 进入我的页，确认登录/注册按钮间隔。
4. 进入商品、购物车、订单，确认顶栏和聊天页一致。
5. 回到聊天页，打开键盘，确认 v5 输入栏贴键盘效果没有回退。
```

崩溃检查：

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb logcat -d -v time \
  | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|System.err" \
  | tail -120
```

## 10. 风险与边界

- 本版继续坚持“不做全面屏适配”，因此会牺牲状态栏/导航栏融合效果，换取布局稳定和全局一致。
- 侧栏顶部“顶到页面最上面”理解为顶到侧栏内容区域最上方，只保留 `10dp` 普通边距，不再避让状态栏。
- `statusBarHeight()` / `navBarHeight()` 方法可暂时保留，避免无关代码清理扩大范围；但页面布局不再调用它们。
