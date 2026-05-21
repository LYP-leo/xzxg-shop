package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/store"
	_ "github.com/go-sql-driver/mysql"
)

type datasetProduct struct {
	ProductID    string         `json:"product_id"`
	Title        string         `json:"title"`
	Brand        string         `json:"brand"`
	Category     string         `json:"category"`
	SubCategory  string         `json:"sub_category"`
	BasePrice    float64        `json:"base_price"`
	ImagePath    string         `json:"image_path"`
	SKUs         []datasetSKU   `json:"skus"`
	RAGKnowledge datasetRAGData `json:"rag_knowledge"`
}

type datasetSKU struct {
	SKUID      string            `json:"sku_id"`
	Properties map[string]string `json:"properties"`
	Price      float64           `json:"price"`
}

type datasetRAGData struct {
	MarketingDescription string          `json:"marketing_description"`
	OfficialFAQ          []datasetFAQ    `json:"official_faq"`
	UserReviews          []datasetReview `json:"user_reviews"`
}

type datasetFAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type datasetReview struct {
	Nickname string `json:"nickname"`
	Rating   int    `json:"rating"`
	Content  string `json:"content"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	dsn := env("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/xzxg_shop?parseTime=true&loc=Local")
	datasetRoot := env("ECOMMERCE_DATASET_ROOT", filepath.Join("..", "quality", "data", "ecommerce_agent_dataset"))

	mysqlStore, err := store.OpenMySQL(ctx, dsn)
	if err != nil {
		logger.Error("mysql unavailable", "error", err)
		os.Exit(1)
	}
	defer mysqlStore.Close()
	if err := mysqlStore.Migrate(ctx); err != nil {
		logger.Error("mysql migration failed", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Error("open mysql failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	summary, err := importDataset(ctx, db, datasetRoot)
	if err != nil {
		logger.Error("import dataset failed", "error", err)
		os.Exit(1)
	}
	logger.Info("dataset imported", "products", summary.Products, "skus", summary.SKUs, "documents", summary.Documents, "chunks", summary.Chunks)
}

type importSummary struct {
	Products  int
	SKUs      int
	Documents int
	Chunks    int
}

func importDataset(ctx context.Context, db *sql.DB, root string) (importSummary, error) {
	files, err := datasetFiles(root)
	if err != nil {
		return importSummary{}, err
	}
	if len(files) == 0 {
		return importSummary{}, errors.New("no dataset product files found")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return importSummary{}, err
	}
	defer tx.Rollback()

	if err := upsertDatasetBase(ctx, tx); err != nil {
		return importSummary{}, err
	}
	if err := cleanupLegacyDatasetCategories(ctx, tx); err != nil {
		return importSummary{}, err
	}

	summary := importSummary{}
	for index, file := range files {
		product, err := readProduct(file)
		if err != nil {
			return importSummary{}, err
		}
		if err := upsertCategory(ctx, tx, product); err != nil {
			return importSummary{}, err
		}
		if err := upsertProduct(ctx, tx, product, index); err != nil {
			return importSummary{}, err
		}
		summary.Products++
		for skuIndex, sku := range product.SKUs {
			if err := upsertSKU(ctx, tx, product, sku, skuIndex == 0); err != nil {
				return importSummary{}, err
			}
			summary.SKUs++
		}
		documents, chunks := buildKnowledge(product)
		for _, document := range documents {
			if err := upsertDocument(ctx, tx, document); err != nil {
				return importSummary{}, err
			}
			summary.Documents++
		}
		for chunkIndex, chunk := range chunks {
			if err := upsertChunk(ctx, tx, chunk, index*100+chunkIndex); err != nil {
				return importSummary{}, err
			}
			summary.Chunks++
		}
	}
	if err := tx.Commit(); err != nil {
		return importSummary{}, err
	}
	return summary, nil
}

func datasetFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".json" || filepath.Base(filepath.Dir(path)) != "data" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func readProduct(path string) (datasetProduct, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return datasetProduct{}, err
	}
	var product datasetProduct
	if err := json.Unmarshal(content, &product); err != nil {
		return datasetProduct{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return product, nil
}

func upsertDatasetBase(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO merchants (merchant_id, name, logo_url, description, service_phone, status, created_at, updated_at)
		VALUES ('m_dataset_001', '真实商品数据集旗舰店', '/placeholder-merchant.svg', '由 quality/data/ecommerce_agent_dataset 导入的比赛演示商品。', '400-888-0000', 'active', ?, ?)
		ON DUPLICATE KEY UPDATE name = VALUES(name), description = VALUES(description), status = VALUES(status), updated_at = VALUES(updated_at)
	`, time.Now(), time.Now())
	return err
}

func cleanupLegacyDatasetCategories(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM categories
		WHERE category_id IN ('c_dataset_cat_4', 'c_dataset__', 'c_dataset___')
			OR (category_id LIKE 'c_dataset_%' AND category_id REGEXP '[^ -~]')
	`)
	return err
}

func upsertCategory(ctx context.Context, tx *sql.Tx, product datasetProduct) error {
	parentID := categoryID(product.Category)
	childID := categoryID(product.Category + "_" + product.SubCategory)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO categories (category_id, parent_id, name, sort_order, created_at, updated_at)
		VALUES (?, '', ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE name = VALUES(name), sort_order = VALUES(sort_order), updated_at = VALUES(updated_at)
	`, parentID, product.Category, categorySort(product.Category), time.Now(), time.Now()); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO categories (category_id, parent_id, name, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE parent_id = VALUES(parent_id), name = VALUES(name), updated_at = VALUES(updated_at)
	`, childID, parentID, product.SubCategory, categorySort(product.Category)+10, time.Now(), time.Now())
	return err
}

func upsertProduct(ctx context.Context, tx *sql.Tx, product datasetProduct, index int) error {
	imageURL := "/api/v1/assets/ecommerce_agent_dataset/" + strings.TrimPrefix(product.ImagePath, "/")
	sellingPoints := sellingPoints(product)
	description := product.RAGKnowledge.MarketingDescription
	_, err := tx.ExecContext(ctx, `
		INSERT INTO products (
			product_id, merchant_id, name, brand, category_id, image_url, image_urls_json,
			price, market_price, stock_quantity, stock_status, tags_json, selling_points_json,
			recommend_reason, risk_notes_json, attributes_json, suitable_for_json,
			not_suitable_for_json, description, status, sort_order, created_at, updated_at
		)
		VALUES (?, 'm_dataset_001', ?, ?, ?, ?, ?, ?, ?, 100, 'in_stock', ?, ?, ?, JSON_ARRAY(), ?, ?, JSON_ARRAY(), ?, 'active', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name), brand = VALUES(brand), category_id = VALUES(category_id),
			image_url = VALUES(image_url), image_urls_json = VALUES(image_urls_json),
			price = VALUES(price), market_price = VALUES(market_price), stock_quantity = VALUES(stock_quantity),
			stock_status = VALUES(stock_status), tags_json = VALUES(tags_json), selling_points_json = VALUES(selling_points_json),
			recommend_reason = VALUES(recommend_reason), risk_notes_json = JSON_ARRAY(),
			attributes_json = VALUES(attributes_json), suitable_for_json = VALUES(suitable_for_json),
			not_suitable_for_json = JSON_ARRAY(), description = VALUES(description),
			status = 'active', sort_order = VALUES(sort_order), updated_at = VALUES(updated_at)
	`, product.ProductID, product.Title, product.Brand, categoryID(product.Category+"_"+product.SubCategory), imageURL, mustJSON([]string{imageURL}),
		product.BasePrice, product.BasePrice, mustJSON([]string{product.Category, product.SubCategory, product.Brand}), mustJSON(sellingPoints),
		recommendReason(product), mustJSON(attributes(product)), mustJSON(suitableFor(product)), description, 1000+index, time.Now(), time.Now())
	return err
}

func upsertSKU(ctx context.Context, tx *sql.Tx, product datasetProduct, sku datasetSKU, isDefault bool) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO product_skus (sku_id, product_id, sku_name, price, stock_quantity, stock_status, specs_json, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, 100, 'in_stock', ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE sku_name = VALUES(sku_name), price = VALUES(price), stock_quantity = VALUES(stock_quantity),
			stock_status = VALUES(stock_status), specs_json = VALUES(specs_json), is_default = VALUES(is_default), updated_at = VALUES(updated_at)
	`, sku.SKUID, product.ProductID, skuName(product, sku), sku.Price, mustJSON(sku.Properties), isDefault, time.Now(), time.Now())
	return err
}

type documentRow struct {
	DocumentID string
	Title      string
	DocType    string
	Content    string
	ChunkCount int
}

type chunkRow struct {
	ChunkID string
	Title   string
	Snippet string
	Source  string
}

func buildKnowledge(product datasetProduct) ([]documentRow, []chunkRow) {
	documents := []documentRow{{
		DocumentID: product.ProductID + "_doc_marketing",
		Title:      product.Title + " 商品说明",
		DocType:    "product_detail",
		Content:    product.RAGKnowledge.MarketingDescription,
		ChunkCount: 1 + len(product.RAGKnowledge.OfficialFAQ),
	}}
	chunks := []chunkRow{{
		ChunkID: product.ProductID + "_ck_marketing",
		Title:   product.Title + " 商品说明",
		Snippet: product.RAGKnowledge.MarketingDescription,
		Source:  product.ProductID,
	}}
	for index, faq := range product.RAGKnowledge.OfficialFAQ {
		documents = append(documents, documentRow{
			DocumentID: fmt.Sprintf("%s_doc_faq_%02d", product.ProductID, index+1),
			Title:      faq.Question,
			DocType:    "faq",
			Content:    faq.Answer,
			ChunkCount: 1,
		})
		chunks = append(chunks, chunkRow{
			ChunkID: fmt.Sprintf("%s_ck_faq_%02d", product.ProductID, index+1),
			Title:   faq.Question,
			Snippet: faq.Answer,
			Source:  product.ProductID,
		})
	}
	for index, review := range product.RAGKnowledge.UserReviews {
		chunks = append(chunks, chunkRow{
			ChunkID: fmt.Sprintf("%s_ck_review_%02d", product.ProductID, index+1),
			Title:   product.Title + " 用户评价",
			Snippet: fmt.Sprintf("%s，评分 %d：%s", review.Nickname, review.Rating, review.Content),
			Source:  product.ProductID,
		})
	}
	return documents, chunks
}

func upsertDocument(ctx context.Context, tx *sql.Tx, row documentRow) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_documents (document_id, merchant_id, title, doc_type, content, status, chunk_count, created_at, updated_at)
		VALUES (?, 'm_dataset_001', ?, ?, ?, 'indexed', ?, ?, ?)
		ON DUPLICATE KEY UPDATE title = VALUES(title), doc_type = VALUES(doc_type), content = VALUES(content),
			status = 'indexed', chunk_count = VALUES(chunk_count), updated_at = VALUES(updated_at)
	`, row.DocumentID, row.Title, row.DocType, row.Content, row.ChunkCount, time.Now(), time.Now())
	return err
}

func upsertChunk(ctx context.Context, tx *sql.Tx, row chunkRow, sortOrder int) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO knowledge_chunks (chunk_id, title, snippet, source, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE title = VALUES(title), snippet = VALUES(snippet), source = VALUES(source),
			sort_order = VALUES(sort_order), updated_at = VALUES(updated_at)
	`, row.ChunkID, row.Title, row.Snippet, row.Source, sortOrder, time.Now(), time.Now())
	return err
}

func categoryID(name string) string {
	if value, ok := categoryIDMap()[name]; ok {
		return value
	}
	parts := strings.Split(name, "_")
	if len(parts) == 2 {
		if parent, ok := categoryIDMap()[parts[0]]; ok {
			if child, ok := categoryIDMap()[parts[1]]; ok {
				return parent + "_" + strings.TrimPrefix(child, "c_dataset_")
			}
		}
	}
	return "c_dataset_" + sanitizeID(name)
}

func sanitizeID(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return fmt.Sprintf("cat_%d", len([]rune(value)))
	}
	return builder.String()
}

func categoryIDMap() map[string]string {
	return map[string]string{
		"美妆护肤":  "c_dataset_beauty",
		"数码电子":  "c_dataset_digital",
		"服饰运动":  "c_dataset_clothes",
		"食品饮料":  "c_dataset_food",
		"精华":    "c_dataset_serum",
		"面霜":    "c_dataset_cream",
		"防晒":    "c_dataset_sunscreen",
		"洁面":    "c_dataset_cleanser",
		"面膜":    "c_dataset_mask",
		"眼霜":    "c_dataset_eye_cream",
		"化妆水":   "c_dataset_toner",
		"卸妆":    "c_dataset_makeup_remover",
		"唇釉":    "c_dataset_lip_glaze",
		"眉笔":    "c_dataset_eyebrow_pencil",
		"粉底液":   "c_dataset_foundation",
		"蜜粉":    "c_dataset_powder",
		"智能手机":  "c_dataset_smartphone",
		"笔记本电脑": "c_dataset_laptop",
		"平板电脑":  "c_dataset_tablet",
		"真无线耳机": "c_dataset_tws",
		"跑步鞋":   "c_dataset_running_shoes",
		"篮球鞋":   "c_dataset_basketball_shoes",
		"瑜伽裤":   "c_dataset_yoga_pants",
		"速干T恤":  "c_dataset_quickdry_tshirt",
		"短袖T恤":  "c_dataset_tshirt",
		"运动短裤":  "c_dataset_sports_shorts",
		"运动长裤":  "c_dataset_sports_pants",
		"卫衣":    "c_dataset_hoodie",
		"户外裤":   "c_dataset_outdoor_pants",
		"徒步鞋":   "c_dataset_hiking_shoes",
		"背包":    "c_dataset_backpack",
		"帽子":    "c_dataset_hat",
		"功能饮料":  "c_dataset_energy_drink",
		"坚果/零食": "c_dataset_snacks",
		"咖啡":    "c_dataset_coffee",
		"方便食品":  "c_dataset_convenience_food",
		"牛奶":    "c_dataset_milk",
		"碳酸饮料":  "c_dataset_soda",
		"茶饮":    "c_dataset_tea",
		"调味品":   "c_dataset_condiment",
		"酸奶":    "c_dataset_yogurt",
	}
}

func categorySort(category string) int {
	switch category {
	case "美妆护肤":
		return 100
	case "数码电子":
		return 200
	case "服饰运动":
		return 300
	case "食品饮料":
		return 400
	default:
		return 900
	}
}

func sellingPoints(product datasetProduct) []string {
	points := []string{product.SubCategory, product.Brand}
	for _, sku := range product.SKUs {
		for key, value := range sku.Properties {
			points = append(points, key+"："+value)
		}
		if len(points) >= 5 {
			break
		}
	}
	return compactStrings(points, 5)
}

func recommendReason(product datasetProduct) string {
	text := product.RAGKnowledge.MarketingDescription
	if len([]rune(text)) > 120 {
		text = string([]rune(text)[:120]) + "..."
	}
	return text
}

func attributes(product datasetProduct) []map[string]string {
	items := []map[string]string{
		{"key": "品类", "value": product.Category},
		{"key": "子品类", "value": product.SubCategory},
		{"key": "品牌", "value": product.Brand},
	}
	return items
}

func suitableFor(product datasetProduct) []string {
	switch product.Category {
	case "美妆护肤":
		return []string{"护肤", "成分功效咨询", "规格选择"}
	case "数码电子":
		return []string{"参数对比", "预算决策", "性能咨询"}
	case "服饰运动":
		return []string{"尺码选择", "穿搭场景", "运动需求"}
	case "食品饮料":
		return []string{"家庭囤货", "口味选择", "生活场景"}
	default:
		return []string{"日常购物咨询"}
	}
}

func skuName(product datasetProduct, sku datasetSKU) string {
	values := make([]string, 0, len(sku.Properties))
	for key, value := range sku.Properties {
		values = append(values, key+value)
	}
	sort.Strings(values)
	if len(values) == 0 {
		return product.Title + " 默认款"
	}
	return product.Title + " " + strings.Join(values, " ")
}

func compactStrings(input []string, limit int) []string {
	seen := make(map[string]bool)
	output := make([]string, 0, limit)
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		output = append(output, item)
		if len(output) == limit {
			break
		}
	}
	return output
}

func mustJSON(value any) string {
	content, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(content)
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
