# 安卓 APP 技术方案 v2

## 1. 背景

本文针对 [安卓APP_问题_v1.md](./安卓APP_问题_v1.md) 中提出的 UI、侧栏、动画和 Toast 问题，设计下一版 Android 原生 APP 的改造方案。

本次版本目标不是新增复杂业务，而是先把顾客端的基础体验打磨到可演示、可继续迭代的状态：

- 首页像 AI 聊天软件，而不是普通表单页。
- 侧栏像豆包、ChatGPT 一类移动端 AI 应用，承担导航和历史会话职责。
- 页面层级清晰，商品、购物车、订单、我的、设置不塞进聊天主页。
- 动画和反馈自然，不出现连续 Toast 排队的问题。
- 每次实现后必须用 Android 模拟器实际验证。

## 2. 当前问题归纳

### 2.1 首页

当前首页存在的问题：

- 左上角侧栏按钮有按钮背景，和参考图不一致。
- 底部 `+`、输入框、语音按钮高度和视觉中心不一致。
- 右上角登录状态只是文本，不像可点击入口。

下一版要求：

- 左上角侧栏按钮使用透明背景图标按钮。
- 底部输入区严格使用一条胶囊形输入栏，`+`、输入区域、语音/发送按钮在同一垂直中心线上。
- 右上角用户状态可点击，点击进入“我的”页面。

### 2.2 侧栏

当前侧栏存在的问题：

- 信息层级不符合参考图。
- 功能入口没有当前页面高亮。
- 历史会话展示方式不自然。
- 底部用户区和设置入口不清晰。
- 打开/关闭没有抽屉动画。

下一版要求：

- 顶部：搜索栏 + 新建会话按钮。
- 中部：`AI导购`、`商品`、`购物车`、`订单` 四个导航入口。
- 当前页面入口使用浅灰背景，其余入口白底。
- 每个入口左侧有语义图标。
- 历史聊天列表可滚动，不使用卡片式按钮背景。
- 历史区域悬浮“返回聊天”按钮。
- 底部左侧展示头像和用户名，点击进入“我的”。
- 底部右侧只保留“设置”按钮，点击进入设置页。

### 2.3 其他页面

当前商品、购物车、订单、我的页面顶部标题和侧栏按钮靠得太近。

下一版要求：

- 所有二级页面统一使用 `PageHeader`。
- 左侧是透明侧栏图标按钮。
- 标题区域和侧栏按钮之间保持稳定间距。
- 标题和副标题垂直排列，页面内容从标题下方留白后开始。

### 2.4 Toast

当前连续点击按钮时，多个 Toast 会排队显示。

下一版要求：

- 新 Toast 出现前必须取消旧 Toast。
- 用户只看到最新的一条提示。
- 不把普通 Toast 全部写入聊天流，避免污染聊天记录。

## 3. 设计原则

### 3.1 聊天优先

顾客端打开 APP 后默认进入 AI 导购聊天页。首页只保留：

- 侧栏入口。
- 用户入口。
- 欢迎语。
- 场景建议。
- 底部输入栏。

商品、购物车、订单、我的、设置统一从侧栏或用户入口进入。

### 3.2 移动端层级优先

不要把 Web 端的多个模块平铺到一个页面。移动端采用：

```text
聊天主页
  -> 左侧侧栏
      -> AI导购
      -> 商品
      -> 购物车
      -> 订单
      -> 历史聊天
      -> 我的
      -> 设置
```

### 3.3 视觉统一

统一使用：

- 背景色：`#F8F9FB`
- 主文本：`#111827`
- 次文本：`#6B7280`
- 弱边框：`#EEF0F3`
- 选中背景：`#F5F6F8`
- 主按钮：黑底白字
- 输入栏：白底胶囊

按钮不能因为 Android 默认 `Button` 样式出现不受控阴影或大面积背景。图标按钮必须清理默认背景、最小宽高和状态动画。

## 4. 页面方案

## 4.1 聊天主页

### 页面结构

```text
状态栏

顶部栏
  左侧：透明侧栏按钮
  右侧：用户状态 / 昵称入口

内容区
  新会话欢迎语
  场景化建议
  发送后展示消息流

底部输入区
  胶囊容器
    左侧：+
    中间：文本输入区
    右侧：语音 / 发送 / 停止按钮
```

### 顶部栏

侧栏按钮：

- 使用透明背景。
- 点击打开侧栏。
- 不使用圆形白底，不使用阴影。

用户入口：

- 未登录显示 `未登录`。
- 已登录显示 `已登录` 或昵称。
- 点击进入“我的”页面。

### 欢迎区

欢迎语来自：

```http
GET /api/v1/agent/home
```

规则：

- 后端返回 `welcome_text` 时展示返回值。
- 后端为空时展示轻占位文本。
- 场景化建议只展示后端返回的 `suggestions`。
- 未登录时不展示个性化建议，只展示登录提示。

### 底部输入栏

底部输入栏参考 `assets/main.jpeg`。

尺寸规则：

| 元素 | 建议尺寸 |
| --- | --- |
| 外层底部区域左右 padding | 28dp |
| 胶囊高度 | 64dp |
| 胶囊圆角 | 28dp |
| `+` 点击区域 | 42dp x 56dp |
| 输入区高度 | 56dp |
| 右侧圆形按钮 | 46dp x 46dp |

状态规则：

| 状态 | 右侧按钮 |
| --- | --- |
| 输入为空 | 语音图标 |
| 输入非空 | 发送图标 |
| SSE 生成中 | 停止图标 |

交互规则：

- 点击 `+`：第一阶段提示图片入口待接入。
- 点击语音：第一阶段提示语音输入待接入。
- 输入非空点击发送：发送聊天消息。
- SSE 生成中点击停止：后续接 `POST /api/v1/agent/runs/{run_id}/cancel`。

## 4.2 侧栏

侧栏参考 `assets/side-new.jpg`。

### 布局结构

```text
侧栏容器
  顶部操作区
    搜索栏
    新建会话按钮

  主导航
    AI导购
    商品
    购物车
    订单

  分隔线

  历史聊天滚动区
    会话 1
    会话 2
    ...
    悬浮：返回聊天按钮

  底部用户区
    头像 + 用户名
    设置按钮
```

### 顶部操作区

搜索栏：

- 第一阶段只做 UI，不做搜索逻辑。
- 文案为 `搜索`。
- 背景浅灰，圆角。

新建会话按钮：

- 使用编辑/新建语义图标。
- 点击后创建新本地会话。
- 创建新会话时，旧当前会话进入历史列表。
- 新会话页关闭侧栏并回到聊天主页。

### 主导航

主导航包含：

| 页面 | activePage 值 | 图标建议 |
| --- | --- | --- |
| AI导购 | `chat` | 星光 / AI |
| 商品 | `products` | 方块 / 商品 |
| 购物车 | `cart` | 购物车 |
| 订单 | `orders` | 列表 / 单据 |

规则：

- 当前页面按钮浅灰底。
- 非当前页面按钮白底。
- 不使用卡片阴影。
- 点击后关闭侧栏并切换页面。

### 历史聊天

历史聊天来自本地 SQLite：

```text
sessions
messages
```

展示规则：

- 刚进入 APP 时，如果只有当前新会话，历史区为空。
- 新建会话后，旧会话出现在历史列表中。
- 当前会话不显示在历史列表里。
- 历史数量多时，列表可以滚动。
- 历史项不使用按钮卡片背景，只使用头像 + 标题行。

历史项标题规则：

1. 优先使用服务端会话标题。
2. 其次使用本地会话标题。
3. 如果还是为空，展示 `导购会话`。

后续扩展：

- 点击历史项加载本地消息。
- 如果本地没有完整消息，则调用服务端会话详情接口拉取。

### 返回聊天悬浮按钮

历史区域底部悬浮一个 `返回聊天` 按钮。

规则：

- 点击关闭侧栏并回到当前聊天。
- 如果当前没有本地会话，则创建新会话。
- 按钮始终悬浮在历史滚动区之上，不随着历史列表滚走。

### 底部用户区

左侧：

- 默认头像。
- 用户名 / 未登录。
- 点击头像或用户名进入“我的”页面。

右侧：

- 只保留设置按钮。
- 点击进入“设置”页面。

## 4.3 商品页

商品页不是本轮重点，但需要符合统一布局。

结构：

```text
顶部 PageHeader
  侧栏按钮
  商品
  页面说明

商品列表
  商品名
  价格
```

要求：

- 顶部标题与侧栏按钮保持距离。
- 商品列表卡片间距统一。
- 侧栏打开后，商品入口高亮。

## 4.4 购物车页

结构：

```text
顶部 PageHeader
购物车内容
```

第一阶段：

- 未登录展示登录提示。
- 已登录展示购物车摘要。

下一阶段：

- 展示购物车项。
- 数量调整。
- 删除。
- 勾选。
- 结算。

## 4.5 订单页

结构：

```text
顶部 PageHeader
订单列表
```

要求：

- 侧栏打开后订单入口高亮。
- 订单卡片展示订单号、状态、金额。

## 4.6 我的页

入口：

- 首页右上角用户状态。
- 侧栏底部头像/用户名。

页面结构：

```text
顶部 PageHeader

未登录：
  账号
  密码
  登录
  注册顾客账号
  注册商家账号

已登录：
  当前账号卡片
  昵称
  保存资料
  原密码
  新密码
  修改密码
  退出登录
```

注意：

- 测试版后端地址不再放在“我的”主流程里，应迁移到“设置”页。
- 如果保留兼容入口，也必须保持布局整洁。

## 4.7 设置页

入口：

- 侧栏底部右侧设置按钮。

页面结构：

```text
顶部 PageHeader

Debug 版：
  后端地址输入框
  保存测试地址

Release 版：
  不展示测试后端地址
```

构建开关：

```gradle
debug {
  buildConfigField 'boolean', 'SHOW_TEST_SERVER_SETTINGS', 'true'
}
release {
  buildConfigField 'boolean', 'SHOW_TEST_SERVER_SETTINGS', 'false'
}
```

## 5. 组件拆分方案

当前 `MainActivity.java` 过重，下一版至少拆出以下内部组件方法。后续如果继续扩展，应拆成独立 Java 类。

### 5.1 基础 UI 方法

```text
baseScreen()
pageBody()
addPageHeader()
rounded()
dp()
```

### 5.2 按钮方法

```text
transparentIconButton()
darkRoundButton()
primaryButton()
secondaryButton()
textOnlyButton()
drawerNavButton()
```

所有按钮创建后统一调用：

```text
flattenButton()
```

用途：

- 移除 Android 默认按钮阴影。
- 清除默认最小尺寸导致的错位。
- 避免非选中导航像卡片。

### 5.3 侧栏方法

```text
showDrawer()
closeDrawer()
closeDrawerAnimated()
drawerNavButton()
historyButton()
bottomUserBar()
```

### 5.4 页面方法

```text
renderChatHome()
renderProducts()
renderCart()
renderOrders()
renderProfile()
renderSettings()
```

## 6. 状态设计

### 6.1 页面状态

新增或保留：

```java
private String activePage = "chat";
```

取值：

```text
chat
products
cart
orders
profile
settings
```

用途：

- 控制侧栏导航高亮。
- 避免侧栏不知道当前页面。

### 6.2 当前会话状态

```java
private String localSessionId;
private String serverSessionId;
```

规则：

- APP 启动创建一个本地会话。
- 点击新建会话创建新的 `localSessionId`。
- 当前会话不展示在历史列表。
- 旧会话进入历史列表。

### 6.3 Toast 状态

新增：

```java
private Toast activeToast;
```

规则：

```java
if (activeToast != null) {
    activeToast.cancel();
}
activeToast = Toast.makeText(this, text, Toast.LENGTH_SHORT);
activeToast.show();
```

注意：

- 不要把所有 Toast 都写入聊天消息区。
- 聊天相关系统提示可以进入聊天流。
- 设置保存、按钮点击提示这类普通反馈只显示 Toast。

## 7. 动画方案

### 7.1 打开侧栏

效果：

- 侧栏从左向右滑入。
- 原页面出现半透明黑色遮罩。

实现：

```text
root: FrameLayout
  content
  drawerLayer
    drawerPanel
```

打开过程：

```text
drawerLayer alpha: 0 -> 1
drawerPanel translationX: -panelWidth -> 0
```

建议时长：

| 动画 | 时长 |
| --- | --- |
| 遮罩淡入 | 160ms |
| 侧栏滑入 | 220ms |

### 7.2 关闭侧栏

效果：

- 侧栏从右向左收回。
- 遮罩淡出。

关闭过程：

```text
drawerLayer alpha: 1 -> 0
drawerPanel translationX: 0 -> -panelWidth
动画结束后 removeView(drawerLayer)
```

建议时长：

| 动画 | 时长 |
| --- | --- |
| 遮罩淡出 | 180ms |
| 侧栏滑出 | 180ms |

### 7.3 点击遮罩关闭

遮罩点击关闭侧栏。

规则：

- 点击侧栏内部不关闭。
- 点击灰色遮罩区域关闭。

## 8. 本地会话与历史规则

当前本地表可继续使用：

```sql
sessions(local_session_id, server_session_id, title, summary, sync_state, created_at, updated_at)
messages(local_message_id, local_session_id, role, content, blocks_json, status, created_at)
```

需要调整：

- `ensureSession()` 不应无意义创建大量重复“新的导购会话”。
- 新建会话时应更新旧会话标题或摘要。
- 历史列表应排除当前会话。
- 如果会话没有消息，不建议展示在历史列表，避免刚进入 APP 就出现空历史。

推荐新增方法：

```java
List<SessionSummary> recentSessionsExcluding(String currentLocalSessionId)
boolean hasMessages(String localSessionId)
void updateSessionTitle(String localSessionId, String title)
```

历史列表查询规则：

```text
WHERE local_session_id != currentLocalSessionId
AND EXISTS (SELECT 1 FROM messages WHERE messages.local_session_id = sessions.local_session_id)
ORDER BY updated_at DESC
LIMIT 50
```

## 9. 验收标准

### 9.1 首页

- 左上角侧栏按钮没有白色背景。
- 右上角用户状态可点击进入“我的”。
- 底部输入栏 `+`、输入区域、语音/发送按钮高度一致且垂直居中。
- 输入为空时显示语音按钮。
- 输入非空时显示发送按钮。
- 生成中显示停止按钮。

### 9.2 侧栏

- 点击侧栏按钮后，侧栏从左向右滑入。
- 原页面变暗。
- 点击遮罩后，侧栏向左收回。
- 顶部存在搜索栏和新建会话按钮。
- 主导航有 `AI导购`、`商品`、`购物车`、`订单`。
- 当前页面对应按钮浅灰高亮。
- 历史会话多时可以滚动。
- 当前会话不出现在历史列表。
- 底部左侧头像/用户名可进入“我的”。
- 底部右侧设置按钮可进入“设置”。

### 9.3 页面间距

- 商品、购物车、订单、我的、设置页面标题和侧栏按钮之间有稳定间距。
- 标题区域不挤压状态栏。
- 页面内容不贴着标题。

### 9.4 Toast

- 快速连续点击“保存测试地址”时，只显示最新 Toast。
- 不出现 Toast 排队现象。
- 不把设置类 Toast 添加到聊天流。

## 10. 模拟器测试要求

每次实现后必须在 Android 模拟器中验证，不允许只靠编译通过。

### 10.1 构建

```bash
cd android-native
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

### 10.2 安装

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb install -r app/build/outputs/apk/debug/app-debug.apk
```

### 10.3 启动

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb shell am start -n com.xzxg.shop/.MainActivity
```

### 10.4 截图检查点

至少保存以下截图：

```text
/private/tmp/xzxg-android-v2-home.png
/private/tmp/xzxg-android-v2-drawer-chat.png
/private/tmp/xzxg-android-v2-products.png
/private/tmp/xzxg-android-v2-drawer-products.png
/private/tmp/xzxg-android-v2-settings-toast.png
```

### 10.5 崩溃检查

```bash
/opt/homebrew/share/android-commandlinetools/platform-tools/adb logcat -d -v time \
  | rg -n "AndroidRuntime|FATAL EXCEPTION|com.xzxg.shop|System.err" \
  | tail -120
```

验收要求：

- 没有 `FATAL EXCEPTION`。
- 没有 `AndroidRuntime` 崩溃。
- 页面切换和侧栏动画无明显卡死。

## 11. 实施步骤

### 第一步：统一 UI 基础能力

- 引入 `FrameLayout root` 支撑遮罩和侧栏浮层。
- 清理按钮默认样式。
- 建立统一 `PageHeader`。
- 建立统一输入栏尺寸。

### 第二步：重做聊天主页

- 重做顶部栏。
- 重做底部胶囊输入栏。
- 右上角用户入口接入 `renderProfile()`。

### 第三步：重做侧栏

- 顶部搜索 + 新建会话。
- 主导航四入口。
- activePage 高亮。
- 历史滚动区。
- 悬浮返回聊天按钮。
- 底部用户区和设置按钮。

### 第四步：补设置页

- 新增 `renderSettings()`。
- Debug 展示测试后端地址。
- Release 不展示测试后端地址。

### 第五步：Toast 覆盖

- 增加 `activeToast`。
- 新 Toast 前 cancel 旧 Toast。
- 区分普通 Toast 与聊天系统消息。

### 第六步：模拟器验证

- 构建。
- 安装。
- 截图。
- 切换页面。
- 连续点击保存测试地址。
- 查 logcat。

## 12. 风险与边界

- 当前项目仍使用原生 Java View 手写 UI，复杂 UI 会导致 `MainActivity` 继续膨胀；下一阶段应拆分组件类。
- 目前图标主要使用文本符号，视觉一致性有限；后续应接入 vector drawable 图标资源。
- 历史会话目前主要依赖本地 SQLite，服务端历史同步和完整恢复仍需要后端接口配合。
- 语音、图片、停止生成仍是入口占位，不属于本次 UI v2 的完成范围。
- 本文只解决顾客端 UI 与交互问题，不覆盖商家端、管理员端完整管理体验。

## 13. 完成定义

本方案完成后，应达到：

- APP 首页接近参考 `main.jpeg` 的视觉结构。
- 侧栏接近参考 `side-new.jpg` 的结构和层级。
- 商品、购物车、订单、我的、设置页面顶部布局一致。
- 连续 Toast 不排队。
- 模拟器实测通过。
- Debug APK 可以构建并安装运行。
