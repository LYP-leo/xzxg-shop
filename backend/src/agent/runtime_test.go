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

func TestNormalizeNonGuideIntentClassifiesServiceDomains(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"我最近的订单到哪了", "order_service"},
		{"我有哪些优惠券可以用", "coupon_service"},
		{"这个商品差评主要说什么", "review_service"},
		{"退货怎么退", "after_sales_service"},
		{"打开购物车页面", "navigation_service"},
		{"我的收货地址在哪里改", "account_service"},
		{"你好", "chitchat"},
	}
	for _, tc := range cases {
		if got := normalizeNonGuideIntent(tc.query, ""); got != tc.want {
			t.Fatalf("normalizeNonGuideIntent(%q) = %q, want %q", tc.query, got, tc.want)
		}
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
