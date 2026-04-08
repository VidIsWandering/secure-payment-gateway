package postgres

import (
	"context"

	"secure-payment-gateway/internal/core/ports"

	"github.com/jackc/pgx/v5"
)

// Transactor implements ports.DBTransactor using pgxpool.Pool.
type Transactor struct {
	pool Pool
}

// NewTransactor creates a new Transactor wrapping the connection pool.
func NewTransactor(pool Pool) *Transactor {
	return &Transactor{pool: pool}
}

// Begin starts a new database transaction.
func (t *Transactor) Begin(ctx context.Context) (ports.Tx, error) {
	return t.pool.Begin(ctx)
}

// UnwrapTx extracts the underlying pgx.Tx from a ports.Tx.
// Panics if the type assertion fails — this indicates a programming error.
func UnwrapTx(tx ports.Tx) pgx.Tx {
	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		panic("postgres.UnwrapTx: tx is not a pgx.Tx — mismatched transactor and repository")
	}
	return pgxTx
}
