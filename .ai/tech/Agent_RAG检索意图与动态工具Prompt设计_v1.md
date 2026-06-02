# Agent RAG 检索意图与动态工具 Prompt 设计 v1

更新时间：2026-05-24

## 背景

当前 Agent 链路存在四个问题：

1. RAG 工具只有 `query/limit`，没有把“品牌词召回、品类词召回、型号词召回、场景词召回”等检索策略显式传给 RAG 系统。
2. 管理员链路追踪能看到模型原始输出，但看不到每次 `llm_call` 的实际 prompt/messages，排查 prompt 拼装和工具选择问题不够直接。
3. `non_guide`、`fast_product`、导购 ReAct 共用一套工具 prompt，边界不清晰；某些固定流程不应该交给 LLM 自由推理。
4. 商品库无对应品类时，RDS/Milvus 弱相关召回可能被当作正常商品候选，导致最终回答误挂无关商品。

## 目标

1. LLM 调用 RAG 工具时显式决策检索意图和召回策略。
2. RAG 系统根据检索意图启用不同召回、过滤、重排权重。
3. 管理员 trace 可视化每次 LLM 调用的完整 messages、最终 system prompt、user prompt 和原始输出。
4. 明确 `guide`、`non_guide`、`fast_product` 边界。
5. 工具协议按意图动态组装，只暴露当前场景需要的工具，减少模型误调用。
6. 对弱相关召回增加 `relevance_status` 和最终挂品白名单，确保无库存场景不误挂商品。

## 1. RAG 检索意图设计

### 1.1 检索意图枚举

建议先定义稳定枚举，写入 `rag.RetrievalPlan`：

```go
type RetrievalIntent string

const (
  RetrievalBrand      RetrievalIntent = "brand"       // 品牌词召回
  RetrievalCategory   RetrievalIntent = "category"    // 品类词召回
  RetrievalModel      RetrievalIntent = "model"       // 型号/系列召回
  RetrievalAttribute  RetrievalIntent = "attribute"   // 属性/功效/参数召回
  RetrievalScenario   RetrievalIntent = "scenario"    // 场景/人群/用途召回
  RetrievalCompare    RetrievalIntent = "compare"     // 多候选对比召回
  RetrievalNegative   RetrievalIntent = "negative"    // 排除/反选约束
  RetrievalPolicy     RetrievalIntent = "policy"      // 售后/活动/规则类资料
  RetrievalHybrid     RetrievalIntent = "hybrid"      // 不确定或多策略混合
)
```

一个 query 可以带多个 intent，因为真实请求经常是组合：

```json
{
  "query": "华为 轻薄办公 笔记本",
  "limit": 5,
  "retrieval_intents": ["brand", "category", "attribute"],
  "entities": {
    "brands": ["华为"],
    "categories": ["笔记本", "电脑"],
    "models": [],
    "attributes": ["轻薄", "办公"],
    "scenarios": ["办公"],
    "negative_terms": []
  }
}
```

### 1.2 工具参数升级

`search_knowledge` 和后续 `search_products` 都应支持结构化检索参数。

#### search_knowledge

```json
{
  "query": "华为 轻薄办公 笔记本",
  "limit": 5,
  "retrieval_intents": ["brand", "category", "attribute"],
  "entities": {
    "brands": ["华为"],
    "categories": ["笔记本", "电脑"],
    "models": [],
    "attributes": ["轻薄", "办公"],
    "scenarios": ["办公"],
    "negative_terms": []
  },
  "filters": {
    "doc_types": ["product_detail", "buying_guide"],
    "product_ids": [],
    "category_ids": []
  }
}
```

#### search_products

```json
{
  "query": "华为 轻薄办公 笔记本",
  "limit": 5,
  "retrieval_intents": ["brand", "category", "attribute"],
  "entities": {
    "brands": ["华为"],
    "categories": ["笔记本", "电脑"],
    "attributes": ["轻薄", "办公"]
  }
}
```

### 1.3 RAG 系统如何使用

LLM 只负责给出意图和实体，不直接控制底层 SQL/Milvus 细节。后端将参数转换为 `rag.RetrievalPlan`：

| 检索意图 | 召回侧重点 | 重排侧重点 |
| --- | --- | --- |
| `brand` | 品牌字段、标题、商品元数据、品牌别名 | `brand_boost` 提高，泛词降权 |
| `category` | 类目字段、品类同义词、标题品类词 | `category_boost` 提高 |
| `model` | 型号/系列精确匹配、标题、SKU | `model_boost` 和 `title_match` 提高 |
| `attribute` | 属性、卖点、参数、功效词 | `term_match` 和 `evidence_density` 提高 |
| `scenario` | 场景词、人群词、用途词、攻略文档 | `doc_type_score` 和 `term_match` 提高 |
| `compare` | 多候选分别召回，再合并去重 | 保证每个候选至少有证据 |
| `negative` | 先召回正向候选，再过滤/降权反向词 | `negative_terms` 命中降权或剔除 |
| `policy` | 售后、活动、规则、FAQ 文档 | 仅查 policy/faq/promotion 文档 |
| `hybrid` | 当前默认混合检索 | 使用 Nacos 默认权重 |

### 1.4 Nacos 配置建议

放在 `xzxg-shop-rag-config.json`：

```json
{
  "configs": [
    {"config_key": "retrieval.intent.brand.weight.brand_boost", "config_value": "0.65", "value_type": "float"},
    {"config_key": "retrieval.intent.category.weight.category_boost", "config_value": "0.45", "value_type": "float"},
    {"config_key": "retrieval.intent.model.weight.model_boost", "config_value": "0.65", "value_type": "float"},
    {"config_key": "retrieval.intent.scenario.doc_types", "config_value": "buying_guide,scenario_guide,product_detail", "value_type": "string"},
    {"config_key": "retrieval.intent.policy.doc_types", "config_value": "policy,faq,promotion", "value_type": "string"}
  ]
}
```

## 2. 弱相关召回与误挂品防线

### 2.1 问题定义

`run_1779597014647992000` 暴露的是“商品库无对应品类，但检索仍返回弱相关商品，最终回答又把弱相关商品挂出来”的问题。

典型链路：

1. 用户问“马桶推荐”。
2. 商品库没有马桶、智能马桶、坐便器相关商品。
3. RDS/Milvus 因“智能”等泛词召回手机、平板、电脑等弱相关商品。
4. 工具把这些结果当作正常结果返回。
5. `react.final` 收到 `product_ids` 后，即使正文说明“暂无马桶类商品”，仍可能输出 `<item>p_digital_001</item>` 或在文本中暴露无关商品 ID。

本阶段不引入硬编码品类规则，也不要求 RAG 检索意图先完成。先用通用的相关性分级和最终挂品约束解决误挂品。

### 2.2 向量最低分与词面保护

Milvus 召回不能只按 topK 返回。需要增加两类通用保护：

1. `vector_min_score`：低于最低相似度的向量结果直接丢弃。
2. `lexical_guard`：向量结果必须和 query 在品牌、品类、型号、核心属性、同义词集合中至少命中一类证据，否则降级为弱相关或剔除。

Nacos 配置放在 `xzxg-shop-rag-config.json`：

```json
{
  "retrieval.vector.min_score": 0.58,
  "retrieval.vector.lexical_guard.enabled": true,
  "retrieval.vector.lexical_guard.min_evidence_count": 1,
  "retrieval.vector.lexical_guard.weak_score_threshold": 0.66,
  "retrieval.product.no_match.max_results": 0,
  "retrieval.product.weak.max_results": 3
}
```

建议规则：

| 场景 | 处理 |
| --- | --- |
| 向量分低于 `vector_min_score` | 丢弃 |
| 向量分达标但无词面证据 | 标记 `weak`，默认不允许最终挂品 |
| 有品牌/品类/型号/核心属性证据 | 可进入正常重排 |
| RDS 和向量都只有弱相关 | 工具返回 `weak` 或 `no_match`，不返回可挂品 ID |

### 2.3 工具返回相关性状态

`search_products` 和 `search_knowledge` 的 observation 增加 `relevance_status`，让主 Agent 和最终输出层知道工具结果是否可信。

```json
{
  "ok": true,
  "tool": "search_products",
  "relevance_status": "weak",
  "relevance_reason": "召回结果仅命中泛词“智能”，没有命中马桶/坐便器等核心需求证据",
  "product_ids": [],
  "candidate_product_ids": ["p_digital_001", "p_digital_002"],
  "dropped_product_ids": ["p_digital_001", "p_digital_002"],
  "results": []
}
```

状态定义：

| status | 含义 | 是否允许最终挂品 |
| --- | --- | --- |
| `ok` | 结果和用户需求有明确品牌、品类、型号或属性证据 | 是 |
| `weak` | 有一些相似或泛词命中，但不足以证明相关 | 否 |
| `no_match` | 没有可用结果 | 否 |

处理原则：

1. `ok=true` 只表示工具执行成功，不等于结果相关。
2. `product_ids` 只放允许最终挂品的商品 ID。
3. `candidate_product_ids` 可用于 trace 排查，但不能进入 final prompt 的可挂品列表。
4. `dropped_product_ids` 记录被相关性防线剔除的商品，管理员 trace 可视化展示。

### 2.4 最终挂品保护

`react.final` 只能看到允许挂品的 ID：

1. 只把 `relevance_status=ok` 的 `product_ids` 汇总到 final instruction。
2. `weak/no_match` 的候选商品只进入 trace，不进入 final 可挂品上下文。
3. 如果本轮所有工具结果都是 `weak/no_match`，final prompt 必须明确：
   - 当前商品库未找到匹配商品。
   - 不要输出 `<item>`。
   - 可以给通用选购建议，但必须说明无法基于当前商品库推荐具体商品。
4. 流式输出过滤器需要兜底校验 `<item>` 内容：
   - `<item>` 内必须是 final 允许列表中的 product_id。
   - 不在允许列表的 `<item>` 整段移除，不保留内部商品 ID 文本。

最终 prompt 增加硬约束：

```markdown
【挂品硬约束】
- 你只能使用本轮 final_allowed_product_ids 中的商品 ID 输出 `<item>...</item>`。
- 如果 final_allowed_product_ids 为空，禁止输出任何 `<item>`。
- search_products 返回 `relevance_status=weak/no_match` 时，说明当前商品库暂无匹配商品，不要把候选商品当作推荐。
- 可以提供通用选购建议，但不能编造商品、品牌或商品 ID。
```

### 2.5 测评场景与指标

新增“无库存/弱相关误挂品”测评集，建议文件：

```text
quality/data/eval/agent_no_inventory_cases.jsonl
```

也可以先合并进 `quality/data/eval/agent_e2e_queries.jsonl`，通过 `case_type=no_inventory` 区分。

基础 case：

```jsonl
{"id":"noinv_toilet_001","case_type":"no_inventory","query":"给我推荐一款好用的马桶","expected_no_product_refs":true,"forbidden_terms":["<item>","p_digital_"],"expected_ack_terms":["暂无","没有找到","当前商品库"]}
{"id":"noinv_toilet_002","case_type":"no_inventory","query":"智能马桶怎么选，顺便推荐几个商品","expected_no_product_refs":true,"forbidden_terms":["<item>","手机","平板","电脑"],"expected_ack_terms":["暂无","没有找到","当前商品库"]}
{"id":"noinv_toilet_003","case_type":"no_inventory","query":"家用坐便器有没有推荐","expected_no_product_refs":true,"forbidden_terms":["<item>","p_digital_"],"expected_ack_terms":["暂无","没有找到","当前商品库"]}
{"id":"noinv_bath_001","case_type":"no_inventory","query":"浴室柜推荐一下","expected_no_product_refs":true,"forbidden_terms":["<item>","p_digital_"],"expected_ack_terms":["暂无","没有找到","当前商品库"]}
{"id":"noinv_bath_002","case_type":"no_inventory","query":"有没有恒温花洒套装","expected_no_product_refs":true,"forbidden_terms":["<item>","p_digital_"],"expected_ack_terms":["暂无","没有找到","当前商品库"]}
```

工具级测评增加：

```jsonl
{"id":"tool_noinv_toilet_001","tool":"search_products","query":"马桶 推荐 好用","expected_relevance_status":["no_match","weak"],"expected_no_product_ids":true}
{"id":"tool_noinv_toilet_002","tool":"search_products","query":"智能马桶 陶瓷 节水","expected_relevance_status":["no_match","weak"],"expected_no_product_ids":true}
{"id":"tool_noinv_bath_001","tool":"search_products","query":"恒温花洒 套装","expected_relevance_status":["no_match","weak"],"expected_no_product_ids":true}
```

核心指标：

| 指标 | 目标 |
| --- | --- |
| `irrelevant_product_ref_rate` | 无库存 case 中无关商品引用率，目标 0 |
| `item_tag_validity_rate` | `<item>` 全部来自允许列表，目标 100% |
| `no_inventory_ack_rate` | 无库存 case 明确说明暂无匹配商品的比例，目标 >= 95% |
| `weak_result_blocked_rate` | `weak` 结果未进入 final 挂品列表的比例，目标 100% |
| `tool_relevance_status_accuracy` | 工具对 no_match/weak/ok 的分类准确率，目标 >= 90% |

验收用例：

1. “马桶推荐”即使底层向量召回平板、手机、电脑，也不能在最终回答中出现 `<item>` 或相关商品 ID。
2. “智能马桶怎么选”可以输出通用选购建议，但必须说明当前商品库暂无匹配商品。
3. 管理员 trace 能看到被剔除的 `candidate_product_ids/dropped_product_ids` 和剔除原因。

## 3. 管理员 trace 展示实际 Prompt

### 3.1 当前不足

现在 `traceLLM` 主要记录：

- stage
- model
- duration
- raw_output
- filtered_output
- raw_length

缺少：

- 完整 messages
- system prompt
- user prompt
- tool protocol prompt
- intent prompt
- prompt 配置版本
- 调用参数 temperature、stream、model

### 3.2 建议记录结构

每次 `llm_call` 的 metadata 增加：

```json
{
  "model": "qwen3.6-plus",
  "temperature": 0.4,
  "stream": true,
  "prompt": {
    "messages": [
      {"role": "system", "content": "..."},
      {"role": "user", "content": "..."}
    ],
    "system_prompt": "...",
    "user_prompt": "...",
    "prompt_keys": [
      "agent.prompt.answer_base",
      "agent.prompt.intent.scene_solution",
      "agent.tool.search_products"
    ],
    "prompt_versions": {
      "agent.prompt.answer_base": 3,
      "agent.prompt.intent.scene_solution": 2
    }
  },
  "raw_output": "...",
  "filtered_output": "..."
}
```

### 3.3 隐私与体积控制

Prompt 可能很长，trace 入库要做控制：

1. 默认完整记录本地开发环境。
2. 生产环境可配置 `trace.llm.prompt_capture=full|truncated|off`。
3. `ai.api_key`、Authorization、用户手机号等敏感字段必须脱敏。
4. 单条 metadata 超过阈值时：
   - `prompt.messages` 截断到 N 字符。
   - 保留 `prompt_hash`。
   - 后续可把大 prompt 写对象存储或单独表。

### 3.4 管理员页面展示

在每个 `llm_call` trace event 下增加折叠区：

- 实际 System Prompt
- 实际 User Prompt
- 完整 Messages JSON
- 模型原始输出
- 前端展示输出
- Prompt Keys / Versions

默认折叠，点击展开，避免页面过长。

## 4. Agent 路由边界重新定义

### 4.1 guide

定义：用户目标是完成商品选购、对比、搭配、场景方案、商品详情咨询。

处理方式：

- 需要 LLM 做意图识别。
- 进入导购 ReAct 主链路。
- 按 P1-P6 组装不同 prompt 和工具集合。

典型：

- “帮我推荐华为电脑”
- “iPhone 和 Mate 哪个适合拍照”
- “露营装备清单”
- “敏感肌面霜怎么选”

### 4.2 fast_product

定义：用户已经给出明确动作，且动作可以被确定性脚本执行，不需要 LLM 推理。

处理方式：

- 尽量不用 LLM。
- 走脚本/规则/状态机。
- 只在缺少关键 ID 时做澄清或读取上下文。

典型：

- “把刚才第一个加入购物车”
- “把购物车第二个删掉”
- “数量改成 2”
- “去结算”

边界：

- 如果用户没有明确商品引用，需要先基于上下文解析；解析不了就澄清。
- 不要把跳转、加购、删除、结算这些固定动作交给主 Agent 自由调用。

### 4.3 non_guide

定义：用户当前核心诉求不是商品选购，但可能需要平台服务、页面跳转、订单售后、优惠权益、闲聊承接。

处理方式：

- 一级路由可用小模型判断。
- 进入 non_guide agent。
- non_guide agent 不暴露导购全量工具，只按需加载 skill。

典型：

- “我的订单在哪”
- “跳到购物车”
- “这个券怎么领”
- “退货规则是什么”
- “你好”

### 4.4 non_guide 与 fast_product 边界

| 用户请求 | route | 原因 |
| --- | --- | --- |
| “打开购物车页面” | `non_guide` | 页面跳转，不修改业务状态 |
| “把这个加购物车” | `fast_product` | 修改购物车状态 |
| “购物车第二个删掉” | `fast_product` | 修改购物车状态 |
| “购物车在哪里” | `non_guide` | 导航/说明 |
| “帮我结算购物车” | `fast_product` | 创建订单 |
| “我的订单到哪了” | `non_guide` | 订单服务查询 |
| “退货怎么退” | `non_guide` | 售后规则 |
| “这款能退货吗” | `guide` 或 `non_guide` | 如果围绕购买决策评估商品风险，偏 guide；如果是已购售后，偏 non_guide |

## 5. Skill 化固定流程

### 5.1 Skill 定义

Skill 是一段固定业务能力，不一定需要 LLM 参与。

建议 MVP skill：

| skill | 能力 | 是否需要 LLM |
| --- | --- | --- |
| `navigate_cart` | 返回前端跳转购物车 action | 否 |
| `navigate_orders` | 返回前端跳转订单 action | 否 |
| `cart_add` | 解析明确 product_id 后加购 | 否 |
| `cart_update` | 修改数量/选中 | 否 |
| `cart_delete` | 删除购物车项 | 否 |
| `checkout` | 创建订单 | 否 |
| `coupon_help` | 优惠券说明/领取入口 | 可选 |
| `order_help` | 订单/物流入口与说明 | 可选 |
| `after_sales_help` | 售后规则说明 | 可选 |

### 5.2 skill 输出协议

Skill 输出结构化 block/action，不靠自然语言硬编码：

```json
{
  "type": "skill_result",
  "skill": "navigate_cart",
  "text": "已为你打开购物车。",
  "blocks": [
    {
      "type": "action",
      "action": {
        "name": "navigate",
        "target": "cart"
      }
    }
  ]
}
```

前端可按 `action.name= navigate`、`target=cart|orders|products` 跳转。

## 6. 动态工具 Prompt 组装

### 6.1 当前问题

现在 `agent.prompt.tool_protocol` 一次性暴露：

- search_products
- search_knowledge
- get_cart
- add_cart_item
- update_cart_item
- delete_cart_item
- checkout

这会导致：

- scene_solution 也看到购物车修改工具。
- non_guide 可能误调用商品搜索。
- 模型在复杂 prompt 下更容易选错工具。

### 6.2 目标结构

把工具说明拆成独立配置项：

```text
agent.tool.search_products
agent.tool.search_knowledge
agent.tool.get_cart
agent.tool.add_cart_item
agent.tool.update_cart_item
agent.tool.delete_cart_item
agent.tool.checkout
agent.skill.navigate_cart
agent.skill.navigate_orders
```

再定义每个 intent 可用工具集合：

```json
{
  "intent": "scene_solution",
  "tools": ["search_products", "search_knowledge"],
  "skills": [],
  "output_blocks": ["inventory", "buyer", "item", "further"]
}
```

### 6.3 P1-P6 工具集合建议

| intent | 可用工具 |
| --- | --- |
| `product_deep` | `search_products`, `search_knowledge` |
| `compare_decide` | `search_products`, `search_knowledge` |
| `outfit_styling` | `search_products`, `search_knowledge` |
| `category_shop_brand` | `search_products`, `search_knowledge` |
| `category_shop_no_brand` | `search_products`, `search_knowledge` |
| `category_shop_complex` | `search_products`, `search_knowledge` |
| `scene_solution` | `search_products`, `search_knowledge` |
| `open_explore` | `search_products`, `search_knowledge` |
| `non_guide` | 按子类加载 skill，例如 `navigate_cart`, `order_help`, `coupon_help` |
| `fast_product` | 不进 ReAct，脚本执行 `cart_add/cart_update/cart_delete/checkout` |

导购意图默认不暴露购物车修改工具。用户要加购时下一轮进入 `fast_product`。

### 6.4 scene_solution prompt 改造

用户给的 scene_solution prompt 方向是对的，但“可用工具与信息侧重”不应写死在 intent prompt 里，而应由工具组装器生成。

拆分后：

#### intent prompt 只描述任务范式

```markdown
当前导购意图是 P5/scene_solution：用户需要特定场景下的整体解决方案与清单。

内容范式：
- 先介绍当前需求场景，说明目标、约束和容易遗漏的风险。
- 根据用户需求列出清单表格，建立品类认知框架。
- 对核心品类逐项说明怎么选，不只堆商品。
- 结合地点、时间、场景、用户画像和品类给最后建议。

输出要求：
- 文案约 900 字。
- 表格后输出一个 `<inventory>`。
- 在具体品类选择建议模块中穿插 `<buyer>` 与 `<item>`，二者必须一起出现。
- 末尾输出一个 `<further>`。
```

#### tool prompt 动态注入

```markdown
本轮可用工具：

1. search_products
参数：
{
  "query": "string",
  "limit": 5,
  "retrieval_intents": ["category", "scenario", "attribute"],
  "entities": {
    "categories": [],
    "attributes": [],
    "scenarios": [],
    "negative_terms": []
  }
}
使用规则：
- 需要商品候选或核心品类候选时调用。
- 场景方案中优先围绕核心品类查，不要一次查过宽。

2. search_knowledge
参数：
{
  "query": "string",
  "limit": 5,
  "retrieval_intents": ["scenario", "category", "attribute"],
  "filters": {
    "doc_types": ["scenario_guide", "buying_guide", "product_detail"]
  }
}
使用规则：
- 需要场景清单、风险、季节、地点、人群约束时调用。
- 必须把检索到的资料片段作为依据，资料不足要说明不足。
```

这样 scene_solution 不会看到 `update_cart_item`。

## 7. Prompt 配置拆分建议

放在 `xzxg-shop-agent-prompts.json` 或 DB Prompt 表中：

```text
agent.prompt.answer_base
agent.prompt.intent.product_deep
agent.prompt.intent.compare_decide
agent.prompt.intent.outfit_styling
agent.prompt.intent.category_shop_brand
agent.prompt.intent.category_shop_no_brand
agent.prompt.intent.category_shop_complex
agent.prompt.intent.scene_solution
agent.prompt.intent.open_explore
agent.prompt.non_guide.base
agent.prompt.tool_header
agent.tool.search_products
agent.tool.search_knowledge
agent.tool.get_cart
agent.tool.add_cart_item
agent.tool.update_cart_item
agent.tool.delete_cart_item
agent.tool.checkout
agent.skill.navigate_cart
agent.skill.navigate_orders
```

其中：

- `agent.prompt.intent.*`：只管任务范式和输出格式。
- `agent.tool.*`：只管工具参数和使用规则。
- `agent.skill.*`：只管固定流程说明和输出 action。
- `agent.prompt.tool_header`：定义 ReAct JSON 协议通用外壳。

## 8. 实施顺序

### Phase 1：Trace Prompt 可视化

1. 新增 `traceLLMWithPrompt`，参数包含 `messages`、`temperature`、`stream`。
2. planner.route、planner.guide_intent、react.step、react.final、followups 都记录实际 messages。
3. 管理员 trace 页面增加 Prompt 折叠展示。

价值：先让后续 prompt 改造可观测。

### Phase 2：弱相关召回与最终挂品保护

1. `search_products/search_knowledge` observation 增加 `relevance_status`、`relevance_reason`、`candidate_product_ids`、`dropped_product_ids`。
2. Milvus 向量召回增加 `vector_min_score` 和 `lexical_guard`，配置从 Nacos 读取。
3. `react.final` 只接收 `relevance_status=ok` 的 `product_ids`。
4. 流式输出过滤器校验 `<item>` 是否在允许列表中，不合法则整段移除。
5. 管理员 trace 展示弱相关候选、剔除原因和最终允许挂品列表。

价值：不靠硬编码品类规则，也能阻断“无库存但误挂无关商品”的问题。

### Phase 3：动态工具 Prompt 组装

1. 建立工具注册表：工具名、参数 schema、使用规则、适用 intent。
2. `reactSystemPromptForPlan` 改为：
   - answer_base
   - intent_prompt
   - tool_header
   - selected_tool_prompts
   - output constraints
3. 导购意图只暴露 search_products/search_knowledge。
4. fast_product 不进入 ReAct。

价值：减少工具误用。

### Phase 4：RAG 检索意图

1. 扩展 `search_products/search_knowledge` 参数。
2. 扩展 `rag.RetrievalPlan`：
   - `Intents []RetrievalIntent`
   - `Entities RetrievalEntities`
3. `retrievalconfig.Apply` 根据 intent 覆盖权重和 doc_types。
4. trace 中展示 retrieval_intents、entities、最终 plan。

价值：让 RAG 召回从“只靠 query”升级为“query + intent + entity + rerank strategy”。

### Phase 5：non_guide skill 化

1. non_guide 二级分类：
   - navigation
   - coupon
   - order_logistics
   - after_sales
   - greeting
   - unknown
2. navigation/cart/order 等固定动作走 skill。
3. 只有需要生成说明文案时才调用小模型。

价值：减少固定业务流程被 LLM 弄复杂。

## 9. 验收标准

1. 管理员 trace 中每个 `llm_call` 都能看到实际 prompt/messages。
2. “马桶推荐”“智能马桶怎么选”“家用坐便器推荐”这类无库存 case 不输出 `<item>`，也不暴露无关商品 ID。
3. `search_products` 对无库存弱召回返回 `relevance_status=weak/no_match`，并且 `product_ids` 为空。
4. 管理员 trace 能看到弱相关候选、剔除商品 ID、剔除原因和 final allowed product IDs。
5. scene_solution 的工具协议里不再出现 `update_cart_item/delete_cart_item/checkout`。
6. fast_product 请求不进入 ReAct LLM 主循环。
7. `search_knowledge` trace 中能看到 `retrieval_intents` 和 `entities`。
8. RAG 测评报告按 query_type 统计后，brand/category/model/scenario 的失败能归因到对应检索策略。
9. Prompt 配置仍可在管理员页面编辑、发布到 Nacos。
