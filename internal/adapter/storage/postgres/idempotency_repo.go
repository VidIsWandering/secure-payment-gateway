package postgres

import (
	"context"
	"errors"
	"fmt"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"

	"github.com/jackc/pgx/v5"
)

// IdempotencyRepo implements ports.IdempotencyRepository.
type IdempotencyRepo struct {
	pool Pool
}

// NewIdempotencyRepo creates a new IdempotencyRepo.
func NewIdempotencyRepo(pool Pool) *IdempotencyRepo {
	return &IdempotencyRepo{pool: pool}
}

// Create inserts an idempotency log within a database transaction.
func (r *IdempotencyRepo) Create(ctx context.Context, tx ports.Tx, log *domain.IdempotencyLog) error {
	pgxTx := UnwrapTx(tx)
	query := `INSERT INTO idempotency_logs (key, transaction_id, response_json, created_at)
		VALUES ($1, $2, $3, $4)`

	_, err := pgxTx.Exec(ctx, query, log.Key, log.TransactionID, log.ResponseJSON, log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert idempotency log: %w", err)
	}
	return nil
}

// Get fetches an idempotency log by key.
func (r *IdempotencyRepo) Get(ctx context.Context, key string) (*domain.IdempotencyLog, error) {
	query := `SELECT key, transaction_id, response_json, created_at FROM idempotency_logs WHERE key = $1`

	log := &domain.IdempotencyLog{}
	err := r.pool.QueryRow(ctx, query, key).Scan(&log.Key, &log.TransactionID, &log.ResponseJSON, &log.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency log: %w", err)
	}
	return log, nil
}
