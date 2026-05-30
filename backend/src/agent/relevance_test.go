package agent

import (
	"context"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestProductSearchRelevanceBlocksWeakNoInventoryCandidates(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	result := runtime.classifyProductSearchRelevance(context.Background(), "智能马桶 陶瓷 节水", []domain.ProductCard{
		{
			ProductID:     "p_digital_001",
			Name:          "华为 MatePad 智能平板",
			Brand:         "华为",
			Tags:          []string{"智能设备", "平板"},
			SellingPoints: []string{"高清屏幕"},
		},
	})
	if result.Status != relevanceWeak {
		t.Fatalf("status = %s, want %s", result.Status, relevanceWeak)
	}
	if len(result.AllowedProductIDs) != 0 {
		t.Fatalf("allowed product ids = %v, want empty", result.AllowedProductIDs)
	}
	if len(result.DroppedProductIDs) != 1 || result.DroppedProductIDs[0] != "p_digital_001" {
		t.Fatalf("dropped product ids = %v, want p_digital_001", result.DroppedProductIDs)
	}
}

func TestProductSearchRelevanceKeepsRelevantProduct(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	result := runtime.classifyProductSearchRelevance(context.Background(), "手机推荐", []domain.ProductCard{
		{
			ProductID:     "p_phone_001",
			Name:          "华为 Mate 70 手机",
			Brand:         "华为",
			Tags:          []string{"手机"},
			SellingPoints: []string{"长续航"},
		},
	})
	if result.Status != relevanceOK {
		t.Fatalf("status = %s, want %s", result.Status, relevanceOK)
	}
	if len(result.AllowedProductIDs) != 1 || result.AllowedProductIDs[0] != "p_phone_001" {
		t.Fatalf("allowed product ids = %v, want p_phone_001", result.AllowedProductIDs)
	}
}

func TestStreamTextFilterDropsItemTagContent(t *testing.T) {
	filter := newStreamTextFilter([]string{"p_phone_001"})
	got := filter.Clean("推荐 <item>p_phone_001</item>，以及 <item>p_digital_001</item>。")
	want := "推荐 ，以及 。"
	if got != want {
		t.Fatalf("cleaned = %q, want %q", got, want)
	}
}

func TestStreamTextFilterDropsMarkdownFenceWrapper(t *testing.T) {
	filter := newStreamTextFilter(nil)
	got := filter.Clean("```mar") + filter.Clean("kdown\n# 标题\n内容\n") + filter.Clean("```")
	want := "# 标题\n内容\n"
	if got != want {
		t.Fatalf("cleaned = %q, want %q", got, want)
	}
}

func TestStreamTextFilterDropsBuyerTagContent(t *testing.T) {
	filter := newStreamTextFilter(nil)
	got := filter.Clean("正文<buyer>适合人群</buyer>继续")
	want := "正文继续"
	if got != want {
		t.Fatalf("cleaned = %q, want %q", got, want)
	}
}

func TestStreamTextFilterEmitsAllowedItemOnce(t *testing.T) {
	var emitted []string
	filter := newStreamTextFilter([]string{"p_001"}, func(productID string) {
		emitted = append(emitted, productID)
	})
	got := filter.Clean("a<item>p_001</item>b<item>p_001</item>c<item>p_002</item>d")
	if got != "abcd" {
		t.Fatalf("cleaned = %q, want abcd", got)
	}
	if len(emitted) != 1 || emitted[0] != "p_001" {
		t.Fatalf("emitted = %v, want [p_001]", emitted)
	}
}
