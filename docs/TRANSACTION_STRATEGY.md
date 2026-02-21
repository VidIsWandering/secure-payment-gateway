# TRANSACTION & CONCURRENCY STRATEGY

System: Secure Payment Gateway
Context: High Concurrency & AES-256 Encrypted Balances

## 1. The Core Challenge

We have two conflicting requirements:

1.  **AES-256 Encryption**: Wallet balances must be encrypted at rest.
2.  **Concurrency Safety**: Prevent race conditions when multiple requests hit the same wallet.

Since we cannot perform arithmetic logic (`balance - x`) inside SQL on an encrypted string, the arithmetic must happen in the **Application Layer (Go)**.

## 2. The Solution: Pessimistic Locking with Decryption Flow

The AI Coding Assistant MUST implement the following strict workflow for any fund movement:

### Step-by-Step Logic

1.  **Start DB Transaction** (`tx`).
2.  **Lock & Fetch**:
    - Execute: `SELECT encrypted_balance FROM wallets WHERE id = $1 FOR UPDATE`.
    - _Why?_ This locks the row. No other transaction can read/write this wallet until we commit.
3.  **Decrypt (Go Layer)**:
    - Use `encryption_service` to decrypt `encrypted_balance` -> `current_balance (float/int)`.
4.  **Business Check**:
    - Check: `if current_balance < transaction_amount` -> Return Error "Insufficient Funds".
5.  **Calculate & Encrypt (Go Layer)**:
    - `new_balance = current_balance - transaction_amount`.
    - Use `encryption_service` to encrypt `new_balance` -> `new_encrypted_balance`.
6.  **Update DB**:
    - Execute: `UPDATE wallets SET encrypted_balance = $1 WHERE id = $2`.
7.  **Log Transaction**:
    - Insert record into `transactions` table.
8.  **Commit Transaction**:
    - Commit `tx`. The row lock is released.

## 3. Idempotency Strategy

To prevent "Replay Attacks" or network retries causing double charges:

1.  **Check Redis First**: Key format `idempotency:{merchant_id}:{ref_id}`.
2.  **Check DB Backup**: If Redis is down/missed, check `idempotency_logs` table.
3.  **Enforce**: If key exists -> Return the **previous result** immediately. Do NOT process logic again.

## 4. Required SQL Queries (for sqlc)

Define these in `db/queries/wallet.sql`:

```sql
-- name: GetWalletForUpdate :one
SELECT * FROM wallets
WHERE id = $1
FOR UPDATE; -- CRITICAL: This is the Pessimistic Lock

-- name: GetWalletByMerchantForUpdate :one
SELECT * FROM wallets
WHERE merchant_id = $1 AND currency = $2
FOR UPDATE;

-- name: UpdateWalletBalance :exec
UPDATE wallets
SET encrypted_balance = $1, updated_at = NOW()
WHERE id = $2;
```

Define these in `db/queries/transaction.sql`:

```sql
-- name: CreateTransaction :one
INSERT INTO transactions (
    reference_id, merchant_id, wallet_id, amount, amount_encrypted,
    transaction_type, status, signature, client_ip, extra_data, original_transaction_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetTransactionByReference :one
SELECT * FROM transactions
WHERE reference_id = $1 AND merchant_id = $2;

-- name: GetTransactionByID :one
SELECT * FROM transactions
WHERE id = $1;

-- name: UpdateTransactionStatus :exec
UPDATE transactions
SET status = $1, processed_at = NOW()
WHERE id = $2;

-- name: CheckRefundExists :one
SELECT COUNT(*) FROM transactions
WHERE original_transaction_id = $1 AND transaction_type = 'REFUND' AND status = 'SUCCESS';
```

Define these in `db/queries/merchant.sql`:

```sql
-- name: CreateMerchant :one
INSERT INTO merchants (username, password_hash, merchant_name, access_key, secret_key_enc, webhook_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMerchantByAccessKey :one
SELECT * FROM merchants
WHERE access_key = $1 AND status = 'ACTIVE';

-- name: GetMerchantByUsername :one
SELECT * FROM merchants
WHERE username = $1 AND status = 'ACTIVE';

-- name: GetMerchantByID :one
SELECT * FROM merchants
WHERE id = $1;
```

Define these in `db/queries/idempotency.sql`:

```sql
-- name: CreateIdempotencyLog :exec
INSERT INTO idempotency_logs (key, transaction_id, response_json)
VALUES ($1, $2, $3);

-- name: GetIdempotencyLog :one
SELECT * FROM idempotency_logs
WHERE key = $1;
```
