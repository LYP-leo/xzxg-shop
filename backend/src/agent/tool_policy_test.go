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
