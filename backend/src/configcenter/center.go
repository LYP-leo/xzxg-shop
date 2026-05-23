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

const DefaultPlannerPrompt = DefaultRoutePrompt

const DefaultAnswerBasePrompt = `你是小猪小狗电商平台的 AI 导购主 Agent。

上下文信息：
- 用户输入来自当前轮 query。
- 意图由前置路由器给出，可能是商品推荐、商品对比、商品详情问答、优惠促销、售后咨询或购物决策。
- 候选商品和资料片段由系统检索提供，只能作为可引用依据，不能当作全部事实。

动态能力约束：
- 你可以在内部使用 ReAct 思路决定是否依赖候选商品、知识片段和当前意图，但不要输出推理过程、Action、Observation 或工具调用文本。
- 根据用户需求和检索结果选择回答角度；不要为了覆盖信息而堆砌无关商品。
- 如果候选商品或资料片段与问题不相关，必须说明“当前资料不足”，并提出下一步澄清问题。
- 不要编造不存在的价格、库存、优惠、售后承诺、参数、商品能力、品牌关系或政策规则。

工具信息使用规范：
- 推荐/对比时，优先使用商品名、价格、卖点、风险提示、库存状态和知识片段中的明确依据。
- 多商品对比时，围绕用户关注维度比较；不要做泛泛的排行榜。
- 资料不足时不要硬答，改为询问预算、品类、使用场景、偏好、候选商品或人群约束。
- 单次回答聚焦 1 个主建议；如有多个候选，最多展开 3 个。

输出规范：
- 输出中文自然语言。
- 先给结论，再给 2 到 4 条依据，最后给 1 到 2 个可执行追问或下一步建议。
- 不输出 JSON，不输出 markdown 表格，不输出内部标签，不输出隐藏推理。`

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
		{ConfigKey: "ai.base_url", ConfigValue: "https://dashscope.aliyuncs.com/compatible-mode/v1", ValueType: "string", Description: "OpenAI 兼容模型服务 Base URL"},
		{ConfigKey: "ai.small_model", ConfigValue: "qwen3.5-flash", ValueType: "string", Description: "低成本小模型，用于意图识别和轻量回答"},
		{ConfigKey: "ai.large_model", ConfigValue: "qwen3.6-plus", ValueType: "string", Description: "复杂导购决策模型"},
		{ConfigKey: "ai.api_key", ConfigValue: envAPIKey, ValueType: "string", Description: "模型服务 API Key，列表接口脱敏", IsSecret: true},
		{ConfigKey: "ai.enabled", ConfigValue: "true", ValueType: "bool", Description: "是否启用真实模型调用"},
		{ConfigKey: "ai.enable_thinking", ConfigValue: "false", ValueType: "bool", Description: "是否启用模型思考模式，默认关闭以降低首 token 延迟"},
		{ConfigKey: "agent.followups_enabled", ConfigValue: "true", ValueType: "bool", Description: "是否生成追问"},
		{ConfigKey: "agent.config_refresh_seconds", ConfigValue: "15", ValueType: "int", Description: "运行时动态配置刷新间隔秒数"},
		{ConfigKey: "agent.prompt.route", ConfigValue: DefaultRoutePrompt, ValueType: "text", Description: "一级路由 Prompt：guide/non_guide/fast_product"},
		{ConfigKey: "agent.prompt.guide_intent", ConfigValue: DefaultGuideIntentPrompt, ValueType: "text", Description: "导购细分 Prompt：P1-P6"},
		{ConfigKey: "agent.prompt.planner", ConfigValue: DefaultPlannerPrompt, ValueType: "text", Description: "兼容旧配置：等同 agent.prompt.route"},
		{ConfigKey: "agent.prompt.answer_base", ConfigValue: DefaultAnswerBasePrompt, ValueType: "text", Description: "主导购 Agent 基础系统 Prompt"},
		{ConfigKey: "agent.prompt.followups", ConfigValue: DefaultFollowupsPrompt, ValueType: "text", Description: "追问生成 Agent 系统 Prompt"},
	}
	for _, intent := range DefaultIntentKeys() {
		configs = append(configs, domain.AppConfig{
			ConfigKey:   "agent.prompt.intent." + intent,
			ConfigValue: defaultIntentPrompts[intent],
			ValueType:   "text",
			Description: "主导购 Agent 意图 Prompt：" + intent,
		})
	}
	return configs
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
		IsSecret:    input.IsSecret,
		UpdatedAt:   time.Now(),
	}
	if item.ValueType == "" {
		item.ValueType = "string"
	}
	return item
}
