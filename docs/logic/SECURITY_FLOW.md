# SECURITY IMPLEMENTATION LOGIC

Focus: HMAC-SHA256 Signature & Replay Attack Prevention

## 1. Authentication Middleware Logic

All incoming requests to `/payments` MUST pass this pipeline before reaching the Controller.

### Step 1: Replay Attack Check

**Requirement:** Verify `X-Timestamp` and `X-Nonce`.

1.  **Check Timestamp:**
    - Get `current_time`.
    - If `abs(current_time - X-Timestamp) > 60 seconds`:
      - Return Error `SEC_003` (Timestamp Expired).
2.  **Check Nonce (Redis):**
    - Key format: `nonce:{merchant_id}:{X-Nonce}`.
    - Command: `SET key 1 EX 120 NX` (Set if Not Exists, expire in 120s).
    - If result is `FALSE` (Key exists):
      - Return Error `SEC_004` (Nonce Used).

### Step 2: Digital Signature Verification

**Requirement:** Verify integrity using HMAC-SHA256.

**Algorithm:**

1.  **Retrieve Keys:** Look up `secret_key_enc` from DB using `X-Merchant-Access-Key`. Decrypt it to get raw `secret_key`.
2.  **Construct Payload (Canonical String):**
    - Format: `{METHOD}|{PATH}|{TIMESTAMP}|{NONCE}|{BODY_STRING}`
    - Example: `POST|/api/v1/payments|1708092000|abc123nonce|{"amount":50000...}`
3.  **Calculate Hash:**
    - `expected_signature = HMAC-SHA256(secret_key, payload)`
    - Output format: Hexadecimal string (lowercase).
4.  **Compare:**
    - If `expected_signature != X-Signature`:
      - Return Error `SEC_002` (Invalid Signature).

## 2. Rate Limiting Strategy

**Purpose:** Protect against DDoS and brute-force attacks. Redis-backed using `ulule/limiter/v3`.

### Rules (Per Merchant, identified by `X-Merchant-Access-Key`)

| Endpoint Pattern        | Limit        | Window     | Strategy       |
| ----------------------- | ------------ | ---------- | -------------- |
| `POST /payments`        | 100 requests | Per minute | Sliding Window |
| `POST /payments/refund` | 30 requests  | Per minute | Sliding Window |
| `POST /auth/login`      | 10 requests  | Per minute | Fixed Window   |
| `POST /auth/register`   | 5 requests   | Per hour   | Fixed Window   |
| `GET /dashboard/*`      | 60 requests  | Per minute | Sliding Window |
| `POST /wallets/topup`   | 20 requests  | Per minute | Sliding Window |

### Implementation Details

1. **Redis Key Format:** `ratelimit:{merchant_access_key}:{endpoint_group}:{window_id}`
2. **Response Headers** (always included):
   - `X-RateLimit-Limit`: Max allowed requests in window
   - `X-RateLimit-Remaining`: Requests left in current window
   - `X-RateLimit-Reset`: Unix timestamp when window resets
3. **When Exceeded:** Return HTTP `429 Too Many Requests` with `Retry-After` header.
4. **Global Fallback:** If Redis is unavailable, apply in-memory rate limit as fallback (degraded mode, stricter limits).

## 3. JWT Authentication (for Dashboard/Management APIs)

**Purpose:** Merchant dashboard and management endpoints use JWT instead of HMAC signatures.

### Token Specification

- **Algorithm:** HS256 (HMAC-SHA256 with server-side secret)
- **Expiry:** 24 hours
- **Claims:**
  - `sub`: Merchant UUID
  - `access_key`: Merchant's Access Key
  - `iat`: Issued at timestamp
  - `exp`: Expiry timestamp

### Flow

1. Merchant calls `POST /auth/login` with username + password.
2. Server verifies password against `bcrypt` hash in DB.
3. If valid: Generate JWT, return token.
4. Merchant includes `Authorization: Bearer <token>` in subsequent dashboard requests.
5. `AuthMiddleware` validates token, extracts `merchant_id`, injects into context.

### Protected Routes

- `GET /wallets/balance`
- `POST /wallets/topup`
- `GET /dashboard/stats`
- `GET /transactions`
