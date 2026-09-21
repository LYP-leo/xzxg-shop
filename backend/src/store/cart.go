package store

import (
	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/pricing"
)

func buildCart(items []domain.CartItem) domain.Cart {
	copied := append([]domain.CartItem{}, items...)
	var total pricing.Money
	selectedCount := 0
	for _, item := range copied {
		if !item.Selected {
			continue
		}
		unit, err := pricing.ParseMoney(item.Price)
		if err != nil {
			return domain.Cart{Items: copied, Summary: domain.CartSummary{ErrorCode: "invalid_cart_amount"}}
		}
		amount, err := unit.Times(item.Quantity)
		if err != nil {
			return domain.Cart{Items: copied, Summary: domain.CartSummary{ErrorCode: "invalid_cart_amount"}}
		}
		total, err = pricing.Add(total, amount)
		if err != nil {
			return domain.Cart{Items: copied, Summary: domain.CartSummary{ErrorCode: "invalid_cart_amount"}}
		}
		selectedCount += item.Quantity
	}
	return domain.Cart{Items: copied, Summary: domain.CartSummary{
		SelectedCount: selectedCount, TotalAmount: total.String(), DiscountAmount: "0.00", PayAmount: total.String(),
	}}
}
