package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LYP-leo/xzxg-shop/backend/src/agent"
)

type guideSuggestionRequest struct {
	Page    string         `json:"page"`
	Context map[string]any `json:"context"`
	Limit   int            `json:"limit"`
}

type guideSuggestionItem struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Reason   string `json:"reason"`
}

func (s *Server) handleGuideSuggestions(w http.ResponseWriter, r *http.Request) {
	var request guideSuggestionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求 JSON 不合法")
		return
	}
	items, errCode := buildGuideSuggestions(request)
	if errCode != "" {
		writeError(w, http.StatusBadRequest, errCode, "向导建议请求参数不合法")
		return
	}
	if s.runtime != nil {
		if generated, err := s.runtime.GenerateGuideSuggestions(r.Context(), agent.GuideSuggestionRequest{
			Page:       strings.TrimSpace(request.Page),
			Context:    request.Context,
			Limit:      request.Limit,
			Candidates: toAgentGuideSuggestionCandidates(items),
		}); err == nil && len(generated) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{"items": fromAgentGuideSuggestionCandidates(generated)})
			return
		} else if err != nil && s.logger != nil {
			s.logger.Warn("guide suggestions model fallback", "request_id", requestIDFromContext(r.Context()), "error", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func buildGuideSuggestions(request guideSuggestionRequest) ([]guideSuggestionItem, string) {
	page := strings.TrimSpace(request.Page)
	if request.Context == nil {
		request.Context = map[string]any{}
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 3 {
		limit = 3
	}

	var candidates []guideSuggestionItem
	switch page {
	case "products":
		candidates = productPageGuideSuggestions(request.Context)
	case "cart":
		candidates = cartPageGuideSuggestions(request.Context)
	case "orders":
		candidates = orderPageGuideSuggestions(request.Context)
	case "product_detail":
		candidates = productDetailGuideSuggestions(request.Context)
	default:
		return nil, "bad_page"
	}

	items := make([]guideSuggestionItem, 0, limit)
	seen := map[string]bool{}
	for _, item := range candidates {
		item.Question = compactQuestion(item.Question)
		if item.ID == "" || item.Question == "" || seen[item.Question] {
			continue
		}
		seen[item.Question] = true
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return items, ""
}

func productPageGuideSuggestions(ctx map[string]any) []guideSuggestionItem {
	category := shortQuestionTerm(contextString(ctx, "category_name"), 8)
	keyword := shortQuestionTerm(contextString(ctx, "keyword"), 8)
	items := []guideSuggestionItem{}
	if keyword != "" {
		items = append(items, guideSuggestionItem{
			ID:       "keyword_recommend",
			Question: "帮我选" + keyword,
			Reason:   "根据当前搜索词生成可直接发送给导购的选购问题",
		})
	}
	if category != "" {
		items = append(items, guideSuggestionItem{
			ID:       "category_budget",
			Question: category + "怎么选？",
			Reason:   "根据当前分类引导用户补充预算和偏好",
		})
	}
	items = append(items,
		guideSuggestionItem{ID: "compare_visible", Question: "比较当前商品", Reason: "用户可能正在浏览多个候选商品"},
		guideSuggestionItem{ID: "budget_recommend", Question: "高性价比推荐", Reason: "泛选场景下给用户一个低门槛入口"},
	)
	return items
}

func cartPageGuideSuggestions(ctx map[string]any) []guideSuggestionItem {
	count := contextInt(ctx, "cart_item_count")
	items := []guideSuggestionItem{}
	if count > 0 {
		items = append(items, guideSuggestionItem{
			ID:       "cart_analyze",
			Question: "帮我分析这" + strconv.Itoa(count) + "件商品",
			Reason:   "购物车内已有商品，适合做购买决策和优惠分析",
		})
	}
	items = append(items,
		guideSuggestionItem{ID: "cart_worth", Question: "购物车哪些值得买？", Reason: "引导用户筛选购物车里的高价值商品"},
		guideSuggestionItem{ID: "cart_checkout", Question: "现在适合直接下单吗？", Reason: "帮助用户确认优惠、库存和下单时机"},
		guideSuggestionItem{ID: "cart_coupon", Question: "看看能省多少", Reason: "购物车页面常见诉求是优惠试算"},
	)
	return items
}

func orderPageGuideSuggestions(ctx map[string]any) []guideSuggestionItem {
	items := []guideSuggestionItem{}
	if contextInt(ctx, "pending_payment_count") > 0 {
		items = append(items, guideSuggestionItem{ID: "pending_payment", Question: "哪些订单还没支付？", Reason: "当前存在待支付订单"})
	}
	if contextInt(ctx, "pending_review_count") > 0 {
		items = append(items, guideSuggestionItem{ID: "pending_review", Question: "待评价怎么写？", Reason: "当前存在可评价订单"})
	}
	items = append(items,
		guideSuggestionItem{ID: "order_summary", Question: "总结订单状态", Reason: "订单页适合聚合物流、支付和售后状态"},
		guideSuggestionItem{ID: "order_action", Question: "哪些订单待办？", Reason: "帮助用户快速定位待办订单"},
	)
	return items
}

func productDetailGuideSuggestions(ctx map[string]any) []guideSuggestionItem {
	name := shortQuestionTerm(contextString(ctx, "product_name"), 10)
	items := []guideSuggestionItem{}
	if name != "" {
		items = append(items, guideSuggestionItem{ID: "product_worth", Question: name + "值得买吗？", Reason: "商品详情页最常见的决策问题"})
	}
	items = append(items,
		guideSuggestionItem{ID: "product_fit", Question: "这个商品适合我吗？", Reason: "引导用户结合自身场景判断适配性"},
		guideSuggestionItem{ID: "product_risk", Question: "这个商品有什么缺点？", Reason: "帮助用户了解风险和避坑点"},
		guideSuggestionItem{ID: "product_compare", Question: "同类里它值得买吗？", Reason: "详情页常见横向对比诉求"},
	)
	return items
}

func contextString(ctx map[string]any, key string) string {
	return strings.TrimSpace(stringValue(ctx[key]))
}

func contextInt(ctx map[string]any, key string) int {
	return intValue(ctx[key])
}

func shortQuestionTerm(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), "")
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

func compactQuestion(question string) string {
	question = strings.TrimSpace(question)
	const maxRunes = 10
	if utf8.RuneCountInString(question) <= maxRunes {
		return question
	}
	runes := []rune(question)
	return string(runes[:maxRunes])
}

func toAgentGuideSuggestionCandidates(items []guideSuggestionItem) []agent.GuideSuggestionCandidate {
	candidates := make([]agent.GuideSuggestionCandidate, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, agent.GuideSuggestionCandidate{
			ID:       item.ID,
			Question: item.Question,
			Reason:   item.Reason,
		})
	}
	return candidates
}

func fromAgentGuideSuggestionCandidates(items []agent.GuideSuggestionCandidate) []guideSuggestionItem {
	candidates := make([]guideSuggestionItem, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, guideSuggestionItem{
			ID:       item.ID,
			Question: item.Question,
			Reason:   item.Reason,
		})
	}
	return candidates
}
