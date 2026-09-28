# Load Testing (k6)

This directory contains load testing scripts utilizing [k6](https://k6.io/) to benchmark the performance and concurrency control mechanisms (pessimistic locking with `SELECT ... FOR UPDATE`) of the Secure Payment Gateway under high traffic.

## Installation

Follow the official instructions to install k6: [k6 Installation Guide](https://k6.io/docs/get-started/installation/).

## Test Scenarios

The `payment_load.js` script is designed to simulate realistic payment gateway traffic patterns by executing multiple scenarios concurrently.

### 1. Stress Test (`stress_payments`)
Simulates a steady, high volume of concurrent payment requests.
- **Target:** ramps 0 → 50 Virtual Users (VUs), holds for 1 minute (~1m45s total)
- **Purpose:** Identifies the system's maximum sustainable throughput (TPS) and assesses the stability of the PostgreSQL Pessimistic Locking algorithm under sustained pressure.

### 2. Spike Test (`spike_payments`)
Simulates sudden, massive surges in traffic (e.g., flash sales, ticket releases).
- **Target:** jumps from 5 to 150 VUs in 5 seconds and holds for 20 seconds (~50s total)
- **Purpose:** Evaluates how the HTTP handlers, Rate Limiter, and PostgreSQL connection pool handle abrupt load changes without dropping transactions.

### 3. Rate Limit Test (`rate_limit_check`)
Sends 200 req/min for 1 minute — twice the default payment limit (`100 req/min/merchant`).
- **Purpose:** Validates that the Redis-backed `RateLimiter` middleware correctly identifies abuse and correctly responds with `HTTP 429 Too Many Requests`.

## Prerequisites

1. **Start the stack:**
   ```bash
   docker compose up -d
   ```

2. **Prepare test data** — registers a merchant, funds its wallet with 500M VND and prints the credentials:
   ```bash
   python3 scripts/demo/setup_loadtest.py
   ```

3. **(Stress/spike only) raise the payment rate limit**, otherwise most requests return `429`:
   ```bash
   SPG_RATELIMIT_PAYMENTS=50000 docker compose up -d app
   ```
   Run `docker compose up -d app` again afterwards to restore the defaults — the rate-limit
   scenario only produces `429`s with the default limit.

## Execution

Run the load test suite by providing the necessary environment variables. Replace the keys with your actual test Merchant credentials:

```bash
k6 run \
  -e BASE_URL=http://localhost:8080/api/v1 \
  -e ACCESS_KEY=ak_your_merchant_access_key \
  -e SECRET_KEY=sk_your_merchant_secret_key \
  tests/load/payment_load.js
```

## Analyzing Results

k6 will output a comprehensive summary report upon completion. Key metrics to observe:
- **`http_req_duration`**: Indicates the latency of the API. Look at the `p(95)` and `p(99)` values to assess tail latency during DB row locks.
- **`http_reqs`**: The total throughput and requests per second.
- **`checks`**: Ensures the required business assertions passed (e.g., successful signatures, correct HTTP status codes).

## Reference Results

Two consecutive runs on a laptop (Intel Core i7-1355U, 20 GB RAM, WSL2, single instance via
`docker compose`), all requests debiting the same wallet:

| Metric | Run 1 | Run 2 |
| --- | --- | --- |
| Requests | 26,214 | 23,396 |
| Failed requests / 5xx | 0 | 0 |
| p50 / p95 latency | 8 ms / 283 ms | 6 ms / 536 ms |

After both runs the wallet balance matched the ledger exactly (top-up − sum of successful
payments), with no duplicate `reference_id`s. The rate-limit scenario only reports `429`s
when the default payment limit is active (see step 3 above).
