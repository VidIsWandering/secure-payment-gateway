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
	dto.SanitizeStruct(&req)

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
	dto.SanitizeStruct(&req)

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

// HealthCheck handles GET /health — deep health check verifying all dependencies.
func HealthCheck(checkers ...ports.HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		type depStatus struct {
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		}

		deps := make(map[string]depStatus)
		allHealthy := true

		for _, checker := range checkers {
			if err := checker.Ping(c.Request.Context()); err != nil {
				deps[checker.Name()] = depStatus{Status: "unhealthy", Error: err.Error()}
				allHealthy = false
			} else {
				deps[checker.Name()] = depStatus{Status: "healthy"}
			}
		}

		status := "healthy"
		httpCode := http.StatusOK
		if !allHealthy {
			status = "degraded"
			httpCode = http.StatusServiceUnavailable
		}

		c.JSON(httpCode, gin.H{
			"status":       status,
			"dependencies": deps,
		})
	}
}
