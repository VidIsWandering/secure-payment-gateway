# WEBHOOK SPECIFICATION

When a payment, refund or top-up completes, the gateway sends a `POST` request to the
merchant's registered `webhook_url`.

## 1. Delivery Guarantees

- **Transactional outbox.** The webhook is recorded in `webhook_delivery_logs` inside the
  same database transaction as the money movement. A webhook exists if and only if the
  transaction committed, and pending webhooks survive restarts.
- **At-least-once.** A background dispatcher delivers due webhooks. A webhook may be
  delivered more than once (e.g. if the gateway stops after the merchant responded but
  before the result was recorded). De-duplicate on the `X-Webhook-Id` header.
- **Idempotent requests do not re-notify.** Replaying a request with the same
  `reference_id` returns the original result without queueing another webhook.
- **Multiple replicas are safe.** Deliveries are claimed with `FOR UPDATE SKIP LOCKED` and a
  one-minute lease, so each attempt is made by a single dispatcher.

## 2. Retry Policy

- **Success**: any `2xx` response within 10 seconds.
- **Attempts**: 1 initial attempt + 5 retries (6 in total), then the delivery is marked `FAILED`.
- **Retry intervals** (exponential backoff): 15s, 60s, 2m, 5m, 10m.

## 3. Request

| Header | Value |
| --- | --- |
| `Content-Type` | `application/json` |
| `X-Webhook-Id` | UUID of the delivery — identical across retries of the same webhook |

```json
{
  "event_type": "PAYMENT_UPDATE",
  "data": {
    "merchant_order_id": "ORD-2026-001",
    "gateway_transaction_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "SUCCESS",
    "amount": 500000,
    "currency": "VND",
    "reason": "Transaction SUCCESS",
    "timestamp": 1708092000
  },
  "signature": "5f8a…"
}
```

`event_type` is one of `PAYMENT_UPDATE`, `REFUND_UPDATE`, `TOPUP_UPDATE`.
`timestamp` is when the event was recorded, not when this attempt was sent.

## 4. Verifying the Signature

`signature` is the lowercase hex `HMAC-SHA256(secret_key, json(data))`, where `json(data)` is
the `data` object serialised exactly as received. It is computed at delivery time with the
merchant's **current** secret key, so after a key rotation pending webhooks are signed with
the new key.

```python
import hashlib, hmac, json

def verify(body: bytes, secret_key: str) -> bool:
    payload = json.loads(body)
    data = json.dumps(payload["data"], separators=(",", ":"), ensure_ascii=False)  # matches Go encoding/json
    expected = hmac.new(secret_key.encode(), data.encode(), hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, payload["signature"])
```
