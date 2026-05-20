package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Runtime struct {
	store  store.Store
	logger *slog.Logger
	llm    *LLMClient
}

func NewRuntime(store store.Store, logger *slog.Logger, config RuntimeConfig) *Runtime {
	return &Runtime{store: store, logger: logger, llm: NewLLMClient(config.Models)}
}

func (r *Runtime) Stream(ctx context.Context, run domain.AgentRun, message domain.UserMessage, emit func(domain.SSEEvent) error) error {
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

	if err := r.emitStatus(ctx, run.RunID, "intent", "正在理解你的需求", emit); err != nil {
		return err
	}
	plan := r.plan(ctx, run, message.Content)
	r.logger.Info("agent plan selected", "run_id", run.RunID, "intent", plan.Intent, "answer_model", plan.AnswerModel)
	r.trace(ctx, run, "planner", "selected", plan.AnswerModel, "ok", 0, "", map[string]any{
		"intent": plan.Intent,
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
	Intent      string `json:"intent"`
	AnswerModel string `json:"-"`
}

func (p runPlan) UsesCatalog() bool {
	switch p.Intent {
	case "product_recommendation", "product_comparison", "product_detail_qa", "promotion_rule_qa", "after_sales_qa", "shopping_decision_support":
		return true
	default:
		return false
	}
}

func (r *Runtime) plan(ctx context.Context, run domain.AgentRun, query string) runPlan {
	fallback := heuristicPlan(query, r.llm.SmallModel(), r.llm.LargeModel())
	if !r.llm.Enabled() {
		return fallback
	}

	startedAt := time.Now()
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), []ChatMessage{
		{
			Role:    "system",
			Content: "你是电商导购 Agent 的意图识别器。只判断用户请求的主意图，只输出 JSON，不要解释。intent 只能是 product_recommendation、product_comparison、product_detail_qa、promotion_rule_qa、after_sales_qa、shopping_decision_support、general_shopping_chat、unsupported。",
		},
		{
			Role:    "user",
			Content: "用户问题：" + query + "\n只输出字段：intent。",
		},
	}, 0.1)
	if err != nil {
		r.logger.Warn("agent planner fallback", "error", err)
		r.traceLLM(ctx, run, "planner", r.llm.SmallModel(), startedAt, err, nil)
		return fallback
	}
	r.traceLLM(ctx, run, "planner", r.llm.SmallModel(), startedAt, nil, map[string]any{"raw_length": len([]rune(content))})

	var plan runPlan
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &plan); err != nil {
		r.logger.Warn("agent planner json fallback", "error", err, "content", content)
		return fallback
	}
	if plan.Intent == "" {
		plan.Intent = fallback.Intent
	}
	plan.AnswerModel = r.answerModelForIntent(plan.Intent)
	return plan
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
			Content: systemPromptForIntent(plan.Intent),
		},
		{
			Role: "user",
			Content: fmt.Sprintf("用户问题：%s\n\n意图：%s\n\n候选商品：\n%s\n\n资料片段：\n%s",
				query,
				plan.Intent,
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
		r.traceLLM(ctx, run, "answer", plan.AnswerModel, startedAt, err, map[string]any{"intent": plan.Intent})
		if content.Len() > 0 {
			return nil
		}
		return r.emitFallbackAnswer(ctx, run, buildAnswer(query, plan, products), emit)
	}
	r.traceLLM(ctx, run, "answer", plan.AnswerModel, startedAt, nil, map[string]any{"intent": plan.Intent, "raw_length": len([]rune(content.String()))})
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
	if !r.llm.Enabled() {
		return fallback
	}
	startedAt := time.Now()
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), []ChatMessage{
		{Role: "system", Content: "你是电商导购追问生成器。只输出 JSON 数组，包含 2 个简短中文追问。"},
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

func heuristicPlan(query string, smallModel string, largeModel string) runPlan {
	plan := runPlan{
		Intent:      "general_shopping_chat",
		AnswerModel: smallModel,
	}
	if isGreeting(query) {
		return plan
	}
	if looksUnsupported(query) {
		plan.Intent = "unsupported"
		return plan
	}
	if looksCatalogRelated(query) {
		plan.Intent = "product_recommendation"
	}
	if strings.Contains(query, "对比") || strings.Contains(query, "比较") || strings.Contains(query, "哪个") || strings.Contains(query, "推荐") {
		plan.Intent = "shopping_decision_support"
		plan.AnswerModel = largeModel
	}
	if strings.Contains(query, "售后") || strings.Contains(query, "退货") || strings.Contains(query, "保修") {
		plan.Intent = "after_sales_qa"
	}
	return plan
}

func (r *Runtime) answerModelForIntent(intent string) string {
	switch intent {
	case "product_comparison", "shopping_decision_support":
		return r.llm.LargeModel()
	default:
		return r.llm.SmallModel()
	}
}

func systemPromptForIntent(intent string) string {
	base := []string{
		"你是小猪小狗电商平台的 AI 导购主 Agent。",
		"你可以在内部使用 ReAct 思路判断下一步，但不要输出推理过程、Action、Observation 或工具调用文本。",
		"请自己判断候选商品和资料片段是否与用户问题相关；不相关时不要强行推荐或引用。",
		"不要编造不存在的价格、库存、优惠、售后承诺或商品能力。",
		"输出中文，先给结论，再给理由、风险提示和需要补充的信息。",
		"不要输出 JSON，不要输出 markdown 表格。",
	}
	intentPrompts := map[string]string{
		"product_recommendation":    "当前意图是商品推荐。重点根据预算、使用场景、人群约束和商品风险给出 1 个优先候选；若候选不足，先澄清。",
		"product_comparison":        "当前意图是商品对比。重点比较用户关心的维度，指出每个候选适合/不适合的人群，并给出明确选择建议。",
		"product_detail_qa":         "当前意图是商品详情问答。只回答商品资料中能支持的信息；资料不足时明确说明不能确认。",
		"promotion_rule_qa":         "当前意图是优惠促销咨询。没有明确促销资料时，必须说明无法确认优惠，不要暗示某商品正在优惠。",
		"after_sales_qa":            "当前意图是售后咨询。没有明确售后规则时，必须说明需要以平台/商家规则为准，并给出用户下单前应确认的问题。",
		"shopping_decision_support": "当前意图是购物决策支持。重点权衡多目标约束，给出稳妥方案、取舍理由和踩坑风险。",
		"general_shopping_chat":     "当前意图是普通购物闲聊。简短回应，并引导用户提供品类、预算、使用场景或候选商品。",
		"unsupported":               "当前意图不属于购物导购。礼貌拒绝非购物任务，并把用户引导回购物相关问题。",
	}
	if prompt, ok := intentPrompts[intent]; ok {
		base = append(base, prompt)
	}
	return strings.Join(base, "\n")
}

func buildAnswer(query string, plan runPlan, products []domain.ProductCard) string {
	if plan.Intent == "unsupported" {
		return "这个请求不属于购物导购范围，我不能帮你完成。你可以告诉我想买的商品、预算、使用场景或想对比的候选商品，我会继续帮你筛选。"
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

func isGreeting(query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(query))
	return normalized == "你好" || normalized == "您好" || normalized == "hello" || normalized == "hi" || normalized == "嗨"
}

func looksCatalogRelated(query string) bool {
	keywords := []string{"推荐", "买", "商品", "手机", "鼠标", "电脑", "耳机", "价格", "预算", "对比", "比较", "售后", "退货", "保修", "优惠", "拍照", "办公"}
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			return true
		}
	}
	return false
}

func looksUnsupported(query string) bool {
	keywords := []string{"论文", "破解", "股票", "吃什么药", "起诉书", "代写"}
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
		lines = append(lines, fmt.Sprintf("%d. 商品ID=%s 名称=%s 品牌=%s 价格=%s 库存=%s 卖点=%s 风险=%s",
			i+1,
			product.ProductID,
			product.Name,
			product.Brand,
			product.Price,
			product.StockStatus,
			strings.Join(product.SellingPoints, "；"),
			strings.Join(product.RiskNotes, "；"),
		))
	}
	return strings.Join(lines, "\n")
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
