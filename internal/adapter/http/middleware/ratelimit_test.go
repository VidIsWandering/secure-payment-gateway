package middleware_test

import (
"context"
"net/http"
"net/http/httptest"
"testing"
"time"

"secure-payment-gateway/internal/adapter/http/middleware"
redisStore "secure-payment-gateway/internal/adapter/storage/redis"

"github.com/alicebob/miniredis/v2"
"github.com/gin-gonic/gin"
goredis "github.com/redis/go-redis/v9"
"github.com/rs/zerolog"
"github.com/stretchr/testify/assert"
)

func setupRateLimitRouter(store *redisStore.RateLimitStore) *gin.Engine {
gin.SetMode(gin.TestMode)
r := gin.New()

rule := middleware.RateLimitRule{Limit: 3, Window: time.Minute}
log := zerolog.Nop()

r.GET("/test", middleware.RateLimiter(store, "test", rule, log), func(c *gin.Context) {
c.JSON(200, gin.H{"status": "ok"})
})
return r
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
mr := miniredis.RunT(t)
client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
defer client.Close()

store := redisStore.NewRateLimitStore(client)
router := setupRateLimitRouter(store)

for i := 0; i < 3; i++ {
w := httptest.NewRecorder()
req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
router.ServeHTTP(w, req)
assert.Equal(t, 200, w.Code, "request %d should succeed", i+1)
assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
mr := miniredis.RunT(t)
client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
defer client.Close()

store := redisStore.NewRateLimitStore(client)
router := setupRateLimitRouter(store)

// Use up the limit
for i := 0; i < 3; i++ {
w := httptest.NewRecorder()
req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
router.ServeHTTP(w, req)
assert.Equal(t, 200, w.Code)
}

// 4th request should be blocked
w := httptest.NewRecorder()
req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
router.ServeHTTP(w, req)
assert.Equal(t, 429, w.Code)
assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimiter_UsesAccessKeyHeader(t *testing.T) {
mr := miniredis.RunT(t)
client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
defer client.Close()

store := redisStore.NewRateLimitStore(client)
router := setupRateLimitRouter(store)

// Merchant A uses up the limit
for i := 0; i < 3; i++ {
w := httptest.NewRecorder()
req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
req.Header.Set("X-Merchant-Access-Key", "merchantA")
router.ServeHTTP(w, req)
assert.Equal(t, 200, w.Code)
}

// Merchant B should still be allowed (independent counter)
w := httptest.NewRecorder()
req, _ := http.NewRequestWithContext(context.Background(), "GET", "/test", nil)
req.Header.Set("X-Merchant-Access-Key", "merchantB")
router.ServeHTTP(w, req)
assert.Equal(t, 200, w.Code)
}

func TestDefaultRateLimitRules(t *testing.T) {
rules := middleware.DefaultRateLimitRules()
assert.Equal(t, int64(100), rules["payments"].Limit)
assert.Equal(t, int64(30), rules["payments_refund"].Limit)
assert.Equal(t, int64(10), rules["auth_login"].Limit)
assert.Equal(t, int64(5), rules["auth_register"].Limit)
assert.Equal(t, int64(60), rules["dashboard"].Limit)
assert.Equal(t, int64(20), rules["wallets_topup"].Limit)
}
