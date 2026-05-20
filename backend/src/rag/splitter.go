package rag

import (
	"regexp"
	"strings"
)

var blankLines = regexp.MustCompile(`\n{3,}`)

func SplitDocument(title string, content string, docType string) []ChunkDraft {
	cleaned := cleanText(content)
	if cleaned == "" {
		return nil
	}
	if docType == "faq" {
		chunks := splitFAQ(title, cleaned)
		if len(chunks) > 0 {
			return chunks
		}
	}
	blocks := splitBlocks(cleaned)
	chunks := make([]ChunkDraft, 0, len(blocks))
	for _, block := range blocks {
		for _, chunk := range recursiveSplit(block, DefaultChunkMaxRunes) {
			chunks = append(chunks, makeChunkDraft(title, chunk, len(chunks)))
		}
	}
	if len(chunks) == 0 {
		chunks = append(chunks, makeChunkDraft(title, cleaned, 0))
	}
	return chunks
}

func cleanText(input string) string {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		cleaned = append(cleaned, line)
	}
	text = strings.Join(cleaned, "\n")
	text = blankLines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func splitBlocks(text string) []string {
	parts := strings.Split(text, "\n\n")
	blocks := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			blocks = append(blocks, part)
		}
	}
	return blocks
}

func recursiveSplit(text string, maxRunes int) []string {
	if len([]rune(text)) <= maxRunes {
		return []string{text}
	}
	for _, separator := range []string{"。", "？", "！", "；", "\n", "，", "、"} {
		parts := strings.Split(text, separator)
		if len(parts) <= 1 {
			continue
		}
		merged := mergeParts(parts, separator, maxRunes)
		result := make([]string, 0, len(merged))
		for _, item := range merged {
			if len([]rune(item)) > maxRunes {
				result = append(result, fixedSplit(item, maxRunes)...)
				continue
			}
			result = append(result, item)
		}
		return result
	}
	return fixedSplit(text, maxRunes)
}

func mergeParts(parts []string, separator string, maxRunes int) []string {
	result := make([]string, 0)
	var current string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if separator != "\n" {
			part += separator
		}
		next := current + part
		if current == "" || len([]rune(next)) <= maxRunes {
			current = next
			continue
		}
		result = append(result, strings.TrimSpace(current))
		current = overlapTail(current) + part
	}
	if strings.TrimSpace(current) != "" {
		result = append(result, strings.TrimSpace(current))
	}
	return result
}

func fixedSplit(text string, maxRunes int) []string {
	runes := []rune(text)
	result := make([]string, 0, len(runes)/maxRunes+1)
	for start := 0; start < len(runes); {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		result = append(result, strings.TrimSpace(string(runes[start:end])))
		if end == len(runes) {
			break
		}
		start = end - 100
		if start < 0 {
			start = end
		}
	}
	return result
}

func overlapTail(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 100 {
		return string(runes)
	}
	return string(runes[len(runes)-100:]) + "\n"
}

func splitFAQ(title string, content string) []ChunkDraft {
	lines := strings.Split(content, "\n")
	chunks := make([]ChunkDraft, 0)
	var question string
	var answer []string
	flush := func() {
		if question == "" || len(answer) == 0 {
			return
		}
		text := question + "\n" + strings.Join(answer, "\n")
		for _, chunk := range recursiveSplit(text, DefaultChunkMaxRunes) {
			chunks = append(chunks, makeChunkDraft(title+" / FAQ", chunk, len(chunks)))
		}
		answer = nil
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Q:") || strings.HasPrefix(trimmed, "问：") || strings.HasPrefix(trimmed, "问题：") {
			flush()
			question = trimmed
			continue
		}
		if strings.HasPrefix(trimmed, "A:") || strings.HasPrefix(trimmed, "答：") || strings.HasPrefix(trimmed, "答案：") {
			answer = append(answer, trimmed)
			continue
		}
		if question != "" && trimmed != "" {
			answer = append(answer, trimmed)
		}
	}
	flush()
	return chunks
}

func makeChunkDraft(title string, content string, index int) ChunkDraft {
	content = strings.TrimSpace(content)
	return ChunkDraft{
		Title:     title,
		Content:   content,
		Snippet:   Snippet(content, DefaultSnippetRunes),
		SortOrder: (index + 1) * 10,
	}
}

func Snippet(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}
