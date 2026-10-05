package services

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FAHD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set FAHD_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = db.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	isolated, e := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sql, e := isolated.DB()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { sql.Close(); db.Exec("DROP SCHEMA " + schema + " CASCADE"); base, _ := db.DB(); base.Close() })
	if e = goose.Up(sql, "../../../migrations"); e != nil {
		t.Fatal(e)
	}
	return isolated
}
func fixture(t *testing.T, db *gorm.DB, stock int) CreateOrderInput {
	t.Helper()
	key := uuid.NewString()
	var id int64
	if e := db.Raw(`INSERT INTO products(title,slug,sku,price,compare_at,stock_total) VALUES('اختبار',?,?,100,150,?) RETURNING id`, key, key, stock).Scan(&id).Error; e != nil {
		t.Fatal(e)
	}
	var city int64
	db.Table("cities").Select("id").Order("sort_order").Limit(1).Scan(&city)
	return CreateOrderInput{Items: []OrderItemInput{{ProductID: id, Qty: 1}}, CityID: city, CustomerName: "عميل اختبار", CustomerPhone: "0500000000", AddressRaw: "عنوان اختبار ABCD1234", PaymentMethod: "cod", IdempotencyKey: uuid.NewString()}
}
func TestCommerce(t *testing.T) {
	db := testDB(t)
	s := NewOrderService(db)
	ctx := context.Background()
	t.Run("canonical tiers and atomic idempotency", func(t *testing.T) {
		in := fixture(t, db, 10)
		in.Items[0].Qty = 2
		in.SessionID = uuid.NewString()
		in.VisitorID = "visitor-test"
		cid, e := Engage(ctx, db, in.SessionID, in.VisitorID, map[string]any{"source": "test"})
		if e != nil {
			t.Fatal(e)
		}
		db.Exec("INSERT INTO pricing_tiers(product_id,qty,original_price,final_price) VALUES(?,2,150,80)", in.Items[0].ProductID)
		q, e := s.Quote(ctx, in)
		if e != nil || q.GrandTotal != 160 {
			t.Fatalf("quote %+v %v", q, e)
		}
		total, e := s.CalculateTotal(ctx, in)
		if e != nil || total["grandTotal"] != float64(160) {
			t.Fatal(total, e)
		}
		in.QuoteID = q.ID
		var wg sync.WaitGroup
		results := make(chan map[string]any, 2)
		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); r, e := s.Create(ctx, in); results <- r; errs <- e }()
		}
		wg.Wait()
		close(results)
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatal(e)
			}
		}
		var id any
		for r := range results {
			if id != nil && id != r["id"] {
				t.Fatal("duplicate order")
			}
			id = r["id"]
			if r["totals"].(map[string]any)["grandTotal"] != float64(160) {
				t.Fatal("wrong order total")
			}
			if _, e = s.Track(ctx, id.(int64), ""); e == nil {
				t.Fatal("tracking without token")
			}
			if _, e = s.Track(ctx, id.(int64), r["trackingToken"].(string)); e != nil {
				t.Fatal(e)
			}
		}
		var stock int
		db.Table("products").Select("stock_total").Where("id=?", in.Items[0].ProductID).Scan(&stock)
		if stock != 8 {
			t.Fatal("inventory deducted twice", stock)
		}
		var linked int64
		db.Table("orders").Select("conversation_id").Where("id=?", id).Scan(&linked)
		if linked != cid {
			t.Fatal("missing conversation")
		}
		if e = s.ChangeStatus(ctx, id.(int64), "cancelled"); e != nil {
			t.Fatal(e)
		}
		_ = s.ChangeStatus(ctx, id.(int64), "cancelled")
		db.Table("products").Select("stock_total").Where("id=?", in.Items[0].ProductID).Scan(&stock)
		if stock != 10 {
			t.Fatal("restock", stock)
		}
		in.CustomerName = "changed"
		if _, e = s.Create(ctx, in); e == nil {
			t.Fatal("idempotency conflict accepted")
		}
	})
	t.Run("last unit concurrent buyers", func(t *testing.T) {
		in := fixture(t, db, 1)
		other := in
		other.IdempotencyKey = uuid.NewString()
		q, e := s.Quote(ctx, in)
		if e != nil {
			t.Fatal(e)
		}
		in.QuoteID = q.ID
		other.QuoteID = q.ID
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, v := range []CreateOrderInput{in, other} {
			wg.Add(1)
			go func(v CreateOrderInput) { defer wg.Done(); _, e := s.Create(ctx, v); errs <- e }(v)
		}
		wg.Wait()
		close(errs)
		success := 0
		for e := range errs {
			if e == nil {
				success++
			}
		}
		if success != 1 {
			t.Fatal("oversold", success)
		}
	})
	t.Run("price change and expired offer require review", func(t *testing.T) {
		in := fixture(t, db, 5)
		in.SessionID = uuid.NewString()
		db.Exec("INSERT INTO session_offers(session_id,product_id,ends_at)VALUES(?,?,?)", in.SessionID, in.Items[0].ProductID, time.Now().Add(time.Hour))
		q, e := s.Quote(ctx, in)
		if e != nil {
			t.Fatal(e)
		}
		in.QuoteID = q.ID
		db.Exec("UPDATE session_offers SET ends_at=NOW()-INTERVAL '1 second' WHERE session_id=?", in.SessionID)
		if _, e = s.Create(ctx, in); e == nil || !strings.Contains(e.Error(), "PRICE_CHANGED") {
			t.Fatal("stale price accepted", e)
		}
		q, e = s.Quote(ctx, in)
		if e != nil || q.GrandTotal != 150 {
			t.Fatal(q, e)
		}
		in.QuoteID = q.ID
		if _, e = s.Create(ctx, in); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("expired quote and coupon invalid", func(t *testing.T) {
		in := fixture(t, db, 2)
		q, e := s.Quote(ctx, in)
		if e != nil {
			t.Fatal(e)
		}
		in.QuoteID = q.ID
		db.Exec("UPDATE checkout_quotes SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=?", q.ID)
		if _, e = s.Create(ctx, in); e == nil {
			t.Fatal("expired quote")
		}
		in.CouponCode = "not-real"
		if _, e = s.Quote(ctx, in); e == nil {
			t.Fatal("invalid coupon")
		}
	})
	t.Run("concurrent first engagement once", func(t *testing.T) {
		sid := uuid.NewString()
		var wg sync.WaitGroup
		ids := make(chan int64, 6)
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id, e := Engage(ctx, db, sid, "v", map[string]any{})
				if e != nil {
					ids <- 0
				} else {
					ids <- id
				}
			}()
		}
		wg.Wait()
		close(ids)
		var expected int64
		for id := range ids {
			if id == 0 {
				t.Fatal("failed")
			}
			if expected != 0 && id != expected {
				t.Fatal("duplicate conversation")
			}
			expected = id
		}
		var count int64
		db.Table("chat_engagements").Where("session_id=?", sid).Count(&count)
		if count != 1 {
			t.Fatal(count)
		}
	})
	t.Run("variant stock and coupon cap", func(t *testing.T) {
		in := fixture(t, db, 0)
		db.Table("products").Where("id=?", in.Items[0].ProductID).Updates(map[string]any{"has_variants": true, "inventory_mode": "variant"})
		var vid int64
		db.Raw("INSERT INTO product_variants(product_id,sku,stock,price_override) VALUES(?,?,2,200) RETURNING id", in.Items[0].ProductID, uuid.NewString()).Scan(&vid)
		if _, e := s.Quote(ctx, in); e == nil {
			t.Fatal("missing variant accepted")
		}
		in.Items[0].VariantID = &vid
		code := "C" + strings.ReplaceAll(uuid.NewString(), "-", "")
		db.Exec("INSERT INTO coupons(code,type,value,max_discount_amount,usage_limit) VALUES(?,'percentage',50,20,1)", code)
		in.CouponCode = code
		q, e := s.Quote(ctx, in)
		if e != nil || q.GrandTotal != 180 {
			t.Fatal(q, e)
		}
		in.QuoteID = q.ID
		if _, e = s.Create(ctx, in); e != nil {
			t.Fatal(e)
		}
		if _, e = s.Quote(ctx, in); e == nil {
			t.Fatal("coupon limit ignored")
		}
	})
}
func TestNormalizePhoneAndSearch(t *testing.T) {
	for _, v := range []string{"0500000000", "+966500000000", "٠٥٠٠٠٠٠٠٠٠"} {
		if got := NormalizePhone(v); got != "966500000000" {
			t.Fatal(fmt.Sprintf("%s -> %s", v, got))
		}
	}
	if NormalizeSearch("إِضاءَة") != "اضاءه" {
		t.Fatal(NormalizeSearch("إِضاءَة"))
	}
}
