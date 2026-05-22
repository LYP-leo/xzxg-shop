package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Runtime struct {
	store      store.Store
	configs    configcenter.Center
	logger     *slog.Logger
	llm        *LLMClient
	baseConfig RuntimeConfig
	configMu   sync.Mutex
	configNext time.Time
}

func NewRuntime(store store.Store, configs configcenter.Center, logger *slog.Logger, config RuntimeConfig) *Runtime {
	return &Runtime{store: store, configs: configs, logger: logger, llm: NewLLMClient(config.Models), baseConfig: config}
}

func (r *Runtime) ClassifyIntent(ctx context.Context, query string) string {
	return r.classifyIntent(ctx, domain.AgentRun{}, query, false).ReferenceIntent()
}

func (r *Runtime) ClassifyPlan(ctx context.Context, query string) runPlan {
	return r.classifyIntent(ctx, domain.AgentRun{}, query, false)
}

func (r *Runtime) Stream(ctx context.Context, run domain.AgentRun, message domain.UserMessage, emit func(domain.SSEEvent) error) error {
	r.refreshDynamicConfig(ctx)
	r.logger.Info("agent run started", "run_id", run.RunID, "session_id", run.SessionID, "message_id", message.MessageID)
	r.trace(ctx, run, "run", "start", "", "ok", 0, "", map[string]any{
		"session_id": run.SessionID,
		"message_id": message.MessageID,
	})

	if err := emit(domain.SSEEvent{
		Type:          "message_start",
		RunID:         run.RunID,
		SessionID:     run.SessionID,
		UserMessageID: message.MessageID,
		TraceID:       run.TraceID,
	}); err != nil {
		return err
	}

	if needsPhotoSearch(message) {
		if err := r.emitText(run, "我已收到拍照找货请求，但当前还没有配置 VLM 图片识别服务，暂时不能可靠识别图片里的商品。可以先用文字描述品牌、品类、颜色、预算，我会继续用商品库帮你找相似商品。", emit); err != nil {
			return err
		}
		block := domain.AgentBlock{Type: "warning", Code: "vlm_not_configured", Message: "拍照找货需要先配置图片识别/VLM 服务，当前不会伪造识别结果。"}
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block}); err != nil {
			return err
		}
		if _, ok := r.store.UpdateRunStatus(ctx, run.AccountID, run.RunID, domain.RunStatusCompleted); !ok {
			r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
		}
		r.trace(ctx, run, "multimodal", "photo_search", "", "degraded", 0, "", map[string]any{"reason": "vlm_not_configured"})
		r.trace(ctx, run, "run", "completed", "", "ok", 0, "", nil)
		return emit(domain.SSEEvent{Type: "message_end", RunID: run.RunID})
	}

	if err := r.emitStatus(ctx, run.RunID, "intent", "正在理解你的需求", emit); err != nil {
		return err
	}
	plan := r.plan(ctx, run, message.Content)
	r.logger.Info("agent plan selected", "run_id", run.RunID, "route", plan.Route, "intent", plan.ReferenceIntent(), "answer_model", plan.AnswerModel)
	r.trace(ctx, run, "planner", "selected", plan.AnswerModel, "ok", 0, "", map[string]any{
		"route":           plan.Route,
		"intent":          plan.ReferenceIntent(),
		"level":           plan.Level,
		"secondary_level": plan.SecondaryLevel,
	})

	var products []domain.ProductCard
	var chunks []domain.Citation
	if plan.UsesCatalog() {
		products = r.store.SearchProducts(ctx, message.Content)
		chunks = r.store.SearchKnowledge(ctx, message.Content)
	}
	r.trace(ctx, run, "tools", "retrieval", "", "ok", 0, "", map[string]any{
		"uses_catalog":  plan.UsesCatalog(),
		"product_count": len(products),
		"chunk_count":   len(chunks),
	})

	if err := r.emitStatus(ctx, run.RunID, "tool", "正在检索商品和知识库", emit); err != nil {
		return err
	}

	if handled, err := r.executeDeterministicIntent(ctx, run, plan, message.Content, products, emit); handled || err != nil {
		if err != nil {
			return err
		}
		if _, ok := r.store.UpdateRunStatus(ctx, run.AccountID, run.RunID, domain.RunStatusCompleted); !ok {
			r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
		}
		r.trace(ctx, run, "run", "completed", "", "ok", 0, "", nil)
		return emit(domain.SSEEvent{Type: "message_end", RunID: run.RunID})
	}

	if err := r.emitStatus(ctx, run.RunID, "answer", "正在生成导购建议", emit); err != nil {
		return err
	}

	if err := r.streamAnswer(ctx, run, plan, message.Content, products, chunks, emit); err != nil {
		return err
	}

	if plan.UsesCatalog() && len(products) > 0 {
		product := products[0]
		if err := emit(domain.SSEEvent{
			Type:  "block_delta",
			RunID: run.RunID,
			Block: &domain.AgentBlock{Type: "product_card", Product: &product},
		}); err != nil {
			return err
		}
	}

	if isComparisonIntent(plan.Intent, message.Content) && len(products) >= 2 {
		block := comparisonBlock(products, 3)
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block}); err != nil {
			return err
		}
	}

	if plan.UsesCatalog() && len(chunks) > 0 {
		citation := chunks[0]
		if err := emit(domain.SSEEvent{
			Type:  "block_delta",
			RunID: run.RunID,
			Block: &domain.AgentBlock{Type: "citation", Citation: &citation},
		}); err != nil {
			return err
		}
	}

	if err := emit(domain.SSEEvent{
		Type:      "followups",
		RunID:     run.RunID,
		Questions: r.followups(ctx, run, message.Content, products),
	}); err != nil {
		return err
	}

	if _, ok := r.store.UpdateRunStatus(ctx, run.AccountID, run.RunID, domain.RunStatusCompleted); !ok {
		r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
	}
	r.trace(ctx, run, "run", "completed", "", "ok", 0, "", nil)
	return emit(domain.SSEEvent{Type: "message_end", RunID: run.RunID})
}

func (r *Runtime) emitStatus(ctx context.Context, runID string, stage string, text string, emit func(domain.SSEEvent) error) error {
	if r.store.IsRunCanceled(ctx, runID) {
		return emit(domain.SSEEvent{Type: "error", RunID: runID, Code: "canceled", Message: "已停止生成"})
	}
	if err := emit(domain.SSEEvent{Type: "status", RunID: runID, Stage: stage, Text: text}); err != nil {
		return err
	}
	time.Sleep(120 * time.Millisecond)
	return nil
}

type runPlan struct {
	Intent         string `json:"intent"`
	Route          string `json:"route,omitempty"`
	Level          string `json:"level,omitempty"`
	SecondaryLevel string `json:"secondary_level,omitempty"`
	Reasoning      string `json:"reasoning,omitempty"`
	AnswerModel    string `json:"-"`
}

func (p runPlan) ReferenceIntent() string {
	if p.Intent != "" {
		return p.Intent
	}
	if p.Route != "" {
		return p.Route
	}
	return p.Intent
}

func (p runPlan) UsesCatalog() bool {
	switch p.Route {
	case "guide":
		return true
	case "fast_product":
		return p.Intent == "cart_add"
	default:
		return false
	}
}

func (r *Runtime) plan(ctx context.Context, run domain.AgentRun, query string) runPlan {
	return r.classifyIntent(ctx, run, query, true)
}

func (r *Runtime) classifyIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool) runPlan {
	fallback := heuristicPlan(query, r.llm.SmallModel(), r.llm.LargeModel())
	if isGreeting(query) || isToolIntent(fallback.Intent) {
		return fallback
	}
	if !r.llm.Enabled() {
		return fallback
	}

	startedAt := time.Now()
	routeContent, err := r.llm.Complete(ctx, r.llm.SmallModel(), []ChatMessage{
		{
			Role:    "system",
			Content: r.stringConfig(ctx, "agent.prompt.route", configcenter.DefaultRoutePrompt),
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：reasoning、route。",
		},
	}, 0.1)
	if err != nil {
		r.logger.Warn("agent route fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.route", r.llm.SmallModel(), startedAt, err, nil)
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.route", r.llm.SmallModel(), startedAt, nil, map[string]any{"raw_length": len([]rune(routeContent))})
	}

	var plan runPlan
	if err := json.Unmarshal([]byte(extractJSONObject(routeContent)), &plan); err != nil {
		r.logger.Warn("agent route json fallback", "error", err, "content", routeContent)
		return fallback
	}
	plan.Route = normalizeRoute(plan.Route, fallback.Route)

	if plan.Route == "guide" {
		guidePlan := r.classifyGuideIntent(ctx, run, query, recordTrace, fallback)
		guidePlan.Route = "guide"
		guidePlan.AnswerModel = r.answerModelForPlan(guidePlan)
		return guidePlan
	}

	plan.Intent = normalizeRouteIntent(plan.Route, query, fallback.Intent)
	if isToolIntent(fallback.Intent) {
		plan.Intent = fallback.Intent
		plan.Route = "fast_product"
	}
	plan.AnswerModel = r.answerModelForPlan(plan)
	return plan
}

func (r *Runtime) classifyGuideIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool, fallback runPlan) runPlan {
	startedAt := time.Now()
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), []ChatMessage{
		{
			Role:    "system",
			Content: r.stringConfig(ctx, "agent.prompt.guide_intent", configcenter.DefaultGuideIntentPrompt),
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：reasoning、is_guide、intent、level、secondary_level。",
		},
	}, 0.1)
	if err != nil {
		r.logger.Warn("agent guide intent fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.guide_intent", r.llm.SmallModel(), startedAt, err, nil)
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.guide_intent", r.llm.SmallModel(), startedAt, nil, map[string]any{"raw_length": len([]rune(content))})
	}

	var guidePlan runPlan
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &guidePlan); err != nil {
		r.logger.Warn("agent guide intent json fallback", "error", err, "content", content)
		return fallback
	}
	guidePlan.Intent = normalizeGuideReferenceIntent(guidePlan)
	if guidePlan.Intent == "" {
		guidePlan.Intent = fallback.ReferenceIntent()
	}
	return guidePlan
}

func normalizeRoute(route string, fallback string) string {
	switch strings.TrimSpace(route) {
	case "guide", "non_guide", "fast_product":
		return strings.TrimSpace(route)
	default:
		if fallback != "" {
			return fallback
		}
		return "guide"
	}
}

func normalizeGuideReferenceIntent(plan runPlan) string {
	intent := strings.TrimSpace(plan.Intent)
	level := strings.ToUpper(strings.TrimSpace(plan.Level))
	secondary := strings.ToUpper(strings.TrimSpace(plan.SecondaryLevel))

	switch intent {
	case "product_deep", "compare_decide", "outfit_styling", "category_shop_brand", "category_shop_no_brand", "category_shop_complex", "scene_solution", "open_explore":
		return intent
	case "category_shop":
		switch secondary {
		case "P4A":
			return "category_shop_brand"
		case "P4C":
			return "category_shop_complex"
		default:
			return "category_shop_no_brand"
		}
	}

	switch level {
	case "P1":
		return "product_deep"
	case "P2":
		return "compare_decide"
	case "P3":
		return "outfit_styling"
	case "P4":
		switch secondary {
		case "P4A":
			return "category_shop_brand"
		case "P4C":
			return "category_shop_complex"
		default:
			return "category_shop_no_brand"
		}
	case "P5":
		return "scene_solution"
	case "P6":
		return "open_explore"
	}
	return ""
}

func normalizeRouteIntent(route string, query string, fallback string) string {
	switch route {
	case "fast_product":
		switch {
		case looksCartAdd(query):
			return "cart_add"
		case looksCartRemove(query):
			return "cart_remove"
		case looksCartQuantityUpdate(query):
			return "cart_update_quantity"
		case looksCheckout(query):
			return "checkout_confirm"
		}
		return fallback
	case "non_guide":
		return "non_guide"
	case "guide":
		return "open_explore"
	default:
		return fallback
	}
}

func (r *Runtime) streamAnswer(ctx context.Context, run domain.AgentRun, plan runPlan, query string, products []domain.ProductCard, chunks []domain.Citation, emit func(domain.SSEEvent) error) error {
	if !r.llm.Enabled() {
		return r.emitFallbackAnswer(ctx, run, buildAnswer(query, plan, products), emit)
	}

	productContext := formatProducts(products, 5)
	knowledgeContext := formatCitations(chunks, 3)
	startedAt := time.Now()
	var content strings.Builder
	err := r.llm.Stream(ctx, plan.AnswerModel, []ChatMessage{
		{
			Role:    "system",
			Content: r.systemPromptForPlan(ctx, plan),
		},
		{
			Role: "user",
			Content: fmt.Sprintf("用户问题：%s\n\n一级路由：%s\n意图：%s\n层级：%s\n二级层级：%s\n\n候选商品：\n%s\n\n资料片段：\n%s",
				query,
				plan.Route,
				plan.ReferenceIntent(),
				plan.Level,
				plan.SecondaryLevel,
				productContext,
				knowledgeContext,
			),
		},
	}, 0.4, func(delta string) error {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		content.WriteString(delta)
		return emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: delta})
	})
	if err != nil {
		r.logger.Warn("agent answer fallback", "error", err, "model", plan.AnswerModel)
		r.traceLLM(ctx, run, "answer", plan.AnswerModel, startedAt, err, map[string]any{"route": plan.Route, "intent": plan.ReferenceIntent()})
		if content.Len() > 0 {
			return nil
		}
		return r.emitFallbackAnswer(ctx, run, buildAnswer(query, plan, products), emit)
	}
	r.traceLLM(ctx, run, "answer", plan.AnswerModel, startedAt, nil, map[string]any{"route": plan.Route, "intent": plan.ReferenceIntent(), "raw_length": len([]rune(content.String()))})
	return nil
}

func (r *Runtime) executeDeterministicIntent(ctx context.Context, run domain.AgentRun, plan runPlan, query string, products []domain.ProductCard, emit func(domain.SSEEvent) error) (bool, error) {
	if isGreeting(query) {
		if err := r.emitText(run, "你好，我是小猪小狗 AI 导购。你可以告诉我想买的品类、预算、用途，或直接说“帮我推荐一款手机”。", emit); err != nil {
			return true, err
		}
		return true, nil
	}
	if plan.Route == "non_guide" {
		if err := r.emitText(run, nonGuideResponse(query), emit); err != nil {
			return true, err
		}
		return true, nil
	}
	if plan.Route != "fast_product" {
		return false, nil
	}
	switch plan.Intent {
	case "cart_add":
		if len(products) == 0 {
			return true, r.emitToolClarification(run, "我还不能确定要加入购物车的是哪件商品。请说清楚商品名称，或先让我推荐一个候选。", emit)
		}
		product := products[0]
		quantity := parsePositiveNumber(query, 1)
		cart, ok := r.store.AddCartItem(ctx, run.AccountID, product.ProductID, product.SkuID, quantity)
		if !ok {
			return true, r.emitToolClarification(run, "加购失败，这个商品可能已经下架或库存状态不可用。", emit)
		}
		r.trace(ctx, run, "tools", "cart_add", "", "ok", 0, "", map[string]any{"product_id": product.ProductID, "quantity": quantity})
		if err := r.emitText(run, fmt.Sprintf("已把 %s 加入购物车，数量 %d。", product.Name, quantity), emit); err != nil {
			return true, err
		}
		return true, emitCartBlock(run.RunID, cart, emit)
	case "cart_update_quantity":
		cart := r.store.GetCart(ctx, run.AccountID)
		if len(cart.Items) == 0 {
			return true, r.emitToolClarification(run, "购物车还是空的，暂时没有商品可以修改数量。", emit)
		}
		index := parseOrdinal(query, 0)
		if index < 0 || index >= len(cart.Items) {
			return true, r.emitToolClarification(run, "我没有找到你说的那一项。可以说“把第 1 个商品数量改成 2”。", emit)
		}
		quantity := parsePositiveNumber(query, 1)
		cart, ok := r.store.UpdateCartItem(ctx, run.AccountID, cart.Items[index].CartItemID, &quantity, nil)
		if !ok {
			return true, r.emitToolClarification(run, "修改购物车数量失败，请刷新购物车后再试。", emit)
		}
		r.trace(ctx, run, "tools", "cart_update_quantity", "", "ok", 0, "", map[string]any{"cart_index": index + 1, "quantity": quantity})
		if err := r.emitText(run, fmt.Sprintf("已把第 %d 个商品数量改成 %d。", index+1, quantity), emit); err != nil {
			return true, err
		}
		return true, emitCartBlock(run.RunID, cart, emit)
	case "cart_remove":
		cart := r.store.GetCart(ctx, run.AccountID)
		if len(cart.Items) == 0 {
			return true, r.emitToolClarification(run, "购物车还是空的，暂时没有商品可以删除。", emit)
		}
		index := parseOrdinal(query, 0)
		if index < 0 || index >= len(cart.Items) {
			return true, r.emitToolClarification(run, "我没有找到你说的那一项。可以说“删除第 2 个商品”。", emit)
		}
		removed := cart.Items[index]
		cart, ok := r.store.DeleteCartItem(ctx, run.AccountID, removed.CartItemID)
		if !ok {
			return true, r.emitToolClarification(run, "删除购物车商品失败，请刷新购物车后再试。", emit)
		}
		r.trace(ctx, run, "tools", "cart_remove", "", "ok", 0, "", map[string]any{"cart_index": index + 1, "product_id": removed.ProductID})
		if err := r.emitText(run, fmt.Sprintf("已从购物车删除 %s。", removed.Name), emit); err != nil {
			return true, err
		}
		return true, emitCartBlock(run.RunID, cart, emit)
	case "checkout_confirm":
		cart := r.store.GetCart(ctx, run.AccountID)
		if len(cart.Items) == 0 || cart.Summary.SelectedCount == 0 {
			return true, r.emitToolClarification(run, "购物车里还没有选中的商品，先加购或勾选商品后我再帮你下单。", emit)
		}
		if !containsAny(query, []string{"确认", "下单", "结算", "提交订单"}) {
			if err := r.emitText(run, fmt.Sprintf("当前选中 %d 件商品，应付 %s 元。确认后我可以帮你提交订单。", cart.Summary.SelectedCount, cart.Summary.PayAmount), emit); err != nil {
				return true, err
			}
			return true, emitCartBlock(run.RunID, cart, emit)
		}
		orders, ok := r.store.CreateOrderFromCart(ctx, run.AccountID)
		if !ok {
			return true, r.emitToolClarification(run, "提交订单失败，请确认购物车里有已选中的有效商品。", emit)
		}
		r.trace(ctx, run, "tools", "checkout", "", "ok", 0, "", map[string]any{"order_count": len(orders)})
		if err := r.emitText(run, fmt.Sprintf("已提交 %d 个订单，商家会按待发货流程处理。", len(orders)), emit); err != nil {
			return true, err
		}
		block := domain.AgentBlock{Type: "order_summary", Orders: orders}
		return true, emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block})
	default:
		return false, nil
	}
}

func (r *Runtime) emitToolClarification(run domain.AgentRun, text string, emit func(domain.SSEEvent) error) error {
	if err := r.emitText(run, text, emit); err != nil {
		return err
	}
	block := domain.AgentBlock{Type: "warning", Code: "need_clarification", Message: text}
	return emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block})
}

func (r *Runtime) emitText(run domain.AgentRun, text string, emit func(domain.SSEEvent) error) error {
	for _, delta := range splitText(text, 24) {
		if err := emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: delta}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) emitFallbackAnswer(ctx context.Context, run domain.AgentRun, text string, emit func(domain.SSEEvent) error) error {
	for _, delta := range splitText(text, 28) {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		if err := emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: delta}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) followups(ctx context.Context, run domain.AgentRun, query string, products []domain.ProductCard) []string {
	fallback := []string{"你更重视价格、性能还是售后？", "要不要我帮你对比前两个候选？"}
	if !r.boolConfig(ctx, "agent.followups_enabled", true) {
		return fallback
	}
	if !r.llm.Enabled() {
		return fallback
	}
	startedAt := time.Now()
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), []ChatMessage{
		{Role: "system", Content: r.stringConfig(ctx, "agent.prompt.followups", configcenter.DefaultFollowupsPrompt)},
		{Role: "user", Content: fmt.Sprintf("用户问题：%s\n候选商品：\n%s", query, formatProducts(products, 3))},
	}, 0.2)
	if err != nil {
		r.logger.Warn("agent followups fallback", "error", err)
		r.traceLLM(ctx, run, "followups", r.llm.SmallModel(), startedAt, err, nil)
		return fallback
	}
	r.traceLLM(ctx, run, "followups", r.llm.SmallModel(), startedAt, nil, map[string]any{"raw_length": len([]rune(content))})
	var questions []string
	if err := json.Unmarshal([]byte(extractJSONArray(content)), &questions); err != nil {
		r.logger.Warn("agent followups json fallback", "error", err, "content", content)
		return fallback
	}
	cleaned := make([]string, 0, 2)
	for _, question := range questions {
		question = strings.TrimSpace(question)
		if question != "" {
			cleaned = append(cleaned, question)
		}
		if len(cleaned) == 2 {
			break
		}
	}
	if len(cleaned) == 0 {
		return fallback
	}
	return cleaned
}

func (r *Runtime) refreshDynamicConfig(ctx context.Context) {
	r.configMu.Lock()
	defer r.configMu.Unlock()
	if time.Now().Before(r.configNext) {
		return
	}

	values := r.configs.GetMap(ctx)
	models := r.baseConfig.Models
	if value := strings.TrimSpace(values["ai.base_url"]); value != "" {
		models.BaseURL = value
	}
	if value := strings.TrimSpace(values["ai.api_key"]); value != "" {
		models.APIKey = value
	}
	if value := strings.TrimSpace(values["ai.small_model"]); value != "" {
		models.SmallModel = value
	}
	if value := strings.TrimSpace(values["ai.large_model"]); value != "" {
		models.LargeModel = value
	}
	if value := strings.TrimSpace(values["ai.enable_thinking"]); value != "" {
		models.EnableThinking = parseBool(value, false)
	}
	if value := strings.TrimSpace(values["ai.enabled"]); value != "" && !parseBool(value, true) {
		models.APIKey = ""
	}
	r.llm.UpdateConfig(models)

	refreshSeconds := 15
	if value := strings.TrimSpace(values["agent.config_refresh_seconds"]); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			refreshSeconds = parsed
		}
	}
	r.configNext = time.Now().Add(time.Duration(refreshSeconds) * time.Second)
}

func (r *Runtime) boolConfig(ctx context.Context, key string, fallback bool) bool {
	r.refreshDynamicConfig(ctx)
	values := r.configs.GetMap(ctx)
	if value, ok := values[key]; ok {
		return parseBool(value, fallback)
	}
	return fallback
}

func (r *Runtime) stringConfig(ctx context.Context, key string, fallback string) string {
	values := r.configs.GetMap(ctx)
	if value := strings.TrimSpace(values[key]); value != "" {
		return value
	}
	return fallback
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func heuristicPlan(query string, smallModel string, largeModel string) runPlan {
	plan := runPlan{
		Route:       "non_guide",
		Intent:      "non_guide",
		AnswerModel: smallModel,
	}
	if isGreeting(query) {
		return plan
	}
	if looksUnsupported(query) {
		plan.Intent = "non_guide"
		return plan
	}
	if looksCartAdd(query) {
		plan.Route = "fast_product"
		plan.Intent = "cart_add"
		return plan
	}
	if looksCartRemove(query) {
		plan.Route = "fast_product"
		plan.Intent = "cart_remove"
		return plan
	}
	if looksCartQuantityUpdate(query) {
		plan.Route = "fast_product"
		plan.Intent = "cart_update_quantity"
		return plan
	}
	if looksCheckout(query) {
		plan.Route = "fast_product"
		plan.Intent = "checkout_confirm"
		return plan
	}
	if strings.Contains(query, "对比") || strings.Contains(query, "比较") {
		plan.Route = "guide"
		plan.Intent = "compare_decide"
		plan.Level = "P2"
		plan.SecondaryLevel = "None"
		plan.AnswerModel = largeModel
		return plan
	}
	if containsAny(query, []string{"优惠", "促销", "满减", "券", "折扣", "活动"}) {
		plan.Intent = "non_guide"
		return plan
	}
	if looksCatalogRelated(query) {
		plan.Route = "guide"
		plan.Intent = "category_shop_no_brand"
		plan.Level = "P4"
		plan.SecondaryLevel = "P4B"
	}
	if strings.Contains(query, "哪个") {
		plan.Route = "guide"
		plan.Intent = "compare_decide"
		plan.Level = "P2"
		plan.SecondaryLevel = "None"
		plan.AnswerModel = largeModel
	}
	if strings.Contains(query, "售后") || strings.Contains(query, "退货") || strings.Contains(query, "保修") {
		plan.Route = "non_guide"
		plan.Intent = "non_guide"
		plan.Level = ""
		plan.SecondaryLevel = ""
	}
	return plan
}

func (r *Runtime) answerModelForPlan(plan runPlan) string {
	switch plan.Intent {
	case "compare_decide", "category_shop_complex", "scene_solution":
		return r.llm.LargeModel()
	default:
		return r.llm.SmallModel()
	}
}

func (r *Runtime) systemPromptForPlan(ctx context.Context, plan runPlan) string {
	base := []string{
		r.stringConfig(ctx, "agent.prompt.answer_base", configcenter.DefaultAnswerBasePrompt),
	}
	intent := plan.ReferenceIntent()
	fallback := configcenter.DefaultIntentPrompt(intent)
	if prompt := r.stringConfig(ctx, "agent.prompt.intent."+intent, fallback); prompt != "" {
		base = append(base, prompt)
	}
	return strings.Join(base, "\n")
}

func buildAnswer(query string, plan runPlan, products []domain.ProductCard) string {
	if plan.Route == "non_guide" {
		return nonGuideResponse(query)
	}
	if !plan.UsesCatalog() {
		return "你好，我是小猪小狗 AI 导购。你可以告诉我预算、品类、使用场景或想对比的商品，我会帮你缩小选择范围。"
	}
	if len(products) == 0 {
		return "我暂时没有检索到足够的商品数据，可以先补充预算、使用场景和偏好，我会继续缩小范围。"
	}
	first := products[0]
	if strings.Contains(query, "鼠标") {
		return "如果主要用于办公，我会优先看静音、握持舒适度和无线稳定性。当前更推荐 " + first.Name + "，它更适合长时间办公使用。"
	}
	return "结合你的需求，我会优先推荐 " + first.Name + "。它的价格、核心卖点和风险提示都来自商品库与知识库，适合先作为第一候选。"
}

func nonGuideResponse(query string) string {
	switch {
	case containsAny(query, []string{"优惠", "优惠券", "券", "红包", "会员权益", "返利", "促销", "活动"}):
		return "这类问题属于优惠权益或平台活动查询，不进入商品导购细分。你可以提供具体商品或活动名称，我会按当前资料说明能否确认；如果要选商品，也可以告诉我品类、预算和使用场景。"
	case containsAny(query, []string{"订单", "物流", "快递", "催发货", "取件", "取件码", "驿站", "复购"}):
		return "这类问题属于订单、物流或复购服务，不进入导购推荐链路。请到订单或物流页面查看；如果你想重新选购同类商品，可以告诉我之前买的商品或目标品类。"
	case containsAny(query, []string{"售后", "退货", "退款", "保修", "换货", "投诉", "改地址", "发票"}):
		return "这类问题属于售后或订单服务，不进入导购细分。具体规则需要以平台和商家政策为准；如果你是在下单前评估风险，可以告诉我商品名称，我会帮你看需要重点确认什么。"
	case isGreeting(query):
		return "你好，我是小猪小狗 AI 导购。你可以告诉我想买的品类、预算、用途，或直接说“帮我推荐一款手机”。"
	default:
		return "这个请求暂时不属于商品选购导购链路。你可以换成商品推荐、对比、搭配、场景清单或商品详情问题，我会继续帮你筛选。"
	}
}

func isGreeting(query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(query))
	return normalized == "你好" || normalized == "您好" || normalized == "hello" || normalized == "hi" || normalized == "嗨"
}

func looksCatalogRelated(query string) bool {
	keywords := []string{"推荐", "买", "商品", "手机", "鼠标", "电脑", "耳机", "价格", "预算", "对比", "比较", "售后", "退货", "保修", "优惠", "拍照", "办公", "护肤", "美妆", "服饰", "食品"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func looksCartAdd(query string) bool {
	return containsAny(query, []string{"加到购物车", "加入购物车", "加购物车", "放进购物车", "加购"})
}

func looksCartRemove(query string) bool {
	return strings.Contains(query, "购物车") && containsAny(query, []string{"删除", "移除", "不要"})
}

func looksCartQuantityUpdate(query string) bool {
	return strings.Contains(query, "购物车") && containsAny(query, []string{"数量", "改成", "改为", "调整到"})
}

func looksCheckout(query string) bool {
	return containsAny(query, []string{"下单", "结算", "提交订单", "确认购买", "确认订单"})
}

func looksUnsupported(query string) bool {
	keywords := []string{"论文", "破解", "绕过登录", "绕过鉴权", "脚本", "黑客", "攻击", "股票", "吃什么药", "起诉书", "代写"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func formatProducts(products []domain.ProductCard, limit int) string {
	if len(products) == 0 {
		return "无候选商品"
	}
	if len(products) < limit {
		limit = len(products)
	}
	lines := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		product := products[i]
		lines = append(lines, fmt.Sprintf("%d. 商品ID=%s 名称=%s 品牌=%s 价格=%s 库存=%s 标签=%s 卖点=%s 推荐依据=%s",
			i+1,
			product.ProductID,
			product.Name,
			product.Brand,
			product.Price,
			product.StockStatus,
			strings.Join(product.Tags, "；"),
			strings.Join(product.SellingPoints, "；"),
			product.RecommendReason,
		))
	}
	return strings.Join(lines, "\n")
}

func emitCartBlock(runID string, cart domain.Cart, emit func(domain.SSEEvent) error) error {
	block := domain.AgentBlock{Type: "cart_state", Cart: &cart}
	return emit(domain.SSEEvent{Type: "block_delta", RunID: runID, Block: &block})
}

func comparisonBlock(products []domain.ProductCard, limit int) domain.AgentBlock {
	if len(products) < limit {
		limit = len(products)
	}
	rows := make([]domain.ComparisonRow, 0, limit)
	for i := 0; i < limit; i++ {
		product := products[i]
		rows = append(rows, domain.ComparisonRow{
			ProductID: product.ProductID,
			Values: []string{
				product.Name,
				product.Brand,
				product.Price,
				strings.Join(product.SellingPoints, "；"),
			},
		})
	}
	return domain.AgentBlock{
		Type:    "comparison_table",
		Columns: []string{"商品", "品牌", "价格", "核心卖点"},
		Rows:    rows,
	}
}

func isComparisonIntent(intent string, query string) bool {
	return intent == "compare_decide" || strings.Contains(query, "对比") || strings.Contains(query, "比较")
}

func isToolIntent(intent string) bool {
	switch intent {
	case "cart_add", "cart_update_quantity", "cart_remove", "checkout_confirm":
		return true
	default:
		return false
	}
}

func parsePositiveNumber(text string, fallback int) int {
	re := regexp.MustCompile(`\d+`)
	values := re.FindAllString(text, -1)
	if len(values) > 0 {
		value := values[len(values)-1]
		if number, err := strconv.Atoi(value); err == nil && number > 0 {
			return number
		}
	}
	numbers := []struct {
		word   string
		number int
	}{
		{"十", 10}, {"九", 9}, {"八", 8}, {"七", 7}, {"六", 6}, {"五", 5}, {"四", 4}, {"三", 3}, {"二", 2}, {"两", 2}, {"一", 1},
	}
	for _, item := range numbers {
		if strings.Contains(text, "改成"+item.word) || strings.Contains(text, "改为"+item.word) || strings.Contains(text, "数量"+item.word) {
			return item.number
		}
	}
	return fallback
}

func parseOrdinal(text string, fallback int) int {
	re := regexp.MustCompile(`第\s*(\d+)\s*个`)
	if match := re.FindStringSubmatch(text); len(match) == 2 {
		if number, err := strconv.Atoi(match[1]); err == nil && number > 0 {
			return number - 1
		}
	}
	ordinals := map[string]int{"第一个": 0, "第一件": 0, "第二个": 1, "第二件": 1, "第三个": 2, "第三件": 2, "第四个": 3, "第四件": 3, "第五个": 4, "第五件": 4}
	for word, index := range ordinals {
		if strings.Contains(text, word) {
			return index
		}
	}
	return fallback
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func needsPhotoSearch(message domain.UserMessage) bool {
	if containsAny(message.Content, []string{"拍照找", "拍照搜", "找同款", "图片找", "识图"}) {
		return true
	}
	for _, attachment := range message.Attachments {
		if attachment.Type == "image" {
			return true
		}
	}
	return false
}

func formatCitations(chunks []domain.Citation, limit int) string {
	if len(chunks) == 0 {
		return "无资料片段"
	}
	if len(chunks) < limit {
		limit = len(chunks)
	}
	lines := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		chunk := chunks[i]
		lines = append(lines, fmt.Sprintf("%d. %s：%s", i+1, chunk.Title, chunk.Snippet))
	}
	return strings.Join(lines, "\n")
}

func extractJSONObject(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}

func extractJSONArray(content string) string {
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end >= start {
		return content[start : end+1]
	}
	return content
}

func (r *Runtime) trace(ctx context.Context, run domain.AgentRun, stage string, eventType string, model string, status string, durationMS int64, errText string, metadata map[string]any) {
	if err := r.store.RecordAgentTrace(ctx, domain.AgentTraceInput{
		RunID:        run.RunID,
		TraceID:      run.TraceID,
		AccountID:    run.AccountID,
		Stage:        stage,
		EventType:    eventType,
		Model:        model,
		Status:       status,
		DurationMS:   durationMS,
		Error:        errText,
		MetadataJSON: metadataJSON(metadata),
	}); err != nil {
		r.logger.Warn("record agent trace failed", "run_id", run.RunID, "stage", stage, "error", err)
	}
}

func (r *Runtime) traceLLM(ctx context.Context, run domain.AgentRun, stage string, model string, startedAt time.Time, callErr error, metadata map[string]any) {
	status := "ok"
	errText := ""
	if callErr != nil {
		status = "failed"
		errText = callErr.Error()
	}
	r.trace(ctx, run, stage, "llm_call", model, status, time.Since(startedAt).Milliseconds(), errText, metadata)
}

func metadataJSON(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "{}"
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func splitText(text string, size int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}
	parts := make([]string, 0, len(runes)/size+1)
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
