package handler

import (
	"secure-payment-gateway/internal/adapter/http/dto"
	"secure-payment-gateway/internal/adapter/http/middleware"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/pkg/apperror"
	"secure-payment-gateway/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WalletHandler handles wallet-related endpoints.
type WalletHandler struct {
	paymentSvc   ports.PaymentService
	reportingSvc ports.ReportingService
	webhookSvc   ports.WebhookService
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(paymentSvc ports.PaymentService, reportingSvc ports.ReportingService, webhookSvc ports.WebhookService) *WalletHandler {
	return &WalletHandler{
		paymentSvc:   paymentSvc,
		reportingSvc: reportingSvc,
		webhookSvc:   webhookSvc,
	}
}

// GetBalance handles GET /api/v1/wallets/balance.
func (h *WalletHandler) GetBalance(c *gin.Context) {
	merchantID, ok := c.Get(middleware.CtxMerchantID)
	if !ok {
		response.Error(c, apperror.ErrInvalidToken())
		return
	}

	balance, currency, err := h.reportingSvc.GetWalletBalance(c.Request.Context(), merchantID.(uuid.UUID))
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, dto.WalletBalanceResponse{
		Balance:  balance,
		Currency: currency,
	})
}

// Topup handles POST /api/v1/wallets/topup.
func (h *WalletHandler) Topup(c *gin.Context) {
	merchantID, ok := c.Get(middleware.CtxMerchantID)
	if !ok {
		response.Error(c, apperror.ErrInvalidToken())
		return
	}

	var req dto.TopupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.Validation(err.Error()))
		return
	}
	dto.SanitizeStruct(&req)

	result, err := h.paymentSvc.ProcessTopup(c.Request.Context(), ports.TopupRequest{
		MerchantID: merchantID.(uuid.UUID),
		Amount:     req.Amount,
		Currency:   req.Currency,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	// Trigger async webhook notification
	if h.webhookSvc != nil {
		_ = h.webhookSvc.EnqueueWebhook(c.Request.Context(), result)
	}

	response.Created(c, toTransactionResponse(result))
}
