package middleware

import (
	"context"
	"net/http"
	"time"

	"secure-payment-gateway/pkg/apperror"
	"secure-payment-gateway/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequestTimeout returns a middleware that aborts the request if it exceeds the given duration.
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		finished := make(chan struct{}, 1)
		go func() {
			c.Next()
			finished <- struct{}{}
		}()

		select {
		case <-finished:
			return
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				response.Error(c, apperror.Wrap("SYS_004", "Request timeout", http.StatusGatewayTimeout, ctx.Err()))
				c.Abort()
			}
		}
	}
}
