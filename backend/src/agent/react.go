package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/configcenter"
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

const maxReactSteps = 6

type reactRunResult struct {
	Observations []toolObservation
	FinalBlocks  []domain.AgentBlock
	ProductIDs   []string
	ChunkIDs     []string
	LastText     string
}

func (r *Runtime) runReactAgent(ctx context.Context, run domain.AgentRun, plan runPlan, query string, emit func(domain.SSEEvent) error) (reactRunResult, error) {
	result := reactRunResult{}
	if !r.llm.Enabled() {
		text := buildAnswer(query, plan, nil)
		if err := r.emitFallbackAnswer(ctx, run, text, emit); err != nil {
			return result, err
		}
		result.LastText = text
		return result, nil
	}

	messages := []ChatMessage{
		{Role: "system", Content: r.reactSystemPromptForPlan(ctx, plan)},
		{Role: "user", Content: reactUserPrompt(query, plan)},
	}

	parseFailures := 0
	searchProductCalls := 0
	finalAction := reactAction{Type: "final"}

	for step := 1; step <= maxReactSteps; step++ {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return result, emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		startedAt := time.Now()
		content, err := r.llm.Complete(ctx, plan.AnswerModel, messages, 0.1)
		if err != nil {
			r.logger.Warn("react step fallback", "run_id", run.RunID, "step", step, "error", err)
			r.traceLLM(ctx, run, fmt.Sprintf("react.step.%d", step), plan.AnswerModel, startedAt, err, llmPromptMetadata(messages, 0.1, map[string]any{"route": plan.Route, "intent": plan.ReferenceIntent()}))
			break
		}
		r.traceLLM(ctx, run, fmt.Sprintf("react.step.%d", step), plan.AnswerModel, startedAt, nil, llmPromptMetadata(messages, 0.1, map[string]any{
			"route":      plan.Route,
			"intent":     plan.ReferenceIntent(),
			"raw_length": len([]rune(content)),
			"raw_output": content,
		}))

		action, err := parseReactAction(content)
		if err != nil {
			parseFailures++
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: content},
				ChatMessage{Role: "user", Content: `上一轮输出不是合法 JSON。请严格输出 {"type":"tool_call",...} 或 {"type":"final",...}，不要输出 Markdown 或解释。`},
			)
			if parseFailures >= 2 {
				break
			}
			continue
		}
		parseFailures = 0
		if action.Type == "tool_call" && r.skillAllowedForPlan(ctx, plan, action.Tool) {
			action.Skill = action.Tool
			action.Tool = ""
			action.Type = "skill_call"
		}

		if action.Type == "final" {
			finalAction = action
			break
		}
		if action.Type == "skill_call" {
			if !r.skillAllowedForPlan(ctx, plan, action.Skill) {
				messages = append(messages,
					ChatMessage{Role: "assistant", Content: content},
					ChatMessage{Role: "user", Content: fmt.Sprintf("skill %q 不在当前子意图的可用 skill 列表中。请遵守“当前子意图工具策略”：需要业务入口则调用允许的 skill；否则输出 final 澄清或说明能力边界。", action.Skill)},
				)
				continue
			}
			if err := r.emitStatus(ctx, run.RunID, "skill", statusTextForSkill(action.Skill), emit); err != nil {
				return result, err
			}
			observation := r.executeSkill(ctx, run, action)
			result.Observations = append(result.Observations, observation)
			r.trace(ctx, run, "skills", action.Skill, "", traceStatus(observation.OK), observation.DurationMS, "", skillObservationForTrace(observation))
			for _, block := range observation.Blocks {
				item := block
				if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &item}); err != nil {
					return result, err
				}
			}
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: content},
				ChatMessage{Role: "user", Content: "Observation:\n" + observationForModel(observation) + "\n\n继续。需要更多信息则继续 tool_call 或 skill_call；信息足够则输出 final。"},
			)
			continue
		}
		if !r.toolAllowedForPlan(ctx, plan, action.Tool) {
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: content},
				ChatMessage{Role: "user", Content: fmt.Sprintf("工具 %q 不在当前子意图的可用工具列表中。请遵守“当前子意图工具策略”：需要更多信息则调用允许的工具；否则输出 final 澄清或说明能力边界。", action.Tool)},
			)
			continue
		}
		if action.Tool == toolSearchProducts {
			searchProductCalls++
			if searchProductCalls > 2 {
				messages = append(messages,
					ChatMessage{Role: "assistant", Content: content},
					ChatMessage{Role: "user", Content: `search_products 已连续调用过多。请基于已有 observation 输出 final。`},
				)
				continue
			}
		}

		if err := r.emitStatus(ctx, run.RunID, "tool", statusTextForTool(action.Tool), emit); err != nil {
			return result, err
		}
		observation := r.executeTool(ctx, run, action)
		result.Observations = append(result.Observations, observation)
		if observation.RelevanceStatus == "" || observation.RelevanceStatus == relevanceOK {
			result.ProductIDs = appendUnique(result.ProductIDs, observation.ProductIDs...)
		}
		result.ChunkIDs = appendUnique(result.ChunkIDs, observation.ChunkIDs...)
		r.trace(ctx, run, "tools", action.Tool, "", traceStatus(observation.OK), observation.DurationMS, "", map[string]any{
			"ok":                    observation.OK,
			"message":               observation.Message,
			"arguments":             json.RawMessage(action.Arguments),
			"result":                observation.Result,
			"observation":           observation,
			"observation_json":      observationForModel(observation),
			"product_ids":           observation.ProductIDs,
			"candidate_product_ids": observation.CandidateProductIDs,
			"dropped_product_ids":   observation.DroppedProductIDs,
			"relevance_status":      observation.RelevanceStatus,
			"relevance_reason":      observation.RelevanceReason,
			"chunk_ids":             observation.ChunkIDs,
			"result_item_count":     resultItemCount(observation.Result),
			"result_truncated":      false,
			"admin_visible_note":    "工具结果完整写入 trace，管理员页面可直接排查检索片段、弱相关候选、剔除原因和工具返回。",
		})
		if observation.Cart != nil {
			if err := emitCartBlock(run.RunID, *observation.Cart, emit); err != nil {
				return result, err
			}
		}
		if len(observation.Orders) > 0 {
			block := domain.AgentBlock{Type: "order_summary", Orders: observation.Orders}
			if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block}); err != nil {
				return result, err
			}
		}

		messages = append(messages,
			ChatMessage{Role: "assistant", Content: content},
			ChatMessage{Role: "user", Content: "Observation:\n" + observationForModel(observation) + "\n\n继续。需要更多信息则继续 tool_call 或 skill_call；信息足够则输出 final。"},
		)
	}

	result.FinalBlocks = blocksFromReact(finalAction, result.ProductIDs, result.ChunkIDs)
	if err := r.streamReactFinal(ctx, run, plan, query, messages, finalAction, result, emit); err != nil {
		return result, err
	}
	return result, nil
}

func (r *Runtime) streamReactFinal(ctx context.Context, run domain.AgentRun, plan runPlan, query string, messages []ChatMessage, finalAction reactAction, result reactRunResult, emit func(domain.SSEEvent) error) error {
	if err := r.emitStatus(ctx, run.RunID, "answer", "正在生成回答", emit); err != nil {
		return err
	}
	finalInstruction := map[string]any{
		"user_query":                query,
		"route":                     plan.Route,
		"intent":                    plan.ReferenceIntent(),
		"final_hint":                finalAction.Text,
		"final_allowed_product_ids": result.ProductIDs,
		"product_ids":               result.ProductIDs,
		"chunk_ids":                 result.ChunkIDs,
		"observations":              compactObservations(result.Observations),
	}
	payload, _ := json.Marshal(finalInstruction)
	streamMessages := []ChatMessage{
		{Role: "system", Content: r.finalSystemPromptForPlan(ctx, plan)},
	}
	for _, message := range messages[1:] {
		streamMessages = append(streamMessages, message)
	}
	streamMessages = append(streamMessages,
		ChatMessage{
			Role:    "user",
			Content: string(payload),
		},
	)

	startedAt := time.Now()
	var content strings.Builder
	var rawContent strings.Builder
	var filterErr error
	filter := newStreamTextFilter(result.ProductIDs, func(productID string) {
		if filterErr != nil {
			return
		}
		product, ok := r.store.GetProduct(ctx, productID)
		if !ok {
			return
		}
		card := product.ProductCard
		part := domain.AgentBlock{Type: "product_card", Product: &card}
		filterErr = emit(domain.SSEEvent{Type: "content_delta", RunID: run.RunID, Part: &part})
	})
	err := r.llm.Stream(ctx, plan.AnswerModel, streamMessages, 0.4, func(delta string) error {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		rawContent.WriteString(delta)
		clean := filter.Clean(delta)
		if filterErr != nil {
			return filterErr
		}
		if clean == "" {
			return nil
		}
		content.WriteString(clean)
		if err := emit(domain.SSEEvent{Type: "content_delta", RunID: run.RunID, Part: &domain.AgentBlock{Type: "text", Content: clean}}); err != nil {
			return err
		}
		return emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: clean})
	})
	if err != nil {
		r.logger.Warn("react final stream fallback", "run_id", run.RunID, "error", err, "model", plan.AnswerModel)
		r.traceLLM(ctx, run, "react.final", plan.AnswerModel, startedAt, err, llmPromptMetadata(streamMessages, 0.4, map[string]any{"route": plan.Route, "intent": plan.ReferenceIntent()}))
		if content.Len() > 0 {
			return r.emitReactFinalBlocks(run.RunID, result.FinalBlocks, emit)
		}
		if strings.TrimSpace(finalAction.Text) != "" {
			if err := r.emitFallbackAnswer(ctx, run, finalAction.Text, emit); err != nil {
				return err
			}
			return r.emitReactFinalBlocks(run.RunID, result.FinalBlocks, emit)
		}
		if err := r.emitFallbackAnswer(ctx, run, buildAnswer(query, plan, nil), emit); err != nil {
			return err
		}
		return r.emitReactFinalBlocks(run.RunID, result.FinalBlocks, emit)
	}
	r.traceLLM(ctx, run, "react.final", plan.AnswerModel, startedAt, nil, llmPromptMetadata(streamMessages, 0.4, map[string]any{
		"route":                     plan.Route,
		"intent":                    plan.ReferenceIntent(),
		"raw_length":                len([]rune(rawContent.String())),
		"raw_output":                rawContent.String(),
		"filtered_length":           len([]rune(content.String())),
		"filtered_output":           content.String(),
		"stream_filter_used":        true,
		"final_allowed_product_ids": result.ProductIDs,
	}))
	return r.emitReactFinalBlocks(run.RunID, result.FinalBlocks, emit)
}

func (r *Runtime) emitReactFinalBlocks(runID string, blocks []domain.AgentBlock, emit func(domain.SSEEvent) error) error {
	for i := range blocks {
		block := blocks[i]
		if block.Type == "" {
			continue
		}
		r.logger.Debug("agent final block", "run_id", runID, "type", block.Type)
		if err := emit(domain.SSEEvent{Type: "block_delta", RunID: runID, Block: &block}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) reactSystemPromptForPlan(ctx context.Context, plan runPlan) string {
	parts := []string{
		r.stringConfig(ctx, "agent.prompt.answer_base", configcenter.DefaultAnswerBasePrompt),
		r.stringConfig(ctx, "agent.prompt.tool_protocol", configcenter.DefaultToolProtocolPrompt),
	}
	intent := plan.ReferenceIntent()
	if prompt := r.stringConfig(ctx, "agent.prompt.intent."+intent, configcenter.DefaultIntentPrompt(intent)); prompt != "" {
		parts = append(parts, prompt)
	}
	parts = append(parts, r.intentToolPolicyPrompt(ctx, plan))
	return strings.Join(parts, "\n\n")
}

func (r *Runtime) finalSystemPromptForPlan(ctx context.Context, plan runPlan) string {
	parts := []string{
		r.stringConfig(ctx, "agent.prompt.answer_base", configcenter.DefaultAnswerBasePrompt),
	}
	intent := plan.ReferenceIntent()
	if prompt := r.stringConfig(ctx, "agent.prompt.intent."+intent, configcenter.DefaultIntentPrompt(intent)); prompt != "" {
		parts = append(parts, prompt)
	}
	return strings.Join(parts, "\n\n")
}

func reactUserPrompt(query string, plan runPlan) string {
	return fmt.Sprintf(`用户问题：%s

一级路由：%s
意图：%s
层级：%s
二级层级：%s

请按工具协议输出下一步 JSON。`, query, plan.Route, plan.ReferenceIntent(), plan.Level, plan.SecondaryLevel)
}

func statusTextForTool(tool string) string {
	switch tool {
	case toolSearchProducts:
		return "正在检索商品"
	case toolSearchKnowledge:
		return "正在检索资料"
	case toolGetCart:
		return "正在读取购物车"
	case toolAddCartItem:
		return "正在加入购物车"
	case toolUpdateCartItem:
		return "正在更新购物车"
	case toolDeleteCartItem:
		return "正在删除购物车商品"
	case toolCheckout:
		return "正在提交订单"
	default:
		return "正在调用工具"
	}
}

func blocksFromReact(action reactAction, productIDs []string, chunkIDs []string) []domain.AgentBlock {
	blocks := make([]domain.AgentBlock, 0, len(action.Blocks)+2)
	for _, block := range action.Blocks {
		switch block.Type {
		case "product_refs":
			ids := appendUnique(nil, block.ProductIDs...)
			if len(ids) > 0 {
				blocks = append(blocks, domain.AgentBlock{Type: "product_refs", ProductIDs: ids})
			}
		case "citation_refs":
			ids := appendUnique(nil, block.ChunkIDs...)
			if len(ids) > 0 {
				blocks = append(blocks, domain.AgentBlock{Type: "citation_refs", ChunkIDs: ids})
			}
		}
	}
	if len(chunkIDs) > 0 {
		hasCitationRefs := false
		for _, block := range blocks {
			if block.Type == "citation_refs" {
				hasCitationRefs = true
				break
			}
		}
		if !hasCitationRefs {
			blocks = append(blocks, domain.AgentBlock{Type: "citation_refs", ChunkIDs: appendUnique(nil, chunkIDs...)})
		}
	}
	if len(productIDs) > 0 {
		hasProductRefs := false
		for _, block := range blocks {
			if block.Type == "product_refs" {
				hasProductRefs = true
				break
			}
		}
		if !hasProductRefs {
			blocks = append(blocks, domain.AgentBlock{Type: "product_refs", ProductIDs: appendUnique(nil, productIDs...)})
		}
	}
	return blocks
}

func compactObservations(observations []toolObservation) []map[string]any {
	items := make([]map[string]any, 0, len(observations))
	for _, observation := range observations {
		items = append(items, map[string]any{
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
		})
	}
	return items
}

func resultItemCount(result map[string]any) int {
	if result == nil {
		return 0
	}
	items, ok := result["items"].([]map[string]any)
	if ok {
		return len(items)
	}
	if rawItems, ok := result["items"].([]any); ok {
		return len(rawItems)
	}
	if orders, ok := result["orders"].([]map[string]any); ok {
		return len(orders)
	}
	return 0
}

func appendUnique(base []string, values ...string) []string {
	seen := make(map[string]bool, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, value := range base {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func traceStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}

type streamTextFilter struct {
	inTag           bool
	tagBuffer       strings.Builder
	itemBuffer      strings.Builder
	inItem          bool
	inBuyer         bool
	allowedItemIDs  map[string]bool
	emittedItemIDs  map[string]bool
	onItem          func(string)
	pendingItemText string
	fenceCarry      string
}

func newStreamTextFilter(allowedProductIDs []string, onItem ...func(string)) *streamTextFilter {
	allowed := make(map[string]bool, len(allowedProductIDs))
	for _, id := range allowedProductIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = true
		}
	}
	var callback func(string)
	if len(onItem) > 0 {
		callback = onItem[0]
	}
	return &streamTextFilter{allowedItemIDs: allowed, emittedItemIDs: map[string]bool{}, onItem: callback}
}

func (f *streamTextFilter) Clean(delta string) string {
	var builder strings.Builder
	if f.pendingItemText != "" {
		builder.WriteString(f.pendingItemText)
		f.pendingItemText = ""
	}
	for _, item := range delta {
		switch {
		case item == '<':
			f.inTag = true
			f.tagBuffer.Reset()
			continue
		case f.inTag && item == '>':
			f.inTag = false
			tag := strings.ToLower(strings.TrimSpace(f.tagBuffer.String()))
			switch tag {
			case "item":
				f.inItem = true
				f.itemBuffer.Reset()
			case "/item":
				id := strings.TrimSpace(f.itemBuffer.String())
				if f.allowedItemIDs[id] && !f.emittedItemIDs[id] {
					f.emittedItemIDs[id] = true
					if f.onItem != nil {
						f.onItem(id)
					}
				}
				f.inItem = false
				f.itemBuffer.Reset()
			case "buyer":
				f.inBuyer = true
			case "/buyer":
				f.inBuyer = false
			}
			continue
		case f.inTag:
			f.tagBuffer.WriteRune(item)
			continue
		case f.inItem:
			f.itemBuffer.WriteRune(item)
			continue
		case f.inBuyer:
			continue
		default:
			builder.WriteRune(item)
		}
	}
	clean := builder.String()
	clean = strings.ReplaceAll(clean, "special_word", "")
	clean = strings.ReplaceAll(clean, "special word", "")
	clean = strings.ReplaceAll(clean, "special", "")
	clean = strings.ReplaceAll(clean, "_word", "")
	clean = strings.ReplaceAll(clean, "（）", "")
	clean = strings.ReplaceAll(clean, "()", "")
	return f.stripMarkdownFenceLines(clean)
}

func (f *streamTextFilter) stripMarkdownFenceLines(text string) string {
	if text == "" && f.fenceCarry == "" {
		return ""
	}
	text = f.fenceCarry + text
	f.fenceCarry = ""
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && !strings.HasSuffix(text, "\n") {
		last := lines[len(lines)-1]
		if isMarkdownFencePrefix(strings.TrimSpace(last)) {
			f.fenceCarry = last
			lines = lines[:len(lines)-1]
		}
	}
	var builder strings.Builder
	for _, line := range lines {
		if isMarkdownFenceLine(strings.TrimSpace(line)) {
			continue
		}
		builder.WriteString(line)
	}
	return builder.String()
}

func isMarkdownFenceLine(line string) bool {
	if line == "" {
		return false
	}
	if line == "```" {
		return true
	}
	if strings.HasPrefix(line, "```") {
		lang := strings.TrimSpace(strings.TrimPrefix(line, "```"))
		return lang == "markdown" || lang == "md"
	}
	return false
}

func isMarkdownFencePrefix(line string) bool {
	if line == "" {
		return false
	}
	for _, fence := range []string{"```", "```markdown", "```md"} {
		if strings.HasPrefix(fence, line) {
			return true
		}
	}
	return isMarkdownFenceLine(line)
}
