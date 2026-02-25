package postgres

import (
	"context"

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
func (t *Transactor) Begin(ctx context.Context) (pgx.Tx, error) {
	return t.pool.Begin(ctx)
}
