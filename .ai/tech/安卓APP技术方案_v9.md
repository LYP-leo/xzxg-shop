# 安卓 APP 技术方案 v9

## 1. 背景

本文针对 [安卓APP_问题_v8.md](./安卓APP_问题_v8.md) 设计下一版 Android 原生 APP 改造方案。

v8 已经补齐了聊天结构化块、历史恢复、商品详情返回位置记忆等能力，但当前仍有四类问题：

- 聊天页的“您还可以继续追问”目前直接点击后会发送消息，但后端提供的推荐内容暂时还不适合直接触发提问。
- 语音输入是否可以仅靠 Android APP 实现，需要明确技术边界。
- 多媒体上传需要先确认后端是否真正支持二进制文件上传。
- 商品详情图片被裁剪，商品列表一次性加载过多导致从详情返回列表时仍会卡顿。

v9 的目标是把这几类问题拆成可实现、可验收的改造项，避免后续开发时再次出现“界面看起来有功能，但底层接口不完整”的情况。

## 2. 设计结论

v9 按以下方向改造：

```text
聊天页
  追问内容本版先改为不可点击提示
  后续如果恢复点击，也只能快捷粘贴到输入框，不能直接发送
  保留现有输入栏设计，把最右侧三条曲线图标改为麦克风图标
  点击麦克风切换到语音输入模式，长按说话后直接识别并发送
  点击 + 展开附件面板，图片/文件先进入输入栏上方缓冲区

商品页
  商品详情图片完整展示，不再裁剪
  点击详情图片进入图片预览
  商品列表改为 20 条一页分页加载
  已加载商品缓存在手机端
  从详情返回列表时不重新整页加载，继续保留原滚动位置
```

其中需要特别说明：当前后端已经支持在聊天消息里携带 `attachments` 元数据，但没有真正的图片/文件二进制上传接口。因此“多媒体上传”不能只改 Android 端，否则本地相册图片无法被后端或模型真实访问。

## 3. 聊天追问提示改造

### 3.1 当前问题

当前 Android 端 `renderFollowups(...)` 会把每条追问渲染成 `Button`：

```java
Button chip = secondaryButton(question);
chip.setOnClickListener(v -> sendMessage(...));
```

这会导致用户点击提示后直接发送一条新消息。

这不是说“追问推荐”这个功能方向错误，而是当前后端给出的推荐内容暂时还不适合直接点击触发提问。本版先把它降级为只展示。

### 3.2 v9 改造规则

追问本版只作为提示展示，不再作为快捷提问入口。

展示形态：

```text
您还可以继续追问：
预算范围大概是多少？
主要用途是什么？
品牌有偏好吗？
```

交互规则：

- 不使用 `Button`。
- 不绑定 `OnClickListener`。
- 可继续使用浅色 chip 样式，但控件类型应改为 `TextView`。
- 视觉上不要表现成“可点击按钮”，避免用户误解。

### 3.3 后续可点击版本规则

如果后续后端推荐内容质量稳定，可以恢复点击交互，但点击后也不能直接发送消息。

正确规则是：

```text
点击追问提示
  -> 把该追问文本快捷粘贴到输入框
  -> 用户确认后再手动发送
```

禁止：

```text
点击追问提示
  -> 直接调用 sendMessage(question)
```

### 3.4 影响范围

Android：

- `MainActivity.renderFollowups(...)`

本地存储：

- `followups_json` 继续保留。
- 历史恢复时仍展示追问提示，只是不可点击。

## 4. 语音输入调研与方案

### 4.1 结论

语音输入可以不依赖本项目后端，由 Android APP 调用系统语音识别能力实现。

推荐第一版使用 Android 原生能力：

- `SpeechRecognizer`
- `RecognizerIntent`
- 权限：`RECORD_AUDIO`

但这里有一个边界：它不依赖我们的后端，不代表完全不联网。实际识别由手机系统中的语音服务完成，不同设备可能调用本地离线包，也可能调用系统服务商的云端识别。

### 4.2 不建议第一版自研离线识别

完全离线、不依赖任何系统云服务的方案需要在 APP 内集成语音识别模型，例如 Vosk 或 whisper.cpp。

这会带来：

- APK 体积明显增加。
- 低端手机 CPU 和电量压力增大。
- 中文识别模型管理复杂。
- 需要额外处理采样率、降噪、端点检测、模型加载耗时。

因此 v9 第一版不走自研离线 STT，而是先用系统 `SpeechRecognizer`。

### 4.3 交互设计

默认情况下保持现有主页底部输入栏设计不变，只把最右侧的“三条曲线”语音图标换成明确的麦克风图标。

默认键盘输入模式：

```text
[ + ] [   输入框   ] [ 麦克风图标 ]
```

用户通过键盘输入。输入框有内容时，发送按钮仍按现有逻辑展示或触发发送；这里不重新设计整条输入栏。

点击麦克风按钮后，进入语音输入模式：

```text
[ + ] [  按住说话  ] [ 键盘图标 ]
```

语音输入模式规则：

```text
1. 检查 RECORD_AUDIO 权限
2. 如果未授权，弹出系统权限申请
3. 用户长按“按住说话”
4. 长按期间调用系统 SpeechRecognizer 进行识别
5. 松手后结束识别
6. 如果识别到文本，直接作为聊天内容发送出去
```

在语音输入模式下，点击右侧键盘图标，回到默认键盘输入模式：

```text
[ + ] [   输入框   ] [ 麦克风图标 ]
```

这里需要区别于键盘模式：语音模式下识别结果不写入输入框，而是直接发送为聊天内容。

### 4.4 异常处理

需要处理以下状态：

- 当前设备不支持 `SpeechRecognizer`：提示“当前设备不支持语音输入”。
- 用户拒绝麦克风权限：提示“需要麦克风权限才能使用语音输入”。
- 没有识别结果：回到普通输入状态。
- 长按过程中识别失败：提示“没有识别到内容，请重试”。
- 语音输入模式下点击键盘图标：停止当前识别并回到键盘输入模式。

### 4.5 影响范围

Android：

- `AndroidManifest.xml` 增加 `RECORD_AUDIO`。
- `MainActivity` 增加语音按钮和识别生命周期。

## 5. 多媒体上传调研与方案

### 5.1 当前后端能力

当前聊天接口 `POST /api/v1/sessions/{session_id}/messages:stream` 支持请求体里的：

```json
{
  "content": "帮我看看这张图",
  "attachments": [
    {
      "attachment_id": "att_001",
      "type": "image",
      "url": "https://example.com/a.jpg",
      "name": "a.jpg"
    }
  ]
}
```

后端领域模型已有：

```go
type Attachment struct {
    AttachmentID string `json:"attachment_id"`
    Type         string `json:"type"`
    URL          string `json:"url,omitempty"`
    Name         string `json:"name,omitempty"`
}
```

Agent 运行时也会根据 `attachment.Type == "image"` 判断是否进入图片相关意图。

### 5.2 当前缺口

后端目前缺少真正的文件上传接口，例如：

```text
POST /api/v1/attachments
Content-Type: multipart/form-data
```

这意味着 Android 本地相册里的图片无法直接被后端访问。仅把本地路径或 `content://...` 传给后端是无效的，因为这些地址只在手机本机可用。

Web 端当前使用的 `URL.createObjectURL(file)` 也只是浏览器本地临时地址，不是服务端可访问 URL，因此不能视为完整上传能力。

### 5.3 v9 推荐方案

先补后端上传接口，再接 Android 多媒体上传。

后端新增：

```text
POST /api/v1/attachments
Content-Type: multipart/form-data
字段：
  file: 二进制文件
  type: image / audio / video / file

返回：
{
  "attachment_id": "att_xxx",
  "type": "image",
  "url": "/api/v1/attachments/att_xxx/content",
  "name": "photo.jpg",
  "mime_type": "image/jpeg",
  "size": 123456
}
```

后端同时新增静态访问接口：

```text
GET /api/v1/attachments/{attachment_id}/content
```

存储方式第一版用本地磁盘即可：

```text
backend/uploads/{account_id}/{attachment_id}
```

需要校验：

- 登录用户只能访问自己的附件。
- 限制单文件大小，第一版建议 10MB。
- 限制 MIME 类型，第一版只开放 `image/jpeg`、`image/png`、`image/webp`。
- 文件名不能直接作为磁盘路径使用，避免路径穿越。

### 5.4 Android 端上传流程

输入栏继续保持现有设计，不新增独立图片按钮：

```text
[ + ] [   输入框   ] [ 麦克风图标 ]
```

点击 `+` 后，在输入栏下方展开附件面板。附件面板第一行展示系统资源入口：

```text
[ 相册 ] [ 文件 ] [ 其他预留入口... ]
```

入口规则：

- `相册`：点击后跳转系统相册选择图片。
- `文件`：点击后跳转系统文件选择器选择文件。
- 后续如果要支持拍照，可以在同一区域增加 `拍照` 入口，但 v9 第一版不做拍照。

流程：

```text
1. 用户点击输入栏左侧 +
2. 下方展开附件面板
3. 用户点击“相册”或“文件”
4. 跳转系统选择器
5. 用户选择图片或文件
6. 选择结果先进入输入栏上方缓冲区
7. 用户可以继续点击缓冲区最右侧 + 添加更多附件
8. 用户也可以点击已选附件右上角 x 删除附件
9. 用户确认后点击发送
10. 先逐个调用 POST /api/v1/attachments 上传附件
11. 全部上传成功后拿到 attachment 元数据
12. 再调用 messages:stream，把 attachments 一起发送
```

缓冲区展示规则参考用户给出的输入栏截图：

- 位于输入框上方。
- 已选择图片展示缩略图。
- 已选择文件展示文件名、类型和大小。
- 每个已选附件右上角有 `x` 删除按钮。
- 最右侧保留一个 `+` 占位卡片，点击继续选择附件。
- 附件还没有点击发送前，不上传到后端。
- 删除附件后，不再上传该附件。

发送规则：

- 上传中禁用发送按钮。
- 任意一个附件上传失败时，不发送聊天消息。
- 上传失败时不发送消息，提示用户重试。
- 纯文本消息仍按原逻辑发送。

### 5.5 Android 权限策略

优先使用 Android Photo Picker：

- Android 13 及以上不需要读取相册权限。
- Android 12 及以下可使用 `ACTION_OPEN_DOCUMENT`，也尽量不申请全量存储权限。

如果后续需要拍照：

- 再增加 `CAMERA` 权限。
- 使用 `FileProvider` 保存拍照结果。

v9 第一版只做相册选图，不做拍照。

### 5.6 后端未完成前的临时规则

如果后端上传接口还没实现，Android 不应该做一个“看起来能上传”的假功能。

临时规则：

- 可以做图片选择和预览。
- 点击发送时提示“当前后端暂不支持图片上传”。
- 不发送 `content://`、本地文件路径、空 URL 给后端。

## 6. 商品详情图片完整展示与预览

### 6.1 当前问题

当前商品图片使用：

```java
image.setScaleType(ImageView.ScaleType.CENTER_CROP);
```

`CENTER_CROP` 会为了填满容器裁剪图片，商品详情页不适合这样做。

### 6.2 v9 改造规则

商品列表缩略图可以继续使用裁剪样式，但商品详情主图必须完整展示。

新增两个图片方法：

```java
productThumbImage(...)
  商品列表卡片使用，允许 CENTER_CROP

productDetailImage(...)
  商品详情主图使用，必须 FIT_CENTER 或 CENTER_INSIDE
```

详情页主图规则：

- 使用 `ImageView.ScaleType.FIT_CENTER`。
- 背景使用浅灰或白色，避免透明图片看不清。
- 不强行裁剪。
- 如果图片比例很长或很宽，也必须完整显示。

### 6.3 点击放大

点击详情主图打开全屏预览。

预览页面规则：

```text
黑色背景
图片居中完整展示
点击关闭按钮或系统返回关闭
```

第一版可以先实现完整展示和关闭，不强制做双指缩放。后续如果需要更精细体验，再增加手势缩放。

## 7. 商品列表分页加载与缓存

### 7.1 当前问题

当前 Android 商品列表调用：

```java
api.products(keyword, categoryId)
```

后端 `GET /api/v1/products` 一次返回所有商品。Android 端拿到全部数据后一次性渲染所有卡片，即使已经记住滚动位置，从详情返回时仍可能因为重新加载和重新渲染大量 View 导致卡顿。

### 7.2 后端接口改造

商品列表接口增加分页参数：

```text
GET /api/v1/products?keyword=手机&category_id=c_phone&limit=20&cursor=0
```

返回结构：

```json
{
  "items": [],
  "next_cursor": "20",
  "has_more": true
}
```

第一版可以用 offset cursor：

```text
cursor = 已加载条数
limit = 20
```

排序必须稳定，避免分页过程中重复或丢失：

```sql
ORDER BY updated_at DESC, product_id DESC
```

如果后续商品数量继续增加，再升级为基于 `updated_at + product_id` 的游标分页。

### 7.3 Android API 改造

`ApiClient.products(...)` 改为支持分页：

```java
ProductPage products(String keyword, String categoryId, int limit, String cursor)
```

数据结构：

```java
class ProductPage {
    JSONArray items;
    String nextCursor;
    boolean hasMore;
}
```

### 7.4 Android 缓存模型

新增商品列表缓存，缓存 key 由筛选条件决定：

```text
cacheKey = keyword + "::" + categoryId
```

缓存内容：

```text
items        已加载商品
nextCursor   下一页 cursor
hasMore      是否还有更多
scrollY      当前滚动位置
loadedAt     加载时间
```

第一版优先使用内存缓存，满足从详情返回不卡顿：

```java
Map<String, ProductListState> productListCache
```

如果要支持 APP 重启后仍保留商品缓存，再增加 SQLite 持久化。v9 实现阶段建议先做内存缓存，因为当前问题主要发生在页面内跳转返回。

### 7.5 加载策略

进入商品页：

```text
1. 根据 keyword + categoryId 找缓存
2. 如果有缓存，直接渲染缓存 items，并恢复 scrollY
3. 如果没有缓存，加载第一页 20 条
```

向下滑动：

```text
1. 当距离底部小于约 5 个商品卡片高度时触发下一页加载
2. 如果 loadingMore = true，不重复请求
3. 如果 hasMore = false，不再请求
4. 请求成功后追加到现有列表，不清空整个列表
```

从详情页返回：

```text
1. 不重新请求第一页
2. 不清空并重建整个商品列表
3. 直接使用缓存里的 View 数据恢复
4. scrollTo(savedScrollY)
```

### 7.6 切换筛选条件

当关键词或分类变化：

```text
1. 生成新的 cacheKey
2. 如果该条件已有缓存，直接恢复
3. 如果没有缓存，加载第一页
4. 新筛选条件下 scrollY 从 0 开始
```

### 7.7 列表底部状态

底部增加状态行：

```text
正在加载更多...
已经到底了
加载失败，点击重试
```

失败时不清空已加载商品，只允许重试下一页。

## 8. 涉及文件

Android：

- `android-native/app/src/main/AndroidManifest.xml`
- `android-native/app/src/main/java/com/xzxg/shop/MainActivity.java`
- `android-native/app/src/main/java/com/xzxg/shop/ApiClient.java`
- `android-native/app/src/main/java/com/xzxg/shop/ImageLoader.java`
- 新增 `ProductListState.java` 或在 `MainActivity` 内部先定义轻量状态类

后端：

- `backend/src/httpapi/server.go`
- `backend/src/domain/types.go`
- `backend/src/store/store.go`
- `backend/src/store/mysql.go`
- `backend/src/store/memory.go`
- 如实现本地附件存储，新增 `backend/uploads/.gitkeep`，但上传文件本身不提交到 Git

## 9. 验收标准

### 9.1 聊天页

- “您还可以继续追问”下方内容不可点击。
- 点击追问文字不会发送消息。
- 后续如果恢复点击，点击结果只能粘贴到输入框，不能直接发送。
- 历史会话恢复后追问提示仍能显示。
- 默认输入栏保持 `[ + ] [ 输入框 ] [ 麦克风图标 ]`。
- 点击麦克风后切换为 `[ + ] [ 按住说话 ] [ 键盘图标 ]`。
- 长按“按住说话”可以调起系统语音识别。
- 语音识别成功后，识别结果直接作为聊天内容发送。
- 点击键盘图标可以回到默认键盘输入模式。

### 9.2 多媒体上传

如果后端上传接口已实现：

- 点击输入栏左侧 `+` 后，下方展示附件面板。
- 附件面板第一项是“相册”，第二项是“文件”。
- 点击“相册”可以跳转系统相册选择图片。
- 点击“文件”可以跳转系统文件选择器选择文件。
- 选择后的图片或文件先进入输入栏上方缓冲区，不会立即发送。
- 已选图片右上角有 `x`，点击可从缓冲区删除。
- 缓冲区最右侧有 `+` 占位卡片，点击可继续添加附件。
- 上传成功后，聊天请求中包含后端返回的 attachment 元数据。
- 上传失败时不发送消息。

如果后端上传接口未实现：

- APP 不发送本地图片路径给后端。
- 用户能看到明确提示：当前后端暂不支持图片上传。

### 9.3 商品详情

- 商品详情主图完整显示，不裁剪。
- 点击主图可以进入大图预览。
- 返回键可以关闭大图预览。

### 9.4 商品列表

- 首次进入商品页只请求约 20 条商品。
- 滑动到底部附近才加载下一页。
- 下一页加载时不清空已有列表。
- 从详情页返回商品列表时，保留原滚动位置。
- 从详情页返回时不重新加载已加载的全部商品。
- 切换分类或搜索词后，列表从新条件第一页开始。

## 10. 实施顺序

建议按以下顺序实现：

```text
1. 修改 followups 为不可点击提示
2. 商品详情图片完整展示 + 大图预览
3. 后端商品分页接口
4. Android 商品分页加载 + 内存缓存 + 返回位置恢复
5. 语音输入
6. 后端附件上传接口
7. Android 图片选择、预览、上传、随消息发送
```

原因是前四项能直接解决当前可见体验问题；语音输入相对独立；多媒体上传需要后端接口和 Android 端一起完成，放在后面可以降低返工。
