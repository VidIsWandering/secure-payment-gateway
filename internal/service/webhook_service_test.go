package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports/mocks"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
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

func TestWebhookService_EnqueueWebhook_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)

	delivered := make(chan struct{}, 1)
	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			delivered <- struct{}{}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(nil),
			}, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger())

	merchantID := uuid.New()
	walletID := uuid.New()
	webhookURL := "https://merchant.example.com/webhook"

	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:           merchantID,
		SecretKeyEnc: "encrypted-secret",
		WebhookURL:   &webhookURL,
	}, nil)
	mockWalletRepo.EXPECT().GetByID(gomock.Any(), walletID).Return(&domain.Wallet{
		ID:       walletID,
		Currency: "VND",
	}, nil)
	mockEncSvc.EXPECT().Decrypt("encrypted-secret").Return("secret-key", nil)
	mockSigSvc.EXPECT().Sign("secret-key", gomock.Any()).Return("signature-hash")

	tx := &domain.Transaction{
		ID:              uuid.New(),
		ReferenceID:     "ref-001",
		MerchantID:      merchantID,
		WalletID:        walletID,
		Amount:          50000,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
		CreatedAt:       time.Now(),
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.NoError(t, err)

	// Wait for async delivery
	select {
	case <-delivered:
	// OK
	case <-time.After(2 * time.Second):
		t.Fatal("webhook delivery timed out")
	}
}

func TestWebhookService_EnqueueWebhook_NoWebhookURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)

	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			t.Fatal("should not be called")
			return nil, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger())

	merchantID := uuid.New()
	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:         merchantID,
		WebhookURL: nil,
	}, nil)

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.NoError(t, err)
}

func TestWebhookService_EnqueueWebhook_MerchantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)

	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger())

	merchantID := uuid.New()
	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(nil, errors.New("db error"))

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		TransactionType: domain.TransactionTypePayment,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.Error(t, err)
}

func TestWebhookService_EnqueueWebhook_DecryptError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)

	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger())

	merchantID := uuid.New()
	walletID := uuid.New()
	webhookURL := "https://merchant.example.com/webhook"

	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:           merchantID,
		SecretKeyEnc: "bad-encrypted",
		WebhookURL:   &webhookURL,
	}, nil)
	mockWalletRepo.EXPECT().GetByID(gomock.Any(), walletID).Return(&domain.Wallet{
		ID:       walletID,
		Currency: "VND",
	}, nil)
	mockEncSvc.EXPECT().Decrypt("bad-encrypted").Return("", errors.New("decrypt failed"))

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		WalletID:        walletID,
		TransactionType: domain.TransactionTypeRefund,
		Status:          domain.TransactionStatusSuccess,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.Error(t, err)
}

func TestWebhookService_EventType_Refund(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)

	var capturedReq *http.Request
	delivered := make(chan struct{}, 1)
	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			delivered <- struct{}{}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(nil),
			}, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger())

	merchantID := uuid.New()
	walletID := uuid.New()
	webhookURL := "https://merchant.example.com/webhook"

	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:           merchantID,
		SecretKeyEnc: "enc-secret",
		WebhookURL:   &webhookURL,
	}, nil)
	mockWalletRepo.EXPECT().GetByID(gomock.Any(), walletID).Return(&domain.Wallet{
		ID:       walletID,
		Currency: "USD",
	}, nil)
	mockEncSvc.EXPECT().Decrypt("enc-secret").Return("key", nil)
	mockSigSvc.EXPECT().Sign("key", gomock.Any()).Return("sig")

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		WalletID:        walletID,
		Amount:          10000,
		TransactionType: domain.TransactionTypeRefund,
		Status:          domain.TransactionStatusSuccess,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.NoError(t, err)

	select {
	case <-delivered:
		assert.NotNil(t, capturedReq)
		assert.Equal(t, "application/json", capturedReq.Header.Get("Content-Type"))
	case <-time.After(2 * time.Second):
		t.Fatal("webhook delivery timed out")
	}
}

func TestWebhookService_PersistsDeliveryLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)
	mockWebhookRepo := mocks.NewMockWebhookRepository(ctrl)

	delivered := make(chan struct{}, 1)
	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			delivered <- struct{}{}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(nil),
			}, nil
		},
	}

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger(), mockWebhookRepo)

	merchantID := uuid.New()
	walletID := uuid.New()
	webhookURL := "https://merchant.example.com/webhook"

	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:           merchantID,
		SecretKeyEnc: "enc",
		WebhookURL:   &webhookURL,
	}, nil)
	mockWalletRepo.EXPECT().GetByID(gomock.Any(), walletID).Return(&domain.Wallet{
		ID:       walletID,
		Currency: "VND",
	}, nil)
	mockEncSvc.EXPECT().Decrypt("enc").Return("key", nil)
	mockSigSvc.EXPECT().Sign("key", gomock.Any()).Return("sig")

	// Expect: Create (initial PENDING log) then Update (DELIVERED after success)
	mockWebhookRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, log *domain.WebhookDeliveryLog) error {
			assert.Equal(t, domain.WebhookStatusPending, log.Status)
			assert.Equal(t, 0, log.Attempt)
			return nil
		},
	)
	mockWebhookRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, log *domain.WebhookDeliveryLog) error {
			assert.Equal(t, domain.WebhookStatusDelivered, log.Status)
			assert.Equal(t, 1, log.Attempt)
			assert.NotNil(t, log.HTTPStatus)
			assert.Equal(t, 200, *log.HTTPStatus)
			return nil
		},
	)

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		WalletID:        walletID,
		Amount:          10000,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.NoError(t, err)

	select {
	case <-delivered:
		time.Sleep(50 * time.Millisecond) // give goroutine time to persist
	case <-time.After(2 * time.Second):
		t.Fatal("webhook delivery timed out")
	}
}

func TestWebhookService_PersistsFailedDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMerchantRepo := mocks.NewMockMerchantRepository(ctrl)
	mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
	mockEncSvc := mocks.NewMockEncryptionService(ctrl)
	mockSigSvc := mocks.NewMockSignatureService(ctrl)
	mockWebhookRepo := mocks.NewMockWebhookRepository(ctrl)

	done := make(chan struct{}, 1)
	httpClient := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("connection refused")
		},
	}

	// Override retry intervals to make test fast
	orig := webhookRetryIntervals
	webhookRetryIntervals = []time.Duration{1 * time.Millisecond}
	defer func() { webhookRetryIntervals = orig }()

	svc := NewWebhookService(mockMerchantRepo, mockWalletRepo, mockEncSvc, mockSigSvc, httpClient, newTestLogger(), mockWebhookRepo)

	merchantID := uuid.New()
	walletID := uuid.New()
	webhookURL := "https://merchant.example.com/webhook"

	mockMerchantRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
		ID:           merchantID,
		SecretKeyEnc: "enc",
		WebhookURL:   &webhookURL,
	}, nil)
	mockWalletRepo.EXPECT().GetByID(gomock.Any(), walletID).Return(&domain.Wallet{
		ID:       walletID,
		Currency: "VND",
	}, nil)
	mockEncSvc.EXPECT().Decrypt("enc").Return("key", nil)
	mockSigSvc.EXPECT().Sign("key", gomock.Any()).Return("sig")

	// Expect Create, then Update for each attempt, then final FAILED Update
	mockWebhookRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	// 2 attempts (initial + 1 retry) => multiple Updates, last one FAILED
	mockWebhookRepo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, log *domain.WebhookDeliveryLog) error {
			if log.Status == domain.WebhookStatusFailed {
				done <- struct{}{}
			}
			return nil
		},
	).AnyTimes()

	tx := &domain.Transaction{
		ID:              uuid.New(),
		MerchantID:      merchantID,
		WalletID:        walletID,
		Amount:          10000,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
	}

	err := svc.EnqueueWebhook(context.Background(), tx)
	assert.NoError(t, err)

	select {
	case <-done:
		// OK - all retries exhausted with FAILED status
	case <-time.After(5 * time.Second):
		t.Fatal("webhook retry timed out")
	}
}
