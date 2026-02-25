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

func newTestWallet(merchantID uuid.UUID) *domain.Wallet {
	return &domain.Wallet{
		ID:               uuid.New(),
		MerchantID:       merchantID,
		Currency:         "VND",
		EncryptedBalance: "aes_encrypted_balance_data",
		LastAuditHash:    nil,
		CreatedAt:        time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:        time.Now().UTC().Truncate(time.Microsecond),
	}
}

func walletColumns() []string {
	return []string{"id", "merchant_id", "currency", "encrypted_balance", "last_audit_hash", "created_at", "updated_at"}
}

func walletRow(w *domain.Wallet) *pgxmock.Rows {
	return pgxmock.NewRows(walletColumns()).AddRow(
		w.ID, w.MerchantID, w.Currency, w.EncryptedBalance,
		w.LastAuditHash, w.CreatedAt, w.UpdatedAt,
	)
}

func TestWalletRepo_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	w := newTestWallet(uuid.New())

	mock.ExpectExec("INSERT INTO wallets").
		WithArgs(w.ID, w.MerchantID, w.Currency, w.EncryptedBalance,
			w.LastAuditHash, w.CreatedAt, w.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), w)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepo_GetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	w := newTestWallet(uuid.New())

	mock.ExpectQuery("SELECT .+ FROM wallets WHERE id").
		WithArgs(w.ID).
		WillReturnRows(walletRow(w))

	result, err := repo.GetByID(context.Background(), w.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, w.ID, result.ID)
	assert.Equal(t, w.EncryptedBalance, result.EncryptedBalance)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepo_GetByMerchantIDForUpdate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	w := newTestWallet(uuid.New())

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .+ FROM wallets WHERE merchant_id .+ FOR UPDATE").
		WithArgs(w.MerchantID, "VND").
		WillReturnRows(walletRow(w))

	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	result, err := repo.GetByMerchantIDForUpdate(context.Background(), tx, w.MerchantID, "VND")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, w.ID, result.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepo_GetByIDForUpdate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	w := newTestWallet(uuid.New())

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .+ FROM wallets WHERE id .+ FOR UPDATE").
		WithArgs(w.ID).
		WillReturnRows(walletRow(w))

	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	result, err := repo.GetByIDForUpdate(context.Background(), tx, w.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, w.ID, result.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepo_UpdateBalance(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	walletID := uuid.New()
	newBalance := "new_encrypted_balance"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets SET encrypted_balance").
		WithArgs(newBalance, walletID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	err = repo.UpdateBalance(context.Background(), tx, walletID, newBalance)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepo_UpdateBalance_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewWalletRepo(mock)
	walletID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets SET encrypted_balance").
		WithArgs("enc_bal", walletID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)

	err = repo.UpdateBalance(context.Background(), tx, walletID, "enc_bal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wallet not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}
