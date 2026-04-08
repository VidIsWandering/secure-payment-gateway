import http from 'k6/http';
import { check, sleep } from 'k6';
import crypto from 'k6/crypto';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const ACCESS_KEY = __ENV.ACCESS_KEY || 'YOUR_ACCESS_KEY';
const SECRET_KEY = __ENV.SECRET_KEY || 'YOUR_SECRET_KEY';

export const options = {
  scenarios: {
    // 1. Stress Test: Sustained heavy load to evaluate DB Locking and throughput
    stress_payments: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 200 }, // Ramp up to 200 VUs
        { duration: '2m', target: 200 },  // Hold at 200 VUs for 2 minutes
        { duration: '30s', target: 0 },   // Ramp down
      ],
      exec: 'processPayment',
    },

    // 2. Spike Test: Sudden surge in traffic to test system resilience
    spike_payments: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 10 },   // Normal load
        { duration: '10s', target: 500 },  // Sudden spike to 500 VUs (Flash Sale)
        { duration: '30s', target: 500 },  // Hold spike
        { duration: '10s', target: 10 },   // Drop back to normal load
        { duration: '10s', target: 0 },
      ],
      startTime: '3m', // Start after stress test finishes
      exec: 'processPayment',
    },

    // 3. Rate Limit Test: Purposefully trigger HTTP 429
    rate_limit_check: {
      executor: 'constant-arrival-rate',
      rate: 150, // 150 iterations per minute (exceeds the 100 req/min rule)
      timeUnit: '1m',
      duration: '2m',
      preAllocatedVUs: 10,
      maxVUs: 20,
      startTime: '4m30s', // Start after spike test finishes
      exec: 'processPaymentRateLimit',
    },
  },
  thresholds: {
    'http_req_duration{scenario:stress_payments}': ['p(95)<1000'], // 95% of requests should be below 1s
    'http_req_failed{scenario:stress_payments}': ['rate<0.05'],    // Max 5% failure rate (excluding 402/429)
  },
};

// Helper: Generate signed headers
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

// Execution block for standard payment processing scenarios (Stress/Spike)
export function processPayment() {
  const reqBody = JSON.stringify({
    amount: 5000,
    currency: 'VND',
    reference_id: `ORDER-${uuidv4()}`
  });

  const path = '/api/v1/payments';
  const headers = generateHeaders(path, reqBody);

  const res = http.post(`${BASE_URL}/payments`, reqBody, { headers });

  // 201 Created, 402 Insufficient Funds (Wallet empty), 400 Bad Request
  check(res, {
    'status is 201 or 402': (r) => r.status === 201 || r.status === 402,
  });

  sleep(Math.random() * 0.5); // Random think time
}

// Execution block specifically testing rate limiting controls
export function processPaymentRateLimit() {
  const reqBody = JSON.stringify({
    amount: 1000,
    currency: 'VND',
    reference_id: `RL-ORDER-${uuidv4()}`
  });

  const path = '/api/v1/payments';
  const headers = generateHeaders(path, reqBody);

  const res = http.post(`${BASE_URL}/payments`, reqBody, { headers });

  // During this scenario, we expect to hit 429 Too Many Requests eventually
  check(res, {
    'status handled (201, 402, 429)': (r) => [201, 402, 429].includes(r.status),
  });
}
