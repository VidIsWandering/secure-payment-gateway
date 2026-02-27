package domain

import (
"time"

"github.com/google/uuid"
)

// WebhookStatus represents the delivery state of a webhook.
type WebhookStatus string

const (
WebhookStatusPending   WebhookStatus = "PENDING"
WebhookStatusDelivered WebhookStatus = "DELIVERED"
WebhookStatusFailed    WebhookStatus = "FAILED"
)

// WebhookDeliveryLog records each webhook delivery attempt.
type WebhookDeliveryLog struct {
ID            uuid.UUID     `json:"id"`
TransactionID uuid.UUID     `json:"transaction_id"`
MerchantID    uuid.UUID     `json:"merchant_id"`
WebhookURL    string        `json:"webhook_url"`
Payload       string        `json:"payload"`   // JSON string
HTTPStatus    *int          `json:"http_status"`
Attempt       int           `json:"attempt"`
Status        WebhookStatus `json:"status"`
NextRetryAt   *time.Time    `json:"next_retry_at"`
LastError     *string       `json:"last_error"`
CreatedAt     time.Time     `json:"created_at"`
UpdatedAt     time.Time     `json:"updated_at"`
}
