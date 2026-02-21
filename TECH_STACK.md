# TECH STACK SPECIFICATION

Project: Secure Payment Gateway
Language: Go (Golang) version 1.22+

## 1. Core Frameworks & Libraries

- **Web Framework**: `github.com/gin-gonic/gin`
  - Reason: High performance, middleware support.
- **Configuration**: `github.com/spf13/viper`
- **Logging**: `github.com/rs/zerolog`
  - Requirement: Structured JSON logging for audit trails.

## 2. Database & Caching (CRITICAL)

- **RDBMS**: PostgreSQL 16+
- **ORM/Driver**: `github.com/sqlc-dev/sqlc` (with `pgx/v5`)
  - **Strict Rule**: Use `pgx` for explicit Transaction control and Pessimistic Locking (`SELECT FOR UPDATE`).
- **Migrations**: `github.com/golang-migrate/migrate/v4`
- **Redis**: `github.com/redis/go-redis/v9`
  - Purpose: Caching user sessions and storing **Idempotency Keys** (TTL required).

## 3. Security & Cryptography (STRICT)

- **Authentication**: `github.com/golang-jwt/jwt/v5`
- **Password Hashing**: `golang.org/x/crypto/argon2` (Argon2id).
  - **Strict Rule**: Use Argon2id variant. Params: Time=1, Memory=64MB, Threads=4, KeyLen=32.
  - Reason: Memory-hard, OWASP recommended for new financial systems. Resistant to GPU/ASIC attacks.
- **Data Encryption (AES-256)**: Standard `crypto/aes` + `crypto/cipher` (GCM mode).
  - Purpose: Encrypt sensitive fields (Balance, Card Info) before DB storage.
- **Digital Signature**: Standard `crypto/hmac` (HMAC-SHA256).
  - Purpose: Verify request integrity from Merchant.
- **Rate Limiting**: `github.com/ulule/limiter/v3` (Redis-backed limiter).

## 4. Documentation & Testing

- **Docs**: `github.com/swaggo/swag`
- **Unit Test**: `github.com/stretchr/testify` + `github.com/uber-go/mock`
- **Load Testing**: `k6` (External tool) or JMeter (as per original req) to simulate concurrency.
