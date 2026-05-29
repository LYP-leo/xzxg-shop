package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type Client struct {
	cfg      Config
	embedder Embedder
	client   *http.Client
	ready    bool
}

func NewClient(cfg Config, embedder Embedder) *Client {
	return &Client{
		cfg:      cfg,
		embedder: embedder,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled && c.ready
}

func (c *Client) Bootstrap(ctx context.Context, productRows []map[string]any, knowledgeRows []map[string]any) error {
	if c == nil || !c.cfg.Enabled {
		return nil
	}
	if c.embedder == nil {
		return errors.New("embedder is nil")
	}
	sample, err := c.embedder.EmbedText(ctx, []string{"xzxg vector bootstrap"})
	if err != nil {
		return err
	}
	if len(sample) == 0 || len(sample[0]) == 0 {
		return errors.New("empty sample embedding")
	}
	dim := len(sample[0])
	if err := c.ensureCollection(ctx, c.cfg.ProductCollection, "product_id", dim); err != nil {
		return err
	}
	if err := c.ensureCollection(ctx, c.cfg.KnowledgeCollection, "chunk_id", dim); err != nil {
		return err
	}
	if err := c.upsertRows(ctx, c.cfg.ProductCollection, productRows, "search_text"); err != nil {
		return err
	}
	if err := c.upsertRows(ctx, c.cfg.KnowledgeCollection, knowledgeRows, "search_text"); err != nil {
		return err
	}
	_ = c.loadCollection(ctx, c.cfg.ProductCollection)
	_ = c.loadCollection(ctx, c.cfg.KnowledgeCollection)
	c.ready = true
	return nil
}

func (c *Client) SearchProducts(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	return c.search(ctx, c.cfg.ProductCollection, "product_id", query, limit, []string{"product_id"})
}

func (c *Client) SearchKnowledge(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	return c.search(ctx, c.cfg.KnowledgeCollection, "chunk_id", query, limit, []string{"chunk_id", "source"})
}

func (c *Client) Status(ctx context.Context) domain.VectorIndexStatus {
	status := domain.VectorIndexStatus{
		Enabled:   c != nil && c.cfg.Enabled,
		Ready:     c != nil && c.ready,
		UpdatedAt: time.Now(),
	}
	if c == nil {
		return status
	}
	status.Address = c.cfg.MilvusAddress
	collections := []struct {
		name    string
		kind    string
		primary string
	}{
		{c.cfg.ProductCollection, "products", "product_id"},
		{c.cfg.KnowledgeCollection, "knowledge", "chunk_id"},
	}
	for _, collection := range collections {
		item, err := c.collectionStatus(ctx, collection.name, collection.kind, collection.primary)
		if err != nil {
			status.Error = err.Error()
			continue
		}
		status.Collections = append(status.Collections, item)
	}
	return status
}

func (c *Client) ensureCollection(ctx context.Context, collection string, primaryField string, dimension int) error {
	payload := map[string]any{
		"dbName":             c.cfg.Database,
		"collectionName":     collection,
		"dimension":          dimension,
		"primaryFieldName":   primaryField,
		"idType":             "VarChar",
		"vectorFieldName":    "embedding",
		"metricType":         DefaultMetricType,
		"autoID":             false,
		"enableDynamicField": true,
		"params": map[string]any{
			"max_length": 256,
		},
	}
	var result milvusResponse
	err := c.post(ctx, "/v2/vectordb/collections/create", payload, &result)
	if err == nil || strings.Contains(strings.ToLower(err.Error()), "already") || strings.Contains(strings.ToLower(err.Error()), "exist") {
		return nil
	}
	return err
}

func (c *Client) loadCollection(ctx context.Context, collection string) error {
	payload := map[string]any{
		"dbName":         c.cfg.Database,
		"collectionName": collection,
	}
	var result milvusResponse
	return c.post(ctx, "/v2/vectordb/collections/load", payload, &result)
}

func (c *Client) collectionStatus(ctx context.Context, collection string, kind string, primary string) (domain.VectorCollectionStatus, error) {
	item := domain.VectorCollectionStatus{
		Name:       collection,
		Kind:       kind,
		PrimaryKey: primary,
		VectorKey:  "embedding",
		MetricType: DefaultMetricType,
	}
	var describe struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Load   string `json:"load"`
			Fields []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Params []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"params"`
			} `json:"fields"`
			Indexes []struct {
				FieldName  string `json:"fieldName"`
				MetricType string `json:"metricType"`
			} `json:"indexes"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/v2/vectordb/collections/describe", map[string]any{
		"dbName":         c.cfg.Database,
		"collectionName": collection,
	}, &describe); err != nil {
		return item, err
	}
	item.LoadState = describe.Data.Load
	for _, field := range describe.Data.Fields {
		if field.Type == "FloatVector" {
			item.VectorKey = field.Name
			for _, param := range field.Params {
				if param.Key == "dim" {
					item.Dimension = parseInt(param.Value, 0)
				}
			}
		}
	}
	for _, index := range describe.Data.Indexes {
		if index.FieldName == item.VectorKey && index.MetricType != "" {
			item.MetricType = index.MetricType
		}
	}
	var stats struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			RowCount any `json:"rowCount"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/v2/vectordb/collections/get_stats", map[string]any{
		"dbName":         c.cfg.Database,
		"collectionName": collection,
	}, &stats); err == nil {
		item.RowCount = int64(number(stats.Data.RowCount))
	}
	return item, nil
}

func (c *Client) upsertRows(ctx context.Context, collection string, rows []map[string]any, textKey string) error {
	if len(rows) == 0 {
		return nil
	}
	batchSize := c.cfg.EmbeddingBatchSize
	if batchSize <= 0 {
		batchSize = 16
	}
	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		texts := make([]string, 0, end-start)
		for _, row := range rows[start:end] {
			texts = append(texts, fmt.Sprint(row[textKey]))
		}
		vectors, err := c.embedder.EmbedText(ctx, texts)
		if err != nil {
			return err
		}
		entities := make([]map[string]any, 0, len(vectors))
		for i, vector := range vectors {
			entity := cloneMap(rows[start+i])
			delete(entity, textKey)
			entity["embedding"] = vector
			entities = append(entities, entity)
		}
		payload := map[string]any{
			"dbName":         c.cfg.Database,
			"collectionName": collection,
			"data":           entities,
		}
		var result milvusResponse
		if err := c.post(ctx, "/v2/vectordb/entities/upsert", payload, &result); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) search(ctx context.Context, collection string, idField string, query string, limit int, outputFields []string) ([]SearchHit, error) {
	if c == nil || !c.Enabled() {
		return nil, errors.New("vector client disabled")
	}
	if limit <= 0 {
		limit = c.cfg.VectorTopN
	}
	embedding, err := c.embedder.EmbedText(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"dbName":         c.cfg.Database,
		"collectionName": collection,
		"data":           embedding,
		"annsField":      "embedding",
		"limit":          limit,
		"outputFields":   outputFields,
		"searchParams": map[string]any{
			"metricType": DefaultMetricType,
		},
	}
	var result struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    []map[string]any `json:"data"`
	}
	if err := c.post(ctx, "/v2/vectordb/entities/search", payload, &result); err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(result.Data))
	for _, row := range result.Data {
		id := fmt.Sprint(row[idField])
		if id == "" || id == "<nil>" {
			id = fmt.Sprint(row["id"])
		}
		if id == "" || id == "<nil>" {
			continue
		}
		score := number(row["distance"])
		if c.cfg.VectorMinScore > 0 && score < c.cfg.VectorMinScore {
			continue
		}
		hits = append(hits, SearchHit{ID: id, Score: score, Fields: row})
	}
	return hits, nil
}

func (c *Client) post(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.MilvusAddress, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.MilvusToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.MilvusToken)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("milvus status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	if result, ok := out.(*milvusResponse); ok && result.Code != 0 && result.Code != 200 {
		return fmt.Errorf("milvus code %d: %s", result.Code, result.Message)
	}
	return nil
}

type milvusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func number(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
