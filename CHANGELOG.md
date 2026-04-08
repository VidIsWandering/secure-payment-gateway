# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Config Validation**: `Validate()` method on `Config` — enforces `JWT_SECRET` ≥ 32 chars, `AES_KEY` = 64 hex chars, valid server mode, port range, and pool size.
- **CORS Middleware**: Handles `Access-Control-*` headers and preflight `OPTIONS` requests.
- **Request Timeout Middleware**: Context-based deadline to prevent long-running requests.
- **Prometheus Metrics**: `PrometheusMetrics()` middleware tracks HTTP request count, duration histogram (p50/p95/p99), and in-flight gauge. Custom counters for payment transactions and webhook deliveries.
- **Prometheus /metrics Endpoint**: Exposes metrics at `GET /metrics` for scraping.
- **Grafana Dashboard**: Pre-built dashboard JSON with 9 panels (request rate, latency, error rate, payment types, webhook status, etc.).
- **Prometheus + Grafana in docker-compose**: Full observability stack with auto-provisioned datasource and dashboards.
- **GET /payments/:id/status**: New endpoint for querying transaction status by ID (JWT authenticated).
- **RateLimitStore Interface**: `ports.RateLimitStore` and `ports.RateLimitResult` abstract rate limiting from Redis implementation.
- **SSRF-safe Webhook URL Validation**: `ValidateWebhookURL()` blocks private IPs, loopback, link-local, and non-HTTPS URLs.
- **`.env.example`**: Documents all environment variables with generation instructions.
- **Audit Logging Middleware**: Records API actions to audit_logs table for compliance.
- **Webhook Delivery Persistence**: Webhook attempts are persisted to `webhook_delivery_logs` with retry status tracking.

### Changed
- **Hexagonal Architecture**: Replaced all `pgx.Tx` references in ports with abstract `ports.Tx` interface. Domain layer is now fully decoupled from PostgreSQL driver.
- **Transactional Registration**: `AuthService.Register()` now creates merchant + wallet atomically in a single DB transaction via `DBTransactor`.
- **Payment Handler**: `ProcessPayment` and `ProcessRefund` now correctly pass `X-Signature` header value into the service request.
- **Migration Schema**: `amount` column changed from `DECIMAL(20,2)` to `BIGINT` (matches Go `int64`). Added `UNIQUE(merchant_id, currency)` on wallets and composite unique index on `transactions(merchant_id, reference_id)`.
- **docker-compose**: Removed deprecated `version` key, fixed `initdb` mount to `.up.sql`, added healthcheck-based `depends_on`.
- **Graceful Shutdown**: HTTP server uses `signal.Notify` + `sync.WaitGroup` to drain in-flight webhook goroutines before exit.
- **Dockerfile**: Now copies `docs/api/openapi.yaml` into runtime image for Swagger UI.
- **Swagger UI**: Conditionally enabled (hidden in `release` mode).
- **Mock Repositories**: Regenerated with `ports.Tx` signatures, added `CreateTx` mocks and `MockRateLimitStore`.
- **Config Tests**: Updated to provide valid secrets for validation, added negative tests for missing `JWT_SECRET` / `AES_KEY`.
- **Webhook Service**: Now accepts `*sync.WaitGroup` for lifecycle tracking.
- **Rate Limit Middleware**: Uses `ports.RateLimitStore` interface instead of concrete Redis store.

### Fixed
- **Signature not stored**: Payment and refund transactions were saving empty signature — now correctly captured from `X-Signature` header.
- **Non-atomic registration**: Merchant and wallet creation could partially succeed — now wrapped in a DB transaction.
- **Config silent failures**: Missing `JWT_SECRET` or `AES_KEY` no longer defaults silently — fails fast with descriptive error.
- **In-memory test repos**: Updated to implement `ports.Tx` interface, fixing integration test compilation.

## [0.1.0] - 2026-03-17

### Added
- Initial release with core payment gateway functionality.
- Merchant registration and JWT authentication.
- HMAC-SHA256 request signing with replay protection (nonce + timestamp).
- AES-256-GCM encrypted wallet balances.
- Payment, refund, and topup operations with pessimistic locking.
- Redis-backed idempotency (dual-layer: Redis cache + DB log).
- Redis-backed rate limiting per endpoint group.
- Webhook notifications with exponential backoff retry.
- Dashboard UI (embedded SPA served from Go backend).
- Swagger UI with OpenAPI 3.0 specification.
- Health check endpoint (PostgreSQL + Redis).
- Docker + docker-compose deployment.
- Integration test suite with miniredis.
