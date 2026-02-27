-- 001_init_schema.up.sql
-- Initial database schema for Secure Payment Gateway

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. MERCHANTS
CREATE TABLE IF NOT EXISTS merchants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    merchant_name VARCHAR(100) NOT NULL,
    access_key VARCHAR(64) NOT NULL UNIQUE,
    secret_key_enc TEXT NOT NULL,
    webhook_url TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. WALLETS
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    currency VARCHAR(3) NOT NULL DEFAULT 'VND',
    encrypted_balance TEXT NOT NULL,
    last_audit_hash VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 3. TRANSACTIONS
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    reference_id VARCHAR(100) NOT NULL,
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    amount DECIMAL(20, 2) NOT NULL,
    amount_encrypted TEXT NOT NULL,
    transaction_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    signature VARCHAR(255) NOT NULL,
    client_ip VARCHAR(45),
    extra_data TEXT,
    original_transaction_id UUID REFERENCES transactions(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- 4. IDEMPOTENCY LOGS
CREATE TABLE IF NOT EXISTS idempotency_logs (
    key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    response_json JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. WEBHOOK DELIVERY LOGS
CREATE TABLE IF NOT EXISTS webhook_delivery_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    webhook_url TEXT NOT NULL,
    payload JSONB NOT NULL,
    http_status INTEGER,
    attempt INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    next_retry_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 6. AUDIT LOGS (for phase H)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    merchant_id UUID REFERENCES merchants(id),
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(100),
    details JSONB,
    ip_address VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- INDEXES
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions(merchant_id);
CREATE INDEX IF NOT EXISTS idx_transactions_ref ON transactions(reference_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_created ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_wallets_merchant ON wallets(merchant_id);
CREATE INDEX IF NOT EXISTS idx_webhook_logs_pending ON webhook_delivery_logs(status, next_retry_at)
    WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_webhook_logs_transaction ON webhook_delivery_logs(transaction_id);
CREATE INDEX IF NOT EXISTS idx_merchants_status ON merchants(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_merchant ON audit_logs(merchant_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);
