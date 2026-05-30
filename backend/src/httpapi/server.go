package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/imagevector"
	"github.com/LYP-leo/xzxg-shop/backend/src/objectstore"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
	"github.com/LYP-leo/xzxg-shop/backend/src/retrievalconfig"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Server struct {
	store   store.Store
	configs configcenter.Center
	runtime *agent.Runtime
	logger  *slog.Logger
}

// NewServer 只装配依赖，不做网络监听，方便测试和命令行入口复用。
func NewServer(store store.Store, configs configcenter.Center, runtime *agent.Runtime, logger *slog.Logger) *Server {
	return &Server{store: store, configs: configs, runtime: runtime, logger: logger}
}

// Routes 集中声明所有 API 路由，并在最后套上跨域、鉴权、限流、日志和请求体限制中间件。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	mux.HandleFunc("GET /api/v1/account/profile", s.handleAccountProfile)
	mux.HandleFunc("PATCH /api/v1/account/profile", s.handleAccountProfile)
	mux.HandleFunc("PATCH /api/v1/account/contact", s.handleAccountContact)
	mux.HandleFunc("DELETE /api/v1/account", s.handleDeleteAccount)
	mux.HandleFunc("POST /api/v1/uploads/avatar", s.handleUploadAvatar)
	mux.Handle("GET /api/v1/uploads/avatar/", http.StripPrefix("/api/v1/uploads/avatar/", http.FileServer(http.Dir(avatarUploadRoot()))))
	mux.Handle("GET /api/v1/assets/ecommerce_agent_dataset/", http.StripPrefix("/api/v1/assets/ecommerce_agent_dataset/", http.FileServer(http.Dir(datasetAssetRoot()))))
	mux.HandleFunc("GET /api/v1/categories/tree", s.handleListCategories)
	mux.HandleFunc("GET /api/v1/merchants", s.handleListMerchants)
	mux.HandleFunc("GET /api/v1/products", s.handleListProducts)
	mux.HandleFunc("GET /api/v1/products/", s.handleProductAction)
	mux.HandleFunc("GET /api/v1/promotions", s.handleListPromotions)
	mux.HandleFunc("GET /api/v1/coupons/available", s.handleListCoupons)
	mux.HandleFunc("GET /api/v1/coupons/mine", s.handleListUserCoupons)
	mux.HandleFunc("POST /api/v1/coupons/", s.handleCouponAction)
	mux.HandleFunc("POST /api/v1/files", s.handleUploadFile)
	mux.HandleFunc("GET /api/v1/files/", s.handleGetFile)
	mux.HandleFunc("POST /api/v1/search/image", s.handleSearchImage)
	mux.HandleFunc("GET /api/v1/merchant/documents", s.handleListMerchantDocuments)
	mux.HandleFunc("POST /api/v1/merchant/documents", s.handleCreateMerchantDocument)
	mux.HandleFunc("GET /api/v1/merchant/orders", s.handleListMerchantOrders)
	mux.HandleFunc("PATCH /api/v1/merchant/orders/", s.handleUpdateMerchantOrder)
	mux.HandleFunc("GET /api/v1/merchant/promotions", s.handleListMerchantPromotions)
	mux.HandleFunc("POST /api/v1/merchant/promotions", s.handleCreateMerchantPromotion)
	mux.HandleFunc("PATCH /api/v1/merchant/promotions/", s.handleUpdateMerchantPromotion)
	mux.HandleFunc("GET /api/v1/merchant/reviews", s.handleListMerchantReviews)
	mux.HandleFunc("POST /api/v1/merchant/reviews/", s.handleMerchantReviewAction)
	mux.HandleFunc("POST /api/v1/merchant/products", s.handleCreateMerchantProduct)
	mux.HandleFunc("PATCH /api/v1/merchant/products/", s.handleUpdateMerchantProduct)
	mux.HandleFunc("DELETE /api/v1/merchant/products/", s.handleDeleteMerchantProduct)
	mux.HandleFunc("GET /api/v1/admin/accounts", s.handleListAdminAccounts)
	mux.HandleFunc("PATCH /api/v1/admin/accounts/", s.handleUpdateAdminAccount)
	mux.HandleFunc("GET /api/v1/admin/configs", s.handleListAdminConfigs)
	mux.HandleFunc("PATCH /api/v1/admin/configs/", s.handleUpdateAdminConfig)
	mux.HandleFunc("GET /api/v1/admin/prompts", s.handleListAdminPrompts)
	mux.HandleFunc("PATCH /api/v1/admin/prompts/", s.handleAdminPromptAction)
	mux.HandleFunc("POST /api/v1/admin/prompts/", s.handleAdminPromptAction)
	mux.HandleFunc("GET /api/v1/admin/vector/status", s.handleAdminVectorStatus)
	mux.HandleFunc("GET /api/v1/admin/evals", s.handleAdminEvalDashboard)
	mux.HandleFunc("GET /api/v1/admin/evals/reports/", s.handleAdminEvalReportDetail)
	mux.HandleFunc("GET /api/v1/admin/agent/runs", s.handleListAdminAgentRuns)
	mux.HandleFunc("GET /api/v1/admin/agent/runs/", s.handleAdminAgentRunTrace)
	mux.HandleFunc("GET /api/v1/admin/documents", s.handleListAdminDocuments)
	mux.HandleFunc("GET /api/v1/admin/orders", s.handleListAdminOrders)
	mux.HandleFunc("GET /api/v1/admin/promotions", s.handleListAdminPromotions)
	mux.HandleFunc("POST /api/v1/admin/promotions", s.handleCreateAdminPromotion)
	mux.HandleFunc("PATCH /api/v1/admin/promotions/", s.handleUpdateAdminPromotion)
	mux.HandleFunc("GET /api/v1/admin/reviews", s.handleListAdminReviews)
	mux.HandleFunc("PATCH /api/v1/admin/reviews/", s.handleUpdateAdminReview)
	mux.HandleFunc("GET /api/v1/admin/products", s.handleListAdminProducts)
	mux.HandleFunc("PATCH /api/v1/admin/products/", s.handleUpdateAdminProduct)
	mux.HandleFunc("POST /api/v1/eval/intent", s.handleEvalIntent)
	mux.HandleFunc("POST /api/v1/eval/rag", s.handleEvalRAGRecall)
	mux.HandleFunc("POST /api/v1/eval/image-search", s.handleEvalImageSearch)
	mux.HandleFunc("GET /api/v1/cart", s.handleGetCart)
	mux.HandleFunc("GET /api/v1/cart/discount-preview", s.handleCartDiscountPreview)
	mux.HandleFunc("POST /api/v1/cart/items", s.handleAddCartItem)
	mux.HandleFunc("PATCH /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("DELETE /api/v1/cart/items/", s.handleCartItemAction)
	mux.HandleFunc("GET /api/v1/orders", s.handleListUserOrders)
	mux.HandleFunc("POST /api/v1/orders:checkout", s.handleCheckout)
	mux.HandleFunc("GET /api/v1/orders/", s.handleUserOrderAction)
	mux.HandleFunc("POST /api/v1/orders/", s.handleUserOrderAction)
	mux.HandleFunc("GET /api/v1/agent/sessions", s.handleListAgentSessions)
	mux.HandleFunc("GET /api/v1/agent/sessions/search", s.handleSearchAgentSessions)
	mux.HandleFunc("POST /api/v1/agent/sessions", s.handleCreateAgentSession)
	mux.HandleFunc("GET /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("PATCH /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("POST /api/v1/agent/sessions/", s.handleAgentSessionAction)
	mux.HandleFunc("DELETE /api/v1/agent/sessions/", s.handleAgentSessionAction)
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

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	username := strings.TrimSpace(request.Username)
	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err := validateRegisterInput(username, request.Password, displayName); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_register_input", err.Error())
		return
	}
	if _, _, ok := s.store.GetAccountByUsername(r.Context(), username); ok {
		writeError(w, http.StatusConflict, "username_exists", "账号已存在")
		return
	}
	account, err := s.store.CreateAccount(r.Context(), domain.AccountCreateInput{
		Username:     username,
		PasswordHash: hashPassword(request.Password),
		DisplayName:  displayName,
		Role:         domain.AccountRoleUser,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			writeError(w, http.StatusConflict, "username_exists", "账号已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_account_failed", "注册失败")
		return
	}
	token, err := s.store.CreateAuthToken(r.Context(), account.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_token_failed", "注册成功但登录失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "account": account})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token != "" {
		s.store.DeleteAuthToken(r.Context(), token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAccountProfile(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, account)
		return
	}
	var request struct {
		DisplayName string `json:"display_name"`
		Nickname    string `json:"nickname"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(request.Nickname)
	}
	if displayName == "" && strings.TrimSpace(request.AvatarURL) == "" {
		writeError(w, http.StatusBadRequest, "empty_profile", "昵称或头像不能为空")
		return
	}
	if displayName != "" && len([]rune(displayName)) > 24 {
		writeError(w, http.StatusBadRequest, "bad_display_name", "昵称最多 24 个字符")
		return
	}
	updated, ok := s.store.UpdateAccountProfile(r.Context(), account.AccountID, displayName, request.AvatarURL)
	if !ok {
		writeError(w, http.StatusNotFound, "account_not_found", "账号不存在")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAccountContact(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	var request struct {
		Phone string `json:"phone"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	phone := strings.TrimSpace(request.Phone)
	email := strings.TrimSpace(request.Email)
	if len([]rune(phone)) > 32 || len([]rune(email)) > 128 {
		writeError(w, http.StatusBadRequest, "bad_contact", "手机号或邮箱过长")
		return
	}
	if email != "" && (!strings.Contains(email, "@") || strings.Contains(email, " ")) {
		writeError(w, http.StatusBadRequest, "bad_email", "邮箱格式不正确")
		return
	}
	updated, ok := s.store.UpdateAccountContact(r.Context(), account.AccountID, phone, email)
	if !ok {
		writeError(w, http.StatusNotFound, "account_not_found", "账号不存在")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	if !s.store.DeleteAccount(r.Context(), account.AccountID) {
		writeError(w, http.StatusInternalServerError, "delete_account_failed", "删除账号失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	if err := r.ParseMultipartForm(3 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "bad_upload", "头像上传请求不合法")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "请选择头像文件")
		return
	}
	defer file.Close()
	if header.Size > 2<<20 {
		writeError(w, http.StatusBadRequest, "file_too_large", "头像不能超过 2MB")
		return
	}
	buffer, err := io.ReadAll(io.LimitReader(file, 2<<20+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_file_failed", "读取头像失败")
		return
	}
	mimeType := http.DetectContentType(buffer)
	ext := ".jpg"
	switch mimeType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		writeError(w, http.StatusBadRequest, "bad_image_type", "头像仅支持 JPG、PNG 或 WebP")
		return
	}
	root := avatarUploadRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "mkdir_failed", "保存头像失败")
		return
	}
	filename := account.AccountID + "_" + time.Now().Format("20060102150405") + ext
	if err := os.WriteFile(filepath.Join(root, filename), buffer, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "save_file_failed", "保存头像失败")
		return
	}
	url := "/api/v1/uploads/avatar/" + filename
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       url,
		"mime_type": mimeType,
		"size":      len(buffer),
	})
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
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListMerchants(r.Context()), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	categoryID := r.URL.Query().Get("category_id")
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListProducts(r.Context(), keyword, categoryID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleProductAction(w http.ResponseWriter, r *http.Request) {
	productID, action, ok := splitProductAction(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if action == "skus" {
		// SKU 与商品详情分开查询，前端可在详情页懒加载规格信息。
		page, pageSize := readPagination(r)
		items, total := paginateList(s.store.ListProductSKUs(r.Context(), productID), page, pageSize)
		writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
		return
	}
	if action == "reviews" {
		// 商品评价是公开信息，只返回 visible 状态的评价。
		page, pageSize := readPagination(r)
		items, total := paginateList(s.store.ListProductReviews(r.Context(), productID), page, pageSize)
		writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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

func (s *Server) handleListPromotions(w http.ResponseWriter, r *http.Request) {
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListPromotions(r.Context(), ""), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleListCoupons(w http.ResponseWriter, r *http.Request) {
	account, _ := accountFromContext(r.Context())
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListCoupons(r.Context(), account.AccountID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleListUserCoupons(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListUserCoupons(r.Context(), account.AccountID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleCouponAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	couponID, action, ok := splitCouponAction(r.URL.Path)
	if !ok || action != "claim" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	coupon, ok := s.store.ClaimCoupon(r.Context(), account.AccountID, couponID)
	if !ok {
		writeError(w, http.StatusConflict, "claim_failed", "优惠券不可领取，可能已领过、已过期或已领完")
		return
	}
	writeJSON(w, http.StatusOK, coupon)
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	account, ok := accountFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	maxBytes := int64FromMap(s.configs.GetMap(r.Context()), "files.max_upload_bytes", 10<<20)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "bad_multipart", "文件上传请求不合法或文件过大")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "empty_file", "缺少 file 字段")
		return
	}
	defer file.Close()

	buffer, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_file_failed", "读取文件失败")
		return
	}
	if int64(len(buffer)) > maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "文件超过上传大小限制")
		return
	}
	mimeType := http.DetectContentType(buffer)
	if header.Header.Get("Content-Type") != "" {
		mimeType = header.Header.Get("Content-Type")
	}
	if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "application/pdf") {
		writeError(w, http.StatusBadRequest, "unsupported_file_type", "当前只支持图片和 PDF 文件")
		return
	}

	fileID := "file_" + newRequestID()
	ext := fileExtension(header.Filename, mimeType)
	now := time.Now()
	objectKey := strings.Join([]string{
		"uploads",
		account.AccountID,
		now.Format("2006"),
		now.Format("01"),
		fileID + ext,
	}, "/")
	hash := sha256.Sum256(buffer)
	storage, err := objectstore.NewMinIOFromConfig(s.configs.GetMap(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "object_store_unavailable", "对象存储配置不可用")
		return
	}
	startedAt := time.Now()
	if _, err := storage.Put(r.Context(), objectKey, bytes.NewReader(buffer), int64(len(buffer)), mimeType); err != nil {
		s.logger.Warn("upload file to minio failed", "error", err)
		writeError(w, http.StatusBadGateway, "object_store_unavailable", "对象存储不可用")
		return
	}
	fileMeta, err := s.store.CreateStoredFile(r.Context(), domain.StoredFileInput{
		FileID:          fileID,
		AccountID:       account.AccountID,
		ObjectKey:       objectKey,
		URL:             "/api/v1/files/" + fileID,
		MimeType:        mimeType,
		SizeBytes:       int64(len(buffer)),
		ContentHash:     hex.EncodeToString(hash[:]),
		StorageProvider: "minio",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save_file_failed", "保存文件元数据失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"file":               fileMeta,
		"upload_duration_ms": time.Since(startedAt).Milliseconds(),
	})
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	fileID = strings.Trim(fileID, "/")
	if fileID == "" || strings.Contains(fileID, "..") {
		writeError(w, http.StatusBadRequest, "bad_file_id", "文件 ID 不合法")
		return
	}
	fileMeta, ok := s.store.GetStoredFile(r.Context(), fileID)
	if !ok {
		writeError(w, http.StatusNotFound, "file_not_found", "文件不存在")
		return
	}
	storage, err := objectstore.NewMinIOFromConfig(s.configs.GetMap(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "object_store_unavailable", "对象存储配置不可用")
		return
	}
	object, err := storage.Get(r.Context(), fileMeta.ObjectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "object_not_found", "对象存储文件不存在")
		return
	}
	defer object.Close()
	w.Header().Set("Content-Type", fileMeta.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if fileMeta.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(fileMeta.SizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, object)
}

func (s *Server) handleSearchImage(w http.ResponseWriter, r *http.Request) {
	if _, ok := accountFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	s.writeImageSearchResult(w, r)
}

func (s *Server) handleEvalImageSearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	s.writeImageSearchResult(w, r)
}

func (s *Server) writeImageSearchResult(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	var request struct {
		FileID    string `json:"file_id"`
		ObjectKey string `json:"object_key"`
		ImageURL  string `json:"image_url"`
		TopK      int    `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	downloadStartedAt := time.Now()
	downloadMS := int64(0)
	if request.FileID != "" {
		fileMeta, ok := s.store.GetStoredFile(r.Context(), request.FileID)
		if !ok {
			writeError(w, http.StatusNotFound, "file_not_found", "文件不存在")
			return
		}
		request.ObjectKey = fileMeta.ObjectKey
		storage, err := objectstore.NewMinIOFromConfig(s.configs.GetMap(r.Context()))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "object_store_unavailable", "对象存储配置不可用")
			return
		}
		object, err := storage.Get(r.Context(), fileMeta.ObjectKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "object_not_found", "对象存储文件不存在")
			return
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(object, 1<<20))
		_ = object.Close()
		downloadMS = time.Since(downloadStartedAt).Milliseconds()
	} else if strings.TrimSpace(request.ObjectKey) != "" {
		downloadMS = time.Since(downloadStartedAt).Milliseconds()
	} else if strings.TrimSpace(request.ImageURL) != "" {
		downloadMS = time.Since(downloadStartedAt).Milliseconds()
	} else {
		writeError(w, http.StatusBadRequest, "empty_image", "file_id、object_key 或 image_url 至少传一个")
		return
	}

	embeddingStartedAt := time.Now()
	vector, err := s.imageVectorFromSearchRequest(r.Context(), request.FileID, request.ObjectKey, request.ImageURL)
	embeddingMS := time.Since(embeddingStartedAt).Milliseconds()
	if err != nil {
		writeError(w, http.StatusBadRequest, "image_embedding_failed", "图片解析失败")
		return
	}
	searchStartedAt := time.Now()
	items, searchErr := s.store.SearchProductsByImageVector(r.Context(), vector, request.TopK)
	vectorSearchMS := time.Since(searchStartedAt).Milliseconds()
	status := "matched"
	matchStatus := "ok"
	if searchErr != nil {
		status = "no_match"
		matchStatus = "image_vector_index_unavailable"
		items = []domain.ProductCard{}
	}
	if len(items) == 0 {
		status = "no_match"
		if matchStatus == "ok" {
			matchStatus = "no_vector_hit"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"relevance_status": status,
		"match_status":     matchStatus,
		"items":            items,
		"durations": map[string]int64{
			"total_ms":         time.Since(startedAt).Milliseconds(),
			"download_ms":      downloadMS,
			"embedding_ms":     embeddingMS,
			"vector_search_ms": vectorSearchMS,
			"rerank_ms":        0,
		},
	})
}

func (s *Server) imageVectorFromSearchRequest(ctx context.Context, fileID string, objectKey string, imageURL string) ([]float32, error) {
	if fileID != "" {
		fileMeta, ok := s.store.GetStoredFile(ctx, fileID)
		if !ok {
			return nil, errText("file not found")
		}
		objectKey = fileMeta.ObjectKey
	}
	if objectKey != "" {
		storage, err := objectstore.NewMinIOFromConfig(s.configs.GetMap(ctx))
		if err != nil {
			return nil, err
		}
		object, err := storage.Get(ctx, objectKey)
		if err != nil {
			return nil, err
		}
		defer object.Close()
		return imagevector.FromReader(object)
	}
	return imagevector.FromSource(ctx, imageURL)
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
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListMerchantDocuments(r.Context(), account.MerchantID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListMerchantOrders(r.Context(), account.MerchantID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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

func (s *Server) handleListMerchantPromotions(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListPromotions(r.Context(), account.MerchantID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleCreateMerchantPromotion(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	var request domain.PromotionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	request.Scope = "merchant"
	request.MerchantID = account.MerchantID
	promotion, err := s.store.CreatePromotion(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_promotion_failed", "创建促销失败")
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}

func (s *Server) handleUpdateMerchantPromotion(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	promotionID := strings.TrimPrefix(r.URL.Path, "/api/v1/merchant/promotions/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if !allowedEnabledStatus(request.Status) {
		writeError(w, http.StatusBadRequest, "bad_status", "促销状态不合法")
		return
	}
	promotion, ok := s.store.UpdatePromotionStatus(r.Context(), promotionID, account.MerchantID, request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "promotion_not_found", "促销不存在或不属于当前商家")
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}

func (s *Server) handleListMerchantReviews(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListMerchantReviews(r.Context(), account.MerchantID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleMerchantReviewAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireMerchant(w, r)
	if !ok {
		return
	}
	reviewID, action, ok := splitReviewAction(r.URL.Path, "/api/v1/merchant/reviews/")
	if !ok || action != "reply" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	var request struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	review, ok := s.store.ReplyReview(r.Context(), account.MerchantID, reviewID, request.Reply)
	if !ok {
		writeError(w, http.StatusNotFound, "review_not_found", "评价不存在或不属于当前商家")
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (s *Server) handleListAdminAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.ListAccountsPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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
	page, pageSize := readPagination(r)
	all := s.configs.List(r.Context(), false)
	itemsForPage := make([]domain.AppConfig, 0, len(all))
	for _, item := range all {
		if item.Domain == "prompt" || strings.HasPrefix(item.ConfigKey, "agent.prompt.") {
			continue
		}
		itemsForPage = append(itemsForPage, item)
	}
	items, total := paginateList(itemsForPage, page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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

func (s *Server) handleListAdminPrompts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.ListAgentPromptsPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":           items,
		"page":            page,
		"page_size":       pageSize,
		"total":           total,
		"publish_records": s.store.ListAgentPromptPublishRecords(r.Context(), "", 20),
	})
}

func (s *Server) handleAdminPromptAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	promptKey, publish, ok := splitAdminPromptPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if publish {
		s.handlePublishAdminPrompt(w, r, account, promptKey)
		return
	}
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	var request struct {
		Title       string `json:"title"`
		Content     string `json:"content"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	prompt, err := s.store.SaveAgentPromptDraft(r.Context(), domain.AgentPromptInput{
		PromptKey:   promptKey,
		Title:       request.Title,
		Content:     request.Content,
		Description: request.Description,
		CreatedBy:   account.AccountID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "save_prompt_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, prompt)
}

func (s *Server) handlePublishAdminPrompt(w http.ResponseWriter, r *http.Request, account domain.Account, promptKey string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不支持")
		return
	}
	prompt, record, err := s.store.PublishAgentPrompt(r.Context(), promptKey, account.AccountID, "xzxg-shop-agent-prompts.json")
	if err != nil {
		writeError(w, http.StatusBadRequest, "publish_prompt_failed", err.Error())
		return
	}
	_, err = s.configs.Upsert(r.Context(), domain.AppConfigInput{
		ConfigKey:   prompt.PromptKey,
		ConfigValue: prompt.Content,
		ValueType:   "text",
		Description: prompt.Description,
		Domain:      "prompt",
	})
	if err != nil {
		s.logger.Error("publish prompt to nacos failed", "error", err, "prompt_key", promptKey)
		writeError(w, http.StatusInternalServerError, "publish_nacos_failed", "Prompt 已入库，但同步 Nacos 失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prompt": prompt, "record": record})
}

func (s *Server) handleAdminVectorStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.store.VectorIndexStatus(r.Context()))
}

func (s *Server) handleListAdminAgentRuns(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.ListAgentRunsPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleAdminAgentRunTrace(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	runID, ok := splitAdminRunTracePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	events := s.store.ListAgentTraceByRun(r.Context(), runID)
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleEvalIntent(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var request struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		writeError(w, http.StatusBadRequest, "empty_query", "query 不能为空")
		return
	}
	plan := s.runtime.ClassifyPlan(r.Context(), query)
	writeJSON(w, http.StatusOK, map[string]string{
		"query":           query,
		"route":           plan.Route,
		"intent":          plan.ReferenceIntent(),
		"level":           plan.Level,
		"secondary_level": plan.SecondaryLevel,
	})
}

func (s *Server) handleEvalRAGRecall(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var request struct {
		Keyword string `json:"keyword"`
		Query   string `json:"query"`
		TopK    int    `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	keyword := strings.TrimSpace(request.Keyword)
	if keyword == "" {
		keyword = strings.TrimSpace(request.Query)
	}
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "empty_keyword", "keyword 不能为空")
		return
	}
	topK := request.TopK
	if topK <= 0 || topK > 20 {
		topK = 5
	}
	plan := s.retrievalPlan(r.Context(), keyword)
	plan.Rerank.TopK = topK
	items := s.store.SearchKnowledgeByPlan(r.Context(), plan)
	if len(items) > topK {
		items = items[:topK]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"keyword": keyword,
		"items":   items,
	})
}

func (s *Server) retrievalPlan(ctx context.Context, query string) rag.RetrievalPlan {
	plan := rag.DefaultRetrievalPlan(query)
	retrievalconfig.Apply(&plan, s.configs.GetMap(ctx))
	return plan
}

func (s *Server) handleAdminEvalDashboard(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	datasets := listEvalDatasets()
	reports := listEvalReports()
	writeJSON(w, http.StatusOK, map[string]any{
		"tools":    evalToolSuites(datasets, reports),
		"datasets": datasets,
		"reports":  reports,
	})
}

func (s *Server) handleAdminEvalReportDetail(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	reportID := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/evals/reports/")
	reportID = strings.Trim(reportID, "/")
	if reportID == "" || strings.Contains(reportID, "..") {
		writeError(w, http.StatusBadRequest, "bad_report_id", "报告 ID 不合法")
		return
	}
	parts := strings.Split(reportID, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "bad_report_id", "报告 ID 应为 run/file")
		return
	}
	reportPath := projectPath("quality", "reports", parts[0], parts[1]+".json")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "report_not_found", "测评报告不存在")
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		writeError(w, http.StatusInternalServerError, "bad_report", "测评报告 JSON 不合法")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      reportID,
		"path":    filepath.ToSlash(reportPath),
		"summary": evalReportSummary(payload),
		"results": reportResults(payload),
		"raw":     evalReportSummary(payload),
	})
}

type adminEvalDataset struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CaseCount int       `json:"case_count"`
	UpdatedAt time.Time `json:"updated_at"`
}

type adminEvalReport struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Dataset     string    `json:"dataset"`
	Path        string    `json:"path"`
	GeneratedAt time.Time `json:"generated_at"`
	Total       int       `json:"total"`
	Evaluated   int       `json:"evaluated"`
	Hits        int       `json:"hits"`
	PassRate    float64   `json:"pass_rate"`
}

type adminEvalToolSuite struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Scope          string `json:"scope"`
	DatasetID      string `json:"dataset_id"`
	LatestReportID string `json:"latest_report_id"`
	Command        string `json:"command"`
}

func listEvalDatasets() []adminEvalDataset {
	root := projectPath("quality", "data", "eval")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	items := make([]adminEvalDataset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		info, _ := entry.Info()
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		items = append(items, adminEvalDataset{
			ID:        id,
			Type:      evalTypeFromDataset(id),
			Name:      evalDatasetName(id),
			Path:      filepath.ToSlash(path),
			CaseCount: countJSONLLines(path),
			UpdatedAt: fileModTime(info),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func listEvalReports() []adminEvalReport {
	root := projectPath("quality", "reports")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	items := make([]adminEvalReport, 0)
	for _, runDir := range entries {
		if !runDir.IsDir() {
			continue
		}
		reportDir := filepath.Join(root, runDir.Name())
		files, err := os.ReadDir(reportDir)
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			path := filepath.Join(reportDir, file.Name())
			if report, ok := readEvalReport(path, runDir.Name()); ok {
				items = append(items, report)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GeneratedAt.After(items[j].GeneratedAt) })
	if len(items) > 50 {
		items = items[:50]
	}
	return items
}

func readEvalReport(path string, runID string) (adminEvalReport, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return adminEvalReport{}, false
	}
	var payload struct {
		Type              string  `json:"type"`
		Dataset           string  `json:"dataset"`
		GeneratedAt       string  `json:"generated_at"`
		Total             int     `json:"total"`
		Evaluated         int     `json:"evaluated"`
		Hits              int     `json:"hits"`
		RecallCaseHitRate float64 `json:"recall_case_hit_rate"`
		HitRateAtK        float64 `json:"hit_rate_at_k"`
		Accuracy          float64 `json:"accuracy"`
		PassRate          float64 `json:"pass_rate"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return adminEvalReport{}, false
	}
	generatedAt, _ := time.Parse(time.RFC3339, payload.GeneratedAt)
	passRate := payload.PassRate
	if passRate == 0 {
		passRate = payload.Accuracy
	}
	if passRate == 0 {
		passRate = payload.RecallCaseHitRate
	}
	if passRate == 0 {
		passRate = payload.HitRateAtK
	}
	return adminEvalReport{
		ID:          runID + "/" + strings.TrimSuffix(filepath.Base(path), ".json"),
		Type:        payload.Type,
		Dataset:     payload.Dataset,
		Path:        filepath.ToSlash(path),
		GeneratedAt: generatedAt,
		Total:       payload.Total,
		Evaluated:   payload.Evaluated,
		Hits:        payload.Hits,
		PassRate:    passRate,
	}, true
}

func evalReportSummary(payload map[string]any) map[string]any {
	return map[string]any{
		"type":                                payload["type"],
		"dataset":                             payload["dataset"],
		"generated_at":                        payload["generated_at"],
		"total":                               payload["total"],
		"evaluated":                           payload["evaluated"],
		"hits":                                payload["hits"],
		"correct":                             payload["correct"],
		"accuracy":                            payload["accuracy"],
		"hit_rate_at_k":                       payload["hit_rate_at_k"],
		"mrr":                                 payload["mrr"],
		"by_query_type":                       payload["by_query_type"],
		"by_group":                            payload["by_group"],
		"by_case_type":                        payload["by_case_type"],
		"latency_ms":                          payload["latency_ms"],
		"image_total_duration_ms_p95":         payload["image_total_duration_ms_p95"],
		"image_download_duration_ms_p95":      payload["image_download_duration_ms_p95"],
		"image_embedding_duration_ms_p95":     payload["image_embedding_duration_ms_p95"],
		"image_vector_search_duration_ms_p95": payload["image_vector_search_duration_ms_p95"],
		"image_rerank_duration_ms_p95":        payload["image_rerank_duration_ms_p95"],
		"pass_rate":                           firstNumber(payload["pass_rate"], payload["accuracy"], payload["hit_rate_at_k"], payload["recall_case_hit_rate"]),
	}
}

func reportResults(payload map[string]any) []any {
	results, ok := payload["results"].([]any)
	if !ok {
		return nil
	}
	return results
}

func firstNumber(values ...any) float64 {
	for _, value := range values {
		if number, ok := value.(float64); ok && number != 0 {
			return number
		}
	}
	return 0
}

func evalToolSuites(datasets []adminEvalDataset, reports []adminEvalReport) []adminEvalToolSuite {
	suites := []adminEvalToolSuite{
		{ID: "rag_retriever_eval", Name: "RAG 检索召回", Scope: "tool", DatasetID: "rag_recall_cases", Command: "node quality/evals/run_rag_recall_eval.mjs quality/data/eval/rag_recall_cases.jsonl"},
		{ID: "image_search_eval", Name: "图片搜索", Scope: "tool", DatasetID: "image_search_cases", Command: "node quality/evals/run_image_search_eval.mjs quality/data/eval/image_search_cases.jsonl"},
		{ID: "intent", Name: "意图识别", Scope: "tool", DatasetID: "intent_cases", Command: "node quality/evals/run_intent_eval.mjs quality/data/eval/intent_cases.jsonl"},
		{ID: "agent_e2e", Name: "导购 Agent 端到端", Scope: "agent", DatasetID: "agent_e2e_queries", Command: "node quality/evals/run_agent_e2e.mjs quality/data/eval/agent_e2e_queries.jsonl"},
		{ID: "agent_no_inventory", Name: "Agent 无库存误挂品", Scope: "agent", DatasetID: "agent_no_inventory_cases", Command: "node quality/evals/run_agent_e2e.mjs quality/data/eval/agent_no_inventory_cases.jsonl"},
	}
	datasetSet := make(map[string]bool, len(datasets))
	for _, item := range datasets {
		datasetSet[item.ID] = true
	}
	for i := range suites {
		if !datasetSet[suites[i].DatasetID] {
			suites[i].DatasetID = ""
		}
		for _, report := range reports {
			if report.Type == suites[i].ID || strings.Contains(report.Type, suites[i].ID) || sameEvalFamily(report.Type, suites[i].ID) {
				suites[i].LatestReportID = report.ID
				break
			}
		}
	}
	return suites
}

func sameEvalFamily(reportType string, suiteID string) bool {
	if strings.HasPrefix(reportType, "rag_") && strings.HasPrefix(suiteID, "rag_") {
		return true
	}
	if strings.HasPrefix(reportType, "intent") && strings.HasPrefix(suiteID, "intent") {
		return true
	}
	if strings.HasPrefix(reportType, "image_search") && strings.HasPrefix(suiteID, "image_search") {
		return true
	}
	return false
}

func evalTypeFromDataset(id string) string {
	switch {
	case strings.Contains(id, "no_inventory"):
		return "agent_no_inventory"
	case strings.Contains(id, "rag"):
		return "rag_recall"
	case strings.Contains(id, "intent"):
		return "intent"
	case strings.Contains(id, "image_search"):
		return "image_search_eval"
	case strings.Contains(id, "agent"):
		return "agent_e2e"
	default:
		return "custom"
	}
}

func evalDatasetName(id string) string {
	switch id {
	case "rag_recall_cases":
		return "RAG 召回测试集"
	case "intent_cases":
		return "意图识别测试集"
	case "image_search_cases":
		return "图片搜索测试集"
	case "agent_e2e_queries":
		return "Agent 端到端测试集"
	case "agent_no_inventory_cases":
		return "Agent 无库存误挂品测试集"
	default:
		return id
	}
}

func projectPath(parts ...string) string {
	path := filepath.Join(parts...)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return filepath.Join(append([]string{".."}, parts...)...)
}

func countJSONLLines(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}

func fileModTime(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (s *Server) handleListAdminDocuments(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.ListAllDocumentsPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleListAdminOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.ListAllOrdersPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleListAdminPromotions(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListPromotions(r.Context(), ""), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleCreateAdminPromotion(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	var request domain.PromotionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	promotion, err := s.store.CreatePromotion(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_promotion_failed", "创建促销失败")
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}

func (s *Server) handleUpdateAdminPromotion(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	promotionID := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/promotions/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if !allowedEnabledStatus(request.Status) {
		writeError(w, http.StatusBadRequest, "bad_status", "促销状态不合法")
		return
	}
	promotion, ok := s.store.UpdatePromotionStatus(r.Context(), promotionID, "", request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "promotion_not_found", "促销不存在")
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}

func (s *Server) handleListAdminReviews(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListAllReviews(r.Context()), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleUpdateAdminReview(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	reviewID := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/reviews/")
	var request struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	if request.Status != "visible" && request.Status != "hidden" && request.Status != "deleted" {
		writeError(w, http.StatusBadRequest, "bad_status", "评价状态不合法")
		return
	}
	review, ok := s.store.UpdateReviewStatus(r.Context(), reviewID, request.Status)
	if !ok {
		writeError(w, http.StatusNotFound, "review_not_found", "评价不存在")
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (s *Server) handleListAdminProducts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	page := readPositiveIntQuery(r, "page", 1, 1, 100000)
	pageSize := readPositiveIntQuery(r, "page_size", 10, 1, 100)
	items, total := s.store.ListAllProductsPage(r.Context(), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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

func (s *Server) handleCartDiscountPreview(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.store.PreviewCartDiscount(r.Context(), account.AccountID))
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
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListUserOrders(r.Context(), account.AccountID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
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
	// checkout 只创建待支付订单，不直接进入待发货；支付动作由 /orders/{id}:pay 完成。
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

func (s *Server) handleUserOrderAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		if orderID, orderItemID, ok := splitOrderItemReviewPath(r.URL.Path); ok {
			// 评价强绑定已完成订单项，防止用户绕过订单直接刷评价。
			var request domain.ProductReviewInput
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
				return
			}
			review, ok := s.store.CreateProductReview(r.Context(), account.AccountID, orderID, orderItemID, request)
			if !ok {
				writeError(w, http.StatusConflict, "review_failed", "评价失败，只有已完成订单项可评价且不可重复评价")
				return
			}
			writeJSON(w, http.StatusOK, review)
			return
		}
	}
	orderID, action, hasAction := splitOrderAction(r.URL.Path)
	if orderID == "" {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	if r.Method == http.MethodGet && !hasAction {
		order, ok := s.store.GetOrder(r.Context(), account.AccountID, orderID)
		if !ok {
			writeError(w, http.StatusNotFound, "order_not_found", "订单不存在")
			return
		}
		writeJSON(w, http.StatusOK, order)
		return
	}
	if r.Method != http.MethodPost || !hasAction {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	switch action {
	case "pay":
		// 虚拟支付只推进本地订单状态，不接第三方支付网关。
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
			return
		}
		order, payment, ok := s.store.PayOrder(r.Context(), account.AccountID, orderID, request.Method)
		if !ok {
			writeError(w, http.StatusConflict, "pay_failed", "订单不可支付，可能已支付、取消或超时关闭")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"order": order, "payment": payment})
	case "cancel":
		// MVP 只允许取消待支付订单；已支付订单后续走售后/退款流程。
		var request struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
			return
		}
		order, ok := s.store.CancelOrder(r.Context(), account.AccountID, orderID, request.Reason)
		if !ok {
			writeError(w, http.StatusConflict, "cancel_failed", "订单不可取消，只有待支付订单可以取消")
			return
		}
		writeJSON(w, http.StatusOK, order)
	case "confirm-receipt":
		// 用户确认收货后订单进入 completed，随后才允许评价。
		order, ok := s.store.ConfirmReceipt(r.Context(), account.AccountID, orderID)
		if !ok {
			writeError(w, http.StatusConflict, "confirm_failed", "订单不可确认收货，只有已发货订单可以确认")
			return
		}
		writeJSON(w, http.StatusOK, order)
	default:
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
	}
}

func (s *Server) handleListAgentSessions(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := paginateList(s.store.ListUserSessions(r.Context(), account.AccountID), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleSearchAgentSessions(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	page, pageSize := readPagination(r)
	items, total := s.store.SearchUserSessions(r.Context(), account.AccountID, r.URL.Query().Get("q"), page, pageSize)
	writeJSON(w, http.StatusOK, pagedPayload(items, page, pageSize, total))
}

func (s *Server) handleAgentSessionAction(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":pin") {
		sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/agent/sessions/"), ":pin")
		if sessionID == "" || strings.Contains(sessionID, "/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		var request struct {
			Pinned bool `json:"pinned"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
			return
		}
		session, ok := s.store.PinSession(r.Context(), account.AccountID, sessionID, request.Pinned)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, ":summarize") {
		sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/agent/sessions/"), ":summarize")
		if sessionID == "" || strings.Contains(sessionID, "/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		detail, ok := s.store.GetSessionDetail(r.Context(), account.AccountID, sessionID)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		title, summary := summarizeSessionDetail(detail)
		session, ok := s.store.UpdateSessionSummary(r.Context(), account.AccountID, sessionID, title, summary)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		writeJSON(w, http.StatusOK, session)
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
	if r.Method == http.MethodPatch {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/sessions/")
		if sessionID == "" || strings.Contains(sessionID, "/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		var request struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
			return
		}
		session, ok := s.store.UpdateSessionSummary(r.Context(), account.AccountID, sessionID, request.Title, request.Summary)
		if !ok {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}
	if r.Method == http.MethodDelete {
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/sessions/")
		if sessionID == "" || strings.Contains(sessionID, "/") {
			writeError(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		if !s.store.DeleteSession(r.Context(), account.AccountID, sessionID) {
			writeError(w, http.StatusNotFound, "session_not_found", "会话不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"session_id": sessionID, "status": "deleted"})
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

	var content strings.Builder
	var currentText strings.Builder
	blocks := make([]domain.AgentBlock, 0)
	followups := make([]string, 0)
	segments := make([]domain.AgentSegment, 0)
	flushText := func() {
		text := strings.TrimSpace(currentText.String())
		if text == "" {
			currentText.Reset()
			return
		}
		segments = append(segments, domain.AgentSegment{Type: "text", Text: text})
		currentText.Reset()
	}

	emit := func(event domain.SSEEvent) error {
		switch event.Type {
		case "text_delta":
			content.WriteString(event.Delta)
			currentText.WriteString(event.Delta)
		case "block_delta":
			flushText()
			if event.Block != nil {
				blocks = append(blocks, *event.Block)
				block := *event.Block
				segments = append(segments, domain.AgentSegment{Type: "block", Block: &block})
			}
		case "followups":
			followups = append([]string{}, event.Questions...)
		}
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
		return
	}
	flushText()
	blocksJSON, _ := json.Marshal(blocks)
	followupsJSON, _ := json.Marshal(followups)
	segmentsJSON, _ := json.Marshal(segments)
	s.store.UpdateRunResult(r.Context(), run.AccountID, run.RunID, strings.TrimSpace(content.String()), string(blocksJSON), string(followupsJSON), string(segmentsJSON))
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
	token := bearerToken(r)
	if token == "" {
		return domain.Account{}, false
	}
	return s.store.GetAccountByToken(r.Context(), token)
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || token == header {
		return ""
	}
	return token
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

func validateRegisterInput(username string, password string, displayName string) error {
	if username == "" || password == "" {
		return errText("账号和密码不能为空")
	}
	if len([]rune(username)) < 3 || len([]rune(username)) > 32 {
		return errText("账号长度需要在 3 到 32 个字符之间")
	}
	for _, item := range username {
		if (item >= 'a' && item <= 'z') || (item >= 'A' && item <= 'Z') || (item >= '0' && item <= '9') || item == '_' || item == '-' {
			continue
		}
		return errText("账号只能包含字母、数字、下划线和短横线")
	}
	if len([]rune(password)) < 8 || len([]rune(password)) > 64 {
		return errText("密码长度需要在 8 到 64 个字符之间")
	}
	if len([]rune(displayName)) > 32 {
		return errText("昵称不能超过 32 个字符")
	}
	return nil
}

type errText string

func (e errText) Error() string {
	return string(e)
}

func datasetAssetRoot() string {
	if root := os.Getenv("ECOMMERCE_DATASET_ROOT"); root != "" {
		return root
	}
	return filepath.Clean(filepath.Join("..", "quality", "data", "ecommerce_agent_dataset"))
}

func allowedOrderStatus(status string) bool {
	switch status {
	case "pending_ship", "shipped", "completed", "canceled", "refund_requested", "refunded":
		return true
	default:
		return false
	}
}

func allowedEnabledStatus(status string) bool {
	return status == "active" || status == "inactive"
}

func readPositiveIntQuery(r *http.Request, key string, fallback int, min int, max int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
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

func avatarUploadRoot() string {
	if root := strings.TrimSpace(os.Getenv("AVATAR_UPLOAD_ROOT")); root != "" {
		return root
	}
	return filepath.Join(".", "uploads", "avatar")
}

func int64FromMap(values map[string]string, key string, fallback int64) int64 {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func fileExtension(filename string, mimeType string) string {
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" && len(ext) <= 12 {
		return ext
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func readPagination(r *http.Request) (int, int) {
	page := readPositiveIntQuery(r, "page", 1, 1, 100000)
	pageSize := readPositiveIntQuery(r, "page_size", 10, 1, 100)
	return page, pageSize
}

func pagedPayload[T any](items []T, page int, pageSize int, total int) map[string]any {
	return map[string]any{
		"items":     items,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	}
}

func paginateList[T any](items []T, page int, pageSize int) ([]T, int) {
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return []T{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], total
}

func splitOrderAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/orders/")
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, ":")
	if len(parts) == 1 {
		return parts[0], "", false
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func splitOrderItemReviewPath(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/orders/")
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] != "items" {
		return "", "", false
	}
	itemParts := strings.Split(parts[2], ":")
	if len(itemParts) != 2 || itemParts[0] == "" || itemParts[1] != "review" {
		return "", "", false
	}
	return parts[0], itemParts[0], true
}

func summarizeSessionDetail(detail domain.ChatSessionDetail) (string, string) {
	parts := make([]string, 0, len(detail.Messages))
	for _, message := range detail.Messages {
		content := strings.TrimSpace(message.Content)
		if content != "" {
			parts = append(parts, content)
		}
		if len(parts) >= 3 {
			break
		}
	}
	summary := strings.TrimSpace(strings.Join(parts, "；"))
	if summary == "" {
		summary = strings.TrimSpace(detail.Session.Summary)
	}
	title := deriveShortSessionTitle(summary)
	if summary != "" {
		summary = truncateText(summary, 500)
	}
	return title, summary
}

func deriveShortSessionTitle(content string) string {
	content = strings.TrimSpace(strings.Join(strings.Fields(content), ""))
	replacer := strings.NewReplacer("帮我", "", "我想", "", "请你", "", "推荐", "", "一下", "", "？", "", "！", "", "。", "", "，", "", ",", "", ".", "", "?", "", "!", "", "：", "", ":", "", "；", "")
	content = strings.TrimSpace(replacer.Replace(content))
	if content == "" {
		return "导购咨询"
	}
	categoryHints := []string{"手机", "电脑", "耳机", "鼠标", "键盘", "洁面", "护肤", "面霜", "运动鞋", "衣服", "食品", "订单", "优惠券", "购物车"}
	for _, hint := range categoryHints {
		if strings.Contains(content, hint) {
			if len([]rune(hint)) >= 4 {
				return truncateText(hint, 8)
			}
			return truncateText(hint+"咨询", 8)
		}
	}
	return truncateText(content, 8)
}

func truncateText(input string, limit int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func splitCouponAction(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/coupons/")
	parts := strings.Split(rest, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func splitReviewAction(path string, prefix string) (string, string, bool) {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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

func splitAdminRunTracePath(path string) (string, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/admin/agent/runs/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "trace" {
		return "", false
	}
	return parts[0], true
}

func splitAdminPromptPath(path string) (string, bool, bool) {
	rest := strings.TrimPrefix(path, "/api/v1/admin/prompts/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && parts[0] != "" {
		key, err := url.PathUnescape(parts[0])
		return key, false, err == nil
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "publish" {
		key, err := url.PathUnescape(parts[0])
		return key, true, err == nil
	}
	return "", false, false
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
