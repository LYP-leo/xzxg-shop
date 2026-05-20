package domain

import "time"

type ChatSession struct {
	SessionID string    `json:"session_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type UserMessage struct {
	MessageID       string       `json:"message_id"`
	SessionID       string       `json:"session_id"`
	ClientMessageID string       `json:"client_message_id"`
	Content         string       `json:"content"`
	Attachments     []Attachment `json:"attachments"`
	CreatedAt       time.Time    `json:"created_at"`
}

type AgentRun struct {
	RunID     string    `json:"run_id"`
	SessionID string    `json:"session_id"`
	MessageID string    `json:"message_id"`
	Status    RunStatus `json:"status"`
	TraceID   string    `json:"trace_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

type Attachment struct {
	AttachmentID string `json:"attachment_id"`
	Type         string `json:"type"`
	URL          string `json:"url,omitempty"`
	Name         string `json:"name,omitempty"`
}

type ProductCard struct {
	ProductID       string   `json:"productId"`
	SkuID           string   `json:"skuId,omitempty"`
	MerchantID      string   `json:"merchantId"`
	MerchantName    string   `json:"merchantName"`
	Name            string   `json:"name"`
	Brand           string   `json:"brand"`
	CategoryID      string   `json:"categoryId"`
	ImageURL        string   `json:"imageUrl"`
	Price           string   `json:"price"`
	MarketPrice     string   `json:"marketPrice,omitempty"`
	StockStatus     string   `json:"stockStatus"`
	Tags            []string `json:"tags"`
	SellingPoints   []string `json:"sellingPoints"`
	RecommendReason string   `json:"recommendReason"`
	RiskNotes       []string `json:"riskNotes"`
}

type Merchant struct {
	MerchantID   string `json:"merchantId"`
	Name         string `json:"name"`
	LogoURL      string `json:"logoUrl"`
	Description  string `json:"description"`
	ServicePhone string `json:"servicePhone,omitempty"`
	Status       string `json:"status"`
}

type Category struct {
	CategoryID string     `json:"categoryId"`
	ParentID   string     `json:"parentId"`
	Name       string     `json:"name"`
	Children   []Category `json:"children,omitempty"`
}

type ProductAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type ProductDetail struct {
	ProductCard
	ImageURLs      []string           `json:"imageUrls"`
	StockQuantity  int                `json:"stockQuantity"`
	Attributes     []ProductAttribute `json:"attributes"`
	SuitableFor    []string           `json:"suitableFor"`
	NotSuitableFor []string           `json:"notSuitableFor"`
	Description    string             `json:"description"`
}

type ProductSKU struct {
	SkuID         string            `json:"skuId"`
	ProductID     string            `json:"productId"`
	SkuName       string            `json:"skuName"`
	Price         string            `json:"price"`
	StockQuantity int               `json:"stockQuantity"`
	StockStatus   string            `json:"stockStatus"`
	Specs         map[string]string `json:"specs"`
}

type CartItem struct {
	CartItemID   string `json:"cartItemId"`
	ProductID    string `json:"productId"`
	SkuID        string `json:"skuId,omitempty"`
	Name         string `json:"name"`
	ImageURL     string `json:"imageUrl"`
	Price        string `json:"price"`
	Quantity     int    `json:"quantity"`
	Selected     bool   `json:"selected"`
	StockStatus  string `json:"stockStatus"`
	MerchantID   string `json:"merchantId"`
	MerchantName string `json:"merchantName"`
}

type CartSummary struct {
	SelectedCount  int    `json:"selectedCount"`
	TotalAmount    string `json:"totalAmount"`
	DiscountAmount string `json:"discountAmount"`
	PayAmount      string `json:"payAmount"`
}

type Cart struct {
	Items   []CartItem  `json:"items"`
	Summary CartSummary `json:"summary"`
}

type Citation struct {
	ChunkID string `json:"chunkId"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Source  string `json:"source,omitempty"`
}

type AgentBlock struct {
	Type     string       `json:"type"`
	Product  *ProductCard `json:"product,omitempty"`
	Citation *Citation    `json:"citation,omitempty"`
	Content  string       `json:"content,omitempty"`
	Code     string       `json:"code,omitempty"`
	Message  string       `json:"message,omitempty"`
}

type SSEEvent struct {
	Type          string      `json:"type"`
	RunID         string      `json:"run_id,omitempty"`
	SessionID     string      `json:"session_id,omitempty"`
	UserMessageID string      `json:"user_message_id,omitempty"`
	TraceID       string      `json:"trace_id,omitempty"`
	Stage         string      `json:"stage,omitempty"`
	Text          string      `json:"text,omitempty"`
	Delta         string      `json:"delta,omitempty"`
	Block         *AgentBlock `json:"block,omitempty"`
	Questions     []string    `json:"questions,omitempty"`
	Code          string      `json:"code,omitempty"`
	Message       string      `json:"message,omitempty"`
}
