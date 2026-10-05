package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

type OrderItemInput struct {
	ProductID int64  `json:"productId"`
	VariantID *int64 `json:"variantId,omitempty"`
	Qty       int    `json:"qty"`
}

type CreateOrderInput struct {
	Items           []OrderItemInput `json:"items"`
	ConversationID  int64            `json:"conversationId"`
	VisitorID       string           `json:"visitorId"`
	SessionID       string           `json:"sessionId"`
	CustomerName    string           `json:"customerName"`
	CustomerPhone   string           `json:"customerPhone"`
	CustomerEmail   string           `json:"customerEmail"`
	AddressRaw      string           `json:"addressRaw"`
	AddressZone     *string          `json:"addressZone"`
	AddressDistrict *string          `json:"addressDistrict"`
	CityID          int64            `json:"cityId"`
	PaymentMethod   string           `json:"paymentMethod"`
	CouponCode      string           `json:"couponCode"`
	QuoteID         string           `json:"quoteId"`
	IdempotencyKey  string           `json:"idempotencyKey"`
	SessionToken    string           `json:"sessionToken"`
}

type OrderService struct {
	db *gorm.DB
}

func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

func (s *OrderService) Create(ctx context.Context, in CreateOrderInput) (map[string]any, error) {
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	in.CustomerPhone = NormalizePhone(in.CustomerPhone)
	in.AddressRaw = strings.TrimSpace(in.AddressRaw)
	if in.CustomerName == "" || in.AddressRaw == "" || !saudiPhone.MatchString(in.CustomerPhone) || in.CityID <= 0 {
		return nil, fmt.Errorf("راجع الاسم ورقم الجوال السعودي والعنوان والمدينة")
	}
	if strings.ToLower(in.PaymentMethod) != "cod" {
		return nil, fmt.Errorf("الدفع عند الاستلام هو المتاح حاليًا")
	}
	if _, e := uuid.Parse(in.IdempotencyKey); e != nil {
		return nil, fmt.Errorf("معرّف الطلب مطلوب")
	}
	// Stable across refreshing a quote after a network error.
	request := in
	request.QuoteID = ""
	request.SessionToken = ""
	request.ConversationID = 0
	hash := digest(request)
	var result map[string]any
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "order:"+in.IdempotencyKey).Error; e != nil {
			return e
		}
		var existing struct {
			ID                         int64
			RequestHash, TrackingToken string
		}
		r := tx.Table("orders").Where("idempotency_key=?", in.IdempotencyKey).Limit(1).Find(&existing)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected > 0 {
			if existing.RequestHash != hash {
				return fmt.Errorf("تم استخدام معرّف الطلب لبيانات أخرى")
			}
			var e error
			result, e = publicOrder(tx, existing.ID, existing.TrackingToken)
			return e
		}
		var quoted quoteRow
		if e := tx.Table("checkout_quotes").Where("id=? AND session_id=?", in.QuoteID, in.SessionID).Take(&quoted).Error; e != nil {
			return fmt.Errorf("راجع ملخص السعر قبل إرسال الطلب")
		}
		if quoted.RequestHash != quoteRequestHash(in) || !time.Now().Before(quoted.ExpiresAt) {
			return fmt.Errorf("PRICE_CHANGED: انتهت صلاحية السعر؛ راجع السعر الجديد ثم أكد الطلب")
		}
		q, e := s.price(ctx, tx, in, true)
		if e != nil {
			return e
		}
		if quoted.PricingHash != pricingHash(q) {
			return fmt.Errorf("PRICE_CHANGED: تغير السعر؛ راجع الملخص ثم أكد الطلب")
		}
		var conv any
		if in.SessionID != "" {
			var id int64
			if e := tx.Table("conversations").Select("id").Where("session_id=?", in.SessionID).Order("id DESC").Limit(1).Scan(&id).Error; e != nil {
				return e
			}
			if id > 0 {
				conv = id
			}
		}
		tracking := uuid.NewString() + uuid.NewString()
		number := "ORD-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))[:12]
		row := map[string]any{"order_number": number, "status": "new", "payment_method": "cod", "payment_status": "pending", "subtotal": q.Subtotal, "shipping": q.Shipping, "discount": q.Discount, "grand_total": q.GrandTotal, "currency": "SAR", "coupon_code": strings.TrimSpace(in.CouponCode), "conversation_id": conv, "customer_name": in.CustomerName, "customer_phone": in.CustomerPhone, "customer_email": in.CustomerEmail, "visitor_id": in.VisitorID, "session_id": in.SessionID, "address_raw": in.AddressRaw, "city_id": in.CityID, "address_zone": in.AddressZone, "address_district": in.AddressDistrict, "idempotency_key": in.IdempotencyKey, "request_hash": hash, "tracking_token": tracking, "stock_reserved": true}
		if e := tx.Table("orders").Create(row).Error; e != nil {
			return e
		}
		var id int64
		if e := tx.Table("orders").Select("id").Where("order_number=?", number).Scan(&id).Error; e != nil {
			return e
		}
		for _, it := range q.Items {
			if e := tx.Table("order_items").Create(map[string]any{"order_id": id, "product_id": it.ProductID, "variant_id": it.VariantID, "title": it.Title, "sku": it.SKU, "qty": it.Qty, "unit_price": it.UnitPrice, "line_total": it.LineTotal}).Error; e != nil {
				return e
			}
		}
		meta, _ := json.Marshal(map[string]any{"orderId": id, "conversationId": conv})
		if e := tx.Table("analytics_events").Create(map[string]any{"event_type": "order_created", "session_id": in.SessionID, "visitor_id": in.VisitorID, "metadata": string(meta)}).Error; e != nil {
			return e
		}
		result, e = publicOrder(tx, id, tracking)
		return e
	})
	return result, err
}
func publicOrder(db *gorm.DB, id int64, token string) (map[string]any, error) {
	var o struct {
		ID                                                                                    int64
		OrderNumber, Status, PaymentMethod, Currency, CustomerName, CustomerPhone, AddressRaw string
		CityID                                                                                int64
		Subtotal, Shipping, Discount, GrandTotal                                              float64
		CreatedAt                                                                             time.Time
	}
	if e := db.Table("orders").Where("id=?", id).Take(&o).Error; e != nil {
		return nil, e
	}
	var city string
	db.Table("cities").Select("name").Where("id=?", o.CityID).Scan(&city)
	var deliveryEstimate string
	db.Table("cities").Select("COALESCE(delivery_estimate,'')").Where("id=?", o.CityID).Scan(&deliveryEstimate)
	items := []PricedItem{}
	if e := db.Table("order_items").Where("order_id=?", id).Order("id").Find(&items).Error; e != nil {
		return nil, e
	}
	return map[string]any{"id": o.ID, "orderId": o.ID, "orderNumber": o.OrderNumber, "status": o.Status, "createdAt": o.CreatedAt, "paymentMethod": o.PaymentMethod, "deliveryEstimate": deliveryEstimate, "grandTotal": o.GrandTotal, "trackingToken": token, "totals": map[string]any{"subtotal": o.Subtotal, "shipping": o.Shipping, "discount": o.Discount, "grandTotal": o.GrandTotal, "currency": o.Currency}, "customer": map[string]any{"name": o.CustomerName, "phone": o.CustomerPhone}, "address": map[string]any{"raw": o.AddressRaw, "city": city}, "items": items}, nil
}
func (s *OrderService) Track(ctx context.Context, id int64, token string) (map[string]any, error) {
	var stored string
	if token == "" {
		return nil, fmt.Errorf("رابط التتبع غير صالح")
	}
	if e := s.db.WithContext(ctx).Table("orders").Select("tracking_token").Where("id=?", id).Scan(&stored).Error; e != nil {
		return nil, e
	}
	if stored == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(token)) != 1 {
		return nil, fmt.Errorf("رابط التتبع غير صالح")
	}
	return publicOrder(s.db.WithContext(ctx), id, token)
}

func (s *OrderService) Lookup(ctx context.Context, orderID int64, orderNumber string, customerPhone string) (map[string]any, error) {
	db := s.db.WithContext(ctx).Table("orders")
	if orderID > 0 {
		db = db.Where("id = ?", orderID)
	} else if orderNumber != "" {
		db = db.Where("order_number = ?", orderNumber)
	} else {
		return nil, fmt.Errorf("orderId or orderNumber is required")
	}

	var order struct {
		ID            int64
		OrderNumber   string
		Status        string
		PaymentMethod string
		Subtotal      float64
		Shipping      float64
		Discount      float64
		GrandTotal    float64
		Currency      string
		CustomerName  string
		CustomerPhone string
		CreatedAt     time.Time
	}
	if err := db.Select("id, order_number, status, payment_method, subtotal, shipping, discount, grand_total, currency, customer_name, customer_phone, created_at").
		First(&order).Error; err != nil {
		return nil, fmt.Errorf("order not found")
	}

	if customerPhone == "" {
		return nil, fmt.Errorf("رقم الجوال مطلوب")
	}
	if customerPhone != "" {
		normalizedStored := NormalizePhone(order.CustomerPhone)
		normalizedInput := NormalizePhone(customerPhone)
		if !strings.EqualFold(normalizedStored, normalizedInput) {
			return nil, fmt.Errorf("customer phone does not match this order")
		}
	}

	return map[string]any{
		"orderId":       order.ID,
		"orderNumber":   order.OrderNumber,
		"status":        order.Status,
		"paymentMethod": order.PaymentMethod,
		"subtotal":      order.Subtotal,
		"shipping":      order.Shipping,
		"discount":      order.Discount,
		"grandTotal":    order.GrandTotal,
		"currency":      order.Currency,
		"customerName":  order.CustomerName,
		"customerPhone": order.CustomerPhone,
		"createdAt":     order.CreatedAt.Format(time.RFC3339),
	}, nil
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
