package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// One reservation per order item makes release/consumption auditable. The order
// row is the serialization point for payment, cancellation and expiry.
func (s *MySQLStore) ensureInventorySchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS inventory_reservations (
		order_item_id VARCHAR(64) PRIMARY KEY,
		order_id VARCHAR(64) NOT NULL,
		product_id VARCHAR(64) NOT NULL,
		sku_id VARCHAR(64) NOT NULL DEFAULT '',
		quantity INT NOT NULL,
		state VARCHAR(16) NOT NULL DEFAULT 'reserved',
		created_at DATETIME(6) NOT NULL,
		updated_at DATETIME(6) NOT NULL,
		INDEX idx_reservation_order (order_id, state)
	) ENGINE=InnoDB`)
	return err
}

// Old pending orders already deducted stock. Adopt their items while holding
// the order lock, without deducting again. Historical closed orders require an
// operator audit: their original stock cannot safely be inferred or fabricated.
func adoptOrderReservations(ctx context.Context, tx *sql.Tx, orderID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO inventory_reservations
		(order_item_id, order_id, product_id, sku_id, quantity, state, created_at, updated_at)
		SELECT i.order_item_id, i.order_id, i.product_id, COALESCE(i.sku_id, ''), i.quantity,
		       'reserved', NOW(6), NOW(6)
		FROM order_items i
		WHERE i.order_id = ? AND NOT EXISTS (
			SELECT 1 FROM inventory_reservations r WHERE r.order_item_id = i.order_item_id
		)`, orderID)
	return err
}

func releaseOrderReservations(ctx context.Context, tx *sql.Tx, orderID string) error {
	if err := adoptOrderReservations(ctx, tx, orderID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT order_item_id, product_id, sku_id, quantity
		FROM inventory_reservations WHERE order_id = ? AND state = 'reserved'
		ORDER BY product_id, sku_id, order_item_id FOR UPDATE`, orderID)
	if err != nil {
		return err
	}
	type reservation struct {
		item, product, sku string
		quantity           int
	}
	items := []reservation{}
	for rows.Next() {
		var item reservation
		if err := rows.Scan(&item.item, &item.product, &item.sku, &item.quantity); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.quantity <= 0 {
			return fmt.Errorf("invalid reservation quantity: %s", item.item)
		}
		if item.sku != "" {
			result, err := tx.ExecContext(ctx, `UPDATE product_skus
				SET stock_quantity = stock_quantity + ?, stock_status = 'in_stock', updated_at = NOW()
				WHERE product_id = ? AND sku_id = ?`, item.quantity, item.product, item.sku)
			if err := requireUpdatedRow(result, err); err != nil {
				return err
			}
		}
		result, err := tx.ExecContext(ctx, `UPDATE products
			SET stock_quantity = stock_quantity + ?, stock_status = 'in_stock', updated_at = NOW()
			WHERE product_id = ?`, item.quantity, item.product)
		if err := requireUpdatedRow(result, err); err != nil {
			return err
		}
		result, err = tx.ExecContext(ctx, `UPDATE inventory_reservations
			SET state = 'released', updated_at = NOW(6) WHERE order_item_id = ? AND state = 'reserved'`, item.item)
		if err := requireUpdatedRow(result, err); err != nil {
			return err
		}
	}
	return nil
}

func requireUpdatedRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("expected one affected row, got %d", count)
	}
	return nil
}

// Caller must hold the pending order's row lock. Every related mutation commits
// together; a failure leaves the order payable and its stock still reserved.
func closePendingOrderTx(ctx context.Context, tx *sql.Tx, orderID, status, reason string) error {
	if status != "canceled" && status != "closed_timeout" {
		return fmt.Errorf("invalid close status")
	}
	if err := releaseOrderReservations(ctx, tx, orderID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE orders SET status = ?, closed_at = NOW(),
		cancel_reason = ?, updated_at = NOW() WHERE order_id = ? AND status = 'pending_payment'`,
		status, truncateRunes(reason, 256), orderID)
	if err := requireUpdatedRow(result, err); err != nil {
		return err
	}
	paymentStatus := "failed"
	if status == "closed_timeout" {
		paymentStatus = "expired"
	}
	_, err = tx.ExecContext(ctx, `UPDATE payments SET status = ?, updated_at = NOW()
		WHERE order_id = ? AND status = 'pending'`, paymentStatus, orderID)
	return err
}

// ExpirePendingOrdersBatch is bounded so a sweeper cannot monopolize the pool.
// Competing sweepers and payment/cancel requests all serialize on the order row.
func (s *MySQLStore) ExpirePendingOrdersBatch(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT order_id FROM orders
		WHERE status = 'pending_payment' AND payment_deadline_at <= ?
		ORDER BY payment_deadline_at, order_id LIMIT 128`, time.Now())
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	closed := 0
	for _, id := range ids {
		didClose, err := s.expireOrder(ctx, id)
		if err != nil {
			return closed, err
		}
		if didClose {
			closed++
		}
	}
	return closed, nil
}

func (s *MySQLStore) expireOrder(ctx context.Context, orderID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var status string
	var deadline sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT status, payment_deadline_at FROM orders
		WHERE order_id = ? FOR UPDATE`, orderID).Scan(&status, &deadline); err != nil {
		return false, err
	}
	if status != "pending_payment" || !deadline.Valid || time.Now().Before(deadline.Time) {
		return false, nil
	}
	if err := closePendingOrderTx(ctx, tx, orderID, "closed_timeout", "支付超时自动关闭"); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
