# RAG 测评方案调研报告 v1

更新时间：2026-05-23

## 结论摘要

当前 `quality/data/eval/rag_recall_cases.jsonl` 的 `keyword` 过长，包含品牌、类目、完整商品标题和 FAQ 问句，实际是在做“精确商品回查”。这会让测评结果虚高，不能反映用户真实搜索“薇诺娜 面霜”“敏感肌面霜”“屏障修护”时，RAG 是否能召回正确商品和知识片段。

RAG 测评应该拆成三层：

1. 检索层：输入真实用户 query，评估召回片段是否相关、排序是否靠前。
2. 生成层：评估回答是否基于召回上下文、是否答到用户问题。
3. Agent 链路层：评估是否调用了正确 tool、参数是否合理、最终目标是否达成。

本项目下一步应把 RAG recall 测试数据从“长标题精确匹配”改成“用户自然短 query + 标注相关商品/片段”的 golden set。

## 为什么当前 keyword 不合理

示例：

```json
{
  "keyword": "薇诺娜 面霜 薇诺娜舒敏保湿特护霜敏感肌修护屏障舒缓干痒保湿面霜50g 这款特护霜能改善哪些敏感肌常见问题？"
}
```

问题：

1. 信息泄漏：完整商品名和 FAQ 原文已经几乎定位到唯一 chunk。
2. 不像真实用户：真实用户更可能输入“薇诺娜 面霜”“敏感肌修护面霜”“脸干痒用什么面霜”。
3. 无法测排序能力：标题命中太强，RDS LIKE、向量召回、重排都容易通过。
4. 无法暴露泛化问题：用户同义词、场景词、模糊需求、预算约束、否定条件都没有覆盖。

因此，RAG 的检索输入不应默认等于商品知识库中的完整文本。测试 query 应该来自用户语言，标注结果才指向商品 ID、chunk ID 或证据集合。

## 行业常见 RAG 测评拆分

### 1. Retrieval Evaluation

LlamaIndex 的检索测评把 retriever 单独拿出来评，常用指标包括 hit rate、MRR、precision、recall、AP、NDCG。核心思想是：给定问题和 ground-truth context，比较检索结果是否命中以及排序是否合理。

适合本项目的指标：

- Hit@K：Top K 是否包含任一相关商品或 chunk。
- Recall@K：相关商品/片段中有多少被召回。
- MRR：第一个相关结果排在第几位。
- NDCG@K：多相关结果时，排序质量是否合理。
- Source Hit@K：是否召回期望商品 ID，例如 `p_beauty_004`。
- Chunk Hit@K：是否召回期望 chunk ID，例如 `p_beauty_004_ck_marketing`。

### 2. RAG End-to-End Evaluation

LangSmith 的 RAG 测评把流程拆成数据集、运行应用、评估输出三步，评估维度包括：

- correctness：回答相对参考答案是否正确。
- relevance：回答是否回应用户输入。
- groundedness：回答是否被召回上下文支撑。
- retrieval relevance：召回内容是否和 query 相关。

这说明 RAG 不应该只测“搜到了没有”，还要测“搜到后有没有正确使用”。

### 3. RAGAS / LLM-as-Judge 指标

RAGAS 把 RAG 质量拆成检索质量和生成质量。常见指标包括：

- Context Precision：相关上下文是否排在不相关上下文前面。
- Context Recall：上下文是否包含回答问题所需信息。
- Answer Relevancy：回答是否贴合问题。
- Faithfulness / Response Groundedness：回答是否忠实于上下文，是否幻觉。
- Noise Sensitivity：无关上下文混入时，回答是否受干扰。

对本项目而言，RAGAS 类指标适合放在第二阶段，因为它需要 LLM judge，会带来成本和稳定性问题。第一阶段应先把 deterministic 的检索指标打牢。

### 4. Agent + Tool 测评

现在我们的导购不是纯 RAG，而是 Agent 调 tool。MLflow/RAGAS 类文档也把 Agent 行为单独作为评估对象，例如 tool call accuracy、tool call F1、goal accuracy。

本项目需要补的指标：

- ToolCallAccuracy：用户问商品知识时是否调用 `search_knowledge`。
- ToolArgQuality：传给工具的 query 是否合理，例如“薇诺娜 面霜”，而不是用户整句无清洗或完整 prompt。
- ToolResultUsefulness：工具返回内容是否足够支撑回答。
- FinalGoalAccuracy：最终是否完成推荐、对比、解释、售后问答等目标。

## 推荐测试集分层

### A. RAG 检索 golden set

目标：只测 search tool / RAG retriever，不经过主 Agent。

字段建议：

```json
{
  "id": "rag_recall_001",
  "query": "薇诺娜 面霜",
  "intent": "product_lookup",
  "query_type": "brand_category",
  "expected_product_ids": ["p_beauty_004"],
  "expected_chunk_ids": ["p_beauty_004_ck_marketing"],
  "must_include_terms": ["敏感肌", "修护", "保湿"],
  "top_k": 10
}
```

query 类型分布建议：

| 类型 | 占比 | 示例 |
| --- | ---: | --- |
| 品牌 + 品类 | 20% | 薇诺娜 面霜、苹果电脑 |
| 场景需求 | 20% | 敏感肌修护面霜、通勤轻薄电脑 |
| 痛点/功效 | 20% | 脸干痒用什么、想淡纹抗初老 |
| 属性约束 | 15% | 预算 5000 的笔记本、256G 苹果手机 |
| 对比型 | 10% | 小棕瓶和小黑瓶区别 |
| 否定/排除 | 5% | 不要酒精的护肤品 |
| 口语/错别字/别名 | 5% | 苹果本、华为电脑、敏感肌霜 |
| 多轮改写后 query | 5% | “那有没有便宜点的”改写为完整 query |

### B. Query Rewrite 测试集

目标：测用户原话到 search query 的改写质量。

示例：

```json
{
  "id": "rewrite_001",
  "history": [],
  "user_query": "薇诺娜那个修护面霜怎么样",
  "expected_search_query": "薇诺娜 修护 面霜 敏感肌 屏障修护",
  "forbidden_terms": ["完整商品标题"]
}
```

这可以直接解决当前问题：Agent/RAG 工具拿到的应该是短而信息密度高的检索 query，而不是完整商品文本。

### C. RAG 回答测评集

目标：经过 RAG 召回 + 回答生成，评估最终回答。

字段建议：

```json
{
  "id": "rag_answer_001",
  "query": "薇诺娜特护霜适合敏感肌吗",
  "reference_answer_points": [
    "适合敏感肌屏障受损、干痒泛红场景",
    "需要提醒先局部测试",
    "不能承诺治疗皮肤病"
  ],
  "expected_product_ids": ["p_beauty_004"]
}
```

### D. Agent E2E 测试集

目标：测真实导购链路。

字段建议：

```json
{
  "id": "agent_e2e_001",
  "messages": ["我脸容易泛红，想买个面霜"],
  "expected_tools": ["search_knowledge", "search_products"],
  "expected_product_ids": ["p_beauty_004"],
  "judge_points": [
    "识别敏感肌场景",
    "给出商品推荐理由",
    "提醒先做局部测试"
  ]
}
```

## 本项目推荐指标

第一阶段先落地可稳定自动化的指标：

| 层级 | 指标 | 是否需要 LLM Judge | 用途 |
| --- | --- | --- | --- |
| RAG 检索 | Product Hit@K | 否 | 是否召回目标商品 |
| RAG 检索 | Chunk Hit@K | 否 | 是否召回目标证据 |
| RAG 检索 | MRR | 否 | 目标结果排序位置 |
| RAG 检索 | NDCG@K | 否 | 多目标相关排序 |
| Query Rewrite | 关键词覆盖率 | 否 | 改写是否保留核心意图 |
| Query Rewrite | 禁止泄漏检查 | 否 | 是否拼入完整商品标题 |
| Agent Tool | Tool Call Accuracy | 否/半自动 | 是否调用正确 tool |
| Agent E2E | Goal Pass/Fail | 是 | 是否完成用户目标 |
| RAG Answer | Faithfulness | 是 | 是否基于证据回答 |

第二阶段再接 LLM judge：

- faithfulness
- answer relevancy
- context precision
- context recall
- final answer correctness

## 改造建议

### 1. 立即修正 RAG recall 数据

把现在 100 条长 keyword 改成短 query，每个商品至少 3 类 query：

1. 品牌 + 品类：`薇诺娜 面霜`
2. 场景/痛点：`敏感肌脸干痒用什么面霜`
3. 功效/属性：`修护屏障保湿面霜`

不要把完整商品标题和 FAQ 原句拼进 query。

### 2. 测评脚本改名

当前字段叫 `keyword`，建议改为：

- `query`：用户检索输入。
- `expected_product_ids`：期望召回商品。
- `expected_chunk_ids`：期望召回证据。
- `query_type`：用于按类型统计短板。

### 3. 报告按 query_type 分组

报告不要只给总命中率，应展示：

- brand_category Hit@K
- scenario_need Hit@K
- pain_point Hit@K
- compare Hit@K
- negative_constraint Hit@K
- alias_typo Hit@K

这样能直接看出“苹果电脑”“华为电脑”“敏感肌面霜”这类问题到底是哪一类弱。

### 4. 管理员页面详情继续增强

现有详情页已经能展示逐条 case。下一步应增加：

- 按 query_type 过滤。
- 只看失败样本。
- 展示 TopK 排名位置。
- 展示 RDS 召回、Milvus 召回、重排后的结果三列。
- 对每条失败样本给失败类型：未召回、召回但排序低、召回错品类、query 改写失败。

## 推荐落地顺序

1. 重生成 RAG recall 100 条短 query 数据。
2. 修改测评脚本支持 `query`、`expected_product_ids`、`expected_chunk_ids`、`query_type`。
3. 增加 Product Hit@K、Chunk Hit@K、MRR、按 query_type 分组统计。
4. 管理员详情页增加失败过滤和排名展示。
5. 再做 Query Rewrite 测评集。
6. 最后接 LLM judge 做 faithfulness / answer relevancy。

## 2026-05-23 落地记录

已完成第一阶段改造：

1. `quality/evals/generate_rag_recall_cases.mjs` 已改为生成短 query，不再拼完整商品标题和 FAQ 原句。
2. `quality/data/eval/rag_recall_cases.jsonl` 字段改为 `query`、`query_type`、`expected_product_ids`、`expected_chunk_ids`。
3. `quality/evals/run_rag_recall_eval.mjs` 已支持 Product Hit@K、Chunk Hit@K、MRR、按 query_type 分组统计。
4. 测评脚本仍调用 `/api/v1/eval/rag`，定位是 RAG 检索组件测评，不代表完整线上 Agent 链路。
5. 管理员测评详情页已展示 query_type、期望商品、期望 chunk、命中排名和按 query_type 的聚合指标。

尚未完成：

1. Query Rewrite 单独测评集。
2. 从真实 Agent trace 做 tool call eval。
3. LLM judge 的 faithfulness / answer relevancy。

## 参考资料

- LlamaIndex Retrieval Evaluation: https://developers.llamaindex.ai/python/examples/evaluation/retrieval/retriever_eval/
- LangSmith RAG Evaluation: https://docs.langchain.com/langsmith/evaluate-rag-tutorial
- RAGAS Tutorial: https://docs.ragas.io/en/v0.3.3/tutorials/rag/
- MLflow RAGAS Scorers: https://learn.microsoft.com/en-us/azure/databricks/mlflow3/genai/eval-monitor/third-party-scorers/ragas
- RAGAS paper: https://arxiv.org/abs/2309.15217
- ARES paper: https://arxiv.org/abs/2311.09476
