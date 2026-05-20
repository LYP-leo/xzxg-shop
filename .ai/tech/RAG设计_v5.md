# 电商 AI 导购 RAG 设计 v5

## 1. v5 调整目标

本文基于 [RAG设计_v4.md](./RAG设计_v4.md) 和 [RAG_v4_问题.md](./RAG_v4_问题.md) 修订。

v5 的核心调整是重新划清 Agent 和 RAG 的职责边界：

- 意图理解、意图分类、意图到检索策略的决策都属于 Agent。
- RAG 不识别 intent，不维护 intent 枚举，不根据 intent 自动决定 doc_type、来源权重或规则补召回。
- RAG 只提供检索能力：文本检索、图片相似检索、过滤、合并、重排、证据压缩。
- Agent 调用 RAG 前，必须把意图结果转换成明确的检索计划 `RetrievalPlan`。
- RAG 执行 `RetrievalPlan`，不关心这个计划是由哪个 intent 推导出来的。

一句话：

```text
Agent 决定“查什么、按什么策略查”。
RAG 负责“把这些东西查出来，并按给定策略排序返回”。
```

本文只做设计，不直接实现代码。

## 2. 为什么要去掉 RAG 内部 intent 决策

v4 中虽然说明了 `intent` 来自 Agent，但仍在 RAG 文档中设计了：

- intent 到 doc_type 的映射。
- intent 到来源权重的映射。
- intent 专用打分公式。
- intent 触发规则补召回。

这会造成职责混乱。

问题在于：

- RAG 会开始理解业务语义，变成半个 Agent。
- 以后新增 intent 时，需要同时改 Agent 和 RAG。
- 检索策略散落在多个模块，Trace 和评测归因会变难。
- RAG 本应是通用检索服务，但会被电商意图强绑定。

v5 重新设计为：

```text
Intent Router
  -> Tool Plan Builder
  -> Retrieval Plan Builder
  -> RAG Retriever
```

其中：

- `Intent Router`：识别用户意图。
- `Retrieval Plan Builder`：根据意图、槽位、上下文生成检索计划。
- `RAG Retriever`：执行检索计划。

## 3. 模块边界

### 3.1 Agent 负责

Agent 负责所有“理解”和“决策”：

- Query 改写。
- 多轮上下文整理。
- 指代消解。
- 意图识别。
- 槽位抽取。
- 判断要不要查知识库。
- 判断查哪些 `doc_type`。
- 判断查哪些 `source_type`。
- 判断是否需要规则补召回。
- 判断不同召回特征权重。
- 判断无证据时回答策略。
- 最终回答生成。

Agent 输出给 RAG 的不是 intent，而是明确的检索计划。

### 3.2 RAG 负责

RAG 只负责检索执行：

- 文本知识清洗、切片、Embedding、入 Milvus。
- 图片语义向量入 Milvus。
- 根据检索计划执行文本向量召回。
- 根据检索计划执行关键词召回。
- 根据检索计划执行规则补召回。
- 执行 metadata filter。
- 合并去重。
- 按检索计划提供的权重重排。
- 证据压缩。
- 返回 citation 或图片相似商品候选。
- 记录检索过程 Trace。

RAG 不负责：

- 不识别 intent。
- 不知道所有 intent 取值。
- 不维护 intent 到 doc_type 的映射。
- 不维护 intent 到来源权重的映射。
- 不决定活动规则是否必须用商家资料。
- 不决定外部内容能不能作为回答依据。

这些都由 Agent 或上层业务策略决定。

## 4. 总体链路

### 4.1 文本问答链路

```text
用户问题
  -> Agent Context Builder
  -> Agent Query Rewriter
  -> Agent Intent Router
  -> Agent Slot Extractor
  -> Agent Retrieval Plan Builder
  -> RAG SearchKnowledge(plan)
  -> RAG 返回 evidence
  -> Agent Main Answer LLM
  -> SSE 输出文本、商品卡、citation
```

### 4.2 图片相似商品链路

```text
用户上传图片
  -> Agent parse_image / image understanding
  -> Agent Retrieval Plan Builder 生成图片检索计划
  -> RAG SearchSimilarProductsByImage(plan)
  -> RAG Milvus 图片向量召回
  -> RAG 返回 product_id 候选
  -> Agent 调商品服务补全商品卡
```

### 4.3 文档构建链路

```text
知识资料接入
  -> RAG 文本解析/清洗/切片
  -> MySQL knowledge_documents / knowledge_chunks
  -> Embedding
  -> Milvus knowledge_text_chunks
```

图片：

```text
商品图片接入
  -> RAG 图像语义理解
  -> RAG 图像 embedding
  -> Milvus product_image_vectors
```

## 5. RetrievalPlan 设计

v5 不再把 `intent` 作为 RAG 的行为输入。RAG 接收 `RetrievalPlan`。

### 5.1 文本 RetrievalPlan

```json
{
  "query": "会员券能不能和满减一起用",
  "filters": {
    "doc_types": ["promotion"],
    "product_ids": [],
    "category_ids": [],
    "merchant_ids": ["m_001"],
    "source_types": ["merchant", "platform"],
    "effective_at": "2026-05-19T22:00:00+08:00",
    "min_quality_score": 0.5
  },
  "recall": {
    "vector": {
      "enabled": true,
      "top_n": 40
    },
    "keyword": {
      "enabled": true,
      "top_n": 40,
      "required_terms": ["会员券", "满减", "叠加"]
    },
    "rule": {
      "enabled": true,
      "top_n": 20,
      "rule_terms": ["会员券", "满减", "叠加", "不可叠加"]
    }
  },
  "rerank": {
    "top_k": 8,
    "min_score": 0.35,
    "weights": {
      "vector_score": 0.20,
      "keyword_score": 0.20,
      "rule_score": 0.18,
      "doc_type_score": 0.16,
      "product_score": 0.08,
      "category_score": 0.04,
      "source_score": 0.06,
      "freshness_score": 0.06,
      "quality_score": 0.01,
      "evidence_density": 0.01
    }
  },
  "compress": {
    "enabled": true,
    "max_chars_per_chunk": 240,
    "keep_rule_exceptions": true
  },
  "trace": {
    "trace_id": "trace_001",
    "message_id": "msg_001",
    "reason": "Agent decided to search promotion rules"
  }
}
```

注意：

- `trace.reason` 可以记录 Agent 决策原因，但 RAG 不解析它。
- 如果上层想记录 intent，可以放在 `trace.agent_intent` 中，仅用于日志，不影响 RAG 行为。

### 5.2 图片 RetrievalPlan

```json
{
  "image": {
    "image_url": "/uploads/a.png",
    "image_vector": null,
    "visual_summary": "黑色无线人体工学鼠标，带侧键"
  },
  "filters": {
    "category_ids": ["c_mouse"],
    "merchant_ids": ["m_001"],
    "min_quality_score": 0.5
  },
  "recall": {
    "image_vector": {
      "enabled": true,
      "top_n": 40
    }
  },
  "rerank": {
    "top_k": 5,
    "min_score": 0.50,
    "weights": {
      "image_similarity": 0.75,
      "category_score": 0.10,
      "merchant_score": 0.05,
      "quality_score": 0.05,
      "product_active_score": 0.05
    }
  },
  "trace": {
    "trace_id": "trace_001",
    "message_id": "msg_001"
  }
}
```

## 6. Agent 如何从 intent 生成 RetrievalPlan

这部分属于 Agent，不属于 RAG，但为了说明集成关系，本文给出示例。

### 6.1 promotion_rule_qa 示例

用户问：

```text
会员券能不能和满减一起用？
```

Agent 识别：

```json
{
  "intent": "promotion_rule_qa",
  "slots": {
    "promotion_terms": ["会员券", "满减", "叠加"]
  }
}
```

Agent 生成 RetrievalPlan：

```json
{
  "query": "会员券能不能和满减一起用",
  "filters": {
    "doc_types": ["promotion"],
    "source_types": ["merchant", "platform"],
    "effective_at": "2026-05-19T22:00:00+08:00"
  },
  "recall": {
    "vector": { "enabled": true, "top_n": 40 },
    "keyword": {
      "enabled": true,
      "top_n": 40,
      "required_terms": ["会员券", "满减", "叠加"]
    },
    "rule": {
      "enabled": true,
      "top_n": 20,
      "rule_terms": ["会员券", "满减", "叠加", "不可叠加"]
    }
  },
  "rerank": {
    "top_k": 8,
    "weights": {
      "vector_score": 0.20,
      "keyword_score": 0.20,
      "rule_score": 0.18,
      "doc_type_score": 0.16,
      "freshness_score": 0.06,
      "source_score": 0.06
    }
  }
}
```

RAG 只执行这个计划，不知道 `promotion_rule_qa` 这个枚举。

### 6.2 product_recommendation 示例

用户问：

```text
预算 3000，推荐拍娃手机。
```

Agent 生成 RetrievalPlan：

```json
{
  "query": "3000 元以内适合拍摄儿童的手机，关注对焦、抓拍、存储、风险提示",
  "filters": {
    "doc_types": ["product_detail", "guide", "review", "faq", "social_post"],
    "category_ids": ["c_phone"],
    "source_types": ["merchant", "platform", "external_article", "external_xhs", "external_bilibili"],
    "min_quality_score": 0.5
  },
  "recall": {
    "vector": { "enabled": true, "top_n": 40 },
    "keyword": {
      "enabled": true,
      "top_n": 40,
      "required_terms": ["拍娃", "对焦", "抓拍", "3000"]
    },
    "rule": { "enabled": false }
  },
  "rerank": {
    "top_k": 8,
    "weights": {
      "vector_score": 0.34,
      "keyword_score": 0.16,
      "doc_type_score": 0.10,
      "category_score": 0.08,
      "source_score": 0.08,
      "quality_score": 0.05
    }
  }
}
```

### 6.3 after_sales_qa 示例

用户问：

```text
这个鼠标支持 7 天无理由吗？
```

Agent 生成 RetrievalPlan：

```json
{
  "query": "Quiet Mouse S 是否支持 7 天无理由退换货，有哪些例外条件",
  "filters": {
    "doc_types": ["after_sales", "faq"],
    "product_ids": ["p_mouse_001"],
    "source_types": ["merchant", "platform"]
  },
  "recall": {
    "vector": { "enabled": true, "top_n": 40 },
    "keyword": {
      "enabled": true,
      "top_n": 40,
      "required_terms": ["7天无理由", "退换", "例外"]
    },
    "rule": {
      "enabled": true,
      "top_n": 20,
      "rule_terms": ["7天无理由", "退换", "保修", "例外"]
    }
  },
  "rerank": {
    "top_k": 8,
    "min_score": 0.35
  }
}
```

## 7. RAG 内部类型

### 7.1 Go 类型建议

```go
type RetrievalPlan struct {
    Query    string
    Filters  RetrievalFilters
    Recall   RecallPlan
    Rerank   RerankPlan
    Compress CompressPlan
    Trace    RetrievalTraceContext
}

type RetrievalFilters struct {
    DocTypes        []string
    ProductIDs      []string
    CategoryIDs     []string
    MerchantIDs     []string
    SourceTypes     []string
    EffectiveAt     time.Time
    MinQualityScore float64
}

type RecallPlan struct {
    Vector  VectorRecallPlan
    Keyword KeywordRecallPlan
    Rule    RuleRecallPlan
}

type VectorRecallPlan struct {
    Enabled bool
    TopN    int
}

type KeywordRecallPlan struct {
    Enabled       bool
    TopN          int
    RequiredTerms []string
}

type RuleRecallPlan struct {
    Enabled   bool
    TopN      int
    RuleTerms []string
}

type RerankPlan struct {
    TopK     int
    MinScore float64
    Weights  map[string]float64
}

type CompressPlan struct {
    Enabled            bool
    MaxCharsPerChunk   int
    KeepRuleExceptions bool
}

type RetrievalTraceContext struct {
    TraceID   string
    MessageID string
    Reason    string
}
```

### 7.2 兼容当前代码

当前接口：

```go
SearchKnowledge(ctx context.Context, query string) []domain.Citation
```

兼容策略：

```go
func (s *Service) SearchKnowledge(ctx context.Context, query string) []domain.Citation {
    plan := DefaultRetrievalPlan(query)
    result, err := s.SearchKnowledgeByPlan(ctx, plan)
    if err != nil {
        return nil
    }
    return result.ToCitations()
}
```

默认计划：

```json
{
  "query": "用户原始输入",
  "filters": {},
  "recall": {
    "vector": { "enabled": false },
    "keyword": { "enabled": true, "top_n": 5 },
    "rule": { "enabled": false }
  },
  "rerank": {
    "top_k": 5,
    "min_score": 0
  }
}
```

这保证当前演示链路不被破坏。

## 8. Milvus 设计

v5 延续 v4：向量数据库使用 Milvus。

### 8.1 文本 collection

```text
knowledge_text_chunks
```

字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | `chunk_id` |
| `embedding` | FloatVector | 文本向量 |
| `chunk_id` | VarChar | chunk ID |
| `document_id` | VarChar | 文档 ID |
| `merchant_id` | VarChar | 商家 ID |
| `product_id` | VarChar | 商品 ID |
| `category_id` | VarChar | 类目 ID |
| `doc_type` | VarChar | 文档类型 |
| `source_type` | VarChar | 来源类型 |
| `quality_score` | Float | 质量分 |
| `effective_start_ts` | Int64 | 生效开始 |
| `effective_end_ts` | Int64 | 生效结束 |
| `created_at_ts` | Int64 | 创建时间 |

### 8.2 图片 collection

```text
product_image_vectors
```

字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `pk` | VarChar primary key | `image_vector_id` |
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
| `created_at_ts` | Int64 | 创建时间 |

图片仍然不做清洗，只做质量标记。

## 9. 文本切片

切片仍属于 RAG，因为它是知识库构建能力，不是意图理解。

### 9.1 参数

```text
max_chars = 800
min_chars = 120
overlap_chars = 100
hard_max_chars = 1100
```

### 9.2 分隔优先级

```text
标题
  -> 空行
  -> 句号/问号/感叹号/分号
  -> 逗号/顿号
  -> 固定长度
```

### 9.3 伪代码

```text
split_document(doc):
  blocks = parse_by_heading(doc.content)
  chunks = []
  for block in blocks:
    if doc.doc_type == "faq":
      chunks += split_faq(block)
    else if block.is_table:
      chunks += split_table(block)
    else:
      chunks += recursive_split(block.text, block.title_path)
  return post_process(chunks)
```

注意：

- `doc_type` 来自文档元数据，不是由 RAG 根据用户 intent 推断。
- 如果上传文档时没有 `doc_type`，由管理端或 Agent/后台导入流程补齐，不由 RAG 猜测。

## 10. 检索执行细节

### 10.1 流程

```text
RetrievalPlan
  -> 校验 plan
  -> 向量召回 enabled ? Milvus search : skip
  -> 关键词召回 enabled ? MySQL/FULLTEXT search : skip
  -> 规则召回 enabled ? rule term search : skip
  -> 按 filters 过滤
  -> 合并去重
  -> 特征打分
  -> 按 RerankPlan 权重排序
  -> 证据压缩
  -> 返回 RetrievalResult
```

### 10.2 特征

RAG 可以计算通用特征：

| 特征 | 说明 |
| --- | --- |
| `vector_score` | 向量相似度 |
| `keyword_score` | 关键词命中 |
| `rule_score` | 规则词命中 |
| `doc_type_score` | 是否匹配 filters.doc_types |
| `product_score` | 是否匹配 filters.product_ids |
| `category_score` | 是否匹配 filters.category_ids |
| `source_score` | 是否匹配 filters.source_types |
| `freshness_score` | 是否满足 effective_at |
| `quality_score` | 文档质量 |
| `evidence_density` | 关键事实词密度 |

这些特征是通用检索特征，不要求 RAG 理解 intent。

### 10.3 打分公式

RAG 不内置 intent 专用公式。RAG 使用 `RerankPlan.Weights`：

```text
final_score = sum(feature_i * weight_i)
```

如果调用方没有提供权重，使用通用默认权重：

```text
final_score =
  0.35 * vector_score
  + 0.25 * keyword_score
  + 0.05 * rule_score
  + 0.10 * doc_type_score
  + 0.08 * product_score
  + 0.05 * category_score
  + 0.04 * source_score
  + 0.03 * freshness_score
  + 0.03 * quality_score
  + 0.02 * evidence_density
```

如果 Agent 要让活动规则更重视 `rule_score` 和 `freshness_score`，应由 Agent 在 RetrievalPlan 中传入权重，而不是让 RAG 根据 intent 自己判断。

### 10.4 关键词分数

```text
keyword_score = min(1.0,
  0.35 * title_match
  + 0.25 * exact_term_match_ratio
  + 0.20 * bm25_norm
  + 0.10 * product_term_match
  + 0.10 * numeric_term_match
)
```

### 10.5 图片分数

图片检索也使用计划传入权重：

```text
image_final_score = sum(image_feature_i * image_weight_i)
```

默认：

```text
image_final_score =
  0.75 * image_similarity
  + 0.10 * category_score
  + 0.05 * merchant_score
  + 0.05 * quality_score
  + 0.05 * product_active_score
```

质量标记只降权，不清洗图片：

```text
if has_watermark: image_final_score -= 0.03
if is_blurry: image_final_score -= 0.08
if has_multiple_products: image_final_score -= 0.05
```

## 11. 输出

### 11.1 文本 RetrievalResult

```json
{
  "chunks": [
    {
      "chunk_id": "ck_001",
      "document_id": "doc_001",
      "title": "618 活动规则",
      "snippet": "会员券可与满减叠加，但不可与新人券叠加。",
      "score": 0.87,
      "metadata": {
        "doc_type": "promotion",
        "source_type": "merchant"
      },
      "features": {
        "vector_score": 0.74,
        "keyword_score": 0.82,
        "rule_score": 1.0
      }
    }
  ],
  "trace": {
    "vector_hits": 40,
    "keyword_hits": 23,
    "rule_hits": 8,
    "returned": 8
  }
}
```

### 11.2 SSE Citation 兼容

Agent 将 `RetrievalResult.chunks` 转成当前前端可渲染的 `Citation`：

```json
{
  "chunkId": "ck_001",
  "title": "618 活动规则",
  "snippet": "会员券可与满减叠加，但不可与新人券叠加。",
  "source": "doc_001"
}
```

RAG 不直接关心 SSE 格式。

## 12. 落地阶段

### Phase 1：保持当前演示链路

- 保留 `SearchKnowledge(ctx, query string)`。
- 增加 `DefaultRetrievalPlan(query)`。
- MySQL 关键词检索先跑通。
- Agent Runtime 暂不改。

### Phase 2：Agent 生成 RetrievalPlan

- Agent 中新增 `RetrievalPlanBuilder`。
- Intent Router 仍在 Agent 中。
- Agent 把 intent 转成 filters、recall、rerank weights。
- RAG 只接收 RetrievalPlan。

### Phase 3：Milvus 文本检索

- 部署 Milvus。
- 新增 `backend/src/vector`。
- chunk embedding 入 Milvus。
- RAG 支持 Milvus 向量召回。

### Phase 4：Rerank 和证据压缩

- RAG 计算通用特征。
- RAG 使用计划中的权重计算 final_score。
- RAG 实现证据压缩。

### Phase 5：图片检索

- 商品图片入 Milvus。
- 用户图片向量召回。
- RAG 返回 product_id 候选。
- Agent 回查商品服务并输出商品卡。

### Phase 6：评测

- 评测 Agent intent 准确率归 Agent。
- 评测 RAG Recall@K、MRR、证据质量归 RAG。
- 评测最终回答准确率归 Agent + RAG 联合链路。

## 13. v4 问题回应

### 13.1 intent 决策不该由 RAG 负责

已调整。

v5 中，RAG 不再根据 intent 做任何决策。RAG 不维护 intent 枚举，也不维护 intent 到 doc_type、source_type、权重的映射。

### 13.2 Agent 与 RAG 新分工

新分工：

```text
Agent:
  理解用户、识别 intent、抽取槽位、生成 RetrievalPlan、生成最终回答

RAG:
  执行 RetrievalPlan、召回证据、排序、压缩、返回结果
```

### 13.3 intent 还会不会进入 RAG

默认不进入 RAG 行为输入。

如果为了日志或调试需要，可以放在：

```json
{
  "trace": {
    "agent_intent": "promotion_rule_qa"
  }
}
```

RAG 只记录它，不使用它改变检索行为。
