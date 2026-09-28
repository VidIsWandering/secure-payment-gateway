package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
)

// TestWebhookOutbox_RealDatabase exercises the outbox SQL against PostgreSQL
// when SPG_TEST_DATABASE_URL is set (CI provides one); skipped otherwise.
func TestWebhookOutbox_RealDatabase(t *testing.T) {
	dsn := os.Getenv("SPG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SPG_TEST_DATABASE_URL not set")
	}
	_, err := Migrate(dsn)
	require.NoError(t, err)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	repo := NewWebhookRepository(pool)

	// Fixture: merchant -> wallet -> transaction (FK targets of the outbox row).
	merchantID, walletID, txID := uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO merchants (id, username, password_hash, merchant_name, access_key, secret_key_enc)
		VALUES ($1, $2, 'h', 'Outbox Test', $3, 'enc')`, merchantID, "outbox_"+merchantID.String()[:8], "ak_"+merchantID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO wallets (id, merchant_id, currency, encrypted_balance) VALUES ($1, $2, 'VND', 'enc')`, walletID, merchantID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO transactions (id, reference_id, merchant_id, wallet_id, amount, amount_encrypted, transaction_type, status, signature)
		VALUES ($1, 'OUTBOX-1', $2, $3, 1000, 'enc', 'PAYMENT', 'SUCCESS', 'sig')`, txID, merchantID, walletID)
	require.NoError(t, err)

	newRow := func() *domain.WebhookDeliveryLog {
		now := time.Now()
		return &domain.WebhookDeliveryLog{
			ID: uuid.New(), TransactionID: txID, MerchantID: merchantID,
			WebhookURL: "https://merchant.example.com/webhook", Payload: `{"event_type":"PAYMENT_UPDATE"}`,
			Status: domain.WebhookStatusPending, NextRetryAt: &now, CreatedAt: now, UpdatedAt: now,
		}
	}
	claimedIDs := func() map[uuid.UUID]domain.WebhookDeliveryLog {
		logs, err := repo.ClaimDue(ctx, 100, time.Minute)
		require.NoError(t, err)
		out := map[uuid.UUID]domain.WebhookDeliveryLog{}
		for _, l := range logs {
			out[l.ID] = l
		}
		return out
	}

	// A rolled-back payment leaves no webhook behind.
	rolledBack := newRow()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateTx(ctx, tx, rolledBack))
	require.NoError(t, tx.Rollback(ctx))
	assert.NotContains(t, claimedIDs(), rolledBack.ID)

	// A committed row is claimed exactly once while leased.
	committed := newRow()
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.CreateTx(ctx, tx, committed))
	require.NoError(t, tx.Commit(ctx))

	first := claimedIDs()
	require.Contains(t, first, committed.ID)
	got := first[committed.ID]
	assert.Equal(t, `{"event_type": "PAYMENT_UPDATE"}`, got.Payload) // jsonb normalises whitespace
	require.NotNil(t, got.NextRetryAt)
	assert.WithinDuration(t, time.Now().Add(time.Minute), *got.NextRetryAt, 10*time.Second, "lease pushes next_retry_at forward")
	assert.NotContains(t, claimedIDs(), committed.ID, "leased row must not be handed out twice")

	// Once delivered it is never claimed again, even after the lease would expire.
	got.Status = domain.WebhookStatusDelivered
	got.Attempt = 1
	past := time.Now().Add(-time.Hour)
	got.NextRetryAt = &past
	require.NoError(t, repo.Update(ctx, &got))
	assert.NotContains(t, claimedIDs(), committed.ID)
}
