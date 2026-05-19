# 电商 AI 导购 Agent 框架设计 v1

## 1. 设计目标

本方案在现有 PRD 和参考 Agent 主链路图基础上，重新设计第一版可落地的 Agent 框架。目标不是复刻复杂平台级导购系统，而是在比赛周期内实现一条稳定、可演示、可评测、可持续优化的核心链路：

```text
多模态输入 -> 意图与约束理解 -> RAG 与商品检索 -> 决策生成 -> 流式 UI 输出 -> Trace 与评测反馈
```

Agent 需要解决四个问题：

- 理解用户真实购买意图，而不是只做普通聊天。
- 基于商品库和知识库回答，关键事实不靠模型编造。
- 输出既包含自然语言，也包含客户端可渲染的商品卡片、对比表、引用来源和追问建议。
- 每次回答都留下可评测、可回放、可优化的过程数据。

## 2. 对参考链路的裁剪

参考图覆盖了成熟业务系统中的很多平台能力，适合大规模导购场景，但对当前比赛 MVP 来说会拉高实现成本。第一版需要保留核心能力，裁剪平台化分支。

### 2.1 保留

保留以下模块：

- 请求入口校验、限流、会话记录。
- 会话历史与短期记忆读取。
- Query 改写。
- 意图分类。
- 用户约束抽取。
- 商品相关性判断。
- RAG 检索。
- 商品检索与商品卡片组装。
- 商品对比。
- 图片 OCR/视觉解析。
- 主 Agent 答案生成。
- SSE 流式输出。
- 会话后处理与评测日志。

这些模块直接对应比赛要求中的意图理解、智能咨询、决策辅助、RAG、多模态、流式交互和评测闭环。

### 2.2 裁剪

第一版不实现以下模块：

- 多套 Brain/Diamond 动态策略配置。
- 多业务线 SkillHandlerFactory。
- 非购物场景下的大量垂直 Skill，例如取件码、物流监控、商品问答以外的泛业务技能。
- 多层 Listener 和 FormatHandler 链式分发。
- 复杂 ReAct 循环和模型自主选择任意工具。
- 独立的 GUI Agent 流程。
- 长期用户画像系统。
- 多 Agent 并行协作框架。

裁剪原因：

- 当前项目的关键价值是 AI 导购闭环，不是通用智能客服平台。
- 第一版需要可控、可调试、可评测，显式编排优于完全交给模型自主规划。
- 比赛演示更看重端到端效果、引用依据和交互体验，复杂策略平台的收益有限。

### 2.3 补充

参考图中对以下比赛必需能力表达不够，需要补充：

- 知识库构建结果和 Agent 检索链路之间的明确契约。
- 商品服务和大模型之间的事实边界：价格、库存、参数、活动规则必须来自商品库或知识库。
- 面向客户端的结构化 UI 输出协议。
- 评测模式与线上问答共用同一套 Agent 链路。
- Agent Trace 数据模型，用于定位失败原因。
- 无证据、低置信度、工具失败时的降级策略。

## 3. 总体框架

第一版 Agent 采用“确定性编排 + 局部 LLM 能力”的框架。

```text
Chat API
  -> Agent Runtime
      -> Input Normalizer
      -> Context Builder
      -> Intent & Slot Analyzer
      -> Plan Builder
      -> Tool Executor
      -> Decision Generator
      -> UI Event Composer
      -> Trace Recorder
```

模块职责：

| 模块 | 职责 | 是否调用 LLM |
| --- | --- | --- |
| Input Normalizer | 整理文本、图片、附件和客户端参数 | 图片解析时调用 Vision/OCR |
| Context Builder | 读取会话历史、摘要、上一轮推荐商品、短期偏好 | 否 |
| Intent & Slot Analyzer | 识别意图、抽取预算/类目/品牌/场景/约束/缺失信息 | 是 |
| Plan Builder | 根据意图和槽位决定需要调用哪些工具 | 否，第一版规则驱动 |
| Tool Executor | 执行 RAG、商品检索、商品对比、图片解析 | 部分工具调用 LLM |
| Decision Generator | 基于证据和商品数据生成导购回答 | 是 |
| UI Event Composer | 将回答结果转换为 SSE 事件 | 否 |
| Trace Recorder | 记录意图、工具输入输出、检索命中、Prompt 版本和耗时 | 否 |

核心原则：

- LLM 负责理解、生成和归纳，不负责直接决定关键事实。
- 工具调用由 Go 代码显式编排，保证稳定性和可测试性。
- Agent 输出先形成结构化结果，再映射为客户端 SSE 事件。
- 评测任务和用户真实对话复用同一套 Agent Runtime。

## 4. Agent 分层设计

### 4.1 Runtime 层

Runtime 是 Agent 的主控制器，对外提供统一入口：

```text
RunChat(ctx, request) -> event stream
RunEval(ctx, evalCase) -> eval result
```

职责：

- 管理一次 Agent 执行生命周期。
- 生成 `trace_id`。
- 控制超时和取消。
- 串联上下文构建、意图识别、工具执行、答案生成和事件输出。
- 统一捕获错误并转为降级响应。

### 4.2 Understanding 层

负责把用户输入转为结构化语义。

输入：

- 当前用户文本。
- 图片解析结果。
- 最近 N 轮对话。
- 会话摘要。
- 上一轮推荐商品。

输出：

```json
{
  "intent": "product_recommendation",
  "confidence": 0.86,
  "slots": {
    "category": "手机",
    "budget_max": "3000",
    "usage_scene": ["拍娃"],
    "brand_preference": [],
    "must_have": ["拍照", "对焦快"],
    "avoid": []
  },
  "missing_slots": ["storage_preference"],
  "need_clarification": false,
  "rewritten_query": "预算3000元以内，适合拍娃、拍照对焦快的手机推荐"
}
```

说明：

- `intent` 决定主流程。
- `slots` 决定商品检索过滤条件和回答重点。
- `missing_slots` 用于判断是否需要追问。
- `rewritten_query` 用于 RAG 检索。

### 4.3 Planning 层

第一版不让模型自由规划工具，使用规则生成 Plan。

Plan 示例：

```json
{
  "intent": "product_recommendation",
  "steps": [
    {
      "tool": "search_products",
      "reason": "根据预算、类目和使用场景召回候选商品"
    },
    {
      "tool": "search_knowledge",
      "reason": "检索商品详情和导购资料作为推荐依据"
    },
    {
      "tool": "rank_products",
      "reason": "结合用户约束和证据排序"
    },
    {
      "tool": "generate_answer",
      "reason": "生成自然语言回答和结构化 UI 数据"
    }
  ]
}
```

不同意图对应的默认 Plan：

| 意图 | 默认工具链 |
| --- | --- |
| `product_recommendation` | `search_products` -> `search_knowledge` -> `rank_products` -> `generate_answer` |
| `product_comparison` | `resolve_products` -> `compare_products` -> `search_knowledge` -> `generate_answer` |
| `product_attribute_qa` | `resolve_products` -> `search_knowledge` -> `generate_answer` |
| `promotion_qa` | `search_knowledge` with `doc_type=promotion` -> `generate_answer` |
| `after_sales_qa` | `search_knowledge` with `doc_type=after_sales` -> `generate_answer` |
| `image_understanding` | `parse_image` -> `intent_analyze` -> follow normal plan |
| `clarification` | `generate_clarification` |
| `general_chat` | `generate_safe_chat` |
| `fallback` | `generate_fallback` |

### 4.4 Tool 层

第一版只保留导购必要工具。

#### parse_image

用途：

- 识别商品图、活动海报、购物车截图。
- 提取商品名、品牌、价格、活动文案、图片中的用户意图线索。

输出：

```json
{
  "detected_text": "满3000减300，会员券可叠加",
  "detected_products": ["X Phone 12"],
  "visual_summary": "图片疑似手机促销海报",
  "confidence": 0.78
}
```

#### search_products

用途：

- 根据类目、预算、品牌、标签、库存状态召回候选商品。
- 返回商品事实数据，作为卡片和推荐依据。

注意：

- 商品价格、库存、图片、参数优先来自商品服务。
- 模型只能解释推荐理由，不能编造商品字段。

#### resolve_products

用途：

- 将用户说的“第一款”“这两个”“刚才那个小米”解析为商品 ID。
- 优先从当前会话上下文和最近推荐商品中解析。

#### compare_products

用途：

- 对 2 到 4 个商品做结构化对比。
- 输出差异字段、适合人群、不适合情况和推荐结论。

#### search_knowledge

用途：

- 从商品详情、营销文档、售后政策、FAQ、导购话术中检索证据。
- 支持向量检索、关键词召回、结构化过滤和重排。

输出：

```json
{
  "query": "会员券和满减是否可以叠加",
  "results": [
    {
      "chunk_id": "ck_001",
      "document_id": "doc_001",
      "title": "618 活动规则",
      "snippet": "会员券可与满减叠加，但不可与新人券叠加。",
      "score": 0.87,
      "metadata": {
        "doc_type": "promotion",
        "source_page": 2
      }
    }
  ]
}
```

#### rank_products

用途：

- 根据用户约束、商品事实、知识片段证据对候选商品排序。
- 第一版可使用规则打分加 LLM 解释，不做复杂推荐算法。

建议规则：

- 硬约束不满足直接降权或过滤，例如超预算、无库存。
- 使用场景命中加分，例如拍娃、养宠、送礼、学生。
- 有知识片段证据支持的卖点加分。
- 风险项命中降权，例如用户强调轻薄但商品偏重。

#### generate_answer

用途：

- 基于工具结果生成最终回答。
- 输出自然语言和结构化 UI 数据。

要求：

- 引用证据必须来自 `search_knowledge`。
- 商品卡片字段必须来自 `search_products`。
- 不确定时明确说明原因。
- 如果信息不足，优先追问最关键的 1 到 2 个问题。

### 4.5 Generation 层

Generation 层输出统一结构，不直接面向客户端。

```json
{
  "answer_text": "如果预算控制在3000以内，并且重点是拍娃，我建议优先看 X Phone 12...",
  "intent": "product_recommendation",
  "answer_type": "recommendation",
  "confidence": 0.82,
  "products": [
    {
      "product_id": "p_001",
      "recommend_rank": 1,
      "reason": "预算内，拍照对焦快，适合抓拍儿童动态场景",
      "risk_notes": ["续航表现不是最强"]
    }
  ],
  "comparison": null,
  "citations": [
    {
      "chunk_id": "ck_001",
      "title": "X Phone 12 商品详情",
      "supporting_claim": "支持高速对焦和儿童抓拍模式"
    }
  ],
  "follow_up_questions": [
    "你更重视拍照还是续航？",
    "是否需要 256GB 以上存储？"
  ],
  "safety_flags": []
}
```

## 5. 核心执行流程

### 5.1 用户对话流程

```text
1. API 接收用户消息
2. 创建 user message 和 assistant placeholder
3. 返回 message_start SSE 事件
4. Input Normalizer 处理文本和附件
5. 如有图片，调用 parse_image
6. Context Builder 读取历史消息、摘要、短期记忆、最近商品
7. Intent & Slot Analyzer 输出意图、槽位、改写 Query
8. 如果必须澄清，直接生成澄清问题
9. Plan Builder 根据意图生成工具调用计划
10. Tool Executor 执行商品检索、RAG、对比等工具
11. Decision Generator 生成结构化回答
12. UI Event Composer 发送 text_delta、product_cards、comparison_table、citations、followups
13. 保存 assistant message 和 trace
14. 投递 chat.postprocess 异步任务
15. 返回 message_end
```

### 5.2 澄清策略

不是所有缺失槽位都要追问。第一版只在缺失信息会显著影响结果时追问。

需要追问：

- 用户只说“推荐一款手机”，没有预算、用途、偏好。
- 用户要求冲突，例如“最便宜但拍照最强且旗舰性能”。
- 对比对象不明确，例如“这两个哪个好”，但上下文没有商品。

不需要追问：

- 用户已给出预算和使用场景，只缺少品牌偏好。
- 能先给出 2 到 3 个候选并说明取舍。
- 用户问明确事实，例如“会员券能不能和满减叠加”。

澄清问题数量：

- 一次最多问 2 个问题。
- 优先问影响排序最大的约束，例如预算、类目、使用场景。

### 5.3 无证据策略

对关键事实必须执行无证据保护：

| 问题类型 | 无证据时处理 |
| --- | --- |
| 商品价格 | 不输出确定价格，只说明当前商品库无价格信息 |
| 库存 | 不输出有货/无货结论 |
| 活动叠加 | 不判断可叠加，建议查看活动规则或补充活动截图 |
| 售后政策 | 不编造政策，提示需要官方政策依据 |
| 参数规格 | 不输出确定参数，可说明知识库未收录 |

## 6. 与 RAG 的关系

Agent 不直接关心文档解析细节，只依赖知识检索契约。

### 6.1 知识检索输入

```json
{
  "query": "预算3000以内适合拍娃的手机",
  "intent": "product_recommendation",
  "filters": {
    "doc_type": ["product_detail", "guide", "faq"],
    "category": "手机",
    "product_ids": ["p_001", "p_002"]
  },
  "top_k": 8,
  "rerank": true
}
```

### 6.2 知识检索输出

```json
{
  "results": [
    {
      "chunk_id": "ck_001",
      "document_id": "doc_001",
      "title": "X Phone 12 拍照能力说明",
      "content": "X Phone 12 支持儿童抓拍模式和高速对焦...",
      "score": 0.89,
      "metadata": {
        "product_id": "p_001",
        "doc_type": "product_detail",
        "source_page": 1,
        "version": "2026-05"
      }
    }
  ]
}
```

### 6.3 证据使用规则

- 每个核心推荐理由尽量关联至少一个 chunk。
- 活动规则、售后政策必须有对应 chunk 才能回答确定结论。
- 引用展示给用户时使用短 snippet，不展示完整原文。
- 检索结果进入 Prompt 前按 token 预算裁剪。

## 7. 客户端流式输出协议

Agent 内部统一输出 `AgentResponse`，再转成 SSE。

推荐 SSE 顺序：

```text
message_start
thinking_status
text_delta
product_cards
comparison_table
citations
followups
message_end
```

事件说明：

| 事件 | 说明 |
| --- | --- |
| `message_start` | 创建助手消息 |
| `thinking_status` | 可选，提示正在理解需求、查找商品、检索知识 |
| `text_delta` | 流式文本 |
| `product_cards` | 商品卡片，可在文本生成中途发送 |
| `comparison_table` | 商品对比表 |
| `citations` | 引用来源 |
| `followups` | 追问建议 |
| `message_end` | 回答完成 |
| `error` | 异常或降级 |

第一版建议：

- `thinking_status` 只返回短状态，不暴露完整推理过程。
- 商品卡片不要由模型自由生成，必须用商品服务数据补全。
- 如果模型先生成了推荐商品 ID，后端必须二次查询商品服务后再发卡片。

## 8. Agent Trace 与评测闭环

每次 Agent 执行都需要记录 Trace。Trace 是评测和排障的基础，不应只保存最终回答。

### 8.1 Trace 结构

```json
{
  "trace_id": "trace_001",
  "session_id": "sess_001",
  "message_id": "msg_001",
  "input": {
    "content": "预算3000以内，拍娃，推荐手机",
    "attachment_ids": []
  },
  "understanding": {
    "intent": "product_recommendation",
    "slots": {
      "category": "手机",
      "budget_max": "3000",
      "usage_scene": ["拍娃"]
    },
    "rewritten_query": "预算3000元以内适合拍娃的手机推荐"
  },
  "plan": ["search_products", "search_knowledge", "rank_products", "generate_answer"],
  "tool_calls": [
    {
      "tool": "search_products",
      "latency_ms": 35,
      "result_count": 5
    },
    {
      "tool": "search_knowledge",
      "latency_ms": 180,
      "result_count": 8,
      "top_chunk_ids": ["ck_001", "ck_002"]
    }
  ],
  "generation": {
    "model": "chat-model",
    "prompt_version": "rag_answer_v1",
    "latency_ms": 1200
  },
  "quality_flags": {
    "has_citation": true,
    "has_product_card": true,
    "low_confidence": false
  }
}
```

### 8.2 评测复用方式

评测任务不单独实现一套问答逻辑，而是调用同一个 Agent Runtime：

```text
eval_case -> Agent Runtime evaluation_mode=true -> AgentResponse + Trace -> scorer -> eval_result
```

评测指标：

- 意图识别准确率。
- 槽位抽取准确率。
- Top-K 检索命中率。
- 商品命中率。
- 引用覆盖率。
- 回答要点覆盖率。
- 多轮约束继承准确率。
- UI 结构化输出合规率。

失败样例归因：

| 失败类型 | 可能原因 |
| --- | --- |
| 意图错误 | Prompt 不清晰，分类边界冲突 |
| 检索未命中 | Query 改写差，切片差，元数据过滤过严 |
| 推荐商品错误 | 商品标签不足，排序规则不合理 |
| 回答编造 | 无证据保护失效，Prompt 约束不足 |
| 卡片不一致 | 模型输出商品 ID 与商品服务结果未对齐 |
| 多轮断裂 | 会话摘要或最近商品上下文缺失 |

## 9. 状态与记忆设计

第一版只做轻量记忆，不做复杂用户画像。

### 9.1 会话短期记忆

保存内容：

- 当前购买类目。
- 用户预算。
- 使用场景。
- 品牌偏好。
- 排除条件。
- 最近推荐商品 ID。
- 最近对比商品 ID。

存储建议：

- Redis 保存热会话记忆。
- MySQL 保存会话摘要和消息历史。

### 9.2 记忆更新时机

- 每次用户消息理解后更新槽位。
- 每次推荐完成后记录推荐商品。
- 每次对比完成后记录对比商品。
- 会话结束或回答完成后异步更新摘要。

### 9.3 记忆合并规则

- 新约束覆盖旧约束，例如“预算改成 4000”。
- 排除条件累积，例如“不要曲面屏”“也不要太重”。
- 商品指代优先解析到最近推荐或最近对比列表。

## 10. 错误与降级

| 场景 | 降级策略 |
| --- | --- |
| 意图识别失败 | 进入 fallback，提示用户补充购买需求 |
| 图片解析失败 | 保留文本问题，提示图片无法识别 |
| 商品检索为空 | 说明没有匹配商品，可放宽预算/品牌/类目 |
| RAG 检索为空 | 只回答非关键建议，不输出确定参数和规则 |
| LLM 生成失败 | 返回可重试错误，保留用户消息 |
| SSE 中断 | 标记 assistant message failed，允许重新生成 |
| 评测执行失败 | 记录失败 case 和错误原因，不中断整个评测集 |

## 11. Prompt 拆分

第一版建议保留 4 类 Prompt：

```text
backend/prompts/
  intent_slot_v1.md
  query_rewrite_v1.md
  rag_answer_v1.md
  eval_judge_v1.md
```

说明：

- `intent_slot_v1.md`：意图识别和槽位抽取。
- `query_rewrite_v1.md`：结合上下文改写检索 Query。
- `rag_answer_v1.md`：基于商品和证据生成回答。
- `eval_judge_v1.md`：自动评分和失败归因。

如果为了减少首次实现成本，`intent_slot_v1.md` 和 `query_rewrite_v1.md` 可以合并为一次模型调用，但接口层仍建议保留两个逻辑概念。

## 12. 第一版实现优先级

P0 必须实现：

- 文本输入 Agent 主链路。
- 意图识别和槽位抽取。
- 商品检索工具。
- RAG 检索工具。
- 推荐回答生成。
- SSE 文本流和商品卡片事件。
- Trace 记录。
- 评测任务复用 Agent Runtime。

P1 建议实现：

- 图片 OCR/视觉解析。
- 商品对比。
- 引用来源展示。
- Follow-up 追问建议。
- 会话短期记忆。
- 简单失败归因。

P2 暂缓：

- 复杂 ReAct 多轮工具自主调用。
- 长期用户画像。
- 多业务 Skill 平台。
- 策略配置中心。
- GUI Agent。
- A/B 实验平台。

## 13. 推荐目录落位

后续开发时建议按以下目录组织 Agent 代码：

```text
backend/internal/agent/
  runtime.go
  context_builder.go
  understanding.go
  planner.go
  executor.go
  generator.go
  trace.go
  types.go

backend/internal/agent/tool/
  product_search.go
  product_resolve.go
  product_compare.go
  knowledge_search.go
  image_parse.go
  product_rank.go

backend/prompts/
  intent_slot_v1.md
  query_rewrite_v1.md
  rag_answer_v1.md
  eval_judge_v1.md
```

## 14. 链路图运行流程详解

链路图文件：[Agent框架链路图_v1.svg](../pics/Agent框架链路图_v1.svg)

这张图表达的是一个“导购任务专用 Agent”的运行链路。它不是通用多 Agent 平台，而是围绕电商导购任务做确定性编排。核心思想是：

```text
流程由后端代码确定性编排，LLM 只负责理解、归纳和生成；
商品事实和知识依据必须来自商品服务与 RAG 知识库。
```

### 14.1 客户端发起请求

用户在客户端输入文字、上传图片，或基于上一轮继续追问。

示例：

```text
预算 3000 以内，想买一台适合拍娃的手机，有什么推荐？
```

客户端将消息发送给 `Chat API`。API 负责基础接入能力：

- 鉴权。
- 限流。
- 参数校验。
- 创建用户消息记录。
- 创建助手消息占位记录。
- 返回 `message_start` SSE（服务器单向推送） 事件。

这一阶段的目标是先建立对话生命周期，让客户端进入“AI 正在回复”的状态。

### 14.2 进入 Agent Runtime

`Agent Runtime` 是一次 Agent 执行的主控制器。它不直接写具体业务判断，而是负责串联各个模块。

主链路如下：

```text
Input Normalizer
  -> Context Builder
  -> Intent & Slot Analyzer
  -> Plan Builder
  -> Tool Executor
  -> Decision Generator
  -> UI Event Composer
  -> Trace Recorder
```

Runtime 负责：

- 生成 `trace_id`（用于追踪本次调用的日志）。
- 管理一次执行的超时和取消。
- 捕获异常并转为降级响应。
- 推动 SSE 事件输出。
- 保证线上问答和评测任务复用同一条链路。

### 14.3 Input Normalizer 整理输入

`Input Normalizer` 将用户输入整理成 Agent 内部标准格式。

如果只有文本，直接保留文本内容。

如果包含图片，例如商品图、活动海报、购物车截图，会调用 `parse_image` 工具，通过 Vision/OCR 得到图片解析结果：

```json
{
  "detected_text": "满3000减300，会员券可叠加",
  "detected_products": ["X Phone 12"],
  "visual_summary": "图片疑似手机促销海报",
  "confidence": 0.78
}
```

图片解析结果会和用户原始文本合并，作为后续意图理解的输入。

### 14.4 Context Builder 构建上下文

`Context Builder` 读取和当前会话相关的信息：

- 最近 N 轮消息。
- 会话摘要。
- 会话短期记忆。
- 图片解析结果。

示例：

```text
上一轮：我主要给老人用，不要太贵。
这一轮：那第一款怎么样？
```

Agent 不能只理解“第一款怎么样”，还需要解析“第一款”指向上一轮推荐列表里的哪个商品，并继承“老人用、不要太贵”等约束。

### 14.5 Intent & Slot Analyzer 理解意图和约束

这是链路中的第一处核心 LLM 能力。它负责将自然语言转换成结构化语义。

用户输入：

```text
预算3000以内，拍娃，推荐手机
```

可能解析为：

```json
{
  "intent": "product_recommendation",
  "confidence": 0.86,
  "slots": {
    "category": "手机",
    "budget_max": "3000",
    "usage_scene": ["拍娃"],
    "must_have": ["拍照", "对焦快"],
    "avoid": []
  },
  "need_clarification": false,
  "rewritten_query": "预算3000元以内适合拍娃的手机推荐"
}
```

关键输出：

- `intent`：决定走推荐、对比、参数问答、优惠规则、售后问答还是兜底。
- `slots`：沉淀预算、类目、品牌、使用场景、禁忌条件等约束。
- `need_clarification`：判断是否必须追问。
- `rewritten_query`：作为 RAG 检索 Query。

### 14.6 澄清判断

链路图中间的判断节点是：

```text
需要澄清？
```

不是缺任何信息都要追问。第一版只在缺失信息会明显影响推荐质量时追问。

需要追问的例子：

```text
推荐一款手机
```

这类输入没有预算、用途、偏好，Agent 应该先问：

```text
你大概预算是多少？主要用来拍照、打游戏，还是给老人/学生使用？
```

不需要追问的例子：

```text
预算3000以内，拍娃，推荐手机
```

虽然缺少品牌偏好和存储偏好，但已经可以先给出候选商品，并在回答末尾给出追问建议。

### 14.7 Plan Builder 生成工具调用计划

`Plan Builder` 根据意图和槽位生成工具链。第一版不让模型自由规划工具，而是由后端规则确定调用顺序。

商品推荐默认工具链：

```text
search_products
  -> search_knowledge
  -> rank_products
  -> generate_answer
```

商品对比默认工具链：

```text
resolve_products
  -> compare_products
  -> search_knowledge
  -> generate_answer
```

优惠规则问答默认工具链：

```text
search_knowledge(doc_type=promotion)
  -> generate_answer
```

这样设计的原因是第一版需要稳定、可测、可排查。复杂 ReAct 和模型自主选择任意工具可以作为后续增强能力。

### 14.8 Tool Executor 执行工具

`Tool Executor` 执行具体工具。链路图右侧的工具与数据侧链主要包含以下能力。

#### parse_image

用于解析图片、海报、购物车截图。

输出：

- 商品名。
- 品牌。
- 活动文案。
- 图片中的使用场景。
- OCR 文本。

#### search_products

用于查询商品服务和 MySQL。

输入通常来自 `slots`：

- 类目。
- 预算。
- 品牌。
- 标签。
- 库存状态。
- 使用场景。

输出是商品事实数据。商品价格、库存、图片、参数等字段必须来自商品服务，不能由模型自由生成。

#### search_knowledge

用于查询 RAG 知识库和向量库。

检索内容包括：

- 商品详情。
- 营销活动。
- 售后政策。
- FAQ。
- 导购话术。

例如用户问：

```text
会员券和满减能叠加吗？
```

Agent 必须从营销文档中检索到对应规则，才能回答确定结论。

#### compare_products

用于对 2 到 4 个商品做横向对比。

输出包括：

- 核心差异。
- 适合人群。
- 不适合情况。
- 推荐结论。

#### rank_products

用于对候选商品排序。

排序依据包括：

- 是否满足硬约束，例如预算、库存、类目。
- 是否命中使用场景，例如拍娃、养宠、送礼。
- 是否有知识片段支持推荐理由。
- 是否存在风险项，例如用户要轻薄但商品偏重。

### 14.9 Decision Generator 生成导购回答

工具执行完成后，`Decision Generator` 基于三类信息生成最终回答：

- 用户意图和约束。
- 商品服务返回的商品事实。
- RAG 返回的知识证据。

输出不是纯文本，而是结构化 `AgentResponse`：

```json
{
  "answer_text": "如果预算控制在3000以内，并且重点是拍娃，我建议优先看 X Phone 12...",
  "products": [
    {
      "product_id": "p_001",
      "recommend_rank": 1,
      "reason": "预算内，拍照对焦快，适合抓拍儿童动态场景"
    }
  ],
  "citations": [
    {
      "chunk_id": "ck_001",
      "title": "X Phone 12 商品详情"
    }
  ],
  "follow_up_questions": [
    "你更重视拍照还是续航？"
  ]
}
```

生成阶段必须遵守事实边界：

- 商品价格、库存、图片、参数来自商品服务。
- 商品卖点、活动规则、售后政策来自知识库证据。
- 没有证据时不能输出确定结论。

### 14.10 UI Event Composer 转换为 SSE 事件

`UI Event Composer` 将 `AgentResponse` 转换成客户端可消费的 SSE 事件。

推荐事件顺序：

```text
message_start
thinking_status
text_delta
product_cards
comparison_table
citations
followups
message_end
```

事件说明：

- `text_delta`：流式文本，支持逐段渲染。
- `product_cards`：商品卡片。
- `comparison_table`：商品对比表。
- `citations`：引用来源。
- `followups`：追问建议。
- `message_end`：本次回答结束。

商品卡片必须二次查询商品服务补全字段。即使模型输出了商品 ID，后端也需要查询真实商品数据后再发送卡片事件。

### 14.11 知识库构建侧链

链路图左侧有一条知识库构建链路：

```text
非结构化文档
  -> 解析清洗切片
  -> Embedding 入库
  -> 知识库 / 向量库
```

运营上传的商品详情、营销文档、售后政策、FAQ，会经过：

- 文档解析。
- 文本清洗。
- 语义切片。
- OCR 处理。
- Embedding。
- 向量入库。
- 元数据保存。

Agent 在线回答时不关心文档解析细节，只依赖 `search_knowledge` 提供稳定检索结果。

### 14.12 Trace 与评测闭环

每一次 Agent 执行都要记录 Trace。Trace 是评测和排障的基础。

Trace 需要记录：

- 用户输入。
- 图片解析结果。
- 意图识别结果。
- 槽位抽取结果。
- Query 改写结果。
- 工具调用计划。
- 商品检索结果数量。
- RAG 命中的 chunk。
- 推荐的商品 ID。
- Prompt 版本。
- 模型版本。
- 各阶段耗时。
- 是否有引用。
- 是否有商品卡片。
- 是否低置信度。

评测任务复用同一个 Agent Runtime：

```text
eval_case
  -> Agent Runtime evaluation_mode=true
  -> AgentResponse + Trace
  -> scorer
  -> eval_result
```

评测结果反哺：

- Prompt 优化。
- 文档补充。
- 切片策略。
- Query 改写。
- 检索参数。
- 商品标签和排序规则。

### 14.13 完整运行示例

以用户输入为例：

```text
预算3000以内，想买适合拍娃的手机
```

完整运行过程：

```text
1. 客户端发送消息到 Chat API
2. API 鉴权、限流、建 user message 和 assistant placeholder
3. 返回 message_start
4. Input Normalizer 标准化文本输入
5. Context Builder 读取历史消息和短期记忆
6. Intent & Slot Analyzer 识别为 product_recommendation
7. 抽取 slots：手机、预算 3000、拍娃、拍照、对焦快
8. 判断信息足够，不追问
9. Plan Builder 生成 search_products -> search_knowledge -> rank_products -> generate_answer
10. search_products 查询预算内手机候选
11. search_knowledge 检索手机拍照、儿童抓拍、导购资料
12. rank_products 根据预算、场景和证据排序
13. Decision Generator 生成推荐回答、商品 ID、引用和追问
14. UI Event Composer 发送 text_delta 和 product_cards
15. 发送 citations、followups、message_end
16. Trace Recorder 保存执行过程
17. chat.postprocess 异步更新会话摘要和短期记忆
18. 后续评测任务基于 Trace 统计准确率、命中率和结构化输出合规率
```

### 14.14 运行链路的核心约束

这条链路必须始终遵守以下约束：

- Agent 是导购任务专用工作流，不是自由发挥的通用 Agent。
- LLM 增强理解和表达，不直接生产关键事实。
- 商品卡片来自商品服务，不来自模型自由文本。
- 活动规则、售后政策和商品参数必须有知识库证据。
- 结构化 UI 输出和自然语言回答必须一致。
- 线上问答与离线评测必须复用同一条链路。
- 每次回答必须留下 Trace，便于复盘和优化。

## 15. 最终取舍结论

第一版 Agent 不做“大而全的通用智能体平台”，而是做“导购任务专用 Agent”。核心取舍是：

- 用确定性流程保证稳定交付。
- 用 LLM 增强理解和生成。
- 用 RAG 和商品服务约束事实。
- 用结构化 UI 协议支撑客户端体验。
- 用 Trace 和评测闭环驱动后续优化。

这个框架足够覆盖比赛要求，也为后续扩展复杂 ReAct、多 Agent、策略配置和长期画像留下了清晰边界。
