package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

type MemoryStore struct {
	mu       sync.RWMutex
	nextID   int64
	sessions map[string]domain.ChatSession
	messages map[string]domain.UserMessage
	runs     map[string]domain.AgentRun
	products []domain.ProductDetail
	skus     []domain.ProductSKU
	cart     []domain.CartItem
	chunks   []domain.Citation
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]domain.ChatSession),
		messages: make(map[string]domain.UserMessage),
		runs:     make(map[string]domain.AgentRun),
		products: seedProducts(),
		skus:     seedSKUs(),
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
		result = append(result, product.ProductCard)
	}
	return result
}

func (s *MemoryStore) ListCategories(ctx context.Context) []domain.Category {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	return []domain.Category{
		{CategoryID: "c_phone", ParentID: "", Name: "手机", Children: []domain.Category{}},
		{CategoryID: "c_mouse", ParentID: "", Name: "鼠标", Children: []domain.Category{}},
	}
}

func (s *MemoryStore) ListMerchants(ctx context.Context) []domain.Merchant {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	return []domain.Merchant{
		{
			MerchantID:   "m_001",
			Name:         "小猪数码旗舰店",
			LogoURL:      "/placeholder-merchant.svg",
			Description:  "主营手机、耳机、智能设备和办公外设。",
			ServicePhone: "400-000-0000",
			Status:       "active",
		},
	}
}

func (s *MemoryStore) ListProducts(ctx context.Context, keyword string, categoryID string) []domain.ProductCard {
	items, _ := s.ListProductsPage(ctx, keyword, categoryID, 0, 0)
	return items
}

func (s *MemoryStore) ListProductsPage(ctx context.Context, keyword string, categoryID string, limit int, offset int) ([]domain.ProductCard, bool) {
	select {
	case <-ctx.Done():
		return nil, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ProductCard, 0, len(s.products))
	for _, product := range s.products {
		if categoryID != "" && product.CategoryID != categoryID {
			continue
		}
		if keyword != "" && !matchesProductKeyword(product, keyword) {
			continue
		}
		result = append(result, product.ProductCard)
	}
	if limit <= 0 {
		return result, false
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(result) {
		return []domain.ProductCard{}, false
	}
	end := offset + limit
	hasMore := end < len(result)
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], hasMore
}

func (s *MemoryStore) GetProduct(ctx context.Context, productID string) (domain.ProductDetail, bool) {
	select {
	case <-ctx.Done():
		return domain.ProductDetail{}, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, product := range s.products {
		if product.ProductID == productID {
			return product, true
		}
	}
	return domain.ProductDetail{}, false
}

func (s *MemoryStore) ListProductSKUs(ctx context.Context, productID string) []domain.ProductSKU {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.ProductSKU, 0, len(s.skus))
	for _, sku := range s.skus {
		if sku.ProductID == productID {
			result = append(result, sku)
		}
	}
	return result
}

func (s *MemoryStore) GetCart(ctx context.Context) domain.Cart {
	select {
	case <-ctx.Done():
		return domain.Cart{}
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildCart(s.cart)
}

func (s *MemoryStore) AddCartItem(ctx context.Context, productID string, skuID string, quantity int) (domain.Cart, bool) {
	select {
	case <-ctx.Done():
		return domain.Cart{}, false
	default:
	}

	if quantity <= 0 {
		quantity = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var product domain.ProductDetail
	found := false
	for _, item := range s.products {
		if item.ProductID == productID {
			product = item
			found = true
			break
		}
	}
	if !found {
		return domain.Cart{}, false
	}

	for i := range s.cart {
		if s.cart[i].ProductID == productID && s.cart[i].SkuID == skuID {
			s.cart[i].Quantity += quantity
			return buildCart(s.cart), true
		}
	}

	s.nextID++
	cartItem := domain.CartItem{
		CartItemID:   fmt.Sprintf("cart_%06d", s.nextID),
		ProductID:    product.ProductID,
		SkuID:        skuID,
		Name:         product.Name,
		ImageURL:     product.ImageURL,
		Price:        product.Price,
		Quantity:     quantity,
		Selected:     true,
		StockStatus:  product.StockStatus,
		MerchantID:   product.MerchantID,
		MerchantName: product.MerchantName,
	}
	s.cart = append(s.cart, cartItem)
	return buildCart(s.cart), true
}

func (s *MemoryStore) UpdateCartItem(ctx context.Context, cartItemID string, quantity *int, selected *bool) (domain.Cart, bool) {
	select {
	case <-ctx.Done():
		return domain.Cart{}, false
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.cart {
		if s.cart[i].CartItemID != cartItemID {
			continue
		}
		if quantity != nil && *quantity > 0 {
			s.cart[i].Quantity = *quantity
		}
		if selected != nil {
			s.cart[i].Selected = *selected
		}
		return buildCart(s.cart), true
	}
	return domain.Cart{}, false
}

func (s *MemoryStore) DeleteCartItem(ctx context.Context, cartItemID string) (domain.Cart, bool) {
	select {
	case <-ctx.Done():
		return domain.Cart{}, false
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.cart {
		if s.cart[i].CartItemID != cartItemID {
			continue
		}
		s.cart = append(s.cart[:i], s.cart[i+1:]...)
		return buildCart(s.cart), true
	}
	return domain.Cart{}, false
}

func (s *MemoryStore) SearchKnowledge(ctx context.Context, query string) []domain.Citation {
	return s.SearchKnowledgeByPlan(ctx, rag.DefaultRetrievalPlan(query))
}

func (s *MemoryStore) SearchKnowledgeByPlan(ctx context.Context, plan rag.RetrievalPlan) []domain.Citation {
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	plan = rag.NormalizePlan(plan)
	candidates := make([]rag.Candidate, 0, len(s.chunks))
	for _, chunk := range s.chunks {
		candidates = append(candidates, rag.Candidate{
			ChunkID: chunk.ChunkID,
			Title:   chunk.Title,
			Snippet: chunk.Snippet,
			Source:  chunk.Source,
		})
	}
	ranked := rag.RankCandidates(plan, candidates)
	result := make([]domain.Citation, 0, len(ranked))
	for _, chunk := range ranked {
		result = append(result, domain.Citation{
			ChunkID: chunk.ChunkID,
			Title:   chunk.Title,
			Snippet: rag.Snippet(chunk.Snippet, plan.Compress.MaxCharsPerChunk),
			Source:  chunk.Source,
		})
	}
	return result
}

func (s *MemoryStore) next(prefix string) string {
	s.nextID++
	return fmt.Sprintf("%s_%06d", prefix, s.nextID)
}

func seedProducts() []domain.ProductDetail {
	return []domain.ProductDetail{
		{
			ProductCard: domain.ProductCard{
				ProductID:       "p_001",
				SkuID:           "sku_001",
				MerchantID:      "m_001",
				MerchantName:    "小猪数码旗舰店",
				Name:            "X Phone 12",
				Brand:           "X",
				CategoryID:      "c_phone",
				ImageURL:        "https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80",
				Price:           "2999.00",
				MarketPrice:     "3299.00",
				StockStatus:     "in_stock",
				Tags:            []string{"拍照", "预算内", "抓拍"},
				SellingPoints:   []string{"高速对焦", "儿童抓拍模式", "256GB 存储"},
				RecommendReason: "预算控制在 3000 以内，抓拍和对焦能力适合拍娃。",
				RiskNotes:       []string{"长时间游戏续航不是最强"},
			},
			ImageURLs:      []string{"https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80"},
			StockQuantity:  84,
			Attributes:     []domain.ProductAttribute{{Key: "存储", Value: "256GB"}, {Key: "重量", Value: "189", Unit: "g"}, {Key: "屏幕", Value: "6.5 英寸 OLED"}},
			SuitableFor:    []string{"拍娃", "日常拍照", "预算敏感"},
			NotSuitableFor: []string{"重度游戏"},
			Description:    "适合预算内拍照和日常使用的手机。",
		},
		{
			ProductCard: domain.ProductCard{
				ProductID:       "p_002",
				SkuID:           "sku_002",
				MerchantID:      "m_001",
				MerchantName:    "小猪数码旗舰店",
				Name:            "Y Camera Max",
				Brand:           "Y",
				CategoryID:      "c_phone",
				ImageURL:        "https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80",
				Price:           "3499.00",
				MarketPrice:     "3899.00",
				StockStatus:     "in_stock",
				Tags:            []string{"影像旗舰", "长焦", "续航"},
				SellingPoints:   []string{"长焦表现好", "夜景稳定", "续航更强"},
				RecommendReason: "影像能力更强，但价格超过 3000。",
				RiskNotes:       []string{"严格 3000 以内预算不适合"},
			},
			ImageURLs:      []string{"https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80"},
			StockQuantity:  32,
			Attributes:     []domain.ProductAttribute{{Key: "存储", Value: "256GB"}, {Key: "重量", Value: "204", Unit: "g"}},
			SuitableFor:    []string{"旅行拍照", "重视续航"},
			NotSuitableFor: []string{"严格 3000 以内预算"},
			Description:    "影像能力更强，但价格超过 3000。",
		},
		{
			ProductCard: domain.ProductCard{
				ProductID:       "p_mouse_001",
				SkuID:           "sku_mouse_001",
				MerchantID:      "m_001",
				MerchantName:    "小猪数码旗舰店",
				Name:            "Quiet Mouse S",
				Brand:           "Q",
				CategoryID:      "c_mouse",
				ImageURL:        "https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80",
				Price:           "129.00",
				MarketPrice:     "159.00",
				StockStatus:     "in_stock",
				Tags:            []string{"静音", "办公", "无线"},
				SellingPoints:   []string{"静音微动", "人体工学", "长续航"},
				RecommendReason: "静音、无线、握持舒适，更适合办公和宿舍。",
				RiskNotes:       []string{"不适合高强度电竞"},
			},
			ImageURLs:      []string{"https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80"},
			StockQuantity:  120,
			Attributes:     []domain.ProductAttribute{{Key: "连接", Value: "2.4G 无线 + 蓝牙"}, {Key: "重量", Value: "88", Unit: "g"}},
			SuitableFor:    []string{"办公", "宿舍", "图书馆"},
			NotSuitableFor: []string{"高强度电竞"},
			Description:    "适合安静办公环境的无线鼠标。",
		},
	}
}

func seedSKUs() []domain.ProductSKU {
	return []domain.ProductSKU{
		{SkuID: "sku_001", ProductID: "p_001", SkuName: "X Phone 12 标准版", Price: "2999.00", StockQuantity: 84, StockStatus: "in_stock", Specs: map[string]string{"版本": "标准版"}},
		{SkuID: "sku_002", ProductID: "p_002", SkuName: "Y Camera Max 标准版", Price: "3499.00", StockQuantity: 32, StockStatus: "in_stock", Specs: map[string]string{"版本": "标准版"}},
		{SkuID: "sku_mouse_001", ProductID: "p_mouse_001", SkuName: "Quiet Mouse S 标准版", Price: "129.00", StockQuantity: 120, StockStatus: "in_stock", Specs: map[string]string{"版本": "标准版"}},
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

func matchesProductKeyword(product domain.ProductDetail, keyword string) bool {
	text := product.Name + product.Brand
	for _, tag := range product.Tags {
		text += tag
	}
	for _, point := range product.SellingPoints {
		text += point
	}
	return containsFold(text, keyword)
}

func containsFold(text string, keyword string) bool {
	return len(keyword) == 0 || strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func buildCart(items []domain.CartItem) domain.Cart {
	copied := make([]domain.CartItem, len(items))
	copy(copied, items)

	total := 0.0
	selectedCount := 0
	for _, item := range copied {
		if !item.Selected {
			continue
		}
		price, _ := strconv.ParseFloat(item.Price, 64)
		total += price * float64(item.Quantity)
		selectedCount += item.Quantity
	}

	amount := fmt.Sprintf("%.2f", total)
	return domain.Cart{
		Items: copied,
		Summary: domain.CartSummary{
			SelectedCount:  selectedCount,
			TotalAmount:    amount,
			DiscountAmount: "0.00",
			PayAmount:      amount,
		},
	}
}
