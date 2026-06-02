# 安卓 APP 技术方案 v13

## 1. 背景

本文针对 [安卓APP_问题_v12.md](./安卓APP_问题_v12.md) 设计下一版 Android 原生 APP 修复方案。

v12 已完成账号资料、头像上传、历史会话搜索接口、设置页和账号管理页的基础闭环。v13 不重新设计账号体系和主业务链路，重点修复当前交互细节和状态同步问题：

- 全局右上角用户入口冗余，影响多数页面顶部导航一致性。
- 登录页处于未登录态，左上角侧栏按钮语义错误。
- 侧栏首次登录后历史会话刷新滞后，搜索态下导航区占用空间，且搜索需要真正筛选历史会话。
- 设置页二级页面返回路径错误，右侧箭头视觉尺寸异常。
- 头像选择后的本地预览和保存时机需要符合用户预期。
- 商品分类弹窗默认选择态左右两列不一致。

## 2. 设计目标

v13 的目标是“小范围修正现有 Java 原生 View 实现”，不引入新的页面框架，不改动业务数据模型，保持 v12 后端接口不变。

验收目标：

1. 所有页面顶部右侧不再出现登录入口、头像或用户名入口。
2. 登录页左上角显示返回按钮，返回到聊天首页未登录态。
3. 登录后第一次打开侧栏即可看到已同步的历史会话。
4. 历史搜索框聚焦时收起四个导航按钮，点击除历史搜索框以外的所有空白区域都可收起键盘。
5. 搜索框输入后立即按标题、摘要和消息内容筛选会话；清空后恢复全部历史。
6. 设置页、帮助页、关于页、账号页、个人资料页返回路径稳定，不直接跳回聊天页。
7. 个人资料页头像裁剪后立即预览，点击“保存”后再上传并更新资料。
8. 商品分类弹窗默认左右两边“全部”都处于选中态。

## 3. 顶部导航改造

### 3.1 删除全局右上角用户入口

当前 `createTopBar(titleText, rightText)` 会调用 `topBarAccountView(rightText)`：

- 已登录时渲染头像+用户名，并点击进入设置。
- 未登录时渲染“登录”。
- `addPageHeader()` 传入空字符串时也会得到一个可点击的右侧状态位。

v13 调整：

```text
createTopBar(title, mode)
  left:
    默认为菜单按钮
    login/detail/secondary 页面可指定为返回按钮
  right:
    所有页面默认不展示登录入口、头像或用户名入口
```

实施建议：

- 删除或停用 `topBarAccountView()` 的整体调用，不再在顶部栏渲染“登录”、头像或用户名。
- `createTopBar("AI导购", null)` 在未登录 chat 页右侧也保持为空。
- `renderSettings()`、`renderProducts()`、`renderCart()`、`renderOrders()`、`renderHelpPage()`、`renderAboutPage()` 等页面不再展示右上角用户入口。
- 设置页入口只保留侧栏底部用户栏和齿轮按钮；未登录时通过左上角入口进入登录页。

### 3.2 登录页左上角改为返回按钮

当前登录页使用 `addPageHeader("登录", "账号登录和注册")`，左上角仍是菜单按钮。v13 为登录页提供独立 header：

```text
renderLoginPage()
  activePage = "login"
  header left = "‹" 或 "←"
  header title = "登录"
  header right = empty
  left click:
    renderChatHome()
    loadHomeCopy()
```

登录页不允许打开侧栏，因为未登录时侧栏没有有效历史和账号操作。

## 4. 侧栏修复

### 4.1 首次登录历史不显示问题

当前问题根因：

- 登录成功后 `createFreshLocalSession()` 和 `renderChatHome()` 会立刻回到首页。
- `syncRemoteSessions()` 只在 `showDrawer()` 里异步调用。
- `showDrawer()` 先用 `chatStore.recentSessionsWithMessages()` 渲染本地列表，再异步同步服务端；同步完成后没有刷新当前 `historyList`。
- 因此第一次打开侧栏时本地为空，关闭后再次打开才看到上次同步写入的结果。

v13 调整：

```text
login/register 成功
  saveAccountSession()
  syncRemoteSessions(callback = null)
  renderChatHome()

showDrawer()
  renderHistoryList(local current)
  syncRemoteSessions(callback = renderHistoryList(latest local))
```

具体改法：

- 将 `syncRemoteSessions()` 改成 `syncRemoteSessions(Runnable onDone)` 或返回同步后的 `List<SessionSummary>`。
- 在 `showDrawer()` 创建 `historyList` 后调用同步，成功后在 UI 线程重新读取 `chatStore.recentSessionsWithMessages()` 并刷新。
- 如果搜索框当前不为空，不用全量刷新，改为重新执行当前搜索，避免覆盖用户搜索结果。

### 4.2 搜索态收起导航区和键盘

当前侧栏结构：

```text
search row
navGroup: AI导购 / 商品 / 购物车 / 订单
divider
historyFrame
bottomUserBar
```

v13 增加搜索焦点状态：

```text
search has focus or keyboard visible:
  navGroup.visibility = GONE
  divider.visibility = GONE
  historyList top padding reduced

search loses focus and keyboard hidden:
  navGroup.visibility = VISIBLE
  divider.visibility = VISIBLE
```

键盘收起规则：

```text
点击除历史搜索框以外的所有空白区域:
  hideKeyboard()
  search.clearFocus()
```

需要覆盖的空白区域：

- 右侧半透明遮罩区域。
- 抽屉面板内搜索框下方的空白区域。
- 历史列表内容不足一屏时剩余的空白区域。
- 历史列表行之间的空白和底部空白。
- 侧栏底部用户栏/设置栏的背景空白。

实现建议：

- 对搜索框设置 `setOnFocusChangeListener`。
- 结合 `WindowInsets.Type.ime()` 判断键盘是否显示，避免只依赖焦点导致返回键收起键盘后导航区不恢复。
- `drawerLayer` 保持点击关闭抽屉，但如果键盘正在显示，第一次点击遮罩优先只收键盘并清除搜索焦点；再次点击再关闭抽屉。
- `drawer`、`historyFrame`、`historyScroll`、`historyList`、`bottomUserBar` 的背景空白区都设置 `hideKeyboardAndClearSearchFocus(search)`。
- 搜索框自身点击不触发收键盘。
- 历史会话行、导航按钮、新建会话按钮、底部设置按钮保持原有点击行为；这些控件点击前可先调用 `hideKeyboard()`，但不能被父级空白点击监听吞掉。

### 4.3 历史会话搜索规则

用户期望是“在历史会话中筛选出包含输入文字的会话”。v13 定义搜索范围为：

- 会话标题 `title`
- 会话摘要 `summary`
- 该会话内用户消息和助手消息正文 `messages.content`

优先级：

1. 本地已缓存会话先即时筛选，保证输入后马上有反馈。
2. 已登录时再请求服务端 `/api/v1/agent/sessions/search?q=...`，补齐跨设备/未缓存会话。
3. 服务端结果写入本地后刷新展示。
4. 搜索框清空时展示 `recentSessionsWithMessages()` 全量历史。

Android 新增本地方法：

```java
List<SessionSummary> searchSessionsLocal(String query)
```

SQL 规则：

```sql
SELECT DISTINCT s.local_session_id, s.server_session_id, s.title, s.summary, s.sync_state
FROM sessions s
LEFT JOIN messages m ON m.local_session_id = s.local_session_id
WHERE
  (s.server_session_id IS NOT NULL AND s.server_session_id != ''
   OR EXISTS (SELECT 1 FROM messages um WHERE um.local_session_id = s.local_session_id AND um.role = 'user'))
  AND (
    s.title LIKE ?
    OR s.summary LIKE ?
    OR m.content LIKE ?
  )
ORDER BY s.updated_at DESC
LIMIT 50;
```

服务端确认：

- 如果 `SearchUserSessions` 当前只查 `chat_sessions.title/summary`，需要扩展为 join `user_messages.content`。
- 仍然只返回当前账号自己的 `message_count > 0` 会话。
- 搜索参数 trim 后为空时不请求服务端，直接展示全部。

### 4.4 侧栏底部背景

`bottomUserBar()` 当前背景是纯白，和历史区不区分。v13 调整为：

```text
bottomUserBar background = #F1F3F5 或 #F3F4F6
top border = #E5E7EB 1px
padding top/bottom 保持 6-10dp
```

如需视觉更稳定，可用一个外层 `LinearLayout` 包住底部栏和顶部细线。

## 5. 设置页和二级页

### 5.1 设置行右侧箭头

当前 `settingsRow()` 右侧 “>” 号过大，超过按钮高度。v13 改为固定小字号文本或轻量图标：

```text
arrow text = "›"
arrow textSize = 24sp
arrow color = #9CA3AF
arrow layout = 28dp width, match height
arrow gravity = CENTER
```

不要复用主按钮字体大小，也不要使用 `title()` 或加粗样式。

### 5.2 二级页面返回路径

当前帮助/关于页面调用 `addPageHeader()`，该 header 左侧是菜单按钮；系统返回键由于 `backStack` 状态不一致，容易直接回聊天页。

v13 为设置相关页面建立统一返回栈：

```text
renderSettings()
  backStack = [renderChatHome]

renderHelpPage()
  activePage = "help"
  header left = back
  back action = renderSettings()
  不清空 backStack

renderAboutPage()
  activePage = "about"
  header left = back
  back action = renderSettings()
  不清空 backStack

renderAccountPage()
  back action = renderSettings()

renderEditProfilePage()
  back/cancel action = renderSettings() 或 renderAccountPage()，取决于入口来源
```

推荐增加轻量方法：

```java
private void pushPage(String page, Runnable backAction)
private View createBackTopBar(String title, Runnable backAction)
```

二级页面顶部右侧保持空，不放用户入口。

### 5.3 Android 返回键策略

`onBackPressed()` 调整顺序：

```text
drawer open -> close drawer
activePage == login -> renderChatHome
activePage in help/about -> renderSettings
activePage == account -> renderSettings
activePage == edit_profile/avatar -> renderSettings 或上级页面
activePage in primary pages -> showDrawer only when logged in, otherwise renderLoginPage or finish
else -> renderChatHome
```

这样帮助/关于不会越过设置页直接回聊天页。

## 6. 头像与个人资料保存

### 6.1 头像预览页左上角返回

`renderAvatarPreviewPage()` 当前需要显式使用返回 header：

```text
title = ""
left = back
right = empty
back action = renderSettings()
```

页面内保留长按保存到相册能力。

### 6.2 裁剪后实时预览

当前 `pendingAvatarChanged` 只 toast，没有替换页面头像。v13 调整：

- 新增字段：

```java
private Bitmap pendingAvatarPreview;
```

- `onActivityResult()` 裁剪成功后：

```text
pendingAvatarBytes = square jpeg bytes
pendingAvatarPreview = decoded bitmap
pendingAvatarMime = image/jpeg
pendingAvatarChanged.run()
```

- `renderEditProfilePage()` 中头像区域使用一个可更新的 `ImageView`：

```text
if pendingAvatarPreview != null:
  imageView.setImageBitmap(pendingAvatarPreview)
else if sessionStore.avatarUrl not empty:
  imageLoader.load(imageView, ...)
else:
  show initial avatar
```

用户裁剪后不立即上传，先只更新本地预览。

### 6.3 “完成”改为“保存”

`renderEditProfilePage()` 顶部右侧文案：

```text
完成 -> 保存
```

保存按钮启用条件：

- 昵称和原昵称不同。
- 或 `pendingAvatarBytes != null`。

禁用态使用浅蓝或灰色；启用态使用主蓝色。

### 6.4 保存时再上传和更新资料

保存流程：

```text
点击保存
  validate nickname
  disable save button, show "保存中..."
  if pendingAvatarBytes != null:
    POST /api/v1/uploads/avatar
    avatarUrl = response.url
  PATCH /api/v1/account/profile { display_name, avatar_url }
  saveAccountSession(token, profile, ...)
  clear pending avatar
  renderSettings()
```

如果只有昵称变化，不上传头像。

如果只有头像变化，`display_name` 仍传当前昵称，避免后端因为空昵称或空头像判断为无效请求。

### 6.5 “保存失败 404 not found”的原因和修复

从当前代码看，Android 请求路径是：

```text
POST  <apiBase>/uploads/avatar
PATCH <apiBase>/account/profile
```

当 `apiBase = http://host:8080/api/v1` 时，实际路径分别是：

```text
POST  /api/v1/uploads/avatar
PATCH /api/v1/account/profile
```

这和 v12 后端路由一致。因此 404 的高概率原因是：

1. APP 当前连接的是旧后端实例，旧后端没有 `/api/v1/uploads/avatar` 或 `/api/v1/account/profile`。
2. 远程后端未部署 v12 后端代码，只部署了 Android。
3. 测试后端地址不是 `/api/v1` 结尾，导致拼接到错误路径。
4. 本地后端启动的不是更新后的代码目录。

v13 修复要求：

- 登录页和设置页测试后端地址保存时做规范化：

```text
去掉末尾 /
如果不包含 /api/v1，则提示“后端地址应包含 /api/v1”
```

- 保存资料前先调用：

```text
GET /api/v1/account/profile
```

如果返回 404，提示：

```text
当前后端不是最新版，请切换到已部署账号资料接口的后端
```

- 后端部署验收必须执行：

```bash
curl -i http://127.0.0.1:8080/api/v1/health
curl -i -H "Authorization: Bearer <token>" http://127.0.0.1:8080/api/v1/account/profile
```

## 7. 商品分类弹窗

当前 `addPrimaryCategoryButton(primary, secondary, "全部", null, ...)` 总是用 `categoryRow(name, false)`，所以左侧“全部”默认不选中；右侧通过 `categoryId.equals(pendingId[0])` 选中。

v13 调整：

```java
private void addPrimaryCategoryButton(..., boolean selected)
```

默认状态：

```text
lastCategoryId == "":
  primary "全部" selected = true
  secondary "全部" selected = true
```

点击左侧任一主类时：

- 清空左侧所有选中背景。
- 当前主类置灰。
- 右侧重新渲染该主类下的“该类全部”和子类。
- `pendingId/pendingName` 只有在用户点击右侧条目时才最终变更；或点击左侧主类时默认选择该主类全部，两者需要保持一致。

推荐行为：

```text
点击左侧主类 = 默认选中右侧“该类全部”
```

这样用户点“大类”后直接点确定也符合预期。

## 8. 实施顺序

1. 导航层：改造 `createTopBar()` / `addPageHeader()`，删除全局右上角用户入口，登录页独立返回 header。
2. 返回栈：修复 `onBackPressed()`，为设置二级页、账号页、头像页提供明确上级。
3. 侧栏：同步完成后刷新当前历史列表；搜索聚焦时隐藏导航区；点击除搜索框以外的空白区域收键盘。
4. 搜索：实现 `LocalChatStore.searchSessionsLocal()`，并确认后端搜索覆盖消息内容。
5. 设置 UI：缩小 `settingsRow()` 右侧箭头；侧栏底部加浅灰背景。
6. 头像：本地裁剪预览、保存按钮文案、保存时上传。
7. 商品分类：修正左右两列“全部”默认选中态。
8. 回归测试：构建、后端接口、模拟器关键路径。

## 9. 回归测试清单

### 9.1 后端

```bash
cd backend
GOTOOLCHAIN=local GOCACHE=/private/tmp/xzxg-go124-build GOPATH=/private/tmp/xzxg-go124 go1.24.0 test ./...
```

重点检查：

- `GET /api/v1/account/profile`
- `POST /api/v1/uploads/avatar`
- `PATCH /api/v1/account/profile`
- `GET /api/v1/agent/sessions/search?q=手机`

### 9.2 Android 构建

共享盘如仍存在 Gradle 写入限制，继续使用临时目录构建：

```bash
ditto /Volumes/shared/xzxg-shop/android-native /private/tmp/xzxg-android-build
cd /private/tmp/xzxg-android-build
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

### 9.3 模拟器手工验收

1. 未登录首页：右上角为空，左上角入口进入登录页。
2. 登录页：左上角是返回，不是侧栏菜单。
3. 登录后任意页面：右上角不再有头像+用户名入口。
4. 第一次登录后打开侧栏：历史会话无需关闭再打开即可显示。
5. 搜索框聚焦：四个导航按钮消失；点击除搜索框以外的空白区域键盘收起，导航恢复。
6. 搜索“手机”：只展示包含“手机”的会话；清空恢复全部。
7. 设置页：帮助/关于/退出登录右侧箭头不超高。
8. 帮助/关于按返回：回到设置页。
9. 点击头像预览：左上角可返回设置页。
10. 编辑个人资料：选择头像后立即预览；右上角显示“保存”；保存后资料更新成功。
11. 商品分类弹窗：默认左右两列“全部”都为选中态。

## 10. 风险与边界

- v13 不新增账号字段，不改变 v12 后端数据库结构。
- 头像上传 404 不应在 Android 端绕过；必须确认连接的是包含 v12 路由的后端。
- 历史搜索如果服务端未覆盖消息内容，本地已缓存会话可以先工作，但跨设备搜索会不完整，需要同步修复后端 SQL。
- 删除右上角用户和登录入口后，设置入口只剩侧栏底部；未登录登录入口只剩首页左上角入口，必须保证该入口稳定进入登录页。
