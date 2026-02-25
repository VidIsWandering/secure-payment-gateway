package domain

import (
	"time"

	"github.com/google/uuid"
)

// Wallet represents a merchant's currency wallet with encrypted balance.
type Wallet struct {
	ID               uuid.UUID `json:"id"`
	MerchantID       uuid.UUID `json:"merchant_id"`
	Currency         string    `json:"currency"`
	EncryptedBalance string    `json:"-"` // AES-256 encrypted, never expose raw
	LastAuditHash    *string   `json:"-"` // Integrity check hash
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
