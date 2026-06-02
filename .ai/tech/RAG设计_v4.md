# 电商 AI 导购 RAG 设计 v4

## 1. v4 调整目标

本文基于 [RAG设计_v3.md](./RAG设计_v3.md) 和 [v3_问题.md](./v3_问题.md) 修订。

v4 重点解决三个问题：

- 解释 `intent` 字段从哪里来、有什么用、可用取值是什么、RAG 中具体怎么用。
- 将向量数据库明确改为 Milvus，不再使用 Qdrant 作为 RAG 目标设计。
- 补足可落地实现细节，包括文本切片算法、Milvus collection 设计、混合召回流程、打分公式、阈值、评测指标和阶段实现边界。

本文只做设计，不直接实现代码。

## 2. 当前工程基线

当前 `main` 已有最小知识检索链路：

```text
用户发送消息
  -> backend/src/httpapi/server.go 创建 user_message 和 agent_run
  -> backend/src/agent/runtime.go 调用 store.SearchKnowledge(ctx, message.Content)
  -> backend/src/store/mysql.go 用 MySQL LIKE 检索 knowledge_chunks
  -> SSE block_delta 返回 Citation
```

当前代码接口：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

当前能力适合演示，但不够支撑正式 RAG：

- 没有结构化检索输入。
- 没有 `intent`。
- 没有 `filters`。
- 没有文本 embedding。
- 没有 Milvus。
- 没有混合召回和 rerank。
- `knowledge_chunks` 字段过少，无法承载元数据过滤。

v4 的落地策略是：

```text
先保留当前 SearchKnowledge(ctx, query string)
  -> 内部适配成结构化 KnowledgeSearchRequest
  -> 再逐步升级 Agent 工具调用到 search_knowledge(query, intent, filters, top_k)
```

## 3. intent 字段设计

### 3.1 intent 是什么

`intent` 是用户本轮问题的一级意图。

它不是 RAG 模块生成的字段，而是 Agent 上游 `Intent Router` 或 `Intent Classify LLM` 的输出。

在整个工程里，用户请求进入 Agent 后会经历：

```text
Message Intake
  -> Input Normalize
  -> Context Builder
  -> Query Rewriter
  -> Intent Router
  -> Tool Plan Builder
  -> search_knowledge
```

所以 RAG 收到的 `intent` 已经是上游识别好的结果。

### 3.2 已有文档哪里提到了 intent

已有设计中多处定义了 `intent`：

- [Agent框架设计_v2.md](./Agent框架设计_v2.md) 的 `2.6 意图枚举` 定义了一级意图和二级场景。
- [Agent框架设计_v2.md](./Agent框架设计_v2.md) 的 `5.1 意图到工具链映射` 定义了不同 intent 应调用哪些工具。
- [技术方案_v2.md](./技术方案_v2.md) 的 `6.2 Intent Classify LLM` 定义了意图识别 LLM 的输入输出。
- [技术方案_v2.md](./技术方案_v2.md) 的 `7. 意图、工具和 Prompt 路由` 定义了不同 intent 对应的工具链和 Prompt。
- [技术方案_v2.md](./技术方案_v2.md) 的 `9.5 search_knowledge` 已经把 `intent` 放入 RAG 工具输入。

因此，`intent` 是 Agent 总体框架已经预留的字段，不是 RAG v3 额外发明的字段。

### 3.3 intent 可用取值

一级意图沿用 Agent 框架 v2：

| intent | 说明 | 示例 |
| --- | --- | --- |
| `product_recommendation` | 商品推荐 | 预算 3000，推荐拍照手机 |
| `product_comparison` | 商品对比 | A 和 B 哪个更适合养宠家庭 |
| `product_detail_qa` | 商品参数/卖点咨询 | 这款鼠标支持蓝牙吗 |
| `promotion_rule_qa` | 优惠和营销规则咨询 | 满减和会员券能叠加吗 |
| `after_sales_qa` | 售后、保修、退换咨询 | 这个耳机支持 7 天无理由吗 |
| `image_product_recognition` | 图片识别商品 | 这张图里的鼠标是哪类产品 |
| `image_rule_explanation` | 图片活动规则解释 | 帮我看下这张海报优惠怎么算 |
| `shopping_decision_support` | 决策辅助 | 我该选便宜的还是性能更好的 |
| `general_shopping_chat` | 泛购物咨询 | 新手买咖啡机要注意什么 |
| `unsupported` | 非导购或暂不支持 | 查天气、写诗、无关闲聊 |

二级场景不放在 RAG 顶层 `intent` 中，可以放入 `filters.scenes` 或上游 slots：

```text
gift / for_elder / for_child / pet_family / student / office / gaming / travel / budget_sensitive / quality_first
```

### 3.4 intent 在 RAG 中有什么用

RAG 不用 `intent` 来生成回答，而是用它影响检索策略。

具体作用：

1. 推断默认 `doc_type`。
2. 决定不同知识来源的权重。
3. 决定召回方式是否需要规则补召回。
4. 决定 rerank 特征权重。
5. 决定无证据时的保护策略。

### 3.5 intent 到 doc_type 的映射

| intent | 默认 doc_type |
| --- | --- |
| `product_recommendation` | `product_detail`, `guide`, `review`, `faq`, `social_post` |
| `product_comparison` | `product_detail`, `review`, `guide`, `faq` |
| `product_detail_qa` | `product_detail`, `faq`, `review` |
| `promotion_rule_qa` | `promotion`, `faq` |
| `after_sales_qa` | `after_sales`, `faq` |
| `image_product_recognition` | 不默认查文本 RAG，优先图片向量检索 |
| `image_rule_explanation` | `promotion` |
| `shopping_decision_support` | `product_detail`, `guide`, `review`, `faq`, `social_post` |
| `general_shopping_chat` | `guide`, `faq` |
| `unsupported` | 不查 RAG 或只查安全兜底知识 |

如果调用方显式传入 `filters.doc_type`，以调用方传入值为准；否则由 `intent` 自动补默认值。

### 3.6 intent 到来源权重的映射

不同意图下，知识来源权重不同：

| intent | merchant | platform | external | 说明 |
| --- | ---: | ---: | ---: | --- |
| `product_detail_qa` | 1.0 | 0.7 | 0.4 | 商品事实优先商家资料 |
| `promotion_rule_qa` | 1.0 | 0.9 | 0.0 | 外部内容不能作为活动规则依据 |
| `after_sales_qa` | 1.0 | 0.9 | 0.0 | 外部内容不能作为售后承诺依据 |
| `product_recommendation` | 0.9 | 0.9 | 0.6 | 可综合选购指南和口碑 |
| `shopping_decision_support` | 0.9 | 0.9 | 0.7 | 外部体验可补充决策 |
| `general_shopping_chat` | 0.4 | 0.9 | 0.6 | 平台指南优先 |

其中：

- `merchant` 表示商家上传资料。
- `platform` 表示平台自建知识库。
- `external` 表示小红书、B 站、测评文章等外部内容。

### 3.7 search_knowledge 输入

沿用已有 `技术方案_v2.md` 的工具契约：

```json
{
  "query": "会员券能不能和满减一起用",
  "intent": "promotion_rule_qa",
  "filters": {
    "doc_type": ["promotion"],
    "product_ids": [],
    "category_id": "c_phone",
    "merchant_ids": ["m_001"],
    "source_type": ["merchant", "platform"],
    "effective_at": "2026-05-19T22:00:00+08:00",
    "min_quality_score": 0.5
  },
  "top_k": 8
}
```

说明：

- `query` 是 Agent 上游改写后的检索 query。字段名沿用技术方案，不改成 `query_text`。
- `intent` 是 Agent 上游识别出的一级意图。
- `filters` 是结构化过滤条件。
- `top_k` 是最终返回的证据数量。

当前代码还没有这个结构，所以需要兼容层：

```go
SearchKnowledge(ctx, query string)
  -> SearchKnowledgeV4(ctx, KnowledgeSearchRequest{
       Query: query,
       Intent: "general_shopping_chat",
       TopK: 5,
     })
```

## 4. 向量数据库：Milvus

### 4.1 选择 Milvus

v4 明确使用 Milvus 作为向量数据库。

原因：

- 比赛要求中明确提到向量数据库，Milvus 是典型选择。
- 现有 [技术方案_v2.md](./技术方案_v2.md) 的数据与知识层也提到 `Milvus：文本向量、图片向量`。
- Milvus 支持大规模向量检索、标量过滤、collection/schema 管理，适合后续扩展文本和图片向量。

需要同步调整：

- `deployments/docker-compose.yml` 后续应从 Qdrant 改为 Milvus standalone 或 Milvus lite/standalone 组合。
- 技术方案中仍出现 Qdrant 的段落，后续需要统一替换为 Milvus。
- 字段名 `vector_id` 可保留，不和具体向量数据库绑定。

### 4.2 Milvus collection 设计

使用两个 collection：

```text
knowledge_text_chunks
product_image_vectors
```

### 4.3 knowledge_text_chunks schema

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | 使用 `chunk_id` |
| `embedding` | FloatVector | 文本向量 |
| `chunk_id` | VarChar | chunk ID |
| `document_id` | VarChar | 文档 ID |
| `merchant_id` | VarChar | 商家 ID，可空 |
| `product_id` | VarChar | 商品 ID，可空 |
| `category_id` | VarChar | 类目 ID，可空 |
| `doc_type` | VarChar | 文档类型 |
| `source_type` | VarChar | 知识来源 |
| `quality_score` | Float | 质量分 |
| `effective_start_ts` | Int64 | 生效开始时间戳，可空用 0 |
| `effective_end_ts` | Int64 | 生效结束时间戳，可空用 0 |
| `created_at_ts` | Int64 | 创建时间戳 |

向量维度：

- 由实际 Embedding 模型决定，例如 768、1024 或 1536。
- 维度必须写入配置，不散落在代码中。

索引建议：

```text
index_type: HNSW
metric_type: COSINE
M: 16
efConstruction: 200
search ef: 64
```

如果 Milvus 部署版本或资源限制不适合 HNSW，可先用 IVF_FLAT：

```text
index_type: IVF_FLAT
metric_type: COSINE
nlist: 1024
search nprobe: 16
```

MVP 优先选择简单可跑的索引；评测稳定后再调参。

### 4.4 product_image_vectors schema

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | 使用 `image_vector_id` |
| `embedding` | FloatVector | 图片向量 |
| `image_vector_id` | VarChar | 图片向量 ID |
| `image_url` | VarChar | 原图 URL |
| `merchant_id` | VarChar | 商家 ID |
| `product_id` | VarChar | 商品 ID |
| `sku_id` | VarChar | SKU ID |
| `category_id` | VarChar | 类目 ID |
| `quality_score` | Float | 图片质量分 |
| `has_watermark` | Bool | 是否有水印 |
| `is_blurry` | Bool | 是否模糊 |
| `has_multiple_products` | Bool | 是否多主体 |
| `main_object_confidence` | Float | 主体置信度 |
| `created_at_ts` | Int64 | 创建时间戳 |

图片不做清洗：

- 不去水印。
- 不裁剪。
- 不修图。
- 只保存质量标记，并在排序中降权。

## 5. 文本切片实现

### 5.1 输入和输出

输入：

```go
type ParsedDocument struct {
    DocumentID string
    Title      string
    DocType    string
    SourceType string
    Content    string
    Sections   []ParsedSection
    Tables     []ParsedTable
}
```

输出：

```go
type KnowledgeChunk struct {
    ChunkID    string
    DocumentID string
    Title      string
    TitlePath  []string
    Content    string
    Snippet    string
    DocType    string
    SourceType string
    ProductID  string
    CategoryID string
    SortOrder  int
}
```

### 5.2 通用递归切片算法

参数：

```text
max_chars = 800
min_chars = 120
overlap_chars = 100
hard_max_chars = 1100
```

分隔符优先级：

```text
"\n# " / "\n## " / "\n### "
  -> "\n\n"
  -> "。|？|！|；"
  -> "，|、"
  -> fixed length
```

伪代码：

```text
split_document(doc):
  blocks = parse_by_heading(doc.content)
  chunks = []
  for block in blocks:
    if doc.doc_type == "faq":
      chunks += split_faq(block)
    else if block.is_table:
      chunks += split_table(block)
    else if doc.doc_type == "promotion":
      chunks += split_promotion(block)
    else:
      chunks += recursive_split(block.text, block.title_path)
  return post_process(chunks)

recursive_split(text, title_path):
  if len(text) <= max_chars:
    return [make_chunk(text, title_path)]

  sep = choose_separator(text)
  if sep == none:
    return fixed_split(text, max_chars, overlap_chars, title_path)

  parts = split(text, sep)
  merged = merge_until_limit(parts, max_chars, min_chars)

  result = []
  for item in merged:
    if len(item) > hard_max_chars:
      result += recursive_split_with_next_separator(item, title_path)
    else:
      result.append(make_chunk(item, title_path))
  return result
```

### 5.3 merge_until_limit 规则

```text
current = ""
for part in parts:
  if len(current) + len(part) <= max_chars:
    current += part
  else:
    if len(current) < min_chars and result not empty:
      result[-1] += current
    else:
      result.append(current)
    current = overlap_tail(current, overlap_chars) + part
append current
```

注意：

- overlap 只取句子边界附近的尾部，不在词中间硬截。
- overlap 内容不计入 `content_hash` 的唯一性判断时需要规范化，否则容易误判重复。

### 5.4 FAQ 切片

识别模式：

```text
Q:
A:
问：
答：
问题：
答案：
```

规则：

- 一问一答一个 chunk。
- 如果答案超过 `max_chars`，按段落递归切，但每个子 chunk 都保留问题。
- 标题格式：`{文档标题} / FAQ / {问题}`

### 5.5 表格切片

表格不直接按字符切。

规则：

- 每行补全表头。
- 同一商品参数表尽量保持在同一个 chunk。
- 如果表格太长，按商品、SKU、活动规则分组。

示例：

```text
表头：型号 | 存储 | 价格 | 适合人群
行：X Phone 12 | 256GB | 2999 | 拍娃、预算敏感
```

转换为：

```text
商品参数：型号=X Phone 12，存储=256GB，价格=2999，适合人群=拍娃、预算敏感。
```

### 5.6 活动规则切片

活动文档按规则块切：

```text
活动名称
  -> 适用商品/类目
  -> 门槛条件
  -> 优惠内容
  -> 叠加限制
  -> 生效时间
  -> 例外情况
```

同一条规则必须尽量在一个 chunk 中，避免把“可叠加”和“不可叠加”的例外拆开。

### 5.7 chunk 去重

规范化文本：

```text
normalize = trim + collapse_spaces + lowercase_ascii + remove_tracking_noise
content_hash = sha256(document_id + doc_type + normalized_content)
```

去重规则：

- 同一 document 内相同 hash 只保留一条。
- 外部内容跨 document 相同 hash，保留质量分最高或发布时间最新的一条。
- 商家资料和外部资料即使内容相似也不互相覆盖，只在检索时降重。

## 6. 文本召回实现

### 6.1 总流程

```text
KnowledgeSearchRequest
  -> normalize filters
  -> apply intent defaults
  -> query embedding
  -> Milvus vector search top_n=40
  -> MySQL keyword search top_n=40
  -> rule supplement recall top_n=20
  -> merge by chunk_id
  -> feature scoring
  -> rerank top_k
  -> evidence compress
```

默认参数：

```text
vector_top_n = 40
keyword_top_n = 40
rule_top_n = 20
rerank_top_k = request.top_k or 8
min_final_score = 0.35
```

### 6.2 Milvus 向量召回

Milvus 查询：

```text
collection: knowledge_text_chunks
vector field: embedding
metric: COSINE
limit: vector_top_n
expr:
  merchant_id in [...]
  and doc_type in [...]
  and category_id == ...
  and quality_score >= ...
  and effective_start_ts <= effective_at
  and (effective_end_ts == 0 or effective_end_ts >= effective_at)
```

Milvus 返回 distance 或 similarity。统一转为：

```text
vector_score = cosine_similarity in [0, 1]
```

如果客户端拿到的是 cosine distance：

```text
vector_score = 1 - distance
```

具体取值以 Milvus SDK 返回为准，封装在 `vector` 包里，不让业务层直接处理 distance。

### 6.3 关键词召回

MVP 可以用 MySQL `LIKE`，正式版建议 MySQL FULLTEXT 或独立倒排索引。

关键词字段：

- `title`
- `content`
- `snippet`
- `brand`
- `product_id`
- `doc_type`

Query 分词：

```text
保留原 query
提取型号：X Phone 12
提取数字和金额：3000、256GB、618
提取活动词：满减、会员券、叠加
提取类目词：手机、鼠标
```

关键词分数：

```text
keyword_score = min(1.0,
  0.35 * title_match
  + 0.25 * exact_term_match_ratio
  + 0.20 * bm25_norm
  + 0.10 * product_term_match
  + 0.10 * numeric_term_match
)
```

其中：

- `title_match` 命中标题为 1，否则 0。
- `exact_term_match_ratio = 命中的关键 token 数 / 关键 token 总数`。
- `bm25_norm` 是 BM25 分数归一化到 0 到 1。
- `product_term_match` 命中商品名、型号、商品 ID 为 1。
- `numeric_term_match` 命中价格、容量、日期等数字条件为 1。

MVP 没有 BM25 时：

```text
bm25_norm = exact_term_match_ratio
```

### 6.4 规则补召回

对于强规则意图，需要额外补召回：

| intent | 补召回 |
| --- | --- |
| `promotion_rule_qa` | `doc_type=promotion` 且命中活动名/券/满减/叠加 |
| `after_sales_qa` | `doc_type=after_sales` 且命中退换/保修/维修 |
| `image_rule_explanation` | `doc_type=promotion` 且结合图片解析出的活动词 |

规则补召回分数：

```text
rule_score = 1.0 if strong_rule_match else 0.0
```

强规则命中示例：

- query 包含“叠加”，chunk 同时包含“会员券”和“满减”。
- query 包含“7天无理由”，chunk 包含“7 天无理由”和“例外”。

### 6.5 合并去重

以 `chunk_id` 合并。

每个候选保存：

```json
{
  "chunk_id": "ck_001",
  "vector_score": 0.82,
  "keyword_score": 0.74,
  "rule_score": 1.0,
  "match_sources": ["vector", "keyword", "rule"]
}
```

如果同一文档连续多个 chunk 命中，保留：

- 分数最高 chunk。
- 前后相邻 chunk 各最多 1 个，作为上下文候选。

但最终给 LLM 的 evidence 仍要压缩，避免塞入过多邻居内容。

## 7. Rerank 和最终打分

### 7.1 特征

每个候选 chunk 计算以下特征：

| 特征 | 取值 | 说明 |
| --- | --- | --- |
| `vector_score` | 0-1 | 向量相似度 |
| `keyword_score` | 0-1 | 关键词/BM25 命中 |
| `rule_score` | 0-1 | 规则补召回命中 |
| `intent_doc_score` | 0-1 | doc_type 是否匹配 intent |
| `product_score` | 0-1 | product_id 是否匹配 |
| `category_score` | 0-1 | category_id 是否匹配 |
| `source_score` | 0-1 | 来源可信度 |
| `freshness_score` | 0-1 | 新鲜度/有效期 |
| `quality_score` | 0-1 | 文档质量 |
| `evidence_density` | 0-1 | 是否包含关键事实词 |

### 7.2 基础打分公式

默认：

```text
final_score =
  0.30 * vector_score
  + 0.20 * keyword_score
  + 0.10 * rule_score
  + 0.12 * intent_doc_score
  + 0.10 * product_score
  + 0.06 * category_score
  + 0.05 * source_score
  + 0.03 * freshness_score
  + 0.02 * quality_score
  + 0.02 * evidence_density
```

说明：

- 公式先用规则可解释版本，便于调试。
- 后续可以把候选 top 30 交给 rerank 模型，再用模型分替换或融合。

### 7.3 intent 专用权重

`promotion_rule_qa` 和 `after_sales_qa` 更重视规则、doc_type 和有效期：

```text
final_score =
  0.20 * vector_score
  + 0.20 * keyword_score
  + 0.18 * rule_score
  + 0.16 * intent_doc_score
  + 0.08 * product_score
  + 0.04 * category_score
  + 0.06 * source_score
  + 0.06 * freshness_score
  + 0.01 * quality_score
  + 0.01 * evidence_density
```

`product_recommendation` 和 `shopping_decision_support` 更重视语义、类目、外部体验：

```text
final_score =
  0.34 * vector_score
  + 0.16 * keyword_score
  + 0.04 * rule_score
  + 0.10 * intent_doc_score
  + 0.08 * product_score
  + 0.08 * category_score
  + 0.08 * source_score
  + 0.03 * freshness_score
  + 0.05 * quality_score
  + 0.04 * evidence_density
```

### 7.4 特征计算

`intent_doc_score`：

```text
1.0  doc_type 在 intent 默认 doc_type 中
0.6  doc_type 与 intent 弱相关
0.0  doc_type 不相关
```

`product_score`：

```text
1.0  chunk.product_id 命中 filters.product_ids
0.7  chunk 通过商品名/型号命中
0.4  query 未指定商品但类目相关
0.0  商品不匹配
```

`source_score`：

```text
source_weight(intent, source_type)
```

使用第 3.6 节的来源权重。

`freshness_score`：

```text
if effective_at 在有效期内: 1.0
else if 无有效期且 doc_type 不是 promotion: 0.7
else: 0.0
```

`evidence_density`：

```text
关键事实词命中数 / 关键事实词总数
```

关键事实词包括：

- 型号。
- 品牌。
- 价格。
- 容量。
- 活动名。
- 时间。
- 叠加/限制/例外。
- 售后关键词。

### 7.5 阈值

建议阈值：

```text
min_final_score = 0.35
high_confidence_score = 0.70
min_vector_score = 0.45
```

返回规则：

- final_score < 0.35 的 chunk 不返回。
- 如果 top1 < 0.35，认为 RAG 证据不足。
- 如果强规则问题没有命中 `merchant` 或 `platform` 来源，不输出确定规则结论。

## 8. 证据压缩

RAG 不应把完整 chunk 都塞给 LLM。

压缩策略：

```text
对每个 selected chunk:
  -> 按句子切分
  -> 保留命中 query 关键词或语义相关的句子
  -> 保留规则例外句
  -> 最多 3 句或 240 字
```

规则：

- `promotion_rule_qa` 必须保留“适用条件、叠加限制、有效期”。
- `after_sales_qa` 必须保留“服务范围、限制条件、例外情况”。
- `product_detail_qa` 必须保留“参数值、适用场景、风险提示”。
- 外部内容 citation 必须保留来源 URL 或平台名。

输出：

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
  ],
  "has_sufficient_evidence": true
}
```

## 9. 图片向量检索

图片继续沿用 v3 决策：不清洗，不去水印，不修图。

### 9.1 Milvus 图片检索流程

```text
image_similar_product_search
  -> image embedding
  -> Milvus product_image_vectors search top_n=40
  -> scalar filter category_id / merchant_id / quality_score
  -> group by product_id
  -> product service 回查商品状态
  -> 返回 top_k 商品候选
```

### 9.2 图片候选打分

```text
image_final_score =
  0.75 * image_similarity
  + 0.10 * category_score
  + 0.05 * merchant_score
  + 0.05 * quality_score
  + 0.05 * product_active_score
```

质量降权：

```text
if has_watermark: image_final_score -= 0.03
if is_blurry: image_final_score -= 0.08
if has_multiple_products: image_final_score -= 0.05
```

阈值：

```text
min_image_similarity = 0.55
min_image_final_score = 0.50
```

返回结果必须回查商品服务，不能直接从 Milvus payload 生成商品卡片。

## 10. MySQL 与 Milvus 数据一致性

MySQL 是元数据和业务事实的主库。

Milvus 只存向量和检索 payload。

写入顺序：

```text
写 knowledge_documents
  -> 写 knowledge_chunks(status=chunked)
  -> 生成 embedding
  -> upsert Milvus
  -> 更新 knowledge_chunks.embedding_status=indexed, vector_id=chunk_id
```

失败处理：

- embedding 失败：`embedding_status=failed`，记录错误。
- Milvus upsert 失败：可重试，不重复创建 chunk。
- 文档重建：旧 chunk 标记 inactive，新 chunk 重新入库。
- 删除文档：MySQL 标记 deleted，同时异步删除 Milvus 向量。

幂等键：

```text
chunk_id 作为 Milvus primary key
image_vector_id 作为图片 Milvus primary key
```

## 11. API 和代码落地

### 11.1 Store 接口演进

保留旧接口：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

新增服务层结构：

```go
type KnowledgeSearchRequest struct {
    Query   string                 `json:"query"`
    Intent  string                 `json:"intent"`
    Filters KnowledgeSearchFilters `json:"filters"`
    TopK    int                    `json:"top_k"`
    Debug   bool                   `json:"debug"`
}

type KnowledgeSearchFilters struct {
    DocTypes        []string  `json:"doc_type"`
    ProductIDs      []string  `json:"product_ids"`
    CategoryID      string    `json:"category_id"`
    MerchantIDs     []string  `json:"merchant_ids"`
    SourceTypes     []string  `json:"source_type"`
    EffectiveAt     time.Time `json:"effective_at"`
    MinQualityScore float64   `json:"min_quality_score"`
}
```

新增 service：

```go
SearchKnowledgeV4(ctx context.Context, req KnowledgeSearchRequest) (KnowledgeSearchResult, error)
```

### 11.2 Milvus 包

建议新增：

```text
backend/src/vector/
  milvus_client.go
  text_index.go
  image_index.go
```

职责：

- 封装 Milvus SDK。
- 创建 collection。
- upsert 向量。
- search 向量。
- 处理 distance 到 similarity 的转换。
- 不把 Milvus SDK 类型泄漏到 agent/store 层。

### 11.3 RAG 服务包

建议新增：

```text
backend/src/rag/
  service.go
  splitter.go
  cleaner.go
  retriever.go
  reranker.go
  compressor.go
  types.go
```

调用方向：

```text
agent.Runtime
  -> rag.Service
  -> store.Repository
  -> vector.MilvusClient
```

## 12. 评测指标

### 12.1 切片质量

| 指标 | 目标 |
| --- | --- |
| chunk 长度 P50 | 300 到 700 字 |
| chunk 超长率 | < 5% |
| chunk 过短率 | < 10% |
| 标题路径保留率 | > 95% |
| 表格表头保留率 | > 95% |
| FAQ 完整率 | > 95% |

### 12.2 召回质量

| 指标 | 目标 |
| --- | --- |
| Recall@5 | >= 80% |
| Recall@8 | >= 85% |
| MRR@8 | >= 0.65 |
| 规则问题 doc_type 命中率 | >= 90% |
| 商品问题 product_id 命中率 | >= 85% |
| 无证据识别率 | >= 80% |

### 12.3 图片检索质量

| 指标 | 目标 |
| --- | --- |
| Image Recall@5 | >= 80% |
| 类目过滤准确率 | >= 90% |
| 无关商品误召回率 | <= 10% |
| 商品服务回查成功率 | >= 95% |

### 12.4 回答质量

| 指标 | 目标 |
| --- | --- |
| 引用覆盖率 | >= 80% |
| 事实准确率 | >= 85% |
| 强事实幻觉率 | <= 5% |
| 外部内容误用为规则依据 | 0 |

## 13. 分期落地计划

### Phase 1：切片和 MySQL 混合检索

- 保留旧 `SearchKnowledge(ctx, query string)`。
- 实现 `rag.Splitter`。
- 上传资料时生成多个 chunk。
- MySQL LIKE 升级为 token-based keyword search。
- 增加 `intent` 默认 doc_type 映射，但先不接 LLM。

### Phase 2：结构化 search_knowledge 输入

- 实现 `KnowledgeSearchRequest`。
- Agent 工具调用改成 `query + intent + filters + top_k`。
- 管理端调试接口支持 debug。

### Phase 3：Milvus 文本向量

- Docker Compose 接入 Milvus。
- 实现 `vector.MilvusClient`。
- 文本 embedding 入 Milvus。
- 实现 Milvus 向量召回 + MySQL 关键词召回。

### Phase 4：规则 Rerank 和证据压缩

- 实现 final_score 公式。
- 实现 intent 专用权重。
- 实现 evidence compressor。
- 输出 Trace。

### Phase 5：图片向量检索

- 商品图片生成 image embedding。
- Milvus `product_image_vectors` 入库。
- 用户图片相似检索。
- 回查商品服务。

### Phase 6：外部知识库和评测闭环

- 外部内容离线导入。
- source_type 和 quality_score。
- RAG 评测集。
- 管理端查看 Recall、MRR、失败样例。

## 14. v3 问题逐条回应

### 14.1 intent 字段有什么用

`intent` 用于 RAG 检索策略，不用于回答生成。它决定默认 doc_type、来源权重、规则补召回、rerank 权重和无证据保护策略。

### 14.2 intent 在整体工程哪里提到

已有文档已定义：

- `Agent框架设计_v2.md` 的 `2.6 意图枚举`。
- `Agent框架设计_v2.md` 的 `5.1 意图到工具链映射`。
- `技术方案_v2.md` 的 `6.2 Intent Classify LLM`。
- `技术方案_v2.md` 的 `7. 意图、工具和 Prompt 路由`。
- `技术方案_v2.md` 的 `9.5 search_knowledge`。

### 14.3 intent 有哪些取值

v4 在第 3.3 节列出了完整取值，包括 `product_recommendation`、`product_comparison`、`product_detail_qa`、`promotion_rule_qa`、`after_sales_qa`、`image_product_recognition`、`image_rule_explanation`、`shopping_decision_support`、`general_shopping_chat`、`unsupported`。

### 14.4 使用 Milvus

已调整。v4 明确 Milvus 为目标向量数据库，并设计了 `knowledge_text_chunks` 和 `product_image_vectors` 两个 collection。

### 14.5 实现细节不足

已补充：

- 递归切片算法。
- FAQ、表格、活动规则专用切片。
- Milvus schema 和索引参数。
- 向量召回、关键词召回、规则补召回。
- keyword_score、final_score、image_final_score 公式。
- 阈值。
- 证据压缩。
- 数据一致性和幂等。
- 评测指标。
