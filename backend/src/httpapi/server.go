package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Server struct {
	store   store.Store
	configs configcenter.Center
	runtime *agent.Runtime
	logger  *slog.Logger
}

func NewServer(store store.Store, configs configcenter.Center, runtime *agent.Runtime, logger *slog.Logger) *Server {
	return &Server{store: store, configs: configs, runtime: runtime, logger: logger}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	mux.Handle("GET /api/v1/assets/ecommerce_agent_dataset/", http.StripPrefix("/api/v1/assets/ecommerce_agent_dataset/", http.FileServer(http.Dir(datasetAssetRoot()))))
	mux.HandleFunc("GET /api/v1/categories/tree", s.handleListCategories)
	mux.HandleFunc("GET /api/v1/merchants", s.handleListMerchants)
	mux.HandleFunc("GET /api/v1/products", s.handleListProducts)
	mux.HandleFunc("GET /api/v1/products/", s.handleProductAction)
	mux.HandleFunc("GET /api/v1/merchant/documents", s.handleListMerchantDocuments)
	mux.HandleFunc("POST /api/v1/merchant/documents", s.handleCreateMerchantDocument)
	mux.HandleFunc("GET /api/v1/merchant/orders", s.handleListMerchantOrders)
	mux.HandleFunc("PATCH /api/v1/merchant/orders/", s.handleUpdateMerchantOrder)
	mux.HandleFunc("POST /api/v1/merchant/products", s.handleCreateMerchantProduct)
	mux.HandleFunc("PATCH /api/v1/merchant/products/", s.handleUpdateMerchantProduct)
	mux.HandleFunc("DELETE /api/v1/merchant/products/", s.handleDeleteMerchantProduct)
	mux.HandleFunc("GET /api/v1/admin/accounts", s.handleListAdminAccounts)
	mux.HandleFunc("PATCH /api/v1/admin/accounts/", s.handleUpdateAdminAccount)
	mux.HandleFunc("GET /api/v1/admin/configs", s.handleListAdminConfigs)
	mux.HandleFunc("PATCH /api/v1/admin/configs/", s.handleUpdateAdminConfig)
	mux.HandleFunc("GET /api/v1/admin/documents", s.handleListAdminDocuments)
	mux.HandleFunc("GET /api/v1/admin/orders", s.handleListAdminOrders)
	mux.HandleFunc("GET /api/v1/admin/products", s.handleListAdminProducts)
	mux.HandleFunc("PATCH /api/v1/admin/products/", s.handleUpdateAdminProduct)
	mux.HandleFunc("GET /api/v1/cart", s.handleGetCart)
	mux.HandleFunc("POST /api/v1/cart/items", s.handleAddCartItem)
	mux.HandleFunc("PATCH /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("DELETE /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("GET /api/v1/orders", s.handleListUserOrders)
	mux.HandleFunc("POST /api/v1/orders:checkout", s.handleCheckout)
	mux.HandleFunc("GET /api/v1/agent/sessions", s.handleListAgentSessions)
	mux.HandleFunc("POST /api/v1/agent/sessions", s.handleCreateAgentSession)
	mux.HandleFunc("GET /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("POST /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("GET /api/v1/agent/runs/", s.handleAgentRunTrace)
	mux.HandleFunc("POST /api/v1/agent/runs/", s.handleAgentRunAction)
	return s.withCORS(s.withRequestContext(s.withAccessLog(s.withBodyLimit(s.withAuth(s.withRateLimit(mux))))))
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
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleCreateAgentSession(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
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
	session, err := s.store.CreateSession(r.Context(), account.AccountID, request.Title)
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

func (s *Server) handleCreateMerchantProduct(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	var request domain.ProductUpsertInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, http.StatusBadRequest, "empty_name", "商品名称不能为空")
		return
	}
	request.MerchantID = account.MerchantID
	product, err := s.store.CreateProduct(r.Context(), request)
	if err != nil {
		s.logger.Error("create merchant product failed", "error", err, "merchant_id", account.MerchantID)
		writeError(w, http.StatusInternalServerError, "create_product_failed", "创建商品失败")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleUpdateMerchantProduct(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	productID := strings.TrimPrefix(r.URL.Path, "/api/v1/merchant/products/")
	if productID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	var request domain.ProductUpsertInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, http.StatusBadRequest, "empty_name", "商品名称不能为空")
		return
	}
	request.MerchantID = account.MerchantID
	product, ok := s.store.UpdateProduct(r.Context(), productID, request)
	if !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在或不属于当前商家")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleDeleteMerchantProduct(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	productID := strings.TrimPrefix(r.URL.Path, "/api/v1/merchant/products/")
	if productID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if _, ok := s.store.UpdateProductStatus(r.Context(), account.MerchantID, productID, "deleted"); !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在或不属于当前商家")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"product_id": productID, "status": "deleted"})
}

func (s *Server) handleListMerchantDocuments(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListMerchantDocuments(r.Context(), account.MerchantID)})
}

func (s *Server) handleCreateMerchantDocument(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	var request domain.KnowledgeDocumentInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if strings.TrimSpace(request.Title) == "" || strings.TrimSpace(request.Content) == "" {
		writeError(w, http.StatusBadRequest, "empty_document", "资料标题和内容不能为空")
		return
	}
	if request.DocType == "" {
		request.DocType = "product_detail"
	}
	request.MerchantID = account.MerchantID
	document, err := s.store.CreateMerchantDocument(r.Context(), request)
	if err != nil {
		s.logger.Error("create merchant document failed", "error", err, "merchant_id", account.MerchantID)
		writeError(w, http.StatusInternalServerError, "create_document_failed", "上传资料失败")
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (s *Server) handleListMerchantOrders(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListMerchantOrders(r.Context(), account.MerchantID)})
}

func (s *Server) handleUpdateMerchantOrder(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	orderID := strings.TrimPrefix(r.URL.Path, "/api/v1/merchant/orders/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if !allowedOrderStatus(request.Status) {
		writeError(w, http.StatusBadRequest, "bad_status", "订单状态不合法")
		return
	}
	order, ok := s.store.UpdateOrderStatus(r.Context(), account.MerchantID, orderID, request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "order_not_found", "订单不存在或不属于当前商家")
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) handleListAdminAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListAccounts(r.Context())})
}

func (s *Server) handleUpdateAdminAccount(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	accountID := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/accounts/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if request.Status != "active" && request.Status != "inactive" {
		writeError(w, http.StatusBadRequest, "bad_status", "账号状态不合法")
		return
	}
	account, ok := s.store.UpdateAccountStatus(r.Context(), accountID, request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "account_not_found", "账号不存在")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleListAdminConfigs(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.configs.List(r.Context(), false)})
}

func (s *Server) handleUpdateAdminConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	configKey := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/configs/")
	if configKey == "" || strings.Contains(configKey, "/") {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	var request struct {
		Value       string `json:"value"`
		ValueType   string `json:"value_type"`
		Description string `json:"description"`
		IsSecret    *bool  `json:"is_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	isSecret := strings.Contains(strings.ToLower(configKey), "key") || strings.Contains(strings.ToLower(configKey), "secret")
	if request.IsSecret != nil {
		isSecret = *request.IsSecret
	}
	if isSecret && request.Value == "******" {
		for _, existing := range s.configs.List(r.Context(), true) {
			if existing.ConfigKey == configKey {
				request.Value = existing.ConfigValue
				break
			}
		}
	}
	config, err := s.configs.Upsert(r.Context(), domain.AppConfigInput{
		ConfigKey:   configKey,
		ConfigValue: request.Value,
		ValueType:   request.ValueType,
		Description: request.Description,
		IsSecret:    isSecret,
	})
	if err != nil {
		s.logger.Error("update app config failed", "error", err, "config_key", configKey)
		writeError(w, http.StatusInternalServerError, "update_config_failed", "更新配置失败")
		return
	}
	if config.IsSecret && config.ConfigValue != "" {
		config.ConfigValue = "******"
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) handleListAdminDocuments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListAllDocuments(r.Context())})
}

func (s *Server) handleListAdminOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListAllOrders(r.Context())})
}

func (s *Server) handleListAdminProducts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListAllProducts(r.Context())})
}

func (s *Server) handleUpdateAdminProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	productID := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/products/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if request.Status != "active" && request.Status != "inactive" && request.Status != "deleted" {
		writeError(w, http.StatusBadRequest, "bad_status", "商品状态不合法")
		return
	}
	product, ok := s.store.UpdateProductStatus(r.Context(), "", productID, request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) handleGetCart(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.store.GetCart(r.Context(), account.AccountID))
}

func (s *Server) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
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
	cart, ok := s.store.AddCartItem(r.Context(), account.AccountID, request.ProductID, request.SkuID, request.Quantity)
	if !ok {
		writeError(w, http.StatusNotFound, "product_not_found", "商品不存在")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) handleCartItemAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	cartItemID := strings.TrimPrefix(r.URL.Path, "/api/v1/cart/items/")
	if cartItemID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}

	if r.Method == http.MethodDelete {
		cart, ok := s.store.DeleteCartItem(r.Context(), account.AccountID, cartItemID)
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
	cart, ok := s.store.UpdateCartItem(r.Context(), account.AccountID, cartItemID, request.Quantity, request.Selected)
	if !ok {
		writeError(w, http.StatusNotFound, "cart_item_not_found", "购物车项不存在")
		return
	}
	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) handleListUserOrders(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListUserOrders(r.Context(), account.AccountID)})
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	orders, ok := s.store.CreateOrderFromCart(r.Context(), account.AccountID)
	if !ok {
		writeError(w, http.StatusBadRequest, "empty_cart", "请选择购物车商品后再下单")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

func (s *Server) handleListAgentSessions(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListUserSessions(r.Context(), account.AccountID)})
}

func (s *Server) handleAgentSessionAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/sessions/")
		if sessionID == "" || strings.Contains(sessionID, "/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		detail, ok := s.store.GetSessionDetail(r.Context(), account.AccountID, sessionID)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		writeJSON(w, http.StatusOK, detail)
		return
	}

	sessionID, action, ok := splitSessionAction(r.URL.Path)
	if !ok || action != "messages:stream" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}

	if _, ok := s.store.GetSession(r.Context(), account.AccountID, sessionID); !ok {
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
		AccountID:       account.AccountID,
		ClientMessageID: request.ClientMessageID,
		Content:         request.Content,
		Attachments:     request.Attachments,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_message_failed", "创建消息失败")
		return
	}

	run, err := s.store.CreateRun(r.Context(), account.AccountID, sessionID, message.MessageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_run_failed", "创建 Agent 执行失败")
		return
	}

	s.streamAgentRun(w, r, run, message)
}

func (s *Server) handleAgentRunAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	runID, action, ok := splitRunAction(r.URL.Path)
	if !ok || action != "cancel" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	run, ok := s.store.UpdateRunStatus(r.Context(), account.AccountID, runID, domain.RunStatusCanceled)
	if !ok {
		writeError(w, http.StatusNotFound, "run_not_found", "执行不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"run_id": run.RunID, "status": string(run.Status)})
}

func (s *Server) handleAgentRunTrace(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	runID, ok := splitRunTracePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	events := s.store.ListAgentTrace(r.Context(), account.AccountID, runID)
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
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
		s.store.UpdateRunStatus(r.Context(), run.AccountID, run.RunID, domain.RunStatusFailed)
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
	if account, ok := accountFromContext(r.Context()); ok {
		return account, true
	}
	header := r.Header.Get("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || token == header {
		return domain.Account{}, false
	}
	return s.store.GetAccountByToken(r.Context(), token)
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.Account, bool) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return domain.Account{}, false
	}
	if account.Role != domain.AccountRoleUser {
		writeError(w, http.StatusForbidden, "forbidden", "当前账号不是用户端账号")
		return domain.Account{}, false
	}
	return account, true
}

func (s *Server) requireMerchant(w http.ResponseWriter, r *http.Request) (domain.Account, bool) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return domain.Account{}, false
	}
	if account.Role != domain.AccountRoleMerchant || account.MerchantID == "" {
		writeError(w, http.StatusForbidden, "forbidden", "当前账号不是商家")
		return domain.Account{}, false
	}
	return account, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (domain.Account, bool) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return domain.Account{}, false
	}
	if account.Role != domain.AccountRoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "当前账号不是管理员")
		return domain.Account{}, false
	}
	return account, true
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func datasetAssetRoot() string {
	if root := os.Getenv("ECOMMERCE_DATASET_ROOT"); root != "" {
		return root
	}
	return filepath.Clean(filepath.Join("..", "quality", "data", "ecommerce_agent_dataset"))
}

func allowedOrderStatus(status string) bool {
	switch status {
	case "pending_ship", "shipped", "completed", "canceled":
		return true
	default:
		return false
	}
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

func splitRunTracePath(path string) (string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/agent/runs/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "trace" {
		return "", false
	}
	return parts[0], true
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
	requestID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, map[string]string{"code": code, "message": message, "request_id": requestID})
}
