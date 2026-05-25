# 安卓 APP 技术方案 v16

## 1. 背景

本文针对 [安卓APP_问题_v15.md](./安卓APP_问题_v15.md) 设计下一版 Android 原生 APP 修复方案。

当前问题：

- 点击历史会话 A 后，返回聊天页。
- 再次打开侧栏历史列表，A 会被移动到第一条。
- 期望行为是：只进入 A 不改变历史排序；只有在 A 中继续向大模型发送新消息后，A 才移动到第一条。

v15 已经要求“点击历史会话不 touch”，但实际仍然发生排序变化，说明排序更新时间不是由点击事件直接触发，而是由进入会话后的其他写库路径间接触发。

## 2. 根因分析

当前 Android 端历史列表排序来自本地 SQLite：

```java
recentSessionsWithMessages()
ORDER BY s.updated_at DESC
```

历史会话点击逻辑本身已经没有显式 `touchSession()`：

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

但首次进入远程历史会话时，如果本地没有消息，会调用 `loadRemoteSessionDetail()` 拉取远程详情，并逐条写入本地消息：

```java
chatStore.saveMessage(localSessionId, "user", content, "synced");
```

`LocalChatStore.saveMessage()` 的副作用是：

```java
sessionValues.put("updated_at", now);
getWritableDatabase().update("sessions", sessionValues, "local_session_id = ?", ...);
```

因此，“进入历史会话并加载远程历史消息”会被误判为“当前刚刚产生了新消息”，导致该 session 的本地 `updated_at` 被更新为当前时间，历史列表排序随之提升到第一条。

另外，`upsertRemoteSession()` 当前对已有本地 session 使用：

```java
values.put("updated_at", Math.max(localTime, remoteTime));
```

这会让被错误提升过的本地 `updated_at` 持续保留，即使后续从服务端同步到真实远程时间，也无法把它恢复到原位置。

## 3. 目标行为

历史会话排序只允许由“真实新对话”触发变化：

```text
点击历史会话:
  不修改 sessions.updated_at
  不修改服务端 updated_at / last_message_at
  不改变历史列表排序

首次进入远程历史会话并拉取历史消息:
  只补齐本地 messages 表
  不 touch session
  不改变 sessions.updated_at
  不改变历史列表排序

在历史会话里发送新消息:
  保存用户新消息
  touch 本地 session
  通过后端对话接口更新服务端 last_message_at / updated_at
  历史列表移动到第一条
```

## 4. Android 端设计

### 4.1 拆分“新增消息”和“导入历史消息”

`LocalChatStore.saveMessage()` 当前同时做两件事：

1. 插入一条 message。
2. 更新 session 的 `updated_at`，并在 role 为 user 时更新 title/summary。

v16 需要把“导入远程历史消息”从这个路径中拆出来，新增无副作用写入方法：

```java
public void saveRemoteMessageSnapshot(
        String localSessionId,
        String role,
        String content,
        String blocksJson,
        String followupsJson,
        String status,
        long createdAt
)
```

该方法只负责：

- 插入 `messages` 表。
- 使用远程消息原始 `created_at` 作为 `messages.created_at`。
- 不更新 `sessions.updated_at`。
- 不更新 session title/summary。
- 不调用 `touchSession()`。

如果需要避免重复导入，可优先新增 `server_message_id` 字段；如果本版不改表结构，则至少在导入前按 `local_session_id + role + content + created_at` 做轻量去重，避免多次进入导致重复消息。

### 4.2 修改 `loadRemoteSessionDetail()`

当前实现：

```java
chatStore.saveMessage(localSessionId, "user", content, "synced");
```

改为：

```java
String role = message.optString("role", "assistant");
long createdAt = parseRemoteTime(message.optString("created_at", ""));
chatStore.saveRemoteMessageSnapshot(
    targetLocalSessionId,
    role,
    content,
    message.optString("blocks_json", "[]"),
    message.optString("followups_json", "[]"),
    "synced",
    createdAt
);
```

关键要求：

- 使用点击时固定下来的 `targetLocalSessionId`，不要在线程回调里直接读取全局 `localSessionId`，避免用户快速切换会话时写错目标会话。
- 使用服务端返回的真实 `role`，不要把所有历史消息都保存为 `"user"`。
- 导入完成后只刷新当前会话 UI，不触发排序更新时间。

建议结构：

```java
private void loadRemoteSessionDetail(String sessionId, String targetLocalSessionId) {
    new Thread(() -> {
        JSONObject detail = api.sessionDetail(sessionId);
        JSONArray messages = detail.optJSONArray("messages");
        for (...) {
            chatStore.saveRemoteMessageSnapshot(targetLocalSessionId, ...);
        }
        runOnUiThread(() -> {
            if (targetLocalSessionId.equals(localSessionId)) {
                renderChatHome();
                loadHomeCopy();
            }
        });
    }).start();
}
```

历史点击处同步改为：

```java
String targetLocalId = item.localSessionId;
localSessionId = targetLocalId;
...
if (!serverSessionId.isEmpty() && !chatStore.hasMessages(targetLocalId)) {
    loadRemoteSessionDetail(serverSessionId, targetLocalId);
}
```

### 4.3 保留发送消息时的排序更新

`sendMessage()` 中保存用户新消息后更新 session 排序是正确行为：

```java
chatStore.saveMessage(localSessionId, "user", visibleText, "pending");
chatStore.touchSession(localSessionId);
```

v16 不取消这条路径，但需要整理职责：

- 如果 `saveMessage()` 已经更新 `updated_at`，则额外的 `touchSession()` 是重复的。
- 为了让语义更清晰，可以保留 `saveMessage()` 的现状，删除发送路径里额外的 `touchSession()`；也可以把 `saveMessage()` 改为可配置是否 touch。
- 本版建议最小改动：保留发送路径行为不变，只确保导入历史消息不走 `saveMessage()`。

## 5. 本地 session 时间同步设计

### 5.1 修复 `upsertRemoteSession()` 的错误保留

当前逻辑：

```java
values.put("updated_at", Math.max(localTime, remoteTime));
```

会保留被历史导入误更新的本地时间。v16 建议改为：

```java
values.put("updated_at", remoteTime);
```

适用前提：

- 该 session 已经有 `server_session_id`，属于服务端会话。
- 远程列表返回的 `remoteTime` 是权威排序时间。

如果担心覆盖本地未同步的新消息，可增加更明确的状态判断：

```java
if ("pending".equals(syncState) || "dirty".equals(syncState)) {
    values.put("updated_at", Math.max(localTime, remoteTime));
} else {
    values.put("updated_at", remoteTime);
}
```

本项目当前发送消息会走后端对话接口，服务端会更新 `last_message_at`，因此对 synced 远程会话使用服务端时间作为排序基准更符合产品预期。

### 5.2 新建/本地会话仍可用本地时间

本地新建但尚未产生服务端 session 的会话，继续使用本地 `updated_at`：

- 新建会话：`ensureSession()` 设置当前时间。
- 发送第一条消息：`saveMessage()` 更新当前时间。
- 服务端创建/绑定后：后续以服务端列表时间校准。

## 6. 后端设计

本轮问题主要在 Android 本地导入历史消息时误更新排序。后端接口无需新增。

需要确认后端排序保持现状：

```sql
ORDER BY COALESCE(last_message_at, created_at) DESC, created_at DESC
```

服务端只应在以下场景更新 `last_message_at`：

- 用户通过对话接口发送新消息。
- 后端实际写入新的 `user_messages`。

服务端不应因为“查询会话详情”更新 `last_message_at` 或 `updated_at`。

## 7. 实施步骤

1. 在 `LocalChatStore` 新增无 touch 的历史消息导入方法。
2. 修改 `loadRemoteSessionDetail()`：
   - 固定 `targetLocalSessionId`。
   - 使用远程消息 `role` / `created_at`。
   - 调用无 touch 导入方法。
3. 修改历史点击处调用签名，传入 `targetLocalSessionId`。
4. 修改 `upsertRemoteSession()`：
   - synced 远程会话用服务端时间覆盖本地 `updated_at`。
   - 本地 dirty/pending 会话才保留本地较新时间。
5. 保持 `sendMessage()` 发送新消息后更新排序。
6. 构建 Android 并做手工回归。

## 8. 回归测试清单

### 8.1 点击历史不改变排序

前置历史顺序：

```text
B
A
C
```

操作：

1. 打开侧栏。
2. 点击 A。
3. 回到聊天页后不发送任何消息。
4. 再打开侧栏。

期望：

```text
B
A
C
```

A 仍在原位置。

### 8.2 首次加载远程历史不改变排序

前置条件：

- A 是服务端历史会话。
- 本地 `messages` 表没有 A 的消息。

操作：

1. 点击 A。
2. 等待远程详情加载完成。
3. 不发送新消息。
4. 再打开侧栏。

期望：

- A 的历史消息正常展示。
- A 不移动到第一条。

### 8.3 发送新消息后移动到第一条

前置历史顺序：

```text
B
A
C
```

操作：

1. 点击 A。
2. 输入并发送一条新消息。
3. 等待消息发送成功或进入 pending 状态。
4. 再打开侧栏。

期望：

```text
A
B
C
```

### 8.4 快速切换会话不串写

操作：

1. 点击远程历史 A，触发详情加载。
2. 立刻打开侧栏点击 B。
3. 等待 A 的详情接口返回。

期望：

- A 的消息只写入 A。
- 当前页面仍显示 B。
- A 不因为异步回调抢占页面或改变排序。

### 8.5 同步后排序可校准

操作：

1. 对已有异常置顶的历史会话执行一次远程 session 列表同步。
2. 打开侧栏。

期望：

- synced 远程会话按服务端 `last_message_at/updated_at` 校准排序。
- 不再永久保留旧的本地错误 `updated_at`。

## 9. 风险与边界

- 如果后端会话详情没有返回 `role` 或 `created_at`，Android 需要做兼容默认值，但这会降低历史消息展示准确性；建议优先确认接口字段。
- 如果本地已存在因旧逻辑被错误提升的会话，仅修复导入路径还不够，需要同步时用服务端时间覆盖本地时间，才能把历史排序恢复。
- 如果用户离线发送消息，本地 dirty 会话仍应保留本地较新时间，避免未同步的新对话在侧栏中丢失优先级。
- 本版不改变搜索接口、不改变个人资料返回逻辑，只修复历史会话排序来源。
