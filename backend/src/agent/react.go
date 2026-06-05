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

type reactStepOutput struct {
	Action     reactAction
	FinalText  string
	RawOutput  string
	Streamed   bool
	FinalFound bool
}

func (r *Runtime) runReactAgent(ctx context.Context, run domain.AgentRun, plan runPlan, query string, attachments []domain.Attachment, emit func(domain.SSEEvent) error) (reactRunResult, error) {
	// runReactAgent 是单轮 Agent 的主执行器。它不负责前置意图识别；进入这里时，
	// plan 已经由 planner 决定好 route / intent / model。函数内部再完成：
	// 1. ReAct 决策循环：让模型决定调用哪些工具或 skill；
	// 2. 工具执行与 observation 回填：把检索、购物车、订单等结果写回上下文；
	// 3. 最终输出：最后一轮用 <final>...</final> 协议直接流式输出用户文案。
	result := reactRunResult{}
	if !r.llm.Enabled() {
		// 没有配置模型 key 时走兜底回答，保证本地开发或模型故障时接口仍可返回。
		// 这条路径不会调用工具，也不会产生完整 ReAct trace。
		text := buildAnswer(query, plan, nil)
		if err := r.emitFallbackAnswer(ctx, run, text, emit); err != nil {
			return result, err
		}
		result.LastText = text
		return result, nil
	}

	// ReAct 决策阶段的上下文。system prompt 由基础回答规范、工具协议、子意图 prompt
	// 和当前子意图允许的工具/skill 策略动态拼接；user prompt 只放本轮 query 与意图信息。
	messages := []ChatMessage{
		{Role: "system", Content: r.reactSystemPromptForPlan(ctx, plan)},
		{Role: "user", Content: reactUserPrompt(query, plan)},
	}

	// parseFailures 用于处理模型偶发输出非 JSON；searchProductCalls 防止检索工具在同一轮
	// 里被重复调用太多次。finalAction 先放一个空 final，避免循环异常退出时没有成文入口。
	parseFailures := 0
	searchProductCalls := 0
	finalAction := reactAction{Type: "final"}
	allowedAddProductIDs := productIDSetFromText(query)

	// ReAct 决策循环。
	// 这里的每一轮 trace 记为 react.step.N，模型输出有两种合法形态：
	// - tool_call / skill_call：继续调用工具或 skill，拿到 observation 后进入下一轮；
	// - <final>...</final>：信息足够，标签内内容直接作为最终回答真流式发给前端。
	// 这套自定义协议避免了第二次 react.final 大模型调用，同时保留最终回答的 token 级流式体验。
	for step := 1; step <= maxReactSteps; step++ {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return result, emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		startedAt := time.Now()
		// 决策轮也使用 Stream：如果模型输出 JSON，后端先缓冲完整 JSON 再解析；
		// 如果模型输出 <final>，后端立即进入正文转发，做到单次大模型调用内的最终真流式。
		stepOutput, err := r.streamReactStep(ctx, run, plan, step, messages, result.ProductIDs, emit)
		if err != nil {
			r.logger.Warn("react step fallback", "run_id", run.RunID, "step", step, "error", err)
			r.traceLLM(ctx, run, fmt.Sprintf("react.step.%d", step), plan.AnswerModel, startedAt, err, llmPromptMetadata(messages, 0.1, map[string]any{"route": plan.Route, "intent": plan.ReferenceIntent(), "protocol": "json_or_final_tag"}))
			break
		}
		// 每一轮原始模型输出都写入 trace。管理员页面可据此判断模型是选错工具、
		// 参数写错，还是过早进入 final。
		r.traceLLM(ctx, run, fmt.Sprintf("react.step.%d", step), plan.AnswerModel, startedAt, nil, llmPromptMetadata(messages, 0.1, map[string]any{
			"route":       plan.Route,
			"intent":      plan.ReferenceIntent(),
			"protocol":    "json_or_final_tag",
			"raw_length":  len([]rune(stepOutput.RawOutput)),
			"raw_output":  stepOutput.RawOutput,
			"streamed":    stepOutput.Streamed,
			"final_found": stepOutput.FinalFound,
		}))

		if stepOutput.FinalFound {
			finalAction = reactAction{Type: "final"}
			result.LastText = stepOutput.FinalText
			break
		}

		action, err := parseReactAction(stepOutput.RawOutput)
		if err != nil {
			// 模型没有按协议输出 JSON 时，把错误输出作为 assistant 历史保留，再追加一条
			// 明确纠偏指令。连续两次失败就跳出，避免一次请求无限消耗模型调用。
			parseFailures++
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
				ChatMessage{Role: "user", Content: `上一轮输出不是合法协议。需要工具时严格输出 {"type":"tool_call",...} 或 {"type":"skill_call",...}；信息足够时输出 <final>最终回答</final>。不要把最终回答放进 JSON。`},
			)
			if parseFailures >= 2 {
				break
			}
			continue
		}
		parseFailures = 0
		if action.Type == "tool_call" && r.skillAllowedForPlan(ctx, plan, action.Tool) {
			// 兼容模型把 skill 名误写到 tool 字段里的情况。只要当前子意图允许该 skill，
			// 就自动归一化为 skill_call，降低 prompt 小偏差导致的失败率。
			action.Skill = action.Tool
			action.Tool = ""
			action.Type = "skill_call"
		}

		if action.Type == "final" {
			// final 是 ReAct 循环的结束信号，同时 text 字段也是最终用户文案。
			// 后续不会再调用第二次 LLM，只会进入 emitReactFinalText 做格式校验、
			// <buyer> 清理、<item> 白名单过滤和 product_card 事件输出。
			finalAction = action
			break
		}
		if action.Type == "skill_call" {
			// skill 是固定业务流程入口，例如跳转购物车、订单页、售后说明等。
			// 它和 tool 的区别是：skill 更偏“动作/页面入口”，tool 更偏“数据查询或写操作”。
			if !r.skillAllowedForPlan(ctx, plan, action.Skill) {
				messages = append(messages,
					ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
					ChatMessage{Role: "user", Content: fmt.Sprintf("skill %q 不在当前子意图的可用 skill 列表中。请遵守“当前子意图工具策略”：需要业务入口则调用允许的 skill；否则输出 final 澄清或说明能力边界。", action.Skill)},
				)
				continue
			}
			if err := r.emitStatus(ctx, run.RunID, "skill", statusTextForSkill(action.Skill), emit); err != nil {
				return result, err
			}
			observation := r.executeSkill(ctx, run, action)
			result.Observations = append(result.Observations, observation)
			// skill 的结果也写入 trace，方便管理员看到模型为什么触发了页面入口或固定流程。
			r.trace(ctx, run, "skills", action.Skill, "", traceStatus(observation.OK), observation.DurationMS, "", skillObservationForTrace(observation))
			for _, block := range observation.Blocks {
				// 部分 skill 会直接产出结构化 block，例如导航提示或订单摘要。
				// 这些 block 不需要等最终成文，直接以 block_delta 发给前端。
				item := block
				if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &item}); err != nil {
					return result, err
				}
			}
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
				// observation 回填给模型，下一轮它才能基于刚才的执行结果决定继续调用工具
				// 还是输出 final。这里保留 assistant 的原 JSON 输出，便于模型理解上下文轨迹。
				ChatMessage{Role: "user", Content: "Observation:\n" + observationForModel(observation) + "\n\n继续。需要更多信息则继续输出 tool_call 或 skill_call JSON；信息足够则输出 <final>最终回答</final>。"},
			)
			continue
		}
		if !r.toolAllowedForPlan(ctx, plan, action.Tool) {
			// 子意图工具策略是硬约束：模型不能绕过策略调用无关工具。
			// 发现不允许的工具时，不执行工具，只把错误反馈给模型让它重新决策。
			messages = append(messages,
				ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
				ChatMessage{Role: "user", Content: fmt.Sprintf("工具 %q 不在当前子意图的可用工具列表中。请遵守“当前子意图工具策略”：需要更多信息则调用允许的工具；否则输出 final 澄清或说明能力边界。", action.Tool)},
			)
			continue
		}
		if action.Tool == toolSearchProducts {
			// search_products 是最容易被模型反复调用的工具；这里给单轮请求加上上限。
			// 超过上限后强制要求基于已有 observation 成文或澄清，避免请求耗时失控。
			searchProductCalls++
			if searchProductCalls > 2 {
				messages = append(messages,
					ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
					ChatMessage{Role: "user", Content: `search_products 已连续调用过多。请基于已有 observation 输出 <final>最终回答</final>。`},
				)
				continue
			}
		}

		if err := r.emitStatus(ctx, run.RunID, "tool", statusTextForTool(action.Tool), emit); err != nil {
			return result, err
		}
		// 真正执行工具。executeTool 内部会根据 tool 名走商品检索、知识检索、购物车、
		// 订单等后端能力，并统一返回 toolObservation。
		observation := r.executeTool(ctx, run, action, toolExecutionContext{
			AllowedAddProductIDs: allowedAddProductIDs,
			Attachments:          attachments,
		})
		result.Observations = append(result.Observations, observation)
		if observation.RelevanceStatus == "" || observation.RelevanceStatus == relevanceOK {
			// 只有相关性通过的商品 ID 才进入最终允许挂品列表。
			// 后续 <item> 过滤器会用这个白名单拦截模型编造或弱相关商品。
			result.ProductIDs = appendUnique(result.ProductIDs, observation.ProductIDs...)
			if action.Tool == toolSearchProducts {
				addProductIDsToSet(allowedAddProductIDs, observation.ProductIDs...)
			}
		}
		result.ChunkIDs = appendUnique(result.ChunkIDs, observation.ChunkIDs...)
		// 工具 observation 尽量完整写入 trace：包含候选、剔除项、相关性原因和原始结果。
		// 这样排查“召回不准/模型没用工具结果/挂了无关商品”时，不需要重新复现请求。
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
		if err := r.emitThinkingStep(ctx, run.RunID, domain.ThoughtStep{
			ID:      "retrieve",
			Title:   "查询商品与资料",
			Status:  "done",
			Summary: thinkingSummaryForToolObservation(observation),
			Order:   2,
		}, emit); err != nil {
			return result, err
		}
		if observation.Cart != nil {
			// 购物车类工具的结构化结果即时发给前端，让页面能直接刷新购物车状态。
			if err := emitCartBlock(run.RunID, *observation.Cart, emit); err != nil {
				return result, err
			}
		}
		if len(observation.Orders) > 0 {
			// 订单类工具同样即时输出结构化 block，不依赖最终自然语言回答。
			block := domain.AgentBlock{Type: "order_summary", Orders: observation.Orders}
			if err := emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block}); err != nil {
				return result, err
			}
		}

		messages = append(messages,
			ChatMessage{Role: "assistant", Content: stepOutput.RawOutput},
			// 把工具结果回填给下一轮模型。注意 observationForModel 是给模型看的压缩 JSON，
			// trace 里仍然保留更完整的 observation，便于后台排查。
			ChatMessage{Role: "user", Content: "Observation:\n" + observationForModel(observation) + "\n\n继续。需要更多信息则继续输出 tool_call 或 skill_call JSON；信息足够则输出 <final>最终回答</final>。"},
		)
	}

	result.FinalBlocks = blocksFromReact(finalAction, result.ProductIDs, result.ChunkIDs)
	// finalAction.blocks 是模型在决策阶段声明的引用块；result.ProductIDs / ChunkIDs 是工具
	// 实际返回并通过校验的引用。blocksFromReact 会把二者合并成最终结构化引用。
	if result.LastText == "" {
		if err := r.emitReactFinalText(ctx, run, plan, query, finalAction, result, emit); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r *Runtime) streamReactStep(ctx context.Context, run domain.AgentRun, plan runPlan, step int, messages []ChatMessage, allowedProductIDs []string, emit func(domain.SSEEvent) error) (reactStepOutput, error) {
	output := reactStepOutput{}
	var raw strings.Builder
	var finalText strings.Builder
	var prefix strings.Builder
	var finalPending strings.Builder
	var filterErr error
	inFinal := false
	doneFinal := false

	filter := r.newAgentOutputFilter(ctx, run, allowedProductIDs, emit, &filterErr)

	err := r.llm.Stream(ctx, plan.AnswerModel, messages, 0.1, func(delta string) error {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		raw.WriteString(delta)
		if doneFinal {
			return nil
		}
		if !inFinal {
			prefix.WriteString(delta)
			text := prefix.String()
			if idx := strings.Index(text, "<final>"); idx >= 0 {
				inFinal = true
				output.FinalFound = true
				if err := r.emitStatus(ctx, run.RunID, "answer", "正在生成回答", emit); err != nil {
					return err
				}
				return r.forwardFinalDelta(run, text[idx+len("<final>"):], &finalText, &finalPending, filter, emit, &doneFinal, &filterErr)
			}
			trimmed := strings.TrimLeft(text, "\ufeff \t\r\n")
			if strings.HasPrefix(trimmed, "{") || (len([]rune(trimmed)) > 32 && !strings.HasPrefix("<final>", trimmed)) {
				return nil
			}
			return nil
		}
		return r.forwardFinalDelta(run, delta, &finalText, &finalPending, filter, emit, &doneFinal, &filterErr)
	})
	if err != nil {
		return output, err
	}
	if filterErr != nil {
		return output, filterErr
	}
	if inFinal && !doneFinal && finalPending.Len() > 0 {
		if err := r.emitCleanFinalText(run, finalPending.String(), &finalText, filter, emit, &filterErr); err != nil {
			return output, err
		}
	}
	output.RawOutput = raw.String()
	output.FinalText = strings.TrimSpace(finalText.String())
	output.Streamed = output.FinalFound
	return output, nil
}

func (r *Runtime) forwardFinalDelta(run domain.AgentRun, delta string, finalText *strings.Builder, pending *strings.Builder, filter *streamTextFilter, emit func(domain.SSEEvent) error, doneFinal *bool, filterErr *error) error {
	if delta == "" || *doneFinal {
		return nil
	}
	const closeTag = "</final>"
	pending.WriteString(delta)
	buffer := pending.String()
	if idx := strings.Index(buffer, closeTag); idx >= 0 {
		*doneFinal = true
		pending.Reset()
		return r.emitCleanFinalText(run, buffer[:idx], finalText, filter, emit, filterErr)
	}
	keep := len(closeTag) - 1
	runes := []rune(buffer)
	if len(runes) <= keep {
		return nil
	}
	emitText := string(runes[:len(runes)-keep])
	pending.Reset()
	pending.WriteString(string(runes[len(runes)-keep:]))
	return r.emitCleanFinalText(run, emitText, finalText, filter, emit, filterErr)
}

func (r *Runtime) emitCleanFinalText(run domain.AgentRun, text string, finalText *strings.Builder, filter *streamTextFilter, emit func(domain.SSEEvent) error, filterErr *error) error {
	if text == "" {
		return nil
	}
	clean := filter.Clean(text)
	if *filterErr != nil {
		return *filterErr
	}
	if clean == "" {
		return nil
	}
	finalText.WriteString(clean)
	if err := emit(domain.SSEEvent{Type: "content_delta", RunID: run.RunID, Part: &domain.AgentBlock{Type: "text", Content: clean}}); err != nil {
		return err
	}
	return emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: clean})
}

func (r *Runtime) emitReactFinalText(ctx context.Context, run domain.AgentRun, plan runPlan, query string, finalAction reactAction, result reactRunResult, emit func(domain.SSEEvent) error) error {
	if err := r.emitStatus(ctx, run.RunID, "answer", "正在整理回答", emit); err != nil {
		return err
	}
	startedAt := time.Now()
	rawText := strings.TrimSpace(finalAction.Text)
	fallbackUsed := false
	if rawText == "" {
		fallbackUsed = true
		rawText = buildAnswer(query, plan, nil)
	}

	var content strings.Builder
	var filterErr error
	filter := r.newAgentOutputFilter(ctx, run, result.ProductIDs, emit, &filterErr)
	clean := strings.TrimSpace(filter.Clean(rawText))
	if filterErr != nil {
		return filterErr
	}
	if clean == "" {
		fallbackUsed = true
		clean = buildAnswer(query, plan, nil)
	}
	content.WriteString(clean)
	result.LastText = clean
	if err := emit(domain.SSEEvent{Type: "content_delta", RunID: run.RunID, Part: &domain.AgentBlock{Type: "text", Content: clean}}); err != nil {
		return err
	}
	if err := emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: clean}); err != nil {
		return err
	}
	r.trace(ctx, run, "react.final_format", "format_check", "", "ok", time.Since(startedAt).Milliseconds(), "", map[string]any{
		"route":               plan.Route,
		"intent":              plan.ReferenceIntent(),
		"raw_length":          len([]rune(rawText)),
		"filtered_length":     len([]rune(content.String())),
		"filtered_output":     content.String(),
		"filter_used":         true,
		"fallback_used":       fallbackUsed,
		"allowed_product_ids": result.ProductIDs,
		"note":                "最终文案直接来自 react.step final.text；为降低响应时间，不再额外调用 react.final LLM。",
	})
	return nil
}

func (r *Runtime) streamReactFinal(ctx context.Context, run domain.AgentRun, plan runPlan, query string, messages []ChatMessage, finalAction reactAction, result reactRunResult, emit func(domain.SSEEvent) error) error {
	if err := r.emitStatus(ctx, run.RunID, "answer", "正在生成回答", emit); err != nil {
		return err
	}
	finalInstruction := map[string]any{
		"user_query": query,
		"route":      plan.Route,
		"intent":     plan.ReferenceIntent(),
		// final_hint 来自第一阶段的 final action，只应是“为什么信息足够回答”的短提示。
		// 如果模型在第一阶段误把完整 Markdown 回答塞进这里，最终成文轮可能会复述；
		// 因此 prompt 和后续防护都应继续收紧这个字段的长度和格式。
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
	filter := r.newAgentOutputFilter(ctx, run, result.ProductIDs, emit, &filterErr)
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

func (r *Runtime) newAgentOutputFilter(ctx context.Context, run domain.AgentRun, allowedProductIDs []string, emit func(domain.SSEEvent) error, filterErr *error) *streamTextFilter {
	return newStreamTextFilter(allowedProductIDs, streamFilterCallbacks{
		OnItem: func(productID string) {
			if *filterErr != nil {
				return
			}
			product, ok := r.store.GetProduct(ctx, productID)
			if !ok {
				return
			}
			card := product.ProductCard
			part := domain.AgentBlock{Type: "product_card", Product: &card}
			*filterErr = emit(domain.SSEEvent{Type: "content_delta", RunID: run.RunID, Part: &part})
		},
		OnBlock: func(block domain.AgentBlock) {
			if *filterErr != nil {
				return
			}
			*filterErr = emit(domain.SSEEvent{Type: "block_delta", RunID: run.RunID, Block: &block})
		},
	})
}

func (r *Runtime) reactSystemPromptForPlan(ctx context.Context, plan runPlan) string {
	template := r.stringConfig(ctx, "agent.prompt.main_template", configcenter.DefaultMainAgentTemplatePrompt)
	intent := plan.ReferenceIntent()
	intentPrompt := r.stringConfig(ctx, "agent.prompt.intent."+intent, configcenter.DefaultIntentPrompt(intent))
	brief, outputRules := splitIntentPrompt(intentPrompt)
	replacements := map[string]string{
		"intent_brief":        brief,
		"tool_call_protocol":  r.stringConfig(ctx, "agent.prompt.tool_call_protocol", configcenter.DefaultToolCallProtocolPrompt),
		"available_tools":     r.availableToolsPrompt(ctx, plan),
		"available_skills":    r.availableSkillsPrompt(ctx, plan),
		"tool_focus":          r.toolFocusPrompt(ctx, plan),
		"final_output_rules":  r.finalOutputRulesPrompt(ctx, plan),
		"intent_output_rules": outputRules,
	}
	return renderPromptTemplate(template, replacements)
}

func (r *Runtime) finalSystemPromptForPlan(ctx context.Context, plan runPlan) string {
	intent := plan.ReferenceIntent()
	intentPrompt := r.stringConfig(ctx, "agent.prompt.intent."+intent, configcenter.DefaultIntentPrompt(intent))
	brief, outputRules := splitIntentPrompt(intentPrompt)
	parts := []string{
		"你是小猪小狗电商平台的 AI 导购主 Agent。最终回答阶段只输出中文自然语言，不输出隐藏推理。",
		brief,
		"输出规范：\n" + r.finalOutputRulesPrompt(ctx, plan),
	}
	if strings.TrimSpace(outputRules) != "" {
		parts = append(parts, "当前子意图输出要求：\n"+outputRules)
	}
	return strings.Join(parts, "\n\n")
}

func reactUserPrompt(query string, plan runPlan) string {
	return fmt.Sprintf(`用户问题：%s

一级路由：%s
意图：%s
层级：%s
二级层级：%s

请按工具协议输出下一步：需要工具时输出 JSON；信息足够时输出 <final>最终回答</final>。`, query, plan.Route, plan.ReferenceIntent(), plan.Level, plan.SecondaryLevel)
}

func renderPromptTemplate(template string, replacements map[string]string) string {
	out := template
	for key, value := range replacements {
		out = strings.ReplaceAll(out, "{"+key+"}", strings.TrimSpace(value))
	}
	return strings.TrimSpace(out)
}

func splitIntentPrompt(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if index := strings.Index(raw, "\n【输出硬约束】"); index >= 0 {
		raw = strings.TrimSpace(raw[:index])
	}
	lines := strings.Split(raw, "\n")
	briefIndex := -1
	for index, line := range lines {
		if strings.TrimSpace(line) != "" {
			briefIndex = index
			break
		}
	}
	if briefIndex < 0 {
		return "", ""
	}
	brief := strings.TrimSpace(lines[briefIndex])
	if briefIndex+1 >= len(lines) {
		return brief, ""
	}
	return brief, strings.TrimSpace(strings.Join(lines[briefIndex+1:], "\n"))
}

func statusTextForTool(tool string) string {
	switch tool {
	case toolSearchProducts:
		return "正在检索商品"
	case toolSearchImage:
		return "正在按图片检索商品"
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

func thinkingSummaryForToolObservation(observation toolObservation) string {
	if strings.TrimSpace(observation.Message) != "" {
		return observation.Message
	}
	if len(observation.ProductIDs) > 0 {
		return fmt.Sprintf("已找到 %d 个可用商品候选。", len(observation.ProductIDs))
	}
	if len(observation.ChunkIDs) > 0 {
		return fmt.Sprintf("已找到 %d 个可参考资料片段。", len(observation.ChunkIDs))
	}
	if observation.OK {
		return "已完成相关信息查询。"
	}
	return "查询未得到可用结果，正在尝试用现有信息回答。"
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

func isStructuredFinalTag(tag string) bool {
	switch tag {
	case "coupon_list", "discount_preview", "review_summary", "after_sales_policy", "navigation_action":
		return true
	default:
		return false
	}
}

func blockFromStructuredFinalTag(tag string, raw string) (domain.AgentBlock, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return domain.AgentBlock{}, false
	}
	var payload struct {
		Title   string           `json:"title"`
		Items   []map[string]any `json:"items"`
		Summary map[string]any   `json:"summary"`
		Message string           `json:"message"`
		Action  map[string]any   `json:"action"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return domain.AgentBlock{}, false
	}
	block := domain.AgentBlock{
		Type:    tag,
		Title:   strings.TrimSpace(payload.Title),
		Items:   payload.Items,
		Summary: payload.Summary,
		Message: strings.TrimSpace(payload.Message),
		Action:  payload.Action,
	}
	if block.Title == "" {
		block.Title = defaultStructuredBlockTitle(tag)
	}
	return block, true
}

func defaultStructuredBlockTitle(tag string) string {
	switch tag {
	case "coupon_list":
		return "优惠券"
	case "discount_preview":
		return "优惠试算"
	case "review_summary":
		return "评价摘要"
	case "after_sales_policy":
		return "售后规则"
	case "navigation_action":
		return "页面入口"
	default:
		return "服务信息"
	}
}

type streamTextFilter struct {
	inTag            bool
	tagBuffer        strings.Builder
	itemBuffer       strings.Builder
	structuredBuffer strings.Builder
	inItem           bool
	inBuyer          bool
	structuredTag    string
	allowedItemIDs   map[string]bool
	emittedItemIDs   map[string]bool
	onItem           func(string)
	onBlock          func(domain.AgentBlock)
	pendingItemText  string
	fenceCarry       string
}

type streamFilterCallbacks struct {
	OnItem  func(string)
	OnBlock func(domain.AgentBlock)
}

func newStreamTextFilter(allowedProductIDs []string, callbacks ...streamFilterCallbacks) *streamTextFilter {
	allowed := make(map[string]bool, len(allowedProductIDs))
	for _, id := range allowedProductIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = true
		}
	}
	var callback streamFilterCallbacks
	if len(callbacks) > 0 {
		callback = callbacks[0]
	}
	return &streamTextFilter{allowedItemIDs: allowed, emittedItemIDs: map[string]bool{}, onItem: callback.OnItem, onBlock: callback.OnBlock}
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
			default:
				if isStructuredFinalTag(tag) {
					f.structuredTag = tag
					f.structuredBuffer.Reset()
					continue
				}
				if f.structuredTag != "" && tag == "/"+f.structuredTag {
					if block, ok := blockFromStructuredFinalTag(f.structuredTag, f.structuredBuffer.String()); ok && f.onBlock != nil {
						f.onBlock(block)
					}
					f.structuredTag = ""
					f.structuredBuffer.Reset()
					continue
				}
			}
			continue
		case f.inTag:
			f.tagBuffer.WriteRune(item)
			continue
		case f.structuredTag != "":
			f.structuredBuffer.WriteRune(item)
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
