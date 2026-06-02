package httpapi

import (
	"context"
	"net/url"
	"regexp"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestParseXunfeiSpeechResult(t *testing.T) {
	raw := []byte(`{
		"msg_type":"result",
		"res_type":"asr",
		"sid":"sid-1",
		"data":{
			"ls":false,
			"cn":{
				"st":{
					"type":"1",
					"rt":[{
						"ws":[
							{"cw":[{"w":"你好"},{"w":"，"}]},
							{"cw":[{"w":"世界"}]}
						]
					}]
				}
			}
		}
	}`)

	result, ok := parseXunfeiSpeechResult(raw)
	if !ok {
		t.Fatal("expected result to parse")
	}
	if result.Text != "你好，世界" {
		t.Fatalf("unexpected text: %q", result.Text)
	}
	if result.Stable {
		t.Fatal("partial result should not be stable")
	}
	if result.Final {
		t.Fatal("partial result should not be final")
	}
	if result.SessionID != "sid-1" {
		t.Fatalf("unexpected session id: %q", result.SessionID)
	}
}

func TestSpeechSignedURL(t *testing.T) {
	cfg := speechRealtimeConfig{
		AppID:       "app-id",
		APIKey:      "api-key",
		APISecret:   "secret",
		BaseURL:     "wss://office-api-ast-dx.iflyaisol.com",
		Path:        "/ast/communicate/v1",
		Lang:        "autodialect",
		AudioEncode: "pcm_s16le",
		SampleRate:  "16000",
	}

	signedURL, err := cfg.SignedURL("req-1")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "wss" || parsed.Host != "office-api-ast-dx.iflyaisol.com" || parsed.Path != "/ast/communicate/v1" {
		t.Fatalf("unexpected endpoint: %s", signedURL)
	}
	if query.Get("appId") != "app-id" || query.Get("accessKeyId") != "api-key" || query.Get("uuid") != "req-1" {
		t.Fatalf("missing auth params: %s", signedURL)
	}
	if query.Get("audio_encode") != "pcm_s16le" || query.Get("lang") != "autodialect" || query.Get("samplerate") != "16000" {
		t.Fatalf("missing audio params: %s", signedURL)
	}
	if query.Get("signature") == "" || query.Get("utc") == "" {
		t.Fatalf("missing signature params: %s", signedURL)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}$`).MatchString(query.Get("utc")) {
		t.Fatalf("unexpected utc format: %q", query.Get("utc"))
	}
}

func TestSpeechRealtimeConfigReadsNacosFirst(t *testing.T) {
	server := &Server{configs: configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "XUNFEI_APP_ID", ConfigValue: "nacos-app", ValueType: "string"},
		{ConfigKey: "XUNFEI_API_KEY", ConfigValue: "nacos-key", ValueType: "string", IsSecret: true},
		{ConfigKey: "XUNFEI_API_SECRET", ConfigValue: "nacos-secret", ValueType: "string", IsSecret: true},
		{ConfigKey: "XUNFEI_RTASR_SAMPLE_RATE", ConfigValue: "8000", ValueType: "string"},
	})}
	t.Setenv("XUNFEI_APP_ID", "env-app")
	t.Setenv("XUNFEI_API_KEY", "env-key")
	t.Setenv("XUNFEI_API_SECRET", "env-secret")

	cfg := server.speechRealtimeConfig(context.Background())
	if cfg.AppID != "nacos-app" || cfg.APIKey != "nacos-key" || cfg.APISecret != "nacos-secret" {
		t.Fatalf("expected nacos credentials first, got %#v", cfg)
	}
	if cfg.SampleRate != "8000" {
		t.Fatalf("expected nacos sample rate, got %q", cfg.SampleRate)
	}
	if cfg.BaseURL != "wss://office-api-ast-dx.iflyaisol.com" {
		t.Fatalf("expected default base url, got %q", cfg.BaseURL)
	}
}
