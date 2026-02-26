package redis

import (
"context"
"fmt"
"time"

goredis "github.com/redis/go-redis/v9"
)

// RateLimitStore implements rate limiting counters backed by Redis.
type RateLimitStore struct {
client *goredis.Client
prefix string
}

// NewRateLimitStore creates a new Redis-backed rate limit store.
func NewRateLimitStore(client *goredis.Client) *RateLimitStore {
return &RateLimitStore{
client: client,
prefix: "ratelimit:",
}
}

// RateLimitResult holds the outcome of a rate limit check.
type RateLimitResult struct {
Allowed   bool
Limit     int64
Remaining int64
ResetAt   int64 // Unix timestamp
}

// Allow checks if a request is within the rate limit.
// It uses a fixed-window counter: INCR + EXPIRE on a key scoped by windowID.
// windowID should be computed as time / windowDuration to form discrete windows.
func (s *RateLimitStore) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error) {
now := time.Now()
windowID := now.Unix() / int64(window.Seconds())
redisKey := fmt.Sprintf("%s%s:%d", s.prefix, key, windowID)

// Increment counter atomically
count, err := s.client.Incr(ctx, redisKey).Result()
if err != nil {
return nil, fmt.Errorf("redis rate limit incr: %w", err)
}

// Set expiry only on first increment (new window)
if count == 1 {
s.client.Expire(ctx, redisKey, window+time.Second) // +1s safety margin
}

resetAt := (windowID + 1) * int64(window.Seconds())
remaining := limit - count
if remaining < 0 {
remaining = 0
}

return &RateLimitResult{
Allowed:   count <= limit,
Limit:     limit,
Remaining: remaining,
ResetAt:   resetAt,
}, nil
}
