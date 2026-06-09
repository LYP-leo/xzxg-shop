package httpapi

import (
	"context"
	"encoding/base64"
	"net/url"
	"regexp"
	"testing"
	"time"

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

func TestSpeechTTSConfigReadsSpecificCredentialsFirst(t *testing.T) {
	server := &Server{configs: configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "XUNFEI_APP_ID", ConfigValue: "base-app", ValueType: "string"},
		{ConfigKey: "XUNFEI_API_KEY", ConfigValue: "base-key", ValueType: "string", IsSecret: true},
		{ConfigKey: "XUNFEI_API_SECRET", ConfigValue: "base-secret", ValueType: "string", IsSecret: true},
		{ConfigKey: "xunfei.tts.app_id", ConfigValue: "tts-app", ValueType: "string"},
		{ConfigKey: "xunfei.tts.api_key", ConfigValue: "tts-key", ValueType: "string", IsSecret: true},
		{ConfigKey: "xunfei.tts.api_secret", ConfigValue: "tts-secret", ValueType: "string", IsSecret: true},
		{ConfigKey: "xunfei.tts.speed", ConfigValue: "120", ValueType: "number"},
		{ConfigKey: "xunfei.tts.timeout_seconds", ConfigValue: "0", ValueType: "number"},
	})}

	cfg := server.speechTTSConfig(context.Background())
	if cfg.AppID != "tts-app" || cfg.APIKey != "tts-key" || cfg.APISecret != "tts-secret" {
		t.Fatalf("expected tts credentials first, got %#v", cfg)
	}
	if cfg.Speed != 100 {
		t.Fatalf("expected bounded speed, got %d", cfg.Speed)
	}
	if cfg.TimeoutSeconds != 1 {
		t.Fatalf("expected bounded timeout, got %d", cfg.TimeoutSeconds)
	}
	if cfg.BaseURL != "wss://tts-api.xfyun.cn/v2/tts" {
		t.Fatalf("expected default tts base url, got %q", cfg.BaseURL)
	}
}

func TestSpeechTTSSignedURLAndPayload(t *testing.T) {
	cfg := speechTTSConfig{
		EnabledFlag:    true,
		AppID:          "app-id",
		APIKey:         "api-key",
		APISecret:      "secret",
		BaseURL:        "wss://tts-api.xfyun.cn/v2/tts",
		Voice:          "xiaoyan",
		Speed:          50,
		Volume:         50,
		Pitch:          50,
		AudioEncoding:  "lame",
		TextEncoding:   "UTF8",
		TimeoutSeconds: 20,
		MaxRunes:       800,
	}

	signedURL, err := cfg.SignedURL(time.Date(2026, 6, 8, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "wss" || parsed.Host != "tts-api.xfyun.cn" || parsed.Path != "/v2/tts" {
		t.Fatalf("unexpected tts endpoint: %s", signedURL)
	}
	if query.Get("host") != "tts-api.xfyun.cn" || query.Get("date") != "Mon, 08 Jun 2026 01:02:03 GMT" {
		t.Fatalf("missing signing params: %s", signedURL)
	}
	authorization, err := base64.StdEncoding.DecodeString(query.Get("authorization"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`api_key="api-key".+hmac-sha256`).Match(authorization) {
		t.Fatalf("unexpected authorization: %s", string(authorization))
	}

	payload := cfg.RequestPayload("你好", "")
	data := payload["data"].(map[string]any)
	if data["text"] != base64.StdEncoding.EncodeToString([]byte("你好")) {
		t.Fatalf("text should be base64 encoded, got %#v", data["text"])
	}
	business := payload["business"].(map[string]any)
	if business["vcn"] != "xiaoyan" || business["aue"] != "lame" {
		t.Fatalf("unexpected business payload: %#v", business)
	}
}

func TestParseXunfeiTTSFrame(t *testing.T) {
	audio, final, err := parseXunfeiTTSFrame([]byte(`{"code":0,"message":"success","data":{"audio":"aGVsbG8=","status":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "hello" || !final {
		t.Fatalf("unexpected frame result audio=%q final=%v", string(audio), final)
	}
}
