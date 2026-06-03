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
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

const (
	toolSearchProducts  = "search_products"
	toolSearchKnowledge = "search_knowledge"
	toolGetCart         = "get_cart"
	toolAddCartItem     = "add_cart_item"
	toolUpdateCartItem  = "update_cart_item"
	toolDeleteCartItem  = "delete_cart_item"
	toolCheckout        = "checkout"
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

func productIDSetFromText(text string) map[string]bool {
	ids := make(map[string]bool)
	for _, id := range productIDPattern.FindAllString(text, -1) {
		ids[id] = true
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
	default:
		observation.Message = "未知工具：" + tool
	}
	observation.Tool = tool
	observation.DurationMS = time.Since(startedAt).Milliseconds()
	return observation
}

func (r *Runtime) toolSearchProducts(ctx context.Context, run domain.AgentRun, raw json.RawMessage) toolObservation {
	var args struct {
		Query       string                 `json:"query"`
		Limit       int                    `json:"limit"`
		Constraints productSearchArguments `json:"constraints"`
		Negative    productSearchArguments `json:"negative"`
	}
	_ = json.Unmarshal(raw, &args)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolObservation{Tool: toolSearchProducts, Message: "query 不能为空"}
	}
	limit := clampLimit(args.Limit, 5, 10)
	products := r.store.SearchProducts(ctx, args.Query)
	structured := normalizeProductSearchArguments(args.Constraints, args.Negative)
	products = rerankProductsWithStructuredConstraints(args.Query, products, structured)
	relevance := r.classifyProductSearchRelevanceWithRun(ctx, run, args.Query, products, structured)
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
		Result:              map[string]any{"items": items, "constraints": structured.Constraints, "negative": structured.Negative},
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
		items = append(items, map[string]any{
			"order_id":            order.OrderID,
			"order_no":            order.OrderNo,
			"merchant_id":         order.MerchantID,
			"merchant_name":       order.MerchantName,
			"status":              order.Status,
			"pay_amount":          order.PayAmount,
			"payment_deadline_at": order.PaymentDeadlineAt,
			"item_count":          len(order.Items),
		})
	}
	return items
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
	return r.classifyProductSearchRelevanceWithRun(ctx, domain.AgentRun{}, query, products, productSearchStructuredArguments{})
}

func (r *Runtime) classifyProductSearchRelevanceWithRun(ctx context.Context, run domain.AgentRun, query string, products []domain.ProductCard, structured productSearchStructuredArguments) productRelevanceResult {
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
	if r.llm != nil && r.llm.Enabled() && boolFromMap(values, "retrieval.product.llm_filter.enabled", true) {
		if llmResult, ok := r.classifyProductSearchRelevanceByLLM(ctx, run, query, products, structured); ok {
			llmResult.CandidateProductIDs = result.CandidateProductIDs
			llmResult.DroppedProductIDs = appendUnique(llmResult.DroppedProductIDs, negativeDropped...)
			return llmResult
		}
	}
	guardEnabled := boolFromMap(values, "retrieval.product.lexical_guard.enabled", true)
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
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if strings.TrimSpace(run.RunID) != "" {
		extra := map[string]any{"candidate_count": len(candidates)}
		if err == nil {
			extra["raw_length"] = len([]rune(content))
			extra["raw_output"] = content
		}
		r.traceLLM(ctx, run, "tools.search_products.filter", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, extra))
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

func rerankProductsWithStructuredConstraints(query string, products []domain.ProductCard, structured productSearchStructuredArguments) []domain.ProductCard {
	if len(products) == 0 || (emptyProductSearchArguments(structured.Constraints) && emptyProductSearchArguments(structured.Negative)) {
		return products
	}
	type scoredProduct struct {
		product domain.ProductCard
		score   int
		index   int
	}
	scored := make([]scoredProduct, 0, len(products))
	for index, product := range products {
		score := 0
		text := strings.ToLower(productSearchText(product))
		for _, term := range append(append([]string{}, structured.Constraints.Brands...), append(structured.Constraints.Terms, structured.Constraints.Categories...)...) {
			if term != "" && strings.Contains(text, strings.ToLower(term)) {
				score += 12
			}
		}
		for _, term := range append(append([]string{}, structured.Negative.Brands...), append(structured.Negative.Terms, structured.Negative.Categories...)...) {
			if term != "" && strings.Contains(text, strings.ToLower(term)) {
				score -= 100
			}
		}
		if score == 0 && query != "" && strings.Contains(text, strings.ToLower(query)) {
			score++
		}
		scored = append(scored, scoredProduct{product: product, score: score, index: index})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].index < scored[j].index
		}
		return scored[i].score > scored[j].score
	})
	out := make([]domain.ProductCard, 0, len(scored))
	for _, item := range scored {
		out = append(out, item.product)
	}
	return out
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

func productCardIDs(products []domain.ProductCard) []string {
	ids := make([]string, 0, len(products))
	for _, product := range products {
		if strings.TrimSpace(product.ProductID) != "" {
			ids = append(ids, product.ProductID)
		}
	}
	return ids
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
