# CORE TRANSACTION LOGIC (ACID + ENCRYPTION)

Focus: Secure Money Transfer with AES-256 and Pessimistic Locking.

## Context

We cannot perform arithmetic on encrypted data. We must Decrypt -> Calculate -> Encrypt inside a Locked Transaction.

## The "Payment" Algorithm

**Input:** `merchant_id`, `amount`, `reference_id`

1.  **Idempotency Check (Layer 1 - Redis)**:

    - Check Redis key `idempotency:{merchant_id}:{reference_id}`.
    - If exists: Return cached response immediately.

2.  **Start Database Transaction (`tx`)**:

    - `tx, err := db.Begin()`

3.  **Lock & Get Wallet (Pessimistic Lock)**:

    - Query: `SELECT encrypted_balance FROM wallets WHERE merchant_id = $1 FOR UPDATE`.
    - _Critical:_ This halts all other transfers for this wallet until commit.

4.  **Secure Decryption**:

    - `current_balance_str = AES_Decrypt(encrypted_balance, system_aes_key)`
    - Convert `current_balance_str` to `BigInt` or `Decimal`.

5.  **Business Rule Check**:

    - If `current_balance < amount`:
      - Rollback `tx`.
      - Return Error `PAY_001`.

6.  **Calculate & Encrypt**:

    - `new_balance = current_balance - amount`.
    - `new_balance_enc = AES_Encrypt(new_balance, system_aes_key)`.

7.  **Persist Changes**:

    - Update Wallet: `UPDATE wallets SET encrypted_balance = new_balance_enc ...`
    - Create Transaction Record: `INSERT INTO transactions ...` (Status: SUCCESS).
    - Save Idempotency Log: `INSERT INTO idempotency_logs ...`

8.  **Commit Transaction**:

    - `tx.Commit()`.

9.  **Post-Process**:
    - Save result to Redis (Idempotency cache).
    - Trigger Webhook Worker (Async).

---

## The "Refund" Algorithm

**Input:** `merchant_id`, `original_reference_id`, `refund_amount (optional)`, `reason`

1.  **Idempotency Check (Layer 1 - Redis)**:

    - Check Redis key `idempotency:{merchant_id}:refund:{original_reference_id}`.
    - If exists: Return cached response immediately.

2.  **Start Database Transaction (`tx`)**:

    - `tx, err := db.Begin()`

3.  **Validate Original Transaction**:

    - Query: `SELECT * FROM transactions WHERE reference_id = $1 AND merchant_id = $2 AND transaction_type = 'PAYMENT'`.
    - If not found: Return Error `PAY_004`.
    - If `status != 'SUCCESS'`: Return Error `PAY_002` ("Cannot refund non-successful transaction").
    - Check no existing REFUND tx linked to this original: `SELECT COUNT(*) FROM transactions WHERE original_transaction_id = $1 AND status = 'SUCCESS'`.
    - If already refunded: Return Error `PAY_003`.

4.  **Determine Refund Amount**:

    - If `refund_amount` provided: validate `refund_amount <= original_amount`.
    - If not provided: `refund_amount = original_amount` (full refund).

5.  **Lock & Get Wallet (Pessimistic Lock)**:

    - Query: `SELECT encrypted_balance FROM wallets WHERE id = $1 FOR UPDATE`.
    - _Critical:_ Same locking strategy as Payment.

6.  **Secure Decryption**:

    - `current_balance_str = AES_Decrypt(encrypted_balance, system_aes_key)`.
    - Convert to Decimal.

7.  **Calculate & Encrypt (ADD back)**:

    - `new_balance = current_balance + refund_amount`.
    - `new_balance_enc = AES_Encrypt(new_balance, system_aes_key)`.

8.  **Persist Changes**:

    - Update Wallet: `UPDATE wallets SET encrypted_balance = new_balance_enc ...`
    - Create Refund Transaction Record: `INSERT INTO transactions ...` (type: REFUND, status: SUCCESS, `original_transaction_id` = original tx id).
    - Update Original Transaction: `UPDATE transactions SET status = 'REVERSED' WHERE id = $1`.
    - Save Idempotency Log.

9.  **Commit Transaction**:

    - `tx.Commit()`.

10. **Post-Process**:
    - Save result to Redis (Idempotency cache).
    - Trigger Webhook Worker (Async, event_type: `REFUND_UPDATE`).

---

## The "Topup" Algorithm

**Input:** `merchant_id`, `amount`, `currency`

_Note: In this simulated system, topup is triggered by authenticated merchant (JWT). In production, it would be triggered by bank transfer verification._

1.  **Start Database Transaction (`tx`)**:

    - `tx, err := db.Begin()`

2.  **Lock & Get Wallet (Pessimistic Lock)**:

    - Query: `SELECT encrypted_balance FROM wallets WHERE merchant_id = $1 AND currency = $2 FOR UPDATE`.

3.  **Secure Decryption**:

    - `current_balance_str = AES_Decrypt(encrypted_balance, system_aes_key)`.
    - Convert to Decimal.

4.  **Calculate & Encrypt (ADD funds)**:

    - `new_balance = current_balance + amount`.
    - `new_balance_enc = AES_Encrypt(new_balance, system_aes_key)`.

5.  **Persist Changes**:

    - Update Wallet: `UPDATE wallets SET encrypted_balance = new_balance_enc ...`
    - Create Transaction Record: `INSERT INTO transactions ...` (type: TOPUP, status: SUCCESS).

6.  **Commit Transaction**:

    - `tx.Commit()`.

7.  **Post-Process**:
    - Log topup event (audit trail).

---

## Common Rules for ALL Transaction Types

- **Always** use `defer tx.Rollback(ctx)` immediately after `Begin()`.
- **Always** encrypt amounts with AES-256 for the `amount_encrypted` field.
- **Never** log decrypted balances in plain text (use zerolog with masked fields).
- **Always** set `processed_at = NOW()` when transaction reaches final state.
- **Context propagation**: Pass `context.Context` through entire chain for timeout/cancellation.
