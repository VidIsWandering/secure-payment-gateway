package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TransactionRepo implements ports.TransactionRepository.
type TransactionRepo struct {
	pool Pool
}

// NewTransactionRepo creates a new TransactionRepo.
func NewTransactionRepo(pool Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

// Create inserts a new transaction within a database transaction.
func (r *TransactionRepo) Create(ctx context.Context, tx pgx.Tx, t *domain.Transaction) error {
	query := `INSERT INTO transactions (id, reference_id, merchant_id, wallet_id, amount, amount_encrypted,
		transaction_type, status, signature, client_ip, extra_data, original_transaction_id, created_at, processed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := tx.Exec(ctx, query,
		t.ID, t.ReferenceID, t.MerchantID, t.WalletID,
		t.Amount, t.AmountEncrypted, t.TransactionType, t.Status,
		t.Signature, t.ClientIP, t.ExtraData, t.OriginalTransactionID,
		t.CreatedAt, t.ProcessedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// GetByID fetches a transaction by UUID.
func (r *TransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `SELECT id, reference_id, merchant_id, wallet_id, amount, amount_encrypted,
		transaction_type, status, signature, client_ip, extra_data, original_transaction_id, created_at, processed_at
		FROM transactions WHERE id = $1`

	return r.scanTransaction(r.pool.QueryRow(ctx, query, id))
}

// GetByReference fetches a transaction by merchant ID and reference ID.
func (r *TransactionRepo) GetByReference(ctx context.Context, merchantID uuid.UUID, referenceID string) (*domain.Transaction, error) {
	query := `SELECT id, reference_id, merchant_id, wallet_id, amount, amount_encrypted,
		transaction_type, status, signature, client_ip, extra_data, original_transaction_id, created_at, processed_at
		FROM transactions WHERE merchant_id = $1 AND reference_id = $2`

	return r.scanTransaction(r.pool.QueryRow(ctx, query, merchantID, referenceID))
}

// UpdateStatus updates a transaction's status within a database transaction.
func (r *TransactionRepo) UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.TransactionStatus) error {
	now := time.Now()
	query := `UPDATE transactions SET status = $1, processed_at = $2 WHERE id = $3`

	tag, err := tx.Exec(ctx, query, status, now, id)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found: %s", id)
	}
	return nil
}

// CheckRefundExists checks if a refund already exists for a given original transaction.
func (r *TransactionRepo) CheckRefundExists(ctx context.Context, originalTxID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM transactions WHERE original_transaction_id = $1 AND transaction_type = 'REFUND' AND status != 'FAILED')`

	var exists bool
	err := r.pool.QueryRow(ctx, query, originalTxID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check refund exists: %w", err)
	}
	return exists, nil
}

// List fetches transactions with filtering and pagination.
func (r *TransactionRepo) List(ctx context.Context, params ports.TransactionListParams) ([]domain.Transaction, int64, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("merchant_id = $%d", argIdx))
	args = append(args, params.MerchantID)
	argIdx++

	if params.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *params.Status)
		argIdx++
	}
	if params.Type != nil {
		conditions = append(conditions, fmt.Sprintf("transaction_type = $%d", argIdx))
		args = append(args, *params.Type)
		argIdx++
	}
	if params.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= to_timestamp($%d)", argIdx))
		args = append(args, *params.From)
		argIdx++
	}
	if params.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= to_timestamp($%d)", argIdx))
		args = append(args, *params.To)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions %s", where)
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Fetch page
	offset := (params.Page - 1) * params.PageSize
	dataQuery := fmt.Sprintf(`SELECT id, reference_id, merchant_id, wallet_id, amount, amount_encrypted,
		transaction_type, status, signature, client_ip, extra_data, original_transaction_id, created_at, processed_at
		FROM transactions %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, params.PageSize, offset)

	rows, err := r.pool.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []domain.Transaction
	for rows.Next() {
		t := domain.Transaction{}
		err := rows.Scan(
			&t.ID, &t.ReferenceID, &t.MerchantID, &t.WalletID,
			&t.Amount, &t.AmountEncrypted, &t.TransactionType, &t.Status,
			&t.Signature, &t.ClientIP, &t.ExtraData, &t.OriginalTransactionID,
			&t.CreatedAt, &t.ProcessedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan transaction row: %w", err)
		}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate transaction rows: %w", err)
	}
	return txns, total, nil
}

// GetStats retrieves aggregated transaction statistics for a merchant.
func (r *TransactionRepo) GetStats(ctx context.Context, merchantID uuid.UUID, periodStart *int64) (*ports.TransactionStats, error) {
	var args []any
	argIdx := 1

	condition := fmt.Sprintf("merchant_id = $%d", argIdx)
	args = append(args, merchantID)
	argIdx++

	if periodStart != nil {
		condition += fmt.Sprintf(" AND created_at >= to_timestamp($%d)", argIdx)
		args = append(args, *periodStart)
	}

	query := fmt.Sprintf(`SELECT
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE status = 'SUCCESS') AS successful,
		COUNT(*) FILTER (WHERE status = 'FAILED') AS failed,
		COUNT(*) FILTER (WHERE status = 'REVERSED') AS reversed,
		COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'PAYMENT' AND status = 'SUCCESS'), 0) AS revenue,
		COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'REFUND' AND status = 'SUCCESS'), 0) AS refunded,
		COALESCE(SUM(amount) FILTER (WHERE transaction_type = 'TOPUP' AND status = 'SUCCESS'), 0) AS topup
		FROM transactions WHERE %s`, condition)

	stats := &ports.TransactionStats{}
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&stats.TotalTransactions, &stats.Successful, &stats.Failed, &stats.Reversed,
		&stats.TotalRevenue, &stats.TotalRefunded, &stats.TotalTopup,
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction stats: %w", err)
	}
	return stats, nil
}

// scanTransaction is a helper to scan a single row into a Transaction.
func (r *TransactionRepo) scanTransaction(row pgx.Row) (*domain.Transaction, error) {
	t := &domain.Transaction{}
	err := row.Scan(
		&t.ID, &t.ReferenceID, &t.MerchantID, &t.WalletID,
		&t.Amount, &t.AmountEncrypted, &t.TransactionType, &t.Status,
		&t.Signature, &t.ClientIP, &t.ExtraData, &t.OriginalTransactionID,
		&t.CreatedAt, &t.ProcessedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan transaction: %w", err)
	}
	return t, nil
}
