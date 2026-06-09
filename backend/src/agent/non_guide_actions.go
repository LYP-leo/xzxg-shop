package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

// runNonGuideAction 执行非导购下的确定性商品/购物车动作。
// 这类请求的核心目标是“真实完成动作”，不适合交给 ReAct 自由决定是否调用工具；
// 否则模型可能直接输出“已加购”，但没有产生 add_cart_item trace，也没有写购物车。
func (r *Runtime) runNonGuideAction(ctx context.Context, run domain.AgentRun, plan runPlan, query string, emit func(domain.SSEEvent) error) (reactRunResult, bool, error) {
	if plan.Route != "non_guide" {
		return reactRunResult{}, false, nil
	}

	switch plan.Intent {
	case "cart_service":
		actionQuery := currentActionQuery(query)
		if looksCartAdd(actionQuery) {
			result, err := r.runFastCartAdd(ctx, run, query, emit)
			return result, true, err
		}
		return reactRunResult{}, false, nil
	case "order_service":
		actionQuery := currentActionQuery(query)
		if looksCheckout(actionQuery) {
			result, err := r.runFastCheckout(ctx, run, emit)
			return result, true, err
		}
		return reactRunResult{}, false, nil
	case "cart_add":
		result, err := r.runFastCartAdd(ctx, run, query, emit)
		return result, true, err
	case "checkout_confirm":
		result, err := r.runFastCheckout(ctx, run, emit)
		return result, true, err
	default:
		return reactRunResult{}, false, nil
	}
}

// currentActionQuery 从带短期记忆的 effective query 中取出当前用户原话。
// 确定性业务动作只能由本轮用户请求触发；历史记忆可用于解析商品 ID，
// 但不能参与“加购/结算”等动作类型判断。
func currentActionQuery(query string) string {
	q := strings.TrimSpace(query)
	const prefix = "当前用户问题："
	if !strings.HasPrefix(q, prefix) {
		return q
	}
	current := strings.TrimSpace(strings.TrimPrefix(q, prefix))
	for _, marker := range []string{
		"\n\n相关会话记忆：",
		"\n\n相关历史商品",
		"\n\n记忆使用规则：",
	} {
		if index := strings.Index(current, marker); index >= 0 {
			current = strings.TrimSpace(current[:index])
			break
		}
	}
	return current
}

func (r *Runtime) runFastCartAdd(ctx context.Context, run domain.AgentRun, query string, emit func(domain.SSEEvent) error) (reactRunResult, error) {
	result := reactRunResult{}
	productIDs := productIDsFromText(query)
	if len(productIDs) == 0 {
		text := "我还不能确定要加购哪件商品。请点选商品卡片，或告诉我明确的商品 ID 后我再帮你加入购物车。"
		if err := r.emitText(run, text, emit); err != nil {
			return result, err
		}
		result.LastText = text
		r.trace(ctx, run, "non_guide", "cart_add", "", "blocked", 0, "", map[string]any{
			"reason": "missing_product_id",
			"query":  query,
		})
		return result, nil
	}

	addedNames := make([]string, 0, len(productIDs))
	failedMessages := make([]string, 0)
	for _, productID := range productIDs {
		allowed := map[string]bool{productID: true}
		args, _ := json.Marshal(map[string]any{
			"product_id": productID,
			"quantity":   1,
		})

		if err := r.emitStatus(ctx, run.RunID, "tool", statusTextForTool(toolAddCartItem), emit); err != nil {
			return result, err
		}
		startedAt := time.Now()
		observation := r.executeTool(ctx, run, reactAction{
			Type:      "tool_call",
			Tool:      toolAddCartItem,
			Arguments: args,
		}, toolExecutionContext{AllowedAddProductIDs: allowed})
		result.Observations = append(result.Observations, observation)
		if observation.OK {
			result.ProductIDs = appendUnique(result.ProductIDs, productID)
			productName := cartProductName(observation.Cart, productID)
			if productName == "" {
				productName = productID
			}
			addedNames = append(addedNames, productName)
		} else if strings.TrimSpace(observation.Message) != "" {
			failedMessages = append(failedMessages, observation.Message)
		}
		r.trace(ctx, run, "tools", toolAddCartItem, "", traceStatus(observation.OK), time.Since(startedAt).Milliseconds(), "", map[string]any{
			"ok":                observation.OK,
			"message":           observation.Message,
			"arguments":         json.RawMessage(args),
			"result":            observation.Result,
			"observation":       observation,
			"observation_json":  observationForModel(observation),
			"product_ids":       result.ProductIDs,
			"result_item_count": resultItemCount(observation.Result),
			"deterministic":     true,
		})
		if observation.Cart != nil {
			if err := emitCartBlock(run.RunID, *observation.Cart, emit); err != nil {
				return result, err
			}
		}
	}

	text := ""
	if len(addedNames) > 0 {
		text = "已将 " + strings.Join(addedNames, "、") + " 加入购物车。"
	}
	if len(addedNames) == 0 {
		text = "加购失败，请确认商品仍然可售后再试。"
		if len(failedMessages) > 0 {
			text = failedMessages[len(failedMessages)-1]
		}
	} else if len(failedMessages) > 0 {
		text += " 其中部分商品未能加购：" + strings.Join(failedMessages, "；")
	}
	if err := r.emitText(run, text, emit); err != nil {
		return result, err
	}
	result.LastText = text
	return result, nil
}

func cartProductName(cart *domain.Cart, productID string) string {
	if cart == nil {
		return ""
	}
	for _, item := range cart.Items {
		if item.ProductID == productID {
			return strings.TrimSpace(item.Name)
		}
	}
	return ""
}

func (r *Runtime) runFastCheckout(ctx context.Context, run domain.AgentRun, emit func(domain.SSEEvent) error) (reactRunResult, error) {
	result := reactRunResult{}
	if err := r.emitStatus(ctx, run.RunID, "tool", statusTextForTool(toolCheckout), emit); err != nil {
		return result, err
	}
	startedAt := time.Now()
	observation := r.executeTool(ctx, run, reactAction{
		Type: "tool_call",
		Tool: toolCheckout,
	}, toolExecutionContext{})
	result.Observations = append(result.Observations, observation)
	r.trace(ctx, run, "tools", toolCheckout, "", traceStatus(observation.OK), time.Since(startedAt).Milliseconds(), "", map[string]any{
		"ok":                observation.OK,
		"message":           observation.Message,
		"result":            observation.Result,
		"observation":       observation,
		"observation_json":  observationForModel(observation),
		"result_item_count": resultItemCount(observation.Result),
		"deterministic":     true,
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
	text := strings.TrimSpace(observation.Message)
	if text == "" {
		text = "结算请求已处理。"
	}
	if err := r.emitText(run, text, emit); err != nil {
		return result, err
	}
	result.LastText = text
	return result, nil
}
