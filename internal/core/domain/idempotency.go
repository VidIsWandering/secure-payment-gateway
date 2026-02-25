package domain

import (
	"time"

	"github.com/google/uuid"
)

// IdempotencyLog represents a cached transaction result to prevent double-processing.
type IdempotencyLog struct {
	Key           string    `json:"key"` // Format: "merchant_id:reference_id"
	TransactionID uuid.UUID `json:"transaction_id"`
	ResponseJSON  []byte    `json:"response_json"` // Cached response to return
	CreatedAt     time.Time `json:"created_at"`
}

// BuildIdempotencyKey constructs the standard key format.
func BuildIdempotencyKey(merchantID uuid.UUID, referenceID string) string {
	return merchantID.String() + ":" + referenceID
}

// BuildRefundIdempotencyKey constructs the key for refund idempotency.
func BuildRefundIdempotencyKey(merchantID uuid.UUID, originalReferenceID string) string {
	return merchantID.String() + ":refund:" + originalReferenceID
}
