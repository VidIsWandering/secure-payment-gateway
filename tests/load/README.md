# Load Testing (k6)

This directory contains load testing scripts utilizing [k6](https://k6.io/) to benchmark the performance and concurrency control mechanisms (Pessimistic Locking / `@Transactional`) of the Secure Payment Gateway under high traffic.

## Installation

Follow the official instructions to install k6: [k6 Installation Guide](https://k6.io/docs/get-started/installation/).

## Test Scenarios

The `payment_load.js` script is designed to simulate realistic payment gateway traffic patterns by executing multiple scenarios concurrently.

### 1. Stress Test (`stress_payments`)
Simulates a steady, high volume of concurrent payment requests.
- **Target:** 200 Virtual Users (VUs)
- **Purpose:** Identifies the system's maximum sustainable throughput (TPS) and assesses the stability of the PostgreSQL Pessimistic Locking algorithm under sustained pressure.

### 2. Spike Test (`spike_payments`)
Simulates sudden, massive surges in traffic (e.g., flash sales, ticket releases).
- **Target:** Spikes up to 500 VUs instantly.
- **Purpose:** Evaluates how the HTTP handlers, Rate Limiter, and PostgeSQL connection pool handle abrupt load changes without dropping transactions.

### 3. Rate Limit Test (`rate_limit_check`)
Intentionally exceeds the defined rate limits (`100 req/min/merchant`) for a specific endpoint.
- **Purpose:** Validates that the Redis-backed `RateLimiter` middleware correctly identifies abuse and correctly responds with `HTTP 429 Too Many Requests`.

## Prerequisites

1. **Start the API Server:**
   Ensure the application is running locally:
   ```bash
   go run cmd/api/main.go
   ```

2. **Prepare Test Data:**
   - Register a Merchant to obtain an `ACCESS_KEY` and `SECRET_KEY`.
   - Ensure the Merchant's VND wallet has sufficient funds (Topup) to prevent continuous `HTTP 402 Insufficient Funds` errors during the test.

## Execution

Run the load test suite by providing the necessary environment variables. Replace the keys with your actual test Merchant credentials:

```bash
k6 run \
  -e BASE_URL=http://localhost:8080/api/v1 \
  -e ACCESS_KEY=ak_your_merchant_access_key \
  -e SECRET_KEY=sk_your_merchant_secret_key \
  payment_load.js
```

## Analyzing Results

k6 will output a comprehensive summary report upon completion. Key metrics to observe:
- **`http_req_duration`**: Indicates the latency of the API. Look at the `p(95)` and `p(99)` values to assess tail latency during DB row locks.
- **`http_reqs`**: The total throughput and requests per second.
- **`checks`**: Ensures the required business assertions passed (e.g., successful signatures, correct HTTP status codes).
