<div align="center">

# Secure Payment Gateway

**A payment gateway API in Go built around one guarantee: money is never lost or double-spent —
even under concurrent traffic, network retries and replayed requests.**

[![CI](https://github.com/VidIsWandering/secure-payment-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/VidIsWandering/secure-payment-gateway/actions/workflows/ci.yml)
[![CodeQL](https://github.com/VidIsWandering/secure-payment-gateway/actions/workflows/codeql.yml/badge.svg)](https://github.com/VidIsWandering/secure-payment-gateway/actions/workflows/codeql.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/VidIsWandering/secure-payment-gateway)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/VidIsWandering/secure-payment-gateway)](https://goreportcard.com/report/github.com/VidIsWandering/secure-payment-gateway)
[![Release](https://img.shields.io/github/v/release/VidIsWandering/secure-payment-gateway)](https://github.com/VidIsWandering/secure-payment-gateway/releases)
[![License: MIT](https://img.shields.io/github/license/VidIsWandering/secure-payment-gateway)](LICENSE)

[Features](#features) · [Quick Start](#quick-start) · [Request Signing](#request-signing) · [API](#api-reference) · [Benchmarks](#load-testing--benchmarks) · [Design Decisions](#design-decisions) · [Docs](#documentation)

<img src="docs/images/dashboard.png" alt="Merchant dashboard after a load test" width="900">

</div>

## Features

| | |
| --- | --- |
| 💸 **Payments, refunds & top-ups** | ACID transactions with `SELECT ... FOR UPDATE` pessimistic locking on the wallet row |
| 🔁 **Idempotency** | Two layers — Redis fast path, PostgreSQL `UNIQUE(merchant_id, reference_id)` as the source of truth |
| ✍️ **Signed requests** | HMAC-SHA256 over `METHOD\|PATH\|TIMESTAMP\|NONCE\|BODY`, constant-time verification |
| 🛡️ **Replay protection** | ±60 s timestamp window + single-use nonces stored in Redis; fails closed if Redis is down |
| 🔐 **Encryption at rest** | Wallet balances, amounts and merchant secret keys encrypted with AES-256-GCM |
| 🔑 **Credentials** | Argon2id password hashing, JWT sessions for the dashboard, rotatable API keys |
| 📣 **Webhooks** | Signed payloads, exponential-backoff retries (15 s → 10 min), SSRF-safe URL validation, delivery log |
| 🚦 **Rate limiting** | Redis-backed per-merchant limits per endpoint group |
| 🧾 **Audit trail** | Every write operation recorded with actor, IP and action |
| 📊 **Observability** | Prometheus metrics + pre-provisioned Grafana dashboard |
| 🖥️ **Merchant dashboard** | Embedded web UI (`go:embed`): revenue stats, history, API keys, sandbox checkout |
| 📘 **API docs** | OpenAPI 3 spec with Swagger UI at `/swagger` |

## Screenshots

| Grafana monitoring | Swagger UI |
| --- | --- |
| <img src="docs/images/grafana.png" alt="Grafana dashboard"> | <img src="docs/images/swagger.png" alt="Swagger UI"> |
| **Landing page** | **Sandbox checkout** |
| <img src="docs/images/landing.png" alt="Landing page"> | <img src="docs/images/checkout-demo.png" alt="Checkout demo"> |

## Architecture

```mermaid
graph TD
    Client["Merchant server / Dashboard"] -->|HTTPS| Router["Gin router"]

    subgraph HTTP["HTTP adapter"]
        Router --> MW["Middleware<br/>request ID · CORS · sanitizer · timeout · metrics"]
        MW --> HMAC["HMAC auth<br/>timestamp · nonce · signature"]
        MW --> JWT["JWT auth"]
        HMAC --> RL["Rate limiter"]
        JWT --> RL
        RL --> Handlers["Handlers + DTO validation"]
    end

    subgraph App["Application services"]
        Handlers --> PaySvc["Payment service"]
        Handlers --> OtherSvc["Auth · Merchant · Reporting"]
        PaySvc --> Crypto["AES-256-GCM · HMAC-SHA256"]
        PaySvc -. async .-> Webhook["Webhook service<br/>retry with backoff"]
    end

    subgraph Storage
        Redis[("Redis<br/>nonces · idempotency cache · rate limits")]
        PG[("PostgreSQL<br/>wallets FOR UPDATE · transactions · audit")]
    end

    HMAC -.-> Redis
    RL -.-> Redis
    PaySvc --> PG
    PaySvc -.-> Redis
    OtherSvc --> PG
    Webhook --> MerchantURL["Merchant webhook URL"]
```

The codebase follows Clean Architecture (ports & adapters): `internal/core` has no
infrastructure dependencies, services depend only on interfaces, and PostgreSQL/Redis/Gin
live in `internal/adapter`. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the layer
rules and full directory layout.

### Payment flow

```mermaid
sequenceDiagram
    autonumber
    participant M as Merchant
    participant A as API
    participant R as Redis
    participant P as PostgreSQL

    M->>A: POST /api/v1/payments + X-Merchant-Access-Key, X-Timestamp, X-Nonce, X-Signature
    A->>A: Reject if timestamp is outside ±60 s
    A->>R: Store nonce (SET NX, TTL 120 s) — reject if already used
    A->>A: Verify HMAC-SHA256 signature (constant time)
    A->>R: Idempotency lookup → return cached result if found
    A->>P: Idempotency lookup (source of truth)
    A->>P: BEGIN · SELECT wallet ... FOR UPDATE
    A->>A: Decrypt balance → check funds → debit → re-encrypt
    A->>P: UPDATE wallet · INSERT transaction · INSERT idempotency log · COMMIT
    A->>R: Cache response (24 h)
    A-->>M: 201 Created
    A-)M: Webhook (async, signed, retried with backoff)
```

## Quick Start

**Prerequisites:** Docker with Docker Compose. For local development also Go 1.26+ and Make.

```bash
git clone https://github.com/VidIsWandering/secure-payment-gateway.git
cd secure-payment-gateway
docker compose up -d
```

| Service | URL |
| --- | --- |
| Web UI & merchant dashboard | http://localhost:8080 (`/register`, `/login`, `/dashboard`) |
| Sandbox checkout | http://localhost:8080/checkout-demo |
| Swagger UI | http://localhost:8080/swagger |
| Health check | http://localhost:8080/health |
| Prometheus metrics | http://localhost:8080/metrics |
| Grafana | http://localhost:3005 (`admin` / `admin`) |
| Prometheus | http://localhost:9090 |

The API applies the versioned database migrations automatically on startup. To see the gateway in action,
run the [demo scripts](#demo-scripts).

> [!NOTE]
> The compose file ships **development defaults** (sample AES key, Grafana `admin/admin`,
> `sslmode=disable`). Override `JWT_SECRET` / `AES_KEY` and harden these before exposing the
> stack anywhere.

### Use the published image

Every release publishes a multi-arch (amd64/arm64) image with an SBOM and signed build
provenance to the GitHub Container Registry:

```bash
docker pull ghcr.io/vidiswandering/secure-payment-gateway:latest
gh attestation verify oci://ghcr.io/vidiswandering/secure-payment-gateway:latest \
  --owner VidIsWandering   # optional: verify provenance
```

### Run the API from source

```bash
docker compose up -d postgres redis   # PostgreSQL on host port 5435, Redis on 6379

cp .env.example .env
# Fill in the two required secrets:
#   SPG_JWT_SECRET=$(openssl rand -base64 48)
#   SPG_AES_KEY=$(openssl rand -hex 32)
set -a && source .env && set +a

make run          # or: go run ./cmd/api — applies pending migrations on startup
make build        # static binary in bin/spg-api

# Manage migrations explicitly (golang-migrate)
export DATABASE_URL="postgres://postgres:postgres@localhost:5435/payment_gateway?sslmode=disable"
make migrate-version
make migrate-create name=add_payouts
```

## Request Signing

Money-moving merchant endpoints (`POST /api/v1/payments`, `POST /api/v1/payments/refund`)
require four headers:

| Header | Value |
| --- | --- |
| `X-Merchant-Access-Key` | Merchant access key |
| `X-Timestamp` | Unix time in seconds — must be within ±60 s of server time |
| `X-Nonce` | Unique random string per request (e.g. UUID v4) |
| `X-Signature` | Lowercase hex `HMAC-SHA256(secret_key, canonical_string)` |

```text
canonical_string = METHOD + "|" + PATH + "|" + TIMESTAMP + "|" + NONCE + "|" + RAW_BODY
```

```python
import hashlib, hmac, json, time, uuid, requests

ACCESS_KEY, SECRET_KEY = "<access_key>", "<secret_key>"   # from /dashboard → Developer Settings

path = "/api/v1/payments"
body = json.dumps({"reference_id": "ORDER-1001", "amount": 150000, "currency": "VND"})
ts, nonce = str(int(time.time())), str(uuid.uuid4())

canonical = f"POST|{path}|{ts}|{nonce}|{body}"
signature = hmac.new(SECRET_KEY.encode(), canonical.encode(), hashlib.sha256).hexdigest()

resp = requests.post(f"http://localhost:8080{path}", data=body, headers={
    "Content-Type": "application/json",
    "X-Merchant-Access-Key": ACCESS_KEY,
    "X-Timestamp": ts,
    "X-Nonce": nonce,
    "X-Signature": signature,
})
print(resp.status_code, resp.json())
```

Sign the **exact bytes** you send. Re-serialising the JSON after signing will change the
body and fail verification. Error codes are listed in
[docs/api/ERROR_CODES.md](docs/api/ERROR_CODES.md).

## API Reference

Full specification: [docs/api/openapi.yaml](docs/api/openapi.yaml) (served at `/swagger`).

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | — | Register a merchant (creates merchant + wallet atomically) |
| `POST` | `/api/v1/auth/login` | — | Log in and obtain a JWT |
| `POST` | `/api/v1/payments` | HMAC signature | Create a payment |
| `POST` | `/api/v1/payments/refund` | HMAC signature | Refund a payment (full or partial) |
| `GET` | `/api/v1/payments/:id/status` | JWT | Get payment status |
| `POST` | `/api/v1/wallets/topup` | JWT | Top up the wallet (sandbox funding) |
| `GET` | `/api/v1/wallets/balance` | JWT | Get wallet balance |
| `GET` | `/api/v1/merchants/me` | JWT | Get merchant profile |
| `PUT` | `/api/v1/merchants/me/webhook` | JWT | Update webhook URL (SSRF-validated) |
| `POST` | `/api/v1/merchants/me/rotate-keys` | JWT | Rotate API keys |
| `GET` | `/api/v1/dashboard/stats` | JWT | Revenue and success-rate summary |
| `GET` | `/api/v1/transactions` | JWT | Paginated transaction history |
| `GET` | `/health` | — | Deep health check (PostgreSQL + Redis) |
| `GET` | `/metrics` | — | Prometheus metrics |
| `GET` | `/swagger` | — | Swagger UI (disabled in `release` mode) |

Default rate limits per merchant: payments 100/min, refunds 30/min, top-ups 20/min,
dashboard 60/min, login 10/min, registration 5/hour.

## Configuration

Configuration is loaded from [config/config.yaml](config/config.yaml) and overridden by
environment variables with the `SPG_` prefix. [.env.example](.env.example) lists every
variable.

| Variable | Default | Description |
| --- | --- | --- |
| `SPG_JWT_SECRET` | — | **Required.** JWT signing key, ≥ 32 characters |
| `SPG_AES_KEY` | — | **Required.** 64 hex characters (AES-256 key) |
| `SPG_SERVER_PORT` | `8080` | HTTP port |
| `SPG_SERVER_MODE` | `debug` | `debug`, `release` or `test` |
| `SPG_DATABASE_HOST` / `_PORT` | `localhost` / `5432` | PostgreSQL address (`5435` when using compose from the host) |
| `SPG_DATABASE_USER` / `_PASSWORD` / `_DBNAME` | `postgres` / `postgres` / `payment_gateway` | PostgreSQL credentials |
| `SPG_DATABASE_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `SPG_DATABASE_MAX_CONNS` | `20` | Connection pool size |
| `SPG_DATABASE_AUTO_MIGRATE` | `true` | Apply embedded migrations at startup |
| `SPG_REDIS_HOST` / `_PORT` | `localhost` / `6379` | Redis address |
| `SPG_JWT_EXPIRY` | `24h` | JWT lifetime |
| `SPG_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `SPG_LOG_PRETTY` | `false` | Human-readable logs (development) |
| `SPG_RATELIMIT_PAYMENTS` | `0` | Override payments limit (req/min, `0` = default) |
| `SPG_RATELIMIT_PAYMENTS_REFUND` | `0` | Override refunds limit (req/min, `0` = default) |
| `SPG_SECURITY_NONCE_FAIL_OPEN` | `false` | Accept signed requests while Redis is down (disables replay protection) |

The application refuses to start if the required secrets are missing or malformed.

## Testing

```bash
make test        # all tests with the race detector
make coverage    # HTML coverage report
make lint        # golangci-lint v2
```

The suite covers services, handlers, middleware and DTO validation; PostgreSQL repositories
(pgxmock); Redis stores (miniredis); end-to-end integration tests on in-memory repositories;
and concurrency tests (100 concurrent payments, idempotency under race). CI runs lint, tests
with `-race`, a coverage gate, `govulncheck`, secret scanning and a Docker build on every
push; CodeQL (`security-extended`) scans Go, JavaScript and workflow files weekly and on
every pull request.

## Load Testing & Benchmarks

[k6](https://k6.io/) scenarios in [tests/load](tests/load/README.md): a sustained stress test
(50 VUs), a flash-sale spike (5 → 150 VUs) and a rate-limit check.

Two consecutive reference runs — single instance via `docker compose` on a laptop (Intel
Core i7-1355U, 12 threads, 20 GB RAM, WSL2). Every request debits the **same wallet row**,
the worst case for lock contention:

| Metric | Run 1 | Run 2 |
| --- | --- | --- |
| Signed payment requests (~4 min, peak 150 VUs) | 26,214 | 23,396 |
| Server errors / failed requests | **0** / **0** | **0** / **0** |
| Latency p50 | 8 ms | 6 ms |
| Latency p95 | 283 ms | 536 ms ¹ |

Ledger check after both runs: top-up 500,000,000 − 49,611 payments totalling 246,435,000 =
wallet balance **253,565,000** ✅ — no lost or duplicated debits, and **0** duplicate
`reference_id`s.

¹ Run 2 shared the machine with other workloads; tail latency is dominated by waiting for
the wallet row lock.

## Demo Scripts

Python scripts in [scripts/demo](scripts/demo/README_DEMO.md) (`pip install -r scripts/demo/requirements.txt`):

| Script | Shows |
| --- | --- |
| `demo_payment.py` | Register → top up → signed payment → refund → webhook received |
| `demo_security.py` | Tampered body → `SEC_002`, replayed request → `SEC_004`, stale timestamp → `SEC_003` |
| `demo_concurrency.py` | 10 simultaneous debits of the full balance → exactly 1 succeeds; 10 retries of one order → charged once |

## Design Decisions

- **Pessimistic over optimistic locking.** A merchant wallet is a hot row. Optimistic
  version checks turn contention into retry storms; `SELECT ... FOR UPDATE` serialises
  debits with predictable latency and no retry logic in clients.
- **Balances encrypted at rest.** A leaked database dump does not reveal balances. The
  trade-off is that arithmetic can't happen in SQL (`balance = balance - x`), so the row is
  locked, decrypted, checked and re-encrypted inside one transaction.
- **Integer minor units.** Amounts are `int64`/`BIGINT` — no floating-point money.
- **Idempotency in two layers.** Redis answers retries cheaply; the PostgreSQL unique
  constraint guarantees correctness even if Redis is empty or unavailable.
- **HMAC + timestamp + nonce** rather than bearer API keys for money movement: a captured
  request can be neither modified nor replayed.
- **Argon2id** for passwords (memory-hard, OWASP-recommended).

## Limitations & Roadmap

This is a learning / portfolio project that applies production patterns; it is not a
certified payment processor. Known gaps:

- [ ] Webhook retries run in-process — pending retries are lost on restart (planned:
      transactional outbox + worker).
- [x] ~~Replay protection failed open when Redis was unavailable~~ — now fails closed
      (`SYS_004`) unless `SPG_SECURITY_NONCE_FAIL_OPEN=true`. Rate limiting still fails open
      by design, favouring availability.
- [ ] JWT uses a shared HS256 secret without refresh tokens or revocation.
- [x] ~~Migrations were applied with `psql`~~ — versioned with golang-migrate, embedded in the
      binary and applied at startup.
- [ ] Top-ups simulate funding; there is no bank or card-network integration.
- [ ] Raise overall test coverage (service layer is ~75%).

## Documentation

| Document | Contents |
| --- | --- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Layers, dependency rules, directory layout |
| [docs/TRANSACTION_STRATEGY.md](docs/TRANSACTION_STRATEGY.md) | Concurrency and locking strategy |
| [docs/logic/CORE_TRANSACTION.md](docs/logic/CORE_TRANSACTION.md) | Payment and refund algorithms |
| [docs/logic/SECURITY_FLOW.md](docs/logic/SECURITY_FLOW.md) | Authentication and signature verification |
| [docs/logic/REPORTING.md](docs/logic/REPORTING.md) | Dashboard and reporting queries |
| [docs/api/openapi.yaml](docs/api/openapi.yaml) | OpenAPI 3 specification |
| [docs/api/ERROR_CODES.md](docs/api/ERROR_CODES.md) | Error code registry |
| [docs/api/WEBHOOK_SPEC.md](docs/api/WEBHOOK_SPEC.md) | Webhook payload and retry policy |

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) and the
[Code of Conduct](CODE_OF_CONDUCT.md). Please report vulnerabilities privately as described
in [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © 2026 Nguyen Quoc Bao ([@VidIsWandering](https://github.com/VidIsWandering))
