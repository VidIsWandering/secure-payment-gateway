package handler

import (
	"secure-payment-gateway/internal/adapter/http/dto"
	"secure-payment-gateway/internal/adapter/http/middleware"
	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/pkg/apperror"
	"secure-payment-gateway/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PaymentHandler handles payment-related endpoints.
type PaymentHandler struct {
	paymentSvc ports.PaymentService
	webhookSvc ports.WebhookService
	txRepo     ports.TransactionRepository
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(paymentSvc ports.PaymentService, webhookSvc ports.WebhookService, txRepo ...ports.TransactionRepository) *PaymentHandler {
	h := &PaymentHandler{paymentSvc: paymentSvc, webhookSvc: webhookSvc}
	if len(txRepo) > 0 {
		h.txRepo = txRepo[0]
	}
	return h
}

// GetPaymentStatus handles GET /api/v1/payments/:id/status.
func (h *PaymentHandler) GetPaymentStatus(c *gin.Context) {
	merchantID, ok := c.Get(middleware.CtxMerchantID)
	if !ok {
		response.Error(c, apperror.ErrInvalidToken())
		return
	}

	txIDStr := c.Param("id")
	txID, err := uuid.Parse(txIDStr)
	if err != nil {
		response.Error(c, apperror.Validation("invalid transaction ID format"))
		return
	}

	if h.txRepo == nil {
		response.Error(c, apperror.InternalError(nil))
		return
	}

	tx, err := h.txRepo.GetByID(c.Request.Context(), txID)
	if err != nil {
		response.Error(c, apperror.InternalError(err))
		return
	}
	if tx == nil || tx.MerchantID != merchantID.(uuid.UUID) {
		response.Error(c, apperror.ErrNotFound("transaction"))
		return
	}

	response.OK(c, toTransactionResponse(tx))
}

// ProcessPayment handles POST /api/v1/payments.
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	merchantID, ok := c.Get(middleware.CtxMerchantID)
	if !ok {
		response.Error(c, apperror.ErrInvalidToken())
		return
	}

	var req dto.PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.Validation(err.Error()))
		return
	}
	dto.SanitizeStruct(&req)

	result, err := h.paymentSvc.ProcessPayment(c.Request.Context(), ports.PaymentRequest{
		MerchantID:  merchantID.(uuid.UUID),
		ReferenceID: req.ReferenceID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Signature:   c.GetHeader(middleware.HeaderSignature),
		ClientIP:    c.ClientIP(),
		ExtraData:   req.ExtraData,
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

// ProcessRefund handles POST /api/v1/payments/refund.
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	merchantID, ok := c.Get(middleware.CtxMerchantID)
	if !ok {
		response.Error(c, apperror.ErrInvalidToken())
		return
	}

	var req dto.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apperror.Validation(err.Error()))
		return
	}
	dto.SanitizeStruct(&req)

	result, err := h.paymentSvc.ProcessRefund(c.Request.Context(), ports.RefundRequest{
		MerchantID:          merchantID.(uuid.UUID),
		OriginalReferenceID: req.OriginalReferenceID,
		Amount:              req.Amount,
		Reason:              req.Reason,
		Signature:           c.GetHeader(middleware.HeaderSignature),
		ClientIP:            c.ClientIP(),
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

// toTransactionResponse converts domain.Transaction to DTO.
func toTransactionResponse(tx *domain.Transaction) dto.TransactionResponse {
	resp := dto.TransactionResponse{
		ID:              tx.ID.String(),
		ReferenceID:     tx.ReferenceID,
		Amount:          tx.Amount,
		TransactionType: string(tx.TransactionType),
		Status:          string(tx.Status),
		CreatedAt:       tx.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if tx.ProcessedAt != nil {
		s := tx.ProcessedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.ProcessedAt = &s
	}
	return resp
}
