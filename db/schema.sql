-- DATABASE SCHEMA: Secure Payment Gateway
-- Dialect: PostgreSQL 16+

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. MERCHANTS TABLE
-- Stores identity and authentication keys 
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL, -- Argon2id (Time=1, Memory=64MB, Threads=4, KeyLen=32)
    merchant_name VARCHAR(100) NOT NULL,
    access_key VARCHAR(64) NOT NULL UNIQUE, -- Public identifier
    secret_key_enc TEXT NOT NULL, -- Encrypted Secret Key (AES-256)
    webhook_url TEXT, -- URL for transaction status callbacks
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, SUSPENDED, DEACTIVATED
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. WALLETS TABLE
-- Decoupled from merchants for flexibility.
-- WARNING: 'encrypted_balance' is AES-256 string, NOT a number 
CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    currency VARCHAR(3) NOT NULL DEFAULT 'VND',
    encrypted_balance TEXT NOT NULL, -- Must decrypt to use
    last_audit_hash VARCHAR(64), -- For integrity check
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. TRANSACTIONS TABLE
-- Immutable ledger of all money movements 
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reference_id VARCHAR(100) NOT NULL, -- Merchant's Order ID
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    
    amount DECIMAL(20, 2) NOT NULL, -- Visible for analytics/reporting
    amount_encrypted TEXT NOT NULL, -- Secure record (AES-256)
    
    transaction_type VARCHAR(20) NOT NULL, -- PAYMENT, REFUND, TOPUP
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, SUCCESS, FAILED, REVERSED
    
    signature VARCHAR(255) NOT NULL, -- Request signature from Merchant
    client_ip VARCHAR(45),
    extra_data TEXT, -- Metadata from Merchant (order info, etc.)
    original_transaction_id UUID REFERENCES transactions(id), -- For REFUND: links to original tx
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- 4. IDEMPOTENCY_LOGS TABLE
-- Backup for Redis to ensure strictly no double-spending
CREATE TABLE idempotency_logs (
    key VARCHAR(255) PRIMARY KEY, -- usually "merchant_id:reference_id"
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    response_json JSONB, -- Cache the response to return identical result
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. WEBHOOK DELIVERY LOGS TABLE
-- Tracks webhook delivery attempts with retry support
CREATE TABLE webhook_delivery_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    webhook_url TEXT NOT NULL,
    payload JSONB NOT NULL,
    http_status INTEGER, -- Response status code from merchant
    attempt INTEGER NOT NULL DEFAULT 1, -- Current attempt number (max 5)
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, DELIVERED, FAILED
    next_retry_at TIMESTAMP WITH TIME ZONE, -- Scheduled time for next retry
    last_error TEXT, -- Error message from last attempt
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- INDEXES FOR PERFORMANCE
CREATE INDEX idx_transactions_merchant ON transactions(merchant_id);
CREATE INDEX idx_transactions_ref ON transactions(reference_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_type ON transactions(transaction_type);
CREATE INDEX idx_transactions_created ON transactions(created_at);
CREATE INDEX idx_wallets_merchant ON wallets(merchant_id);
CREATE INDEX idx_webhook_logs_pending ON webhook_delivery_logs(status, next_retry_at)
    WHERE status = 'PENDING';
CREATE INDEX idx_webhook_logs_transaction ON webhook_delivery_logs(transaction_id);
CREATE INDEX idx_merchants_status ON merchants(status);