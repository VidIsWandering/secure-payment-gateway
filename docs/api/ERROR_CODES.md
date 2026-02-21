# SYSTEM ERROR CODES

## 1. Standard Error Response Structure

All API responses with HTTP Status `4xx` or `5xx` MUST follow this JSON format exactly. This helps the Merchant's system to parse errors automatically.

```json
{
  "error_code": "PAY_001",
  "message": "Insufficient balance in wallet",
  "request_id": "req_550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2024-02-20T10:00:00Z"
}
```

## 2. Error Code Registry

### A. Security & Authentication (Prefix: SEC)

These errors occur in the `middleware` layer before reaching business logic.

| Code      | HTTP Status | Description        | Recommended Action                                           |
| :-------- | :---------- | :----------------- | :----------------------------------------------------------- |
| `SEC_001` | 401         | Invalid Access Key | Check `X-Merchant-Access-Key` header.                        |
| `SEC_002` | 401         | Invalid Signature  | Verify HMAC-SHA256 logic using Secret Key.                   |
| `SEC_003` | 403         | Timestamp Expired  | Request is older than 60s (Replay Attack Protection).        |
| `SEC_004` | 403         | Nonce Used         | `X-Nonce` has been used recently (Replay Attack Protection). |

### B. Payment Business Logic (Prefix: PAY)

These errors occur in the `service` layer during transaction processing.

| Code      | HTTP Status | Description                    | Recommended Action                                                              |
| :-------- | :---------- | :----------------------------- | :------------------------------------------------------------------------------ |
| `PAY_001` | 402         | Insufficient Funds             | Wallet balance is lower than transaction amount.                                |
| `PAY_002` | 400         | Invalid Amount                 | Amount must be positive integer. Check Currency.                                |
| `PAY_003` | 409         | Duplicate Transaction          | `reference_id` already exists. Check Idempotency.                               |
| `PAY_004` | 404         | Merchant/Wallet Not Found      | Check Merchant ID or Wallet ID UUIDs.                                           |
| `PAY_005` | 422         | Transaction Limit Exceeded     | Merchant has reached daily/monthly limit.                                       |
| `PAY_006` | 400         | Invalid Refund                 | Original transaction not eligible for refund (not SUCCESS or already reversed). |
| `PAY_007` | 400         | Refund Amount Exceeds Original | Refund amount cannot be greater than original transaction amount.               |

### C. Authentication (Prefix: AUTH)

These errors occur during merchant registration and login.

| Code       | HTTP Status | Description             | Recommended Action                       |
| :--------- | :---------- | :---------------------- | :--------------------------------------- |
| `AUTH_001` | 401         | Invalid Credentials     | Username or password is incorrect.       |
| `AUTH_002` | 409         | Username Already Exists | Choose a different username.             |
| `AUTH_003` | 401         | Invalid/Expired JWT     | Token is malformed or expired. Re-login. |
| `AUTH_004` | 403         | Merchant Suspended      | Account is suspended. Contact support.   |

### D. Rate Limiting (Prefix: RATE)

| Code       | HTTP Status | Description         | Recommended Action                                         |
| :--------- | :---------- | :------------------ | :--------------------------------------------------------- |
| `RATE_001` | 429         | Rate Limit Exceeded | Too many requests. Retry after `Retry-After` header value. |

### E. System & Infrastructure (Prefix: SYS)

These errors indicate internal failures.

| Code      | HTTP Status | Description                | Recommended Action                                          |
| :-------- | :---------- | :------------------------- | :---------------------------------------------------------- |
| `SYS_001` | 500         | Internal Database Error    | Contact Support. Do not retry immediately.                  |
| `SYS_002` | 503         | Lock Acquisition Timeout   | High concurrency on wallet. Retry with Exponential Backoff. |
| `SYS_003` | 500         | Encryption Service Failure | AES key missing or rotation error.                          |
