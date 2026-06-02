package rag

import (
	"context"
	"strconv"
	"strings"
)

const (
	DefaultMilvusAddress       = "http://127.0.0.1:19530"
	DefaultMilvusToken         = "root:Milvus"
	DefaultProductCollection   = "product_text_vectors"
	DefaultKnowledgeCollection = "knowledge_text_chunks"
	DefaultImageCollection     = "product_image_vectors_v2"
	DefaultEmbeddingModel      = "text-embedding-v4"
	DefaultEmbeddingBatchSize  = 10
	MaxEmbeddingBatchSize      = 10
	DefaultMetricType          = "COSINE"
	DefaultVectorTopN          = 40
	DefaultVectorMinScore      = 0.58
)

type Config struct {
	Enabled             bool
	MilvusAddress       string
	MilvusToken         string
	Database            string
	ProductCollection   string
	KnowledgeCollection string
	ImageCollection     string
	EmbeddingBaseURL    string
	EmbeddingAPIKey     string
	EmbeddingModel      string
	EmbeddingBatchSize  int
	VectorTopN          int
	VectorMinScore      float64
}

type Embedder interface {
	EmbedText(ctx context.Context, texts []string) ([][]float32, error)
}

type SearchHit struct {
	ID     string
	Score  float64
	Fields map[string]any
}

func ConfigFromMap(values map[string]string, fallbackAPIKey string, fallbackBaseURL string) Config {
	cfg := Config{
		Enabled:             parseBool(values["vector.enabled"], true),
		MilvusAddress:       valueOr(values["milvus.address"], DefaultMilvusAddress),
		MilvusToken:         valueOr(values["milvus.token"], DefaultMilvusToken),
		Database:            strings.TrimSpace(values["milvus.database"]),
		ProductCollection:   valueOr(values["milvus.collection.products"], DefaultProductCollection),
		KnowledgeCollection: valueOr(values["milvus.collection.knowledge"], DefaultKnowledgeCollection),
		ImageCollection:     valueOr(values["milvus.collection.product_images"], DefaultImageCollection),
		EmbeddingBaseURL:    valueOr(values["embedding.base_url"], valueOr(values["ai.base_url"], fallbackBaseURL)),
		EmbeddingAPIKey:     valueOr(values["embedding.api_key"], valueOr(values["ai.api_key"], fallbackAPIKey)),
		EmbeddingModel:      valueOr(values["embedding.model"], DefaultEmbeddingModel),
		EmbeddingBatchSize:  parseInt(values["embedding.batch_size"], DefaultEmbeddingBatchSize),
		VectorTopN:          parseInt(values["retrieval.vector.top_n"], DefaultVectorTopN),
		VectorMinScore:      parseFloat(values["retrieval.vector.min_score"], DefaultVectorMinScore),
	}
	if cfg.EmbeddingBatchSize <= 0 {
		cfg.EmbeddingBatchSize = DefaultEmbeddingBatchSize
	}
	if cfg.EmbeddingBatchSize > MaxEmbeddingBatchSize {
		cfg.EmbeddingBatchSize = MaxEmbeddingBatchSize
	}
	if cfg.VectorTopN <= 0 {
		cfg.VectorTopN = DefaultVectorTopN
	}
	return cfg
}

func valueOr(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func parseInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	total := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return fallback
		}
		total = total*10 + int(r-'0')
	}
	return total
}

func parseFloat(value string, fallback float64) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
