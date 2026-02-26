package middleware

import (
"fmt"
"strconv"
"time"

redisStore "secure-payment-gateway/internal/adapter/storage/redis"
"secure-payment-gateway/pkg/apperror"
"secure-payment-gateway/pkg/response"

"github.com/gin-gonic/gin"
"github.com/rs/zerolog"
)

// RateLimitRule defines a rate limit for an endpoint group.
type RateLimitRule struct {
Limit  int64
Window time.Duration
}

// DefaultRateLimitRules returns the spec-defined rate limits per endpoint group.
func DefaultRateLimitRules() map[string]RateLimitRule {
return map[string]RateLimitRule{
"payments":        {Limit: 100, Window: time.Minute},
"payments_refund": {Limit: 30, Window: time.Minute},
"auth_login":      {Limit: 10, Window: time.Minute},
"auth_register":   {Limit: 5, Window: time.Hour},
"dashboard":       {Limit: 60, Window: time.Minute},
"wallets_topup":   {Limit: 20, Window: time.Minute},
}
}

// RateLimiter creates a rate-limiting middleware for a given endpoint group.
func RateLimiter(store *redisStore.RateLimitStore, group string, rule RateLimitRule, log zerolog.Logger) gin.HandlerFunc {
return func(c *gin.Context) {
identifier := extractIdentifier(c)
key := fmt.Sprintf("%s:%s", identifier, group)

result, err := store.Allow(c.Request.Context(), key, rule.Limit, rule.Window)
if err != nil {
log.Warn().Err(err).Str("group", group).Msg("rate limit check failed, allowing request (degraded mode)")
c.Next()
return
}

// Always set rate limit headers
c.Header("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
c.Header("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
c.Header("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt, 10))

if !result.Allowed {
retryAfter := result.ResetAt - time.Now().Unix()
if retryAfter < 1 {
retryAfter = 1
}
c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
response.Error(c, apperror.ErrRateLimitExceeded())
c.Abort()
return
}

c.Next()
}
}

// extractIdentifier determines the rate limit key source.
func extractIdentifier(c *gin.Context) string {
if ak := c.GetHeader(HeaderAccessKey); ak != "" {
return ak
}
if mid, exists := c.Get(CtxMerchantID); exists {
return fmt.Sprintf("%v", mid)
}
return c.ClientIP()
}
