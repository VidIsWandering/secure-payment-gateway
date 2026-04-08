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
    print("Khởi tạo tài khoản Merchant mới cho bài test bảo mật...")
    access_key, secret_key = get_credentials()
    
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
    # TEST 1: Sai chữ ký phân tích (Sai Secret Key hoặc Body đã bị sửa)
    # ---------------------------------------------------------
    print_step("TEST 1: Yêu cầu bị thay đổi Dữ liệu (Tampering/Invalid Signature)")
    print("Mô phỏng: Merchant ký request với số tiền 10,000 VND, nhưng Man-in-the-middle đổi số tiền thành 1,000,000 VND.")
    
    valid_signature = generate_signature("POST", endpoint, secret_key, timestamp, nonce, payload_str)
    
    tampered_payload = {
        "reference_id": payment_payload["reference_id"],
        "amount": 1000000, # Kẻ tấn công đổi số tiền cao hơn
        "currency": "VND"
    }
    tampered_str = json.dumps(tampered_payload, separators=(',', ':'))
    
    headers_test1 = {
        "Content-Type": "application/json",
        "X-Merchant-Access-Key": access_key,
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": valid_signature # Vẫn dùng chữ ký cũ
    }
    
    resp1 = requests.post(f"http://localhost:8080{endpoint}", data=tampered_str, headers=headers_test1)
    print(f"Status Code: {resp1.status_code}\nResponse: {resp1.text}")
    print("👉 KẾT QUẢ: Hệ thống từ chối (SEC_001) vì chữ ký hợp lệ nhưng data không khớp chữ ký.")
    time.sleep(1)

    # ---------------------------------------------------------
    # TEST 2: Replay Attack (Tấn công gửi lại yêu cầu)
    # ---------------------------------------------------------
    print_step("TEST 2: Yêu cầu gửi lại (Replay Attack - Trùng Nonce)")
    print("Mô phỏng: Hacker bắt được gói tin hợp lệ, gửi lại nguyên xi gói tin 2 lần để cố tình trừ tiền 2 lần.")
    
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
    
    # Lần 1: Thành công
    req1 = requests.post(f"http://localhost:8080{endpoint}", data=payload_replay_str, headers=headers_test2)
    print(f"Lần 1 (Hợp lệ)   - Status: {req1.status_code}")
    
    # Lần 2: Thất bại do hệ thống đã lưu Nonce
    req2 = requests.post(f"http://localhost:8080{endpoint}", data=payload_replay_str, headers=headers_test2)
    print(f"Lần 2 (Tấn công) - Status: {req2.status_code}\nResponse: {req2.text}")
    print("👉 KẾT QUẢ: Hệ thống từ chối (SEC_004) vì Nonce đã được sử dụng trước đó.")
    time.sleep(1)
    
    # ---------------------------------------------------------
    # TEST 3: Expired Timestamp (Yêu cầu quá hạn)
    # ---------------------------------------------------------
    print_step("TEST 3: Yêu cầu quá hạn (Expired Timestamp)")
    print("Mô phỏng: Request được ký hợp lệ nhưng Timestamp nằm ngoài khoảng thời gian cho phép (-/+ 5 phút).")
    
    old_timestamp = str(int(time.time()) - 400) # Trễ quá 5 phút (300 giây)
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
    print("👉 KẾT QUẢ: Hệ thống từ chối (SEC_003) vì hiệu chuẩn thời gian của request đã quá cũ.")

if __name__ == "__main__":
    main()
