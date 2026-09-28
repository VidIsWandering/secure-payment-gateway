package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// webhookRetryIntervals defines the retry intervals per WEBHOOK_SPEC.md.
// A delivery is attempted once plus one retry per interval, then marked FAILED.
var webhookRetryIntervals = []time.Duration{
	15 * time.Second,
	60 * time.Second,
	2 * time.Minute,
	5 * time.Minute,
	10 * time.Minute,
}

const (
	webhookPollInterval = time.Second
	webhookBatchSize    = 20
	// webhookLease must exceed the HTTP client timeout so a row is never handed
	// to a second dispatcher while its delivery is still in flight.
	webhookLease = time.Minute
	// HeaderWebhookID lets merchants de-duplicate at-least-once deliveries.
	HeaderWebhookID = "X-Webhook-Id"
)

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

// webhookService implements ports.WebhookService with a transactional outbox:
// Enqueue writes a PENDING webhook_delivery_logs row in the payment's DB
// transaction, and Run delivers due rows in the background. Pending webhooks
// therefore survive restarts and are never sent for a rolled-back payment.
type webhookService struct {
	merchantRepo ports.MerchantRepository
	webhookRepo  ports.WebhookRepository
	encSvc       ports.EncryptionService
	sigSvc       ports.SignatureService
	httpClient   HTTPClient
	log          zerolog.Logger
	now          func() time.Time
}

// HTTPClient interface for testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewWebhookService creates a new webhook service.
func NewWebhookService(
	merchantRepo ports.MerchantRepository,
	webhookRepo ports.WebhookRepository,
	encSvc ports.EncryptionService,
	sigSvc ports.SignatureService,
	httpClient HTTPClient,
	log zerolog.Logger,
) ports.WebhookService {
	return &webhookService{
		merchantRepo: merchantRepo,
		webhookRepo:  webhookRepo,
		encSvc:       encSvc,
		sigSvc:       sigSvc,
		httpClient:   httpClient,
		log:          log,
		now:          time.Now,
	}
}

// Enqueue records a webhook for the transaction in the outbox. It must be
// called with the same DB transaction that persists the money movement.
func (s *webhookService) Enqueue(ctx context.Context, tx ports.Tx, txn *domain.Transaction, currency string) error {
	merchant, err := s.merchantRepo.GetByID(ctx, txn.MerchantID)
	if err != nil {
		return fmt.Errorf("webhook: fetch merchant: %w", err)
	}
	if merchant == nil || merchant.WebhookURL == nil || *merchant.WebhookURL == "" {
		return nil // merchant has not configured a webhook
	}

	eventType := EventPaymentUpdate
	switch txn.TransactionType {
	case domain.TransactionTypeRefund:
		eventType = EventRefundUpdate
	case domain.TransactionTypeTopup:
		eventType = EventTopupUpdate
	}

	now := s.now()
	// The signature is added at delivery time with the merchant's current key.
	payload, err := json.Marshal(WebhookPayload{
		EventType: eventType,
		Data: WebhookPayloadData{
			MerchantOrderID:      txn.ReferenceID,
			GatewayTransactionID: txn.ID.String(),
			Status:               string(txn.Status),
			Amount:               txn.Amount,
			Currency:             currency,
			Reason:               fmt.Sprintf("Transaction %s", txn.Status),
			Timestamp:            now.Unix(),
		},
	})
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	return s.webhookRepo.CreateTx(ctx, tx, &domain.WebhookDeliveryLog{
		ID:            uuid.New(),
		TransactionID: txn.ID,
		MerchantID:    txn.MerchantID,
		WebhookURL:    *merchant.WebhookURL,
		Payload:       string(payload),
		Attempt:       0,
		Status:        domain.WebhookStatusPending,
		NextRetryAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
}

// Run polls the outbox and delivers due webhooks until ctx is cancelled.
// Deliveries already in flight are allowed to finish before Run returns.
func (s *webhookService) Run(ctx context.Context) {
	ticker := time.NewTicker(webhookPollInterval)
	defer ticker.Stop()
	for {
		s.dispatchDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// dispatchDue claims one batch of due deliveries and sends them concurrently.
func (s *webhookService) dispatchDue(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	due, err := s.webhookRepo.ClaimDue(ctx, webhookBatchSize, webhookLease)
	if err != nil {
		s.log.Warn().Err(err).Msg("webhook: failed to claim due deliveries")
		return
	}

	var wg sync.WaitGroup
	for i := range due {
		wg.Add(1)
		go func(d *domain.WebhookDeliveryLog) {
			defer wg.Done()
			// Detached from ctx so shutdown does not abort a delivery half-way.
			s.deliver(context.WithoutCancel(ctx), d)
		}(&due[i])
	}
	wg.Wait()
}

// deliver makes one delivery attempt and records the outcome.
func (s *webhookService) deliver(ctx context.Context, d *domain.WebhookDeliveryLog) {
	d.Attempt++
	logger := s.log.With().Str("tx_id", d.TransactionID.String()).Int("attempt", d.Attempt).Logger()

	httpStatus, err := s.send(ctx, d)
	d.HTTPStatus = httpStatus
	if err == nil {
		d.Status = domain.WebhookStatusDelivered
		d.LastError = nil
		d.NextRetryAt = nil
		logger.Info().Msg("webhook: delivered")
	} else {
		errMsg := err.Error()
		d.LastError = &errMsg
		if d.Attempt > len(webhookRetryIntervals) {
			d.Status = domain.WebhookStatusFailed
			d.NextRetryAt = nil
			logger.Error().Err(err).Msg("webhook: all retry attempts exhausted")
		} else {
			next := s.now().Add(webhookRetryIntervals[d.Attempt-1])
			d.NextRetryAt = &next
			logger.Warn().Err(err).Time("next_retry_at", next).Msg("webhook: delivery failed, will retry")
		}
	}

	if err := s.webhookRepo.Update(ctx, d); err != nil {
		logger.Error().Err(err).Msg("webhook: failed to persist delivery result")
	}
}

// send signs the stored payload with the merchant's current secret and POSTs it.
func (s *webhookService) send(ctx context.Context, d *domain.WebhookDeliveryLog) (*int, error) {
	var payload WebhookPayload
	if err := json.Unmarshal([]byte(d.Payload), &payload); err != nil {
		return nil, fmt.Errorf("decode stored payload: %w", err)
	}

	merchant, err := s.merchantRepo.GetByID(ctx, d.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("fetch merchant: %w", err)
	}
	if merchant == nil {
		return nil, fmt.Errorf("merchant %s not found", d.MerchantID)
	}
	secretKey, err := s.encSvc.Decrypt(merchant.SecretKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt merchant secret: %w", err)
	}
	dataBytes, err := json.Marshal(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal payload data: %w", err)
	}
	payload.Signature = s.sigSvc.Sign(secretKey, string(dataBytes))

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderWebhookID, d.ID.String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		return &status, fmt.Errorf("HTTP %d", status)
	}
	return &status, nil
}
