package store

import (
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestRankProductSearchResultsUsesBrandAndCategoryFacets(t *testing.T) {
	items := []domain.ProductCard{
		{
			ProductID:     "p_digital_004",
			Name:          "华为HUAWEI MateBook 14 鸿蒙版 14英寸轻薄高性能生产力笔记本",
			Brand:         "华为",
			CategoryID:    "c_dataset_digital_laptop",
			Tags:          []string{"数码电子", "笔记本电脑", "华为"},
			SellingPoints: []string{"笔记本电脑", "华为", "存储：16GB+512GB"},
		},
		{
			ProductID:     "p_digital_005",
			Name:          "华为HUAWEI MatePad Pro Max 12.6英寸高刷屏多任务办公平板电脑",
			Brand:         "华为",
			CategoryID:    "c_dataset_digital_tablet",
			Tags:          []string{"数码电子", "平板电脑", "华为"},
			SellingPoints: []string{"平板电脑", "华为", "存储规格：8GB+256GB"},
		},
		{
			ProductID:     "p_digital_006",
			Name:          "Apple MacBook Pro 14英寸 M5 芯片 16GB 512GB 专业高性能笔记本电脑",
			Brand:         "Apple 苹果",
			CategoryID:    "c_dataset_digital_laptop",
			Tags:          []string{"数码电子", "笔记本电脑", "Apple 苹果"},
			SellingPoints: []string{"笔记本电脑", "Apple 苹果", "屏幕尺寸：14英寸"},
		},
	}

	got := rankProductSearchResults("华为电脑 笔记本", items)
	if len(got) != 1 || got[0].ProductID != "p_digital_004" {
		t.Fatalf("expected only Huawei MateBook, got %#v", productIDs(got))
	}

	got = rankProductSearchResults("苹果电脑 MacBook", items)
	if len(got) != 1 || got[0].ProductID != "p_digital_006" {
		t.Fatalf("expected only Apple MacBook, got %#v", productIDs(got))
	}
}

func productIDs(items []domain.ProductCard) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
	}
	return ids
}
