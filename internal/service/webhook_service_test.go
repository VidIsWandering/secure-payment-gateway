package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports/mocks"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func newTestLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func httpResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(http.NoBody)}
}

type webhookTestDeps struct {
	svc          *webhookService
	merchantRepo *mocks.MockMerchantRepository
	webhookRepo  *mocks.MockWebhookRepository
	encSvc       *mocks.MockEncryptionService
	sigSvc       *mocks.MockSignatureService
	http         *mockHTTPClient
	now          time.Time
}

func setupWebhookService(t *testing.T) *webhookTestDeps {
	ctrl := gomock.NewController(t)
	d := &webhookTestDeps{
		merchantRepo: mocks.NewMockMerchantRepository(ctrl),
		webhookRepo:  mocks.NewMockWebhookRepository(ctrl),
		encSvc:       mocks.NewMockEncryptionService(ctrl),
		sigSvc:       mocks.NewMockSignatureService(ctrl),
		http:         &mockHTTPClient{},
		now:          time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC),
	}
	svc := NewWebhookService(d.merchantRepo, d.webhookRepo, d.encSvc, d.sigSvc, d.http, newTestLogger()).(*webhookService)
	svc.now = func() time.Time { return d.now }
	d.svc = svc
	return d
}

func testMerchant(webhookURL *string) *domain.Merchant {
	return &domain.Merchant{ID: uuid.New(), SecretKeyEnc: "enc_secret", WebhookURL: webhookURL, Status: domain.MerchantStatusActive}
}

// ==================== Enqueue ====================

func TestWebhookService_Enqueue_WritesPendingRowInCallerTx(t *testing.T) {
	d := setupWebhookService(t)
	url := "https://merchant.example.com/webhook"
	merchant := testMerchant(&url)
	dbTx := &mockTx{}
	txn := &domain.Transaction{
		ID: uuid.New(), MerchantID: merchant.ID, ReferenceID: "ORDER-1",
		Amount: 50000, TransactionType: domain.TransactionTypeRefund, Status: domain.TransactionStatusSuccess,
	}

	d.merchantRepo.EXPECT().GetByID(gomock.Any(), merchant.ID).Return(merchant, nil)
	var written *domain.WebhookDeliveryLog
	d.webhookRepo.EXPECT().CreateTx(gomock.Any(), dbTx, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ any, l *domain.WebhookDeliveryLog) error {
			written = l
			return nil
		})

	require.NoError(t, d.svc.Enqueue(context.Background(), dbTx, txn, "VND"))

	require.NotNil(t, written)
	assert.Equal(t, domain.WebhookStatusPending, written.Status)
	assert.Equal(t, 0, written.Attempt)
	assert.Equal(t, url, written.WebhookURL)
	assert.Equal(t, txn.ID, written.TransactionID)
	require.NotNil(t, written.NextRetryAt)
	assert.Equal(t, d.now, *written.NextRetryAt, "new rows are due immediately")

	var payload WebhookPayload
	require.NoError(t, json.Unmarshal([]byte(written.Payload), &payload))
	assert.Equal(t, EventRefundUpdate, payload.EventType)
	assert.Equal(t, "ORDER-1", payload.Data.MerchantOrderID)
	assert.Equal(t, int64(50000), payload.Data.Amount)
	assert.Equal(t, "VND", payload.Data.Currency)
	assert.Empty(t, payload.Signature, "signature is added at delivery time")
}

func TestWebhookService_Enqueue_NoWebhookURL(t *testing.T) {
	d := setupWebhookService(t)
	merchant := testMerchant(nil)
	d.merchantRepo.EXPECT().GetByID(gomock.Any(), merchant.ID).Return(merchant, nil)
	// No CreateTx expected.

	err := d.svc.Enqueue(context.Background(), &mockTx{}, &domain.Transaction{ID: uuid.New(), MerchantID: merchant.ID}, "VND")
	assert.NoError(t, err)
}

func TestWebhookService_Enqueue_PropagatesErrors(t *testing.T) {
	d := setupWebhookService(t)
	url := "https://merchant.example.com/webhook"
	merchant := testMerchant(&url)

	d.merchantRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	assert.Error(t, d.svc.Enqueue(context.Background(), &mockTx{}, &domain.Transaction{MerchantID: merchant.ID}, "VND"))

	d.merchantRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(merchant, nil)
	d.webhookRepo.EXPECT().CreateTx(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("insert failed"))
	assert.Error(t, d.svc.Enqueue(context.Background(), &mockTx{}, &domain.Transaction{MerchantID: merchant.ID}, "VND"),
		"an outbox write failure must abort the payment transaction")
}

// ==================== Delivery ====================

func pendingDelivery(t *testing.T, merchantID uuid.UUID, attempt int) domain.WebhookDeliveryLog {
	payload, err := json.Marshal(WebhookPayload{EventType: EventPaymentUpdate, Data: WebhookPayloadData{MerchantOrderID: "ORDER-1", Amount: 1000}})
	require.NoError(t, err)
	return domain.WebhookDeliveryLog{
		ID: uuid.New(), TransactionID: uuid.New(), MerchantID: merchantID,
		WebhookURL: "https://merchant.example.com/webhook", Payload: string(payload),
		Attempt: attempt, Status: domain.WebhookStatusPending,
	}
}

func TestWebhookService_Dispatch_DeliversSignedPayload(t *testing.T) {
	d := setupWebhookService(t)
	merchant := testMerchant(nil)
	delivery := pendingDelivery(t, merchant.ID, 0)

	d.webhookRepo.EXPECT().ClaimDue(gomock.Any(), webhookBatchSize, webhookLease).Return([]domain.WebhookDeliveryLog{delivery}, nil)
	d.merchantRepo.EXPECT().GetByID(gomock.Any(), merchant.ID).Return(merchant, nil)
	d.encSvc.EXPECT().Decrypt("enc_secret").Return("raw_secret", nil)
	d.sigSvc.EXPECT().Sign("raw_secret", gomock.Any()).Return("sig123")

	var sent WebhookPayload
	d.http.doFunc = func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, delivery.ID.String(), req.Header.Get(HeaderWebhookID))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(req.Body).Decode(&sent))
		return httpResponse(http.StatusOK), nil
	}
	d.webhookRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *domain.WebhookDeliveryLog) error {
		assert.Equal(t, domain.WebhookStatusDelivered, l.Status)
		assert.Equal(t, 1, l.Attempt)
		assert.Nil(t, l.NextRetryAt)
		require.NotNil(t, l.HTTPStatus)
		assert.Equal(t, http.StatusOK, *l.HTTPStatus)
		return nil
	})

	d.svc.dispatchDue(context.Background())

	assert.Equal(t, "sig123", sent.Signature)
	assert.Equal(t, "ORDER-1", sent.Data.MerchantOrderID)
}

func TestWebhookService_Dispatch_SchedulesRetryWithBackoff(t *testing.T) {
	d := setupWebhookService(t)
	merchant := testMerchant(nil)
	delivery := pendingDelivery(t, merchant.ID, 1) // second attempt

	d.webhookRepo.EXPECT().ClaimDue(gomock.Any(), gomock.Any(), gomock.Any()).Return([]domain.WebhookDeliveryLog{delivery}, nil)
	d.merchantRepo.EXPECT().GetByID(gomock.Any(), merchant.ID).Return(merchant, nil)
	d.encSvc.EXPECT().Decrypt(gomock.Any()).Return("raw_secret", nil)
	d.sigSvc.EXPECT().Sign(gomock.Any(), gomock.Any()).Return("sig")
	d.http.doFunc = func(*http.Request) (*http.Response, error) { return httpResponse(http.StatusInternalServerError), nil }
	d.webhookRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *domain.WebhookDeliveryLog) error {
		assert.Equal(t, domain.WebhookStatusPending, l.Status)
		assert.Equal(t, 2, l.Attempt)
		require.NotNil(t, l.NextRetryAt)
		assert.Equal(t, d.now.Add(webhookRetryIntervals[1]), *l.NextRetryAt)
		require.NotNil(t, l.LastError)
		assert.Equal(t, "HTTP 500", *l.LastError)
		return nil
	})

	d.svc.dispatchDue(context.Background())
}

func TestWebhookService_Dispatch_FailsAfterLastRetry(t *testing.T) {
	d := setupWebhookService(t)
	merchant := testMerchant(nil)
	delivery := pendingDelivery(t, merchant.ID, len(webhookRetryIntervals)) // final attempt

	d.webhookRepo.EXPECT().ClaimDue(gomock.Any(), gomock.Any(), gomock.Any()).Return([]domain.WebhookDeliveryLog{delivery}, nil)
	d.merchantRepo.EXPECT().GetByID(gomock.Any(), merchant.ID).Return(merchant, nil)
	d.encSvc.EXPECT().Decrypt(gomock.Any()).Return("raw_secret", nil)
	d.sigSvc.EXPECT().Sign(gomock.Any(), gomock.Any()).Return("sig")
	d.http.doFunc = func(*http.Request) (*http.Response, error) { return nil, errors.New("connection refused") }
	d.webhookRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, l *domain.WebhookDeliveryLog) error {
		assert.Equal(t, domain.WebhookStatusFailed, l.Status)
		assert.Equal(t, len(webhookRetryIntervals)+1, l.Attempt)
		assert.Nil(t, l.NextRetryAt)
		return nil
	})

	d.svc.dispatchDue(context.Background())
}

func TestWebhookService_Dispatch_ClaimErrorIsNonFatal(t *testing.T) {
	d := setupWebhookService(t)
	d.webhookRepo.EXPECT().ClaimDue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	d.svc.dispatchDue(context.Background()) // must not panic or deliver anything
}

func TestWebhookService_Run_StopsOnCancel(t *testing.T) {
	d := setupWebhookService(t)
	var polls atomic.Int32
	d.webhookRepo.EXPECT().ClaimDue(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, int, time.Duration) ([]domain.WebhookDeliveryLog, error) {
			polls.Add(1)
			return nil, nil
		}).AnyTimes()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.svc.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return polls.Load() >= 1 }, time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}
