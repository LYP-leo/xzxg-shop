package agent

import (
	"context"
	"log/slog"
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
		{Route: "non_guide", Intent: "non_guide"},
	}
	for _, tc := range cases {
		if got := runtime.answerModelForPlan(tc); got != "large" {
			t.Fatalf("answerModelForPlan(%+v) = %q, want large", tc, got)
		}
	}
}

func TestAnswerModelForPlanKeepsSmallForFastProduct(t *testing.T) {
	runtime := NewRuntime(nil, nil, slog.Default(), RuntimeConfig{
		Models: ModelConfig{
			SmallModel: "small",
			LargeModel: "large",
		},
	})

	cases := []runPlan{
		{Route: "fast_product", Intent: "cart_add"},
		{Route: "fast_product", Intent: "checkout_confirm"},
	}
	for _, tc := range cases {
		if got := runtime.answerModelForPlan(tc); got != "small" {
			t.Fatalf("answerModelForPlan(%+v) = %q, want small", tc, got)
		}
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
