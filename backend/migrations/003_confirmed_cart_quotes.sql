-- Existing installations: apply once in a maintenance window after 002.
-- The Go Migrate path checks columns before ALTER and is safe to repeat.
CREATE TABLE IF NOT EXISTS cart_revisions (
  account_id VARCHAR(64) PRIMARY KEY,
  version BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS cart_quotes (
  quote_id VARCHAR(64) PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL,
  cart_version BIGINT NOT NULL,
  fingerprint CHAR(64) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  payload JSON NOT NULL,
  pricing_json JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'open',
  orders_json JSON NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  INDEX idx_quote_owner (account_id, created_at),
  INDEX idx_quote_expiry (status, expires_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_coupon_allocations (
  order_id VARCHAR(64) NOT NULL,
  user_coupon_id VARCHAR(64) NOT NULL,
  quote_id VARCHAR(64) NOT NULL,
  amount_cents BIGINT NOT NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'reserved',
  PRIMARY KEY (order_id, user_coupon_id),
  INDEX idx_coupon_allocation (user_coupon_id, quote_id, state)
) ENGINE=InnoDB;

ALTER TABLE coupons ADD COLUMN stackable BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_coupons ADD COLUMN checkout_id VARCHAR(64) NOT NULL DEFAULT '';
