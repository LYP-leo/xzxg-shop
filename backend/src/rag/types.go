package rag

import "time"

const (
	DefaultTopK          = 5
	DefaultKeywordTopN   = 200
	DefaultMinScore      = 0
	DefaultChunkMaxRunes = 800
	DefaultChunkMinRunes = 120
	DefaultSnippetRunes  = 160
)

type RetrievalPlan struct {
	Query    string
	Filters  RetrievalFilters
	Recall   RecallPlan
	Rerank   RerankPlan
	Compress CompressPlan
	Trace    RetrievalTraceContext
}

type RetrievalFilters struct {
	DocTypes        []string
	ProductIDs      []string
	CategoryIDs     []string
	MerchantIDs     []string
	SourceTypes     []string
	EffectiveAt     time.Time
	MinQualityScore float64
}

type RecallPlan struct {
	Vector  VectorRecallPlan
	Keyword KeywordRecallPlan
	Rule    RuleRecallPlan
}

type VectorRecallPlan struct {
	Enabled bool
	TopN    int
}

type KeywordRecallPlan struct {
	Enabled       bool
	TopN          int
	RequiredTerms []string
}

type RuleRecallPlan struct {
	Enabled   bool
	TopN      int
	RuleTerms []string
}

type RerankPlan struct {
	TopK     int
	MinScore float64
	Weights  map[string]float64
	Terms    RerankTerms
}

type RerankTerms struct {
	GenericTerms       []string
	BrandBoostTerms    []string
	ModelBoostTerms    []string
	CategoryBoostTerms []string
}

type CompressPlan struct {
	Enabled            bool
	MaxCharsPerChunk   int
	KeepRuleExceptions bool
}

type RetrievalTraceContext struct {
	TraceID   string
	MessageID string
	Reason    string
}

type ChunkDraft struct {
	Title     string
	Content   string
	Snippet   string
	SortOrder int
}

type Candidate struct {
	ChunkID string
	Title   string
	Snippet string
	Source  string
	Score   float64
}

func DefaultRetrievalPlan(query string) RetrievalPlan {
	return RetrievalPlan{
		Query: query,
		Recall: RecallPlan{
			Vector: VectorRecallPlan{
				Enabled: true,
				TopN:    DefaultKeywordTopN,
			},
			Keyword: KeywordRecallPlan{
				Enabled: true,
				TopN:    DefaultKeywordTopN,
			},
		},
		Rerank: RerankPlan{
			TopK:     DefaultTopK,
			MinScore: DefaultMinScore,
			Weights:  DefaultWeights(),
		},
		Compress: CompressPlan{
			Enabled:          true,
			MaxCharsPerChunk: DefaultSnippetRunes,
		},
	}
}

func DefaultWeights() map[string]float64 {
	return map[string]float64{
		"keyword_score":    0.55,
		"doc_type_score":   0.10,
		"product_score":    0.10,
		"category_score":   0.05,
		"source_score":     0.05,
		"freshness_score":  0.03,
		"quality_score":    0.05,
		"evidence_density": 0.07,
		"title_match":      0.35,
		"term_match":       0.45,
		"required_match":   0.20,
		"brand_boost":      0.35,
		"model_boost":      0.30,
		"category_boost":   0.15,
		"generic_penalty":  0.45,
	}
}

func NormalizePlan(plan RetrievalPlan) RetrievalPlan {
	if plan.Rerank.TopK <= 0 {
		plan.Rerank.TopK = DefaultTopK
	}
	if plan.Rerank.Weights == nil {
		plan.Rerank.Weights = DefaultWeights()
	}
	if plan.Recall.Keyword.TopN <= 0 {
		plan.Recall.Keyword.TopN = DefaultKeywordTopN
	}
	if !plan.Recall.Keyword.Enabled && !plan.Recall.Vector.Enabled && !plan.Recall.Rule.Enabled {
		plan.Recall.Keyword.Enabled = true
	}
	if plan.Compress.MaxCharsPerChunk <= 0 {
		plan.Compress.MaxCharsPerChunk = DefaultSnippetRunes
	}
	return plan
}
