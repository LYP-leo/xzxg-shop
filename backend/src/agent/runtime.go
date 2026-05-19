package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type Runtime struct {
	store  *store.MemoryStore
	logger *slog.Logger
}

func NewRuntime(store *store.MemoryStore, logger *slog.Logger) *Runtime {
	return &Runtime{store: store, logger: logger}
}

func (r *Runtime) Stream(ctx context.Context, run domain.AgentRun, message domain.UserMessage, emit func(domain.SSEEvent) error) error {
	r.logger.Info("agent run started", "run_id", run.RunID, "session_id", run.SessionID, "message_id", message.MessageID)

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

	products := r.store.SearchProducts(ctx, message.Content)
	chunks := r.store.SearchKnowledge(ctx, message.Content)

	if err := r.emitStatus(ctx, run.RunID, "tool", "正在检索商品和知识库", emit); err != nil {
		return err
	}

	text := buildAnswer(message.Content, products)
	for _, delta := range splitText(text, 14) {
		if r.store.IsRunCanceled(ctx, run.RunID) {
			return emit(domain.SSEEvent{Type: "error", RunID: run.RunID, Code: "canceled", Message: "已停止生成"})
		}
		if err := emit(domain.SSEEvent{Type: "text_delta", RunID: run.RunID, Delta: delta}); err != nil {
			return err
		}
		time.Sleep(80 * time.Millisecond)
	}

	if len(products) > 0 {
		product := products[0]
		if err := emit(domain.SSEEvent{
			Type:  "block_delta",
			RunID: run.RunID,
			Block: &domain.AgentBlock{Type: "product_card", Product: &product},
		}); err != nil {
			return err
		}
	}

	if len(chunks) > 0 {
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
		Questions: []string{"你更重视价格、续航还是拍照？", "是否有品牌或尺寸偏好？"},
	}); err != nil {
		return err
	}

	if _, ok := r.store.UpdateRunStatus(ctx, run.RunID, domain.RunStatusCompleted); !ok {
		r.logger.Warn("agent run status update skipped", "run_id", run.RunID)
	}
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

func buildAnswer(query string, products []domain.ProductCard) string {
	if len(products) == 0 {
		return "我暂时没有检索到足够的商品数据，可以先补充预算、使用场景和偏好，我会继续缩小范围。"
	}
	first := products[0]
	if strings.Contains(query, "鼠标") {
		return "如果主要用于办公，我会优先看静音、握持舒适度和无线稳定性。当前更推荐 " + first.Name + "，它更适合长时间办公使用。"
	}
	return "结合你的需求，我会优先推荐 " + first.Name + "。它的价格、核心卖点和风险提示都来自商品库与知识库，适合先作为第一候选。"
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
