package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// IdempotencyCache implements ports.IdempotencyCache using Redis.
type IdempotencyCache struct {
	client *goredis.Client
	prefix string
}

// NewIdempotencyCache creates a new Redis-backed idempotency cache.
func NewIdempotencyCache(client *goredis.Client) *IdempotencyCache {
	return &IdempotencyCache{
		client: client,
		prefix: "idempotency:",
	}
}

// Get retrieves a cached response by idempotency key.
// Returns nil, nil if the key does not exist.
func (c *IdempotencyCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, c.prefix+key).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis idempotency get: %w", err)
	}
	return val, nil
}

// Set stores a response in the idempotency cache with TTL.
func (c *IdempotencyCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := c.client.Set(ctx, c.prefix+key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("redis idempotency set: %w", err)
	}
	return nil
}
