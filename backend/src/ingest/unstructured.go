package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

const (
	DefaultMaxFetchBytes = 2 << 20
	DefaultMaxTextRunes  = 120_000
)

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	tagRe         = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRe       = regexp.MustCompile(`[ \t\f\v]+`)
	blankLineRe   = regexp.MustCompile(`\n{3,}`)
)

type ParsedDocument struct {
	Title      string
	DocType    string
	Content    string
	SourceURL  string
	Metadata   map[string]any
	TextRunes  int
	SourceType string
}

func Parse(ctx context.Context, request domain.UnstructuredIngestRequest) (ParsedDocument, error) {
	sourceType := normalizeSourceType(request.SourceType)
	raw := firstNonEmpty(request.Content, request.HTML, request.JSONText)
	sourceURL := strings.TrimSpace(request.SourceURL)
	if raw == "" && sourceURL != "" {
		fetched, err := FetchURL(ctx, sourceURL, DefaultMaxFetchBytes)
		if err != nil {
			return ParsedDocument{}, err
		}
		raw = fetched
		if sourceType == "text" {
			sourceType = inferSourceTypeFromURL(sourceURL, fetched)
		}
	}
	if raw == "" {
		return ParsedDocument{}, errors.New("empty unstructured content")
	}

	content := ""
	switch sourceType {
	case "html":
		content = HTMLToText(raw)
	case "json":
		content = JSONToText(raw)
	default:
		content = PlainText(raw)
	}
	content = truncateRunes(content, DefaultMaxTextRunes)
	if content == "" {
		return ParsedDocument{}, errors.New("empty parsed text")
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = InferTitle(content, sourceURL)
	}
	return ParsedDocument{
		Title:      title,
		DocType:    docTypeForSource(sourceType),
		Content:    content,
		SourceURL:  sourceURL,
		Metadata:   request.Metadata,
		TextRunes:  len([]rune(content)),
		SourceType: sourceType,
	}, nil
}

func FetchURL(ctx context.Context, sourceURL string, maxBytes int64) (string, error) {
	if !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
		return "", errors.New("source_url must be http or https")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "xzxg-shop-rag-ingest/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch source_url failed: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > maxBytes {
		return "", errors.New("fetched document too large")
	}
	return string(body), nil
}

func HTMLToText(input string) string {
	text := scriptStyleRe.ReplaceAllString(input, "\n")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = tagRe.ReplaceAllString(text, "\n")
	return PlainText(html.UnescapeString(text))
}

func JSONToText(input string) string {
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		return PlainText(input)
	}
	lines := make([]string, 0)
	flattenJSON("", value, &lines)
	return PlainText(strings.Join(lines, "\n"))
}

func PlainText(input string) string {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = spaceRe.ReplaceAllString(strings.TrimSpace(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	text = strings.Join(cleaned, "\n")
	text = blankLineRe.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func InferTitle(content string, sourceURL string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 80)
		}
	}
	if sourceURL != "" {
		return sourceURL
	}
	return "非结构化资料"
}

func normalizeSourceType(sourceType string) string {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "html", "web", "page":
		return "html"
	case "json", "jsonl":
		return "json"
	default:
		return "text"
	}
}

func inferSourceTypeFromURL(sourceURL string, body string) string {
	lowerURL := strings.ToLower(sourceURL)
	if strings.HasSuffix(lowerURL, ".json") || strings.HasSuffix(lowerURL, ".jsonl") {
		return "json"
	}
	if strings.Contains(strings.ToLower(body), "<html") || strings.Contains(strings.ToLower(body), "<body") {
		return "html"
	}
	return "text"
}

func docTypeForSource(sourceType string) string {
	switch sourceType {
	case "html":
		return "web_article"
	case "json":
		return "structured_note"
	default:
		return "unstructured_note"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func flattenJSON(prefix string, value any, lines *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSON(next, child, lines)
		}
	case []any:
		for index, child := range typed {
			flattenJSON(fmt.Sprintf("%s[%d]", prefix, index), child, lines)
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			*lines = append(*lines, prefix+": "+typed)
		}
	case float64, bool:
		*lines = append(*lines, fmt.Sprintf("%s: %v", prefix, typed))
	}
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
