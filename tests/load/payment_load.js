import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import crypto from 'k6/crypto';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// ─── Configuration ──────────────────────────────────────────────────────────────
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const ACCESS_KEY = __ENV.ACCESS_KEY || '';
const SECRET_KEY = __ENV.SECRET_KEY || '';

// Pre-flight check: ensure credentials are configured
if (!ACCESS_KEY || !SECRET_KEY) {
  console.error('ERROR: ACCESS_KEY and SECRET_KEY environment variables must be set.');
  console.error('Run: python3 scripts/demo/setup_loadtest.py');
  console.error('Then export the variables printed by the script before running k6.');
}

// ─── Custom Metrics ─────────────────────────────────────────────────────────────
// These provide granular insight beyond the default http_req_failed metric.
const paymentSuccess   = new Counter('payment_success');   // HTTP 201
const paymentDenied    = new Counter('payment_denied');     // HTTP 402 (insufficient funds)
const paymentRateLimited = new Counter('payment_rate_limited'); // HTTP 429
const paymentErrors    = new Counter('payment_errors');     // 4xx/5xx other than 402/429
const paymentSuccessRate = new Rate('payment_success_rate');
const paymentLatency   = new Trend('payment_latency_ms');

// ─── Scenarios ──────────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    // Phase 1 — Stress Test: Sustained heavy load to evaluate DB locking throughput.
    // Requires SPG_RATELIMIT_PAYMENTS to be raised (e.g., 50000) to avoid
    // rate-limit noise masking the actual DB locking behavior.
    stress_payments: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50 },  // Ramp up
        { duration: '1m', target: 50 },   // Sustained load
        { duration: '15s', target: 0 },   // Ramp down
      ],
      exec: 'processPayment',
    },

    // Phase 2 — Spike Test: Sudden traffic surge (flash sale scenario).
    spike_payments: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 5 },    // Normal
        { duration: '5s', target: 150 },   // Sudden spike
        { duration: '20s', target: 150 },  // Hold spike
        { duration: '10s', target: 5 },    // Drop
        { duration: '5s', target: 0 },
      ],
      startTime: '1m50s',
      exec: 'processPayment',
    },

    // Phase 3 — Rate Limit Verification: uses the DEFAULT rate limit (100/min)
    // by sending requests fast enough to trigger HTTP 429.
    // This scenario runs AFTER stress/spike tests complete.
    rate_limit_check: {
      executor: 'constant-arrival-rate',
      rate: 200,       // 200 req/min — must exceed the 100/min default limit
      timeUnit: '1m',
      duration: '1m',
      preAllocatedVUs: 10,
      maxVUs: 20,
      startTime: '3m',
      exec: 'processPaymentRateLimit',
    },
  },

  thresholds: {
    // Stress test: p95 latency under 500ms, and at least 80% payment success+denied
    // (this excludes 429 from the failure metric)
    'payment_latency_ms{scenario:stress_payments}': ['p(95)<500'],
    'payment_success_rate{scenario:stress_payments}': ['rate>0.80'],

    // Overall: no unexpected errors (5xx etc.)
    'payment_errors': ['count<50'],
  },
};

// ─── Setup: Validate credentials ────────────────────────────────────────────────
export function setup() {
  if (!ACCESS_KEY || !SECRET_KEY) {
    throw new Error('ACCESS_KEY and SECRET_KEY must be set. Run: python3 scripts/demo/setup_loadtest.py');
  }

  console.log(`BASE_URL:   ${BASE_URL}`);
  console.log(`ACCESS_KEY: ${ACCESS_KEY.substring(0, 8)}...`);

  // Dry-run single request to verify HMAC auth
  const testBody = JSON.stringify({
    amount: 1000,
    currency: 'VND',
    reference_id: `SETUP-${Date.now()}`
  });

  const path = '/api/v1/payments';
  const headers = generateHeaders(path, testBody);
  const res = http.post(`${BASE_URL}/payments`, testBody, { headers });

  if (res.status === 401) {
    throw new Error(
      `Auth failed (HTTP 401). Re-run: python3 scripts/demo/setup_loadtest.py\n` +
      `Response: ${res.body}`
    );
  }

  console.log(`Setup OK: HTTP ${res.status}`);
  return {};
}

// ─── HMAC Signature Helper ──────────────────────────────────────────────────────
function generateHeaders(path, bodyStr) {
  const nonce = uuidv4();
  const timestamp = Math.floor(Date.now() / 1000).toString();

  const canonicalString = `POST|${path}|${timestamp}|${nonce}|${bodyStr}`;
  const signature = crypto.hmac('sha256', SECRET_KEY, canonicalString, 'hex');

  return {
    'Content-Type': 'application/json',
    'X-Merchant-Access-Key': ACCESS_KEY,
    'X-Signature': signature,
    'X-Timestamp': timestamp,
    'X-Nonce': nonce,
  };
}

// ─── Response classifier ────────────────────────────────────────────────────────
function classifyResponse(res, scenarioTag) {
  const tags = { scenario: scenarioTag };

  switch (res.status) {
    case 201:
      paymentSuccess.add(1, tags);
      paymentSuccessRate.add(true, tags);
      break;
    case 402:
      paymentDenied.add(1, tags);
      paymentSuccessRate.add(true, tags); // 402 = business logic, not a failure
      break;
    case 429:
      paymentRateLimited.add(1, tags);
      paymentSuccessRate.add(false, tags);
      break;
    default:
      paymentErrors.add(1, tags);
      paymentSuccessRate.add(false, tags);
      break;
  }

  paymentLatency.add(res.timings.duration, tags);
}

// ─── Stress / Spike Test executor ───────────────────────────────────────────────
export function processPayment() {
  const reqBody = JSON.stringify({
    amount: 5000,
    currency: 'VND',
    reference_id: `ORDER-${uuidv4()}`
  });

  const path = '/api/v1/payments';
  const headers = generateHeaders(path, reqBody);
  const res = http.post(`${BASE_URL}/payments`, reqBody, { headers });

  classifyResponse(res, __ENV.__ITER !== undefined ? 'stress_payments' : 'stress_payments');

  check(res, {
    'payment processed (201/402) or rate limited (429)': (r) =>
      [201, 402, 429].includes(r.status),
  });

  sleep(Math.random() * 0.3 + 0.1); // 100-400ms think time
}

// ─── Rate Limit Test executor ───────────────────────────────────────────────────
export function processPaymentRateLimit() {
  const reqBody = JSON.stringify({
    amount: 1000,
    currency: 'VND',
    reference_id: `RL-${uuidv4()}`
  });

  const path = '/api/v1/payments';
  const headers = generateHeaders(path, reqBody);
  const res = http.post(`${BASE_URL}/payments`, reqBody, { headers });

  classifyResponse(res, 'rate_limit_check');

  // In this scenario we specifically WANT to see 429s
  check(res, {
    'rate limit: response is 201, 402, or 429': (r) =>
      [201, 402, 429].includes(r.status),
  });

  const is429 = res.status === 429;
  check(res, {
    'rate limit: 429 triggered': () => is429,
  });
}
