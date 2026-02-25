package service

import (
"context"
"fmt"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"
"secure-payment-gateway/pkg/apperror"

"github.com/google/uuid"
)

// reportingService implements ports.ReportingService.
type reportingService struct {
txRepo     ports.TransactionRepository
walletRepo ports.WalletRepository
encSvc     ports.EncryptionService
}

// NewReportingService creates a new reporting service.
func NewReportingService(
txRepo ports.TransactionRepository,
walletRepo ports.WalletRepository,
encSvc ports.EncryptionService,
) ports.ReportingService {
return &reportingService{
txRepo:     txRepo,
walletRepo: walletRepo,
encSvc:     encSvc,
}
}

// GetDashboardStats returns aggregated transaction stats for the merchant.
func (s *reportingService) GetDashboardStats(ctx context.Context, merchantID uuid.UUID, period string) (*ports.TransactionStats, error) {
var periodStart *int64

switch period {
case "day":
t := time.Now().AddDate(0, 0, -1).Unix()
periodStart = &t
case "week":
t := time.Now().AddDate(0, 0, -7).Unix()
periodStart = &t
case "month":
t := time.Now().AddDate(0, -1, 0).Unix()
periodStart = &t
case "all", "":
// No time filter
default:
return nil, apperror.Validation("invalid period: must be day, week, month, or all")
}

stats, err := s.txRepo.GetStats(ctx, merchantID, periodStart)
if err != nil {
return nil, apperror.InternalError(err)
}

return stats, nil
}

// ListTransactions returns a paginated list of transactions.
func (s *reportingService) ListTransactions(ctx context.Context, params ports.TransactionListParams) ([]domain.Transaction, int64, error) {
txns, total, err := s.txRepo.List(ctx, params)
if err != nil {
return nil, 0, apperror.InternalError(err)
}
return txns, total, nil
}

// GetWalletBalance decrypts and returns the current balance for the merchant VND wallet.
func (s *reportingService) GetWalletBalance(ctx context.Context, merchantID uuid.UUID) (int64, string, error) {
wallet, err := s.walletRepo.GetByMerchantID(ctx, merchantID, "VND")
if err != nil {
return 0, "", apperror.InternalError(err)
}
if wallet == nil {
return 0, "", apperror.ErrNotFound("wallet")
}

balanceStr, err := s.encSvc.Decrypt(wallet.EncryptedBalance)
if err != nil {
return 0, "", apperror.InternalError(err)
}

var balance int64
_, err = fmt.Sscanf(balanceStr, "%d", &balance)
if err != nil {
return 0, "", apperror.InternalError(err)
}

return balance, wallet.Currency, nil
}
