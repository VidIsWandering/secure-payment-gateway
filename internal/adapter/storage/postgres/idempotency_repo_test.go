package postgres

import (
	"context"
	"testing"
	"time"

	"secure-payment-gateway/internal/core/domain"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyRepo_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIdempotencyRepo(mock)
	log := &domain.IdempotencyLog{
		Key:           "merchant-id:ORDER-001",
		TransactionID: uuid.New(),
		ResponseJSON:  []byte(`{"status":"SUCCESS"}`),
		CreatedAt:     time.Now().UTC().Truncate(time.Microsecond),
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO idempotency_logs").
		WithArgs(log.Key, log.TransactionID, log.ResponseJSON, log.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	err = repo.Create(context.Background(), tx, log)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIdempotencyRepo_Get(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIdempotencyRepo(mock)
	txID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	mock.ExpectQuery("SELECT .+ FROM idempotency_logs WHERE key").
		WithArgs("merchant-id:ORDER-001").
		WillReturnRows(pgxmock.NewRows([]string{"key", "transaction_id", "response_json", "created_at"}).
			AddRow("merchant-id:ORDER-001", txID, []byte(`{"status":"SUCCESS"}`), now))

	result, err := repo.Get(context.Background(), "merchant-id:ORDER-001")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txID, result.TransactionID)
	assert.Equal(t, []byte(`{"status":"SUCCESS"}`), result.ResponseJSON)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestIdempotencyRepo_Get_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIdempotencyRepo(mock)

	mock.ExpectQuery("SELECT .+ FROM idempotency_logs WHERE key").
		WithArgs("nonexistent-key").
		WillReturnRows(pgxmock.NewRows([]string{"key", "transaction_id", "response_json", "created_at"}))

	result, err := repo.Get(context.Background(), "nonexistent-key")
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}
