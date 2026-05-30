package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestProductIDSetFromTextExtractsLiteralIDs(t *testing.T) {
	got := productIDSetFromText("把 p_dummyjson_016 和 p_mouse_001 加购")
	for _, id := range []string{"p_dummyjson_016", "p_mouse_001"} {
		if !got[id] {
			t.Fatalf("productIDSetFromText missing %s in %v", id, got)
		}
	}
}

func TestToolAddCartItemRejectsUnallowedProductID(t *testing.T) {
	runtime := &Runtime{}
	raw, _ := json.Marshal(map[string]any{
		"product_id": "p_mouse_001",
		"sku_id":     "sku_mouse_001",
		"quantity":   1,
	})

	observation := runtime.toolAddCartItem(context.Background(), "acct_001", raw, map[string]bool{})
	if observation.OK {
		t.Fatalf("toolAddCartItem OK = true, want false")
	}
	if observation.Message == "" {
		t.Fatalf("toolAddCartItem message is empty")
	}
}
