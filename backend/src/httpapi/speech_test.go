package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

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

func TestSpeechTTSConfigReadsDoubaoProvider(t *testing.T) {
	server := &Server{configs: configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "tts.provider", ConfigValue: "doubao", ValueType: "string"},
		{ConfigKey: "doubao.tts.enabled", ConfigValue: "true", ValueType: "bool"},
		{ConfigKey: "doubao.tts.app_id", ConfigValue: "doubao-app", ValueType: "string"},
		{ConfigKey: "doubao.tts.api_key", ConfigValue: "doubao-key", ValueType: "string", IsSecret: true},
		{ConfigKey: "doubao.tts.base_url", ConfigValue: "https://example.com/tts", ValueType: "string"},
		{ConfigKey: "doubao.tts.cluster", ConfigValue: "cluster-1", ValueType: "string"},
		{ConfigKey: "doubao.tts.voice", ConfigValue: "voice-1", ValueType: "string"},
		{ConfigKey: "doubao.tts.encoding", ConfigValue: "mp3", ValueType: "string"},
		{ConfigKey: "doubao.tts.speed_ratio", ConfigValue: "1.5", ValueType: "number"},
		{ConfigKey: "doubao.tts.timeout_seconds", ConfigValue: "10", ValueType: "int"},
		{ConfigKey: "doubao.tts.max_runes", ConfigValue: "600", ValueType: "int"},
	})}

	cfg := server.speechTTSConfig(context.Background())
	if !cfg.Enabled() {
		t.Fatalf("expected doubao tts enabled, got %#v", cfg)
	}
	if cfg.Provider != "doubao" || cfg.DoubaoAppID != "doubao-app" || cfg.DoubaoAPIKey != "doubao-key" {
		t.Fatalf("unexpected doubao config: %#v", cfg)
	}
	if cfg.DefaultVoice() != "voice-1" || cfg.ContentType() != "audio/mpeg" {
		t.Fatalf("unexpected voice/content-type: %q %q", cfg.DefaultVoice(), cfg.ContentType())
	}
	if cfg.TimeoutSeconds != 10 || cfg.MaxRunes != 600 {
		t.Fatalf("unexpected limits: timeout=%d max=%d", cfg.TimeoutSeconds, cfg.MaxRunes)
	}

	payload := cfg.DoubaoRequestPayload("你好", "", "req-1")
	app := payload["app"].(map[string]any)
	if app["appid"] != "doubao-app" || app["token"] != "doubao-key" || app["cluster"] != "cluster-1" {
		t.Fatalf("unexpected app payload: %#v", app)
	}
	audio := payload["audio"].(map[string]any)
	if audio["voice_type"] != "voice-1" || audio["speed_ratio"] != 1.5 {
		t.Fatalf("unexpected audio payload: %#v", audio)
	}
	request := payload["request"].(map[string]any)
	if request["reqid"] != "req-1" || request["text"] != "你好" || request["operation"] != "query" {
		t.Fatalf("unexpected request payload: %#v", request)
	}
}

func TestHandleSpeechTTSConfig(t *testing.T) {
	server := &Server{configs: configcenter.NewMemoryCenter([]domain.AppConfig{
		{ConfigKey: "xunfei.tts.enabled", ConfigValue: "true", ValueType: "bool"},
		{ConfigKey: "xunfei.tts.app_id", ConfigValue: "tts-app", ValueType: "string"},
		{ConfigKey: "xunfei.tts.api_key", ConfigValue: "tts-key", ValueType: "string", IsSecret: true},
		{ConfigKey: "xunfei.tts.api_secret", ConfigValue: "tts-secret", ValueType: "string", IsSecret: true},
		{ConfigKey: "xunfei.tts.voice", ConfigValue: "xiaomei", ValueType: "string"},
		{ConfigKey: "xunfei.tts.max_runes", ConfigValue: "1200", ValueType: "int"},
	})}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/speech/tts/config", nil)
	server.handleSpeechTTSConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["enabled"] != true || response["provider"] != "xunfei" || response["voice"] != "xiaomei" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response["max_text_chars"] != float64(1200) {
		t.Fatalf("unexpected max_text_chars: %#v", response["max_text_chars"])
	}
}

func TestHandleSpeechTTSConfigDisabled(t *testing.T) {
	server := &Server{configs: configcenter.NewMemoryCenter(nil)}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/speech/tts/config", nil)
	server.handleSpeechTTSConfig(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["enabled"] != false || response["provider"] != "xunfei" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestSynthesizeDoubaoSpeechTTS(t *testing.T) {
	var gotAuth string
	var gotPayload map[string]any
	originalClient := speechHTTPClient
	speechHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotAuth = request.Header.Get("Authorization")
		raw, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &gotPayload); err != nil {
			return nil, err
		}
		body, err := json.Marshal(map[string]any{
			"code": 3000,
			"data": base64.StdEncoding.EncodeToString([]byte("mp3-bytes")),
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	defer func() {
		speechHTTPClient = originalClient
	}()

	cfg := speechTTSConfig{
		Provider:           "doubao",
		DoubaoEnabledFlag:  true,
		DoubaoAppID:        "doubao-app",
		DoubaoAPIKey:       "doubao-key",
		DoubaoBaseURL:      "https://example.com/tts",
		DoubaoCluster:      "cluster-1",
		DoubaoVoice:        "voice-1",
		DoubaoEncoding:     "mp3",
		DoubaoUID:          "uid-1",
		DoubaoTextType:     "plain",
		DoubaoOperation:    "query",
		DoubaoWithFrontend: 1,
		DoubaoFrontendType: "unitTson",
		DoubaoSpeedRatio:   1,
		DoubaoVolumeRatio:  1,
		DoubaoPitchRatio:   1,
	}

	audio, err := (&Server{}).synthesizeDoubaoSpeechTTS(context.Background(), cfg, "你好", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(audio) != "mp3-bytes" {
		t.Fatalf("unexpected audio: %q", string(audio))
	}
	if gotAuth != "Bearer;doubao-key" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	app := gotPayload["app"].(map[string]any)
	if app["appid"] != "doubao-app" || app["cluster"] != "cluster-1" {
		t.Fatalf("unexpected app payload: %#v", app)
	}
}

func TestDoubaoAuthorizationHeader(t *testing.T) {
	cfg := speechTTSConfig{DoubaoAPIKey: "doubao-key"}
	if cfg.DoubaoAuthorizationHeader() != "Bearer;doubao-key" {
		t.Fatalf("unexpected authorization header: %q", cfg.DoubaoAuthorizationHeader())
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
