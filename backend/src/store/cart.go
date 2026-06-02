package store

import (
	"fmt"
	"strconv"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

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
