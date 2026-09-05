package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	mysql "github.com/go-sql-driver/mysql"
)

// The DSN must point to a disposable MySQL server with CREATE/DROP DATABASE
// privileges. Each test creates its own randomly named database; no existing
// schema is reused, truncated or dropped.
func integrationStore(t *testing.T) *MySQLStore {
	t.Helper()
	dsn := os.Getenv("XZXG_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set XZXG_TEST_MYSQL_DSN to run real MySQL integration tests")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName, cfg.ParseTime = "", true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := fmt.Sprintf("xzxg_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := admin.Exec("DROP DATABASE `" + name + "`")
		if err != nil {
			t.Error(err)
		}
	})
	cfg.DBName = name
	s, err := OpenMySQL(context.Background(), cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	// Migrate uses the same migration file as the application.
	t.Chdir("../..")
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.SetMockPaymentsEnabled(true)
	return s
}

func execInventoryTest(t *testing.T, s *MySQLStore, query string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func pendingInventoryOrder(t *testing.T, s *MySQLStore) domain.Order {
	t.Helper()
	execInventoryTest(t, s, "UPDATE products SET stock_quantity = 8, stock_status = 'in_stock' WHERE product_id = 'p_001'")
	execInventoryTest(t, s, "UPDATE product_skus SET stock_quantity = 8, stock_status = 'in_stock' WHERE sku_id = 'sku_001'")
	if _, ok := s.AddCartItem(context.Background(), "test_buyer", "p_001", "sku_001", 5); !ok {
		t.Fatal("add cart failed")
	}
	orders, ok := s.CreateOrderFromCart(context.Background(), "test_buyer")
	if !ok || len(orders) != 1 {
		t.Fatalf("checkout failed: %#v", orders)
	}
	assertInventoryStock(t, s, 3, "in_stock")
	return orders[0]
}

func assertInventoryStock(t *testing.T, s *MySQLStore, quantity int, status string) {
	t.Helper()
	for _, table := range []string{"products", "product_skus"} {
		var got int
		var gotStatus string
		if err := s.db.QueryRow("SELECT stock_quantity, stock_status FROM "+table+" WHERE product_id = 'p_001'").Scan(&got, &gotStatus); err != nil {
			t.Fatal(err)
		}
		if got != quantity || gotStatus != status {
			t.Fatalf("%s: got %d/%s, want %d/%s", table, got, gotStatus, quantity, status)
		}
	}
}

func assertReservation(t *testing.T, s *MySQLStore, id, state string) {
	t.Helper()
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM inventory_reservations WHERE order_id = ? AND state = ? AND quantity = 5", id, state).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reservation %s: want one %s, got %d", id, state, count)
	}
}

func TestMySQLInventoryCancelIsAtomicAndIdempotent(t *testing.T) {
	s := integrationStore(t)
	order := pendingInventoryOrder(t, s)
	if _, ok := s.CancelOrder(context.Background(), "someone_else", order.OrderID, ""); ok {
		t.Fatal("cross-account cancellation allowed")
	}
	assertInventoryStock(t, s, 3, "in_stock")
	// Fail after stock updates, before the reservation transition. All prior
	// writes in the close transaction must roll back, including SKU stock.
	execInventoryTest(t, s, `CREATE TRIGGER fail_release BEFORE UPDATE ON inventory_reservations FOR EACH ROW
		BEGIN IF NEW.state = 'released' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected release failure'; END IF; END`)
	if _, ok := s.CancelOrder(context.Background(), "test_buyer", order.OrderID, ""); ok {
		t.Fatal("injected failure succeeded")
	}
	assertInventoryStock(t, s, 3, "in_stock")
	assertReservation(t, s, order.OrderID, "reserved")
	execInventoryTest(t, s, "DROP TRIGGER fail_release")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.CancelOrder(context.Background(), "test_buyer", order.OrderID, "") }()
	}
	wg.Wait()
	assertInventoryStock(t, s, 8, "in_stock")
	assertReservation(t, s, order.OrderID, "released")
	got, ok := s.GetOrder(context.Background(), "test_buyer", order.OrderID)
	if !ok || got.Status != "canceled" {
		t.Fatalf("unexpected order: %#v", got)
	}
	var payment string
	if err := s.db.QueryRow("SELECT status FROM payments WHERE order_id = ?", order.OrderID).Scan(&payment); err != nil {
		t.Fatal(err)
	}
	if payment != "failed" {
		t.Fatalf("payment = %s", payment)
	}
	issues, err := s.InventoryAudit(context.Background())
	if err != nil || len(issues) != 0 {
		t.Fatalf("post-cancel audit: %#v, %v", issues, err)
	}
	execInventoryTest(t, s, "UPDATE inventory_reservations SET quantity = 6 WHERE order_id = ?", order.OrderID)
	issues, err = s.InventoryAudit(context.Background())
	if err != nil || len(issues) != 1 || issues[0].Kind != "reservation_mismatch" {
		t.Fatalf("corruption not detected: %#v, %v", issues, err)
	}
}

func TestMySQLInventoryPaymentCancelRace(t *testing.T) {
	s := integrationStore(t)
	for i := 0; i < 20; i++ {
		order := pendingInventoryOrder(t, s)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			s.PayOrder(context.Background(), "test_buyer", order.OrderID, "mock_balance")
		}()
		go func() { defer wg.Done(); <-start; s.CancelOrder(context.Background(), "test_buyer", order.OrderID, "") }()
		close(start)
		wg.Wait()
		got, ok := s.GetOrder(context.Background(), "test_buyer", order.OrderID)
		if !ok {
			t.Fatal("order disappeared")
		}
		switch got.Status {
		case "pending_ship":
			assertInventoryStock(t, s, 3, "in_stock")
			assertReservation(t, s, order.OrderID, "consumed")
		case "canceled":
			assertInventoryStock(t, s, 8, "in_stock")
			assertReservation(t, s, order.OrderID, "released")
		default:
			t.Fatalf("unexpected race outcome: %s", got.Status)
		}
	}
}

func TestMySQLInventoryLegacyExpiryAndPaymentRace(t *testing.T) {
	s := integrationStore(t)
	order := pendingInventoryOrder(t, s)
	// Simulate a pending order made before reservation tracking was introduced.
	execInventoryTest(t, s, "DELETE FROM inventory_reservations WHERE order_id = ?", order.OrderID)
	execInventoryTest(t, s, "UPDATE orders SET payment_deadline_at = ? WHERE order_id = ?", time.Now().Add(-time.Second), order.OrderID)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		if _, err := s.ExpirePendingOrdersBatch(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, _, ok := s.PayOrder(context.Background(), "test_buyer", order.OrderID, "mock_balance"); ok {
			t.Error("expired order paid")
		}
	}()
	go func() { defer wg.Done(); s.CancelOrder(context.Background(), "test_buyer", order.OrderID, "") }()
	wg.Wait()
	if _, err := s.ExpirePendingOrdersBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertInventoryStock(t, s, 8, "in_stock")
	assertReservation(t, s, order.OrderID, "released")
	got, ok := s.GetOrder(context.Background(), "test_buyer", order.OrderID)
	if !ok || got.Status != "closed_timeout" {
		t.Fatalf("unexpected expiry: %#v", got)
	}
}

func TestMySQLProductionCannotSimulatePayment(t *testing.T) {
	s := integrationStore(t)
	order := pendingInventoryOrder(t, s)
	s.SetMockPaymentsEnabled(false)
	if _, _, ok := s.PayOrder(context.Background(), "test_buyer", order.OrderID, "mock_balance"); ok {
		t.Fatal("disabled payment changed an order")
	}
	got, ok := s.GetOrder(context.Background(), "test_buyer", order.OrderID)
	if !ok || got.Status != "pending_payment" {
		t.Fatalf("order changed: %#v", got)
	}
	assertInventoryStock(t, s, 3, "in_stock")
	assertReservation(t, s, order.OrderID, "reserved")
}

func TestMySQLInventoryInsufficientStockRollsBackWholeCart(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	execInventoryTest(t, s, "UPDATE products SET stock_quantity = 8, stock_status = 'in_stock' WHERE product_id = 'p_001'")
	execInventoryTest(t, s, "UPDATE product_skus SET stock_quantity = 8, stock_status = 'in_stock' WHERE sku_id = 'sku_001'")
	execInventoryTest(t, s, "UPDATE products SET stock_quantity = 0, stock_status = 'out_of_stock' WHERE product_id = 'p_002'")
	execInventoryTest(t, s, "UPDATE product_skus SET stock_quantity = 0, stock_status = 'out_of_stock' WHERE sku_id = 'sku_002'")
	s.AddCartItem(ctx, "test_buyer", "p_001", "sku_001", 5)
	s.AddCartItem(ctx, "test_buyer", "p_002", "sku_002", 1)
	if _, ok := s.CreateOrderFromCart(ctx, "test_buyer"); ok {
		t.Fatal("oversold cart")
	}
	assertInventoryStock(t, s, 8, "in_stock")
	for table, want := range map[string]int{"orders": 0, "inventory_reservations": 0, "cart_items": 2} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("%s count = %d, want %d", table, count, want)
		}
	}
}

func TestMySQLInventoryFinalUnitAndReopen(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	execInventoryTest(t, s, "UPDATE products SET stock_quantity = 1, stock_status = 'in_stock' WHERE product_id = 'p_001'")
	execInventoryTest(t, s, "UPDATE product_skus SET stock_quantity = 1, stock_status = 'in_stock' WHERE sku_id = 'sku_001'")
	s.AddCartItem(ctx, "test_buyer", "p_001", "sku_001", 1)
	orders, ok := s.CreateOrderFromCart(ctx, "test_buyer")
	if !ok || len(orders) != 1 {
		t.Fatal("last unit checkout failed")
	}
	assertInventoryStock(t, s, 0, "out_of_stock")
	if _, ok := s.CancelOrder(ctx, "test_buyer", orders[0].OrderID, ""); !ok {
		t.Fatal("cancel failed")
	}
	assertInventoryStock(t, s, 1, "in_stock")
}
