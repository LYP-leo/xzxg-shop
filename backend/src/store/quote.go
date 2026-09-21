package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/pricing"
)

var (
	ErrQuoteChanged      = errors.New("quote_changed")
	ErrQuoteExpired      = errors.New("quote_expired")
	ErrQuoteConfirmation = errors.New("quote_confirmation_required")
	ErrCartUnavailable   = errors.New("cart_unavailable")
)

type QuoteStore interface {
	CreateCartQuote(context.Context, string) (domain.CartQuote, error)
	CheckoutQuote(context.Context, string, domain.CheckoutConfirmation) ([]domain.Order, error)
}

type quoteRule struct {
	pricing.Rule
	StartAt   time.Time
	EndAt     time.Time
	UpdatedAt time.Time
}

func hashQuote(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }

func (s *MySQLStore) CreateCartQuote(ctx context.Context, accountID string) (domain.CartQuote, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CartQuote{}, err
	}
	defer tx.Rollback()
	version, err := lockCartRevision(ctx, tx, accountID)
	if err != nil {
		return domain.CartQuote{}, err
	}
	items, result, fingerprint, expires, err := s.priceCartTx(ctx, tx, accountID, version)
	if err != nil {
		return domain.CartQuote{}, err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return domain.CartQuote{}, err
	}
	quote := domain.CartQuote{AmountScope: "merchandise", FulfillmentStatus: "unconfirmed", QuoteID: nextID("quote"), CartVersion: version, Cart: buildCart(items), Currency: "CNY", PricingVersion: pricing.EngineVersion, ExpiresAt: expires,
		DiscountPreview: domain.DiscountPreview{TotalAmount: result.Total.String(), DiscountAmount: result.Discount.String(), PayAmount: result.Pay.String(), Lines: []domain.DiscountLine{}}}
	quote.Cart.Summary.DiscountAmount = result.Discount.String()
	quote.Cart.Summary.PayAmount = result.Pay.String()
	for _, line := range result.Applied {
		quote.Lines = append(quote.Lines, domain.DiscountLine{ID: line.ID, Type: line.Kind, Name: line.Name, Amount: line.Amount.String()})
	}
	payload, err := json.Marshal(quote)
	if err != nil {
		return domain.CartQuote{}, err
	}
	priceJSON, err := json.Marshal(result)
	if err != nil {
		return domain.CartQuote{}, err
	}
	quote.ConfirmationToken = hex.EncodeToString(token)
	if _, err := tx.ExecContext(ctx, `INSERT INTO cart_quotes
		(quote_id, account_id, cart_version, fingerprint, token_hash, payload, pricing_json, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, quote.QuoteID, accountID, version, fingerprint, hashQuote([]byte(quote.ConfirmationToken)), payload, priceJSON, expires, time.Now()); err != nil {
		return domain.CartQuote{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.CartQuote{}, err
	}
	return quote, nil
}

func (s *MySQLStore) CheckoutQuote(ctx context.Context, accountID string, confirmation domain.CheckoutConfirmation) ([]domain.Order, error) {
	if confirmation.QuoteID == "" || len(confirmation.ConfirmationToken) != 64 {
		return nil, ErrQuoteConfirmation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var version int64
	var fingerprint, tokenHash, status string
	var ordersJSON []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT cart_version, fingerprint, token_hash, status, orders_json, expires_at
		FROM cart_quotes WHERE quote_id = ? AND account_id = ? FOR UPDATE`, confirmation.QuoteID, accountID).
		Scan(&version, &fingerprint, &tokenHash, &status, &ordersJSON, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQuoteConfirmation
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(tokenHash), []byte(hashQuote([]byte(confirmation.ConfirmationToken)))) != 1 {
		return nil, ErrQuoteConfirmation
	}
	if status == "consumed" {
		var orders []domain.Order
		if err := json.Unmarshal(ordersJSON, &orders); err != nil {
			return nil, err
		}
		return orders, nil
	}
	if status != "open" || !time.Now().Before(expires) {
		return nil, ErrQuoteExpired
	}
	currentVersion, err := lockCartRevision(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	if currentVersion != version {
		return nil, ErrQuoteChanged
	}
	items, result, currentFingerprint, _, err := s.priceCartTx(ctx, tx, accountID, version)
	if err != nil {
		return nil, err
	}
	if currentFingerprint != fingerprint {
		return nil, ErrQuoteChanged
	}
	if !time.Now().Before(expires) {
		return nil, ErrQuoteExpired
	}
	orders, err := s.createQuotedOrdersTx(ctx, tx, accountID, confirmation.QuoteID, items, result)
	if err != nil {
		return nil, err
	}
	receipt, err := json.Marshal(orders)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cart_quotes SET status = 'consumed', orders_json = ? WHERE quote_id = ?`, receipt, confirmation.QuoteID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (s *MySQLStore) priceCartTx(ctx context.Context, tx *sql.Tx, accountID string, version int64) ([]domain.CartItem, pricing.Result, string, time.Time, error) {
	fail := func(err error) ([]domain.CartItem, pricing.Result, string, time.Time, error) {
		return nil, pricing.Result{}, "", time.Time{}, err
	}
	selected, err := s.lockSelectedCartItems(ctx, tx, accountID)
	if err != nil {
		return fail(err)
	}
	if len(selected) == 0 {
		return fail(ErrCartUnavailable)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cart_items WHERE account_id = ? AND selected = TRUE`, accountID).Scan(&count); err != nil {
		return fail(err)
	}
	if count != len(selected) {
		return fail(ErrCartUnavailable)
	}
	input := []pricing.Item{}
	remaining := make(map[string]int)
	for itemIndex, item := range selected {
		var category string
		var stock int
		if err := tx.QueryRowContext(ctx, `SELECT category_id, stock_quantity FROM products WHERE product_id = ?`, item.ProductID).Scan(&category, &stock); err != nil {
			return fail(err)
		}
		if available, seen := remaining[item.ProductID]; seen {
			stock = available
		}
		if stock < item.Quantity {
			return fail(ErrCartUnavailable)
		}
		remaining[item.ProductID] = stock - item.Quantity
		if item.SkuID != "" {
			var specsJSON []byte
			if err := tx.QueryRowContext(ctx, `SELECT stock_quantity, sku_name, specs_json FROM product_skus WHERE product_id = ? AND sku_id = ?`, item.ProductID, item.SkuID).Scan(&stock, &item.SkuName, &specsJSON); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fail(ErrCartUnavailable)
				}
				return fail(err)
			}
			if stock < item.Quantity {
				return fail(ErrCartUnavailable)
			}
			if err := json.Unmarshal(specsJSON, &item.Specs); err != nil {
				return fail(err)
			}
			selected[itemIndex] = item
		} else {
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM product_skus WHERE product_id = ?`, item.ProductID).Scan(&count); err != nil {
				return fail(err)
			}
			if count > 0 {
				return fail(ErrCartUnavailable)
			}
		}
		unit, err := pricing.ParseMoney(item.Price)
		if err != nil {
			return fail(err)
		}
		input = append(input, pricing.Item{ID: item.CartItemID, ProductID: item.ProductID, SkuID: item.SkuID, MerchantID: item.MerchantID, CategoryID: category, Unit: unit, Quantity: item.Quantity})
	}
	now := time.Now()
	rules, err := loadQuoteRules(ctx, tx, accountID, now)
	if err != nil {
		return fail(err)
	}
	plainRules := []pricing.Rule{}
	expires := now.Add(5 * time.Minute).Truncate(time.Second)
	for _, rule := range rules {
		plainRules = append(plainRules, rule.Rule)
		if rule.EndAt.Before(expires) {
			expires = rule.EndAt
		}
	}
	result, err := pricing.Calculate(input, plainRules)
	if err != nil {
		return fail(err)
	}
	data, err := json.Marshal(struct {
		Version int64
		Engine  string
		Items   []pricing.Item
		Cart    []domain.CartItem
		Rules   []quoteRule
		Result  pricing.Result
	}{version, pricing.EngineVersion, input, selected, rules, result})
	if err != nil {
		return fail(err)
	}
	return selected, result, hashQuote(data), expires, nil
}

func loadQuoteRules(ctx context.Context, tx *sql.Tx, accountID string, now time.Time) ([]quoteRule, error) {
	rules := []quoteRule{}
	queries := []struct {
		query string
		args  []any
		kind  string
	}{
		{`SELECT promotion_id, name, scope, merchant_id, product_id, category_id, type,
		threshold_amount, discount_amount, discount_rate, stackable, start_at, end_at, updated_at
		FROM promotion_rules WHERE status = 'active' AND start_at <= ? AND end_at > ?
		ORDER BY promotion_id FOR SHARE`, []any{now, now}, "promotion"},
		{`SELECT uc.user_coupon_id, c.name, c.scope, c.merchant_id, '', '', c.type,
		c.threshold_amount, c.discount_amount, '0.0000', c.stackable, c.start_at, c.end_at, c.updated_at
		FROM user_coupons uc JOIN coupons c ON c.coupon_id = uc.coupon_id
		WHERE uc.account_id = ? AND uc.status = 'unused' AND c.status = 'active' AND c.start_at <= ? AND c.end_at > ?
		ORDER BY uc.user_coupon_id FOR UPDATE`, []any{accountID, now, now}, "coupon"},
	}
	for _, q := range queries {
		rows, err := tx.QueryContext(ctx, q.query, q.args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var rule quoteRule
			var threshold, fixed, rate string
			rule.Kind = q.kind
			if err := rows.Scan(&rule.ID, &rule.Name, &rule.Scope, &rule.MerchantID, &rule.ProductID, &rule.CategoryID, &rule.Type, &threshold, &fixed, &rate, &rule.Stackable, &rule.StartAt, &rule.EndAt, &rule.UpdatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			var parseErr error
			rule.Threshold, parseErr = pricing.ParseMoney(threshold)
			if parseErr == nil {
				rule.Fixed, parseErr = pricing.ParseMoney(fixed)
			}
			if parseErr == nil {
				rule.RateBPS, parseErr = pricing.ParseRate(rate)
			}
			if parseErr != nil {
				rows.Close()
				return nil, fmt.Errorf("offer %s: %w", rule.ID, parseErr)
			}
			rules = append(rules, rule)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return rules, nil
}
