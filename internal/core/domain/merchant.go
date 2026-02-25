package domain

import (
	"time"

	"github.com/google/uuid"
)

// MerchantStatus represents the state of a merchant account.
type MerchantStatus string

const (
	MerchantStatusActive      MerchantStatus = "ACTIVE"
	MerchantStatusSuspended   MerchantStatus = "SUSPENDED"
	MerchantStatusDeactivated MerchantStatus = "DEACTIVATED"
)

// Merchant represents a registered merchant in the system.
type Merchant struct {
	ID           uuid.UUID      `json:"id"`
	Username     string         `json:"username"`
	PasswordHash string         `json:"-"` // Never expose
	MerchantName string         `json:"merchant_name"`
	AccessKey    string         `json:"access_key"`
	SecretKeyEnc string         `json:"-"` // Encrypted, never expose
	WebhookURL   *string        `json:"webhook_url,omitempty"`
	Status       MerchantStatus `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// IsActive returns true if the merchant account is active.
func (m *Merchant) IsActive() bool {
	return m.Status == MerchantStatusActive
}
