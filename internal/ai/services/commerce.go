package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"sort"
	"strings"
	"time"
)

type PricedItem struct {
	ProductID int64   `json:"productId"`
	VariantID *int64  `json:"variantId,omitempty"`
	Title     string  `json:"title"`
	SKU       string  `json:"sku"`
	Qty       int     `json:"qty"`
	UnitPrice float64 `json:"unitPrice"`
	LineTotal float64 `json:"lineTotal"`
}
type Quote struct {
	ID               string       `json:"quoteId,omitempty"`
	Items            []PricedItem `json:"items"`
	Subtotal         float64      `json:"subtotal"`
	Shipping         float64      `json:"shipping"`
	Discount         float64      `json:"discount"`
	GrandTotal       float64      `json:"grandTotal"`
	Currency         string       `json:"currency"`
	DeliveryEstimate string       `json:"deliveryEstimate"`
	ShippingKnown    bool         `json:"shippingKnown"`
	ExpiresAt        time.Time    `json:"expiresAt"`
}
type quoteRow struct {
	ID, RequestHash, PricingHash, SessionID string
	ExpiresAt                               time.Time
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func pricingHash(q Quote) string { q.ID = ""; q.ExpiresAt = time.Time{}; return digest(q) }
func quoteRequestHash(in CreateOrderInput) string {
	return digest(struct {
		Items           []OrderItemInput
		City            int64
		Coupon, Session string
	}{in.Items, in.CityID, strings.ToUpper(strings.TrimSpace(in.CouponCode)), in.SessionID})
}
func NormalizePhone(s string) string {
	s = strings.Map(func(r rune) rune {
		if r >= '٠' && r <= '٩' {
			return '0' + r - '٠'
		}
		if r >= '۰' && r <= '۹' {
			return '0' + r - '۰'
		}
		if r == ' ' || r == '-' || r == '(' || r == ')' {
			return -1
		}
		return r
	}, s)
	s = strings.TrimPrefix(s, "+")
	s = strings.TrimPrefix(s, "00")
	if strings.HasPrefix(s, "05") {
		s = "966" + s[1:]
	}
	return s
}

var saudiPhone = regexp.MustCompile(`^9665[0-9]{8}$`)

func (s *OrderService) price(ctx context.Context, db *gorm.DB, in CreateOrderInput, lock bool) (Quote, error) {
	q := Quote{Items: []PricedItem{}, Currency: "SAR", ExpiresAt: time.Now().Add(10 * time.Minute)}
	if len(in.Items) == 0 || len(in.Items) > 50 {
		return q, fmt.Errorf("اختر منتجًا واحدًا على الأقل")
	}
	items := append([]OrderItemInput(nil), in.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProductID != items[j].ProductID {
			return items[i].ProductID < items[j].ProductID
		}
		return variantNumber(items[i]) < variantNumber(items[j])
	})
	seen := map[string]bool{}
	for _, it := range items {
		if it.ProductID <= 0 || it.Qty < 1 || it.Qty > 1000 {
			return q, fmt.Errorf("الكمية غير صالحة")
		}
		key := fmt.Sprintf("%d:%d", it.ProductID, variantNumber(it))
		if seen[key] {
			return q, fmt.Errorf("اجمع كمية المنتج في سطر واحد")
		}
		seen[key] = true
		var p struct {
			ID                        int64
			Title, SKU, InventoryMode string
			Price                     float64
			CompareAt                 *float64
			StockTotal                int
			HasVariants               bool
		}
		query := db.WithContext(ctx).Table("products").Where("id=? AND status='active'", it.ProductID)
		if lock {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Take(&p).Error; err != nil {
			return q, fmt.Errorf("المنتج غير متاح")
		}
		unit := p.Price
		stock := p.StockTotal
		override := false
		if p.HasVariants && it.VariantID == nil {
			return q, fmt.Errorf("اختر خيارات المنتج")
		}
		if it.VariantID != nil {
			var v struct {
				PriceOverride *float64
				Stock         int
				SKU           string
			}
			vq := db.Table("product_variants").Where("id=? AND product_id=? AND is_active=TRUE", *it.VariantID, it.ProductID)
			if lock {
				vq = vq.Clauses(clause.Locking{Strength: "UPDATE"})
			}
			if err := vq.Take(&v).Error; err != nil {
				return q, fmt.Errorf("خيار المنتج غير متاح")
			}
			if p.InventoryMode == "variant" {
				stock = v.Stock
			}
			p.SKU = v.SKU
			if v.PriceOverride != nil {
				unit = *v.PriceOverride
				override = true
			}
		}
		if it.Qty > stock {
			return q, fmt.Errorf("الكمية المطلوبة من %s غير متاحة؛ المتوفر %d", p.Title, stock)
		}
		if !override {
			var tier struct{ FinalPrice float64 }
			r := db.Table("pricing_tiers").Select("final_price").Where("product_id=? AND qty<=?", p.ID, it.Qty).Order("qty DESC").Limit(1).Find(&tier)
			if r.Error != nil {
				return q, r.Error
			}
			if r.RowsAffected > 0 {
				unit = tier.FinalPrice
			}
		}
		if !p.HasVariants && p.CompareAt != nil && *p.CompareAt > p.Price && in.SessionID != "" {
			var offer struct{ EndsAt time.Time }
			r := db.Table("session_offers").Where("session_id=? AND product_id=?", in.SessionID, p.ID).Limit(1).Find(&offer)
			if r.Error != nil {
				return q, r.Error
			}
			if r.RowsAffected > 0 {
				if !time.Now().Before(offer.EndsAt) {
					unit = *p.CompareAt
				} else if offer.EndsAt.Before(q.ExpiresAt) {
					q.ExpiresAt = offer.EndsAt
				}
			}
		}
		if lock {
			var r *gorm.DB
			if p.InventoryMode == "variant" && it.VariantID != nil {
				r = db.Table("product_variants").Where("id=? AND stock>=?", *it.VariantID, it.Qty).Update("stock", gorm.Expr("stock - ?", it.Qty))
			} else {
				r = db.Table("products").Where("id=? AND stock_total>=?", p.ID, it.Qty).Update("stock_total", gorm.Expr("stock_total - ?", it.Qty))
			}
			if r.Error != nil {
				return q, r.Error
			}
			if r.RowsAffected != 1 {
				return q, fmt.Errorf("نفدت الكمية المطلوبة")
			}
		}
		line := roundMoney(unit * float64(it.Qty))
		q.Subtotal += line
		q.Items = append(q.Items, PricedItem{it.ProductID, it.VariantID, p.Title, p.SKU, it.Qty, roundMoney(unit), line})
	}
	q.Subtotal = roundMoney(q.Subtotal)
	if in.CityID > 0 {
		var city struct {
			ShippingFee      float64
			DeliveryEstimate *string
		}
		if err := db.Table("cities").Where("id=? AND is_active=TRUE", in.CityID).Take(&city).Error; err != nil {
			return q, fmt.Errorf("اختر مدينة ضمن مناطق التوصيل")
		}
		q.Shipping = city.ShippingFee
		q.ShippingKnown = true
		if city.DeliveryEstimate != nil {
			q.DeliveryEstimate = *city.DeliveryEstimate
		}
	}
	if code := strings.TrimSpace(in.CouponCode); code != "" {
		var c struct {
			ID                          int64
			Type                        string
			Value                       float64
			MinOrder, MaxDiscountAmount *float64
			UsageLimit                  *int
			UsageCount                  int
			StartsAt, ExpiresAt         *time.Time
		}
		cq := db.Table("coupons").Where("LOWER(code)=LOWER(?) AND is_active=TRUE", code)
		if lock {
			cq = cq.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := cq.Take(&c).Error; err != nil {
			return q, fmt.Errorf("كود الخصم غير صالح")
		}
		now := time.Now()
		if (c.StartsAt != nil && now.Before(*c.StartsAt)) || (c.ExpiresAt != nil && !now.Before(*c.ExpiresAt)) || (c.UsageLimit != nil && c.UsageCount >= *c.UsageLimit) || (c.MinOrder != nil && q.Subtotal < *c.MinOrder) {
			return q, fmt.Errorf("شروط كود الخصم غير متحققة")
		}
		q.Discount = c.Value
		if c.Type == "percentage" {
			q.Discount = q.Subtotal * c.Value / 100
		}
		if c.MaxDiscountAmount != nil && q.Discount > *c.MaxDiscountAmount {
			q.Discount = *c.MaxDiscountAmount
		}
		if q.Discount > q.Subtotal {
			q.Discount = q.Subtotal
		}
		q.Discount = roundMoney(q.Discount)
		if lock {
			if err := db.Table("coupons").Where("id=?", c.ID).Update("usage_count", gorm.Expr("usage_count+1")).Error; err != nil {
				return q, err
			}
		}
	}
	q.GrandTotal = roundMoney(q.Subtotal + q.Shipping - q.Discount)
	return q, nil
}
func variantNumber(i OrderItemInput) int64 {
	if i.VariantID == nil {
		return 0
	}
	return *i.VariantID
}
func (s *OrderService) Quote(ctx context.Context, in CreateOrderInput) (Quote, error) {
	q, err := s.price(ctx, s.db, in, false)
	if err != nil {
		return q, err
	}
	q.ID = uuid.NewString()
	err = s.db.WithContext(ctx).Table("checkout_quotes").Create(&quoteRow{ID: q.ID, RequestHash: quoteRequestHash(in), PricingHash: pricingHash(q), SessionID: in.SessionID, ExpiresAt: q.ExpiresAt}).Error
	return q, err
}
func (s *OrderService) CalculateTotal(ctx context.Context, in CreateOrderInput) (map[string]any, error) {
	q, e := s.price(ctx, s.db, in, false)
	if e != nil {
		return nil, e
	}
	b, _ := json.Marshal(q)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m, nil
}

// Session-level advisory locking prevents HTTP buttons and the first WS message from creating separate conversations.
func EnsureConversation(ctx context.Context, db *gorm.DB, sessionID, visitorID string) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("جلسة المحادثة مطلوبة")
	}
	var id int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "chat:"+sessionID).Error; e != nil {
			return e
		}
		if e := tx.Table("conversations").Select("id").Where("session_id=?", sessionID).Order("id DESC").Limit(1).Scan(&id).Error; e != nil {
			return e
		}
		if id == 0 {
			row := struct {
				ID                           int64
				Status, SessionID, VisitorID string
			}{Status: "active", SessionID: sessionID, VisitorID: visitorID}
			if e := tx.Table("conversations").Create(&row).Error; e != nil {
				return e
			}
			id = row.ID
		}
		return nil
	})
	return id, err
}
func Engage(ctx context.Context, db *gorm.DB, sessionID, visitorID string, meta map[string]any) (int64, error) {
	id, e := EnsureConversation(ctx, db, sessionID, visitorID)
	if e != nil {
		return 0, e
	}
	source, device, exp := "direct", "unknown", "baseline"
	if v, ok := meta["source"].(string); ok && len(v) < 120 && v != "" {
		source = v
	}
	if v, ok := meta["device"].(string); ok && (v == "mobile" || v == "desktop") {
		device = v
	}
	if v, ok := meta["experiment"].(string); ok && (v == "offer-value-v1:control" || v == "offer-value-v1:variant") {
		exp = v
	}
	returning, _ := meta["returning"].(bool)
	var productID any
	if v, ok := meta["productId"].(float64); ok && v > 0 {
		productID = int64(v)
	}
	e = db.WithContext(ctx).Exec(`INSERT INTO chat_engagements(conversation_id,session_id,visitor_id,product_id,source,device,returning_customer,experiment) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(session_id) DO NOTHING`, id, sessionID, visitorID, productID, source, device, returning, exp).Error
	return id, e
}

func NormalizeSearch(s string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		switch r {
		case 'أ', 'إ', 'آ', 'ٱ':
			return 'ا'
		case 'ى':
			return 'ي'
		case 'ة':
			return 'ه'
		case 'ـ':
			return -1
		}
		if r >= 'ً' && r <= 'ٟ' {
			return -1
		}
		return r
	}, strings.TrimSpace(s)))
}
