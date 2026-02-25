package postgres

import (
	"context"
	"errors"
	"fmt"

	"secure-payment-gateway/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WalletRepo implements ports.WalletRepository.
type WalletRepo struct {
	pool Pool
}

// NewWalletRepo creates a new WalletRepo.
func NewWalletRepo(pool Pool) *WalletRepo {
	return &WalletRepo{pool: pool}
}

// Create inserts a new wallet into the database.
func (r *WalletRepo) Create(ctx context.Context, w *domain.Wallet) error {
	query := `INSERT INTO wallets (id, merchant_id, currency, encrypted_balance, last_audit_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.pool.Exec(ctx, query,
		w.ID, w.MerchantID, w.Currency, w.EncryptedBalance,
		w.LastAuditHash, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert wallet: %w", err)
	}
	return nil
}

// GetByID fetches a wallet by its UUID (without locking).
func (r *WalletRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	query := `SELECT id, merchant_id, currency, encrypted_balance, last_audit_hash, created_at, updated_at
		FROM wallets WHERE id = $1`

	w := &domain.Wallet{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.MerchantID, &w.Currency, &w.EncryptedBalance,
		&w.LastAuditHash, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get wallet by id: %w", err)
	}
	return w, nil
}

// GetByMerchantID fetches a wallet by merchant ID and currency (non-locking read).
func (r *WalletRepo) GetByMerchantID(ctx context.Context, merchantID uuid.UUID, currency string) (*domain.Wallet, error) {
	query := `SELECT id, merchant_id, currency, encrypted_balance, last_audit_hash, created_at, updated_at
		FROM wallets WHERE merchant_id = $1 AND currency = $2`

	w := &domain.Wallet{}
	err := r.pool.QueryRow(ctx, query, merchantID, currency).Scan(
		&w.ID, &w.MerchantID, &w.Currency, &w.EncryptedBalance,
		&w.LastAuditHash, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get wallet by merchant id: %w", err)
	}
	return w, nil
}

// GetByMerchantIDForUpdate fetches a wallet by merchant ID and currency with pessimistic locking.
// This MUST be called within a transaction.
func (r *WalletRepo) GetByMerchantIDForUpdate(ctx context.Context, tx pgx.Tx, merchantID uuid.UUID, currency string) (*domain.Wallet, error) {
	query := `SELECT id, merchant_id, currency, encrypted_balance, last_audit_hash, created_at, updated_at
		FROM wallets WHERE merchant_id = $1 AND currency = $2 FOR UPDATE`

	w := &domain.Wallet{}
	err := tx.QueryRow(ctx, query, merchantID, currency).Scan(
		&w.ID, &w.MerchantID, &w.Currency, &w.EncryptedBalance,
		&w.LastAuditHash, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get wallet for update by merchant: %w", err)
	}
	return w, nil
}

// GetByIDForUpdate fetches a wallet by ID with pessimistic locking.
// This MUST be called within a transaction.
func (r *WalletRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Wallet, error) {
	query := `SELECT id, merchant_id, currency, encrypted_balance, last_audit_hash, created_at, updated_at
		FROM wallets WHERE id = $1 FOR UPDATE`

	w := &domain.Wallet{}
	err := tx.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.MerchantID, &w.Currency, &w.EncryptedBalance,
		&w.LastAuditHash, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get wallet for update by id: %w", err)
	}
	return w, nil
}

// UpdateBalance updates a wallet's encrypted balance within a transaction.
func (r *WalletRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, walletID uuid.UUID, encryptedBalance string) error {
	query := `UPDATE wallets SET encrypted_balance = $1, updated_at = NOW() WHERE id = $2`

	tag, err := tx.Exec(ctx, query, encryptedBalance, walletID)
	if err != nil {
		return fmt.Errorf("update wallet balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found: %s", walletID)
	}
	return nil
}
