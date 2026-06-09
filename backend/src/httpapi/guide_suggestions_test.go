package httpapi

import "testing"

func TestBuildGuideSuggestionsProducts(t *testing.T) {
	items, errCode := buildGuideSuggestions(guideSuggestionRequest{
		Page: "products",
		Context: map[string]any{
			"keyword":       "手机",
			"category_name": "数码电子",
		},
		Limit: 2,
	})
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Question != "帮我选手机" {
		t.Fatalf("unexpected first question: %q", items[0].Question)
	}
}

func TestBuildGuideSuggestionsRejectsBadPage(t *testing.T) {
	_, errCode := buildGuideSuggestions(guideSuggestionRequest{Page: "unknown"})
	if errCode != "bad_page" {
		t.Fatalf("expected bad_page, got %q", errCode)
	}
}

func TestBuildGuideSuggestionsClampsLimit(t *testing.T) {
	items, errCode := buildGuideSuggestions(guideSuggestionRequest{
		Page:  "cart",
		Limit: 10,
		Context: map[string]any{
			"cart_item_count": 5,
		},
	})
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if len(items) != 3 {
		t.Fatalf("expected limit to clamp to 3, got %d", len(items))
	}
	for _, item := range items {
		if len([]rune(item.Question)) > 10 {
			t.Fatalf("question should be <= 10 runes: %#v", item)
		}
	}
}
