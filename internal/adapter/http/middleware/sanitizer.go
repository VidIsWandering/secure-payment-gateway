package middleware

import (
"net/http"

"github.com/gin-gonic/gin"
)

// MaxBodySize returns middleware that limits the request body size.
// Once the limit is exceeded the reader returns an error and the
// request is rejected with 413 Payload Too Large.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
return func(c *gin.Context) {
if c.Request.Body != nil {
c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
}
c.Next()
}
}
