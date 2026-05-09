package http

import (
	"net/http"
	"time"

	"fahd-backend/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB) *gin.Engine {
	router := gin.New()
	handler := NewHandler(db, cfg)

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.FrontendOrigin},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api")
	api.GET("/products", handler.ListProducts)
	api.GET("/products/:id", handler.GetProduct)
	api.GET("/categories", handler.ListCategories)
	api.POST("/coupons/validate", handler.ValidateCoupon)
	api.POST("/orders", handler.CreateOrder)
	api.GET("/orders/:id", handler.GetOrder)
	api.POST("/chat/start", handler.StartChat)
	api.POST("/chat/message", handler.SendChatMessage)
	api.POST("/auth/login", handler.Login)
	api.POST("/auth/refresh", handler.Refresh)
	api.POST("/auth/logout", handler.Logout)
	api.GET("/auth/me", handler.Me)

	admin := api.Group("/admin")
	admin.Use(handler.RequireAdmin())
	admin.GET("/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
	})

	return router
}
