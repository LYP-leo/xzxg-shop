package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
	"github.com/LYP-leo/xzxg-shop/backend/src/retrievalconfig"
	"github.com/LYP-leo/xzxg-shop/backend/src/risk"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Runtime struct {
	store      store.Store
	configs    configcenter.Center
	logger     *slog.Logger
	llm        *LLMClient
	baseConfig RuntimeConfig

	// 动态配置有短缓存，避免每个 token 流式回调都访问 Nacos。
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

func (r *Runtime) retrievalPlan(ctx context.Context, query string) rag.RetrievalPlan {
	plan := rag.DefaultRetrievalPlan(query)
	retrievalconfig.Apply(&plan, r.configs.GetMap(ctx))
	return plan
}

// Stream 是 Agent 单轮对话主链路。
// 输出协议为 SSE：正文用 text_delta，商品/订单/购物车等结构化信息用 block_delta。
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

	if result := risk.CheckText(message.Content, r.configs.GetMap(ctx)); result.Blocked {
		if err := r.emitText(run, result.Message, emit); err != nil {
			return err
		}
		block := domain.AgentBlock{Type: "warning", Code: result.Code, Message: result.Message}
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block}); err != nil {
			return err
		}
		if _, ok := r.store.UpdateRunStatus(ctx, run.AccountID, run.RunID, domain.RunStatusCompleted); !ok {
			r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
		}
		r.trace(ctx, run, "risk", "blocked", "", "blocked", 0, "", map[string]any{"code": result.Code, "matched": result.Matched})
		r.trace(ctx, run, "run", "completed", "", "blocked", 0, "", nil)
		return emit(domain.SSEEvent{Type: "message_end", RunID: run.RunID})
	}

	if needsPhotoSearch(message) {
		// 当前没有接 VLM，必须显式降级，不能假装识别图片。
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
	// planner 先做一级 route，再在 guide 路由内做 P1-P6 细分。
	plan := r.plan(ctx, run, message.Content)
	r.logger.Info("agent plan selected", "run_id", run.RunID, "route", plan.Route, "intent", plan.ReferenceIntent(), "answer_model", plan.AnswerModel)
	r.trace(ctx, run, "planner", "selected", plan.AnswerModel, "ok", 0, "", map[string]any{
		"route":           plan.Route,
		"intent":          plan.ReferenceIntent(),
		"level":           plan.Level,
		"secondary_level": plan.SecondaryLevel,
	})

	result, err := r.runReactAgent(ctx, run, plan, message.Content, emit)
	if err != nil {
		return err
	}
	for _, block := range result.FinalBlocks {
		item := block
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &item}); err != nil {
			return err
		}
	}

	if err := emit(domain.SSEEvent{
		Type:      "followups",
		RunID:     run.RunID,
		Questions: r.followups(ctx, run, message.Content, nil),
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

// classifyIntent 使用“两阶段意图识别”：
// 先判断 guide/non_guide/fast_product，再对 guide 做 P1-P6 细分。
func (r *Runtime) classifyIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool) runPlan {
	fallback := heuristicPlan(query, r.llm.SmallModel(), r.llm.LargeModel())
	if isGreeting(query) || isToolIntent(fallback.Intent) {
		return fallback
	}
	if !r.llm.Enabled() {
		return fallback
	}

	startedAt := time.Now()
	temperature := 0.1
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: r.stringConfig(ctx, "agent.prompt.route", configcenter.DefaultRoutePrompt),
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：reasoning、route。",
		},
	}
	routeContent, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if err != nil {
		r.logger.Warn("agent route fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.route", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, nil))
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.route", r.llm.SmallModel(), startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(routeContent))}))
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

// classifyGuideIntent 只处理导购内部的细分类；非导购和快速商品动作不会进入这里。
func (r *Runtime) classifyGuideIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool, fallback runPlan) runPlan {
	startedAt := time.Now()
	temperature := 0.1
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: r.stringConfig(ctx, "agent.prompt.guide_intent", configcenter.DefaultGuideIntentPrompt),
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：reasoning、is_guide、intent、level、secondary_level。",
		},
	}
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if err != nil {
		r.logger.Warn("agent guide intent fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.guide_intent", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, nil))
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.guide_intent", r.llm.SmallModel(), startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
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

// normalizeRoute 对模型输出做白名单收敛，防止 prompt 漂移导致未知 route。
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

// normalizeGuideReferenceIntent 把 P1-P6/P4A-P4C 统一映射到代码内部使用的 intent key。
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

// normalizeRouteIntent 为非导购和快速商品动作补齐可执行 intent。
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
	temperature := 0.2
	messages := []ChatMessage{
		{Role: "system", Content: r.stringConfig(ctx, "agent.prompt.followups", configcenter.DefaultFollowupsPrompt)},
		{Role: "user", Content: fmt.Sprintf("用户问题：%s\n候选商品：\n%s", query, formatProducts(products, 3))},
	}
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if err != nil {
		r.logger.Warn("agent followups fallback", "error", err)
		r.traceLLM(ctx, run, "followups", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, nil))
		return fallback
	}
	r.traceLLM(ctx, run, "followups", r.llm.SmallModel(), startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
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
	if provider := activeModelProvider(values); provider != "" {
		prefix := "ai." + provider + "."
		if value := strings.TrimSpace(values[prefix+"base_url"]); value != "" {
			models.BaseURL = value
		}
		if value := strings.TrimSpace(values[prefix+"api_key"]); value != "" {
			models.APIKey = value
		}
		if value := strings.TrimSpace(values[prefix+"small_model"]); value != "" {
			models.SmallModel = value
		}
		if value := strings.TrimSpace(values[prefix+"large_model"]); value != "" {
			models.LargeModel = value
		}
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

func activeModelProvider(values map[string]string) string {
	provider := strings.ToLower(strings.TrimSpace(values["ai.active_provider"]))
	if provider == "" {
		return ""
	}
	provider = strings.ReplaceAll(provider, "-", "_")
	for _, char := range provider {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return ""
		}
	}
	return provider
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
	if plan.Route == "fast_product" || isToolIntent(plan.Intent) {
		return r.llm.SmallModel()
	}
	return r.llm.LargeModel()
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

func isToolIntent(intent string) bool {
	switch intent {
	case "cart_add", "cart_update_quantity", "cart_remove", "checkout_confirm":
		return true
	default:
		return false
	}
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

func llmPromptMetadata(messages []ChatMessage, temperature float64, extra map[string]any) map[string]any {
	metadata := map[string]any{
		"temperature":  temperature,
		"messages":     sanitizeTraceMessages(messages),
		"prompt_chars": promptChars(messages),
		"prompt_hash":  promptHash(messages),
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

func sanitizeTraceMessages(messages []ChatMessage) []map[string]string {
	out := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		out = append(out, map[string]string{
			"role":    message.Role,
			"content": redactTraceText(message.Content),
		})
	}
	return out
}

func promptChars(messages []ChatMessage) int {
	total := 0
	for _, message := range messages {
		total += len([]rune(message.Content))
	}
	return total
}

func promptHash(messages []ChatMessage) string {
	var builder strings.Builder
	for _, message := range messages {
		builder.WriteString(message.Role)
		builder.WriteString("\n")
		builder.WriteString(message.Content)
		builder.WriteString("\n---\n")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

var traceSecretPattern = regexp.MustCompile(`(?i)(authorization:\s*bearer\s+|api[_-]?key["']?\s*[:=]\s*["']?|token["']?\s*[:=]\s*["']?|password["']?\s*[:=]\s*["']?)[^"',\s]+`)

func redactTraceText(text string) string {
	return traceSecretPattern.ReplaceAllString(text, `${1}******`)
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
