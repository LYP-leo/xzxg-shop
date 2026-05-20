package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
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

func (s *MySQLStore) ensureAccessSchema(ctx context.Context) error {
	columns := []struct {
		table string
		name  string
		ddl   string
	}{
		{"cart_items", "account_id", "ALTER TABLE cart_items ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER cart_item_id"},
		{"chat_sessions", "account_id", "ALTER TABLE chat_sessions ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER session_id"},
		{"user_messages", "account_id", "ALTER TABLE user_messages ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER session_id"},
		{"agent_runs", "account_id", "ALTER TABLE agent_runs ADD COLUMN account_id VARCHAR(64) NOT NULL DEFAULT '' AFTER message_id"},
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

	updates := []string{
		"UPDATE cart_items SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE chat_sessions SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE user_messages SET account_id = 'acct_user_001' WHERE account_id = ''",
		"UPDATE agent_runs SET account_id = 'acct_user_001' WHERE account_id = ''",
	}
	for _, stmt := range updates {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("backfill access column: %w", err)
		}
	}

	if exists, err := s.indexExists(ctx, "cart_items", "uk_cart_product_sku"); err != nil {
		return err
	} else if exists {
		if _, err := s.db.ExecContext(ctx, "ALTER TABLE cart_items DROP INDEX uk_cart_product_sku"); err != nil {
			return fmt.Errorf("drop legacy cart unique index: %w", err)
		}
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
		{"agent_runs", "idx_agent_runs_account_id", "CREATE INDEX idx_agent_runs_account_id ON agent_runs (account_id)"},
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

func (s *MySQLStore) GetAccountByUsername(ctx context.Context, username string) (domain.Account, string, bool) {
	var account domain.Account
	var role string
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT account_id, username, password_hash, display_name, role, merchant_id, created_at
		FROM accounts
		WHERE username = ? AND status = 'active'
	`, username).Scan(
		&account.AccountID,
		&account.Username,
		&passwordHash,
		&account.DisplayName,
		&role,
		&account.MerchantID,
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
		SELECT a.account_id, a.username, a.display_name, a.role, a.merchant_id, a.created_at
		FROM auth_tokens t
		JOIN accounts a ON a.account_id = t.account_id
		WHERE t.token = ? AND t.expires_at > ? AND a.status = 'active'
	`, token, time.Now()).Scan(
		&account.AccountID,
		&account.Username,
		&account.DisplayName,
		&role,
		&account.MerchantID,
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
		SELECT account_id, username, display_name, role, merchant_id, status, created_at
		FROM accounts
		WHERE account_id = ?
	`, accountID).Scan(&account.AccountID, &account.Username, &account.DisplayName, &role, &account.MerchantID, &account.Status, &account.CreatedAt)
	if err != nil {
		return domain.Account{}, false
	}
	account.Role = domain.AccountRole(role)
	return account, true
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
		SessionID: nextID("sess"),
		AccountID: accountID,
		Title:     title,
		CreatedAt: now,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chat_sessions (session_id, account_id, title, created_at)
		VALUES (?, ?, ?, ?)
	`, session.SessionID, session.AccountID, session.Title, session.CreatedAt)
	if err != nil {
		return domain.ChatSession{}, fmt.Errorf("insert chat session: %w", err)
	}
	return session, nil
}

func (s *MySQLStore) GetSession(ctx context.Context, accountID string, sessionID string) (domain.ChatSession, bool) {
	var session domain.ChatSession
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, account_id, title, created_at
		FROM chat_sessions
		WHERE session_id = ? AND account_id = ?
	`, sessionID, accountID).Scan(&session.SessionID, &session.AccountID, &session.Title, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChatSession{}, false
	}
	return session, err == nil
}

func (s *MySQLStore) ListUserSessions(ctx context.Context, accountID string) []domain.ChatSession {
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, account_id, title, created_at
		FROM chat_sessions
		WHERE account_id = ?
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	items := make([]domain.ChatSession, 0)
	for rows.Next() {
		var item domain.ChatSession
		if err := rows.Scan(&item.SessionID, &item.AccountID, &item.Title, &item.CreatedAt); err != nil {
			return nil
		}
		items = append(items, item)
	}
	return items
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

func (s *MySQLStore) CreateUserMessage(ctx context.Context, input domain.UserMessage) (domain.UserMessage, error) {
	input.MessageID = nextID("msg")
	input.CreatedAt = time.Now()
	attachments, err := json.Marshal(input.Attachments)
	if err != nil {
		return domain.UserMessage{}, fmt.Errorf("marshal attachments: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_messages (message_id, session_id, account_id, client_message_id, content, attachments_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, input.MessageID, input.SessionID, input.AccountID, input.ClientMessageID, input.Content, string(attachments), input.CreatedAt)
	if err != nil {
		return domain.UserMessage{}, fmt.Errorf("insert user message: %w", err)
	}
	return input, nil
}

func (s *MySQLStore) listRunsByMessage(ctx context.Context, accountID string, messageID string) []domain.AgentRun {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, created_at, updated_at
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
		var item domain.AgentRun
		var status string
		if err := rows.Scan(
			&item.RunID,
			&item.SessionID,
			&item.MessageID,
			&item.AccountID,
			&status,
			&item.TraceID,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil
		}
		item.Status = domain.RunStatus(status)
		items = append(items, item)
	}
	return items
}

func (s *MySQLStore) CreateRun(ctx context.Context, accountID string, sessionID string, messageID string) (domain.AgentRun, error) {
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
		return domain.AgentRun{}, fmt.Errorf("insert agent run: %w", err)
	}
	return run, nil
}

func (s *MySQLStore) UpdateRunStatus(ctx context.Context, accountID string, runID string, status domain.RunStatus) (domain.AgentRun, bool) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = ?, updated_at = ?
		WHERE run_id = ? AND account_id = ?
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

func (s *MySQLStore) SearchProducts(ctx context.Context, query string) []domain.ProductCard {
	items := s.ListProducts(ctx, query, "")
	if len(items) == 0 && query != "" {
		return s.ListProducts(ctx, "", "")
	}
	return items
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
		query += " AND p.category_id = ?"
		args = append(args, categoryID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query += ` AND (
			p.name LIKE ? OR p.brand LIKE ? OR p.tags_json LIKE ? OR p.selling_points_json LIKE ?
		)`
		args = append(args, like, like, like, like)
	}
	query += " ORDER BY p.sort_order, p.product_id"
	return s.queryProductCards(ctx, query, args...)
}

func (s *MySQLStore) ListAllProducts(ctx context.Context) []domain.ProductCard {
	return s.queryProductCards(ctx, productCardSelect()+" ORDER BY p.sort_order, p.product_id")
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
	if skuID == "" {
		skuID = s.firstSKU(ctx, productID)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE cart_items
		SET quantity = quantity + ?, updated_at = ?
		WHERE account_id = ? AND product_id = ? AND sku_id = ?
	`, quantity, time.Now(), accountID, productID, skuID)
	if err != nil {
		return domain.Cart{}, false
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Cart{}, false
	}
	if affected == 0 {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO cart_items (cart_item_id, account_id, product_id, sku_id, quantity, selected, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, nextID("cart"), accountID, productID, skuID, quantity, true, time.Now(), time.Now())
		if err != nil {
			return domain.Cart{}, false
		}
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
	cart := s.loadCart(ctx, accountID)
	selected := make([]domain.CartItem, 0)
	for _, item := range cart.Items {
		if item.Selected {
			selected = append(selected, item)
		}
	}
	if len(selected) == 0 {
		return nil, false
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false
	}
	defer tx.Rollback()

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
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO orders (order_id, account_id, merchant_id, status, total_amount, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, orderID, accountID, merchantID, "pending_ship", amount, createdAt, createdAt); err != nil {
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
			OrderID:      orderID,
			AccountID:    accountID,
			MerchantID:   merchantID,
			MerchantName: merchantName,
			Status:       "pending_ship",
			TotalAmount:  amount,
			Items:        orderItems,
			CreatedAt:    createdAt,
		})
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE account_id = ? AND selected = TRUE`, accountID); err != nil {
		return nil, false
	}
	if err := tx.Commit(); err != nil {
		return nil, false
	}
	return orders, true
}

func (s *MySQLStore) ListUserOrders(ctx context.Context, accountID string) []domain.Order {
	return s.listOrders(ctx, "WHERE o.account_id = ?", accountID)
}

func (s *MySQLStore) ListMerchantOrders(ctx context.Context, merchantID string) []domain.Order {
	return s.listOrders(ctx, "WHERE o.merchant_id = ?", merchantID)
}

func (s *MySQLStore) ListAllOrders(ctx context.Context) []domain.Order {
	return s.listOrders(ctx, "", nil)
}

func (s *MySQLStore) UpdateOrderStatus(ctx context.Context, merchantID string, orderID string, status string) (domain.Order, bool) {
	args := []any{status, time.Now(), orderID}
	query := `UPDATE orders SET status = ?, updated_at = ? WHERE order_id = ?`
	if merchantID != "" {
		query += ` AND merchant_id = ?`
		args = append(args, merchantID)
	}
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

func (s *MySQLStore) SearchKnowledge(ctx context.Context, query string) []domain.Citation {
	items := s.searchKnowledge(ctx, query)
	if len(items) == 0 && query != "" {
		return s.searchKnowledge(ctx, "")
	}
	return items
}

func (s *MySQLStore) searchKnowledge(ctx context.Context, query string) []domain.Citation {
	args := make([]any, 0, 2)
	sqlQuery := `
		SELECT chunk_id, title, snippet, source
		FROM knowledge_chunks
	`
	if query != "" {
		like := "%" + query + "%"
		sqlQuery += ` WHERE title LIKE ? OR snippet LIKE ?`
		args = append(args, like, like)
	}
	sqlQuery += ` ORDER BY sort_order, chunk_id LIMIT 5`
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
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
	return items
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

func (s *MySQLStore) CreateMerchantDocument(ctx context.Context, input domain.KnowledgeDocumentInput) (domain.KnowledgeDocument, error) {
	document := domain.KnowledgeDocument{
		DocumentID: nextID("doc"),
		MerchantID: input.MerchantID,
		Title:      input.Title,
		DocType:    input.DocType,
		Status:     "indexed",
		ChunkCount: 1,
		CreatedAt:  time.Now(),
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge_documents (document_id, merchant_id, title, doc_type, content, status, chunk_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, document.DocumentID, document.MerchantID, document.Title, document.DocType, input.Content, document.Status, document.ChunkCount, document.CreatedAt, document.CreatedAt)
	if err != nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("insert knowledge document: %w", err)
	}
	snippet := input.Content
	if len([]rune(snippet)) > 160 {
		snippet = string([]rune(snippet)[:160])
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO knowledge_chunks (chunk_id, title, snippet, source, sort_order)
		VALUES (?, ?, ?, ?, ?)
	`, nextID("ck"), document.Title, snippet, document.DocumentID, 100)
	if err != nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("insert knowledge chunk: %w", err)
	}
	return document, nil
}

func (s *MySQLStore) getRun(ctx context.Context, runID string) (domain.AgentRun, bool) {
	var run domain.AgentRun
	var status string
	err := s.db.QueryRowContext(ctx, `
		SELECT run_id, session_id, message_id, account_id, status, trace_id, created_at, updated_at
		FROM agent_runs
		WHERE run_id = ?
	`, runID).Scan(&run.RunID, &run.SessionID, &run.MessageID, &run.AccountID, &status, &run.TraceID, &run.CreatedAt, &run.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AgentRun{}, false
	}
	run.Status = domain.RunStatus(status)
	return run, err == nil
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

func (s *MySQLStore) listOrders(ctx context.Context, where string, arg any) []domain.Order {
	query := `
		SELECT o.order_id, o.account_id, o.merchant_id, m.name, o.status, o.total_amount, o.created_at
		FROM orders o
		JOIN merchants m ON m.merchant_id = o.merchant_id
	`
	args := make([]any, 0, 1)
	if where != "" {
		query += " " + where
		if arg != nil {
			args = append(args, arg)
		}
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
		if err := rows.Scan(&order.OrderID, &order.AccountID, &order.MerchantID, &order.MerchantName, &order.Status, &order.TotalAmount, &order.CreatedAt); err != nil {
			return nil
		}
		order.Items = s.listOrderItems(ctx, order.OrderID)
		orders = append(orders, order)
	}
	return orders
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
		LEFT JOIN product_skus ps ON ps.product_id = p.product_id AND ps.is_default = TRUE
	`
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
