package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

type quoteHandlerStore struct {
	store.Store
	called       bool
	owner        string
	confirmation domain.CheckoutConfirmation
}

func (q *quoteHandlerStore) CreateCartQuote(context.Context, string) (domain.CartQuote, error) {
	return domain.CartQuote{}, nil
}
func (q *quoteHandlerStore) CheckoutQuote(_ context.Context, owner string, confirmation domain.CheckoutConfirmation) ([]domain.Order, error) {
	q.called = true
	q.owner = owner
	q.confirmation = confirmation
	if confirmation.QuoteID == "" || confirmation.ConfirmationToken == "" {
		return nil, store.ErrQuoteConfirmation
	}
	return []domain.Order{{OrderID: "confirmed_order"}}, nil
}

func TestCheckoutRequiresAuthenticatedConfirmation(t *testing.T) {
	fake := &quoteHandlerStore{}
	s := &Server{store: fake}
	for _, tc := range []struct {
		body   string
		role   domain.AccountRole
		status int
	}{
		{`{}`, domain.AccountRoleUser, http.StatusConflict},
		{`{"quote_id":"q","confirmation_token":"token","account_id":"attacker"}`, domain.AccountRoleUser, http.StatusOK},
		{`{"quote_id":"q","confirmation_token":"token"}`, domain.AccountRoleMerchant, http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/orders:checkout", strings.NewReader(tc.body))
		request = request.WithContext(context.WithValue(request.Context(), accountKey, domain.Account{AccountID: "signed_in_user", Role: tc.role}))
		response := httptest.NewRecorder()
		s.handleCheckout(response, request)
		if response.Code != tc.status {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if tc.status == http.StatusOK && fake.owner != "signed_in_user" {
			t.Fatal("used client supplied owner")
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders:checkout", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	s.handleCheckout(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated checkout")
	}
}

// Temporary storage failures preserve the client's quote identity for safe retry.
func TestQuoteErrorsDistinguishRetryFromReconfirmation(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errors.New("database unavailable"), http.StatusServiceUnavailable, "quote_unavailable"},
		{store.ErrCartUnavailable, http.StatusConflict, "cart_unavailable"},
		{store.ErrQuoteChanged, http.StatusConflict, "quote_changed"},
		{store.ErrQuoteExpired, http.StatusConflict, "quote_expired"},
	} {
		response := httptest.NewRecorder()
		writeQuoteError(response, tc.err)
		if response.Code != tc.status || !strings.Contains(response.Body.String(), tc.code) {
			t.Fatalf("wrong recovery contract: %d %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "database unavailable") {
			t.Fatal("internal storage failure leaked")
		}
	}
}
