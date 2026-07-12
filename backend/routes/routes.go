package routes

import (
	"os"
	"time"

	"profhit-backend/controllers"
	"profhit-backend/middleware"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(middleware.JSONRecoveryMiddleware())

	// Initialize WebSocket Hub
	go services.HandleConnections()

	// ── SECURITY HEADERS & CORS ───────────────────────────────────────────────
	r.Use(func(c *gin.Context) {
		// HTTP Security Headers
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com https://checkout.razorpay.com https://fonts.googleapis.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self' http://localhost:8080 https://api.razorpay.com; frame-src https://checkout.razorpay.com;")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Strict CORS
		appURL := os.Getenv("APP_URL")
		if appURL == "" {
			appURL = "http://localhost:5173" // fallback for dev frontend
		}
		
		origin := c.Request.Header.Get("Origin")
		allowedOrigin := ""
		if appURL != "" && origin == appURL {
			allowedOrigin = origin
		} else if origin == "http://localhost:3000" ||
			origin == "http://localhost:5173" ||
			origin == "http://127.0.0.1:3000" ||
			origin == "http://127.0.0.1:5173" {
			allowedOrigin = origin
		}
		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		// ── PUBLIC AUTH ROUTES (rate-limited) ─────────────────────────────────
		// 5 requests per IP per 5 minutes protects against brute-force and enumeration
		authLimit := middleware.RateLimit(5, 5*time.Minute)
		auth := api.Group("/auth")
		auth.Use(authLimit)
		{
			// Public Auth & Webhooks
			auth.POST("/register", controllers.RegisterUser)
			auth.POST("/login", controllers.LoginUser)
			auth.POST("/forgot-password", controllers.ForgotPassword)
			auth.POST("/reset-password", controllers.ResetPassword)
		}
		
		api.GET("/health", controllers.HealthCheck)
		api.POST("/webhooks/hyperverge", controllers.HypervergeWebhook)
		api.POST("/webhooks/razorpay", controllers.RazorpayWebhook)
		api.POST("/simulator/sign-webhook", controllers.SimulatorSignWebhook)

		// Public market browsing
		api.GET("/markets", controllers.GetAllMarkets)
		api.GET("/markets/:id", controllers.GetMarketByID)
		api.GET("/markets/:id/comments", controllers.GetComments)

		// Public leaderboard & activity
		api.GET("/leaderboard", controllers.GetUnifiedLeaderboard)
		api.GET("/leaderboard/legacy", controllers.GetLeaderboard) // rename old points leaderboard to legacy temporarily
		api.GET("/leaderboard/streak", controllers.GetTopStreak)
		api.GET("/leaderboard/winrate", controllers.GetTopWinRate)
		api.GET("/activity", controllers.GetActivity)

		// Public user profiles
		api.GET("/users/:id", controllers.GetUser)

		// WebSockets
		api.GET("/ws", controllers.WsHandler)

		// ── PROTECTED ROUTES (any authenticated user) ─────────────────────────
		// General prediction limit: 30 req / 1 min to prevent spam
		predLimit := middleware.RateLimit(30, 1*time.Minute)
		// Strict limit for financial endpoints (e.g. 5 requests / 1 minute)
		financeLimit := middleware.RateLimit(5, 1*time.Minute)

		protected := api.Group("/")
		protected.Use(middleware.AuthRequired())
		{
			// Auth
			protected.POST("/auth/logout", controllers.LogoutUser)

			// Wallet & Identity
			protected.GET("/me", controllers.GetMe)
			// Real KYC Flow
			protected.POST("/kyc/start", financeLimit, controllers.StartKYCSession)
			protected.GET("/kyc/status", controllers.GetKYCStatus)
			protected.POST("/me/daily-login", controllers.DailyLogin)
			protected.GET("/me/streak", controllers.GetStreakInfo)
			protected.GET("/wallet/history", controllers.GetWalletHistory)
			protected.GET("/wallet/transaction/:id", controllers.GetWalletTransaction)
			
			// Referrals
			protected.GET("/referrals/analytics", controllers.GetReferralAnalytics)

			// Payments & Redemption
			protected.POST("/payments/order", financeLimit, controllers.CreateRazorpayOrder)
			protected.POST("/payments/verify", financeLimit, controllers.VerifyPayment)
			protected.POST("/payments/redeem", financeLimit, controllers.RedeemVoucher)

			// Predictions (Fixed-Odds) — rate limited
			protected.POST("/predictions", predLimit, controllers.SubmitPrediction)
			protected.GET("/predictions", controllers.GetUserPredictions)
			protected.GET("/portfolio", controllers.GetPortfolio) // Enriched prediction history

			// Market proposals (any logged-in user can propose)
			protected.POST("/markets/propose", predLimit, controllers.ProposeMarket)

			// Comments (any logged-in user can comment)
			protected.POST("/markets/:id/comments", predLimit, controllers.AddComment)

			// Reports (user reporting)
			protected.POST("/reports", predLimit, controllers.SubmitReport)
			protected.GET("/me/reports", controllers.GetUserReports)

			// Rewards & Redemptions
			protected.GET("/rewards", controllers.GetRewardCatalog)
			protected.POST("/rewards/redeem", financeLimit, controllers.SubmitRedemption)
			protected.GET("/me/redemptions", controllers.GetUserRedemptions)
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
			contentRoutes.PUT("/markets/:id/transition", controllers.TransitionMarketState)
			contentRoutes.DELETE("/markets/:id", controllers.DeleteMarket)
		}

		// ── ADMIN ROUTES ──────────────────────────────────────────────────────
		adminRoutes := api.Group("/")
		adminRoutes.Use(middleware.AuthRequired())
		adminRoutes.Use(middleware.RoleRequired(
			models.RoleSuperAdmin, models.RoleAdmin,
		))
		{
			adminRoutes.POST("/markets/:id/resolve", controllers.ResolveMarket)
			adminRoutes.GET("/admin/redemptions", controllers.AdminGetRedemptions)
			adminRoutes.PUT("/admin/redemptions/:id", controllers.AdminProcessRedemption)
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

			// Audit Logs (super_admin ONLY)
			adminPanel.GET("/audit-logs", middleware.RoleRequired(
				models.RoleSuperAdmin,
			), controllers.GetAuditLogs)

			// KYC queue (admin, super_admin)
			adminPanel.GET("/kyc", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetAdminKycRequests)
			adminPanel.GET("/kyc/:id", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetAdminKycRequestByID)

			// Withdrawal/Redemption queue (admin, super_admin)
			adminPanel.GET("/withdrawals", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetWithdrawals)
			adminPanel.POST("/withdrawals/:id/approve", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.ApproveWithdrawal)
			adminPanel.POST("/withdrawals/:id/reject", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.RejectWithdrawal)

			// Reports & Moderation (admin, super_admin)
			adminPanel.GET("/reports", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.GetReports)
			adminPanel.POST("/reports/:id/resolve", middleware.RoleRequired(models.RoleSuperAdmin, models.RoleAdmin), controllers.ResolveReport)
		}
	}

	return r
}
