package handler

import (
"secure-payment-gateway/internal/adapter/http/middleware"
"secure-payment-gateway/internal/core/ports"

"github.com/gin-gonic/gin"
"github.com/rs/zerolog"
)

// RouterDeps holds all dependencies needed to set up routes.
type RouterDeps struct {
AuthSvc      ports.AuthService
PaymentSvc   ports.PaymentService
ReportingSvc ports.ReportingService
MerchantRepo ports.MerchantRepository
EncSvc       ports.EncryptionService
SigSvc       ports.SignatureService
NonceStore   ports.NonceStore
TokenSvc     ports.TokenService
Logger       zerolog.Logger
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

// API v1 routes
v1 := r.Group("/api/v1")

// --- Public routes (no auth) ---
authHandler := NewAuthHandler(deps.AuthSvc)
auth := v1.Group("/auth")
{
auth.POST("/register", authHandler.Register)
auth.POST("/login", authHandler.Login)
}

// --- HMAC-authenticated routes (merchant API) ---
hmacAuth := middleware.HMACAuth(deps.MerchantRepo, deps.EncSvc, deps.SigSvc, deps.NonceStore, deps.Logger)
paymentHandler := NewPaymentHandler(deps.PaymentSvc)
payments := v1.Group("/payments", hmacAuth)
{
payments.POST("", paymentHandler.ProcessPayment)
payments.POST("/refund", paymentHandler.ProcessRefund)
}

// --- JWT-authenticated routes (dashboard) ---
jwtAuth := middleware.JWTAuth(deps.TokenSvc, deps.Logger)
walletHandler := NewWalletHandler(deps.PaymentSvc, deps.ReportingSvc)
dashboardHandler := NewDashboardHandler(deps.ReportingSvc)

wallets := v1.Group("/wallets", jwtAuth)
{
wallets.GET("/balance", walletHandler.GetBalance)
wallets.POST("/topup", walletHandler.Topup)
}

dashboard := v1.Group("/dashboard", jwtAuth)
{
dashboard.GET("/stats", dashboardHandler.GetStats)
}

transactions := v1.Group("/transactions", jwtAuth)
{
transactions.GET("", dashboardHandler.ListTransactions)
}

return r
}
