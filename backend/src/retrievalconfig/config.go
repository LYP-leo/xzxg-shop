package retrievalconfig

import (
	"strconv"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

func Apply(plan *rag.RetrievalPlan, values map[string]string) {
	if plan.Rerank.Weights == nil {
		plan.Rerank.Weights = rag.DefaultWeights()
	}
	weightKeys := map[string]string{
		"title_match":     "retrieval.rerank.weight.title_match",
		"term_match":      "retrieval.rerank.weight.term_match",
		"required_match":  "retrieval.rerank.weight.required_match",
		"brand_boost":     "retrieval.rerank.weight.brand_boost",
		"model_boost":     "retrieval.rerank.weight.model_boost",
		"category_boost":  "retrieval.rerank.weight.category_boost",
		"generic_penalty": "retrieval.rerank.weight.generic_penalty",
	}
	for weightKey, configKey := range weightKeys {
		if value, ok := parseFloat(values[configKey]); ok {
			plan.Rerank.Weights[weightKey] = value
		}
	}
	if value, ok := parseInt(values["retrieval.keyword.top_n"]); ok && value > 0 {
		plan.Recall.Keyword.TopN = value
	}
	if value, ok := parseInt(values["retrieval.vector.top_n"]); ok && value > 0 {
		plan.Recall.Vector.TopN = value
	}
	terms := rag.QueryTerms(plan.Query)
	plan.Rerank.Terms.GenericTerms = configTerms(values["retrieval.rerank.generic_terms"])
	plan.Rerank.Terms.BrandBoostTerms = terms
	plan.Rerank.Terms.ModelBoostTerms = modelLikeTerms(terms)
	plan.Rerank.Terms.CategoryBoostTerms = categoryLikeTerms(terms)
}

func parseInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseFloat(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func configTerms(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n' || r == ';' || r == '；'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func modelLikeTerms(terms []string) []string {
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		if isModelLikeTerm(term) {
			out = append(out, term)
		}
	}
	return out
}

func categoryLikeTerms(terms []string) []string {
	keywords := []string{"手机", "平板", "电脑", "笔记本", "面霜", "精华", "防晒", "洁面", "咖啡", "茶饮", "调味", "跑鞋", "t恤", "夹克", "碳酸", "饮料"}
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		lower := strings.ToLower(term)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				out = append(out, term)
				break
			}
		}
	}
	return out
}

func isModelLikeTerm(term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if len(term) >= 4 && strings.IndexFunc(term, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0 {
		return true
	}
	modelTokens := []string{"iphone", "ipad", "macbook", "matepad", "matebook", "thinkpad", "mix", "fold", "pro", "max", "airism", "ultraboost"}
	for _, token := range modelTokens {
		if strings.Contains(term, token) {
			return true
		}
	}
	return false
}
