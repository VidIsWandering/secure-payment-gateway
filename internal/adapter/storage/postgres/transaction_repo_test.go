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

func newTestTransaction(merchantID, walletID uuid.UUID) *domain.Transaction {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Transaction{
		ID:                    uuid.New(),
		ReferenceID:           "ORDER-001",
		MerchantID:            merchantID,
		WalletID:              walletID,
		Amount:                100000,
		AmountEncrypted:       "aes_encrypted_amount",
		TransactionType:       domain.TransactionTypePayment,
		Status:                domain.TransactionStatusSuccess,
		Signature:             "hmac_sig_data",
		ClientIP:              "192.168.1.1",
		ExtraData:             strPtr("extra info"),
		OriginalTransactionID: nil,
		CreatedAt:             now,
		ProcessedAt:           &now,
	}
}

func txColumns() []string {
	return []string{"id", "reference_id", "merchant_id", "wallet_id", "amount", "amount_encrypted",
		"transaction_type", "status", "signature", "client_ip", "extra_data", "original_transaction_id",
		"created_at", "processed_at"}
}

func txRow(t *domain.Transaction) *pgxmock.Rows {
	return pgxmock.NewRows(txColumns()).AddRow(
		t.ID, t.ReferenceID, t.MerchantID, t.WalletID,
		t.Amount, t.AmountEncrypted, t.TransactionType, t.Status,
		t.Signature, t.ClientIP, t.ExtraData, t.OriginalTransactionID,
		t.CreatedAt, t.ProcessedAt,
	)
}

func TestTransactionRepo_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	txn := newTestTransaction(uuid.New(), uuid.New())

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").
		WithArgs(
			txn.ID, txn.ReferenceID, txn.MerchantID, txn.WalletID,
			txn.Amount, txn.AmountEncrypted, txn.TransactionType, txn.Status,
			txn.Signature, txn.ClientIP, txn.ExtraData, txn.OriginalTransactionID,
			txn.CreatedAt, txn.ProcessedAt,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	dbTx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	err = repo.Create(context.Background(), dbTx, txn)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_GetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	txn := newTestTransaction(uuid.New(), uuid.New())

	mock.ExpectQuery("SELECT .+ FROM transactions WHERE id").
		WithArgs(txn.ID).
		WillReturnRows(txRow(txn))

	result, err := repo.GetByID(context.Background(), txn.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txn.ID, result.ID)
	assert.Equal(t, txn.ReferenceID, result.ReferenceID)
	assert.Equal(t, txn.Amount, result.Amount)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)

	mock.ExpectQuery("SELECT .+ FROM transactions WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(txColumns()))

	result, err := repo.GetByID(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_GetByReference(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	txn := newTestTransaction(uuid.New(), uuid.New())

	mock.ExpectQuery("SELECT .+ FROM transactions WHERE merchant_id .+ AND reference_id").
		WithArgs(txn.MerchantID, txn.ReferenceID).
		WillReturnRows(txRow(txn))

	result, err := repo.GetByReference(context.Background(), txn.MerchantID, txn.ReferenceID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, txn.ReferenceID, result.ReferenceID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_UpdateStatus(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	txID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE transactions SET status").
		WithArgs(domain.TransactionStatusSuccess, pgxmock.AnyArg(), txID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	dbTx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	err = repo.UpdateStatus(context.Background(), dbTx, txID, domain.TransactionStatusSuccess)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_CheckRefundExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	origID := uuid.New()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(origID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	exists, err := repo.CheckRefundExists(context.Background(), origID)
	assert.NoError(t, err)
	assert.False(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_CheckRefundExists_True(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	origID := uuid.New()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(origID).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := repo.CheckRefundExists(context.Background(), origID)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestTransactionRepo_GetStats(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewTransactionRepo(mock)
	merchantID := uuid.New()

	mock.ExpectQuery("SELECT .+ FROM transactions WHERE merchant_id").
		WithArgs(merchantID).
		WillReturnRows(pgxmock.NewRows(
			[]string{"total", "successful", "failed", "reversed", "revenue", "refunded", "topup"},
		).AddRow(int64(100), int64(80), int64(15), int64(5), int64(5000000), int64(200000), int64(1000000)))

	stats, err := repo.GetStats(context.Background(), merchantID, nil)
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.TotalTransactions)
	assert.Equal(t, int64(80), stats.Successful)
	assert.Equal(t, int64(15), stats.Failed)
	assert.Equal(t, int64(5), stats.Reversed)
	assert.Equal(t, int64(5000000), stats.TotalRevenue)
	assert.NoError(t, mock.ExpectationsWereMet())
}
