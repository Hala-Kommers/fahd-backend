package services

import (
	"context"
	"fmt"
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
	Items         []OrderItemInput `json:"items"`
	CustomerName  string           `json:"customerName"`
	CustomerPhone string           `json:"customerPhone"`
	CustomerEmail string           `json:"customerEmail"`
	AddressRaw    string           `json:"addressRaw"`
	CityID        int64            `json:"cityId"`
	PaymentMethod string           `json:"paymentMethod"`
	CouponCode    string           `json:"couponCode"`
}

type OrderService struct {
	db *gorm.DB
}

func NewOrderService(db *gorm.DB) *OrderService {
	return &OrderService{db: db}
}

func (s *OrderService) Create(ctx context.Context, input CreateOrderInput) (map[string]any, error) {
	if len(input.Items) == 0 {
		return nil, fmt.Errorf("items are required")
	}
	if input.CustomerName == "" || input.CustomerPhone == "" || input.AddressRaw == "" {
		return nil, fmt.Errorf("customer name, phone, and address are required")
	}
	if input.CityID <= 0 {
		return nil, fmt.Errorf("cityId is required")
	}
	rawPayment := strings.TrimSpace(input.PaymentMethod)
	paymentMethod := strings.ToLower(rawPayment)
	switch paymentMethod {
	case "cod", "paymob":
	default:
		return nil, fmt.Errorf("invalid payment method, must be 'cod' or 'paymob'")
	}

	var city struct{ Name string }
	if err := s.db.WithContext(ctx).Table("cities").
		Select("name").
		Where("id = ? AND is_active = TRUE", input.CityID).
		First(&city).Error; err != nil {
		return nil, fmt.Errorf("invalid cityId: city not found or inactive")
	}

	type pricedItem struct {
		ProductID int64
		VariantID *int64
		Title     string
		SKU       string
		Qty       int
		UnitPrice float64
		LineTotal float64
	}
	pricedItems := make([]pricedItem, 0, len(input.Items))
	subtotal := 0.0

	for _, item := range input.Items {
		if item.ProductID <= 0 || item.Qty <= 0 {
			return nil, fmt.Errorf("each item must include valid productId and qty")
		}
		var product struct {
			ID    int64
			Title string
			SKU   string
			Price float64
		}
		if err := s.db.WithContext(ctx).Table("products").
			Select("id, title, sku, price").
			Where("id = ? AND status = ?", item.ProductID, "active").
			First(&product).Error; err != nil {
			return nil, fmt.Errorf("invalid product: %d", item.ProductID)
		}
		unitPrice := product.Price
		if item.VariantID != nil {
			var variant struct {
				ID            int64
				PriceOverride *float64
			}
			if err := s.db.WithContext(ctx).Table("product_variants").
				Select("id, price_override").
				Where("id = ? AND product_id = ? AND is_active = TRUE", *item.VariantID, item.ProductID).
				First(&variant).Error; err != nil {
				return nil, fmt.Errorf("invalid variant for product: %d", item.ProductID)
			}
			if variant.PriceOverride != nil {
				unitPrice = *variant.PriceOverride
			}
		}
		lineTotal := roundMoney(unitPrice * float64(item.Qty))
		subtotal += lineTotal
		pricedItems = append(pricedItems, pricedItem{
			ProductID: item.ProductID,
			VariantID: item.VariantID,
			Title:     product.Title,
			SKU:       product.SKU,
			Qty:       item.Qty,
			UnitPrice: roundMoney(unitPrice),
			LineTotal: lineTotal,
		})
	}

	subtotal = roundMoney(subtotal)
	shipping := 0.0
	discount := 0.0

	if code := strings.TrimSpace(input.CouponCode); code != "" {
		var coupon struct {
			Type      string
			Value     float64
			MinOrder  *float64
			ExpiresAt *time.Time
		}
		if err := s.db.WithContext(ctx).Table("coupons").
			Select("type, value, min_order, expires_at").
			Where("LOWER(code) = LOWER(?) AND is_active = TRUE", code).
			First(&coupon).Error; err == nil {
			valid := true
			if coupon.ExpiresAt != nil && coupon.ExpiresAt.Before(time.Now()) {
				valid = false
			}
			if coupon.MinOrder != nil && subtotal < *coupon.MinOrder {
				valid = false
			}
			if valid {
				if coupon.Type == "percentage" {
					discount = roundMoney((subtotal * coupon.Value) / 100)
				} else {
					discount = roundMoney(coupon.Value)
				}
			}
		}
	}
	if discount > subtotal {
		discount = subtotal
	}
	grandTotal := roundMoney(subtotal + shipping - discount)

	orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano())

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("begin transaction: %w", tx.Error)
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	var orderID int64
	if err := tx.Table("orders").Create(&map[string]any{
		"order_number":      orderNumber,
		"status":            "new",
		"payment_method":    paymentMethod,
		"payment_status":    "pending",
		"subtotal":          subtotal,
		"shipping":          shipping,
		"discount":          discount,
		"grand_total":       grandTotal,
		"currency":          "SAR",
		"coupon_code":       strings.TrimSpace(input.CouponCode),
		"customer_name":     input.CustomerName,
		"customer_phone":    input.CustomerPhone,
		"customer_email":    strings.TrimSpace(input.CustomerEmail),
		"address_raw":       input.AddressRaw,
		"city_id":           input.CityID,
		"address_confidence": 0,
	}).Error; err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := tx.Table("orders").Select("id").Where("order_number = ?", orderNumber).Scan(&orderID).Error; err != nil {
		return nil, fmt.Errorf("read order id: %w", err)
	}

	for _, item := range pricedItems {
		payload := map[string]any{
			"order_id":   orderID,
			"product_id": item.ProductID,
			"sku":        item.SKU,
			"title":      item.Title,
			"qty":        item.Qty,
			"unit_price": item.UnitPrice,
			"line_total": item.LineTotal,
		}
		if item.VariantID != nil {
			payload["variant_id"] = *item.VariantID
		}
		if err := tx.Table("order_items").Create(&payload).Error; err != nil {
			return nil, fmt.Errorf("create order item: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("commit order: %w", err)
	}
	committed = true

	itemsJSON := make([]map[string]any, 0, len(pricedItems))
	for _, item := range pricedItems {
		itemsJSON = append(itemsJSON, map[string]any{
			"productId": item.ProductID,
			"title":     item.Title,
			"sku":       item.SKU,
			"qty":       item.Qty,
			"unitPrice": item.UnitPrice,
			"lineTotal": item.LineTotal,
		})
	}

	return map[string]any{
		"orderId":     orderID,
		"orderNumber": orderNumber,
		"status":      "new",
		"items":       itemsJSON,
		"subtotal":    subtotal,
		"shipping":    shipping,
		"discount":    discount,
		"grandTotal":  grandTotal,
		"currency":    "SAR",
		"paymentMethod": paymentMethod,
		"customerName":  input.CustomerName,
		"customerPhone": input.CustomerPhone,
		"city":          city.Name,
	}, nil
}

func (s *OrderService) CalculateTotal(ctx context.Context, input CreateOrderInput) (map[string]any, error) {
	if len(input.Items) == 0 {
		return nil, fmt.Errorf("items are required")
	}

	type pricedItem struct {
		ProductID int64
		VariantID *int64
		Title     string
		SKU       string
		Qty       int
		UnitPrice float64
		LineTotal float64
	}
	pricedItems := make([]pricedItem, 0, len(input.Items))
	subtotal := 0.0

	for _, item := range input.Items {
		if item.ProductID <= 0 || item.Qty <= 0 {
			return nil, fmt.Errorf("each item must include valid productId and qty")
		}
		var product struct {
			ID    int64
			Title string
			SKU   string
			Price float64
		}
		if err := s.db.WithContext(ctx).Table("products").
			Select("id, title, sku, price").
			Where("id = ? AND status = ?", item.ProductID, "active").
			First(&product).Error; err != nil {
			return nil, fmt.Errorf("invalid product: %d", item.ProductID)
		}
		unitPrice := product.Price
		if item.VariantID != nil {
			var variant struct {
				ID            int64
				PriceOverride *float64
			}
			if err := s.db.WithContext(ctx).Table("product_variants").
				Select("id, price_override").
				Where("id = ? AND product_id = ? AND is_active = TRUE", *item.VariantID, item.ProductID).
				First(&variant).Error; err != nil {
				return nil, fmt.Errorf("invalid variant for product: %d", item.ProductID)
			}
			if variant.PriceOverride != nil {
				unitPrice = *variant.PriceOverride
			}
		}
		lineTotal := roundMoney(unitPrice * float64(item.Qty))
		subtotal += lineTotal
		pricedItems = append(pricedItems, pricedItem{
			ProductID: item.ProductID,
			VariantID: item.VariantID,
			Title:     product.Title,
			SKU:       product.SKU,
			Qty:       item.Qty,
			UnitPrice: roundMoney(unitPrice),
			LineTotal: lineTotal,
		})
	}

	subtotal = roundMoney(subtotal)
	shipping := 0.0
	discount := 0.0

	if code := strings.TrimSpace(input.CouponCode); code != "" {
		var coupon struct {
			Type      string
			Value     float64
			MinOrder  *float64
			ExpiresAt *time.Time
		}
		if err := s.db.WithContext(ctx).Table("coupons").
			Select("type, value, min_order, expires_at").
			Where("LOWER(code) = LOWER(?) AND is_active = TRUE", code).
			First(&coupon).Error; err == nil {
			valid := true
			if coupon.ExpiresAt != nil && coupon.ExpiresAt.Before(time.Now()) {
				valid = false
			}
			if coupon.MinOrder != nil && subtotal < *coupon.MinOrder {
				valid = false
			}
			if valid {
				if coupon.Type == "percentage" {
					discount = roundMoney((subtotal * coupon.Value) / 100)
				} else {
					discount = roundMoney(coupon.Value)
				}
			}
		}
	}
	if discount > subtotal {
		discount = subtotal
	}
	grandTotal := roundMoney(subtotal + shipping - discount)

	itemsJSON := make([]map[string]any, 0, len(pricedItems))
	for _, item := range pricedItems {
		itemsJSON = append(itemsJSON, map[string]any{
			"productId": item.ProductID,
			"title":     item.Title,
			"sku":       item.SKU,
			"qty":       item.Qty,
			"unitPrice": item.UnitPrice,
			"lineTotal": item.LineTotal,
		})
	}

	return map[string]any{
		"items":      itemsJSON,
		"subtotal":   subtotal,
		"shipping":   shipping,
		"discount":   discount,
		"grandTotal": grandTotal,
		"currency":   "SAR",
		"couponCode": strings.TrimSpace(input.CouponCode),
	}, nil
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

	if customerPhone != "" {
		normalizedStored := strings.TrimSpace(order.CustomerPhone)
		normalizedInput := strings.TrimSpace(customerPhone)
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
