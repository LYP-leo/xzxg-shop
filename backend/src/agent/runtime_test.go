package agent

import (
	"log/slog"
	"testing"
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
