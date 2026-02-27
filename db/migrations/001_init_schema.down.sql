-- 001_init_schema.down.sql
-- Rollback initial schema

DROP INDEX IF EXISTS idx_audit_logs_created;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_merchant;
DROP INDEX IF EXISTS idx_merchants_status;
DROP INDEX IF EXISTS idx_webhook_logs_transaction;
DROP INDEX IF EXISTS idx_webhook_logs_pending;
DROP INDEX IF EXISTS idx_wallets_merchant;
DROP INDEX IF EXISTS idx_transactions_created;
DROP INDEX IF EXISTS idx_transactions_type;
DROP INDEX IF EXISTS idx_transactions_status;
DROP INDEX IF EXISTS idx_transactions_ref;
DROP INDEX IF EXISTS idx_transactions_merchant;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS webhook_delivery_logs;
DROP TABLE IF EXISTS idempotency_logs;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS wallets;
DROP TABLE IF EXISTS merchants;

DROP EXTENSION IF EXISTS "uuid-ossp";
