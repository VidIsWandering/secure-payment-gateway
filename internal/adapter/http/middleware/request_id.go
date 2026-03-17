package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// HeaderRequestID is echoed in response headers and used in response bodies.
	HeaderRequestID = "X-Request-ID"
	CtxRequestID    = "request_id"
)

// RequestID ensures every request has a stable ID for tracing and error correlation.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = "req_" + uuid.NewString()
		}

		c.Set(CtxRequestID, requestID)
		c.Writer.Header().Set(HeaderRequestID, requestID)
		c.Next()
	}
}
