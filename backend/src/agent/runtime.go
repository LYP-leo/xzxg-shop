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

	configValues := r.configs.GetMap(ctx)
	if result := risk.CheckText(message.Content, configValues); result.Blocked {
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
	if account, ok := r.store.GetAccount(ctx, run.AccountID); ok {
		if result := risk.CheckAccount(account, configValues); result.Blocked {
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
			r.trace(ctx, run, "risk", "blocked_account", "", "blocked", 0, "", map[string]any{
				"code":           result.Code,
				"account_status": result.Matched,
			})
			r.trace(ctx, run, "run", "completed", "", "blocked", 0, "", nil)
			return emit(domain.SSEEvent{Type: "message_end", RunID: run.RunID})
		}
	}

	if err := r.emitStatus(ctx, run.RunID, "intent", "正在理解你的需求", emit); err != nil {
		return err
	}
	normalizedQuery := normalizeQueryWithAttachments(message.Content, message.Attachments)
	if normalizedQuery != strings.TrimSpace(message.Content) {
		r.trace(ctx, run, "query", "normalized", "", "ok", 0, "", map[string]any{
			"original_query":   message.Content,
			"normalized_query": normalizedQuery,
			"attachment_count": len(message.Attachments),
		})
	}
	memory := r.buildConversationMemory(ctx, run, normalizedQuery)
	effectiveQuery := r.formatQueryWithMemory(normalizedQuery, memory)
	if memory.HasRelevantMemory {
		r.trace(ctx, run, "memory", "applied", "", "ok", 0, "", map[string]any{
			"used_record_ids":        memory.UsedRecordIDs,
			"referenced_product_ids": memory.ReferencedProductIDs,
			"memory_summary":         memory.MemorySummary,
		})
	}
	// planner 先做一级 route，再在 guide 路由内做 P1-P6 细分。
	plan := r.plan(ctx, run, effectiveQuery)
	r.logger.Info("agent plan selected", "run_id", run.RunID, "route", plan.Route, "intent", plan.ReferenceIntent(), "answer_model", plan.AnswerModel)
	r.trace(ctx, run, "planner", "selected", plan.AnswerModel, "ok", 0, "", map[string]any{
		"route":           plan.Route,
		"intent":          plan.ReferenceIntent(),
		"level":           plan.Level,
		"secondary_level": plan.SecondaryLevel,
	})
	needSummaryCtx, cancelNeedSummary := context.WithCancel(ctx)
	needSummaryDone := make(chan struct{})
	defer cancelNeedSummary()
	go func() {
		defer close(needSummaryDone)
		r.emitNeedSummaryStep(needSummaryCtx, run, effectiveQuery, plan, emit)
	}()

	result, handled, err := r.runNonGuideAction(ctx, run, plan, effectiveQuery, emit)
	if err != nil {
		return err
	}
	if !handled {
		result, err = r.runReactAgent(ctx, run, plan, effectiveQuery, message.Attachments, emit)
	}
	if err != nil {
		return err
	}
	for _, block := range result.FinalBlocks {
		item := block
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &item}); err != nil {
			return err
		}
	}

	if plan.Route == "guide" {
		if err := emit(domain.SSEEvent{
			Type:      "followups",
			RunID:     run.RunID,
			Questions: r.followups(ctx, run, effectiveQuery, nil),
		}); err != nil {
			return err
		}
	}

	if _, ok := r.store.UpdateRunStatus(ctx, run.AccountID, run.RunID, domain.RunStatusCompleted); !ok {
		r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
	}
	if err := r.emitThinkingStep(ctx, run.RunID, domain.ThoughtStep{
		ID:      "answer",
		Title:   "总结答案",
		Status:  "done",
		Summary: "已根据当前可用信息生成回答。",
		Order:   3,
	}, emit); err != nil {
		return err
	}
	select {
	case <-needSummaryDone:
	default:
		cancelNeedSummary()
		select {
		case <-needSummaryDone:
		case <-time.After(150 * time.Millisecond):
		}
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
	if step, ok := thinkingStepForStatus(stage, text); ok {
		if err := r.emitThinkingStep(ctx, runID, step, emit); err != nil {
			return err
		}
	}
	time.Sleep(120 * time.Millisecond)
	return nil
}

func (r *Runtime) emitThinkingStep(ctx context.Context, runID string, step domain.ThoughtStep, emit func(domain.SSEEvent) error) error {
	if step.ID == "" || step.Title == "" || step.Status == "" {
		return nil
	}
	if r.store != nil && r.store.IsRunCanceled(ctx, runID) {
		return emit(domain.SSEEvent{Type: "error", RunID: runID, Code: "canceled", Message: "已停止生成"})
	}
	return emit(domain.SSEEvent{Type: "thinking_delta", RunID: runID, Step: &step})
}

func thinkingStepForStatus(stage string, text string) (domain.ThoughtStep, bool) {
	switch stage {
	case "intent":
		return domain.ThoughtStep{ID: "intent", Title: "分析用户需求", Status: "running", Order: 1}, true
	case "tool", "skill":
		return domain.ThoughtStep{ID: "retrieve", Title: "查询商品与资料", Status: "running", Summary: text, Order: 2}, true
	case "answer":
		return domain.ThoughtStep{ID: "answer", Title: "总结答案", Status: "running", Summary: text, Order: 3}, true
	default:
		return domain.ThoughtStep{}, false
	}
}

func (r *Runtime) emitNeedSummaryStep(ctx context.Context, run domain.AgentRun, query string, plan runPlan, emit func(domain.SSEEvent) error) {
	summary := r.generateNeedSummary(ctx, run, query, plan)
	if ctx.Err() != nil {
		return
	}
	if summary == "" {
		summary = fallbackNeedSummary(query, plan)
	}
	if err := r.emitThinkingStep(ctx, run.RunID, domain.ThoughtStep{
		ID:      "intent",
		Title:   "分析用户需求",
		Status:  "done",
		Summary: summary,
		Order:   1,
	}, emit); err != nil && r.logger != nil {
		r.logger.Warn("emit need summary failed", "run_id", run.RunID, "error", err)
	}
}

func (r *Runtime) generateNeedSummary(ctx context.Context, run domain.AgentRun, query string, plan runPlan) string {
	if !r.llm.Enabled() {
		return ""
	}
	startedAt := time.Now()
	temperature := 0.2
	messages := []ChatMessage{
		{Role: "system", Content: `你是电商导购的需求理解摘要器。根据用户问题和已识别意图，输出给用户看的需求理解摘要。
要求：
- 只输出一段中文，50-90字。
- 概括用户想买什么、核心偏好、预算/场景/功能/外观/风险等约束。
- 不输出内部意图名、模型思考、工具名、JSON 或列表。
- 不编造用户没有表达或上下文没有提供的信息。`},
		{Role: "user", Content: fmt.Sprintf("用户问题：%s\n路由：%s\n意图：%s\n层级：%s\n二级层级：%s\n模型分类理由：%s", query, plan.Route, plan.ReferenceIntent(), plan.Level, plan.SecondaryLevel, plan.Reasoning)},
	}
	model := r.modelForRole(ctx, modelRoleNeedSummary, r.llm.SmallModel())
	content, err := r.llm.Complete(ctx, model, messages, temperature)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("need summary fallback", "run_id", run.RunID, "error", err)
		}
		r.traceLLM(ctx, run, "thinking.need_summary", model, startedAt, err, llmPromptMetadata(messages, temperature, nil))
		return ""
	}
	r.traceLLM(ctx, run, "thinking.need_summary", model, startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{
		"raw_length": len([]rune(content)),
	}))
	return cleanNeedSummary(content)
}

func cleanNeedSummary(content string) string {
	content = strings.TrimSpace(extractJSONObject(content))
	if strings.HasPrefix(content, "{") {
		var decoded map[string]string
		if err := json.Unmarshal([]byte(content), &decoded); err == nil {
			for _, key := range []string{"summary", "content", "text"} {
				if value := strings.TrimSpace(decoded[key]); value != "" {
					content = value
					break
				}
			}
		}
	}
	content = strings.TrimSpace(strings.Trim(content, "` \n\t"))
	content = strings.TrimPrefix(content, "需求理解：")
	content = strings.TrimPrefix(content, "用户需求：")
	return truncateRunes(strings.TrimSpace(content), 110)
}

func fallbackNeedSummary(query string, plan runPlan) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "已识别当前需求，正在结合可用商品和资料继续处理。"
	}
	prefix := "已识别当前需求："
	if plan.Route == "non_guide" {
		prefix = "已识别当前服务诉求："
	}
	return truncateRunes(prefix+query, 90)
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
	default:
		return false
	}
}

func (r *Runtime) plan(ctx context.Context, run domain.AgentRun, query string) runPlan {
	return r.classifyIntent(ctx, run, query, true)
}

// classifyIntent 使用分层意图识别：
// 先判断 guide/non_guide；guide 再做 P1-P6 细分；non_guide 再按电商服务域细分。
// 加购、结算等需要强一致落库的动作仍保留确定性 intent，避免模型口头承诺但没有写业务数据。
func (r *Runtime) classifyIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool) runPlan {
	fallback := heuristicPlan(query, r.llm.SmallModel(), r.llm.LargeModel())
	if isGreeting(query) || isToolIntent(fallback.Intent) {
		return fallback
	}
	if fallback.Route == "non_guide" && fallback.Intent != "" && fallback.Intent != "unsupported" && fallback.Intent != "non_guide" {
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
	routeModel := r.modelForRole(ctx, modelRolePlannerRoute, r.llm.SmallModel())
	routeContent, err := r.llm.Complete(ctx, routeModel, messages, temperature)
	if err != nil {
		r.logger.Warn("agent route fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.route", routeModel, startedAt, err, llmPromptMetadata(messages, temperature, nil))
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.route", routeModel, startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(routeContent))}))
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
		guidePlan.AnswerModel = r.answerModelForPlan(ctx, guidePlan)
		return guidePlan
	}

	if isToolIntent(fallback.Intent) {
		plan.Intent = fallback.Intent
		plan.Route = "non_guide"
		plan.AnswerModel = r.answerModelForPlan(ctx, plan)
		return plan
	}
	nonGuidePlan := r.classifyNonGuideIntent(ctx, run, query, recordTrace, fallback)
	nonGuidePlan.Route = "non_guide"
	nonGuidePlan.AnswerModel = r.answerModelForPlan(ctx, nonGuidePlan)
	return nonGuidePlan
}

// classifyGuideIntent 只处理导购内部的细分类；非导购和固定动作不会进入这里。
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
	guideIntentModel := r.modelForRole(ctx, modelRolePlannerGuideIntent, r.llm.SmallModel())
	content, err := r.llm.Complete(ctx, guideIntentModel, messages, temperature)
	if err != nil {
		r.logger.Warn("agent guide intent fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.guide_intent", guideIntentModel, startedAt, err, llmPromptMetadata(messages, temperature, nil))
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.guide_intent", guideIntentModel, startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
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

// classifyNonGuideIntent 把非导购请求拆到具体服务域，后续按服务域装配工具和 skill。
func (r *Runtime) classifyNonGuideIntent(ctx context.Context, run domain.AgentRun, query string, recordTrace bool, fallback runPlan) runPlan {
	fallback.Intent = normalizeNonGuideIntent(query, fallback.Intent)
	startedAt := time.Now()
	temperature := 0.1
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: r.stringConfig(ctx, "agent.prompt.non_guide_intent", configcenter.DefaultNonGuideIntentPrompt),
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：reasoning、intent。",
		},
	}
	nonGuideModel := r.modelForRole(ctx, modelRolePlannerNonGuide, r.llm.SmallModel())
	content, err := r.llm.Complete(ctx, nonGuideModel, messages, temperature)
	if err != nil {
		r.logger.Warn("agent non guide intent fallback", "error", err)
		if recordTrace {
			r.traceLLM(ctx, run, "planner.non_guide_intent", nonGuideModel, startedAt, err, llmPromptMetadata(messages, temperature, nil))
		}
		return fallback
	}
	if recordTrace {
		r.traceLLM(ctx, run, "planner.non_guide_intent", nonGuideModel, startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
	}

	var plan runPlan
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &plan); err != nil {
		r.logger.Warn("agent non guide intent json fallback", "error", err, "content", content)
		return fallback
	}
	plan.Route = "non_guide"
	plan.Intent = normalizeNonGuideIntent(query, plan.Intent)
	if plan.Intent == "" {
		plan.Intent = fallback.Intent
	}
	return plan
}

// normalizeRoute 对模型输出做白名单收敛，防止 prompt 漂移导致未知 route。
func normalizeRoute(route string, fallback string) string {
	switch strings.TrimSpace(route) {
	case "guide", "non_guide":
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

// normalizeRouteIntent 为非导购固定动作补齐可执行 intent。
func normalizeRouteIntent(route string, query string, fallback string) string {
	switch route {
	case "non_guide":
		return normalizeNonGuideIntent(query, fallback)
	case "guide":
		return "open_explore"
	default:
		return fallback
	}
}

func normalizeNonGuideIntent(query string, intent string) string {
	intent = strings.TrimSpace(intent)
	switch intent {
	case "cart_add", "cart_remove", "cart_update_quantity", "checkout_confirm":
		return intent
	case "cart_service", "order_service", "coupon_service", "review_service", "after_sales_service", "account_service", "navigation_service", "chitchat", "unsupported":
		return intent
	case "non_guide", "":
	default:
		intent = ""
	}
	switch {
	case looksCartAdd(query):
		return "cart_add"
	case looksCartRemove(query):
		return "cart_remove"
	case looksCartQuantityUpdate(query):
		return "cart_update_quantity"
	case looksCheckout(query):
		return "checkout_confirm"
	case looksNavigation(query):
		return "navigation_service"
	case looksCouponService(query):
		return "coupon_service"
	case looksReviewService(query):
		return "review_service"
	case looksOrderService(query):
		return "order_service"
	case looksAfterSalesService(query):
		return "after_sales_service"
	case looksAccountService(query):
		return "account_service"
	case isGreeting(query):
		return "chitchat"
	case looksUnsupported(query):
		return "unsupported"
	case strings.Contains(query, "购物车"):
		return "cart_service"
	}
	if intent != "" {
		return intent
	}
	return "unsupported"
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
	followupsModel := r.modelForRole(ctx, modelRoleFollowups, r.llm.SmallModel())
	content, err := r.llm.Complete(ctx, followupsModel, messages, temperature)
	if err != nil {
		r.logger.Warn("agent followups fallback", "error", err)
		r.traceLLM(ctx, run, "followups", followupsModel, startedAt, err, llmPromptMetadata(messages, temperature, nil))
		return fallback
	}
	r.traceLLM(ctx, run, "followups", followupsModel, startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
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

func (r *Runtime) intConfig(ctx context.Context, key string, fallback int) int {
	r.refreshDynamicConfig(ctx)
	values := r.configs.GetMap(ctx)
	if value := strings.TrimSpace(values[key]); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
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
		plan.Intent = "chitchat"
		return plan
	}
	if looksUnsupported(query) {
		plan.Intent = "unsupported"
		return plan
	}
	if looksCartAdd(query) {
		plan.Route = "non_guide"
		plan.Intent = "cart_add"
		return plan
	}
	if looksCartRemove(query) {
		plan.Route = "non_guide"
		plan.Intent = "cart_remove"
		return plan
	}
	if looksCartQuantityUpdate(query) {
		plan.Route = "non_guide"
		plan.Intent = "cart_update_quantity"
		return plan
	}
	if looksCheckout(query) {
		plan.Route = "non_guide"
		plan.Intent = "checkout_confirm"
		return plan
	}
	if serviceIntent := normalizeNonGuideIntent(query, ""); serviceIntent != "unsupported" && serviceIntent != "non_guide" {
		plan.Route = "non_guide"
		plan.Intent = serviceIntent
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
		plan.Intent = "coupon_service"
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
		plan.Intent = "after_sales_service"
		plan.Level = ""
		plan.SecondaryLevel = ""
	}
	plan.Intent = normalizeNonGuideIntent(query, plan.Intent)
	return plan
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
	return normalized == "你好" || normalized == "您好" || normalized == "hello" || normalized == "hi" || normalized == "嗨" || normalized == "谢谢" || normalized == "谢谢你" || normalized == "你是谁"
}

func looksCatalogRelated(query string) bool {
	query = intentSurfaceQuery(query)
	keywords := []string{"推荐", "买", "商品", "手机", "鼠标", "电脑", "耳机", "价格", "预算", "对比", "比较", "售后", "退货", "保修", "优惠", "拍照", "办公", "护肤", "美妆", "服饰", "食品"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func looksCartAdd(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"加到购物车", "加入购物车", "加购物车", "放进购物车", "加购"})
}

func looksCartRemove(query string) bool {
	query = intentSurfaceQuery(query)
	return strings.Contains(query, "购物车") && containsAny(query, []string{"删除", "移除", "不要"})
}

func looksCartQuantityUpdate(query string) bool {
	query = intentSurfaceQuery(query)
	return strings.Contains(query, "购物车") && containsAny(query, []string{"数量", "改成", "改为", "调整到"})
}

func looksNavigation(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"打开", "跳转", "进入", "去", "页面", "入口"}) &&
		containsAny(query, []string{"购物车", "订单", "商品列表", "优惠券", "优惠券中心", "个人中心", "地址", "评价"})
}

func looksOrderService(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"订单", "物流", "快递", "催发货", "待支付", "付款", "支付", "取消订单", "确认收货", "发货", "取件", "取件码", "驿站", "复购"})
}

func looksCouponService(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"优惠", "优惠券", "券", "红包", "会员权益", "返利", "促销", "活动", "满减", "折扣", "领券", "卡券", "凑单", "更便宜", "省钱"})
}

func looksReviewService(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"评价", "评论", "评分", "晒单", "追评", "差评", "好评", "口碑"})
}

func looksAfterSalesService(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"售后", "退货", "退款", "保修", "换货", "投诉", "改地址", "发票", "客服", "赔付"})
}

func looksAccountService(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"账号", "账户", "登录", "注册", "手机号", "收货地址", "地址", "个人资料", "会员等级", "风险用户"})
}

func looksCheckout(query string) bool {
	query = intentSurfaceQuery(query)
	return containsAny(query, []string{"下单", "结算", "提交订单", "确认购买", "确认订单"})
}

func looksUnsupported(query string) bool {
	query = intentSurfaceQuery(query)
	keywords := []string{"论文", "破解", "绕过登录", "绕过鉴权", "脚本", "黑客", "攻击", "股票", "吃什么药", "起诉书", "代写", "美团", "外卖订单", "其他平台订单"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func intentSurfaceQuery(query string) string {
	query = strings.TrimSpace(query)
	const prefix = "当前用户问题："
	if !strings.HasPrefix(query, prefix) {
		return query
	}
	query = strings.TrimSpace(strings.TrimPrefix(query, prefix))
	for _, delimiter := range []string{"\n\n相关会话记忆：", "\n\n记忆使用规则："} {
		if idx := strings.Index(query, delimiter); idx >= 0 {
			query = query[:idx]
		}
	}
	return strings.TrimSpace(query)
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

func normalizeQueryWithAttachments(query string, attachments []domain.Attachment) string {
	trimmed := strings.TrimSpace(query)
	imageSources := imageAttachmentDescriptors(attachments)
	if len(imageSources) == 0 {
		return trimmed
	}
	if trimmed == "" || isGenericImageQuery(trimmed) {
		return "找同款 " + strings.Join(imageSources, " ")
	}
	return trimmed + "\n\n本轮附件：" + strings.Join(imageSources, " ") + "。如果用户要找同款、拍照找货、识图搜索或相似商品推荐，必须基于本轮附件调用 search_image_products。"
}

func imageAttachmentDescriptors(attachments []domain.Attachment) []string {
	out := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment.Type != "image" {
			continue
		}
		id := strings.TrimSpace(attachment.FileID)
		if id == "" {
			id = strings.TrimSpace(attachment.AttachmentID)
		}
		switch {
		case id != "":
			out = append(out, "[附件图片链接:/api/v1/files/"+id+"]")
		case strings.TrimSpace(attachment.URL) != "":
			out = append(out, "[附件图片链接:"+strings.TrimSpace(attachment.URL)+"]")
		case strings.TrimSpace(attachment.ObjectKey) != "":
			out = append(out, "[附件图片对象:"+strings.TrimSpace(attachment.ObjectKey)+"]")
		default:
			out = append(out, "[附件图片]")
		}
	}
	return out
}

func isGenericImageQuery(query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(query))
	if normalized == "" {
		return true
	}
	generic := []string{"找同款", "同款", "拍照找", "拍照搜", "图片找", "识图", "搜一下", "帮我看看", "看看", "这个", "这款", "这个呢", "这款呢", "??", "???", "？", "?"}
	for _, item := range generic {
		if normalized == item {
			return true
		}
	}
	return len([]rune(normalized)) <= 4 && !looksCartAdd(normalized) && !looksCheckout(normalized)
}

func formatQueryWithAttachments(query string, attachments []domain.Attachment) string {
	imageCount := 0
	for _, attachment := range attachments {
		if attachment.Type == "image" {
			imageCount++
		}
	}
	if imageCount == 0 {
		return query
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\n本轮附件：用户已上传 ")
	b.WriteString(strconv.Itoa(imageCount))
	b.WriteString(" 张图片。涉及拍照找货、图片找同款、识图搜索、相似商品推荐时，调用 search_image_products；不要回答 VLM 未配置。")
	return b.String()
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
