package imagevector

import "testing"

func TestConfigFromMapUsesActiveProviderAPIKey(t *testing.T) {
	cfg := ConfigFromMap(map[string]string{
		"image_embedding.provider": "dashscope",
		"ai.active_provider":       "qwen",
		"ai.qwen.api_key":          "qwen-key",
	}, "")
	if cfg.APIKey != "qwen-key" {
		t.Fatalf("APIKey = %q, want qwen-key", cfg.APIKey)
	}
}

func TestConfigFromMapDisablesProxyByDefault(t *testing.T) {
	cfg := ConfigFromMap(map[string]string{}, "")
	if cfg.ProxyEnabled {
		t.Fatalf("ProxyEnabled = true, want false")
	}
}
