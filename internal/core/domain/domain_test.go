package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMerchant_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status MerchantStatus
		want   bool
	}{
		{"active", MerchantStatusActive, true},
		{"suspended", MerchantStatusSuspended, false},
		{"deactivated", MerchantStatusDeactivated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Merchant{Status: tt.status}
			assert.Equal(t, tt.want, m.IsActive())
		})
	}
}

func TestTransaction_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status TransactionStatus
		want   bool
	}{
		{"pending", TransactionStatusPending, false},
		{"success", TransactionStatusSuccess, true},
		{"failed", TransactionStatusFailed, true},
		{"reversed", TransactionStatusReversed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{Status: tt.status}
			assert.Equal(t, tt.want, tx.IsTerminal())
		})
	}
}

func TestTransaction_IsRefundable(t *testing.T) {
	tests := []struct {
		name   string
		txType TransactionType
		status TransactionStatus
		want   bool
	}{
		{"successful payment", TransactionTypePayment, TransactionStatusSuccess, true},
		{"failed payment", TransactionTypePayment, TransactionStatusFailed, false},
		{"reversed payment", TransactionTypePayment, TransactionStatusReversed, false},
		{"successful refund", TransactionTypeRefund, TransactionStatusSuccess, false},
		{"successful topup", TransactionTypeTopup, TransactionStatusSuccess, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				TransactionType: tt.txType,
				Status:          tt.status,
			}
			assert.Equal(t, tt.want, tx.IsRefundable())
		})
	}
}

func TestBuildIdempotencyKey(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	key := BuildIdempotencyKey(id, "ORD-001")
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000:ORD-001", key)
}

func TestBuildRefundIdempotencyKey(t *testing.T) {
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	key := BuildRefundIdempotencyKey(id, "ORD-001")
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000:refund:ORD-001", key)
}

func TestMerchantStatus_Constants(t *testing.T) {
	assert.Equal(t, MerchantStatus("ACTIVE"), MerchantStatusActive)
	assert.Equal(t, MerchantStatus("SUSPENDED"), MerchantStatusSuspended)
	assert.Equal(t, MerchantStatus("DEACTIVATED"), MerchantStatusDeactivated)
}

func TestTransactionType_Constants(t *testing.T) {
	assert.Equal(t, TransactionType("PAYMENT"), TransactionTypePayment)
	assert.Equal(t, TransactionType("REFUND"), TransactionTypeRefund)
	assert.Equal(t, TransactionType("TOPUP"), TransactionTypeTopup)
}

func TestTransactionStatus_Constants(t *testing.T) {
	assert.Equal(t, TransactionStatus("PENDING"), TransactionStatusPending)
	assert.Equal(t, TransactionStatus("SUCCESS"), TransactionStatusSuccess)
	assert.Equal(t, TransactionStatus("FAILED"), TransactionStatusFailed)
	assert.Equal(t, TransactionStatus("REVERSED"), TransactionStatusReversed)
}
