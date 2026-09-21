package httpapi

import (
	"errors"
	"net/http"

	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

func (s *Server) handleCreateCartQuote(w http.ResponseWriter, r *http.Request) {
	account, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	quotes, ok := s.store.(store.QuoteStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "quote_unavailable", "报价服务暂不可用")
		return
	}
	quote, err := quotes.CreateCartQuote(r.Context(), account.AccountID)
	if err != nil {
		writeQuoteError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, quote)
}

func writeQuoteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrQuoteConfirmation):
		writeError(w, http.StatusConflict, "quote_confirmation_required", "请先查看最新报价并确认提交")
	case errors.Is(err, store.ErrQuoteChanged):
		writeError(w, http.StatusConflict, "quote_changed", "商品、购物车或优惠已变化，请重新核对报价后提交")
	case errors.Is(err, store.ErrQuoteExpired):
		writeError(w, http.StatusConflict, "quote_expired", "报价已过期，请重新获取并确认")
	case errors.Is(err, store.ErrCartUnavailable):
		writeError(w, http.StatusConflict, "cart_unavailable", "请选择有效规格并检查库存，当前商品无法按原条件结算")
	default:
		writeError(w, http.StatusServiceUnavailable, "quote_unavailable", "报价或结算暂不可用，请稍后重试；请勿重复更换报价提交")
	}
}
