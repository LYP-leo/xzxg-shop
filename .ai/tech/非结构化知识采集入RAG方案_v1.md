# 非结构化知识采集入 RAG 方案 v1

## 1. 背景

当前商品 RAG 主要围绕平台自建知识、商品结构化信息、商品详情、评价、商家资料构建。导购回答如果只依赖商品库，容易缺少真实用户经验、横评、避坑、使用场景等非结构化知识。

用户希望补充小红书、知乎等帖子类内容，建立自动采集、解析、入向量库机制。

本文只做方案设计，不直接实现代码。

## 1.1 生产级目标

这套机制不能按 demo 标准做。生产级要求是：

1. **可控输入**：所有数据源必须可追踪、可审核、可停用、可删除。
2. **可恢复执行**：采集、解析、切片、embedding、入库任何一步失败，都能重试或人工处理。
3. **幂等一致**：同一 URL、同一文件、同一内容重复提交不会产生重复向量和重复 chunk。
4. **安全隔离**：URL 抓取不能造成 SSRF，文件解析不能执行危险内容，管理员权限必须分级。
5. **合规可审计**：保留来源、授权状态、操作者、采集日志、删除日志。
6. **质量可控**：外部内容必须经过质量评分、风控、去重，不能无条件进入 RAG。
7. **召回可解释**：每次命中外部内容，都能看到分数、来源、过滤原因和引用片段。
8. **可观测**：任务吞吐、失败率、embedding 耗时、Milvus upsert 失败、低质量比例必须有指标和告警。
9. **可回滚**：支持禁用单个 source、单个 doc、单批 ingestion job，并让检索实时生效。
10. **不影响主链路**：采集和 embedding 是后台异步任务，不能阻塞用户导购请求。

## 2. 合规边界

小红书、知乎这类平台内容涉及用户原创内容、平台服务条款、robots、登录态和反爬策略。MVP 不做绕登录、绕反爬、批量抓取平台全站内容。

允许的输入方式优先级：

| 类型 | 说明 | 是否 MVP |
| --- | --- | --- |
| `owned_content` | 平台自建选购指南、FAQ、商家授权资料 | 是 |
| `manual_import` | 运营导入 Markdown、HTML、PDF、Excel | 是 |
| `submitted_url` | 管理员手动提交公开 URL，系统抓取解析 | 是 |
| `authorized_feed` | 商家/达人/合作方授权的数据包或接口 | 是 |
| `public_web_allowlist` | 明确允许抓取的网站白名单 | 可选 |
| `login_scraping` | 使用登录态采集小红书/知乎等平台内容 | 暂不做 |

原则：

1. 所有外部内容必须保留来源 URL、采集时间、授权状态。
2. 不把外部帖子长段原文直接展示给用户，只作为证据摘要和引用来源。
3. 支持按 `doc_id`、`source_url` 删除 MySQL 和 Milvus 数据。
4. 支持审核、禁用、重新解析、重新 embedding。
5. 遵守 robots 和站点条款；对未知授权状态内容标记 `needs_review`。
6. 生产环境默认只召回 `license_status in ('authorized', 'owned', 'approved_public')` 的内容；`unknown` 和 `needs_review` 只能在后台调试中可见。
7. 所有外部内容的引用必须保留 `source_url`，不能只保留二次加工后的文本。

## 3. 总体链路

```text
Source Config / URL / 文件
  -> Ingestion Job
  -> Compliance Check
  -> Fetch / Upload Read
  -> Parse Main Content
  -> Clean & Normalize
  -> Metadata Extract
  -> Dedup
  -> Quality & Risk Filter
  -> Chunk
  -> Embedding
  -> MySQL metadata
  -> Milvus vector upsert
  -> Admin Review
  -> RAG search_knowledge
```

## 4. 模块设计

### 4.1 Source Registry

新增数据源注册中心，管理来源能力、权限和采集策略。

```json
{
  "source_id": "source_zhihu_manual",
  "source_type": "zhihu",
  "input_mode": "submitted_url",
  "enabled": true,
  "allow_auto_fetch": true,
  "requires_review": true,
  "respect_robots": true,
  "rate_limit": {
    "qps": 0.1,
    "daily_limit": 200
  },
  "allowed_domains": ["zhihu.com", "zhuanlan.zhihu.com"],
  "license_policy": "manual_review"
}
```

第一版建议只开放：

- `owned_content`
- `manual_import`
- `submitted_url`

生产级要求：

- source 配置必须进入 Nacos 或数据库，并有版本号。
- source 配置变更必须写审计日志。
- source 停用后，新采集任务拒绝创建，已有 active 文档在检索时默认过滤。
- 每个 source 必须配置默认 `license_policy`、`requires_review`、`rate_limit`。
- 不允许代码里硬编码站点特殊逻辑；站点差异放 source config 或 parser template。

### 4.2 Ingestion Job

所有采集动作都落任务表，便于重试、追踪、审计。

状态机：

```text
pending
 -> compliance_checking
 -> fetching
 -> parsing
 -> cleaning
 -> chunking
 -> embedding
 -> indexed
 -> needs_review
 -> failed
 -> disabled
```

任务字段：

```json
{
  "job_id": "ingest_001",
  "source_id": "source_zhihu_manual",
  "input_type": "url",
  "input_value": "https://...",
  "status": "indexed",
  "doc_id": "doc_001",
  "error_message": "",
  "retry_count": 0,
  "created_by": "admin_001",
  "created_at": "...",
  "updated_at": "..."
}
```

生产级要求：

- `job_id` 由服务端生成，客户端可传 `client_request_id` 做幂等键。
- 同一 `source_id + input_type + normalized_input_hash` 默认只允许一个 active job。
- 状态流转必须单向且可审计，禁止任意状态覆盖。
- 任务执行必须有 lease/lock，避免多 worker 重复处理同一任务。
- 每一步写 `step_started_at`、`step_finished_at`、`error_code`、`error_message`。
- 重试使用指数退避，超过阈值进入 `failed` 或 `needs_review`。
- 支持 dead letter 队列视图：长期失败任务可批量重试或关闭。
- 支持取消任务；取消后 worker 在安全检查点停止。

### 4.3 Fetcher

Fetcher 根据 input_type 分流：

| input_type | 处理方式 |
| --- | --- |
| `url` | HTTP GET 公开页面，限制跳转、大小、超时 |
| `html` | 直接解析管理员上传 HTML |
| `markdown` | 直接进入清洗和 chunk |
| `pdf` | PDF 文本抽取 |
| `excel` | 按行或 sheet 解析成 docs |

URL fetch 约束：

- 只允许 http/https。
- 禁止内网 IP、localhost、file URL，防 SSRF。
- 读取 robots。
- domain allowlist。
- 单页大小限制，例如 5MB。
- 超时 10s。
- 不执行登录态绕过。
- 不自动滚动抓 App 页面。

生产级要求：

- SSRF 防护必须做 DNS 解析后 IP 校验，禁止内网、环回、链路本地、云 metadata 地址。
- 跳转后 URL 也必须重新做 allowlist 和 IP 校验。
- HTTP client 必须限制超时、响应大小、content-type、redirect 次数。
- User-Agent 必须标识系统和联系邮箱/站点，不伪装浏览器绕过限制。
- robots 检查结果要缓存，但缓存需要 TTL。
- 对每个 domain 做令牌桶限速，避免突发抓取。
- 原始 HTML 不长期保存大文件；如需留存，放 MinIO 并设置生命周期和权限。
- 文件上传必须做 MIME、扩展名、大小、病毒/恶意内容基础检查。

### 4.4 Parser

目标是把网页、文件解析成统一文档结构。

```json
{
  "title": "敏感肌面霜怎么选",
  "author": "xxx",
  "published_at": "2026-05-01T00:00:00+08:00",
  "raw_text": "...",
  "main_text": "...",
  "images": [],
  "outbound_links": [],
  "metadata": {}
}
```

解析策略：

- HTML：优先使用 Readability 风格正文抽取。
- Markdown：保留标题层级、列表、表格。
- PDF：按页抽文本，记录页码。
- Excel：每行转成一条 doc 或 chunk，按模板配置字段。

生产级要求：

- Parser 输出必须包含 `parse_confidence`，低于阈值进入人工审核。
- 解析失败不能直接丢弃，必须保留失败原因和可下载的诊断信息。
- HTML parser 要保留标题层级，避免正文抽取把商品参数表丢掉。
- PDF/Excel 解析必须设置页数、行数、单元格文本长度上限。
- 解析器版本写入 metadata，方便后续重跑和问题追溯。

### 4.5 Cleaner

清洗规则：

- 去导航、广告、推荐阅读、评论区噪声。
- 去重复空白、表情泛滥、无意义符号。
- 识别“广告/带货/推广/利益相关”信号。
- 保留商品名、品牌名、品类词、规格、适用人群、风险点。
- 保留原始正文 hash，便于去重。

生产级要求：

- Cleaner 规则必须可版本化，清洗前后文本长度、删除比例要记录。
- 删除比例异常高或异常低都进入 `needs_review`。
- 不允许清洗阶段删除来源、作者、发布时间等 provenance 字段。
- 对医疗、美妆功效、食品、母婴等高风险类目，保留“用户经验”与“确定事实”的标签区分。

### 4.6 Metadata Extractor

用规则 + 小模型抽取元信息。

```json
{
  "doc_type": "external_post",
  "source_type": "zhihu",
  "topic": "敏感肌面霜",
  "category_ids": ["beauty_skincare"],
  "brand_terms": ["薇诺娜", "理肤泉"],
  "product_terms": ["面霜", "屏障修护"],
  "scenario_terms": ["敏感肌", "干痒", "换季"],
  "risk_terms": ["刷酸", "过敏"],
  "quality_score": 0.76,
  "commercial_intent_score": 0.42,
  "license_status": "needs_review"
}
```

第一版不要求完全准确，但必须能用于过滤和后台审核。

生产级要求：

- 元信息抽取必须有规则兜底，小模型失败不能让任务失败。
- 小模型抽取输出必须做 JSON schema 校验。
- 抽取结果只作为检索 metadata，不作为事实直接写入回答。
- `quality_score`、`commercial_intent_score`、`risk_terms` 必须可在后台查看和人工修正。
- 高风险字段修正必须写审计日志。

### 4.7 Dedup

去重维度：

- `source_url` 精确去重。
- `content_hash` 精确去重。
- `simhash` 或 embedding 相似度近似去重。

重复策略：

- 同 URL 已存在：更新 metadata，不重复入向量。
- 同内容不同 URL：保留多个 source，但 chunk 只入一份或做 source 聚合。
- 低质量重复内容：标记 disabled。

生产级要求：

- 去重必须在 embedding 前执行，避免重复消耗 token。
- `content_hash` 用规范化正文计算，不能用原始 HTML。
- 近重复判断只作为候选，自动合并需要阈值足够高；低于阈值进入人工审核。
- 删除某个 source URL 时，不能误删仍被其他 source 引用的共享 chunk。

### 4.8 Chunking

Chunk 应保留语义完整性，而不是固定字数硬切。

建议：

- 按标题、段落、列表、表格切。
- 每 chunk 300-600 中文字。
- overlap 60-120 字。
- 表格单独 chunk。
- 每个 chunk 带 doc metadata。

Chunk schema：

```json
{
  "chunk_id": "ck_ext_001_0001",
  "doc_id": "doc_ext_001",
  "chunk_index": 1,
  "title": "敏感肌面霜怎么选",
  "text": "...",
  "source_type": "zhihu",
  "doc_type": "external_post",
  "category_ids": ["beauty_skincare"],
  "brand_terms": ["薇诺娜"],
  "quality_score": 0.76,
  "risk_status": "normal"
}
```

生产级要求：

- chunk 必须稳定生成：同一 doc 同一 parser/chunker 版本下 chunk_id 可复现。
- chunk 不能跨越不相关标题段落，避免召回片段语义混杂。
- 每个 chunk 必须带 `source_url`、`doc_title`、`license_status`、`risk_status`。
- chunk 过短、纯目录、纯广告 CTA 不入向量库。
- chunk 生成后要抽样测评：语义完整率、噪声率、重复率。

### 4.9 Embedding & Milvus

文本 chunk 入现有知识向量集合，或新增集合：

```text
knowledge_text_chunks
```

Milvus metadata 建议包含：

- `chunk_id`
- `doc_id`
- `source_type`
- `doc_type`
- `category_id`
- `quality_score`
- `risk_status`
- `published_at`
- `updated_at`

Embedding 配置放 Nacos：

```json
{
  "external_knowledge": {
    "embedding_model": "text-embedding-v4",
    "chunk_size": 500,
    "chunk_overlap": 100,
    "batch_size": 32,
    "retry": {
      "max_attempts": 3,
      "backoff_ms": 500
    }
  }
}
```

生产级要求：

- embedding job 必须异步批处理，不能在 HTTP 请求内同步完成。
- embedding 请求要有批大小、并发、超时、重试、熔断配置。
- 同一 chunk 文本 hash + embedding model 相同则复用向量，不重复调用模型。
- Milvus upsert 必须幂等，主键使用 `chunk_id + embedding_model_version`。
- MySQL chunk 状态和 Milvus upsert 状态要有补偿任务定期校验。
- embedding model 变更必须触发 reindex plan，不允许新旧向量混用且无版本标记。
- 向量写入失败不影响 doc 保存，但该 chunk 不应进入线上召回。

## 5. 数据表设计

### 5.1 external_sources

```sql
CREATE TABLE external_sources (
  source_id VARCHAR(64) PRIMARY KEY,
  source_type VARCHAR(64) NOT NULL,
  input_mode VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  config_json JSON NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
```

### 5.2 knowledge_ingestion_jobs

```sql
CREATE TABLE knowledge_ingestion_jobs (
  job_id VARCHAR(64) PRIMARY KEY,
  source_id VARCHAR(64) NOT NULL,
  input_type VARCHAR(32) NOT NULL,
  input_value TEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  doc_id VARCHAR(64),
  error_message TEXT,
  retry_count INT NOT NULL DEFAULT 0,
  created_by VARCHAR(64),
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_ingestion_jobs_status (status, updated_at),
  INDEX idx_ingestion_jobs_source (source_id, created_at)
);
```

### 5.3 external_knowledge_documents

```sql
CREATE TABLE external_knowledge_documents (
  doc_id VARCHAR(64) PRIMARY KEY,
  source_id VARCHAR(64) NOT NULL,
  source_type VARCHAR(64) NOT NULL,
  source_url TEXT,
  title VARCHAR(512),
  author VARCHAR(256),
  published_at DATETIME,
  content_hash VARCHAR(128) NOT NULL,
  main_text MEDIUMTEXT NOT NULL,
  metadata_json JSON NOT NULL,
  quality_score DECIMAL(5,4) NOT NULL DEFAULT 0,
  license_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  risk_status VARCHAR(32) NOT NULL DEFAULT 'normal',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_external_docs_source (source_type, status),
  INDEX idx_external_docs_hash (content_hash),
  INDEX idx_external_docs_quality (quality_score)
);
```

### 5.4 external_knowledge_chunks

```sql
CREATE TABLE external_knowledge_chunks (
  chunk_id VARCHAR(64) PRIMARY KEY,
  doc_id VARCHAR(64) NOT NULL,
  chunk_index INT NOT NULL,
  text MEDIUMTEXT NOT NULL,
  metadata_json JSON NOT NULL,
  quality_score DECIMAL(5,4) NOT NULL DEFAULT 0,
  vector_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uniq_doc_chunk (doc_id, chunk_index),
  INDEX idx_external_chunks_doc (doc_id),
  INDEX idx_external_chunks_vector (vector_status, status)
);
```

### 5.5 删除一致性设计

生产级删除不能只删 MySQL 或只删 Milvus。

删除流程：

```text
Admin delete doc
  -> mark document status=deleting
  -> mark chunks status=deleting
  -> enqueue vector_delete_job
  -> delete Milvus vectors by chunk_id
  -> mark chunks status=deleted
  -> mark document status=deleted
  -> write audit log
```

要求：

- 删除接口必须幂等，多次调用结果一致。
- Milvus 删除失败时文档保持 `deleting`，线上检索必须过滤 `deleting`。
- 定时补偿任务扫描 `deleting` 超时记录继续删除。
- 不建议物理删除 MySQL 主记录，保留 tombstone，便于审计和防重复导入。
- 管理员页面必须能看到删除中、删除失败、已删除状态。

### 5.6 审计日志

新增审计表或复用现有审计机制，记录：

- source 创建、修改、停用。
- job 创建、取消、重试。
- doc 审核通过、驳回、禁用、删除。
- metadata 人工修正。
- 风控状态变更。
- 批量操作。

审计字段：

```json
{
  "audit_id": "aud_001",
  "actor_id": "admin_001",
  "action": "knowledge_doc.disable",
  "target_type": "external_knowledge_document",
  "target_id": "doc_001",
  "before": {},
  "after": {},
  "reason": "低质量营销内容",
  "created_at": "..."
}
```

## 6. RAG 检索接入

`search_knowledge` 增加外部知识 corpus。

RetrievalPlan 示例：

```json
{
  "query": "敏感肌面霜怎么选",
  "filters": {
    "doc_types": ["external_post", "guide", "review_summary"],
    "source_types": ["owned", "zhihu", "xiaohongshu_manual"],
    "category_ids": ["beauty_skincare"],
    "min_quality_score": 0.55,
    "risk_status": "normal"
  },
  "recall": {
    "vector": {
      "enabled": true,
      "top_n": 40
    },
    "keyword": {
      "enabled": true,
      "top_n": 40,
      "required_terms": ["敏感肌", "面霜"]
    }
  },
  "rerank": {
    "top_k": 8,
    "weights": {
      "vector_score": 0.45,
      "keyword_score": 0.20,
      "source_score": 0.10,
      "freshness_score": 0.05,
      "quality_score": 0.15,
      "evidence_density": 0.05
    }
  }
}
```

回答约束：

- 外部帖子只能作为参考依据，不能编造为平台官方结论。
- 医疗、美妆功效、母婴、食品等高风险品类要加可信来源优先级。
- 不直接输出长段原文。
- 引用时展示标题、来源、URL。

生产级要求：

- 线上默认只召回 `status=active`、`risk_status=normal`、`license_status` 已通过的 chunk。
- 外部内容权重默认低于自建知识和商家授权资料。
- 高风险品类必须优先平台自建知识、商家规则、商品结构化参数；外部帖子只能补充用户体验。
- RAG trace 必须记录每个外部 chunk 的 `doc_id`、`chunk_id`、source、分数和过滤原因。
- 如果外部内容与平台自建知识冲突，回答优先自建知识，并标记外部内容为冲突候选供后台复核。
- search result 返回给 Agent 前要做证据压缩，避免把长帖片段直接塞进主模型。
- 前端/管理员可视化必须能区分“外部帖子证据”和“平台官方/商家证据”。

## 7. 管理员页面

新增“知识采集”模块：

1. 数据源管理
   - 新增/编辑 source。
   - 设置域名白名单、限速、是否需要审核。

2. 内容导入
   - 单 URL 提交。
   - 批量 URL 上传。
   - Markdown/HTML/PDF/Excel 上传。

3. 任务列表
   - 状态筛选。
   - 失败原因。
   - 重试。
   - 取消。

4. 文档库
   - 查看原文摘要。
   - 查看 metadata。
   - 查看 chunks。
   - 禁用/启用/删除。

5. 向量状态
   - embedding 进度。
   - Milvus upsert 状态。
   - 重建索引。

6. 召回调试
   - 输入 query。
   - 展示召回 chunk。
   - 展示分数来源：vector/keyword/source/quality/freshness。
   - 展示过滤原因。

生产级要求：

- 所有接口必须分页、筛选、排序，默认 page_size 不超过 20。
- 批量操作必须二次确认，并记录 audit log。
- 权限至少区分：只读、运营编辑、审核员、管理员。
- 未审核内容默认不可被普通导购召回。
- 原文展示要做脱敏和安全渲染，禁止直接渲染未清洗 HTML。
- 任务详情页必须展示完整 pipeline step 和耗时。
- 文档详情页必须展示原文、清洗后正文、chunks、metadata、向量状态、召回测试入口。
- 删除、禁用、重建 embedding 必须有操作结果反馈和失败原因。

## 8. 质量与风控

### 8.1 质量过滤

低质量信号：

- 正文过短。
- 广告词密度高。
- 纯营销口号。
- 重复内容。
- 无明确主题。
- 标题党。
- 内容过期。

质量评分低于阈值时：

- 不入向量库，或入库但默认不召回。
- 后台标记 `needs_review`。

### 8.2 风控过滤

外部内容命中风控词时：

- 文档 `risk_status=blocked`。
- chunk 不入向量库或召回时过滤。
- trace 记录过滤原因。

商品/商家风控仍然优先：

- 外部帖子提到被风控商品，不允许让该商品进入推荐候选。

### 8.3 质量门禁

外部知识进入线上召回必须满足：

| 门禁 | 要求 |
| --- | --- |
| 授权状态 | `owned`、`authorized` 或人工审核通过 |
| 解析质量 | `parse_confidence >= 0.7` |
| 内容质量 | `quality_score >= 0.55` |
| 商业倾向 | `commercial_intent_score <= 0.8`，超过进入审核 |
| 风控 | `risk_status=normal` |
| 重复 | 非重复或已合并 |
| 向量 | `vector_status=indexed` |

### 8.4 内容冲突处理

外部帖子和平台自建知识冲突时：

- 检索层不直接删除外部证据，但降低排序分。
- Agent 回答不采用外部内容作为确定事实。
- Trace 记录 conflict candidate。
- 管理后台提供“冲突内容”筛选，运营可禁用或修正。

## 9. API 设计

通用生产级要求：

- 所有管理接口必须管理员鉴权。
- 写接口必须有 CSRF/幂等保护，至少支持 `client_request_id`。
- 批量接口必须限制单次数量，例如最多 100 条。
- 所有列表接口必须分页。
- 错误返回统一 `code/message/request_id`。
- 文件上传必须走 MinIO 或受控对象存储，不直接把大文件塞进 MySQL。
- API 日志不能打印完整原文和敏感授权信息。

### 9.1 创建采集任务

```http
POST /api/v1/admin/knowledge/ingestion-jobs
```

```json
{
  "source_id": "source_zhihu_manual",
  "input_type": "url",
  "input_value": "https://...",
  "options": {
    "requires_review": true
  }
}
```

### 9.2 批量创建

```http
POST /api/v1/admin/knowledge/ingestion-jobs:batch
```

### 9.3 任务列表

```http
GET /api/v1/admin/knowledge/ingestion-jobs?page=1&page_size=20&status=failed
```

### 9.4 文档列表

```http
GET /api/v1/admin/knowledge/documents?page=1&page_size=20&source_type=zhihu&status=active
```

### 9.5 文档详情

```http
GET /api/v1/admin/knowledge/documents/{doc_id}
```

### 9.6 删除文档

```http
DELETE /api/v1/admin/knowledge/documents/{doc_id}
```

删除动作必须同步：

- MySQL document。
- MySQL chunks。
- Milvus chunk vectors。

### 9.7 召回调试

```http
POST /api/v1/admin/knowledge/search:debug
```

### 9.8 审核文档

```http
POST /api/v1/admin/knowledge/documents/{doc_id}:review
```

```json
{
  "decision": "approve|reject|needs_edit",
  "reason": "来源已授权，内容质量通过"
}
```

### 9.9 重建向量

```http
POST /api/v1/admin/knowledge/documents/{doc_id}:reembed
```

### 9.10 禁用/启用文档

```http
POST /api/v1/admin/knowledge/documents/{doc_id}:disable
POST /api/v1/admin/knowledge/documents/{doc_id}:enable
```

## 10. 后台任务与调度

### 10.1 Worker 模型

生产环境使用后台 worker 处理 pipeline：

```text
api server
  -> create job
  -> DB queue / Redis stream / task table
  -> ingestion worker
  -> embedding worker
  -> vector sync worker
  -> compensation worker
```

要求：

- Worker 支持水平扩展。
- 每个 job step 有 lease，worker 崩溃后可被其他 worker 接管。
- 同一 doc 的 reparse/reembed/delete 不能并发执行。
- embedding worker 单独限流，避免耗尽模型额度。
- compensation worker 定期修复 MySQL/Milvus 状态不一致。

### 10.2 幂等键

幂等键建议：

| 对象 | 幂等键 |
| --- | --- |
| URL job | `source_id + normalized_url` |
| 文件 job | `source_id + file_sha256` |
| document | `source_id + content_hash` |
| chunk | `doc_id + chunk_index + chunker_version` |
| vector | `chunk_id + embedding_model + embedding_version` |

### 10.3 重试策略

| 阶段 | 可重试 | 策略 |
| --- | --- | --- |
| compliance | 否 | 配置错误直接失败 |
| fetch | 是 | 3 次指数退避 |
| parse | 部分 | parser 异常可重试，质量不足转审核 |
| clean | 是 | 代码异常可重试 |
| metadata_extract | 是 | 小模型失败走规则兜底 |
| chunk | 是 | 稳定算法可重试 |
| embedding | 是 | 限流/超时重试，余额不足熔断 |
| milvus_upsert | 是 | 重试 + 补偿任务 |
| delete | 是 | 直到 MySQL tombstone 和 Milvus 删除一致 |

## 11. 可观测与告警

### 11.1 指标

必须打点：

- `ingestion_jobs_created_total`
- `ingestion_jobs_success_total`
- `ingestion_jobs_failed_total`
- `ingestion_job_duration_ms`
- `fetch_failure_rate`
- `parse_failure_rate`
- `low_quality_doc_rate`
- `embedding_request_total`
- `embedding_token_or_char_total`
- `embedding_failure_rate`
- `milvus_upsert_failure_total`
- `milvus_delete_failure_total`
- `external_rag_recall_count`
- `external_rag_filtered_count`
- `external_rag_answer_citation_count`

### 11.2 日志

结构化日志字段：

- `trace_id`
- `job_id`
- `doc_id`
- `chunk_id`
- `source_id`
- `step`
- `status`
- `duration_ms`
- `error_code`

禁止日志：

- 不打印完整原文。
- 不打印授权 token。
- 不打印管理员上传文件内容。

### 11.3 告警

告警条件：

- 采集失败率 10 分钟内超过 20%。
- embedding 连续失败超过阈值。
- Milvus upsert/delete 失败。
- `deleting` 状态文档超过 30 分钟未完成。
- `needs_review` 堆积超过阈值。
- 外部内容召回量异常上升，可能污染主回答。

## 12. MVP 实施步骤

### M1：手动导入和 URL 抓取

目标：先以生产标准跑通外部非结构化知识入库，而不是只做 demo。

范围：

1. 新增数据表。
2. 支持 Markdown/HTML 文本导入。
3. 支持管理员提交公开 URL。
4. HTML main content 抽取。
5. 清洗、chunk、embedding、Milvus upsert。
6. 后台任务列表和文档列表。
7. `search_knowledge` 支持 `external_post`。
8. 幂等、重试、删除一致性、审计日志。
9. 基础指标和结构化日志。
10. 管理后台可查看 pipeline step 和失败原因。

不做：

- 不做登录态爬取。
- 不做小红书/知乎全站搜索。
- 不做绕反爬。

M1 验收标准：

- 重复提交同一 URL 不产生重复 doc/chunk/vector。
- fetch/parse/embedding 任一步失败后可重试。
- 删除 doc 后线上检索不再召回对应 chunk。
- 未审核 doc 不进入线上 `search_knowledge`。
- 管理员能看到 job step、耗时、错误码。
- `go test ./...` 和质量测评脚本通过。

### M2：质量审核和召回调试

1. 文档质量评分。
2. 风控过滤。
3. 重复检测。
4. 管理员查看 chunk 和召回结果。
5. 支持禁用、删除、重建 embedding。
6. 外部内容 RAG 召回测评集。
7. 召回可视化显示分数构成和过滤原因。

### M3：合作/授权数据源

1. 接商家/达人授权内容包。
2. 定时同步。
3. 增量更新。
4. 来源质量权重。
5. 授权撤回后的批量删除。

### M4：可控网页采集

1. 白名单站点采集。
2. robots 检查。
3. 站点级解析模板。
4. 失败自动降级为人工审核。
5. domain 级限速、robots 缓存和采集审计。

## 13. 测试与质量评估

### 13.1 单元测试

- URL normalize。
- SSRF IP 校验。
- robots allow/deny。
- content hash。
- chunk 稳定性。
- metadata schema 校验。
- dedup。
- delete state machine。

### 13.2 集成测试

- URL -> doc -> chunk -> vector 全链路。
- Markdown 文件导入。
- HTML 文件导入。
- embedding 失败重试。
- Milvus upsert 失败补偿。
- 删除后 Milvus 不再召回。
- 未审核文档不被线上召回。

### 13.3 RAG 测评

新增数据集：

```text
quality/data/eval/external_knowledge_cases.jsonl
```

case 类型：

- 外部经验可补充回答。
- 外部内容与平台知识冲突。
- 低质量帖子应过滤。
- 广告软文应降权。
- 过期内容应降权。
- 高风险品类外部内容不能作为确定事实。

指标：

- `external_context_hit_rate@k`
- `source_precision`
- `low_quality_filter_rate`
- `conflict_suppression_rate`
- `citation_correctness`
- `answer_grounding_score`
- `external_pollution_rate`

### 13.4 性能目标

- 创建采集任务 API p95 < 300ms。
- URL fetch 单任务超时 10s。
- HTML parse p95 < 2s。
- 单 doc chunk p95 < 1s。
- embedding 后台异步，不影响用户请求。
- `search_knowledge` 增加外部 corpus 后 p95 增幅 < 20%。

## 14. 发布策略

### 14.1 灰度

配置开关：

```json
{
  "external_knowledge": {
    "enabled": false,
    "online_recall_enabled": false,
    "admin_debug_enabled": true,
    "require_review": true
  }
}
```

上线顺序：

1. 只开放导入和后台查看。
2. 开放后台召回调试。
3. 小流量开启 `search_knowledge` 外部 corpus。
4. 按类目逐步放开。
5. 监控污染率、引用率、失败率。

### 14.2 回滚

回滚必须能做到：

- 关闭外部知识召回，不影响原 RAG。
- 禁用某个 source。
- 禁用某批 job 导入的所有 doc。
- 回滚 embedding model 配置。
- 删除一批向量。

## 15. 关键风险

1. 合规风险：未经授权抓取平台内容，后续无法上线。
2. 内容污染：外部帖子营销、虚假测评、软广会污染 RAG。
3. 时效风险：帖子旧结论可能不适用当前商品。
4. 版权风险：不能把原文大段输出给用户。
5. 召回噪声：帖子内容泛，容易压过平台自建知识。
6. 删除风险：如果没有完整删除链路，后续无法处理投诉或授权撤回。
7. 成本风险：重复 embedding 或异常重试会快速消耗模型额度。
8. 运维风险：Milvus/MySQL 状态不一致会导致“后台显示删除但仍可召回”。

## 16. 推荐结论

先做“管理员可控输入”的 RAG ingestion，不做自动抓小红书/知乎。

第一阶段目标是以生产标准把非结构化知识管线建设起来：

```text
导入 -> 解析 -> 清洗 -> chunk -> embedding -> Milvus -> search_knowledge -> 管理后台可视化
```

但 M1 必须同时具备：

- 幂等。
- 重试。
- 审计。
- 删除一致性。
- 审核门禁。
- 风控过滤。
- 召回可解释。
- 指标告警。

等机制稳定后，再根据授权情况接具体平台数据源。
