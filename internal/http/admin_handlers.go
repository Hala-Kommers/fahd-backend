package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fahd-backend/internal/ai"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminProductUpsertRequest struct {
	Title            string  `json:"title"`
	Slug             string  `json:"slug"`
	SKU              string  `json:"sku"`
	Status           string  `json:"status"`
	Category         struct {
		ID *int64 `json:"id"`
	} `json:"category"`
	DescriptionShort string  `json:"descriptionShort"`
	DescriptionLong  string  `json:"descriptionLong"`
	Pricing          struct {
		Cost      *float64 `json:"cost"`
		Price     float64  `json:"price"`
		CompareAt *float64 `json:"compareAt"`
		Currency  string   `json:"currency"`
	} `json:"pricing"`
	Inventory struct {
		Mode              string `json:"mode"`
		StockTotal        int    `json:"stockTotal"`
		LowStockThreshold int    `json:"lowStockThreshold"`
	} `json:"inventory"`
	IsFeatured        bool                   `json:"isFeatured"`
	HasVariants       bool                   `json:"hasVariants"`
	VariantOptions    []any                  `json:"variantOptions"`
	Specs             []map[string]any       `json:"specs"`
	FAQ               []map[string]any       `json:"faq"`
	UsageInstructions *string                `json:"usageInstructions"`
	PricingTiers      []adminPricingTierReq  `json:"pricingTiers"`
	Images            []adminImageReq        `json:"images"`
	Variants          []adminVariantReq      `json:"variants"`
}

type adminPricingTierReq struct {
	Qty           int      `json:"qty"`
	Label         *string  `json:"label"`
	OriginalPrice float64  `json:"originalPrice"`
	FinalPrice    float64  `json:"finalPrice"`
}

type adminImageReq struct {
	URL       string `json:"url"`
	IsPrimary bool   `json:"isPrimary"`
	SortOrder int    `json:"sortOrder"`
}

type adminVariantReq struct {
	SKU           string         `json:"sku"`
	Attributes    map[string]any `json:"attributes"`
	PriceOverride *float64       `json:"priceOverride"`
	Stock         int            `json:"stock"`
	Image         *string        `json:"image"`
	IsActive      *bool          `json:"isActive"`
}

func (h *Handler) AdminListProducts(c *gin.Context) {
	type adminProductListItem struct {
		ID            int64    `json:"id"`
		Title         string   `json:"title"`
		IsActive      bool     `json:"isActive"`
		CategoryName  *string  `json:"categoryName"`
		SKU           string   `json:"sku"`
		Price         float64  `json:"price"`
		StockTotal    int      `json:"stockTotal"`
		PrimaryImage  *string  `json:"primaryImage"`
	}

	var rows []adminProductListItem
	q := h.db.Table("products p").
		Select("p.id, p.title, (p.status = 'active') as is_active, c.name as category_name, p.sku, p.price, p.stock_total, img.url as primary_image").
		Joins("LEFT JOIN categories c ON c.id = p.category_id").
		Joins("LEFT JOIN LATERAL (SELECT url FROM product_images i WHERE i.product_id = p.id ORDER BY i.is_primary DESC, i.sort_order ASC, i.id ASC LIMIT 1) img ON TRUE").
		Order("p.updated_at DESC")

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

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q = q.Where("p.status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count admin products"})
		return
	}

	offset := (page - 1) * limit
	if err := q.Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load admin products"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{"data": rows, "meta": gin.H{"page": page, "limit": limit, "total": total, "totalPages": totalPages}})
}

func (h *Handler) AdminGetProduct(c *gin.Context) {
	id := c.Param("id")

	var product Product
	if err := h.db.Where("id = ?", id).First(&product).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load product"})
		return
	}

	var category struct {
		ID   int64   `json:"id"`
		Name *string `json:"name"`
	}
	if product.CategoryID != nil {
		_ = h.db.Table("categories").Select("id, name").Where("id = ?", *product.CategoryID).Take(&category).Error
	}

	var images []ProductImage
	if err := h.db.Where("product_id = ?", product.ID).Order("is_primary DESC, sort_order ASC, id ASC").Find(&images).Error; err != nil {
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
			var attrs any
			if err := json.Unmarshal(variant.Attributes, &attrs); err == nil {
				item["attributes"] = attrs
			}
		}
		variantPayload = append(variantPayload, item)
	}

	resp := gin.H{
		"id":               product.ID,
		"title":            product.Title,
		"slug":             product.Slug,
		"sku":              product.SKU,
		"status":           product.Status,
		"isActive":         product.Status == "active",
		"category":         gin.H{"id": product.CategoryID, "name": category.Name},
		"descriptionShort": product.DescriptionShort,
		"descriptionLong":  product.DescriptionLong,
		"pricing": gin.H{
			"cost":      product.Cost,
			"price":     product.Price,
			"compareAt": product.CompareAt,
			"currency":  product.Currency,
		},
		"pricingTiers": pricingTiers,
		"images":       images,
		"inventory": gin.H{
			"mode":              product.InventoryMode,
			"stockTotal":        product.StockTotal,
			"lowStockThreshold": product.LowStockThreshold,
		},
		"hasVariants":       product.HasVariants,
		"variants":          variantPayload,
		"usageInstructions": product.UsageInstructions,
		"salesCount":        product.SalesCount,
		"rating":            product.Rating,
		"isFeatured":        product.IsFeatured,
		"createdAt":         product.CreatedAt,
		"updatedAt":         product.UpdatedAt,
	}

	if len(product.VariantOptions) > 0 {
		var v any
		if err := json.Unmarshal(product.VariantOptions, &v); err == nil {
			resp["variantOptions"] = v
		}
	}
	if len(product.Specs) > 0 {
		var v any
		if err := json.Unmarshal(product.Specs, &v); err == nil {
			resp["specs"] = v
		}
	}
	if len(product.FAQ) > 0 {
		var v any
		if err := json.Unmarshal(product.FAQ, &v); err == nil {
			resp["faq"] = v
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (h *Handler) AdminCreateProduct(c *gin.Context) {
	var req adminProductUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product payload"})
		return
	}
	if err := validateAdminProductUpsert(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
		return
	}

	prod := map[string]any{}
	applyProductBaseMap(&prod, req)
	if err := tx.Table("products").Create(&prod).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create product"})
		return
	}

	var created struct{ ID int64 }
	if err := tx.Table("products").Select("id").Where("sku = ?", req.SKU).First(&created).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read created product"})
		return
	}

	if err := upsertProductChildren(tx, created.ID, req); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create product"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": created.ID}})
}

func (h *Handler) AdminUpdateProduct(c *gin.Context) {
	id := c.Param("id")
	productID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}

	var req adminProductUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product payload"})
		return
	}
	if err := validateAdminProductUpsert(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := h.db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
		return
	}

	base := map[string]any{}
	applyProductBaseMap(&base, req)
	if err := tx.Table("products").Where("id = ?", productID).Updates(base).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}

	if err := upsertProductChildren(tx, productID, req); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": productID}})
}

func (h *Handler) AdminDeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Table("products").Where("id = ?", id).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete product"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "deleted": true}})
}

func validateAdminProductUpsert(req adminProductUpsertRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.SKU) == "" {
		return fmt.Errorf("sku is required")
	}
	if req.Pricing.Price <= 0 {
		return fmt.Errorf("pricing.price must be greater than 0")
	}
	if req.Inventory.StockTotal < 0 {
		return fmt.Errorf("inventory.stockTotal cannot be negative")
	}
	if req.Inventory.LowStockThreshold < 0 {
		return fmt.Errorf("inventory.lowStockThreshold cannot be negative")
	}
	for _, item := range req.PricingTiers {
		if item.Qty <= 0 {
			return fmt.Errorf("pricingTiers.qty must be greater than 0")
		}
	}
	for _, image := range req.Images {
		if strings.TrimSpace(image.URL) == "" {
			return fmt.Errorf("images.url is required")
		}
	}
	for _, variant := range req.Variants {
		if strings.TrimSpace(variant.SKU) == "" {
			return fmt.Errorf("variants.sku is required")
		}
		if variant.Stock < 0 {
			return fmt.Errorf("variants.stock cannot be negative")
		}
	}
	return nil
}

func applyProductBaseMap(base *map[string]any, req adminProductUpsertRequest) {
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "draft"
	}
	currency := strings.TrimSpace(req.Pricing.Currency)
	if currency == "" {
		currency = "SAR"
	}
	inventoryMode := strings.TrimSpace(req.Inventory.Mode)
	if inventoryMode == "" {
		inventoryMode = "global"
	}

	variantOptionsJSON, _ := json.Marshal(req.VariantOptions)
	specsJSON, _ := json.Marshal(req.Specs)
	faqJSON, _ := json.Marshal(req.FAQ)

	*base = map[string]any{
		"title":               strings.TrimSpace(req.Title),
		"slug":                strings.TrimSpace(req.Slug),
		"sku":                 strings.TrimSpace(req.SKU),
		"status":              status,
		"category_id":         req.Category.ID,
		"description_short":   strings.TrimSpace(req.DescriptionShort),
		"description_long":    strings.TrimSpace(req.DescriptionLong),
		"cost":                req.Pricing.Cost,
		"price":               req.Pricing.Price,
		"compare_at":          req.Pricing.CompareAt,
		"currency":            currency,
		"inventory_mode":      inventoryMode,
		"stock_total":         req.Inventory.StockTotal,
		"low_stock_threshold": req.Inventory.LowStockThreshold,
		"is_featured":         req.IsFeatured,
		"has_variants":        req.HasVariants,
		"variant_options":     variantOptionsJSON,
		"specs":               specsJSON,
		"faq":                 faqJSON,
		"usage_instructions":  req.UsageInstructions,
	}
}

func upsertProductChildren(tx *gorm.DB, productID int64, req adminProductUpsertRequest) error {
	if err := tx.Table("pricing_tiers").Where("product_id = ?", productID).Delete(nil).Error; err != nil {
		return fmt.Errorf("failed to replace pricing tiers")
	}
	for _, tier := range req.PricingTiers {
		row := map[string]any{
			"product_id":      productID,
			"qty":             tier.Qty,
			"label":           tier.Label,
			"original_price":  tier.OriginalPrice,
			"final_price":     tier.FinalPrice,
		}
		if err := tx.Table("pricing_tiers").Create(&row).Error; err != nil {
			return fmt.Errorf("failed to save pricing tiers")
		}
	}

	if err := tx.Table("product_images").Where("product_id = ?", productID).Delete(nil).Error; err != nil {
		return fmt.Errorf("failed to replace images")
	}
	for i, image := range req.Images {
		sortOrder := image.SortOrder
		if sortOrder == 0 {
			sortOrder = i
		}
		row := map[string]any{
			"product_id":  productID,
			"url":         strings.TrimSpace(image.URL),
			"is_primary":  image.IsPrimary,
			"sort_order":  sortOrder,
		}
		if err := tx.Table("product_images").Create(&row).Error; err != nil {
			return fmt.Errorf("failed to save images")
		}
	}

	if err := tx.Table("product_variants").Where("product_id = ?", productID).Delete(nil).Error; err != nil {
		return fmt.Errorf("failed to replace variants")
	}
	for _, variant := range req.Variants {
		attributesJSON, _ := json.Marshal(variant.Attributes)
		isActive := true
		if variant.IsActive != nil {
			isActive = *variant.IsActive
		}
		row := map[string]any{
			"product_id":      productID,
			"sku":             strings.TrimSpace(variant.SKU),
			"attributes":      attributesJSON,
			"price_override":  variant.PriceOverride,
			"stock":           variant.Stock,
			"image":           variant.Image,
			"is_active":       isActive,
		}
		if err := tx.Table("product_variants").Create(&row).Error; err != nil {
			return fmt.Errorf("failed to save variants")
		}
	}

	return nil
}

func (h *Handler) AdminListOrders(c *gin.Context) {
	type adminOrderListItem struct {
		ID                int64   `json:"id"`
		OrderNumber       string  `json:"orderNumber"`
		CreatedAt         time.Time `json:"createdAt"`
		CustomerName      string  `json:"customerName"`
		CustomerPhone     string  `json:"customerPhone"`
		AddressCity       string  `json:"addressCity"`
		Total             float64 `json:"total"`
		PaymentMethod     string  `json:"paymentMethod"`
		Status            string  `json:"status"`
		AddressConfidence float64 `json:"confidence"`
		RiskScore         float64 `json:"risk"`
	}

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

	var rows []adminOrderListItem
	q := h.db.Table("orders o").
		Select("o.id, o.order_number, o.created_at, o.customer_name, o.customer_phone, c.name AS address_city, o.grand_total AS total, o.payment_method, o.status, o.address_confidence, o.risk_score").
		Joins("LEFT JOIN cities c ON c.id = o.city_id").
		Order("created_at DESC")

	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q = q.Where("o.status = ?", status)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		q = q.Where("o.order_number ILIKE ? OR o.customer_name ILIKE ? OR o.customer_phone ILIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count orders"})
		return
	}

	offset := (page - 1) * limit
	if err := q.Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load orders"})
		return
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{"data": rows, "meta": gin.H{"page": page, "limit": limit, "total": total, "totalPages": totalPages}})
}

func (h *Handler) AdminGetOrder(c *gin.Context) {
	id := c.Param("id")
	var order map[string]any
	if err := h.db.Table("orders").Where("id = ?", id).Take(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load order"})
		return
	}
	var items []map[string]any
	_ = h.db.Table("order_items").Where("order_id = ?", id).Find(&items).Error
	order["items"] = items
	c.JSON(http.StatusOK, gin.H{"data": order})
}

func (h *Handler) AdminUpdateOrder(c *gin.Context) {
	id := c.Param("id")
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order payload"})
		return
	}
	delete(payload, "id")
	if err := h.db.Table("orders").Where("id = ?", id).Updates(payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *Handler) AdminListCoupons(c *gin.Context) {
	var rows []map[string]any
	if err := h.db.Table("coupons").Order("created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load coupons"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *Handler) AdminCreateCoupon(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coupon payload"})
		return
	}
	if strings.TrimSpace(toString(payload["code"])) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	if err := h.db.Table("coupons").Create(&payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create coupon"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": payload})
}

func (h *Handler) AdminUpdateCoupon(c *gin.Context) {
	code := c.Param("code")
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid coupon payload"})
		return
	}
	delete(payload, "code")
	if err := h.db.Table("coupons").Where("LOWER(code) = LOWER(?)", code).Updates(payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update coupon"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"code": code}})
}

func (h *Handler) AdminDeleteCoupon(c *gin.Context) {
	code := c.Param("code")
	if err := h.db.Table("coupons").Where("LOWER(code) = LOWER(?)", code).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete coupon"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"code": code, "deleted": true}})
}

func (h *Handler) AdminListPolicies(c *gin.Context) {
	var rows []map[string]any
	if err := h.db.Table("policies").Order("updated_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load policies"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *Handler) AdminCreatePolicy(c *gin.Context) { h.createTableRow(c, "policies", "title") }
func (h *Handler) AdminUpdatePolicy(c *gin.Context) { h.updateTableRow(c, "policies", "id", c.Param("id")) }
func (h *Handler) AdminDeletePolicy(c *gin.Context) { h.deleteTableRow(c, "policies", "id", c.Param("id")) }

func (h *Handler) AdminGetFAQ(c *gin.Context) {
	var rows []map[string]any
	if err := h.db.Table("global_faq").Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load faq"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *Handler) AdminPatchFAQ(c *gin.Context) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid faq payload"})
		return
	}
	if err := h.db.Exec("DELETE FROM global_faq").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear faq"})
		return
	}
	for i, item := range payload.Items {
		item["sort_order"] = i + 1
		if err := h.db.Table("global_faq").Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save faq"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"count": len(payload.Items)}})
}

func (h *Handler) AdminGetBotConfig(c *gin.Context) {
	var row map[string]any
	if err := h.db.Table("bot_config").Order("updated_at DESC").Take(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"data": gin.H{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load bot config"})
		return
	}
	hasKey := false
	if raw, ok := row["api_key"].(string); ok && raw != "" {
		hasKey = true
	}
	delete(row, "api_key")
	row["hasApiKey"] = hasKey
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) AdminPatchBotConfig(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot config payload"})
		return
	}

	apiKeyValue, hasCamel := payload["apiKey"]
	_, hasSnake := payload["api_key"]
	delete(payload, "apiKey")
	if hasCamel && !hasSnake {
		switch v := apiKeyValue.(type) {
		case string:
			if v != "" {
				encrypted, err := ai.EncryptAPIKey(v, h.cfg.JWTSecret)
				if err == nil {
					payload["api_key"] = encrypted
				}
			} else {
				payload["api_key"] = ""
			}
		}
	} else if hasSnake {
		if raw, ok := payload["api_key"].(string); ok && raw != "" {
			encrypted, err := ai.EncryptAPIKey(raw, h.cfg.JWTSecret)
			if err == nil {
				payload["api_key"] = encrypted
			}
		}
	}

	var existing map[string]any
	err := h.db.Table("bot_config").Order("updated_at DESC").Take(&existing).Error
	if err == gorm.ErrRecordNotFound {
		if e := h.db.Table("bot_config").Create(&payload).Error; e != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create bot config"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": payload})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load bot config"})
		return
	}
	if e := h.db.Table("bot_config").Where("id = ?", existing["id"]).Updates(payload).Error; e != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update bot config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": existing["id"]}})
}

func (h *Handler) AdminTestBotConnection(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true, "provider": h.cfg.AIProvider}})
}

func (h *Handler) AdminAIStats(c *gin.Context) {
	dateFilter := ""
	dateArgs := []any{}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		dateFilter += " AND m.created_at >= ?"
		dateArgs = append(dateArgs, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		dateFilter += " AND m.created_at <= ?"
		dateArgs = append(dateArgs, to)
	}

	providerFilter := strings.TrimSpace(c.Query("provider"))
	modelFilter := strings.TrimSpace(c.Query("model"))

	var totalMessages int64
	var totalConversations int64
	_ = h.db.Table("messages").Count(&totalMessages).Error
	_ = h.db.Table("conversations").Count(&totalConversations).Error

	usage := struct {
		PromptTokens     int64 `json:"promptTokens"`
		CompletionTokens int64 `json:"completionTokens"`
		CacheWriteTokens int64 `json:"cacheWriteTokens"`
		CacheReadTokens  int64 `json:"cacheReadTokens"`
		ReasoningTokens  int64 `json:"reasoningTokens"`
	}{}
	usageQ := "SELECT COALESCE(SUM(usage_prompt_tokens),0) AS prompt_tokens, COALESCE(SUM(usage_completion_tokens),0) AS completion_tokens, COALESCE(SUM(usage_cache_write_tokens),0) AS cache_write_tokens, COALESCE(SUM(usage_cache_read_tokens),0) AS cache_read_tokens, COALESCE(SUM(usage_reasoning_tokens),0) AS reasoning_tokens FROM messages m WHERE 1=1"
	usageArgs := []any{}
	usageArgs = append(usageArgs, dateArgs...)
	if providerFilter != "" {
		usageQ += " AND m.provider = ?"
		usageArgs = append(usageArgs, providerFilter)
	}
	if modelFilter != "" {
		usageQ += " AND m.model = ?"
		usageArgs = append(usageArgs, modelFilter)
	}
	_ = h.db.Raw(usageQ+dateFilter, usageArgs...).Scan(&usage).Error

	type providerStat struct {
		Provider        string `json:"provider"`
		TotalMessages   int64  `json:"totalMessages"`
		PromptTokens    int64  `json:"promptTokens"`
		CompletionTokens int64 `json:"completionTokens"`
	}
	var byProvider []providerStat
	_ = h.db.Raw(`
		SELECT COALESCE(m.provider,'') AS provider,
		       COUNT(*) AS total_messages,
		       COALESCE(SUM(usage_prompt_tokens),0) AS prompt_tokens,
		       COALESCE(SUM(usage_completion_tokens),0) AS completion_tokens
		FROM messages m
		WHERE m.provider IS NOT NULL AND m.provider != ''
		GROUP BY m.provider
		ORDER BY total_messages DESC
	`).Scan(&byProvider).Error

	type modelStat struct {
		Model           string `json:"model"`
		TotalMessages   int64  `json:"totalMessages"`
		PromptTokens    int64  `json:"promptTokens"`
		CompletionTokens int64 `json:"completionTokens"`
	}
	var byModel []modelStat
	_ = h.db.Raw(`
		SELECT COALESCE(m.model,'') AS model,
		       COUNT(*) AS total_messages,
		       COALESCE(SUM(usage_prompt_tokens),0) AS prompt_tokens,
		       COALESCE(SUM(usage_completion_tokens),0) AS completion_tokens
		FROM messages m
		WHERE m.model IS NOT NULL AND m.model != ''
		GROUP BY m.model
		ORDER BY total_messages DESC
	`).Scan(&byModel).Error

	type statusStat struct {
		Status           string `json:"status"`
		TotalConversations int64 `json:"totalConversations"`
	}
	var byStatus []statusStat
	_ = h.db.Table("conversations").
		Select("COALESCE(status,'unknown') AS status, COUNT(*) AS total_conversations").
		Group("status").
		Order("total_conversations DESC").
		Scan(&byStatus).Error

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"messages":      totalMessages,
		"conversations": totalConversations,
		"usage":         usage,
		"byProvider":    byProvider,
		"byModel":       byModel,
		"byStatus":      byStatus,
	}})
}

func (h *Handler) AdminListMessages(c *gin.Context) {
	conversationID := c.Param("id")

	page := 1
	limit := 50
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
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}

	var total int64
	countQ := h.db.Table("messages").Where("conversation_id = ?", conversationID)
	if role := strings.TrimSpace(c.Query("role")); role != "" {
		countQ = countQ.Where("role = ?", role)
	}
	if err := countQ.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count messages"})
		return
	}

	type messageRow struct {
		ID                    int64     `json:"id"`
		Role                  string    `json:"role"`
		Content               string    `json:"content"`
		ToolCalls             *string   `json:"toolCalls"`
		ToolResults           *string   `json:"toolResults"`
		UsagePromptTokens     int       `json:"usagePromptTokens"`
		UsageCompletionTokens int       `json:"usageCompletionTokens"`
		UsageCacheWriteTokens int       `json:"usageCacheWriteTokens"`
		UsageCacheReadTokens  int       `json:"usageCacheReadTokens"`
		UsageReasoningTokens  int       `json:"usageReasoningTokens"`
		Provider              string    `json:"provider"`
		Model                 string    `json:"model"`
		CreatedAt             time.Time `json:"createdAt"`
	}
	var messages []messageRow
	q := h.db.Table("messages").
		Select("id, role, content, tool_calls, tool_results, COALESCE(usage_prompt_tokens,0) AS usage_prompt_tokens, COALESCE(usage_completion_tokens,0) AS usage_completion_tokens, COALESCE(usage_cache_write_tokens,0) AS usage_cache_write_tokens, COALESCE(usage_cache_read_tokens,0) AS usage_cache_read_tokens, COALESCE(usage_reasoning_tokens,0) AS usage_reasoning_tokens, COALESCE(provider,'') AS provider, COALESCE(model,'') AS model, created_at").
		Where("conversation_id = ?", conversationID)
	if role := strings.TrimSpace(c.Query("role")); role != "" {
		q = q.Where("role = ?", role)
	}
	offset := (page - 1) * limit
	if err := q.Order("created_at ASC").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{"data": messages, "meta": gin.H{"page": page, "limit": limit, "total": total, "totalPages": totalPages}})
}

func (h *Handler) AdminListToolCalls(c *gin.Context) {
	page := 1
	limit := 50
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
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}

	type toolCallRow struct {
		MessageID      int64     `json:"messageId"`
		ConversationID int64     `json:"conversationId"`
		ToolName       string    `json:"toolName"`
		Arguments      string    `json:"arguments"`
		ToolResult     string    `json:"toolResult"`
		Role           string    `json:"role"`
		CreatedAt      time.Time `json:"createdAt"`
	}

	whereClause := "WHERE m.tool_calls IS NOT NULL"
	whereArgs := []any{}

	if convID := strings.TrimSpace(c.Query("conversationId")); convID != "" {
		whereClause += " AND m.conversation_id = ?"
		whereArgs = append(whereArgs, convID)
	}
	if toolName := strings.TrimSpace(c.Query("toolName")); toolName != "" {
		whereClause += " AND m.tool_calls::text ILIKE ?"
		whereArgs = append(whereArgs, "%"+toolName+"%")
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		whereClause += " AND m.created_at >= ?"
		whereArgs = append(whereArgs, from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		whereClause += " AND m.created_at <= ?"
		whereArgs = append(whereArgs, to)
	}

	var total int64
	countQ := "SELECT COUNT(*) FROM messages m " + whereClause
	if err := h.db.Raw(countQ, whereArgs...).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count tool calls"})
		return
	}

	var rows []toolCallRow
	offset := (page - 1) * limit
	dataQ := fmt.Sprintf(`
		SELECT m.id AS message_id, m.conversation_id,
		       m.tool_calls::text AS arguments,
		       COALESCE(m.tool_results::text,'') AS tool_result,
		       m.role, m.created_at
		FROM messages m %s
		ORDER BY m.created_at DESC
		OFFSET ? LIMIT ?
	`, whereClause)
	dataArgs := append(whereArgs, offset, limit)
	if err := h.db.Raw(dataQ, dataArgs...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tool calls"})
		return
	}

	type toolCallEntry struct {
		MessageID      int64     `json:"messageId"`
		ConversationID int64     `json:"conversationId"`
		ToolName       string    `json:"toolName"`
		Arguments      any       `json:"arguments"`
		ToolResult     any       `json:"toolResult"`
		Role           string    `json:"role"`
		CreatedAt      time.Time `json:"createdAt"`
	}

	entries := make([]toolCallEntry, 0)
	for _, row := range rows {
		var calls []struct {
			ID        string         `json:"id"`
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(row.Arguments), &calls); err != nil {
			continue
		}
		var results []struct {
			ToolCallID string `json:"toolCallId"`
			Name       string `json:"name"`
			Content    string `json:"content"`
		}
		_ = json.Unmarshal([]byte(row.ToolResult), &results)
		resultMap := map[string]string{}
		for _, r := range results {
			resultMap[r.ToolCallID] = r.Content
		}
		for _, call := range calls {
			entry := toolCallEntry{
				MessageID:      row.MessageID,
				ConversationID: row.ConversationID,
				ToolName:       call.Name,
				Arguments:      call.Arguments,
				ToolResult:     resultMap[call.ID],
				Role:           row.Role,
				CreatedAt:      row.CreatedAt,
			}
			entries = append(entries, entry)
		}
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{"data": entries, "meta": gin.H{"page": page, "limit": limit, "total": total, "totalPages": totalPages}})
}

func (h *Handler) AdminListConversations(c *gin.Context) {
	type convoRow struct {
		ID                    int64     `json:"id"`
		Status                string    `json:"status"`
		CustomerName          *string   `json:"customerName"`
		CustomerPhone         *string   `json:"customerPhone"`
		CreatedAt             time.Time `json:"createdAt"`
		UpdatedAt             time.Time `json:"updatedAt"`
		MessageCount          int64     `json:"messageCount"`
		LastMessage           string    `json:"lastMessage"`
		LastMessageRole       string    `json:"lastMessageRole"`
		TotalPromptTokens     int64     `json:"totalPromptTokens"`
		TotalCompletionTokens int64     `json:"totalCompletionTokens"`
	}

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

	countQ := `SELECT COUNT(*) FROM conversations c`
	countArgs := []any{}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		countQ += " WHERE c.status = ?"
		countArgs = append(countArgs, status)
	}
	var total int64
	if err := h.db.Raw(countQ, countArgs...).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count conversations"})
		return
	}

	var rows []convoRow
	q := `
		SELECT c.id, c.status, c.customer_name, c.customer_phone,
		       c.created_at, c.updated_at,
		       COALESCE(mc.cnt,0) AS message_count,
		       COALESCE(lm.content,'') AS last_message,
		       COALESCE(lm.role,'') AS last_message_role,
		       COALESCE(tu.prompt_tokens,0) AS total_prompt_tokens,
		       COALESCE(tu.completion_tokens,0) AS total_completion_tokens
		FROM conversations c
		LEFT JOIN (SELECT conversation_id, COUNT(*) AS cnt FROM messages GROUP BY conversation_id) mc ON mc.conversation_id = c.id
		LEFT JOIN LATERAL (
			SELECT content, role FROM messages WHERE conversation_id = c.id AND content IS NOT NULL ORDER BY created_at DESC LIMIT 1
		) lm ON TRUE
		LEFT JOIN (
			SELECT conversation_id,
			       SUM(COALESCE(usage_prompt_tokens,0)) AS prompt_tokens,
			       SUM(COALESCE(usage_completion_tokens,0)) AS completion_tokens
			FROM messages GROUP BY conversation_id
		) tu ON tu.conversation_id = c.id
	`
	args := []any{}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q += " WHERE c.status = ?"
		args = append(args, status)
	}
	q += " ORDER BY c.updated_at DESC"
	offset := (page - 1) * limit
	q += " OFFSET ? LIMIT ?"
	args = append(args, offset, limit)
	if err := h.db.Raw(q, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load conversations"})
		return
	}
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, gin.H{"data": rows, "meta": gin.H{"page": page, "limit": limit, "total": total, "totalPages": totalPages}})
}

func (h *Handler) AdminGetConversation(c *gin.Context) {
	id := c.Param("id")
	var convo map[string]any
	if err := h.db.Table("conversations").Where("id = ?", id).Take(&convo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load conversation"})
		return
	}
	type messageRow struct {
		ID                    int64     `json:"id"`
		Role                  string    `json:"role"`
		Content               string    `json:"content"`
		ToolCalls             *string   `json:"toolCalls"`
		ToolResults           *string   `json:"toolResults"`
		UsagePromptTokens     int       `json:"usagePromptTokens"`
		UsageCompletionTokens int       `json:"usageCompletionTokens"`
		UsageCacheWriteTokens int       `json:"usageCacheWriteTokens"`
		UsageCacheReadTokens  int       `json:"usageCacheReadTokens"`
		UsageReasoningTokens  int       `json:"usageReasoningTokens"`
		Provider              string    `json:"provider"`
		Model                 string    `json:"model"`
		CreatedAt             time.Time `json:"createdAt"`
	}
	var messages []messageRow
	if err := h.db.Table("messages").
		Select("id, role, content, tool_calls, tool_results, COALESCE(usage_prompt_tokens,0) AS usage_prompt_tokens, COALESCE(usage_completion_tokens,0) AS usage_completion_tokens, COALESCE(usage_cache_write_tokens,0) AS usage_cache_write_tokens, COALESCE(usage_cache_read_tokens,0) AS usage_cache_read_tokens, COALESCE(usage_reasoning_tokens,0) AS usage_reasoning_tokens, COALESCE(provider,'') AS provider, COALESCE(model,'') AS model, created_at").
		Where("conversation_id = ?", id).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}
	convo["messages"] = messages
	c.JSON(http.StatusOK, gin.H{"data": convo})
}

func (h *Handler) AdminCloseConversation(c *gin.Context) {
	id := c.Param("id")
	var exists int64
	if err := h.db.Table("conversations").Where("id = ?", id).Count(&exists).Error; err != nil || exists == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
		return
	}
	if err := h.db.Table("conversations").Where("id = ?", id).Update("status", "closed").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close conversation"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "closed"}})
}

func (h *Handler) createTableRow(c *gin.Context, table, requiredField string) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if requiredField != "" && strings.TrimSpace(toString(payload[requiredField])) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": requiredField + " is required"})
		return
	}
	if err := h.db.Table(table).Create(&payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create resource"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": payload})
}

func (h *Handler) updateTableRow(c *gin.Context, table, key string, value any) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	delete(payload, key)
	if err := h.db.Table(table).Where(key+" = ?", value).Updates(payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update resource"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{key: value}})
}

func (h *Handler) deleteTableRow(c *gin.Context, table, key string, value any) {
	if err := h.db.Table(table).Where(key+" = ?", value).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete resource"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{key: value, "deleted": true}})
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return ""
	}
}
