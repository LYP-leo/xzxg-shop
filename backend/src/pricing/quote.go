package pricing

import (
	"fmt"
	"math/big"
	"sort"
)

const EngineVersion = "cny-basket-v1"

type Item struct {
	ID, ProductID, SkuID, MerchantID, CategoryID string
	Unit                                         Money
	Quantity                                     int
}

type Rule struct {
	ID, Kind, Name, Scope, MerchantID, ProductID, CategoryID, Type string
	Threshold, Fixed                                               Money
	RateBPS                                                        int64
	Stackable                                                      bool
}

type AppliedRule struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind"`
	Name        string           `json:"name"`
	Amount      Money            `json:"amount_cents"`
	ItemAmounts map[string]Money `json:"item_amounts"`
}

type Result struct {
	Total         Money            `json:"total_cents"`
	Discount      Money            `json:"discount_cents"`
	Pay           Money            `json:"pay_cents"`
	ItemDiscounts map[string]Money `json:"item_discounts"`
	Applied       []AppliedRule    `json:"applied"`
}

// The contract is basket-wide exclusivity: a non-stackable offer cannot combine
// with any other offer in this checkout. Choose the better of the stackable
// bundle and each exclusive offer. Thresholds use the original eligible subtotal;
// discounts apply to the remaining eligible amount, in stable rule-ID order.
func Calculate(input []Item, rules []Rule) (Result, error) {
	items := append([]Item(nil), input...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	seen := map[string]bool{}
	var total Money
	for _, item := range items {
		if item.ID == "" || seen[item.ID] {
			return Result{}, fmt.Errorf("invalid or duplicate cart line")
		}
		seen[item.ID] = true
		amount, err := item.Unit.Times(item.Quantity)
		if err != nil {
			return Result{}, err
		}
		total, err = Add(total, amount)
		if err != nil {
			return Result{}, err
		}
	}
	rules = append([]Rule(nil), rules...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Kind+rules[i].ID < rules[j].Kind+rules[j].ID })
	stackable := []Rule{}
	seenRules := map[string]bool{}
	for _, rule := range rules {
		key := rule.Kind + ":" + rule.ID
		if rule.ID == "" || seenRules[key] {
			return Result{}, fmt.Errorf("invalid or duplicate offer")
		}
		seenRules[key] = true
		if err := validateRule(rule); err != nil {
			return Result{}, err
		}
		if rule.Stackable {
			stackable = append(stackable, rule)
		}
	}
	best := apply(items, stackable, total)
	for _, rule := range rules {
		if rule.Stackable {
			continue
		}
		candidate := apply(items, []Rule{rule}, total)
		if candidate.Discount > best.Discount {
			best = candidate
		}
	}
	return best, nil
}

func validateRule(rule Rule) error {
	validScope := rule.Scope == "platform" || rule.Scope == "global" ||
		(rule.Scope == "merchant" && rule.MerchantID != "") ||
		(rule.Scope == "product" && rule.ProductID != "") ||
		(rule.Scope == "category" && rule.CategoryID != "")
	if !validScope || rule.Threshold < 0 || rule.Fixed < 0 || rule.Threshold > MaxMoney || rule.Fixed > MaxMoney || rule.RateBPS < 0 || rule.RateBPS > 10000 {
		return fmt.Errorf("invalid offer %s", rule.ID)
	}
	switch rule.Type {
	case "fixed", "fixed_amount", "full_reduction", "amount", "percentage":
		return nil
	}
	return fmt.Errorf("unsupported discount type %q", rule.Type)
}

func matches(item Item, rule Rule) bool {
	// Merchant ownership narrows product/category offers too, preventing an
	// unrelated merchant's similarly named category from receiving its discount.
	if rule.MerchantID != "" && rule.MerchantID != item.MerchantID {
		return false
	}
	switch rule.Scope {
	case "platform", "global":
		return true
	case "merchant":
		return item.MerchantID == rule.MerchantID
	case "product":
		return item.ProductID == rule.ProductID
	case "category":
		return item.CategoryID == rule.CategoryID
	}
	return false
}

func apply(items []Item, rules []Rule, total Money) Result {
	result := Result{Total: total, Pay: total, ItemDiscounts: map[string]Money{}, Applied: []AppliedRule{}}
	for _, rule := range rules {
		var original, remaining Money
		for _, item := range items {
			if matches(item, rule) {
				amount, _ := item.Unit.Times(item.Quantity)
				original += amount
				remaining += amount - result.ItemDiscounts[item.ID]
			}
		}
		if original < rule.Threshold || remaining <= 0 {
			continue
		}
		discount := min(rule.Fixed, remaining)
		if rule.Type == "percentage" {
			discount = Money((int64(remaining)*rule.RateBPS + 5000) / 10000)
		}
		if discount <= 0 {
			continue
		}
		applied := AppliedRule{ID: rule.ID, Kind: rule.Kind, Name: rule.Name, Amount: discount, ItemAmounts: map[string]Money{}}
		left, base := discount, remaining
		for _, item := range items {
			if !matches(item, rule) {
				continue
			}
			amount, _ := item.Unit.Times(item.Quantity)
			amount -= result.ItemDiscounts[item.ID]
			if amount == 0 {
				continue
			}
			// Avoid multiplication overflow for DECIMAL(10,2) basket amounts.
			n := new(big.Int).Mul(big.NewInt(int64(left)), big.NewInt(int64(amount)))
			share := Money(n.Quo(n, big.NewInt(int64(base))).Int64())
			applied.ItemAmounts[item.ID] = share
			result.ItemDiscounts[item.ID] += share
			left -= share
			base -= amount
		}
		result.Discount += discount
		result.Applied = append(result.Applied, applied)
	}
	result.Pay = result.Total - result.Discount
	return result
}
