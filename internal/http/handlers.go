package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fahd-backend/internal/ai"
	aiagent "fahd-backend/internal/ai/agent"
	"fahd-backend/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db  *gorm.DB
	cfg config.Config
}

type productListItem struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	SKU          string   `json:"sku"`
	Status       string   `json:"status"`
	CategoryID   *int64   `json:"categoryId"`
	CategoryName *string  `json:"categoryName"`
	PrimaryImage *string  `json:"primaryImage"`
	Price        float64  `json:"price"`
	CompareAt    *float64 `json:"compareAt"`
	Currency     string   `json:"currency"`
	StockTotal   int      `json:"stockTotal"`
	IsFeatured   bool     `json:"isFeatured"`
	SalesCount   int      `json:"salesCount"`
	Rating       float64  `json:"rating"`
}

func NewHandler(db *gorm.DB, cfg config.Config) *Handler {
	return &Handler{db: db, cfg: cfg}
}

func (h *Handler) ListProducts(c *gin.Context) {
	var products []productListItem
	query := h.db.Model(&Product{}).
		Select("products.id, products.title, products.slug, products.sku, products.status, products.category_id, categories.name AS category_name, img.url AS primary_image, products.price, products.compare_at, products.currency, products.stock_total, products.is_featured, products.sales_count, products.rating").
		Joins("LEFT JOIN categories ON categories.id = products.category_id").
		Joins("LEFT JOIN LATERAL (SELECT url FROM product_images i WHERE i.product_id = products.id ORDER BY i.is_primary DESC, i.sort_order ASC, i.id ASC LIMIT 1) img ON TRUE").
		Where("status = ?", "active")

	page := 1
	limit := 20
	if v := c.Query("page"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 100 {
				parsed = 100
			}
			limit = parsed
		}
	}

	if category := strings.TrimSpace(c.Query("category")); category != "" {
		query = query.Where("categories.slug = ?", category)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		query = query.Where("products.title ILIKE ? OR products.sku ILIKE ?", like, like)
	}
	if minPrice := strings.TrimSpace(c.Query("minPrice")); minPrice != "" {
		if value, err := strconv.ParseFloat(minPrice, 64); err == nil {
			query = query.Where("price >= ?", value)
		}
	}
	if maxPrice := strings.TrimSpace(c.Query("maxPrice")); maxPrice != "" {
		if value, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			query = query.Where("price <= ?", value)
		}
	}

	sort := c.DefaultQuery("sort", "updated_desc")
	switch sort {
	case "price_asc":
		query = query.Order("price ASC")
	case "price_desc":
		query = query.Order("price DESC")
	case "title_asc":
		query = query.Order("title ASC")
	default:
		query = query.Order("products.updated_at DESC")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count products"})
		return
	}

	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load products"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{
		"data": products,
		"meta": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func (h *Handler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	var product Product
	var categoryName *string
	if err := h.db.Where("id = ?", id).First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}

	if product.CategoryID != nil {
		if err := h.db.Model(&Category{}).Select("name").Where("id = ?", *product.CategoryID).Scan(&categoryName).Error; err != nil {
			categoryName = nil
		}
	}
	var images []ProductImage
	if err := h.db.Where("product_id = ?", product.ID).Order("is_primary DESC, sort_order ASC").Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product images"})
		return
	}

	var variants []ProductVariant
	if err := h.db.Where("product_id = ?", product.ID).Order("created_at ASC").Find(&variants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product variants"})
		return
	}

	var pricingTiers []PricingTier
	if err := h.db.Where("product_id = ?", product.ID).Order("qty ASC").Find(&pricingTiers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load pricing tiers"})
		return
	}

	variantPayload := make([]gin.H, 0, len(variants))
	for _, variant := range variants {
		item := gin.H{
			"id":            variant.ID,
			"productId":     variant.ProductID,
			"sku":           variant.SKU,
			"priceOverride": variant.PriceOverride,
			"stock":         variant.Stock,
			"image":         variant.Image,
			"isActive":      variant.IsActive,
			"createdAt":     variant.CreatedAt,
			"updatedAt":     variant.UpdatedAt,
		}
		if len(variant.Attributes) > 0 {
			var attributes any
			if err := json.Unmarshal(variant.Attributes, &attributes); err == nil {
				item["attributes"] = attributes
			}
		}
		variantPayload = append(variantPayload, item)
	}

	data := gin.H{
		"id":                product.ID,
		"title":             product.Title,
		"slug":              product.Slug,
		"sku":               product.SKU,
		"status":            product.Status,
		"descriptionShort":  product.DescriptionShort,
		"descriptionLong":   product.DescriptionLong,
		"categoryId":        product.CategoryID,
		"categoryName":      categoryName,
		"cost":              product.Cost,
		"price":             product.Price,
		"compareAt":         product.CompareAt,
		"currency":          product.Currency,
		"inventoryMode":     product.InventoryMode,
		"stockTotal":        product.StockTotal,
		"lowStockThreshold": product.LowStockThreshold,
		"isFeatured":        product.IsFeatured,
		"hasVariants":       product.HasVariants,
		"usageInstructions": product.UsageInstructions,
		"salesCount":        product.SalesCount,
		"rating":            product.Rating,
		"images":            images,
		"variants":          variantPayload,
		"pricingTiers":      pricingTiers,
		"createdAt":         product.CreatedAt,
		"updatedAt":         product.UpdatedAt,
	}

	if len(product.VariantOptions) > 0 {
		var variantOptions any
		if err := json.Unmarshal(product.VariantOptions, &variantOptions); err == nil {
			data["variantOptions"] = variantOptions
		}
	}
	if len(product.Specs) > 0 {
		var specs any
		if err := json.Unmarshal(product.Specs, &specs); err == nil {
			data["specs"] = specs
		}
	}
	if len(product.FAQ) > 0 {
		var faq any
		if err := json.Unmarshal(product.FAQ, &faq); err == nil {
			data["faq"] = faq
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) ListCategories(c *gin.Context) {
	var categories []Category
	if err := h.db.Order("sort_order ASC, name ASC").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": categories})
}

func (h *Handler) ListCities(c *gin.Context) {
	var cities []City
	if err := h.db.Where("is_active = TRUE").Order("sort_order ASC, name ASC").Find(&cities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load cities"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cities})
}

type validateCouponRequest struct {
	Code     string  `json:"code"`
	Subtotal float64 `json:"subtotal"`
}

func (h *Handler) ValidateCoupon(c *gin.Context) {
	var req validateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coupon payload"})
		return
	}

	var coupon Coupon
	if err := h.db.Where("LOWER(code) = LOWER(?) AND is_active = TRUE", req.Code).First(&coupon).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "coupon not found"})
		return
	}

	if coupon.ExpiresAt != nil && coupon.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "coupon expired"})
		return
	}
	if coupon.MinOrder != nil && req.Subtotal < *coupon.MinOrder {
		c.JSON(http.StatusBadRequest, gin.H{"error": "minimum order amount not met"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"code": coupon.Code, "discount": coupon.Value, "type": coupon.Type}})
}

type createOrderRequest struct {
	PaymentMethod string             `json:"paymentMethod"`
	CustomerName  string             `json:"customerName"`
	CustomerPhone string             `json:"customerPhone"`
	CustomerEmail string             `json:"customerEmail"`
	AddressRaw    string             `json:"addressRaw"`
	CityID        int64              `json:"cityId"`
	CouponCode    string             `json:"couponCode"`
	Items         []createOrderItem  `json:"items"`
}

type createOrderItem struct {
	ProductID int64  `json:"productId"`
	VariantID *int64 `json:"variantId"`
	Qty       int    `json:"qty"`
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order payload"})
		return
	}
	if req.CustomerName == "" || req.CustomerPhone == "" || req.AddressRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer name, phone, and address are required"})
		return
	}
	if req.CityID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cityId is required"})
		return
	}
	var city City
	if err := h.db.Where("id = ? AND is_active = TRUE", req.CityID).First(&city).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cityId"})
		return
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	switch strings.ToLower(paymentMethod) {
	case "cod":
		paymentMethod = "COD"
	case "paymob":
		paymentMethod = "Paymob"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "items are required"})
		return
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

	pricedItems := make([]pricedItem, 0, len(req.Items))
	subtotal := 0.0

	for _, item := range req.Items {
		if item.ProductID <= 0 || item.Qty <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "each item must include valid productId and qty"})
			return
		}

		var product struct {
			ID    int64
			Title string
			SKU   string
			Price float64
		}
		if err := h.db.Table("products").
			Select("id, title, sku, price").
			Where("id = ? AND status = ?", item.ProductID, "active").
			First(&product).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid product: %d", item.ProductID)})
			return
		}

		unitPrice := product.Price
		if item.VariantID != nil {
			var variant struct {
				ID            int64
				ProductID     int64
				PriceOverride *float64
			}
			if err := h.db.Table("product_variants").
				Select("id, product_id, price_override").
				Where("id = ? AND product_id = ? AND is_active = TRUE", *item.VariantID, item.ProductID).
				First(&variant).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid variant for product: %d", item.ProductID)})
				return
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

	if code := strings.TrimSpace(req.CouponCode); code != "" {
		var coupon struct {
			Code      string
			Type      string
			Value     float64
			MinOrder  *float64
			ExpiresAt *time.Time
			IsActive  bool
		}
		if err := h.db.Table("coupons").
			Select("code, type, value, min_order, expires_at, is_active").
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

	order := Order{
		OrderNumber:   fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		Status:        "new",
		PaymentMethod: paymentMethod,
		Subtotal:      subtotal,
		Shipping:      shipping,
		Discount:      discount,
		GrandTotal:    grandTotal,
		Currency:      "SAR",
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		AddressRaw:    req.AddressRaw,
		CityID:        &req.CityID,
	}

	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table("orders").Create(&map[string]any{
		"order_number":      order.OrderNumber,
		"status":            order.Status,
		"payment_method":    order.PaymentMethod,
		"payment_status":    "pending",
		"subtotal":          order.Subtotal,
		"shipping":          order.Shipping,
		"discount":          order.Discount,
		"grand_total":       order.GrandTotal,
		"currency":          order.Currency,
		"coupon_code":       strings.TrimSpace(req.CouponCode),
		"customer_name":     req.CustomerName,
		"customer_phone":    req.CustomerPhone,
		"customer_email":    strings.TrimSpace(req.CustomerEmail),
		"address_raw":       req.AddressRaw,
		"city_id":           req.CityID,
		"address_confidence": 0,
	}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	var created struct{ ID int64 }
	if err := tx.Table("orders").Select("id").Where("order_number = ?", order.OrderNumber).First(&created).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read created order"})
		return
	}

	for _, item := range pricedItems {
		payload := map[string]any{
			"order_id":    created.ID,
			"product_id":  item.ProductID,
			"sku":         item.SKU,
			"title":       item.Title,
			"qty":         item.Qty,
			"unit_price":  item.UnitPrice,
			"line_total":  item.LineTotal,
		}
		if item.VariantID != nil {
			payload["variant_id"] = *item.VariantID
		}
		if err := tx.Table("order_items").Create(&payload).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order items"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to finalize order"})
		return
	}

	order.ID = created.ID

	c.JSON(http.StatusCreated, gin.H{"data": order})
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func (h *Handler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	var order Order
	if err := h.db.Where("id = ?", id).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (h *Handler) StartChat(c *gin.Context) {
	conversation := Conversation{Status: "active"}
	if err := h.db.Create(&conversation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start conversation"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"conversationId": conversation.ID}})
}

type chatMessageRequest struct {
	ConversationID int64  `json:"conversationId"`
	Message        string `json:"message"`
}

func (h *Handler) SendChatMessage(c *gin.Context) {
	var req chatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat payload"})
		return
	}

	service := aiagent.NewService(h.db, h.cfg)
	response, err := service.HandleMessage(c.Request.Context(), aiagent.MessageRequest{
		ConversationID: req.ConversationID,
		Message:        req.Message,
	})
	if err != nil {
		attrs := []any{"conversation_id", req.ConversationID, "error", err}
		message := "sales assistant is temporarily unavailable"
		var providerErr ai.ProviderError
		if errors.As(err, &providerErr) {
			attrs = append(attrs, "provider", providerErr.Provider, "provider_error_code", providerErr.Code, "status_code", providerErr.StatusCode)
			if providerErr.Code == ai.ErrorCodeProviderQuota || providerErr.Code == ai.ErrorCodeProviderRate {
				message = "sales assistant is temporarily busy. please try again later"
			}
		}
		slog.Error("sales assistant message failed", attrs...)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}
