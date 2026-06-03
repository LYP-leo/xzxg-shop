package store

import (
	"context"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

// Store 是后端业务层唯一依赖的数据访问边界。
// HTTP handler 和 Agent runtime 都只面向这个接口，避免直接绑定 MySQL 细节。
type Store interface {
	// 账号与鉴权。
	GetAccount(ctx context.Context, accountID string) (domain.Account, bool)
	GetAccountByUsername(ctx context.Context, username string) (domain.Account, string, bool)
	GetAccountByToken(ctx context.Context, token string) (domain.Account, bool)
	ListAccounts(ctx context.Context) []domain.Account
	ListAccountsPage(ctx context.Context, page int, pageSize int) ([]domain.Account, int)
	CreateAccount(ctx context.Context, input domain.AccountCreateInput) (domain.Account, error)
	UpdateAccountProfile(ctx context.Context, accountID string, displayName string, avatarURL string) (domain.Account, bool)
	UpdateAccountContact(ctx context.Context, accountID string, phone string, email string) (domain.Account, bool)
	UpdateAccountStatus(ctx context.Context, accountID string, status string) (domain.Account, bool)
	CreateAuthToken(ctx context.Context, accountID string) (string, error)
	DeleteAuthToken(ctx context.Context, token string) bool
	DeleteAuthTokensByAccount(ctx context.Context, accountID string) bool
	DeleteAccount(ctx context.Context, accountID string) bool

	// Agent 会话、消息、run 和 trace。
	ListUserSessions(ctx context.Context, accountID string) []domain.ChatSession
	SearchUserSessions(ctx context.Context, accountID string, keyword string, page int, pageSize int) ([]domain.ChatSession, int)
	CreateSession(ctx context.Context, accountID string, title string) (domain.ChatSession, error)
	GetSession(ctx context.Context, accountID string, sessionID string) (domain.ChatSession, bool)
	GetSessionDetail(ctx context.Context, accountID string, sessionID string) (domain.ChatSessionDetail, bool)
	UpdateSessionSummary(ctx context.Context, accountID string, sessionID string, title string, summary string) (domain.ChatSession, bool)
	PinSession(ctx context.Context, accountID string, sessionID string, pinned bool) (domain.ChatSession, bool)
	DeleteSession(ctx context.Context, accountID string, sessionID string) bool
	CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, bool, error)
	CreateRun(ctx context.Context, accountID string, sessionID string, messageID string) (domain.AgentRun, bool, error)
	UpdateRunStatus(ctx context.Context, accountID string, runID string, status domain.RunStatus) (domain.AgentRun, bool)
	UpdateRunResult(ctx context.Context, accountID string, runID string, content string, blocksJSON string, followupsJSON string, segmentsJSON string) bool
	ListRecentConversationRecords(ctx context.Context, accountID string, sessionID string, since time.Time, limit int) []domain.ConversationRecord
	IsRunCanceled(ctx context.Context, runID string) bool
	RecordAgentTrace(ctx context.Context, input domain.AgentTraceInput) error
	ListAgentTrace(ctx context.Context, accountID string, runID string) []domain.AgentTraceEvent
	ListAgentTraceByRun(ctx context.Context, runID string) []domain.AgentTraceEvent
	ListRecentAgentRuns(ctx context.Context, limit int) []domain.AgentRun
	ListAgentRunsPage(ctx context.Context, page int, pageSize int) ([]domain.AgentRun, int)
	ListAgentPromptsPage(ctx context.Context, page int, pageSize int) ([]domain.AgentPrompt, int)
	SeedAgentPrompts(ctx context.Context, defaults []domain.AgentPromptInput) error
	SaveAgentPromptDraft(ctx context.Context, input domain.AgentPromptInput) (domain.AgentPrompt, error)
	PublishAgentPrompt(ctx context.Context, promptKey string, publishedBy string, nacosDataID string) (domain.AgentPrompt, domain.AgentPromptPublishRecord, error)
	ListAgentPromptPublishRecords(ctx context.Context, promptKey string, limit int) []domain.AgentPromptPublishRecord

	// 商品、类目、商家和 SKU。
	SearchProducts(ctx context.Context, query string) []domain.ProductCard
	ListCategories(ctx context.Context) []domain.Category
	ListMerchants(ctx context.Context) []domain.Merchant
	ListAllMerchantsPage(ctx context.Context, page int, pageSize int) ([]domain.Merchant, int)
	UpdateMerchantStatus(ctx context.Context, merchantID string, status string) (domain.Merchant, bool)
	ListProducts(ctx context.Context, keyword string, categoryID string) []domain.ProductCard
	ListAllProducts(ctx context.Context) []domain.ProductCard
	ListAllProductsPage(ctx context.Context, page int, pageSize int) ([]domain.ProductCard, int)
	GetProduct(ctx context.Context, productID string) (domain.ProductDetail, bool)
	SearchProductsByImageVector(ctx context.Context, vector []float32, limit int) ([]domain.ProductCard, error)
	CreateProduct(ctx context.Context, input domain.ProductUpsertInput) (domain.ProductDetail, error)
	UpdateProduct(ctx context.Context, productID string, input domain.ProductUpsertInput) (domain.ProductDetail, bool)
	UpdateProductStatus(ctx context.Context, merchantID string, productID string, status string) (domain.ProductDetail, bool)
	ListProductSKUs(ctx context.Context, productID string) []domain.ProductSKU

	// 购物车、订单和虚拟支付。
	GetCart(ctx context.Context, accountID string) domain.Cart
	AddCartItem(ctx context.Context, accountID string, productID string, skuID string, quantity int) (domain.Cart, bool)
	UpdateCartItem(ctx context.Context, accountID string, cartItemID string, quantity *int, selected *bool) (domain.Cart, bool)
	DeleteCartItem(ctx context.Context, accountID string, cartItemID string) (domain.Cart, bool)
	PreviewCartDiscount(ctx context.Context, accountID string) domain.DiscountPreview
	CreateOrderFromCart(ctx context.Context, accountID string) ([]domain.Order, bool)
	GetOrder(ctx context.Context, accountID string, orderID string) (domain.Order, bool)
	PayOrder(ctx context.Context, accountID string, orderID string, method string) (domain.Order, domain.Payment, bool)
	CancelOrder(ctx context.Context, accountID string, orderID string, reason string) (domain.Order, bool)
	ConfirmReceipt(ctx context.Context, accountID string, orderID string) (domain.Order, bool)
	ExpirePendingOrders(ctx context.Context) int
	ListUserOrders(ctx context.Context, accountID string) []domain.Order
	ListMerchantOrders(ctx context.Context, merchantID string) []domain.Order
	ListAllOrders(ctx context.Context) []domain.Order
	ListAllOrdersPage(ctx context.Context, page int, pageSize int) ([]domain.Order, int)
	UpdateOrderStatus(ctx context.Context, merchantID string, orderID string, status string) (domain.Order, bool)

	// 促销、优惠券和评价。
	ListPromotions(ctx context.Context, merchantID string) []domain.PromotionRule
	CreatePromotion(ctx context.Context, input domain.PromotionRuleInput) (domain.PromotionRule, error)
	UpdatePromotionStatus(ctx context.Context, promotionID string, merchantID string, status string) (domain.PromotionRule, bool)
	ListCoupons(ctx context.Context, accountID string) []domain.Coupon
	ListUserCoupons(ctx context.Context, accountID string) []domain.UserCoupon
	ClaimCoupon(ctx context.Context, accountID string, couponID string) (domain.UserCoupon, bool)
	ListProductReviews(ctx context.Context, productID string) []domain.ProductReview
	CreateProductReview(ctx context.Context, accountID string, orderID string, orderItemID string, input domain.ProductReviewInput) (domain.ProductReview, bool)
	ListMerchantReviews(ctx context.Context, merchantID string) []domain.ProductReview
	ReplyReview(ctx context.Context, merchantID string, reviewID string, reply string) (domain.ProductReview, bool)
	ListAllReviews(ctx context.Context) []domain.ProductReview
	UpdateReviewStatus(ctx context.Context, reviewID string, status string) (domain.ProductReview, bool)

	// RAG 知识库。
	VectorIndexStatus(ctx context.Context) domain.VectorIndexStatus
	SearchKnowledge(ctx context.Context, query string) []domain.Citation
	SearchKnowledgeByPlan(ctx context.Context, plan rag.RetrievalPlan) []domain.Citation
	ListMerchantDocuments(ctx context.Context, merchantID string) []domain.KnowledgeDocument
	ListAllDocuments(ctx context.Context) []domain.KnowledgeDocument
	ListAllDocumentsPage(ctx context.Context, page int, pageSize int) ([]domain.KnowledgeDocument, int)
	CreateMerchantDocument(ctx context.Context, input domain.KnowledgeDocumentInput) (domain.KnowledgeDocument, error)

	// 文件与对象存储。
	CreateStoredFile(ctx context.Context, input domain.StoredFileInput) (domain.StoredFile, error)
	GetStoredFile(ctx context.Context, fileID string) (domain.StoredFile, bool)
}
