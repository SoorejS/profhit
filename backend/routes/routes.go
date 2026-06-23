package routes

import (
	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	go controllers.HandleMessages()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		// --- PUBLIC ROUTES (no auth needed) ---
		auth := api.Group("/auth")
		{
			auth.POST("/register", controllers.RegisterUser)
			auth.POST("/login", controllers.LoginUser)
			auth.POST("/forgot-password", controllers.ForgotPassword)
		}

		// Public market browsing
		api.GET("/markets", controllers.GetAllMarkets)
		api.GET("/markets/:id", controllers.GetMarketByID)
		api.GET("/markets/:id/comments", controllers.GetComments)

		// Public leaderboard & activity
		api.GET("/stats/leaderboard", controllers.GetLeaderboard)
		api.GET("/stats/activity", controllers.GetActivity)

		// Public user profiles
		api.GET("/users/:id", controllers.GetUser)

		// WebSockets
		api.GET("/ws", controllers.WsHandler)

		// --- PROTECTED ROUTES (any authenticated user) ---
		protected := api.Group("/")
		protected.Use(middleware.AuthRequired())
		{
			// Own profile
			protected.GET("/me", controllers.GetMe)
			protected.POST("/me/kyc", controllers.UpdateKycStatus)

			// Payments
			protected.POST("/payments/create-order", controllers.CreateRazorpayOrder)
			protected.POST("/payments/verify", controllers.VerifyPayment)
			protected.POST("/payments/withdraw", controllers.WithdrawFunds)

			// Trading
			protected.POST("/trades", controllers.PlaceTrade)
			protected.POST("/trades/:id/sell", controllers.SellTrade)
			protected.GET("/portfolio", controllers.GetUserTrades)

			// Limit Orders
			protected.POST("/trades/limit", controllers.CreateLimitOrder)
			protected.GET("/trades/limit", controllers.GetUserLimitOrders)
			protected.DELETE("/trades/limit/:id", controllers.CancelLimitOrder)

			// Market proposals (any logged-in user can propose)
			protected.POST("/markets/propose", controllers.ProposeMarket)

			// Comments (any logged-in user can comment)
			protected.POST("/markets/:id/comments", controllers.AddComment)
		}

		// --- CONTENT CREATOR ROUTES ---
		contentRoutes := api.Group("/")
		contentRoutes.Use(middleware.AuthRequired())
		contentRoutes.Use(middleware.RoleRequired(
			models.RoleSuperAdmin, models.RoleAdmin, models.RoleContentCreator,
		))
		{
			contentRoutes.POST("/markets", controllers.CreateMarket)
			contentRoutes.GET("/markets/proposed", controllers.GetProposedMarkets)
			contentRoutes.POST("/markets/:id/approve", controllers.ApproveMarket)
		}

		// --- ADMIN ROUTES ---
		adminRoutes := api.Group("/")
		adminRoutes.Use(middleware.AuthRequired())
		adminRoutes.Use(middleware.RoleRequired(
			models.RoleSuperAdmin, models.RoleAdmin,
		))
		{
			adminRoutes.POST("/markets/:id/resolve", controllers.ResolveMarket)
		}

		// --- ADMIN PANEL ROUTES ---
		adminPanel := api.Group("/admin")
		adminPanel.Use(middleware.AuthRequired())
		{
			// Platform stats (admin, super_admin, it_support)
			adminPanel.GET("/stats", middleware.RoleRequired(
				models.RoleSuperAdmin, models.RoleAdmin, models.RoleITSupport,
			), controllers.GetPlatformStats)

			// User management (admin, super_admin, it_support)
			adminPanel.GET("/users", middleware.RoleRequired(
				models.RoleSuperAdmin, models.RoleAdmin, models.RoleITSupport,
			), controllers.GetAllUsers)

			adminPanel.POST("/users/:id/ban", middleware.RoleRequired(
				models.RoleSuperAdmin, models.RoleAdmin, models.RoleITSupport,
			), controllers.BanUser)

			adminPanel.POST("/users/:id/unban", middleware.RoleRequired(
				models.RoleSuperAdmin, models.RoleAdmin, models.RoleITSupport,
			), controllers.UnbanUser)

			// Role assignment (super_admin ONLY)
			adminPanel.PUT("/users/:id/role", middleware.RoleRequired(
				models.RoleSuperAdmin,
			), controllers.UpdateUserRole)
		}
	}

	return r
}
