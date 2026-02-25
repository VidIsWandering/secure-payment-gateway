package ports

import (
	"context"

	"secure-payment-gateway/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MerchantRepository defines persistence operations for merchants.
type MerchantRepository interface {
	Create(ctx context.Context, merchant *domain.Merchant) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Merchant, error)
	GetByAccessKey(ctx context.Context, accessKey string) (*domain.Merchant, error)
	GetByUsername(ctx context.Context, username string) (*domain.Merchant, error)
}

// WalletRepository defines persistence operations for wallets.
// Methods accepting pgx.Tx are used inside transaction blocks for pessimistic locking.
type WalletRepository interface {
	Create(ctx context.Context, wallet *domain.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)
	GetByMerchantID(ctx context.Context, merchantID uuid.UUID, currency string) (*domain.Wallet, error)
	GetByMerchantIDForUpdate(ctx context.Context, tx pgx.Tx, merchantID uuid.UUID, currency string) (*domain.Wallet, error)
	GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Wallet, error)
	UpdateBalance(ctx context.Context, tx pgx.Tx, walletID uuid.UUID, encryptedBalance string) error
}

// TransactionRepository defines persistence operations for transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx pgx.Tx, transaction *domain.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)
	GetByReference(ctx context.Context, merchantID uuid.UUID, referenceID string) (*domain.Transaction, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.TransactionStatus) error
	CheckRefundExists(ctx context.Context, originalTxID uuid.UUID) (bool, error)
	// Reporting queries
	List(ctx context.Context, params TransactionListParams) ([]domain.Transaction, int64, error)
	GetStats(ctx context.Context, merchantID uuid.UUID, periodStart *int64) (*TransactionStats, error)
}

// TransactionListParams holds filter + pagination for listing transactions.
type TransactionListParams struct {
	MerchantID uuid.UUID
	Status     *domain.TransactionStatus
	Type       *domain.TransactionType
	From       *int64 // Unix timestamp
	To         *int64 // Unix timestamp
	Page       int
	PageSize   int
}

// TransactionStats holds aggregated statistics for dashboard.
type TransactionStats struct {
	TotalTransactions int64
	Successful        int64
	Failed            int64
	Reversed          int64
	TotalRevenue      int64 // Sum of successful payment amounts
	TotalRefunded     int64 // Sum of successful refund amounts
	TotalTopup        int64 // Sum of successful topup amounts
}

// IdempotencyRepository defines persistence for idempotency logs (DB backup).
type IdempotencyRepository interface {
	Create(ctx context.Context, tx pgx.Tx, log *domain.IdempotencyLog) error
	Get(ctx context.Context, key string) (*domain.IdempotencyLog, error)
}

// DBTransactor provides database transaction management.
type DBTransactor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
