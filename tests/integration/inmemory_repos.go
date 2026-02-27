package integration

import (
	"context"
	"fmt"
	"sync"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- In-Memory Merchant Repo ---

type inMemoryMerchantRepo struct {
	mu        sync.RWMutex
	merchants map[uuid.UUID]*domain.Merchant
}

func newInMemoryMerchantRepo() *inMemoryMerchantRepo {
	return &inMemoryMerchantRepo{merchants: make(map[uuid.UUID]*domain.Merchant)}
}

func (r *inMemoryMerchantRepo) Create(ctx context.Context, m *domain.Merchant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.merchants {
		if existing.Username == m.Username {
			return fmt.Errorf("username already exists")
		}
	}
	r.merchants[m.ID] = m
	return nil
}

func (r *inMemoryMerchantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.merchants[id]
	if !ok {
		return nil, nil
	}
	return m, nil
}

func (r *inMemoryMerchantRepo) GetByAccessKey(ctx context.Context, accessKey string) (*domain.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.merchants {
		if m.AccessKey == accessKey {
			return m, nil
		}
	}
	return nil, nil
}

func (r *inMemoryMerchantRepo) GetByUsername(ctx context.Context, username string) (*domain.Merchant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.merchants {
		if m.Username == username {
			return m, nil
		}
	}
	return nil, nil
}

func (r *inMemoryMerchantRepo) Update(ctx context.Context, m *domain.Merchant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.merchants[m.ID]; !ok {
		return fmt.Errorf("merchant not found")
	}
	r.merchants[m.ID] = m
	return nil
}

// --- In-Memory Wallet Repo ---

type inMemoryWalletRepo struct {
	mu      sync.RWMutex
	wallets map[uuid.UUID]*domain.Wallet
}

func newInMemoryWalletRepo() *inMemoryWalletRepo {
	return &inMemoryWalletRepo{wallets: make(map[uuid.UUID]*domain.Wallet)}
}

func (r *inMemoryWalletRepo) Create(ctx context.Context, w *domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wallets[w.ID] = w
	return nil
}

func (r *inMemoryWalletRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.wallets[id]
	if !ok {
		return nil, nil
	}
	return w, nil
}

func (r *inMemoryWalletRepo) GetByMerchantID(ctx context.Context, merchantID uuid.UUID, currency string) (*domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, w := range r.wallets {
		if w.MerchantID == merchantID && w.Currency == currency {
			return w, nil
		}
	}
	return nil, nil
}

func (r *inMemoryWalletRepo) GetByMerchantIDForUpdate(ctx context.Context, tx pgx.Tx, merchantID uuid.UUID, currency string) (*domain.Wallet, error) {
	return r.GetByMerchantID(ctx, merchantID, currency)
}

func (r *inMemoryWalletRepo) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Wallet, error) {
	return r.GetByID(ctx, id)
}

func (r *inMemoryWalletRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, walletID uuid.UUID, encryptedBalance string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.wallets[walletID]
	if !ok {
		return fmt.Errorf("wallet not found")
	}
	w.EncryptedBalance = encryptedBalance
	return nil
}

// --- In-Memory Transaction Repo ---

type inMemoryTransactionRepo struct {
	mu           sync.RWMutex
	transactions map[uuid.UUID]*domain.Transaction
}

func newInMemoryTransactionRepo() *inMemoryTransactionRepo {
	return &inMemoryTransactionRepo{transactions: make(map[uuid.UUID]*domain.Transaction)}
}

func (r *inMemoryTransactionRepo) Create(ctx context.Context, tx pgx.Tx, t *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transactions[t.ID] = t
	return nil
}

func (r *inMemoryTransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.transactions[id]
	if !ok {
		return nil, nil
	}
	return t, nil
}

func (r *inMemoryTransactionRepo) GetByReference(ctx context.Context, merchantID uuid.UUID, referenceID string) (*domain.Transaction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.transactions {
		if t.MerchantID == merchantID && t.ReferenceID == referenceID {
			return t, nil
		}
	}
	return nil, nil
}

func (r *inMemoryTransactionRepo) UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.TransactionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.transactions[id]
	if !ok {
		return fmt.Errorf("transaction not found")
	}
	t.Status = status
	return nil
}

func (r *inMemoryTransactionRepo) CheckRefundExists(ctx context.Context, originalTxID uuid.UUID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.transactions {
		if t.OriginalTransactionID != nil && *t.OriginalTransactionID == originalTxID && t.TransactionType == domain.TransactionTypeRefund {
			return true, nil
		}
	}
	return false, nil
}

func (r *inMemoryTransactionRepo) List(ctx context.Context, params ports.TransactionListParams) ([]domain.Transaction, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []domain.Transaction
	for _, t := range r.transactions {
		if t.MerchantID != params.MerchantID {
			continue
		}
		if params.Status != nil && t.Status != *params.Status {
			continue
		}
		if params.Type != nil && t.TransactionType != *params.Type {
			continue
		}
		result = append(result, *t)
	}
	total := int64(len(result))

	// Simple pagination
	start := (params.Page - 1) * params.PageSize
	if start >= len(result) {
		return []domain.Transaction{}, total, nil
	}
	end := start + params.PageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], total, nil
}

func (r *inMemoryTransactionRepo) GetStats(ctx context.Context, merchantID uuid.UUID, periodStart *int64) (*ports.TransactionStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stats := &ports.TransactionStats{}
	for _, t := range r.transactions {
		if t.MerchantID != merchantID {
			continue
		}
		if periodStart != nil && t.CreatedAt.Unix() < *periodStart {
			continue
		}
		stats.TotalTransactions++
		switch t.Status {
		case domain.TransactionStatusSuccess:
			stats.Successful++
		case domain.TransactionStatusFailed:
			stats.Failed++
		case domain.TransactionStatusReversed:
			stats.Reversed++
		}
		if t.Status == domain.TransactionStatusSuccess {
			switch t.TransactionType {
			case domain.TransactionTypePayment:
				stats.TotalRevenue += t.Amount
			case domain.TransactionTypeRefund:
				stats.TotalRefunded += t.Amount
			case domain.TransactionTypeTopup:
				stats.TotalTopup += t.Amount
			}
		}
	}
	return stats, nil
}

// --- In-Memory Idempotency Repo ---

type inMemoryIdempotencyRepo struct {
	mu   sync.RWMutex
	logs map[string]*domain.IdempotencyLog
}

func newInMemoryIdempotencyRepo() *inMemoryIdempotencyRepo {
	return &inMemoryIdempotencyRepo{logs: make(map[string]*domain.IdempotencyLog)}
}

func (r *inMemoryIdempotencyRepo) Create(ctx context.Context, tx pgx.Tx, log *domain.IdempotencyLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs[log.Key] = log
	return nil
}

func (r *inMemoryIdempotencyRepo) Get(ctx context.Context, key string) (*domain.IdempotencyLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.logs[key]
	if !ok {
		return nil, nil
	}
	return l, nil
}

// --- In-Memory Transactor (no-op tx) ---

type inMemoryTransactor struct{}

func newInMemoryTransactor() *inMemoryTransactor {
	return &inMemoryTransactor{}
}

func (t *inMemoryTransactor) Begin(ctx context.Context) (pgx.Tx, error) {
	return &noopTx{}, nil
}

// noopTx is a no-op pgx.Tx implementation for in-memory testing.
type noopTx struct{}

func (t *noopTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *noopTx) Commit(ctx context.Context) error          { return nil }
func (t *noopTx) Rollback(ctx context.Context) error        { return nil }
func (t *noopTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *noopTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *noopTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *noopTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *noopTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (t *noopTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *noopTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
func (t *noopTx) Conn() *pgx.Conn { return nil }
