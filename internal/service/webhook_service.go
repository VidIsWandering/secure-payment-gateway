package service

import (
"bytes"
"context"
"encoding/json"
"fmt"
"net/http"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"

"github.com/rs/zerolog"
)

// webhookRetryIntervals defines the retry intervals per WEBHOOK_SPEC.md.
var webhookRetryIntervals = []time.Duration{
15 * time.Second,
60 * time.Second,
2 * time.Minute,
5 * time.Minute,
10 * time.Minute,
}

// WebhookEvent types
const (
EventPaymentUpdate = "PAYMENT_UPDATE"
EventRefundUpdate  = "REFUND_UPDATE"
EventTopupUpdate   = "TOPUP_UPDATE"
)

// WebhookPayload is the JSON structure sent to merchant webhook_url.
type WebhookPayload struct {
EventType string              `json:"event_type"`
Data      WebhookPayloadData  `json:"data"`
Signature string              `json:"signature"`
}

// WebhookPayloadData holds the transaction details in the webhook.
type WebhookPayloadData struct {
MerchantOrderID      string `json:"merchant_order_id"`
GatewayTransactionID string `json:"gateway_transaction_id"`
Status               string `json:"status"`
Amount               int64  `json:"amount"`
Currency             string `json:"currency"`
Reason               string `json:"reason"`
Timestamp            int64  `json:"timestamp"`
}

// webhookService implements ports.WebhookService.
type webhookService struct {
merchantRepo ports.MerchantRepository
walletRepo   ports.WalletRepository
encSvc       ports.EncryptionService
sigSvc       ports.SignatureService
httpClient   HTTPClient
log          zerolog.Logger
}

// HTTPClient interface for testability.
type HTTPClient interface {
Do(req *http.Request) (*http.Response, error)
}

// NewWebhookService creates a new webhook service.
func NewWebhookService(
merchantRepo ports.MerchantRepository,
walletRepo ports.WalletRepository,
encSvc ports.EncryptionService,
sigSvc ports.SignatureService,
httpClient HTTPClient,
log zerolog.Logger,
) ports.WebhookService {
return &webhookService{
merchantRepo: merchantRepo,
walletRepo:   walletRepo,
encSvc:       encSvc,
sigSvc:       sigSvc,
httpClient:   httpClient,
log:          log,
}
}

// EnqueueWebhook sends a webhook to the merchant asynchronously with retries.
func (s *webhookService) EnqueueWebhook(ctx context.Context, transaction *domain.Transaction) error {
// Lookup merchant to get webhook_url and secret_key
merchant, err := s.merchantRepo.GetByID(ctx, transaction.MerchantID)
if err != nil {
s.log.Error().Err(err).Str("merchant_id", transaction.MerchantID.String()).Msg("webhook: failed to fetch merchant")
return err
}
if merchant == nil || merchant.WebhookURL == nil || *merchant.WebhookURL == "" {
s.log.Debug().Str("merchant_id", transaction.MerchantID.String()).Msg("webhook: no webhook URL configured, skipping")
return nil
}

// Determine event type
eventType := EventPaymentUpdate
switch transaction.TransactionType {
case domain.TransactionTypeRefund:
eventType = EventRefundUpdate
case domain.TransactionTypeTopup:
eventType = EventTopupUpdate
}

// Determine currency from wallet
currency := "VND"
wallet, err := s.walletRepo.GetByID(ctx, transaction.WalletID)
if err == nil && wallet != nil {
currency = wallet.Currency
}

// Build reason
reason := fmt.Sprintf("Transaction %s", transaction.Status)

// Build payload data
data := WebhookPayloadData{
MerchantOrderID:      transaction.ReferenceID,
GatewayTransactionID: transaction.ID.String(),
Status:               string(transaction.Status),
Amount:               transaction.Amount,
Currency:             currency,
Reason:               reason,
Timestamp:            time.Now().Unix(),
}

// Sign the payload data with merchant secret
secretKey, err := s.encSvc.Decrypt(merchant.SecretKeyEnc)
if err != nil {
s.log.Error().Err(err).Msg("webhook: failed to decrypt merchant secret key")
return err
}

dataBytes, _ := json.Marshal(data)
signature := s.sigSvc.Sign(secretKey, string(dataBytes))

payload := WebhookPayload{
EventType: eventType,
Data:      data,
Signature: signature,
}

// Fire async with retries
go s.deliverWithRetries(*merchant.WebhookURL, payload, transaction.ID.String())

return nil
}

// deliverWithRetries attempts to deliver the webhook with exponential backoff.
func (s *webhookService) deliverWithRetries(url string, payload WebhookPayload, txID string) {
payloadBytes, err := json.Marshal(payload)
if err != nil {
s.log.Error().Err(err).Str("tx_id", txID).Msg("webhook: failed to marshal payload")
return
}

for attempt := 0; attempt <= len(webhookRetryIntervals); attempt++ {
if attempt > 0 {
time.Sleep(webhookRetryIntervals[attempt-1])
}

req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
if err != nil {
s.log.Error().Err(err).Str("tx_id", txID).Int("attempt", attempt+1).Msg("webhook: failed to create request")
continue
}
req.Header.Set("Content-Type", "application/json")

resp, err := s.httpClient.Do(req)
if err != nil {
s.log.Warn().Err(err).Str("tx_id", txID).Int("attempt", attempt+1).Msg("webhook: delivery failed")
continue
}
resp.Body.Close()

if resp.StatusCode >= 200 && resp.StatusCode < 300 {
s.log.Info().Str("tx_id", txID).Int("attempt", attempt+1).Int("status", resp.StatusCode).Msg("webhook: delivered successfully")
return
}

s.log.Warn().Str("tx_id", txID).Int("attempt", attempt+1).Int("status", resp.StatusCode).Msg("webhook: non-2xx response, retrying")
}

s.log.Error().Str("tx_id", txID).Msg("webhook: all retry attempts exhausted")
}
