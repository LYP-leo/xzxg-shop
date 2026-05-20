# 电商 AI 导购 RAG 设计 v3

## 1. v3 调整目标

本文基于 [RAG设计_v2.md](./RAG设计_v2.md) 和 [RAG_v2_问题.md](./RAG_v2_问题.md) 修订。

v3 只调整两个关键点：

- 明确图片不做传统意义上的“清洗”，例如不去水印、不裁剪、不修图、不改像素内容；图片只做语义理解、质量标记和向量化。
- 重新核对已有设计文档和当前代码，确定 RAG 检索输入到底使用什么格式。

结论：

- 已有目标设计中已经为 RAG 预留了工具输入格式，见 [技术方案_v2.md](./技术方案_v2.md) 的 `9.5 search_knowledge`：

```json
{
  "query": "会员券能不能和满减一起用",
  "intent": "promotion_rule_qa",
  "filters": {
    "doc_type": ["promotion"],
    "product_ids": [],
    "category": null,
    "effective_at": "2026-05-19T22:00:00+08:00"
  },
  "top_k": 8
}
```

- 当前代码实现中还没有结构化输入，只存在 `SearchKnowledge(ctx, query string)`，调用点在 [backend/src/agent/runtime.go](../../backend/src/agent/runtime.go)。
- 所以 v3 采用“目标输入格式以 `技术方案_v2.md` 的 `search_knowledge` 为准，当前代码用兼容适配层过渡”的方案。

本文只做设计，不直接实现代码。

## 2. 当前代码与文档核对

### 2.1 当前代码已有输入

当前后端 Store 接口：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

当前 Agent Runtime 调用：

```go
chunks := r.store.SearchKnowledge(ctx, message.Content)
```

这说明当前代码层面，RAG 只有一个字符串输入，也就是用户原始消息内容。它能支撑演示，但不能表达：

- 意图。
- 文档类型过滤。
- 商品 ID 过滤。
- 类目过滤。
- 生效时间。
- 来源类型。
- top_k。
- debug。

### 2.2 现有设计文档已经预留的输入

`技术方案_v2.md` 的 `9.5 search_knowledge` 已经给出 RAG 工具输入：

```json
{
  "query": "会员券能不能和满减一起用",
  "intent": "promotion_rule_qa",
  "filters": {
    "doc_type": ["promotion"],
    "product_ids": [],
    "category": null,
    "effective_at": "2026-05-19T22:00:00+08:00"
  },
  "top_k": 8
}
```

`技术方案_v2.md` 的 `22.8.7 知识库检索调试` 也预留了管理端调试输入：

```json
{
  "query": "会员券能不能和满减叠加",
  "filters": {
    "doc_type": ["promotion"],
    "product_id": "p_001",
    "category_id": "c_phone"
  },
  "top_k": 8,
  "debug": true
}
```

因此，v3 不再另造一套 `query_text` 顶层字段，而是回到既有技术方案的命名：

- 文本检索主字段用 `query`。
- 结构化限制放在 `filters`。
- `intent` 作为排序和 doc_type 推断依据。
- `top_k` 控制最终返回数量。
- 管理调试接口额外支持 `debug`。

### 2.3 v2 设计里的偏差

v2 使用了：

```json
{
  "query_text": "...",
  "merchant_ids": [],
  "product_ids": [],
  "category_ids": [],
  "doc_types": [],
  "source_types": []
}
```

这个格式表达力更强，但和已有 `技术方案_v2.md` 的 `search_knowledge` 预留格式不一致。

v3 调整为：

- 对外工具输入遵循已有 `query + intent + filters + top_k`。
- 内部 service 可以把它 normalize 成更丰富的结构。
- 不让 Agent 工具契约和 RAG 内部实现结构混在一起。

## 3. 模块边界

RAG 模块负责：

- 文本知识构建：清洗、切片、元数据提取、Embedding、向量入库。
- 图片索引构建：图像语义理解、图像 Embedding、图片向量入库。
- 文本知识检索：向量召回、关键词召回、过滤、去重、重排、证据压缩。
- 图片相似检索：用户图片向量召回相似商品图片，再映射到商品。
- 返回 citation 或相似商品候选。
- 记录检索 Trace。

RAG 模块不负责：

- Query 改写。
- 意图识别。
- 指代消解。
- 最终回答生成。
- 商品价格、库存、上下架状态的最终确认。

Query 改写由 Agent 上游完成。RAG 的 `query` 字段应当是“已改写、适合检索”的文本。字段名仍叫 `query`，是为了和现有技术方案工具契约保持一致。

## 4. 知识来源

v3 继续保留 v2 的三类知识来源。

| 来源 | source_type | 用途 | 可信度 |
| --- | --- | --- | --- |
| 商家资料 | `merchant` | 商品详情、活动规则、售后政策、FAQ | 高 |
| 平台自建知识 | `platform` | 选购指南、横评、标准 FAQ | 中高 |
| 外部内容 | `external_xhs` / `external_bilibili` / `external_article` | 种草、测评、口碑、使用体验 | 中低 |

使用原则：

- 商品价格、库存、SKU、上下架状态只以商品服务为准。
- 活动规则和售后承诺优先使用商家或平台资料。
- 外部内容只用于口碑、体验、选购建议，不作为强事实依据。
- 如果外部内容和商家资料冲突，商家资料优先。

## 5. 文本构建链路

```text
知识来源接入
  -> 保存 knowledge_documents
  -> 解析内容
  -> 文本清洗
  -> 结构化内容归一
  -> 语义切片
  -> 元数据提取
  -> 写入 knowledge_chunks
  -> 文本 Embedding
  -> 写入 Qdrant knowledge_text_chunks
  -> 更新索引状态 indexed
```

### 5.1 文本清洗

文本清洗保留 v2 设计：

- 删除页眉、页脚、广告、导航、版权重复文本。
- 合并多余空白。
- 保留标题层级。
- 保留型号、规格、价格、时间、限制条件。
- 表格行补全表头。
- 外部内容保留平台、作者、发布时间、原链接。
- 对爬取内容做去重和低质量过滤。

外部内容额外规则：

- 删除“点赞收藏”“点个关注”等互动话术。
- 删除与商品无关的情绪化噪声。
- 保留体验事实，例如“续航大约一天”“鼠标按键声音很轻”。
- 标记主观内容，不把它当官方事实。

### 5.2 结构化内容归一

结构化知识也要转成可 embedding 的文本 chunk。

示例：

```json
{
  "product_id": "p_001",
  "brand": "X",
  "features": ["高速对焦", "儿童抓拍模式", "256GB 存储"]
}
```

归一为：

```text
标题：X Phone 12 商品参数
内容：X Phone 12 属于 X 品牌手机，支持高速对焦、儿童抓拍模式，提供 256GB 存储版本。
```

## 6. 图片处理设计

### 6.1 图片不做清洗

v3 明确：图片不做传统清洗。

不做：

- 不去水印。
- 不裁剪水印。
- 不修图。
- 不擦除背景。
- 不调整商品主体。
- 不把图片转成“干净图”后再入库。

原因：

- 去水印、裁剪、修图会改变原始图片证据，不利于溯源。
- 商品相似检索需要保留真实图片分布，包括背景、拍摄角度、包装、海报风格等上下文。
- 图像 embedding 模型通常能容忍一定水印、背景和压缩噪声。
- 对比赛项目而言，图片处理链路越简单越容易稳定演示。

### 6.2 图片只做语义理解和质量标记

图片入库时做：

- 调用图像语义理解模型生成 `visual_summary`。
- 调用图像 embedding 模型生成 `image_vector`。
- 记录图片元数据。
- 标记质量问题，但不修改图片本身。

质量标记示例：

```json
{
  "has_watermark": true,
  "is_blurry": false,
  "has_multiple_products": false,
  "main_object_confidence": 0.86
}
```

这些质量标记用于排序和调试，而不是用于图像清洗。

### 6.3 图片向量构建

```text
商品图片 URL
  -> 读取原图
  -> Vision 模型生成 visual_summary
  -> 图像 embedding 模型生成 image_vector
  -> 写入 Qdrant product_image_vectors
  -> payload 绑定 product_id / sku_id / merchant_id / category_id
```

### 6.4 用户上传图片检索

```text
用户上传图片
  -> 读取原图
  -> Vision 模型生成 visual_summary
  -> 图像 embedding 模型生成 image_vector
  -> Qdrant 图片向量检索
  -> 得到相似 product_id
  -> 回查商品服务
  -> 返回相似商品候选
```

OCR 只作为辅助：

- 活动海报规则解释。
- 包装文字识别。
- 商品规格截图。
- 购物车截图。

无文字图片不应被判定为解析失败。

## 7. RAG 检索输入

### 7.1 对外工具输入：采用已有 search_knowledge 格式

RAG 文本检索工具输入：

```json
{
  "query": "会员券能不能和满减一起用",
  "intent": "promotion_rule_qa",
  "filters": {
    "doc_type": ["promotion"],
    "product_ids": [],
    "category_id": null,
    "merchant_ids": ["m_001"],
    "source_type": ["merchant", "platform"],
    "effective_at": "2026-05-19T22:00:00+08:00",
    "min_quality_score": 0.5
  },
  "top_k": 8
}
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `query` | Agent 上游已经改写好的检索文本 |
| `intent` | 意图，用于默认过滤和排序权重 |
| `filters.doc_type` | 文档类型过滤 |
| `filters.product_ids` | 商品过滤 |
| `filters.category_id` | 类目过滤，沿用技术方案调试接口的单值形式 |
| `filters.merchant_ids` | 商家过滤 |
| `filters.source_type` | 来源过滤 |
| `filters.effective_at` | 活动或政策生效时间点 |
| `filters.min_quality_score` | 外部内容最低质量分 |
| `top_k` | 最终返回 chunk 数量 |

说明：

- `query` 字段名沿用已有 `search_knowledge` 工具契约。
- 语义上，`query` 不是用户原话，而是 Agent 上游处理后的检索 query。
- 如果上游没有完成 Query 改写，MVP 可以临时传用户原话，但这属于 Agent 未完成，不属于 RAG 职责。

### 7.2 内部归一结构

RAG service 内部可以把外部输入归一为更强类型结构：

```go
type KnowledgeSearchRequest struct {
    Query     string
    Intent    string
    Filters   KnowledgeSearchFilters
    TopK      int
    Debug     bool
    TraceID   string
    MessageID string
}

type KnowledgeSearchFilters struct {
    DocTypes        []string
    ProductIDs      []string
    CategoryID      string
    MerchantIDs     []string
    SourceTypes     []string
    EffectiveAt     time.Time
    MinQualityScore float64
}
```

### 7.3 当前代码兼容层

当前 Store 仍是：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

兼容策略：

```text
SearchKnowledge(ctx, query)
  -> 构造 KnowledgeSearchRequest{
       Query: query,
       TopK: 5,
       Filters: empty
     }
  -> 调用 SearchKnowledgeV3
```

这样可以先保持当前 Agent Runtime 不变，再逐步把 Agent 工具输入升级为结构化格式。

## 8. 图片相似检索输入

现有技术方案中已经预留 `image_similar_product_search`：

```json
{
  "image_url": "/uploads/a.png",
  "detected_category": "鼠标",
  "top_k": 5
}
```

v3 在此基础上扩展，但保持兼容：

```json
{
  "image_url": "/uploads/a.png",
  "detected_category": "鼠标",
  "filters": {
    "category_id": "c_mouse",
    "merchant_ids": ["m_001"],
    "min_quality_score": 0.5
  },
  "top_k": 5
}
```

内部可选字段：

```json
{
  "image_vector": [0.012, -0.031],
  "visual_summary": "黑色无线人体工学鼠标，带侧键"
}
```

规则：

- 如果已有 `image_vector`，直接查 Qdrant。
- 如果只有 `image_url`，RAG 图片检索子模块负责生成 image embedding。
- `visual_summary` 只用于调试、Trace 和回答解释，不替代图片向量检索。

## 9. 在线检索流程

### 9.1 文本检索

```text
search_knowledge 输入
  -> 校验 query/top_k/filters
  -> 根据 intent 补默认 doc_type
  -> 文本 embedding
  -> Qdrant 向量召回 top 30
  -> MySQL 关键词召回 top 30
  -> 元数据过滤
  -> 合并去重
  -> 来源和意图加权
  -> Rerank
  -> 证据压缩
  -> 返回 chunks
```

`intent` 默认过滤：

| intent | 默认 doc_type |
| --- | --- |
| `product_recommendation` | `product_detail`、`guide`、`review`、`faq` |
| `product_detail_qa` | `product_detail`、`faq`、`review` |
| `promotion_rule_qa` | `promotion` |
| `after_sales_qa` | `after_sales`、`faq` |
| `product_comparison` | `product_detail`、`review`、`guide` |
| `image_rule_explanation` | `promotion` |

如果调用方显式传了 `filters.doc_type`，以调用方为准。

### 9.2 图片检索

```text
image_similar_product_search 输入
  -> 读取原图
  -> 生成 image embedding
  -> Qdrant product_image_vectors 检索 top 30
  -> 类目/商家/质量分过滤
  -> 合并同 product_id
  -> 回查商品服务
  -> 返回相似商品候选
```

图片检索不返回 citation，它返回商品候选。最终商品卡片仍由商品服务补全。

## 10. 输出格式

### 10.1 search_knowledge 输出

沿用 `技术方案_v2.md`：

```json
{
  "chunks": [
    {
      "chunk_id": "ck_001",
      "title": "618 活动规则",
      "snippet": "会员券可与满减叠加，但不可与新人券叠加。",
      "score": 0.87,
      "metadata": {
        "document_id": "doc_001",
        "doc_type": "promotion",
        "source_type": "merchant"
      }
    }
  ]
}
```

当前前端和后端 SSE 使用 `domain.Citation`：

```json
{
  "chunkId": "ck_001",
  "title": "618 活动规则",
  "snippet": "会员券可与满减叠加，但不可与新人券叠加。",
  "source": "doc_001"
}
```

过渡期做法：

- RAG service 返回完整 `chunks`。
- Agent 对外 SSE 只取 `chunk_id/title/snippet/source` 组装 `Citation`。
- 调试接口返回完整 metadata。

### 10.2 image_similar_product_search 输出

```json
{
  "products": [
    {
      "product_id": "p_mouse_001",
      "matched_image_url": "https://example.com/mouse.jpg",
      "similarity": 0.87,
      "reason": "图片外观相似：黑色、无线、人体工学、侧键"
    }
  ]
}
```

## 11. 数据模型调整

### 11.1 文本知识表

当前表先兼容：

```text
knowledge_documents
knowledge_chunks
```

后续扩展字段：

```text
knowledge_documents
  document_id
  merchant_id
  title
  doc_type
  source_type
  source_url
  product_id
  category_id
  version
  content
  status
  chunk_count
  quality_score
  effective_start
  effective_end
  created_at
  updated_at

knowledge_chunks
  chunk_id
  document_id
  merchant_id
  source_type
  doc_type
  title
  content
  snippet
  product_id
  category_id
  vector_id
  embedding_status
  quality_score
  created_at
  updated_at
```

### 11.2 图片向量元数据表

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
  has_watermark
  is_blurry
  has_multiple_products
  main_object_confidence
  created_at
  updated_at
```

注意：

- `has_watermark` 只是质量标记。
- 不对图片本体做去水印处理。

## 12. API 设计

### 12.1 管理端知识库调试

沿用现有技术方案：

```http
POST /api/v1/admin/knowledge:search
```

请求：

```json
{
  "query": "会员券能不能和满减叠加",
  "filters": {
    "doc_type": ["promotion"],
    "product_id": "p_001",
    "category_id": "c_phone"
  },
  "top_k": 8,
  "debug": true
}
```

服务端内部把 `product_id` 归一成 `product_ids: ["p_001"]`。

### 12.2 Agent 工具 search_knowledge

```json
{
  "query": "会员券能不能和满减一起用",
  "intent": "promotion_rule_qa",
  "filters": {
    "doc_type": ["promotion"],
    "product_ids": [],
    "category_id": null,
    "effective_at": "2026-05-19T22:00:00+08:00"
  },
  "top_k": 8
}
```

### 12.3 Agent 工具 image_similar_product_search

```json
{
  "image_url": "/uploads/a.png",
  "detected_category": "鼠标",
  "top_k": 5
}
```

## 13. 分期落地

### Phase 1：兼容当前代码

- 保留 `SearchKnowledge(ctx, query string)`。
- 内部构造默认结构化请求。
- 上传资料时按段落切多个 chunk。
- 不改 SSE `Citation` 结构。

### Phase 2：接入已有 search_knowledge 工具输入

- 新增 `KnowledgeSearchRequest`。
- Agent 工具按 `query + intent + filters + top_k` 调用。
- 管理端调试接口按 `POST /api/v1/admin/knowledge:search` 实现。
- RAG 不做 Query 改写，只消费 `query`。

### Phase 3：Qdrant 文本向量

- 扩展 chunk 元数据。
- 接文本 Embedding。
- 写入 `knowledge_text_chunks`。
- 实现向量召回 + 关键词召回。

### Phase 4：图片向量

- 不做图片清洗。
- 为商品图生成 `visual_summary` 和 `image_vector`。
- 写入 `product_image_vectors`。
- 实现用户图片相似商品检索。

### Phase 5：外部知识和评测

- 支持平台自建知识导入。
- 支持外部内容离线导入。
- 增加 source_type、quality_score。
- 建立文本 RAG 和图片检索评测集。

## 14. v2 问题逐条回应

### 14.1 图片要不要清洗

结论：不清洗。

v3 明确图片不去水印、不裁剪、不修图、不改变像素内容。只做语义理解、向量化和质量标记。水印、模糊、多主体等问题通过 metadata 标记和排序降权处理。

### 14.2 RAG 检索输入到底是什么

结论：已有目标设计已经预留输入格式，采用 `技术方案_v2.md` 的 `search_knowledge` 工具输入：

```json
{
  "query": "...",
  "intent": "...",
  "filters": {},
  "top_k": 8
}
```

当前代码还没有这个结构化输入，只有 `SearchKnowledge(ctx, query string)`。因此 v3 设计采用两层：

- 目标工具契约：`query + intent + filters + top_k`。
- 当前兼容入口：`SearchKnowledge(ctx, query string)` 内部转换为默认结构化请求。

这样既对齐已有设计文档，也能从当前实现平滑演进。
