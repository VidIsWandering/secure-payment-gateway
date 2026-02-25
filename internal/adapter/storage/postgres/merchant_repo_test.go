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

func newTestMerchant() *domain.Merchant {
	return &domain.Merchant{
		ID:           uuid.New(),
		Username:     "test_user",
		PasswordHash: "$argon2id$v=19$m=65536,t=1,p=4$salt$hash",
		MerchantName: "Test Shop",
		AccessKey:    "ak_" + uuid.New().String()[:16],
		SecretKeyEnc: "encrypted_secret_key_data",
		WebhookURL:   strPtr("https://example.com/webhook"),
		Status:       domain.MerchantStatusActive,
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
		UpdatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
}

func strPtr(s string) *string { return &s }

func merchantColumns() []string {
	return []string{"id", "username", "password_hash", "merchant_name", "access_key", "secret_key_enc", "webhook_url", "status", "created_at", "updated_at"}
}

func merchantRow(m *domain.Merchant) *pgxmock.Rows {
	return pgxmock.NewRows(merchantColumns()).AddRow(
		m.ID, m.Username, m.PasswordHash, m.MerchantName,
		m.AccessKey, m.SecretKeyEnc, m.WebhookURL, m.Status,
		m.CreatedAt, m.UpdatedAt,
	)
}

func TestMerchantRepo_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMerchantRepo(mock)
	m := newTestMerchant()

	mock.ExpectExec("INSERT INTO merchants").
		WithArgs(m.ID, m.Username, m.PasswordHash, m.MerchantName,
			m.AccessKey, m.SecretKeyEnc, m.WebhookURL, m.Status,
			m.CreatedAt, m.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), m)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMerchantRepo_GetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMerchantRepo(mock)
	m := newTestMerchant()

	mock.ExpectQuery("SELECT .+ FROM merchants WHERE id").
		WithArgs(m.ID).
		WillReturnRows(merchantRow(m))

	result, err := repo.GetByID(context.Background(), m.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, m.ID, result.ID)
	assert.Equal(t, m.Username, result.Username)
	assert.Equal(t, m.AccessKey, result.AccessKey)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMerchantRepo_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMerchantRepo(mock)

	mock.ExpectQuery("SELECT .+ FROM merchants WHERE id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(merchantColumns()))

	result, err := repo.GetByID(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMerchantRepo_GetByAccessKey(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMerchantRepo(mock)
	m := newTestMerchant()

	mock.ExpectQuery("SELECT .+ FROM merchants WHERE access_key").
		WithArgs(m.AccessKey).
		WillReturnRows(merchantRow(m))

	result, err := repo.GetByAccessKey(context.Background(), m.AccessKey)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, m.AccessKey, result.AccessKey)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMerchantRepo_GetByUsername(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewMerchantRepo(mock)
	m := newTestMerchant()

	mock.ExpectQuery("SELECT .+ FROM merchants WHERE username").
		WithArgs(m.Username).
		WillReturnRows(merchantRow(m))

	result, err := repo.GetByUsername(context.Background(), m.Username)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, m.Username, result.Username)
	assert.NoError(t, mock.ExpectationsWereMet())
}
