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
			candidates[i].Score = ScoreCandidate(candidates[i], terms, requiredTerms)
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
	score := 0.35*titleScore + 0.45*exactRatio + 0.20*ruleScore
	if score > 1 {
		return 1
	}
	return score
}
