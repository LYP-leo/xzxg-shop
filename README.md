# 项目部署与体验方式

本文面向比赛评委、验收同学和项目演示人员，目标是用最短路径体验【小猪小狗 AI 导购系统】

## **一、最快体验方式**

如果只想快速体验，不需要本地部署，可以使用我们已部署的公网环境。

> 注意：由于自费服务器带宽较小，上传高清图像时可能卡顿，烦请耐心等待。
> 
> 

|服务|地址|用途|
|---|---|---|
|Web 管理/商家后台|`http://82.156.207.98:5173/`|管理员后台、商家后台|
|后端 API|`http://82.156.207.98:8080/`|API 服务入口|
|健康检查|`http://82.156.207.98:8080/api/v1/health`|判断后端是否在线|



客户端APK：

\[app\-debug\.apk\]



演示账号：

|角色|账号|密码|主要体验内容|
|---|---|---|---|
|管理员|`admin`|`admin123456`|商品、订单、Prompt、配置、链路追踪、质量测评、风控|
|商家|`merchant`|`merchant123456`|商品管理、资料上传、订单查看、发货履约|
|用户|`user`|`user123456`|Android 客户端导购、加购、下单、图片找同款、语音/TTS|

如果用户账号不可用，可以在 Android 客户端注册新用户。

管理员和商家账号建议**使用上表所示的已有账号**，便于看到已有商品、订单和测评数据。



## **二、推荐体验路线**

### **1\. 系统是否可用**

1. 打开 `http://82.156.207.98:8080/api/v1/health`。

2. 返回健康状态后，打开 `http://82.156.207.98:5173/`。

3. 使用 `admin / admin123456` 登录管理员后台。

这一段主要确认后端、数据库、中间件和 Web 后台都已连通。



### **2\. 管理员后台看平台能力**

建议优先查看以下页面：

|页面|看点|
|---|---|
|平台管理|商品、订单、用户、商家等基础电商数据|
|Prompt 管理|Agent Prompt 统一落库，可在后台编辑、发布和回滚|
|调试观测 / 链路追踪|每次 Agent 请求的意图、Prompt、模型输出、工具调用、RAG 片段、耗时|
|质量测评|意图识别、RAG 召回、Agent 端到端测评报告和明细|
|风控管理|用户、商品、商家、输入内容等风险控制|



### **3\. 用户端体验 AI 导购闭环**

用户端以 `android-native/` 原生 Android 客户端为主。若现场已提供 APK，直接安装 APK；如果需要本地打包，参考本文“Android 客户端打包”。

推荐测试问题：

```Plain Text
我主要写代码和做演示，华为电脑和苹果电脑怎么选？
帮我找一双适合夏天通勤的运动鞋，不要太厚重
推荐一款适合敏感肌的面霜，预算 300 左右
把刚才推荐的第一个商品加入购物车
```



### **4\. 商家和管理员看履约闭环**

1. 使用 `merchant / merchant123456` 登录 Web 后台。

2. 进入商家订单页面，查看待发货订单。

3. 对可发货订单执行发货。

4. 切换管理员账号，在订单管理中查看订单状态变化。

如果需要演示管理员代运营能力，也可以在管理员端查看可发货订单并执行发货。



### **5\. 多模态能力体验**

图片找同款：

1. 在 Android 客户端上传或拍摄商品图片。

2. 输入“帮我找同款”或“找类似这件的商品”。

3. 观察后端是否生成图片向量、是否进入图片检索工具、是否返回相似商品。

语音/TTS：

1. 使用客户端语音输入问题。

2. 等待 Agent 回复后播放语音。

3. 如果语音不可用，优先检查网络、麦克风权限和 TTS 配置。



## **三、本地完整部署教程**

### **1\. 环境要求**

|组件|用途|
|---|---|
|Docker / Docker Compose|启动 MySQL、Nacos、Milvus、MinIO、Redis|
|Go|运行后端服务|
|Node\.js / npm|运行 React 管理/商家后台|
|JDK / Android Studio|构建和运行 Android 原生客户端|



### **2\. 启动中间件**

在仓库根目录执行：

```Bash
docker compose -f deployments/docker-compose.yml up -d
```



默认会启动：

|服务|地址|
|---|---|
|MySQL|`127.0.0.1:3306`|
|Redis|`127.0.0.1:6379`|
|Nacos|`http://localhost:8848/nacos`|
|Milvus|`http://localhost:19530`|
|MinIO 控制台|`http://localhost:9001`|



### **3\. 启动后端**

```Bash
cd backend
go run ./cmd/api
```

默认后端地址：

```Plain Text
http://localhost:8080
```



健康检查：

```Bash
curl http://localhost:8080/api/v1/health
```

本地开发默认会自动迁移数据库，并在中间件可用时初始化基础配置、商品数据和向量索引。Prompt 的主来源是数据库和管理员页面，Nacos 主要负责应用动态配置。



### **4\. 启动 Web 后台**

```Bash
cd frontend
npm install
npm run dev -- --port 5173
```



访问：

```Plain Text
http://localhost:5173
```

Web 前端通过 Vite proxy 将 `/api` 转发到 `http://localhost:8080`。如果后端端口变化，需要修改 `frontend/vite.config.ts`。



### **5\. Android 客户端打包**

```Bash
cd android-native
sh ./gradlew :app:assembleDebug
```

Debug APK 默认输出路径：

```Plain Text
android-native/app/build/outputs/apk/debug/app-debug.apk
```



Android 默认后端地址配置在：

```Plain Text
android-native/app/build.gradle
```



当前 Debug 包默认指向公网后端：

```Plain Text
http://82.156.207.98:8080/api/v1
```



如果要连接本地后端，可以在 Debug 设置页修改服务地址，或修改 `DEFAULT_API_BASE` 后重新打包。



## **四、常见问题**

### **Web 能打开，但接口失败**

先检查后端：

```Bash
curl http://localhost:8080/api/v1/health
```

如果本地后端不是 8080，需要修改 `frontend/vite.config.ts` 的 proxy target。



### **Agent 回复像模板或没有真实模型输出**

检查模型配置：

|配置|说明|
|---|---|
|`ai.active_provider`|当前模型供应商|
|`ai.qwen.api_key` / 其他供应商 key|模型密钥|
|`ai.model.react_guide`|导购主 Agent 模型|
|`ai.model.react_non_guide`|非导购主 Agent 模型|
|`ai.enable_thinking`|是否开启 thinking|

无模型密钥时，系统会降级到本地规则，便于服务启动，但不适合评审真实 AI 效果。



### **图片找同款没有结果**

优先检查：

1. 图片是否上传成功。

2. Milvus 是否运行。

3. 图片 embedding 模型配置是否可用。

4. 商品库是否有对应类别和图片向量。



### **管理员看不到最近请求**

确认当前请求是否真正打到同一个后端环境。公网 Web、Android、本地后端如果混用，会导致管理员后台看不到另一个环境的 trace。



### **登录失败**

优先使用本文固定演示账号。用户端如果固定账号不可用，可以直接注册新用户；管理员和商家账号需要使用预置账号或由数据库初始化脚本创建。







