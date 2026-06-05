package imagevector

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

const Dimension = 64

const (
	DefaultProvider = "local_histogram"
	DefaultModel    = "qwen3-vl-embedding"
	DefaultBaseURL  = "https://dashscope.aliyuncs.com"
)

type Config struct {
	Provider     string
	BaseURL      string
	APIKey       string
	Model        string
	Dimension    int
	ProxyEnabled bool
}

type Embedder interface {
	EmbedReader(ctx context.Context, reader io.Reader) ([]float32, error)
	EmbedSource(ctx context.Context, source string) ([]float32, error)
	Dimension() int
}

type LocalHistogramEmbedder struct{}

func NewLocalHistogramEmbedder() *LocalHistogramEmbedder {
	return &LocalHistogramEmbedder{}
}

func (e *LocalHistogramEmbedder) EmbedReader(ctx context.Context, reader io.Reader) ([]float32, error) {
	return FromReader(reader)
}

func (e *LocalHistogramEmbedder) EmbedSource(ctx context.Context, source string) ([]float32, error) {
	return FromSource(ctx, source)
}

func (e *LocalHistogramEmbedder) Dimension() int {
	return Dimension
}

type DashScopeEmbedder struct {
	cfg    Config
	client *http.Client
}

func NewDashScopeEmbedder(cfg Config) *DashScopeEmbedder {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 512
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.ProxyEnabled {
		transport.Proxy = nil
	}
	return &DashScopeEmbedder{
		cfg:    cfg,
		client: &http.Client{Timeout: 20 * time.Second, Transport: transport},
	}
}

func NewEmbedderFromMap(values map[string]string, fallbackAPIKey string) Embedder {
	cfg := ConfigFromMap(values, fallbackAPIKey)
	if strings.EqualFold(cfg.Provider, "dashscope") {
		return NewDashScopeEmbedder(cfg)
	}
	return NewLocalHistogramEmbedder()
}

func ConfigFromMap(values map[string]string, fallbackAPIKey string) Config {
	dimension := parseInt(values["image_embedding.dimension"], 512)
	return Config{
		Provider:     valueOr(values["image_embedding.provider"], DefaultProvider),
		BaseURL:      valueOr(values["image_embedding.base_url"], DefaultBaseURL),
		APIKey:       valueOr(values["image_embedding.api_key"], valueOr(activeProviderAPIKey(values), valueOr(values["ai.api_key"], fallbackAPIKey))),
		Model:        valueOr(values["image_embedding.model"], DefaultModel),
		Dimension:    dimension,
		ProxyEnabled: parseBool(values["image_embedding.proxy_enabled"], false),
	}
}

func activeProviderAPIKey(values map[string]string) string {
	provider := strings.TrimSpace(strings.ToLower(values["ai.active_provider"]))
	if provider == "" {
		return ""
	}
	return strings.TrimSpace(values["ai."+provider+".api_key"])
}

func (e *DashScopeEmbedder) EmbedReader(ctx context.Context, reader io.Reader) ([]float32, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, 10<<20))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("empty image")
	}
	mimeType := http.DetectContentType(raw)
	return e.embedImage(ctx, "data:"+mimeType+";base64,"+base64.StdEncoding.EncodeToString(raw))
}

func (e *DashScopeEmbedder) EmbedSource(ctx context.Context, source string) ([]float32, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("empty image source")
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return e.embedImage(ctx, source)
	}
	file, err := openSource(source)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return e.EmbedReader(ctx, file)
}

func (e *DashScopeEmbedder) Dimension() int {
	return e.cfg.Dimension
}

func (e *DashScopeEmbedder) embedImage(ctx context.Context, image string) ([]float32, error) {
	if strings.TrimSpace(e.cfg.APIKey) == "" {
		return nil, errors.New("image embedding api key is empty")
	}
	payload := map[string]any{
		"model": e.cfg.Model,
		"input": map[string]any{
			"contents": []map[string]string{{"image": image}},
		},
		"parameters": map[string]any{
			"dimension":   e.cfg.Dimension,
			"output_type": "dense",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(e.cfg.BaseURL, "/") + "/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("image embedding status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var decoded struct {
		Output struct {
			Embeddings []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"embeddings"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	if decoded.Code != "" && decoded.Code != "200" {
		return nil, errors.New(strings.TrimSpace(decoded.Message))
	}
	if len(decoded.Output.Embeddings) == 0 || len(decoded.Output.Embeddings[0].Embedding) == 0 {
		return nil, errors.New("empty image embedding")
	}
	return decoded.Output.Embeddings[0].Embedding, nil
}

func FromReader(reader io.Reader) ([]float32, error) {
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}
	return FromImage(img), nil
}

func FromImage(img image.Image) []float32 {
	bounds := img.Bounds()
	vector := make([]float32, Dimension)
	total := float32(0)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			ri := int((r >> 8) / 64)
			gi := int((g >> 8) / 64)
			bi := int((b >> 8) / 64)
			index := ri*16 + gi*4 + bi
			if index >= 0 && index < len(vector) {
				vector[index] += 1
				total += 1
			}
		}
	}
	if total == 0 {
		return vector
	}
	for i := range vector {
		vector[i] = vector[i] / total
	}
	return vector
}

func FromSource(ctx context.Context, source string) ([]float32, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, errors.New("empty image source")
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return FromReader(io.LimitReader(resp.Body, 10<<20))
	}
	file, err := openSource(source)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return FromReader(file)
}

func openSource(source string) (*os.File, error) {
	if strings.HasPrefix(source, "/api/v1/assets/ecommerce_agent_dataset/") {
		source = filepath.Join("..", "quality", "data", "ecommerce_agent_dataset", strings.TrimPrefix(source, "/api/v1/assets/ecommerce_agent_dataset/"))
	}
	file, err := os.Open(source)
	if err != nil && !filepath.IsAbs(source) {
		file, err = os.Open(filepath.Join("..", source))
	}
	return file, err
}

func valueOr(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func parseInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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
