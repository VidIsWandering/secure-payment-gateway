# Demo Walkthrough Guide: Secure Payment Gateway

This document provides a structured guide to demonstrating the core capabilities of the Secure Payment Gateway. The demonstrations are categorized into Functional Operations, Security Mechanisms, Concurrency Handlers, and Performance Load Testing.

Before proceeding, ensure the entire environment is running via `docker compose up -d`. All scripts mentioned below are located in the `scripts/demo/` directory.

## 1. System Overview & Interfaces

The system exposes several distinct interfaces for operation and monitoring:

*   **API Documentation (Swagger UI):** `http://localhost:8080/swagger`
    *   Provides interactive documentation for all endpoints, including authentication schemes (JWT for Dashboard, HMAC-SHA256 for Payments).
*   **Infrastructure Monitoring (Prometheus):** `http://localhost:9090`
    *   Scrapes telemetry data and hardware metrics from the application, database, and cache.
*   **Analytics Dashboard (Grafana):** `http://localhost:3000`
    *   Visualizes real-time performance, transaction throughput, request latency, and HTTP error rates.

---

## 2. Functional Requirements (Happy Path)

**Objective:** Demonstrate the complete lifecycle of a merchant using the gateway, from registration to processing a payment and receiving a webhook callback.

**Execution Steps:**

1.  **Start the Webhook Receiver:** In a new terminal, launch the mock merchant server to listen for asynchronous callbacks.
    ```bash
    python3 scripts/demo/demo_webhook_server.py
    ```
2.  **Execute the Happy Path Script:** In another terminal, run the functional demo script.
    ```bash
    python3 scripts/demo/demo_payment.py
    ```
    
**Observing the Workflow:**
*   The script registers a merchant and retrieves the `access_key` and `secret_key`.
*   It generates an HMAC-SHA256 digital signature dynamically using the `secret_key` and initiates a simulated payment.
*   The Webhook Server terminal will immediately log an incoming HTTP POST request containing the `SUCCESS` status of the transaction, validating the callback functionality.

> **Note (Docker Networking):** The application runs inside a Docker container. The webhook URL uses `host.docker.internal` to route callbacks from the container to the host machine where `demo_webhook_server.py` is running. This is automatically configured via `extra_hosts` in `docker-compose.yml`.

---

## 3. Advanced Security & Edge Cases

**Objective:** Validate the implementation of critical security measures, specifically preventing data tampering and replay attacks.

**Execution Steps:**

```bash
python3 scripts/demo/demo_security.py
```

**Demonstrated Concepts:**

1.  **Tampering Attack Prevention:** The script generates a valid signature for a 10,000 VND internal transaction payload. However, the outgoing HTTP request body is tampered to request 1,000,000 VND. The system recalculates the signature (H') and rejects the request as H ≠ H'.
2.  **Replay Attack Prevention:** The system captures a wholly valid request and resends it identically. The application intercepts the request via the Redis nonce-store checks and rejects the duplicate to prevent double-charging.
3.  **Timestamp Expiration:** A correctly signed request is dispatched using a timestamp older than the allowed tolerance window (e.g., > 5 minutes). The system rejects the request to prevent delayed replay vectors.
4.  **SSRF Webhook Validation:** (Internal) Merchant webhook URLs pointing to private subnets (e.g., `192.168.x.x`, `127.0.0.1` non-development environments) are blocked to prevent Server-Side Request Forgery.

---

## 4. ACID Compliance, Concurrency & Idempotency

**Objective:** Verify Data Consistency and ACID compliance under concurrent traffic environments to prevent negative balances and duplicate transaction processing.

**Execution Steps:**

```bash
python3 scripts/demo/demo_concurrency.py
```

**Demonstrated Concepts:**

1.  **Pessimistic Locking (`SELECT FOR UPDATE`):** The script tops up 10,000 VND to a wallet and fires 10 multi-threaded requests *simultaneously*, each attempting to deduct 10,000 VND via different reference IDs. The database serializes the conflicting transactions. Only the first request succeeds; the remaining 9 fail with an `Insufficient Funds` response. The wallet balance never falls below zero.
2.  **Idempotency Handling:** The script simulates a network retry logic by firing 10 simultaneous payment requests utilizing the *same* Reference ID. The system detects the collision and safely processes exactly 1 transaction. The subsequent 9 identical requests are returned safely without duplicating the balance deduction.

---

## 5. Performance and Load Testing

**Objective:** Assess system resilience, API rate restrictions, and Database Locking throughput levels under heavy traffic utilizing **k6**.

*Prerequisite: Install k6 (`sudo apt install k6` or equivalent).*

**Execution Steps:**

1.  **Run the automated setup script** to create a test merchant with a pre-funded wallet:
    ```bash
    python3 scripts/demo/setup_loadtest.py
    ```
    The script registers a merchant, funds 500M VND, verifies HMAC auth, and prints the next steps.

2.  **Raise the payment rate limit** for stress/spike testing (phases 1 & 2 need higher throughput):
    ```bash
    SPG_RATELIMIT_PAYMENTS=50000 docker compose up -d app
    ```
    > This overrides the default 100 req/min payment rate limit so the stress test can measure actual DB locking throughput instead of being dominated by HTTP 429 responses.

3.  **Export credentials and run the load test:**
    ```bash
    # Paste the export commands printed by setup_loadtest.py:
    export ACCESS_KEY="<printed-value>"
    export SECRET_KEY="<printed-value>"
    export BASE_URL="http://localhost:8080/api/v1"

    k6 run tests/load/payment_load.js
    ```

4.  **Restore default rate limits** after testing:
    ```bash
    docker compose up -d app
    ```

**Test Phases:**

| Phase | Scenario | VUs | Duration | What It Tests |
|-------|----------|-----|----------|---------------|
| 1 | Stress Test | 0→50 | ~1m45s | Sustained DB locking throughput under concurrent load |
| 2 | Spike Test | 5→150 | ~50s | System resilience under sudden traffic bursts (Flash Sale) |
| 3 | Rate Limit | 200 req/min | 1m | HTTP 429 defense mechanism with default rate limits |

**Custom Metrics to Observe:**
*   `payment_success` — Successful payments (HTTP 201)
*   `payment_denied` — Insufficient funds (HTTP 402, expected when wallet runs dry)
*   `payment_rate_limited` — Rate-limited requests (HTTP 429)
*   `payment_errors` — Unexpected errors (5xx, etc.)
*   `payment_success_rate` — Ratio of successful business outcomes (201 + 402) vs errors
*   `payment_latency_ms` — Transaction processing time

**Grafana Telemetry:** While the k6 script executes, view the Grafana Dashboard (`http://localhost:3000`) for real-time visualization of HTTP Latency (ms), TPS (Transactions Per Second), and Error rate behaviors.
