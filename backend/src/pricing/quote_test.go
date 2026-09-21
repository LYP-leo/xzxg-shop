package pricing

import (
	"math/rand"
	"testing"
)

func TestExactMoneyBoundaries(t *testing.T) {
	for _, value := range []string{"-1", "NaN", "1e3", "0.001", "100000000.00", "1.", "", "1,000", "+1"} {
		if _, err := ParseMoney(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	a, _ := ParseMoney("0.1")
	b, _ := ParseMoney("0.20")
	total, err := Add(a, b)
	if err != nil || total.String() != "0.30" {
		t.Fatalf("decimal addition = %s, %v", total, err)
	}
	if _, err := MaxMoney.Times(2); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestEligibilityAndBasketExclusivity(t *testing.T) {
	items := []Item{{ID: "a", ProductID: "phone", MerchantID: "m1", CategoryID: "digital", Unit: 10000, Quantity: 1}, {ID: "b", ProductID: "shirt", MerchantID: "m2", CategoryID: "clothes", Unit: 5000, Quantity: 1}}
	rules := []Rule{
		{ID: "product", Kind: "promotion", Scope: "product", ProductID: "phone", Type: "fixed", Fixed: 1000, Stackable: true},
		{ID: "category", Kind: "promotion", Scope: "category", CategoryID: "clothes", Type: "fixed", Fixed: 500, Stackable: true},
		{ID: "wrong_owner", Kind: "promotion", Scope: "category", MerchantID: "m1", CategoryID: "clothes", Type: "fixed", Fixed: 5000, Stackable: true},
		{ID: "not_eligible", Kind: "coupon", Scope: "merchant", MerchantID: "m2", Threshold: 5001, Type: "fixed", Fixed: 1000, Stackable: true},
		{ID: "exclusive", Kind: "promotion", Scope: "platform", Type: "fixed", Fixed: 1400},
	}
	got, err := Calculate(items, rules)
	if err != nil || got.Discount != 1500 || got.ItemDiscounts["a"] != 1000 || got.ItemDiscounts["b"] != 500 {
		t.Fatalf("wrong eligibility: %#v, %v", got, err)
	}
	rules[len(rules)-1].Fixed = 2000
	got, err = Calculate(items, rules)
	if err != nil || got.Discount != 2000 || len(got.Applied) != 1 || got.Applied[0].ID != "exclusive" {
		t.Fatalf("exclusive did not win: %#v %v", got, err)
	}
}

func TestDiscountAllocationConservesEveryCent(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 1000; trial++ {
		items := []Item{{ID: "a", Unit: Money(rng.Intn(100000) + 1), Quantity: 1}, {ID: "b", Unit: Money(rng.Intn(100000) + 1), Quantity: 3}, {ID: "c", Unit: Money(rng.Intn(100000) + 1), Quantity: 2}}
		result, err := Calculate(items, []Rule{{ID: "percent", Kind: "promotion", Scope: "platform", Type: "percentage", RateBPS: int64(rng.Intn(10001)), Stackable: true}, {ID: "fixed", Kind: "coupon", Scope: "platform", Type: "fixed", Fixed: Money(rng.Intn(100000)), Stackable: true}})
		if err != nil {
			t.Fatal(err)
		}
		var allocated Money
		for _, item := range items {
			amount, _ := item.Unit.Times(item.Quantity)
			discount := result.ItemDiscounts[item.ID]
			if discount < 0 || discount > amount {
				t.Fatalf("invalid line allocation: %#v", result)
			}
			allocated += discount
		}
		if allocated != result.Discount || result.Pay+allocated != result.Total {
			t.Fatalf("lost money: %#v", result)
		}
		for _, rule := range result.Applied {
			var sum Money
			for _, amount := range rule.ItemAmounts {
				sum += amount
			}
			if sum != rule.Amount {
				t.Fatalf("offer allocation mismatch: %#v", rule)
			}
		}
	}
}
