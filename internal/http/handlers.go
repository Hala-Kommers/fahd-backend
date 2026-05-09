package http

import (
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

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) ListProducts(c *gin.Context) {
	var products []Product
	query := h.db.Model(&Product{}).Where("status = ?", "active")

	if category := strings.TrimSpace(c.Query("category")); category != "" {
		query = query.Joins("JOIN categories ON categories.id = products.category_id").Where("categories.slug = ?", category)
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
		query = query.Order("updated_at DESC")
	}

	if err := query.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": products})
}

func (h *Handler) GetProduct(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{"data": product})
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
	if req.PaymentMethod != "COD" && req.PaymentMethod != "Online" {
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
