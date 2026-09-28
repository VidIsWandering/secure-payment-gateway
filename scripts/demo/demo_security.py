#!/usr/bin/env python3
"""Demonstrates the gateway's request-integrity protections:
tampered bodies, replayed requests and stale timestamps are all rejected."""

import hashlib
import hmac
import json
import time
import uuid
import requests
import sys

BASE_URL = "http://localhost:8080/api/v1"

def print_step(title):
    print(f"\n{'='*50}")
    print(f" 🛡️ {title}")
    print(f"{'='*50}")

def generate_signature(method, endpoint, secret_key, timestamp, nonce, payload_str):
    canonical_string = f"{method.upper()}|{endpoint}|{timestamp}|{nonce}|{payload_str}"
    mac = hmac.new(
        secret_key.encode('utf-8'),
        msg=canonical_string.encode('utf-8'),
        digestmod=hashlib.sha256
    )
    return mac.hexdigest()

def expect_rejection(resp, expected_code, reason):
    """Prints the verdict for an attack request and returns True if it was rejected as expected."""
    try:
        error_code = resp.json().get("error_code")
    except ValueError:
        error_code = None
    if error_code == expected_code:
        print(f"✅ RESULT: rejected with {expected_code} — {reason}")
        return True
    print(f"❌ RESULT: expected {expected_code}, got HTTP {resp.status_code} ({error_code})")
    return False

def get_credentials():
    username = f"sec_merchant_{int(time.time())}"
    resp = requests.post(f"{BASE_URL}/auth/register", json={
        "username": username,
        "password": "StrongPassword123!",
        "merchant_name": "Security Test Merchant"
    })

    if resp.status_code != 201:
        print("Failed to register merchant.")
        sys.exit(1)

    data = resp.json()["data"]

    # Login to top-up
    login = requests.post(f"{BASE_URL}/auth/login", json={"username": username, "password": "StrongPassword123!"})
    jwt_token = login.json()["data"]["token"]
    requests.post(f"{BASE_URL}/wallets/topup", json={"amount": 5000000, "currency": "VND"}, headers={"Authorization": f"Bearer {jwt_token}"})

    return data["access_key"], data["secret_key"]

def main():
    print("Creating a new merchant account for the security tests...")
    access_key, secret_key = get_credentials()
    passed = []

    payment_payload = {
        "reference_id": f"SEC-ORD-{int(time.time())}",
        "amount": 10000,
        "currency": "VND"
    }
    payload_str = json.dumps(payment_payload, separators=(',', ':'))
    endpoint = "/api/v1/payments"
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())

    # ---------------------------------------------------------
    # TEST 1: Tampered body (signature no longer matches the payload)
    # ---------------------------------------------------------
    print_step("TEST 1: Tampered request body (invalid signature)")
    print("Scenario: the merchant signs a 10,000 VND payment, but a man-in-the-middle changes the amount to 1,000,000 VND.")

    valid_signature = generate_signature("POST", endpoint, secret_key, timestamp, nonce, payload_str)

    tampered_payload = {
        "reference_id": payment_payload["reference_id"],
        "amount": 1000000, # attacker raises the amount
        "currency": "VND"
    }
    tampered_str = json.dumps(tampered_payload, separators=(',', ':'))

    headers_test1 = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": valid_signature # signature of the original body
    }

    resp1 = requests.post(f"http://localhost:8080{endpoint}", data=tampered_str, headers=headers_test1)
    print(f"Status Code: {resp1.status_code}\nResponse: {resp1.text}")
    passed.append(expect_rejection(resp1, "SEC_002", "the signature does not match the modified body."))
    time.sleep(1)

    # ---------------------------------------------------------
    # TEST 2: Replay attack (same nonce sent twice)
    # ---------------------------------------------------------
    print_step("TEST 2: Replay attack (reused nonce)")
    print("Scenario: an attacker captures a valid request and sends it again, trying to charge the merchant twice.")

    nonce_replay = str(uuid.uuid4())
    timestamp_replay = str(int(time.time()))
    payload_replay_str = json.dumps({"reference_id": str(uuid.uuid4()), "amount": 10000, "currency": "VND"}, separators=(',', ':'))
    sig_replay = generate_signature("POST", endpoint, secret_key, timestamp_replay, nonce_replay, payload_replay_str)

    headers_test2 = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp_replay,
        "X-Nonce": nonce_replay,
        "X-Signature": sig_replay
    }

    # 1st attempt: legitimate request succeeds
    req1 = requests.post(f"http://localhost:8080{endpoint}", data=payload_replay_str, headers=headers_test2)
    print(f"Attempt 1 (legitimate) - Status: {req1.status_code}")

    # 2nd attempt: rejected because the nonce has already been stored
    req2 = requests.post(f"http://localhost:8080{endpoint}", data=payload_replay_str, headers=headers_test2)
    print(f"Attempt 2 (replay)     - Status: {req2.status_code}\nResponse: {req2.text}")
    passed.append(req1.status_code == 201 and expect_rejection(req2, "SEC_004", "the nonce has already been used."))
    time.sleep(1)

    # ---------------------------------------------------------
    # TEST 3: Expired timestamp
    # ---------------------------------------------------------
    print_step("TEST 3: Expired timestamp")
    print("Scenario: a correctly signed request whose timestamp is outside the allowed window (±60 seconds).")

    old_timestamp = str(int(time.time()) - 400) # well outside the 60-second window
    new_nonce = str(uuid.uuid4())
    sig_expired = generate_signature("POST", endpoint, secret_key, old_timestamp, new_nonce, payload_str)

    headers_test3 = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": old_timestamp,
        "X-Nonce": new_nonce,
        "X-Signature": sig_expired
    }

    resp3 = requests.post(f"http://localhost:8080{endpoint}", data=payload_str, headers=headers_test3)
    print(f"Status Code: {resp3.status_code}\nResponse: {resp3.text}")
    passed.append(expect_rejection(resp3, "SEC_003", "the request timestamp is too old."))

    print(f"\n{sum(passed)}/{len(passed)} attacks blocked.")
    sys.exit(0 if all(passed) else 1)

if __name__ == "__main__":
    main()
