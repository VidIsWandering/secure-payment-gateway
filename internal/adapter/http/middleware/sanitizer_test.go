package middleware

import (
"bytes"
"io"
"net/http"
"net/http/httptest"
"strings"
"testing"

"github.com/gin-gonic/gin"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func init() {
gin.SetMode(gin.TestMode)
}

func TestMaxBodySize_Allowed(t *testing.T) {
r := gin.New()
r.Use(MaxBodySize(1024))
r.POST("/test", func(c *gin.Context) {
b, err := io.ReadAll(c.Request.Body)
if err != nil {
c.String(http.StatusRequestEntityTooLarge, "too large")
return
}
c.String(http.StatusOK, string(b))
})

body := []byte("hello world")
w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)
assert.Equal(t, "hello world", w.Body.String())
}

func TestMaxBodySize_Exceeded(t *testing.T) {
r := gin.New()
r.Use(MaxBodySize(16)) // 16 bytes limit
r.POST("/test", func(c *gin.Context) {
_, err := io.ReadAll(c.Request.Body)
if err != nil {
c.String(http.StatusRequestEntityTooLarge, "too large")
return
}
c.String(http.StatusOK, "ok")
})

bigBody := []byte(strings.Repeat("A", 100))
w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(bigBody))
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestMaxBodySize_NilBody(t *testing.T) {
r := gin.New()
r.Use(MaxBodySize(1024))
r.GET("/test", func(c *gin.Context) {
c.String(http.StatusOK, "ok")
})

w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/test", nil)
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)
}

func TestMaxBodySize_ExactLimit(t *testing.T) {
r := gin.New()
r.Use(MaxBodySize(5))
r.POST("/test", func(c *gin.Context) {
b, err := io.ReadAll(c.Request.Body)
if err != nil {
c.String(http.StatusRequestEntityTooLarge, "too large")
return
}
c.String(http.StatusOK, string(b))
})

body := []byte("12345")
w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
r.ServeHTTP(w, req)

require.Equal(t, http.StatusOK, w.Code)
assert.Equal(t, "12345", w.Body.String())
}
