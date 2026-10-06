package http

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"regexp"
	"strings"
)

type marketingSettings struct {
	PixelID string `json:"pixelId" gorm:"column:tiktok_pixel_id"`
	Enabled bool   `json:"enabled"`
}

func (h *Handler) GetMarketingSettings(c *gin.Context) {
	var settings marketingSettings
	if err := h.db.Table("marketing_settings").Where("id=1").Take(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "تعذر تحميل إعدادات البيكسل"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": settings})
}
func (h *Handler) SaveMarketingSettings(c *gin.Context) {
	var settings marketingSettings
	if c.ShouldBindJSON(&settings) != nil {
		c.JSON(400, gin.H{"error": "بيانات غير صحيحة"})
		return
	}
	settings.PixelID = strings.TrimSpace(settings.PixelID)
	if (settings.PixelID != "" && !regexp.MustCompile(`^[A-Za-z0-9]{10,40}$`).MatchString(settings.PixelID)) || (settings.Enabled && settings.PixelID == "") {
		c.JSON(400, gin.H{"error": "أدخل معرّف TikTok Pixel الصحيح، وليس كود السكربت"})
		return
	}
	if err := h.db.Table("marketing_settings").Where("id=1").Updates(map[string]any{"tiktok_pixel_id": settings.PixelID, "enabled": settings.Enabled}).Error; err != nil {
		c.JSON(500, gin.H{"error": "تعذر حفظ الإعدادات"})
		return
	}
	c.JSON(200, gin.H{"data": settings})
}
