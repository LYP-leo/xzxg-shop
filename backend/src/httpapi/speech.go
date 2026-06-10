package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

type speechTTSConfig struct {
	Provider       string
	EnabledFlag    bool
	AppID          string
	APIKey         string
	APISecret      string
	BaseURL        string
	Voice          string
	Speed          int
	Volume         int
	Pitch          int
	AudioEncoding  string
	TextEncoding   string
	TimeoutSeconds int
	MaxRunes       int

	DoubaoEnabledFlag  bool
	DoubaoAppID        string
	DoubaoAPIKey       string
	DoubaoBaseURL      string
	DoubaoCluster      string
	DoubaoVoice        string
	DoubaoEncoding     string
	DoubaoUID          string
	DoubaoTextType     string
	DoubaoOperation    string
	DoubaoWithFrontend int
	DoubaoFrontendType string
	DoubaoSpeedRatio   float64
	DoubaoVolumeRatio  float64
	DoubaoPitchRatio   float64
}

type speechTTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
}

type xunfeiTTSFrame struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	SID     string `json:"sid"`
	Data    struct {
		Audio  string `json:"audio"`
		Status int    `json:"status"`
	} `json:"data"`
}

type doubaoTTSResponse struct {
	ReqID    string `json:"reqid"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Sequence int    `json:"sequence"`
	Data     string `json:"data"`
}

var speechHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil,
	},
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
	cfg := s.speechRealtimeConfig(r.Context())
	if !cfg.Enabled() {
		writeError(w, http.StatusNotImplemented, "speech_not_enabled", "当前后端未配置讯飞语音识别")
		return
	}
	if sampleRate := r.URL.Query().Get("sample_rate"); sampleRate != "" && sampleRate != cfg.SampleRate {
		writeError(w, http.StatusBadRequest, "bad_sample_rate", "当前仅支持 16000Hz PCM 音频")
		return
	}

	websocket.Server{
		Handler: func(client *websocket.Conn) {
			s.proxySpeechRealtime(r.Context(), client, cfg)
		},
		Handshake: func(config *websocket.Config, request *http.Request) error {
			return nil
		},
	}.ServeHTTP(w, r)
}

func (s *Server) handleSpeechTTSConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.speechTTSConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":        cfg.Enabled(),
		"provider":       cfg.Provider,
		"voice":          cfg.DefaultVoice(),
		"max_text_chars": cfg.MaxRunes,
	})
}

func (s *Server) handleSpeechTTS(w http.ResponseWriter, r *http.Request) {
	cfg := s.speechTTSConfig(r.Context())
	if !cfg.Enabled() {
		writeError(w, http.StatusNotImplemented, "tts_not_enabled", "当前后端未配置讯飞语音合成")
		return
	}

	var request speechTTSRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	text := strings.TrimSpace(request.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "empty_text", "合成文本不能为空")
		return
	}
	if cfg.MaxRunes > 0 && len([]rune(text)) > cfg.MaxRunes {
		writeError(w, http.StatusBadRequest, "text_too_long", "合成文本过长")
		return
	}
	voice := strings.TrimSpace(request.Voice)
	if voice == "" {
		voice = cfg.DefaultVoice()
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	audio, err := s.synthesizeSpeechTTS(ctx, cfg, text, voice)
	if err != nil {
		s.logger.Warn("tts synthesis failed", "provider", cfg.Provider, "request_id", requestIDFromContext(r.Context()), "error", redactTTSError(err))
		writeError(w, http.StatusBadGateway, "tts_failed", "语音合成服务暂时不可用")
		return
	}
	w.Header().Set("Content-Type", cfg.ContentType())
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
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
		s.logger.Warn("xunfei speech websocket dial failed", "request_id", requestID, "error", redactXunfeiDialError(err))
		sendSpeechClientEvent(client, nil, speechClientEvent{
			Type:    "error",
			Code:    "speech_network_error",
			Message: "语音识别服务暂时不可用",
		})
		return
	}
	defer xf.Close()
	s.logger.Info("xunfei speech websocket connected", "request_id", requestID)

	var clientWriteMu sync.Mutex
	if err := sendSpeechClientEvent(client, &clientWriteMu, speechClientEvent{Type: "ready"}); err != nil {
		s.logger.Warn("speech client ready send failed", "request_id", requestID, "error", err)
		return
	}
	s.logger.Info("speech client ready sent", "request_id", requestID)

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
			s.logger.Info("speech client websocket closed", "request_id", requestID, "error", err)
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

func (s *Server) synthesizeSpeechTTS(ctx context.Context, cfg speechTTSConfig, text string, voice string) ([]byte, error) {
	if cfg.Provider == "doubao" {
		return s.synthesizeDoubaoSpeechTTS(ctx, cfg, text, voice)
	}
	return s.synthesizeXunfeiSpeechTTS(ctx, cfg, text, voice)
}

func (s *Server) synthesizeXunfeiSpeechTTS(ctx context.Context, cfg speechTTSConfig, text string, voice string) ([]byte, error) {
	xfURL, err := cfg.SignedURL(time.Now())
	if err != nil {
		return nil, err
	}
	xf, err := websocket.Dial(xfURL, "", "http://localhost/")
	if err != nil {
		return nil, err
	}
	defer xf.Close()

	if err := websocket.JSON.Send(xf, cfg.RequestPayload(text, voice)); err != nil {
		return nil, err
	}

	done := make(chan struct{})
	var audio []byte
	var receiveErr error
	go func() {
		defer close(done)
		for {
			var raw []byte
			if err := websocket.Message.Receive(xf, &raw); err != nil {
				if err == io.EOF && len(audio) > 0 {
					return
				}
				receiveErr = err
				return
			}
			chunk, final, err := parseXunfeiTTSFrame(raw)
			if err != nil {
				receiveErr = err
				return
			}
			audio = append(audio, chunk...)
			if final {
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		if receiveErr != nil {
			return nil, receiveErr
		}
		if len(audio) == 0 {
			return nil, fmt.Errorf("xunfei tts returned empty audio")
		}
		return audio, nil
	}
}

func (s *Server) synthesizeDoubaoSpeechTTS(ctx context.Context, cfg speechTTSConfig, text string, voice string) ([]byte, error) {
	payload, err := json.Marshal(cfg.DoubaoRequestPayload(text, voice, newRequestID()))
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.DoubaoBaseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", cfg.DoubaoAuthorizationHeader())

	response, err := speechHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("doubao tts http %d: %s", response.StatusCode, string(body))
	}

	var result doubaoTTSResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 3000 && result.Code != 0 {
		if result.Message == "" {
			result.Message = "doubao tts error"
		}
		return nil, fmt.Errorf("doubao tts code %d: %s", result.Code, result.Message)
	}
	if result.Data == "" {
		return nil, fmt.Errorf("doubao tts returned empty audio")
	}
	audio, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 {
		return nil, fmt.Errorf("doubao tts returned empty audio")
	}
	return audio, nil
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

func (s *Server) speechRealtimeConfig(ctx context.Context) speechRealtimeConfig {
	values := map[string]string{}
	if s != nil && s.configs != nil {
		values = s.configs.GetMap(ctx)
	}
	return speechRealtimeConfig{
		AppID:       speechConfigValue(values, "XUNFEI_APP_ID", "xunfei.app_id", ""),
		APIKey:      speechConfigValue(values, "XUNFEI_API_KEY", "xunfei.api_key", ""),
		APISecret:   speechConfigValue(values, "XUNFEI_API_SECRET", "xunfei.api_secret", ""),
		BaseURL:     speechConfigValue(values, "XUNFEI_RTASR_BASE_URL", "xunfei.rtasr.base_url", "wss://office-api-ast-dx.iflyaisol.com"),
		Path:        speechConfigValue(values, "XUNFEI_RTASR_PATH", "xunfei.rtasr.path", "/ast/communicate/v1"),
		Lang:        speechConfigValue(values, "XUNFEI_RTASR_LANG", "xunfei.rtasr.lang", "autodialect"),
		AudioEncode: speechConfigValue(values, "XUNFEI_RTASR_AUDIO_ENCODE", "xunfei.rtasr.audio_encode", "pcm_s16le"),
		SampleRate:  speechConfigValue(values, "XUNFEI_RTASR_SAMPLE_RATE", "xunfei.rtasr.sample_rate", "16000"),
	}
}

func (s *Server) speechTTSConfig(ctx context.Context) speechTTSConfig {
	values := map[string]string{}
	if s != nil && s.configs != nil {
		values = s.configs.GetMap(ctx)
	}
	provider := strings.ToLower(speechConfigValue(values, "TTS_PROVIDER", "tts.provider", "xunfei"))
	if provider != "doubao" && provider != "xunfei" {
		provider = "xunfei"
	}
	timeoutSeconds := boundedConfigInt(values, "XUNFEI_TTS_TIMEOUT_SECONDS", "xunfei.tts.timeout_seconds", 20, 1, 60)
	maxRunes := boundedConfigInt(values, "XUNFEI_TTS_MAX_RUNES", "xunfei.tts.max_runes", 800, 1, 2000)
	if provider == "doubao" {
		timeoutSeconds = boundedConfigInt(values, "DOUBAO_TTS_TIMEOUT_SECONDS", "doubao.tts.timeout_seconds", timeoutSeconds, 1, 60)
		maxRunes = boundedConfigInt(values, "DOUBAO_TTS_MAX_RUNES", "doubao.tts.max_runes", maxRunes, 1, 2000)
	}
	return speechTTSConfig{
		Provider:       provider,
		EnabledFlag:    boolConfigValue(values, "XUNFEI_TTS_ENABLED", "xunfei.tts.enabled", false),
		AppID:          speechConfigValue(values, "XUNFEI_TTS_APP_ID", "xunfei.tts.app_id", speechConfigValue(values, "XUNFEI_APP_ID", "xunfei.app_id", "")),
		APIKey:         speechConfigValue(values, "XUNFEI_TTS_API_KEY", "xunfei.tts.api_key", speechConfigValue(values, "XUNFEI_API_KEY", "xunfei.api_key", "")),
		APISecret:      speechConfigValue(values, "XUNFEI_TTS_API_SECRET", "xunfei.tts.api_secret", speechConfigValue(values, "XUNFEI_API_SECRET", "xunfei.api_secret", "")),
		BaseURL:        speechConfigValue(values, "XUNFEI_TTS_BASE_URL", "xunfei.tts.base_url", "wss://tts-api.xfyun.cn/v2/tts"),
		Voice:          speechConfigValue(values, "XUNFEI_TTS_VOICE", "xunfei.tts.voice", "xiaoyan"),
		Speed:          boundedConfigInt(values, "XUNFEI_TTS_SPEED", "xunfei.tts.speed", 50, 0, 100),
		Volume:         boundedConfigInt(values, "XUNFEI_TTS_VOLUME", "xunfei.tts.volume", 50, 0, 100),
		Pitch:          boundedConfigInt(values, "XUNFEI_TTS_PITCH", "xunfei.tts.pitch", 50, 0, 100),
		AudioEncoding:  speechConfigValue(values, "XUNFEI_TTS_AUDIO_ENCODING", "xunfei.tts.audio_encoding", "lame"),
		TextEncoding:   speechConfigValue(values, "XUNFEI_TTS_TEXT_ENCODING", "xunfei.tts.text_encoding", "UTF8"),
		TimeoutSeconds: timeoutSeconds,
		MaxRunes:       maxRunes,

		DoubaoEnabledFlag:  boolConfigValue(values, "DOUBAO_TTS_ENABLED", "doubao.tts.enabled", false),
		DoubaoAppID:        speechConfigValue(values, "DOUBAO_TTS_APP_ID", "doubao.tts.app_id", ""),
		DoubaoAPIKey:       speechConfigValue(values, "DOUBAO_TTS_API_KEY", "doubao.tts.api_key", speechConfigValue(values, "DOUBAO_TTS_TOKEN", "doubao.tts.token", "")),
		DoubaoBaseURL:      speechConfigValue(values, "DOUBAO_TTS_BASE_URL", "doubao.tts.base_url", "https://openspeech.bytedance.com/api/v1/tts"),
		DoubaoCluster:      speechConfigValue(values, "DOUBAO_TTS_CLUSTER", "doubao.tts.cluster", "volcano_tts"),
		DoubaoVoice:        speechConfigValue(values, "DOUBAO_TTS_VOICE", "doubao.tts.voice", "BV700_streaming"),
		DoubaoEncoding:     speechConfigValue(values, "DOUBAO_TTS_ENCODING", "doubao.tts.encoding", speechConfigValue(values, "DOUBAO_TTS_AUDIO_ENCODING", "doubao.tts.audio_encoding", "mp3")),
		DoubaoUID:          speechConfigValue(values, "DOUBAO_TTS_UID", "doubao.tts.uid", "xzxg-shop"),
		DoubaoTextType:     speechConfigValue(values, "DOUBAO_TTS_TEXT_TYPE", "doubao.tts.text_type", "plain"),
		DoubaoOperation:    speechConfigValue(values, "DOUBAO_TTS_OPERATION", "doubao.tts.operation", "query"),
		DoubaoWithFrontend: boundedConfigInt(values, "DOUBAO_TTS_WITH_FRONTEND", "doubao.tts.with_frontend", 1, 0, 1),
		DoubaoFrontendType: speechConfigValue(values, "DOUBAO_TTS_FRONTEND_TYPE", "doubao.tts.frontend_type", "unitTson"),
		DoubaoSpeedRatio:   boundedConfigFloat(values, "DOUBAO_TTS_SPEED_RATIO", "doubao.tts.speed_ratio", 1.0, 0.5, 2.0),
		DoubaoVolumeRatio:  boundedConfigFloat(values, "DOUBAO_TTS_VOLUME_RATIO", "doubao.tts.volume_ratio", 1.0, 0.1, 3.0),
		DoubaoPitchRatio:   boundedConfigFloat(values, "DOUBAO_TTS_PITCH_RATIO", "doubao.tts.pitch_ratio", 1.0, 0.5, 2.0),
	}
}

func speechConfigValue(values map[string]string, envKey string, configKey string, fallback string) string {
	if value := strings.TrimSpace(values[envKey]); value != "" {
		return value
	}
	if value := strings.TrimSpace(values[configKey]); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
		return value
	}
	return fallback
}

func boundedConfigInt(values map[string]string, envKey string, configKey string, fallback int, min int, max int) int {
	raw := speechConfigValue(values, envKey, configKey, "")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func boundedConfigFloat(values map[string]string, envKey string, configKey string, fallback float64, min float64, max float64) float64 {
	raw := speechConfigValue(values, envKey, configKey, "")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func boolConfigValue(values map[string]string, envKey string, configKey string, fallback bool) bool {
	raw := strings.ToLower(speechConfigValue(values, envKey, configKey, ""))
	if raw == "" {
		return fallback
	}
	return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
}

func (c speechRealtimeConfig) Enabled() bool {
	return c.AppID != "" && c.APIKey != "" && c.APISecret != ""
}

func (c speechTTSConfig) Enabled() bool {
	if c.Provider == "doubao" {
		return c.DoubaoEnabledFlag && c.DoubaoAppID != "" && c.DoubaoAPIKey != "" && c.DoubaoBaseURL != "" && c.DoubaoCluster != ""
	}
	return c.EnabledFlag && c.AppID != "" && c.APIKey != "" && c.APISecret != "" && c.BaseURL != ""
}

func (c speechTTSConfig) DefaultVoice() string {
	if c.Provider == "doubao" {
		return c.DoubaoVoice
	}
	return c.Voice
}

func (c speechTTSConfig) ContentType() string {
	encoding := strings.ToLower(c.AudioEncoding)
	if c.Provider == "doubao" {
		encoding = strings.ToLower(c.DoubaoEncoding)
	}
	switch encoding {
	case "mp3", "lame":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "ogg", "opus":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
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
	values.Set("utc", time.Now().Format("2006-01-02T15:04:05-0700"))
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

func (c speechTTSConfig) SignedURL(now time.Time) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}
	host := base.Host
	date := now.UTC().Format(http.TimeFormat)
	path := base.EscapedPath()
	if path == "" {
		path = "/v2/tts"
	}
	signatureOrigin := "host: " + host + "\n" + "date: " + date + "\n" + "GET " + path + " HTTP/1.1"
	mac := hmac.New(sha256.New, []byte(c.APISecret))
	_, _ = mac.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`, c.APIKey, signature)

	query := base.Query()
	query.Set("authorization", base64.StdEncoding.EncodeToString([]byte(authorizationOrigin)))
	query.Set("date", date)
	query.Set("host", host)
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func (c speechTTSConfig) RequestPayload(text string, voice string) map[string]any {
	if voice == "" {
		voice = c.Voice
	}
	return map[string]any{
		"common": map[string]any{"app_id": c.AppID},
		"business": map[string]any{
			"aue":    c.AudioEncoding,
			"sfl":    1,
			"vcn":    voice,
			"speed":  c.Speed,
			"volume": c.Volume,
			"pitch":  c.Pitch,
			"tte":    c.TextEncoding,
		},
		"data": map[string]any{
			"status": 2,
			"text":   base64.StdEncoding.EncodeToString([]byte(text)),
		},
	}
}

func (c speechTTSConfig) DoubaoRequestPayload(text string, voice string, reqID string) map[string]any {
	if voice == "" {
		voice = c.DoubaoVoice
	}
	return map[string]any{
		"app": map[string]any{
			"appid":   c.DoubaoAppID,
			"token":   c.DoubaoAPIKey,
			"cluster": c.DoubaoCluster,
		},
		"user": map[string]any{
			"uid": c.DoubaoUID,
		},
		"audio": map[string]any{
			"voice_type":   voice,
			"encoding":     c.DoubaoEncoding,
			"speed_ratio":  c.DoubaoSpeedRatio,
			"volume_ratio": c.DoubaoVolumeRatio,
			"pitch_ratio":  c.DoubaoPitchRatio,
		},
		"request": map[string]any{
			"reqid":         reqID,
			"text":          text,
			"text_type":     c.DoubaoTextType,
			"operation":     c.DoubaoOperation,
			"with_frontend": c.DoubaoWithFrontend,
			"frontend_type": c.DoubaoFrontendType,
		},
	}
}

func (c speechTTSConfig) DoubaoAuthorizationHeader() string {
	return "Bearer;" + c.DoubaoAPIKey
}

func redactTTSError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if index := strings.Index(message, "?"); index >= 0 {
		message = message[:index] + "?<redacted>"
	}
	return message
}

func redactXunfeiDialError(err error) string {
	return redactTTSError(err)
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

func parseXunfeiTTSFrame(raw []byte) ([]byte, bool, error) {
	var frame xunfeiTTSFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, false, err
	}
	if frame.Code != 0 {
		if frame.Message == "" {
			frame.Message = "xunfei tts error"
		}
		return nil, false, fmt.Errorf("xunfei tts code %d: %s", frame.Code, frame.Message)
	}
	var audio []byte
	if frame.Data.Audio != "" {
		decoded, err := base64.StdEncoding.DecodeString(frame.Data.Audio)
		if err != nil {
			return nil, false, err
		}
		audio = decoded
	}
	return audio, frame.Data.Status == 2, nil
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
