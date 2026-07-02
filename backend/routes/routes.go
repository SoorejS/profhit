package routes

import (
	"time"

	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	go controllers.HandleMessages()

	// ── CORS ──────────────────────────────────────────────────────────────────
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
		// ── PUBLIC AUTH ROUTES (rate-limited) ─────────────────────────────────
		// 10 requests per IP per 5 minutes protects against brute-force and enumeration
		authLimit := middleware.RateLimit(10, 5*time.Minute)
		auth := api.Group("/auth")
		auth.Use(authLimit)
		{
			// Public Auth & Webhooks
			auth.POST("/register", controllers.RegisterUser)
			auth.POST("/login", controllers.LoginUser)
			auth.POST("/forgot-password", controllers.ForgotPassword)
			auth.POST("/reset-password", controllers.ResetPassword)
		}
		api.POST("/webhooks/hyperverge", controllers.HypervergeWebhook)
		api.POST("/simulator/sign-webhook", controllers.SimulatorSignWebhook)

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

		// ── PROTECTED ROUTES (any authenticated user) ─────────────────────────
		// General prediction limit: 30 req / 1 min to prevent spam
		predLimit := middleware.RateLimit(30, 1*time.Minute)

		protected := api.Group("/")
		protected.Use(middleware.AuthRequired())
		{
			// Wallet & Identity
			protected.GET("/me", controllers.GetMe)
			// Real KYC Flow
			protected.POST("/kyc/start", controllers.StartKYCSession)
			protected.GET("/kyc/status", controllers.GetKYCStatus)
			protected.POST("/me/daily-login", controllers.DailyLogin)
			protected.GET("/me/streak", controllers.GetStreakInfo)
			protected.GET("/wallet/history", controllers.GetWalletHistory)
			protected.GET("/wallet/transaction/:id", controllers.GetWalletTransaction)
			
			// Referrals
			protected.GET("/referrals/analytics", controllers.GetReferralAnalytics)

			// Payments & Redemption
			protected.POST("/payments/order", controllers.CreateRazorpayOrder)
			protected.POST("/payments/verify", controllers.VerifyPayment)
			protected.POST("/payments/redeem", controllers.RedeemVoucher)

			// Predictions (Fixed-Odds) — rate limited
			protected.POST("/predictions", predLimit, controllers.SubmitPrediction)
			protected.GET("/predictions", controllers.GetUserPredictions)
			protected.GET("/portfolio", controllers.GetPortfolio) // Enriched prediction history

			// Market proposals (any logged-in user can propose)
			protected.POST("/markets/propose", predLimit, controllers.ProposeMarket)

			// Comments (any logged-in user can comment)
			protected.POST("/markets/:id/comments", predLimit, controllers.AddComment)
		}

		// ── CONTENT CREATOR ROUTES ────────────────────────────────────────────
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

		// ── ADMIN ROUTES ──────────────────────────────────────────────────────
		adminRoutes := api.Group("/")
		adminRoutes.Use(middleware.AuthRequired())
		adminRoutes.Use(middleware.RoleRequired(
			models.RoleSuperAdmin, models.RoleAdmin,
		))
		{
			adminRoutes.POST("/markets/:id/resolve", controllers.ResolveMarket)
		}

		// ── ADMIN PANEL ROUTES ────────────────────────────────────────────────
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

			// KYC queue (admin, super_admin)
			adminPanel.GET("/kyc", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetAdminKycRequests)
			adminPanel.GET("/kyc/:id", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetAdminKycRequestByID)

			// Withdrawal/Redemption queue (admin, super_admin)
			adminPanel.GET("/withdrawals", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetWithdrawals)
			adminPanel.POST("/withdrawals/:id/approve", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.ApproveWithdrawal)
			adminPanel.POST("/withdrawals/:id/reject", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.RejectWithdrawal)
		}
	}

	return r
}
