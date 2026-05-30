package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestFormatQueryWithMemoryIncludesReferencedProduct(t *testing.T) {
	runtime := &Runtime{}
	query := runtime.formatQueryWithMemory("把刚才那个加入购物车。", conversationMemory{
		HasRelevantMemory:    true,
		MemorySummary:        "上一轮推荐了 Quiet Mouse S。",
		ReferencedProductIDs: []string{"p_mouse_001"},
		Records: []domain.ConversationRecord{
			{
				RunID:      "run_1",
				ProductIDs: []string{"p_mouse_001"},
				ProductRefs: []domain.ProductCard{
					{ProductID: "p_mouse_001", SkuID: "sku_mouse_001", Name: "Quiet Mouse S", Price: "129.00"},
				},
			},
		},
	})

	for _, want := range []string{"当前用户问题", "相关会话记忆", "来自第1轮的第1个商品", "item_id=p_mouse_001", "sku_id=sku_mouse_001", "Quiet Mouse S"} {
		if !strings.Contains(query, want) {
			t.Fatalf("formatted query missing %q:\n%s", want, query)
		}
	}
}

func TestMemoryRetrievalUserPromptLabelsTurnsAndProductOrder(t *testing.T) {
	runtime := NewRuntime(nil, configcenter.NewMemoryCenter(configcenter.DefaultConfigs("")), slog.Default(), RuntimeConfig{})
	prompt := runtime.memoryRetrievalUserPrompt(context.Background(), "把第一个面霜加到购物车里", []domain.ConversationRecord{
		{
			RunID:       "run_1",
			UserQuery:   "推荐面霜",
			FinalAnswer: "第一款适合修护。",
			ProductIDs:  []string{"p_beauty_007", "p_beauty_012"},
			CreatedAt:   time.Date(2026, 5, 30, 15, 58, 0, 0, time.Local),
		},
		{
			RunID:       "run_2",
			UserQuery:   "把第一个商品加到购物车",
			FinalAnswer: "已加购鼠标。",
			ProductIDs:  []string{"p_mouse_001"},
			CreatedAt:   time.Date(2026, 5, 30, 15, 59, 0, 0, time.Local),
		},
	})

	for _, want := range []string{
		"第1轮：record_id=run_1",
		"第2轮【最新一轮】：record_id=run_2",
		"本轮商品顺序",
		"第1个商品 item_id=p_beauty_007",
		"第2个商品 item_id=p_beauty_012",
		"第一个/第二个/第N个 + 品类词",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("memory retrieval prompt missing %q:\n%s", want, prompt)
		}
	}
}
