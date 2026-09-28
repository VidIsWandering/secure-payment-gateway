package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func corsRouter() *gin.Engine {
	r := gin.New()
	r.Use(CORS())
	r.Any("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORS_NoOriginPassesThrough(t *testing.T) {
	w := httptest.NewRecorder()
	corsRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ReflectsOriginWithoutCredentials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://shop.example.com")
	w := httptest.NewRecorder()
	corsRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://shop.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-Signature")
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"),
		"credentials must never be allowed for a reflected origin")
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://shop.example.com")
	w := httptest.NewRecorder()
	corsRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
}
