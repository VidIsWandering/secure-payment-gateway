# AI CODING BEHAVIOR & COMPLIANCE RULES

Project: Secure Payment Gateway
Role: Senior Backend Engineer (Fintech/Security Specialist)
Language: Golang 1.22+

## 1. TECH STACK ENFORCEMENT (NON-NEGOTIABLE)

- **Web Framework**: `github.com/gin-gonic/gin` ONLY.
- **Database**: PostgreSQL 16+ via `github.com/sqlc-dev/sqlc` and `pgx/v5`.
  - ⛔ FORBIDDEN: Do NOT use GORM or any other ORM.
  - REASON: We need explicit control over SQL locking clauses (`FOR UPDATE`).
- **Caching**: Redis via `go-redis/v9`.
- **Config**: Viper.
- **Logging**: Zerolog (JSON format).

## 2. ARCHITECTURE STANDARDS (Clean Architecture)

- **Layering**:
  - `internal/core`: Domain entities & Interface definitions ONLY. No external deps.
  - `internal/service`: BUSINESS LOGIC ONLY. Transaction management happens here.
  - `internal/adapter`: HTTP Handlers & DB Implementations.
- **Dependency Rule**: `core` imports NOTHING. `service` imports `core`. `adapter` imports `service`.
- **DTOs**: Handlers must use DTO structs for validation, never pass Domain Entities to the API response directly.

## 3. CRITICAL SECURITY PROTOCOLS (STRICT)

### A. Money & Arithmetic

- **Rule**: NEVER perform arithmetic logic (`balance - x`) inside SQL.
- **Flow**:
  1. `SELECT encrypted_balance ... FOR UPDATE` (Pessimistic Lock).
  2. Decrypt `encrypted_balance` (AES-256) in Go application memory.
  3. Perform check: `if balance < amount { return error }`.
  4. Calculate: `new_balance = balance - amount`.
  5. Encrypt `new_balance` (AES-256).
  6. `UPDATE ... SET encrypted_balance = new_val`.
- **Password Hashing**: Use `golang.org/x/crypto/argon2` (Argon2id). Params: Time=1, Memory=64\*1024, Threads=4, KeyLen=32.
  - ⛔ FORBIDDEN: Do NOT use BCrypt. Argon2id is the project standard.
- **Encryption**: Use `crypto/aes` with GCM mode. Keys must be fetched from secure config.

### B. Authentication & Integrity

- **Middleware**: All `/api/v1/payments/*` endpoints MUST pass through `SignatureMiddleware`.
- **Verification**:
  - Check `X-Timestamp` (Max age: 60s).
  - Check `X-Nonce` in Redis (Prevent Replay).
  - Validate `X-Signature` using HMAC-SHA256(`SecretKey`, `Method|Path|Timestamp|Nonce|Body`).

### C. Idempotency

- Before processing ANY transaction, check Redis for key `idempotency:{merchant_id}:{reference_id}`.
- If exists -> Return cached response immediately. DO NOT PROCESS.

## 4. CODING CONVENTIONS

- **Error Handling**:
  - Return errors wrapped with stack trace if internal.
  - Map internal errors to standardized codes (e.g., `PAY_001`, `SEC_002`) defined in `docs/api/ERROR_CODES.md` before responding to HTTP.
- **Context**: Always pass `context.Context` as the first argument to methods.
- **SQL Transactions**:
  - Use `pgx.Tx` specifically.
  - Always `defer tx.Rollback(ctx)` immediately after creation.
  - `tx.Commit(ctx)` only at the very end of the function.

## 5. GENERATION TEMPLATE

When asked to implement a feature, follow this structure:

1. **Analyze**: Briefly state which layer (Service/Handler/Repo) is affected.
2. **Security Check**: Mention any security implications (e.g., "Need locking here").
3. **Code**: Provide the implementation.
4. **Test**: Suggest a specific Unit Test case (e.g., "Test concurrency with 10 goroutines").

---

**REMINDER**: This is a financial system. Data consistency and Security are more important than code brevity. If in doubt, use strict Locking and Validation.
