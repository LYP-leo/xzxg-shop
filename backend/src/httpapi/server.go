package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Server struct {
	store   store.Store
	runtime *agent.Runtime
	logger  *slog.Logger
}

func NewServer(store store.Store, runtime *agent.Runtime, logger *slog.Logger) *Server {
	return &Server{store: store, runtime: runtime, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	mux.HandleFunc("GET /api/v1/categories/tree", s.handleListCategories)
	mux.HandleFunc("GET /api/v1/merchants", s.handleListMerchants)
	mux.HandleFunc("GET /api/v1/products", s.handleListProducts)
	mux.HandleFunc("GET /api/v1/products/", s.handleProductAction)
	mux.HandleFunc("GET /api/v1/cart", s.handleGetCart)
	mux.HandleFunc("POST /api/v1/cart/items", s.handleAddCartItem)
	mux.HandleFunc("PATCH /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("DELETE /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("POST /api/v1/agent/sessions", s.handleCreateAgentSession)
	mux.HandleFunc("POST /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("POST /api/v1/agent/runs/", s.handleAgentRunAction)
	return s.withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	username := strings.TrimSpace(request.Username)
	if username == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "empty_credential", "账号和密码不能为空")
		return
	}
	account, passwordHash, ok := s.store.GetAccountByUsername(r.Context(), username)
	if !ok || passwordHash != hashPassword(request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credential", "账号或密码错误")
		return
	}
	token, err := s.store.CreateAuthToken(r.Context(), account.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_token_failed", "登录失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "account": account})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	account, ok := s.accountFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	writeJSON(w, http.StatusOK, account)
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

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListCategories(r.Context())})
}

func (s *Server) handleListMerchants(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListMerchants(r.Context())})
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	categoryID := r.URL.Query().Get("category_id")
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListProducts(r.Context(), keyword, categoryID)})
}

func (s *Server) handleProductAction(w http.ResponseWriter, r *http.Request) {
	productID, action, ok := splitProductAction(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if action == "skus" {
		writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListProductSKUs(r.Context(), productID)})
		return
	}
	if action != "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	product, ok := s.store.GetProduct(r.Context(), productID)
	if !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleGetCart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetCart(r.Context()))
}

func (s *Server) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ProductID string `json:"product_id"`
		SkuID     string `json:"sku_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if request.ProductID == "" {
		writeError(w, http.StatusBadRequest, "empty_product_id", "商品 ID 不能为空")
		return
	}
	cart, ok := s.store.AddCartItem(r.Context(), request.ProductID, request.SkuID, request.Quantity)
	if !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) handleCartItemAction(w http.ResponseWriter, r *http.Request) {
	cartItemID := strings.TrimPrefix(r.URL.Path, "/api/v1/cart/items/")
	if cartItemID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}

	if r.Method == http.MethodDelete {
		cart, ok := s.store.DeleteCartItem(r.Context(), cartItemID)
		if !ok {
			writeError(w, http.StatusNotFound, "cart_item_not_found", "购物车项不存在")
			return
		}
		writeJSON(w, http.StatusOK, cart)
		return
	}

	var request struct {
		Quantity *int  `json:"quantity"`
		Selected *bool `json:"selected"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	cart, ok := s.store.UpdateCartItem(r.Context(), cartItemID, request.Quantity, request.Selected)
	if !ok {
		writeError(w, http.StatusNotFound, "cart_item_not_found", "购物车项不存在")
		return
	}
	writeJSON(w, http.StatusOK, cart)
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) accountFromRequest(r *http.Request) (domain.Account, bool) {
	header := r.Header.Get("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || token == header {
		return domain.Account{}, false
	}
	return s.store.GetAccountByToken(r.Context(), token)
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
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

func splitProductAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/products/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], "", true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
