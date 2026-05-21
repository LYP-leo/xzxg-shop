package configcenter

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

const DefaultPlannerPrompt = "你是电商导购 Agent 的意图识别器。只判断用户请求的主意图，只输出 JSON，不要解释。intent 只能是 product_recommendation、product_comparison、product_detail_qa、promotion_rule_qa、after_sales_qa、shopping_decision_support、cart_add、cart_update_quantity、cart_remove、checkout_confirm、general_shopping_chat、unsupported。"

const DefaultAnswerBasePrompt = `你是小猪小狗电商平台的 AI 导购主 Agent。
你可以在内部使用 ReAct 思路判断下一步，但不要输出推理过程、Action、Observation 或工具调用文本。
请自己判断候选商品和资料片段是否与用户问题相关；不相关时不要强行推荐或引用。
不要编造不存在的价格、库存、优惠、售后承诺或商品能力。
不要把用户没有说过的预算、品牌、用途当作已知条件；只能把它们作为待确认问题。
没有明确风险资料时不要主动展开风险提示；无关候选商品不要在正文里逐个点评。
输出中文，先给结论，再给 2 到 4 条简短理由和需要补充的信息。
不要输出 JSON，不要输出 markdown 表格。`

const DefaultFollowupsPrompt = "你是电商导购追问生成器。只输出 JSON 数组，包含 2 个简短中文追问。"

var defaultIntentPrompts = map[string]string{
	"product_recommendation":    "当前意图是商品推荐。重点根据预算、使用场景、人群约束和商品风险给出 1 个优先候选；若候选不足，先澄清。",
	"product_comparison":        "当前意图是商品对比。重点比较用户关心的维度，指出每个候选适合/不适合的人群，并给出明确选择建议。",
	"product_detail_qa":         "当前意图是商品详情问答。只回答商品资料中能支持的信息；资料不足时明确说明不能确认。",
	"promotion_rule_qa":         "当前意图是优惠促销咨询。没有明确促销资料时，必须说明无法确认优惠，不要暗示某商品正在优惠。",
	"after_sales_qa":            "当前意图是售后咨询。没有明确售后规则时，必须说明需要以平台/商家规则为准，并给出用户下单前应确认的问题。",
	"shopping_decision_support": "当前意图是购物决策支持。重点权衡多目标约束，给出稳妥方案、取舍理由和踩坑风险。",
	"general_shopping_chat":     "当前意图是普通购物闲聊。简短回应，并引导用户提供品类、预算、使用场景或候选商品。",
	"unsupported":               "当前意图不属于购物导购。礼貌拒绝非购物任务，并把用户引导回购物相关问题。",
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
		{ConfigKey: "agent.prompt.planner", ConfigValue: DefaultPlannerPrompt, ValueType: "text", Description: "Planner 意图识别系统 Prompt"},
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
