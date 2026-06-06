package ingest

import (
	"context"
	"strings"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestParseHTMLNormalizesVisibleText(t *testing.T) {
	parsed, err := Parse(context.Background(), domain.UnstructuredIngestRequest{
		Title:      "跑鞋帖子",
		SourceType: "html",
		HTML:       `<html><head><style>.x{}</style><script>alert(1)</script></head><body><h1>跑鞋怎么选</h1><p>关注缓震和足弓支撑。</p></body></html>`,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Title != "跑鞋帖子" || parsed.DocType != "web_article" {
		t.Fatalf("unexpected metadata: %#v", parsed)
	}
	for _, want := range []string{"跑鞋怎么选", "关注缓震和足弓支撑"} {
		if !strings.Contains(parsed.Content, want) {
			t.Fatalf("content missing %q: %q", want, parsed.Content)
		}
	}
	if strings.Contains(parsed.Content, "alert") || strings.Contains(parsed.Content, ".x") {
		t.Fatalf("script/style leaked into content: %q", parsed.Content)
	}
}

func TestParseJSONFlattensFields(t *testing.T) {
	parsed, err := Parse(context.Background(), domain.UnstructuredIngestRequest{
		SourceType: "json",
		JSONText:   `{"product":{"name":"保温杯","notes":["316不锈钢","适合通勤"]}}`,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	for _, want := range []string{"product.name: 保温杯", "product.notes[0]: 316不锈钢"} {
		if !strings.Contains(parsed.Content, want) {
			t.Fatalf("content missing %q: %q", want, parsed.Content)
		}
	}
}

func TestParseRejectsEmptyContent(t *testing.T) {
	if _, err := Parse(context.Background(), domain.UnstructuredIngestRequest{}); err == nil {
		t.Fatal("expected empty content error")
	}
}
