package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
)

type intentToolPolicy struct {
	Tools    []string `json:"tools"`
	Skills   []string `json:"skills"`
	Disabled []string `json:"disabled"`
	Focus    []string `json:"focus"`
}

func (r *Runtime) intentToolPolicyPrompt(ctx context.Context, plan runPlan) string {
	policy := r.policyForPlan(ctx, plan)
	intent := plan.ReferenceIntent()
	var builder strings.Builder
	builder.WriteString("当前子意图工具策略：\n\n")
	builder.WriteString("可用工具：\n")
	writeNamedList(&builder, policy.Tools, toolDescriptions())
	builder.WriteString("\n可用 skill：\n")
	writeNamedList(&builder, policy.Skills, skillDescriptions())
	builder.WriteString("\n禁用能力：\n")
	writePlainList(&builder, policy.Disabled)
	builder.WriteString("\n工具使用侧重：\n")
	writePlainList(&builder, policy.Focus)
	builder.WriteString("\n硬约束：\n")
	builder.WriteString("- 本轮只能调用“可用工具”列表中的工具；禁止调用“禁用能力”中的工具。\n")
	builder.WriteString("- 如果需要的能力不在可用工具或可用 skill 中，输出 final 澄清或说明能力边界，不要绕过策略。\n")
	builder.WriteString("- search_products 返回 relevance_status=weak/no_match 时，不得把候选商品当作推荐或挂品。\n")
	builder.WriteString(fmt.Sprintf("- 当前 route=%s，intent=%s。\n", plan.Route, intent))
	return builder.String()
}

func (r *Runtime) availableToolsPrompt(ctx context.Context, plan runPlan) string {
	policy := r.policyForPlan(ctx, plan)
	return namedListText(policy.Tools, toolDescriptions())
}

func (r *Runtime) availableSkillsPrompt(ctx context.Context, plan runPlan) string {
	policy := r.policyForPlan(ctx, plan)
	return namedListText(policy.Skills, skillDescriptions())
}

func (r *Runtime) toolFocusPrompt(ctx context.Context, plan runPlan) string {
	policy := r.policyForPlan(ctx, plan)
	lines := append([]string{}, policy.Focus...)
	if containsString(policy.Tools, toolSearchProducts) {
		lines = append(lines,
			"商品推荐、商品对比、商品详情、价格、库存、卖点、风险提示，必须先调用 search_products。",
			"调用 search_products 时，query 优先使用品牌/产品线/核心品类/明确型号，保持短而稳定；写代码、做演示、办公、通勤、送礼等使用场景不要塞进首次检索 query。",
		)
	}
	if containsString(policy.Tools, toolSearchKnowledge) {
		lines = append(lines, "平台规则、选购知识、材料解释、售后边界，必要时调用 search_knowledge。")
	}
	if containsString(policy.Tools, toolAddCartItem) {
		lines = append(lines, "加购前必须有明确 product_id；不能根据购物车内容、列表位置或猜测的 product_id 加购。")
	}
	if containsString(policy.Tools, toolUpdateCartItem) || containsString(policy.Tools, toolDeleteCartItem) {
		lines = append(lines, "修改或删除购物车前必须有 cart_item_id；如果用户按序号描述，先调用 get_cart。")
	}
	if containsString(policy.Tools, toolCheckout) {
		lines = append(lines, "checkout 前先调用 get_cart，确认存在选中商品。")
	}
	lines = append(lines,
		"本轮只能调用“可用工具”列表中的工具；禁止调用未列出的工具。",
		"如果需要的能力不在可用工具或可用 skill 中，输出 final 澄清或说明能力边界。",
	)
	return plainListText(uniqueNonEmpty(lines))
}

func (r *Runtime) finalOutputRulesPrompt(ctx context.Context, plan runPlan) string {
	rules := strings.TrimSpace(r.stringConfig(ctx, "agent.prompt.final_output_rules", configcenter.DefaultFinalOutputRulesPrompt))
	policy := r.policyForPlan(ctx, plan)
	extras := make([]string, 0, 2)
	if containsString(policy.Tools, toolSearchKnowledge) {
		extras = append(extras, "search_knowledge 返回的资料片段只能作为依据引用，不要编造未出现在工具结果中的内容。")
	}
	if containsString(policy.Tools, toolSearchProducts) {
		extras = append(extras, "商品检索结果如果没有可靠命中，不得把候选商品当作推荐或挂品；只能用用户能理解的话说明当前商品库暂时没有找到符合条件的商品。")
	}
	if len(extras) > 0 {
		rules = strings.TrimSpace(rules + "\n" + plainListText(extras))
	}
	return rules
}

func (r *Runtime) toolAllowedForPlan(ctx context.Context, plan runPlan, tool string) bool {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return false
	}
	policy := r.policyForPlan(ctx, plan)
	if len(policy.Tools) == 0 {
		return true
	}
	for _, item := range policy.Tools {
		if item == tool {
			return true
		}
	}
	return false
}

func (r *Runtime) skillAllowedForPlan(ctx context.Context, plan runPlan, skill string) bool {
	skill = strings.TrimSpace(skill)
	if skill == "" {
		return false
	}
	policy := r.policyForPlan(ctx, plan)
	if len(policy.Skills) == 0 {
		return false
	}
	for _, item := range policy.Skills {
		if item == skill {
			return true
		}
	}
	return false
}

func (r *Runtime) policyForPlan(ctx context.Context, plan runPlan) intentToolPolicy {
	policies := parseIntentToolPolicies(r.stringConfig(ctx, "agent.prompt.intent_tool_policy", configcenter.DefaultIntentToolPolicyPrompt))
	keys := []string{plan.ReferenceIntent(), plan.Route}
	if plan.Route == "guide" {
		keys = append(keys, "category_shop_no_brand")
	}
	keys = append(keys, "non_guide")
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if policy, ok := policies[key]; ok {
			return normalizeIntentToolPolicy(policy)
		}
	}
	return intentToolPolicy{Tools: []string{toolSearchProducts, toolSearchKnowledge}}
}

func parseIntentToolPolicies(raw string) map[string]intentToolPolicy {
	var policies map[string]intentToolPolicy
	if err := json.Unmarshal([]byte(raw), &policies); err == nil && len(policies) > 0 {
		return policies
	}
	var fallback map[string]intentToolPolicy
	_ = json.Unmarshal([]byte(configcenter.DefaultIntentToolPolicyPrompt), &fallback)
	return fallback
}

func normalizeIntentToolPolicy(policy intentToolPolicy) intentToolPolicy {
	policy.Tools = uniqueNonEmpty(policy.Tools)
	policy.Skills = uniqueNonEmpty(policy.Skills)
	policy.Disabled = uniqueNonEmpty(policy.Disabled)
	policy.Focus = uniqueNonEmpty(policy.Focus)
	sort.Strings(policy.Tools)
	sort.Strings(policy.Skills)
	sort.Strings(policy.Disabled)
	return policy
}

func writeNamedList(builder *strings.Builder, items []string, descriptions map[string]string) {
	if len(items) == 0 {
		builder.WriteString("- 无\n")
		return
	}
	for _, item := range items {
		if description := strings.TrimSpace(descriptions[item]); description != "" {
			builder.WriteString(fmt.Sprintf("- %s：%s\n", item, description))
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s\n", item))
	}
}

func namedListText(items []string, descriptions map[string]string) string {
	var builder strings.Builder
	writeNamedList(&builder, items, descriptions)
	return strings.TrimSpace(builder.String())
}

func writePlainList(builder *strings.Builder, items []string) {
	if len(items) == 0 {
		builder.WriteString("- 无\n")
		return
	}
	for _, item := range items {
		builder.WriteString(fmt.Sprintf("- %s\n", item))
	}
}

func plainListText(items []string) string {
	var builder strings.Builder
	writePlainList(&builder, items)
	return strings.TrimSpace(builder.String())
}

func uniqueNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func toolDescriptions() map[string]string {
	return map[string]string{
		toolSearchProducts:  `搜索当前商品库，用于查找可推荐商品、价格、库存、卖点和风险；只允许推荐 relevance_status=ok 的商品。参数 {"query":"正向商品关键词，2-4个词，不要包含否定词","limit":5,"constraints":{"brands":[],"terms":[],"categories":[]},"negative":{"brands":[],"terms":[],"categories":[]}}。用户说“不要/不买/排除/非 某品牌或属性”时，query 只放正向需求，把被排除项放到 negative。`,
		toolSearchKnowledge: `搜索知识库资料，用于查找选购依据、场景清单、平台规则、材料解释和售后边界。参数 {"query":"需要查证的问题","limit":3}。`,
		toolGetCart:         `读取当前用户购物车，用于确认 cart_item_id、选中状态和数量。参数 {}。`,
		toolAddCartItem:     `加入购物车；必须已有明确 product_id。参数 {"product_id":"商品ID","sku_id":"SKU ID，可为空","quantity":1}。`,
		toolUpdateCartItem:  `修改购物车项数量或选中状态；必须已有 cart_item_id。参数 {"cart_item_id":"购物车项ID","quantity":2,"selected":true}。`,
		toolDeleteCartItem:  `删除购物车项；必须已有 cart_item_id。参数 {"cart_item_id":"购物车项ID"}。`,
		toolCheckout:        `基于当前选中购物车项创建待支付订单。参数 {}。`,
	}
}

func skillDescriptions() map[string]string {
	return map[string]string{
		"navigate_cart":     "返回前端跳转购物车的 action。",
		"navigate_orders":   "返回前端跳转订单页的 action。",
		"navigate_products": "返回前端跳转商品列表页的 action。",
		"coupon_help":       "说明优惠券入口、领取方式和使用边界。",
		"order_help":        "说明订单、物流、支付状态入口和需要的信息。",
		"after_sales_help":  "说明退换货、售后规则和下一步入口。",
	}
}
