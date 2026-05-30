package store

import (
	"encoding/json"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestProductRefsFromBlocksJSON(t *testing.T) {
	blocks := []domain.AgentBlock{
		{Type: "product_refs", ProductIDs: []string{"p_mouse_001", "p_mouse_001"}},
		{Type: "product_card", Product: &domain.ProductCard{ProductID: "p_keyboard_001", SkuID: "sku_keyboard_001", Name: "Keyboard", Price: "199.00"}},
	}
	payload, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}

	ids, refs := productRefsFromBlocksJSON(string(payload))
	if got, want := len(ids), 2; got != want {
		t.Fatalf("ids length = %d, want %d: %#v", got, want, ids)
	}
	if ids[0] != "p_mouse_001" || ids[1] != "p_keyboard_001" {
		t.Fatalf("ids = %#v", ids)
	}
	if got, want := len(refs), 1; got != want {
		t.Fatalf("refs length = %d, want %d", got, want)
	}
	if refs[0].ProductID != "p_keyboard_001" || refs[0].SkuID != "sku_keyboard_001" {
		t.Fatalf("refs[0] = %#v", refs[0])
	}
}
