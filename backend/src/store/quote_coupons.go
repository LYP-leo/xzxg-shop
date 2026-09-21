package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/pricing"
)

func reserveOrderCoupons(ctx context.Context, tx *sql.Tx, accountID, quoteID, orderID string, items []domain.CartItem, result pricing.Result) error {
	for _, rule := range result.Applied {
		if rule.Kind != "coupon" {
			continue
		}
		var amount pricing.Money
		for _, item := range items {
			amount += rule.ItemAmounts[item.CartItemID]
		}
		if amount == 0 {
			continue
		}
		var status, group string
		if err := tx.QueryRowContext(ctx, `SELECT status, checkout_id FROM user_coupons
			WHERE user_coupon_id = ? AND account_id = ? FOR UPDATE`, rule.ID, accountID).Scan(&status, &group); err != nil {
			return err
		}
		if status == "unused" {
			if _, err := tx.ExecContext(ctx, `UPDATE user_coupons SET status = 'reserved', checkout_id = ?
				WHERE user_coupon_id = ? AND status = 'unused'`, quoteID, rule.ID); err != nil {
				return err
			}
		} else if status != "reserved" || group != quoteID {
			return ErrQuoteChanged
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_coupon_allocations
			(order_id, user_coupon_id, quote_id, amount_cents, state) VALUES (?, ?, ?, ?, 'reserved')`,
			orderID, rule.ID, quoteID, int64(amount)); err != nil {
			return err
		}
	}
	return nil
}

// Lock order -> coupon -> allocations in all payment/closure paths. One coupon
// may be apportioned to several merchant orders in the same confirmed checkout.
// Any paid allocation consumes the coupon; only a fully canceled unpaid group
// returns it to the wallet. Partial cancellation does not reprice remaining orders.
func transitionOrderCoupons(ctx context.Context, tx *sql.Tx, orderID, next string) error {
	if next != "consumed" && next != "released" {
		return fmt.Errorf("invalid coupon transition")
	}
	rows, err := tx.QueryContext(ctx, `SELECT user_coupon_id, quote_id FROM order_coupon_allocations
		WHERE order_id = ? AND state = 'reserved' ORDER BY user_coupon_id`, orderID)
	if err != nil {
		return err
	}
	groups := map[string]string{}
	for rows.Next() {
		var id, group string
		if err := rows.Scan(&id, &group); err != nil {
			rows.Close()
			return err
		}
		groups[id] = group
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var group, status string
		if err := tx.QueryRowContext(ctx, `SELECT checkout_id, status FROM user_coupons WHERE user_coupon_id = ? FOR UPDATE`, id).Scan(&group, &status); err != nil {
			return err
		}
		if group != groups[id] || (status != "reserved" && status != "used") {
			return fmt.Errorf("coupon reservation mismatch")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE order_coupon_allocations SET state = ?
			WHERE order_id = ? AND user_coupon_id = ? AND state = 'reserved'`, next, orderID, id); err != nil {
			return err
		}
		// A locking read observes the preceding payer/canceller after a lock wait,
		// even if this transaction began under REPEATABLE READ before it committed.
		states, err := tx.QueryContext(ctx, `SELECT state FROM order_coupon_allocations
			WHERE user_coupon_id = ? AND quote_id = ? FOR UPDATE`, id, group)
		if err != nil {
			return err
		}
		reserved, consumed := false, false
		for states.Next() {
			var state string
			if err := states.Scan(&state); err != nil {
				states.Close()
				return err
			}
			reserved = reserved || state == "reserved"
			consumed = consumed || state == "consumed"
		}
		err = states.Err()
		states.Close()
		if err != nil {
			return err
		}
		if consumed {
			_, err = tx.ExecContext(ctx, `UPDATE user_coupons SET status = 'used', order_id = IF(order_id = '', ?, order_id), used_at = COALESCE(used_at, ?) WHERE user_coupon_id = ?`, orderID, time.Now(), id)
		} else if !reserved {
			_, err = tx.ExecContext(ctx, `UPDATE user_coupons SET status = 'unused', checkout_id = '', order_id = '', used_at = NULL WHERE user_coupon_id = ?`, id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
