package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestAnswerModelForPlanUsesLargeForReactRoutes(t *testing.T) {
	runtime := NewRuntime(nil, nil, slog.Default(), RuntimeConfig{
		Models: ModelConfig{
			SmallModel: "small",
			LargeModel: "large",
		},
	})

	cases := []runPlan{
		{Route: "guide", Intent: "product_deep"},
		{Route: "guide", Intent: "category_shop_no_brand"},
	}
	for _, tc := range cases {
		if got := runtime.answerModelForPlan(context.Background(), tc); got != "large" {
			t.Fatalf("answerModelForPlan(%+v) = %q, want large", tc, got)
		}
	}
}

func TestNormalizeQueryWithImageAttachmentRewritesGenericQuery(t *testing.T) {
	got := normalizeQueryWithAttachments("???", []domain.Attachment{
		{AttachmentID: "file_abc", Type: "image", URL: "/api/v1/files/file_abc"},
	})
	want := "找同款 [附件图片链接:/api/v1/files/file_abc]"
	if got != want {
		t.Fatalf("normalizeQueryWithAttachments = %q, want %q", got, want)
	}
}

func TestNormalizeQueryWithImageAttachmentAppendsSpecificQuery(t *testing.T) {
	got := normalizeQueryWithAttachments("这个适合通勤吗", []domain.Attachment{
		{FileID: "file_xyz", Type: "image"},
	})
	for _, want := range []string{"这个适合通勤吗", "[附件图片链接:/api/v1/files/file_xyz]", "search_image_products"} {
		if !strings.Contains(got, want) {
			t.Fatalf("normalized query missing %q:\n%s", want, got)
		}
	}
}

func TestAnswerModelForPlanUsesConfiguredRoleModel(t *testing.T) {
	center := configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))
	_, _ = center.Upsert(context.Background(), domain.AppConfigInput{
		ConfigKey:   "ai.model.react_guide",
		ConfigValue: "configured-guide",
		ValueType:   "string",
		Domain:      "app",
	})
	runtime := NewRuntime(nil, center, slog.Default(), RuntimeConfig{
		Models: ModelConfig{
			SmallModel: "small",
			LargeModel: "large",
		},
	})
	if got := runtime.answerModelForPlan(context.Background(), runPlan{Route: "guide", Intent: "product_deep"}); got != "configured-guide" {
		t.Fatalf("answerModelForPlan configured role = %q, want configured-guide", got)
	}
}

func TestAnswerModelForPlanKeepsSmallForToolIntents(t *testing.T) {
	runtime := NewRuntime(nil, nil, slog.Default(), RuntimeConfig{
		Models: ModelConfig{
			SmallModel: "small",
			LargeModel: "large",
		},
	})

	cases := []runPlan{
		{Route: "non_guide", Intent: "cart_add"},
		{Route: "non_guide", Intent: "checkout_confirm"},
	}
	for _, tc := range cases {
		if got := runtime.answerModelForPlan(context.Background(), tc); got != "small" {
			t.Fatalf("answerModelForPlan(%+v) = %q, want small", tc, got)
		}
	}
}

func TestNormalizeNonGuideIntentAcceptsPlannerOutputOnly(t *testing.T) {
	cases := []struct {
		intent string
		want   string
	}{
		{"cart_add", "cart_service"},
		{"cart_remove", "cart_service"},
		{"cart_update_quantity", "cart_service"},
		{"checkout_confirm", "order_service"},
		{"cart_service", "cart_service"},
		{"order_service", "order_service"},
		{"coupon_service", "coupon_service"},
		{"review_service", "review_service"},
		{"after_sales_service", "after_sales_service"},
		{"navigation_service", "navigation_service"},
		{"chitchat", "chitchat"},
		{"unknown_intent", "unsupported"},
	}
	for _, tc := range cases {
		if got := normalizeNonGuideIntent(tc.intent, "unsupported"); got != tc.want {
			t.Fatalf("normalizeNonGuideIntent(%q) = %q, want %q", tc.intent, got, tc.want)
		}
	}
}

func TestNormalizeNonGuideIntentDoesNotInferFromQueryText(t *testing.T) {
	effectiveQuery := `当前用户问题：
我是干性皮肤

相关会话记忆：
- 用户上一轮询问化妆品推荐。

相关历史商品（后续如需加购或引用商品，优先使用这里的 item_id/product_id）：
- 来自第1轮的第5个商品 item_id=p_beauty_009 sku_id=s_p_beauty_009_1 name=珀莱雅双抗精华 price=210.00

记忆使用规则：只把相关会话记忆作为指代消解和上下文补充。`

	if got := normalizeNonGuideIntent(effectiveQuery, "unsupported"); got == "cart_add" {
		t.Fatalf("normalizeNonGuideIntent = %q, want non cart_add", got)
	}
}

func TestCurrentActionQueryIgnoresMemoryActionText(t *testing.T) {
	effectiveQuery := `当前用户问题：
请你结算这两件商品

相关会话记忆：
- 用户上一轮已将第一件和第二件商品加入购物车。

相关历史商品（后续如需加购或引用商品，优先使用这里的 item_id/product_id）：
- 来自第1轮的第1个商品 item_id=p_dummyjson_002 sku_id=s_p_dummyjson_002_1 name=Glamour Beauty 带镜眼影盘 price=143.93
- 来自第1轮的第2个商品 item_id=p_dummyjson_001 sku_id=s_p_dummyjson_001_1 name=Essence 卷翘浓密睫毛膏 Lash Princess price=71.93

记忆使用规则：只把相关会话记忆作为指代消解和上下文补充；最终回答仍直接回应当前用户问题，不要复述这些规则。`

	actionQuery := currentActionQuery(effectiveQuery)
	if actionQuery != "请你结算这两件商品" {
		t.Fatalf("currentActionQuery = %q", actionQuery)
	}
	if !looksCheckout(actionQuery) {
		t.Fatalf("current action query should trigger checkout")
	}
	if looksCartAdd(actionQuery) {
		t.Fatalf("current action query should not trigger cart add")
	}
}

func TestNavigationPolicyDisablesTools(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	plan := runPlan{Route: "non_guide", Intent: "navigation_service"}
	if runtime.toolAllowedForPlan(context.Background(), plan, toolGetCart) {
		t.Fatalf("navigation_service should not allow tools")
	}
	if !runtime.skillAllowedForPlan(context.Background(), plan, "navigate_cart") {
		t.Fatalf("navigation_service should allow navigate_cart")
	}
}

func TestRefreshDynamicConfigUsesActiveProvider(t *testing.T) {
	center := configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "ai.active_provider", ConfigValue: "doubao"},
		{ConfigKey: "ai.base_url", ConfigValue: "https://dashscope.example/v1"},
		{ConfigKey: "ai.api_key", ConfigValue: "qwen-key"},
		{ConfigKey: "ai.small_model", ConfigValue: "qwen-small"},
		{ConfigKey: "ai.large_model", ConfigValue: "qwen-large"},
		{ConfigKey: "ai.doubao.base_url", ConfigValue: "https://ark.example/api/v3/"},
		{ConfigKey: "ai.doubao.api_key", ConfigValue: "doubao-key"},
		{ConfigKey: "ai.doubao.small_model", ConfigValue: "doubao-small"},
		{ConfigKey: "ai.doubao.large_model", ConfigValue: "doubao-large"},
	})
	runtime := NewRuntime(nil, center, slog.Default(), RuntimeConfig{})

	runtime.refreshDynamicConfig(context.Background())

	config := runtime.llm.Config()
	if config.BaseURL != "https://ark.example/api/v3/" {
		t.Fatalf("BaseURL = %q, want doubao base url", config.BaseURL)
	}
	if config.APIKey != "doubao-key" {
		t.Fatalf("APIKey = %q, want doubao key", config.APIKey)
	}
	if config.SmallModel != "doubao-small" {
		t.Fatalf("SmallModel = %q, want doubao small model", config.SmallModel)
	}
	if config.LargeModel != "doubao-large" {
		t.Fatalf("LargeModel = %q, want doubao large model", config.LargeModel)
	}
}

func TestRefreshDynamicConfigFallsBackToLegacyModelConfig(t *testing.T) {
	center := configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "ai.active_provider", ConfigValue: "missing"},
		{ConfigKey: "ai.base_url", ConfigValue: "https://dashscope.example/v1"},
		{ConfigKey: "ai.api_key", ConfigValue: "qwen-key"},
		{ConfigKey: "ai.small_model", ConfigValue: "qwen-small"},
		{ConfigKey: "ai.large_model", ConfigValue: "qwen-large"},
	})
	runtime := NewRuntime(nil, center, slog.Default(), RuntimeConfig{})

	runtime.refreshDynamicConfig(context.Background())

	config := runtime.llm.Config()
	if config.BaseURL != "https://dashscope.example/v1" {
		t.Fatalf("BaseURL = %q, want legacy base url", config.BaseURL)
	}
	if config.APIKey != "qwen-key" {
		t.Fatalf("APIKey = %q, want legacy key", config.APIKey)
	}
	if config.SmallModel != "qwen-small" {
		t.Fatalf("SmallModel = %q, want legacy small model", config.SmallModel)
	}
	if config.LargeModel != "qwen-large" {
		t.Fatalf("LargeModel = %q, want legacy large model", config.LargeModel)
	}
}
