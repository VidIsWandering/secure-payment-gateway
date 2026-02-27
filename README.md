# Secure Payment Gateway

A production-ready payment gateway API built in Go following Clean Architecture principles, with a focus on security, reliability, and testability.

## Features

- **Payment Processing** — Create payments with idempotency protection, automatic balance deduction, and signature verification
- **Refund & Top-up** — Full refund workflow and wallet top-up with transaction history
- **Merchant Authentication** — API key/secret + HMAC signature auth, JWT-based session tokens
- **Security** — AES-256-GCM encryption, HMAC-SHA256 signatures, Argon2id password hashing, replay-attack prevention via nonce store
- **Webhook Delivery** — Asynchronous webhook notifications with retry logic and delivery persistence
- **Rate Limiting** — Redis-backed sliding-window rate limiter per merchant
- **Audit Logging** — Automatic audit trail for all write operations
- **Reporting Dashboard** — Revenue summaries, success rates, and transaction history
- **Swagger UI** — Built-in API documentation at `/swagger`
- **Health Checks** — Deep health check endpoint with per-dependency status (PostgreSQL, Redis)
- **Input Sanitization** — XSS protection, strict input validation, request body size limit

## Architecture

```
cmd/api/              → Application entry point & wire-up
internal/
  core/
    domain/           → Business entities (Transaction, Merchant, Wallet, etc.)
    ports/            → Interface definitions (repositories, services)
      mocks/          → Auto-generated gomock mocks
  service/            → Business logic implementations
  adapter/
    http/
      handler/        → Gin HTTP handlers & router
      dto/            → Request/response DTOs with validation
      middleware/     → Auth, rate-limit, audit, sanitizer, logging
    storage/
      postgres/       → PostgreSQL repository implementations
      redis/          → Redis store implementations (nonce, idempotency, rate-limit)
config/               → Configuration loading (Viper, env vars)
pkg/                  → Shared packages (apperror, logger, response)
tests/integration/    → End-to-end integration & concurrency tests
db/migrations/        → SQL migration files
docs/api/             → OpenAPI spec, webhook spec, error codes
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.23+ |
| Web Framework | Gin |
| Database | PostgreSQL 16 |
| Cache/Store | Redis 7 |
| Config | Viper (env prefix: `SPG_`) |
| Logging | Zerolog (structured JSON) |
| Auth | JWT (HS256), HMAC-SHA256, API Keys |
| Encryption | AES-256-GCM |
| Hashing | Argon2id |
| Testing | testify, gomock, pgxmock, miniredis |
| CI/CD | GitHub Actions |
| Container | Docker (multi-stage build) |

## Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- Make (optional)

### Run with Docker Compose

```bash
# Start all services (PostgreSQL, Redis, App)
docker compose up -d

# The API is available at http://localhost:8080
# Swagger UI at http://localhost:8080/swagger
# Health check at http://localhost:8080/health
```

### Run Locally

```bash
# Start dependencies
docker compose up -d postgres redis

# Set required environment variables
export SPG_JWT_SECRET="your-secret-key-min-32-characters-long"
export SPG_AES_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# Run the application
make run
# or: go run ./cmd/api
```

### Build

```bash
make build
# Output: bin/spg-api
```

## Configuration

All configuration is via environment variables with the `SPG_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `SPG_SERVER_PORT` | `8080` | HTTP server port |
| `SPG_SERVER_MODE` | `debug` | Gin mode (`debug`, `release`, `test`) |
| `SPG_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `SPG_DATABASE_PORT` | `5432` | PostgreSQL port |
| `SPG_DATABASE_USER` | `postgres` | Database user |
| `SPG_DATABASE_PASSWORD` | `postgres` | Database password |
| `SPG_DATABASE_DBNAME` | `payment_gateway` | Database name |
| `SPG_DATABASE_SSLMODE` | `disable` | SSL mode |
| `SPG_DATABASE_MAX_CONNS` | `20` | Max pool connections |
| `SPG_REDIS_HOST` | `localhost` | Redis host |
| `SPG_REDIS_PORT` | `6379` | Redis port |
| `SPG_JWT_SECRET` | — | **Required.** JWT signing key (min 32 chars) |
| `SPG_JWT_EXPIRY` | `24h` | JWT token expiry |
| `SPG_AES_KEY` | — | **Required.** 64-char hex key for AES-256-GCM |
| `SPG_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `SPG_LOG_PRETTY` | `false` | Human-readable logs (dev only) |

## API Endpoints

### Authentication
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/auth/register` | Register a new merchant |
| `POST` | `/api/v1/auth/login` | Login and obtain JWT token |

### Payments
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/payments` | API Key + Signature | Create a payment |
| `POST` | `/api/v1/payments/refund` | API Key + Signature | Refund a transaction |
| `GET` | `/api/v1/payments/:id/status` | JWT | Get payment status |

### Wallets
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/wallets/topup` | API Key + Signature | Top up wallet |
| `GET` | `/api/v1/wallets/balance` | JWT | Get wallet balance |

### Merchant Management
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/merchants/me` | JWT | Get merchant profile |
| `PUT` | `/api/v1/merchants/me/webhook` | JWT | Update webhook URL |
| `POST` | `/api/v1/merchants/me/rotate-keys` | JWT | Rotate API keys |

### Reporting
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/dashboard/summary` | JWT | Revenue & success rate summary |
| `GET` | `/api/v1/transactions` | JWT | Transaction history |

### System
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Deep health check |
| `GET` | `/swagger` | Swagger UI |
| `GET` | `/swagger/spec` | OpenAPI YAML spec |

## Testing

```bash
# Run all tests
make test

# Run tests with verbose output
make test-v

# Run with coverage report
make coverage

# Run a specific test
go test ./internal/service/... -run TestPayment -v
```

**226 tests** across 12 packages covering:
- Unit tests for all services, handlers, middleware, DTOs
- PostgreSQL repository tests (pgxmock)
- Redis store tests (miniredis)
- Integration tests with in-memory repositories
- Concurrency stress tests (100 concurrent payments, idempotency under race)

## Development

```bash
# Regenerate mocks after changing port interfaces
make mocks

# Run linter
make lint

# Apply database migrations
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/payment_gateway?sslmode=disable"
make migrate-up

# Build Docker image
make docker-build

# See all available commands
make help
```

## Security

This gateway implements multiple security layers:

1. **Authentication**: Dual-layer — API key + HMAC-SHA256 signature for payment operations, JWT for session-based access
2. **Encryption at Rest**: Merchant secret keys encrypted with AES-256-GCM before storage
3. **Password Hashing**: Argon2id with per-user salt
4. **Replay Protection**: Redis-backed nonce store prevents request replay attacks
5. **Input Validation**: Strict validation rules, HTML entity escaping, 1MB body size limit
6. **Rate Limiting**: Per-merchant sliding-window rate limiter
7. **Audit Trail**: All write operations are automatically logged with IP, action, and details

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
