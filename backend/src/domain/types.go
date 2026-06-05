package domain

import "time"

type AccountRole string

const (
	AccountRoleUser     AccountRole = "user"
	AccountRoleMerchant AccountRole = "merchant"
	AccountRoleAdmin    AccountRole = "admin"
)

const (
	AccountStatusActive   = "active"
	AccountStatusInactive = "inactive"
	AccountStatusRisk     = "risk"
)

const (
	ProductStatusActive   = "active"
	ProductStatusInactive = "inactive"
	ProductStatusDeleted  = "deleted"
	ProductStatusRisk     = "risk"
)

const (
	MerchantStatusActive   = "active"
	MerchantStatusInactive = "inactive"
	MerchantStatusRisk     = "risk"
)

type Account struct {
	AccountID   string      `json:"account_id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	AvatarURL   string      `json:"avatar_url,omitempty"`
	Phone       string      `json:"phone,omitempty"`
	Email       string      `json:"email,omitempty"`
	Role        AccountRole `json:"role"`
	MerchantID  string      `json:"merchant_id,omitempty"`
	Status      string      `json:"status,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

type AccountCreateInput struct {
	Username     string
	PasswordHash string
	DisplayName  string
	AvatarURL    string
	Phone        string
	Email        string
	Role         AccountRole
	MerchantID   string
}

type ChatSession struct {
	SessionID     string    `json:"session_id"`
	AccountID     string    `json:"account_id,omitempty"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary,omitempty"`
	MessageCount  int       `json:"message_count"`
	LastMessageAt time.Time `json:"last_message_at,omitempty"`
	PinnedAt      time.Time `json:"pinned_at,omitempty"`
	DeletedAt     time.Time `json:"deleted_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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

type ConversationRecord struct {
	RunID       string        `json:"run_id"`
	SessionID   string        `json:"session_id"`
	MessageID   string        `json:"message_id"`
	AccountID   string        `json:"account_id,omitempty"`
	UserQuery   string        `json:"user_query"`
	FinalAnswer string        `json:"final_answer"`
	ProductIDs  []string      `json:"product_ids,omitempty"`
	ProductRefs []ProductCard `json:"product_refs,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type AgentRun struct {
	RunID      string         `json:"run_id"`
	SessionID  string         `json:"session_id"`
	MessageID  string         `json:"message_id"`
	AccountID  string         `json:"account_id,omitempty"`
	Status     RunStatus      `json:"status"`
	TraceID    string         `json:"trace_id"`
	QueryTitle string         `json:"query_title,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Content    string         `json:"content"`
	Blocks     []AgentBlock   `json:"blocks,omitempty"`
	Followups  []string       `json:"followups,omitempty"`
	Segments   []AgentSegment `json:"segments,omitempty"`
}

type AgentSegment struct {
	Type    string       `json:"type"`
	Text    string       `json:"text,omitempty"`
	Block   *AgentBlock  `json:"block,omitempty"`
	Thought *ThoughtStep `json:"thought,omitempty"`
}

type ThoughtStep struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Order   int    `json:"order,omitempty"`
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
	Domain      string    `json:"domain,omitempty"`
	IsSecret    bool      `json:"is_secret"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AppConfigInput struct {
	ConfigKey   string
	ConfigValue string
	ValueType   string
	Description string
	Domain      string
	IsSecret    bool
}

type AgentPrompt struct {
	PromptID    string    `json:"prompt_id"`
	PromptKey   string    `json:"prompt_key"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Status      string    `json:"status"`
	Version     int       `json:"version"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentPromptInput struct {
	PromptKey   string
	Title       string
	Content     string
	Description string
	CreatedBy   string
}

type AgentPromptPublishRecord struct {
	RecordID    string    `json:"record_id"`
	PromptKey   string    `json:"prompt_key"`
	PromptID    string    `json:"prompt_id"`
	Version     int       `json:"version"`
	PublishedBy string    `json:"published_by,omitempty"`
	NacosDataID string    `json:"nacos_data_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type VectorIndexStatus struct {
	Enabled     bool                     `json:"enabled"`
	Ready       bool                     `json:"ready"`
	Address     string                   `json:"address"`
	Collections []VectorCollectionStatus `json:"collections"`
	Error       string                   `json:"error,omitempty"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type VectorCollectionStatus struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	PrimaryKey string `json:"primary_key"`
	VectorKey  string `json:"vector_key"`
	MetricType string `json:"metric_type"`
	Dimension  int    `json:"dimension"`
	RowCount   int64  `json:"row_count"`
	LoadState  string `json:"load_state"`
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
	FileID       string `json:"file_id,omitempty"`
	Type         string `json:"type"`
	URL          string `json:"url,omitempty"`
	Name         string `json:"name,omitempty"`
	ObjectKey    string `json:"object_key,omitempty"`
}

type StoredFile struct {
	FileID          string    `json:"file_id"`
	AccountID       string    `json:"account_id,omitempty"`
	ObjectKey       string    `json:"object_key"`
	URL             string    `json:"url"`
	MimeType        string    `json:"mime_type"`
	SizeBytes       int64     `json:"size_bytes"`
	ContentHash     string    `json:"content_hash"`
	StorageProvider string    `json:"storage_provider"`
	SourceURL       string    `json:"source_url,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type StoredFileInput struct {
	FileID          string
	AccountID       string
	ObjectKey       string
	URL             string
	MimeType        string
	SizeBytes       int64
	ContentHash     string
	StorageProvider string
	SourceURL       string
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
	Status          string   `json:"status,omitempty"`
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

type DiscountLine struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Amount      string `json:"amount"`
	Description string `json:"description,omitempty"`
}

type DiscountPreview struct {
	TotalAmount    string         `json:"total_amount"`
	DiscountAmount string         `json:"discount_amount"`
	PayAmount      string         `json:"pay_amount"`
	Lines          []DiscountLine `json:"lines"`
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
	OrderID           string      `json:"order_id"`
	OrderNo           string      `json:"order_no"`
	AccountID         string      `json:"account_id"`
	MerchantID        string      `json:"merchant_id"`
	MerchantName      string      `json:"merchant_name"`
	Status            string      `json:"status"`
	TotalAmount       string      `json:"total_amount"`
	DiscountAmount    string      `json:"discount_amount"`
	PayAmount         string      `json:"pay_amount"`
	PaymentDeadlineAt time.Time   `json:"payment_deadline_at,omitempty"`
	PaidAt            time.Time   `json:"paid_at,omitempty"`
	ClosedAt          time.Time   `json:"closed_at,omitempty"`
	CompletedAt       time.Time   `json:"completed_at,omitempty"`
	CancelReason      string      `json:"cancel_reason,omitempty"`
	Items             []OrderItem `json:"items"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type Payment struct {
	PaymentID     string    `json:"payment_id"`
	OrderID       string    `json:"order_id"`
	AccountID     string    `json:"account_id"`
	Amount        string    `json:"amount"`
	Status        string    `json:"status"`
	Method        string    `json:"method"`
	TransactionNo string    `json:"transaction_no,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
	PaidAt        time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PromotionRule struct {
	PromotionID     string    `json:"promotion_id"`
	Name            string    `json:"name"`
	Scope           string    `json:"scope"`
	MerchantID      string    `json:"merchant_id,omitempty"`
	ProductID       string    `json:"product_id,omitempty"`
	CategoryID      string    `json:"category_id,omitempty"`
	Type            string    `json:"type"`
	ThresholdAmount string    `json:"threshold_amount"`
	DiscountAmount  string    `json:"discount_amount"`
	DiscountRate    string    `json:"discount_rate"`
	Stackable       bool      `json:"stackable"`
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PromotionRuleInput struct {
	Name            string `json:"name"`
	Scope           string `json:"scope"`
	MerchantID      string `json:"merchant_id,omitempty"`
	ProductID       string `json:"product_id,omitempty"`
	CategoryID      string `json:"category_id,omitempty"`
	Type            string `json:"type"`
	ThresholdAmount string `json:"threshold_amount"`
	DiscountAmount  string `json:"discount_amount"`
	DiscountRate    string `json:"discount_rate"`
	Stackable       bool   `json:"stackable"`
	StartAt         string `json:"start_at,omitempty"`
	EndAt           string `json:"end_at,omitempty"`
	Status          string `json:"status"`
}

type Coupon struct {
	CouponID        string    `json:"coupon_id"`
	Name            string    `json:"name"`
	Scope           string    `json:"scope"`
	MerchantID      string    `json:"merchant_id,omitempty"`
	Type            string    `json:"type"`
	ThresholdAmount string    `json:"threshold_amount"`
	DiscountAmount  string    `json:"discount_amount"`
	TotalCount      int       `json:"total_count"`
	ClaimedCount    int       `json:"claimed_count"`
	PerUserLimit    int       `json:"per_user_limit"`
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type UserCoupon struct {
	UserCouponID string    `json:"user_coupon_id"`
	CouponID     string    `json:"coupon_id"`
	AccountID    string    `json:"account_id"`
	Status       string    `json:"status"`
	OrderID      string    `json:"order_id,omitempty"`
	ClaimedAt    time.Time `json:"claimed_at"`
	UsedAt       time.Time `json:"used_at,omitempty"`
	Coupon       Coupon    `json:"coupon"`
}

type ProductReview struct {
	ReviewID          string    `json:"review_id"`
	OrderID           string    `json:"order_id"`
	OrderItemID       string    `json:"order_item_id"`
	ProductID         string    `json:"product_id"`
	SkuID             string    `json:"sku_id,omitempty"`
	AccountID         string    `json:"account_id"`
	Username          string    `json:"username,omitempty"`
	Rating            int       `json:"rating"`
	Content           string    `json:"content"`
	Tags              []string  `json:"tags"`
	Status            string    `json:"status"`
	MerchantReply     string    `json:"merchant_reply,omitempty"`
	MerchantRepliedAt time.Time `json:"merchant_replied_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProductReviewInput struct {
	Rating  int      `json:"rating"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
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
	Type       string                 `json:"type"`
	Title      string                 `json:"title,omitempty"`
	Product    *ProductCard           `json:"product,omitempty"`
	ProductIDs []string               `json:"product_ids,omitempty"`
	Citation   *Citation              `json:"citation,omitempty"`
	ChunkIDs   []string               `json:"chunk_ids,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Columns    []string               `json:"columns,omitempty"`
	Rows       []ComparisonRow        `json:"rows,omitempty"`
	Items      []map[string]any       `json:"items,omitempty"`
	Summary    map[string]any         `json:"summary,omitempty"`
	Cart       *Cart                  `json:"cart,omitempty"`
	Orders     []Order                `json:"orders,omitempty"`
	Code       string                 `json:"code,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Action     map[string]interface{} `json:"action,omitempty"`
}

type ComparisonRow struct {
	ProductID string   `json:"productId"`
	Values    []string `json:"values"`
}

type SSEEvent struct {
	Type          string       `json:"type"`
	RunID         string       `json:"run_id,omitempty"`
	SessionID     string       `json:"session_id,omitempty"`
	UserMessageID string       `json:"user_message_id,omitempty"`
	TraceID       string       `json:"trace_id,omitempty"`
	Stage         string       `json:"stage,omitempty"`
	Text          string       `json:"text,omitempty"`
	Delta         string       `json:"delta,omitempty"`
	Block         *AgentBlock  `json:"block,omitempty"`
	Part          *AgentBlock  `json:"part,omitempty"`
	Step          *ThoughtStep `json:"step,omitempty"`
	Questions     []string     `json:"questions,omitempty"`
	Code          string       `json:"code,omitempty"`
	Message       string       `json:"message,omitempty"`
}
