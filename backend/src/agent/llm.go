package agent

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
)

const defaultDashScopeBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

type ModelConfig struct {
	BaseURL    string
	APIKey     string
	SmallModel string
	LargeModel string
}

func (c ModelConfig) withDefaults() ModelConfig {
	if c.BaseURL == "" {
		c.BaseURL = defaultDashScopeBaseURL
	}
	if c.SmallModel == "" {
		c.SmallModel = "qwen3.5-flash"
	}
	if c.LargeModel == "" {
		c.LargeModel = "qwen3.6-plus"
	}
	return c
}

type LLMClient struct {
	config ModelConfig
	http   *http.Client
}

func NewLLMClient(config ModelConfig) *LLMClient {
	config = config.withDefaults()
	return &LLMClient{
		config: config,
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *LLMClient) Enabled() bool {
	return strings.TrimSpace(c.config.APIKey) != ""
}

func (c *LLMClient) SmallModel() string {
	return c.config.SmallModel
}

func (c *LLMClient) LargeModel() string {
	return c.config.LargeModel
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *LLMClient) Complete(ctx context.Context, model string, messages []ChatMessage, temperature float64) (string, error) {
	if !c.Enabled() {
		return "", errors.New("llm disabled: missing api key")
	}
	if model == "" {
		model = c.config.SmallModel
	}

	requestBody := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal llm request: %w", err)
	}

	url := strings.TrimRight(c.config.BaseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create llm request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("call llm: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("llm status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", errors.New(decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("llm response has no choices")
	}
	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("llm response is empty")
	}
	return content, nil
}
