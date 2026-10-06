package http

import (
	"github.com/gin-gonic/gin"
	"strings"
)

func (h *Handler) AdminListCities(c *gin.Context) {
	var rows []City
	if err := h.db.Table("cities").Order("sort_order ASC, name ASC").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "تعذر تحميل المدن"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}

func (h *Handler) AdminCreateCity(c *gin.Context) {
	var p struct {
		Name             string  `json:"name"`
		ShippingFee      float64 `json:"shippingFee"`
		DeliveryEstimate string  `json:"deliveryEstimate"`
		IsActive         *bool   `json:"isActive"`
	}
	if c.ShouldBindJSON(&p) != nil || strings.TrimSpace(p.Name) == "" || len([]rune(p.Name)) > 100 || p.ShippingFee < 0 {
		c.JSON(400, gin.H{"error": "بيانات المدينة غير صالحة"})
		return
	}
	active := true
	if p.IsActive != nil {
		active = *p.IsActive
	}
	row := map[string]any{"name": strings.TrimSpace(p.Name), "shipping_fee": p.ShippingFee, "delivery_estimate": strings.TrimSpace(p.DeliveryEstimate), "is_active": active}
	if err := h.db.Table("cities").Create(&row).Error; err != nil {
		c.JSON(409, gin.H{"error": "تعذر إضافة المدينة أو أنها موجودة"})
		return
	}
	c.JSON(201, gin.H{"data": row})
}
func (h *Handler) AdminUpdateCity(c *gin.Context) {
	var p struct {
		Name             *string  `json:"name"`
		ShippingFee      *float64 `json:"shippingFee"`
		DeliveryEstimate *string  `json:"deliveryEstimate"`
		IsActive         *bool    `json:"isActive"`
	}
	if c.ShouldBindJSON(&p) != nil {
		c.JSON(400, gin.H{"error": "بيانات غير صالحة"})
		return
	}
	updates := map[string]any{}
	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" || len([]rune(*p.Name)) > 100 {
			c.JSON(400, gin.H{"error": "اسم المدينة غير صالح"})
			return
		}
		updates["name"] = strings.TrimSpace(*p.Name)
	}
	if p.ShippingFee != nil {
		if *p.ShippingFee < 0 || *p.ShippingFee > 10000 {
			c.JSON(400, gin.H{"error": "رسوم الشحن غير صالحة"})
			return
		}
		updates["shipping_fee"] = *p.ShippingFee
	}
	if p.DeliveryEstimate != nil {
		updates["delivery_estimate"] = strings.TrimSpace(*p.DeliveryEstimate)
	}
	if p.IsActive != nil {
		updates["is_active"] = *p.IsActive
	}
	if len(updates) == 0 {
		c.JSON(400, gin.H{"error": "لا توجد تغييرات"})
		return
	}
	if err := h.db.Table("cities").Where("id=?", c.Param("id")).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"error": "تعذر حفظ المدينة"})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"saved": true}})
}
