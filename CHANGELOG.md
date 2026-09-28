# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security
- **Replay protection fails closed**: if the Redis nonce store is unavailable, signed requests are rejected with `503 SYS_004` instead of being accepted without replay protection. Opt out with `SPG_SECURITY_NONCE_FAIL_OPEN=true`.

### Added
- **CodeQL** code scanning (Go, JavaScript, GitHub Actions) with the `security-extended` query suite.
- **Release pipeline**: publishing a GitHub release builds and pushes a multi-arch (amd64/arm64) image to `ghcr.io/vidiswandering/secure-payment-gateway` with SBOM and build-provenance attestation.

### Changed
- **Demo scripts** print English output and check the actual responses: `demo_security.py` and `demo_concurrency.py` report ✅/❌ per scenario and exit non-zero on failure.
- **Dockerfile** cross-compiles for the target platform (`TARGETOS`/`TARGETARCH`) and builds with `-trimpath`.
- **Go 1.26**: minimum Go version is now 1.26; CI and the Docker build use Go 1.26.8, the runtime image uses Alpine 3.24. All Go modules and GitHub Actions updated (pgx held at v5.10.0 until pgxmock supports v5.11).

## [0.2.0] - 2026-09-28

### Added
- **Business metrics wired up**: payment/refund/top-up counters (`spg_payment_transactions_total`) and webhook delivery outcomes (`spg_webhook_deliveries_total`) are now recorded, so the corresponding Grafana panels show data.
- **Project documentation**: `docs/ARCHITECTURE.md`, `SECURITY.md`, request-signing guide, benchmark results and design decisions in the README, screenshots in `docs/images/`.
- **Repository tooling**: Dependabot (Go modules, GitHub Actions, Docker), YAML issue forms.
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
- **Go module path** is now `github.com/VidIsWandering/secure-payment-gateway`.
- **golangci-lint v2**: configuration migrated; CI pins golangci-lint `v2.14.0` and TruffleHog `v3.97.9`.
- **docker-compose**: PostgreSQL published on host port `5435` (container port `5432`); Grafana on `3005`.
- **Audit logging** now uses `context.WithoutCancel` instead of `context.Background`, keeping request-scoped values.
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
- **Placeholder JWT secret**: `config/config.yaml` no longer ships a secret that passes validation — `SPG_JWT_SECRET` must be provided.
- **docker-compose PostgreSQL port mapping** pointed to the wrong container port, breaking the local-run workflow.
- **Documentation drift**: auth type of `POST /wallets/topup`, timestamp tolerance (±60 s), Grafana port, k6 scenario sizes.
- **Signature not stored**: Payment and refund transactions were saving empty signature — now correctly captured from `X-Signature` header.
- **Non-atomic registration**: Merchant and wallet creation could partially succeed — now wrapped in a DB transaction.
- **Config silent failures**: Missing `JWT_SECRET` or `AES_KEY` no longer defaults silently — fails fast with descriptive error.
- **In-memory test repos**: Updated to implement `ports.Tx` interface, fixing integration test compilation.

## [0.1.0] - 2026-03-18

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

[Unreleased]: https://github.com/VidIsWandering/secure-payment-gateway/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/VidIsWandering/secure-payment-gateway/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/VidIsWandering/secure-payment-gateway/releases/tag/v0.1.0
