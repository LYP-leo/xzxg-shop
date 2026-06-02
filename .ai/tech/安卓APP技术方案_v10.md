# 安卓 APP 技术方案 v10

## 1. 背景

本文针对 [安卓APP_问题_v9.md](./安卓APP_问题_v9.md) 设计下一版 Android 原生 APP 改造方案。

本次问题不只是 UI 细节，还涉及后端 v3 API 与 Agent 输出协议变化。已从 `origin/main` 读取：

- `.ai/api/API接口文档_v3.md`
- `.ai/api/Agent输出协议_v1.md`

v10 的目标是让 Android APP 按新 API 协议工作，并统一聊天页、侧栏、商品页、订单页的交互风格。

## 2. 设计结论

v10 按以下方向改造：

```text
聊天页
  修复长按语音无文字输出
  替换真实麦克风照片为线性麦克风图标
  追问点击后只填充输入框，不自动发送
  展示 SSE status 节点，替代单一“正在思考”
  支持 product_refs 和 <item>p_xxx</item> 商品 ID 渲染商品卡
  收紧 Markdown 标题字号，一级标题不再过大

侧栏
  导航行文本左对齐
  接入远程会话列表与会话详情
  本地会话与远程会话按 server_session_id 同步

商品页
  首屏严格只请求 20 条
  下滑触底前再加载下一页
  搜索栏与筛选器去掉圆角矩形按钮感
  分类选择改成双栏表格式列表，双栏均可滚动
  修复推荐理由展开后仍显示不全

订单页
  只展示当前用户订单
  接入订单详情、支付、取消、确认收货
  已完成订单项支持发布评价

新增页面
  优惠券页面
  活动/促销页面
  商品评价列表
```

## 3. 聊天页改造

### 3.1 语音输入修复

当前问题：长按“按住说话”后没有文字输出。

修复方向：

```text
按下
  -> startListening()
  -> 输入栏文案改为“正在聆听...”

松手
  -> stopListening()
  -> 等待 onResults 回调

onResults
  -> 取 RESULTS_RECOGNITION 第一条非空文本
  -> sendMessage(text)

onError
  -> 根据错误码展示明确提示
  -> 不发送空消息
```

注意：不要在松手后立即销毁 `SpeechRecognizer`，否则部分系统语音服务还没来得及回调 `onResults` 就被中断。正确做法是：

```text
stopListening()
等待 onResults / onError
下一次识别前再 destroy 并重建
```

错误提示建议：

- `ERROR_NO_MATCH`：没有识别到内容，请重试。
- `ERROR_SPEECH_TIMEOUT`：没有听到声音。
- `ERROR_NETWORK`：语音服务网络异常。
- 其他错误：语音识别失败，请重试。

### 3.2 麦克风图标统一

当前问题：麦克风使用真实照片，和键盘图标风格不统一。

Android 原生 View 项目不引入图标库时，采用自绘 `TextView` / `Drawable` 图标：

```text
键盘：线性键盘符号
麦克风：线性麦克风符号
发送：箭头符号
停止：方块符号
```

禁止使用真实图片作为图标。按钮视觉保持：

```text
黑色圆形底
白色线性符号
图标居中
```

### 3.3 追问点击行为

当前问题：追问点不了。

v10 改为可点击，但点击后只填充输入框，不自动发送：

```text
点击追问
  -> exitVoiceMode()
  -> input.setText(question)
  -> input.setSelection(question.length)
  -> actionButton 显示发送图标
```

禁止：

```text
点击追问 -> sendMessage(question)
```

这样既能保留快捷性，也给用户修改问题的机会。

### 3.4 SSE status 节点展示

`Agent输出协议_v1.md` 明确新增 `status` 事件：

```json
{
  "type": "status",
  "stage": "retrieval",
  "text": "正在检索商品"
}
```

Android 端处理规则：

```text
message_start
  -> 绑定 run_id、user_message_id
  -> 新建当前 assistant turn

status
  -> 更新 loading bubble 文案
  -> 例如“正在理解你的需求”“正在检索商品”“正在整理答案”
  -> 不保存进最终回答正文

text_delta
  -> 移除 loading bubble 或改为正文 bubble
  -> 追加 Markdown 正文

block_delta
  -> 追加结构化卡片

message_end
  -> 保存完整 turn
```

本地存储建议增加：

```text
run_id
user_message_id
status_text
```

如果暂时不升级表，也至少在内存态处理 `status`，不能继续只显示固定“正在思考...”。

### 3.5 商品 ID 渲染商品卡

新协议中 Agent 不再总是返回完整 `product_card`，而是可能返回：

```json
{
  "type": "product_refs",
  "product_ids": ["p_001", "p_002"]
}
```

或正文中包含：

```text
<item>p_001</item>
```

Android 处理规则：

```text
product_card
  -> 兼容旧逻辑，直接渲染 product

product_refs
  -> 读取 product_ids
  -> 去重
  -> 调用 GET /api/v1/products/{product_id}
  -> 渲染完整商品卡

text_delta 中 <item>productId</item>
  -> 正文展示时隐藏标签
  -> 提取 productId
  -> 如果本轮没有 product_refs，则按 productId 拉取商品详情并渲染卡片
```

需要增加商品详情缓存：

```text
Map<String, JSONObject> productDetailCache
```

避免同一轮或历史恢复时重复请求。

### 3.6 Markdown 字号统一

当前问题：一级标题过大。

Markdown 渲染规则调整：

```text
正文：15sp
h1：17sp，加粗
h2：16sp，加粗
h3：15.5sp，加粗
列表、引用、代码：14sp-15sp
```

原则：

- 不用大标题制造页面跳动。
- AI 回复中的标题只是结构提示，不应该像文章页标题。
- 流式渲染时字号变化不能导致整页明显抖动。

## 4. 侧栏与远程会话同步

### 4.1 侧栏左对齐

当前问题：“AI导购”“商品”“购物车”“订单”四行看起来没有左对齐。

改造规则：

```text
导航行 = 横向 LinearLayout
左侧固定 28dp 图标区域
右侧 TextView 从同一 x 坐标开始
整行透明/浅灰背景，不做按钮凸起
```

不要继续把“图标 + 空格 + 文本”拼成一个 Button 文本。

### 4.2 远程会话列表

接入接口：

```text
GET /api/v1/agent/sessions
```

侧栏历史展示规则：

```text
已登录
  -> 优先展示远程会话
  -> 同步到本地 sessions 表

未登录
  -> 展示本地会话
```

历史项只展示标题，不展示副信息，保持之前要求。

### 4.3 远程会话详情

接入接口：

```text
GET /api/v1/agent/sessions/{session_id}
```

返回结构是：

```text
session
messages[]
  user message
  runs[]
```

当前 API 主要返回用户消息和 run 元数据，不一定包含完整 assistant 文本和 blocks。因此同步策略分两层：

```text
能从远程拿到的
  session_id
  title
  summary
  message_count
  last_message_at
  用户消息
  run_id / trace_id / run status

Android 本地继续保存的
  assistant text
  blocks_json
  followups_json
```

如果远程详情后续补 assistant turn 明细，Android 再迁移为完全远程恢复。

### 4.4 本地与远程合并规则

本地 `sessions` 表已有 `server_session_id`，以它作为合并键：

```text
server_session_id 相同
  -> 更新本地 title/summary/updated_at
  -> 不重复创建历史项

server_session_id 为空
  -> 本地草稿会话
  -> 登录后发送第一条消息时创建远程 session 并 bind

远程有、本地没有
  -> 插入本地 session shell
  -> 点击后按远程详情加载用户消息
```

会话重命名接入：

```text
PATCH /api/v1/agent/sessions/{session_id}
```

## 5. 优惠券与活动页面

### 5.1 新增入口

侧栏新增：

```text
优惠券
活动
```

风格与现有导航行一致，文本左对齐。

### 5.2 优惠券页面

接口：

```text
GET /api/v1/coupons/available
GET /api/v1/coupons/mine
POST /api/v1/coupons/{coupon_id}:claim
```

页面结构：

```text
顶部 tabs：
  可领取
  我的

可领取列表：
  券名
  门槛金额
  优惠金额
  有效期
  领取按钮

我的券列表：
  券名
  状态
  门槛/优惠
  有效期
```

交互：

- 未登录点击进入时跳登录页。
- 已领取/过期/领完时禁用领取按钮。
- 领取成功后刷新“可领取”和“我的”。

### 5.3 活动页面

接口：

```text
GET /api/v1/promotions
```

页面结构：

```text
平台活动
商家活动
```

每条活动展示：

- 名称
- 满减门槛
- 优惠金额或折扣
- 是否可叠加
- 有效期
- 适用范围：平台 / 商家 / 商品 / 分类

## 6. 购物车与下单流程

### 6.1 购物车优惠预览

接口：

```text
GET /api/v1/cart/discount-preview
```

购物车底部展示：

```text
商品总额
优惠金额
应付金额
优惠明细 lines
结算按钮
```

每次勾选商品、调整数量后刷新优惠预览。

### 6.2 下单流程

接口：

```text
POST /api/v1/orders:checkout
```

新流程：

```text
购物车点击结算
  -> 请求 discount-preview
  -> 展示确认下单弹窗/页面
  -> 用户确认
  -> POST /orders:checkout
  -> 返回 pending_payment 订单
  -> 跳转订单支付页或订单页
```

不要再把 checkout 理解为“已完成下单待发货”。后端文档说明 checkout 创建的是待支付订单。

### 6.3 支付流程

接口：

```text
POST /api/v1/orders/{order_id}:pay
```

订单状态为 `pending_payment` 时展示：

```text
去支付
取消订单
支付截止时间
```

点击去支付：

```json
{"method": "mock_balance"}
```

支付成功：

```text
刷新订单详情
状态变成 pending_ship
展示支付成功提示
```

### 6.4 取消与确认收货

接口：

```text
POST /api/v1/orders/{order_id}:cancel
POST /api/v1/orders/{order_id}:confirm-receipt
```

状态动作：

```text
pending_payment
  -> 可取消、可支付

pending_ship
  -> 等待商家发货

shipped
  -> 可确认收货

completed
  -> 可评价

canceled / closed_timeout
  -> 只读
```

### 6.5 顾客只能看自己的订单

Android 只调用用户接口：

```text
GET /api/v1/orders
GET /api/v1/orders/{order_id}
```

禁止用户端调用：

```text
GET /api/v1/admin/orders
GET /api/v1/merchant/orders
```

如果当前顾客看到所有订单，需要检查：

- Android 是否误用 admin 接口。
- 登录 token 是否是 admin/merchant。
- 后端 `ListUserOrders(account_id)` 是否按 account_id 过滤。

APP 侧必须在用户端订单页只接用户订单接口。

## 7. 商品评价

### 7.1 商品详情查看评价

接口：

```text
GET /api/v1/products/{product_id}/reviews
```

商品详情页增加“用户评价”区块：

```text
评分均值
评价数量
评价列表
商家回复
```

如果评价多，第一版可以只展示前 3 条，并提供“查看全部”。

### 7.2 订单页发布评价

接口：

```text
POST /api/v1/orders/{order_id}/items/{order_item_id}:review
```

展示规则：

```text
completed 订单
  每个订单项展示“评价”按钮

已评价
  展示“已评价”，按钮禁用

未完成订单
  不展示评价入口
```

评价表单：

```text
评分：1-5 星
内容：多行输入
标签：可选快捷标签，例如 做工好、物流快、性价比高
提交按钮
```

提交成功后刷新订单详情。

## 8. 商品页改造

### 8.1 严格 20 条分页

问题文档指出“现在还是没有实现每次刚进入只加载 20 条”。v10 实现时必须用接口日志或断点验证：

```text
首次进入商品页
  -> GET /api/v1/products?limit=20&cursor=0

下滑到底部附近
  -> GET /api/v1/products?limit=20&cursor=20
```

禁止：

```text
首次进入 -> GET /api/v1/products
然后本地截取 20 条
```

如果后端分页接口还没有合并到当前分支，先从 `origin/main` 合并或按 API 文档补齐。

### 8.2 搜索栏样式

要求：不要圆角矩形设计。

改为：

```text
透明背景
底部 1px 分割线
左侧搜索图标
中间输入框
右侧“搜索”文本按钮
```

不要使用大圆角灰底输入框。

### 8.3 筛选器样式

要求：背景透明，不要圆角矩形框。

商品类别入口：

```text
商品类别    全部 >
```

用普通文本行 + 分割线，不做按钮外观。

### 8.4 分类选择双栏

当前问题：双栏元素像按钮，选中状态不明显，内容多时不能滚动。

新结构：

```text
Dialog / Fullscreen panel
  左栏 ScrollView
    一级分类表格行
  右栏 ScrollView
    二级分类表格行
```

表格行样式：

```text
高度 48dp
左右 padding 16dp
透明背景
底部分割线
选中行背景 #EEF2F7
选中行左侧 3dp 黑色竖线
```

左栏点击一级分类：

```text
更新 pendingPrimary
左栏选中背景加深
右栏刷新对应 children
```

右栏点击二级分类：

```text
更新 pendingCategoryId / pendingCategoryName
右栏选中背景加深
```

确定后刷新商品列表，并清空当前分页缓存。

### 8.5 推荐理由展开修复

当前问题：推荐理由即使展开仍显示不全。

修复方向：

- 展开时 `TextView.setMaxLines(Integer.MAX_VALUE)`。
- 同时设置 `setSingleLine(false)`。
- 不要给展开后的容器固定高度。
- 父级 `ScrollView` 允许自然撑高。
- 如果内容仍很长，保持在详情页主滚动中，而不是内嵌不可滚动小框。

## 9. Agent Block 新增渲染

Android `renderAgentBlock` 增加：

```text
product_refs
  -> 拉商品详情，渲染商品卡

discount_preview
  -> 渲染优惠明细卡

coupon_list
  -> 渲染优惠券列表，可领取按钮

review_summary
  -> 渲染评价摘要

navigation_action
  -> 渲染跳转行，点击进入对应页面
```

未识别 block：

```text
不崩溃
不展示原始 JSON 给普通用户
只展示“暂不支持该内容展示”
```

## 10. UI 风格统一规则

本次需要减少“到处都是圆角矩形按钮”的感觉。

统一规则：

- 导航、筛选、分类、订单动作优先使用文本行、图标、分割线。
- 卡片只用于商品卡、订单卡、优惠券卡、评价卡等内容容器。
- 页面级筛选器不做卡片。
- 表单输入可以用下划线样式，不用大灰底圆角框。
- 主要动作按钮只保留在关键提交处，例如“支付”“提交评价”“确认下单”。
- 同一页面内图标风格统一：线性、单色、居中。

## 11. 涉及文件

Android：

- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
- `android-native/app/src/main/java/com/xzxg/shop/ApiClient.java`
- `android-native/app/src/main/java/com/xzxg/shop/LocalChatStore.java`
- `android-native/app/src/main/java/com/xzxg/shop/MarkdownRenderer.java`
- `android-native/app/src/main/AndroidManifest.xml`

后端/API 对齐：

- 从 `origin/main` 合并或同步 `.ai/api/API接口文档_v3.md`
- 从 `origin/main` 合并或同步 `.ai/api/Agent输出协议_v1.md`
- 确认当前分支包含订单、优惠、评价、远程会话 API

## 12. 验收标准

### 12.1 聊天页

- 长按语音后，能拿到识别文本并发送。
- 麦克风图标不是照片，和键盘图标风格一致。
- 点击追问后，追问文本进入输入框，不自动发送。
- 发送后能看到后端 `status.text`，例如“正在检索商品”。
- Agent 返回 `product_refs` 或 `<item>p_xxx</item>` 后，Android 能拉取商品详情并展示商品卡。
- Markdown 一级标题不再明显大于正文。

### 12.2 侧栏

- 四个导航行文本左对齐。
- 已登录时能从远程拉取历史会话。
- 同一个远程会话不会在本地重复出现。

### 12.3 商品页

- 首次进入只请求 20 条商品。
- 下滑时继续分页请求。
- 搜索栏不是圆角矩形。
- 商品类别筛选不是圆角按钮。
- 分类选择双栏均可滚动。
- 左栏选中分类背景变深。
- 推荐理由展开后全文可见。

### 12.4 优惠与活动

- 可查看可领取优惠券。
- 可领取优惠券。
- 可查看我的优惠券。
- 可查看活动/促销列表。
- 购物车能展示优惠预览。

### 12.5 订单与评价

- 顾客只能看到自己的订单。
- 待支付订单可支付、可取消。
- 已发货订单可确认收货。
- 已完成订单项可发布评价。
- 商品详情可查看评价。

## 13. 实施顺序

建议按以下顺序实现：

```text
1. 同步 origin/main 的 API 文档和后端 v3 接口到当前分支
2. 修复聊天页：语音、图标、追问填充、status 展示、Markdown 字号
3. 实现 product_refs / <item> 商品卡渲染
4. 实现远程会话列表与本地同步
5. 修复商品页分页、搜索栏、筛选器、双栏分类、推荐理由展开
6. 实现优惠券和活动页面
7. 改造购物车优惠预览与下单支付流程
8. 实现订单详情动作与商品评价
9. 模拟器逐页验收并截图记录
```

其中第 1 步必须先做，否则 Android 端会继续基于旧后端接口开发，后续还会反复返工。
