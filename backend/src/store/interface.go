package store

import (
	"context"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
)

type Store interface {
	GetAccountByUsername(ctx context.Context, username string) (domain.Account, string, bool)
	GetAccountByToken(ctx context.Context, token string) (domain.Account, bool)
	CreateAccount(ctx context.Context, input domain.AccountCreateInput, passwordHash string) (domain.Account, error)
	ListAccounts(ctx context.Context) []domain.Account
	UpdateAccountStatus(ctx context.Context, accountID string, status string) (domain.Account, bool)
	UpdateAccountProfile(ctx context.Context, accountID string, nickname string) (domain.Account, bool)
	UpdateAccountPassword(ctx context.Context, accountID string, passwordHash string) bool
	CreateAuthToken(ctx context.Context, accountID string) (string, error)
	ListUserSessions(ctx context.Context, accountID string) []domain.ChatSession
	CreateSession(ctx context.Context, accountID string, title string) (domain.ChatSession, error)
	GetSession(ctx context.Context, accountID string, sessionID string) (domain.ChatSession, bool)
	GetSessionDetail(ctx context.Context, accountID string, sessionID string) (domain.ChatSessionDetail, bool)
	CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, error)
	CreateRun(ctx context.Context, accountID string, sessionID string, messageID string) (domain.AgentRun, error)
	UpdateRunStatus(ctx context.Context, accountID string, runID string, status domain.RunStatus) (domain.AgentRun, bool)
	IsRunCanceled(ctx context.Context, runID string) bool
	RecordAgentTrace(ctx context.Context, input domain.AgentTraceInput) error
	ListAgentTrace(ctx context.Context, accountID string, runID string) []domain.AgentTraceEvent
	SearchProducts(ctx context.Context, query string) []domain.ProductCard
	ListCategories(ctx context.Context) []domain.Category
	ListMerchants(ctx context.Context) []domain.Merchant
	ListProducts(ctx context.Context, keyword string, categoryID string) []domain.ProductCard
	ListProductsPage(ctx context.Context, keyword string, categoryID string, limit int, offset int) ([]domain.ProductCard, bool)
	ListAllProducts(ctx context.Context) []domain.ProductCard
	GetProduct(ctx context.Context, productID string) (domain.ProductDetail, bool)
	CreateProduct(ctx context.Context, input domain.ProductUpsertInput) (domain.ProductDetail, error)
	UpdateProduct(ctx context.Context, productID string, input domain.ProductUpsertInput) (domain.ProductDetail, bool)
	UpdateProductStatus(ctx context.Context, merchantID string, productID string, status string) (domain.ProductDetail, bool)
	ListProductSKUs(ctx context.Context, productID string) []domain.ProductSKU
	GetCart(ctx context.Context, accountID string) domain.Cart
	AddCartItem(ctx context.Context, accountID string, productID string, skuID string, quantity int) (domain.Cart, bool)
	UpdateCartItem(ctx context.Context, accountID string, cartItemID string, quantity *int, selected *bool) (domain.Cart, bool)
	DeleteCartItem(ctx context.Context, accountID string, cartItemID string) (domain.Cart, bool)
	CreateOrderFromCart(ctx context.Context, accountID string) ([]domain.Order, bool)
	ListUserOrders(ctx context.Context, accountID string) []domain.Order
	ListMerchantOrders(ctx context.Context, merchantID string) []domain.Order
	ListAllOrders(ctx context.Context) []domain.Order
	UpdateOrderStatus(ctx context.Context, merchantID string, orderID string, status string) (domain.Order, bool)
	SearchKnowledge(ctx context.Context, query string) []domain.Citation
	SearchKnowledgeByPlan(ctx context.Context, plan rag.RetrievalPlan) []domain.Citation
	ListMerchantDocuments(ctx context.Context, merchantID string) []domain.KnowledgeDocument
	ListAllDocuments(ctx context.Context) []domain.KnowledgeDocument
	CreateMerchantDocument(ctx context.Context, input domain.KnowledgeDocumentInput) (domain.KnowledgeDocument, error)
}
