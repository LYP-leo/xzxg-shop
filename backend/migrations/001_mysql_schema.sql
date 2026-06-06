CREATE TABLE IF NOT EXISTS merchants (
  merchant_id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  logo_url VARCHAR(512) NOT NULL DEFAULT '',
  description TEXT NOT NULL,
  service_phone VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS categories (
  category_id VARCHAR(64) PRIMARY KEY,
  parent_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_categories_parent_id (parent_id)
);

CREATE TABLE IF NOT EXISTS products (
  product_id VARCHAR(64) PRIMARY KEY,
  merchant_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  brand VARCHAR(64) NOT NULL,
  category_id VARCHAR(64) NOT NULL,
  image_url VARCHAR(512) NOT NULL,
  image_urls_json JSON NOT NULL,
  price DECIMAL(10, 2) NOT NULL,
  market_price DECIMAL(10, 2) NOT NULL,
  stock_quantity INT NOT NULL,
  stock_status VARCHAR(32) NOT NULL,
  tags_json JSON NOT NULL,
  selling_points_json JSON NOT NULL,
  recommend_reason TEXT NOT NULL,
  risk_notes_json JSON NOT NULL,
  attributes_json JSON NOT NULL,
  suitable_for_json JSON NOT NULL,
  not_suitable_for_json JSON NOT NULL,
  description TEXT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_products_category_id (category_id),
  INDEX idx_products_merchant_id (merchant_id),
  INDEX idx_products_status (status)
);

CREATE TABLE IF NOT EXISTS product_skus (
  sku_id VARCHAR(64) PRIMARY KEY,
  product_id VARCHAR(64) NOT NULL,
  sku_name VARCHAR(128) NOT NULL,
  price DECIMAL(10, 2) NOT NULL,
  stock_quantity INT NOT NULL,
  stock_status VARCHAR(32) NOT NULL,
  specs_json JSON NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_product_skus_product_id (product_id)
);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
  chunk_id VARCHAR(64) PRIMARY KEY,
  title VARCHAR(256) NOT NULL,
  snippet TEXT NOT NULL,
  source VARCHAR(256) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS knowledge_documents (
  document_id VARCHAR(64) PRIMARY KEY,
  merchant_id VARCHAR(64) NOT NULL,
  title VARCHAR(256) NOT NULL,
  doc_type VARCHAR(64) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  chunk_count INT NOT NULL DEFAULT 0,
  source_url VARCHAR(1024) NOT NULL DEFAULT '',
  content_hash VARCHAR(64) NOT NULL DEFAULT '',
  metadata_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_knowledge_documents_merchant_id (merchant_id),
  INDEX idx_knowledge_documents_status (status),
  INDEX idx_knowledge_documents_content_hash (content_hash)
);

CREATE TABLE IF NOT EXISTS accounts (
  account_id VARCHAR(64) PRIMARY KEY,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  avatar_url VARCHAR(512) NOT NULL DEFAULT '',
  phone VARCHAR(32) NOT NULL DEFAULT '',
  email VARCHAR(128) NOT NULL DEFAULT '',
  role VARCHAR(32) NOT NULL,
  merchant_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  deleted_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_accounts_username (username),
  INDEX idx_accounts_status (status),
  INDEX idx_accounts_role (role)
);

CREATE TABLE IF NOT EXISTS auth_tokens (
  token VARCHAR(96) PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME NOT NULL,
  INDEX idx_auth_tokens_account_id (account_id),
  INDEX idx_auth_tokens_expires_at (expires_at)
);

CREATE TABLE IF NOT EXISTS cart_items (
  cart_item_id VARCHAR(64) PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  product_id VARCHAR(64) NOT NULL,
  sku_id VARCHAR(64) NOT NULL DEFAULT '',
  quantity INT NOT NULL,
  selected BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_cart_account_product_sku (account_id, product_id, sku_id),
  INDEX idx_cart_items_account_id (account_id),
  INDEX idx_cart_items_product_id (product_id)
);

CREATE TABLE IF NOT EXISTS chat_sessions (
  session_id VARCHAR(64) PRIMARY KEY,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  title VARCHAR(128) NOT NULL,
  summary TEXT,
  message_count INT NOT NULL DEFAULT 0,
  last_message_at DATETIME NULL,
  pinned_at DATETIME NULL,
  deleted_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_chat_sessions_account_id (account_id),
  INDEX idx_chat_sessions_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS user_messages (
  message_id VARCHAR(64) PRIMARY KEY,
  session_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  client_message_id VARCHAR(128) NOT NULL DEFAULT '',
  content TEXT NOT NULL,
  attachments_json JSON NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_user_messages_account_id (account_id),
  INDEX idx_user_messages_session_id (session_id),
  INDEX idx_user_messages_client_message_id (client_message_id),
  UNIQUE KEY uk_user_messages_client_message (account_id, session_id, client_message_id)
);

CREATE TABLE IF NOT EXISTS agent_runs (
  run_id VARCHAR(64) PRIMARY KEY,
  session_id VARCHAR(64) NOT NULL,
  message_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  trace_id VARCHAR(64) NOT NULL,
  content TEXT,
  blocks_json JSON NULL,
  followups_json JSON NULL,
  segments_json JSON NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_agent_runs_account_id (account_id),
  INDEX idx_agent_runs_session_id (session_id),
  INDEX idx_agent_runs_message_id (message_id),
  UNIQUE KEY uk_agent_runs_message (account_id, message_id)
);

CREATE TABLE IF NOT EXISTS agent_trace_events (
  trace_event_id VARCHAR(64) PRIMARY KEY,
  run_id VARCHAR(64) NOT NULL,
  trace_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  stage VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  model VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  error TEXT,
  metadata_json JSON,
  created_at DATETIME NOT NULL,
  INDEX idx_agent_trace_run_id (run_id),
  INDEX idx_agent_trace_trace_id (trace_id),
  INDEX idx_agent_trace_account_id (account_id),
  INDEX idx_agent_trace_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS agent_prompts (
  prompt_id VARCHAR(64) PRIMARY KEY,
  prompt_key VARCHAR(128) NOT NULL,
  title VARCHAR(128) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  version INT NOT NULL,
  description TEXT,
  created_by VARCHAR(64) NOT NULL DEFAULT '',
  published_at DATETIME NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_agent_prompts_key_version (prompt_key, version),
  INDEX idx_agent_prompts_key_status (prompt_key, status),
  INDEX idx_agent_prompts_updated_at (updated_at)
);

CREATE TABLE IF NOT EXISTS agent_prompt_publish_records (
  record_id VARCHAR(64) PRIMARY KEY,
  prompt_key VARCHAR(128) NOT NULL,
  prompt_id VARCHAR(64) NOT NULL,
  version INT NOT NULL,
  published_by VARCHAR(64) NOT NULL DEFAULT '',
  nacos_data_id VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL,
  INDEX idx_prompt_publish_key (prompt_key),
  INDEX idx_prompt_publish_created_at (created_at)
);

CREATE TABLE IF NOT EXISTS orders (
  order_id VARCHAR(64) PRIMARY KEY,
  order_no VARCHAR(64) NOT NULL DEFAULT '',
  account_id VARCHAR(64) NOT NULL,
  merchant_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  total_amount DECIMAL(10, 2) NOT NULL,
  discount_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  pay_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  payment_deadline_at DATETIME NULL,
  paid_at DATETIME NULL,
  closed_at DATETIME NULL,
  completed_at DATETIME NULL,
  cancel_reason VARCHAR(256) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_orders_order_no (order_no),
  INDEX idx_orders_account_id (account_id),
  INDEX idx_orders_merchant_id (merchant_id),
  INDEX idx_orders_status (status),
  INDEX idx_orders_payment_deadline_at (payment_deadline_at)
);

CREATE TABLE IF NOT EXISTS payments (
  payment_id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(10, 2) NOT NULL,
  status VARCHAR(32) NOT NULL,
  method VARCHAR(32) NOT NULL,
  transaction_no VARCHAR(96) NOT NULL DEFAULT '',
  expires_at DATETIME NOT NULL,
  paid_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_payments_order_id (order_id),
  INDEX idx_payments_account_id (account_id),
  INDEX idx_payments_status (status),
  INDEX idx_payments_expires_at (expires_at)
);

CREATE TABLE IF NOT EXISTS promotion_rules (
  promotion_id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  scope VARCHAR(32) NOT NULL,
  merchant_id VARCHAR(64) NOT NULL DEFAULT '',
  product_id VARCHAR(64) NOT NULL DEFAULT '',
  category_id VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(32) NOT NULL,
  threshold_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  discount_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  discount_rate DECIMAL(5, 4) NOT NULL DEFAULT 0.0000,
  stackable BOOLEAN NOT NULL DEFAULT TRUE,
  start_at DATETIME NOT NULL,
  end_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_promotion_rules_scope (scope),
  INDEX idx_promotion_rules_merchant_id (merchant_id),
  INDEX idx_promotion_rules_status (status),
  INDEX idx_promotion_rules_time (start_at, end_at)
);

CREATE TABLE IF NOT EXISTS coupons (
  coupon_id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  scope VARCHAR(32) NOT NULL,
  merchant_id VARCHAR(64) NOT NULL DEFAULT '',
  type VARCHAR(32) NOT NULL,
  threshold_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  discount_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00,
  total_count INT NOT NULL DEFAULT 0,
  claimed_count INT NOT NULL DEFAULT 0,
  per_user_limit INT NOT NULL DEFAULT 1,
  start_at DATETIME NOT NULL,
  end_at DATETIME NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_coupons_scope (scope),
  INDEX idx_coupons_merchant_id (merchant_id),
  INDEX idx_coupons_status (status),
  INDEX idx_coupons_time (start_at, end_at)
);

CREATE TABLE IF NOT EXISTS user_coupons (
  user_coupon_id VARCHAR(64) PRIMARY KEY,
  coupon_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'unused',
  order_id VARCHAR(64) NOT NULL DEFAULT '',
  claimed_at DATETIME NOT NULL,
  used_at DATETIME NULL,
  INDEX idx_user_coupons_account_id (account_id),
  INDEX idx_user_coupons_coupon_id (coupon_id),
  INDEX idx_user_coupons_status (status)
);

CREATE TABLE IF NOT EXISTS product_reviews (
  review_id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL,
  order_item_id VARCHAR(64) NOT NULL,
  product_id VARCHAR(64) NOT NULL,
  sku_id VARCHAR(64) NOT NULL DEFAULT '',
  account_id VARCHAR(64) NOT NULL,
  rating INT NOT NULL,
  content TEXT NOT NULL,
  tags_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'visible',
  merchant_reply TEXT,
  merchant_replied_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_product_reviews_order_item (order_item_id),
  INDEX idx_product_reviews_product_id (product_id),
  INDEX idx_product_reviews_account_id (account_id),
  INDEX idx_product_reviews_status (status)
);

CREATE TABLE IF NOT EXISTS order_items (
  order_item_id VARCHAR(64) PRIMARY KEY,
  order_id VARCHAR(64) NOT NULL,
  product_id VARCHAR(64) NOT NULL,
  sku_id VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL,
  image_url VARCHAR(512) NOT NULL,
  price DECIMAL(10, 2) NOT NULL,
  quantity INT NOT NULL,
  merchant_id VARCHAR(64) NOT NULL,
  merchant_name VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_order_items_order_id (order_id),
  INDEX idx_order_items_product_id (product_id)
);

INSERT IGNORE INTO merchants (merchant_id, name, logo_url, description, service_phone, status) VALUES
('m_001', '小猪数码旗舰店', '/placeholder-merchant.svg', '主营手机、耳机、智能设备和办公外设。', '400-000-0000', 'active');

INSERT IGNORE INTO promotion_rules (
  promotion_id, name, scope, merchant_id, type, threshold_amount, discount_amount, discount_rate, stackable, start_at, end_at, status
) VALUES
('promo_platform_001', '平台满 300 减 30', 'platform', '', 'full_reduction', 300.00, 30.00, 0.0000, TRUE, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active'),
('promo_m_001_001', '小猪数码满 1000 减 80', 'merchant', 'm_001', 'full_reduction', 1000.00, 80.00, 0.0000, TRUE, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active');

INSERT IGNORE INTO coupons (
  coupon_id, name, scope, merchant_id, type, threshold_amount, discount_amount, total_count, claimed_count, per_user_limit, start_at, end_at, status
) VALUES
('coupon_platform_001', '平台新人满 200 减 20', 'platform', '', 'fixed_amount', 200.00, 20.00, 10000, 0, 1, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active'),
('coupon_m_001_001', '小猪数码满 500 减 50', 'merchant', 'm_001', 'fixed_amount', 500.00, 50.00, 10000, 0, 1, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active');

INSERT IGNORE INTO categories (category_id, parent_id, name, sort_order) VALUES
('c_phone', '', '手机', 10),
('c_mouse', '', '鼠标', 20);

INSERT IGNORE INTO products (
  product_id, merchant_id, name, brand, category_id, image_url, image_urls_json,
  price, market_price, stock_quantity, stock_status, tags_json, selling_points_json,
  recommend_reason, risk_notes_json, attributes_json, suitable_for_json,
  not_suitable_for_json, description, status, sort_order
) VALUES
(
  'p_001', 'm_001', 'X Phone 12', 'X', 'c_phone',
  'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80',
  JSON_ARRAY('https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=640&q=80'),
  2999.00, 3299.00, 84, 'in_stock',
  JSON_ARRAY('拍照', '预算内', '抓拍'),
  JSON_ARRAY('高速对焦', '儿童抓拍模式', '256GB 存储'),
  '抓拍和对焦能力适合日常拍照，价格为 2999 元。',
  JSON_ARRAY(),
  JSON_ARRAY(JSON_OBJECT('key', '存储', 'value', '256GB'), JSON_OBJECT('key', '重量', 'value', '189', 'unit', 'g'), JSON_OBJECT('key', '屏幕', 'value', '6.5 英寸 OLED')),
  JSON_ARRAY('拍娃', '日常拍照', '预算敏感'),
  JSON_ARRAY(),
  '适合预算内拍照和日常使用的手机。',
  'active', 10
),
(
  'p_002', 'm_001', 'Y Camera Max', 'Y', 'c_phone',
  'https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80',
  JSON_ARRAY('https://images.unsplash.com/photo-1598327105666-5b89351aff97?auto=format&fit=crop&w=640&q=80'),
  3499.00, 3899.00, 32, 'in_stock',
  JSON_ARRAY('影像旗舰', '长焦', '续航'),
  JSON_ARRAY('长焦表现好', '夜景稳定', '续航更强'),
  '影像和续航配置更高，价格为 3499 元。',
  JSON_ARRAY(),
  JSON_ARRAY(JSON_OBJECT('key', '存储', 'value', '256GB'), JSON_OBJECT('key', '重量', 'value', '204', 'unit', 'g')),
  JSON_ARRAY('旅行拍照', '重视续航'),
  JSON_ARRAY(),
  '影像能力更强，但价格超过 3000。',
  'active', 20
),
(
  'p_mouse_001', 'm_001', 'Quiet Mouse S', 'Q', 'c_mouse',
  'https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80',
  JSON_ARRAY('https://images.unsplash.com/photo-1527814050087-3793815479db?auto=format&fit=crop&w=640&q=80'),
  129.00, 159.00, 120, 'in_stock',
  JSON_ARRAY('静音', '办公', '无线'),
  JSON_ARRAY('静音微动', '人体工学', '长续航'),
  '静音、无线、握持舒适，更适合办公和宿舍。',
  JSON_ARRAY('不适合高强度电竞'),
  JSON_ARRAY(JSON_OBJECT('key', '连接', 'value', '2.4G 无线 + 蓝牙'), JSON_OBJECT('key', '重量', 'value', '88', 'unit', 'g')),
  JSON_ARRAY('办公', '宿舍', '图书馆'),
  JSON_ARRAY('高强度电竞'),
  '适合安静办公环境的无线鼠标。',
  'active', 30
);

INSERT IGNORE INTO product_skus (sku_id, product_id, sku_name, price, stock_quantity, stock_status, specs_json, is_default) VALUES
('sku_001', 'p_001', 'X Phone 12 标准版', 2999.00, 84, 'in_stock', JSON_OBJECT('版本', '标准版'), TRUE),
('sku_002', 'p_002', 'Y Camera Max 标准版', 3499.00, 32, 'in_stock', JSON_OBJECT('版本', '标准版'), TRUE),
('sku_mouse_001', 'p_mouse_001', 'Quiet Mouse S 标准版', 129.00, 120, 'in_stock', JSON_OBJECT('版本', '标准版'), TRUE);

INSERT IGNORE INTO knowledge_chunks (chunk_id, title, snippet, source, sort_order) VALUES
('ck_phone_001', 'X Phone 12 商品详情', 'X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。', 'mysql_seed', 10),
('ck_mouse_001', 'Quiet Mouse S 商品详情', 'Quiet Mouse S 主打静音按键、无线连接和人体工学握持。', 'mysql_seed', 20);

INSERT IGNORE INTO knowledge_documents (document_id, merchant_id, title, doc_type, content, status, chunk_count) VALUES
('doc_001', 'm_001', '手机商品详情', 'product_detail', 'X Phone 12 支持高速对焦、儿童抓拍模式，官方零售价 2999 元。', 'indexed', 1),
('doc_002', 'm_001', 'Quiet Mouse S 商品详情', 'product_detail', 'Quiet Mouse S 主打静音按键、无线连接和人体工学握持。', 'indexed', 1);

INSERT IGNORE INTO accounts (account_id, username, password_hash, display_name, role, merchant_id, status) VALUES
('acct_user_001', 'user', '90aae915da86d3b3a4da7a996bc264bfbaf50a953cbbe8cd3478a2a6ccc7b900', '演示用户', 'user', '', 'active'),
('acct_merchant_001', 'merchant', '0b2a8a42a665ad403419c5f3f0d6cea853357272459d8e4c30a0900dd4718ebc', '小猪数码运营', 'merchant', 'm_001', 'active'),
('acct_admin_001', 'admin', 'ac0e7d037817094e9e0b4441f9bae3209d67b02fa484917065f71b16109a1a78', '平台管理员', 'admin', '', 'active');
