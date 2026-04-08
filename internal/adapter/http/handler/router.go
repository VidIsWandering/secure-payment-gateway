package handler

import (
	"secure-payment-gateway/internal/adapter/http/middleware"
	"secure-payment-gateway/internal/core/ports"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// RouterDeps holds all dependencies needed to set up routes.
type RouterDeps struct {
	AuthSvc        ports.AuthService
	PaymentSvc     ports.PaymentService
	ReportingSvc   ports.ReportingService
	WebhookSvc     ports.WebhookService
	MerchantRepo   ports.MerchantRepository
	EncSvc         ports.EncryptionService
	SigSvc         ports.SignatureService
	NonceStore     ports.NonceStore
	TokenSvc       ports.TokenService
	RateLimitStore ports.RateLimitStore            // nil = rate limiting disabled
	HealthCheckers []ports.HealthChecker
	MerchantSvc    ports.MerchantManagementService // nil = merchant management disabled
	AuditSvc       ports.AuditService              // nil = audit logging disabled
	TxRepo         ports.TransactionRepository      // nil = payment status disabled
	Logger         zerolog.Logger
	ServerMode     string // "debug", "release", "test"
}

// SetupRouter initialises the Gin engine with all routes and middleware.
func SetupRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.RequestLogger(deps.Logger))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MB request body limit

	// Prometheus metrics
	r.Use(middleware.PrometheusMetrics())

	// CORS middleware
	r.Use(middleware.CORS())

	// Audit logging (after response)
	if deps.AuditSvc != nil {
		r.Use(middleware.AuditLog(deps.AuditSvc))
	}

	// Health check (deep — verifies PostgreSQL + Redis)
	r.GET("/health", HealthCheck(deps.HealthCheckers...))

	// Prometheus metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Web UI routes (Full App, Landing, Auth, Dashboard, Checkout Simulator)
	uiHandler := NewUIHandler(deps.ServerMode)
	uiHandler.RegisterRoutes(r)

	// Swagger documentation — only in non-release mode
	if deps.ServerMode != "release" {
		swagger := r.Group("/swagger")
		{
			swagger.GET("", SwaggerUI)
			swagger.GET("/spec", SwaggerSpec)
		}
	}

	// Rate limit rules
	rules := middleware.DefaultRateLimitRules()

	// Helper: return rate limiter middleware if store is available, else noop.
	rl := func(group string) gin.HandlerFunc {
		if deps.RateLimitStore == nil {
			return func(c *gin.Context) { c.Next() }
		}
		rule, ok := rules[group]
		if !ok {
			return func(c *gin.Context) { c.Next() }
		}
		return middleware.RateLimiter(deps.RateLimitStore, group, rule, deps.Logger)
	}

	// API v1 routes
	v1 := r.Group("/api/v1")

	// --- Public routes (no auth) ---
	authHandler := NewAuthHandler(deps.AuthSvc)
	auth := v1.Group("/auth")
	{
		auth.POST("/register", rl("auth_register"), authHandler.Register)
		auth.POST("/login", rl("auth_login"), authHandler.Login)
	}

	// --- HMAC-authenticated routes (merchant API) ---
	hmacAuth := middleware.HMACAuth(deps.MerchantRepo, deps.EncSvc, deps.SigSvc, deps.NonceStore, deps.Logger)
	paymentHandler := NewPaymentHandler(deps.PaymentSvc, deps.WebhookSvc, deps.TxRepo)
	payments := v1.Group("/payments", hmacAuth)
	{
		payments.POST("", rl("payments"), paymentHandler.ProcessPayment)
		payments.POST("/refund", rl("payments_refund"), paymentHandler.ProcessRefund)
	}

	// --- JWT-authenticated routes (dashboard) ---
	jwtAuth := middleware.JWTAuth(deps.TokenSvc, deps.Logger)
	walletHandler := NewWalletHandler(deps.PaymentSvc, deps.ReportingSvc, deps.WebhookSvc)
	dashboardHandler := NewDashboardHandler(deps.ReportingSvc)

	// Payment status (JWT auth)
	paymentStatus := v1.Group("/payments", jwtAuth)
	{
		paymentStatus.GET("/:id/status", rl("dashboard"), paymentHandler.GetPaymentStatus)
	}

	wallets := v1.Group("/wallets", jwtAuth)
	{
		wallets.GET("/balance", rl("dashboard"), walletHandler.GetBalance)
		wallets.POST("/topup", rl("wallets_topup"), walletHandler.Topup)
	}

	dashboard := v1.Group("/dashboard", jwtAuth)
	{
		dashboard.GET("/stats", rl("dashboard"), dashboardHandler.GetStats)
	}

	transactions := v1.Group("/transactions", jwtAuth)
	{
		transactions.GET("", rl("dashboard"), dashboardHandler.ListTransactions)
	}

	// --- Merchant management (JWT-authenticated) ---
	if deps.MerchantSvc != nil {
		merchantHandler := NewMerchantHandler(deps.MerchantSvc)
		merchants := v1.Group("/merchants/me", jwtAuth)
		{
			merchants.GET("", rl("dashboard"), merchantHandler.GetProfile)
			merchants.PUT("/webhook", rl("dashboard"), merchantHandler.UpdateWebhookURL)
			merchants.POST("/rotate-keys", rl("dashboard"), merchantHandler.RotateKeys)
		}
	}

	return r
}
