package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type checkoutProposalStore struct {
	store.Store
	submitted bool
}

func (s *checkoutProposalStore) CreateCartQuote(context.Context, string) (domain.CartQuote, error) {
	return domain.CartQuote{QuoteID: "q", ConfirmationToken: "private-ui-confirmation", DiscountPreview: domain.DiscountPreview{PayAmount: "12.34"}}, nil
}
func (s *checkoutProposalStore) CheckoutQuote(context.Context, string, domain.CheckoutConfirmation) ([]domain.Order, error) {
	s.submitted = true
	return nil, nil
}

func TestAgentCheckoutCannotConfirmItsOwnQuoteOrExposeToken(t *testing.T) {
	fake := &checkoutProposalStore{}
	runtime := &Runtime{store: fake}
	observation := runtime.toolCheckout(context.Background(), "user")
	payload, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if fake.submitted || !observation.OK || len(observation.Orders) > 0 {
		t.Fatal("agent submitted its own proposal")
	}
	if strings.Contains(string(payload), "private-ui-confirmation") || strings.Contains(string(payload), "confirmation_token") {
		t.Fatal("credential exposed in observation/trace")
	}
	if observation.Result["requires_confirmation"] != true || observation.Result["order_created"] != false {
		t.Fatal("ambiguous operation result")
	}
}
