#!/usr/bin/env python3

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
    print(f" {title}")
    print(f"{'='*50}")

def generate_signature(method, endpoint, secret_key, timestamp, nonce, payload_str):
    """
    Generate an HMAC-SHA256 signature for the given request parameters.
    The canonical format must exactly match: METHOD|/path|TIMESTAMP|NONCE|BODY
    """
    canonical_string = f"{method.upper()}|{endpoint}|{timestamp}|{nonce}|{payload_str}"
    
    # We generate the HMAC using SHA256
    mac = hmac.new(
        secret_key.encode('utf-8'),
        msg=canonical_string.encode('utf-8'),
        digestmod=hashlib.sha256
    )
    return mac.hexdigest()

def main():
    username = f"demo_merchant_{int(time.time())}"
    password = "StrongPassword123!"

    # ---------------------------------------------------------
    # 1. Register a new Merchant
    # ---------------------------------------------------------
    print_step("1. Registering New Merchant")
    register_payload = {
        "username": username,
        "password": password,
        "merchant_name": "Demo Merchant Inc.",
        "webhook_url": "http://localhost:9000/webhook" # localhost is valid for dev mode
    }
    
    resp = requests.post(f"{BASE_URL}/auth/register", json=register_payload)
    if resp.status_code != 201:
        print(f"Registration failed: {resp.text}")
        sys.exit(1)
        
    data = resp.json()["data"]
    access_key = data["access_key"]
    secret_key = data["secret_key"]
    merchant_id = data["merchant_id"]
    
    print(f"✅ Registered successfuly!")
    print(f"Merchant ID: {merchant_id}")
    print(f"Access Key:  {access_key}")
    print(f"Secret Key:  {secret_key}")

    # ---------------------------------------------------------
    # 2. Login to get JWT (For Management APIs like Top-up)
    # ---------------------------------------------------------
    print_step("2. Login to get JWT Token")
    login_payload = {
        "username": username,
        "password": password
    }
    
    resp = requests.post(f"{BASE_URL}/auth/login", json=login_payload)
    if resp.status_code != 200:
        print(f"Login failed: {resp.text}")
        sys.exit(1)
        
    jwt_token = resp.json()["data"]["token"]
    print(f"✅ Login successful! Received JWT Token.")
    
    jwt_headers = {
        "Authorization": f"Bearer {jwt_token}"
    }

    # ---------------------------------------------------------
    # 3. Check Wallet Balance
    # ---------------------------------------------------------
    print_step("3. Getting Initial Wallet Balance")
    resp = requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers)
    print(f"Balance Info: {json.dumps(resp.json(), indent=2)}")

    # ---------------------------------------------------------
    # 4. Top-up Wallet using JWT Auth
    # ---------------------------------------------------------
    print_step("4. Topping up Wallet (Simulating Funding)")
    topup_payload = {
        "amount": 5000000, # 5,000,000 VND
        "currency": "VND"
    }
    resp = requests.post(f"{BASE_URL}/wallets/topup", json=topup_payload, headers=jwt_headers)
    if resp.status_code != 201:
        print(f"Top-up failed: {resp.text}")
        sys.exit(1)
    print(f"✅ Top-up successful! Result: {json.dumps(resp.json()['data'], indent=2)}")

    # ---------------------------------------------------------
    # 5. Process a Payment using HMAC Auth
    # ---------------------------------------------------------
    print_step("5. Processing Payment (HMAC Signature Auth)")
    payment_payload = {
        "reference_id": f"ORD-{int(time.time())}",
        "amount": 150000, # 150,000 VND
        "currency": "VND"
    }
    
    # We must format the json without spaces after separators for the signature to match exactly across languages.
    # requests.post(json=...) uses no spaces generally, but to be 100% safe, we serialize it ourselves:
    payload_str = json.dumps(payment_payload, separators=(',', ':'))
    
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())
    endpoint = "/api/v1/payments"
    
    signature = generate_signature("POST", endpoint, secret_key, timestamp, nonce, payload_str)
    
    print(f"Generated Timestamp: {timestamp}")
    print(f"Generated Nonce: {nonce}")
    print(f"Generated Canonical: POST|{endpoint}|{timestamp}|{nonce}|{payload_str}")
    print(f"Generated Signature: {signature}")
    
    hmac_headers = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": signature
    }
    
    resp = requests.post(f"http://localhost:8080{endpoint}", data=payload_str, headers=hmac_headers)
    if resp.status_code != 201:
        print(f"Payment failed: {resp.status_code} - {resp.text}")
        sys.exit(1)
        
    payment_data = resp.json()["data"]
    print(f"\n✅ Payment successful! Details:")
    print(json.dumps(payment_data, indent=2))
    
    original_tx_id = payment_data["reference_id"] # Or transaction_id, wait, refund uses original_reference_id
    
    # ---------------------------------------------------------
    # 6. Refund the Output Payment
    # ---------------------------------------------------------
    print_step("6. Refunding the Payment")
    refund_payload = {
        "original_reference_id": original_tx_id,
        "amount": 50000, # Partially refund 50,000 VND
        "reason": "Customer cancellation"
    }
    refund_payload_str = json.dumps(refund_payload, separators=(',', ':'))
    
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())
    endpoint = "/api/v1/payments/refund"
    
    signature = generate_signature("POST", endpoint, secret_key, timestamp, nonce, refund_payload_str)
    
    hmac_headers = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": signature
    }
    
    resp = requests.post(f"http://localhost:8080{endpoint}", data=refund_payload_str, headers=hmac_headers)
    if resp.status_code != 201 and resp.status_code != 200:
        print(f"Refund failed: {resp.status_code} - {resp.text}")
        sys.exit(1)
        
    print(f"✅ Refund successful! Details:")
    print(json.dumps(resp.json()["data"], indent=2))

    # ---------------------------------------------------------
    # 7. Check Final Balance
    # ---------------------------------------------------------
    print_step("7. Getting Final Wallet Balance")
    resp = requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers)
    print(f"Balance Info: {json.dumps(resp.json(), indent=2)}")

if __name__ == "__main__":
    main()
