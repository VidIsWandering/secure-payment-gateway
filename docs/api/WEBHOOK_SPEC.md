# WEBHOOK SPECIFICATION

When a transaction reaches a final state (SUCCESS/FAILED), the system MUST send a POST request to the Merchant's registered `webhook_url`.

## 1. Retry Policy

- **Strategy**: Exponential Backoff.
- **Attempts**: Max 5 times.
- **Intervals**: 15s, 60s, 2m, 5m, 10m.

## 2. Payload Structure (JSON)

The payload allows the Merchant to update their own order status.

```json
{
  "event_type": "PAYMENT_UPDATE",
  "data": {
    "merchant_order_id": "ORD-2026-001",
    "gateway_transaction_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "SUCCESS",
    "amount": 500000,
    "currency": "VND",
    "reason": "Transaction completed successfully",
    "timestamp": 1708092000
  },
  "signature": "hmac_sha256_of_payload_content"
}
```
