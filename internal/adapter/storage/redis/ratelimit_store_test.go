package redis_test

import (
"context"
"testing"
"time"

"secure-payment-gateway/internal/adapter/storage/redis"

"github.com/alicebob/miniredis/v2"
goredis "github.com/redis/go-redis/v9"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestRateLimitStore_Allow(t *testing.T) {
mr := miniredis.RunT(t)
client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
defer client.Close()

store := redis.NewRateLimitStore(client)
ctx := context.Background()

t.Run("allows requests within limit", func(t *testing.T) {
for i := int64(1); i <= 3; i++ {
result, err := store.Allow(ctx, "merchant1:payments", 3, time.Minute)
require.NoError(t, err)
assert.True(t, result.Allowed, "request %d should be allowed", i)
assert.Equal(t, int64(3), result.Limit)
assert.Equal(t, 3-i, result.Remaining)
}
})

t.Run("blocks requests over limit", func(t *testing.T) {
// 4th request should be blocked (limit is 3 from above)
result, err := store.Allow(ctx, "merchant1:payments", 3, time.Minute)
require.NoError(t, err)
assert.False(t, result.Allowed)
assert.Equal(t, int64(0), result.Remaining)
})

t.Run("different keys are independent", func(t *testing.T) {
result, err := store.Allow(ctx, "merchant2:payments", 5, time.Minute)
require.NoError(t, err)
assert.True(t, result.Allowed)
assert.Equal(t, int64(4), result.Remaining)
})

t.Run("reset after window expires", func(t *testing.T) {
// Use a short window key that we can expire
key := "merchant3:auth"
_, err := store.Allow(ctx, key, 1, time.Minute)
require.NoError(t, err)

// Second request in same window is blocked
result, err := store.Allow(ctx, key, 1, time.Minute)
require.NoError(t, err)
assert.False(t, result.Allowed)

// Fast-forward time in miniredis
mr.FastForward(61 * time.Second)

// Now a new window should allow
result, err = store.Allow(ctx, key, 1, time.Minute)
require.NoError(t, err)
assert.True(t, result.Allowed)
})

t.Run("sets correct ResetAt", func(t *testing.T) {
result, err := store.Allow(ctx, "merchant4:dashboard", 10, time.Minute)
require.NoError(t, err)
assert.True(t, result.Allowed)
assert.Greater(t, result.ResetAt, time.Now().Unix()-1)
})
}
