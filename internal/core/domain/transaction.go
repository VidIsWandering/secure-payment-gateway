package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType represents the kind of money movement.
type TransactionType string

const (
	TransactionTypePayment TransactionType = "PAYMENT"
	TransactionTypeRefund  TransactionType = "REFUND"
	TransactionTypeTopup   TransactionType = "TOPUP"
)

// TransactionStatus represents the lifecycle state of a transaction.
type TransactionStatus string

const (
	TransactionStatusPending  TransactionStatus = "PENDING"
	TransactionStatusSuccess  TransactionStatus = "SUCCESS"
	TransactionStatusFailed   TransactionStatus = "FAILED"
	TransactionStatusReversed TransactionStatus = "REVERSED"
)

// Transaction represents an immutable ledger entry for money movement.
type Transaction struct {
	ID                    uuid.UUID         `json:"id"`
	ReferenceID           string            `json:"reference_id"`
	MerchantID            uuid.UUID         `json:"merchant_id"`
	WalletID              uuid.UUID         `json:"wallet_id"`
	Amount                int64             `json:"amount"` // In smallest unit (e.g., VND)
	AmountEncrypted       string            `json:"-"`      // AES-256 encrypted record
	TransactionType       TransactionType   `json:"transaction_type"`
	Status                TransactionStatus `json:"status"`
	Signature             string            `json:"-"` // Request signature
	ClientIP              string            `json:"client_ip,omitempty"`
	ExtraData             *string           `json:"extra_data,omitempty"`
	OriginalTransactionID *uuid.UUID        `json:"original_transaction_id,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	ProcessedAt           *time.Time        `json:"processed_at,omitempty"`
}

// IsTerminal returns true if the transaction is in a final state.
func (t *Transaction) IsTerminal() bool {
	return t.Status == TransactionStatusSuccess ||
		t.Status == TransactionStatusFailed ||
		t.Status == TransactionStatusReversed
}

// IsRefundable returns true if this transaction can be refunded.
func (t *Transaction) IsRefundable() bool {
	return t.TransactionType == TransactionTypePayment &&
		t.Status == TransactionStatusSuccess
}
