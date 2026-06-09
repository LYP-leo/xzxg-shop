package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/imagevector"
	"github.com/LYP-leo/xzxg-shop/backend/src/objectstore"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

const (
	toolSearchProducts  = "search_products"
	toolSearchImage     = "search_image_products"
	toolSearchKnowledge = "search_knowledge"
	toolGetCart         = "get_cart"
	toolAddCartItem     = "add_cart_item"
	toolUpdateCartItem  = "update_cart_item"
	toolDeleteCartItem  = "delete_cart_item"
	toolCheckout        = "checkout"
	toolListOrders      = "list_orders"
	toolGetOrder        = "get_order"
	toolPayOrder        = "pay_order"
	toolCancelOrder     = "cancel_order"
	toolConfirmReceipt  = "confirm_receipt"
	toolPreviewDiscount = "preview_discount"
	toolListCoupons     = "list_coupons"
	toolListUserCoupons = "list_user_coupons"
	toolClaimCoupon     = "claim_coupon"
	toolListPromotions  = "list_promotions"
	toolListReviews     = "list_product_reviews"
	toolCreateReview    = "create_product_review"
)

const (
	relevanceOK      = "ok"
	relevanceWeak    = "weak"
	relevanceNoMatch = "no_match"
)

type reactAction struct {
	Type      string          `json:"type"`
	Tool      string          `json:"tool,omitempty"`
	Skill     string          `json:"skill,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Text      string          `json:"text,omitempty"`
	Blocks    []reactBlock    `json:"blocks,omitempty"`
}

type reactBlock struct {
	Type       string   `json:"type"`
	ProductIDs []string `json:"product_ids,omitempty"`
	ChunkIDs   []string `json:"chunk_ids,omitempty"`
}

type toolObservation struct {
	Tool                string              `json:"tool"`
	OK                  bool                `json:"ok"`
	Message             string              `json:"message,omitempty"`
	Result              map[string]any      `json:"result,omitempty"`
	ProductIDs          []string            `json:"product_ids,omitempty"`
	CandidateProductIDs []string            `json:"candidate_product_ids,omitempty"`
	DroppedProductIDs   []string            `json:"dropped_product_ids,omitempty"`
	ChunkIDs            []string            `json:"chunk_ids,omitempty"`
	RelevanceStatus     string              `json:"relevance_status,omitempty"`
	RelevanceReason     string              `json:"relevance_reason,omitempty"`
	Blocks              []domain.AgentBlock `json:"blocks,omitempty"`
	Cart                *domain.Cart        `json:"-"`
	Orders              []domain.Order      `json:"-"`
	DurationMS          int64               `json:"duration_ms"`
}

type toolExecutionContext struct {
	AllowedAddProductIDs map[string]bool
	Attachments          []domain.Attachment
}

type imageToolSource struct {
	FileID    string
	ObjectKey string
	ImageURL  string
}

var productIDPattern = regexp.MustCompile(`\bp_[A-Za-z0-9_]+\b`)

type productSearchArguments struct {
	Brands     []string `json:"brands,omitempty"`
	Terms      []string `json:"terms,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type productSearchStructuredArguments struct {
	Constraints productSearchArguments `json:"constraints,omitempty"`
	Negative    productSearchArguments `json:"negative,omitempty"`
}

type ProductSearchEvalRequest struct {
	Query              string                 `json:"query"`
	Limit              int                    `json:"limit"`
	Constraints        productSearchArguments `json:"constraints,omitempty"`
	Negative           productSearchArguments `json:"negative,omitempty"`
	RerankModelEnabled *bool                  `json:"rerank_model_enabled,omitempty"`
	LLMFilterEnabled   *bool                  `json:"llm_filter_enabled,omitempty"`
}

type ProductSearchEvalResult struct {
	Query               string   `json:"query"`
	ProductIDs          []string `json:"product_ids"`
	CandidateProductIDs []string `json:"candidate_product_ids"`
	DroppedProductIDs   []string `json:"dropped_product_ids"`
	RelevanceStatus     string   `json:"relevance_status"`
	RelevanceReason     string   `json:"relevance_reason"`
	DurationMS          int64    `json:"duration_ms"`
	Items               []any    `json:"items"`
	Rerank              any      `json:"rerank,omitempty"`
}

func (r *Runtime) EvaluateProductSearch(ctx context.Context, run domain.AgentRun, request ProductSearchEvalRequest) ProductSearchEvalResult {
	payload, _ := json.Marshal(request)
	startedAt := time.Now()
	observation := r.toolSearchProducts(ctx, run, payload)
	items := []any{}
	if rawItems, ok := observation.Result["items"].([]map[string]any); ok {
		for _, item := range rawItems {
			items = append(items, item)
		}
	}
	rerank := observation.Result["rerank"]
	duration := observation.DurationMS
	if duration <= 0 {
		duration = time.Since(startedAt).Milliseconds()
	}
	return ProductSearchEvalResult{
		Query:               strings.TrimSpace(request.Query),
		ProductIDs:          observation.ProductIDs,
		CandidateProductIDs: observation.CandidateProductIDs,
		DroppedProductIDs:   observation.DroppedProductIDs,
		RelevanceStatus:     observation.RelevanceStatus,
		RelevanceReason:     observation.RelevanceReason,
		DurationMS:          duration,
		Items:               items,
		Rerank:              rerank,
	}
}

func productIDSetFromText(text string) map[string]bool {
	ids := make(map[string]bool)
	for _, id := range productIDPattern.FindAllString(text, -1) {
		ids[id] = true
	}
	return ids
}

func productIDsFromText(text string) []string {
	seen := make(map[string]bool)
	ids := make([]string, 0)
	for _, id := range productIDPattern.FindAllString(text, -1) {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func addProductIDsToSet(set map[string]bool, ids ...string) {
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
}

func (r *Runtime) executeTool(ctx context.Context, run domain.AgentRun, call reactAction, execCtx toolExecutionContext) toolObservation {
	startedAt := time.Now()
	tool := strings.TrimSpace(call.Tool)
	observation := toolObservation{Tool: tool, OK: false}
	defer func() {
		observation.DurationMS = time.Since(startedAt).Milliseconds()
	}()

	if tool == "" {
		observation.Message = "工具名称为空"
		return observation
	}
	if r.store.IsRunCanceled(ctx, run.RunID) {
		observation.Message = "运行已取消"
		return observation
	}

	switch tool {
	case toolSearchProducts:
		observation = r.toolSearchProducts(ctx, run, call.Arguments)
	case toolSearchImage:
		observation = r.toolSearchImageProducts(ctx, run, call.Arguments, execCtx.Attachments)
	case toolSearchKnowledge:
		observation = r.toolSearchKnowledge(ctx, call.Arguments)
	case toolGetCart:
		observation = r.toolGetCart(ctx, run.AccountID)
	case toolAddCartItem:
		observation = r.toolAddCartItem(ctx, run.AccountID, call.Arguments, execCtx.AllowedAddProductIDs)
	case toolUpdateCartItem:
		observation = r.toolUpdateCartItem(ctx, run.AccountID, call.Arguments)
	case toolDeleteCartItem:
		observation = r.toolDeleteCartItem(ctx, run.AccountID, call.Arguments)
	case toolCheckout:
		observation = r.toolCheckout(ctx, run.AccountID)
	case toolListOrders:
		observation = r.toolListOrders(ctx, run.AccountID, call.Arguments)
	case toolGetOrder:
		observation = r.toolGetOrder(ctx, run.AccountID, call.Arguments)
	case toolPayOrder:
		observation = r.toolPayOrder(ctx, run.AccountID, call.Arguments)
	case toolCancelOrder:
		observation = r.toolCancelOrder(ctx, run.AccountID, call.Arguments)
	case toolConfirmReceipt:
		observation = r.toolConfirmReceipt(ctx, run.AccountID, call.Arguments)
	case toolPreviewDiscount:
		observation = r.toolPreviewDiscount(ctx, run.AccountID)
	case toolListCoupons:
		observation = r.toolListCoupons(ctx, run.AccountID, call.Arguments)
	case toolListUserCoupons:
		observation = r.toolListUserCoupons(ctx, run.AccountID, call.Arguments)
	case toolClaimCoupon:
		observation = r.toolClaimCoupon(ctx, run.AccountID, call.Arguments)
	case toolListPromotions:
		observation = r.toolListPromotions(ctx, call.Arguments)
	case toolListReviews:
		observation = r.toolListProductReviews(ctx, call.Arguments)
	case toolCreateReview:
		observation = r.toolCreateProductReview(ctx, run.AccountID, call.Arguments)
	default:
		observation.Message = "未知工具：" + tool
	}
	observation.Tool = tool
	observation.DurationMS = time.Since(startedAt).Milliseconds()
	return observation
}

func (r *Runtime) toolSearchImageProducts(ctx context.Context, run domain.AgentRun, raw json.RawMessage, attachments []domain.Attachment) toolObservation {
	var args struct {
		FileID    string `json:"file_id"`
		ObjectKey string `json:"object_key"`
		ImageURL  string `json:"image_url"`
		Limit     int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	args.FileID = strings.TrimSpace(args.FileID)
	args.ObjectKey = strings.TrimSpace(args.ObjectKey)
	args.ImageURL = strings.TrimSpace(args.ImageURL)
	normalizeImageToolSource(&args.FileID, &args.ObjectKey, &args.ImageURL)
	if args.FileID == "" && args.ObjectKey == "" && args.ImageURL == "" {
		if source := firstImageAttachment(attachments); source != nil {
			args.FileID = source.FileID
			args.ObjectKey = source.ObjectKey
			args.ImageURL = source.ImageURL
		}
	}
	if args.FileID == "" && args.ObjectKey == "" && args.ImageURL == "" {
		return toolObservation{Tool: toolSearchImage, Message: "没有可检索的图片；请先上传图片或提供 image_url"}
	}

	limit := clampLimit(args.Limit, 5, 10)
	embeddingStartedAt := time.Now()
	vector, err := r.imageVectorFromToolArgs(ctx, args.FileID, args.ObjectKey, args.ImageURL)
	embeddingMS := time.Since(embeddingStartedAt).Milliseconds()
	if err != nil {
		return toolObservation{
			Tool:            toolSearchImage,
			Message:         "图片向量生成失败",
			RelevanceStatus: relevanceNoMatch,
			RelevanceReason: err.Error(),
			Result: map[string]any{
				"embedding_ms": embeddingMS,
				"source":       map[string]any{"file_id": args.FileID, "object_key": args.ObjectKey, "image_url": args.ImageURL},
			},
		}
	}

	searchStartedAt := time.Now()
	products, searchErr := r.store.SearchProductsByImageVector(ctx, vector, limit)
	searchMS := time.Since(searchStartedAt).Milliseconds()
	if searchErr != nil {
		return toolObservation{
			Tool:            toolSearchImage,
			Message:         "图片向量库检索失败",
			RelevanceStatus: relevanceNoMatch,
			RelevanceReason: searchErr.Error(),
			Result: map[string]any{
				"embedding_ms": embeddingMS,
				"search_ms":    searchMS,
				"source":       map[string]any{"file_id": args.FileID, "object_key": args.ObjectKey, "image_url": args.ImageURL},
			},
		}
	}
	items := make([]map[string]any, 0, len(products))
	productIDs := make([]string, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ProductID)
		items = append(items, map[string]any{
			"product_id":       product.ProductID,
			"sku_id":           product.SkuID,
			"name":             product.Name,
			"brand":            product.Brand,
			"price":            product.Price,
			"image_url":        product.ImageURL,
			"stock_status":     product.StockStatus,
			"merchant_name":    product.MerchantName,
			"selling_points":   product.SellingPoints,
			"recommend_reason": truncateRunes(product.RecommendReason, 180),
		})
	}
	status := relevanceOK
	reason := "图片向量检索命中相似商品"
	message := fmt.Sprintf("根据图片检索到 %d 个相似商品", len(items))
	if len(items) == 0 {
		status = relevanceNoMatch
		reason = "图片向量库没有命中相似商品"
		message = "没有检索到相似商品"
	}
	return toolObservation{
		Tool:            toolSearchImage,
		OK:              true,
		Message:         message,
		Result:          map[string]any{"items": items, "embedding_ms": embeddingMS, "search_ms": searchMS, "source": map[string]any{"file_id": args.FileID, "object_key": args.ObjectKey, "image_url": args.ImageURL}},
		ProductIDs:      productIDs,
		RelevanceStatus: status,
		RelevanceReason: reason,
	}
}

func (r *Runtime) imageVectorFromToolArgs(ctx context.Context, fileID string, objectKey string, imageURL string) ([]float32, error) {
	embedder := imagevector.NewEmbedderFromMap(r.configs.GetMap(ctx), "")
	if fileID != "" {
		fileMeta, ok := r.store.GetStoredFile(ctx, fileID)
		if !ok {
			return nil, errors.New("file not found")
		}
		objectKey = fileMeta.ObjectKey
	}
	if objectKey != "" {
		storage, err := objectstore.NewMinIOFromConfig(r.configs.GetMap(ctx))
		if err != nil {
			return nil, err
		}
		object, err := storage.Get(ctx, objectKey)
		if err != nil {
			return nil, err
		}
		defer object.Close()
		return embedder.EmbedReader(ctx, object)
	}
	return embedder.EmbedSource(ctx, imageURL)
}

func firstImageAttachment(attachments []domain.Attachment) *imageToolSource {
	for _, attachment := range attachments {
		if attachment.Type != "image" {
			continue
		}
		fileID := strings.TrimSpace(attachment.FileID)
		if fileID == "" {
			fileID = strings.TrimSpace(attachment.AttachmentID)
		}
		source := imageToolSource{
			FileID:    fileID,
			ObjectKey: strings.TrimSpace(attachment.ObjectKey),
			ImageURL:  strings.TrimSpace(attachment.URL),
		}
		normalizeImageToolSource(&source.FileID, &source.ObjectKey, &source.ImageURL)
		if source.FileID != "" || source.ObjectKey != "" || source.ImageURL != "" {
			return &source
		}
	}
	return nil
}

func normalizeImageToolSource(fileID *string, objectKey *string, imageURL *string) {
	*fileID = strings.TrimSpace(*fileID)
	*objectKey = strings.TrimSpace(*objectKey)
	*imageURL = strings.TrimSpace(*imageURL)
	if *fileID != "" || *imageURL == "" {
		return
	}
	if parsed := fileIDFromAPIFileURL(*imageURL); parsed != "" {
		*fileID = parsed
		*imageURL = ""
	}
}

func fileIDFromAPIFileURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	const marker = "/api/v1/files/"
	index := strings.Index(raw, marker)
	if index < 0 {
		return ""
	}
	rest := raw[index+len(marker):]
	if cut := strings.IndexAny(rest, "?#/"); cut >= 0 {
		rest = rest[:cut]
	}
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "file_") {
		return rest
	}
	return ""
}

func (r *Runtime) toolSearchProducts(ctx context.Context, run domain.AgentRun, raw json.RawMessage) toolObservation {
	var args struct {
		Query              string                 `json:"query"`
		Limit              int                    `json:"limit"`
		Constraints        productSearchArguments `json:"constraints"`
		Negative           productSearchArguments `json:"negative"`
		RerankModelEnabled *bool                  `json:"rerank_model_enabled,omitempty"`
		LLMFilterEnabled   *bool                  `json:"llm_filter_enabled,omitempty"`
	}
	_ = json.Unmarshal(raw, &args)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolObservation{Tool: toolSearchProducts, Message: "query 不能为空"}
	}
	limit := clampLimit(args.Limit, 5, 10)
	configValues := map[string]string{}
	if r.configs != nil {
		configValues = r.configs.GetMap(ctx)
	}
	initialVectorLimit := intFromMap(configValues, "retrieval.product.initial_vector_limit", 80)
	products := r.store.SearchProductsLimit(ctx, args.Query, initialVectorLimit)
	candidateProductIDs := productCardIDs(products)
	structured := normalizeProductSearchArguments(args.Constraints, args.Negative)
	products, negativeDropped := filterProductsByStructuredNegative(products, structured.Negative)
	if len(products) == 0 && len(candidateProductIDs) > 0 {
		return toolObservation{
			Tool:                toolSearchProducts,
			OK:                  true,
			Message:             productSearchMessage(relevanceWeak, 0),
			Result:              map[string]any{"items": []map[string]any{}, "constraints": structured.Constraints, "negative": structured.Negative},
			CandidateProductIDs: candidateProductIDs,
			DroppedProductIDs:   negativeDropped,
			RelevanceStatus:     relevanceWeak,
			RelevanceReason:     "候选商品全部命中用户否定约束",
		}
	}
	rerank := r.rerankProductsForSearch(ctx, args.Query, products, structured, args.RerankModelEnabled)
	products = rerank.Products
	rerankPayload := map[string]any{
		"enabled":  rerank.Enabled,
		"provider": rerank.Provider,
		"model":    rerank.Model,
		"fallback": rerank.Fallback,
		"error":    rerank.Error,
		"scores":   traceRerankScores(rerank.Scores, 20),
	}
	if strings.TrimSpace(run.RunID) != "" && rerank.Enabled {
		r.trace(ctx, run, "tools.search_products.rerank", "rerank", "", "ok", 0, "", map[string]any{
			"query":                 args.Query,
			"provider":              rerank.Provider,
			"model":                 rerank.Model,
			"fallback":              rerank.Fallback,
			"error":                 rerank.Error,
			"candidate_product_ids": candidateProductIDs,
			"product_ids":           productCardIDs(products),
			"dropped_product_ids":   negativeDropped,
			"rerank_scores":         traceRerankScores(rerank.Scores, 20),
			"constraints":           structured.Constraints,
			"negative":              structured.Negative,
		})
	}
	relevanceStructured := structured
	relevanceStructured.Negative = productSearchArguments{}
	relevance := r.classifyProductSearchRelevanceWithRun(ctx, run, args.Query, products, relevanceStructured, args.LLMFilterEnabled)
	relevance.CandidateProductIDs = candidateProductIDs
	relevance.DroppedProductIDs = appendUnique(relevance.DroppedProductIDs, negativeDropped...)
	products = relevance.AllowedProducts
	if len(products) > limit {
		products = products[:limit]
		relevance.AllowedProductIDs = productCardIDs(products)
	}
	items := make([]map[string]any, 0, len(products))
	productIDs := make([]string, 0, len(products))
	for _, product := range products {
		productIDs = append(productIDs, product.ProductID)
		items = append(items, map[string]any{
			"product_id":       product.ProductID,
			"sku_id":           product.SkuID,
			"name":             product.Name,
			"brand":            product.Brand,
			"price":            product.Price,
			"image_url":        product.ImageURL,
			"stock_status":     product.StockStatus,
			"merchant_name":    product.MerchantName,
			"selling_points":   product.SellingPoints,
			"risk_notes":       product.RiskNotes,
			"recommend_reason": truncateRunes(product.RecommendReason, 180),
		})
	}
	if len(productIDs) == 0 && relevance.Status == relevanceOK {
		relevance.Status = relevanceNoMatch
		relevance.Reason = "检索成功但没有达到可挂品相关性要求的商品"
	}
	return toolObservation{
		Tool:                toolSearchProducts,
		OK:                  true,
		Message:             productSearchMessage(relevance.Status, len(items)),
		Result:              map[string]any{"items": items, "constraints": structured.Constraints, "negative": structured.Negative, "rerank": rerankPayload},
		ProductIDs:          productIDs,
		CandidateProductIDs: relevance.CandidateProductIDs,
		DroppedProductIDs:   relevance.DroppedProductIDs,
		RelevanceStatus:     relevance.Status,
		RelevanceReason:     relevance.Reason,
	}
}

func (r *Runtime) toolSearchKnowledge(ctx context.Context, raw json.RawMessage) toolObservation {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolObservation{Tool: toolSearchKnowledge, Message: "query 不能为空"}
	}
	limit := clampLimit(args.Limit, 3, 8)
	plan := r.retrievalPlan(ctx, args.Query)
	plan.Rerank.TopK = limit
	citations := r.store.SearchKnowledgeByPlan(ctx, plan)
	if len(citations) > limit {
		citations = citations[:limit]
	}
	items := make([]map[string]any, 0, len(citations))
	chunkIDs := make([]string, 0, len(citations))
	for _, citation := range citations {
		chunkIDs = append(chunkIDs, citation.ChunkID)
		items = append(items, map[string]any{
			"chunk_id": citation.ChunkID,
			"title":    citation.Title,
			"snippet":  truncateRunes(citation.Snippet, 220),
			"source":   citation.Source,
		})
	}
	return toolObservation{
		Tool:     toolSearchKnowledge,
		OK:       true,
		Message:  fmt.Sprintf("检索到 %d 个资料片段", len(items)),
		Result:   map[string]any{"items": items},
		ChunkIDs: chunkIDs,
	}
}

func (r *Runtime) toolGetCart(ctx context.Context, accountID string) toolObservation {
	cart := r.store.GetCart(ctx, accountID)
	return toolObservation{
		Tool:    toolGetCart,
		OK:      true,
		Message: fmt.Sprintf("购物车当前有 %d 件商品，选中 %d 件", len(cart.Items), cart.Summary.SelectedCount),
		Result:  map[string]any{"cart": compactCart(cart)},
		Cart:    &cart,
	}
}

func (r *Runtime) toolAddCartItem(ctx context.Context, accountID string, raw json.RawMessage, allowedProductIDs map[string]bool) toolObservation {
	var args struct {
		ProductID string `json:"product_id"`
		SkuID     string `json:"sku_id"`
		Quantity  int    `json:"quantity"`
	}
	_ = json.Unmarshal(raw, &args)
	args.ProductID = strings.TrimSpace(args.ProductID)
	args.SkuID = strings.TrimSpace(args.SkuID)
	if args.ProductID == "" {
		return toolObservation{Tool: toolAddCartItem, Message: "product_id 不能为空；如不确定商品，请先调用 search_products"}
	}
	if !allowedProductIDs[args.ProductID] {
		return toolObservation{Tool: toolAddCartItem, Message: "当前轮没有明确可加购的商品 ID；禁止根据购物车内容、列表位置或猜测的 product_id 加购，请先让用户明确要加购哪个商品"}
	}
	if args.Quantity <= 0 {
		args.Quantity = 1
	}
	cart, ok := r.store.AddCartItem(ctx, accountID, args.ProductID, args.SkuID, args.Quantity)
	if !ok {
		return toolObservation{Tool: toolAddCartItem, Message: "加购失败，商品可能不存在或不可售"}
	}
	return toolObservation{
		Tool:    toolAddCartItem,
		OK:      true,
		Message: "已加入购物车",
		Result:  map[string]any{"cart": compactCart(cart)},
		Cart:    &cart,
	}
}

func (r *Runtime) toolUpdateCartItem(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		CartItemID string `json:"cart_item_id"`
		Quantity   *int   `json:"quantity"`
		Selected   *bool  `json:"selected"`
	}
	_ = json.Unmarshal(raw, &args)
	args.CartItemID = strings.TrimSpace(args.CartItemID)
	if args.CartItemID == "" {
		return toolObservation{Tool: toolUpdateCartItem, Message: "cart_item_id 不能为空；如用户按序号描述，请先调用 get_cart"}
	}
	cart, ok := r.store.UpdateCartItem(ctx, accountID, args.CartItemID, args.Quantity, args.Selected)
	if !ok {
		return toolObservation{Tool: toolUpdateCartItem, Message: "修改购物车失败，请重新获取购物车后再试"}
	}
	return toolObservation{
		Tool:    toolUpdateCartItem,
		OK:      true,
		Message: "购物车已更新",
		Result:  map[string]any{"cart": compactCart(cart)},
		Cart:    &cart,
	}
}

func (r *Runtime) toolDeleteCartItem(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		CartItemID string `json:"cart_item_id"`
	}
	_ = json.Unmarshal(raw, &args)
	args.CartItemID = strings.TrimSpace(args.CartItemID)
	if args.CartItemID == "" {
		return toolObservation{Tool: toolDeleteCartItem, Message: "cart_item_id 不能为空；如用户按序号描述，请先调用 get_cart"}
	}
	cart, ok := r.store.DeleteCartItem(ctx, accountID, args.CartItemID)
	if !ok {
		return toolObservation{Tool: toolDeleteCartItem, Message: "删除购物车商品失败，请重新获取购物车后再试"}
	}
	return toolObservation{
		Tool:    toolDeleteCartItem,
		OK:      true,
		Message: "商品已从购物车删除",
		Result:  map[string]any{"cart": compactCart(cart)},
		Cart:    &cart,
	}
}

func (r *Runtime) toolCheckout(ctx context.Context, accountID string) toolObservation {
	cart := r.store.GetCart(ctx, accountID)
	if len(cart.Items) == 0 || cart.Summary.SelectedCount == 0 {
		return toolObservation{Tool: toolCheckout, Message: "购物车没有选中的商品，不能提交订单", Cart: &cart, Result: map[string]any{"cart": compactCart(cart)}}
	}
	orders, ok := r.store.CreateOrderFromCart(ctx, accountID)
	if !ok {
		return toolObservation{Tool: toolCheckout, Message: "提交订单失败，请确认购物车商品仍然有效"}
	}
	return toolObservation{
		Tool:    toolCheckout,
		OK:      true,
		Message: fmt.Sprintf("已创建 %d 个待支付订单", len(orders)),
		Result:  map[string]any{"orders": compactOrders(orders)},
		Orders:  orders,
	}
}

func (r *Runtime) toolListOrders(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	status := strings.TrimSpace(args.Status)
	limit := clampLimit(args.Limit, 5, 10)
	orders := filterOrdersByStatus(r.store.ListUserOrders(ctx, accountID), status)
	if len(orders) > limit {
		orders = orders[:limit]
	}
	return toolObservation{
		Tool:    toolListOrders,
		OK:      true,
		Message: fmt.Sprintf("查询到 %d 个订单", len(orders)),
		Result:  map[string]any{"items": compactOrders(orders), "status": status},
		Orders:  orders,
	}
}

func (r *Runtime) toolGetOrder(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		OrderID string `json:"order_id"`
	}
	_ = json.Unmarshal(raw, &args)
	args.OrderID = strings.TrimSpace(args.OrderID)
	if args.OrderID == "" {
		return toolObservation{Tool: toolGetOrder, Message: "order_id 不能为空；如果用户没有提供订单号，请先调用 list_orders"}
	}
	order, ok := r.store.GetOrder(ctx, accountID, args.OrderID)
	if !ok {
		return toolObservation{Tool: toolGetOrder, Message: "订单不存在或不属于当前用户"}
	}
	return toolObservation{
		Tool:    toolGetOrder,
		OK:      true,
		Message: "已查询到订单",
		Result:  map[string]any{"order": compactOrder(order)},
		Orders:  []domain.Order{order},
	}
}

func (r *Runtime) toolPayOrder(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		OrderID string `json:"order_id"`
		Method  string `json:"method"`
	}
	_ = json.Unmarshal(raw, &args)
	args.OrderID = strings.TrimSpace(args.OrderID)
	args.Method = strings.TrimSpace(args.Method)
	if args.OrderID == "" {
		return toolObservation{Tool: toolPayOrder, Message: "order_id 不能为空；支付前必须明确订单"}
	}
	order, payment, ok := r.store.PayOrder(ctx, accountID, args.OrderID, args.Method)
	if !ok {
		return toolObservation{Tool: toolPayOrder, Message: "订单不可支付，可能已支付、取消或超时关闭"}
	}
	return toolObservation{
		Tool:    toolPayOrder,
		OK:      true,
		Message: "支付成功，订单已进入待发货",
		Result:  map[string]any{"order": compactOrder(order), "payment": compactPayment(payment)},
		Orders:  []domain.Order{order},
	}
}

func (r *Runtime) toolCancelOrder(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		OrderID string `json:"order_id"`
		Reason  string `json:"reason"`
	}
	_ = json.Unmarshal(raw, &args)
	args.OrderID = strings.TrimSpace(args.OrderID)
	if args.OrderID == "" {
		return toolObservation{Tool: toolCancelOrder, Message: "order_id 不能为空；取消订单前必须明确订单"}
	}
	order, ok := r.store.CancelOrder(ctx, accountID, args.OrderID, args.Reason)
	if !ok {
		return toolObservation{Tool: toolCancelOrder, Message: "订单不可取消，只有待支付订单可以取消"}
	}
	return toolObservation{
		Tool:    toolCancelOrder,
		OK:      true,
		Message: "订单已取消",
		Result:  map[string]any{"order": compactOrder(order)},
		Orders:  []domain.Order{order},
	}
}

func (r *Runtime) toolConfirmReceipt(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		OrderID string `json:"order_id"`
	}
	_ = json.Unmarshal(raw, &args)
	args.OrderID = strings.TrimSpace(args.OrderID)
	if args.OrderID == "" {
		return toolObservation{Tool: toolConfirmReceipt, Message: "order_id 不能为空；确认收货前必须明确订单"}
	}
	order, ok := r.store.ConfirmReceipt(ctx, accountID, args.OrderID)
	if !ok {
		return toolObservation{Tool: toolConfirmReceipt, Message: "订单不可确认收货，只有已发货订单可以确认"}
	}
	return toolObservation{
		Tool:    toolConfirmReceipt,
		OK:      true,
		Message: "已确认收货，订单已完成",
		Result:  map[string]any{"order": compactOrder(order)},
		Orders:  []domain.Order{order},
	}
}

func (r *Runtime) toolPreviewDiscount(ctx context.Context, accountID string) toolObservation {
	preview := r.store.PreviewCartDiscount(ctx, accountID)
	return toolObservation{
		Tool:    toolPreviewDiscount,
		OK:      true,
		Message: fmt.Sprintf("当前可优惠 %s，应付 %s", preview.DiscountAmount, preview.PayAmount),
		Result:  map[string]any{"discount": preview},
	}
}

func (r *Runtime) toolListCoupons(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	limit := clampLimit(args.Limit, 5, 20)
	coupons := r.store.ListCoupons(ctx, accountID)
	if len(coupons) > limit {
		coupons = coupons[:limit]
	}
	return toolObservation{
		Tool:    toolListCoupons,
		OK:      true,
		Message: fmt.Sprintf("查询到 %d 张可领取优惠券", len(coupons)),
		Result:  map[string]any{"items": compactCoupons(coupons)},
	}
}

func (r *Runtime) toolListUserCoupons(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	status := strings.TrimSpace(args.Status)
	limit := clampLimit(args.Limit, 5, 20)
	coupons := filterUserCouponsByStatus(r.store.ListUserCoupons(ctx, accountID), status)
	if len(coupons) > limit {
		coupons = coupons[:limit]
	}
	return toolObservation{
		Tool:    toolListUserCoupons,
		OK:      true,
		Message: fmt.Sprintf("查询到 %d 张我的优惠券", len(coupons)),
		Result:  map[string]any{"items": compactUserCoupons(coupons), "status": status},
	}
}

func (r *Runtime) toolClaimCoupon(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		CouponID string `json:"coupon_id"`
	}
	_ = json.Unmarshal(raw, &args)
	args.CouponID = strings.TrimSpace(args.CouponID)
	if args.CouponID == "" {
		return toolObservation{Tool: toolClaimCoupon, Message: "coupon_id 不能为空；领券前必须明确优惠券"}
	}
	coupon, ok := r.store.ClaimCoupon(ctx, accountID, args.CouponID)
	if !ok {
		return toolObservation{Tool: toolClaimCoupon, Message: "领券失败，可能已领取、已过期或库存不足"}
	}
	return toolObservation{
		Tool:    toolClaimCoupon,
		OK:      true,
		Message: "优惠券领取成功",
		Result:  map[string]any{"coupon": compactUserCoupon(coupon)},
	}
}

func (r *Runtime) toolListPromotions(ctx context.Context, raw json.RawMessage) toolObservation {
	var args struct {
		MerchantID string `json:"merchant_id"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	limit := clampLimit(args.Limit, 5, 20)
	promotions := r.store.ListPromotions(ctx, strings.TrimSpace(args.MerchantID))
	if len(promotions) > limit {
		promotions = promotions[:limit]
	}
	return toolObservation{
		Tool:    toolListPromotions,
		OK:      true,
		Message: fmt.Sprintf("查询到 %d 个促销活动", len(promotions)),
		Result:  map[string]any{"items": compactPromotions(promotions)},
	}
}

func (r *Runtime) toolListProductReviews(ctx context.Context, raw json.RawMessage) toolObservation {
	var args struct {
		ProductID string `json:"product_id"`
		Limit     int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	args.ProductID = strings.TrimSpace(args.ProductID)
	if args.ProductID == "" {
		return toolObservation{Tool: toolListReviews, Message: "product_id 不能为空；查询评价前必须明确商品"}
	}
	limit := clampLimit(args.Limit, 5, 20)
	reviews := r.store.ListProductReviews(ctx, args.ProductID)
	if len(reviews) > limit {
		reviews = reviews[:limit]
	}
	return toolObservation{
		Tool:    toolListReviews,
		OK:      true,
		Message: fmt.Sprintf("查询到 %d 条可见评价", len(reviews)),
		Result:  map[string]any{"items": compactReviews(reviews), "summary": reviewSummary(reviews)},
	}
}

func (r *Runtime) toolCreateProductReview(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
	var args struct {
		OrderID     string   `json:"order_id"`
		OrderItemID string   `json:"order_item_id"`
		Rating      int      `json:"rating"`
		Content     string   `json:"content"`
		Tags        []string `json:"tags"`
	}
	_ = json.Unmarshal(raw, &args)
	args.OrderID = strings.TrimSpace(args.OrderID)
	args.OrderItemID = strings.TrimSpace(args.OrderItemID)
	if args.OrderID == "" || args.OrderItemID == "" {
		return toolObservation{Tool: toolCreateReview, Message: "order_id 和 order_item_id 不能为空；评价前必须明确已完成订单项"}
	}
	review, ok := r.store.CreateProductReview(ctx, accountID, args.OrderID, args.OrderItemID, domain.ProductReviewInput{
		Rating:  args.Rating,
		Content: args.Content,
		Tags:    args.Tags,
	})
	if !ok {
		return toolObservation{Tool: toolCreateReview, Message: "评价失败，只有已完成订单项可评价且不可重复评价"}
	}
	return toolObservation{
		Tool:    toolCreateReview,
		OK:      true,
		Message: "评价已发布",
		Result:  map[string]any{"review": compactReview(review)},
	}
}

func parseReactAction(content string) (reactAction, error) {
	var action reactAction
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &action); err != nil {
		return action, err
	}
	action.Type = strings.TrimSpace(action.Type)
	action.Tool = strings.TrimSpace(action.Tool)
	action.Skill = strings.TrimSpace(action.Skill)
	switch action.Type {
	case "tool_call", "skill_call", "final":
		return action, nil
	default:
		return action, errors.New("unknown action type: " + action.Type)
	}
}

func observationForModel(observation toolObservation) string {
	payload := map[string]any{
		"tool":                  observation.Tool,
		"ok":                    observation.OK,
		"message":               observation.Message,
		"result":                observation.Result,
		"duration_ms":           observation.DurationMS,
		"relevance_status":      observation.RelevanceStatus,
		"relevance_reason":      observation.RelevanceReason,
		"product_ids":           observation.ProductIDs,
		"candidate_product_ids": observation.CandidateProductIDs,
		"dropped_product_ids":   observation.DroppedProductIDs,
		"blocks":                observation.Blocks,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"tool":%q,"ok":false,"message":"observation marshal failed"}`, observation.Tool)
	}
	return string(bytes)
}

func compactCart(cart domain.Cart) map[string]any {
	items := make([]map[string]any, 0, len(cart.Items))
	for i, item := range cart.Items {
		items = append(items, map[string]any{
			"index":        i + 1,
			"cart_item_id": item.CartItemID,
			"product_id":   item.ProductID,
			"sku_id":       item.SkuID,
			"name":         item.Name,
			"price":        item.Price,
			"quantity":     item.Quantity,
			"selected":     item.Selected,
			"merchant_id":  item.MerchantID,
		})
	}
	return map[string]any{
		"items":   items,
		"summary": cart.Summary,
	}
}

func compactOrders(orders []domain.Order) []map[string]any {
	items := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		items = append(items, compactOrder(order))
	}
	return items
}

func compactOrder(order domain.Order) map[string]any {
	return map[string]any{
		"order_id":            order.OrderID,
		"order_no":            order.OrderNo,
		"merchant_id":         order.MerchantID,
		"merchant_name":       order.MerchantName,
		"status":              order.Status,
		"total_amount":        order.TotalAmount,
		"discount_amount":     order.DiscountAmount,
		"pay_amount":          order.PayAmount,
		"payment_deadline_at": order.PaymentDeadlineAt,
		"paid_at":             order.PaidAt,
		"closed_at":           order.ClosedAt,
		"completed_at":        order.CompletedAt,
		"cancel_reason":       order.CancelReason,
		"created_at":          order.CreatedAt,
		"item_count":          len(order.Items),
		"items":               compactOrderItems(order.Items),
	}
}

func compactOrderItems(items []domain.OrderItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"order_item_id": item.OrderItemID,
			"product_id":    item.ProductID,
			"sku_id":        item.SkuID,
			"name":          item.Name,
			"price":         item.Price,
			"quantity":      item.Quantity,
			"merchant_id":   item.MerchantID,
		})
	}
	return out
}

func compactPayment(payment domain.Payment) map[string]any {
	return map[string]any{
		"payment_id":     payment.PaymentID,
		"order_id":       payment.OrderID,
		"amount":         payment.Amount,
		"status":         payment.Status,
		"method":         payment.Method,
		"transaction_no": payment.TransactionNo,
		"paid_at":        payment.PaidAt,
	}
}

func compactCoupons(coupons []domain.Coupon) []map[string]any {
	out := make([]map[string]any, 0, len(coupons))
	for _, coupon := range coupons {
		out = append(out, compactCoupon(coupon))
	}
	return out
}

func compactCoupon(coupon domain.Coupon) map[string]any {
	return map[string]any{
		"coupon_id":        coupon.CouponID,
		"name":             coupon.Name,
		"scope":            coupon.Scope,
		"merchant_id":      coupon.MerchantID,
		"type":             coupon.Type,
		"threshold_amount": coupon.ThresholdAmount,
		"discount_amount":  coupon.DiscountAmount,
		"claimed_count":    coupon.ClaimedCount,
		"total_count":      coupon.TotalCount,
		"per_user_limit":   coupon.PerUserLimit,
		"start_at":         coupon.StartAt,
		"end_at":           coupon.EndAt,
		"status":           coupon.Status,
	}
}

func compactUserCoupons(coupons []domain.UserCoupon) []map[string]any {
	out := make([]map[string]any, 0, len(coupons))
	for _, coupon := range coupons {
		out = append(out, compactUserCoupon(coupon))
	}
	return out
}

func compactUserCoupon(coupon domain.UserCoupon) map[string]any {
	return map[string]any{
		"user_coupon_id": coupon.UserCouponID,
		"coupon_id":      coupon.CouponID,
		"status":         coupon.Status,
		"order_id":       coupon.OrderID,
		"claimed_at":     coupon.ClaimedAt,
		"used_at":        coupon.UsedAt,
		"coupon":         compactCoupon(coupon.Coupon),
	}
}

func compactPromotions(promotions []domain.PromotionRule) []map[string]any {
	out := make([]map[string]any, 0, len(promotions))
	for _, promotion := range promotions {
		out = append(out, map[string]any{
			"promotion_id":     promotion.PromotionID,
			"name":             promotion.Name,
			"scope":            promotion.Scope,
			"merchant_id":      promotion.MerchantID,
			"product_id":       promotion.ProductID,
			"category_id":      promotion.CategoryID,
			"type":             promotion.Type,
			"threshold_amount": promotion.ThresholdAmount,
			"discount_amount":  promotion.DiscountAmount,
			"discount_rate":    promotion.DiscountRate,
			"stackable":        promotion.Stackable,
			"start_at":         promotion.StartAt,
			"end_at":           promotion.EndAt,
			"status":           promotion.Status,
		})
	}
	return out
}

func compactReviews(reviews []domain.ProductReview) []map[string]any {
	out := make([]map[string]any, 0, len(reviews))
	for _, review := range reviews {
		out = append(out, compactReview(review))
	}
	return out
}

func compactReview(review domain.ProductReview) map[string]any {
	return map[string]any{
		"review_id":           review.ReviewID,
		"order_id":            review.OrderID,
		"order_item_id":       review.OrderItemID,
		"product_id":          review.ProductID,
		"sku_id":              review.SkuID,
		"username":            review.Username,
		"rating":              review.Rating,
		"content":             truncateRunes(review.Content, 260),
		"tags":                review.Tags,
		"status":              review.Status,
		"merchant_reply":      truncateRunes(review.MerchantReply, 220),
		"merchant_replied_at": review.MerchantRepliedAt,
		"created_at":          review.CreatedAt,
	}
}

func reviewSummary(reviews []domain.ProductReview) map[string]any {
	if len(reviews) == 0 {
		return map[string]any{"count": 0, "average_rating": 0}
	}
	total := 0
	tagCounts := map[string]int{}
	for _, review := range reviews {
		total += review.Rating
		for _, tag := range review.Tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagCounts[tag]++
			}
		}
	}
	tags := make([]string, 0, len(tagCounts))
	for tag := range tagCounts {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool {
		if tagCounts[tags[i]] == tagCounts[tags[j]] {
			return tags[i] < tags[j]
		}
		return tagCounts[tags[i]] > tagCounts[tags[j]]
	})
	if len(tags) > 5 {
		tags = tags[:5]
	}
	return map[string]any{
		"count":          len(reviews),
		"average_rating": float64(total) / float64(len(reviews)),
		"top_tags":       tags,
	}
}

func filterOrdersByStatus(orders []domain.Order, status string) []domain.Order {
	if status == "" {
		return orders
	}
	out := make([]domain.Order, 0, len(orders))
	for _, order := range orders {
		if order.Status == status {
			out = append(out, order)
		}
	}
	return out
}

func filterUserCouponsByStatus(coupons []domain.UserCoupon, status string) []domain.UserCoupon {
	if status == "" {
		return coupons
	}
	out := make([]domain.UserCoupon, 0, len(coupons))
	for _, coupon := range coupons {
		if coupon.Status == status {
			out = append(out, coupon)
		}
	}
	return out
}

func clampLimit(value int, fallback int, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		return max
	}
	return value
}

func truncateRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "..."
}

type productRelevanceResult struct {
	Status              string
	Reason              string
	AllowedProducts     []domain.ProductCard
	AllowedProductIDs   []string
	CandidateProductIDs []string
	DroppedProductIDs   []string
}

func (r *Runtime) classifyProductSearchRelevance(ctx context.Context, query string, products []domain.ProductCard) productRelevanceResult {
	return r.classifyProductSearchRelevanceWithRun(ctx, domain.AgentRun{}, query, products, productSearchStructuredArguments{}, nil)
}

func (r *Runtime) classifyProductSearchRelevanceWithRun(ctx context.Context, run domain.AgentRun, query string, products []domain.ProductCard, structured productSearchStructuredArguments, llmEnabledOverride *bool) productRelevanceResult {
	result := productRelevanceResult{
		Status:              relevanceNoMatch,
		Reason:              "没有召回到商品",
		CandidateProductIDs: productCardIDs(products),
	}
	if len(products) == 0 {
		return result
	}
	products, negativeDropped := filterProductsByStructuredNegative(products, structured.Negative)
	if len(products) == 0 {
		return productRelevanceResult{
			Status:              relevanceWeak,
			Reason:              "候选商品全部命中用户否定约束",
			CandidateProductIDs: result.CandidateProductIDs,
			DroppedProductIDs:   negativeDropped,
		}
	}

	values := r.configs.GetMap(ctx)
	llmFilterEnabled := boolFromMap(values, "retrieval.product.llm_filter.enabled", true)
	if llmEnabledOverride != nil {
		llmFilterEnabled = *llmEnabledOverride
	}
	if r.llm != nil && r.llm.Enabled() && llmFilterEnabled {
		if llmResult, ok := r.classifyProductSearchRelevanceByLLM(ctx, run, query, products, structured); ok {
			llmResult.CandidateProductIDs = result.CandidateProductIDs
			llmResult.DroppedProductIDs = appendUnique(llmResult.DroppedProductIDs, negativeDropped...)
			return llmResult
		}
	}
	guardEnabled := boolFromMap(values, "retrieval.product.lexical_guard.enabled", false)
	minEvidence := intFromMap(values, "retrieval.product.lexical_guard.min_evidence_count", 1)
	minRatio := floatFromMap(values, "retrieval.product.lexical_guard.min_match_ratio", 0.35)
	terms := productRelevanceTerms(query, values["retrieval.rerank.generic_terms"])
	if len(terms) == 0 || !guardEnabled {
		allowed := limitProductCards(products, intFromMap(values, "retrieval.product.ok.max_results", len(products)))
		return productRelevanceResult{
			Status:              relevanceOK,
			Reason:              "未启用词面保护或 query 缺少有效词面证据，保留原始检索结果",
			AllowedProducts:     allowed,
			AllowedProductIDs:   productCardIDs(allowed),
			CandidateProductIDs: result.CandidateProductIDs,
			DroppedProductIDs:   negativeDropped,
		}
	}

	type scoredProduct struct {
		product domain.ProductCard
		score   int
		ratio   float64
	}
	scored := make([]scoredProduct, 0, len(products))
	for _, product := range products {
		score := productEvidenceCount(productSearchText(product), terms)
		ratio := 0.0
		if len(terms) > 0 {
			ratio = float64(score) / float64(len(terms))
		}
		scored = append(scored, scoredProduct{product: product, score: score, ratio: ratio})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			if scored[i].ratio == scored[j].ratio {
				return scored[i].product.ProductID < scored[j].product.ProductID
			}
			return scored[i].ratio > scored[j].ratio
		}
		return scored[i].score > scored[j].score
	})

	allowed := make([]domain.ProductCard, 0, len(scored))
	dropped := make([]string, 0, len(scored))
	for _, item := range scored {
		allowedByEvidence := item.score >= minEvidence && (item.ratio >= minRatio || (len(terms) <= 3 && item.score >= 1) || item.score >= 2)
		if allowedByEvidence {
			allowed = append(allowed, item.product)
			continue
		}
		dropped = append(dropped, item.product.ProductID)
	}

	if len(allowed) == 0 {
		return productRelevanceResult{
			Status:              relevanceWeak,
			Reason:              fmt.Sprintf("召回到 %d 个候选，但没有商品达到词面证据要求；有效词：%s", len(products), strings.Join(terms, ",")),
			CandidateProductIDs: result.CandidateProductIDs,
			DroppedProductIDs:   appendUnique(dropped, negativeDropped...),
		}
	}
	allowed = limitProductCards(allowed, intFromMap(values, "retrieval.product.ok.max_results", len(allowed)))
	status := relevanceOK
	reason := fmt.Sprintf("有 %d 个商品达到词面证据要求", len(allowed))
	if len(dropped) > 0 {
		reason += fmt.Sprintf("，剔除 %d 个弱相关候选", len(dropped))
	}
	return productRelevanceResult{
		Status:              status,
		Reason:              reason,
		AllowedProducts:     allowed,
		AllowedProductIDs:   productCardIDs(allowed),
		CandidateProductIDs: result.CandidateProductIDs,
		DroppedProductIDs:   appendUnique(dropped, negativeDropped...),
	}
}

type productRelevanceLLMOutput struct {
	RelevantProductIDs []string `json:"relevant_product_ids"`
	Reason             string   `json:"reason"`
}

func (r *Runtime) classifyProductSearchRelevanceByLLM(ctx context.Context, run domain.AgentRun, query string, products []domain.ProductCard, structured productSearchStructuredArguments) (productRelevanceResult, bool) {
	limit := r.intConfig(ctx, "retrieval.product.llm_filter.max_candidates", 10)
	if limit <= 0 {
		limit = 10
	}
	candidates := products
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	messages := []ChatMessage{
		{Role: "system", Content: productRelevanceLLMSystemPrompt()},
		{Role: "user", Content: productRelevanceLLMUserPrompt(query, candidates, structured)},
	}
	startedAt := time.Now()
	temperature := 0.0
	model := r.modelForRole(ctx, modelRoleProductFilter, r.llm.SmallModel())
	content, err := r.llm.Complete(ctx, model, messages, temperature)
	if strings.TrimSpace(run.RunID) != "" {
		extra := map[string]any{"candidate_count": len(candidates)}
		if err == nil {
			extra["raw_length"] = len([]rune(content))
			extra["raw_output"] = content
		}
		r.traceLLM(ctx, run, "tools.search_products.filter", model, startedAt, err, llmPromptMetadata(messages, temperature, extra))
	}
	if err != nil {
		r.logger.Warn("product relevance llm fallback", "run_id", run.RunID, "error", err)
		return productRelevanceResult{}, false
	}
	var parsed productRelevanceLLMOutput
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &parsed); err != nil {
		r.logger.Warn("product relevance llm json fallback", "run_id", run.RunID, "error", err, "content", content)
		return productRelevanceResult{}, false
	}
	candidateByID := make(map[string]domain.ProductCard, len(candidates))
	for _, product := range candidates {
		candidateByID[product.ProductID] = product
	}
	seen := map[string]bool{}
	allowed := make([]domain.ProductCard, 0, len(candidates))
	for _, id := range parsed.RelevantProductIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		product, ok := candidateByID[id]
		if !ok {
			continue
		}
		seen[id] = true
		allowed = append(allowed, product)
	}
	dropped := make([]string, 0, len(candidates)-len(allowed))
	for _, product := range candidates {
		if !seen[product.ProductID] {
			dropped = append(dropped, product.ProductID)
		}
	}
	reason := strings.TrimSpace(parsed.Reason)
	if reason == "" {
		reason = "小模型完成商品相关性过滤"
	}
	if len(allowed) == 0 {
		return productRelevanceResult{
			Status:              relevanceWeak,
			Reason:              reason,
			CandidateProductIDs: productCardIDs(candidates),
			DroppedProductIDs:   dropped,
		}, true
	}
	return productRelevanceResult{
		Status:              relevanceOK,
		Reason:              reason,
		AllowedProducts:     allowed,
		AllowedProductIDs:   productCardIDs(allowed),
		CandidateProductIDs: productCardIDs(candidates),
		DroppedProductIDs:   dropped,
	}, true
}

func productRelevanceLLMSystemPrompt() string {
	return `你是电商商品检索相关性过滤器。你会收到用户检索词和候选商品，只判断候选商品是否可以作为当前检索词的直接推荐结果。

判断规则：
1. 只保留与用户品类、品牌、型号、场景或属性直接相关的商品。
2. 宽泛品类词可以匹配其合理子类，例如“化妆品”可以保留护肤、彩妆、香水等；“手机数码”可以保留手机、耳机、充电配件等。
3. 如果用户同时给出硬约束，例如品牌、预算、材质、适用对象，候选商品必须不明显违背这些约束。
4. negative 是用户明确不要的品牌、词或类目，是硬约束；命中 negative 的商品必须剔除，不能出现在 relevant_product_ids。
5. 不要因为商品标题没有逐字出现所有检索词就剔除，只要类目/标签/卖点语义匹配即可保留。
6. 输出只能使用候选里的 product_id，不能编造。

只输出 JSON：
{"relevant_product_ids":["商品ID"],"reason":"中文，80字以内，说明保留/剔除依据"}`
}

func productRelevanceLLMUserPrompt(query string, products []domain.ProductCard, structured productSearchStructuredArguments) string {
	var b strings.Builder
	b.WriteString("用户检索词：")
	b.WriteString(strings.TrimSpace(query))
	if !emptyProductSearchArguments(structured.Constraints) || !emptyProductSearchArguments(structured.Negative) {
		raw, _ := json.Marshal(structured)
		b.WriteString("\n结构化约束：")
		b.Write(raw)
	}
	b.WriteString("\n\n候选商品：\n")
	for i, product := range products {
		b.WriteString(fmt.Sprintf("%d. product_id=%s\n", i+1, product.ProductID))
		b.WriteString("名称：")
		b.WriteString(product.Name)
		b.WriteString("\n品牌：")
		b.WriteString(product.Brand)
		b.WriteString("\n类目ID：")
		b.WriteString(product.CategoryID)
		if len(product.Tags) > 0 {
			b.WriteString("\n标签：")
			b.WriteString(strings.Join(product.Tags, "、"))
		}
		if len(product.SellingPoints) > 0 {
			b.WriteString("\n卖点：")
			b.WriteString(strings.Join(product.SellingPoints, "、"))
		}
		if strings.TrimSpace(product.RecommendReason) != "" {
			b.WriteString("\n说明：")
			b.WriteString(truncateRunes(product.RecommendReason, 120))
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func normalizeProductSearchArguments(constraints productSearchArguments, negative productSearchArguments) productSearchStructuredArguments {
	return productSearchStructuredArguments{
		Constraints: normalizeProductSearchArgument(constraints),
		Negative:    normalizeProductSearchArgument(negative),
	}
}

func normalizeProductSearchArgument(input productSearchArguments) productSearchArguments {
	return productSearchArguments{
		Brands:     uniqueNonEmpty(input.Brands),
		Terms:      uniqueNonEmpty(input.Terms),
		Categories: uniqueNonEmpty(input.Categories),
	}
}

func emptyProductSearchArguments(input productSearchArguments) bool {
	return len(input.Brands) == 0 && len(input.Terms) == 0 && len(input.Categories) == 0
}

type productRerankResult struct {
	Products []domain.ProductCard
	Scores   []productRerankScore
	Enabled  bool
	Provider string
	Model    string
	Fallback bool
	Error    string
}

type productRerankScore struct {
	ProductID string
	Score     float64
	Reasons   []string
	Index     int
}

func (r *Runtime) rerankProductsForSearch(ctx context.Context, query string, products []domain.ProductCard, structured productSearchStructuredArguments, modelEnabledOverride *bool) productRerankResult {
	result := productRerankResult{Products: products}
	if len(products) == 0 {
		return result
	}
	if r.configs == nil {
		return result
	}
	values := r.configs.GetMap(ctx)
	if !boolFromMap(values, "retrieval.product.rerank.enabled", true) {
		return result
	}
	rerankErr := ""
	modelEnabled := boolFromMap(values, "retrieval.product.rerank.model_enabled", true)
	if modelEnabledOverride != nil {
		modelEnabled = *modelEnabledOverride
	}
	if modelEnabled && r.llm != nil {
		modelResult, err := r.rerankProductsByModel(ctx, query, products, structured, values)
		if err == nil {
			return modelResult
		}
		rerankErr = err.Error()
		result.Error = rerankErr
		if r.logger != nil {
			r.logger.Warn("product rerank model fallback", "error", err)
		}
		if !boolFromMap(values, "retrieval.product.rerank.fallback_rule_enabled", true) {
			return result
		}
	}
	result = r.rerankProductsByRule(query, products, structured, values)
	if rerankErr != "" {
		result.Error = rerankErr
		result.Fallback = true
	}
	return result
}

func (r *Runtime) rerankProductsByModel(ctx context.Context, query string, products []domain.ProductCard, structured productSearchStructuredArguments, values map[string]string) (productRerankResult, error) {
	maxCandidates := intFromMap(values, "retrieval.product.rerank.max_candidates", len(products))
	if maxCandidates <= 0 || maxCandidates > len(products) {
		maxCandidates = len(products)
	}
	candidates := products
	if len(candidates) > maxCandidates {
		candidates = candidates[:maxCandidates]
	}
	documents := make([]RerankDocument, 0, len(candidates))
	for _, product := range candidates {
		documents = append(documents, RerankDocument{ID: product.ProductID, Text: productRerankDocument(product)})
	}
	config := RerankConfig{
		BaseURL: strings.TrimSpace(values["retrieval.product.rerank.base_url"]),
		APIKey:  firstNonEmpty(values["retrieval.product.rerank.api_key"], values["ai.qwen.api_key"], values["ai.api_key"]),
		Model:   firstNonEmpty(values["retrieval.product.rerank.model"], "qwen3-vl-rerank"),
		TopN:    maxCandidates,
	}
	results, err := r.llm.Rerank(ctx, config, rerankQuery(query, structured), documents)
	if err != nil {
		return productRerankResult{}, err
	}
	byID := make(map[string]domain.ProductCard, len(candidates))
	for _, product := range candidates {
		byID[product.ProductID] = product
	}
	seen := map[string]bool{}
	out := make([]domain.ProductCard, 0, len(results))
	scores := make([]productRerankScore, 0, len(results))
	for index, item := range results {
		product, ok := byID[item.ID]
		if !ok || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, product)
		scores = append(scores, productRerankScore{
			ProductID: item.ID,
			Score:     item.Score,
			Reasons:   []string{"qwen3_vl_rerank"},
			Index:     index,
		})
	}
	for _, product := range candidates {
		if seen[product.ProductID] {
			continue
		}
		out = append(out, product)
		scores = append(scores, productRerankScore{
			ProductID: product.ProductID,
			Score:     0,
			Reasons:   []string{"rerank_not_returned"},
			Index:     len(scores),
		})
	}
	out = filterProductsByModelRerankScore(out, scores, values)
	return productRerankResult{
		Products: out,
		Scores:   scores,
		Enabled:  true,
		Provider: "dashscope",
		Model:    config.Model,
	}, nil
}

func filterProductsByModelRerankScore(products []domain.ProductCard, scores []productRerankScore, values map[string]string) []domain.ProductCard {
	if len(products) == 0 || len(scores) == 0 {
		return products
	}
	minScore := floatFromMap(values, "retrieval.product.rerank.model_min_score", 0.50)
	maxDelta := floatFromMap(values, "retrieval.product.rerank.model_max_score_delta", 0.12)
	topScore := scores[0].Score
	threshold := minScore
	if maxDelta > 0 && topScore-maxDelta > threshold {
		threshold = topScore - maxDelta
	}
	allowed := make(map[string]bool, len(scores))
	for _, score := range scores {
		if score.Score >= threshold {
			allowed[score.ProductID] = true
		}
	}
	filtered := make([]domain.ProductCard, 0, len(products))
	for _, product := range products {
		if allowed[product.ProductID] {
			filtered = append(filtered, product)
		}
	}
	return filtered
}

func (r *Runtime) rerankProductsByRule(query string, products []domain.ProductCard, structured productSearchStructuredArguments, values map[string]string) productRerankResult {
	result := productRerankResult{Products: products, Enabled: true, Provider: "rule"}
	type scoredProduct struct {
		product domain.ProductCard
		score   productRerankScore
		index   int
	}
	genericTerms := values["retrieval.rerank.generic_terms"]
	terms := productRelevanceTerms(query, genericTerms)
	weights := productRerankWeights{
		QueryTerm:   floatFromMap(values, "retrieval.product.rerank.weight.query_term", 6),
		Constraint:  floatFromMap(values, "retrieval.product.rerank.weight.constraint", 12),
		Brand:       floatFromMap(values, "retrieval.product.rerank.weight.brand", 18),
		Category:    floatFromMap(values, "retrieval.product.rerank.weight.category", 10),
		QueryPhrase: floatFromMap(values, "retrieval.product.rerank.weight.query_phrase", 2),
	}
	scored := make([]scoredProduct, 0, len(products))
	for index, product := range products {
		score := scoreProductForRerank(query, terms, product, structured.Constraints, weights, index)
		scored = append(scored, scoredProduct{product: product, score: score, index: index})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score.Score == scored[j].score.Score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score.Score > scored[j].score.Score
	})
	maxCandidates := intFromMap(values, "retrieval.product.rerank.max_candidates", len(scored))
	if maxCandidates <= 0 || maxCandidates > len(scored) {
		maxCandidates = len(scored)
	}
	out := make([]domain.ProductCard, 0, len(scored))
	scores := make([]productRerankScore, 0, len(scored))
	for i, item := range scored {
		item.score.Index = i
		scores = append(scores, item.score)
		if i >= maxCandidates {
			continue
		}
		out = append(out, item.product)
	}
	result.Products = out
	result.Scores = scores
	result.Enabled = true
	return result
}

type productRerankWeights struct {
	QueryTerm   float64
	Constraint  float64
	Brand       float64
	Category    float64
	QueryPhrase float64
}

func scoreProductForRerank(query string, queryTerms []string, product domain.ProductCard, constraints productSearchArguments, weights productRerankWeights, index int) productRerankScore {
	text := strings.ToLower(productSearchText(product))
	name := strings.ToLower(product.Name)
	brand := strings.ToLower(product.Brand)
	category := strings.ToLower(product.CategoryID)
	score := 0.0
	reasons := make([]string, 0, 6)
	for _, term := range queryTerms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || !strings.Contains(text, term) {
			continue
		}
		score += weights.QueryTerm
		reasons = append(reasons, "query_term:"+term)
		if strings.Contains(name, term) {
			score += weights.QueryTerm * 0.5
			reasons = append(reasons, "title:"+term)
		}
	}
	for _, term := range constraints.Brands {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || !strings.Contains(text, term) {
			continue
		}
		score += weights.Constraint
		reasons = append(reasons, "constraint:"+term)
		if strings.Contains(brand, term) {
			score += weights.Brand
			reasons = append(reasons, "brand:"+term)
		}
	}
	for _, term := range constraints.Categories {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || !strings.Contains(text, term) {
			continue
		}
		score += weights.Constraint + weights.Category
		reasons = append(reasons, "category:"+term)
		if strings.Contains(category, term) {
			score += weights.Category
		}
	}
	for _, term := range constraints.Terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || !strings.Contains(text, term) {
			continue
		}
		score += weights.Constraint
		reasons = append(reasons, "constraint:"+term)
	}
	query = strings.TrimSpace(strings.ToLower(query))
	if query != "" && strings.Contains(text, query) {
		score += weights.QueryPhrase
		reasons = append(reasons, "query_phrase")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "original_order")
	}
	return productRerankScore{ProductID: product.ProductID, Score: score, Reasons: reasons, Index: index}
}

func productRerankDocument(product domain.ProductCard) string {
	parts := []string{
		"product_id: " + product.ProductID,
		"name: " + product.Name,
		"brand: " + product.Brand,
		"category_id: " + product.CategoryID,
		"merchant: " + product.MerchantName,
		"stock_status: " + product.StockStatus,
	}
	if len(product.Tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(product.Tags, "、"))
	}
	if len(product.SellingPoints) > 0 {
		parts = append(parts, "selling_points: "+strings.Join(product.SellingPoints, "、"))
	}
	if strings.TrimSpace(product.RecommendReason) != "" {
		parts = append(parts, "recommend_reason: "+truncateRunes(product.RecommendReason, 240))
	}
	return strings.Join(parts, "\n")
}

func rerankQuery(query string, structured productSearchStructuredArguments) string {
	parts := []string{"用户需求：" + strings.TrimSpace(query)}
	if !emptyProductSearchArguments(structured.Constraints) {
		raw, _ := json.Marshal(structured.Constraints)
		parts = append(parts, "正向约束："+string(raw))
	}
	if !emptyProductSearchArguments(structured.Negative) {
		raw, _ := json.Marshal(structured.Negative)
		parts = append(parts, "负向约束："+string(raw))
	}
	return strings.Join(parts, "\n")
}

func filterProductsByStructuredNegative(products []domain.ProductCard, negative productSearchArguments) ([]domain.ProductCard, []string) {
	if len(products) == 0 || emptyProductSearchArguments(negative) {
		return products, nil
	}
	terms := append(append([]string{}, negative.Brands...), append(negative.Terms, negative.Categories...)...)
	allowed := make([]domain.ProductCard, 0, len(products))
	dropped := make([]string, 0)
	for _, product := range products {
		text := strings.ToLower(productSearchText(product))
		blocked := false
		for _, term := range terms {
			if strings.TrimSpace(term) != "" && strings.Contains(text, strings.ToLower(term)) {
				blocked = true
				break
			}
		}
		if blocked {
			dropped = append(dropped, product.ProductID)
			continue
		}
		allowed = append(allowed, product)
	}
	return allowed, dropped
}

func productRelevanceTerms(query string, genericConfig string) []string {
	lower := strings.ToLower(query)
	generic := map[string]bool{
		"推荐": true, "怎么选": true, "好用": true, "商品": true, "产品": true, "一下": true, "几个": true, "一款": true, "适合": true,
	}
	for _, term := range strings.FieldsFunc(genericConfig, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == ' '
	}) {
		term = strings.TrimSpace(strings.ToLower(term))
		if term != "" {
			generic[term] = true
		}
	}
	out := make([]string, 0)
	seen := map[string]bool{}
	fieldTerms := strings.Fields(lower)
	for _, term := range fieldTerms {
		fieldTerms = append(fieldTerms, stripProductGenericAffix(term, generic)...)
	}
	for _, term := range append(fieldTerms, rag.QueryTerms(query)...) {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || seen[term] || generic[term] {
			continue
		}
		if strings.HasSuffix(term, "推") && strings.Contains(lower, term+"荐") {
			continue
		}
		containsGeneric := false
		for item := range generic {
			if item != "" && strings.Contains(term, item) {
				containsGeneric = true
				break
			}
		}
		if containsGeneric && len([]rune(term)) > 2 {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return removeCoveredProductRelevanceTerms(out, generic)
}

func stripProductGenericAffix(term string, generic map[string]bool) []string {
	out := make([]string, 0, 2)
	for item := range generic {
		if item == "" {
			continue
		}
		if strings.HasSuffix(term, item) {
			value := strings.TrimSpace(strings.TrimSuffix(term, item))
			if value != "" {
				out = append(out, value)
			}
		}
		if strings.HasPrefix(term, item) {
			value := strings.TrimSpace(strings.TrimPrefix(term, item))
			if value != "" {
				out = append(out, value)
			}
		}
	}
	return out
}

func removeCoveredProductRelevanceTerms(terms []string, generic map[string]bool) []string {
	if len(terms) <= 1 {
		return terms
	}
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		covered := false
		termRunes := len([]rune(term))
		for _, other := range terms {
			if term == other {
				continue
			}
			if containsProductGenericTerm(other, generic) {
				continue
			}
			if termRunes < len([]rune(other)) && strings.Contains(other, term) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, term)
		}
	}
	return out
}

func containsProductGenericTerm(term string, generic map[string]bool) bool {
	for item := range generic {
		if item != "" && strings.Contains(term, item) {
			return true
		}
	}
	return false
}

func productEvidenceCount(text string, terms []string) int {
	text = strings.ToLower(text)
	count := 0
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			count++
		}
	}
	return count
}

func productSearchText(product domain.ProductCard) string {
	parts := []string{
		product.ProductID,
		product.Name,
		product.Brand,
		product.CategoryID,
		product.MerchantName,
		product.RecommendReason,
	}
	parts = append(parts, product.Tags...)
	parts = append(parts, product.SellingPoints...)
	parts = append(parts, product.RiskNotes...)
	return strings.ToLower(strings.Join(parts, "\n"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func productCardIDs(products []domain.ProductCard) []string {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		if strings.TrimSpace(product.ProductID) != "" {
			ids = append(ids, product.ProductID)
		}
	}
	return ids
}

func traceRerankScores(scores []productRerankScore, limit int) []map[string]any {
	if limit <= 0 || limit > len(scores) {
		limit = len(scores)
	}
	out := make([]map[string]any, 0, limit)
	for _, score := range scores[:limit] {
		out = append(out, map[string]any{
			"product_id": score.ProductID,
			"score":      score.Score,
			"reasons":    score.Reasons,
			"rank":       score.Index + 1,
		})
	}
	return out
}

func limitProductCards(products []domain.ProductCard, limit int) []domain.ProductCard {
	if limit <= 0 || limit >= len(products) {
		return products
	}
	return products[:limit]
}

func productSearchMessage(status string, count int) string {
	switch status {
	case relevanceOK:
		return fmt.Sprintf("检索到 %d 个相关商品", count)
	case relevanceWeak:
		return "检索到弱相关候选，但没有达到可推荐要求"
	case relevanceNoMatch:
		return "没有检索到匹配商品"
	default:
		return fmt.Sprintf("检索到 %d 个商品", count)
	}
}

func boolFromMap(values map[string]string, key string, fallback bool) bool {
	if value, ok := values[key]; ok {
		return parseBool(value, fallback)
	}
	return fallback
}

func intFromMap(values map[string]string, key string, fallback int) int {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatFromMap(values map[string]string, key string, fallback float64) float64 {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func rawArgs(values map[string]any) json.RawMessage {
	bytes, _ := json.Marshal(values)
	return bytes
}

func parseQuantityArgument(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case int:
		if typed > 0 {
			return typed
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
