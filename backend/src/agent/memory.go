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

type conversationMemory struct {
	Enabled              bool
	Records              []domain.ConversationRecord
	HasRelevantMemory    bool
	MemorySummary        string
	ReferencedProductIDs []string
	UsedRecordIDs        []string
}

type memoryRetrievalOutput struct {
	HasRelevantMemory    bool     `json:"has_relevant_memory"`
	MemorySummary        string   `json:"memory_summary"`
	ReferencedProductIDs []string `json:"referenced_product_ids"`
	UsedRecordIDs        []string `json:"used_record_ids"`
}

func (r *Runtime) buildConversationMemory(ctx context.Context, run domain.AgentRun, query string) conversationMemory {
	memory := conversationMemory{Enabled: r.boolConfig(ctx, "memory.enabled", true)}
	if !memory.Enabled || strings.TrimSpace(run.AccountID) == "" || strings.TrimSpace(run.SessionID) == "" {
		return memory
	}
	windowHours := r.intConfig(ctx, "memory.window_hours", 24)
	windowTurns := r.intConfig(ctx, "memory.window_turns", 5)
	if windowHours <= 0 {
		windowHours = 24
	}
	if windowTurns <= 0 {
		windowTurns = 5
	}
	records := r.store.ListRecentConversationRecords(ctx, run.AccountID, run.SessionID, time.Now().Add(-time.Duration(windowHours)*time.Hour), windowTurns)
	memory.Records = records
	if len(records) == 0 {
		r.trace(ctx, run, "memory", "retrieval", "", "skipped", 0, "", map[string]any{"reason": "empty_candidates"})
		return memory
	}
	if !r.llm.Enabled() {
		memory = r.heuristicConversationMemory(query, memory)
		r.trace(ctx, run, "memory", "retrieval", "", traceStatus(memory.HasRelevantMemory), 0, "", map[string]any{
			"mode":                   "heuristic",
			"candidate_count":        len(records),
			"has_relevant_memory":    memory.HasRelevantMemory,
			"referenced_product_ids": memory.ReferencedProductIDs,
			"used_record_ids":        memory.UsedRecordIDs,
		})
		return memory
	}

	startedAt := time.Now()
	temperature := 0.1
	messages := []ChatMessage{
		{Role: "system", Content: r.stringConfig(ctx, "agent.prompt.memory_retrieval", configcenter.DefaultMemoryRetrievalPrompt)},
		{Role: "user", Content: r.memoryRetrievalUserPrompt(ctx, query, records)},
	}
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if err != nil {
		r.logger.Warn("memory retrieval fallback", "run_id", run.RunID, "error", err)
		r.traceLLM(ctx, run, "memory.retrieval", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, map[string]any{"candidate_count": len(records)}))
		return r.heuristicConversationMemory(query, memory)
	}
	r.traceLLM(ctx, run, "memory.retrieval", r.llm.SmallModel(), startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"candidate_count": len(records), "raw_length": len([]rune(content))}))

	var parsed memoryRetrievalOutput
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &parsed); err != nil {
		r.logger.Warn("memory retrieval json fallback", "run_id", run.RunID, "error", err, "content", content)
		return r.heuristicConversationMemory(query, memory)
	}
	memory.HasRelevantMemory = parsed.HasRelevantMemory
	memory.MemorySummary = strings.TrimSpace(parsed.MemorySummary)
	memory.ReferencedProductIDs = filterMemoryProductIDs(parsed.ReferencedProductIDs, records)
	memory.UsedRecordIDs = filterMemoryRecordIDs(parsed.UsedRecordIDs, records)
	if memory.HasRelevantMemory && memory.MemorySummary == "" && len(memory.ReferencedProductIDs) == 0 {
		memory.HasRelevantMemory = false
	}
	return memory
}

func (r *Runtime) memoryRetrievalUserPrompt(ctx context.Context, query string, records []domain.ConversationRecord) string {
	var b strings.Builder
	b.WriteString("当前用户问题：")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\n最近历史候选（按会话轮次从早到晚排列；带【最新一轮】的是当前问题之前最近的一轮）：\n")
	maxChars := r.intConfig(ctx, "memory.max_turn_chars", 1200)
	if maxChars <= 0 {
		maxChars = 1200
	}
	for i, record := range records {
		recordID := memoryRecordID(record)
		turnLabel := fmt.Sprintf("第%d轮", i+1)
		if i == len(records)-1 {
			turnLabel += "【最新一轮】"
		}
		b.WriteString(fmt.Sprintf("\n%s：record_id=%s created_at=%s\n", turnLabel, recordID, record.CreatedAt.Format(time.RFC3339)))
		b.WriteString("用户：")
		b.WriteString(truncateRunes(record.UserQuery, maxChars/3))
		b.WriteString("\n助手：")
		b.WriteString(truncateRunes(record.FinalAnswer, maxChars))
		if len(record.ProductRefs) > 0 || len(record.ProductIDs) > 0 {
			b.WriteString("\n本轮商品顺序：")
			writeMemoryProducts(&b, record)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n轮次使用提示：如果当前问题出现“第一个/第二个/第N个 + 品类词”，优先选择最近一轮与该品类词或主题匹配的推荐轮次，并按该轮“本轮商品顺序”解析商品 ID；不要被更近但不同品类或不同动作的轮次覆盖。")
	return b.String()
}

func (r *Runtime) formatQueryWithMemory(query string, memory conversationMemory) string {
	if !memory.HasRelevantMemory {
		return query
	}
	var b strings.Builder
	b.WriteString("当前用户问题：\n")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\n相关会话记忆：\n")
	if strings.TrimSpace(memory.MemorySummary) != "" {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(memory.MemorySummary))
		b.WriteString("\n")
	}
	if len(memory.ReferencedProductIDs) > 0 {
		b.WriteString("\n相关历史商品（后续如需加购或引用商品，优先使用这里的 item_id/product_id）：\n")
		indexed := indexedMemoryProductsByID(memory.Records, memory.ReferencedProductIDs)
		written := map[string]bool{}
		for _, product := range indexed {
			written[product.ProductID] = true
			b.WriteString(fmt.Sprintf("- 来自第%d轮的第%d个商品 item_id=%s sku_id=%s name=%s price=%s\n", product.TurnIndex, product.ProductIndex, product.ProductID, product.SkuID, product.Name, product.Price))
		}
		for _, item := range missingIndexedProductIDs(memory.Records, memory.ReferencedProductIDs, written) {
			b.WriteString(fmt.Sprintf("- 来自第%d轮的第%d个商品 item_id=%s\n", item.TurnIndex, item.ProductIndex, item.ProductID))
		}
	}
	b.WriteString("\n记忆使用规则：只把相关会话记忆作为指代消解和上下文补充；最终回答仍直接回应当前用户问题，不要复述这些规则。")
	return b.String()
}

func (r *Runtime) heuristicConversationMemory(query string, memory conversationMemory) conversationMemory {
	if len(memory.Records) == 0 || !looksMemoryDependent(query) {
		return memory
	}
	record := memory.Records[len(memory.Records)-1]
	memory.HasRelevantMemory = true
	memory.MemorySummary = "当前问题承接上一轮对话：" + truncateRunes(record.UserQuery, 80)
	memory.ReferencedProductIDs = append([]string{}, record.ProductIDs...)
	memory.UsedRecordIDs = []string{memoryRecordID(record)}
	return memory
}

func (r *Runtime) UpdateSessionSummaryAfterRun(ctx context.Context, run domain.AgentRun, message domain.UserMessage, finalAnswer string) {
	if !r.boolConfig(ctx, "memory.summary_enabled", true) || !r.llm.Enabled() || strings.TrimSpace(finalAnswer) == "" {
		return
	}
	session, ok := r.store.GetSession(ctx, run.AccountID, run.SessionID)
	if !ok {
		return
	}
	startedAt := time.Now()
	temperature := 0.1
	messages := []ChatMessage{
		{Role: "system", Content: r.stringConfig(ctx, "agent.prompt.session_summary", configcenter.DefaultSessionSummaryPrompt)},
		{Role: "user", Content: fmt.Sprintf("旧摘要：%s\n\n最新用户问题：%s\n\n最新助手回答：%s", session.Summary, message.Content, truncateRunes(finalAnswer, 1800))},
	}
	content, err := r.llm.Complete(ctx, r.llm.SmallModel(), messages, temperature)
	if err != nil {
		r.logger.Warn("session summary update failed", "run_id", run.RunID, "error", err)
		r.traceLLM(ctx, run, "memory.summary", r.llm.SmallModel(), startedAt, err, llmPromptMetadata(messages, temperature, nil))
		return
	}
	r.traceLLM(ctx, run, "memory.summary", r.llm.SmallModel(), startedAt, nil, llmPromptMetadata(messages, temperature, map[string]any{"raw_length": len([]rune(content))}))
	var parsed struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &parsed); err != nil {
		r.logger.Warn("session summary json ignored", "run_id", run.RunID, "error", err, "content", content)
		return
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return
	}
	if len([]rune(summary)) > 260 {
		summary = truncateRunes(summary, 260)
	}
	r.store.UpdateSessionSummary(ctx, run.AccountID, run.SessionID, session.Title, summary)
}

func looksMemoryDependent(query string) bool {
	query = strings.TrimSpace(query)
	return containsAny(query, []string{"刚才", "上面", "前面", "那个", "这个", "第二个", "第一个", "第三个", "加购物车", "加入购物车", "继续", "换一个", "再来"})
}

func filterMemoryProductIDs(ids []string, records []domain.ConversationRecord) []string {
	allowed := map[string]bool{}
	for _, record := range records {
		for _, id := range record.ProductIDs {
			allowed[id] = true
		}
	}
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !allowed[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func filterMemoryRecordIDs(ids []string, records []domain.ConversationRecord) []string {
	allowed := map[string]bool{}
	for _, record := range records {
		allowed[memoryRecordID(record)] = true
	}
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !allowed[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

type indexedMemoryProduct struct {
	domain.ProductCard
	TurnIndex    int
	ProductIndex int
}

type indexedMemoryProductID struct {
	ProductID    string
	TurnIndex    int
	ProductIndex int
}

func memoryProductsByID(records []domain.ConversationRecord, ids []string) []domain.ProductCard {
	indexed := indexedMemoryProductsByID(records, ids)
	out := make([]domain.ProductCard, 0, len(indexed))
	for _, product := range indexed {
		out = append(out, product.ProductCard)
	}
	return out
}

func indexedMemoryProductsByID(records []domain.ConversationRecord, ids []string) []indexedMemoryProduct {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := make([]indexedMemoryProduct, 0, len(ids))
	seen := map[string]bool{}
	for turnIndex, record := range records {
		for productIndex, product := range record.ProductRefs {
			if want[product.ProductID] && !seen[product.ProductID] {
				seen[product.ProductID] = true
				out = append(out, indexedMemoryProduct{
					ProductCard:  product,
					TurnIndex:    turnIndex + 1,
					ProductIndex: productIndex + 1,
				})
			}
		}
	}
	return out
}

func missingProductIDs(records []domain.ConversationRecord, ids []string) []string {
	have := map[string]bool{}
	for _, product := range memoryProductsByID(records, ids) {
		have[product.ProductID] = true
	}
	out := make([]string, 0)
	for _, id := range ids {
		if strings.TrimSpace(id) != "" && !have[id] {
			out = append(out, id)
		}
	}
	return out
}

func missingIndexedProductIDs(records []domain.ConversationRecord, ids []string, have map[string]bool) []indexedMemoryProductID {
	out := make([]indexedMemoryProductID, 0)
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || have[id] || seen[id] {
			continue
		}
		seen[id] = true
		turnIndex, productIndex := memoryProductOrdinal(records, id)
		out = append(out, indexedMemoryProductID{
			ProductID:    id,
			TurnIndex:    turnIndex,
			ProductIndex: productIndex,
		})
	}
	return out
}

func memoryProductOrdinal(records []domain.ConversationRecord, productID string) (int, int) {
	for turnIndex, record := range records {
		for productIndex, id := range record.ProductIDs {
			if id == productID {
				return turnIndex + 1, productIndex + 1
			}
		}
	}
	return 0, 0
}

func writeMemoryProducts(b *strings.Builder, record domain.ConversationRecord) {
	if len(record.ProductRefs) > 0 {
		for i, product := range record.ProductRefs {
			b.WriteString(fmt.Sprintf("\n- 第%d个商品 item_id=%s sku_id=%s name=%s brand=%s category_id=%s price=%s", i+1, product.ProductID, product.SkuID, product.Name, product.Brand, product.CategoryID, product.Price))
		}
		return
	}
	for i, id := range record.ProductIDs {
		b.WriteString(fmt.Sprintf("\n- 第%d个商品 item_id=%s", i+1, id))
	}
}

func memoryRecordID(record domain.ConversationRecord) string {
	if strings.TrimSpace(record.RunID) != "" {
		return record.RunID
	}
	return record.MessageID
}
