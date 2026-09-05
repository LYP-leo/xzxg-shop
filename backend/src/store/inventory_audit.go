package store

import "context"

type InventoryAuditIssue struct {
	Kind      string `json:"kind"`
	OrderID   string `json:"order_id,omitempty"`
	ProductID string `json:"product_id,omitempty"`
	SkuID     string `json:"sku_id,omitempty"`
}

// Audit only: historical stock cannot be reconstructed from order status alone.
// Do not "repair" ambiguous historical rows by increasing sellable inventory.
func (s *MySQLStore) InventoryAudit(ctx context.Context) ([]InventoryAuditIssue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT 'reservation_mismatch', r.order_id, r.product_id, r.sku_id
		FROM inventory_reservations r
		LEFT JOIN orders o ON o.order_id = r.order_id
		LEFT JOIN order_items i ON i.order_item_id = r.order_item_id
		WHERE o.order_id IS NULL OR i.order_item_id IS NULL OR i.order_id <> r.order_id
		   OR r.product_id <> i.product_id OR r.sku_id <> COALESCE(i.sku_id, '')
		   OR r.quantity <> i.quantity OR r.quantity <= 0
		   OR (o.status = 'pending_payment' AND r.state <> 'reserved')
		   OR (o.status IN ('pending_ship', 'shipped', 'completed') AND r.state <> 'consumed')
		   OR (o.status IN ('canceled', 'closed_timeout') AND r.state <> 'released')
		   OR r.state NOT IN ('reserved', 'consumed', 'released')
		UNION ALL
		SELECT CASE WHEN o.status = 'pending_payment' THEN 'legacy_pending_reservation'
		            ELSE 'legacy_order_requires_reconciliation' END,
		       i.order_id, i.product_id, COALESCE(i.sku_id, '')
		FROM order_items i JOIN orders o ON o.order_id = i.order_id
		LEFT JOIN inventory_reservations r ON r.order_item_id = i.order_item_id
		WHERE r.order_item_id IS NULL
		UNION ALL
		SELECT 'negative_product_stock', '', product_id, '' FROM products WHERE stock_quantity < 0
		UNION ALL
		SELECT 'negative_sku_stock', '', product_id, sku_id FROM product_skus WHERE stock_quantity < 0
		LIMIT 1001`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []InventoryAuditIssue{}
	for rows.Next() {
		var issue InventoryAuditIssue
		if err := rows.Scan(&issue.Kind, &issue.OrderID, &issue.ProductID, &issue.SkuID); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}
