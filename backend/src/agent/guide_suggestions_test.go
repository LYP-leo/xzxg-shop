package agent

import "testing"

func TestParseGuideSuggestionModelOutputKeepsShortQuestions(t *testing.T) {
	items, err := ParseGuideSuggestionModelOutput(`{
		"items": [
			{"id": "a", "question": "帮我选手机", "reason": "搜索页"},
			{"id": "b", "question": "这是一条超过十个中文字的问题", "reason": "过长"},
			{"id": "c", "question": "看优惠？", "reason": "购物车"}
		]
	}`, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 valid items, got %d", len(items))
	}
	if items[0].Question != "帮我选手机" || items[1].Question != "看优惠" {
		t.Fatalf("unexpected questions: %#v", items)
	}
}

func TestParseGuideSuggestionModelOutputRejectsNoValidItems(t *testing.T) {
	_, err := ParseGuideSuggestionModelOutput(`{"items":[{"question":"这是一条超过十个中文字的问题"}]}`, 3)
	if err == nil {
		t.Fatal("expected error")
	}
}
