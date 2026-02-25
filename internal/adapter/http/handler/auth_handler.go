package handler

import (
"net/http"

"secure-payment-gateway/internal/adapter/http/dto"
"secure-payment-gateway/internal/core/ports"
"secure-payment-gateway/pkg/apperror"
"secure-payment-gateway/pkg/response"

"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
authSvc ports.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc ports.AuthService) *AuthHandler {
return &AuthHandler{authSvc: authSvc}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
var req dto.RegisterRequest
if err := c.ShouldBindJSON(&req); err != nil {
response.Error(c, apperror.Validation(err.Error()))
return
}

result, err := h.authSvc.Register(c.Request.Context(), ports.RegisterRequest{
Username:     req.Username,
Password:     req.Password,
MerchantName: req.MerchantName,
WebhookURL:   req.WebhookURL,
})
if err != nil {
response.Error(c, err)
return
}

response.Created(c, dto.RegisterResponse{
MerchantID: result.MerchantID.String(),
AccessKey:  result.AccessKey,
SecretKey:  result.SecretKey,
})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
var req dto.LoginRequest
if err := c.ShouldBindJSON(&req); err != nil {
response.Error(c, apperror.Validation(err.Error()))
return
}

token, expiry, err := h.authSvc.Login(c.Request.Context(), req.Username, req.Password)
if err != nil {
response.Error(c, err)
return
}

response.OK(c, dto.LoginResponse{
Token:  token,
Expiry: expiry.Unix(),
})
}

// HealthCheck handles GET /health.
func HealthCheck() gin.HandlerFunc {
return func(c *gin.Context) {
c.JSON(http.StatusOK, gin.H{
"status": "healthy",
})
}
}
