package services

import (
	"context"
	"testing"
)

func TestShemgBundlePricingAndShipping(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	s := NewOrderService(db)
	var productID, cityID int64
	if err := db.Table("products").Select("id").Where("sku=?", "Hala0002074").Take(&productID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("INSERT INTO cities(name,shipping_fee,is_active) VALUES('مدينة اختبار الباقات',25,TRUE) RETURNING id").Scan(&cityID).Error; err != nil {
		t.Fatal(err)
	}
	in := fixture(t, db, 10)
	in.CityID = cityID
	in.Items = []OrderItemInput{{ProductID: productID, Qty: 2}}
	for _, tc := range []struct {
		qty   int
		total float64
	}{{2, 178}, {3, 209}, {5, 277}} {
		in.Items[0].Qty = tc.qty
		q, err := s.Quote(ctx, in)
		if err != nil || q.Subtotal != tc.total || q.GrandTotal != tc.total || q.Items[0].LineTotal != tc.total || q.Shipping != 0 || !q.ShippingKnown {
			t.Fatalf("bundle %d: %+v, %v", tc.qty, q, err)
		}
		botTotal, err := s.CalculateTotal(ctx, in)
		if err != nil || botTotal["grandTotal"] != tc.total {
			t.Fatalf("bot total does not match checkout: %+v, %v", botTotal, err)
		}
	}
	for _, qty := range []int{1, 4, 6} {
		in.Items[0].Qty = qty
		if _, err := s.Quote(ctx, in); err == nil {
			t.Fatalf("undefined bundle %d was purchasable", qty)
		}
	}
	in.Items[0].Qty = 3
	q, err := s.Quote(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	in.QuoteID = q.ID
	r, err := s.Create(ctx, in)
	if err != nil || r["totals"].(map[string]any)["grandTotal"] != float64(209) {
		t.Fatalf("order failed or rounded bundle total changed: %+v, %v", r, err)
	}
	var stock int
	if err := db.Table("products").Select("stock_total").Where("id=?", productID).Scan(&stock).Error; err != nil || stock != 997 {
		t.Fatalf("bundle stock should deduct pieces: %d, %v", stock, err)
	}
	// An ordinary product in the same basket must retain the city's delivery fee.
	ordinary := fixture(t, db, 10)
	in.Items = append(in.Items, ordinary.Items[0])
	q, err = s.Quote(ctx, in)
	if err != nil || q.Shipping != 25 || q.GrandTotal != 334 {
		t.Fatalf("mixed basket shipping: %+v, %v", q, err)
	}
	if err := db.Table("products").Where("id=?", productID).Update("stock_total", 2).Error; err != nil {
		t.Fatal(err)
	}
	in.Items = in.Items[:1]
	if _, err := s.Quote(ctx, in); err == nil {
		t.Fatal("bundle oversold its pieces")
	}
}

func TestExactBundleDoesNotBecomeAnImplicitUnitDiscount(t *testing.T) {
	db := testDB(t)
	in := fixture(t, db, 10)
	if err := db.Exec("INSERT INTO pricing_tiers(product_id,qty,original_price,final_price,bundle_price) VALUES(?,2,150,80,NULL),(?,3,150,69.67,209)", in.Items[0].ProductID, in.Items[0].ProductID).Error; err != nil {
		t.Fatal(err)
	}
	in.Items[0].Qty = 4
	q, err := NewOrderService(db).Quote(context.Background(), in)
	if err != nil || q.Subtotal != 320 {
		t.Fatalf("legacy unit tier changed: %+v, %v", q, err)
	}
}
