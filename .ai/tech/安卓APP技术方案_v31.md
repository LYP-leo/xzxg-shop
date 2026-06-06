# 安卓 APP 技术方案 v31

## 1. 背景

本文针对 [安卓APP_问题_v30.md](./安卓APP_问题_v30.md) 设计 Android 原生 APP 下一版小修方案。

本轮只处理两个明确问题：

1. 账号管理页中手机号、邮箱、删除账号、退出登录的底部弹窗出现动画怪异。
2. 未登录时聊天主界面欢迎语显示“晚上好，用户名”，应改为“晚上好”。

修改原则：

1. 不改后端接口、不改 HTTP 协议、不改账号/会话/聊天业务流程。
2. 不重做弹窗样式，只去掉问题中提到的出现/消失动画，并保持底部出现。
3. 欢迎语只调整未登录态文案；已登录态继续展示“时间问候 + 昵称”。
4. 修改范围控制在 `MainActivity.java`，避免牵动其它模块。

## 2. 当前实现核对

### 2.1 账号管理页弹窗

当前账号管理页四个入口：

```java
accountInfoRow("☎", "手机号", ..., v -> editContact(true))
accountInfoRow("@", "邮箱", ..., v -> editContact(false))
settingsRow("×", "删除账号", v -> confirmDeleteAccount(), ...)
settingsRow("→", "退出登录", v -> confirmLogout(), ...)
```

实现链路：

- 手机号/邮箱：`editContact(boolean phone)` 创建 `makeBottomSheetBox()`，最后调用 `showBottomSheetDialog(box)`。
- 删除账号/退出登录：`confirmDeleteAccount()` / `confirmLogout()` 调用 `showConfirmBottomSheet(...)`，内部也调用 `showBottomSheetDialog(box)`。

当前统一底部弹窗方法：

```java
private AlertDialog showBottomSheetDialog(View body) {
    AlertDialog dialog = new AlertDialog.Builder(this).create();
    dialog.setView(body);
    dialog.setOnShowListener(d -> {
        Window window = dialog.getWindow();
        window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
        window.setGravity(Gravity.BOTTOM);
        ...
    });
    dialog.show();
    return dialog;
}
```

问题判断：

- 弹窗位置已经设置为 `Gravity.BOTTOM`。
- 用户看到“从屏幕中间/页面最下面怪异出现”的主要原因大概率是系统 `AlertDialog` 默认 window animation 与设置 `Gravity.BOTTOM` 叠加后视觉不自然。
- 该问题不需要改布局，只需要为底部弹窗窗口禁用动画。

### 2.2 主界面欢迎语

当前欢迎语创建和刷新：

```java
welcome.setText(greetingText());
...
welcome.setText(greetingText());
```

当前 `greetingText()`：

```java
return greeting + "，" + displayNickname();
```

当前 `displayNickname()`：

```java
return nickname == null || nickname.trim().isEmpty() ? "用户名" : nickname.trim();
```

问题判断：

- 未登录时 `sessionStore.nickname()` 为空，因此回退为“用户名”。
- 所以未登录欢迎语会显示“晚上好，用户名”。
- 应区分登录态：未登录只显示时间问候；已登录且昵称为空时再回退“用户名”。

## 3. 目标

1. 点击账号管理页的手机号、邮箱、删除账号、退出登录后，弹窗不再有进入/退出动画。
2. 弹窗仍直接位于页面底部，并保留安全区 padding、遮罩、圆角、按钮和原有校验逻辑。
3. 未登录时欢迎语显示：

```text
早上好
中午好
晚上好
夜深了
```

4. 已登录时欢迎语保持：

```text
早上好，张三
中午好，张三
晚上好，张三
夜深了，张三
```

已登录但昵称为空时可继续显示“用户名”，避免影响已有账号页兜底逻辑。

## 4. 弹窗方案

### 4.1 新增无动画底部弹窗入口

为了不影响其它页面已经使用 `showBottomSheetDialog()` 的弹窗行为，建议新增重载：

```java
private AlertDialog showBottomSheetDialog(View body, boolean animated)
```

原方法保留并委托：

```java
private AlertDialog showBottomSheetDialog(View body) {
    return showBottomSheetDialog(body, true);
}
```

账号管理页四个入口使用：

```java
showBottomSheetDialog(box, false)
```

这样其它页面，如分类选择、评价、订单项选择等底部弹窗不会被本轮改动影响。

### 4.2 禁用 window 动画

在 `showBottomSheetDialog(View body, boolean animated)` 的 `onShow` 中：

```java
if (!animated) {
    window.setWindowAnimations(0);
}
```

同时建议在 `dialog.show()` 之前也尽量设置一次 no animation，避免部分 ROM 在 `onShow` 前已经启动默认动画：

```java
dialog.setOnShowListener(...);
dialog.show();
Window window = dialog.getWindow();
if (window != null && !animated) {
    window.setWindowAnimations(0);
}
```

但核心仍放在 `onShow` 内，因为窗口对象通常在 `show()` 后才可靠。

### 4.3 账号管理页调用点

#### 手机号/邮箱

当前：

```java
dialogRef[0] = showBottomSheetDialog(box);
```

改为：

```java
dialogRef[0] = showBottomSheetDialog(box, false);
```

范围：`editContact(boolean phone)`。

#### 删除账号/退出登录

`showConfirmBottomSheet(...)` 当前是通用确认底部弹窗。为避免影响其它未来调用，新增参数：

```java
private void showConfirmBottomSheet(
    String titleText,
    String message,
    String confirmText,
    boolean danger,
    boolean animated,
    Runnable onConfirm
)
```

原方法保留并委托：

```java
private void showConfirmBottomSheet(String titleText, String message, String confirmText, boolean danger, Runnable onConfirm) {
    showConfirmBottomSheet(titleText, message, confirmText, danger, true, onConfirm);
}
```

账号管理页两个调用改为：

```java
showConfirmBottomSheet("退出登录", "...", "退出", false, false, this::logoutAccount);
showConfirmBottomSheet("删除账号", "...", "删除", true, false, this::deleteAccount);
```

内部最后：

```java
dialogRef[0] = showBottomSheetDialog(box, animated);
```

### 4.4 不改内容

本轮不改：

- `makeBottomSheetBox()` 的 padding、圆角、背景。
- `currentBottomSafeInset()` 的安全区逻辑。
- 手机号/邮箱校验和保存逻辑。
- 退出登录、删除账号的 API 调用和页面跳转逻辑。
- AlertDialog 遮罩 `dimAmount`。

## 5. 欢迎语方案

### 5.1 拆分问候和昵称拼接

建议新增：

```java
private String greetingPrefix()
```

只负责按时间返回：

```java
早上好 / 中午好 / 晚上好 / 夜深了
```

`greetingText()` 改为：

```java
private String greetingText() {
    String greeting = greetingPrefix();
    if (sessionStore == null || sessionStore.token().isEmpty()) {
        return greeting;
    }
    return greeting + "，" + displayNickname();
}
```

### 5.2 昵称兜底只服务已登录态

`displayNickname()` 可以保持现状：

```java
private String displayNickname() {
    String nickname = sessionStore == null ? "" : sessionStore.nickname();
    return nickname == null || nickname.trim().isEmpty() ? "用户名" : nickname.trim();
}
```

因为 `greetingText()` 未登录时不再调用 `displayNickname()` 拼接，所以不会再出现“晚上好，用户名”。

### 5.3 登录态变化后刷新

当前登录、退出登录、删除账号后会重新 `renderChatHome()` / `loadHomeCopy()`，欢迎语会重新创建和刷新。

因此本轮无需新增事件总线或观察器。

## 6. 修改清单

目标文件：

```text
android-native/app/src/main/java/com/xzxg/shop/MainActivity.java
```

计划修改：

1. 新增 `showBottomSheetDialog(View body, boolean animated)`。
2. 原 `showBottomSheetDialog(View body)` 委托到 animated=true。
3. `editContact(boolean phone)` 改用 `showBottomSheetDialog(box, false)`。
4. 新增 `showConfirmBottomSheet(..., boolean animated, Runnable onConfirm)`。
5. 原 `showConfirmBottomSheet(..., Runnable onConfirm)` 委托 animated=true。
6. `confirmLogout()` 改用 animated=false。
7. `confirmDeleteAccount()` 改用 animated=false。
8. 将 `greetingText()` 改成未登录不拼昵称。
9. 可选新增 `greetingPrefix()`，降低 `greetingText()` 复杂度。

## 7. 验证方案

### 7.1 编译验证

继续使用临时目录构建，规避共享卷 Gradle FileHasher 问题：

```bash
cp -R android-native/. /private/tmp/xzxg-android-v31-build/
cd /private/tmp/xzxg-android-v31-build
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

预期：

- `assembleDebug` 成功。
- 允许保留当前 JDK/source target 8、deprecated API 警告。

### 7.2 功能回归

未登录主界面：

1. 清除登录态或退出登录。
2. 回到聊天首页。
3. 按当前时间确认显示：
   - 白天：`早上好` / `中午好` / `晚上好`
   - 深夜：`夜深了`
4. 不应出现 `，用户名`。

已登录主界面：

1. 登录后回到聊天首页。
2. 有昵称时显示 `晚上好，昵称`。
3. 昵称为空时仍允许显示 `晚上好，用户名`，保持已登录兜底。

账号管理页：

1. 点击手机号。
2. 点击邮箱。
3. 点击退出登录。
4. 点击删除账号。

预期：

- 四个弹窗均直接出现在底部。
- 不出现从屏幕中间滑动/落到底部的怪异动画。
- 遮罩、圆角、底部安全区、取消/确认按钮保留。
- 手机号/邮箱输入校验仍可用。
- 取消按钮、保存按钮、退出/删除确认逻辑不变。

### 7.3 非目标页面确认

因为默认 `showBottomSheetDialog(body)` 仍保持 animated=true，以下弹窗理论上不受影响：

- 商品分类选择弹窗。
- 订单评价弹窗。
- 订单商品选择弹窗。
- 其它复用默认底部弹窗的方法。

若实测发现默认弹窗也存在同类怪异动画，可在下一轮统一改默认动画策略；本轮先按问题范围最小化修改。

## 8. 风险与规避

### 8.1 不同 ROM 对 `setWindowAnimations(0)` 支持差异

风险：少数 ROM 可能仍保留极短默认 alpha 动画。

规避：

- 同时在 `onShow` 内和 `show()` 后设置 `window.setWindowAnimations(0)`。
- 不自定义主题，避免扩大影响。

### 8.2 未登录判断依赖 token

风险：token 为空但 nickname 仍残留时，欢迎语仍应按未登录处理。

规避：

- 以 `sessionStore.token().isEmpty()` 作为登录态判断，不以 nickname 判断。

### 8.3 影响其它确认弹窗

风险：如果直接修改 `showConfirmBottomSheet` 默认行为，可能影响其它页面。

规避：

- 保留原签名默认 animated=true。
- 只有账号管理页两个确认操作显式传 `animated=false`。

## 9. 结论

v31 是小范围 UI 行为修复：

1. 用无动画底部弹窗重载解决账号管理页四个弹窗出现动画异常。
2. 用登录态判断修正未登录欢迎语，未登录只显示“晚上好”等问候词。

方案不改变后端、不改变业务流程、不影响其它底部弹窗默认行为，适合直接进入实现。
