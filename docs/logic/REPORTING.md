# REPORTING & DASHBOARD LOGIC

Focus: Transaction statistics, cash flow analytics, and transaction history.

## 1. Context

The Dashboard provides Merchants with real-time visibility into their transaction activity.
All dashboard endpoints require JWT authentication (not HMAC signature).

## 2. Dashboard Statistics (`GET /dashboard/stats`)

### Input

- `merchant_id` (from JWT token)
- `period`: `today | week | month | all`

### Aggregation Queries

```sql
-- Transaction counts by status
SELECT
    COUNT(*) as total_transactions,
    COUNT(*) FILTER (WHERE status = 'SUCCESS') as successful,
    COUNT(*) FILTER (WHERE status = 'FAILED') as failed,
    COUNT(*) FILTER (WHERE status = 'REVERSED') as reversed
FROM transactions
WHERE merchant_id = $1
  AND created_at >= $2; -- period start date

-- Revenue and refund totals
SELECT
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'PAYMENT' AND status = 'SUCCESS'), 0) as total_revenue,
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'REFUND' AND status = 'SUCCESS'), 0) as total_refunded,
    COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'TOPUP' AND status = 'SUCCESS'), 0) as total_topup
FROM transactions
WHERE merchant_id = $1
  AND created_at >= $2;
```

### Period Calculation

| Period  | Start Date                                  |
| ------- | ------------------------------------------- |
| `today` | Start of current day (00:00:00 UTC)         |
| `week`  | Start of current week (Monday 00:00:00 UTC) |
| `month` | Start of current month (1st 00:00:00 UTC)   |
| `all`   | No filter (all time)                        |

### Response

- `net_balance` is calculated as: `total_revenue - total_refunded + total_topup`
- Note: `net_balance` is computed from transaction records, NOT from the encrypted wallet balance (which is the source of truth for fund movements).

## 3. Transaction History (`GET /transactions`)

### Input

- `merchant_id` (from JWT token)
- `page`, `page_size` (pagination, max 100 per page)
- `status` (optional filter: PENDING, SUCCESS, FAILED, REVERSED)
- `type` (optional filter: PAYMENT, REFUND, TOPUP)
- `from`, `to` (optional date range filter)

### Query Pattern

```sql
-- name: ListTransactions :many
SELECT id, reference_id, amount, transaction_type, status, created_at, processed_at, extra_data
FROM transactions
WHERE merchant_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND ($3::varchar IS NULL OR transaction_type = $3)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
  AND ($5::timestamptz IS NULL OR created_at <= $5)
ORDER BY created_at DESC
LIMIT $6 OFFSET $7;

-- name: CountTransactions :one
SELECT COUNT(*)
FROM transactions
WHERE merchant_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND ($3::varchar IS NULL OR transaction_type = $3)
  AND ($4::timestamptz IS NULL OR created_at >= $4)
  AND ($5::timestamptz IS NULL OR created_at <= $5);
```

### Security Notes

- **Never return** `amount_encrypted`, `signature`, or `wallet_id` in list responses.
- **Never return** transactions belonging to other merchants.
- `merchant_id` is always extracted from JWT, never from query params.

## 4. Performance Considerations

- Dashboard stats queries benefit from indexes: `idx_transactions_merchant`, `idx_transactions_status`, `idx_transactions_created`.
- For high-traffic merchants, consider caching stats in Redis with short TTL (30-60s).
- Pagination uses `LIMIT/OFFSET` for simplicity; for very large datasets, consider keyset pagination.
