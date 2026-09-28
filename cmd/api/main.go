package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/config"
	httpHandler "github.com/VidIsWandering/secure-payment-gateway/internal/adapter/http/handler"
	"github.com/VidIsWandering/secure-payment-gateway/internal/adapter/http/middleware"
	pgStorage "github.com/VidIsWandering/secure-payment-gateway/internal/adapter/storage/postgres"
	redisStorage "github.com/VidIsWandering/secure-payment-gateway/internal/adapter/storage/redis"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"
	"github.com/VidIsWandering/secure-payment-gateway/internal/service"
	"github.com/VidIsWandering/secure-payment-gateway/pkg/logger"
)

func main() {
	// Load configuration (includes validation for JWT_SECRET, AES_KEY)
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Log.Level, cfg.Log.Pretty)

	log.Info().
		Str("mode", cfg.Server.Mode).
		Int("port", cfg.Server.Port).
		Msg("Starting Secure Payment Gateway")

	ctx := context.Background()

	// Apply pending schema migrations (embedded in the binary)
	if cfg.Database.AutoMigrate {
		version, err := pgStorage.Migrate(cfg.Database.DSN())
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to apply database migrations")
		}
		log.Info().Uint("schema_version", version).Msg("Database schema up to date")
	}

	// Initialize PostgreSQL pool
	pool, err := pgStorage.NewPool(ctx, cfg.Database, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to PostgreSQL")
	}
	defer pool.Close()
	log.Info().Msg("PostgreSQL connected")

	// Initialize Redis client
	rdb, err := redisStorage.NewClient(ctx, cfg.Redis, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer rdb.Close()
	log.Info().Msg("Redis connected")

	// Initialize repositories
	merchantRepo := pgStorage.NewMerchantRepo(pool)
	walletRepo := pgStorage.NewWalletRepo(pool)
	txRepo := pgStorage.NewTransactionRepo(pool)
	idempotencyRepo := pgStorage.NewIdempotencyRepo(pool)
	transactor := pgStorage.NewTransactor(pool)

	// Initialize Redis stores
	idempotencyCache := redisStorage.NewIdempotencyCache(rdb)
	nonceStore := redisStorage.NewNonceStore(rdb)

	// Initialize core services
	encSvc, err := service.NewAESEncryptionService(cfg.AES.Key)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize encryption service")
	}
	sigSvc := service.NewHMACSignatureService()
	hashSvc := service.NewArgon2HashService()
	tokenSvc := service.NewJWTTokenService(cfg.JWT.Secret, cfg.JWT.Expiry, cfg.JWT.Issuer)

	// Initialize business services
	authSvc := service.NewAuthService(merchantRepo, walletRepo, hashSvc, encSvc, tokenSvc, transactor)
	webhookRepo := middleware.InstrumentWebhookRepository(pgStorage.NewWebhookRepository(pool))
	webhookSvc := service.NewWebhookService(merchantRepo, webhookRepo, encSvc, sigSvc, &http.Client{Timeout: 10 * time.Second}, log)
	paymentSvc := service.NewPaymentService(
		txRepo,
		walletRepo,
		idempotencyRepo,
		idempotencyCache,
		encSvc,
		transactor,
		webhookSvc,
		log,
	)
	reportingSvc := service.NewReportingService(txRepo, walletRepo, encSvc)
	merchantSvc := service.NewMerchantService(merchantRepo, encSvc)
	auditRepo := pgStorage.NewAuditRepository(pool)
	auditSvc := service.NewAuditService(auditRepo, log)

	// Initialize rate limit store
	rateLimitStore := redisStorage.NewRateLimitStore(rdb)

	// Initialize health checkers
	pgHealth := pgStorage.NewHealthCheck(pool)
	redisHealth := redisStorage.NewHealthCheck(rdb)

	// Load OpenAPI spec for Swagger UI
	if specBytes, err := os.ReadFile("docs/api/openapi.yaml"); err == nil {
		httpHandler.SetSwaggerSpec(specBytes)
		log.Info().Msg("OpenAPI spec loaded for Swagger UI at /swagger")
	} else {
		log.Warn().Err(err).Msg("OpenAPI spec not found, Swagger UI will be unavailable")
	}

	// Setup Gin router with all routes
	router := httpHandler.SetupRouter(httpHandler.RouterDeps{
		AuthSvc:                 authSvc,
		PaymentSvc:              paymentSvc,
		ReportingSvc:            reportingSvc,
		MerchantRepo:            merchantRepo,
		EncSvc:                  encSvc,
		SigSvc:                  sigSvc,
		NonceStore:              nonceStore,
		TokenSvc:                tokenSvc,
		RateLimitStore:          rateLimitStore,
		HealthCheckers:          []ports.HealthChecker{pgHealth, redisHealth},
		MerchantSvc:             merchantSvc,
		AuditSvc:                auditSvc,
		TxRepo:                  txRepo,
		Logger:                  log,
		ServerMode:              cfg.Server.Mode,
		RateLimitPayments:       cfg.RateLimit.Payments,
		RateLimitPaymentsRefund: cfg.RateLimit.PaymentsRefund,
		NonceFailOpen:           cfg.Security.NonceFailOpen,
	})

	// HTTP Server with graceful shutdown
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Deliver webhooks from the outbox in the background
	dispatchCtx, stopDispatcher := context.WithCancel(context.Background())
	dispatcherDone := make(chan struct{})
	go func() {
		defer close(dispatcherDone)
		webhookSvc.Run(dispatchCtx)
	}()

	// Start server in goroutine
	go func() {
		log.Info().Str("addr", addr).Msg("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	// Stop the webhook dispatcher; in-flight deliveries finish, and anything
	// still pending stays in the outbox for the next start.
	stopDispatcher()
	select {
	case <-dispatcherDone:
		log.Info().Msg("Webhook dispatcher stopped")
	case <-time.After(15 * time.Second):
		log.Warn().Msg("Webhook dispatcher did not stop in time; pending webhooks remain queued")
	}

	log.Info().Msg("Server exited")
}
