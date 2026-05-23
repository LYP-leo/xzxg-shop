# 安卓 APP 技术方案 v11

## 1. 背景

本文针对 [安卓APP_问题_v10.md](./安卓APP_问题_v10.md) 设计下一版 Android 原生 APP 改造方案。

v10 已经接入了后端 v3 的聊天、商品、订单、优惠券、活动、评价等能力，但仍存在三个关键问题：

1. 侧栏信息层级不对：“优惠券”和“活动”不应该作为一级侧栏入口。
2. 历史会话同步不够可靠：发送消息后如果没有及时同步到远程，切换设备或重进 APP 可能丢会话。
3. 聊天生成过程缺少停止能力：当前停止按钮只是占位提示，没有真正停止后端生成和本地 SSE。

v11 的目标是修正信息架构、补齐会话同步机制，并实现真正可用的“停止生成”。

## 2. 设计结论

```text
侧栏
  保留一级入口：AI导购、商品、购物车、订单
  不再展示：优惠券、活动
  优惠券入口放入“我的”
  活动入口放入“商品”页面底部 bar 的“活动”Tab

商品页
  底部固定 bar：左侧“商品列表”，右侧“活动”
  点击“商品列表”展示商品分页列表
  点击“活动”展示活动页面
  商品列表首屏只请求并渲染 20 条
  不允许用全量商品数据撑出滚动条
  用户滑到底部后再次上拉，才加载后 20 条

历史会话
  用户每发送任何内容，都触发一次远程会话同步
  同步作业在后台继续执行，不依赖当前打开的会话
  用户点击某个历史会话后，本地立即 touchSession，让它排到历史列表顶部
  已登录状态下同步远程 sessions 后，仍以本地最新交互时间优先刷新侧栏

聊天生成停止
  收到 message_start 后保存 activeRunId
  流式请求返回可取消的 StreamCall
  点击停止键时同时做两件事：
    1. POST /api/v1/agent/runs/{run_id}:cancel
    2. disconnect 当前 SSE HttpURLConnection
  已产生的部分回复按 canceled 状态保存
```

## 3. 侧栏与页面层级

### 3.1 侧栏一级入口

侧栏只保留高频、主流程入口：

```text
AI导购
商品
购物车
订单
```

移除：

```text
优惠券
活动
```

理由：

- 优惠券属于用户资产，应归到“我的”。
- 活动属于商品浏览上下文，应归到“商品”。
- 侧栏承担主导航，不应把所有业务页面平铺成一级入口。

### 3.2 “我的”里的优惠券入口

当前顶部右侧用户入口进入个人页后，在“我的服务”区域增加：

```text
我的优惠券
```

点击后进入现有 `renderCoupons()` 页面。

未登录处理：

```text
点击我的优惠券
  -> 如果未登录，跳转登录页
  -> 如果已登录，进入优惠券页
```

优惠券页内部仍保持 v10 的两个列表：

```text
可领取
我的
```

### 3.3 “商品”里的活动入口

商品页不在顶部搜索和分类区域下方放活动入口，改为底部固定 bar：

```text
商品页
  内容区
    商品列表 Tab：搜索、分类筛选、商品列表
    活动 Tab：平台优惠、店铺满减、限时促销

  底部 bar
    左侧：商品列表
    右侧：活动
```

交互规则：

- 默认进入“商品列表”。
- 点击底部左侧“商品列表”，展示商品列表内容。
- 点击底部右侧“活动”，展示活动内容，可复用现有 `renderPromotions()` 的列表渲染能力。
- 底部 bar 固定在页面底部，不随商品列表滚动。
- 当前选中的 Tab 使用更深的文字颜色和顶部细线或浅底色标识。
- 商品列表和活动是商品页内部的两个子页面，不再通过侧栏直接进入。

底部 bar 样式：

```text
高度：56dp
分栏：左右各 50%
左侧文案：商品列表
右侧文案：活动
背景：白色
顶部边线：#EEF0F3
选中态：黑色文字 + 轻量选中标识
未选中态：灰色文字
```

### 3.4 商品列表分页规则

商品列表必须是真分页，而不是“全量加载后本地只显示前 20 条”。

首屏进入商品列表时：

```text
只请求 20 条
只渲染 20 条
RecyclerView / ScrollView 中只有这 20 条 item
滚动范围只由这 20 条 item 决定
```

禁止实现：

```text
一次性请求所有商品
本地截取前 20 条展示
列表容器仍知道后面还有大量 item
滚动条表现出全量商品高度
```

正确交互：

```text
进入商品列表
  -> GET /api/v1/products?limit=20&cursor=0
  -> 渲染 20 条
  -> 页面看起来就像当前只有 20 个商品

用户滑到底部
  -> 不立即自动加载

用户在底部再次上拉 / 继续下滑
  -> 显示“正在加载更多...”
  -> GET /api/v1/products?limit=20&cursor=20
  -> 将新 20 条 append 到现有列表后面
  -> 滚动范围随 append 后的真实 item 数增加
```

分页状态：

```text
productCursor = 0
productPageSize = 20
productItems = []
productLoading = false
productHasMore = true
productReachedBottomOnce = false
```

加载规则：

```text
首次加载：
  productCursor = 0
  loadProducts(reset=true)

触底：
  productReachedBottomOnce = true
  显示“继续上拉加载更多”

触底后继续上拉：
  if productHasMore && !productLoading:
    loadProducts(reset=false)
```

接口返回不足 20 条时：

```text
productHasMore = false
底部展示“没有更多商品了”
```

切换到“活动”Tab 时：

```text
保留商品列表已加载的 productItems 和滚动位置
不重新请求商品列表
```

从“活动”Tab 切回“商品列表”时：

```text
恢复原商品列表和滚动位置
```

## 4. 历史会话同步

### 4.1 问题定义

当前逻辑主要在以下时机同步远程会话：

```text
打开侧栏
  -> GET /api/v1/agent/sessions
  -> upsert 到本地
```

但这不能保证“发送消息后立即进入远程历史”。用户发完消息后如果切换会话、关闭 APP 或换设备，远程会话可能还没更新标题、摘要和最后活跃时间。

v11 必须做到：

```text
只要用户发送了任何内容
  -> 本地先保存
  -> 远程会话必须创建或绑定
  -> 后台同步远程会话元数据
  -> 同步任务不因为切换当前会话而中断
```

### 4.2 本地立即保存

发送时先保本地，保证用户界面马上有反馈：

```text
sendMessage(text, attachments)
  -> visibleText = 文本或附件摘要
  -> chatStore.saveMessage(localSessionId, "user", visibleText, "pending")
  -> chatStore.touchSession(localSessionId)
  -> 侧栏下一次打开时本地排序已经更新
```

如果当前会话还没有 `serverSessionId`：

```text
创建远程 session
  -> bindServerSession(localSessionId, serverSessionId)
  -> 再发送 stream
```

### 4.3 每次发送都同步远程 session

新增后台同步方法：

```java
enqueueSessionSync(localSessionId, serverSessionId, title, summary)
```

触发时机：

```text
保存用户消息后
远程 session 创建成功后
message_end 后
error/canceled 后
```

同步内容：

```json
{
  "title": "从用户最新消息生成的标题",
  "summary": "用户最新消息或本轮摘要"
}
```

调用接口：

```text
PATCH /api/v1/agent/sessions/{session_id}
```

注意：同步不是为了阻塞聊天发送，而是为了让远程历史尽快可恢复。因此同步失败不能让聊天失败。

### 4.4 后台同步队列

新增 `RemoteSessionSyncQueue`，或在 `MainActivity` 内先实现一个单线程队列：

```text
队列元素 SessionSyncJob
  localSessionId
  serverSessionId
  title
  summary
  enqueueAt
  retryCount
```

执行规则：

```text
单线程顺序执行
同一个 localSessionId 有新任务时，合并为最新 title/summary
任务执行时只使用 job 自己携带的 localSessionId/serverSessionId
禁止读取 MainActivity 当前 active localSessionId/serverSessionId
```

这样即使用户切到另一个会话，旧会话的同步仍能完成。

失败重试：

```text
第 1 次失败：2 秒后重试
第 2 次失败：5 秒后重试
第 3 次失败：标记 sync_state=failed
```

本地状态：

```text
pending_sync
syncing
synced
failed
```

### 4.5 数据库调整

当前 `sessions` 表已有 `sync_state`，v11 建议升级到 version 3，补充：

```text
last_sync_at INTEGER
sync_error TEXT
```

用途：

- `last_sync_at`：判断最近一次成功同步时间。
- `sync_error`：调试远程同步失败原因。

`messages` 表建议补充：

```text
run_id TEXT
server_message_id TEXT
```

用途：

- 停止生成后能记录哪个 run 被取消。
- 后续远程详情补全时，可用 server message id 合并本地消息。

如果为了控制改动范围，第一版实现可以先不持久化同步队列表，只保证当前进程内后台任务不因会话切换而中断。后续再扩展为可跨进程恢复。

### 4.6 点击历史会话置顶

用户点击历史会话时立即执行：

```text
chatStore.touchSession(item.localSessionId)
```

再进入会话：

```text
localSessionId = item.localSessionId
serverSessionId = item.serverSessionId
loadLocalMessages()
closeDrawerAnimated()
renderChatHome()
```

排序规则：

```text
历史列表 ORDER BY updated_at DESC
```

因此点击某个历史会话后，它会立刻排到顶部。

远程同步补充：

```text
如果已登录且 item.serverSessionId 不为空
  -> 可异步 PATCH session summary/title 原值
  -> 主要目的不是修改内容，而是让远程 updated_at 也尽量靠前
```

如果后端 PATCH 不更新 `updated_at`，Android 端仍以本地 `updated_at` 保证当前设备内的置顶体验。

## 5. 停止生成

### 5.1 当前问题

当前点击停止按钮只提示：

```text
停止生成能力已预留，后续接入 run cancel。
```

这不符合需求。v11 必须真正停止：

```text
前端停止接收 SSE
后端 run 标记为 canceled
UI 恢复可输入状态
部分内容按“已停止”保存
```

### 5.2 API

使用后端已有接口：

```text
POST /api/v1/agent/runs/{run_id}:cancel
```

响应：

```json
{
  "run_id": "run_xxx",
  "status": "canceled"
}
```

`run_id` 来自 SSE `message_start`：

```json
{
  "type": "message_start",
  "run_id": "run_xxx",
  "session_id": "sess_xxx",
  "user_message_id": "msg_xxx"
}
```

### 5.3 ApiClient 改造

当前 `streamMessage()` 没有返回值，无法主动断开连接。v11 改为返回可取消对象：

```java
ApiClient.StreamCall call = api.streamMessage(...);
call.cancel();
```

`StreamCall` 内部保存：

```text
volatile boolean canceled
HttpURLConnection connection
Thread thread
```

`cancel()` 行为：

```text
canceled = true
connection.disconnect()
```

读取 SSE 时：

```text
while (!call.canceled && (line = reader.readLine()) != null)
```

如果是用户主动 cancel 导致的异常，不再走普通“发送失败”提示。

同时新增：

```java
public JSONObject cancelAgentRun(String runId)
```

内部调用：

```text
POST /agent/runs/{run_id}:cancel
```

### 5.4 MainActivity 状态

新增字段：

```text
activeRunId
activeStreamCall
activeStreamLocalSessionId
stopRequested
```

生命周期：

```text
sendMessage
  -> streaming = true
  -> stopRequested = false
  -> activeRunId = ""
  -> activeStreamLocalSessionId = 当前 localSessionId
  -> actionButton 显示停止图标

message_start
  -> activeRunId = event.run_id

message_end
  -> finishStream()
  -> 清空 activeRunId/activeStreamCall

error canceled
  -> finishCanceledStream()
```

### 5.5 点击停止键

点击停止键时：

```text
if streaming:
  stopRequested = true
  actionButton 暂时禁用
  loading/status 文案改为“正在停止...”

  if activeRunId 不为空:
    后台 POST /agent/runs/{activeRunId}:cancel

  if activeStreamCall 不为空:
    activeStreamCall.cancel()

  finishCanceledStream()
```

`finishCanceledStream()`：

```text
移除 loading bubble
如果已有 assistant 文本或 blocks:
  保存 assistant turn，status=canceled
否则:
  不保存空 assistant
在聊天列表展示“已停止生成”
streaming = false
actionButton 恢复发送/麦克风状态
```

### 5.6 竞态处理

停止生成存在几个竞态，必须显式处理。

#### 情况一：用户点击停止时还没收到 message_start

此时没有 `run_id`：

```text
只 disconnect SSE
不调用 cancel run
UI 标记已停止
```

后端可能已经创建 run 但前端尚未收到 `message_start`。如果断开连接后后端仍继续执行，后端会在请求 context 取消时停止；如果后端没有及时停止，也至少不会继续写入当前 UI。

#### 情况二：停止后仍收到少量 SSE 事件

所有 SSE 处理前先判断：

```text
if stopRequested:
  忽略 text_delta/block_delta/followups/message_end
```

同时检查会话归属：

```text
if activeStreamLocalSessionId != 当前事件所属 localSessionId:
  不写入当前页面
```

#### 情况三：用户切换会话时旧流还在返回

切换会话不应该把旧回复写进新会话 UI。

规则：

```text
每个 stream callback 捕获 ownerLocalSessionId
保存消息时写 ownerLocalSessionId
渲染 UI 前判断 ownerLocalSessionId == 当前 localSessionId
不相等则只保存，不渲染
```

## 6. 发送、同步、停止的完整流程

### 6.1 正常发送

```text
用户发送
  -> 本地保存 user message
  -> touchSession
  -> 如果没有 serverSessionId，创建远程 session
  -> enqueueSessionSync
  -> 发起 stream
  -> message_start 绑定 run_id
  -> status 更新加载文案
  -> text_delta 每字渲染 Markdown
  -> block_delta 渲染结构化卡片
  -> followups 展示不可直接发送的追问建议
  -> message_end 保存 assistant completed
  -> enqueueSessionSync
```

### 6.2 生成中停止

```text
用户点击停止
  -> stopRequested = true
  -> POST runs/{run_id}:cancel
  -> disconnect SSE
  -> 保存 partial assistant canceled
  -> enqueueSessionSync
  -> UI 恢复输入
```

### 6.3 切换会话

```text
用户点击历史会话
  -> touchSession(clickedLocalSessionId)
  -> 当前列表排序立即更新
  -> 切换到目标会话
  -> 旧会话未完成的同步任务继续后台执行
```

## 7. 验收标准

### 7.1 侧栏

- 侧栏不再出现“优惠券”。
- 侧栏不再出现“活动”。
- “我的”页面能进入“我的优惠券”。
- “商品”页面底部 bar 左侧是“商品列表”，右侧是“活动”。
- 点击“商品列表”展示商品列表，点击“活动”展示活动内容。

### 7.2 商品列表

- 首次进入商品列表时只请求 20 条商品。
- 首次进入商品列表时只渲染 20 条商品。
- 如果全库商品超过 20 条，首屏滚动条也不能表现出全量商品高度。
- 用户滑到底部后，再继续上拉/下滑才加载下一页 20 条。
- 新加载的商品 append 到当前列表后方，不清空已加载商品。
- 切到“活动”再切回“商品列表”后，已加载商品和滚动位置不丢失。

### 7.3 历史会话

- 已登录用户发送任意文本消息后，会触发远程 session 创建或更新。
- 已登录用户发送带附件消息后，也会触发远程 session 创建或更新。
- 发送后马上切换到另一个会话，原会话的远程同步仍继续执行。
- 点击任意历史会话后，该会话排到历史列表顶部。
- 历史会话仍只展示标题，不展示副信息。

### 7.4 停止生成

- 生成中按钮显示停止状态。
- 点击停止后，后端 run cancel 接口被调用。
- 点击停止后，本地 SSE 连接断开。
- 已生成的部分内容保留，并标记为已停止。
- 停止后输入栏恢复可用。
- 停止后不会再把旧流的内容追加到当前页面。

## 8. 实施顺序

```text
1. 调整侧栏导航
   - 移除优惠券、活动一级入口
   - 我的页增加优惠券入口
   - 商品页增加底部 bar：商品列表 / 活动
   - 商品列表和活动作为商品页内部 Tab 切换

2. 改造商品列表分页
   - 首屏只请求并渲染 20 条
   - 触底后继续上拉再加载下一页
   - append 新数据，不用全量数据撑滚动条

3. 实现历史会话同步队列
   - 增加 enqueueSessionSync
   - 发送后、结束后、取消后都入队
   - 点击历史会话时 touchSession

4. 实现可取消 SSE
   - ApiClient.streamMessage 返回 StreamCall
   - StreamCall.cancel disconnect 连接
   - 增加 cancelAgentRun

5. 接入停止生成 UI
   - message_start 保存 run_id
   - 停止键调用 cancel + disconnect
   - 保存 canceled assistant turn

6. 模拟器验证
   - 打开侧栏检查入口层级
   - 商品页检查底部 bar 和首屏 20 条分页
   - 发送消息后切会话检查同步不中断
   - 生成中点击停止检查 UI 和日志
```

## 9. 风险与边界

1. 如果后端 `PATCH /agent/sessions/{session_id}` 不更新 `updated_at`，远程会话排序可能不会因点击历史而置顶；Android 本地仍必须先保证当前设备内置顶。
2. 如果后端在 SSE 断开后仍继续生成，前端也必须忽略停止后的旧事件，避免污染当前 UI。
3. 当前第一版同步队列可以只做进程内后台队列；如果要保证 APP 被系统杀死后仍继续同步，需要后续引入持久化任务表或 WorkManager。
