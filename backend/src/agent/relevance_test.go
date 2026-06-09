package agent

import (
	"context"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestProductSearchRelevanceBlocksWeakNoInventoryCandidatesWhenLexicalGuardEnabled(t *testing.T) {
	configs := configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))
	_, _ = configs.Upsert(context.Background(), domain.AppConfigInput{
		ConfigKey:   "retrieval.product.lexical_guard.enabled",
		ConfigValue: "true",
		ValueType:   "bool",
	})
	runtime := &Runtime{configs: configs}
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
		t.Fatalf("status = %s, want %s; reason=%s", result.Status, relevanceOK, result.Reason)
	}
	if len(result.AllowedProductIDs) != 1 || result.AllowedProductIDs[0] != "p_phone_001" {
		t.Fatalf("allowed product ids = %v, want p_phone_001", result.AllowedProductIDs)
	}
}

func TestProductSearchRelevanceDropsStructuredNegativeBrand(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	result := runtime.classifyProductSearchRelevanceWithRun(context.Background(), domain.AgentRun{}, "电脑", []domain.ProductCard{
		{
			ProductID:     "p_apple_mac",
			Name:          "Apple MacBook Air 13英寸 M5 芯片",
			Brand:         "Apple 苹果",
			CategoryID:    "c_dataset_digital_laptop",
			Tags:          []string{"笔记本电脑", "Apple 苹果"},
			SellingPoints: []string{"轻薄便携"},
		},
		{
			ProductID:     "p_huawei_pc",
			Name:          "华为 MateBook 14 笔记本电脑",
			Brand:         "华为",
			CategoryID:    "c_dataset_digital_laptop",
			Tags:          []string{"笔记本电脑", "华为"},
			SellingPoints: []string{"轻薄办公"},
		},
	}, productSearchStructuredArguments{
		Negative: productSearchArguments{Brands: []string{"苹果", "Apple"}},
	}, nil)
	if result.Status != relevanceOK {
		t.Fatalf("status = %s, want %s; reason=%s", result.Status, relevanceOK, result.Reason)
	}
	if len(result.AllowedProductIDs) != 1 || result.AllowedProductIDs[0] != "p_huawei_pc" {
		t.Fatalf("allowed product ids = %v, want p_huawei_pc", result.AllowedProductIDs)
	}
	if len(result.DroppedProductIDs) != 1 || result.DroppedProductIDs[0] != "p_apple_mac" {
		t.Fatalf("dropped product ids = %v, want p_apple_mac", result.DroppedProductIDs)
	}
}

func TestProductSearchRerankPrioritizesStructuredConstraints(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	result := runtime.rerankProductsForSearch(context.Background(), "华为电脑 笔记本", []domain.ProductCard{
		{
			ProductID:     "p_huawei_tablet",
			Name:          "华为 MatePad Pro 平板电脑",
			Brand:         "华为",
			CategoryID:    "c_dataset_digital_tablet",
			Tags:          []string{"平板电脑", "华为"},
			SellingPoints: []string{"多任务办公"},
		},
		{
			ProductID:     "p_huawei_pc",
			Name:          "华为 MateBook 14 笔记本电脑",
			Brand:         "华为",
			CategoryID:    "c_dataset_digital_laptop",
			Tags:          []string{"笔记本电脑", "华为"},
			SellingPoints: []string{"轻薄办公"},
		},
	}, productSearchStructuredArguments{
		Constraints: productSearchArguments{
			Brands:     []string{"华为"},
			Categories: []string{"笔记本电脑"},
		},
	}, nil)
	if len(result.Products) != 2 {
		t.Fatalf("products len = %d, want 2", len(result.Products))
	}
	if result.Products[0].ProductID != "p_huawei_pc" {
		t.Fatalf("first product = %s, want p_huawei_pc; scores=%#v", result.Products[0].ProductID, result.Scores)
	}
}

func TestModelRerankScoreFilterDropsLowConfidenceTail(t *testing.T) {
	products := []domain.ProductCard{
		{ProductID: "p_beauty_003", Name: "SK-II神仙水"},
		{ProductID: "p_dummyjson_140", Name: "篮球"},
		{ProductID: "p_dummyjson_141", Name: "篮球框"},
	}
	filtered := filterProductsByModelRerankScore(products, []productRerankScore{
		{ProductID: "p_beauty_003", Score: 0.6298},
		{ProductID: "p_dummyjson_140", Score: 0.4744},
		{ProductID: "p_dummyjson_141", Score: 0.4667},
	}, map[string]string{
		"retrieval.product.rerank.model_min_score":       "0.50",
		"retrieval.product.rerank.model_max_score_delta": "0.12",
	})
	if len(filtered) != 1 || filtered[0].ProductID != "p_beauty_003" {
		t.Fatalf("filtered = %v, want only p_beauty_003", productCardIDs(filtered))
	}
}

func TestProductSearchNegativeHardFilterRunsBeforeRerank(t *testing.T) {
	products := []domain.ProductCard{
		{
			ProductID:     "p_apple_mac",
			Name:          "Apple MacBook Air 笔记本电脑",
			Brand:         "Apple 苹果",
			Tags:          []string{"笔记本电脑", "Apple 苹果"},
			SellingPoints: []string{"轻薄办公"},
		},
		{
			ProductID:     "p_huawei_pc",
			Name:          "华为 MateBook 14 笔记本电脑",
			Brand:         "华为",
			Tags:          []string{"笔记本电脑", "华为"},
			SellingPoints: []string{"轻薄办公"},
		},
	}
	allowed, dropped := filterProductsByStructuredNegative(products, productSearchArguments{Brands: []string{"苹果", "Apple"}})
	if len(allowed) != 1 || allowed[0].ProductID != "p_huawei_pc" {
		t.Fatalf("allowed = %v, want only p_huawei_pc", productCardIDs(allowed))
	}
	if len(dropped) != 1 || dropped[0] != "p_apple_mac" {
		t.Fatalf("dropped = %v, want p_apple_mac", dropped)
	}
}

func TestProductSearchRelevanceKeepsCategoryProductWhenQueryHasCoveredTerms(t *testing.T) {
	runtime := &Runtime{configs: configcenter.NewMemoryCenter(configcenter.DefaultConfigs(""))}
	result := runtime.classifyProductSearchRelevance(context.Background(), "化妆品 护肤 彩妆", []domain.ProductCard{
		{
			ProductID:     "p_dummyjson_001",
			Name:          "Essence 卷翘浓密睫毛膏 Lash Princess",
			Brand:         "Essence",
			CategoryID:    "c_dataset_beauty_personal_care_makeup",
			Tags:          []string{"美妆个护", "彩妆", "Essence"},
			SellingPoints: []string{"彩妆", "Essence"},
		},
	})
	if result.Status != relevanceOK {
		t.Fatalf("status = %s, want %s; reason=%s", result.Status, relevanceOK, result.Reason)
	}
	if len(result.AllowedProductIDs) != 1 || result.AllowedProductIDs[0] != "p_dummyjson_001" {
		t.Fatalf("allowed product ids = %v, want p_dummyjson_001", result.AllowedProductIDs)
	}
}

func TestProductRelevanceTermsDropCoveredSubterms(t *testing.T) {
	got := productRelevanceTerms("化妆品 护肤 彩妆", "")
	for _, term := range []string{"化妆", "妆品"} {
		for _, actual := range got {
			if actual == term {
				t.Fatalf("terms = %v, should not contain covered subterm %q", got, term)
			}
		}
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
	filter := newStreamTextFilter([]string{"p_001"}, streamFilterCallbacks{
		OnItem: func(productID string) {
			emitted = append(emitted, productID)
		},
	})
	got := filter.Clean("a<item>p_001</item>b<item>p_001</item>c<item>p_002</item>d")
	if got != "abcd" {
		t.Fatalf("cleaned = %q, want abcd", got)
	}
	if len(emitted) != 1 || emitted[0] != "p_001" {
		t.Fatalf("emitted = %v, want [p_001]", emitted)
	}
}

func TestStreamTextFilterEmitsStructuredServiceBlock(t *testing.T) {
	var blocks []domain.AgentBlock
	filter := newStreamTextFilter(nil, streamFilterCallbacks{
		OnBlock: func(block domain.AgentBlock) {
			blocks = append(blocks, block)
		},
	})
	got := filter.Clean(`前文<coupon_list>{"title":"可用券","items":[{"coupon_id":"c_1","name":"满100减10"}],"summary":{"count":1}}</coupon_list>后文`)
	if got != "前文后文" {
		t.Fatalf("cleaned = %q, want 前文后文", got)
	}
	if len(blocks) != 1 {
		t.Fatalf("blocks len = %d, want 1", len(blocks))
	}
	if blocks[0].Type != "coupon_list" || blocks[0].Title != "可用券" || len(blocks[0].Items) != 1 {
		t.Fatalf("block = %#v", blocks[0])
	}
}

func TestBlocksFromReactDoesNotAutoAppendAllToolProducts(t *testing.T) {
	blocks := blocksFromReact(reactAction{}, []string{"p_beauty_003", "p_dummyjson_140"}, []string{"chunk_001"})
	if len(blocks) != 1 {
		t.Fatalf("blocks len = %d, want only citation block: %#v", len(blocks), blocks)
	}
	if blocks[0].Type != "citation_refs" {
		t.Fatalf("block type = %s, want citation_refs", blocks[0].Type)
	}
}

func TestBlocksFromReactFiltersExplicitProductRefsByAllowedIDs(t *testing.T) {
	blocks := blocksFromReact(reactAction{Blocks: []reactBlock{
		{Type: "product_refs", ProductIDs: []string{"p_beauty_003", "p_dummyjson_140"}},
	}}, []string{"p_beauty_003"}, nil)
	if len(blocks) != 1 {
		t.Fatalf("blocks len = %d, want 1: %#v", len(blocks), blocks)
	}
	if len(blocks[0].ProductIDs) != 1 || blocks[0].ProductIDs[0] != "p_beauty_003" {
		t.Fatalf("product ids = %v, want [p_beauty_003]", blocks[0].ProductIDs)
	}
}
