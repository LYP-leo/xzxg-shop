package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
)

type GuideSuggestionCandidate struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Reason   string `json:"reason"`
}

type GuideSuggestionRequest struct {
	Page       string                     `json:"page"`
	Context    map[string]any             `json:"context"`
	Limit      int                        `json:"limit"`
	Candidates []GuideSuggestionCandidate `json:"candidates"`
}

func (r *Runtime) GenerateGuideSuggestions(ctx context.Context, request GuideSuggestionRequest) ([]GuideSuggestionCandidate, error) {
	if r == nil || r.llm == nil {
		return nil, fmt.Errorf("runtime unavailable")
	}
	r.refreshDynamicConfig(ctx)
	if !r.llm.Enabled() {
		return nil, fmt.Errorf("llm disabled")
	}
	if request.Limit <= 0 {
		request.Limit = 3
	}
	if request.Limit > 3 {
		request.Limit = 3
	}
	if request.Context == nil {
		request.Context = map[string]any{}
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	prompt := r.stringConfig(ctx, "agent.prompt.guide_suggestions", configcenter.DefaultGuideSuggestionsPrompt)
	messages := []ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: string(payload)},
	}
	model := r.modelForRole(ctx, "guide_suggestions", r.llm.SmallModel())
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	content, err := r.llm.Complete(callCtx, model, messages, 0.2)
	if err != nil {
		return nil, err
	}
	return ParseGuideSuggestionModelOutput(content, request.Limit)
}

func ParseGuideSuggestionModelOutput(content string, limit int) ([]GuideSuggestionCandidate, error) {
	content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(content), "```"), "```json"))
	content = strings.TrimSpace(strings.TrimPrefix(content, "json"))
	var decoded struct {
		Items []GuideSuggestionCandidate `json:"items"`
	}
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 3 {
		limit = 3
	}
	items := make([]GuideSuggestionCandidate, 0, limit)
	seen := map[string]bool{}
	for _, item := range decoded.Items {
		item.ID = strings.TrimSpace(item.ID)
		item.Question = normalizeGuideSuggestionQuestion(item.Question)
		item.Reason = strings.TrimSpace(item.Reason)
		if item.Question == "" || seen[item.Question] {
			continue
		}
		if item.ID == "" {
			item.ID = fmt.Sprintf("model_%d", len(items)+1)
		}
		seen[item.Question] = true
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("model returned no valid guide suggestions")
	}
	return items, nil
}

func normalizeGuideSuggestionQuestion(question string) string {
	question = strings.TrimSpace(question)
	question = strings.Trim(question, " 　。.!！?？")
	if question == "" {
		return ""
	}
	runes := []rune(question)
	if len(runes) > 10 {
		return ""
	}
	return question
}
