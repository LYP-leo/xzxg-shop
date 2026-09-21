package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
	"github.com/LYP-leo/xzxg-shop/backend/src/pricing"
)

func quoteCartFixture(t *testing.T, s *MySQLStore) {
	t.Helper()
	execInventoryTest(t, s, "UPDATE promotion_rules SET status = 'inactive'")
	execInventoryTest(t, s, `INSERT INTO merchants (merchant_id,name,description,status) VALUES ('m_quote','Second merchant','','active')`)
	execInventoryTest(t, s, "UPDATE products SET price = 12.34, stock_quantity = 20 WHERE product_id = 'p_001'")
	execInventoryTest(t, s, "UPDATE product_skus SET price = 12.34, stock_quantity = 20 WHERE sku_id = 'sku_001'")
	execInventoryTest(t, s, "UPDATE products SET price = 7.89, stock_quantity = 20, merchant_id = 'm_quote' WHERE product_id = 'p_002'")
	execInventoryTest(t, s, "UPDATE product_skus SET price = 7.89, stock_quantity = 20 WHERE sku_id = 'sku_002'")
	if _, ok := s.AddCartItem(context.Background(), "test_buyer", "p_001", "sku_001", 2); !ok {
		t.Fatal("add phone")
	}
	if _, ok := s.AddCartItem(context.Background(), "test_buyer", "p_002", "sku_002", 1); !ok {
		t.Fatal("add camera")
	}
	execInventoryTest(t, s, `INSERT INTO promotion_rules
		(promotion_id,name,scope,product_id,type,threshold_amount,discount_amount,stackable,start_at,end_at,status)
		VALUES ('quote_product','Product discount','product','p_001','full_reduction',20,2,TRUE,?,?,'active')`, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	execInventoryTest(t, s, `INSERT INTO coupons (coupon_id,name,scope,type,threshold_amount,discount_amount,stackable,start_at,end_at,status)
		VALUES ('quote_coupon','Basket coupon','platform','fixed_amount',30,3.01,TRUE,?,?,'active')`, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	execInventoryTest(t, s, `INSERT INTO user_coupons (user_coupon_id,coupon_id,account_id,status,claimed_at)
		VALUES ('uc_quote','quote_coupon','test_buyer','unused',?)`, time.Now())
}

func makeTestQuote(t *testing.T, s *MySQLStore) domain.CartQuote {
	t.Helper()
	quote, err := s.CreateCartQuote(context.Background(), "test_buyer")
	if err != nil {
		t.Fatal(err)
	}
	if quote.TotalAmount != "32.57" || quote.DiscountAmount != "5.01" || quote.PayAmount != "27.56" {
		t.Fatalf("wrong quote: %#v", quote)
	}
	return quote
}

func quoteConfirmation(q domain.CartQuote) domain.CheckoutConfirmation {
	return domain.CheckoutConfirmation{QuoteID: q.QuoteID, ConfirmationToken: q.ConfirmationToken}
}

func assertCouponStatus(t *testing.T, s *MySQLStore, want string) {
	t.Helper()
	var got string
	if err := s.db.QueryRow("SELECT status FROM user_coupons WHERE user_coupon_id = 'uc_quote'").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("coupon=%s want %s", got, want)
	}
}

func TestMySQLQuoteExactAmountsIdempotencyAndPartialCancellation(t *testing.T) {
	s := integrationStore(t)
	quoteCartFixture(t, s)
	quote := makeTestQuote(t, s)
	ctx := context.Background()
	var wg sync.WaitGroup
	outputs := make([][]domain.Order, 6)
	errs := make([]error, 6)
	for i := range outputs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outputs[i], errs[i] = s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(quote))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
		if len(outputs[i]) != 2 {
			t.Fatal("wrong merchant split")
		}
		for j := range outputs[0] {
			if outputs[i][j].OrderID != outputs[0][j].OrderID {
				t.Fatal("duplicate order")
			}
		}
	}
	orders := outputs[0]
	var pay, total, discount pricing.Money
	for _, order := range orders {
		p, _ := pricing.ParseMoney(order.PayAmount)
		a, _ := pricing.ParseMoney(order.TotalAmount)
		d, _ := pricing.ParseMoney(order.DiscountAmount)
		pay += p
		total += a
		discount += d
		var payment string
		if err := s.db.QueryRow("SELECT amount FROM payments WHERE order_id = ?", order.OrderID).Scan(&payment); err != nil {
			t.Fatal(err)
		}
		if payment != order.PayAmount {
			t.Fatalf("payment %s != order %s", payment, order.PayAmount)
		}
	}
	if pay.String() != quote.PayAmount || total.String() != quote.TotalAmount || discount.String() != quote.DiscountAmount {
		t.Fatal("split lost money")
	}
	assertCouponStatus(t, s, "reserved")
	if _, _, ok := s.PayOrder(ctx, "test_buyer", orders[0].OrderID, "mock_balance"); !ok {
		t.Fatal("pay failed")
	}
	if _, ok := s.CancelOrder(ctx, "test_buyer", orders[1].OrderID, ""); !ok {
		t.Fatal("cancel failed")
	}
	assertCouponStatus(t, s, "used")
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count); err != nil || count != 2 {
		t.Fatalf("orders=%d err=%v", count, err)
	}
}

func TestMySQLQuoteRejectsChangedFactsAndWrongAuthorization(t *testing.T) {
	s := integrationStore(t)
	quoteCartFixture(t, s)
	ctx := context.Background()
	q := makeTestQuote(t, s)
	if _, err := s.CheckoutQuote(ctx, "other_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteConfirmation) {
		t.Fatalf("ownership: %v", err)
	}
	bad := quoteConfirmation(q)
	bad.ConfirmationToken = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := s.CheckoutQuote(ctx, "test_buyer", bad); !errors.Is(err, ErrQuoteConfirmation) {
		t.Fatalf("token: %v", err)
	}
	var cartID string
	if err := s.db.QueryRow("SELECT cart_item_id FROM cart_items WHERE account_id='test_buyer' AND product_id='p_001'").Scan(&cartID); err != nil {
		t.Fatal(err)
	}
	quantity := 3
	s.UpdateCartItem(ctx, "test_buyer", cartID, &quantity, nil)
	quantity = 2
	s.UpdateCartItem(ctx, "test_buyer", cartID, &quantity, nil)
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("cart revision: %v", err)
	}
	q = makeTestQuote(t, s)
	execInventoryTest(t, s, "UPDATE product_skus SET price=12.35 WHERE sku_id='sku_001'")
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("price: %v", err)
	}
	execInventoryTest(t, s, "UPDATE product_skus SET price=12.34 WHERE sku_id='sku_001'")
	q = makeTestQuote(t, s)
	execInventoryTest(t, s, `UPDATE product_skus SET specs_json=JSON_OBJECT('color','white') WHERE sku_id='sku_001'`)
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("SKU specification: %v", err)
	}
	q = makeTestQuote(t, s)
	execInventoryTest(t, s, "UPDATE promotion_rules SET discount_amount=1.99 WHERE promotion_id='quote_product'")
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("rule: %v", err)
	}
	execInventoryTest(t, s, "UPDATE promotion_rules SET discount_amount=2 WHERE promotion_id='quote_product'")
	q = makeTestQuote(t, s)
	execInventoryTest(t, s, "UPDATE cart_quotes SET expires_at=? WHERE quote_id=?", time.Now().Add(-time.Second), q.QuoteID)
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); !errors.Is(err, ErrQuoteExpired) {
		t.Fatalf("expiry: %v", err)
	}
	assertCouponStatus(t, s, "unused")
}

func TestMySQLQuoteFailureRollbackAndCouponRelease(t *testing.T) {
	s := integrationStore(t)
	quoteCartFixture(t, s)
	q := makeTestQuote(t, s)
	ctx := context.Background()
	execInventoryTest(t, s, `CREATE TRIGGER fail_coupon BEFORE INSERT ON order_coupon_allocations FOR EACH ROW
		SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected coupon allocation failure'`)
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); err == nil {
		t.Fatal("failure injection succeeded")
	}
	assertCouponStatus(t, s, "unused")
	for _, table := range []string{"orders", "payments", "inventory_reservations", "order_coupon_allocations"} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s leaked rows: %d %v", table, count, err)
		}
	}
	var stock int
	if err := s.db.QueryRow("SELECT stock_quantity FROM products WHERE product_id='p_001'").Scan(&stock); err != nil || stock != 20 {
		t.Fatalf("stock not restored: %d %v", stock, err)
	}
	execInventoryTest(t, s, "DROP TRIGGER fail_coupon")
	orders, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, order := range orders {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, ok := s.CancelOrder(ctx, "test_buyer", id, ""); !ok {
				t.Error("cancel failed")
			}
		}(order.OrderID)
	}
	wg.Wait()
	assertCouponStatus(t, s, "unused")
}

func TestMySQLQuoteDifferentConfirmationsCannotReuseCartOrCoupon(t *testing.T) {
	s := integrationStore(t)
	quoteCartFixture(t, s)
	ctx := context.Background()
	first, second := makeTestQuote(t, s), makeTestQuote(t, s)
	orders, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(first))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(second)); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("second quote reused cart: %v", err)
	}
	for _, order := range orders {
		execInventoryTest(t, s, "UPDATE orders SET payment_deadline_at=? WHERE order_id=?", time.Now().Add(-time.Second), order.OrderID)
	}
	if _, err := s.ExpirePendingOrdersBatch(ctx); err != nil {
		t.Fatal(err)
	}
	assertCouponStatus(t, s, "unused")
}

func TestMySQLQuoteCrossMerchantPayCancelRaceKeepsCouponConsumed(t *testing.T) {
	s := integrationStore(t)
	quoteCartFixture(t, s)
	q := makeTestQuote(t, s)
	ctx := context.Background()
	orders, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, _, ok := s.PayOrder(ctx, "test_buyer", orders[0].OrderID, "mock_balance"); !ok {
			t.Error("pay failed")
		}
	}()
	go func() {
		defer wg.Done()
		if _, ok := s.CancelOrder(ctx, "test_buyer", orders[1].OrderID, ""); !ok {
			t.Error("cancel failed")
		}
	}()
	wg.Wait()
	assertCouponStatus(t, s, "used")
}

func TestMySQLQuoteTransientFailurePreservesConfirmationForRetry(t *testing.T) {
	for _, stage := range []struct{ table, action string }{
		{"product_skus", "UPDATE"}, {"products", "UPDATE"},
		{"orders", "INSERT"}, {"payments", "INSERT"},
		{"order_items", "INSERT"}, {"inventory_reservations", "INSERT"},
		{"cart_items", "DELETE"},
	} {
		t.Run(stage.table, func(t *testing.T) {
			s := integrationStore(t)
			quoteCartFixture(t, s)
			q := makeTestQuote(t, s)
			ctx := context.Background()
			execInventoryTest(t, s, "CREATE TRIGGER fail_checkout BEFORE "+stage.action+" ON "+stage.table+" FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected storage failure'")
			if _, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q)); err == nil || errors.Is(err, ErrCartUnavailable) || errors.Is(err, ErrQuoteChanged) {
				t.Fatalf("storage failure must remain retryable: %v", err)
			}
			for _, table := range []string{"orders", "payments", "order_items", "inventory_reservations", "order_coupon_allocations"} {
				var count int
				if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
					t.Fatalf("%s leaked writes: %d %v", table, count, err)
				}
			}
			var cartCount, productStock, skuStock int
			var status string
			if err := s.db.QueryRow("SELECT COUNT(*) FROM cart_items WHERE account_id='test_buyer'").Scan(&cartCount); err != nil || cartCount != 2 {
				t.Fatalf("cart not restored: %d %v", cartCount, err)
			}
			if err := s.db.QueryRow("SELECT stock_quantity FROM products WHERE product_id='p_001'").Scan(&productStock); err != nil || productStock != 20 {
				t.Fatalf("product stock not restored: %d %v", productStock, err)
			}
			if err := s.db.QueryRow("SELECT stock_quantity FROM product_skus WHERE sku_id='sku_001'").Scan(&skuStock); err != nil || skuStock != 20 {
				t.Fatalf("SKU stock not restored: %d %v", skuStock, err)
			}
			if err := s.db.QueryRow("SELECT status FROM cart_quotes WHERE quote_id=?", q.QuoteID).Scan(&status); err != nil || status != "open" {
				t.Fatalf("quote consumed: %s %v", status, err)
			}
			assertCouponStatus(t, s, "unused")
			execInventoryTest(t, s, "DROP TRIGGER fail_checkout")
			orders, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q))
			if err != nil || len(orders) != 2 {
				t.Fatalf("same confirmation retry failed: %v %v", orders, err)
			}
			replay, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q))
			if err != nil || len(replay) != len(orders) {
				t.Fatalf("receipt replay failed: %v", err)
			}
			for i := range orders {
				if replay[i].OrderID != orders[i].OrderID {
					t.Fatal("retry created a different order")
				}
			}
		})
	}
}

func TestMySQLQuoteChecksCombinedProductStockAcrossSKUs(t *testing.T) {
	s := integrationStore(t)
	ctx := context.Background()
	execInventoryTest(t, s, "UPDATE promotion_rules SET status='inactive'")
	execInventoryTest(t, s, "UPDATE products SET stock_quantity=3, stock_status='in_stock' WHERE product_id='p_001'")
	execInventoryTest(t, s, "UPDATE product_skus SET stock_quantity=3, stock_status='in_stock' WHERE sku_id='sku_001'")
	execInventoryTest(t, s, `INSERT INTO product_skus (sku_id,product_id,sku_name,price,stock_quantity,stock_status,specs_json)
		SELECT 'sku_quote_second',product_id,'Second variant',price,3,'in_stock','{}' FROM product_skus WHERE sku_id='sku_001'`)
	for _, sku := range []string{"sku_001", "sku_quote_second"} {
		if _, ok := s.AddCartItem(ctx, "test_buyer", "p_001", sku, 2); !ok {
			t.Fatal("fixture add failed")
		}
	}
	if _, err := s.CreateCartQuote(ctx, "test_buyer"); !errors.Is(err, ErrCartUnavailable) {
		t.Fatalf("unfulfillable combined product quantity quoted: %v", err)
	}
	execInventoryTest(t, s, "UPDATE products SET stock_quantity=4 WHERE product_id='p_001'")
	q, err := s.CreateCartQuote(ctx, "test_buyer")
	if err != nil {
		t.Fatal(err)
	}
	orders, err := s.CheckoutQuote(ctx, "test_buyer", quoteConfirmation(q))
	if err != nil || len(orders) != 1 || len(orders[0].Items) != 2 {
		t.Fatalf("combined stock boundary failed: %v %v", orders, err)
	}
	var stock int
	var status string
	if err := s.db.QueryRow("SELECT stock_quantity,stock_status FROM products WHERE product_id='p_001'").Scan(&stock, &status); err != nil || stock != 0 || status != "out_of_stock" {
		t.Fatalf("final inventory mismatch: %d %s %v", stock, status, err)
	}
}
