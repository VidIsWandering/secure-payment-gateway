package service

import (
	"context"
	"encoding/json"
	"testing"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/internal/core/ports/mocks"
	"secure-payment-gateway/pkg/apperror"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type paymentTestDeps struct {
	svc        *PaymentServiceImpl
	txRepo     *mocks.MockTransactionRepository
	walletRepo *mocks.MockWalletRepository
	idempRepo  *mocks.MockIdempotencyRepository
	idempCache *mocks.MockIdempotencyCache
	encSvc     *mocks.MockEncryptionService
	transactor *mocks.MockDBTransactor
	ctrl       *gomock.Controller
}

func setupPaymentService(t *testing.T) *paymentTestDeps {
	ctrl := gomock.NewController(t)
	d := &paymentTestDeps{
		txRepo:     mocks.NewMockTransactionRepository(ctrl),
		walletRepo: mocks.NewMockWalletRepository(ctrl),
		idempRepo:  mocks.NewMockIdempotencyRepository(ctrl),
		idempCache: mocks.NewMockIdempotencyCache(ctrl),
		encSvc:     mocks.NewMockEncryptionService(ctrl),
		transactor: mocks.NewMockDBTransactor(ctrl),
		ctrl:       ctrl,
	}
	d.svc = NewPaymentService(
		d.txRepo, d.walletRepo, d.idempRepo, d.idempCache,
		d.encSvc, d.transactor, zerolog.Nop(),
	)
	return d
}

// mockTx implements pgx.Tx for testing
type mockTx struct{ pgx.Tx }

func (m *mockTx) Rollback(_ context.Context) error { return nil }
func (m *mockTx) Commit(_ context.Context) error   { return nil }

// ==================== ProcessPayment Tests ====================

func TestPaymentService_ProcessPayment_Success(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	walletID := uuid.New()
	tx := &mockTx{}

	req := ports.PaymentRequest{
		MerchantID:  merchantID,
		ReferenceID: "ORDER-001",
		Amount:      50000,
		Currency:    "VND",
		Signature:   "sig_valid",
		ClientIP:    "1.2.3.4",
	}

	idempKey := domain.BuildIdempotencyKey(merchantID, "ORDER-001")

	// Redis cache miss
	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	// DB idempotency miss
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	// Begin tx
	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	// Lock wallet
	d.walletRepo.EXPECT().GetByMerchantIDForUpdate(ctx, tx, merchantID, "VND").Return(&domain.Wallet{
		ID:               walletID,
		MerchantID:       merchantID,
		Currency:         "VND",
		EncryptedBalance: "enc_100000",
	}, nil)
	// Decrypt balance
	d.encSvc.EXPECT().Decrypt("enc_100000").Return("100000", nil)
	// Encrypt new balance (100000 - 50000 = 50000)
	d.encSvc.EXPECT().Encrypt("50000").Return("enc_50000", nil)
	// Encrypt amount for audit
	d.encSvc.EXPECT().Encrypt("50000").Return("enc_amount_50000", nil)
	// Update wallet balance
	d.walletRepo.EXPECT().UpdateBalance(ctx, tx, walletID, "enc_50000").Return(nil)
	// Create transaction
	d.txRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	// Save idempotency log
	d.idempRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	// Cache in Redis
	d.idempCache.EXPECT().Set(ctx, idempKey, gomock.Any(), idempotencyTTL).Return(nil)

	result, err := d.svc.ProcessPayment(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain.TransactionTypePayment, result.TransactionType)
	assert.Equal(t, domain.TransactionStatusSuccess, result.Status)
	assert.Equal(t, int64(50000), result.Amount)
	assert.Equal(t, merchantID, result.MerchantID)
}

func TestPaymentService_ProcessPayment_InvalidAmount(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	req := ports.PaymentRequest{
		MerchantID:  uuid.New(),
		ReferenceID: "ORDER-002",
		Amount:      0,
		Currency:    "VND",
	}

	result, err := d.svc.ProcessPayment(context.Background(), req)
	assert.Nil(t, result)
	require.Error(t, err)
	assertAppError(t, err, "PAY_002")
}

func TestPaymentService_ProcessPayment_InsufficientFunds(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	walletID := uuid.New()
	tx := &mockTx{}

	req := ports.PaymentRequest{
		MerchantID:  merchantID,
		ReferenceID: "ORDER-003",
		Amount:      200000,
		Currency:    "VND",
		Signature:   "sig",
	}

	idempKey := domain.BuildIdempotencyKey(merchantID, "ORDER-003")

	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	d.walletRepo.EXPECT().GetByMerchantIDForUpdate(ctx, tx, merchantID, "VND").Return(&domain.Wallet{
		ID: walletID, MerchantID: merchantID, EncryptedBalance: "enc_100000",
	}, nil)
	d.encSvc.EXPECT().Decrypt("enc_100000").Return("100000", nil)

	result, err := d.svc.ProcessPayment(ctx, req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_001")
}

func TestPaymentService_ProcessPayment_IdempotentRedisHit(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()

	cachedTx := &domain.Transaction{
		ID:     uuid.New(),
		Status: domain.TransactionStatusSuccess,
		Amount: 50000,
	}
	cachedJSON, _ := json.Marshal(cachedTx)

	idempKey := domain.BuildIdempotencyKey(merchantID, "ORDER-CACHED")
	d.idempCache.EXPECT().Get(ctx, idempKey).Return(cachedJSON, nil)

	req := ports.PaymentRequest{
		MerchantID:  merchantID,
		ReferenceID: "ORDER-CACHED",
		Amount:      50000,
		Currency:    "VND",
	}

	result, err := d.svc.ProcessPayment(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, cachedTx.ID, result.ID)
}

// ==================== ProcessRefund Tests ====================

func TestPaymentService_ProcessRefund_FullRefund(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	walletID := uuid.New()
	origTxID := uuid.New()
	tx := &mockTx{}

	req := ports.RefundRequest{
		MerchantID:          merchantID,
		OriginalReferenceID: "ORDER-001",
		Amount:              nil, // full refund
		Reason:              "Customer request",
		Signature:           "sig",
		ClientIP:            "1.2.3.4",
	}

	idempKey := domain.BuildRefundIdempotencyKey(merchantID, "ORDER-001")

	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	// Find original transaction
	d.txRepo.EXPECT().GetByReference(ctx, merchantID, "ORDER-001").Return(&domain.Transaction{
		ID:              origTxID,
		MerchantID:      merchantID,
		WalletID:        walletID,
		Amount:          100000,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
	}, nil)
	// Check no existing refund
	d.txRepo.EXPECT().CheckRefundExists(ctx, origTxID).Return(false, nil)
	// Begin tx
	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	// Lock wallet by ID
	d.walletRepo.EXPECT().GetByIDForUpdate(ctx, tx, walletID).Return(&domain.Wallet{
		ID: walletID, MerchantID: merchantID, EncryptedBalance: "enc_50000",
	}, nil)
	// Decrypt balance
	d.encSvc.EXPECT().Decrypt("enc_50000").Return("50000", nil)
	// Encrypt new balance (50000 + 100000 = 150000)
	d.encSvc.EXPECT().Encrypt("150000").Return("enc_150000", nil)
	// Encrypt refund amount
	d.encSvc.EXPECT().Encrypt("100000").Return("enc_refund_100000", nil)
	// Update wallet balance
	d.walletRepo.EXPECT().UpdateBalance(ctx, tx, walletID, "enc_150000").Return(nil)
	// Create refund transaction
	d.txRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	// Mark original as REVERSED
	d.txRepo.EXPECT().UpdateStatus(ctx, tx, origTxID, domain.TransactionStatusReversed).Return(nil)
	// Save idempotency log
	d.idempRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	// Cache in Redis
	d.idempCache.EXPECT().Set(ctx, idempKey, gomock.Any(), idempotencyTTL).Return(nil)

	result, err := d.svc.ProcessRefund(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain.TransactionTypeRefund, result.TransactionType)
	assert.Equal(t, domain.TransactionStatusSuccess, result.Status)
	assert.Equal(t, int64(100000), result.Amount) // full refund
	assert.Equal(t, &origTxID, result.OriginalTransactionID)
}

func TestPaymentService_ProcessRefund_PartialRefund(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	walletID := uuid.New()
	origTxID := uuid.New()
	tx := &mockTx{}
	refundAmount := int64(30000)

	req := ports.RefundRequest{
		MerchantID:          merchantID,
		OriginalReferenceID: "ORDER-002",
		Amount:              &refundAmount,
		Reason:              "Partial refund",
		Signature:           "sig",
	}

	idempKey := domain.BuildRefundIdempotencyKey(merchantID, "ORDER-002")

	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.txRepo.EXPECT().GetByReference(ctx, merchantID, "ORDER-002").Return(&domain.Transaction{
		ID: origTxID, MerchantID: merchantID, WalletID: walletID,
		Amount: 100000, TransactionType: domain.TransactionTypePayment, Status: domain.TransactionStatusSuccess,
	}, nil)
	d.txRepo.EXPECT().CheckRefundExists(ctx, origTxID).Return(false, nil)
	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	d.walletRepo.EXPECT().GetByIDForUpdate(ctx, tx, walletID).Return(&domain.Wallet{
		ID: walletID, EncryptedBalance: "enc_0",
	}, nil)
	d.encSvc.EXPECT().Decrypt("enc_0").Return("0", nil)
	d.encSvc.EXPECT().Encrypt("30000").Return("enc_30000", nil)
	d.encSvc.EXPECT().Encrypt("30000").Return("enc_refund_30000", nil)
	d.walletRepo.EXPECT().UpdateBalance(ctx, tx, walletID, "enc_30000").Return(nil)
	d.txRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	d.txRepo.EXPECT().UpdateStatus(ctx, tx, origTxID, domain.TransactionStatusReversed).Return(nil)
	d.idempRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)
	d.idempCache.EXPECT().Set(ctx, idempKey, gomock.Any(), idempotencyTTL).Return(nil)

	result, err := d.svc.ProcessRefund(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(30000), result.Amount)
}

func TestPaymentService_ProcessRefund_OriginalNotFound(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()

	req := ports.RefundRequest{
		MerchantID:          merchantID,
		OriginalReferenceID: "NONEXISTENT",
		Signature:           "sig",
	}

	idempKey := domain.BuildRefundIdempotencyKey(merchantID, "NONEXISTENT")
	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.txRepo.EXPECT().GetByReference(ctx, merchantID, "NONEXISTENT").Return(nil, nil)

	result, err := d.svc.ProcessRefund(ctx, req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_004")
}

func TestPaymentService_ProcessRefund_NotRefundable(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()

	req := ports.RefundRequest{
		MerchantID:          merchantID,
		OriginalReferenceID: "ORDER-FAILED",
		Signature:           "sig",
	}

	idempKey := domain.BuildRefundIdempotencyKey(merchantID, "ORDER-FAILED")
	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.txRepo.EXPECT().GetByReference(ctx, merchantID, "ORDER-FAILED").Return(&domain.Transaction{
		ID:              uuid.New(),
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusFailed, // Not SUCCESS
	}, nil)

	result, err := d.svc.ProcessRefund(ctx, req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_006")
}

func TestPaymentService_ProcessRefund_AmountExceeds(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	origTxID := uuid.New()
	over := int64(999999)

	req := ports.RefundRequest{
		MerchantID:          merchantID,
		OriginalReferenceID: "ORDER-005",
		Amount:              &over,
		Signature:           "sig",
	}

	idempKey := domain.BuildRefundIdempotencyKey(merchantID, "ORDER-005")
	d.idempCache.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.idempRepo.EXPECT().Get(ctx, idempKey).Return(nil, nil)
	d.txRepo.EXPECT().GetByReference(ctx, merchantID, "ORDER-005").Return(&domain.Transaction{
		ID: origTxID, Amount: 50000, TransactionType: domain.TransactionTypePayment, Status: domain.TransactionStatusSuccess,
	}, nil)
	d.txRepo.EXPECT().CheckRefundExists(ctx, origTxID).Return(false, nil)

	result, err := d.svc.ProcessRefund(ctx, req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_007")
}

// ==================== ProcessTopup Tests ====================

func TestPaymentService_ProcessTopup_Success(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	walletID := uuid.New()
	tx := &mockTx{}

	req := ports.TopupRequest{
		MerchantID: merchantID,
		Amount:     500000,
		Currency:   "VND",
	}

	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	d.walletRepo.EXPECT().GetByMerchantIDForUpdate(ctx, tx, merchantID, "VND").Return(&domain.Wallet{
		ID: walletID, MerchantID: merchantID, EncryptedBalance: "enc_100000",
	}, nil)
	d.encSvc.EXPECT().Decrypt("enc_100000").Return("100000", nil)
	d.encSvc.EXPECT().Encrypt("600000").Return("enc_600000", nil) // 100000 + 500000
	d.encSvc.EXPECT().Encrypt("500000").Return("enc_amount_500000", nil)
	d.walletRepo.EXPECT().UpdateBalance(ctx, tx, walletID, "enc_600000").Return(nil)
	d.txRepo.EXPECT().Create(ctx, tx, gomock.Any()).Return(nil)

	result, err := d.svc.ProcessTopup(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain.TransactionTypeTopup, result.TransactionType)
	assert.Equal(t, domain.TransactionStatusSuccess, result.Status)
	assert.Equal(t, int64(500000), result.Amount)
}

func TestPaymentService_ProcessTopup_InvalidAmount(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	req := ports.TopupRequest{
		MerchantID: uuid.New(),
		Amount:     -100,
		Currency:   "VND",
	}

	result, err := d.svc.ProcessTopup(context.Background(), req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_002")
}

func TestPaymentService_ProcessTopup_WalletNotFound(t *testing.T) {
	d := setupPaymentService(t)
	defer d.ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	tx := &mockTx{}

	req := ports.TopupRequest{
		MerchantID: merchantID,
		Amount:     100000,
		Currency:   "USD",
	}

	d.transactor.EXPECT().Begin(ctx).Return(tx, nil)
	d.walletRepo.EXPECT().GetByMerchantIDForUpdate(ctx, tx, merchantID, "USD").Return(nil, nil)

	result, err := d.svc.ProcessTopup(ctx, req)
	assert.Nil(t, result)
	assertAppError(t, err, "PAY_004")
}

// ==================== Helper ====================

func assertAppError(t *testing.T, err error, expectedCode string) {
	t.Helper()
	var appErr *apperror.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, expectedCode, appErr.Code)
}
