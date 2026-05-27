package configcenter

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

const DefaultRoutePrompt = `你是电商前置意图路由器。根据当前用户 query，只判断它应该进入哪个下游 pipeline。只输出 JSON，不要输出解释文本。

一级路由：
- guide：用户核心诉求是选购、对比、搭配、场景方案、开放探索、商品详情咨询。
- non_guide：用户核心诉求不是选购商品，而是优惠权益、价格提醒、页面跳转、订单/物流、评价/口碑、取件/驿站、复购、售后投诉、平台活动、闲聊、知识地点咨询、无意义文本等。
- fast_product：用户已明确要求把商品加入购物车、修改购物车、删除购物车或确认下单。

判定优先级：
1. fast_product：加入购物车、修改购物车、删除购物车、确认下单。
2. non_guide：命中非导购强信号。
3. guide：兜底原则，无法明确判定时归 guide。

non_guide 强信号：
- 优惠券、红包、会员权益、返利、领券、卡券、活动兑换。
- 价格提醒、降价提醒、到价提醒。
- 页面跳转、平台功能入口、购物车查看、订单、物流、催发货、评价查询、取件码、复购、退货退款、改地址、发票、投诉。
- 闲聊、地点/知识/技术原理、无意义文本。
即使 query 中出现商品名，只要核心动作是上述非导购动作，也判 non_guide。

guide 典型场景：
- 单品咨询：含具体品牌+型号/商品名，问详情、参数、评价、用法、价格。
- 品类选购：含具体品类词，如跑鞋、防水冲锋衣、纸巾、猫粮。
- 对比决策：两个及以上候选，并含比较、区别、哪个更好、怎么选。
- 穿搭/搭配：服饰、鞋靴、包包等含搭配、穿搭、造型动作。
- 场景方案：露营清单、厨房收纳、徒步装备等跨品类方案。
- 开放探索：风格、IP、美学、模糊描述但有潜在购买意图。

输出格式：
{
  "reasoning": "中文，80字以内，说明判定依据",
  "route": "guide|non_guide|fast_product"
}

字段约束：
- route 只能输出 guide、non_guide、fast_product 三者之一。
- reasoning 要简洁说明命中了哪类信号词，或为什么走兜底。
- 不输出任何 JSON 之外的内容。`

const DefaultGuideIntentPrompt = `你是电商导购意图细分类器。当前 query 已被前置路由判定为 guide，你只需要把它细分到 P1-P6 中唯一一个类别。只输出 JSON，不要输出解释文本。

判定顺序：
P1 product_deep -> P2 compare_decide -> P3 outfit_styling -> P4 category_shop -> P5 scene_solution -> P6 open_explore。
信息不足时也必须输出最可能类别。

导购意图 P1-P6：
P1 product_deep：已锚定具体商品、品牌+型号/系列/规格、商品链接或唯一商品，问详情、评价、参数、用法、价格。
P2 compare_decide：有两个及以上候选，并包含对比、区别、哪个更好、怎么选等比较选择语义。
P3 outfit_styling：服饰/鞋靴/包包等含明确搭配、穿搭、造型动作。
P4 category_shop：有品类、店铺或品牌泛选方向，但未锚定唯一商品。若有具体品牌/店铺，secondary_level=P4A；无品牌/店铺，secondary_level=P4B；含预算/价格/多个硬属性筛选或继续看更多，secondary_level=P4C。
P5 scene_solution：场景驱动的跨品类清单或方案，例如露营、厨房收纳、徒步装备、搭配灵感、必备清单。
P6 open_explore：有潜在购买意图，但没有明确品类，只由风格、IP、美学、模糊描述或纯探索构成。若出现明确品类词，不能判 P6，应回到 P4/P1/P2。

关键边界：
- “品牌+型号/系列/具体规格/商品链接/淘口令”能唯一定位商品 -> P1。
- “品牌/店铺/平台名 + 品类泛选”但不能唯一定位商品 -> P4A。
- “纯品类 + 推荐/怎么选/好用吗/预算/价格/属性筛选” -> P4B 或 P4C。
- 预算、价格、多硬属性、继续看更多、换一批、下一页等复杂筛选 -> P4C。
- 两个及以上候选 + 比较语义 -> P2，不受候选是品牌/商品/品类/属性维度限制。
- 穿搭/搭配/造型动作 + 服饰鞋包 -> P3。
- 场景关键词 + 多品类清单/方案/必备/收纳/搭配灵感 -> P5。
- 只有风格/IP/美学/模糊探索，没有任何具体品类锚点 -> P6。

输出格式：
{
  "reasoning": "中文，150字以内，说明为什么属于该类",
  "is_guide": "Yes",
  "intent": "product_deep|compare_decide|outfit_styling|category_shop_brand|category_shop_no_brand|category_shop_complex|scene_solution|open_explore",
  "level": "P1|P2|P3|P4|P5|P6|None",
  "secondary_level": "P4A|P4B|P4C|None"
}

字段约束：
- is_guide 固定为 Yes。
- P4A 对应 category_shop_brand，P4B 对应 category_shop_no_brand，P4C 对应 category_shop_complex。
- level=P4 时 secondary_level 必须是 P4A/P4B/P4C；非 P4 时 secondary_level=None。
- 不输出任何 JSON 之外的内容。`

const DefaultAnswerBasePrompt = `你是小猪小狗电商平台的 AI 导购主 Agent。

上下文信息：
- 用户输入来自当前轮 query。
- 意图由前置路由器给出，可能是商品推荐、商品对比、商品详情问答、优惠促销、售后咨询或购物决策。
- 商品、知识库、购物车和订单信息都必须通过工具获取；没有工具结果时不能编造事实。

动态能力约束：
- 你必须按工具协议先决定是否调用工具，信息足够后再输出 final。
- 根据用户需求和工具结果选择回答角度；不要为了覆盖信息而堆砌无关商品。
- 如果工具结果与问题不相关，必须说明“当前资料不足”，并提出下一步澄清问题。
- 不要编造不存在的价格、库存、优惠、售后承诺、参数、商品能力、品牌关系或政策规则。

工具信息使用规范：
- 推荐/对比时，优先使用商品名、价格、卖点、风险提示、库存状态和知识片段中的明确依据。
- 多商品对比时，围绕用户关注维度比较；不要做泛泛的排行榜。
- 资料不足时不要硬答，改为询问预算、品类、使用场景、偏好、候选商品或人群约束。
- 单次回答聚焦 1 个主建议；如有多个候选，最多展开 3 个。

输出规范：
- ReAct 决策阶段只能输出工具协议 JSON。
- 最终回答阶段输出中文自然语言。
- 最终回答先给结论，再给 2 到 4 条依据，最后给 1 到 2 个可执行追问或下一步建议。
- 可使用 Markdown 的短标题、列表和加粗来突出重点词；禁止输出 "special_word"、"special word"、"（special_word）" 等内部标识。
- 如果引用挂品标签 <item>...</item>，标签内容必须是商品 ID，例如 <item>p_001</item>；禁止在 <item> 内放商品名、品牌名或自然语言挂品指令。
- 不输出 markdown 表格，不输出隐藏推理。`

const DefaultToolProtocolPrompt = `你正在一个 ReAct 工具循环中工作。每一步只能输出一个 JSON 对象，不能输出 Markdown、解释文字或隐藏推理。

可用工具：
1. search_products：搜索商品。参数 {"query":"用户需求或商品关键词","limit":5}
2. search_knowledge：搜索知识库资料。参数 {"query":"需要查证的问题","limit":3}
3. get_cart：读取当前用户购物车。参数 {}
4. add_cart_item：加入购物车。参数 {"product_id":"商品ID","sku_id":"SKU ID，可为空","quantity":1}
5. update_cart_item：修改购物车项。参数 {"cart_item_id":"购物车项ID","quantity":2,"selected":true}
6. delete_cart_item：删除购物车项。参数 {"cart_item_id":"购物车项ID"}
7. checkout：基于当前选中购物车项创建待支付订单。参数 {}

输出格式二选一：
工具调用：
{
  "type": "tool_call",
  "tool": "search_products|search_knowledge|get_cart|add_cart_item|update_cart_item|delete_cart_item|checkout",
  "arguments": {}
}

最终决策：
{
  "type": "final",
  "text": "给最终回答阶段的简短依据，不要写长文",
  "blocks": [
    {"type": "product_refs", "product_ids": ["p_001"]},
    {"type": "citation_refs", "chunk_ids": ["k_001"]}
  ]
}

规则：
- 商品推荐、商品对比、商品详情、价格、库存、卖点、风险提示，必须先调用 search_products。
- 平台规则、选购知识、材料解释、售后边界，必要时调用 search_knowledge。
- 加购前必须有明确 product_id；如果用户只说“第一个/刚才那个”，先根据已有 observation 判断，不能判断则重新 search_products 或 final 澄清。
- 修改/删除购物车前必须有 cart_item_id；如果用户按序号描述，先调用 get_cart。
- checkout 前建议先调用 get_cart，确认存在选中商品。
- 不要伪造商品 ID、购物车项 ID、价格、库存、优惠或订单。
- 如果用户问题不是导购或工具动作，可以直接 final，简短说明能力边界或给出自然问候。
- final blocks 只允许 product_refs 和 citation_refs。
- 如需在 text 中额外输出 <item> 标签，<item> 内只能写已由工具返回的 product_id，例如 <item>p_001</item>；不能写商品名或推荐语。
- 重点词、品牌词、系列词不要用 special_word 标识，统一用 Markdown 加粗，例如 **耐克**。`

const DefaultIntentToolPolicyPrompt = `{
  "product_deep": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先围绕用户锚定的商品或型号调用 search_products。", "需要解释参数、材料、售后或选购依据时再调用 search_knowledge。"]
  },
  "compare_decide": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先分别检索候选商品，确保对比对象来自当前商品库。", "围绕用户提到的维度比较，不要引入无关候选。"]
  },
  "outfit_styling": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先把风格、场合、颜色、版型约束转成可检索品类。", "search_products 用于找可购买单品，search_knowledge 用于补充搭配原则。"]
  },
  "category_shop_brand": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["检索时保留品牌/店铺和品类约束。", "需要品牌系列或选购知识时调用 search_knowledge。"]
  },
  "category_shop_no_brand": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先按品类、预算、场景、人群和硬属性收敛 query。", "search_products 是主工具，必要时用 search_knowledge 补充选购依据。"]
  },
  "category_shop_complex": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先拆解预算、价格、属性、排除项等硬约束，再检索商品。", "不要为了凑结果推荐违反硬约束的商品。"]
  },
  "scene_solution": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先建立场景清单框架，再对核心品类查资料和商品。", "search_products 优先围绕核心品类逐项查询。"]
  },
  "open_explore": {
    "tools": ["search_products", "search_knowledge"],
    "skills": [],
    "disabled": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["先把用户的风格、IP、美学或模糊诉求收敛成可购买品类。", "可先 search_knowledge 获取品类框架，再 search_products 找探索式候选。"]
  },
  "non_guide": {
    "tools": ["get_cart"],
    "skills": ["navigate_cart", "navigate_orders", "navigate_products", "coupon_help", "order_help", "after_sales_help"],
    "disabled": ["search_products", "search_knowledge", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "focus": ["非导购请求优先用自然语言或 skill 承接，不主动搜索商品。", "只有读取购物车状态时允许 get_cart。"]
  },
  "fast_product": {
    "tools": ["get_cart", "add_cart_item", "update_cart_item", "delete_cart_item", "checkout"],
    "skills": [],
    "disabled": ["search_knowledge"],
    "focus": ["固定商品动作优先脚本化执行。", "缺少 product_id 或 cart_item_id 时先澄清或 get_cart，不要猜 ID。"]
  }
}`

const DefaultFollowupsPrompt = `你是电商导购追问生成器。根据用户原始问题和本轮回答，生成 2 个能推进购买决策的简短中文追问。只输出 JSON 数组。
要求：
- 追问必须围绕预算、使用场景、人群、偏好、候选商品、尺寸规格、售后顾虑之一。
- 不要重复主回答中已经明确的信息。
- 不要生成平台无关、闲聊或营销口号。`

var defaultIntentPrompts = map[string]string{
	"product_deep":           "当前导购意图是 P1/product_deep：用户已锚定具体商品。只围绕该商品回答详情、参数、评价、用法、价格等问题；资料不足时明确说明不能确认，并让用户补充型号、链接或规格。",
	"compare_decide":         "当前导购意图是 P2/compare_decide：用户在多个候选之间比较或决策。围绕候选和比较维度给明确选择建议，说明适合/不适合人群，不要加入无关候选。",
	"outfit_styling":         "当前导购意图是 P3/outfit_styling：用户需要穿搭、搭配或造型建议。优先给风格、场合、颜色/版型搭配逻辑，再推荐可购买品类或候选商品。",
	"category_shop_brand":    "当前导购意图是 P4A/category_shop_brand：用户有品牌/店铺/平台名和品类泛选方向。围绕该品牌或店铺做筛选，避免把描述性风格词误当品牌。",
	"category_shop_no_brand": "当前导购意图是 P4B/category_shop_no_brand：用户有明确品类但无品牌/店铺。按预算、使用场景、硬属性、人群偏好收敛候选，优先推荐 1 个最稳商品。",
	"category_shop_complex":  "当前导购意图是 P4C/category_shop_complex：用户带有预算、价格、多属性、复杂筛选或继续看更多诉求。先拆解硬约束，再给符合条件的候选和取舍。",
	"scene_solution":         "当前导购意图是 P5/scene_solution：用户需要场景驱动的跨品类清单或方案。先给场景方案结构，再按必要性推荐关键品类和候选商品。",
	"open_explore":           "当前导购意图是 P6/open_explore：用户有潜在购买意图但没有明确品类。先把风格/IP/美学描述收敛为可购买品类，再给探索式建议和澄清问题。",
	"non_guide":              "当前请求是 non_guide：核心诉求不是商品选购。简短说明当前导购能力边界；如果属于优惠、售后、订单、物流等平台服务，引导用户到对应页面或补充订单/商品信息。",
}

type Center interface {
	List(ctx context.Context, includeSecrets bool) []domain.AppConfig
	GetMap(ctx context.Context) map[string]string
	Upsert(ctx context.Context, input domain.AppConfigInput) (domain.AppConfig, error)
}

type MemoryCenter struct {
	mu      sync.RWMutex
	configs map[string]domain.AppConfig
}

func NewMemoryCenter(defaults []domain.AppConfig) *MemoryCenter {
	configs := make(map[string]domain.AppConfig, len(defaults))
	for _, item := range defaults {
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = time.Now()
		}
		configs[item.ConfigKey] = item
	}
	return &MemoryCenter{configs: configs}
}

func DefaultConfigs(envAPIKey string) []domain.AppConfig {
	configs := []domain.AppConfig{
		{ConfigKey: "ai.base_url", ConfigValue: "https://dashscope.aliyuncs.com/compatible-mode/v1", ValueType: "string", Description: "OpenAI 兼容模型服务 Base URL", Domain: "app"},
		{ConfigKey: "ai.small_model", ConfigValue: "qwen3.5-flash", ValueType: "string", Description: "低成本小模型，用于意图识别和轻量回答", Domain: "app"},
		{ConfigKey: "ai.large_model", ConfigValue: "qwen3.6-plus", ValueType: "string", Description: "复杂导购决策模型", Domain: "app"},
		{ConfigKey: "ai.api_key", ConfigValue: envAPIKey, ValueType: "string", Description: "模型服务 API Key，列表接口脱敏", Domain: "app", IsSecret: true},
		{ConfigKey: "ai.enabled", ConfigValue: "true", ValueType: "bool", Description: "是否启用真实模型调用", Domain: "app"},
		{ConfigKey: "ai.enable_thinking", ConfigValue: "false", ValueType: "bool", Description: "是否启用模型思考模式，默认关闭以降低首 token 延迟", Domain: "app"},
		{ConfigKey: "vector.enabled", ConfigValue: "true", ValueType: "bool", Description: "是否启用 Milvus 向量召回；不可用时自动降级关键词检索", Domain: "infra"},
		{ConfigKey: "milvus.address", ConfigValue: "http://127.0.0.1:19530", ValueType: "string", Description: "Milvus REST 地址", Domain: "infra"},
		{ConfigKey: "milvus.token", ConfigValue: "root:Milvus", ValueType: "string", Description: "Milvus Token，本地 standalone 默认 root:Milvus", Domain: "infra", IsSecret: true},
		{ConfigKey: "milvus.database", ConfigValue: "", ValueType: "string", Description: "Milvus 数据库，空表示 default", Domain: "infra"},
		{ConfigKey: "milvus.collection.products", ConfigValue: "product_text_vectors", ValueType: "string", Description: "商品文本向量 collection", Domain: "infra"},
		{ConfigKey: "milvus.collection.knowledge", ConfigValue: "knowledge_text_chunks", ValueType: "string", Description: "知识片段文本向量 collection", Domain: "infra"},
		{ConfigKey: "embedding.base_url", ConfigValue: "https://dashscope.aliyuncs.com/compatible-mode/v1", ValueType: "string", Description: "Embedding OpenAI 兼容 Base URL", Domain: "infra"},
		{ConfigKey: "embedding.model", ConfigValue: "text-embedding-v4", ValueType: "string", Description: "文本 Embedding 模型", Domain: "infra"},
		{ConfigKey: "embedding.batch_size", ConfigValue: "10", ValueType: "int", Description: "Embedding 批量大小", Domain: "infra"},
		{ConfigKey: "retrieval.keyword.top_n", ConfigValue: "200", ValueType: "int", Description: "关键词召回候选数，宽 query 需要更大的候选池再重排", Domain: "rag"},
		{ConfigKey: "retrieval.vector.top_n", ConfigValue: "80", ValueType: "int", Description: "向量召回候选数", Domain: "rag"},
		{ConfigKey: "retrieval.vector.min_score", ConfigValue: "0.58", ValueType: "float", Description: "Milvus 向量召回最低相似度，低于该分数直接丢弃", Domain: "rag"},
		{ConfigKey: "retrieval.product.lexical_guard.enabled", ConfigValue: "true", ValueType: "bool", Description: "商品检索词面保护，弱相关候选不得进入最终挂品列表", Domain: "rag"},
		{ConfigKey: "retrieval.product.lexical_guard.min_evidence_count", ConfigValue: "1", ValueType: "int", Description: "商品候选至少需要命中的有效词面证据数", Domain: "rag"},
		{ConfigKey: "retrieval.product.lexical_guard.min_match_ratio", ConfigValue: "0.35", ValueType: "float", Description: "商品候选有效词面命中比例阈值；低于阈值时降为弱相关", Domain: "rag"},
		{ConfigKey: "retrieval.product.ok.max_results", ConfigValue: "10", ValueType: "int", Description: "商品检索相关结果最多进入 Agent 的数量", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.title_match", ConfigValue: "0.30", ValueType: "float", Description: "RAG 重排：标题命中基础权重", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.term_match", ConfigValue: "0.35", ValueType: "float", Description: "RAG 重排：query term 命中比例权重", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.required_match", ConfigValue: "0.15", ValueType: "float", Description: "RAG 重排：强约束 term 命中权重", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.brand_boost", ConfigValue: "0.45", ValueType: "float", Description: "RAG 重排：品牌精确命中加权", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.model_boost", ConfigValue: "0.35", ValueType: "float", Description: "RAG 重排：型号/系列命中加权", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.category_boost", ConfigValue: "0.20", ValueType: "float", Description: "RAG 重排：品类命中加权", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.weight.generic_penalty", ConfigValue: "0.50", ValueType: "float", Description: "RAG 重排：只命中推荐/怎么选等泛词时降权", Domain: "rag"},
		{ConfigKey: "retrieval.rerank.generic_terms", ConfigValue: "推荐,怎么选,同类,区别,性价比,通勤,办公,好喝,不腻,续航,性能,修护", ValueType: "string", Description: "RAG 重排泛意图词，命中这些词不应主导召回", Domain: "rag"},
		{ConfigKey: "agent.followups_enabled", ConfigValue: "true", ValueType: "bool", Description: "是否生成追问", Domain: "app"},
		{ConfigKey: "agent.config_refresh_seconds", ConfigValue: "15", ValueType: "int", Description: "运行时动态配置刷新间隔秒数", Domain: "app"},
		{ConfigKey: "agent.prompt.route", ConfigValue: DefaultRoutePrompt, ValueType: "text", Description: "一级路由 Prompt：guide/non_guide/fast_product", Domain: "prompt"},
		{ConfigKey: "agent.prompt.guide_intent", ConfigValue: DefaultGuideIntentPrompt, ValueType: "text", Description: "导购细分 Prompt：P1-P6", Domain: "prompt"},
		{ConfigKey: "agent.prompt.answer_base", ConfigValue: DefaultAnswerBasePrompt, ValueType: "text", Description: "主导购 Agent 基础系统 Prompt", Domain: "prompt"},
		{ConfigKey: "agent.prompt.tool_protocol", ConfigValue: DefaultToolProtocolPrompt, ValueType: "text", Description: "主 Agent ReAct 工具协议 Prompt", Domain: "prompt"},
		{ConfigKey: "agent.prompt.intent_tool_policy", ConfigValue: DefaultIntentToolPolicyPrompt, ValueType: "json", Description: "各子意图可用工具、skill、禁用能力和使用侧重", Domain: "prompt"},
		{ConfigKey: "agent.prompt.followups", ConfigValue: DefaultFollowupsPrompt, ValueType: "text", Description: "追问生成 Agent 系统 Prompt", Domain: "prompt"},
	}
	for _, intent := range DefaultIntentKeys() {
		configs = append(configs, domain.AppConfig{
			ConfigKey:   "agent.prompt.intent." + intent,
			ConfigValue: defaultIntentPrompts[intent],
			ValueType:   "text",
			Description: "主导购 Agent 意图 Prompt：" + intent,
			Domain:      "prompt",
		})
	}
	return configs
}

func PromptDefaults() []domain.AgentPromptInput {
	defaults := []domain.AgentPromptInput{
		{PromptKey: "agent.prompt.route", Title: "一级路由 Prompt", Content: DefaultRoutePrompt, Description: "guide/non_guide/fast_product 路由"},
		{PromptKey: "agent.prompt.guide_intent", Title: "导购细分 Prompt", Content: DefaultGuideIntentPrompt, Description: "P1-P6 导购意图识别"},
		{PromptKey: "agent.prompt.answer_base", Title: "主 Agent 基础 Prompt", Content: DefaultAnswerBasePrompt, Description: "主导购 Agent 系统提示词"},
		{PromptKey: "agent.prompt.tool_protocol", Title: "工具协议 Prompt", Content: DefaultToolProtocolPrompt, Description: "ReAct 工具调用协议"},
		{PromptKey: "agent.prompt.intent_tool_policy", Title: "意图工具策略 Prompt", Content: DefaultIntentToolPolicyPrompt, Description: "按 route/intent 注入可用工具、skill 和禁用能力"},
		{PromptKey: "agent.prompt.followups", Title: "追问生成 Prompt", Content: DefaultFollowupsPrompt, Description: "导购追问生成"},
	}
	for _, intent := range DefaultIntentKeys() {
		defaults = append(defaults, domain.AgentPromptInput{
			PromptKey:   "agent.prompt.intent." + intent,
			Title:       "意图 Prompt：" + intent,
			Content:     defaultIntentPrompts[intent],
			Description: "主导购 Agent 意图 Prompt：" + intent,
		})
	}
	return defaults
}

func DefaultIntentKeys() []string {
	keys := make([]string, 0, len(defaultIntentPrompts))
	for key := range defaultIntentPrompts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func DefaultIntentPrompt(intent string) string {
	return defaultIntentPrompts[intent]
}

func (c *MemoryCenter) List(ctx context.Context, includeSecrets bool) []domain.AppConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := make([]domain.AppConfig, 0, len(c.configs))
	for _, item := range c.configs {
		if item.IsSecret && !includeSecrets && item.ConfigValue != "" {
			item.ConfigValue = "******"
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })
	return items
}

func (c *MemoryCenter) GetMap(ctx context.Context) map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	values := make(map[string]string, len(c.configs))
	for key, item := range c.configs {
		values[key] = item.ConfigValue
	}
	return values
}

func (c *MemoryCenter) Upsert(ctx context.Context, input domain.AppConfigInput) (domain.AppConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item := normalizeInput(input)
	c.configs[item.ConfigKey] = item
	return item, nil
}

func normalizeInput(input domain.AppConfigInput) domain.AppConfig {
	item := domain.AppConfig{
		ConfigKey:   strings.TrimSpace(input.ConfigKey),
		ConfigValue: input.ConfigValue,
		ValueType:   input.ValueType,
		Description: input.Description,
		Domain:      input.Domain,
		IsSecret:    input.IsSecret,
		UpdatedAt:   time.Now(),
	}
	if item.Domain == "" {
		item.Domain = DomainForKey(item.ConfigKey)
	}
	if item.ValueType == "" {
		item.ValueType = "string"
	}
	return item
}

func DomainForKey(key string) string {
	switch {
	case strings.HasPrefix(key, "agent.prompt."):
		return "prompt"
	case strings.HasPrefix(key, "retrieval."):
		return "rag"
	case strings.HasPrefix(key, "vector."), strings.HasPrefix(key, "milvus."), strings.HasPrefix(key, "embedding."):
		return "infra"
	default:
		return "app"
	}
}
