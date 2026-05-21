package domain

import "time"

type AccountRole string

const (
	AccountRoleUser     AccountRole = "user"
	AccountRoleMerchant AccountRole = "merchant"
	AccountRoleAdmin    AccountRole = "admin"
)

type Account struct {
	AccountID   string      `json:"account_id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	Role        AccountRole `json:"role"`
	MerchantID  string      `json:"merchant_id,omitempty"`
	Status      string      `json:"status,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

type ChatSession struct {
	SessionID string    `json:"session_id"`
	AccountID string    `json:"account_id,omitempty"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type UserMessage struct {
	MessageID       string       `json:"message_id"`
	SessionID       string       `json:"session_id"`
	AccountID       string       `json:"account_id,omitempty"`
	ClientMessageID string       `json:"client_message_id"`
	Content         string       `json:"content"`
	Attachments     []Attachment `json:"attachments"`
	CreatedAt       time.Time    `json:"created_at"`
}

type ChatSessionDetail struct {
	Session  ChatSession           `json:"session"`
	Messages []UserMessageWithRuns `json:"messages"`
}

type UserMessageWithRuns struct {
	UserMessage
	Runs []AgentRun `json:"runs"`
}

type AgentRun struct {
	RunID     string    `json:"run_id"`
	SessionID string    `json:"session_id"`
	MessageID string    `json:"message_id"`
	AccountID string    `json:"account_id,omitempty"`
	Status    RunStatus `json:"status"`
	TraceID   string    `json:"trace_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AgentTraceEvent struct {
	TraceEventID string    `json:"trace_event_id"`
	RunID        string    `json:"run_id"`
	TraceID      string    `json:"trace_id"`
	AccountID    string    `json:"account_id,omitempty"`
	Stage        string    `json:"stage"`
	EventType    string    `json:"event_type"`
	Model        string    `json:"model,omitempty"`
	Status       string    `json:"status"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
	Error        string    `json:"error,omitempty"`
	MetadataJSON string    `json:"metadata_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type AgentTraceInput struct {
	RunID        string
	TraceID      string
	AccountID    string
	Stage        string
	EventType    string
	Model        string
	Status       string
	DurationMS   int64
	Error        string
	MetadataJSON string
}

type AppConfig struct {
	ConfigKey   string    `json:"config_key"`
	ConfigValue string    `json:"config_value"`
	ValueType   string    `json:"value_type"`
	Description string    `json:"description"`
	IsSecret    bool      `json:"is_secret"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AppConfigInput struct {
	ConfigKey   string
	ConfigValue string
	ValueType   string
	Description string
	IsSecret    bool
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

type ProductUpsertInput struct {
	ProductID       string   `json:"product_id,omitempty"`
	MerchantID      string   `json:"merchant_id"`
	Name            string   `json:"name"`
	Brand           string   `json:"brand"`
	CategoryID      string   `json:"category_id"`
	ImageURL        string   `json:"image_url"`
	Price           string   `json:"price"`
	MarketPrice     string   `json:"market_price"`
	StockQuantity   int      `json:"stock_quantity"`
	StockStatus     string   `json:"stock_status"`
	Tags            []string `json:"tags"`
	SellingPoints   []string `json:"selling_points"`
	RecommendReason string   `json:"recommend_reason"`
	RiskNotes       []string `json:"risk_notes"`
	Description     string   `json:"description"`
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

type OrderItem struct {
	OrderItemID  string `json:"order_item_id"`
	ProductID    string `json:"product_id"`
	SkuID        string `json:"sku_id,omitempty"`
	Name         string `json:"name"`
	ImageURL     string `json:"image_url"`
	Price        string `json:"price"`
	Quantity     int    `json:"quantity"`
	MerchantID   string `json:"merchant_id"`
	MerchantName string `json:"merchant_name"`
}

type Order struct {
	OrderID      string      `json:"order_id"`
	AccountID    string      `json:"account_id"`
	MerchantID   string      `json:"merchant_id"`
	MerchantName string      `json:"merchant_name"`
	Status       string      `json:"status"`
	TotalAmount  string      `json:"total_amount"`
	Items        []OrderItem `json:"items"`
	CreatedAt    time.Time   `json:"created_at"`
}

type Citation struct {
	ChunkID string `json:"chunkId"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Source  string `json:"source,omitempty"`
}

type KnowledgeDocument struct {
	DocumentID string    `json:"document_id"`
	MerchantID string    `json:"merchant_id"`
	Title      string    `json:"title"`
	DocType    string    `json:"doc_type"`
	Status     string    `json:"status"`
	ChunkCount int       `json:"chunk_count"`
	CreatedAt  time.Time `json:"created_at"`
}

type KnowledgeDocumentInput struct {
	MerchantID string `json:"merchant_id"`
	Title      string `json:"title"`
	DocType    string `json:"doc_type"`
	Content    string `json:"content"`
}

type AgentBlock struct {
	Type     string                 `json:"type"`
	Product  *ProductCard           `json:"product,omitempty"`
	Citation *Citation              `json:"citation,omitempty"`
	Content  string                 `json:"content,omitempty"`
	Columns  []string               `json:"columns,omitempty"`
	Rows     []ComparisonRow        `json:"rows,omitempty"`
	Cart     *Cart                  `json:"cart,omitempty"`
	Orders   []Order                `json:"orders,omitempty"`
	Code     string                 `json:"code,omitempty"`
	Message  string                 `json:"message,omitempty"`
	Action   map[string]interface{} `json:"action,omitempty"`
}

type ComparisonRow struct {
	ProductID string   `json:"productId"`
	Values    []string `json:"values"`
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
