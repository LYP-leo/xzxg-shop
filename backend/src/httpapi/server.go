package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Server struct {
	store   *store.MemoryStore
	runtime *agent.Runtime
	logger  *slog.Logger
}

func NewServer(store *store.MemoryStore, runtime *agent.Runtime, logger *slog.Logger) *Server {
	return &Server{store: store, runtime: runtime, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/agent/sessions", s.handleCreateAgentSession)
	mux.HandleFunc("POST /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("POST /api/v1/agent/runs/", s.handleAgentRunAction)
	return s.withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateAgentSession(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if request.Title == "" {
		request.Title = "AI 导购"
	}
	session, err := s.store.CreateSession(r.Context(), request.Title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_session_failed", "创建会话失败")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleAgentSessionAction(w http.ResponseWriter, r *http.Request) {
	sessionID, action, ok := splitSessionAction(r.URL.Path)
	if !ok || action != "messages:stream" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}

	if _, ok := s.store.GetSession(r.Context(), sessionID); !ok {
		writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
		return
	}

	var request struct {
		ClientMessageID string              `json:"client_message_id"`
		Content         string              `json:"content"`
		Attachments     []domain.Attachment `json:"attachments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if strings.TrimSpace(request.Content) == "" {
		writeError(w, http.StatusBadRequest, "empty_content", "消息内容不能为空")
		return
	}

	message, err := s.store.CreateUserMessage(r.Context(), domain.UserMessage{
		SessionID:       sessionID,
		ClientMessageID: request.ClientMessageID,
		Content:         request.Content,
		Attachments:     request.Attachments,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_message_failed", "创建消息失败")
		return
	}

	run, err := s.store.CreateRun(r.Context(), sessionID, message.MessageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_run_failed", "创建 Agent 执行失败")
		return
	}

	s.streamAgentRun(w, r, run, message)
}

func (s *Server) handleAgentRunAction(w http.ResponseWriter, r *http.Request) {
	runID, action, ok := splitRunAction(r.URL.Path)
	if !ok || action != "cancel" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	run, ok := s.store.UpdateRunStatus(r.Context(), runID, domain.RunStatusCanceled)
	if !ok {
		writeError(w, http.StatusNotFound, "run_not_found", "执行不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"run_id": run.RunID, "status": string(run.Status)})
}

func (s *Server) streamAgentRun(w http.ResponseWriter, r *http.Request, run domain.AgentRun, message domain.UserMessage) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "当前环境不支持流式响应")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	emit := func(event domain.SSEEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte("event: " + event.Type + "\n")); err != nil {
			return err
		}
		if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := s.runtime.Stream(r.Context(), run, message, emit); err != nil {
		s.logger.Error("stream agent run failed", "run_id", run.RunID, "error", err)
		s.store.UpdateRunStatus(r.Context(), run.RunID, domain.RunStatusFailed)
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func splitSessionAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/agent/sessions/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func splitRunAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/agent/runs/")
	parts := strings.Split(rest, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
