package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

type productListItem struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	SKU          string   `json:"sku"`
	Status       string   `json:"status"`
	CategoryID   *int64   `json:"categoryId"`
	CategoryName *string  `json:"categoryName"`
	Price        float64  `json:"price"`
	CompareAt    *float64 `json:"compareAt"`
	Currency     string   `json:"currency"`
	StockTotal   int      `json:"stockTotal"`
	IsFeatured   bool     `json:"isFeatured"`
	SalesCount   int      `json:"salesCount"`
	Rating       float64  `json:"rating"`
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) ListProducts(c *gin.Context) {
	var products []productListItem
	query := h.db.Model(&Product{}).
		Select("products.id, products.title, products.slug, products.sku, products.status, products.category_id, categories.name AS category_name, products.price, products.compare_at, products.currency, products.stock_total, products.is_featured, products.sales_count, products.rating").
		Joins("LEFT JOIN categories ON categories.id = products.category_id").
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

	discount := coupon.Value
	if coupon.Type == "percentage" {
		discount = (req.Subtotal * coupon.Value) / 100
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"code": coupon.Code, "discount": discount, "type": coupon.Type}})
}

type createOrderRequest struct {
	PaymentMethod string  `json:"paymentMethod"`
	Subtotal      float64 `json:"subtotal"`
	Shipping      float64 `json:"shipping"`
	Discount      float64 `json:"discount"`
	GrandTotal    float64 `json:"grandTotal"`
	CustomerName  string  `json:"customerName"`
	CustomerPhone string  `json:"customerPhone"`
	AddressRaw    string  `json:"addressRaw"`
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
	if req.PaymentMethod != "COD" && req.PaymentMethod != "Paymob" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment method"})
		return
	}

	order := Order{
		OrderNumber:   fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		Status:        "new",
		PaymentMethod: req.PaymentMethod,
		Subtotal:      req.Subtotal,
		Shipping:      req.Shipping,
		Discount:      req.Discount,
		GrandTotal:    req.GrandTotal,
		Currency:      "SAR",
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		AddressRaw:    req.AddressRaw,
	}

	if err := h.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": order})
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
	if err := c.ShouldBindJSON(&req); err != nil || req.ConversationID == 0 || strings.TrimSpace(req.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat payload"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"conversationId": req.ConversationID,
			"reply":          "Thanks for your message. AI replies will be enabled in Phase 7.",
		},
	})
}
