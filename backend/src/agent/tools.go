package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
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

type reactAction struct {
	Type      string          `json:"type"`
	Tool      string          `json:"tool,omitempty"`
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
	Tool       string         `json:"tool"`
	OK         bool           `json:"ok"`
	Message    string         `json:"message,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
	ProductIDs []string       `json:"product_ids,omitempty"`
	ChunkIDs   []string       `json:"chunk_ids,omitempty"`
	Cart       *domain.Cart   `json:"-"`
	Orders     []domain.Order `json:"-"`
	DurationMS int64          `json:"duration_ms"`
}

func (r *Runtime) executeTool(ctx context.Context, run domain.AgentRun, call reactAction) toolObservation {
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
		observation = r.toolSearchProducts(ctx, call.Arguments)
	case toolSearchKnowledge:
		observation = r.toolSearchKnowledge(ctx, call.Arguments)
	case toolGetCart:
		observation = r.toolGetCart(ctx, run.AccountID)
	case toolAddCartItem:
		observation = r.toolAddCartItem(ctx, run.AccountID, call.Arguments)
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

func (r *Runtime) toolSearchProducts(ctx context.Context, raw json.RawMessage) toolObservation {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	_ = json.Unmarshal(raw, &args)
	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return toolObservation{Tool: toolSearchProducts, Message: "query 不能为空"}
	}
	limit := clampLimit(args.Limit, 5, 10)
	products := r.store.SearchProducts(ctx, args.Query)
	if len(products) > limit {
		products = products[:limit]
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
	return toolObservation{
		Tool:       toolSearchProducts,
		OK:         true,
		Message:    fmt.Sprintf("检索到 %d 个商品", len(items)),
		Result:     map[string]any{"items": items},
		ProductIDs: productIDs,
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
	citations := r.store.SearchKnowledge(ctx, args.Query)
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

func (r *Runtime) toolAddCartItem(ctx context.Context, accountID string, raw json.RawMessage) toolObservation {
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
	switch action.Type {
	case "tool_call", "final":
		return action, nil
	default:
		return action, errors.New("unknown action type: " + action.Type)
	}
}

func observationForModel(observation toolObservation) string {
	payload := map[string]any{
		"tool":        observation.Tool,
		"ok":          observation.OK,
		"message":     observation.Message,
		"result":      observation.Result,
		"duration_ms": observation.DurationMS,
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
