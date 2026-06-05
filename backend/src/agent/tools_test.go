package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestProductIDSetFromTextExtractsLiteralIDs(t *testing.T) {
	got := productIDSetFromText("把 p_dummyjson_016 和 p_mouse_001 加购")
	for _, id := range []string{"p_dummyjson_016", "p_mouse_001"} {
		if !got[id] {
			t.Fatalf("productIDSetFromText missing %s in %v", id, got)
		}
	}
}

func TestProductIDsFromTextKeepsFirstOccurrenceOrder(t *testing.T) {
	got := productIDsFromText("先看 p_mouse_001，再把 p_beauty_019 加购，别重复 p_mouse_001")
	want := []string{"p_mouse_001", "p_beauty_019"}
	if len(got) != len(want) {
		t.Fatalf("productIDsFromText len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("productIDsFromText[%d] = %s, want %s: %v", i, got[i], want[i], got)
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

func TestFirstImageAttachmentSupportsFileIDAliases(t *testing.T) {
	cases := []struct {
		name string
		in   domain.Attachment
		want string
	}{
		{name: "file_id", in: domain.Attachment{FileID: "file_new", Type: "image"}, want: "file_new"},
		{name: "attachment_id", in: domain.Attachment{AttachmentID: "file_old", Type: "image"}, want: "file_old"},
		{name: "file_url", in: domain.Attachment{URL: "/api/v1/files/file_url", Type: "image"}, want: "file_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := firstImageAttachment([]domain.Attachment{tc.in})
			if source == nil {
				t.Fatalf("firstImageAttachment returned nil")
			}
			if source.FileID != tc.want {
				t.Fatalf("FileID = %q, want %q", source.FileID, tc.want)
			}
		})
	}
}

func TestCartProductNameReturnsDisplayName(t *testing.T) {
	cart := &domain.Cart{Items: []domain.CartItem{
		{ProductID: "p_beauty_019", Name: "兰蔻清滢柔肤水大粉水保湿舒缓干性肌肤柔润爽肤水400ml"},
	}}
	got := cartProductName(cart, "p_beauty_019")
	if got != "兰蔻清滢柔肤水大粉水保湿舒缓干性肌肤柔润爽肤水400ml" {
		t.Fatalf("cartProductName = %q", got)
	}
}
