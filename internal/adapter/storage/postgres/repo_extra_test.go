package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockPool(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock
}

// anyArgs matches n arguments of any value (pgxmock otherwise expects none).
func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func beginMockTx(t *testing.T, mock pgxmock.PgxPoolIface) pgx.Tx {
	t.Helper()
	mock.ExpectBegin()
	tx, err := mock.Begin(context.Background())
	require.NoError(t, err)
	return tx
}

// ---- Merchant ----

func TestMerchantRepo_CreateTx(t *testing.T) {
	mock := newMockPool(t)
	m := newTestMerchant()
	tx := beginMockTx(t, mock)
	mock.ExpectExec("INSERT INTO merchants").
		WithArgs(m.ID, m.Username, m.PasswordHash, m.MerchantName, m.AccessKey, m.SecretKeyEnc, m.WebhookURL, m.Status, m.CreatedAt, m.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	assert.NoError(t, NewMerchantRepo(mock).CreateTx(context.Background(), tx, m))
}

func TestMerchantRepo_CreateTx_Error(t *testing.T) {
	mock := newMockPool(t)
	tx := beginMockTx(t, mock)
	mock.ExpectExec("INSERT INTO merchants").WithArgs(anyArgs(10)...).WillReturnError(errors.New("duplicate key"))

	err := NewMerchantRepo(mock).CreateTx(context.Background(), tx, newTestMerchant())
	assert.ErrorContains(t, err, "insert merchant (tx)")
}

func TestMerchantRepo_Update(t *testing.T) {
	mock := newMockPool(t)
	m := newTestMerchant()
	mock.ExpectExec("UPDATE merchants").
		WithArgs(m.MerchantName, m.WebhookURL, m.AccessKey, m.SecretKeyEnc, m.Status, m.ID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	assert.NoError(t, NewMerchantRepo(mock).Update(context.Background(), m))

	mock.ExpectExec("UPDATE merchants").WithArgs(anyArgs(6)...).WillReturnError(errors.New("db down"))
	assert.ErrorContains(t, NewMerchantRepo(mock).Update(context.Background(), m), "update merchant")
}

// ---- Wallet ----

func TestWalletRepo_CreateTx(t *testing.T) {
	mock := newMockPool(t)
	w := newTestWallet(uuid.New())
	tx := beginMockTx(t, mock)
	mock.ExpectExec("INSERT INTO wallets").
		WithArgs(w.ID, w.MerchantID, w.Currency, w.EncryptedBalance, w.LastAuditHash, w.CreatedAt, w.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	assert.NoError(t, NewWalletRepo(mock).CreateTx(context.Background(), tx, w))
}

func TestWalletRepo_GetByMerchantID(t *testing.T) {
	mock := newMockPool(t)
	w := newTestWallet(uuid.New())
	repo := NewWalletRepo(mock)

	mock.ExpectQuery("SELECT .+ FROM wallets WHERE merchant_id = \\$1 AND currency = \\$2$").
		WithArgs(w.MerchantID, "VND").WillReturnRows(walletRow(w))
	got, err := repo.GetByMerchantID(context.Background(), w.MerchantID, "VND")
	require.NoError(t, err)
	assert.Equal(t, w.ID, got.ID)

	mock.ExpectQuery("SELECT .+ FROM wallets").WithArgs(anyArgs(2)...).WillReturnError(pgx.ErrNoRows)
	got, err = repo.GetByMerchantID(context.Background(), w.MerchantID, "USD")
	assert.NoError(t, err)
	assert.Nil(t, got, "missing wallet is not an error")

	mock.ExpectQuery("SELECT .+ FROM wallets").WithArgs(anyArgs(2)...).WillReturnError(errors.New("db down"))
	_, err = repo.GetByMerchantID(context.Background(), w.MerchantID, "VND")
	assert.Error(t, err)
}

// ---- Transaction list ----

func TestTransactionRepo_List_AllFilters(t *testing.T) {
	mock := newMockPool(t)
	merchantID := uuid.New()
	status, txType := domain.TransactionStatusSuccess, domain.TransactionTypePayment
	from, to := int64(1700000000), int64(1800000000)
	txn := newTestTransaction(merchantID, uuid.New())

	where := `WHERE merchant_id = \$1 AND status = \$2 AND transaction_type = \$3 AND created_at >= to_timestamp\(\$4\) AND created_at <= to_timestamp\(\$5\)`
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM transactions `+where).
		WithArgs(merchantID, status, txType, from, to).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(41)))
	mock.ExpectQuery(`FROM transactions `+where+` ORDER BY created_at DESC LIMIT \$6 OFFSET \$7`).
		WithArgs(merchantID, status, txType, from, to, 20, 40).
		WillReturnRows(txRow(txn))

	txns, total, err := NewTransactionRepo(mock).List(context.Background(), ports.TransactionListParams{
		MerchantID: merchantID, Status: &status, Type: &txType, From: &from, To: &to, Page: 3, PageSize: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(41), total)
	require.Len(t, txns, 1)
	assert.Equal(t, txn.ID, txns[0].ID)
}

func TestTransactionRepo_List_Errors(t *testing.T) {
	mock := newMockPool(t)
	repo := NewTransactionRepo(mock)
	params := ports.TransactionListParams{MerchantID: uuid.New(), Page: 1, PageSize: 10}

	mock.ExpectQuery("SELECT COUNT").WithArgs(anyArgs(1)...).WillReturnError(errors.New("db down"))
	_, _, err := repo.List(context.Background(), params)
	assert.ErrorContains(t, err, "count transactions")

	mock.ExpectQuery("SELECT COUNT").WithArgs(anyArgs(1)...).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("ORDER BY created_at DESC").WithArgs(anyArgs(3)...).WillReturnError(errors.New("db down"))
	_, _, err = repo.List(context.Background(), params)
	assert.ErrorContains(t, err, "list transactions")
}

// ---- Audit ----

func TestAuditRepo_Create(t *testing.T) {
	mock := newMockPool(t)
	merchantID := uuid.New()
	entry := &domain.AuditLog{
		ID: uuid.New(), MerchantID: &merchantID, Action: domain.AuditAction("PAYMENT_CREATE"),
		ResourceType: "transaction", ResourceID: "tx-1", Details: `{"amount":1000}`,
		IPAddress: "203.0.113.7", CreatedAt: time.Now(),
	}
	mock.ExpectExec("INSERT INTO audit_logs").
		WithArgs(entry.ID, entry.MerchantID, "PAYMENT_CREATE", "transaction", "tx-1", entry.Details, entry.IPAddress, entry.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	assert.NoError(t, NewAuditRepository(mock).Create(context.Background(), entry))
}

// ---- Webhook deliveries ----

func TestWebhookRepo_GetByTransactionID(t *testing.T) {
	mock := newMockPool(t)
	txID, id := uuid.New(), uuid.New()
	now := time.Now()
	status := 200
	mock.ExpectQuery("FROM webhook_delivery_logs\\s+WHERE transaction_id=\\$1").
		WithArgs(txID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "transaction_id", "merchant_id", "webhook_url", "payload",
			"http_status", "attempt", "status", "next_retry_at", "last_error", "created_at", "updated_at"}).
			AddRow(id, txID, uuid.New(), "https://m.example.com/hook", `{}`, &status, 1, "DELIVERED", (*time.Time)(nil), (*string)(nil), now, now))

	logs, err := NewWebhookRepository(mock).GetByTransactionID(context.Background(), txID)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, domain.WebhookStatusDelivered, logs[0].Status)
	assert.Equal(t, 200, *logs[0].HTTPStatus)
}

// ---- Health ----

func TestHealthCheck(t *testing.T) {
	mock := newMockPool(t)
	hc := NewHealthCheck(mock)
	assert.Equal(t, "postgresql", hc.Name())

	mock.ExpectExec("SELECT 1").WillReturnResult(pgxmock.NewResult("SELECT", 1))
	assert.NoError(t, hc.Ping(context.Background()))

	mock.ExpectExec("SELECT 1").WillReturnError(errors.New("connection refused"))
	assert.Error(t, hc.Ping(context.Background()))
}
