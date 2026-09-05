-- Existing installations: apply before deploying the new API with
-- RUN_MIGRATIONS=false. The application's Migrate method applies this same DDL.
-- No historical stock is changed by this migration.
CREATE TABLE IF NOT EXISTS inventory_reservations (
  order_item_id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL,
  product_id VARCHAR(64) NOT NULL,
  sku_id VARCHAR(64) NOT NULL DEFAULT '',
  quantity INT NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'reserved',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  INDEX idx_reservation_order (order_id, state)
) ENGINE=InnoDB;
