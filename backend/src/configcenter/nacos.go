package configcenter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type nacosDocument struct {
	Configs []domain.AppConfig `json:"configs"`
}

type nacosPromptDocument struct {
	Prompts []domain.AgentPrompt `json:"prompts"`
}

type NacosCenter struct {
	baseURL      string
	namespace    string
	group        string
	legacyDataID string
	dataIDs      map[string]string
	client       *http.Client
	mu           sync.RWMutex
	fallback     *MemoryCenter
}

func NewNacosCenter(baseURL string, namespace string, group string, dataID string, defaults []domain.AppConfig) *NacosCenter {
	if strings.TrimSpace(group) == "" {
		group = "XZXG_SHOP"
	}
	if strings.TrimSpace(dataID) == "" {
		dataID = "xzxg-shop-app-config.json"
	}
	return &NacosCenter{
		baseURL:      strings.TrimRight(baseURL, "/"),
		namespace:    strings.TrimSpace(namespace),
		group:        strings.TrimSpace(group),
		legacyDataID: strings.TrimSpace(dataID),
		dataIDs: map[string]string{
			"app":    "xzxg-shop-app-config.json",
			"rag":    "xzxg-shop-rag-config.json",
			"infra":  "xzxg-shop-infra-config.json",
			"prompt": "xzxg-shop-agent-prompts.json",
		},
		client:   &http.Client{Timeout: 3 * time.Second},
		fallback: NewMemoryCenter(defaults),
	}
}

func (c *NacosCenter) Seed(ctx context.Context) error {
	items, err := c.readAll(ctx)
	if err == nil && len(items) > 0 {
		items = mergeDefaults(items, c.fallbackList(ctx, true))
		if err := c.publishAll(ctx, items); err != nil {
			return err
		}
		c.replaceFallback(ctx, items)
		return nil
	}
	items = c.fallbackList(ctx, true)
	return c.publishAll(ctx, items)
}

func (c *NacosCenter) List(ctx context.Context, includeSecrets bool) []domain.AppConfig {
	items, err := c.readAll(ctx)
	if err != nil {
		return c.fallbackList(ctx, includeSecrets)
	}
	c.replaceFallback(ctx, items)
	for index := range items {
		if items[index].IsSecret && !includeSecrets && items[index].ConfigValue != "" {
			items[index].ConfigValue = "******"
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })
	return items
}

func (c *NacosCenter) GetMap(ctx context.Context) map[string]string {
	items, err := c.readAll(ctx)
	if err != nil {
		return c.fallbackMap(ctx)
	}
	c.replaceFallback(ctx, items)
	values := make(map[string]string, len(items))
	for _, item := range items {
		values[item.ConfigKey] = item.ConfigValue
	}
	return values
}

func (c *NacosCenter) Upsert(ctx context.Context, input domain.AppConfigInput) (domain.AppConfig, error) {
	item := normalizeInput(input)
	if strings.TrimSpace(item.ConfigKey) == "" {
		return domain.AppConfig{}, errors.New("config key is empty")
	}

	items, err := c.readAll(ctx)
	if err != nil {
		items = c.fallbackList(ctx, true)
	}
	replaced := false
	for index := range items {
		if items[index].ConfigKey == item.ConfigKey {
			items[index] = item
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })

	if err := c.publishAll(ctx, items); err != nil {
		return domain.AppConfig{}, err
	}
	c.upsertFallback(ctx, input)
	return item, nil
}

func (c *NacosCenter) readAll(ctx context.Context) ([]domain.AppConfig, error) {
	merged := map[string]domain.AppConfig{}
	readAny := false
	for _, dataID := range c.readDataIDs() {
		items, err := c.readDocument(ctx, dataID)
		if err != nil {
			continue
		}
		readAny = true
		for _, item := range items {
			if item.Domain == "" {
				item.Domain = DomainForKey(item.ConfigKey)
			}
			merged[item.ConfigKey] = item
		}
	}
	if !readAny {
		return nil, errors.New("nacos config not found")
	}
	items := make([]domain.AppConfig, 0, len(merged))
	for _, item := range merged {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })
	return items, nil
}

func (c *NacosCenter) readDocument(ctx context.Context, dataID string) ([]domain.AppConfig, error) {
	endpoint, err := c.configURL("/nacos/v1/cs/configs")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("dataId", dataID)
	query.Set("group", c.group)
	if c.namespace != "" {
		query.Set("tenant", c.namespace)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound || bytes.Contains(body, []byte("config data not exist")) {
		return nil, errors.New("nacos config not found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nacos get config failed: status=%d", resp.StatusCode)
	}
	var doc nacosDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if len(doc.Configs) == 0 {
		var promptDoc nacosPromptDocument
		if err := json.Unmarshal(body, &promptDoc); err == nil {
			for _, prompt := range promptDoc.Prompts {
				doc.Configs = append(doc.Configs, domain.AppConfig{
					ConfigKey:   prompt.PromptKey,
					ConfigValue: prompt.Content,
					ValueType:   "text",
					Description: prompt.Description,
					Domain:      "prompt",
					UpdatedAt:   prompt.UpdatedAt,
				})
			}
		}
	}
	for index := range doc.Configs {
		if doc.Configs[index].UpdatedAt.IsZero() {
			doc.Configs[index].UpdatedAt = time.Now()
		}
		if doc.Configs[index].Domain == "" {
			doc.Configs[index].Domain = DomainForKey(doc.Configs[index].ConfigKey)
		}
	}
	sort.Slice(doc.Configs, func(i, j int) bool { return doc.Configs[i].ConfigKey < doc.Configs[j].ConfigKey })
	return doc.Configs, nil
}

func (c *NacosCenter) publishAll(ctx context.Context, items []domain.AppConfig) error {
	byDomain := map[string][]domain.AppConfig{}
	for _, item := range items {
		if item.Domain == "" {
			item.Domain = DomainForKey(item.ConfigKey)
		}
		byDomain[item.Domain] = append(byDomain[item.Domain], item)
	}
	for domainName, domainItems := range byDomain {
		if err := c.publish(ctx, c.dataIDForDomain(domainName), domainItems); err != nil {
			return err
		}
	}
	c.replaceFallback(ctx, items)
	return nil
}

func (c *NacosCenter) publish(ctx context.Context, dataID string, items []domain.AppConfig) error {
	endpoint, err := c.configURL("/nacos/v1/cs/configs")
	if err != nil {
		return err
	}
	payload, err := c.marshalDocument(dataID, items)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("dataId", dataID)
	form.Set("group", c.group)
	form.Set("content", string(payload))
	form.Set("type", "json")
	if c.namespace != "" {
		form.Set("tenant", c.namespace)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("nacos publish config failed: status=%d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "true" {
		return fmt.Errorf("nacos publish config rejected: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *NacosCenter) marshalDocument(dataID string, items []domain.AppConfig) ([]byte, error) {
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })
	if dataID == c.dataIDForDomain("prompt") {
		prompts := make([]domain.AgentPrompt, 0, len(items))
		for _, item := range items {
			prompts = append(prompts, domain.AgentPrompt{
				PromptKey:   item.ConfigKey,
				Title:       item.Description,
				Content:     item.ConfigValue,
				Status:      "active",
				Description: item.Description,
				UpdatedAt:   item.UpdatedAt,
			})
		}
		return json.MarshalIndent(nacosPromptDocument{Prompts: prompts}, "", "  ")
	}
	return json.MarshalIndent(nacosDocument{Configs: items}, "", "  ")
}

func (c *NacosCenter) readDataIDs() []string {
	ids := []string{c.legacyDataID}
	for _, domainName := range []string{"app", "infra", "rag", "prompt"} {
		dataID := c.dataIDForDomain(domainName)
		if dataID == "" || containsString(ids, dataID) {
			continue
		}
		ids = append(ids, dataID)
	}
	return ids
}

func (c *NacosCenter) dataIDForDomain(domainName string) string {
	if dataID := c.dataIDs[domainName]; dataID != "" {
		return dataID
	}
	return c.dataIDs["app"]
}

func (c *NacosCenter) configURL(path string) (*url.URL, error) {
	if strings.TrimSpace(c.baseURL) == "" {
		return nil, errors.New("nacos addr is empty")
	}
	return url.Parse(c.baseURL + path)
}

func (c *NacosCenter) replaceFallback(ctx context.Context, items []domain.AppConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fallback = NewMemoryCenter(items)
}

func (c *NacosCenter) fallbackList(ctx context.Context, includeSecrets bool) []domain.AppConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fallback.List(ctx, includeSecrets)
}

func (c *NacosCenter) fallbackMap(ctx context.Context) map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fallback.GetMap(ctx)
}

func (c *NacosCenter) upsertFallback(ctx context.Context, input domain.AppConfigInput) {
	c.mu.RLock()
	fallback := c.fallback
	c.mu.RUnlock()
	_, _ = fallback.Upsert(ctx, input)
}

func mergeDefaults(items []domain.AppConfig, defaults []domain.AppConfig) []domain.AppConfig {
	exists := make(map[string]bool, len(items))
	for _, item := range items {
		exists[item.ConfigKey] = true
	}
	for _, item := range defaults {
		if exists[item.ConfigKey] {
			continue
		}
		if item.Domain == "" {
			item.Domain = DomainForKey(item.ConfigKey)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ConfigKey < items[j].ConfigKey })
	return items
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
