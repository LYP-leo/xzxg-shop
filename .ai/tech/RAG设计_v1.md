# 电商 AI 导购 RAG 设计 v1

## 1. 设计目标

本文是 `xzxg-shop` 的 RAG 专项设计，承接 [需求_v1.md](../prd/需求_v1.md)、[技术方案_v2.md](./技术方案_v2.md) 和 [Agent框架设计_v2.md](./Agent框架设计_v2.md)。

RAG 部分要解决的问题：

- 商家上传商品详情、活动规则、售后政策、FAQ 等非结构化资料后，系统能构建可检索知识库。
- 用户咨询时，Agent 能先检索可信证据，再生成导购回答。
- 商品参数、价格、库存、活动规则、售后政策等关键事实必须来自商品服务或知识库证据，不能让模型自由编造。
- 回答能附带引用来源，方便用户和评测系统追溯。
- 检索链路可观测、可评测、可迭代。

本设计只覆盖 RAG，不直接实现代码。

## 2. 核心概念

### 2.1 RAG 是什么

RAG 全称是 Retrieval-Augmented Generation，中文可理解为“检索增强生成”。

普通 LLM 回答问题时，主要依赖模型训练时学到的通用知识。电商导购场景不能只依赖这种方式，因为商品价格、库存、活动规则、售后政策会频繁变化，而且不同商家的资料不同。

RAG 的做法是：

```text
用户问题
  -> 检索项目自己的商品资料和知识库
  -> 找到相关证据
  -> 把证据连同问题一起交给大模型
  -> 大模型基于证据生成回答
```

一句话：RAG 不是让模型凭记忆回答，而是先把“参考资料”找出来，再让模型照着资料回答。

### 2.2 本项目中的知识来源

知识来源分为两类：

| 来源 | 存储 | 用途 |
| --- | --- | --- |
| 结构化商品数据 | MySQL `products`、`product_skus` | 价格、库存、商品卡片、SKU、类目、品牌 |
| 非结构化知识文档 | MySQL `knowledge_documents`、`knowledge_chunks` + Qdrant | 商品详情补充、活动规则、售后政策、FAQ、导购话术 |

原则：

- 商品卡片必须来自商品服务。
- 精确价格和库存优先来自商品表。
- 活动规则、售后政策、FAQ 必须有知识库证据。
- 如果知识库没有命中明确证据，回答必须说明“不确定”或“当前资料没有明确说明”。

## 3. 总体架构

RAG 分为离线构建和在线检索两条链路。

### 3.1 离线构建链路

```text
商家上传文档
  -> 保存原始文档记录
  -> 异步解析文档
  -> 清洗正文
  -> 语义切片
  -> 提取元数据
  -> 写入 knowledge_chunks
  -> 调用 Embedding 模型生成向量
  -> 写入 Qdrant
  -> 更新文档状态 indexed
```

离线构建关注“把资料处理成适合检索的知识块”。

### 3.2 在线检索链路

```text
用户问题 + 会话上下文
  -> Query 改写
  -> 意图识别
  -> 构造检索请求
  -> 关键词召回
  -> 向量召回
  -> 元数据过滤
  -> 合并去重
  -> Rerank 重排
  -> 证据压缩
  -> 交给 Agent 生成回答
```

在线检索关注“从大量知识块里找出当前问题最相关、最可信的证据”。

## 4. 离线构建设计

### 4.1 文档状态

`knowledge_documents.status` 建议使用以下状态：

| 状态 | 说明 |
| --- | --- |
| `uploaded` | 已上传，等待处理 |
| `parsing` | 正在解析 |
| `parsed` | 解析完成 |
| `chunking` | 正在切片 |
| `embedding` | 正在生成向量 |
| `indexed` | 已入库，可检索 |
| `failed` | 处理失败 |

当前代码已有 `knowledge_documents` 和 `knowledge_chunks`，后续实现时可以在现有表上增量扩展。

### 4.2 文档解析

不同格式使用不同解析策略：

| 文件类型 | 解析策略 |
| --- | --- |
| Markdown | 保留标题层级，按标题和段落解析 |
| TXT | 按空行、标点、长度解析 |
| PDF | 提取文本、页码、表格；扫描件走 OCR |
| DOCX | 提取标题、段落、表格 |
| HTML | 抽取正文，删除导航、脚本、样式 |
| 图片 | OCR + Vision 描述 |

MVP 阶段可以先支持纯文本输入和 Markdown；PDF、Word、图片解析作为后续能力。

### 4.3 文档清洗

清洗规则：

- 删除页眉、页脚、导航、版权重复文本。
- 合并多余空格和连续空行。
- 保留标题层级。
- 保留型号、价格、规格、时间、适用条件、限制条件。
- 表格转为“表头 + 行内容”的文本，避免切片后丢失表头。
- 删除过短、无业务含义的片段。

示例：

```text
原始表格：
型号 | 电池 | 重量
X Phone 12 | 5000mAh | 189g

清洗后：
商品参数：型号=X Phone 12，电池=5000mAh，重量=189g。
```

### 4.4 语义切片

切片不是简单按固定字数切。电商知识要尽量按业务含义切，否则一个 chunk 里可能混入多个商品或多个活动规则，检索会变差。

#### 通用切片规则

递归分隔优先级：

```text
一级标题
  -> 二级标题
  -> 三级标题
  -> 空行
  -> 句号/问号/感叹号/分号
  -> 逗号/顿号
  -> 固定长度兜底
```

建议参数：

| 参数 | 建议值 |
| --- | --- |
| `max_chars` | 800 中文字符 |
| `min_chars` | 120 中文字符 |
| `overlap_chars` | 80 到 120 中文字符 |

`overlap` 的作用是保留上下文。例如上一片结尾写了“优惠券不可叠加”，下一片开头写了“会员券除外”，如果完全断开，模型可能理解错规则。

#### 按文档类型切片

| 文档类型 | 切片方式 |
| --- | --- |
| 商品详情 | 按商品、参数、卖点、适用人群、风险提示切 |
| 营销规则 | 按活动名称、适用条件、叠加规则、生效时间切 |
| 售后政策 | 按服务类型、适用条件、例外情况切 |
| FAQ | 一问一答一个 chunk |
| 导购话术 | 按使用场景和目标人群切 |

### 4.5 元数据提取

每个 chunk 必须带元数据。元数据用于过滤、排序、引用和评测。

建议字段：

```json
{
  "chunk_id": "ck_001",
  "document_id": "doc_001",
  "merchant_id": "m_001",
  "doc_type": "promotion",
  "title": "618 会员券叠加规则",
  "title_path": ["618 活动", "会员券"],
  "product_id": "p_001",
  "category_id": "c_phone",
  "brand": "X",
  "source_page": 2,
  "effective_start": "2026-05-01T00:00:00+08:00",
  "effective_end": "2026-06-20T23:59:59+08:00",
  "content_hash": "sha256"
}
```

MVP 必填字段：

- `chunk_id`
- `document_id`
- `merchant_id`
- `doc_type`
- `title`
- `content`
- `content_hash`

推荐尽早补齐字段：

- `product_id`
- `category_id`
- `brand`
- `effective_start`
- `effective_end`

### 4.6 向量入库

向量库使用 Qdrant。仓库的 `deployments/docker-compose.yml` 已包含 Qdrant 服务。

Collection 建议：

| Collection | 用途 |
| --- | --- |
| `knowledge_chunks` | 文本知识 chunk |
| `product_images` | 后续图片相似商品召回 |

`knowledge_chunks` payload 建议：

```json
{
  "chunk_id": "ck_001",
  "document_id": "doc_001",
  "merchant_id": "m_001",
  "doc_type": "promotion",
  "title": "618 会员券叠加规则",
  "product_id": "p_001",
  "category_id": "c_phone",
  "brand": "X",
  "effective_start": "2026-05-01T00:00:00+08:00",
  "effective_end": "2026-06-20T23:59:59+08:00"
}
```

向量 ID 可以使用 `chunk_id`，方便 MySQL 和 Qdrant 对齐。

### 4.7 幂等与增量更新

文档处理必须支持重复执行。

规则：

- 对 chunk 内容计算 `content_hash`。
- 如果同一文档重新上传，先将旧 chunk 标记为不可用，再写入新 chunk。
- 如果 chunk 内容没变，可以复用已有 embedding。
- Qdrant 写入使用 upsert。
- 文档处理失败后可重试，不产生重复 chunk。

## 5. 在线检索设计

### 5.1 检索输入

Agent 调用 RAG 时不要只传用户原话，应传结构化输入。

```json
{
  "trace_id": "trace_001",
  "session_id": "sess_001",
  "message_id": "msg_001",
  "original_query": "这个手机适合拍娃吗？",
  "rewritten_query": "X Phone 12 是否适合拍摄儿童？请结合对焦、抓拍、存储、续航和风险提示回答。",
  "intent": "product_detail_qa",
  "merchant_id": "m_001",
  "product_ids": ["p_001"],
  "category_ids": ["c_phone"],
  "doc_types": ["product_detail", "faq"],
  "top_k": 8
}
```

### 5.2 Query 改写

Query 改写解决两个问题：

- 用户常说“这个”“第一款”“刚才那个”，需要根据会话上下文补全。
- 用户问题可能太口语化，需要改写成更适合检索的句子。

示例：

```text
用户原话：那第一款适合老人吗？
改写结果：X Phone 12 是否适合老人使用？请结合屏幕、重量、续航、操作难度、售后和价格回答。
```

改写后的 query 用于：

- 向量检索。
- 关键词检索。
- Rerank。
- 最终 Prompt 中的问题描述。

### 5.3 混合召回

只用向量检索不够，因为电商场景有很多精确词：

- 型号：`X Phone 12`
- 参数：`256GB`
- 活动名：`618`
- 金额：`满 300 减 50`
- SKU：`标准版`

因此采用混合召回：

```text
向量召回 top 30
  + 关键词召回 top 30
  + 元数据过滤
  -> 合并去重
  -> Rerank top 8
```

#### 向量召回

适合处理“意思相近但字面不同”的问题。

例子：

```text
用户问：适合拍小孩吗？
知识库写：支持儿童抓拍模式和高速对焦。
```

这两个句子字面不同，但语义接近，向量检索应该能召回。

#### 关键词召回

适合处理精确匹配。

例子：

```text
用户问：X Phone 12 有没有 256GB？
```

这里 `X Phone 12`、`256GB` 不能只靠向量相似度，必须做关键词匹配。

MVP 可以先用 MySQL `LIKE` 做关键词召回；后续可以接入 Elasticsearch、Meilisearch 或 MySQL FULLTEXT。

### 5.4 元数据过滤

过滤优先级：

1. `merchant_id`：只检索当前商家或平台允许的商家资料。
2. `product_id`：问题已明确商品时，优先过滤到该商品。
3. `category_id`：问题明确类目时，过滤到相关类目。
4. `doc_type`：不同意图优先不同文档类型。
5. `effective_start/effective_end`：活动规则必须在有效期内。

意图和文档类型映射：

| 意图 | 优先 doc_type |
| --- | --- |
| `product_recommendation` | `product_detail`、`faq`、`guide` |
| `product_detail_qa` | `product_detail`、`faq` |
| `promotion_qa` | `promotion` |
| `after_sale_qa` | `after_sale`、`faq` |
| `comparison` | `product_detail`、`faq` |

### 5.5 合并去重

向量召回和关键词召回可能命中同一个 chunk。

合并规则：

- 按 `chunk_id` 去重。
- 保留来源列表，例如 `["vector", "keyword"]`。
- 初始分数可以由向量分、关键词分、元数据加权组成。

建议初始分数：

```text
score = vector_score * 0.45
      + keyword_score * 0.30
      + metadata_score * 0.20
      + freshness_score * 0.05
```

MVP 可以先使用简单排序；上线评测后再调权重。

### 5.6 Rerank 重排

Rerank 的目标是从召回的 30 到 60 个候选 chunk 中选出最相关的 5 到 8 个。

可选方案：

| 方案 | 优点 | 缺点 |
| --- | --- | --- |
| 规则重排 | 简单、便宜、可控 | 语义判断弱 |
| Rerank 模型 | 相关性更好 | 多一个模型依赖 |
| LLM 轻量评分 | 解释性好 | 成本和延迟较高 |

建议落地顺序：

1. MVP：规则重排。
2. v2：接入 rerank 模型。
3. 调试阶段：允许用 LLM 对疑难样例评分，但不作为默认链路。

Rerank 特征：

- 是否直接回答了问题。
- 是否匹配商品 ID。
- 是否匹配文档类型。
- 是否包含关键事实词。
- 是否在活动有效期内。
- 是否来自更新版本文档。

### 5.7 证据压缩

不能把整篇文档都塞进 Prompt。证据压缩只保留和问题直接相关的句子。

输出格式：

```json
{
  "items": [
    {
      "chunk_id": "ck_phone_001",
      "title": "X Phone 12 商品详情",
      "source": "mysql_seed",
      "snippet": "X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。",
      "score": 0.91,
      "reason": "命中拍娃、对焦、价格"
    }
  ],
  "has_sufficient_evidence": true
}
```

如果证据不足：

```json
{
  "items": [],
  "has_sufficient_evidence": false,
  "missing_evidence": ["活动叠加规则", "生效时间"]
}
```

## 6. Agent 集成方式

RAG 在 Agent 中作为工具存在。

工具名：

```text
search_knowledge
```

输入：

```json
{
  "query": "X Phone 12 是否适合拍娃？",
  "intent": "product_detail_qa",
  "product_ids": ["p_001"],
  "category_ids": ["c_phone"],
  "doc_types": ["product_detail", "faq"],
  "top_k": 8
}
```

输出：

```json
{
  "evidence": [
    {
      "chunk_id": "ck_phone_001",
      "title": "X Phone 12 商品详情",
      "snippet": "X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。",
      "source": "mysql_seed",
      "score": 0.91
    }
  ],
  "has_sufficient_evidence": true
}
```

Agent 回答约束：

- 回答商品事实时，必须优先使用商品服务和 RAG 证据。
- 有证据时，回答中附带引用。
- 没证据时，不输出确定性结论。
- 证据冲突时，优先使用更新时间更新、有效期内、结构化商品表中的信息，并提示存在资料差异。

## 7. 数据库设计建议

当前迁移已有：

- `knowledge_documents`
- `knowledge_chunks`

后续建议扩展或新增以下字段。

### 7.1 knowledge_documents

建议字段：

| 字段 | 说明 |
| --- | --- |
| `document_id` | 文档 ID |
| `merchant_id` | 商家 ID |
| `title` | 文档标题 |
| `doc_type` | 文档类型 |
| `content` | 原始或清洗后正文 |
| `status` | 处理状态 |
| `chunk_count` | chunk 数 |
| `error_message` | 失败原因 |
| `content_hash` | 文档内容 hash |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

### 7.2 knowledge_chunks

建议字段：

| 字段 | 说明 |
| --- | --- |
| `chunk_id` | chunk ID |
| `document_id` | 文档 ID |
| `merchant_id` | 商家 ID |
| `doc_type` | 文档类型 |
| `title` | chunk 标题 |
| `content` | chunk 正文 |
| `snippet` | 展示摘要 |
| `source` | 来源 |
| `product_id` | 关联商品 |
| `category_id` | 关联类目 |
| `brand` | 品牌 |
| `content_hash` | chunk 内容 hash |
| `embedding_status` | 向量状态 |
| `vector_id` | Qdrant 向量 ID |
| `effective_start` | 生效开始时间 |
| `effective_end` | 生效结束时间 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

## 8. API 设计建议

### 8.1 商家上传知识文档

当前已有：

```http
POST /api/v1/merchant/documents
```

后续可以扩展支持文件上传：

```http
POST /api/v1/merchant/documents:upload
```

### 8.2 查询文档处理状态

```http
GET /api/v1/merchant/documents
```

返回：

```json
{
  "items": [
    {
      "document_id": "doc_001",
      "title": "X Phone 12 商品详情",
      "doc_type": "product_detail",
      "status": "indexed",
      "chunk_count": 12,
      "created_at": "2026-05-20T12:00:00+08:00"
    }
  ]
}
```

### 8.3 知识库检索调试

用于管理员或研发调试。

```http
POST /api/v1/admin/knowledge/search
```

请求：

```json
{
  "query": "X Phone 12 适合拍娃吗",
  "product_ids": ["p_001"],
  "doc_types": ["product_detail"],
  "top_k": 8,
  "debug": true
}
```

返回：

```json
{
  "rewritten_query": "X Phone 12 是否适合拍摄儿童？",
  "vector_hits": [],
  "keyword_hits": [],
  "rerank_hits": [
    {
      "chunk_id": "ck_phone_001",
      "title": "X Phone 12 商品详情",
      "snippet": "X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。",
      "score": 0.91,
      "reasons": ["product_match", "semantic_match"]
    }
  ]
}
```

## 9. 评测指标

RAG 必须可评测，否则无法迭代。

### 9.1 检索指标

| 指标 | 说明 |
| --- | --- |
| Top-K 命中率 | 标准证据是否出现在前 K 个结果中 |
| MRR | 标准证据排名越靠前分数越高 |
| 关键词命中率 | 型号、参数、活动名等精确词是否召回 |
| 过滤准确率 | 商品、类目、文档类型、有效期过滤是否正确 |
| 无证据识别率 | 知识库没有答案时，系统是否正确判定无证据 |

### 9.2 回答指标

| 指标 | 说明 |
| --- | --- |
| 引用覆盖率 | 关键结论是否有 citation |
| 事实准确率 | 参数、价格、规则是否和证据一致 |
| 幻觉率 | 是否编造知识库没有的事实 |
| 证据一致性 | 回答是否正确使用被召回的 chunk |

### 9.3 Trace 记录

每次 RAG 检索记录：

```json
{
  "trace_id": "trace_001",
  "message_id": "msg_001",
  "original_query": "这个适合拍娃吗？",
  "rewritten_query": "X Phone 12 是否适合拍摄儿童？",
  "filters": {
    "product_ids": ["p_001"],
    "doc_types": ["product_detail"]
  },
  "vector_hit_ids": ["ck_001"],
  "keyword_hit_ids": ["ck_002"],
  "rerank_hit_ids": ["ck_001", "ck_002"],
  "selected_evidence_ids": ["ck_001"],
  "latency_ms": 128
}
```

## 10. 分期落地计划

### Phase 1：可演示 RAG

目标：在当前代码基础上最小改动跑通“上传资料 -> 切片 -> 检索 -> Agent 引用”。

- 使用 MySQL 保存文档和 chunk。
- 文档输入先支持纯文本。
- 切片先实现标题、段落、长度兜底。
- 关键词检索先用 MySQL `LIKE`。
- Agent 回答展示 citation。
- 暂不接 Qdrant 和真实 embedding。

### Phase 2：接入向量检索

目标：解决语义相似问题。

- 接入 Embedding 模型。
- 接入 Qdrant `knowledge_chunks` collection。
- 写入 chunk 向量。
- 在线检索增加向量召回。
- 和关键词召回合并去重。

### Phase 3：提升准确率

目标：提高复杂导购问题的命中率和可信度。

- 加入 Query 改写。
- 加入元数据过滤。
- 加入规则 Rerank。
- 增加知识库检索调试页。
- 增加检索 Trace。

### Phase 4：评测闭环

目标：能用数据证明 RAG 效果。

- 建立评测集。
- 统计 Top-K 命中率、引用覆盖率、幻觉率。
- 支持失败样例归因。
- 用评测结果反向优化切片、检索和 Prompt。

### Phase 5：多模态和高级检索

目标：覆盖图片输入和复杂商品识别。

- 图片 OCR。
- Vision 描述。
- 商品图片 embedding。
- 图片相似商品召回。
- 活动规则、表格、FAQ 专用切片优化。

## 11. 风险与处理

| 风险 | 表现 | 处理 |
| --- | --- | --- |
| 切片太碎 | 检索到的 chunk 没上下文 | 增加 overlap，保留标题路径 |
| 切片太大 | Prompt 塞太多无关内容 | 控制 max_chars，做证据压缩 |
| 只用向量检索 | 型号、金额、SKU 命中不稳 | 混合关键词检索 |
| 知识过期 | 活动规则失效仍被引用 | 增加有效期过滤 |
| 模型编造 | 回答出现证据没有的信息 | Prompt 强约束 + citation 校验 |
| 多商家串数据 | A 商家的资料回答 B 商家问题 | merchant_id 强过滤 |
| 评测缺失 | 不知道检索是否变好 | 建立标准问题和标准证据 |

## 12. MVP 编码边界建议

下一步如果进入编码，建议不要一次性实现完整 RAG。优先做以下最小闭环：

1. 扩展 `knowledge_chunks` 字段或在现有字段中保存足够的元数据。
2. 实现纯文本资料切片。
3. 上传资料时自动生成 chunk。
4. `SearchKnowledge` 从简单关键词匹配升级为带 doc_type、merchant、product 过滤的检索。
5. Agent 输出 citation，并在无证据时给出不确定提示。
6. 增加一个最小检索调试接口，方便后续评测。

这个边界可以最快验证 RAG 对导购回答的价值，同时保留后续接 Qdrant、Embedding、Rerank 的扩展空间。
