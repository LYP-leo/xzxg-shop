# 安卓 APP 技术方案 v15

## 1. 背景

本文针对 [安卓APP_问题_v14.md](./安卓APP_问题_v14.md) 设计下一版 Android 原生 APP 修复方案。

v14 修复了搜索框键盘失焦、搜索结果被远程空结果覆盖、侧栏底部视觉和个人资料顶部按钮对齐。但 v14 的部分交互与最新产品要求冲突，因此 v15 需要回退并统一以下行为：

- 历史会话搜索改为后端接口提供，不再用本地 SQLite 搜索作为主要结果源。
- 点击历史会话只进入，不改变排序；只有在该会话里发送消息时才更新活跃时间并移动到第一条。
- 点击历史会话搜索框时，不再自动收起四个导航按钮。
- 收起键盘时，不再自动让搜索框失焦。
- 个人资料页“取消”应返回进入该页之前的页面，当前主要来源是“账号管理”页。

## 2. 总体设计

```text
侧栏搜索:
  搜索框聚焦:
    不隐藏 AI导购 / 商品 / 购物车 / 订单
    不改变历史列表 padding

  键盘收起:
    不主动 clearFocus
    搜索框可继续保持焦点

  输入搜索:
    调用后端 GET /api/v1/agent/sessions/search?q=...
    以后端返回结果为准
    清空搜索框时恢复全量历史

历史会话排序:
  点击历史会话:
    不 touch 本地 session
    不 PATCH 服务端 updated_at
    不改变历史排序

  在历史会话里发送消息:
    本地 saveMessage/touchSession 更新 updated_at
    服务端消息流接口更新 last_message_at/updated_at
    下次侧栏刷新时该会话排到前面

个人资料:
  renderEditProfilePage 接收来源页 backAction
  从账号管理进入时，取消返回账号管理
  从设置页/头像页进入时，取消返回对应来源
```

## 3. 侧栏搜索改造

### 3.1 移除搜索聚焦折叠导航

当前 v14 代码中 `refreshSearchChrome` 根据 `search.hasFocus()` 控制导航区：

```java
boolean expanded = search.hasFocus();
navGroup.setVisibility(expanded ? View.GONE : View.VISIBLE);
divider.setVisibility(expanded ? View.GONE : View.VISIBLE);
historyList.setPadding(0, expanded ? dp(4) : dp(12), 0, dp(72));
```

v15 要删除这套逻辑：

- 删除 `searchExpanded`。
- 删除 `refreshSearchChrome`。
- 删除 `navGroup.setVisibility(...)` 和 `divider.setVisibility(...)`。
- 历史列表 padding 固定为 `top = 12dp`。
- 搜索框聚焦仅弹出键盘，不影响导航按钮展示。

### 3.2 移除自动失焦逻辑

当前 v14 通过以下路径自动清焦点：

- `collapseDrawerSearch(search, refreshSearchChrome)`
- `ImeAwareEditText.onKeyPreIme`
- `drawerLayer.setOnApplyWindowInsetsListener`
- `search.setOnEditorActionListener`
- 抽屉空白点击监听

v15 需要改为：

```text
收起键盘:
  只收起键盘
  不 clearFocus

点击搜索框外空白:
  可只隐藏键盘
  不强制 search.clearFocus()

点击遮罩:
  如果只是想关闭抽屉，保持原关闭抽屉逻辑
```

实施建议：

- 删除 `ImeAwareEditText`，搜索框恢复为普通 `EditText`。
- 删除 `collapseDrawerSearch()` 或不再用于搜索框。
- `search.setOnEditorActionListener` 可移除；如保留，只调用 `hideKeyboardFrom(search)`，不清焦点。
- `drawerLayer` 点击逻辑恢复为 `closeDrawerAnimated()`。
- `drawer` / `historyFrame` / `historyScroll` / `historyList` 的空白点击不再调用清焦点逻辑。

### 3.3 搜索由后端接口提供

最新要求“聊天记录搜索功能应当改为后端接口提供”。v15 搜索流程：

```text
query 为空:
  renderHistoryList(chatStore.recentSessionsWithMessages())

query 非空:
  renderHistoryLoading 或保留旧结果
  debounce 300ms
  GET /api/v1/agent/sessions/search?q=<query>
  将服务端结果 upsert 到 LocalChatStore
  按服务端返回顺序渲染
```

关键点：

- 不再调用 `chatStore.searchSessionsLocal(query)` 作为展示依据。
- 不再 merge 本地/远程结果。
- 后端返回空，就展示“暂无历史聊天”或“没有匹配会话”。
- 请求失败时展示“搜索失败，请重试”，不要显示本地旧结果伪装成搜索结果。
- 清空搜索框时恢复本地全量历史。

伪代码：

```java
search.addTextChangedListener(new TextWatcher() {
    afterTextChanged(Editable editable) {
        String query = editable.toString().trim();
        int version = ++searchVersion[0];
        if (query.isEmpty()) {
            renderHistoryList(historyList, chatStore.recentSessionsWithMessages());
            return;
        }
        renderHistoryStatus(historyList, "搜索中...");
        new Thread(() -> {
            try {
                Thread.sleep(300);
                if (version != searchVersion[0]) return;
                JSONArray sessions = api.searchSessions(query);
                List<SessionSummary> results = upsertRemoteSearchResults(sessions);
                runOnUiThread(() -> {
                    if (version == searchVersion[0] && query.equals(search.getText().toString().trim())) {
                        renderHistoryList(historyList, results);
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (version == searchVersion[0]) {
                        renderHistoryStatus(historyList, "搜索失败，请重试");
                    }
                });
            }
        }).start();
    }
});
```

建议新增：

```java
private List<LocalChatStore.SessionSummary> upsertRemoteSearchResults(JSONArray sessions)
```

该方法只负责：

- 逐条 `chatStore.upsertRemoteSession(...)`
- `chatStore.sessionSummary(localId)`
- 按后端返回顺序组装结果

### 3.4 后端搜索职责

后端 `SearchUserSessions` 必须负责完整搜索：

- `chat_sessions.title`
- `chat_sessions.summary`
- `user_messages.content`

当前后端已 join `user_messages`，v15 只需确认：

```sql
WHERE s.account_id = ?
  AND s.message_count > 0
  AND (s.title LIKE ? OR s.summary LIKE ? OR m.content LIKE ?)
ORDER BY COALESCE(s.last_message_at, s.updated_at, s.created_at) DESC
```

如测试发现后端只返回标题/摘要匹配，需要补齐 SQL。

## 4. 历史会话排序一致性

### 4.1 需求定义

历史会话排序规则统一为：

```text
只进入历史会话:
  不更新 updated_at
  不改变排序

进入后发送消息:
  更新本地 updated_at
  更新服务端 last_message_at / updated_at
  该会话移动到历史第一条
```

### 4.2 当前代码核对

历史会话点击 `historyButton()` 当前应保持：

```java
row.setOnClickListener(v -> {
    localSessionId = item.localSessionId;
    serverSessionId = item.serverSessionId == null ? "" : item.serverSessionId;
    closeDrawerAnimated();
    renderChatHome();
    loadHomeCopy();
    if (!serverSessionId.isEmpty() && !chatStore.hasMessages(localSessionId)) {
        loadRemoteSessionDetail(serverSessionId);
    }
});
```

不得调用：

```java
chatStore.touchSession(localSessionId)
enqueueSessionSync(...)
api.updateSession(...)
```

发送消息路径允许更新：

```java
chatStore.saveMessage(localSessionId, "user", ...)
chatStore.touchSession(localSessionId)
enqueueSessionSync(localSessionId, serverSessionId, ...)
api.streamMessage(...)
```

### 4.3 修复建议

1. 全局搜索 `touchSession(`：
   - 保留发送消息路径中的调用。
   - 删除历史会话点击、远程会话加载、仅查看详情路径中的调用。

2. 全局搜索 `upsertRemoteSession`：
   - 同步远程列表时可以更新本地记录为服务端时间。
   - 打开历史会话详情时不要把本地 `updated_at` 改为当前时间。

3. `LocalChatStore.upsertRemoteSession`：
   - 对已存在 session，使用 `Math.max(localTime, remoteTime)` 是可接受的，因为仅查看不会产生新的 remoteTime。
   - 如果远程加载详情使用当前时间作为 `remoteTime`，需要改为服务端 `updated_at/last_message_at`。

4. 验收方式：
   - 记录会话 A/B/C 顺序。
   - 点击 B 进入后返回侧栏，顺序仍为 A/B/C。
   - 在 B 中发送消息，返回侧栏，顺序变为 B/A/C。

## 5. 个人资料页返回来源

### 5.1 当前问题根因

当前 `renderEditProfilePage()` 中“取消”固定执行：

```java
cancel.setOnClickListener(v -> renderSettings());
```

因此即使从“账号管理”页进入，也会返回“设置”页。

### 5.2 修复方案

将个人资料页改为带来源参数：

```java
private void renderEditProfilePage() {
    renderEditProfilePage(() -> renderSettings());
}

private void renderEditProfilePage(Runnable backAction) {
    ...
    cancel.setOnClickListener(v -> backAction.run());
}
```

调用方分别传入：

```java
// 设置页点击昵称
name.setOnClickListener(v -> renderEditProfilePage(() -> renderSettings()));

// 账号管理页点击个人资料
group1.addView(settingsRow("▦", "个人资料", v -> renderEditProfilePage(() -> renderAccountPage()), ...));

// 头像预览页点击编辑
edit.setOnClickListener(v -> renderEditProfilePage(() -> renderAvatarPreviewPage()));
```

保存成功后的返回目标建议：

- 如果从账号管理进入，保存后返回账号管理，用户能看到资料更新。
- 如果从设置进入，保存后返回设置。
- 如果从头像预览进入，保存后返回设置更合理，因为头像预览依赖旧 URL，保存后需要重新渲染资料入口。

为了简单一致，v15 可以让 `saveProfileChanges` 接收 `Runnable backActionAfterSave`：

```java
saveProfileChanges(nickname, backAction)
```

保存成功后：

```java
backAction.run();
```

## 6. 代码清理

由于 v15 取消 v14 搜索自动失焦和本地合并，以下代码应删除或停用：

- `ImeAwareEditText`
- `collapseDrawerSearch`
- `mergeSessionResults`
- `addSessionResults`
- `LocalChatStore.searchSessionsLocal` 如果没有其他调用，可以保留但不再用于侧栏搜索；建议删除以避免误用。
- `searchExpanded` / `imeWasVisible`
- `refreshSearchChrome`
- 搜索框 focus listener 中的导航隐藏逻辑

保留：

- `syncRemoteSessions(onDone)`，用于打开侧栏时刷新服务端历史。
- `recentSessionsWithMessages()`，用于非搜索态全量历史。

## 7. 实施顺序

1. 侧栏搜索 UI：
   - 恢复普通 `EditText`。
   - 删除聚焦隐藏导航和自动失焦逻辑。
   - 抽屉空白点击不再清搜索焦点。

2. 侧栏搜索数据：
   - 输入非空时只调用 `api.searchSessions(query)`。
   - 后端返回结果按顺序渲染。
   - 清空输入恢复本地全量历史。

3. 历史排序：
   - 确认历史点击不 touch。
   - 确认发送消息会 touch，并同步服务端更新时间。

4. 个人资料返回：
   - `renderEditProfilePage` 增加来源 backAction。
   - 账号管理进入时取消返回账号管理。
   - 保存成功后按来源返回。

5. 构建和模拟器回归。

## 8. 回归测试清单

### 8.1 Android 构建

```bash
ditto /Volumes/shared/xzxg-shop/android-native /private/tmp/xzxg-android-build
cd /private/tmp/xzxg-android-build
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

### 8.2 模拟器手工验收

1. 打开侧栏，点击“历史会话搜索”，四个导航按钮仍然显示。
2. 收起键盘后，搜索框不被强制失焦。
3. 输入关键字，搜索结果来自后端接口；断网或接口失败时显示搜索失败。
4. 清空搜索框，恢复全部历史会话。
5. 点击历史会话 B 进入，不发送消息，返回侧栏，B 不移动到第一条。
6. 在历史会话 B 中发送消息，返回侧栏，B 移动到第一条。
7. 从“账号管理”进入“个人资料”，点击“取消”，返回“账号管理”。
8. 从“设置”进入“个人资料”，点击“取消”，返回“设置”。

## 9. 风险与边界

- v15 明确以后端搜索为准，本地未同步但实际存在的历史会话不会出现在搜索结果中；这是符合“由后端接口提供”的要求。
- 如果后端搜索性能不足，需要后续给 `user_messages.content` 加全文索引或倒排索引，本版先不扩展。
- 搜索框保留焦点可能导致导航区和键盘状态不完全同步，但这是本轮产品要求。
- 个人资料保存后的返回目标需要和入口来源一致，避免再次固定跳设置页。
