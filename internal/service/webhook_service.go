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

	"github.com/google/uuid"
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
	EventType string             `json:"event_type"`
	Data      WebhookPayloadData `json:"data"`
	Signature string             `json:"signature"`
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
	webhookRepo  ports.WebhookRepository // nil = persistence disabled
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
	webhookRepo ...ports.WebhookRepository,
) ports.WebhookService {
	var repo ports.WebhookRepository
	if len(webhookRepo) > 0 {
		repo = webhookRepo[0]
	}
	return &webhookService{
		merchantRepo: merchantRepo,
		walletRepo:   walletRepo,
		webhookRepo:  repo,
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
	go s.deliverWithRetries(*merchant.WebhookURL, payload, transaction.ID, transaction.MerchantID)

	return nil
}

// deliverWithRetries attempts to deliver the webhook with exponential backoff.
func (s *webhookService) deliverWithRetries(url string, payload WebhookPayload, txID uuid.UUID, merchantID uuid.UUID) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		s.log.Error().Err(err).Str("tx_id", txID.String()).Msg("webhook: failed to marshal payload")
		return
	}

	// Create initial log entry
	logID := uuid.New()
	now := time.Now()
	deliveryLog := &domain.WebhookDeliveryLog{
		ID:            logID,
		TransactionID: txID,
		MerchantID:    merchantID,
		WebhookURL:    url,
		Payload:       string(payloadBytes),
		Attempt:       0,
		Status:        domain.WebhookStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if s.webhookRepo != nil {
		if err := s.webhookRepo.Create(context.Background(), deliveryLog); err != nil {
			s.log.Warn().Err(err).Str("tx_id", txID.String()).Msg("webhook: failed to persist initial log")
		}
	}

	for attempt := 0; attempt <= len(webhookRetryIntervals); attempt++ {
		if attempt > 0 {
			time.Sleep(webhookRetryIntervals[attempt-1])
		}

		deliveryLog.Attempt = attempt + 1
		deliveryLog.UpdatedAt = time.Now()

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
		if err != nil {
			errMsg := err.Error()
			deliveryLog.LastError = &errMsg
			s.persistLog(deliveryLog)
			s.log.Error().Err(err).Str("tx_id", txID.String()).Int("attempt", attempt+1).Msg("webhook: failed to create request")
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			errMsg := err.Error()
			deliveryLog.LastError = &errMsg
			if attempt < len(webhookRetryIntervals) {
				nextRetry := time.Now().Add(webhookRetryIntervals[attempt])
				deliveryLog.NextRetryAt = &nextRetry
			}
			s.persistLog(deliveryLog)
			s.log.Warn().Err(err).Str("tx_id", txID.String()).Int("attempt", attempt+1).Msg("webhook: delivery failed")
			continue
		}
		resp.Body.Close()

		httpStatus := resp.StatusCode
		deliveryLog.HTTPStatus = &httpStatus

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			deliveryLog.Status = domain.WebhookStatusDelivered
			deliveryLog.LastError = nil
			deliveryLog.NextRetryAt = nil
			s.persistLog(deliveryLog)
			s.log.Info().Str("tx_id", txID.String()).Int("attempt", attempt+1).Int("status", resp.StatusCode).Msg("webhook: delivered successfully")
			return
		}

		errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		deliveryLog.LastError = &errMsg
		if attempt < len(webhookRetryIntervals) {
			nextRetry := time.Now().Add(webhookRetryIntervals[attempt])
			deliveryLog.NextRetryAt = &nextRetry
		}
		s.persistLog(deliveryLog)
		s.log.Warn().Str("tx_id", txID.String()).Int("attempt", attempt+1).Int("status", resp.StatusCode).Msg("webhook: non-2xx response, retrying")
	}

	deliveryLog.Status = domain.WebhookStatusFailed
	deliveryLog.NextRetryAt = nil
	s.persistLog(deliveryLog)
	s.log.Error().Str("tx_id", txID.String()).Msg("webhook: all retry attempts exhausted")
}

func (s *webhookService) persistLog(log *domain.WebhookDeliveryLog) {
	if s.webhookRepo == nil {
		return
	}
	if err := s.webhookRepo.Update(context.Background(), log); err != nil {
		s.log.Warn().Err(err).Str("log_id", log.ID.String()).Msg("webhook: failed to persist delivery log")
	}
}
