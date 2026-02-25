package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/pkg/apperror"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

const idempotencyTTL = 24 * time.Hour

// PaymentServiceImpl implements ports.PaymentService.
type PaymentServiceImpl struct {
	txRepo     ports.TransactionRepository
	walletRepo ports.WalletRepository
	idempRepo  ports.IdempotencyRepository
	idempCache ports.IdempotencyCache
	encSvc     ports.EncryptionService
	transactor ports.DBTransactor
	log        zerolog.Logger
}

// NewPaymentService creates a new PaymentServiceImpl.
func NewPaymentService(
	txRepo ports.TransactionRepository,
	walletRepo ports.WalletRepository,
	idempRepo ports.IdempotencyRepository,
	idempCache ports.IdempotencyCache,
	encSvc ports.EncryptionService,
	transactor ports.DBTransactor,
	log zerolog.Logger,
) *PaymentServiceImpl {
	return &PaymentServiceImpl{
		txRepo:     txRepo,
		walletRepo: walletRepo,
		idempRepo:  idempRepo,
		idempCache: idempCache,
		encSvc:     encSvc,
		transactor: transactor,
		log:        log,
	}
}

// ProcessPayment implements the Payment algorithm with pessimistic locking.
func (s *PaymentServiceImpl) ProcessPayment(ctx context.Context, req ports.PaymentRequest) (*domain.Transaction, error) {
	if req.Amount <= 0 {
		return nil, apperror.ErrInvalidAmount()
	}

	idempKey := domain.BuildIdempotencyKey(req.MerchantID, req.ReferenceID)

	// Layer 1: Redis idempotency check
	cached, err := s.idempCache.Get(ctx, idempKey)
	if err != nil {
		s.log.Warn().Err(err).Str("key", idempKey).Msg("redis idempotency check failed, falling through to DB")
	}
	if cached != nil {
		return s.unmarshalCachedTransaction(cached)
	}

	// Layer 2: DB idempotency check
	idempLog, err := s.idempRepo.Get(ctx, idempKey)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("db idempotency check: %w", err))
	}
	if idempLog != nil {
		return s.unmarshalCachedTransaction(idempLog.ResponseJSON)
	}

	// Begin database transaction
	dbTx, err := s.transactor.Begin(ctx)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("begin tx: %w", err))
	}
	defer dbTx.Rollback(ctx) //nolint:errcheck

	// Lock & get wallet
	wallet, err := s.walletRepo.GetByMerchantIDForUpdate(ctx, dbTx, req.MerchantID, req.Currency)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("lock wallet: %w", err))
	}
	if wallet == nil {
		return nil, apperror.ErrNotFound("wallet")
	}

	// Decrypt balance
	balanceStr, err := s.encSvc.Decrypt(wallet.EncryptedBalance)
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("decrypt balance: %w", err))
	}
	currentBalance, err := strconv.ParseInt(balanceStr, 10, 64)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("parse balance: %w", err))
	}

	// Business rule: sufficient funds
	if currentBalance < req.Amount {
		return nil, apperror.ErrInsufficientFunds()
	}

	// Calculate new balance
	newBalance := currentBalance - req.Amount
	newBalanceEnc, err := s.encSvc.Encrypt(strconv.FormatInt(newBalance, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt new balance: %w", err))
	}

	// Encrypt amount for audit
	amountEncrypted, err := s.encSvc.Encrypt(strconv.FormatInt(req.Amount, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt amount: %w", err))
	}

	now := time.Now().UTC()
	txn := &domain.Transaction{
		ID:              uuid.New(),
		ReferenceID:     req.ReferenceID,
		MerchantID:      req.MerchantID,
		WalletID:        wallet.ID,
		Amount:          req.Amount,
		AmountEncrypted: amountEncrypted,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
		Signature:       req.Signature,
		ClientIP:        req.ClientIP,
		ExtraData:       req.ExtraData,
		CreatedAt:       now,
		ProcessedAt:     &now,
	}

	// Persist: update wallet balance
	if err := s.walletRepo.UpdateBalance(ctx, dbTx, wallet.ID, newBalanceEnc); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("update balance: %w", err))
	}

	// Persist: create transaction
	if err := s.txRepo.Create(ctx, dbTx, txn); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("create transaction: %w", err))
	}

	// Persist: idempotency log
	respJSON, err := json.Marshal(txn)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("marshal response: %w", err))
	}

	idempLogEntry := &domain.IdempotencyLog{
		Key:           idempKey,
		TransactionID: txn.ID,
		ResponseJSON:  respJSON,
		CreatedAt:     now,
	}
	if err := s.idempRepo.Create(ctx, dbTx, idempLogEntry); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("save idempotency log: %w", err))
	}

	// Commit
	if err := dbTx.Commit(ctx); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("commit tx: %w", err))
	}

	// Post-process: cache in Redis (best-effort)
	if err := s.idempCache.Set(ctx, idempKey, respJSON, idempotencyTTL); err != nil {
		s.log.Warn().Err(err).Str("key", idempKey).Msg("failed to cache idempotency in redis")
	}

	s.log.Info().
		Str("tx_id", txn.ID.String()).
		Str("merchant_id", req.MerchantID.String()).
		Int64("amount", req.Amount).
		Msg("payment processed successfully")

	return txn, nil
}

// ProcessRefund implements the Refund algorithm.
func (s *PaymentServiceImpl) ProcessRefund(ctx context.Context, req ports.RefundRequest) (*domain.Transaction, error) {
	idempKey := domain.BuildRefundIdempotencyKey(req.MerchantID, req.OriginalReferenceID)

	// Layer 1: Redis idempotency check
	cached, err := s.idempCache.Get(ctx, idempKey)
	if err != nil {
		s.log.Warn().Err(err).Str("key", idempKey).Msg("redis idempotency check failed, falling through to DB")
	}
	if cached != nil {
		return s.unmarshalCachedTransaction(cached)
	}

	// Layer 2: DB idempotency check
	idempLog, err := s.idempRepo.Get(ctx, idempKey)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("db idempotency check: %w", err))
	}
	if idempLog != nil {
		return s.unmarshalCachedTransaction(idempLog.ResponseJSON)
	}

	// Find original transaction
	origTx, err := s.txRepo.GetByReference(ctx, req.MerchantID, req.OriginalReferenceID)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("find original tx: %w", err))
	}
	if origTx == nil {
		return nil, apperror.ErrNotFound("original transaction")
	}
	if !origTx.IsRefundable() {
		return nil, apperror.ErrInvalidRefund()
	}

	// Check no existing refund
	refundExists, err := s.txRepo.CheckRefundExists(ctx, origTx.ID)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("check refund exists: %w", err))
	}
	if refundExists {
		return nil, apperror.ErrDuplicateTransaction()
	}

	// Determine refund amount
	refundAmount := origTx.Amount
	if req.Amount != nil {
		if *req.Amount <= 0 {
			return nil, apperror.ErrInvalidAmount()
		}
		if *req.Amount > origTx.Amount {
			return nil, apperror.ErrRefundAmountExceedsOriginal()
		}
		refundAmount = *req.Amount
	}

	// Begin database transaction
	dbTx, err := s.transactor.Begin(ctx)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("begin tx: %w", err))
	}
	defer dbTx.Rollback(ctx) //nolint:errcheck

	// Lock & get wallet
	wallet, err := s.walletRepo.GetByIDForUpdate(ctx, dbTx, origTx.WalletID)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("lock wallet: %w", err))
	}
	if wallet == nil {
		return nil, apperror.ErrNotFound("wallet")
	}

	// Decrypt balance
	balanceStr, err := s.encSvc.Decrypt(wallet.EncryptedBalance)
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("decrypt balance: %w", err))
	}
	currentBalance, err := strconv.ParseInt(balanceStr, 10, 64)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("parse balance: %w", err))
	}

	// Calculate new balance (ADD back)
	newBalance := currentBalance + refundAmount
	newBalanceEnc, err := s.encSvc.Encrypt(strconv.FormatInt(newBalance, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt new balance: %w", err))
	}

	amountEncrypted, err := s.encSvc.Encrypt(strconv.FormatInt(refundAmount, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt amount: %w", err))
	}

	now := time.Now().UTC()
	refundRefID := "REFUND-" + req.OriginalReferenceID
	txn := &domain.Transaction{
		ID:                    uuid.New(),
		ReferenceID:           refundRefID,
		MerchantID:            req.MerchantID,
		WalletID:              wallet.ID,
		Amount:                refundAmount,
		AmountEncrypted:       amountEncrypted,
		TransactionType:       domain.TransactionTypeRefund,
		Status:                domain.TransactionStatusSuccess,
		Signature:             req.Signature,
		ClientIP:              req.ClientIP,
		ExtraData:             &req.Reason,
		OriginalTransactionID: &origTx.ID,
		CreatedAt:             now,
		ProcessedAt:           &now,
	}

	// Persist: update wallet balance
	if err := s.walletRepo.UpdateBalance(ctx, dbTx, wallet.ID, newBalanceEnc); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("update balance: %w", err))
	}

	// Persist: create refund transaction
	if err := s.txRepo.Create(ctx, dbTx, txn); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("create refund tx: %w", err))
	}

	// Persist: mark original transaction as REVERSED
	if err := s.txRepo.UpdateStatus(ctx, dbTx, origTx.ID, domain.TransactionStatusReversed); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("reverse original tx: %w", err))
	}

	// Persist: idempotency log
	respJSON, err := json.Marshal(txn)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("marshal response: %w", err))
	}

	idempLogEntry := &domain.IdempotencyLog{
		Key:           idempKey,
		TransactionID: txn.ID,
		ResponseJSON:  respJSON,
		CreatedAt:     now,
	}
	if err := s.idempRepo.Create(ctx, dbTx, idempLogEntry); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("save idempotency log: %w", err))
	}

	// Commit
	if err := dbTx.Commit(ctx); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("commit tx: %w", err))
	}

	// Post-process: cache in Redis (best-effort)
	if err := s.idempCache.Set(ctx, idempKey, respJSON, idempotencyTTL); err != nil {
		s.log.Warn().Err(err).Str("key", idempKey).Msg("failed to cache idempotency in redis")
	}

	s.log.Info().
		Str("tx_id", txn.ID.String()).
		Str("original_tx_id", origTx.ID.String()).
		Int64("refund_amount", refundAmount).
		Msg("refund processed successfully")

	return txn, nil
}

// ProcessTopup implements the Topup algorithm.
func (s *PaymentServiceImpl) ProcessTopup(ctx context.Context, req ports.TopupRequest) (*domain.Transaction, error) {
	if req.Amount <= 0 {
		return nil, apperror.ErrInvalidAmount()
	}

	// Begin database transaction
	dbTx, err := s.transactor.Begin(ctx)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("begin tx: %w", err))
	}
	defer dbTx.Rollback(ctx) //nolint:errcheck

	// Lock & get wallet
	wallet, err := s.walletRepo.GetByMerchantIDForUpdate(ctx, dbTx, req.MerchantID, req.Currency)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("lock wallet: %w", err))
	}
	if wallet == nil {
		return nil, apperror.ErrNotFound("wallet")
	}

	// Decrypt balance
	balanceStr, err := s.encSvc.Decrypt(wallet.EncryptedBalance)
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("decrypt balance: %w", err))
	}
	currentBalance, err := strconv.ParseInt(balanceStr, 10, 64)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("parse balance: %w", err))
	}

	// Calculate new balance (ADD funds)
	newBalance := currentBalance + req.Amount
	newBalanceEnc, err := s.encSvc.Encrypt(strconv.FormatInt(newBalance, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt new balance: %w", err))
	}

	amountEncrypted, err := s.encSvc.Encrypt(strconv.FormatInt(req.Amount, 10))
	if err != nil {
		return nil, apperror.ErrEncryptionFailure(fmt.Errorf("encrypt amount: %w", err))
	}

	now := time.Now().UTC()
	refID := fmt.Sprintf("TOPUP-%s-%d", req.MerchantID.String()[:8], now.UnixMilli())
	txn := &domain.Transaction{
		ID:              uuid.New(),
		ReferenceID:     refID,
		MerchantID:      req.MerchantID,
		WalletID:        wallet.ID,
		Amount:          req.Amount,
		AmountEncrypted: amountEncrypted,
		TransactionType: domain.TransactionTypeTopup,
		Status:          domain.TransactionStatusSuccess,
		Signature:       "SYSTEM_TOPUP",
		CreatedAt:       now,
		ProcessedAt:     &now,
	}

	// Persist: update wallet balance
	if err := s.walletRepo.UpdateBalance(ctx, dbTx, wallet.ID, newBalanceEnc); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("update balance: %w", err))
	}

	// Persist: create transaction
	if err := s.txRepo.Create(ctx, dbTx, txn); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("create transaction: %w", err))
	}

	// Commit
	if err := dbTx.Commit(ctx); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("commit tx: %w", err))
	}

	s.log.Info().
		Str("tx_id", txn.ID.String()).
		Str("merchant_id", req.MerchantID.String()).
		Int64("amount", req.Amount).
		Msg("topup processed successfully")

	return txn, nil
}

// unmarshalCachedTransaction deserializes a cached transaction.
func (s *PaymentServiceImpl) unmarshalCachedTransaction(data []byte) (*domain.Transaction, error) {
	txn := &domain.Transaction{}
	if err := json.Unmarshal(data, txn); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("unmarshal cached tx: %w", err))
	}
	return txn, nil
}
