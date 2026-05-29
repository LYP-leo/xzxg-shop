# 电商 AI 导购 Agent 框架设计 v2

> 实现状态更新（2026-05-20）：当前 `main` 已具备可演示 Agent 主链路：用户创建会话、发送问题、后端返回 SSE、前端渲染文本/商品卡片/引用/追问。当前 Agent Runtime 仍是规则化演示实现，尚未接入真实 Query Rewrite、Intent Router、LLM、Embedding、Qdrant 和重排链路。

## 1. v2 设计目标

v2 基于 [Agent框架设计_v1.md](./Agent框架设计_v1.md) 和 [v1_问题.md](./v1_问题.md) 调整。核心目标是把 v1 中偏抽象或不够明确的部分落到可实现的工程契约上，尤其解决以下问题：

- 明确会话、用户消息、助手消息的生命周期。
- 将 Query 改写从上下文构建中拆出来，作为显式链路节点。
- 取消“必须先澄清”的主策略，改为“先给可用回答，末尾追问补充信息”。
- 明确意图枚举和不同意图对应的工具链与 Prompt。
- 设计前端容易解析的 `AgentResponse` 和 SSE 事件。
- 补充图片无文字场景的识别与商品召回方案。
- 细化 RAG 文档解析、清洗、切片、召回和重排策略。

v2 主链路：

```text
Chat Session
  -> Message Intake
  -> Input Normalizer
  -> Context Builder
  -> Query Rewriter
  -> Intent Router
  -> Tool Plan Builder
  -> Tool Executor
  -> Prompt Router
  -> Decision Generator
  -> Response Composer
  -> SSE Stream
  -> Trace & Evaluation
```

当前已落地主链路：

```text
Login
  -> Role Portal
  -> Agent Session
  -> User Message
  -> MySQL Product Search
  -> MySQL Knowledge Chunk Search
  -> Rule-based Runtime
  -> SSE Stream
  -> React Render
```

当前已落地事件：

- `message_start`
- `status`
- `text_delta`
- `block_delta` with `product_card`
- `block_delta` with `citation`
- `followups`
- `message_end`

当前缺口：

- Query Rewriter 未接真实 LLM。
- Intent Router 未接真实意图模型。
- Tool Plan Builder 仍是固定流程。
- `assistant_message` 调用日志未落库。
- 图片理解、OCR、图片向量召回未实现。
- 评测 Trace 和自动评分未实现。

## 2. 对 v1 问题的逐条调整

### 2.1 会话与消息记录

问题：

```text
用户从客户端发起请求时，是否要创建用户消息记录？
如果每次进入聊天窗口都是一次新的会话，可以省去管理长期记忆的麻烦。
```

v2 决策：

- 用户每次进入聊天窗口时创建一个新的 `chat_session`。
- 用户在同一个聊天窗口内连续追问时，复用当前 `session_id`。
- 每次用户发送消息，都必须创建一条 `user_message`。
- 每次 Agent 开始生成前，创建一条 `assistant_message`，状态为 `streaming`。
- 流式完成后，将 `assistant_message.status` 更新为 `completed`。
- 用户停止生成或异常时，状态更新为 `canceled` 或 `failed`。

这样做的好处：

- 会话生命周期简单。
- 不需要 v1 中复杂长期记忆系统。
- 仍然支持同一个窗口内的多轮上下文。
- 便于停止生成、重新生成、SSE 断线恢复和评测回放。

### 2.2 assistant placeholder 的定义

问题：

```text
assistant placeholder 是什么？
```

v2 调整：

不再使用 `assistant placeholder` 这个不清晰的名字，改名为：

```text
assistant_message(status=streaming)
```

它是一条真实的助手消息记录，只是在流式生成过程中内容尚未完整。

用途：

- 提前生成 `message_id`，让前端可以绑定流式内容。
- 用户点击停止生成时，可以明确取消哪条助手消息。
- SSE 中断后，可以按 `message_id` 标记失败或重试。
- Trace、检索日志、工具调用日志都可以关联到这条助手消息。

### 2.3 无文字图片的识别

问题：

```text
如果图片没有文字，比如鼠标图片，怎么识别？
```

v2 调整：

图片解析不只做 OCR，而是拆成三层：

```text
OCR 文本识别
  + Vision 多模态描述
  + 图片向量相似商品召回
```

处理流程：

1. 对图片执行 OCR。
2. 对图片执行多模态大模型理解，输出品类、品牌线索、外观特征、使用场景。
3. 如图片中无文字或 OCR 置信度低，使用图片 embedding 检索相似商品。
4. 将视觉描述和相似商品候选一起交给后续意图识别和商品召回。

图片解析输出：

```json
{
  "ocr_text": "",
  "visual_summary": "图片中是一只黑色人体工学无线鼠标，带侧键，疑似办公或游戏鼠标",
  "detected_category": "鼠标",
  "detected_brand": "",
  "visual_attributes": ["黑色", "无线", "人体工学", "侧键"],
  "similar_product_candidates": [
    {
      "product_id": "p_mouse_001",
      "score": 0.82,
      "source": "image_embedding"
    }
  ],
  "confidence": 0.76
}
```

### 2.4 显式 Query 改写

问题：

```text
Context Builder 后面增加一个显式 query 改写环节。
```

v2 调整：

将 Query 改写从 `Intent & Slot Analyzer` 中拆出，成为显式节点：

```text
Context Builder -> Query Rewriter -> Intent Router
```

示例：

```text
上一轮：我主要给老人用，不要太贵。（推荐了 A 手机、B 手机）
这一轮：那第一款怎么样？
```

Query Rewriter 输出：

```json
{
  "original_query": "那第一款怎么样？",
  "rewritten_query": "A 手机是否适合老人使用？请结合价格、易用性、屏幕、续航和售后风险分析。",
  "resolved_refs": [
    {
      "text": "第一款",
      "type": "product",
      "product_id": "p_a_phone"
    }
  ],
  "inherited_constraints": {
    "user_group": "老人",
    "price_preference": "不要太贵"
  }
}
```

Query 改写结果会同时用于：

- 意图判断。
- RAG 检索。
- 商品指代解析。
- 最终回答上下文。

### 2.5 取消 need_clarification 主开关

问题：

```text
"need_clarification": false 不需要。如果用户意图不明确，先输出一版回答，最后再追问。
```

v2 调整：

取消 `need_clarification` 字段，不再把“澄清”作为主流程分支。

改为以下字段：

```json
{
  "answer_strategy": "answer_then_ask",
  "missing_info": ["预算", "使用场景"],
  "follow_up_questions": ["你的预算大概是多少？", "主要用于办公、游戏还是送礼？"]
}
```

回答策略：

| 策略 | 说明 |
| --- | --- |
| `direct_answer` | 信息充分，直接回答 |
| `answer_then_ask` | 信息不完整，但可以先给通用建议，末尾追问 |
| `cannot_answer_without_context` | 指代无法解析或问题无法落地，先请用户补充 |
| `safe_fallback` | 非导购、风险或系统无法处理场景 |

默认策略：

- 能回答就先回答。
- 信息不足时给一版有边界的建议，末尾提出 1 到 2 个关键问题。
- 只有“这两个哪个好”“第一款怎么样”但上下文完全不存在时，才进入 `cannot_answer_without_context`。

### 2.6 意图枚举

问题：

```text
意图需要分成几类？
```

v2 意图采用“一级意图 + 二级场景”的设计。

一级意图：

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

二级场景可选：

- `gift`
- `for_elder`
- `for_child`
- `pet_family`
- `student`
- `office`
- `gaming`
- `travel`
- `budget_sensitive`
- `quality_first`

二级场景不决定主链路，只影响检索过滤、排序权重和回答角度。

### 2.7 AgentResponse 设计

问题：

```text
为了方便前端解析，AgentResponse 该如何设计？
```

v2 采用“消息元信息 + blocks”的响应结构。前端只需要按 `blocks[].type` 渲染，不解析大模型自由文本。

完整结构：

```json
{
  "message_id": "msg_001",
  "session_id": "sess_001",
  "intent": "product_recommendation",
  "answer_strategy": "answer_then_ask",
  "text": "如果你还没确定品牌，我会优先按预算和拍娃场景推荐...",
  "blocks": [
    {
      "type": "markdown",
      "content": "如果你预算在 3000 元以内，拍娃场景优先看对焦速度、抓拍稳定性和存储空间。"
    },
    {
      "type": "product_card_list",
      "products": [
        {
          "product_id": "p_001",
          "name": "X Phone 12",
          "image_url": "/static/x-phone.png",
          "price": "2999.00",
          "stock_status": "in_stock",
          "tags": ["预算内", "拍照", "抓拍"],
          "recommend_reason": "预算内，对焦和抓拍能力更适合拍娃。",
          "risk_notes": ["续航不是最强"]
        }
      ]
    },
    {
      "type": "citation_list",
      "citations": [
        {
          "chunk_id": "ck_001",
          "title": "X Phone 12 商品详情",
          "snippet": "支持儿童抓拍模式和高速对焦。",
          "source": "商品详情文档"
        }
      ]
    },
    {
      "type": "followup_questions",
      "questions": ["你更重视拍照还是续航？", "是否需要 256GB 以上存储？"]
    }
  ],
  "trace_id": "trace_001",
  "quality_flags": {
    "has_citation": true,
    "has_product_card": true,
    "low_confidence": false
  }
}
```

支持的 block 类型：

| type | 前端渲染方式 |
| --- | --- |
| `markdown` | 普通 AI 文本 |
| `product_card_list` | 商品卡片列表 |
| `comparison_table` | 商品横向对比 |
| `citation_list` | 引用来源 |
| `followup_questions` | 追问建议 |
| `warning` | 风险提示或无证据说明 |
| `image_understanding` | 图片识别结果 |

### 2.8 RAG 构建与召回

问题：

```text
RAG 构建部分，非结构化文档如何解析清洗切片？如何召回意思相近的文档？
```

v2 将 RAG 分为离线构建和在线召回两部分。

#### 离线构建

```text
文档上传
  -> 文件类型识别
  -> 文档解析
  -> 文本清洗
  -> 语义切片
  -> 元数据抽取
  -> 质量检查
  -> Embedding
  -> 向量库入库
  -> MySQL 保存 chunk 元数据
```

文档解析策略：

| 类型 | 解析方式 |
| --- | --- |
| Markdown / TXT | 直接按标题和段落解析 |
| PDF | 提取文本、页码、表格；必要时 OCR |
| Word | 按标题、段落、表格解析 |
| HTML | 去除导航、脚本、样式，保留正文结构 |
| 图片 | OCR + Vision 描述 |
| 表格 | 保留表头，将行转换为结构化文本 |

清洗规则：

- 删除页眉页脚、重复导航、版权噪声。
- 合并多余空白。
- 保留标题层级。
- 保留价格、型号、活动规则、日期等关键事实。
- 对表格补全表头上下文，避免切片后失去语义。

切片策略：

- 普通正文：500 到 800 中文字符一个 chunk。
- overlap：80 到 120 字符。
- 表格：按主题或连续行分块，不按固定字符硬切。
- 活动规则：按规则条款分块，保留生效时间、适用商品、叠加限制。
- 商品详情：按商品 ID、参数、卖点、适用人群分块。

chunk 元数据：

```json
{
  "chunk_id": "ck_001",
  "document_id": "doc_001",
  "doc_type": "product_detail",
  "product_id": "p_001",
  "category": "手机",
  "brand": "X",
  "title_path": ["手机", "X Phone 12", "拍照能力"],
  "source_page": 3,
  "effective_start": "2026-05-01T00:00:00+08:00",
  "effective_end": null,
  "content_hash": "hash"
}
```

#### 在线召回

在线召回采用混合检索：

```text
Query Rewrite
  -> 向量召回
  -> 关键词召回
  -> 元数据过滤
  -> 规则补召回
  -> Rerank
  -> 证据压缩
```

召回方式：

- 向量召回：解决“意思相近但字面不同”的问题。
- 关键词召回：保证型号、品牌、活动名、参数等精确词命中。
- 元数据过滤：按 `doc_type`、`product_id`、`category`、有效期过滤。
- 规则补召回：优惠、售后类问题优先补召回规则文档。
- Rerank：用重排模型或 LLM 轻量评分挑选最相关 chunk。

示例：

```text
用户问：会员券能不能跟满减一起用？
关键词召回：会员券、满减、叠加
向量召回：优惠能否同时使用、活动规则组合
过滤：doc_type=promotion，effective_time 当前有效
```

### 2.9 不同意图使用不同 Prompt

问题：

```text
不同意图最后的 Decision Generator 需要不同 Prompt 驱动。
```

v2 增加 `Prompt Router`。

```text
Tool Executor -> Prompt Router -> Decision Generator
```

Prompt 路由规则：

| intent | prompt |
| --- | --- |
| `product_recommendation` | `answer_recommendation_v2.md` |
| `product_comparison` | `answer_comparison_v2.md` |
| `product_detail_qa` | `answer_product_qa_v2.md` |
| `promotion_rule_qa` | `answer_promotion_v2.md` |
| `after_sales_qa` | `answer_after_sales_v2.md` |
| `image_product_recognition` | `answer_image_product_v2.md` |
| `image_rule_explanation` | `answer_image_rule_v2.md` |
| `shopping_decision_support` | `answer_decision_support_v2.md` |
| `general_shopping_chat` | `answer_general_shopping_v2.md` |
| `unsupported` | `answer_fallback_v2.md` |

Prompt 不同，但输出必须统一为 `AgentResponse`。

## 3. v2 总体架构

v2 架构分为五层：

```text
客户端交互层
  -> Chat API 层
  -> Agent 编排层
  -> 工具与数据层
  -> 评测与反馈层
```

### 3.1 客户端交互层

职责：

- 新建聊天窗口时创建 session。
- 发送文本和图片。
- 接收 SSE 事件。
- 按 `blocks[].type` 渲染文本、商品卡片、对比表、引用和追问。
- 支持停止生成和重新生成。

### 3.2 Chat API 层

职责：

- 创建会话。
- 创建用户消息。
- 创建 `assistant_message(status=streaming)`。
- 调用 Agent Runtime。
- 将 Agent 事件转发给前端。
- 保存最终消息状态。

### 3.3 Agent 编排层

职责：

- 标准化输入。
- 构建上下文。
- 改写 Query。
- 识别意图。
- 构建工具计划。
- 执行工具。
- 路由 Prompt。
- 生成结构化回答。
- 记录 Trace。

### 3.4 工具与数据层

工具：

- `parse_image`
- `rewrite_query`
- `classify_intent`
- `search_products`
- `resolve_products`
- `compare_products`
- `search_knowledge`
- `rank_products`
- `compose_response`

数据：

- MySQL 商品库。
- MySQL 会话和消息。
- MySQL 文档和 chunk 元数据。
- Redis 会话短期状态。
- 向量数据库。
- 对象存储或本地上传目录。

### 3.5 评测与反馈层

职责：

- 复用 Agent Runtime 执行评测 Case。
- 记录 AgentResponse 和 Trace。
- 计算意图准确率、检索命中率、商品命中率、回答要点覆盖率、结构化输出合规率。
- 将失败归因到 Prompt、Query 改写、RAG、商品数据或 Response 结构。

## 4. v2 运行链路

### 4.1 新会话创建

```text
用户进入聊天窗口
  -> POST /api/v1/chat/sessions
  -> 创建 chat_session
  -> 返回 session_id
```

说明：

- 每次进入聊天窗口创建新 session。
- 当前 session 内支持多轮追问。
- 历史聊天可以只作为列表记录，不参与新 session 的默认上下文。

### 4.2 发送消息

```text
用户发送消息
  -> 创建 user_message
  -> 创建 assistant_message(status=streaming)
  -> 返回 message_start
  -> Agent Runtime 开始执行
```

消息状态：

| 状态 | 说明 |
| --- | --- |
| `streaming` | 正在生成 |
| `completed` | 生成完成 |
| `failed` | 生成失败 |
| `canceled` | 用户取消 |

### 4.3 Agent 执行

```text
Input Normalizer
  -> 如果有图片，执行 OCR + Vision + 图片相似商品召回
Context Builder
  -> 读取当前 session 内上下文
Query Rewriter
  -> 改写省略、指代和多轮继承问题
Intent Router
  -> 输出一级意图、二级场景、槽位、置信度
Tool Plan Builder
  -> 根据意图选择工具链
Tool Executor
  -> 执行商品、RAG、对比、排序等工具
Prompt Router
  -> 根据 intent 选择回答 Prompt
Decision Generator
  -> 生成统一 AgentResponse
Response Composer
  -> 校验、补全、转 blocks
SSE Stream
  -> 分段推送给前端
Trace Recorder
  -> 保存完整链路日志
```

### 4.4 回答与追问策略

v2 不默认阻塞式澄清。

策略：

- 信息充分：直接回答。
- 信息不完整但可给建议：先回答，再追问。
- 指代无法解析：请求用户补充上下文。
- 无证据：明确说明知识库未命中，不输出确定事实。

示例：

```text
用户：推荐一个鼠标

回答：
如果你还没确定具体场景，我可以先按通用办公和轻度游戏两个方向推荐。
办公优先看握持舒适度、无线连接和续航；游戏优先看重量、传感器和按键响应。

你可以补充两个信息，我能推荐得更准：
1. 主要办公还是游戏？
2. 预算大概是多少？
```

## 5. v2 Intent Router 设计

Intent Router 输出结构：

```json
{
  "intent": "product_recommendation",
  "secondary_scenes": ["budget_sensitive", "for_child"],
  "confidence": 0.86,
  "slots": {
    "category": "手机",
    "budget_min": null,
    "budget_max": "3000",
    "brand_preference": [],
    "usage_scene": ["拍娃"],
    "target_user": "家长",
    "must_have": ["拍照", "对焦快"],
    "avoid": []
  },
  "missing_info": ["storage_preference"],
  "answer_strategy": "answer_then_ask"
}
```

### 5.1 意图到工具链映射

| intent | 工具链 |
| --- | --- |
| `product_recommendation` | `search_products` -> `search_knowledge` -> `rank_products` |
| `product_comparison` | `resolve_products` -> `compare_products` -> `search_knowledge` |
| `product_detail_qa` | `resolve_products` -> `search_knowledge` |
| `promotion_rule_qa` | `search_knowledge(doc_type=promotion)` |
| `after_sales_qa` | `search_knowledge(doc_type=after_sales)` |
| `image_product_recognition` | `parse_image` -> `image_similar_product_search` -> `search_products` |
| `image_rule_explanation` | `parse_image` -> `search_knowledge(doc_type=promotion)` |
| `shopping_decision_support` | `resolve_products/search_products` -> `search_knowledge` -> `compare_products/rank_products` |
| `general_shopping_chat` | `search_knowledge(optional)` |
| `unsupported` | 无工具或安全兜底 |

### 5.2 意图到 Prompt 映射

| intent | Prompt | 回答重点 |
| --- | --- | --- |
| `product_recommendation` | `answer_recommendation_v2.md` | 推荐理由、适合/不适合、风险、追问 |
| `product_comparison` | `answer_comparison_v2.md` | 差异、结论、适用场景 |
| `product_detail_qa` | `answer_product_qa_v2.md` | 参数事实、证据引用 |
| `promotion_rule_qa` | `answer_promotion_v2.md` | 规则解释、生效条件、叠加限制 |
| `after_sales_qa` | `answer_after_sales_v2.md` | 售后范围、限制、无证据保护 |
| `image_product_recognition` | `answer_image_product_v2.md` | 图片识别、相似商品、置信度 |
| `image_rule_explanation` | `answer_image_rule_v2.md` | 海报规则解释、风险提示 |
| `shopping_decision_support` | `answer_decision_support_v2.md` | 取舍建议、决策矩阵 |
| `general_shopping_chat` | `answer_general_shopping_v2.md` | 通用购物建议 |
| `unsupported` | `answer_fallback_v2.md` | 能力边界说明 |

## 6. v2 AgentResponse 与 SSE

### 6.1 AgentResponse

`AgentResponse` 是后端内部最终输出，必须可被前端稳定解析。

```json
{
  "message_id": "msg_001",
  "session_id": "sess_001",
  "trace_id": "trace_001",
  "intent": "product_recommendation",
  "answer_strategy": "answer_then_ask",
  "text": "预算 3000 以内拍娃，我建议优先看对焦快、抓拍稳定、存储足够的机型。",
  "blocks": [
    {
      "type": "markdown",
      "content": "预算 3000 以内拍娃，我建议优先看对焦快、抓拍稳定、存储足够的机型。"
    },
    {
      "type": "product_card_list",
      "products": []
    },
    {
      "type": "citation_list",
      "citations": []
    },
    {
      "type": "followup_questions",
      "questions": []
    }
  ],
  "quality_flags": {
    "has_citation": true,
    "has_product_card": true,
    "low_confidence": false,
    "no_evidence_for_key_claim": false
  }
}
```

### 6.2 SSE 事件

SSE 事件与 `AgentResponse.blocks` 对齐。

```text
message_start
status
text_delta
block_delta
block_done
message_end
error
```

事件说明：

| 事件 | 说明 |
| --- | --- |
| `message_start` | 返回 `message_id`、`session_id`、`trace_id` |
| `status` | 正在识别图片、检索商品、查询知识库等短状态 |
| `text_delta` | 流式文本 |
| `block_delta` | 商品卡片、引用、对比表等结构化块 |
| `block_done` | 某个 block 已完整 |
| `message_end` | 返回最终 `AgentResponse` 摘要 |
| `error` | 返回错误码和降级文案 |

第一版前端可以先只处理：

- `message_start`
- `text_delta`
- `block_delta`
- `message_end`
- `error`

## 7. v2 RAG 方案

### 7.1 文档构建流水线

```text
Upload
  -> Parse
  -> Clean
  -> Segment
  -> Metadata Extract
  -> Quality Check
  -> Embed
  -> Index
```

#### Parse

按文件类型选择解析器：

- Markdown/TXT：按标题和段落。
- PDF：文本、表格、页码，必要时 OCR。
- Word：标题、段落、表格。
- HTML：正文抽取，去掉导航和脚本。
- 图片：OCR + Vision 描述。

#### Clean

清洗目标：

- 删除重复文本。
- 删除页眉页脚和无关导航。
- 保留标题层级。
- 保留活动规则中的条件、限制和时间。
- 表格行补齐表头。

#### Segment

切片类型：

| 文档类型 | 切片方式 |
| --- | --- |
| 商品详情 | 按商品、参数、卖点、适用人群 |
| 营销规则 | 按活动、条件、限制、叠加规则 |
| 售后政策 | 按服务类型、适用条件、例外情况 |
| FAQ | 一问一答为一个 chunk |
| 导购话术 | 按场景和目标用户 |

#### Quality Check

质量检查：

- chunk 是否过短或过长。
- 是否缺失标题。
- 是否缺失商品 ID 或类目。
- 活动规则是否有生效时间。
- 表格切片是否保留表头。

### 7.2 在线检索

```text
rewritten_query
  -> query embedding
  -> vector search
  -> keyword search
  -> metadata filter
  -> merge
  -> rerank
  -> evidence compress
```

混合检索：

- 向量检索处理语义相似。
- 关键词检索处理型号、参数、活动名。
- 元数据过滤保证类目、商品、文档类型、有效期正确。
- Rerank 过滤弱相关 chunk。

证据压缩：

- 只保留与回答相关的句子。
- 保留 `chunk_id`、标题、来源和页码。
- 不把长文档整体塞进 Prompt。

## 8. v2 Trace 与评测

Trace 必须覆盖 v2 新增节点：

```json
{
  "trace_id": "trace_001",
  "session_id": "sess_001",
  "message_id": "msg_001",
  "query_rewrite": {
    "original_query": "那第一款怎么样？",
    "rewritten_query": "A 手机是否适合老人使用？",
    "resolved_refs": []
  },
  "intent": {
    "intent": "product_detail_qa",
    "confidence": 0.82,
    "answer_strategy": "direct_answer"
  },
  "tool_plan": ["resolve_products", "search_knowledge"],
  "prompt": {
    "name": "answer_product_qa_v2.md",
    "version": "v2"
  },
  "quality_flags": {
    "has_citation": true,
    "has_product_card": false,
    "low_confidence": false
  }
}
```

v2 新增评测指标：

- Query 改写准确率。
- 指代解析准确率。
- 意图路由准确率。
- Prompt 路由准确率。
- AgentResponse block 合规率。
- 图片识别 Top-K 商品命中率。
- RAG 语义召回命中率。

## 9. v2 推荐目录

```text
backend/internal/agent/
  runtime.go
  intake.go
  input_normalizer.go
  context_builder.go
  query_rewriter.go
  intent_router.go
  planner.go
  executor.go
  prompt_router.go
  decision_generator.go
  response_composer.go
  trace.go
  types.go

backend/internal/agent/tool/
  image_parse.go
  image_similar_search.go
  product_search.go
  product_resolve.go
  product_compare.go
  product_rank.go
  knowledge_search.go

backend/prompts/
  query_rewrite_v2.md
  intent_router_v2.md
  answer_recommendation_v2.md
  answer_comparison_v2.md
  answer_product_qa_v2.md
  answer_promotion_v2.md
  answer_after_sales_v2.md
  answer_image_product_v2.md
  answer_image_rule_v2.md
  answer_decision_support_v2.md
  answer_general_shopping_v2.md
  answer_fallback_v2.md
  eval_judge_v2.md
```

## 10. v2 实现优先级

P0：

- 新会话创建策略。
- `assistant_message(status=streaming)`。
- Query Rewriter。
- Intent Router。
- Prompt Router。
- 统一 AgentResponse blocks。
- 文本商品推荐链路。
- 基础 RAG 混合召回。

P1：

- 图片 Vision 解析。
- 图片相似商品召回。
- 商品对比链路。
- 优惠规则问答。
- 售后问答。
- Trace v2。
- AgentResponse block 合规评测。

P2：

- 更复杂的 rerank。
- 图片向量库优化。
- 长期用户画像。
- 多 Prompt 自动实验。
- 更复杂的多步 ReAct。

## 11. v2 总结

v2 相比 v1 的关键变化：

- 会话策略更明确：进入窗口创建 session，发送消息创建 user/assistant 两条消息。
- `assistant placeholder` 改为明确的 `assistant_message(status=streaming)`。
- 图片理解从 OCR 扩展为 OCR + Vision + 图片相似商品召回。
- Query 改写成为显式节点。
- 取消 `need_clarification`，改为 `answer_strategy + follow_up_questions`。
- 意图体系更具体，支持 Prompt Router。
- AgentResponse 改为前端友好的 blocks 协议。
- RAG 构建和召回策略更完整。
- 评测指标覆盖 Query 改写、意图路由、Prompt 路由和结构化输出。

这版架构仍然坚持一个原则：第一阶段不做复杂通用 Agent 平台，而是做一个稳定、可解释、可评测的电商导购 Agent。
