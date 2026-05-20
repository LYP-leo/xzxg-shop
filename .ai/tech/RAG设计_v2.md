# 电商 AI 导购 RAG 设计 v2

## 1. v2 调整目标

本文基于 [RAG设计_v1.md](./RAG设计_v1.md) 和 [RAG_v1_问题.md](./RAG_v1_问题.md) 修订，目标是把 RAG 模块边界重新定义清楚，并让设计和当前仓库已有代码、已有技术方案耦合。

v2 重点调整：

- 知识来源不只包含商家商品资料，还包含平台自建知识库，例如小红书、B 站、测评文章、种草笔记、选购攻略等外部内容。
- RAG 模块接收的不是用户原始问题，而是 Agent 上游已经完成 Query 改写、意图识别、商品解析后的结构化检索请求。
- 图片不以 OCR 为主。商品图片和用户上传图片应通过图像语义理解模型或多模态 embedding 模型向量化，然后进入向量数据库做相似检索。
- RAG 模块需要和现有代码中的 `knowledge_documents`、`knowledge_chunks`、`SearchKnowledge`、`Citation`、`AgentBlock` 逐步演进，而不是另起一套无法落地的抽象。
- 设计必须支持先落地 MVP，再逐步接入 Qdrant、Embedding、图片向量和外部知识采集。

本文只做设计，不直接实现代码。

## 2. 当前代码基线

当前 `main` 已有一个最小 RAG 雏形：

- 后端入口：[backend/cmd/api/main.go](../../backend/cmd/api/main.go)
- HTTP 路由：[backend/src/httpapi/server.go](../../backend/src/httpapi/server.go)
- Store 接口：[backend/src/store/interface.go](../../backend/src/store/interface.go)
- MySQL 实现：[backend/src/store/mysql.go](../../backend/src/store/mysql.go)
- 领域类型：[backend/src/domain/types.go](../../backend/src/domain/types.go)
- Agent Runtime：[backend/src/agent/runtime.go](../../backend/src/agent/runtime.go)
- 数据库迁移：[backend/migrations/001_mysql_schema.sql](../../backend/migrations/001_mysql_schema.sql)

已有能力：

```text
商家上传资料
  -> POST /api/v1/merchant/documents
  -> 写入 knowledge_documents
  -> 生成 1 条 knowledge_chunks

用户咨询
  -> Agent Runtime
  -> store.SearchProducts
  -> store.SearchKnowledge
  -> SSE 返回 product_card 和 citation
```

当前限制：

- `SearchKnowledge(ctx, query string)` 只接受字符串，无法表达结构化过滤条件。
- `knowledge_chunks` 表只有 `chunk_id/title/snippet/source/sort_order`，缺少 `doc_type/product_id/category_id/source_type/vector_id` 等字段。
- RAG 检索只用 MySQL `LIKE`，没有 Qdrant 向量召回。
- 文档上传只是纯文本资料入库，没有异步解析、清洗、切片、Embedding。
- 图片只存在于商品字段 `image_url/image_urls_json`，没有图片向量库。
- Agent Runtime 仍是规则化演示版，Query 改写、意图识别、工具规划尚未实现。

v2 设计必须以这些现状为起点分阶段演进。

## 3. 模块边界

### 3.1 RAG 模块负责什么

RAG 模块负责：

- 接收结构化知识素材，构建可检索知识库。
- 对文本知识进行清洗、切片、元数据抽取、Embedding、向量入库。
- 对图片素材生成图像语义向量，写入图片向量库。
- 接收 Agent 上游传入的结构化检索请求。
- 执行文本向量检索、关键词检索、图片向量检索、元数据过滤、合并去重、重排。
- 返回可引用的证据 `Citation` 或相似商品候选。
- 记录检索 Trace，服务评测闭环。

### 3.2 RAG 模块不负责什么

RAG 模块不负责：

- 不负责 Query 改写。
- 不负责意图识别。
- 不负责多轮上下文解析。
- 不负责“这个”“第一款”等指代消解。
- 不负责最终自然语言回答生成。
- 不负责商品价格、库存、SKU 的权威查询。

这些能力归属：

| 能力 | 所属模块 |
| --- | --- |
| Query 改写 | Agent `Query Rewriter` |
| 意图识别 | Agent `Intent Router` |
| 指代消解 | Agent `Context Builder` / `Query Rewriter` |
| 商品价格库存 | 商品服务 / MySQL 商品表 |
| 最终回答 | Agent 主回答 LLM |
| 证据检索 | RAG 模块 |

RAG 的输入应该已经是“可检索”的结构化请求，而不是用户原始自然语言。

## 4. 知识来源设计

v1 主要围绕商家上传资料。v2 将知识来源扩展为三类。

### 4.1 商家知识库

来源：

- 商品详情。
- 商品参数。
- 活动规则。
- 售后政策。
- FAQ。
- 导购话术。

特点：

- 权威性最高。
- 和商家、商品、类目强绑定。
- 可直接用于回答商品事实、活动规则和售后政策。

当前代码已支持最小商家资料上传：

```http
POST /api/v1/merchant/documents
```

后续应在这个接口基础上扩展文件上传、异步处理和多 chunk 切片。

### 4.2 平台自建知识库

来源：

- 平台运营沉淀的选购攻略。
- 商品横评。
- 类目选购指南。
- 标准 FAQ。
- 比赛演示中人工整理的知识材料。

特点：

- 权威性次于商家资料，但高于开放社区内容。
- 常用于导购建议、选购方法、对比维度解释。
- 可以跨商家、跨商品使用。

示例：

```text
手机拍娃选购指南：
优先关注对焦速度、快门响应、动态范围、存储空间、夜景抓拍能力。
```

这类知识不一定绑定单个商品，但可以绑定 `category_id = c_phone` 和 `doc_type = guide`。

### 4.3 外部内容知识库

来源：

- 小红书种草笔记。
- B 站测评视频文案或字幕。
- 数码媒体测评文章。
- 用户口碑摘要。
- 公共选购攻略。

特点：

- 适合补充使用体验、口碑、真实场景优缺点。
- 权威性低于商家资料，不能覆盖价格、库存、活动规则等强事实。
- 必须保存来源、抓取时间、作者或链接、平台名。
- 需要做清洗、去重、可信度分级和过期处理。

使用原则：

- 可以用于“用户体验”“口碑倾向”“常见吐槽”“适合场景”等软性建议。
- 不可作为价格、库存、活动规则、售后承诺的唯一依据。
- 如果外部内容与商家资料冲突，商家资料和结构化商品数据优先。

来源类型建议：

| source_type | 说明 | 可信度 |
| --- | --- | --- |
| `merchant` | 商家上传资料 | 高 |
| `platform` | 平台自建资料 | 中高 |
| `external_xhs` | 小红书内容 | 中低 |
| `external_bilibili` | B 站内容 | 中低 |
| `external_article` | 第三方文章 | 中 |
| `user_feedback` | 用户反馈沉淀 | 中 |

## 5. 离线构建链路

RAG 构建链路分为文本知识构建和图片向量构建。

### 5.1 文本知识构建

```text
知识来源接入
  -> 保存原始记录 knowledge_documents
  -> 解析结构化/非结构化内容
  -> 清洗正文
  -> 语义切片
  -> 元数据抽取
  -> 写入 knowledge_chunks
  -> 生成文本 embedding
  -> 写入 Qdrant text collection
  -> 更新索引状态
```

输入可能是：

- 商家手填的纯文本 `content`。
- 上传文件。
- 外部平台抓取内容。
- 结构化商品参数。
- 视频字幕转写文本。
- 已经整理好的 JSON 数据。

无论原始形态是什么，最终都要统一成 chunk。

### 5.2 图片向量构建

图片不以 OCR 为主。图片处理目标不是“尽量读出图片里的字”，而是“理解图片语义并建立视觉相似索引”。

图片来源：

- 商品主图。
- 商品详情图。
- 商家上传的商品图片。
- 用户上传的咨询图片。
- 外部内容中的图片。

离线商品图片构建：

```text
商品图片 URL
  -> 下载或读取图片
  -> 图像语义理解模型生成 visual_summary
  -> 图像 embedding 模型生成 image_vector
  -> 写入 Qdrant image collection
  -> payload 绑定 product_id / sku_id / merchant_id / category_id
```

用户上传图片在线检索：

```text
用户上传图片
  -> 图像语义理解模型生成 visual_summary
  -> 图像 embedding 模型生成 image_vector
  -> Qdrant 图片 collection 检索相似图片
  -> 得到相似 product_id
  -> 调商品服务补全商品卡片
  -> 必要时再调用 search_knowledge 补充文本证据
```

OCR 只作为辅助：

- 活动海报、规格截图、包装文字等场景可以使用 OCR。
- 无文字图片不应被判定为解析失败。
- 商品相似查找优先使用图片向量检索。

## 6. 切片与清洗设计

### 6.1 文本清洗

通用清洗：

- 删除页眉、页脚、广告、导航、版权重复文本。
- 合并多余空白。
- 保留标题层级。
- 保留型号、规格、价格、时间、限制条件。
- 表格行补全表头。
- 外部内容保留平台、作者、发布时间、原链接。
- 对爬取内容做去重和低质量过滤。

外部内容额外清洗：

- 删除互动话术，例如“点赞收藏”“点个关注”。
- 删除和商品无关的情绪化噪声。
- 保留体验事实，例如“续航大约一天”“鼠标按键声音很轻”。
- 标记主观内容，不把它当成官方事实。

### 6.2 结构化内容归一

如果获取到的是结构化知识，也仍然要转成可检索 chunk。

示例：

```json
{
  "product_id": "p_001",
  "brand": "X",
  "model": "X Phone 12",
  "features": ["高速对焦", "儿童抓拍模式", "256GB 存储"]
}
```

归一为 chunk：

```text
标题：X Phone 12 商品参数
内容：X Phone 12 属于 X 品牌手机，支持高速对焦、儿童抓拍模式，提供 256GB 存储版本。
元数据：product_id=p_001, category_id=c_phone, doc_type=product_detail, source_type=merchant
```

原因：向量库检索的是语义内容，结构化数据也需要有可 embedding 的文本表达。

### 6.3 切片策略

按文档类型切片：

| doc_type | 切片方式 |
| --- | --- |
| `product_detail` | 按商品、参数、卖点、适用人群、风险提示 |
| `promotion` | 按活动名称、适用条件、叠加规则、生效时间 |
| `after_sales` | 按服务类型、适用条件、例外情况 |
| `faq` | 一问一答一个 chunk |
| `guide` | 按类目、场景、选购维度 |
| `review` | 按商品、体验维度、优缺点 |
| `social_post` | 按商品、场景、口碑观点 |
| `video_transcript` | 按主题段落和时间戳 |

建议参数：

| 参数 | 建议值 |
| --- | --- |
| `max_chars` | 600 到 900 中文字符 |
| `min_chars` | 80 到 150 中文字符 |
| `overlap_chars` | 80 到 120 中文字符 |

切片必须保留 `title_path`，避免 chunk 脱离上下文。

## 7. 向量库设计

向量库使用 Qdrant。仓库的 `deployments/docker-compose.yml` 已规划 Qdrant。

### 7.1 Collection 划分

建议分为两个 collection：

| Collection | 向量类型 | 用途 |
| --- | --- | --- |
| `knowledge_text_chunks` | 文本 embedding | 文本知识召回 |
| `product_image_vectors` | 图像 embedding | 图片相似商品召回 |

如果选择多模态同空间 embedding 模型，也可以后续扩展为统一 collection。但 MVP 阶段分开更清晰。

### 7.2 文本向量 payload

```json
{
  "chunk_id": "ck_001",
  "document_id": "doc_001",
  "source_type": "merchant",
  "source_url": "",
  "merchant_id": "m_001",
  "doc_type": "product_detail",
  "title": "X Phone 12 商品详情",
  "product_id": "p_001",
  "category_id": "c_phone",
  "brand": "X",
  "version": "v1",
  "effective_start": "",
  "effective_end": "",
  "quality_score": 0.95,
  "created_at": "2026-05-20T12:00:00+08:00"
}
```

### 7.3 图片向量 payload

```json
{
  "image_vector_id": "imgvec_001",
  "image_url": "https://example.com/x-phone-12.jpg",
  "source_type": "merchant",
  "merchant_id": "m_001",
  "product_id": "p_001",
  "sku_id": "sku_001",
  "category_id": "c_phone",
  "brand": "X",
  "visual_summary": "一台黑色直板智能手机，背部双摄，适合手机商品图识别",
  "quality_score": 0.9,
  "created_at": "2026-05-20T12:00:00+08:00"
}
```

图片向量的返回结果不能直接展示给用户，必须回查商品服务，确认商品仍然上架、价格和库存有效。

## 8. RAG 检索输入

v2 明确：RAG 不接收原始 query，也不做 Query 改写。RAG 接收 Agent 上游传入的结构化检索请求。

### 8.1 文本知识检索请求

```json
{
  "trace_id": "trace_001",
  "session_id": "sess_001",
  "message_id": "msg_001",
  "query_text": "X Phone 12 是否适合拍摄儿童？需要结合对焦、抓拍、存储和风险提示。",
  "intent": "product_detail_qa",
  "merchant_ids": ["m_001"],
  "product_ids": ["p_001"],
  "category_ids": ["c_phone"],
  "doc_types": ["product_detail", "faq", "review"],
  "source_types": ["merchant", "platform", "external_article"],
  "top_k": 8,
  "filters": {
    "effective_only": true,
    "min_quality_score": 0.5
  }
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `query_text` | 已改写好的检索文本，由 Agent 上游提供 |
| `intent` | 意图，用于选择 doc_type 和排序权重 |
| `merchant_ids` | 限制商家资料范围 |
| `product_ids` | 限制关联商品 |
| `category_ids` | 限制类目 |
| `doc_types` | 限制文档类型 |
| `source_types` | 限制知识来源 |
| `top_k` | 最终返回证据数量 |
| `filters` | 有效期、质量分等附加过滤 |

### 8.2 图片相似检索请求

```json
{
  "trace_id": "trace_001",
  "session_id": "sess_001",
  "message_id": "msg_001",
  "image_url": "/uploads/user/a.png",
  "image_vector": [0.012, -0.031],
  "visual_summary": "用户上传了一只黑色无线人体工学鼠标图片，可能在咨询相似商品",
  "category_ids": ["c_mouse"],
  "merchant_ids": ["m_001"],
  "top_k": 10
}
```

说明：

- `image_vector` 可由 RAG 图片服务生成，也可由 Agent 的 `parse_image` 工具生成后传入。
- 如果上游已经生成图片向量，RAG 直接检索 Qdrant。
- 如果只传 `image_url`，RAG 图片检索子模块负责调用图像 embedding。
- `visual_summary` 用于 Trace、调试和后续文本检索补充，不作为图片相似检索的唯一依据。

## 9. 在线检索流程

### 9.1 文本知识检索

```text
结构化检索请求
  -> 读取 query_text
  -> 文本 embedding
  -> Qdrant 文本向量召回 top 30
  -> MySQL 关键词召回 top 30
  -> 元数据过滤
  -> 合并去重
  -> source_type/doc_type/product/category 加权
  -> Rerank top_k
  -> 证据压缩
  -> 返回 Citation 列表
```

关键词召回仍然有价值：

- 型号。
- SKU。
- 活动名。
- 金额。
- 参数名。
- 外部内容标题。

### 9.2 图片相似检索

```text
用户图片或图片向量
  -> 图像 embedding
  -> Qdrant 图片向量召回 top 30
  -> category/merchant 过滤
  -> 合并同 product_id
  -> 按最高相似度和商品状态排序
  -> 回查商品服务
  -> 返回相似商品候选
```

返回结果：

```json
{
  "items": [
    {
      "product_id": "p_mouse_001",
      "sku_id": "sku_mouse_001",
      "matched_image_url": "https://example.com/mouse.jpg",
      "similarity": 0.87,
      "visual_summary": "黑色无线人体工学鼠标，带侧键",
      "source": "image_vector"
    }
  ]
}
```

### 9.3 元数据过滤

过滤维度：

| 字段 | 用途 |
| --- | --- |
| `merchant_id` | 避免商家资料串用 |
| `product_id` | 精确商品问答 |
| `category_id` | 类目导购和图片识别 |
| `doc_type` | 匹配意图 |
| `source_type` | 控制知识来源可信度 |
| `effective_start/effective_end` | 活动规则有效期 |
| `quality_score` | 过滤低质量外部内容 |
| `status` | 只使用 indexed/active 内容 |

### 9.4 来源优先级

不同问题对来源的优先级不同。

商品事实：

```text
结构化商品表 > 商家资料 > 平台资料 > 外部内容
```

活动规则：

```text
商家/平台活动资料 > FAQ > 外部内容不可作为规则依据
```

使用体验：

```text
商家资料 + 平台评测 + 外部内容综合
```

选购建议：

```text
平台指南 + 商家资料 + 外部内容口碑
```

## 10. RAG 输出

### 10.1 文本证据输出

保持和当前 `domain.Citation` 兼容，同时扩展调试字段。

对 Agent 的稳定输出：

```json
{
  "evidence": [
    {
      "chunk_id": "ck_phone_001",
      "title": "X Phone 12 商品详情",
      "snippet": "X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。",
      "source": "merchant:doc_001"
    }
  ],
  "has_sufficient_evidence": true
}
```

内部调试输出：

```json
{
  "chunk_id": "ck_phone_001",
  "document_id": "doc_001",
  "source_type": "merchant",
  "doc_type": "product_detail",
  "product_id": "p_001",
  "score": 0.91,
  "match_sources": ["vector", "keyword"],
  "reasons": ["product_match", "semantic_match"]
}
```

### 10.2 图片相似输出

图片检索输出不等同于 citation，它更像商品候选：

```json
{
  "similar_products": [
    {
      "product_id": "p_mouse_001",
      "similarity": 0.87,
      "matched_image_url": "https://example.com/mouse.jpg",
      "reason": "外观与用户上传图片相似：黑色、无线、人体工学、侧键"
    }
  ]
}
```

Agent 后续必须调用商品服务补全商品卡片，不能直接使用图片向量 payload 生成商品卡。

## 11. 数据模型演进

### 11.1 兼容当前表

当前表：

```text
knowledge_documents(document_id, merchant_id, title, doc_type, content, status, chunk_count)
knowledge_chunks(chunk_id, title, snippet, source, sort_order)
```

Phase 1 不强行重构，可以先在当前表基础上提升检索：

- `knowledge_chunks.source` 继续保存 `document_id`。
- `snippet` 继续作为可展示摘要。
- `SearchKnowledge(ctx, query)` 保持兼容。

### 11.2 后续建议扩展 documents

```text
knowledge_documents
  document_id
  merchant_id
  title
  doc_type
  source_type
  source_url
  external_platform
  external_author
  product_id
  category_id
  version
  content
  status
  chunk_count
  quality_score
  error_message
  created_at
  updated_at
```

### 11.3 后续建议扩展 chunks

```text
knowledge_chunks
  chunk_id
  document_id
  merchant_id
  source_type
  doc_type
  title
  title_path_json
  content
  snippet
  product_id
  category_id
  brand
  vector_id
  embedding_status
  quality_score
  effective_start
  effective_end
  created_at
  updated_at
```

### 11.4 新增图片向量映射表

Qdrant 保存向量，MySQL 保存可审计的图片索引元数据。

```text
product_image_vectors
  image_vector_id
  product_id
  sku_id
  merchant_id
  category_id
  image_url
  visual_summary
  vector_id
  embedding_status
  quality_score
  created_at
  updated_at
```

## 12. Store 与接口演进

### 12.1 当前接口

当前 Store：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

这个接口可以支持演示，但无法承载 v2 RAG。

### 12.2 v2 建议接口

新增结构化接口，保留旧接口兼容：

```go
type KnowledgeSearchInput struct {
    QueryText   string
    Intent      string
    MerchantIDs []string
    ProductIDs  []string
    CategoryIDs []string
    DocTypes    []string
    SourceTypes []string
    TopK        int
}

type ImageSearchInput struct {
    ImageURL      string
    ImageVector   []float32
    VisualSummary string
    MerchantIDs   []string
    CategoryIDs   []string
    TopK          int
}
```

建议 Store 或 service 层提供：

```go
SearchKnowledgeV2(ctx context.Context, input KnowledgeSearchInput) ([]domain.Citation, error)
SearchSimilarProductsByImage(ctx context.Context, input ImageSearchInput) ([]domain.ProductCard, error)
```

MVP 可以先在 service 层适配旧 Store，不必一步到位改完整接口。

## 13. API 设计调整

### 13.1 商家资料上传

当前已有：

```http
POST /api/v1/merchant/documents
```

保留，作为商家纯文本资料上传入口。

后续新增：

```http
POST /api/v1/merchant/documents:upload
Content-Type: multipart/form-data
```

字段：

| 字段 | 说明 |
| --- | --- |
| `file` | 文档文件 |
| `title` | 标题 |
| `doc_type` | 文档类型 |
| `product_id` | 可选，关联商品 |
| `category_id` | 可选，关联类目 |
| `version` | 可选，版本 |

### 13.2 平台自建知识导入

```http
POST /api/v1/admin/knowledge/documents
```

用于导入平台选购指南、评测整理、标准 FAQ。

### 13.3 外部内容导入

不建议直接在用户请求链路实时爬取外部平台。应走离线导入：

```http
POST /api/v1/admin/knowledge/external-documents
```

请求：

```json
{
  "source_type": "external_xhs",
  "source_url": "https://example.com/post/1",
  "title": "X Phone 12 拍娃体验",
  "author": "demo_author",
  "published_at": "2026-05-01T00:00:00+08:00",
  "content": "这台手机抓拍孩子时对焦比较快，但夜景下会有噪点。",
  "product_id": "p_001",
  "category_id": "c_phone"
}
```

### 13.4 图片向量重建

```http
POST /api/v1/admin/products/images:index
```

用途：

- 为商品主图生成图片向量。
- 批量写入 Qdrant 图片 collection。
- 后续支持用户上传图片找相似商品。

### 13.5 RAG 调试接口

```http
POST /api/v1/admin/rag/search
```

请求必须是结构化检索请求，不负责 Query 改写：

```json
{
  "query_text": "X Phone 12 是否适合拍摄儿童？",
  "intent": "product_detail_qa",
  "product_ids": ["p_001"],
  "doc_types": ["product_detail", "review"],
  "source_types": ["merchant", "platform", "external_article"],
  "top_k": 8
}
```

## 14. Agent 集成

RAG 在 Agent 中继续作为工具存在，但工具输入要升级。

Agent 主链路：

```text
Context Builder
  -> Query Rewriter
  -> Intent Router
  -> Tool Plan Builder
  -> RAG search_knowledge / image_similar_product_search
  -> Product Service resolve_products
  -> Main Answer LLM
```

`search_knowledge` 工具输入：

```json
{
  "query_text": "X Phone 12 是否适合拍摄儿童？",
  "intent": "product_detail_qa",
  "product_ids": ["p_001"],
  "category_ids": ["c_phone"],
  "doc_types": ["product_detail", "review"],
  "source_types": ["merchant", "platform", "external_article"],
  "top_k": 8
}
```

`image_similar_product_search` 工具输入：

```json
{
  "image_url": "/uploads/user/mouse.png",
  "visual_summary": "黑色无线人体工学鼠标，带侧键",
  "category_ids": ["c_mouse"],
  "top_k": 5
}
```

回答约束：

- 商品价格、库存、SKU、上下架状态必须来自商品服务。
- 商家资料优先于平台资料，平台资料优先于外部内容。
- 外部内容只能作为体验和口碑参考，不能作为活动规则和售后承诺依据。
- 图片相似检索只给候选商品，最终商品卡片必须回查商品服务。

## 15. 评测指标调整

### 15.1 文本 RAG 指标

| 指标 | 说明 |
| --- | --- |
| Top-K 命中率 | 标准证据是否进入前 K |
| MRR | 标准证据排序质量 |
| source_type 准确率 | 是否使用了合适来源 |
| doc_type 准确率 | 是否检索到正确文档类型 |
| 商品过滤准确率 | 是否命中正确 product_id |
| 无证据识别率 | 无资料时是否不编造 |

### 15.2 图片检索指标

| 指标 | 说明 |
| --- | --- |
| 图片 Top-K 商品命中率 | 相似商品是否在前 K |
| 类目识别准确率 | 图片是否过滤到正确类目 |
| 误召回率 | 无关商品进入候选的比例 |
| 商品卡片补全成功率 | 候选 product_id 是否能回查商品服务 |

### 15.3 外部知识质量指标

| 指标 | 说明 |
| --- | --- |
| 去重率 | 重复外部内容是否被消除 |
| 低质量过滤率 | 广告、灌水、无关内容是否被过滤 |
| 冲突识别率 | 外部内容和商家资料冲突时是否降权 |
| 引用可追溯率 | 外部内容是否保留来源 URL |

## 16. 分期落地计划

### Phase 1：对齐当前代码的最小 RAG

目标：不大改架构，先让当前 MySQL RAG 更可信。

- 保留 `POST /api/v1/merchant/documents`。
- 上传资料时按段落切多个 chunk，不再只生成 1 个 chunk。
- `knowledge_chunks.source` 继续保存 `document_id`。
- `SearchKnowledge` 增加基础关键词切词，不只整句 LIKE。
- Agent citation 继续使用当前 `domain.Citation`。

### Phase 2：结构化检索输入

目标：把 RAG 和 Agent 上游边界拆清楚。

- 新增 `KnowledgeSearchInput`。
- `search_knowledge` 工具接收 `query_text/product_ids/category_ids/doc_types/source_types`。
- Query 改写仍由 Agent 上游负责。
- 增加 RAG Trace。

### Phase 3：文本向量库

目标：接入 Qdrant 文本向量检索。

- 扩展 `knowledge_documents` 和 `knowledge_chunks` 元数据。
- 接入文本 Embedding 模型。
- 写入 Qdrant `knowledge_text_chunks`。
- 实现向量召回 + 关键词召回 + 合并去重。

### Phase 4：平台自建和外部知识库

目标：扩展知识来源。

- 支持管理员导入平台指南。
- 支持外部内容离线导入。
- 增加 `source_type/source_url/quality_score`。
- 外部内容只用于体验和口碑，不覆盖强事实。

### Phase 5：图片向量检索

目标：支持用户上传图片查找相似商品。

- 为商品图片生成 image embedding。
- 写入 Qdrant `product_image_vectors`。
- 用户上传图片生成 image embedding。
- 图片向量召回相似商品。
- 回查商品服务生成商品卡片。

### Phase 6：重排与评测闭环

目标：提升质量并可量化证明。

- 规则 Rerank。
- 可选接入 rerank 模型。
- 建立文本 RAG、图片检索、外部知识质量评测集。
- 在管理员端展示检索 Trace 和失败归因。

## 17. v1 问题逐条回应

### 17.1 知识库不只包含商家商品库

已调整。v2 将知识来源分为商家知识库、平台自建知识库、外部内容知识库，并通过 `source_type` 区分可信度和使用边界。

### 17.2 结构化和非结构化知识都要切分清洗后入向量库

已调整。v2 明确结构化知识也要归一为可 embedding 的文本 chunk，再写入 MySQL 和 Qdrant。

### 17.3 图片不要以 OCR 为主

已调整。v2 将图片处理改为图像语义理解和图像 embedding 优先，OCR 只作为活动海报、包装文字等场景的辅助能力。

### 17.4 用户上传图片查找类似商品

已调整。v2 新增 `product_image_vectors` collection 和图片相似检索流程，通过图片向量召回相似商品，再回查商品服务生成商品卡片。

### 17.5 Query 改写不属于 RAG

已调整。v2 明确 RAG 不接收原始 query，不做 Query 改写，只接收 Agent 上游传入的结构化检索请求。

### 17.6 RAG 模块要和已有工作耦合

已调整。v2 增加当前代码基线、兼容当前 `SearchKnowledge` 和 `Citation` 的阶段性演进方案，避免直接设计一个无法从当前仓库迁移过去的新系统。
