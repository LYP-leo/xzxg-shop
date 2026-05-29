package rag

import (
	"sort"
	"strings"
)

func RankCandidates(plan RetrievalPlan, candidates []Candidate) []Candidate {
	plan = NormalizePlan(plan)
	terms := QueryTerms(plan.Query)
	requiredTerms := append([]string{}, plan.Recall.Keyword.RequiredTerms...)
	requiredTerms = append(requiredTerms, plan.Recall.Rule.RuleTerms...)
	for i := range candidates {
		if candidates[i].Score == 0 {
			candidates[i].Score = ScoreCandidateWithPlan(plan, candidates[i], terms, requiredTerms)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ChunkID < candidates[j].ChunkID
		}
		return candidates[i].Score > candidates[j].Score
	})
	limit := plan.Rerank.TopK
	if limit <= 0 || limit > len(candidates) {
		limit = len(candidates)
	}
	result := make([]Candidate, 0, limit)
	for _, candidate := range candidates {
		if len(result) >= limit {
			break
		}
		if candidate.Score < plan.Rerank.MinScore {
			continue
		}
		result = append(result, candidate)
	}
	return result
}

func ScoreCandidate(candidate Candidate, terms []string, requiredTerms []string) float64 {
	return ScoreCandidateWithPlan(RetrievalPlan{Rerank: RerankPlan{Weights: DefaultWeights()}}, candidate, terms, requiredTerms)
}

func ScoreCandidateWithPlan(plan RetrievalPlan, candidate Candidate, terms []string, requiredTerms []string) float64 {
	weights := plan.Rerank.Weights
	if weights == nil {
		weights = DefaultWeights()
	}
	text := strings.ToLower(candidate.Title + "\n" + candidate.Snippet)
	if len(terms) == 0 && len(requiredTerms) == 0 {
		return 0.1
	}
	titleScore := 0.0
	title := strings.ToLower(candidate.Title)
	for _, term := range terms {
		if strings.Contains(title, term) {
			titleScore = 1
			break
		}
	}
	matched := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			matched++
		}
	}
	exactRatio := 0.0
	if len(terms) > 0 {
		exactRatio = float64(matched) / float64(len(terms))
	}
	requiredMatched := 0
	for _, term := range requiredTerms {
		if strings.Contains(text, strings.ToLower(term)) {
			requiredMatched++
		}
	}
	ruleScore := 0.0
	if len(requiredTerms) > 0 {
		ruleScore = float64(requiredMatched) / float64(len(requiredTerms))
	}
	score := weight(weights, "title_match", 0.35)*titleScore + weight(weights, "term_match", 0.45)*exactRatio + weight(weights, "required_match", 0.20)*ruleScore
	score += weight(weights, "brand_boost", 0.35) * boolScore(anyTermMatches(text, plan.Rerank.Terms.BrandBoostTerms))
	score += weight(weights, "model_boost", 0.30) * boolScore(anyTermMatches(text, plan.Rerank.Terms.ModelBoostTerms))
	score += weight(weights, "category_boost", 0.15) * boolScore(anyTermMatches(text, plan.Rerank.Terms.CategoryBoostTerms))
	if len(plan.Rerank.Terms.GenericTerms) > 0 {
		genericMatches := matchedCount(text, plan.Rerank.Terms.GenericTerms)
		if matched > 0 && matched == genericMatches && !anyTermMatches(text, append(append([]string{}, plan.Rerank.Terms.BrandBoostTerms...), append(plan.Rerank.Terms.ModelBoostTerms, plan.Rerank.Terms.CategoryBoostTerms...)...)) {
			score -= weight(weights, "generic_penalty", 0.45)
		}
	}
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func anyTermMatches(text string, terms []string) bool {
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func matchedCount(text string, terms []string) int {
	count := 0
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(text, term) {
			count++
		}
	}
	return count
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func weight(weights map[string]float64, key string, fallback float64) float64 {
	if value, ok := weights[key]; ok {
		return value
	}
	return fallback
}
