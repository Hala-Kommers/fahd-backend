package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) AdminListProducts(c *gin.Context) {
	var rows []map[string]any
	q := h.db.Table("products p").
		Select("p.*, c.name as category_name").
		Joins("LEFT JOIN categories c ON c.id = p.category_id").
		Order("p.updated_at DESC")
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q = q.Where("p.status = ?", status)
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load admin products"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

func (h *Handler) AdminCreateProduct(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product payload"})
		return
	}
	if strings.TrimSpace(toString(payload["title"])) == "" || strings.TrimSpace(toString(payload["sku"])) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and sku are required"})
		return
	}
	if _, ok := payload["status"]; !ok {
		payload["status"] = "draft"
	}
	if _, ok := payload["currency"]; !ok {
		payload["currency"] = "SAR"
	}
	if err := h.db.Table("products").Create(&payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create product"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": payload})
}

func (h *Handler) AdminUpdateProduct(c *gin.Context) {
	id := c.Param("id")
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product payload"})
		return
	}
	delete(payload, "id")
	if err := h.db.Table("products").Where("id = ?", id).Updates(payload).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update product"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

func (h *Handler) AdminDeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Table("products").Where("id = ?", id).Delete(nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete product"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "deleted": true}})
}

func (h *Handler) AdminListOrders(c *gin.Context) {
	var rows []Order
	q := h.db.Order("created_at DESC")
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
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
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func (h *Handler) AdminPatchBotConfig(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bot config payload"})
		return
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
	var totalMessages int64
	var totalConversations int64
	_ = h.db.Table("messages").Count(&totalMessages).Error
	_ = h.db.Table("conversations").Count(&totalConversations).Error
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"messages": totalMessages, "conversations": totalConversations}})
}

func (h *Handler) AdminListConversations(c *gin.Context) {
	var rows []map[string]any
	q := h.db.Table("conversations").Order("updated_at DESC")
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load conversations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
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
	var messages []map[string]any
	_ = h.db.Table("messages").Where("conversation_id = ?", id).Order("timestamp ASC").Find(&messages).Error
	convo["messages"] = messages
	c.JSON(http.StatusOK, gin.H{"data": convo})
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
