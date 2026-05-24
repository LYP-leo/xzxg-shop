# 安卓 APP 技术方案 v12

## 1. 背景

本文针对 [安卓APP_问题_v11.md](./安卓APP_问题_v11.md) 设计下一版 Android 原生 APP 和后端配套改造方案。

本次设计已先通读 `.ai/tech` 下现有 49 份技术文档，并重点核对以下连续约束：

- [技术方案_v3.md](./技术方案_v3.md)：Android 原生端、账号体系、聊天记录本地与服务端同步、三角色能力。
- [Agent框架设计_v2.md](./Agent框架设计_v2.md)：会话、用户消息、run、SSE 事件和停止生成生命周期。
- [Agent_ReAct工具化改造计划_v1.md](./Agent_ReAct工具化改造计划_v1.md)：Agent 工具链、结构化输出、购物车/订单动作。
- [动态配置拆分与Prompt管理设计_v1.md](./动态配置拆分与Prompt管理设计_v1.md)：Prompt 和配置中心，后续可承载会话标题总结 Prompt。
- [质量测评体系设计_v1.md](./质量测评体系设计_v1.md)：质量回归与验收需要可复测。
- [安卓APP技术方案_v2.md](./安卓APP技术方案_v2.md) 到 [安卓APP技术方案_v11.md](./安卓APP技术方案_v11.md)：Android 原生单 Activity、侧栏、历史会话、商品分页、流式 Markdown、结构化卡片、远程会话同步和停止生成的演进。

v11 已解决主导航、商品分页、远程会话同步和停止生成问题。v12 的核心问题变成：

1. 账号体系从“能登录”升级到“能管理资料、注销、删除账号”。
2. 会话历史从“有同步”升级到“没有空会话、有服务端标题总结、可搜索”。
3. 未登录态从“部分功能仍像登录态”收敛为“单次本地聊天 + 明确登录入口”。
4. 设置页和账号管理页从临时表单升级为正式移动端页面。

## 2. 历史方案取舍

### 2.1 继续沿用的约束

v12 继续沿用以下结论：

- Android 端继续使用 `android-native/` 单 Activity + Java 原生 View，不在本版引入 Compose。
- 聊天页继续保持 v7-v11 已形成的能力：流式 Markdown、结构化商品卡、followups、SSE status、停止生成。
- 商品、购物车、订单、优惠券、活动和评价沿用 v10-v11 的页面层级，不在本版重新调整。
- 侧栏一级入口仍只保留：`AI导购`、`商品`、`购物车`、`订单`。
- 本地聊天仍用 `LocalChatStore` SQLite 保存，服务端会话仍用 `chat_sessions` / `user_messages` / `agent_runs`。
- 头像、昵称、token、角色等轻量登录态仍用 `SessionStore` 保存。

### 2.2 需要修正的历史冲突

历史文档里对“点击历史会话是否置顶”有冲突：

- v3 明确要求：点击历史只是查看，不改变排序；只有新消息才置顶。
- v11 为了当前设备体验，提出点击历史后 `touchSession()` 置顶。
- v12 新问题再次强调：只有发送消息后会话才算正式开始，聊天历史不该出现空聊天。

v12 采用更一致的规则：

```text
历史排序只由真实消息活跃时间决定。
点击历史会话只打开，不更新 updated_at，不上报服务端活跃时间。
发送用户消息、助手回复完成、停止生成保存 partial turn 时，才更新本地和服务端活跃时间。
```

理由：

- 搜索、侧栏和服务端历史都应代表真实聊天活动，而不是浏览行为。
- 这样可以避免用户只是查看旧会话就打乱历史顺序。
- 也和“空聊天不进入历史”保持同一套语义。

### 2.3 当前代码差距

截至本方案编写时，当前代码有以下差距：

- 后端已有 `POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`GET /api/v1/auth/me`，但缺少注销、账号删除、资料修改、头像上传、手机号/邮箱修改。
- Android `ApiClient` 已经预留 `/account/profile`、`/auth/password:change`、`/auth/verification-codes` 调用，但后端没有对应路由。
- `accounts` 表当前只有 `username/display_name/role/merchant_id/status`，没有 `avatar_url/phone/email/deleted_at`。
- `ListUserSessions` 当前不筛 `message_count > 0`，服务端可能返回空会话。
- Android `showDrawer()` 未登录仍展示侧栏；v12 要改成直接进入独立登录页。
- Android `ensureActiveSessionVisible()` 会把当前空会话插入历史，v12 必须删除这个逻辑。
- Android 当前麦克风空输入态仍使用 `"mic"` 文本，v12 必须换成图标。
- 当前“我的”页同时承担登录、资料编辑、账号管理，v12 要拆成登录页、设置页、账号管理页、编辑资料页。

## 3. 总体设计结论

```text
后端
  账号资料：
    增加头像、手机号、邮箱、软删除字段
    提供 profile/contact/avatar/logout/delete 接口

  会话历史：
    列表只返回 message_count > 0 的正式会话
    发送首条消息时才创建服务端 session
    每轮完成后生成不超过 8 个字的标题
    提供历史会话搜索接口

Android
  未登录：
    不展示侧栏
    点击侧栏按钮先收键盘，再进入独立登录页
    只保留当前本地单次会话，不提供新建会话和历史

  已登录：
    侧栏展示搜索、导航、历史会话
    新建会话只创建本地空会话，不创建服务端会话
    用户发送第一条有效消息后创建服务端会话并同步
    侧栏历史展示服务端总结标题

  页面：
    登录页独立
    设置页参考 settings_01.jpg
    头像预览页参考 head_icon.jpg
    编辑个人资料页参考 pri_info.jpg
    账号管理页参考 my_page.jpg
```

## 4. 后端账号体系

### 4.1 数据模型

扩展 `accounts` 表：

```sql
ALTER TABLE accounts ADD COLUMN avatar_url VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN phone VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN email VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN deleted_at DATETIME NULL;
CREATE INDEX idx_accounts_status ON accounts(status);
```

`domain.Account` 扩展：

```go
type Account struct {
    AccountID   string      `json:"account_id"`
    Username    string      `json:"username"`
    DisplayName string      `json:"display_name"`
    AvatarURL   string      `json:"avatar_url,omitempty"`
    Phone       string      `json:"phone,omitempty"`
    Email       string      `json:"email,omitempty"`
    Role        AccountRole `json:"role"`
    MerchantID  string      `json:"merchant_id,omitempty"`
    Status      string      `json:"status,omitempty"`
    CreatedAt   time.Time   `json:"created_at"`
}
```

注册输入扩展：

```go
type AccountCreateInput struct {
    Username     string
    PasswordHash string
    DisplayName  string
    AvatarURL    string
    Phone        string
    Email        string
    Role         AccountRole
    MerchantID   string
}
```

### 4.2 Store 接口

新增账号相关方法：

```go
UpdateAccountProfile(ctx, accountID string, displayName string, avatarURL string) (domain.Account, bool)
UpdateAccountContact(ctx, accountID string, phone string, email string) (domain.Account, bool)
DeleteAccount(ctx, accountID string) bool
DeleteAuthToken(ctx, token string) bool
DeleteAuthTokensByAccount(ctx, accountID string) bool
```

已有 `UpdateAccountStatus` 可以复用为删除账号的底层能力，但建议提供 `DeleteAccount`，明确软删除语义：

```text
status = deleted
deleted_at = now()
清理该账号所有 token
```

### 4.3 API

#### 当前用户资料

```http
GET /api/v1/account/profile
Authorization: Bearer <token>
```

响应：

```json
{
  "account_id": "acc_xxx",
  "username": "user",
  "display_name": "用户名",
  "avatar_url": "/api/v1/uploads/avatar/acc_xxx.jpg",
  "phone": "",
  "email": "",
  "role": "user",
  "status": "active",
  "created_at": "2026-05-24T10:00:00Z"
}
```

#### 修改个人资料

```http
PATCH /api/v1/account/profile
Authorization: Bearer <token>
Content-Type: application/json
```

请求：

```json
{
  "display_name": "新昵称",
  "avatar_url": "/api/v1/uploads/avatar/acc_xxx.jpg"
}
```

兼容 Android 当前字段：

```json
{
  "nickname": "新昵称",
  "avatar_url": "/api/v1/uploads/avatar/acc_xxx.jpg"
}
```

服务端处理规则：

- `display_name` 和 `nickname` 二选一，优先 `display_name`。
- 昵称不能为空，最多 24 个字符。
- `avatar_url` 为空表示不修改，不表示清空头像；如需清空后续单独设计。

#### 修改手机号/邮箱

```http
PATCH /api/v1/account/contact
Authorization: Bearer <token>
Content-Type: application/json
```

请求：

```json
{
  "phone": "+86 17800000098",
  "email": "user@example.com"
}
```

第一版不强制验证码，先完成资料修改闭环；如需验证码，后续接入 `/auth/verification-codes`。

#### 注销登录

```http
POST /api/v1/auth/logout
Authorization: Bearer <token>
```

行为：

- 删除当前 token。
- 返回 `204 No Content` 或 `{ "ok": true }`。
- Android 成功后清空本地 `SessionStore`，回到聊天页未登录态。

#### 删除账号

```http
DELETE /api/v1/account
Authorization: Bearer <token>
```

行为：

- 二次确认由 Android 弹窗完成。
- 后端将账号软删除，清理所有 token。
- 不物理删除订单、消息、评价等业务记录，避免破坏审计和订单数据。
- 后续查询账号时 `status = deleted` 不允许登录。

### 4.4 头像上传

新增：

```http
POST /api/v1/uploads/avatar
Authorization: Bearer <token>
Content-Type: multipart/form-data

file=<jpg/png/webp>
```

响应：

```json
{
  "url": "/api/v1/uploads/avatar/acc_xxx_1716540000.jpg",
  "mime_type": "image/jpeg",
  "size": 123456
}
```

第一版存储方式：

```text
backend/uploads/avatar/
```

静态访问：

```http
GET /api/v1/uploads/avatar/{filename}
```

限制：

- 只允许 `image/jpeg`、`image/png`、`image/webp`。
- 上传大小限制 2MB。
- 服务端再次解码校验图片，避免伪造 MIME。
- Android 端已经裁剪为正方形，但服务端仍校验宽高比例，允许 1px 误差。

## 5. 后端会话历史

### 5.1 正式会话定义

服务端正式会话定义：

```text
chat_sessions.message_count > 0
```

任何历史列表、历史搜索、侧栏展示都只返回正式会话。

因此修改 `ListUserSessions`：

```sql
SELECT session_id, account_id, title, summary, message_count, last_message_at, created_at, updated_at
FROM chat_sessions
WHERE account_id = ?
  AND message_count > 0
ORDER BY COALESCE(last_message_at, updated_at, created_at) DESC, created_at DESC
```

这样即使客户端误创建了服务端空 session，也不会进入侧栏。

### 5.2 首条消息创建服务端会话

Android 新建会话按钮只创建本地空会话：

```text
createFreshLocalSession()
  -> localSessionId = local_sess_xxx
  -> chatStore.ensureSession(localSessionId, "新的导购会话")
  -> serverSessionId = ""
```

发送第一条有效消息时：

```text
sendMessage()
  -> 本地保存 user message
  -> 如果未登录：只保留本地单次会话，不请求后端
  -> 如果已登录且 serverSessionId 为空：
       POST /api/v1/agent/sessions
       绑定 serverSessionId
       POST /api/v1/agent/sessions/{id}/messages:stream
```

服务端 `CreateSession` 仍可保留，但客户端不能在点击“新建会话”时调用它。

### 5.3 会话标题总结

新增接口：

```http
POST /api/v1/agent/sessions/{session_id}:summarize
Authorization: Bearer <token>
```

响应：

```json
{
  "session_id": "sess_xxx",
  "title": "手机推荐",
  "summary": "用户咨询老人用手机，关注价格、续航和易用性。"
}
```

也可以在 `message_end` 后由后端自动触发，不要求 Android 主动调用。建议本版采用自动触发 + 手动接口兼容：

```text
CreateUserMessage
  -> message_count 从 0 变 1 后，先写默认标题

Runtime message_end
  -> 后台 summarizer 读取最近消息
  -> 生成 <= 8 字标题
  -> 更新 chat_sessions.title / summary
```

标题生成规则：

- 不超过 8 个中文字符。
- 不带标点。
- 不输出“导购会话”“新聊天”这类泛标题。
- 优先提炼商品品类、用途、关键诉求，例如：
  - `老人手机`
  - `洁面推荐`
  - `订单支付`
  - `运动鞋对比`

后备算法：

```text
如果 LLM 总结失败：
  1. 清理用户首条消息中的标点、停用词
  2. 优先匹配商品品类词和场景词
  3. 最多取 8 个字符
  4. 仍为空则使用“导购咨询”
```

Prompt 建议进入配置中心或 `agent_prompts`：

```text
你是电商导购会话标题生成器。
根据用户和助手的聊天内容，生成一个不超过 8 个中文字符的标题。
标题必须具体，体现商品、场景或问题。
不要输出标点、解释、引号。
```

### 5.4 历史搜索

新增 Store 方法：

```go
SearchUserSessions(ctx context.Context, accountID string, keyword string, page int, pageSize int) ([]domain.ChatSession, int)
```

新增接口：

```http
GET /api/v1/agent/sessions/search?q=关键词&page=1&page_size=20
Authorization: Bearer <token>
```

SQL 第一版：

```sql
SELECT DISTINCT s.session_id, s.account_id, s.title, s.summary, s.message_count,
       s.last_message_at, s.created_at, s.updated_at
FROM chat_sessions s
LEFT JOIN user_messages m ON m.session_id = s.session_id AND m.account_id = s.account_id
WHERE s.account_id = ?
  AND s.message_count > 0
  AND (
    s.title LIKE ?
    OR s.summary LIKE ?
    OR m.content LIKE ?
  )
ORDER BY COALESCE(s.last_message_at, s.updated_at, s.created_at) DESC, s.created_at DESC
LIMIT ? OFFSET ?
```

后续优化：

- MySQL 8 可加 `FULLTEXT(title, summary)` 和 `FULLTEXT(content)`。
- 搜索命中内容片段可后续扩展 `matched_snippet`，本版先只返回会话。

### 5.5 路由顺序

因为已有：

```go
mux.HandleFunc("GET /api/v1/agent/sessions", ...)
mux.HandleFunc("GET /api/v1/agent/sessions/", ...)
```

搜索建议用非尾斜杠固定路由：

```go
mux.HandleFunc("GET /api/v1/agent/sessions/search", s.handleSearchAgentSessions)
```

并在 `handleAgentSessionAction` 中识别 `:summarize`：

```text
POST /api/v1/agent/sessions/{id}:summarize
POST /api/v1/agent/sessions/{id}/messages:stream
```

## 6. Android 未登录态

### 6.1 状态定义

未登录态规则：

```text
token 为空
  -> 不支持侧栏
  -> 不支持历史
  -> 不支持新建多个会话
  -> 当前仅有一个本地临时会话
```

未登录不是“可离线使用完整聊天系统”，而是“单次本地草稿体验 + 登录引导”。

### 6.2 侧栏按钮行为

修改顶部栏菜单按钮：

```java
menu.setOnClickListener(v -> {
    hideKeyboard();
    if (sessionStore.token().isEmpty()) {
        renderLoginPage();
        return;
    }
    showDrawer();
});
```

`showDrawer()` 自身也要加保护：

```java
private void showDrawer() {
    hideKeyboard();
    if (sessionStore.token().isEmpty()) {
        renderLoginPage();
        return;
    }
    ...
}
```

这样无论从返回键、菜单键还是其它路径进入侧栏，都不会在未登录时打开侧栏。

### 6.3 未登录本地会话

APP 启动：

```text
createFreshLocalSession()
```

未登录发送消息：

```text
本地保存 user message
显示登录提示 assistant message
不创建服务端 session
不进入历史列表
不提供新建会话入口
```

未登录再次点击聊天主页：

```text
保留当前 localSessionId
不创建第二个未登录会话
```

如果用户想清空当前单次会话，可后续提供“清空当前聊天”，本版不是必要需求。

### 6.4 登录后处理

登录成功：

```text
保存 token/role/nickname/avatar/account_id
回到聊天主页
创建新的已登录本地空会话
不自动把未登录本地临时聊天上传到服务端
```

理由：

- 未登录单次会话没有用户身份，不应默认合并到账号历史。
- 自动迁移会让“未登录不支持创建新会话”的边界变复杂。

如果后续要支持迁移，必须做明确弹窗：“是否保存刚才的聊天到账号历史？”

## 7. Android 侧栏和历史搜索

### 7.1 侧栏只服务登录用户

已登录侧栏结构保持 v11：

```text
顶部：
  历史会话搜索框
  新建会话按钮

导航：
  AI导购
  商品
  购物车
  订单

历史：
  服务端总结标题
  不显示空会话

底部：
  用户头像 / 用户名
  设置
```

### 7.2 搜索栏

把当前 `TextView search` 改成 `EditText`：

```text
hint = "历史会话搜索"
singleLine = true
background = 浅灰搜索条
左侧搜索图标
```

交互：

```text
输入为空：
  展示最近历史

输入非空：
  debounce 300ms
  GET /api/v1/agent/sessions/search?q=...
  用结果替换历史列表

点击搜索结果：
  关闭键盘
  打开对应会话
```

Android API：

```java
public JSONArray searchSessions(String keyword) throws Exception {
    JSONObject response = get("/agent/sessions/search?q=" + urlEncode(keyword));
    return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
}
```

### 7.3 空会话过滤

删除或修改 `ensureActiveSessionVisible()`：

```text
不再把 active session 强行插入 histories。
```

历史列表来源：

```text
已登录：
  本地 recentSessionsWithMessages()
  + 服务端 syncRemoteSessions() 返回 message_count > 0 的会话

未登录：
  不打开侧栏
```

同时修改 `LocalChatStore.recentSessions()`，建议也过滤空会话，避免调用方误用：

```sql
WHERE EXISTS (
  SELECT 1 FROM messages m
  WHERE m.local_session_id = s.local_session_id
    AND m.role = 'user'
)
```

是否要求 user message：

- 本版建议只要有 `user` 消息才算正式开始。
- 只有 assistant 的登录提示不能让未登录会话进入历史。

### 7.4 历史排序

v12 排序规则：

```text
本地：sessions.updated_at 只在保存 user message 或 assistant turn 时更新。
服务端：chat_sessions.last_message_at 只在 user_messages 写入时更新。
点击历史：不 touchSession，不 PATCH 服务端。
```

点击历史只做：

```text
localSessionId = item.localSessionId
serverSessionId = item.serverSessionId
loadLocalMessages()
如果本地无消息且 serverSessionId 不为空：
  GET /api/v1/agent/sessions/{id}
  转成本地消息缓存
closeDrawerAnimated()
renderChatHome()
```

## 8. Android 聊天页

### 8.1 麦克风图标

禁止继续使用：

```java
"mic"
```

推荐新增 vector drawable：

```text
android-native/app/src/main/res/drawable/ic_mic.xml
android-native/app/src/main/res/drawable/ic_send.xml
android-native/app/src/main/res/drawable/ic_keyboard.xml
android-native/app/src/main/res/drawable/ic_stop.xml
```

如果为控制改动先不新增 XML，也至少使用统一风格 Unicode：

```text
麦克风：🎙
键盘：⌨
发送：➤
停止：■
```

但最终验收以“不出现 mic 三个字”为准。

### 8.2 隐藏键盘再开侧栏或登录

新增 helper：

```java
private void hideKeyboard() {
    View view = getCurrentFocus();
    if (view == null) {
        view = input;
    }
    if (view == null) {
        return;
    }
    InputMethodManager imm = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
    if (imm != null) {
        imm.hideSoftInputFromWindow(view.getWindowToken(), 0);
    }
    view.clearFocus();
}
```

调用点：

- 顶部菜单按钮点击。
- `showDrawer()` 开头。
- 搜索结果点击。
- 登录页进入前。

## 9. Android 登录页

### 9.1 独立页面

新增：

```java
private void renderLoginPage()
```

登录页不是“我的”页。结构：

```text
顶部：
  返回 / 标题：登录

内容：
  账号输入框
  密码输入框
  登录按钮
  注册账号入口
  测试后端地址入口仅在 SHOW_TEST_SERVER_SETTINGS 开启时显示
```

注册可以同页切换，也可以用 `renderRegisterPage()`：

```text
登录页 -> 注册账号 -> 注册页
注册成功 -> 保存 token -> 回聊天页
```

### 9.2 登录成功

登录成功后：

```text
sessionStore.saveAuth(token, role, display_name, avatar_url, account_id)
createFreshLocalSession()
renderChatHome()
loadHomeCopy()
```

`SessionStore` 建议新增：

```java
public String accountId()
public String username()
public String phone()
public String email()
```

保存账号资料时一次性写入，设置页和账号管理页不再重复请求。

## 10. Android 设置页

### 10.1 入口

侧栏底部右侧设置按钮进入：

```java
renderSettingsPage()
```

未登录时不会出现侧栏；如果其它路径进入设置页，先跳登录。

### 10.2 页面结构

参考 `assets/settings_01.jpg`：

```text
顶部左侧：
  菜单按钮

中间用户区：
  大头像
  用户名
  用户id：{account_id}
  账号管理按钮

列表：
  帮助
  关于
  退出登录

底部：
  Version: {BuildConfig.VERSION_NAME}
  由 {大模型名称} 提供支持
```

交互：

- 点击头像：进入头像全屏预览页。
- 点击用户名：进入编辑个人资料页。
- 点击账号管理：进入账号管理页。
- 点击帮助：进入帮助页。
- 点击关于：进入关于页。
- 点击退出登录：二次确认，确认后调用后端 logout。

### 10.3 页面视觉

沿用项目 v5 后的普通布局，不重新做全面屏：

- 背景：`#F8F9FB`。
- 大头像：约 120dp，圆形。
- 用户名：28sp 左右，居中。
- 用户 id：14sp，浅灰。
- 账号管理按钮：白底，圆角 12dp，高 52dp。
- 功能列表：白色分组，8dp 圆角，行高 64dp。
- 不嵌套卡片。

## 11. 头像预览页

### 11.1 页面结构

参考 `assets/head_icon.jpg`：

```text
黑色背景
中间显示头像大图
底部按钮：编辑个人资料
```

交互：

- 点击空白或系统返回：回设置页。
- 长按头像：保存到相册。
- 点击“编辑个人资料”：进入编辑个人资料页。

### 11.2 保存头像

Android 保存规则：

- 如果头像是远程 URL，先下载到 app cache。
- 调用 `MediaStore.Images.Media` 写入相册。
- Android 10+ 使用 scoped storage，不申请外部存储权限。
- 保存成功 Toast：`已保存到相册`。

第一版如果保存实现成本过高，可以先完成长按弹出菜单和下载逻辑，但验收要求是长按可保存。

## 12. 编辑个人资料页

### 12.1 页面结构

参考 `assets/pri_info.jpg`：

```text
顶部：
  取消
  标题：个人资料
  完成

内容：
  大头像 + 右下角加号
  昵称 label
  昵称输入框
```

规则：

- 初始状态“完成”置灰不可点。
- 头像或昵称任一变化后，“完成”点亮。
- 点击取消：丢弃本地未保存修改并返回。
- 点击完成：上传头像后 PATCH profile。

### 12.2 头像裁剪

头像只能是正方形。流程：

```text
点击头像
  -> ACTION_PICK / ACTION_GET_CONTENT 选择图片
  -> 解码 Bitmap
  -> 进入正方形裁剪页
  -> 用户确认
  -> 输出 512x512 JPEG
  -> 暂存到 cache
  -> 返回编辑页预览
```

裁剪页第一版可以实现为中心裁剪 + 简单预览：

```text
取原图中心区域：
  side = min(width, height)
  left = (width - side) / 2
  top = (height - side) / 2
  Bitmap.createBitmap(src, left, top, side, side)
  scale 到 512x512
```

如果时间允许，再加拖拽和缩放：

- 使用自定义 `SquareCropView`。
- 图片可双指缩放、单指拖拽。
- 中间固定正方形裁剪框。

本版验收下限：

- 选任意长方形图片后，上传头像必须是正方形。
- 不能把原始长方形直接传给后端。

### 12.3 保存流程

```text
点击完成
  if nickname 为空：提示
  if 头像已变更：
    POST /api/v1/uploads/avatar
    avatar_url = response.url
  PATCH /api/v1/account/profile
  更新 SessionStore
  返回设置页
```

失败处理：

- 上传失败：不 PATCH profile，提示 `头像上传失败`。
- profile 失败：保留当前编辑页，提示 `保存失败`。

## 13. 账号管理页

### 13.1 页面结构

参考 `assets/my_page.jpg`：

```text
顶部：
  标题：账号管理
  右上角 x

用户卡：
  头像
  用户名
  用户id
  只读，不可点击

分组一：
  个人资料 >

分组二：
  手机号     +86 178******98 >
  邮箱       user***@example.com >

分组三：
  删除账号 >
  退出登录 >
```

右上角 `x`：

```text
等同系统返回
```

### 13.2 个人资料

点击 `个人资料`：

```text
renderEditProfilePage()
```

### 13.3 手机号/邮箱弹窗

点击手机号：

```text
AlertDialog
  标题：修改手机号
  输入框
  取消 / 保存
```

保存：

```text
PATCH /api/v1/account/contact
```

邮箱同理。

第一版校验：

- 手机号允许数字、空格、`+`、`-`，长度 6-32。
- 邮箱为空或满足基础邮箱正则。

### 13.4 删除账号

点击删除账号：

```text
第一次弹窗：
  标题：删除账号
  内容：删除后将无法继续使用当前账号登录，订单和历史记录会保留用于业务审计。
  取消 / 继续

第二次确认：
  标题：确认删除
  内容：请再次确认删除账号。
  取消 / 删除
```

确认后：

```text
DELETE /api/v1/account
清空 SessionStore
createFreshLocalSession()
renderChatHome()
```

### 13.5 退出登录

点击退出登录：

```text
弹窗：是否退出登录？
取消 / 退出
```

确认后：

```text
POST /api/v1/auth/logout
无论后端是否成功，都清空本地 token
createFreshLocalSession()
renderChatHome()
```

本地清空原因：token 可能已经过期，用户点击退出时不能被后端错误卡住。

## 14. 帮助页和关于页

### 14.1 帮助页

新增：

```java
renderHelpPage()
```

内容第一版保持克制：

```text
帮助
  AI导购
  商品与购物车
  订单与支付
  账号与资料
```

不要写大段说明文，使用简短问题和回答。

### 14.2 关于页

新增：

```java
renderAboutPage()
```

内容：

```text
小猪小狗导购
Version: {BuildConfig.VERSION_NAME}
由 {大模型名称} 提供支持
```

大模型名称来源：

1. 优先后端配置接口，例如后续 `GET /api/v1/app/about`。
2. 本版可先从 `BuildConfig.MODEL_NAME` 或常量读取。

## 15. Android API Client

新增方法：

```java
public JSONObject logout() throws Exception
public JSONObject profile() throws Exception
public JSONObject updateProfile(String displayName, String avatarUrl) throws Exception
public JSONObject updateContact(String phone, String email) throws Exception
public JSONObject deleteAccount() throws Exception
public JSONObject uploadAvatar(File file) throws Exception
public JSONArray searchSessions(String keyword) throws Exception
public JSONObject summarizeSession(String sessionId) throws Exception
```

`uploadAvatar` 需要 multipart：

```text
Content-Type: multipart/form-data; boundary=...
```

当前 `ApiClient` 是手写 `HttpURLConnection`，可以继续手写 multipart，避免引入新依赖。

## 16. 本地存储调整

### 16.1 SessionStore

新增字段：

```text
account_id
username
phone
email
```

方法：

```java
saveAuth(token, role, nickname, avatarUrl, accountId, username, phone, email)
saveProfile(accountJson)
clearAuth()
```

### 16.2 LocalChatStore

数据库版本升级到 3：

```java
super(context, "xzxg_chat.db", null, 3);
```

建议新增字段：

```sql
ALTER TABLE sessions ADD COLUMN started INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN title_source TEXT NOT NULL DEFAULT 'local';
ALTER TABLE sessions ADD COLUMN last_sync_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN sync_error TEXT NOT NULL DEFAULT '';
```

`started` 规则：

```text
第一次保存 user message 时置 1。
只有 started = 1 的会话能进入历史。
```

如果不新增 `started`，也必须统一用 `EXISTS user message` 判断，不能用 assistant 登录提示判断。

## 17. 实施顺序

### Phase 1：后端账号接口

1. 扩展 `accounts` 表和 `domain.Account`。
2. Store 增加 profile/contact/logout/delete 方法。
3. 新增后端路由：
   - `GET /api/v1/account/profile`
   - `PATCH /api/v1/account/profile`
   - `PATCH /api/v1/account/contact`
   - `POST /api/v1/auth/logout`
   - `DELETE /api/v1/account`
   - `POST /api/v1/uploads/avatar`
4. 更新 `routePolicy`，账号接口要求 user/merchant/admin 已登录。

### Phase 2：后端会话能力

1. `ListUserSessions` 增加 `message_count > 0`。
2. 新增 `SearchUserSessions` 和 `/agent/sessions/search`。
3. 新增会话标题总结接口和后备算法。
4. 在 `message_end` 或 run 完成后触发服务端标题总结。

### Phase 3：Android 未登录和侧栏

1. `showDrawer()` 未登录直接进入登录页。
2. 顶栏菜单点击先 `hideKeyboard()`。
3. 删除 `ensureActiveSessionVisible()` 对空会话的强行插入。
4. 侧栏搜索框改成 `EditText`，接入历史搜索。
5. 新建会话只创建本地空会话，不请求后端。

### Phase 4：Android 账号页面

1. 新增独立登录页和注册页。
2. 新增设置页。
3. 新增头像预览页。
4. 新增编辑个人资料页。
5. 新增账号管理页。
6. 新增帮助页和关于页。

### Phase 5：头像裁剪上传

1. 图库选择图片。
2. 正方形裁剪。
3. 上传头像。
4. PATCH profile。
5. 更新本地头像缓存。

### Phase 6：验证和回归

1. 后端 `go test ./...`。
2. Android `gradle assembleDebug`。
3. 模拟器手工验证未登录、登录、历史、搜索、资料编辑、退出、删除账号。

## 18. 验收标准

### 18.1 未登录

- 未登录点击侧栏按钮时，键盘先收起，然后进入登录页。
- 未登录不会打开侧栏。
- 未登录不会显示历史会话。
- 未登录不会创建多个会话。
- 未登录发送消息后只保留当前本地单次会话，不创建服务端 session。

### 18.2 会话历史

- 新建会话但不发送消息，服务端不创建 session，侧栏不显示。
- 已登录用户发送第一条消息后，才创建服务端 session。
- 服务端 `GET /agent/sessions` 不返回 `message_count = 0` 的会话。
- 每轮结束后侧栏标题变成不超过 8 个字的总结标题。
- 搜索“聊天中出现过的关键字”能找到对应会话。
- 点击历史会话只打开，不改变排序。

### 18.3 聊天页

- 空输入时右侧按钮不再显示 `mic` 文本。
- 麦克风、键盘、发送、停止按钮风格统一。
- 输入框聚焦且键盘弹出时，点击侧栏按钮先收键盘。

### 18.4 设置页

- 设置页顶部展示头像、用户名、用户 id、账号管理按钮。
- 点击头像进入全屏头像预览。
- 长按头像可以保存图片。
- 点击用户名进入编辑个人资料页。
- 点击账号管理进入账号管理页。
- 帮助、关于、退出登录入口可用。
- 底部展示 `Version: {版本}` 和 `由 {大模型名称} 提供支持`。

### 18.5 编辑个人资料

- 初始状态“完成”不可点。
- 修改昵称或头像后“完成”点亮。
- 点击头像可以从图库选择图片。
- 非正方形图片保存前会被裁剪成正方形。
- 保存后后端和本地设置页都展示新头像/昵称。

### 18.6 账号管理

- 顶部头像、用户名、用户 id 只读，不可点击。
- 个人资料入口进入编辑个人资料页。
- 手机号和邮箱点击后弹窗修改。
- 删除账号有二次确认，确认后 token 失效并回到未登录聊天页。
- 退出登录有确认弹窗，确认后回到未登录聊天页。
- 右上角 `x` 等同返回。

## 19. 风险与边界

1. 头像裁剪如果要支持拖拽缩放，自定义 View 工作量较高；第一版可以中心裁剪，但必须保证输出正方形。
2. 会话标题总结如果依赖 LLM，可能延迟几秒；Android 侧栏应先显示本地临时标题，再在下次同步后刷新服务端标题。
3. 删除账号本版采用软删除，不物理删除订单和消息；如果产品要求彻底删除，需要单独设计数据合规策略。
4. `/auth/verification-codes` 和 `/auth/password:change` 当前 Android 已调用但后端缺路由；本版可以顺手补齐或暂时从 UI 隐藏改密/验证码入口，避免假功能。
5. 历史搜索第一版用 `LIKE`，数据量大后性能会下降；后续再切 FULLTEXT 或独立搜索索引。
6. 未登录聊天不自动迁移到账号历史，避免身份边界混乱；如要迁移需新增显式确认。

