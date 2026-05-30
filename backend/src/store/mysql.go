package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/imagevector"
	"github.com/LYP-leo/xzxg-shop/backend/src/rag"
	mysql "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db     *sql.DB
	vector *rag.Client
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func OpenMySQL(ctx context.Context, dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return NewMySQLStore(db), nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func (s *MySQLStore) SetVectorClient(client *rag.Client) {
	s.vector = client
}

// Migrate 先执行基础 SQL，再执行兼容迁移。
// 兼容迁移用于老库平滑升级，避免每次加字段都要求手动清库。
func (s *MySQLStore) Migrate(ctx context.Context) error {
	content, err := readMigrationFile()
	if err != nil {
		return err
	}
	for _, stmt := range splitSQLStatements(string(content)) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	if err := s.ensureAccessSchema(ctx); err != nil {
		return err
	}
	return nil
}

func (s *MySQLStore) BootstrapVectorIndex(ctx context.Context) error {
	if s.vector == nil {
		return nil
	}
	return s.vector.Bootstrap(ctx, s.productVectorRows(ctx), s.knowledgeVectorRows(ctx), s.productImageVectorRows(ctx))
}

func (s *MySQLStore) VectorIndexStatus(ctx context.Context) domain.VectorIndexStatus {
	if s.vector == nil {
		return domain.VectorIndexStatus{Enabled: false, UpdatedAt: time.Now()}
	}
	return s.vector.Status(ctx)
}

// ensureAccessSchema 补齐历史版本缺少的字段、索引和种子数据。
// 这里保留幂等写法，保证本地 Docker 重启或线上滚动发布时重复执行也安全。
func (s *MySQLStore) ensureAccessSchema(ctx context.Context) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"cart_items", "account_id", "ALTER TABLE cart_items ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER cart_item_id"},
		{"chat_sessions", "account_id", "ALTER TABLE chat_sessions ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER session_id"},
		{"chat_sessions", "summary", "ALTER TABLE chat_sessions ADD COLUMN summary TEXT AFTER title"},
		{"chat_sessions", "message_count", "ALTER TABLE chat_sessions ADD COLUMN message_count INT NOT NULL DEFAULT 0 AFTER summary"},
		{"chat_sessions", "last_message_at", "ALTER TABLE chat_sessions ADD COLUMN last_message_at DATETIME NULL AFTER message_count"},
		{"chat_sessions", "pinned_at", "ALTER TABLE chat_sessions ADD COLUMN pinned_at DATETIME NULL AFTER last_message_at"},
		{"chat_sessions", "deleted_at", "ALTER TABLE chat_sessions ADD COLUMN deleted_at DATETIME NULL AFTER pinned_at"},
		{"chat_sessions", "updated_at", "ALTER TABLE chat_sessions ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AFTER created_at"},
		{"user_messages", "account_id", "ALTER TABLE user_messages ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER session_id"},
		{"agent_runs", "account_id", "ALTER TABLE agent_runs ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER message_id"},
		{"agent_runs", "content", "ALTER TABLE agent_runs ADD COLUMN content TEXT AFTER trace_id"},
		{"agent_runs", "blocks_json", "ALTER TABLE agent_runs ADD COLUMN blocks_json JSON NULL AFTER content"},
		{"agent_runs", "followups_json", "ALTER TABLE agent_runs ADD COLUMN followups_json JSON NULL AFTER blocks_json"},
		{"agent_runs", "segments_json", "ALTER TABLE agent_runs ADD COLUMN segments_json JSON NULL AFTER followups_json"},
		{"orders", "order_no", "ALTER TABLE orders ADD COLUMN order_no VARCHAR(64) NOT NULL DEFAULT '' AFTER order_id"},
		{"orders", "discount_amount", "ALTER TABLE orders ADD COLUMN discount_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00 AFTER total_amount"},
		{"orders", "pay_amount", "ALTER TABLE orders ADD COLUMN pay_amount DECIMAL(10, 2) NOT NULL DEFAULT 0.00 AFTER discount_amount"},
		{"orders", "payment_deadline_at", "ALTER TABLE orders ADD COLUMN payment_deadline_at DATETIME NULL AFTER pay_amount"},
		{"orders", "paid_at", "ALTER TABLE orders ADD COLUMN paid_at DATETIME NULL AFTER payment_deadline_at"},
		{"orders", "closed_at", "ALTER TABLE orders ADD COLUMN closed_at DATETIME NULL AFTER paid_at"},
		{"orders", "completed_at", "ALTER TABLE orders ADD COLUMN completed_at DATETIME NULL AFTER closed_at"},
		{"orders", "cancel_reason", "ALTER TABLE orders ADD COLUMN cancel_reason VARCHAR(256) NOT NULL DEFAULT '' AFTER completed_at"},
		{"accounts", "avatar_url", "ALTER TABLE accounts ADD COLUMN avatar_url VARCHAR(512) NOT NULL DEFAULT '' AFTER display_name"},
		{"accounts", "phone", "ALTER TABLE accounts ADD COLUMN phone VARCHAR(32) NOT NULL DEFAULT '' AFTER avatar_url"},
		{"accounts", "email", "ALTER TABLE accounts ADD COLUMN email VARCHAR(128) NOT NULL DEFAULT '' AFTER phone"},
		{"accounts", "deleted_at", "ALTER TABLE accounts ADD COLUMN deleted_at DATETIME NULL AFTER status"},
	}
	for _, column := range columns {
		exists, err := s.columnExists(ctx, column.table, column.name)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := s.db.ExecContext(ctx, column.ddl); err != nil {
				return fmt.Errorf("add column %s.%s: %w", column.table, column.name, err)
			}
		}
	}

	// 早期版本没有 account_id 和订单支付字段，这里统一回填到可用状态。
	updates := []string{
		"UPDATE cart_items SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE chat_sessions SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE user_messages SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE user_messages SET client_message_id = CONCAT('server_', message_id) WHERE client_message_id = ''",
		"UPDATE agent_runs SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE chat_sessions cs SET message_count = (SELECT COUNT(*) FROM user_messages um WHERE um.session_id = cs.session_id), last_message_at = COALESCE((SELECT MAX(um.created_at) FROM user_messages um WHERE um.session_id = cs.session_id), cs.created_at) WHERE message_count = 0",
		"UPDATE orders SET order_no = order_id WHERE order_no = ''",
		"UPDATE orders SET pay_amount = total_amount WHERE pay_amount = 0 AND discount_amount = 0",
		"UPDATE orders SET status = 'pending_ship', paid_at = COALESCE(paid_at, created_at) WHERE status = 'pending_ship'",
		"UPDATE products SET recommend_reason = '抓拍和对焦能力适合日常拍照，价格为 2999 元。', risk_notes_json = JSON_ARRAY(), not_suitable_for_json = JSON_ARRAY() WHERE product_id = 'p_001'",
		"UPDATE products SET recommend_reason = '影像和续航配置更高，价格为 3499 元。', risk_notes_json = JSON_ARRAY(), not_suitable_for_json = JSON_ARRAY() WHERE product_id = 'p_002'",
	}
	for _, stmt := range updates {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("backfill access column: %w", err)
		}
	}

	if err := s.repairDuplicateUserMessageIdempotencyKeys(ctx); err != nil {
		return err
	}

	// 旧购物车唯一索引没有 account_id，会导致多用户加同一商品冲突，必须移除。
	if exists, err := s.indexExists(ctx, "cart_items", "uk_cart_product_sku"); err != nil {
		return err
	} else if exists {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE cart_items DROP INDEX uk_cart_product_sku"); err != nil {
			return fmt.Errorf("drop legacy cart unique index: %w", err)
		}
	}

	if err := s.ensureCommerceV3Tables(ctx); err != nil {
		return err
	}

	indexes := []struct {
		table string
		name  string
		ddl   string
	}{
		{"cart_items", "idx_cart_items_account_id", "CREATE INDEX idx_cart_items_account_id ON cart_items (account_id)"},
		{"cart_items", "uk_cart_account_product_sku", "CREATE UNIQUE INDEX uk_cart_account_product_sku ON cart_items (account_id, product_id, sku_id)"},
		{"chat_sessions", "idx_chat_sessions_account_id", "CREATE INDEX idx_chat_sessions_account_id ON chat_sessions (account_id)"},
		{"user_messages", "idx_user_messages_account_id", "CREATE INDEX idx_user_messages_account_id ON user_messages (account_id)"},
		{"user_messages", "uk_user_messages_client_message", "CREATE UNIQUE INDEX uk_user_messages_client_message ON user_messages (account_id, session_id, client_message_id)"},
		{"agent_runs", "idx_agent_runs_account_id", "CREATE INDEX idx_agent_runs_account_id ON agent_runs (account_id)"},
		{"agent_runs", "uk_agent_runs_message", "CREATE UNIQUE INDEX uk_agent_runs_message ON agent_runs (account_id, message_id)"},
		{"orders", "uk_orders_order_no", "CREATE UNIQUE INDEX uk_orders_order_no ON orders (order_no)"},
		{"orders", "idx_orders_payment_deadline_at", "CREATE INDEX idx_orders_payment_deadline_at ON orders (payment_deadline_at)"},
		{"accounts", "idx_accounts_status", "CREATE INDEX idx_accounts_status ON accounts (status)"},
	}
	for _, index := range indexes {
		exists, err := s.indexExists(ctx, index.table, index.name)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := s.db.ExecContext(ctx, index.ddl); err != nil {
				return fmt.Errorf("create index %s.%s: %w", index.table, index.name, err)
			}
		}
	}
	return nil
}

func (s *MySQLStore) repairDuplicateUserMessageIdempotencyKeys(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_messages um
		JOIN (
			SELECT message_id
			FROM (
				SELECT
					message_id,
					ROW_NUMBER() OVER (
						PARTITION BY account_id, session_id, client_message_id
						ORDER BY created_at, message_id
					) AS rn
				FROM user_messages
				WHERE client_message_id <> ''
			) ranked
			WHERE rn > 1
		) dup ON dup.message_id = um.message_id
		SET um.client_message_id = CONCAT(um.client_message_id, '_dup_', um.message_id)
	`)
	if err != nil {
		return fmt.Errorf("repair duplicate user message idempotency keys: %w", err)
	}
	return nil
}

// ensureCommerceV3Tables 创建优惠、券、评价等 v3 业务表，并插入演示数据。
func (s *MySQLStore) ensureCommerceV3Tables(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS promotion_rules (
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
		)`,
		`CREATE TABLE IF NOT EXISTS coupons (
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
		)`,
		`CREATE TABLE IF NOT EXISTS user_coupons (
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
		)`,
		`CREATE TABLE IF NOT EXISTS product_reviews (
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
		)`,
		`CREATE TABLE IF NOT EXISTS stored_files (
			file_id VARCHAR(64) PRIMARY KEY,
			account_id VARCHAR(64) NOT NULL DEFAULT '',
			object_key VARCHAR(512) NOT NULL,
			url VARCHAR(512) NOT NULL,
			mime_type VARCHAR(128) NOT NULL,
			size_bytes BIGINT NOT NULL DEFAULT 0,
			content_hash VARCHAR(128) NOT NULL,
			storage_provider VARCHAR(32) NOT NULL DEFAULT 'minio',
			source_url VARCHAR(1024) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_stored_files_object_key (object_key),
			INDEX idx_stored_files_account_id (account_id),
			INDEX idx_stored_files_content_hash (content_hash)
		)`,
		`INSERT IGNORE INTO promotion_rules (
			promotion_id, name, scope, merchant_id, type, threshold_amount, discount_amount, discount_rate, stackable, start_at, end_at, status
		) VALUES
		('promo_platform_001', '平台满 300 减 30', 'platform', '', 'full_reduction', 300.00, 30.00, 0.0000, TRUE, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active'),
		('promo_m_001_001', '小猪数码满 1000 减 80', 'merchant', 'm_001', 'full_reduction', 1000.00, 80.00, 0.0000, TRUE, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active')`,
		`INSERT IGNORE INTO coupons (
			coupon_id, name, scope, merchant_id, type, threshold_amount, discount_amount, total_count, claimed_count, per_user_limit, start_at, end_at, status
		) VALUES
		('coupon_platform_001', '平台新人满 200 减 20', 'platform', '', 'fixed_amount', 200.00, 20.00, 10000, 0, 1, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active'),
		('coupon_m_001_001', '小猪数码满 500 减 50', 'merchant', 'm_001', 'fixed_amount', 500.00, 50.00, 10000, 0, 1, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 'active')`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure commerce v3 table: %w", err)
		}
	}
	return nil
}

func (s *MySQLStore) GetAccountByUsername(ctx context.Context, username string) (domain.Account, string, bool) {
	var account domain.Account
	var role string
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id, username, password_hash, display_name, avatar_url, phone, email, role, merchant_id, status, created_at
		FROM accounts
		WHERE username = ? AND status = 'active'
	`, username).Scan(
		&account.AccountID,
		&account.Username,
		&passwordHash,
		&account.DisplayName,
		&account.AvatarURL,
		&account.Phone,
		&account.Email,
		&role,
		&account.MerchantID,
		&account.Status,
		&account.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, "", false
	}
	account.Role = domain.AccountRole(role)
	return account, passwordHash, err == nil
}

func (s *MySQLStore) GetAccountByToken(ctx context.Context, token string) (domain.Account, bool) {
	var account domain.Account
	var role string
	err := s.db.QueryRowContext(ctx, `
		SELECT a.account_id, a.username, a.display_name, a.avatar_url, a.phone, a.email, a.role, a.merchant_id, a.status, a.created_at
		FROM auth_tokens t
		JOIN accounts a ON a.account_id = t.account_id
		WHERE t.token = ? AND t.expires_at > ? AND a.status = 'active'
	`, token, time.Now()).Scan(
		&account.AccountID,
		&account.Username,
		&account.DisplayName,
		&account.AvatarURL,
		&account.Phone,
		&account.Email,
		&role,
		&account.MerchantID,
		&account.Status,
		&account.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, false
	}
	account.Role = domain.AccountRole(role)
	return account, err == nil
}

func (s *MySQLStore) ListAccounts(ctx context.Context) []domain.Account {
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, username, display_name, role, merchant_id, status, created_at
		FROM accounts
		ORDER BY created_at, account_id
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.Account, 0)
	for rows.Next() {
		var item domain.Account
		var role string
		if err := rows.Scan(&item.AccountID, &item.Username, &item.DisplayName, &role, &item.MerchantID, &item.Status, &item.CreatedAt); err != nil {
			return nil
		}
		item.Role = domain.AccountRole(role)
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListAccountsPage(ctx context.Context, page int, pageSize int) ([]domain.Account, int) {
	page, pageSize = normalizePage(page, pageSize)
	total := s.countRows(ctx, "accounts")
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, username, display_name, role, merchant_id, status, created_at
		FROM accounts
		ORDER BY created_at, account_id
		LIMIT ? OFFSET ?
	`, pageSize, pageOffset(page, pageSize))
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	items := make([]domain.Account, 0)
	for rows.Next() {
		var item domain.Account
		var role string
		if err := rows.Scan(&item.AccountID, &item.Username, &item.DisplayName, &role, &item.MerchantID, &item.Status, &item.CreatedAt); err != nil {
			return nil, 0
		}
		item.Role = domain.AccountRole(role)
		items = append(items, item)
	}
	return items, total
}

func (s *MySQLStore) CreateAccount(ctx context.Context, input domain.AccountCreateInput) (domain.Account, error) {
	now := time.Now()
	role := strings.TrimSpace(string(input.Role))
	if role == "" {
		role = string(domain.AccountRoleUser)
	}
	account := domain.Account{
		AccountID:   nextID("acct"),
		Username:    strings.TrimSpace(input.Username),
		DisplayName: strings.TrimSpace(input.DisplayName),
		AvatarURL:   strings.TrimSpace(input.AvatarURL),
		Phone:       strings.TrimSpace(input.Phone),
		Email:       strings.TrimSpace(input.Email),
		Role:        domain.AccountRole(role),
		MerchantID:  strings.TrimSpace(input.MerchantID),
		Status:      "active",
		CreatedAt:   now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO accounts (account_id, username, password_hash, display_name, avatar_url, phone, email, role, merchant_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
	`, account.AccountID, account.Username, input.PasswordHash, account.DisplayName, account.AvatarURL, account.Phone, account.Email, role, account.MerchantID, now, now)
	if err != nil {
		return domain.Account{}, fmt.Errorf("insert account: %w", err)
	}
	return account, nil
}

func (s *MySQLStore) UpdateAccountStatus(ctx context.Context, accountID string, status string) (domain.Account, bool) {
	result, err := s.db.ExecContext(ctx, `UPDATE accounts SET status = ?, updated_at = ? WHERE account_id = ?`, status, time.Now(), accountID)
	if err != nil {
		return domain.Account{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Account{}, false
	}
	var account domain.Account
	var role string
	err = s.db.QueryRowContext(ctx, `
		SELECT account_id, username, display_name, avatar_url, phone, email, role, merchant_id, status, created_at
		FROM accounts
		WHERE account_id = ?
	`, accountID).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.AvatarURL, &account.Phone, &account.Email, &role, &account.MerchantID, &account.Status, &account.CreatedAt)
	if err != nil {
		return domain.Account{}, false
	}
	account.Role = domain.AccountRole(role)
	return account, true
}

func (s *MySQLStore) UpdateAccountProfile(ctx context.Context, accountID string, displayName string, avatarURL string) (domain.Account, bool) {
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if strings.TrimSpace(displayName) != "" {
		sets = append(sets, "display_name = ?")
		args = append(args, truncateRunes(strings.TrimSpace(displayName), 24))
	}
	if strings.TrimSpace(avatarURL) != "" {
		sets = append(sets, "avatar_url = ?")
		args = append(args, truncateRunes(strings.TrimSpace(avatarURL), 512))
	}
	args = append(args, accountID)
	result, err := s.db.ExecContext(ctx, `UPDATE accounts SET `+strings.Join(sets, ", ")+` WHERE account_id = ? AND status = 'active'`, args...)
	if err != nil {
		return domain.Account{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Account{}, false
	}
	return s.getAccountByID(ctx, accountID)
}

func (s *MySQLStore) UpdateAccountContact(ctx context.Context, accountID string, phone string, email string) (domain.Account, bool) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE accounts
		SET phone = ?, email = ?, updated_at = ?
		WHERE account_id = ? AND status = 'active'
	`, truncateRunes(strings.TrimSpace(phone), 32), truncateRunes(strings.TrimSpace(email), 128), time.Now(), accountID)
	if err != nil {
		return domain.Account{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Account{}, false
	}
	return s.getAccountByID(ctx, accountID)
}

func (s *MySQLStore) DeleteAuthToken(ctx context.Context, token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE token = ?`, strings.TrimSpace(token))
	return err == nil
}

func (s *MySQLStore) DeleteAuthTokensByAccount(ctx context.Context, accountID string) bool {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE account_id = ?`, accountID)
	return err == nil
}

func (s *MySQLStore) DeleteAccount(ctx context.Context, accountID string) bool {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET status = 'deleted', deleted_at = ?, updated_at = ?
		WHERE account_id = ? AND status = 'active'
	`, time.Now(), time.Now(), accountID)
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return false
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_tokens WHERE account_id = ?`, accountID); err != nil {
		return false
	}
	return tx.Commit() == nil
}

func (s *MySQLStore) getAccountByID(ctx context.Context, accountID string) (domain.Account, bool) {
	var account domain.Account
	var role string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id, username, display_name, avatar_url, phone, email, role, merchant_id, status, created_at
		FROM accounts
		WHERE account_id = ?
	`, accountID).Scan(&account.AccountID, &account.Username, &account.DisplayName, &account.AvatarURL, &account.Phone, &account.Email, &role, &account.MerchantID, &account.Status, &account.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Account{}, false
	}
	account.Role = domain.AccountRole(role)
	return account, err == nil
}

func (s *MySQLStore) CreateAuthToken(ctx context.Context, accountID string) (string, error) {
	token := nextID("tok")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_tokens (token, account_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, token, accountID, time.Now(), time.Now().Add(24*time.Hour))
	if err != nil {
		return "", fmt.Errorf("insert auth token: %w", err)
	}
	return token, nil
}

func (s *MySQLStore) CreateSession(ctx context.Context, accountID string, title string) (domain.ChatSession, error) {
	now := time.Now()
	session := domain.ChatSession{
		SessionID:     nextID("sess"),
		AccountID:     accountID,
		Title:         title,
		Summary:       "",
		MessageCount:  0,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chat_sessions (session_id, account_id, title, summary, message_count, last_message_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, session.SessionID, session.AccountID, session.Title, session.Summary, session.MessageCount, session.LastMessageAt, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return domain.ChatSession{}, fmt.Errorf("insert chat session: %w", err)
	}
	return session, nil
}

func (s *MySQLStore) GetSession(ctx context.Context, accountID string, sessionID string) (domain.ChatSession, bool) {
	var session domain.ChatSession
	var summary sql.NullString
	var lastMessageAt, pinnedAt, deletedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, account_id, title, summary, message_count, last_message_at, pinned_at, deleted_at, created_at, updated_at
		FROM chat_sessions
		WHERE session_id = ? AND account_id = ? AND deleted_at IS NULL
	`, sessionID, accountID).Scan(&session.SessionID, &session.AccountID, &session.Title, &summary, &session.MessageCount, &lastMessageAt, &pinnedAt, &deletedAt, &session.CreatedAt, &session.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChatSession{}, false
	}
	if summary.Valid {
		session.Summary = summary.String
	}
	if lastMessageAt.Valid {
		session.LastMessageAt = lastMessageAt.Time
	}
	if pinnedAt.Valid {
		session.PinnedAt = pinnedAt.Time
	}
	if deletedAt.Valid {
		session.DeletedAt = deletedAt.Time
	}
	return session, err == nil
}

func (s *MySQLStore) ListUserSessions(ctx context.Context, accountID string) []domain.ChatSession {
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, account_id, title, summary, message_count, last_message_at, pinned_at, deleted_at, created_at, updated_at
		FROM chat_sessions
		WHERE account_id = ? AND message_count > 0 AND deleted_at IS NULL
		ORDER BY CASE WHEN pinned_at IS NULL THEN 1 ELSE 0 END, pinned_at DESC, COALESCE(last_message_at, created_at) DESC, created_at DESC
	`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.ChatSession, 0)
	for rows.Next() {
		var item domain.ChatSession
		var summary sql.NullString
		var lastMessageAt, pinnedAt, deletedAt sql.NullTime
		if err := rows.Scan(&item.SessionID, &item.AccountID, &item.Title, &summary, &item.MessageCount, &lastMessageAt, &pinnedAt, &deletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil
		}
		if summary.Valid {
			item.Summary = summary.String
		}
		if lastMessageAt.Valid {
			item.LastMessageAt = lastMessageAt.Time
		}
		if pinnedAt.Valid {
			item.PinnedAt = pinnedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = deletedAt.Time
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) SearchUserSessions(ctx context.Context, accountID string, keyword string, page int, pageSize int) ([]domain.ChatSession, int) {
	page, pageSize = normalizePage(page, pageSize)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		items := s.ListUserSessions(ctx, accountID)
		total := len(items)
		start := pageOffset(page, pageSize)
		if start >= total {
			return []domain.ChatSession{}, total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		return items[start:end], total
	}
	like := "%" + keyword + "%"
	var total int
	_ = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT s.session_id
			FROM chat_sessions s
			LEFT JOIN user_messages m ON m.session_id = s.session_id AND m.account_id = s.account_id
			WHERE s.account_id = ?
			  AND s.message_count > 0
			  AND s.deleted_at IS NULL
			  AND (s.title LIKE ? OR s.summary LIKE ? OR m.content LIKE ?)
		) hits
	`, accountID, like, like, like).Scan(&total)

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT s.session_id, s.account_id, s.title, s.summary, s.message_count, s.last_message_at, s.pinned_at, s.deleted_at, s.created_at, s.updated_at
		FROM chat_sessions s
		LEFT JOIN user_messages m ON m.session_id = s.session_id AND m.account_id = s.account_id
		WHERE s.account_id = ?
		  AND s.message_count > 0
		  AND s.deleted_at IS NULL
		  AND (s.title LIKE ? OR s.summary LIKE ? OR m.content LIKE ?)
		ORDER BY CASE WHEN s.pinned_at IS NULL THEN 1 ELSE 0 END, s.pinned_at DESC, COALESCE(s.last_message_at, s.updated_at, s.created_at) DESC, s.created_at DESC
		LIMIT ? OFFSET ?
	`, accountID, like, like, like, pageSize, pageOffset(page, pageSize))
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	items := make([]domain.ChatSession, 0)
	for rows.Next() {
		var item domain.ChatSession
		var summary sql.NullString
		var lastMessageAt, pinnedAt, deletedAt sql.NullTime
		if err := rows.Scan(&item.SessionID, &item.AccountID, &item.Title, &summary, &item.MessageCount, &lastMessageAt, &pinnedAt, &deletedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0
		}
		if summary.Valid {
			item.Summary = summary.String
		}
		if lastMessageAt.Valid {
			item.LastMessageAt = lastMessageAt.Time
		}
		if pinnedAt.Valid {
			item.PinnedAt = pinnedAt.Time
		}
		if deletedAt.Valid {
			item.DeletedAt = deletedAt.Time
		}
		items = append(items, item)
	}
	return items, total
}

func (s *MySQLStore) UpdateSessionSummary(ctx context.Context, accountID string, sessionID string, title string, summary string) (domain.ChatSession, bool) {
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if strings.TrimSpace(title) != "" {
		sets = append(sets, "title = ?")
		args = append(args, truncateRunes(strings.TrimSpace(title), 128))
	}
	if strings.TrimSpace(summary) != "" {
		sets = append(sets, "summary = ?")
		args = append(args, truncateRunes(strings.TrimSpace(summary), 500))
	}
	args = append(args, sessionID, accountID)
	result, err := s.db.ExecContext(ctx, `UPDATE chat_sessions SET `+strings.Join(sets, ", ")+` WHERE session_id = ? AND account_id = ? AND deleted_at IS NULL`, args...)
	if err != nil {
		return domain.ChatSession{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ChatSession{}, false
	}
	return s.GetSession(ctx, accountID, sessionID)
}

func (s *MySQLStore) PinSession(ctx context.Context, accountID string, sessionID string, pinned bool) (domain.ChatSession, bool) {
	var pinnedAt any
	if pinned {
		pinnedAt = time.Now()
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE chat_sessions
		SET pinned_at = ?, updated_at = ?
		WHERE session_id = ? AND account_id = ? AND deleted_at IS NULL
	`, pinnedAt, time.Now(), sessionID, accountID)
	if err != nil {
		return domain.ChatSession{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ChatSession{}, false
	}
	return s.GetSession(ctx, accountID, sessionID)
}

func (s *MySQLStore) DeleteSession(ctx context.Context, accountID string, sessionID string) bool {
	result, err := s.db.ExecContext(ctx, `
		UPDATE chat_sessions
		SET deleted_at = ?, updated_at = ?
		WHERE session_id = ? AND account_id = ? AND deleted_at IS NULL
	`, time.Now(), time.Now(), sessionID, accountID)
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected > 0
}

func (s *MySQLStore) GetSessionDetail(ctx context.Context, accountID string, sessionID string) (domain.ChatSessionDetail, bool) {
	session, ok := s.GetSession(ctx, accountID, sessionID)
	if !ok {
		return domain.ChatSessionDetail{}, false
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_id, session_id, account_id, client_message_id, content, attachments_json, created_at
		FROM user_messages
		WHERE account_id = ? AND session_id = ?
		ORDER BY created_at, message_id
	`, accountID, sessionID)
	if err != nil {
		return domain.ChatSessionDetail{}, false
	}
	defer rows.Close()

	messages := make([]domain.UserMessageWithRuns, 0)
	for rows.Next() {
		var message domain.UserMessageWithRuns
		var attachmentsJSON string
		if err := rows.Scan(
			&message.MessageID,
			&message.SessionID,
			&message.AccountID,
			&message.ClientMessageID,
			&message.Content,
			&attachmentsJSON,
			&message.CreatedAt,
		); err != nil {
			return domain.ChatSessionDetail{}, false
		}
		if strings.TrimSpace(attachmentsJSON) != "" {
			_ = json.Unmarshal([]byte(attachmentsJSON), &message.Attachments)
		}
		message.Runs = s.listRunsByMessage(ctx, accountID, message.MessageID)
		messages = append(messages, message)
	}
	return domain.ChatSessionDetail{Session: session, Messages: messages}, true
}

func (s *MySQLStore) CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, bool, error) {
	input.MessageID = nextID("msg")
	if strings.TrimSpace(input.ClientMessageID) == "" {
		input.ClientMessageID = "server_" + input.MessageID
	}
	input.CreatedAt = time.Now()
	attachments, err := json.Marshal(input.Attachments)
	if err != nil {
		return domain.UserMessage{}, false, fmt.Errorf("marshal attachments: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_messages (message_id, session_id, account_id, client_message_id, content, attachments_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, input.MessageID, input.SessionID, input.AccountID, input.ClientMessageID, input.Content, string(attachments), input.CreatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			if existing, ok := s.getUserMessageByClientMessageID(ctx, input.AccountID, input.SessionID, input.ClientMessageID); ok {
				return existing, false, nil
			}
		}
		return domain.UserMessage{}, false, fmt.Errorf("insert user message: %w", err)
	}
	title := deriveSessionTitle(input.Content)
	summary := deriveSessionSummary(input.Content)
	_, _ = s.db.ExecContext(ctx, `
		UPDATE chat_sessions
		SET
			title = CASE WHEN title = '' OR title = 'AI 导购' THEN ? ELSE title END,
			summary = CASE WHEN summary IS NULL OR summary = '' THEN ? ELSE summary END,
			message_count = message_count + 1,
			last_message_at = ?,
			updated_at = ?
		WHERE session_id = ? AND account_id = ?
	`, title, summary, input.CreatedAt, input.CreatedAt, input.SessionID, input.AccountID)
	return input, true, nil
}

func (s *MySQLStore) getUserMessageByClientMessageID(ctx context.Context, accountID string, sessionID string, clientMessageID string) (domain.UserMessage, bool) {
	var message domain.UserMessage
	var attachmentsRaw string
	err := s.db.QueryRowContext(ctx, `
		SELECT message_id, session_id, account_id, client_message_id, content, attachments_json, created_at
		FROM user_messages
		WHERE account_id = ? AND session_id = ? AND client_message_id = ?
	`, accountID, sessionID, clientMessageID).Scan(
		&message.MessageID,
		&message.SessionID,
		&message.AccountID,
		&message.ClientMessageID,
		&message.Content,
		&attachmentsRaw,
		&message.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserMessage{}, false
	}
	if err != nil {
		return domain.UserMessage{}, false
	}
	_ = json.Unmarshal([]byte(attachmentsRaw), &message.Attachments)
	return message, true
}

func (s *MySQLStore) listRunsByMessage(ctx context.Context, accountID string, messageID string) []domain.AgentRun {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, COALESCE(content, ''), COALESCE(CAST(blocks_json AS CHAR), ''), COALESCE(CAST(followups_json AS CHAR), ''), COALESCE(CAST(segments_json AS CHAR), ''), created_at, updated_at
		FROM agent_runs
		WHERE account_id = ? AND message_id = ?
		ORDER BY created_at, run_id
	`, accountID, messageID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.AgentRun, 0)
	for rows.Next() {
		item, err := scanAgentRun(rows)
		if err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListRecentAgentRuns(ctx context.Context, limit int) []domain.AgentRun {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, COALESCE(content, ''), COALESCE(CAST(blocks_json AS CHAR), ''), COALESCE(CAST(followups_json AS CHAR), ''), COALESCE(CAST(segments_json AS CHAR), ''), created_at, updated_at
		FROM agent_runs
		ORDER BY created_at DESC, run_id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.AgentRun, 0)
	for rows.Next() {
		item, err := scanAgentRun(rows)
		if err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListAgentRunsPage(ctx context.Context, page int, pageSize int) ([]domain.AgentRun, int) {
	page, pageSize = normalizePage(page, pageSize)
	total := s.countRows(ctx, "agent_runs")
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, COALESCE(content, ''), COALESCE(CAST(blocks_json AS CHAR), ''), COALESCE(CAST(followups_json AS CHAR), ''), COALESCE(CAST(segments_json AS CHAR), ''), created_at, updated_at
		FROM agent_runs
		ORDER BY created_at DESC, run_id DESC
		LIMIT ? OFFSET ?
	`, pageSize, pageOffset(page, pageSize))
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	items := make([]domain.AgentRun, 0)
	for rows.Next() {
		item, err := scanAgentRun(rows)
		if err != nil {
			return nil, 0
		}
		items = append(items, item)
	}
	return items, total
}

func (s *MySQLStore) CreateRun(ctx context.Context, accountID string, sessionID string, messageID string) (domain.AgentRun, bool, error) {
	now := time.Now()
	run := domain.AgentRun{
		RunID:     nextID("run"),
		SessionID: sessionID,
		MessageID: messageID,
		AccountID: accountID,
		Status:    domain.RunStatusRunning,
		TraceID:   nextID("trace"),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_runs (run_id, session_id, message_id, account_id, status, trace_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, run.RunID, run.SessionID, run.MessageID, run.AccountID, string(run.Status), run.TraceID, run.CreatedAt, run.UpdatedAt)
	if err != nil {
		if isDuplicateKeyError(err) {
			if existing, ok := s.getRunByMessage(ctx, accountID, messageID); ok {
				return existing, false, nil
			}
		}
		return domain.AgentRun{}, false, fmt.Errorf("insert agent run: %w", err)
	}
	return run, true, nil
}

func (s *MySQLStore) getRunByMessage(ctx context.Context, accountID string, messageID string) (domain.AgentRun, bool) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, created_at, updated_at
		FROM agent_runs
		WHERE account_id = ? AND message_id = ?
		ORDER BY created_at, run_id
		LIMIT 1
	`, accountID, messageID)
	if err != nil {
		return domain.AgentRun{}, false
	}
	defer rows.Close()
	if !rows.Next() {
		return domain.AgentRun{}, false
	}
	item, err := scanAgentRun(rows)
	return item, err == nil
}

func (s *MySQLStore) UpdateRunStatus(ctx context.Context, accountID string, runID string, status domain.RunStatus) (domain.AgentRun, bool) {
	now := time.Now()
	whereStatus := ""
	switch status {
	case domain.RunStatusCompleted, domain.RunStatusFailed, domain.RunStatusCanceled:
		whereStatus = " AND status = 'running'"
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = ?, updated_at = ?
		WHERE run_id = ? AND account_id = ?`+whereStatus+`
	`, string(status), now, runID, accountID)
	if err != nil {
		return domain.AgentRun{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.AgentRun{}, false
	}
	return s.getRun(ctx, runID)
}

func (s *MySQLStore) UpdateRunResult(ctx context.Context, accountID string, runID string, content string, blocksJSON string, followupsJSON string, segmentsJSON string) bool {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET content = ?, blocks_json = CAST(? AS JSON), followups_json = CAST(? AS JSON), segments_json = CAST(? AS JSON), updated_at = ?
		WHERE run_id = ? AND account_id = ?
	`, content, emptyJSON(blocksJSON), emptyJSON(followupsJSON), emptyJSON(segmentsJSON), time.Now(), runID, accountID)
	return err == nil
}

func (s *MySQLStore) ListRecentConversationRecords(ctx context.Context, accountID string, sessionID string, since time.Time, limit int) []domain.ConversationRecord {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ar.run_id,
			ar.session_id,
			ar.message_id,
			ar.account_id,
			um.content,
			COALESCE(ar.content, ''),
			COALESCE(CAST(ar.blocks_json AS CHAR), ''),
			ar.created_at,
			ar.updated_at
		FROM agent_runs ar
		JOIN user_messages um ON um.message_id = ar.message_id AND um.account_id = ar.account_id
		JOIN chat_sessions cs ON cs.session_id = ar.session_id AND cs.account_id = ar.account_id
		WHERE ar.account_id = ?
			AND ar.session_id = ?
			AND ar.status = 'completed'
			AND COALESCE(ar.content, '') <> ''
			AND ar.created_at >= ?
			AND cs.deleted_at IS NULL
		ORDER BY ar.created_at DESC, ar.run_id DESC
		LIMIT ?
	`, accountID, sessionID, since, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.ConversationRecord, 0, limit)
	for rows.Next() {
		var item domain.ConversationRecord
		var blocksJSON string
		if err := rows.Scan(
			&item.RunID,
			&item.SessionID,
			&item.MessageID,
			&item.AccountID,
			&item.UserQuery,
			&item.FinalAnswer,
			&blocksJSON,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil
		}
		item.ProductIDs, item.ProductRefs = productRefsFromBlocksJSON(blocksJSON)
		items = append(items, item)
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items
}

func productRefsFromBlocksJSON(blocksJSON string) ([]string, []domain.ProductCard) {
	var blocks []domain.AgentBlock
	if strings.TrimSpace(blocksJSON) == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
		return nil, nil
	}
	ids := make([]string, 0)
	seen := map[string]bool{}
	refs := make([]domain.ProductCard, 0)
	for _, block := range blocks {
		for _, id := range block.ProductIDs {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
		if block.Product != nil {
			product := *block.Product
			id := strings.TrimSpace(product.ProductID)
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
			if id != "" {
				refs = append(refs, product)
			}
		}
	}
	return ids, refs
}

func emptyJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "[]"
	}
	return value
}

func (s *MySQLStore) IsRunCanceled(ctx context.Context, runID string) bool {
	run, ok := s.getRun(ctx, runID)
	return ok && run.Status == domain.RunStatusCanceled
}

func (s *MySQLStore) RecordAgentTrace(ctx context.Context, input domain.AgentTraceInput) error {
	metadata := input.MetadataJSON
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_trace_events (
			trace_event_id, run_id, trace_id, account_id, stage, event_type, model, status,
			duration_ms, error, metadata_json, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nextID("trc"), input.RunID, input.TraceID, input.AccountID, input.Stage, input.EventType, input.Model, input.Status, input.DurationMS, input.Error, metadata, time.Now())
	if err != nil {
		return fmt.Errorf("insert agent trace event: %w", err)
	}
	return nil
}

func (s *MySQLStore) ListAgentTrace(ctx context.Context, accountID string, runID string) []domain.AgentTraceEvent {
	rows, err := s.db.QueryContext(ctx, `
		SELECT trace_event_id, run_id, trace_id, account_id, stage, event_type, model, status,
			duration_ms, COALESCE(error, ''), COALESCE(CAST(metadata_json AS CHAR), ''), created_at
		FROM agent_trace_events
		WHERE run_id = ? AND account_id = ?
		ORDER BY created_at, trace_event_id
	`, runID, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanAgentTraceEvents(rows)
}

func (s *MySQLStore) ListAgentTraceByRun(ctx context.Context, runID string) []domain.AgentTraceEvent {
	rows, err := s.db.QueryContext(ctx, `
		SELECT trace_event_id, run_id, trace_id, account_id, stage, event_type, model, status,
			duration_ms, COALESCE(error, ''), COALESCE(CAST(metadata_json AS CHAR), ''), created_at
		FROM agent_trace_events
		WHERE run_id = ?
		ORDER BY created_at, trace_event_id
	`, runID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanAgentTraceEvents(rows)
}

func (s *MySQLStore) SeedAgentPrompts(ctx context.Context, defaults []domain.AgentPromptInput) error {
	for _, input := range defaults {
		key := strings.TrimSpace(input.PromptKey)
		if key == "" || strings.TrimSpace(input.Content) == "" {
			continue
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_prompts WHERE prompt_key = ?`, key).Scan(&count); err != nil {
			return fmt.Errorf("count prompt %s: %w", key, err)
		}
		if count > 0 {
			continue
		}
		now := time.Now()
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO agent_prompts (
				prompt_id, prompt_key, title, content, status, version, description,
				created_by, published_at, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, 'active', 1, ?, 'system', ?, ?, ?)
		`, nextID("prm"), key, emptyFallback(input.Title, key), input.Content, input.Description, now, now, now)
		if err != nil {
			return fmt.Errorf("seed prompt %s: %w", key, err)
		}
	}
	return nil
}

func (s *MySQLStore) ListAgentPromptsPage(ctx context.Context, page int, pageSize int) ([]domain.AgentPrompt, int) {
	page, pageSize = normalizePage(page, pageSize)
	total := 0
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_prompts p
		JOIN (
			SELECT prompt_key, MAX(version) AS version
			FROM agent_prompts
			GROUP BY prompt_key
		) latest ON latest.prompt_key = p.prompt_key AND latest.version = p.version
	`).Scan(&total); err != nil {
		return nil, 0
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.prompt_id, p.prompt_key, p.title, p.content, p.status, p.version,
			COALESCE(p.description, ''), p.created_by, p.published_at, p.created_at, p.updated_at
		FROM agent_prompts p
		JOIN (
			SELECT prompt_key, MAX(version) AS version
			FROM agent_prompts
			GROUP BY prompt_key
		) latest ON latest.prompt_key = p.prompt_key AND latest.version = p.version
		ORDER BY p.prompt_key
		LIMIT ? OFFSET ?
	`, pageSize, pageOffset(page, pageSize))
	if err != nil {
		return nil, 0
	}
	defer rows.Close()
	return scanAgentPrompts(rows), total
}

func (s *MySQLStore) SaveAgentPromptDraft(ctx context.Context, input domain.AgentPromptInput) (domain.AgentPrompt, error) {
	key := strings.TrimSpace(input.PromptKey)
	content := strings.TrimSpace(input.Content)
	if key == "" {
		return domain.AgentPrompt{}, errors.New("prompt key is empty")
	}
	if content == "" {
		return domain.AgentPrompt{}, errors.New("prompt content is empty")
	}
	var latestVersion int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM agent_prompts WHERE prompt_key = ?`, key).Scan(&latestVersion); err != nil {
		return domain.AgentPrompt{}, fmt.Errorf("read latest prompt version: %w", err)
	}
	now := time.Now()
	item := domain.AgentPrompt{
		PromptID:    nextID("prm"),
		PromptKey:   key,
		Title:       emptyFallback(input.Title, key),
		Content:     input.Content,
		Status:      "draft",
		Version:     latestVersion + 1,
		Description: input.Description,
		CreatedBy:   input.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_prompts (
			prompt_id, prompt_key, title, content, status, version, description,
			created_by, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.PromptID, item.PromptKey, item.Title, item.Content, item.Status, item.Version, item.Description, item.CreatedBy, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return domain.AgentPrompt{}, fmt.Errorf("insert prompt draft: %w", err)
	}
	return item, nil
}

func (s *MySQLStore) PublishAgentPrompt(ctx context.Context, promptKey string, publishedBy string, nacosDataID string) (domain.AgentPrompt, domain.AgentPromptPublishRecord, error) {
	promptKey = strings.TrimSpace(promptKey)
	if promptKey == "" {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, errors.New("prompt key is empty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, err
	}
	defer tx.Rollback()

	var prompt domain.AgentPrompt
	var publishedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT prompt_id, prompt_key, title, content, status, version,
			COALESCE(description, ''), created_by, published_at, created_at, updated_at
		FROM agent_prompts
		WHERE prompt_key = ?
		ORDER BY version DESC
		LIMIT 1
	`, promptKey).Scan(
		&prompt.PromptID, &prompt.PromptKey, &prompt.Title, &prompt.Content, &prompt.Status, &prompt.Version,
		&prompt.Description, &prompt.CreatedBy, &publishedAt, &prompt.CreatedAt, &prompt.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, errors.New("prompt not found")
	}
	if err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, fmt.Errorf("read prompt: %w", err)
	}
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE agent_prompts SET status = 'archived', updated_at = ? WHERE prompt_key = ? AND status = 'active'`, now, promptKey); err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, fmt.Errorf("archive active prompt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE agent_prompts SET status = 'active', published_at = ?, updated_at = ? WHERE prompt_id = ?`, now, now, prompt.PromptID); err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, fmt.Errorf("activate prompt: %w", err)
	}
	record := domain.AgentPromptPublishRecord{
		RecordID:    nextID("prmpub"),
		PromptKey:   promptKey,
		PromptID:    prompt.PromptID,
		Version:     prompt.Version,
		PublishedBy: publishedBy,
		NacosDataID: emptyFallback(nacosDataID, "xzxg-shop-agent-prompts.json"),
		CreatedAt:   now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_prompt_publish_records (
			record_id, prompt_key, prompt_id, version, published_by, nacos_data_id, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.RecordID, record.PromptKey, record.PromptID, record.Version, record.PublishedBy, record.NacosDataID, record.CreatedAt); err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, fmt.Errorf("insert prompt publish record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.AgentPrompt{}, domain.AgentPromptPublishRecord{}, err
	}
	prompt.Status = "active"
	prompt.PublishedAt = now
	prompt.UpdatedAt = now
	return prompt, record, nil
}

func (s *MySQLStore) ListAgentPromptPublishRecords(ctx context.Context, promptKey string, limit int) []domain.AgentPromptPublishRecord {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `
		SELECT record_id, prompt_key, prompt_id, version, published_by, nacos_data_id, created_at
		FROM agent_prompt_publish_records
	`
	args := []any{}
	if strings.TrimSpace(promptKey) != "" {
		query += " WHERE prompt_key = ?"
		args = append(args, strings.TrimSpace(promptKey))
	}
	query += " ORDER BY created_at DESC, record_id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.AgentPromptPublishRecord, 0)
	for rows.Next() {
		var item domain.AgentPromptPublishRecord
		if err := rows.Scan(&item.RecordID, &item.PromptKey, &item.PromptID, &item.Version, &item.PublishedBy, &item.NacosDataID, &item.CreatedAt); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) SearchProducts(ctx context.Context, query string) []domain.ProductCard {
	items := s.ListProducts(ctx, query, "")
	if s.vector != nil && s.vector.Enabled() {
		if hits, err := s.vector.SearchProducts(ctx, query, 40); err == nil {
			items = mergeProductCards(items, s.productCardsByIDs(ctx, hitIDs(hits)))
		}
	}
	return rankProductSearchResults(query, items)
}

func (s *MySQLStore) ListCategories(ctx context.Context) []domain.Category {
	rows, err := s.db.QueryContext(ctx, `
		SELECT category_id, COALESCE(parent_id, ''), name
		FROM categories
		ORDER BY sort_order, category_id
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	categories := make([]domain.Category, 0)
	for rows.Next() {
		var item domain.Category
		if err := rows.Scan(&item.CategoryID, &item.ParentID, &item.Name); err != nil {
			return nil
		}
		item.Children = []domain.Category{}
		categories = append(categories, item)
	}
	return buildCategoryTree(categories)
}

func (s *MySQLStore) ListMerchants(ctx context.Context) []domain.Merchant {
	rows, err := s.db.QueryContext(ctx, `
		SELECT merchant_id, name, logo_url, description, service_phone, status
		FROM merchants
		ORDER BY merchant_id
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.Merchant, 0)
	for rows.Next() {
		var item domain.Merchant
		if err := rows.Scan(&item.MerchantID, &item.Name, &item.LogoURL, &item.Description, &item.ServicePhone, &item.Status); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListProducts(ctx context.Context, keyword string, categoryID string) []domain.ProductCard {
	args := make([]any, 0, 3)
	query := productCardSelect() + ` WHERE p.status = 'active'`
	if categoryID != "" {
		query += " AND (p.category_id = ? OR c.parent_id = ?)"
		args = append(args, categoryID, categoryID)
	}
	if keyword != "" {
		terms := append([]string{keyword}, rag.QueryTerms(keyword)...)
		clauses := make([]string, 0, len(terms)*5)
		for _, term := range uniqueTerms(terms, 10) {
			like := "%" + term + "%"
			clauses = append(clauses, `p.name LIKE ?`, `p.brand LIKE ?`, `c.name LIKE ?`, `p.tags_json LIKE ?`, `p.selling_points_json LIKE ?`)
			args = append(args, like, like, like, like, like)
		}
		if len(clauses) > 0 {
			query += ` AND (` + strings.Join(clauses, ` OR `) + `)`
		}
	}
	query += " ORDER BY p.sort_order, p.product_id"
	return s.queryProductCards(ctx, query, args...)
}

func (s *MySQLStore) ListAllProducts(ctx context.Context) []domain.ProductCard {
	return s.queryProductCards(ctx, productCardSelect()+" ORDER BY p.sort_order, p.product_id")
}

func (s *MySQLStore) ListAllProductsPage(ctx context.Context, page int, pageSize int) ([]domain.ProductCard, int) {
	page, pageSize = normalizePage(page, pageSize)
	total := 0
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM products p
		JOIN merchants m ON m.merchant_id = p.merchant_id
		LEFT JOIN categories c ON c.category_id = p.category_id
	`).Scan(&total); err != nil {
		return nil, 0
	}
	items := s.queryProductCards(ctx, productCardSelect()+" ORDER BY p.sort_order, p.product_id LIMIT ? OFFSET ?", pageSize, pageOffset(page, pageSize))
	return items, total
}

func (s *MySQLStore) GetProduct(ctx context.Context, productID string) (domain.ProductDetail, bool) {
	row := s.db.QueryRowContext(ctx, productDetailSelect()+` WHERE p.product_id = ?`, productID)
	product, err := scanProductDetail(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProductDetail{}, false
	}
	if err != nil {
		return domain.ProductDetail{}, false
	}
	return product, true
}

func (s *MySQLStore) SearchProductsByImageVector(ctx context.Context, vector []float32, limit int) ([]domain.ProductCard, error) {
	if s.vector == nil {
		return nil, errors.New("image vector index disabled")
	}
	hits, err := s.vector.SearchProductImages(ctx, vector, limit)
	if err != nil {
		return nil, err
	}
	productIDs := make([]string, 0, len(hits))
	seen := make(map[string]bool, len(hits))
	for _, hit := range hits {
		productID := fmt.Sprint(hit.Fields["product_id"])
		if productID == "" || productID == "<nil>" || seen[productID] {
			continue
		}
		seen[productID] = true
		productIDs = append(productIDs, productID)
	}
	if len(productIDs) == 0 {
		return []domain.ProductCard{}, nil
	}
	cards := s.productCardMapByIDs(ctx, productIDs)
	out := make([]domain.ProductCard, 0, len(productIDs))
	for _, id := range productIDs {
		if card, ok := cards[id]; ok {
			out = append(out, card)
		}
	}
	return out, nil
}

func (s *MySQLStore) CreateProduct(ctx context.Context, input domain.ProductUpsertInput) (domain.ProductDetail, error) {
	productID := input.ProductID
	if productID == "" {
		productID = nextID("prod")
	}
	input.ProductID = productID
	normalized := normalizeProductInput(input)
	if err := s.insertProduct(ctx, normalized); err != nil {
		return domain.ProductDetail{}, err
	}
	if err := s.upsertDefaultSKU(ctx, normalized); err != nil {
		return domain.ProductDetail{}, err
	}
	product, ok := s.GetProduct(ctx, productID)
	if !ok {
		return domain.ProductDetail{}, errors.New("created product not found")
	}
	return product, nil
}

func (s *MySQLStore) insertProduct(ctx context.Context, input domain.ProductUpsertInput) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO products (
			product_id, merchant_id, name, brand, category_id, image_url, image_urls_json,
			price, market_price, stock_quantity, stock_status, tags_json, selling_points_json,
			recommend_reason, risk_notes_json, attributes_json, suitable_for_json,
			not_suitable_for_json, description, status, sort_order, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', 100, ?, ?)
	`,
		input.ProductID,
		input.MerchantID,
		input.Name,
		input.Brand,
		input.CategoryID,
		input.ImageURL,
		mustJSON([]string{input.ImageURL}),
		input.Price,
		input.MarketPrice,
		input.StockQuantity,
		input.StockStatus,
		mustJSON(input.Tags),
		mustJSON(input.SellingPoints),
		input.RecommendReason,
		mustJSON(input.RiskNotes),
		mustJSON([]domain.ProductAttribute{}),
		mustJSON([]string{}),
		mustJSON([]string{}),
		input.Description,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

func (s *MySQLStore) upsertDefaultSKU(ctx context.Context, input domain.ProductUpsertInput) error {
	skuID := "sku_" + input.ProductID
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO product_skus (sku_id, product_id, sku_name, price, stock_quantity, stock_status, specs_json, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?)
		ON DUPLICATE KEY UPDATE sku_name = VALUES(sku_name), price = VALUES(price),
			stock_quantity = VALUES(stock_quantity), stock_status = VALUES(stock_status),
			specs_json = VALUES(specs_json), is_default = TRUE, updated_at = VALUES(updated_at)
	`, skuID, input.ProductID, input.Name+" 默认款", input.Price, input.StockQuantity, input.StockStatus, mustJSON(map[string]string{"版本": "默认款"}), time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("upsert default sku: %w", err)
	}
	return nil
}

func (s *MySQLStore) UpdateProduct(ctx context.Context, productID string, input domain.ProductUpsertInput) (domain.ProductDetail, bool) {
	input.ProductID = productID
	normalized := normalizeProductInput(input)
	result, err := s.db.ExecContext(ctx, `
		UPDATE products
		SET name = ?, brand = ?, category_id = ?, image_url = ?, image_urls_json = ?,
			price = ?, market_price = ?, stock_quantity = ?, stock_status = ?,
			tags_json = ?, selling_points_json = ?, recommend_reason = ?,
			risk_notes_json = ?, attributes_json = ?, suitable_for_json = ?,
			not_suitable_for_json = ?, description = ?, updated_at = ?
		WHERE product_id = ? AND merchant_id = ?
	`,
		normalized.Name,
		normalized.Brand,
		normalized.CategoryID,
		normalized.ImageURL,
		mustJSON([]string{normalized.ImageURL}),
		normalized.Price,
		normalized.MarketPrice,
		normalized.StockQuantity,
		normalized.StockStatus,
		mustJSON(normalized.Tags),
		mustJSON(normalized.SellingPoints),
		normalized.RecommendReason,
		mustJSON(normalized.RiskNotes),
		mustJSON([]domain.ProductAttribute{}),
		mustJSON([]string{}),
		mustJSON([]string{}),
		normalized.Description,
		time.Now(),
		productID,
		normalized.MerchantID,
	)
	if err != nil {
		return domain.ProductDetail{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ProductDetail{}, false
	}
	if err := s.upsertDefaultSKU(ctx, normalized); err != nil {
		return domain.ProductDetail{}, false
	}
	product, ok := s.GetProduct(ctx, productID)
	return product, ok
}

func (s *MySQLStore) UpdateProductStatus(ctx context.Context, merchantID string, productID string, status string) (domain.ProductDetail, bool) {
	args := []any{status, time.Now(), productID}
	query := `UPDATE products SET status = ?, updated_at = ? WHERE product_id = ?`
	if merchantID != "" {
		query += ` AND merchant_id = ?`
		args = append(args, merchantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.ProductDetail{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ProductDetail{}, false
	}
	product, ok := s.GetProduct(ctx, productID)
	return product, ok
}

func (s *MySQLStore) ListProductSKUs(ctx context.Context, productID string) []domain.ProductSKU {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sku_id, product_id, sku_name, price, stock_quantity, stock_status, specs_json
		FROM product_skus
		WHERE product_id = ?
		ORDER BY sku_id
	`, productID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.ProductSKU, 0)
	for rows.Next() {
		item, err := scanSKU(rows)
		if err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) GetCart(ctx context.Context, accountID string) domain.Cart {
	return s.loadCart(ctx, accountID)
}

func (s *MySQLStore) AddCartItem(ctx context.Context, accountID string, productID string, skuID string, quantity int) (domain.Cart, bool) {
	if quantity <= 0 {
		quantity = 1
	}
	if _, ok := s.GetProduct(ctx, productID); !ok {
		return domain.Cart{}, false
	}
	// 前端可以只传商品 ID；未指定 SKU 时默认选择该商品的第一个 SKU。
	if skuID == "" {
		skuID = s.firstSKU(ctx, productID)
	}
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cart_items (cart_item_id, account_id, product_id, sku_id, quantity, selected, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, TRUE, ?, ?)
		ON DUPLICATE KEY UPDATE
			quantity = quantity + VALUES(quantity),
			selected = TRUE,
			updated_at = VALUES(updated_at)
	`, nextID("cart"), accountID, productID, skuID, quantity, now, now)
	if err != nil {
		return domain.Cart{}, false
	}
	return s.loadCart(ctx, accountID), true
}

func (s *MySQLStore) UpdateCartItem(ctx context.Context, accountID string, cartItemID string, quantity *int, selected *bool) (domain.Cart, bool) {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if quantity != nil && *quantity > 0 {
		sets = append(sets, "quantity = ?")
		args = append(args, *quantity)
	}
	if selected != nil {
		sets = append(sets, "selected = ?")
		args = append(args, *selected)
	}
	if len(sets) == 0 {
		return s.loadCart(ctx, accountID), true
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now(), cartItemID, accountID)
	result, err := s.db.ExecContext(ctx, `UPDATE cart_items SET `+strings.Join(sets, ", ")+` WHERE cart_item_id = ? AND account_id = ?`, args...)
	if err != nil {
		return domain.Cart{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Cart{}, false
	}
	return s.loadCart(ctx, accountID), true
}

func (s *MySQLStore) DeleteCartItem(ctx context.Context, accountID string, cartItemID string) (domain.Cart, bool) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM cart_items WHERE cart_item_id = ? AND account_id = ?`, cartItemID, accountID)
	if err != nil {
		return domain.Cart{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Cart{}, false
	}
	return s.loadCart(ctx, accountID), true
}

func (s *MySQLStore) CreateOrderFromCart(ctx context.Context, accountID string) ([]domain.Order, bool) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false
	}
	defer tx.Rollback()

	selected, ok := s.lockSelectedCartItems(ctx, tx, accountID)
	if !ok || len(selected) == 0 {
		return nil, false
	}
	for _, item := range selected {
		if !s.reserveOrderStock(ctx, tx, item.ProductID, item.SkuID, item.Quantity) {
			return nil, false
		}
	}

	// 一个购物车可能包含多个商家的商品，按商家拆单，便于后续商家侧发货和售后。
	byMerchant := make(map[string][]domain.CartItem)
	for _, item := range selected {
		byMerchant[item.MerchantID] = append(byMerchant[item.MerchantID], item)
	}

	orders := make([]domain.Order, 0, len(byMerchant))
	for merchantID, items := range byMerchant {
		orderID := nextID("ord")
		total := 0.0
		merchantName := ""
		for _, item := range items {
			price, _ := parseAmount(item.Price)
			total += price * float64(item.Quantity)
			merchantName = item.MerchantName
		}
		amount := fmt.Sprintf("%.2f", total)
		createdAt := time.Now()
		orderNo := nextOrderNo(createdAt)
		deadline := createdAt.Add(30 * time.Minute)
		// 订单初始状态固定为待支付，同时写入支付截止时间，超时任务会自动关单。
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders (
				order_id, order_no, account_id, merchant_id, status, total_amount, discount_amount, pay_amount,
				payment_deadline_at, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, orderID, orderNo, accountID, merchantID, "pending_payment", amount, "0.00", amount, deadline, createdAt, createdAt); err != nil {
			return nil, false
		}
		// 当前支付仍是内部模拟支付；保留 payments 表是为了让真实支付网关后续能无缝接入。
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO payments (payment_id, order_id, account_id, amount, status, method, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nextID("pay"), orderID, accountID, amount, "pending", "mock", deadline, createdAt, createdAt); err != nil {
			return nil, false
		}
		orderItems := make([]domain.OrderItem, 0, len(items))
		for _, item := range items {
			orderItem := domain.OrderItem{
				OrderItemID:  nextID("ord_item"),
				ProductID:    item.ProductID,
				SkuID:        item.SkuID,
				Name:         item.Name,
				ImageURL:     item.ImageURL,
				Price:        item.Price,
				Quantity:     item.Quantity,
				MerchantID:   item.MerchantID,
				MerchantName: item.MerchantName,
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO order_items (order_item_id, order_id, product_id, sku_id, name, image_url, price, quantity, merchant_id, merchant_name, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, orderItem.OrderItemID, orderID, orderItem.ProductID, orderItem.SkuID, orderItem.Name, orderItem.ImageURL, orderItem.Price, orderItem.Quantity, orderItem.MerchantID, orderItem.MerchantName, createdAt); err != nil {
				return nil, false
			}
			orderItems = append(orderItems, orderItem)
		}
		orders = append(orders, domain.Order{
			OrderID:           orderID,
			OrderNo:           orderNo,
			AccountID:         accountID,
			MerchantID:        merchantID,
			MerchantName:      merchantName,
			Status:            "pending_payment",
			TotalAmount:       amount,
			DiscountAmount:    "0.00",
			PayAmount:         amount,
			PaymentDeadlineAt: deadline,
			Items:             orderItems,
			CreatedAt:         createdAt,
			UpdatedAt:         createdAt,
		})
	}
	// 订单创建成功后清空已结算项，避免用户重复提交同一批购物车商品。
	if _, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE account_id = ? AND selected = TRUE`, accountID); err != nil {
		return nil, false
	}
	if err := tx.Commit(); err != nil {
		return nil, false
	}
	return orders, true
}

func (s *MySQLStore) lockSelectedCartItems(ctx context.Context, tx *sql.Tx, accountID string) ([]domain.CartItem, bool) {
	rows, err := tx.QueryContext(ctx, `
		SELECT
			ci.cart_item_id,
			ci.product_id,
			ci.sku_id,
			ci.quantity,
			ci.selected,
			p.name,
			p.image_url,
			COALESCE(ps.price, p.price) AS price,
			p.stock_status,
			p.merchant_id,
			m.name AS merchant_name
		FROM cart_items ci
		JOIN products p ON p.product_id = ci.product_id AND p.status = 'active'
		JOIN merchants m ON m.merchant_id = p.merchant_id
		LEFT JOIN product_skus ps ON ps.product_id = p.product_id AND ps.sku_id = ci.sku_id
		WHERE ci.account_id = ? AND ci.selected = TRUE
		ORDER BY ci.created_at, ci.cart_item_id
		FOR UPDATE
	`, accountID)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	items := make([]domain.CartItem, 0)
	for rows.Next() {
		var item domain.CartItem
		if err := rows.Scan(
			&item.CartItemID,
			&item.ProductID,
			&item.SkuID,
			&item.Quantity,
			&item.Selected,
			&item.Name,
			&item.ImageURL,
			&item.Price,
			&item.StockStatus,
			&item.MerchantID,
			&item.MerchantName,
		); err != nil {
			return nil, false
		}
		if item.Quantity <= 0 {
			return nil, false
		}
		items = append(items, item)
	}
	return items, rows.Err() == nil
}

func (s *MySQLStore) reserveOrderStock(ctx context.Context, tx *sql.Tx, productID string, skuID string, quantity int) bool {
	if quantity <= 0 {
		return false
	}
	now := time.Now()
	if skuID != "" {
		result, err := tx.ExecContext(ctx, `
			UPDATE product_skus
			SET stock_quantity = stock_quantity - ?,
				stock_status = CASE WHEN stock_quantity - ? <= 0 THEN 'out_of_stock' ELSE stock_status END,
				updated_at = ?
			WHERE product_id = ? AND sku_id = ? AND stock_quantity >= ?
		`, quantity, quantity, now, productID, skuID, quantity)
		if err != nil {
			return false
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			return false
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE products
		SET stock_quantity = stock_quantity - ?,
			stock_status = CASE WHEN stock_quantity - ? <= 0 THEN 'out_of_stock' ELSE stock_status END,
			updated_at = ?
		WHERE product_id = ? AND stock_quantity >= ?
	`, quantity, quantity, now, productID, quantity)
	if err != nil {
		return false
	}
	affected, err := result.RowsAffected()
	return err == nil && affected > 0
}

func (s *MySQLStore) ListUserOrders(ctx context.Context, accountID string) []domain.Order {
	// 查询订单前先收敛超时待支付订单，保证列表看到的是最新业务状态。
	s.ExpirePendingOrders(ctx)
	return s.listOrders(ctx, "WHERE o.account_id = ?", accountID)
}

func (s *MySQLStore) ListMerchantOrders(ctx context.Context, merchantID string) []domain.Order {
	s.ExpirePendingOrders(ctx)
	return s.listOrders(ctx, "WHERE o.merchant_id = ?", merchantID)
}

func (s *MySQLStore) ListAllOrders(ctx context.Context) []domain.Order {
	s.ExpirePendingOrders(ctx)
	return s.listOrders(ctx, "")
}

func (s *MySQLStore) ListAllOrdersPage(ctx context.Context, page int, pageSize int) ([]domain.Order, int) {
	s.ExpirePendingOrders(ctx)
	page, pageSize = normalizePage(page, pageSize)
	total := s.countRows(ctx, "orders")
	items := s.listOrders(ctx, "")
	return paginateSlice(items, page, pageSize), total
}

func (s *MySQLStore) GetOrder(ctx context.Context, accountID string, orderID string) (domain.Order, bool) {
	s.ExpirePendingOrders(ctx)
	orders := s.listOrders(ctx, "WHERE o.order_id = ? AND o.account_id = ?", orderID, accountID)
	if len(orders) == 0 {
		return domain.Order{}, false
	}
	return orders[0], true
}

func (s *MySQLStore) PayOrder(ctx context.Context, accountID string, orderID string, method string) (domain.Order, domain.Payment, bool) {
	s.ExpirePendingOrders(ctx)
	method = strings.TrimSpace(method)
	if method == "" {
		method = "mock_balance"
	}
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Order{}, domain.Payment{}, false
	}
	defer tx.Rollback()

	var amount string
	var deadline sql.NullTime
	// 使用 FOR UPDATE 锁定订单行，避免同一订单被并发支付两次。
	err = tx.QueryRowContext(ctx, `
		SELECT pay_amount, payment_deadline_at
		FROM orders
		WHERE order_id = ? AND account_id = ? AND status = 'pending_payment'
		FOR UPDATE
	`, orderID, accountID).Scan(&amount, &deadline)
	if err != nil {
		return domain.Order{}, domain.Payment{}, false
	}
	// 锁内再次校验支付截止时间，处理查询和支付提交之间刚好过期的边界情况。
	if deadline.Valid && now.After(deadline.Time) {
		_, _ = tx.ExecContext(ctx, `
			UPDATE orders SET status = 'closed_timeout', closed_at = ?, cancel_reason = '支付超时自动关闭', updated_at = ?
			WHERE order_id = ?
		`, now, now, orderID)
		_, _ = tx.ExecContext(ctx, `
			UPDATE payments SET status = 'expired', updated_at = ?
			WHERE order_id = ? AND status = 'pending'
		`, now, orderID)
		_ = tx.Commit()
		return domain.Order{}, domain.Payment{}, false
	}
	// 支付成功后订单进入待发货，真实接入支付渠道时这里应由支付回调驱动。
	if _, err := tx.ExecContext(ctx, `
		UPDATE orders
		SET status = 'pending_ship', paid_at = ?, updated_at = ?
		WHERE order_id = ? AND account_id = ? AND status = 'pending_payment'
	`, now, now, orderID, accountID); err != nil {
		return domain.Order{}, domain.Payment{}, false
	}
	transactionNo := nextID("txn")
	if _, err := tx.ExecContext(ctx, `
		UPDATE payments
		SET status = 'success', method = ?, transaction_no = ?, paid_at = ?, updated_at = ?
		WHERE order_id = ? AND account_id = ? AND status = 'pending'
	`, method, transactionNo, now, now, orderID, accountID); err != nil {
		return domain.Order{}, domain.Payment{}, false
	}
	if err := tx.Commit(); err != nil {
		return domain.Order{}, domain.Payment{}, false
	}
	order, ok := s.GetOrder(ctx, accountID, orderID)
	if !ok {
		return domain.Order{}, domain.Payment{}, false
	}
	payment, _ := s.getLatestPayment(ctx, accountID, orderID)
	return order, payment, true
}

func (s *MySQLStore) CancelOrder(ctx context.Context, accountID string, orderID string, reason string) (domain.Order, bool) {
	s.ExpirePendingOrders(ctx)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "用户取消"
	}
	now := time.Now()
	// 当前只允许用户取消待支付订单；已支付订单后续应走售后/退款流程。
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders
		SET status = 'canceled', closed_at = ?, cancel_reason = ?, updated_at = ?
		WHERE order_id = ? AND account_id = ? AND status = 'pending_payment'
	`, now, truncateRunes(reason, 256), now, orderID, accountID)
	if err != nil {
		return domain.Order{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Order{}, false
	}
	_, _ = s.db.ExecContext(ctx, `
		UPDATE payments SET status = 'failed', updated_at = ?
		WHERE order_id = ? AND account_id = ? AND status = 'pending'
	`, now, orderID, accountID)
	return s.GetOrder(ctx, accountID, orderID)
}

func (s *MySQLStore) ConfirmReceipt(ctx context.Context, accountID string, orderID string) (domain.Order, bool) {
	s.ExpirePendingOrders(ctx)
	now := time.Now()
	// 用户只能确认已发货订单，确认后进入 completed，之后才允许发布商品评价。
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders
		SET status = 'completed', completed_at = ?, updated_at = ?
		WHERE order_id = ? AND account_id = ? AND status = 'shipped'
	`, now, now, orderID, accountID)
	if err != nil {
		return domain.Order{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Order{}, false
	}
	return s.GetOrder(ctx, accountID, orderID)
}

func (s *MySQLStore) ExpirePendingOrders(ctx context.Context) int {
	now := time.Now()
	// 这是轻量的惰性过期机制：每次订单相关查询/操作都会顺手关闭超时待支付订单。
	result, err := s.db.ExecContext(ctx, `
		UPDATE orders
		SET status = 'closed_timeout', closed_at = ?, cancel_reason = '支付超时自动关闭', updated_at = ?
		WHERE status = 'pending_payment' AND payment_deadline_at IS NOT NULL AND payment_deadline_at < ?
	`, now, now, now)
	if err != nil {
		return 0
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return 0
	}
	// 订单超时后同步把仍处于 pending 的支付单标记为 expired，保持两张表状态一致。
	_, _ = s.db.ExecContext(ctx, `
		UPDATE payments p
		JOIN orders o ON o.order_id = p.order_id
		SET p.status = 'expired', p.updated_at = ?
		WHERE p.status = 'pending' AND o.status = 'closed_timeout'
	`, now)
	return int(affected)
}

func (s *MySQLStore) UpdateOrderStatus(ctx context.Context, merchantID string, orderID string, status string) (domain.Order, bool) {
	s.ExpirePendingOrders(ctx)
	now := time.Now()
	// 商家侧目前支持待发货/已发货之间的流转；管理员可传空 merchantID 跨商家操作。
	sets := `status = ?, updated_at = ?`
	args := []any{status, now}
	if status == "completed" {
		sets += `, completed_at = ?`
		args = append(args, now)
	}
	query := `UPDATE orders SET ` + sets + ` WHERE order_id = ?`
	args = append(args, orderID)
	if merchantID != "" {
		query += ` AND merchant_id = ?`
		args = append(args, merchantID)
	}
	query += ` AND status IN ('pending_ship', 'shipped')`
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.Order{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.Order{}, false
	}
	orders := s.listOrders(ctx, "WHERE o.order_id = ?", orderID)
	if len(orders) == 0 {
		return domain.Order{}, false
	}
	return orders[0], true
}

func (s *MySQLStore) PreviewCartDiscount(ctx context.Context, accountID string) domain.DiscountPreview {
	cart := s.loadCart(ctx, accountID)
	total := 0.0
	merchantTotals := make(map[string]float64)
	// 优惠试算只基于购物车选中项，并同时维护整单金额和商家维度金额。
	for _, item := range cart.Items {
		if !item.Selected {
			continue
		}
		price, _ := parseAmount(item.Price)
		amount := price * float64(item.Quantity)
		total += amount
		merchantTotals[item.MerchantID] += amount
	}
	lines := make([]domain.DiscountLine, 0)
	discount := 0.0
	now := time.Now()
	for _, promotion := range s.ListPromotions(ctx, "") {
		// 平台券按整单金额计算，商家券只按该商家的商品金额计算。
		if promotion.Status != "active" || now.Before(promotion.StartAt) || now.After(promotion.EndAt) {
			continue
		}
		base := total
		if promotion.Scope == "merchant" {
			base = merchantTotals[promotion.MerchantID]
		}
		lineAmount := discountAmountFor(base, promotion.Type, promotion.ThresholdAmount, promotion.DiscountAmount, promotion.DiscountRate)
		if lineAmount <= 0 {
			continue
		}
		discount += lineAmount
		lines = append(lines, domain.DiscountLine{Type: "promotion", ID: promotion.PromotionID, Name: promotion.Name, Amount: fmt.Sprintf("%.2f", lineAmount)})
	}
	for _, userCoupon := range s.ListUserCoupons(ctx, accountID) {
		// 这里只做试算，不会把优惠券置为 used；真正核销应放在支付/下单链路里。
		if userCoupon.Status != "unused" || userCoupon.Coupon.Status != "active" || now.Before(userCoupon.Coupon.StartAt) || now.After(userCoupon.Coupon.EndAt) {
			continue
		}
		base := total
		if userCoupon.Coupon.Scope == "merchant" {
			base = merchantTotals[userCoupon.Coupon.MerchantID]
		}
		lineAmount := discountAmountFor(base, userCoupon.Coupon.Type, userCoupon.Coupon.ThresholdAmount, userCoupon.Coupon.DiscountAmount, "0")
		if lineAmount <= 0 {
			continue
		}
		discount += lineAmount
		lines = append(lines, domain.DiscountLine{Type: "coupon", ID: userCoupon.UserCouponID, Name: userCoupon.Coupon.Name, Amount: fmt.Sprintf("%.2f", lineAmount)})
	}
	if discount > total {
		discount = total
	}
	return domain.DiscountPreview{
		TotalAmount:    fmt.Sprintf("%.2f", total),
		DiscountAmount: fmt.Sprintf("%.2f", discount),
		PayAmount:      fmt.Sprintf("%.2f", total-discount),
		Lines:          lines,
	}
}

func (s *MySQLStore) ListPromotions(ctx context.Context, merchantID string) []domain.PromotionRule {
	query := `
		SELECT promotion_id, name, scope, merchant_id, product_id, category_id, type, threshold_amount,
			discount_amount, discount_rate, stackable, start_at, end_at, status, created_at, updated_at
		FROM promotion_rules
	`
	args := []any{}
	if merchantID != "" {
		query += ` WHERE merchant_id = ?`
		args = append(args, merchantID)
	}
	query += ` ORDER BY created_at DESC, promotion_id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.PromotionRule, 0)
	for rows.Next() {
		var item domain.PromotionRule
		if err := rows.Scan(
			&item.PromotionID, &item.Name, &item.Scope, &item.MerchantID, &item.ProductID, &item.CategoryID, &item.Type,
			&item.ThresholdAmount, &item.DiscountAmount, &item.DiscountRate, &item.Stackable, &item.StartAt, &item.EndAt,
			&item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) CreatePromotion(ctx context.Context, input domain.PromotionRuleInput) (domain.PromotionRule, error) {
	now := time.Now()
	startAt := parseAPITime(input.StartAt, now)
	endAt := parseAPITime(input.EndAt, now.Add(30*24*time.Hour))
	if input.Scope == "" {
		input.Scope = "platform"
	}
	if input.Type == "" {
		input.Type = "full_reduction"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.ThresholdAmount == "" {
		input.ThresholdAmount = "0.00"
	}
	if input.DiscountAmount == "" {
		input.DiscountAmount = "0.00"
	}
	if input.DiscountRate == "" {
		input.DiscountRate = "0.0000"
	}
	item := domain.PromotionRule{
		PromotionID:     nextID("promo"),
		Name:            strings.TrimSpace(input.Name),
		Scope:           strings.TrimSpace(input.Scope),
		MerchantID:      strings.TrimSpace(input.MerchantID),
		ProductID:       strings.TrimSpace(input.ProductID),
		CategoryID:      strings.TrimSpace(input.CategoryID),
		Type:            strings.TrimSpace(input.Type),
		ThresholdAmount: input.ThresholdAmount,
		DiscountAmount:  input.DiscountAmount,
		DiscountRate:    input.DiscountRate,
		Stackable:       input.Stackable,
		StartAt:         startAt,
		EndAt:           endAt,
		Status:          input.Status,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if item.Name == "" {
		return domain.PromotionRule{}, errors.New("empty promotion name")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO promotion_rules (
			promotion_id, name, scope, merchant_id, product_id, category_id, type, threshold_amount,
			discount_amount, discount_rate, stackable, start_at, end_at, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.PromotionID, item.Name, item.Scope, item.MerchantID, item.ProductID, item.CategoryID, item.Type,
		item.ThresholdAmount, item.DiscountAmount, item.DiscountRate, item.Stackable, item.StartAt, item.EndAt,
		item.Status, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return domain.PromotionRule{}, err
	}
	return item, nil
}

func (s *MySQLStore) UpdatePromotionStatus(ctx context.Context, promotionID string, merchantID string, status string) (domain.PromotionRule, bool) {
	args := []any{status, time.Now(), promotionID}
	query := `UPDATE promotion_rules SET status = ?, updated_at = ? WHERE promotion_id = ?`
	if merchantID != "" {
		query += ` AND merchant_id = ?`
		args = append(args, merchantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return domain.PromotionRule{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.PromotionRule{}, false
	}
	items := s.ListPromotions(ctx, merchantID)
	for _, item := range items {
		if item.PromotionID == promotionID {
			return item, true
		}
	}
	return domain.PromotionRule{}, false
}

func (s *MySQLStore) ListCoupons(ctx context.Context, accountID string) []domain.Coupon {
	now := time.Now()
	rows, err := s.db.QueryContext(ctx, `
		SELECT coupon_id, name, scope, merchant_id, type, threshold_amount, discount_amount, total_count,
			claimed_count, per_user_limit, start_at, end_at, status, created_at, updated_at
		FROM coupons
		WHERE status = 'active' AND start_at <= ? AND end_at >= ?
		ORDER BY created_at DESC, coupon_id DESC
	`, now, now)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.Coupon, 0)
	for rows.Next() {
		var item domain.Coupon
		if err := rows.Scan(&item.CouponID, &item.Name, &item.Scope, &item.MerchantID, &item.Type, &item.ThresholdAmount,
			&item.DiscountAmount, &item.TotalCount, &item.ClaimedCount, &item.PerUserLimit, &item.StartAt, &item.EndAt,
			&item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListUserCoupons(ctx context.Context, accountID string) []domain.UserCoupon {
	rows, err := s.db.QueryContext(ctx, `
		SELECT uc.user_coupon_id, uc.coupon_id, uc.account_id, uc.status, uc.order_id, uc.claimed_at, uc.used_at,
			c.coupon_id, c.name, c.scope, c.merchant_id, c.type, c.threshold_amount, c.discount_amount,
			c.total_count, c.claimed_count, c.per_user_limit, c.start_at, c.end_at, c.status, c.created_at, c.updated_at
		FROM user_coupons uc
		JOIN coupons c ON c.coupon_id = uc.coupon_id
		WHERE uc.account_id = ?
		ORDER BY uc.claimed_at DESC, uc.user_coupon_id DESC
	`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.UserCoupon, 0)
	for rows.Next() {
		var item domain.UserCoupon
		var usedAt sql.NullTime
		if err := rows.Scan(&item.UserCouponID, &item.CouponID, &item.AccountID, &item.Status, &item.OrderID, &item.ClaimedAt, &usedAt,
			&item.Coupon.CouponID, &item.Coupon.Name, &item.Coupon.Scope, &item.Coupon.MerchantID, &item.Coupon.Type,
			&item.Coupon.ThresholdAmount, &item.Coupon.DiscountAmount, &item.Coupon.TotalCount, &item.Coupon.ClaimedCount,
			&item.Coupon.PerUserLimit, &item.Coupon.StartAt, &item.Coupon.EndAt, &item.Coupon.Status,
			&item.Coupon.CreatedAt, &item.Coupon.UpdatedAt); err != nil {
			return nil
		}
		if usedAt.Valid {
			item.UsedAt = usedAt.Time
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ClaimCoupon(ctx context.Context, accountID string, couponID string) (domain.UserCoupon, bool) {
	now := time.Now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.UserCoupon{}, false
	}
	defer tx.Rollback()
	var coupon domain.Coupon
	// 锁定优惠券行后再判断库存和每人限领，避免高并发下超发。
	err = tx.QueryRowContext(ctx, `
		SELECT coupon_id, name, scope, merchant_id, type, threshold_amount, discount_amount, total_count,
			claimed_count, per_user_limit, start_at, end_at, status, created_at, updated_at
		FROM coupons
		WHERE coupon_id = ? AND status = 'active' AND start_at <= ? AND end_at >= ?
		FOR UPDATE
	`, couponID, now, now).Scan(&coupon.CouponID, &coupon.Name, &coupon.Scope, &coupon.MerchantID, &coupon.Type,
		&coupon.ThresholdAmount, &coupon.DiscountAmount, &coupon.TotalCount, &coupon.ClaimedCount, &coupon.PerUserLimit,
		&coupon.StartAt, &coupon.EndAt, &coupon.Status, &coupon.CreatedAt, &coupon.UpdatedAt)
	if err != nil {
		return domain.UserCoupon{}, false
	}
	if coupon.TotalCount > 0 && coupon.ClaimedCount >= coupon.TotalCount {
		return domain.UserCoupon{}, false
	}
	var owned int
	// 同一个用户对同一张券最多领取 per_user_limit 次。
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_coupons WHERE account_id = ? AND coupon_id = ?`, accountID, couponID).Scan(&owned); err != nil {
		return domain.UserCoupon{}, false
	}
	if owned >= coupon.PerUserLimit {
		return domain.UserCoupon{}, false
	}
	item := domain.UserCoupon{UserCouponID: nextID("uc"), CouponID: couponID, AccountID: accountID, Status: "unused", ClaimedAt: now, Coupon: coupon}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_coupons (user_coupon_id, coupon_id, account_id, status, claimed_at)
		VALUES (?, ?, ?, ?, ?)
	`, item.UserCouponID, item.CouponID, item.AccountID, item.Status, item.ClaimedAt); err != nil {
		return domain.UserCoupon{}, false
	}
	if _, err := tx.ExecContext(ctx, `UPDATE coupons SET claimed_count = claimed_count + 1, updated_at = ? WHERE coupon_id = ?`, now, couponID); err != nil {
		return domain.UserCoupon{}, false
	}
	if err := tx.Commit(); err != nil {
		return domain.UserCoupon{}, false
	}
	coupon.ClaimedCount++
	item.Coupon = coupon
	return item, true
}

func (s *MySQLStore) ListProductReviews(ctx context.Context, productID string) []domain.ProductReview {
	return s.listReviews(ctx, "WHERE r.product_id = ? AND r.status = 'visible'", productID)
}

func (s *MySQLStore) CreateProductReview(ctx context.Context, accountID string, orderID string, orderItemID string, input domain.ProductReviewInput) (domain.ProductReview, bool) {
	if input.Rating < 1 || input.Rating > 5 || strings.TrimSpace(input.Content) == "" {
		return domain.ProductReview{}, false
	}
	var productID string
	var skuID string
	var status string
	// 评价必须绑定到当前用户的已完成订单项，避免未购买商品被随意刷评价。
	err := s.db.QueryRowContext(ctx, `
		SELECT oi.product_id, oi.sku_id, o.status
		FROM order_items oi
		JOIN orders o ON o.order_id = oi.order_id
		WHERE oi.order_item_id = ? AND oi.order_id = ? AND o.account_id = ?
	`, orderItemID, orderID, accountID).Scan(&productID, &skuID, &status)
	if err != nil || status != "completed" {
		return domain.ProductReview{}, false
	}
	// tags 以 JSON 存储，便于前端做标签化展示，也方便后续扩展为结构化评价维度。
	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return domain.ProductReview{}, false
	}
	now := time.Now()
	review := domain.ProductReview{
		ReviewID:    nextID("rev"),
		OrderID:     orderID,
		OrderItemID: orderItemID,
		ProductID:   productID,
		SkuID:       skuID,
		AccountID:   accountID,
		Rating:      input.Rating,
		Content:     strings.TrimSpace(input.Content),
		Tags:        input.Tags,
		Status:      "visible",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO product_reviews (
			review_id, order_id, order_item_id, product_id, sku_id, account_id, rating, content, tags_json, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, review.ReviewID, review.OrderID, review.OrderItemID, review.ProductID, review.SkuID, review.AccountID,
		review.Rating, review.Content, string(tagsJSON), review.Status, review.CreatedAt, review.UpdatedAt)
	if err != nil {
		return domain.ProductReview{}, false
	}
	return review, true
}

func (s *MySQLStore) ListMerchantReviews(ctx context.Context, merchantID string) []domain.ProductReview {
	return s.listReviews(ctx, "JOIN products p ON p.product_id = r.product_id WHERE p.merchant_id = ?", merchantID)
}

func (s *MySQLStore) ReplyReview(ctx context.Context, merchantID string, reviewID string, reply string) (domain.ProductReview, bool) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return domain.ProductReview{}, false
	}
	now := time.Now()
	// 回复时通过 products 关联校验商家归属，防止商家回复其他店铺的评价。
	result, err := s.db.ExecContext(ctx, `
		UPDATE product_reviews r
		JOIN products p ON p.product_id = r.product_id
		SET r.merchant_reply = ?, r.merchant_replied_at = ?, r.updated_at = ?
		WHERE r.review_id = ? AND p.merchant_id = ?
	`, truncateRunes(reply, 1000), now, now, reviewID, merchantID)
	if err != nil {
		return domain.ProductReview{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ProductReview{}, false
	}
	reviews := s.listReviews(ctx, "JOIN products p ON p.product_id = r.product_id WHERE r.review_id = ? AND p.merchant_id = ?", reviewID, merchantID)
	if len(reviews) == 0 {
		return domain.ProductReview{}, false
	}
	return reviews[0], true
}

func (s *MySQLStore) ListAllReviews(ctx context.Context) []domain.ProductReview {
	return s.listReviews(ctx, "")
}

func (s *MySQLStore) UpdateReviewStatus(ctx context.Context, reviewID string, status string) (domain.ProductReview, bool) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `UPDATE product_reviews SET status = ?, updated_at = ? WHERE review_id = ?`, status, now, reviewID)
	if err != nil {
		return domain.ProductReview{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil || affected == 0 {
		return domain.ProductReview{}, false
	}
	reviews := s.listReviews(ctx, "WHERE r.review_id = ?", reviewID)
	if len(reviews) == 0 {
		return domain.ProductReview{}, false
	}
	return reviews[0], true
}

func (s *MySQLStore) listReviews(ctx context.Context, where string, args ...any) []domain.ProductReview {
	// 评价查询统一走这个方法，保证用户昵称、标签 JSON、商家回复时间等字段解析一致。
	query := `
		SELECT r.review_id, r.order_id, r.order_item_id, r.product_id, r.sku_id, r.account_id,
			COALESCE(a.username, ''), r.rating, r.content, r.tags_json, r.status, COALESCE(r.merchant_reply, ''),
			r.merchant_replied_at, r.created_at, r.updated_at
		FROM product_reviews r
		LEFT JOIN accounts a ON a.account_id = r.account_id
	`
	if where != "" {
		query += " " + where
	}
	query += " ORDER BY r.created_at DESC, r.review_id DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.ProductReview, 0)
	for rows.Next() {
		var item domain.ProductReview
		var tagsJSON string
		var repliedAt sql.NullTime
		if err := rows.Scan(
			&item.ReviewID, &item.OrderID, &item.OrderItemID, &item.ProductID, &item.SkuID, &item.AccountID,
			&item.Username, &item.Rating, &item.Content, &tagsJSON, &item.Status, &item.MerchantReply,
			&repliedAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil
		}
		_ = json.Unmarshal([]byte(tagsJSON), &item.Tags)
		if repliedAt.Valid {
			item.MerchantRepliedAt = repliedAt.Time
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) SearchKnowledge(ctx context.Context, query string) []domain.Citation {
	// RAG 入口会把自然语言查询转成默认召回计划；空结果时退回默认知识片段兜底。
	plan := rag.DefaultRetrievalPlan(query)
	items := s.SearchKnowledgeByPlan(ctx, plan)
	if len(items) == 0 && query != "" {
		return s.SearchKnowledgeByPlan(ctx, rag.DefaultRetrievalPlan(""))
	}
	return items
}

func (s *MySQLStore) SearchKnowledgeByPlan(ctx context.Context, plan rag.RetrievalPlan) []domain.Citation {
	plan = rag.NormalizePlan(plan)
	args := make([]any, 0, 8)
	sqlQuery := `
		SELECT chunk_id, title, snippet, source
		FROM knowledge_chunks
	`
	terms := rag.QueryTerms(plan.Query)
	if plan.Query != "" {
		clauses := []string{`title LIKE ?`, `snippet LIKE ?`}
		like := "%" + plan.Query + "%"
		args = append(args, like, like)
		// 轻量版本先用 MySQL LIKE 做关键词召回，后续可替换为向量库或混合检索。
		for _, term := range terms {
			clauses = append(clauses, `title LIKE ?`, `snippet LIKE ?`)
			termLike := "%" + term + "%"
			args = append(args, termLike, termLike)
			if len(clauses) >= 14 {
				break
			}
		}
		sqlQuery += ` WHERE ` + strings.Join(clauses, ` OR `)
	}
	sqlQuery += ` ORDER BY sort_order, chunk_id LIMIT ?`
	limit := plan.Recall.Keyword.TopN
	if limit <= 0 {
		limit = rag.DefaultKeywordTopN
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	candidates := make([]rag.Candidate, 0)
	for rows.Next() {
		var item domain.Citation
		if err := rows.Scan(&item.ChunkID, &item.Title, &item.Snippet, &item.Source); err != nil {
			return nil
		}
		candidates = append(candidates, rag.Candidate{
			ChunkID: item.ChunkID,
			Title:   item.Title,
			Snippet: item.Snippet,
			Source:  item.Source,
		})
	}
	if s.vector != nil && s.vector.Enabled() && plan.Recall.Vector.Enabled {
		if hits, err := s.vector.SearchKnowledge(ctx, plan.Query, plan.Recall.Vector.TopN); err == nil {
			candidates = mergeCandidates(candidates, citationsToCandidates(s.knowledgeChunksByIDs(ctx, hitIDs(hits))))
		}
	}
	// 召回后交给 rag 包做统一排序和摘要截断，避免存储层承载过多策略逻辑。
	rankPlan := plan
	rankPlan.Rerank.TopK = len(candidates)
	ranked := rag.RankCandidates(rankPlan, candidates)
	ranked = s.filterKnowledgeCandidatesByProductFacets(ctx, plan.Query, ranked)
	if len(ranked) > plan.Rerank.TopK {
		ranked = ranked[:plan.Rerank.TopK]
	}
	items := make([]domain.Citation, 0, len(ranked))
	for _, item := range ranked {
		items = append(items, domain.Citation{
			ChunkID: item.ChunkID,
			Title:   item.Title,
			Snippet: rag.Snippet(item.Snippet, plan.Compress.MaxCharsPerChunk),
			Source:  item.Source,
		})
	}
	return items
}

func (s *MySQLStore) filterKnowledgeCandidatesByProductFacets(ctx context.Context, query string, candidates []rag.Candidate) []rag.Candidate {
	if query == "" || len(candidates) == 0 {
		return candidates
	}
	products := make(map[string]domain.ProductCard)
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate.Source, "p_") {
			continue
		}
		if _, ok := products[candidate.Source]; ok {
			continue
		}
		product, ok := s.GetProduct(ctx, candidate.Source)
		if ok {
			products[candidate.Source] = product.ProductCard
		}
	}
	if len(products) == 0 {
		return candidates
	}
	items := make([]domain.ProductCard, 0, len(products))
	for _, product := range products {
		items = append(items, product)
	}
	constraint := parseProductSearchConstraint(query, items)
	if len(constraint.brands) == 0 && len(constraint.categories) == 0 {
		return candidates
	}
	filtered := make([]rag.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		product, ok := products[candidate.Source]
		if !ok {
			filtered = append(filtered, candidate)
			continue
		}
		if _, ok := scoreProductSearchCandidate(product, constraint); ok {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		return candidates
	}
	return filtered
}

func (s *MySQLStore) ListMerchantDocuments(ctx context.Context, merchantID string) []domain.KnowledgeDocument {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_id, merchant_id, title, doc_type, status, chunk_count, created_at
		FROM knowledge_documents
		WHERE merchant_id = ?
		ORDER BY created_at DESC, document_id DESC
	`, merchantID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.KnowledgeDocument, 0)
	for rows.Next() {
		var item domain.KnowledgeDocument
		if err := rows.Scan(&item.DocumentID, &item.MerchantID, &item.Title, &item.DocType, &item.Status, &item.ChunkCount, &item.CreatedAt); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListAllDocuments(ctx context.Context) []domain.KnowledgeDocument {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_id, merchant_id, title, doc_type, status, chunk_count, created_at
		FROM knowledge_documents
		ORDER BY created_at DESC, document_id DESC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.KnowledgeDocument, 0)
	for rows.Next() {
		var item domain.KnowledgeDocument
		if err := rows.Scan(&item.DocumentID, &item.MerchantID, &item.Title, &item.DocType, &item.Status, &item.ChunkCount, &item.CreatedAt); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) ListAllDocumentsPage(ctx context.Context, page int, pageSize int) ([]domain.KnowledgeDocument, int) {
	page, pageSize = normalizePage(page, pageSize)
	total := s.countRows(ctx, "knowledge_documents")
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_id, merchant_id, title, doc_type, status, chunk_count, created_at
		FROM knowledge_documents
		ORDER BY created_at DESC, document_id DESC
		LIMIT ? OFFSET ?
	`, pageSize, pageOffset(page, pageSize))
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	items := make([]domain.KnowledgeDocument, 0)
	for rows.Next() {
		var item domain.KnowledgeDocument
		if err := rows.Scan(&item.DocumentID, &item.MerchantID, &item.Title, &item.DocType, &item.Status, &item.ChunkCount, &item.CreatedAt); err != nil {
			return nil, 0
		}
		items = append(items, item)
	}
	return items, total
}

func (s *MySQLStore) CreateMerchantDocument(ctx context.Context, input domain.KnowledgeDocumentInput) (domain.KnowledgeDocument, error) {
	chunks := rag.SplitDocument(input.Title, input.Content, input.DocType)
	if len(chunks) == 0 {
		chunks = []rag.ChunkDraft{{Title: input.Title, Content: input.Content, Snippet: rag.Snippet(input.Content, rag.DefaultSnippetRunes), SortOrder: 10}}
	}
	document := domain.KnowledgeDocument{
		DocumentID: nextID("doc"),
		MerchantID: input.MerchantID,
		Title:      input.Title,
		DocType:    input.DocType,
		Status:     "indexed",
		ChunkCount: len(chunks),
		CreatedAt:  time.Now(),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_documents (document_id, merchant_id, title, doc_type, content, status, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, document.DocumentID, document.MerchantID, document.Title, document.DocType, input.Content, document.Status, document.ChunkCount, document.CreatedAt, document.CreatedAt)
	if err != nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("insert knowledge document: %w", err)
	}
	for _, chunk := range chunks {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO knowledge_chunks (chunk_id, title, snippet, source, sort_order)
			VALUES (?, ?, ?, ?, ?)
		`, nextID("ck"), chunk.Title, chunk.Snippet, document.DocumentID, chunk.SortOrder)
		if err != nil {
			return domain.KnowledgeDocument{}, fmt.Errorf("insert knowledge chunk: %w", err)
		}
	}
	return document, nil
}

func (s *MySQLStore) CreateStoredFile(ctx context.Context, input domain.StoredFileInput) (domain.StoredFile, error) {
	if input.FileID == "" {
		input.FileID = nextID("file")
	}
	if input.StorageProvider == "" {
		input.StorageProvider = "minio"
	}
	now := time.Now()
	file := domain.StoredFile{
		FileID:          input.FileID,
		AccountID:       input.AccountID,
		ObjectKey:       input.ObjectKey,
		URL:             input.URL,
		MimeType:        input.MimeType,
		SizeBytes:       input.SizeBytes,
		ContentHash:     input.ContentHash,
		StorageProvider: input.StorageProvider,
		SourceURL:       input.SourceURL,
		CreatedAt:       now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO stored_files (file_id, account_id, object_key, url, mime_type, size_bytes, content_hash, storage_provider, source_url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, file.FileID, file.AccountID, file.ObjectKey, file.URL, file.MimeType, file.SizeBytes, file.ContentHash, file.StorageProvider, file.SourceURL, file.CreatedAt)
	if err != nil {
		return domain.StoredFile{}, fmt.Errorf("insert stored file: %w", err)
	}
	return file, nil
}

func (s *MySQLStore) GetStoredFile(ctx context.Context, fileID string) (domain.StoredFile, bool) {
	var file domain.StoredFile
	err := s.db.QueryRowContext(ctx, `
		SELECT file_id, account_id, object_key, url, mime_type, size_bytes, content_hash, storage_provider, source_url, created_at
		FROM stored_files
		WHERE file_id = ?
	`, fileID).Scan(
		&file.FileID,
		&file.AccountID,
		&file.ObjectKey,
		&file.URL,
		&file.MimeType,
		&file.SizeBytes,
		&file.ContentHash,
		&file.StorageProvider,
		&file.SourceURL,
		&file.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StoredFile{}, false
	}
	return file, err == nil
}

func (s *MySQLStore) getRun(ctx context.Context, runID string) (domain.AgentRun, bool) {
	var run domain.AgentRun
	var status string
	var blocksJSON, followupsJSON, segmentsJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, COALESCE(content, ''), COALESCE(CAST(blocks_json AS CHAR), ''), COALESCE(CAST(followups_json AS CHAR), ''), COALESCE(CAST(segments_json AS CHAR), ''), created_at, updated_at
		FROM agent_runs
		WHERE run_id = ?
	`, runID).Scan(&run.RunID, &run.SessionID, &run.MessageID, &run.AccountID, &status, &run.TraceID, &run.Content, &blocksJSON, &followupsJSON, &segmentsJSON, &run.CreatedAt, &run.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AgentRun{}, false
	}
	run.Status = domain.RunStatus(status)
	decodeRunJSON(&run, blocksJSON, followupsJSON, segmentsJSON)
	return run, err == nil
}

func scanAgentRun(rows *sql.Rows) (domain.AgentRun, error) {
	var item domain.AgentRun
	var status string
	var blocksJSON, followupsJSON, segmentsJSON string
	err := rows.Scan(
		&item.RunID,
		&item.SessionID,
		&item.MessageID,
		&item.AccountID,
		&status,
		&item.TraceID,
		&item.Content,
		&blocksJSON,
		&followupsJSON,
		&segmentsJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	item.Status = domain.RunStatus(status)
	decodeRunJSON(&item, blocksJSON, followupsJSON, segmentsJSON)
	return item, err
}

func decodeRunJSON(run *domain.AgentRun, blocksJSON string, followupsJSON string, segmentsJSON string) {
	if strings.TrimSpace(blocksJSON) != "" {
		_ = json.Unmarshal([]byte(blocksJSON), &run.Blocks)
	}
	if strings.TrimSpace(followupsJSON) != "" {
		_ = json.Unmarshal([]byte(followupsJSON), &run.Followups)
	}
	if strings.TrimSpace(segmentsJSON) != "" {
		_ = json.Unmarshal([]byte(segmentsJSON), &run.Segments)
	}
}

func scanAgentTraceEvents(rows *sql.Rows) []domain.AgentTraceEvent {
	items := make([]domain.AgentTraceEvent, 0)
	for rows.Next() {
		var item domain.AgentTraceEvent
		if err := rows.Scan(
			&item.TraceEventID,
			&item.RunID,
			&item.TraceID,
			&item.AccountID,
			&item.Stage,
			&item.EventType,
			&item.Model,
			&item.Status,
			&item.DurationMS,
			&item.Error,
			&item.MetadataJSON,
			&item.CreatedAt,
		); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func scanAgentPrompts(rows *sql.Rows) []domain.AgentPrompt {
	items := make([]domain.AgentPrompt, 0)
	for rows.Next() {
		var item domain.AgentPrompt
		var publishedAt sql.NullTime
		if err := rows.Scan(
			&item.PromptID,
			&item.PromptKey,
			&item.Title,
			&item.Content,
			&item.Status,
			&item.Version,
			&item.Description,
			&item.CreatedBy,
			&publishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil
		}
		if publishedAt.Valid {
			item.PublishedAt = publishedAt.Time
		}
		items = append(items, item)
	}
	return items
}

func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func pageOffset(page int, pageSize int) int {
	return (page - 1) * pageSize
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func paginateSlice[T any](items []T, page int, pageSize int) []T {
	page, pageSize = normalizePage(page, pageSize)
	start := pageOffset(page, pageSize)
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func (s *MySQLStore) countRows(ctx context.Context, table string) int {
	total := 0
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&total); err != nil {
		return 0
	}
	return total
}

func (s *MySQLStore) queryProductCards(ctx context.Context, query string, args ...any) []domain.ProductCard {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.ProductCard, 0)
	for rows.Next() {
		item, err := scanProductCard(rows)
		if err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) productCardsByIDs(ctx context.Context, productIDs []string) []domain.ProductCard {
	productIDs = uniqueTerms(productIDs, 100)
	if len(productIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(productIDs)), ",")
	args := make([]any, 0, len(productIDs))
	for _, id := range productIDs {
		args = append(args, id)
	}
	items := s.queryProductCards(ctx, productCardSelect()+` WHERE p.product_id IN (`+placeholders+`)`, args...)
	byID := make(map[string]domain.ProductCard, len(items))
	for _, item := range items {
		byID[item.ProductID] = item
	}
	ordered := make([]domain.ProductCard, 0, len(items))
	for _, id := range productIDs {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func (s *MySQLStore) productCardMapByIDs(ctx context.Context, productIDs []string) map[string]domain.ProductCard {
	cards := s.productCardsByIDs(ctx, productIDs)
	byID := make(map[string]domain.ProductCard, len(cards))
	for _, card := range cards {
		byID[card.ProductID] = card
	}
	return byID
}

func (s *MySQLStore) knowledgeChunksByIDs(ctx context.Context, chunkIDs []string) []domain.Citation {
	chunkIDs = uniqueTerms(chunkIDs, 100)
	if len(chunkIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(chunkIDs)), ",")
	args := make([]any, 0, len(chunkIDs))
	for _, id := range chunkIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT chunk_id, title, snippet, source
		FROM knowledge_chunks
		WHERE chunk_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	items := make([]domain.Citation, 0)
	for rows.Next() {
		var item domain.Citation
		if err := rows.Scan(&item.ChunkID, &item.Title, &item.Snippet, &item.Source); err != nil {
			return nil
		}
		items = append(items, item)
	}
	byID := make(map[string]domain.Citation, len(items))
	for _, item := range items {
		byID[item.ChunkID] = item
	}
	ordered := make([]domain.Citation, 0, len(items))
	for _, id := range chunkIDs {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func (s *MySQLStore) productVectorRows(ctx context.Context) []map[string]any {
	rows, err := s.db.QueryContext(ctx, productDetailSelect()+` WHERE p.status = 'active' ORDER BY p.product_id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		product, err := scanProductDetail(rows)
		if err != nil {
			return nil
		}
		out = append(out, map[string]any{
			"product_id":    product.ProductID,
			"merchant_id":   product.MerchantID,
			"category_id":   product.CategoryID,
			"brand":         product.Brand,
			"status":        "active",
			"stock_status":  product.StockStatus,
			"price":         parseFloat(product.Price),
			"updated_at_ts": time.Now().Unix(),
			"search_text":   productSearchText(product),
		})
	}
	return out
}

func (s *MySQLStore) productImageVectorRows(ctx context.Context) []map[string]any {
	rows, err := s.db.QueryContext(ctx, productDetailSelect()+` WHERE p.status = 'active' ORDER BY p.product_id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		product, err := scanProductDetail(rows)
		if err != nil {
			return nil
		}
		sources := product.ImageURLs
		if len(sources) == 0 && product.ImageURL != "" {
			sources = []string{product.ImageURL}
		}
		for index, source := range sources {
			vector, err := imagevector.FromSource(ctx, source)
			if err != nil || len(vector) == 0 {
				continue
			}
			imageType := "detail"
			if index == 0 {
				imageType = "main"
			}
			out = append(out, map[string]any{
				"image_vector_id": fmt.Sprintf("img_%s_%d", product.ProductID, index),
				"product_id":      product.ProductID,
				"merchant_id":     product.MerchantID,
				"category_id":     product.CategoryID,
				"brand":           product.Brand,
				"image_url":       source,
				"image_type":      imageType,
				"quality_score":   0.8,
				"updated_at_ts":   time.Now().Unix(),
				"embedding":       vector,
			})
		}
	}
	return out
}

func (s *MySQLStore) knowledgeVectorRows(ctx context.Context) []map[string]any {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chunk_id, title, snippet, source, sort_order
		FROM knowledge_chunks
		ORDER BY sort_order, chunk_id
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var chunkID, title, snippet, source string
		var sortOrder int
		if err := rows.Scan(&chunkID, &title, &snippet, &source, &sortOrder); err != nil {
			return nil
		}
		row := map[string]any{
			"chunk_id":      chunkID,
			"source":        source,
			"source_type":   sourceTypeForKnowledge(source),
			"doc_type":      docTypeForChunk(chunkID),
			"quality_score": 0.8,
			"created_at_ts": time.Now().Unix(),
			"search_text":   strings.Join([]string{title, snippet}, "\n"),
		}
		if strings.HasPrefix(source, "p_") {
			if product, ok := s.GetProduct(ctx, source); ok {
				row["product_id"] = product.ProductID
				row["merchant_id"] = product.MerchantID
				row["category_id"] = product.CategoryID
				row["brand"] = product.Brand
			}
		}
		out = append(out, row)
	}
	return out
}

func (s *MySQLStore) firstSKU(ctx context.Context, productID string) string {
	var skuID string
	_ = s.db.QueryRowContext(ctx, `
		SELECT sku_id
		FROM product_skus
		WHERE product_id = ?
		ORDER BY sku_id
		LIMIT 1
	`, productID).Scan(&skuID)
	return skuID
}

func (s *MySQLStore) loadCart(ctx context.Context, accountID string) domain.Cart {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.cart_item_id, c.product_id, c.sku_id, p.name, p.image_url,
			COALESCE(ps.price, p.price), c.quantity, c.selected,
			COALESCE(ps.stock_status, p.stock_status), p.merchant_id, m.name
		FROM cart_items c
		JOIN products p ON p.product_id = c.product_id
		JOIN merchants m ON m.merchant_id = p.merchant_id
		LEFT JOIN product_skus ps ON ps.sku_id = c.sku_id
		WHERE c.account_id = ?
		ORDER BY c.created_at, c.cart_item_id
	`, accountID)
	if err != nil {
		return domain.Cart{}
	}
	defer rows.Close()

	items := make([]domain.CartItem, 0)
	for rows.Next() {
		var item domain.CartItem
		if err := rows.Scan(
			&item.CartItemID,
			&item.ProductID,
			&item.SkuID,
			&item.Name,
			&item.ImageURL,
			&item.Price,
			&item.Quantity,
			&item.Selected,
			&item.StockStatus,
			&item.MerchantID,
			&item.MerchantName,
		); err != nil {
			return domain.Cart{}
		}
		items = append(items, item)
	}
	return buildCart(items)
}

func (s *MySQLStore) listOrders(ctx context.Context, where string, args ...any) []domain.Order {
	query := `
		SELECT
			o.order_id, o.order_no, o.account_id, o.merchant_id, m.name, o.status,
			o.total_amount, o.discount_amount, o.pay_amount, o.payment_deadline_at,
			o.paid_at, o.closed_at, o.completed_at, o.cancel_reason, o.created_at, o.updated_at
		FROM orders o
		JOIN merchants m ON m.merchant_id = o.merchant_id
	`
	if where != "" {
		query += " " + where
	}
	query += " ORDER BY o.created_at DESC, o.order_id DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var order domain.Order
		var deadline sql.NullTime
		var paidAt sql.NullTime
		var closedAt sql.NullTime
		var completedAt sql.NullTime
		if err := rows.Scan(
			&order.OrderID,
			&order.OrderNo,
			&order.AccountID,
			&order.MerchantID,
			&order.MerchantName,
			&order.Status,
			&order.TotalAmount,
			&order.DiscountAmount,
			&order.PayAmount,
			&deadline,
			&paidAt,
			&closedAt,
			&completedAt,
			&order.CancelReason,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil
		}
		if deadline.Valid {
			order.PaymentDeadlineAt = deadline.Time
		}
		if paidAt.Valid {
			order.PaidAt = paidAt.Time
		}
		if closedAt.Valid {
			order.ClosedAt = closedAt.Time
		}
		if completedAt.Valid {
			order.CompletedAt = completedAt.Time
		}
		order.Items = s.listOrderItems(ctx, order.OrderID)
		orders = append(orders, order)
	}
	return orders
}

func (s *MySQLStore) getLatestPayment(ctx context.Context, accountID string, orderID string) (domain.Payment, bool) {
	var payment domain.Payment
	var paidAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT payment_id, order_id, account_id, amount, status, method, transaction_no, expires_at, paid_at, created_at, updated_at
		FROM payments
		WHERE account_id = ? AND order_id = ?
		ORDER BY created_at DESC, payment_id DESC
		LIMIT 1
	`, accountID, orderID).Scan(
		&payment.PaymentID,
		&payment.OrderID,
		&payment.AccountID,
		&payment.Amount,
		&payment.Status,
		&payment.Method,
		&payment.TransactionNo,
		&payment.ExpiresAt,
		&paidAt,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	)
	if err != nil {
		return domain.Payment{}, false
	}
	if paidAt.Valid {
		payment.PaidAt = paidAt.Time
	}
	return payment, true
}

func (s *MySQLStore) listOrderItems(ctx context.Context, orderID string) []domain.OrderItem {
	rows, err := s.db.QueryContext(ctx, `
		SELECT order_item_id, product_id, sku_id, name, image_url, price, quantity, merchant_id, merchant_name
		FROM order_items
		WHERE order_id = ?
		ORDER BY order_item_id
	`, orderID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.OrderItemID, &item.ProductID, &item.SkuID, &item.Name, &item.ImageURL, &item.Price, &item.Quantity, &item.MerchantID, &item.MerchantName); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
}

type productCardScanner interface {
	Scan(dest ...any) error
}

func parseAmount(input string) (float64, error) {
	var value float64
	_, err := fmt.Sscanf(input, "%f", &value)
	return value, err
}

func parseFloat(input string) float64 {
	value, _ := parseAmount(input)
	return value
}

func productSearchText(product domain.ProductDetail) string {
	attributes := make([]string, 0, len(product.Attributes))
	for _, item := range product.Attributes {
		attributes = append(attributes, item.Key+"："+item.Value)
	}
	return strings.Join([]string{
		product.Name,
		product.Brand,
		product.CategoryID,
		strings.Join(product.Tags, " "),
		strings.Join(product.SellingPoints, " "),
		strings.Join(attributes, " "),
		strings.Join(product.SuitableFor, " "),
		product.RecommendReason,
		product.Description,
	}, "\n")
}

func sourceTypeForKnowledge(source string) string {
	if strings.HasPrefix(source, "p_") {
		return "product"
	}
	if strings.HasPrefix(source, "doc_") {
		return "merchant_doc"
	}
	return "platform_doc"
}

func docTypeForChunk(chunkID string) string {
	switch {
	case strings.Contains(chunkID, "_faq_"):
		return "faq"
	case strings.Contains(chunkID, "_review_"):
		return "review"
	case strings.Contains(chunkID, "_marketing"):
		return "product_desc"
	default:
		return "document"
	}
}

func nextOrderNo(now time.Time) string {
	return "NO" + now.Format("20060102150405") + strings.TrimPrefix(nextID(""), "_")
}

func deriveSessionTitle(content string) string {
	content = strings.TrimSpace(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return "AI 导购"
	}
	cleaner := strings.NewReplacer("，", "", "。", "", "？", "", "！", "", ",", "", ".", "", "?", "", "!", "", "我想", "", "帮我", "", "推荐", "")
	content = strings.TrimSpace(cleaner.Replace(content))
	if content == "" {
		return "导购咨询"
	}
	return truncateRunes(content, 8)
}

func deriveSessionSummary(content string) string {
	content = strings.TrimSpace(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return ""
	}
	return truncateRunes("用户咨询："+content, 120)
}

func truncateRunes(input string, limit int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func parseAPITime(input string, fallback time.Time) time.Time {
	input = strings.TrimSpace(input)
	if input == "" {
		return fallback
	}
	if value, err := time.Parse(time.RFC3339, input); err == nil {
		return value
	}
	if value, err := time.ParseInLocation("2006-01-02 15:04:05", input, time.Local); err == nil {
		return value
	}
	if value, err := time.ParseInLocation("2006-01-02", input, time.Local); err == nil {
		return value
	}
	return fallback
}

func discountAmountFor(base float64, discountType string, threshold string, fixedAmount string, rate string) float64 {
	thresholdValue, _ := parseAmount(threshold)
	if thresholdValue > 0 && base < thresholdValue {
		return 0
	}
	switch discountType {
	case "percentage":
		rateValue, _ := parseAmount(rate)
		if rateValue <= 0 || rateValue >= 1 {
			return 0
		}
		return base * rateValue
	default:
		amount, _ := parseAmount(fixedAmount)
		if amount < 0 {
			return 0
		}
		if amount > base {
			return base
		}
		return amount
	}
}

func scanProductCard(scanner productCardScanner) (domain.ProductCard, error) {
	var item domain.ProductCard
	var tagsJSON string
	var sellingPointsJSON string
	var riskNotesJSON string
	err := scanner.Scan(
		&item.ProductID,
		&item.SkuID,
		&item.MerchantID,
		&item.MerchantName,
		&item.Name,
		&item.Brand,
		&item.CategoryID,
		&item.ImageURL,
		&item.Price,
		&item.MarketPrice,
		&item.StockStatus,
		&tagsJSON,
		&sellingPointsJSON,
		&item.RecommendReason,
		&riskNotesJSON,
	)
	if err != nil {
		return domain.ProductCard{}, err
	}
	decodeJSON(tagsJSON, &item.Tags)
	decodeJSON(sellingPointsJSON, &item.SellingPoints)
	decodeJSON(riskNotesJSON, &item.RiskNotes)
	return item, nil
}

func scanProductDetail(scanner productCardScanner) (domain.ProductDetail, error) {
	var product domain.ProductDetail
	var tagsJSON string
	var sellingPointsJSON string
	var riskNotesJSON string
	var imageURLsJSON string
	var attributesJSON string
	var suitableForJSON string
	var notSuitableForJSON string
	err := scanner.Scan(
		&product.ProductID,
		&product.SkuID,
		&product.MerchantID,
		&product.MerchantName,
		&product.Name,
		&product.Brand,
		&product.CategoryID,
		&product.ImageURL,
		&product.Price,
		&product.MarketPrice,
		&product.StockStatus,
		&tagsJSON,
		&sellingPointsJSON,
		&product.RecommendReason,
		&riskNotesJSON,
		&imageURLsJSON,
		&product.StockQuantity,
		&attributesJSON,
		&suitableForJSON,
		&notSuitableForJSON,
		&product.Description,
	)
	if err != nil {
		return domain.ProductDetail{}, err
	}
	decodeJSON(tagsJSON, &product.Tags)
	decodeJSON(sellingPointsJSON, &product.SellingPoints)
	decodeJSON(riskNotesJSON, &product.RiskNotes)
	decodeJSON(imageURLsJSON, &product.ImageURLs)
	decodeJSON(attributesJSON, &product.Attributes)
	decodeJSON(suitableForJSON, &product.SuitableFor)
	decodeJSON(notSuitableForJSON, &product.NotSuitableFor)
	return product, nil
}

func scanSKU(scanner productCardScanner) (domain.ProductSKU, error) {
	var item domain.ProductSKU
	var specsJSON string
	err := scanner.Scan(&item.SkuID, &item.ProductID, &item.SkuName, &item.Price, &item.StockQuantity, &item.StockStatus, &specsJSON)
	if err != nil {
		return domain.ProductSKU{}, err
	}
	decodeJSON(specsJSON, &item.Specs)
	return item, nil
}

func productCardSelect() string {
	return `
		SELECT p.product_id, COALESCE(ps.sku_id, ''), p.merchant_id, m.name,
			p.name, p.brand, p.category_id, p.image_url, p.price, p.market_price,
			p.stock_status, p.tags_json, p.selling_points_json, p.recommend_reason,
			p.risk_notes_json
		FROM products p
		JOIN merchants m ON m.merchant_id = p.merchant_id
		LEFT JOIN categories c ON c.category_id = p.category_id
		LEFT JOIN product_skus ps ON ps.product_id = p.product_id AND ps.is_default = TRUE
	`
}

func productDetailSelect() string {
	return `
		SELECT p.product_id, COALESCE(ps.sku_id, ''), p.merchant_id, m.name,
			p.name, p.brand, p.category_id, p.image_url, p.price, p.market_price,
			p.stock_status, p.tags_json, p.selling_points_json, p.recommend_reason,
			p.risk_notes_json, p.image_urls_json, p.stock_quantity,
			p.attributes_json, p.suitable_for_json, p.not_suitable_for_json, p.description
		FROM products p
		JOIN merchants m ON m.merchant_id = p.merchant_id
		LEFT JOIN categories c ON c.category_id = p.category_id
		LEFT JOIN product_skus ps ON ps.product_id = p.product_id AND ps.is_default = TRUE
	`
}

func uniqueTerms(input []string, limit int) []string {
	seen := make(map[string]bool)
	terms := make([]string, 0, limit)
	for _, term := range input {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
		if len(terms) == limit {
			break
		}
	}
	return terms
}

func rankProductSearchResults(query string, items []domain.ProductCard) []domain.ProductCard {
	if len(items) == 0 {
		return items
	}
	constraint := parseProductSearchConstraint(query, items)
	ranked := make([]productSearchCandidate, 0, len(items))
	for index, item := range items {
		score, ok := scoreProductSearchCandidate(item, constraint)
		if !ok {
			continue
		}
		ranked = append(ranked, productSearchCandidate{item: item, score: score, index: index})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].index < ranked[j].index
		}
		return ranked[i].score > ranked[j].score
	})
	out := make([]domain.ProductCard, 0, len(ranked))
	for _, candidate := range ranked {
		out = append(out, candidate.item)
	}
	return out
}

type productSearchCandidate struct {
	item  domain.ProductCard
	score int
	index int
}

type productSearchConstraint struct {
	terms      []string
	brands     []string
	categories []string
}

func parseProductSearchConstraint(query string, items []domain.ProductCard) productSearchConstraint {
	lower := strings.ToLower(query)
	constraint := productSearchConstraint{
		terms: uniqueTerms(append([]string{query}, rag.QueryTerms(query)...), 12),
	}
	brandTerms := make([]string, 0)
	categoryTerms := make([]string, 0)
	for _, item := range items {
		brandTerms = append(brandTerms, productSearchBrandTerms(item.Brand)...)
		categoryTerms = append(categoryTerms, productSearchCategoryTerms(item)...)
	}
	for _, term := range uniqueTerms(brandTerms, 80) {
		if productSearchQueryMatchesFacet(lower, term) {
			constraint.brands = append(constraint.brands, term)
		}
	}
	for _, term := range uniqueTerms(append(categoryTerms, productSearchCategoryAliases(lower)...), 120) {
		if containsAny(lower, term) {
			constraint.categories = append(constraint.categories, term)
		}
	}
	return constraint
}

func scoreProductSearchCandidate(item domain.ProductCard, constraint productSearchConstraint) (int, bool) {
	text := strings.ToLower(strings.Join([]string{
		item.Name,
		item.Brand,
		item.CategoryID,
		strings.Join(item.Tags, " "),
		strings.Join(item.SellingPoints, " "),
		item.RecommendReason,
	}, " "))
	facetText := strings.ToLower(strings.Join(append([]string{
		item.Brand,
		item.CategoryID,
	}, productSearchCategoryTerms(item)...), " "))

	if len(constraint.brands) > 0 && !containsAny(text, constraint.brands...) {
		return 0, false
	}
	if len(constraint.categories) > 0 && !containsAny(facetText, constraint.categories...) {
		return 0, false
	}

	score := 0
	for _, term := range constraint.terms {
		if strings.Contains(text, strings.ToLower(term)) {
			score++
		}
	}
	firstTerm := ""
	if len(constraint.terms) > 0 {
		firstTerm = strings.TrimSpace(constraint.terms[0])
	}
	if firstTerm != "" && strings.Contains(strings.ToLower(item.Name), strings.ToLower(firstTerm)) {
		score += 4
	}
	if len(constraint.brands) > 0 && containsAny(strings.ToLower(item.Brand), constraint.brands...) {
		score += 10
	}
	if len(constraint.categories) > 0 && containsAny(facetText, constraint.categories...) {
		score += 8
	}
	return score, true
}

func productSearchBrandTerms(brand string) []string {
	terms := rag.QueryTerms(brand)
	brand = strings.TrimSpace(strings.ToLower(brand))
	if brand != "" {
		terms = append(terms, brand)
	}
	return terms
}

func productSearchCategoryTerms(item domain.ProductCard) []string {
	terms := make([]string, 0)
	brandTerms := productSearchBrandTerms(item.Brand)
	add := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" || containsAny(strings.ToLower(term), brandTerms...) || isBroadProductCategoryTerm(term) {
			return
		}
		terms = append(terms, term)
	}
	for _, tag := range item.Tags {
		add(tag)
	}
	for _, point := range item.SellingPoints {
		if strings.Contains(point, "：") || strings.Contains(point, ":") {
			continue
		}
		add(point)
	}
	add(strings.TrimPrefix(item.CategoryID, "c_dataset_"))
	return terms
}

func isBroadProductCategoryTerm(term string) bool {
	switch strings.TrimSpace(strings.ToLower(term)) {
	case "数码电子", "美妆护肤", "服饰运动", "食品饮料", "digital", "beauty", "clothes", "food":
		return true
	default:
		return false
	}
}

func productSearchCategoryAliases(query string) []string {
	aliases := map[string][]string{
		"电脑":        {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"笔记本":       {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"matebook":  {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"macbook":   {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"thinkbook": {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"thinkpad":  {"笔记本电脑", "笔记本", "轻薄本", "laptop"},
		"手机":        {"智能手机", "手机", "iphone"},
		"平板":        {"平板电脑", "平板", "ipad", "matepad", "pad"},
		"耳机":        {"真无线耳机", "耳机", "freebuds"},
	}
	out := make([]string, 0)
	for trigger, terms := range aliases {
		if strings.Contains(query, trigger) {
			out = append(out, terms...)
		}
	}
	return out
}

func productSearchQueryMatchesFacet(query string, facet string) bool {
	facet = strings.TrimSpace(strings.ToLower(facet))
	if facet == "" {
		return false
	}
	if strings.Contains(query, facet) {
		return true
	}
	for _, term := range rag.QueryTerms(facet) {
		if strings.Contains(query, term) {
			return true
		}
	}
	return false
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func hitIDs(hits []rag.SearchHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		if strings.TrimSpace(hit.ID) != "" {
			ids = append(ids, hit.ID)
		}
	}
	return ids
}

func mergeProductCards(primary []domain.ProductCard, extra []domain.ProductCard) []domain.ProductCard {
	seen := make(map[string]bool, len(primary)+len(extra))
	out := make([]domain.ProductCard, 0, len(primary)+len(extra))
	for _, item := range append(primary, extra...) {
		if item.ProductID == "" || seen[item.ProductID] {
			continue
		}
		seen[item.ProductID] = true
		out = append(out, item)
	}
	return out
}

func mergeCandidates(primary []rag.Candidate, extra []rag.Candidate) []rag.Candidate {
	seen := make(map[string]bool, len(primary)+len(extra))
	out := make([]rag.Candidate, 0, len(primary)+len(extra))
	for _, item := range append(primary, extra...) {
		if item.ChunkID == "" || seen[item.ChunkID] {
			continue
		}
		seen[item.ChunkID] = true
		out = append(out, item)
	}
	return out
}

func citationsToCandidates(items []domain.Citation) []rag.Candidate {
	out := make([]rag.Candidate, 0, len(items))
	for _, item := range items {
		out = append(out, rag.Candidate{
			ChunkID: item.ChunkID,
			Title:   item.Title,
			Snippet: item.Snippet,
			Source:  item.Source,
		})
	}
	return out
}

func decodeJSON(input string, output any) {
	if input == "" {
		return
	}
	_ = json.Unmarshal([]byte(input), output)
}

func (s *MySQLStore) columnExists(ctx context.Context, table string, column string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?
	`, table, column).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check column exists: %w", err)
	}
	return count > 0, nil
}

func (s *MySQLStore) indexExists(ctx context.Context, table string, index string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?
	`, table, index).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check index exists: %w", err)
	}
	return count > 0, nil
}

func mustJSON(input any) string {
	payload, err := json.Marshal(input)
	if err != nil {
		return "null"
	}
	return string(payload)
}

func normalizeProductInput(input domain.ProductUpsertInput) domain.ProductUpsertInput {
	if input.Brand == "" {
		input.Brand = "未设置"
	}
	if input.CategoryID == "" {
		input.CategoryID = "c_phone"
	}
	if input.ImageURL == "" {
		input.ImageURL = "https://images.unsplash.com/photo-1516321318423-f06f85e504b3?auto=format&fit=crop&w=640&q=80"
	}
	if input.Price == "" {
		input.Price = "0.00"
	}
	if input.MarketPrice == "" {
		input.MarketPrice = input.Price
	}
	if input.StockStatus == "" {
		input.StockStatus = "in_stock"
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	if input.SellingPoints == nil {
		input.SellingPoints = []string{}
	}
	if input.RiskNotes == nil {
		input.RiskNotes = []string{}
	}
	return input
}

func buildCategoryTree(categories []domain.Category) []domain.Category {
	byID := make(map[string]*domain.Category, len(categories))
	rootIDs := make([]string, 0)
	for i := range categories {
		byID[categories[i].CategoryID] = &categories[i]
	}
	for i := range categories {
		item := &categories[i]
		if item.ParentID == "" {
			rootIDs = append(rootIDs, item.CategoryID)
			continue
		}
		parent, ok := byID[item.ParentID]
		if !ok {
			rootIDs = append(rootIDs, item.CategoryID)
			continue
		}
		parent.Children = append(parent.Children, *item)
	}
	roots := make([]domain.Category, 0, len(rootIDs))
	for _, id := range rootIDs {
		roots = append(roots, *byID[id])
	}
	return roots
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func readMigrationFile() ([]byte, error) {
	paths := []string{
		filepath.Join("migrations", "001_mysql_schema.sql"),
		filepath.Join("backend", "migrations", "001_mysql_schema.sql"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			return content, nil
		}
	}
	return nil, errors.New("migration file not found")
}

func splitSQLStatements(input string) []string {
	lines := make([]string, 0)
	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Split(strings.Join(lines, "\n"), ";")
}
