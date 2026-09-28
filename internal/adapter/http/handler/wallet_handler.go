package handler

import (
	"github.com/VidIsWandering/secure-payment-gateway/internal/adapter/http/dto"
	"github.com/VidIsWandering/secure-payment-gateway/internal/adapter/http/middleware"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"
	"github.com/VidIsWandering/secure-payment-gateway/pkg/apperror"
	"github.com/VidIsWandering/secure-payment-gateway/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WalletHandler handles wallet-related endpoints.
type WalletHandler struct {
	paymentSvc   ports.PaymentService
	reportingSvc ports.ReportingService
}

// NewWalletHandler creates a new WalletHandler.
func NewWalletHandler(paymentSvc ports.PaymentService, reportingSvc ports.ReportingService) *WalletHandler {
	return &WalletHandler{
		paymentSvc:   paymentSvc,
		reportingSvc: reportingSvc,
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

	middleware.RecordTransaction(string(result.TransactionType), string(result.Status))

	response.Created(c, toTransactionResponse(result))
}
