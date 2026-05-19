package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	nextID   int64
	sessions map[string]domain.ChatSession
	messages map[string]domain.UserMessage
	runs     map[string]domain.AgentRun
	products []domain.ProductCard
	chunks   []domain.Citation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]domain.ChatSession),
		messages: make(map[string]domain.UserMessage),
		runs:     make(map[string]domain.AgentRun),
		products: seedProducts(),
		chunks:   seedChunks(),
	}
}

func (s *MemoryStore) CreateSession(ctx context.Context, title string) (domain.ChatSession, error) {
	select {
	case <-ctx.Done():
		return domain.ChatSession{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.next("sess")
	session := domain.ChatSession{
		SessionID: id,
		Title:     title,
		CreatedAt: time.Now(),
	}
	s.sessions[id] = session
	return session, nil
}

func (s *MemoryStore) GetSession(ctx context.Context, sessionID string) (domain.ChatSession, bool) {
	select {
	case <-ctx.Done():
		return domain.ChatSession{}, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *MemoryStore) CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, error) {
	select {
	case <-ctx.Done():
		return domain.UserMessage{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	input.MessageID = s.next("msg")
	input.CreatedAt = time.Now()
	s.messages[input.MessageID] = input
	return input, nil
}

func (s *MemoryStore) CreateRun(ctx context.Context, sessionID string, messageID string) (domain.AgentRun, error) {
	select {
	case <-ctx.Done():
		return domain.AgentRun{}, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	run := domain.AgentRun{
		RunID:     s.next("run"),
		SessionID: sessionID,
		MessageID: messageID,
		Status:    domain.RunStatusRunning,
		TraceID:   s.next("trace"),
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.runs[run.RunID] = run
	return run, nil
}

func (s *MemoryStore) UpdateRunStatus(ctx context.Context, runID string, status domain.RunStatus) (domain.AgentRun, bool) {
	select {
	case <-ctx.Done():
		return domain.AgentRun{}, false
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return domain.AgentRun{}, false
	}
	run.Status = status
	run.UpdatedAt = time.Now()
	s.runs[runID] = run
	return run, true
}

func (s *MemoryStore) IsRunCanceled(ctx context.Context, runID string) bool {
	select {
	case <-ctx.Done():
		return true
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[runID]
	return ok && run.Status == domain.RunStatusCanceled
}

func (s *MemoryStore) SearchProducts(ctx context.Context, query string) []domain.ProductCard {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.ProductCard, 0, len(s.products))
	for _, product := range s.products {
		result = append(result, product)
	}
	return result
}

func (s *MemoryStore) SearchKnowledge(ctx context.Context, query string) []domain.Citation {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.Citation, 0, len(s.chunks))
	for _, chunk := range s.chunks {
		result = append(result, chunk)
	}
	return result
}

func (s *MemoryStore) next(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s_%06d", prefix, s.nextID)
}

func seedProducts() []domain.ProductCard {
	return []domain.ProductCard{
		{
			ProductID:       "p_phone_001",
			SkuID:           "sku_phone_001",
			Name:            "X Phone 12",
			Brand:           "XZXG",
			CategoryID:      "cat_phone",
			ImageURL:        "/mock/x-phone.png",
			Price:           "2999.00",
			StockStatus:     "in_stock",
			Tags:            []string{"预算内", "拍照", "抓拍"},
			RecommendReason: "预算控制在 3000 以内，抓拍和对焦能力适合拍娃。",
			RiskNotes:       []string{"长时间游戏续航不是最强"},
		},
		{
			ProductID:       "p_mouse_001",
			SkuID:           "sku_mouse_001",
			Name:            "Quiet Mouse Pro",
			Brand:           "PiggyDog",
			CategoryID:      "cat_mouse",
			ImageURL:        "/mock/quiet-mouse.png",
			Price:           "199.00",
			StockStatus:     "in_stock",
			Tags:            []string{"静音", "办公", "无线"},
			RecommendReason: "静音按键和人体工学外形适合办公室长期使用。",
			RiskNotes:       []string{"不适合高强度 FPS 游戏"},
		},
	}
}

func seedChunks() []domain.Citation {
	return []domain.Citation{
		{
			ChunkID: "ck_phone_001",
			Title:   "X Phone 12 商品详情",
			Snippet: "X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。",
			Source:  "mock_knowledge",
		},
		{
			ChunkID: "ck_mouse_001",
			Title:   "Quiet Mouse Pro 商品详情",
			Snippet: "Quiet Mouse Pro 主打静音按键、无线连接和人体工学握持。",
			Source:  "mock_knowledge",
		},
	}
}
