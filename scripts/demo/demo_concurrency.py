#!/usr/bin/env python3

import hashlib
import hmac
import json
import time
import uuid
import requests
import sys
import threading
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
    # Mỗi request gửi đi sẽ có Timestamp và Nonce riêng biệt
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

def main():
    print("Khởi tạo tài khoản Merchant với Số dư CHÍNH XÁC là: 10,000 VND...")
    access_key, secret_key, jwt_token = get_credentials()
    
    jwt_headers = {"Authorization": f"Bearer {jwt_token}"}
    balance_resp = requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers)
    print(f"Số dư hiện tại: {balance_resp.json()['data']['balance']} VND")
    
    # ---------------------------------------------------------
    # TEST 4: Race Condition (Concurrent Transactions)
    # ---------------------------------------------------------
    print_step("TEST: Xử lý đồng thời (Race Condition) với Pessimistic Locking")
    print("Kịch bản: Bắn ĐỒNG THỜI 10 requests cùng lúc, mỗi request là MỘT GIAO DỊCH KHÁC NHAU (Reference ID khác nhau) trừ 10,000 VND.")
    print("Mục tiêu: Đảm bảo chỉ 1 request thành công, 9 cái còn lại phải thất bại (Insufficient Funds), số dư cuối cùng KO BAO GIỜ bị âm.")
    print("Gửi 10 requests...")
    
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
            print(f"Request #{idx}: Thành công! (Đã trừ tiền)")
        else:
            fail_count += 1
            # print(f"Request #{idx}: Thất bại! Status {status} - {text}")
            
    print(f"\nThời gian chạy: {time.time() - start_time:.2f}s")
    print(f"Tổng thành công: {success_count} / 10 | Tổng thất bại: {fail_count} / 10")
    
    # Kiểm tra số dư cuối
    balance_resp = requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers)
    print(f"👉 SỐ DƯ CUỐI CÙNG: {balance_resp.json()['data']['balance']} VND")
    if balance_resp.json()['data']['balance'] < 0:
        print("❌ LỖI NGHIÊM TRỌNG: Số dư bị âm!!!")
    else:
        print("✅ SUCCESS: Pessimistic Locking hoạt động đúng!")
        
    time.sleep(2)

    # ---------------------------------------------------------
    # TEST 5: Idempotency (Gửi NHIỀU request TRÙNG Reference ID)
    # ---------------------------------------------------------
    print_step("TEST: Idempotency (Tính Lũy Đẳng)")
    print("Kịch bản: 1 hệ thống Merchant bị lag, bắn ra 5 requests đồng thời yêu cầu thanh toán CÙNG MỘT Order ID (reference_id).")
    print("Mục tiêu: Phát hiện trùng lặp, chỉ xử lý transaction 1 lần duy nhất, các request còn lại phải bị từ chối (HTTP 409).")
    
    # Nạp thêm 20,000 VND
    print("Nạp thêm 20,000 VND...")
    requests.post(f"{BASE_URL}/wallets/topup", json={"amount": 20000, "currency": "VND"}, headers=jwt_headers)
    
    target_reference_id = f"IDEMPOTENCY-{uuid.uuid4()}"
    p = {
            "reference_id": target_reference_id,
            "amount": 10000,
            "currency": "VND"
    }
    payload_str_idem = json.dumps(p, separators=(',', ':'))
    print(f"Gửi 10 requests đồng thời cùng chung Reference ID: {target_reference_id}")
    
    futures_idem = []
    with ThreadPoolExecutor(max_workers=10) as executor:
        for i in range(10):
            # Tất cả requests xài chung 1 payload_str_idem
            futures_idem.append(executor.submit(send_payment, i, "/api/v1/payments", payload_str_idem, access_key, secret_key))
            
    success_idem = 0
    fail_idem = 0
    
    for f in as_completed(futures_idem):
        idx, status, text = f.result()
        if status == 201 or status == 200:
            success_idem += 1
        else:
            fail_idem += 1
            
    print(f"Tổng Created/Ok: {success_idem} / 10 | Tổng Conflict/Bỏ qua: {fail_idem} / 10")
    
    # Kiểm tra số dư cuối
    balance_resp = requests.get(f"{BASE_URL}/wallets/balance", headers=jwt_headers)
    print(f"👉 SỐ DƯ SAU CÙNG: {balance_resp.json()['data']['balance']} VND")
    if balance_resp.json()['data']['balance'] != 10000:
        print(f"❌ LỖI NGHIÊM TRỌNG: Cảnh báo trừ tiền quá nhiều lần. Số tiền đúng là 10,000 VND.")
    else:
        print("✅ SUCCESS: Idempotency hoạt động siêu hiệu quả! Giao dịch không bị lặp.")

if __name__ == "__main__":
    main()
