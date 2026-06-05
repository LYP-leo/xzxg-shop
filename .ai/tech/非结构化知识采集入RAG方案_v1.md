# 非结构化知识采集入 RAG 方案 v1

## 1. 背景

当前商品 RAG 主要围绕平台自建知识、商品结构化信息、商品详情、评价、商家资料构建。导购回答如果只依赖商品库，容易缺少真实用户经验、横评、避坑、使用场景等非结构化知识。

用户希望补充小红书、知乎等帖子类内容，建立自动采集、解析、入向量库机制。

本文只做方案设计，不直接实现代码。

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

### 4.5 Cleaner

清洗规则：

- 去导航、广告、推荐阅读、评论区噪声。
- 去重复空白、表情泛滥、无意义符号。
- 识别“广告/带货/推广/利益相关”信号。
- 保留商品名、品牌名、品类词、规格、适用人群、风险点。
- 保留原始正文 hash，便于去重。

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

### 4.7 Dedup

去重维度：

- `source_url` 精确去重。
- `content_hash` 精确去重。
- `simhash` 或 embedding 相似度近似去重。

重复策略：

- 同 URL 已存在：更新 metadata，不重复入向量。
- 同内容不同 URL：保留多个 source，但 chunk 只入一份或做 source 聚合。
- 低质量重复内容：标记 disabled。

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

## 9. API 设计

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

## 10. MVP 实施步骤

### M1：手动导入和 URL 抓取

目标：先跑通外部非结构化知识入库。

范围：

1. 新增数据表。
2. 支持 Markdown/HTML 文本导入。
3. 支持管理员提交公开 URL。
4. HTML main content 抽取。
5. 清洗、chunk、embedding、Milvus upsert。
6. 后台任务列表和文档列表。
7. `search_knowledge` 支持 `external_post`。

不做：

- 不做登录态爬取。
- 不做小红书/知乎全站搜索。
- 不做绕反爬。

### M2：质量审核和召回调试

1. 文档质量评分。
2. 风控过滤。
3. 重复检测。
4. 管理员查看 chunk 和召回结果。
5. 支持禁用、删除、重建 embedding。

### M3：合作/授权数据源

1. 接商家/达人授权内容包。
2. 定时同步。
3. 增量更新。
4. 来源质量权重。

### M4：可控网页采集

1. 白名单站点采集。
2. robots 检查。
3. 站点级解析模板。
4. 失败自动降级为人工审核。

## 11. 关键风险

1. 合规风险：未经授权抓取平台内容，后续无法上线。
2. 内容污染：外部帖子营销、虚假测评、软广会污染 RAG。
3. 时效风险：帖子旧结论可能不适用当前商品。
4. 版权风险：不能把原文大段输出给用户。
5. 召回噪声：帖子内容泛，容易压过平台自建知识。
6. 删除风险：如果没有完整删除链路，后续无法处理投诉或授权撤回。

## 12. 推荐结论

先做“管理员可控输入”的 RAG ingestion，不做自动抓小红书/知乎。

第一阶段目标是把非结构化知识管线建设起来：

```text
导入 -> 解析 -> 清洗 -> chunk -> embedding -> Milvus -> search_knowledge -> 管理后台可视化
```

等机制稳定后，再根据授权情况接具体平台数据源。
