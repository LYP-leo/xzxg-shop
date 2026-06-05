package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestIntentToolPolicyPromptInjectsSceneSolutionTools(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	plan := runPlan{Route: "guide", Intent: "scene_solution"}
	prompt := runtime.intentToolPolicyPrompt(context.Background(), plan)
	for _, want := range []string{"可用工具", toolSearchProducts, toolSearchKnowledge, "可用 skill", "禁用能力", toolUpdateCartItem, "工具使用侧重"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("policy prompt missing %q:\n%s", want, prompt)
		}
	}
	if !runtime.toolAllowedForPlan(context.Background(), plan, toolSearchProducts) {
		t.Fatalf("search_products should be allowed for scene_solution")
	}
	if runtime.toolAllowedForPlan(context.Background(), plan, toolUpdateCartItem) {
		t.Fatalf("update_cart_item should be blocked for scene_solution")
	}
	if runtime.skillAllowedForPlan(context.Background(), plan, skillNavigateCart) {
		t.Fatalf("navigate_cart should be blocked for scene_solution")
	}
}

func TestIntentToolPolicyFallbacksWhenConfigIsBadJSON(t *testing.T) {
	center := configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))
	_, _ = center.Upsert(context.Background(), domain.AppConfigInput{
		ConfigKey:   "agent.prompt.intent_tool_policy",
		ConfigValue: "{bad json",
		ValueType:   "json",
		Domain:      "prompt",
	})
	runtime := &Runtime{configs: center}
	plan := runPlan{Route: "guide", Intent: "category_shop_no_brand"}
	if !runtime.toolAllowedForPlan(context.Background(), plan, toolSearchProducts) {
		t.Fatalf("fallback policy should allow search_products")
	}
	if runtime.toolAllowedForPlan(context.Background(), plan, toolCheckout) {
		t.Fatalf("fallback policy should block checkout for guide intent")
	}
}

func TestIntentToolPolicyAllowsNonGuideSkills(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	plan := runPlan{Route: "non_guide", Intent: "non_guide"}
	if !runtime.skillAllowedForPlan(context.Background(), plan, skillNavigateCart) {
		t.Fatalf("navigate_cart should be allowed for non_guide")
	}
	if !runtime.skillAllowedForPlan(context.Background(), plan, skillNavigateProducts) {
		t.Fatalf("navigate_products should be allowed for non_guide")
	}
	if runtime.toolAllowedForPlan(context.Background(), plan, toolSearchProducts) {
		t.Fatalf("search_products should be blocked for non_guide")
	}
}

func TestFinalOutputRulesOnlyInjectsServiceBlocksForNonGuide(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}

	guideRules := runtime.finalOutputRulesPrompt(context.Background(), runPlan{Route: "guide", Intent: "category_shop_no_brand"})
	if strings.Contains(guideRules, "<coupon_list>") {
		t.Fatalf("guide final rules should not contain non-guide service tags:\n%s", guideRules)
	}

	nonGuideRules := runtime.finalOutputRulesPrompt(context.Background(), runPlan{Route: "non_guide", Intent: "coupon_service"})
	if !strings.Contains(nonGuideRules, "<coupon_list>") {
		t.Fatalf("non-guide final rules should contain service tags:\n%s", nonGuideRules)
	}
}

func TestFinalOutputRulesStripsLegacyServiceBlockRulesFromGlobalConfig(t *testing.T) {
	center := configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))
	_, _ = center.Upsert(context.Background(), domain.AppConfigInput{
		ConfigKey: "agent.prompt.final_output_rules",
		ConfigValue: `- 通用规则
- 非导购服务结果如果需要前端结构化渲染，可以在 <final> 内输出以下 XML-like 标签；标签内部只能放 JSON 对象，后端会解析为 block_delta，不会把标签原文展示给用户：
  - <coupon_list>{"title":"优惠券"}</coupon_list>
- 结构化标签中的数据必须来自已调用工具或 skill 的 observation；禁止为了生成卡片编造优惠券、订单、评价或政策。
- 不输出隐藏推理。`,
		ValueType: "text",
		Domain:    "prompt",
	})
	runtime := &Runtime{configs: center}
	guideRules := runtime.finalOutputRulesPrompt(context.Background(), runPlan{Route: "guide", Intent: "category_shop_no_brand"})
	if strings.Contains(guideRules, "<coupon_list>") || strings.Contains(guideRules, "非导购服务结果") {
		t.Fatalf("legacy service rules should be stripped from guide rules:\n%s", guideRules)
	}
	if !strings.Contains(guideRules, "不输出隐藏推理") {
		t.Fatalf("strip should keep following generic rules:\n%s", guideRules)
	}
}
