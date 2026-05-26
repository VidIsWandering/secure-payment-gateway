#!/usr/bin/env python3
"""
Automated setup for k6 load testing.
Creates a merchant, logs in, tops up the wallet, and prints export commands
for the environment variables needed by payment_load.js.

Also provides instructions for raising the rate limit for stress/spike testing.
"""

import hashlib
import hmac
import json
import time
import uuid
import sys

try:
    import requests
except ImportError:
    print("Error: 'requests' module is required. Install via: pip3 install requests")
    sys.exit(1)

BASE_URL = "http://localhost:8080/api/v1"
TOPUP_AMOUNT = 500_000_000  # 500M VND — sufficient for sustained load testing


def main():
    username = f"loadtest_merchant_{int(time.time())}"
    password = "LoadTest_Str0ng!Pass"

    print("=" * 60)
    print(" k6 Load Test — Automated Merchant Setup")
    print("=" * 60)

    # 1. Register merchant
    print("\n[1/4] Registering new merchant...")
    register_payload = {
        "username": username,
        "password": password,
        "merchant_name": "Load Test Merchant",
    }

    resp = requests.post(f"{BASE_URL}/auth/register", json=register_payload)
    if resp.status_code != 201:
        print(f"  ❌ Registration failed: {resp.status_code} — {resp.text}")
        sys.exit(1)

    data = resp.json()["data"]
    access_key = data["access_key"]
    secret_key = data["secret_key"]
    print(f"  ✅ Registered: {username}")

    # 2. Login to get JWT
    print("\n[2/4] Logging in...")
    resp = requests.post(f"{BASE_URL}/auth/login", json={
        "username": username,
        "password": password,
    })
    if resp.status_code != 200:
        print(f"  ❌ Login failed: {resp.status_code} — {resp.text}")
        sys.exit(1)

    jwt_token = resp.json()["data"]["token"]
    print("  ✅ JWT token obtained.")

    # 3. Top-up wallet
    print(f"\n[3/4] Topping up wallet with {TOPUP_AMOUNT:,} VND...")
    resp = requests.post(
        f"{BASE_URL}/wallets/topup",
        json={"amount": TOPUP_AMOUNT, "currency": "VND"},
        headers={"Authorization": f"Bearer {jwt_token}"},
    )
    if resp.status_code != 201:
        print(f"  ❌ Top-up failed: {resp.status_code} — {resp.text}")
        sys.exit(1)
    print(f"  ✅ Wallet funded: {TOPUP_AMOUNT:,} VND")

    # 4. Verify with a test payment using HMAC
    print("\n[4/4] Verifying HMAC auth with a test payment...")
    test_payload = {
        "reference_id": f"SETUP-VERIFY-{int(time.time())}",
        "amount": 1000,
        "currency": "VND",
    }
    payload_str = json.dumps(test_payload, separators=(",", ":"))
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())
    endpoint = "/api/v1/payments"

    canonical = f"POST|{endpoint}|{timestamp}|{nonce}|{payload_str}"
    signature = hmac.new(
        secret_key.encode("utf-8"),
        msg=canonical.encode("utf-8"),
        digestmod=hashlib.sha256,
    ).hexdigest()

    hmac_headers = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": signature,
    }

    resp = requests.post(
        f"http://localhost:8080{endpoint}",
        data=payload_str,
        headers=hmac_headers,
    )
    if resp.status_code == 201:
        print("  ✅ HMAC authentication verified — payment processed!")
    else:
        print(f"  ⚠️  Test payment returned HTTP {resp.status_code}: {resp.text}")

    # Print instructions
    print("\n" + "=" * 60)
    print(" SETUP COMPLETE")
    print("=" * 60)

    print("\n── Step 1: Raise rate limit for stress/spike testing ──")
    print("  (This restarts the app container with a higher payment rate limit)")
    print()
    print('  SPG_RATELIMIT_PAYMENTS=50000 docker compose up -d app')

    print("\n── Step 2: Export credentials ──")
    print()
    print(f'  export ACCESS_KEY="{access_key}"')
    print(f'  export SECRET_KEY="{secret_key}"')
    print(f'  export BASE_URL="{BASE_URL}"')

    print("\n── Step 3: Run load test ──")
    print()
    print("  k6 run tests/load/payment_load.js")

    print("\n── Step 4 (optional): Restore default rate limit ──")
    print()
    print("  docker compose up -d app")
    print("=" * 60)


if __name__ == "__main__":
    main()
