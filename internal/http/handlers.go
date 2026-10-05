package http

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fahd-backend/internal/ai/services"
	"fahd-backend/internal/config"
	"fahd-backend/internal/session"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db        *gorm.DB
	cfg       config.Config
	wsHandler http.Handler
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

func NewHandler(db *gorm.DB, cfg config.Config, wsHandler http.Handler) *Handler {
	return &Handler{db: db, cfg: cfg, wsHandler: wsHandler}
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
		like := "%" + services.NormalizeSearch(search) + "%"
		query = query.Where("normalize_arabic(products.title || ' ' || products.sku || ' ' || COALESCE(products.description_short,'')) ILIKE ?", like)
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

	if product.CompareAt != nil && h.sessionOfferExpired(c.Query("sessionId"), product.ID) {
		data["price"] = *product.CompareAt
		data["compareAt"] = nil
		data["pricingTiers"] = []any{}
	}
	var content struct {
		VideoURL          *string
		RelatedProductIDs []byte
	}
	h.db.Table("products").Select("video_url,related_product_ids").Where("id=?", product.ID).Take(&content)
	data["videoUrl"] = content.VideoURL
	var ids []int64
	_ = json.Unmarshal(content.RelatedProductIDs, &ids)
	data["relatedProductIds"] = ids
	reviews := []struct {
		Rating      int    `json:"rating"`
		Body        string `json:"body"`
		DisplayName string `json:"displayName"`
	}{}
	h.db.Table("product_reviews").Select("rating,body,display_name").Where("product_id=?", product.ID).Order("created_at DESC").Limit(20).Find(&reviews)
	data["reviews"] = reviews
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

func (h *Handler) CreateOrder(c *gin.Context) {
	var in services.CreateOrderInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "بيانات الطلب غير صالحة"})
		return
	}
	if !h.validChatSession(in.SessionID, in.SessionToken) {
		c.JSON(401, gin.H{"error": "انتهت جلسة الشات؛ أعد فتح المحادثة"})
		return
	}
	if in.SessionID != "" {
		if _, e := services.EnsureConversation(c.Request.Context(), h.db, in.SessionID, in.VisitorID); e != nil {
			c.JSON(500, gin.H{"error": "تعذر تجهيز الطلب"})
			return
		}
	}
	result, err := services.NewOrderService(h.db).Create(c.Request.Context(), in)
	if err != nil {
		status := 400
		if strings.HasPrefix(err.Error(), "PRICE_CHANGED:") {
			status = 409
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"data": result})
}
func (h *Handler) QuoteOrder(c *gin.Context) {
	var in services.CreateOrderInput
	if c.ShouldBindJSON(&in) != nil {
		c.JSON(400, gin.H{"error": "بيانات السعر غير صالحة"})
		return
	}
	if !h.validChatSession(in.SessionID, in.SessionToken) {
		c.JSON(401, gin.H{"error": "جلسة غير صالحة"})
		return
	}
	q, e := services.NewOrderService(h.db).Quote(c.Request.Context(), in)
	if e != nil {
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, gin.H{"data": q})
}
func (h *Handler) validChatSession(id, token string) bool {
	if id == "" {
		return true
	}
	claims, e := session.NewTokenService(h.cfg.JWTSecret, time.Hour).Validate(token)
	return e == nil && claims.SessionID == id
}

type analyticsEventRequest struct {
	VisitorID    string         `json:"visitorId"`
	SessionID    string         `json:"sessionId"`
	EventType    string         `json:"eventType"`
	Path         string         `json:"path"`
	Referrer     string         `json:"referrer"`
	Metadata     map[string]any `json:"metadata"`
	SessionToken string         `json:"sessionToken"`
}

func (h *Handler) TrackAnalyticsEvent(c *gin.Context) {
	var req analyticsEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid analytics event payload"})
		return
	}
	visitorID := strings.TrimSpace(req.VisitorID)
	sessionID := strings.TrimSpace(req.SessionID)
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		eventType = "page_view"
	}
	if !isValidAnalyticsEventType(eventType) || eventType == "bot_latency" || eventType == "order_created" || strings.HasPrefix(eventType, "order_") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid eventType"})
		return
	}
	if visitorID == "" && sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "visitorId or sessionId is required"})
		return
	}
	if eventType != "page_view" && eventType != "web_vital" {
		if sessionID == "" || !h.validChatSession(sessionID, req.SessionToken) {
			c.JSON(401, gin.H{"error": "invalid session"})
			return
		}
		if _, err := services.Engage(c.Request.Context(), h.db, sessionID, visitorID, req.Metadata); err != nil {
			c.JSON(500, gin.H{"error": "tracking unavailable"})
			return
		}
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		path = c.Request.URL.Path
	}
	referrer := strings.TrimSpace(req.Referrer)
	if referrer == "" {
		referrer = c.Request.Referer()
	}
	var visitorPtr *string
	if visitorID != "" {
		visitorPtr = &visitorID
	}
	var sessionPtr *string
	if sessionID != "" {
		sessionPtr = &sessionID
	}
	if err := recordAnalyticsEvent(h.db, analyticsEventInput{VisitorID: visitorPtr, SessionID: sessionPtr, EventType: eventType, Path: path, Referrer: referrer, UserAgent: c.Request.UserAgent(), Metadata: req.Metadata}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save analytics event"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"tracked": true}})
}

type analyticsEventInput struct {
	VisitorID *string
	SessionID *string
	EventType string
	Path      string
	Referrer  string
	UserAgent string
	Metadata  map[string]any
}

func recordAnalyticsEvent(db *gorm.DB, input analyticsEventInput) error {
	if !isValidAnalyticsEventType(input.EventType) {
		return nil
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, _ := json.Marshal(metadata)
	row := map[string]any{
		"visitor_id": input.VisitorID,
		"session_id": input.SessionID,
		"event_type": input.EventType,
		"path":       strings.TrimSpace(input.Path),
		"referrer":   strings.TrimSpace(input.Referrer),
		"user_agent": strings.TrimSpace(input.UserAgent),
		"metadata":   string(encoded),
	}
	return db.Table("analytics_events").Create(&row).Error
}

func isValidAnalyticsEventType(eventType string) bool {
	switch eventType {
	case "page_view", "chat_started", "order_created", "chat_engaged", "offer_viewed", "offer_selected", "checkout_started", "checkout_error", "complement_clicked", "web_vital", "bot_latency":
		return true
	default:
		return false
	}
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func normalizedOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func generateUniqueOrderNumber(db *gorm.DB) (string, error) {
	for i := 0; i < 10; i++ {
		orderNumber, err := randomOrderNumber()
		if err != nil {
			return "", err
		}
		var count int64
		if err := db.Table("orders").Where("order_number = ?", orderNumber).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return orderNumber, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique order number")
}

func randomOrderNumber() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	code := make([]byte, 4)
	for i := range code {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		code[i] = alphabet[idx.Int64()]
	}
	return "ORD-" + string(code), nil
}

func (h *Handler) GetOrder(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	data, e := services.NewOrderService(h.db).Track(c.Request.Context(), id, c.GetHeader("X-Order-Token"))
	if e != nil {
		c.JSON(404, gin.H{"error": "رابط التتبع غير صالح"})
		return
	}
	c.JSON(200, gin.H{"data": data})
}
