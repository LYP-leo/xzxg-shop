# Milvus 向量库接入设计 v1

> 日期：2026-05-23  
> 目标：把商品与知识资料接入 Milvus，形成 `RDS 结构化过滤 + Milvus 向量召回 + 关键词召回 + 通用重排` 的混合检索链路，避免用单品/单品牌特殊逻辑解决召回问题。

## 1. 背景与问题

当前后端的商品和知识检索主要依赖 MySQL LIKE 关键词召回。这个方案能快速演示，但存在几个问题：

- 关键词 OR 召回容易混入弱相关商品，例如“华为电脑”会召回华为手机、华为平板、华为耳机。
- 单纯靠代码写特殊规则不可持续，例如针对 Apple/iPad 的特判会越来越多。
- 商品详情、FAQ、评价、商家资料等非结构化内容无法做真正语义召回。
- Agent 无法稳定获得“真实商品候选 + 可靠证据片段”的一致结果。

目标方案不是让 Milvus 替代 MySQL。MySQL/RDS 仍然是商品、库存、价格、订单和商家资料的真实主库；Milvus 只负责语义召回候选。

## 2. 总体原则

1. RDS 负责硬约束。
   - 商品状态、库存状态、价格、品牌、类目、商家、上下架等必须以 MySQL 为准。

2. Milvus 负责语义召回。
   - 商品语义、知识片段语义、后续图片语义都通过向量召回补充候选。

3. 检索策略必须通用化。
   - 不写“iPad 排除”“Apple 特判”这类针对个别商品的逻辑。
   - 用品牌、类目、商品 ID、商家 ID、doc_type、source_type 等 metadata filter 解决。

4. Agent 不直接拼 Milvus 查询。
   - Agent 只生成 `RetrievalPlan` 或调用现有工具。
   - RAG 层根据 `RetrievalPlan` 执行 RDS、Milvus、keyword、rerank。

5. 兼容当前接口。
   - 保留 `SearchProducts(ctx, query)` 和 `SearchKnowledge(ctx, query)`。
   - 内部逐步升级为混合召回，不破坏现有前后端调用。

## 3. 工具边界

检索链路必须封装在 Agent 工具内部。Agent 只看到稳定工具，不直接感知 RDS、Milvus、keyword、rerank 等内部路径。

对 Agent 暴露的工具保持为：

```text
search_products(query, filters?, limit?)
search_knowledge(query, filters?, limit?)
```

工具内部再执行：

```text
Search Tool
  -> Query Analyzer / RetrievalPlan Builder
  -> RDS structured search
  -> Milvus vector search
  -> Keyword search
  -> Rule recall
  -> merge / deduplicate
  -> rerank
  -> RDS hydrate
  -> return normalized result
```

这样做的原因：

- Agent 只负责“什么时候搜、搜什么”，不负责“怎么搜”。
- 检索策略可以独立升级，例如从 MySQL LIKE 升级到 MySQL FULLTEXT 或 Milvus，不需要改 Agent prompt。
- 管理员 trace 可以展示工具内部每条检索路径，便于排查。
- 前端和 Agent 都只依赖工具返回协议，不依赖底层存储实现。

### 3.1 工具内部路径

`search_products` 内部路径：

```text
search_products(query)
  -> 抽取品牌/类目/价格/库存等 facet
  -> RDS 商品硬过滤候选
  -> Milvus product_text_vectors 语义候选
  -> MySQL keyword 候选
  -> 合并去重
  -> 商品重排
  -> 回查 RDS 补全商品卡
  -> 返回 product cards + product_ids + trace
```

`search_knowledge` 内部路径：

```text
search_knowledge(query)
  -> 构造 RetrievalPlan
  -> Milvus knowledge_text_chunks 语义候选
  -> MySQL knowledge_chunks keyword 候选
  -> Rule recall 候选
  -> metadata filter
  -> 合并去重
  -> chunk 重排
  -> snippet 压缩
  -> 返回 citations + chunk_ids + trace
```

工具返回仍保持当前 Agent 可消费结构：

```json
{
  "ok": true,
  "tool": "search_products",
  "result": {
    "items": []
  },
  "product_ids": [],
  "trace": {
    "rds_hit_ids": [],
    "vector_hit_ids": [],
    "keyword_hit_ids": [],
    "rerank_top_ids": []
  }
}
```

## 4. 总体链路

### 4.1 商品推荐链路

```text
用户 query
  -> Agent 理解需求
  -> 工具 search_products(query)
  -> 工具内部完成 RDS + Milvus + keyword + rerank
  -> 返回 Agent
```

### 4.2 知识问答链路

```text
用户 query
  -> Agent 生成 RetrievalPlan
  -> 工具 search_knowledge(query 或 plan)
  -> 工具内部完成 Milvus + keyword + rule + rerank
  -> citation_refs 返回 Agent
```

### 4.3 商品与知识一致性

如果商品检索已确定候选商品 ID，例如 `p_digital_004`，知识检索应优先限制：

```text
product_id in ["p_digital_004"]
```

如果没有明确商品候选，但 query 抽到了品牌/类目，例如“华为电脑”，知识检索应使用：

```text
brand == "华为" AND category_id == "c_dataset_digital_laptop"
```

这能避免商品推荐是 MateBook，但知识片段却召回华为平板或手机。

## 5. Milvus Collection 设计

### 5.1 `product_text_vectors`

用途：商品语义召回。

每个商品写一条向量。向量文本由结构化商品信息拼接而成：

```text
商品名
品牌
类目
tags
selling_points
recommend_reason
attributes
suitable_for
description 摘要
```

字段设计：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | `product_id` |
| `embedding` | FloatVector | 商品文本向量 |
| `product_id` | VarChar | 商品 ID |
| `merchant_id` | VarChar | 商家 ID |
| `category_id` | VarChar | 类目 ID |
| `brand` | VarChar | 品牌 |
| `status` | VarChar | 商品状态 |
| `stock_status` | VarChar | 库存状态 |
| `price` | Float | 当前价格 |
| `updated_at_ts` | Int64 | 更新时间 |

索引建议：

```text
metric_type: COSINE
index_type: HNSW 或 IVF_FLAT
```

MVP 阶段数据量较小，优先 HNSW，便于低延迟查询。

### 5.2 `knowledge_text_chunks`

用途：商品资料、FAQ、评价、商家资料、平台规则等文本知识召回。

字段设计：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | `chunk_id` |
| `embedding` | FloatVector | 文本片段向量 |
| `chunk_id` | VarChar | chunk ID |
| `document_id` | VarChar | 文档 ID |
| `source` | VarChar | 来源 ID，商品资料可填 product_id |
| `source_type` | VarChar | `product` / `merchant_doc` / `platform_doc` / `faq` / `review` |
| `product_id` | VarChar | 商品 ID |
| `merchant_id` | VarChar | 商家 ID |
| `category_id` | VarChar | 类目 ID |
| `brand` | VarChar | 品牌 |
| `doc_type` | VarChar | `product_desc` / `faq` / `review` / `policy` / `promotion` |
| `quality_score` | Float | 质量分 |
| `effective_start_ts` | Int64 | 生效开始时间 |
| `effective_end_ts` | Int64 | 生效结束时间 |
| `created_at_ts` | Int64 | 创建时间 |

### 5.3 后续：`product_image_vectors`

用途：图片搜同款、图片搜相似款。

字段设计：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | 图片向量 ID |
| `embedding` | FloatVector | 图片向量 |
| `product_id` | VarChar | 商品 ID |
| `merchant_id` | VarChar | 商家 ID |
| `category_id` | VarChar | 类目 ID |
| `brand` | VarChar | 品牌 |
| `image_url` | VarChar | 图片地址 |
| `image_type` | VarChar | 主图、详情图、用户图 |
| `updated_at_ts` | Int64 | 更新时间 |

## 6. Embedding 设计

### 6.1 接口抽象

新增统一接口：

```go
type Embedder interface {
    EmbedText(ctx context.Context, texts []string) ([][]float32, error)
}
```

MVP 使用 DashScope/OpenAI compatible embedding 模型。模型名放 Nacos，不写死。

### 6.2 配置项

建议放入 Nacos：

```json
{
  "embedding.enabled": true,
  "embedding.provider": "dashscope",
  "embedding.model": "text-embedding-v4",
  "embedding.batch_size": 16,
  "embedding.timeout_ms": 15000
}
```

最终模型维度以实际 embedding 模型返回为准，collection 初始化时必须使用同一维度。

## 7. 后端模块设计

建议新增模块：

```text
backend/src/vector/
  milvus.go
  schema.go
  embedder.go
  indexer.go
  hybrid_search.go
```

职责：

- `milvus.go`：Milvus 连接、health check、search、upsert。
- `schema.go`：collection schema、index 创建、load collection。
- `embedder.go`：embedding 客户端。
- `indexer.go`：商品和知识片段批量向量化、写入 Milvus。
- `hybrid_search.go`：向量召回结果与 RDS/keyword 结果合并。

RAG 层建议新增：

```text
backend/src/rag/service.go
backend/src/rag/reranker.go
```

职责：

- `rag.Service.SearchKnowledgeByPlan`
- `rag.Service.SearchProductsByPlan`
- 统一执行 RetrievalPlan、metadata filter、rerank、trace。

当前 `store.SearchKnowledge`、`store.SearchProducts` 保留为兼容入口。Agent 工具调用仍进入 `toolSearchProducts`、`toolSearchKnowledge`，工具内部调用新的 search service。

建议工具调用关系：

```text
backend/src/agent/tools.go
  -> search_products
  -> backend/src/search.Service.SearchProducts
      -> RDS / Milvus / keyword / rerank

backend/src/agent/tools.go
  -> search_knowledge
  -> backend/src/search.Service.SearchKnowledge
      -> Milvus / keyword / rule / rerank
```

不建议让 Agent Runtime 直接调用 Milvus client，也不建议在 prompt 中暴露“向量召回/RDS 召回”。

## 8. Nacos 配置设计

建议配置：

```json
{
  "vector.enabled": true,
  "vector.provider": "milvus",
  "milvus.address": "127.0.0.1:19530",
  "milvus.database": "xzxg_shop",
  "milvus.collection.products": "product_text_vectors",
  "milvus.collection.knowledge": "knowledge_text_chunks",
  "milvus.collection.images": "product_image_vectors",
  "retrieval.vector.enabled": true,
  "retrieval.vector.top_n": 40,
  "retrieval.keyword.enabled": true,
  "retrieval.keyword.top_n": 40,
  "retrieval.rerank.top_k": 5,
  "retrieval.min_score": 0.0
}
```

本地开发可允许 `vector.enabled=false`，自动降级到 MySQL keyword 召回。

## 9. Docker 设计

当前 `deployments/docker-compose.yml` 中已有 MySQL、Redis、Nacos 和旧的 Qdrant。接 Milvus 后建议：

- 保留 MySQL、Redis、Nacos。
- 移除或停用 Qdrant。
- 增加 Milvus standalone 依赖：
  - `etcd`
  - `minio`
  - `milvus-standalone`

端口：

```text
19530  Milvus gRPC
9091   Milvus metrics/HTTP
9000   MinIO API
9001   MinIO Console
2379   etcd
```

## 10. 混合召回与重排

### 10.1 商品召回

输入：

```go
type ProductSearchRequest struct {
    Query       string
    Brand       string
    CategoryIDs []string
    MerchantIDs []string
    PriceMin    *decimal.Decimal
    PriceMax    *decimal.Decimal
    InStockOnly bool
    TopK        int
}
```

候选来源：

- RDS exact/facet candidates
- Milvus semantic candidates
- keyword candidates

合并后统一打分：

```text
final_score =
  0.40 * vector_score
  0.25 * keyword_score
  0.20 * facet_match_score
  0.10 * business_score
  0.05 * freshness_score
```

其中 `business_score` 可包含库存、价格合理性、商家质量、活动状态。

### 10.2 知识召回

输入沿用当前 `RetrievalPlan`：

```go
type RetrievalPlan struct {
    Query    string
    Filters  RetrievalFilters
    Recall   RecallPlan
    Rerank   RerankPlan
    Compress CompressPlan
    Trace    RetrievalTraceContext
}
```

召回来源：

- Milvus vector recall
- MySQL keyword recall
- rule recall

合并后统一使用 `chunk_id` 去重。

## 11. Trace 设计

管理员全链路追踪需要记录：

```json
{
  "stage": "rag.vector_search",
  "event_type": "milvus_search",
  "metadata": {
    "collection": "knowledge_text_chunks",
    "query": "华为电脑推荐",
    "filters": "brand == '华为' AND category_id == 'c_dataset_digital_laptop'",
    "top_n": 40,
    "duration_ms": 18,
    "hit_count": 6,
    "chunk_ids": ["p_digital_004_ck_faq_01"]
  }
}
```

商品搜索也记录：

```json
{
  "stage": "products.hybrid_search",
  "metadata": {
    "rds_hit_ids": ["p_digital_004"],
    "vector_hit_ids": ["p_digital_004"],
    "keyword_hit_ids": ["p_digital_004"],
    "rerank_top_ids": ["p_digital_004"]
  }
}
```

工具层 trace 建议按内部路径拆开：

```text
tools.search_products
  products.query_analyze
  products.rds_search
  products.vector_search
  products.keyword_search
  products.merge
  products.rerank
  products.hydrate
```

管理员页面展示时仍归属于一次 `search_products` 工具调用，但可以展开看每条内部路径。

## 12. 实施阶段

### Phase 1：基础设施

- 修改 `deployments/docker-compose.yml`，加入 Milvus standalone、etcd、minio。
- 增加 Nacos vector/embedding 配置。
- 增加 Milvus health check。

验收：

- 本地 `docker-compose up -d` 后 Milvus 可连通。
- 后端启动时可以检测 Milvus 状态。

### Phase 2：Schema 与索引

- 新增 `vector` 包。
- 初始化 `product_text_vectors`。
- 初始化 `knowledge_text_chunks`。

验收：

- collection 存在。
- index 创建成功。
- collection load 成功。

### Phase 3：商品向量入库

- 批量读取 MySQL active 商品。
- 构造商品检索文本。
- 调 embedding。
- upsert 到 `product_text_vectors`。

验收：

- 真实商品全部有向量。
- “华为电脑”“苹果电脑”“跑步鞋”等 query 能通过 Milvus 召回正确商品。

### Phase 4：知识向量入库

- 批量读取 `knowledge_chunks`。
- 补齐 metadata：product_id、brand、category_id、doc_type、source_type。
- 调 embedding。
- upsert 到 `knowledge_text_chunks`。

验收：

- 商品 FAQ、评价、说明可向量召回。
- “华为电脑协同能力”优先召回 MateBook 相关 chunk。

### Phase 5：混合召回接入

- `search_products` 工具内部接入 RDS + Milvus + keyword 混合召回。
- `search_knowledge` 工具内部接入 Milvus + keyword + rule 混合召回。
- `SearchProducts`、`SearchKnowledgeByPlan` 保持兼容，作为工具内部 service 的适配入口。
- 保留向量失败时的 MySQL 降级。

验收：

- Agent 工具返回稳定。
- trace 可看到 RDS、vector、keyword、rerank 各阶段结果。

### Phase 6：质量评测

增加检索评测集：

- 华为电脑推荐
- 苹果电脑推荐
- 华为手机推荐
- 平板电脑推荐
- 服饰运动分类筛选
- 品牌 + 价格 + 场景复合查询

指标：

- Recall@5
- Precision@5
- MRR
- facet violation rate
- no-result false negative rate

## 13. 关键验收用例

### 13.1 华为电脑

输入：

```text
华为电脑推荐
```

预期：

- 商品结果包含 `p_digital_004`。
- 不应返回华为手机、华为平板、华为耳机作为商品候选。
- 知识片段优先来自 `p_digital_004`。

### 13.2 苹果电脑

输入：

```text
苹果电脑推荐
```

预期：

- 返回 Apple MacBook。
- 不返回 iPhone。
- 不返回 iPad，除非用户明确说“平板”“iPad”。

### 13.3 华为办公生态

输入：

```text
华为电脑和平板怎么搭配办公
```

预期：

- 可以同时返回 MateBook 和 MatePad。
- 因为 query 明确包含“电脑和平板”，这里允许跨类目。

## 14. 风险与注意事项

1. Embedding 维度必须固定。
   - 更换 embedding 模型后，如果维度变化，需要重建 collection 或新建 collection。

2. Milvus 只返回候选，不做最终事实源。
   - 商品价格、库存、上下架必须回查 MySQL。

3. metadata filter 必须优先。
   - 否则语义相似会把“平板电脑”误召回到“电脑”场景。

4. 向量召回必须可降级。
   - Milvus 不可用时，系统仍能用 RDS + keyword 工作。

5. 不要把意图逻辑写进 RAG。
   - Agent 负责意图和 RetrievalPlan。
   - RAG 只负责执行计划。

## 15. 下一步

建议下一步先做：

1. 修改 docker-compose 接入 Milvus standalone。
2. 增加 `vector` 包和 Milvus 连接。
3. 初始化 `product_text_vectors`、`knowledge_text_chunks`。
4. 做商品批量向量入库。
5. 用“华为电脑推荐”“苹果电脑推荐”验证商品向量召回。
