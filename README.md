# xzxg-shop

小猪小狗电商 AI 导购系统，包含 Go 后端、React 管理/商家/用户 Web 前端、Java 原生 Android 客户端，以及本地评测和中间件配置。

## 目录结构

```text
backend/        Go API、Agent runtime、RAG、MySQL 存储
frontend/       React Web 前端，包含用户、商家、管理员页面
android-native/ Java 原生 Android 客户端
quality/        评测数据、评测脚本和报告
deployments/    本地 MySQL、Nacos、Milvus、Redis、MinIO 等中间件
.ai/            产品、API、技术文档
```

## 本地启动顺序

建议按下面顺序启动：

```bash
docker compose -f deployments/docker-compose.yml up -d
cd backend && go run ./cmd/api
cd frontend && npm run dev -- --port 5173
```

默认访问地址：

```text
后端 API: http://localhost:8080
Web 前端: http://localhost:5173
Nacos:    http://localhost:8848/nacos
MinIO:    http://localhost:9001
Milvus:   http://localhost:19530
MySQL:    127.0.0.1:3306
Redis:    127.0.0.1:6379
```

8080 和 5173 端口都已经开放在公网，可快速体验：  
http://82.156.207.98:8080/  
http://82.156.207.98:5173/  

评委快速体验路线、演示账号和本地部署说明见 [.ai/docs/部署与评委快速体验指南.md](.ai/docs/部署与评委快速体验指南.md)。

后端健康检查：

```bash
curl http://localhost:8080/api/v1/health
```

## 前端配置

### Web 前端

Web 前端位于 `frontend/`，使用 React、TypeScript 和 Vite。

常用命令：

```bash
cd frontend
npm install
npm run dev -- --port 5173
npm run build
npm run lint
```

前端请求统一使用相对路径 `/api/v1`。开发环境通过 [frontend/vite.config.ts](frontend/vite.config.ts) 里的 Vite proxy 转发到后端：

```ts
server: {
  port: 5173,
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true
    }
  }
}
```

如果后端不在 `localhost:8080`，修改 `frontend/vite.config.ts` 中的 `target`，例如：

```ts
target: 'http://127.0.0.1:8082'
```

生产构建后，仍需要由网关或静态资源服务器把 `/api` 代理到后端服务。

### Android 原生客户端

Android 原生客户端位于 `android-native/`，使用 Java 原生实现。

常用命令：

```bash
cd android-native
sh ./gradlew :app:compileDebugJavaWithJavac
sh ./gradlew :app:assembleDebug
```

默认后端地址配置在 [android-native/app/build.gradle](android-native/app/build.gradle)：

```gradle
buildConfigField 'String', 'DEFAULT_API_BASE', '"http://82.156.207.98:8080/api/v1"'
```

Debug 包开启了测试后端设置：

```gradle
buildConfigField 'boolean', 'SHOW_TEST_SERVER_SETTINGS', 'true'
```

因此调试时可以在 App 登录页或设置页修改后端地址。Release 包默认关闭测试后端设置，需要在 `build.gradle` 中修改 `DEFAULT_API_BASE` 后重新打包。

Android 端本地会话历史使用 SQLite 数据库 `xzxg_chat.db`，账号信息和接口地址保存在 SharedPreferences `xzxg_session` 中。历史会话按 `account_id` 隔离。

### Capacitor 前端壳

仓库中还保留了 `frontend/android` 和 `frontend/ios` 的 Capacitor 工程。相关命令：

```bash
cd frontend
npm run native:sync
npm run native:android
npm run native:ios
```

当前主要移动端体验以 `android-native/` 为准。

## 后端配置

后端位于 `backend/`，入口为 [backend/cmd/api/main.go](backend/cmd/api/main.go)。

### 基础启动

```bash
cd backend
go run ./cmd/api
```

默认监听地址：

```text
:8080
```

可通过环境变量覆盖：

```bash
API_ADDR=0.0.0.0:8082 go run ./cmd/api
```

### MySQL

默认 MySQL DSN：

```text
root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local
```

覆盖方式：

```bash
MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local' go run ./cmd/api
```

本地开发默认自动执行数据库迁移：

```text
RUN_MIGRATIONS=true
```

生产环境 `APP_ENV=production` 时默认不自动迁移。如需显式开启：

```bash
APP_ENV=production RUN_MIGRATIONS=true go run ./cmd/api
```

### Nacos 配置中心

后端启动时会连接 Nacos，并把默认配置写入配置中心。Nacos 不可用时会回退到内存默认值，方便本地开发。

默认连接配置：

```text
NACOS_ADDR=http://127.0.0.1:8848
NACOS_NAMESPACE=
NACOS_GROUP=XZXG_SHOP
NACOS_DATA_ID=xzxg-shop-app-config.json
```

覆盖示例：

```bash
NACOS_ADDR=http://127.0.0.1:8848 \
NACOS_GROUP=XZXG_SHOP \
NACOS_DATA_ID=xzxg-shop-app-config.json \
go run ./cmd/api
```

后台配置接口：

```text
GET   /api/v1/admin/configs
PATCH /api/v1/admin/configs/{config_key}
```

Agent Prompt 有独立的后台接口：

```text
GET   /api/v1/admin/prompts
PATCH /api/v1/admin/prompts/{prompt_key}
POST  /api/v1/admin/prompts/{prompt_key}/publish
```

运行时会优先读取数据库中 active 状态的 Prompt，然后再回退到代码默认 Prompt。

### 模型配置

本地开发可以直接用环境变量配置模型：

```bash
export DASHSCOPE_API_KEY='your-api-key'
export AI_BASE_URL='https://dashscope.aliyuncs.com/compatible-mode/v1'
export AI_SMALL_MODEL='qwen3.5-flash'
export AI_LARGE_MODEL='qwen3.7-plus'
export AI_ENABLE_THINKING=false
go run ./cmd/api
```

对应代码入口：

```text
backend/src/agent/config.go
```

也可以在 Nacos/后台配置中心中配置：

```text
ai.active_provider
ai.qwen.base_url
ai.qwen.api_key
ai.qwen.small_model
ai.qwen.large_model
ai.doubao.base_url
ai.doubao.api_key
ai.doubao.small_model
ai.doubao.large_model
ai.enable_thinking
ai.model.planner_route
ai.model.planner_guide_intent
ai.model.planner_non_guide_intent
ai.model.react_guide
ai.model.react_non_guide
```

如果没有配置 `DASHSCOPE_API_KEY` 或 `ai.qwen.api_key`，Agent 会退回本地规则响应，便于无模型密钥时启动项目。

### RAG 和向量检索

本地中间件包含 Milvus、etcd 和 MinIO。默认配置：

```text
vector.enabled=true
milvus.address=http://127.0.0.1:19530
milvus.token=root:Milvus
milvus.collection.products=product_text_vectors
milvus.collection.knowledge=knowledge_text_chunks
milvus.collection.product_images=product_image_vectors_v2
embedding.base_url=https://dashscope.aliyuncs.com/compatible-mode/v1
embedding.model=text-embedding-v4
image_embedding.provider=dashscope
image_embedding.model=qwen3-vl-embedding
image_embedding.dimension=512
```

本地环境默认会启动向量索引初始化。生产环境默认关闭，需要显式开启：

```bash
BOOTSTRAP_VECTOR_INDEX=true go run ./cmd/api
```

如果 Milvus 不可用，商品和知识检索会降级到 MySQL 关键词检索。

### 文件与对象存储

默认对象存储使用本地 MinIO：

```text
minio.endpoint=127.0.0.1:9000
minio.access_key=minioadmin
minio.secret_key=minioadmin
minio.bucket=xzxg-shop-assets
minio.use_ssl=false
```

文件上传大小限制：

```text
files.max_upload_bytes=10485760
```

文件下载需要登录，且只能访问自己的文件或由管理员访问。

### 语音配置

后端同时支持讯飞语音识别、讯飞 TTS 和豆包 TTS。

实时语音识别相关配置：

```text
xunfei.app_id
xunfei.api_key
xunfei.api_secret
xunfei.rtasr.base_url
xunfei.rtasr.path
xunfei.rtasr.lang
xunfei.rtasr.audio_encode
xunfei.rtasr.sample_rate
```

TTS 供应商选择：

```text
tts.provider=xunfei
```

讯飞 TTS：

```text
xunfei.tts.enabled=true
xunfei.tts.app_id
xunfei.tts.api_key
xunfei.tts.api_secret
xunfei.tts.base_url=wss://tts-api.xfyun.cn/v2/tts
xunfei.tts.voice=xiaoyan
xunfei.tts.speed=50
xunfei.tts.volume=50
xunfei.tts.pitch=50
xunfei.tts.timeout_seconds=20
xunfei.tts.max_runes=800
```

豆包 TTS：

```text
tts.provider=doubao
doubao.tts.enabled=true
doubao.tts.app_id
doubao.tts.api_key
doubao.tts.base_url=https://openspeech.bytedance.com/api/v1/tts
doubao.tts.cluster=volcano_tts
doubao.tts.voice=BV700_streaming
doubao.tts.encoding=mp3
doubao.tts.uid=xzxg-shop
doubao.tts.speed_ratio=1.0
doubao.tts.volume_ratio=1.0
doubao.tts.pitch_ratio=1.0
doubao.tts.timeout_seconds=20
doubao.tts.max_runes=800
```

客户端会调用：

```text
GET  /api/v1/speech/tts/config
POST /api/v1/speech/tts
```

### HTTP 与安全配置

常用配置：

```text
http.cors.allowed_origins=*
http.trusted_proxy_cidrs=
http.trust_all_proxies=false
risk.enabled=true
risk.account_statuses=risk
risk.account_message=当前账号命中平台风控限制，暂时无法继续使用导购 Agent。
```

生产环境会校验不安全配置。`APP_ENV=production` 时，以下配置不能使用本地默认值：

```text
MYSQL_DSN 不能是 root:root
http.cors.allowed_origins 不能是 *
minio.access_key / minio.secret_key 不能是 minioadmin
milvus.token 不能是 root:Milvus
ai.qwen.api_key 不能为空
```

## 本地中间件

启动：

```bash
docker compose -f deployments/docker-compose.yml up -d
```

停止：

```bash
docker compose -f deployments/docker-compose.yml down
```

清空本地数据卷：

```bash
docker compose -f deployments/docker-compose.yml down -v
```

中间件用途：

```text
MySQL: 业务数据、用户、商品、购物车、订单、Agent 会话、trace、Prompt
Nacos: 动态配置和应用配置
Milvus: 商品、知识、图片向量检索
etcd: Milvus 依赖
MinIO: Milvus 依赖和对象存储
Redis: 预留给缓存、限流等扩展
```

## 评测

常用评测命令：

```bash
node quality/evals/run_intent_eval.mjs quality/data/eval/intent_cases.jsonl
node quality/evals/run_rag_recall_eval.mjs quality/data/eval/rag_recall_cases.jsonl
node quality/evals/run_agent_e2e.mjs quality/data/eval/agent_e2e_20_scenarios.jsonl
```

报告输出到：

```text
quality/reports
```

## 常见问题

### 前端 5173 能打开，但接口 404 或连接失败

确认后端是否运行在 8080：

```bash
curl http://localhost:8080/api/v1/health
```

如果后端端口不是 8080，需要修改 `frontend/vite.config.ts` 的 proxy target。

### 后端启动时报 MySQL 连接失败

先启动本地中间件：

```bash
docker compose -f deployments/docker-compose.yml up -d mysql
```

确认 DSN 是否和本地端口一致：

```text
MYSQL_DSN=root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local
```

### Agent 不调用真实模型

检查是否配置了模型 Key：

```bash
echo $DASHSCOPE_API_KEY
```

或者在后台配置中心确认：

```text
ai.qwen.api_key
ai.enabled
ai.active_provider
```

### TTS 提示不可用

检查当前供应商和密钥配置：

```text
tts.provider
xunfei.tts.enabled
xunfei.tts.app_id
xunfei.tts.api_key
xunfei.tts.api_secret
doubao.tts.enabled
doubao.tts.app_id
doubao.tts.api_key
```

讯飞可用时，建议保持：

```text
tts.provider=xunfei
xunfei.tts.enabled=true
```
