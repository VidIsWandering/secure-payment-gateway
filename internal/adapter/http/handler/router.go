package handler

import (
	"secure-payment-gateway/internal/adapter/http/middleware"
	redisStore "secure-payment-gateway/internal/adapter/storage/redis"
	"secure-payment-gateway/internal/core/ports"

	"github.com/gin-gonic/gin"
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
	RateLimitStore *redisStore.RateLimitStore // nil = rate limiting disabled
	Logger         zerolog.Logger
}

// SetupRouter initialises the Gin engine with all routes and middleware.
func SetupRouter(deps RouterDeps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Global middleware
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.RequestLogger(deps.Logger))

	// Health check
	r.GET("/health", HealthCheck())

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
	paymentHandler := NewPaymentHandler(deps.PaymentSvc, deps.WebhookSvc)
	payments := v1.Group("/payments", hmacAuth)
	{
		payments.POST("", rl("payments"), paymentHandler.ProcessPayment)
		payments.POST("/refund", rl("payments_refund"), paymentHandler.ProcessRefund)
	}

	// --- JWT-authenticated routes (dashboard) ---
	jwtAuth := middleware.JWTAuth(deps.TokenSvc, deps.Logger)
	walletHandler := NewWalletHandler(deps.PaymentSvc, deps.ReportingSvc, deps.WebhookSvc)
	dashboardHandler := NewDashboardHandler(deps.ReportingSvc)

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

	return r
}
