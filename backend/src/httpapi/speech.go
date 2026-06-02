package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type speechRealtimeConfig struct {
	AppID       string
	APIKey      string
	APISecret   string
	BaseURL     string
	Path        string
	Lang        string
	AudioEncode string
	SampleRate  string
}

type speechClientControl struct {
	Type       string `json:"type"`
	Language   string `json:"language,omitempty"`
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

type speechClientEvent struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Seq     int    `json:"seq,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type xunfeiSpeechResult struct {
	Text      string
	Stable    bool
	Final     bool
	SessionID string
}

type speechAccumulator struct {
	stable []string
}

func (s *Server) handleSpeechRealtime(w http.ResponseWriter, r *http.Request) {
	cfg := speechRealtimeConfigFromEnv()
	if !cfg.Enabled() {
		writeError(w, http.StatusNotImplemented, "speech_not_enabled", "当前后端未配置讯飞语音识别")
		return
	}
	if sampleRate := r.URL.Query().Get("sample_rate"); sampleRate != "" && sampleRate != cfg.SampleRate {
		writeError(w, http.StatusBadRequest, "bad_sample_rate", "当前仅支持 16000Hz PCM 音频")
		return
	}

	websocket.Handler(func(client *websocket.Conn) {
		s.proxySpeechRealtime(r.Context(), client, cfg)
	}).ServeHTTP(w, r)
}

func (s *Server) proxySpeechRealtime(ctx context.Context, client *websocket.Conn, cfg speechRealtimeConfig) {
	defer client.Close()

	requestID := requestIDFromContext(ctx)
	xfURL, err := cfg.SignedURL(requestID)
	if err != nil {
		sendSpeechClientEvent(client, nil, speechClientEvent{
			Type:    "error",
			Code:    "speech_config_error",
			Message: "语音识别配置不正确",
		})
		return
	}

	xf, err := websocket.Dial(xfURL, "", "http://localhost/")
	if err != nil {
		s.logger.Warn("xunfei speech websocket dial failed", "request_id", requestID, "error", err)
		sendSpeechClientEvent(client, nil, speechClientEvent{
			Type:    "error",
			Code:    "speech_network_error",
			Message: "语音识别服务暂时不可用",
		})
		return
	}
	defer xf.Close()

	var clientWriteMu sync.Mutex
	sendSpeechClientEvent(client, &clientWriteMu, speechClientEvent{Type: "ready"})

	done := make(chan struct{})
	sessionIDCh := make(chan string, 1)
	go s.readXunfeiSpeechResults(requestID, xf, client, &clientWriteMu, sessionIDCh, done)

	sessionID := requestID
	for {
		select {
		case sid := <-sessionIDCh:
			if sid != "" {
				sessionID = sid
			}
		default:
		}

		var payload []byte
		if err := websocket.Message.Receive(client, &payload); err != nil {
			close(done)
			return
		}
		if len(payload) == 0 {
			continue
		}
		if isSpeechControlPayload(payload) {
			var control speechClientControl
			if err := json.Unmarshal(payload, &control); err != nil {
				continue
			}
			switch control.Type {
			case "start":
				continue
			case "end":
				sendXunfeiEndMessage(xf, sessionID)
			case "cancel":
				close(done)
				return
			}
			continue
		}
		if err := websocket.Message.Send(xf, payload); err != nil {
			s.logger.Warn("xunfei speech audio send failed", "request_id", requestID, "error", err)
			sendSpeechClientEvent(client, &clientWriteMu, speechClientEvent{
				Type:    "error",
				Code:    "speech_send_failed",
				Message: "语音音频发送失败",
			})
			close(done)
			return
		}
	}
}

func (s *Server) readXunfeiSpeechResults(requestID string, xf *websocket.Conn, client *websocket.Conn, clientWriteMu *sync.Mutex, sessionIDCh chan<- string, done <-chan struct{}) {
	acc := speechAccumulator{}
	seq := 0
	for {
		select {
		case <-done:
			return
		default:
		}

		var raw []byte
		if err := websocket.Message.Receive(xf, &raw); err != nil {
			s.logger.Info("xunfei speech websocket closed", "request_id", requestID, "error", err)
			sendSpeechClientEvent(client, clientWriteMu, speechClientEvent{Type: "closed"})
			return
		}
		result, ok := parseXunfeiSpeechResult(raw)
		if !ok {
			continue
		}
		if result.SessionID != "" {
			select {
			case sessionIDCh <- result.SessionID:
			default:
			}
		}
		eventType, text := acc.Apply(result)
		if text == "" {
			continue
		}
		seq++
		if err := sendSpeechClientEvent(client, clientWriteMu, speechClientEvent{Type: eventType, Text: text, Seq: seq}); err != nil {
			return
		}
		if eventType == "final" {
			sendSpeechClientEvent(client, clientWriteMu, speechClientEvent{Type: "closed"})
			return
		}
	}
}

func (a *speechAccumulator) Apply(result xunfeiSpeechResult) (string, string) {
	if result.Stable && result.Text != "" {
		a.stable = append(a.stable, result.Text)
	}
	stableText := strings.Join(a.stable, "")
	if result.Final {
		if stableText != "" {
			return "final", stableText
		}
		return "final", result.Text
	}
	if result.Stable {
		return "partial", stableText
	}
	if stableText != "" || result.Text != "" {
		return "partial", stableText + result.Text
	}
	return "", ""
}

func sendSpeechClientEvent(client *websocket.Conn, mu *sync.Mutex, event speechClientEvent) error {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	return websocket.JSON.Send(client, event)
}

func sendXunfeiEndMessage(xf *websocket.Conn, sessionID string) {
	payload := map[string]any{"end": true}
	if sessionID != "" {
		payload["sessionId"] = sessionID
	}
	_ = websocket.JSON.Send(xf, payload)
}

func isSpeechControlPayload(payload []byte) bool {
	trimmed := strings.TrimSpace(string(payload))
	return strings.HasPrefix(trimmed, "{") && json.Valid([]byte(trimmed))
}

func speechRealtimeConfigFromEnv() speechRealtimeConfig {
	return speechRealtimeConfig{
		AppID:       strings.TrimSpace(os.Getenv("XUNFEI_APP_ID")),
		APIKey:      strings.TrimSpace(os.Getenv("XUNFEI_API_KEY")),
		APISecret:   strings.TrimSpace(os.Getenv("XUNFEI_API_SECRET")),
		BaseURL:     speechEnvDefault("XUNFEI_RTASR_BASE_URL", "wss://office-api-ast-dx.iflyaisol.com"),
		Path:        speechEnvDefault("XUNFEI_RTASR_PATH", "/ast/communicate/v1"),
		Lang:        speechEnvDefault("XUNFEI_RTASR_LANG", "autodialect"),
		AudioEncode: speechEnvDefault("XUNFEI_RTASR_AUDIO_ENCODE", "pcm_s16le"),
		SampleRate:  speechEnvDefault("XUNFEI_RTASR_SAMPLE_RATE", "16000"),
	}
}

func speechEnvDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (c speechRealtimeConfig) Enabled() bool {
	return c.AppID != "" && c.APIKey != "" && c.APISecret != ""
}

func (c speechRealtimeConfig) SignedURL(uuid string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(c.Path, "/")

	values := url.Values{}
	values.Set("appId", c.AppID)
	values.Set("accessKeyId", c.APIKey)
	values.Set("utc", strconv.FormatInt(time.Now().UTC().UnixMilli(), 10))
	values.Set("audio_encode", c.AudioEncode)
	values.Set("lang", c.Lang)
	values.Set("samplerate", c.SampleRate)
	if uuid != "" {
		values.Set("uuid", uuid)
	}
	signingText := canonicalQuery(values)
	mac := hmac.New(sha1.New, []byte(c.APISecret))
	_, _ = mac.Write([]byte(signingText))
	values.Set("signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, value := range items {
			parts = append(parts, fmt.Sprintf("%s=%s", url.QueryEscape(key), url.QueryEscape(value)))
		}
	}
	return strings.Join(parts, "&")
}

func parseXunfeiSpeechResult(raw []byte) (xunfeiSpeechResult, bool) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return xunfeiSpeechResult{}, false
	}
	if code := intValue(payload["code"]); code != 0 {
		return xunfeiSpeechResult{}, false
	}
	if msgType := stringValue(payload["msg_type"]); msgType != "" && msgType != "result" {
		return xunfeiSpeechResult{SessionID: xunfeiSessionID(payload)}, true
	}
	if resType := stringValue(payload["res_type"]); resType != "" && resType != "asr" {
		return xunfeiSpeechResult{SessionID: xunfeiSessionID(payload)}, true
	}

	data, _ := payload["data"].(map[string]any)
	cn, _ := data["cn"].(map[string]any)
	st, _ := cn["st"].(map[string]any)
	text := xunfeiWords(st["rt"])
	if text == "" && stringValue(payload["text"]) != "" {
		text = stringValue(payload["text"])
	}

	stType := stringValue(st["type"])
	return xunfeiSpeechResult{
		Text:      text,
		Stable:    stType == "0",
		Final:     boolValue(data["ls"]),
		SessionID: xunfeiSessionID(payload),
	}, text != "" || boolValue(data["ls"]) || xunfeiSessionID(payload) != ""
}

func xunfeiWords(value any) string {
	var builder strings.Builder
	rtItems, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, rtItem := range rtItems {
		rt, _ := rtItem.(map[string]any)
		wsItems, _ := rt["ws"].([]any)
		for _, wsItem := range wsItems {
			ws, _ := wsItem.(map[string]any)
			cwItems, _ := ws["cw"].([]any)
			for _, cwItem := range cwItems {
				cw, _ := cwItem.(map[string]any)
				word := stringValue(cw["w"])
				if word == "" {
					continue
				}
				builder.WriteString(word)
			}
		}
	}
	return builder.String()
}

func xunfeiSessionID(payload map[string]any) string {
	for _, key := range []string{"sessionId", "sid", "session_id"} {
		if value := stringValue(payload[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true" || typed == "1"
	case float64:
		return typed != 0
	default:
		return false
	}
}
