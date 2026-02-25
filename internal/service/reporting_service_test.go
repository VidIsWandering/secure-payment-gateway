package service

import (
"context"
"errors"
"testing"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"
"secure-payment-gateway/internal/core/ports/mocks"
"secure-payment-gateway/pkg/apperror"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"go.uber.org/mock/gomock"
)

func TestReportingService_GetDashboardStats_All(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
expected := &ports.TransactionStats{
TotalTransactions: 100,
Successful:        80,
Failed:            15,
Reversed:          5,
TotalRevenue:      5000000,
TotalRefunded:     200000,
TotalTopup:        1000000,
}

mockTxRepo.EXPECT().GetStats(gomock.Any(), merchantID, (*int64)(nil)).Return(expected, nil)

result, err := svc.GetDashboardStats(context.Background(), merchantID, "all")
require.NoError(t, err)
assert.Equal(t, expected, result)
}

func TestReportingService_GetDashboardStats_WithPeriod(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
expected := &ports.TransactionStats{TotalTransactions: 10}

// For "day" period, periodStart should be non-nil
mockTxRepo.EXPECT().GetStats(gomock.Any(), merchantID, gomock.Not(gomock.Nil())).Return(expected, nil)

result, err := svc.GetDashboardStats(context.Background(), merchantID, "day")
require.NoError(t, err)
assert.Equal(t, int64(10), result.TotalTransactions)
}

func TestReportingService_GetDashboardStats_InvalidPeriod(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

_, err := svc.GetDashboardStats(context.Background(), uuid.New(), "invalid")
require.Error(t, err)

var appErr *apperror.AppError
assert.ErrorAs(t, err, &appErr)
assert.Equal(t, "PAY_002", appErr.Code)
}

func TestReportingService_ListTransactions_Success(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
params := ports.TransactionListParams{
MerchantID: merchantID,
Page:       1,
PageSize:   20,
}

txns := []domain.Transaction{
{ID: uuid.New(), ReferenceID: "ref-1"},
{ID: uuid.New(), ReferenceID: "ref-2"},
}
mockTxRepo.EXPECT().List(gomock.Any(), params).Return(txns, int64(2), nil)

result, total, err := svc.ListTransactions(context.Background(), params)
require.NoError(t, err)
assert.Len(t, result, 2)
assert.Equal(t, int64(2), total)
}

func TestReportingService_ListTransactions_Error(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

params := ports.TransactionListParams{MerchantID: uuid.New(), Page: 1, PageSize: 20}
mockTxRepo.EXPECT().List(gomock.Any(), params).Return(nil, int64(0), errors.New("db error"))

_, _, err := svc.ListTransactions(context.Background(), params)
require.Error(t, err)
}

func TestReportingService_GetWalletBalance_Success(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
mockWalletRepo.EXPECT().GetByMerchantID(gomock.Any(), merchantID, "VND").Return(&domain.Wallet{
ID:               uuid.New(),
MerchantID:       merchantID,
Currency:         "VND",
EncryptedBalance: "encrypted-100000",
}, nil)
mockEncSvc.EXPECT().Decrypt("encrypted-100000").Return("100000", nil)

balance, currency, err := svc.GetWalletBalance(context.Background(), merchantID)
require.NoError(t, err)
assert.Equal(t, int64(100000), balance)
assert.Equal(t, "VND", currency)
}

func TestReportingService_GetWalletBalance_WalletNotFound(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
mockWalletRepo.EXPECT().GetByMerchantID(gomock.Any(), merchantID, "VND").Return(nil, nil)

_, _, err := svc.GetWalletBalance(context.Background(), merchantID)
require.Error(t, err)

var appErr *apperror.AppError
assert.ErrorAs(t, err, &appErr)
assert.Equal(t, "PAY_004", appErr.Code)
}

func TestReportingService_GetWalletBalance_DecryptError(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockTxRepo := mocks.NewMockTransactionRepository(ctrl)
mockWalletRepo := mocks.NewMockWalletRepository(ctrl)
mockEncSvc := mocks.NewMockEncryptionService(ctrl)

svc := NewReportingService(mockTxRepo, mockWalletRepo, mockEncSvc)

merchantID := uuid.New()
mockWalletRepo.EXPECT().GetByMerchantID(gomock.Any(), merchantID, "VND").Return(&domain.Wallet{
EncryptedBalance: "bad",
}, nil)
mockEncSvc.EXPECT().Decrypt("bad").Return("", errors.New("decrypt fail"))

_, _, err := svc.GetWalletBalance(context.Background(), merchantID)
require.Error(t, err)
}
