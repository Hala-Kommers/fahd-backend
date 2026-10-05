package http

import (
	"encoding/json"
	"fahd-backend/internal/ai/services"
	"fahd-backend/internal/session"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) EngageChat(c *gin.Context) {
	var r struct {
		SessionID    string         `json:"sessionId"`
		SessionToken string         `json:"sessionToken"`
		VisitorID    string         `json:"visitorId"`
		Metadata     map[string]any `json:"metadata"`
	}
	if c.ShouldBindJSON(&r) != nil || r.SessionID == "" || !h.validChatSession(r.SessionID, r.SessionToken) {
		c.JSON(401, gin.H{"error": "جلسة غير صالحة"})
		return
	}
	id, e := services.Engage(c.Request.Context(), h.db, r.SessionID, r.VisitorID, r.Metadata)
	if e != nil {
		c.JSON(500, gin.H{"error": "تعذر تسجيل التفاعل"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"conversationId": id}})
}
func (h *Handler) AdminFunnel(c *gin.Context) {
	from := time.Now().AddDate(0, 0, -30)
	to := time.Now()
	if v, e := time.Parse("2006-01-02", c.Query("from")); e == nil {
		from = v
	}
	if v, e := time.Parse("2006-01-02", c.Query("to")); e == nil {
		to = v.Add(24 * time.Hour)
	}
	// A cohort is selected by first engagement, not by the date of its order.
	filter := " WHERE e.is_test=FALSE AND e.engaged_at>=? AND e.engaged_at<?"
	args := []any{from, to}
	for _, field := range []string{"source", "device", "experiment"} {
		if v := c.Query(field); v != "" {
			filter += " AND e." + field + "=?"
			args = append(args, v)
		}
	}
	if p, e := strconv.ParseInt(c.Query("productId"), 10, 64); e == nil && p > 0 {
		filter += " AND e.product_id=?"
		args = append(args, p)
	}
	if v := c.Query("returning"); v == "true" || v == "false" {
		filter += " AND e.returning_customer=?"
		args = append(args, v == "true")
	}
	type row struct {
		Source          string `json:"source"`
		Device          string `json:"device"`
		ProductID       *int64 `json:"productId"`
		Returning       bool   `json:"returning" gorm:"column:returning_customer"`
		Experiment      string `json:"experiment"`
		Engaged         int64  `json:"engaged"`
		Converted       int64  `json:"converted"`
		Confirmed       int64  `json:"confirmed"`
		Delivered       int64  `json:"delivered"`
		Cancelled       int64  `json:"cancelled"`
		Returned        int64  `json:"returned"`
		Mature          int64  `json:"mature"`
		MatureConverted int64  `json:"matureConverted"`
	}
	rows := []row{}
	q := `SELECT e.source,e.device,e.product_id,e.returning_customer,e.experiment,COUNT(*) AS engaged,
 COUNT(*) FILTER(WHERE o.converted) AS converted, COUNT(*) FILTER(WHERE o.confirmed) AS confirmed,
 COUNT(*) FILTER(WHERE o.delivered) AS delivered, COUNT(*) FILTER(WHERE o.cancelled) AS cancelled, COUNT(*) FILTER(WHERE o.returned) AS returned,
 COUNT(*) FILTER(WHERE e.engaged_at<=NOW()-INTERVAL '7 days') AS mature,
 COUNT(*) FILTER(WHERE e.engaged_at<=NOW()-INTERVAL '7 days' AND o.converted) AS mature_converted
 FROM chat_engagements e LEFT JOIN LATERAL (
 SELECT bool_or(status NOT IN ('cancelled','returned')) AS converted,
 bool_or(status IN ('confirmed','processing','shipped','delivered')) AS confirmed,
 bool_or(status='delivered') AS delivered,bool_or(status='cancelled') AS cancelled,bool_or(status='returned') AS returned
 FROM orders WHERE conversation_id=e.conversation_id AND is_test=FALSE AND created_at>=e.engaged_at AND created_at<e.engaged_at+INTERVAL '7 days'
 ) o ON TRUE` + filter + ` GROUP BY e.source,e.device,e.product_id,e.returning_customer,e.experiment`
	if err := h.db.Raw(q, args...).Scan(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "تعذر حساب التحويل"})
		return
	}
	stages := []struct {
		Event string `json:"event"`
		Count int64  `json:"count"`
	}{}
	if e := h.db.Raw(`SELECT a.event_type AS event,COUNT(DISTINCT e.conversation_id) AS count FROM chat_engagements e JOIN analytics_events a ON a.session_id=e.session_id AND a.created_at>=e.engaged_at AND a.created_at<e.engaged_at+INTERVAL '7 days'`+filter+` AND a.event_type IN ('offer_viewed','offer_selected','checkout_started','checkout_error','complement_clicked') GROUP BY a.event_type`, args...).Scan(&stages).Error; e != nil {
		c.JSON(500, gin.H{"error": "تعذر حساب مراحل الشراء"})
		return
	}
	var total row
	for _, r := range rows {
		total.Engaged += r.Engaged
		total.Converted += r.Converted
		total.Confirmed += r.Confirmed
		total.Delivered += r.Delivered
		total.Cancelled += r.Cancelled
		total.Returned += r.Returned
		total.Mature += r.Mature
		total.MatureConverted += r.MatureConverted
	}
	c.JSON(200, gin.H{"data": gin.H{"totals": total, "segments": rows, "stages": stages, "conversionRate": percentage(total.Converted, total.Engaged), "matureConversionRate": percentage(total.MatureConverted, total.Mature), "windowDays": 7, "target": 10}})
}
func (h *Handler) UpdateProductContent(c *gin.Context) {
	var r struct {
		VideoURL          string  `json:"videoUrl"`
		RelatedProductIDs []int64 `json:"relatedProductIds"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid content"})
		return
	}
	if r.VideoURL != "" {
		u, e := url.Parse(r.VideoURL)
		if e != nil || u.Scheme != "https" || u.Host == "" {
			c.JSON(400, gin.H{"error": "استخدم رابط فيديو HTTPS مباشر"})
			return
		}
	}
	if len(r.RelatedProductIDs) > 3 {
		c.JSON(400, gin.H{"error": "اختر ٣ منتجات مكملة بحد أقصى"})
		return
	}
	ids, _ := json.Marshal(r.RelatedProductIDs)
	if e := h.db.Table("products").Where("id=?", c.Param("id")).Updates(map[string]any{"video_url": r.VideoURL, "related_product_ids": string(ids)}).Error; e != nil {
		c.JSON(500, gin.H{"error": "تعذر الحفظ"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"saved": true}})
}
func (h *Handler) CreateReview(c *gin.Context) {
	var r struct {
		OrderID   int64  `json:"orderId"`
		ProductID int64  `json:"productId"`
		Token     string `json:"token"`
		Rating    int    `json:"rating"`
		Body      string `json:"body"`
	}
	if c.ShouldBindJSON(&r) != nil || r.Rating < 1 || r.Rating > 5 || len(strings.TrimSpace(r.Body)) < 3 || len(r.Body) > 2000 {
		c.JSON(400, gin.H{"error": "اكتب تقييمًا صحيحًا"})
		return
	}
	order, e := services.NewOrderService(h.db).Track(c.Request.Context(), r.OrderID, r.Token)
	if e != nil || order["status"] != "delivered" {
		c.JSON(http.StatusForbidden, gin.H{"error": "التقييم متاح لصاحب الطلب بعد التسليم"})
		return
	}
	var n int64
	h.db.Table("order_items").Where("order_id=? AND product_id=?", r.OrderID, r.ProductID).Count(&n)
	if n == 0 {
		c.JSON(400, gin.H{"error": "المنتج غير موجود في الطلب"})
		return
	}
	name := strings.Fields(order["customer"].(map[string]any)["name"].(string))[0]
	e = h.db.Exec(`INSERT INTO product_reviews(order_id,product_id,rating,body,display_name) VALUES(?,?,?,?,?) ON CONFLICT(order_id,product_id) DO NOTHING`, r.OrderID, r.ProductID, r.Rating, strings.TrimSpace(r.Body), name).Error
	if e != nil {
		c.JSON(500, gin.H{"error": "تعذر حفظ التقييم"})
		return
	}
	c.JSON(201, gin.H{"data": gin.H{"saved": true}})
}

func (h *Handler) StartChatSession(c *gin.Context) {
	id := uuid.NewString()
	token, expiry, e := session.NewTokenService(h.cfg.JWTSecret, 24*time.Hour).Generate(id)
	if e != nil {
		c.JSON(500, gin.H{"error": "تعذر فتح المحادثة"})
		return
	}
	c.JSON(201, gin.H{"data": gin.H{"sessionId": id, "token": token, "expiresAt": expiry}})
}

func (h *Handler) MarkTestOrder(c *gin.Context) {
	var r struct {
		IsTest bool `json:"isTest"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid payload"})
		return
	}
	var o struct {
		ID             int64
		ConversationID *int64
	}
	if e := h.db.Table("orders").Where("id=?", c.Param("id")).Take(&o).Error; e != nil {
		c.JSON(404, gin.H{"error": "order not found"})
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Table("orders").Where("id=?", o.ID).Update("is_test", r.IsTest).Error; e != nil {
			return e
		}
		if o.ConversationID != nil {
			return tx.Table("chat_engagements").Where("conversation_id=?", *o.ConversationID).Update("is_test", r.IsTest).Error
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "تعذر تحديث الطلب"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"isTest": r.IsTest}})
}
func (h *Handler) UpdateCityDelivery(c *gin.Context) {
	var r struct {
		ShippingFee      float64 `json:"shippingFee"`
		DeliveryEstimate string  `json:"deliveryEstimate"`
	}
	if c.ShouldBindJSON(&r) != nil || r.ShippingFee < 0 || r.ShippingFee > 10000 || len(r.DeliveryEstimate) > 200 {
		c.JSON(400, gin.H{"error": "بيانات التوصيل غير صالحة"})
		return
	}
	if e := h.db.Table("cities").Where("id=?", c.Param("id")).Updates(map[string]any{"shipping_fee": r.ShippingFee, "delivery_estimate": r.DeliveryEstimate}).Error; e != nil {
		c.JSON(500, gin.H{"error": "تعذر الحفظ"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"saved": true}})
}
func (h *Handler) Performance(c *gin.Context) {
	rows := []struct {
		Name    string  `json:"name"`
		Device  string  `json:"device"`
		P75     float64 `json:"p75"`
		P95     float64 `json:"p95"`
		Samples int64   `json:"samples"`
	}{}
	e := h.db.Raw(`SELECT metadata->>'name' AS name,COALESCE(metadata->>'device','server') AS device,percentile_cont(.75) WITHIN GROUP(ORDER BY (metadata->>'value')::numeric) AS p75,percentile_cont(.95) WITHIN GROUP(ORDER BY (metadata->>'value')::numeric) AS p95,COUNT(*) AS samples FROM analytics_events WHERE event_type IN ('web_vital','bot_latency') AND created_at>NOW()-INTERVAL '7 days' AND jsonb_typeof(metadata->'value')='number' GROUP BY 1,2`).Scan(&rows).Error
	if e != nil {
		c.JSON(500, gin.H{"error": "تعذر قراءة الأداء"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
