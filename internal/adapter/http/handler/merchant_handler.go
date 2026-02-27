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

// MerchantHandler handles merchant self-service endpoints.
type MerchantHandler struct {
merchantSvc ports.MerchantManagementService
}

// NewMerchantHandler creates a new merchant handler.
func NewMerchantHandler(merchantSvc ports.MerchantManagementService) *MerchantHandler {
return &MerchantHandler{merchantSvc: merchantSvc}
}

// GetProfile returns the authenticated merchant's profile.
func (h *MerchantHandler) GetProfile(c *gin.Context) {
merchantID, ok := c.Get(middleware.CtxMerchantID)
if !ok {
response.Error(c, apperror.ErrInvalidToken())
return
}

profile, err := h.merchantSvc.GetProfile(c.Request.Context(), merchantID.(uuid.UUID))
if err != nil {
response.Error(c, err)
return
}

response.OK(c, gin.H{
"id":            profile.ID.String(),
"username":      profile.Username,
"merchant_name": profile.MerchantName,
"webhook_url":   profile.WebhookURL,
"status":        string(profile.Status),
"created_at":    profile.CreatedAt,
})
}

// UpdateWebhookURL updates the merchant's webhook URL.
func (h *MerchantHandler) UpdateWebhookURL(c *gin.Context) {
merchantID, ok := c.Get(middleware.CtxMerchantID)
if !ok {
response.Error(c, apperror.ErrInvalidToken())
return
}

var req dto.UpdateWebhookRequest
if err := c.ShouldBindJSON(&req); err != nil {
response.Error(c, apperror.Validation(err.Error()))
return
}
dto.SanitizeStruct(&req)

err := h.merchantSvc.UpdateWebhookURL(c.Request.Context(), merchantID.(uuid.UUID), req.WebhookURL)
if err != nil {
response.Error(c, err)
return
}

response.OK(c, gin.H{"message": "webhook URL updated"})
}

// RotateKeys generates new access and secret keys for the merchant.
func (h *MerchantHandler) RotateKeys(c *gin.Context) {
merchantID, ok := c.Get(middleware.CtxMerchantID)
if !ok {
response.Error(c, apperror.ErrInvalidToken())
return
}

result, err := h.merchantSvc.RotateKeys(c.Request.Context(), merchantID.(uuid.UUID))
if err != nil {
response.Error(c, err)
return
}

response.OK(c, gin.H{
"access_key": result.AccessKey,
"secret_key": result.SecretKey,
})
}
