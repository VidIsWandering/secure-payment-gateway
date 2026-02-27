package domain

import (
"time"

"github.com/google/uuid"
)

// AuditAction represents the type of audited action.
type AuditAction string

const (
AuditActionPayment       AuditAction = "PAYMENT"
AuditActionRefund        AuditAction = "REFUND"
AuditActionTopup         AuditAction = "TOPUP"
AuditActionRegister      AuditAction = "REGISTER"
AuditActionLogin         AuditAction = "LOGIN"
AuditActionRotateKeys    AuditAction = "ROTATE_KEYS"
AuditActionUpdateWebhook AuditAction = "UPDATE_WEBHOOK"
)

// AuditLog records a single audited action in the system.
type AuditLog struct {
ID           uuid.UUID   `json:"id"`
MerchantID   *uuid.UUID  `json:"merchant_id,omitempty"`
Action       AuditAction `json:"action"`
ResourceType string      `json:"resource_type"`
ResourceID   string      `json:"resource_id,omitempty"`
Details      string      `json:"details,omitempty"` // JSON string
IPAddress    string      `json:"ip_address"`
CreatedAt    time.Time   `json:"created_at"`
}
