package store

import (
	"context"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type Store interface {
	GetAccountByUsername(ctx context.Context, username string) (domain.Account, string, bool)
	GetAccountByToken(ctx context.Context, token string) (domain.Account, bool)
	CreateAuthToken(ctx context.Context, accountID string) (string, error)
	CreateSession(ctx context.Context, title string) (domain.ChatSession, error)
	GetSession(ctx context.Context, sessionID string) (domain.ChatSession, bool)
	CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, error)
	CreateRun(ctx context.Context, sessionID string, messageID string) (domain.AgentRun, error)
	UpdateRunStatus(ctx context.Context, runID string, status domain.RunStatus) (domain.AgentRun, bool)
	IsRunCanceled(ctx context.Context, runID string) bool
	SearchProducts(ctx context.Context, query string) []domain.ProductCard
	ListCategories(ctx context.Context) []domain.Category
	ListMerchants(ctx context.Context) []domain.Merchant
	ListProducts(ctx context.Context, keyword string, categoryID string) []domain.ProductCard
	GetProduct(ctx context.Context, productID string) (domain.ProductDetail, bool)
	ListProductSKUs(ctx context.Context, productID string) []domain.ProductSKU
	GetCart(ctx context.Context) domain.Cart
	AddCartItem(ctx context.Context, productID string, skuID string, quantity int) (domain.Cart, bool)
	UpdateCartItem(ctx context.Context, cartItemID string, quantity *int, selected *bool) (domain.Cart, bool)
	DeleteCartItem(ctx context.Context, cartItemID string) (domain.Cart, bool)
	SearchKnowledge(ctx context.Context, query string) []domain.Citation
}
