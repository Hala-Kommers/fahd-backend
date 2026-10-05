package http

import (
	"net/http"
	"time"

	"fahd-backend/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB, wsHandler http.Handler) *gin.Engine {
	router := gin.New()
	handler := NewHandler(db, cfg, wsHandler)

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
	api.GET("/cities", handler.ListCities)
	api.POST("/analytics/events", handler.TrackAnalyticsEvent)
	api.POST("/coupons/validate", handler.ValidateCoupon)
	api.POST("/orders", handler.CreateOrder)
	api.POST("/offers/session", handler.SessionOffer)
	api.GET("/orders/:id", handler.GetOrder)
	api.GET("/ws/chat", func(c *gin.Context) {
		handler.wsHandler.ServeHTTP(c.Writer, c.Request)
	})
	api.POST("/auth/login", handler.Login)
	api.POST("/auth/refresh", handler.Refresh)
	api.POST("/auth/logout", handler.Logout)
	api.GET("/auth/me", handler.Me)

	admin := api.Group("/admin")
	admin.Use(handler.RequireAdmin())
	admin.GET("/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
	})
	admin.GET("/products", handler.AdminListProducts)
	admin.GET("/products/:id", handler.AdminGetProduct)
	admin.POST("/products", handler.AdminCreateProduct)
	admin.PATCH("/products/:id", handler.AdminUpdateProduct)
	admin.DELETE("/products/:id", handler.AdminDeleteProduct)
	admin.GET("/orders", handler.AdminListOrders)
	admin.GET("/orders/:id", handler.AdminGetOrder)
	admin.PATCH("/orders/:id", handler.AdminUpdateOrder)
	admin.GET("/coupons", handler.AdminListCoupons)
	admin.GET("/coupons/:code", handler.AdminGetCoupon)
	admin.POST("/coupons", handler.AdminCreateCoupon)
	admin.PATCH("/coupons/:code", handler.AdminUpdateCoupon)
	admin.DELETE("/coupons/:code", handler.AdminDeleteCoupon)
	admin.GET("/bot/config", handler.AdminGetBotConfig)
	admin.PATCH("/bot/config", handler.AdminPatchBotConfig)
	admin.POST("/bot/test-connection", handler.AdminTestBotConnection)
	admin.GET("/ai/stats", handler.AdminAIStats)
	admin.GET("/analytics/overview", handler.AdminAnalyticsOverview)
	admin.GET("/analytics/orders", handler.AdminAnalyticsOrders)
	admin.GET("/analytics/sales-chart", handler.AdminAnalyticsSalesChart)
	admin.GET("/conversations", handler.AdminListConversations)
	admin.GET("/conversations/:id", handler.AdminGetConversation)
	admin.GET("/conversations/:id/messages", handler.AdminListMessages)
	admin.POST("/conversations/:id/close", handler.AdminCloseConversation)

	return router
}
