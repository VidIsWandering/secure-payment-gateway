package response

import (
	"errors"
	"net/http"
	"time"

	"secure-payment-gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SuccessResponse is the standard success envelope.
type SuccessResponse struct {
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id"`
	Timestamp string      `json:"timestamp"`
}

// ErrorResponse is the standard error envelope per ERROR_CODES.md.
type ErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// OK sends a 200 response with data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Created sends a 201 response with data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, SuccessResponse{
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Error sends an error response. It checks if err is an *apperror.AppError
// and maps it accordingly, otherwise returns 500.
func Error(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, ErrorResponse{
			ErrorCode: appErr.Code,
			Message:   appErr.Message,
			RequestID: getRequestID(c),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	// Unknown error -> 500
	c.JSON(http.StatusInternalServerError, ErrorResponse{
		ErrorCode: "SYS_000",
		Message:   "Internal server error",
		RequestID: getRequestID(c),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// getRequestID retrieves request ID from context, or generates one.
func getRequestID(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return uuid.New().String()
}
