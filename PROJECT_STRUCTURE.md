# PROJECT STRUCTURE GUIDELINES

Architecture Pattern: Clean Architecture (Port & Adapter)

## Directory Layout

```
├── cmd/
│   └── api/
│       └── main.go                # Entry point: Wire DI, start server.
├── config/
│   ├── config.go                  # Viper config struct & loader.
│   └── config.yaml                # Default config (overridden by env vars).
├── internal/
│   ├── core/                      # DOMAIN LAYER (Pure Go, zero external deps)
│   │   ├── domain/                # Entity structs.
│   │   │   ├── merchant.go        # Merchant entity.
│   │   │   ├── wallet.go          # Wallet entity.
│   │   │   ├── transaction.go     # Transaction entity + enums (Type, Status).
│   │   │   └── idempotency.go     # IdempotencyLog entity.
│   │   └── ports/                 # Interface definitions.
│   │       ├── repositories.go    # MerchantRepo, WalletRepo, TransactionRepo, IdempotencyRepo.
│   │       └── services.go        # PaymentService, AuthService, EncryptionService, ReportingService.
│   ├── service/                   # BUSINESS LOGIC LAYER
│   │   ├── auth_service.go        # Registration, login, JWT, bcrypt.
│   │   ├── payment_service.go     # CORE: Payment + Refund + Topup (ACID, Locking).
│   │   ├── reporting_service.go   # Dashboard stats, transaction listing.
│   │   ├── encryption_service.go  # AES-256-GCM encrypt/decrypt.
│   │   ├── signature_service.go   # HMAC-SHA256 sign/verify.
│   │   └── webhook_service.go     # Async webhook dispatch + retry logic.
│   └── adapter/                   # INFRASTRUCTURE LAYER
│       ├── handler/               # HTTP Controllers.
│       │   ├── dto/               # Request/Response structs with validation tags.
│       │   │   ├── auth_dto.go
│       │   │   ├── payment_dto.go
│       │   │   ├── wallet_dto.go
│       │   │   └── reporting_dto.go
│       │   └── http/              # Gin handler implementations.
│       │       ├── auth_handler.go
│       │       ├── payment_handler.go
│       │       ├── wallet_handler.go
│       │       ├── reporting_handler.go
│       │       └── router.go      # Route registration + middleware binding.
│       ├── middleware/            # CRITICAL CROSS-CUTTING CONCERNS
│       │   ├── auth.go            # JWT verification middleware.
│       │   ├── signature.go       # HMAC-SHA256 signature + replay attack middleware.
│       │   ├── ratelimit.go       # Redis-backed rate limiting.
│       │   └── request_id.go      # Unique request ID injection.
│       └── storage/               # Database & Cache implementations.
│           ├── postgres/          # PostgreSQL repository implementations.
│           │   ├── merchant_repo.go
│           │   ├── wallet_repo.go
│           │   ├── transaction_repo.go
│           │   ├── idempotency_repo.go
│           │   ├── reporting_repo.go
│           │   └── db.go          # Connection pool + pgx setup.
│           └── redis/             # Redis implementations.
│               ├── idempotency_store.go  # Idempotency key cache.
│               ├── nonce_store.go        # Nonce uniqueness check.
│               ├── ratelimit_store.go    # Rate limit counter.
│               └── redis.go             # Redis client setup.
├── pkg/                           # Shared utilities (importable by any layer).
│   ├── logger/                    # Zerolog wrapper.
│   ├── response/                  # Standardized HTTP response + error mapper.
│   └── apperror/                  # Custom error types mapped to error codes.
├── db/
│   ├── schema.sql                 # Full DDL schema.
│   ├── migrations/                # golang-migrate migration files.
│   └── queries/                   # sqlc query files.
│       ├── merchant.sql
│       ├── wallet.sql
│       ├── transaction.sql
│       └── idempotency.sql
└── docs/
    ├── TRANSACTION_STRATEGY.md
    └── api/
        ├── ERROR_CODES.md
        ├── openapi.yaml
        └── WEBHOOK_SPEC.md
    └── logic/
        ├── CORE_TRANSACTION.md
        ├── SECURITY_FLOW.md
        └── REPORTING.md
```

## Rules for AI Coding Assistant

1. **Architecture Violation**: `core` layer must NEVER import `adapter` or `service`.
2. **Concurrency Control**: All balance updates MUST be inside a `service` layer transaction block using `pgx.Tx` with strict Pessimistic Locking.
3. **Sensitive Data**: Never log raw credit card info or unencrypted balances. Use `encryption_service` before saving to `storage`.
4. **Idempotency**: Check Redis for existing `TransactionID` in `middleware` or early `service` stage before processing payment.
