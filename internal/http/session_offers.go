package http

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"time"
)

func (h *Handler) SessionOffer(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId"`
		ProductID int64  `json:"productId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid offer request"})
		return
	}
	if _, err := uuid.Parse(req.SessionID); err != nil {
		c.JSON(400, gin.H{"error": "invalid session"})
		return
	}
	var p struct {
		Price       float64
		CompareAt   *float64
		HasVariants bool
	}
	if h.db.Table("products").Select("price, compare_at, has_variants").Where("id = ? AND status = 'active'", req.ProductID).First(&p).Error != nil {
		c.JSON(404, gin.H{"error": "product not found"})
		return
	}
	if p.CompareAt == nil || *p.CompareAt <= p.Price || p.HasVariants {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"available": false}})
		return
	}
	if err := h.db.Exec("INSERT INTO session_offers (session_id,product_id) VALUES (?,?) ON CONFLICT DO NOTHING", req.SessionID, req.ProductID).Error; err != nil {
		c.JSON(500, gin.H{"error": "offer unavailable"})
		return
	}
	var offer struct{ EndsAt time.Time }
	if err := h.db.Table("session_offers").Where("session_id = ? AND product_id = ?", req.SessionID, req.ProductID).First(&offer).Error; err != nil {
		c.JSON(500, gin.H{"error": "offer unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"available": true, "endsAt": offer.EndsAt, "originalPrice": p.CompareAt, "offerPrice": p.Price, "expired": !time.Now().Before(offer.EndsAt)}})
}

func (h *Handler) sessionOfferExpired(sessionID string, productID int64) bool {
	if sessionID == "" {
		return false
	}
	var count int64
	h.db.Table("session_offers").Where("session_id = ? AND product_id = ? AND ends_at <= NOW()", sessionID, productID).Count(&count)
	return count > 0
}
