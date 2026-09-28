#!/usr/bin/env python3

import hashlib
import hmac
import json
import time
import uuid
import requests
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed

BASE_URL = "http://localhost:8080/api/v1"

def print_step(title):
    print(f"\n{'='*50}")
    print(f" ⚡ {title}")
    print(f"{'='*50}")

def generate_signature(method, endpoint, secret_key, timestamp, nonce, payload_str):
    canonical_string = f"{method.upper()}|{endpoint}|{timestamp}|{nonce}|{payload_str}"
    mac = hmac.new(
        secret_key.encode('utf-8'),
        msg=canonical_string.encode('utf-8'),
        digestmod=hashlib.sha256
    )
    return mac.hexdigest()

def get_credentials():
    username = f"race_merchant_{int(time.time())}"
    resp = requests.post(f"{BASE_URL}/auth/register", json={
        "username": username,
        "password": "StrongPassword123!",
        "merchant_name": "Concurrent Test Merchant"
    })
    
    if resp.status_code != 201:
        print("Failed to register merchant.")
        sys.exit(1)
        
    data = resp.json()["data"]
    
    login = requests.post(f"{BASE_URL}/auth/login", json={"username": username, "password": "StrongPassword123!"})
    jwt_token = login.json()["data"]["token"]
    
    # Top-up a small amount: Let's top up exactly 10,000 VND
    # So the wallet can only process ONE payment of 10,000 VND.
    requests.post(f"{BASE_URL}/wallets/topup", json={"amount": 10000, "currency": "VND"}, headers={"Authorization": f"Bearer {jwt_token}"})
    
    return data["access_key"], data["secret_key"], jwt_token

def send_payment(req_id, endpoint, payload_str, access_key, secret_key):
    # Every request gets its own timestamp and nonce
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())
    sig = generate_signature("POST", endpoint, secret_key, timestamp, nonce, payload_str)
    
    headers = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": sig
    }
    
    try:
        resp = requests.post(f"http://localhost:8080{endpoint}", data=payload_str, headers=headers, timeout=10)
        return req_id, resp.status_code, resp.text
    except Exception as e:
        return req_id, 000, str(e)

def get_balance(jwt_headers):
    return requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers).json()["data"]["balance"]

def main():
    print("Creating a merchant whose wallet holds EXACTLY 10,000 VND...")
    access_key, secret_key, jwt_token = get_credentials()
    
    jwt_headers = {"Authorization": f"Bearer {jwt_token}"}
    print(f"Current balance: {get_balance(jwt_headers)} VND")
    passed = []
    
    # ---------------------------------------------------------
    # TEST 1: Race condition (concurrent debits)
    # ---------------------------------------------------------
    print_step("TEST: Concurrent debits with pessimistic locking")
    print("Scenario: fire 10 requests AT THE SAME TIME, each a DIFFERENT payment (different reference ID) of 10,000 VND.")
    print("Goal: exactly 1 succeeds, the other 9 fail with insufficient funds, and the balance NEVER goes negative.")
    print("Sending 10 requests...")
    
    futures = []
    
    # Preparing payloads
    payloads = []
    for i in range(10):
        p = {
            "reference_id": f"RACE-ORD-{int(time.time()*1000)}-{i}",
            "amount": 10000,
            "currency": "VND"
        }
        payloads.append(json.dumps(p, separators=(',', ':')))
    
    start_time = time.time()
    with ThreadPoolExecutor(max_workers=10) as executor:
        for i, payload_str in enumerate(payloads):
            futures.append(executor.submit(send_payment, i, "/api/v1/payments", payload_str, access_key, secret_key))
            
    success_count = 0
    fail_count = 0
    
    for f in as_completed(futures):
        idx, status, text = f.result()
        if status == 201:
            success_count += 1
            print(f"Request #{idx}: succeeded (wallet debited)")
        else:
            fail_count += 1
            
    print(f"\nElapsed: {time.time() - start_time:.2f}s")
    print(f"Succeeded: {success_count} / 10 | Failed: {fail_count} / 10")
    
    balance = get_balance(jwt_headers)
    print(f"👉 FINAL BALANCE: {balance} VND")
    if success_count == 1 and balance == 0:
        print("✅ SUCCESS: pessimistic locking serialised the debits — no overdraft.")
        passed.append(True)
    else:
        print("❌ FAILURE: expected exactly 1 successful debit and a final balance of 0.")
        passed.append(False)
        
    time.sleep(2)

    # ---------------------------------------------------------
    # TEST 2: Idempotency (many requests with the SAME reference ID)
    # ---------------------------------------------------------
    print_step("TEST: Idempotency")
    print("Scenario: a lagging merchant system fires 10 concurrent requests to pay the SAME order (reference_id).")
    print("Goal: the order is charged exactly once; duplicates get the original result back or 409 Conflict.")
    
    print("Topping up another 20,000 VND...")
    requests.post(f"{BASE_URL}/wallets/topup", json={"amount": 20000, "currency": "VND"}, headers=jwt_headers)
    
    target_reference_id = f"IDEMPOTENCY-{uuid.uuid4()}"
    p = {
            "reference_id": target_reference_id,
            "amount": 10000,
            "currency": "VND"
    }
    payload_str_idem = json.dumps(p, separators=(',', ':'))
    print(f"Sending 10 concurrent requests sharing reference ID: {target_reference_id}")
    
    futures_idem = []
    with ThreadPoolExecutor(max_workers=10) as executor:
        for i in range(10):
            # All requests share the same payload
            futures_idem.append(executor.submit(send_payment, i, "/api/v1/payments", payload_str_idem, access_key, secret_key))
            
    success_idem = 0
    fail_idem = 0
    
    for f in as_completed(futures_idem):
        idx, status, text = f.result()
        if status == 201 or status == 200:
            success_idem += 1
        else:
            fail_idem += 1
            
    print(f"Created/OK (original result replayed): {success_idem} / 10 | Conflict: {fail_idem} / 10")
    
    balance = get_balance(jwt_headers)
    print(f"👉 FINAL BALANCE: {balance} VND")
    if balance != 10000:
        print("❌ FAILURE: the order was charged more than once. Expected balance: 10,000 VND.")
        passed.append(False)
    else:
        print("✅ SUCCESS: idempotency held — the order was charged exactly once.")
        passed.append(True)

    sys.exit(0 if all(passed) else 1)

if __name__ == "__main__":
    main()
