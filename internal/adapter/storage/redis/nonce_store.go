package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// NonceStore implements ports.NonceStore using Redis SET NX.
type NonceStore struct {
	client *goredis.Client
	prefix string
}

// NewNonceStore creates a new Redis-backed nonce store.
func NewNonceStore(client *goredis.Client) *NonceStore {
	return &NonceStore{
		client: client,
		prefix: "nonce:",
	}
}

// CheckAndSet atomically checks if a nonce exists, sets it if not.
// Returns true if the nonce is new (valid), false if already used.
func (s *NonceStore) CheckAndSet(ctx context.Context, merchantID string, nonce string, ttl time.Duration) (bool, error) {
	key := s.prefix + merchantID + ":" + nonce
	result, err := s.client.SetArgs(ctx, key, 1, goredis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err != nil {
		if err == goredis.Nil {
			// Key already exists — nonce was already used
			return false, nil
		}
		return false, fmt.Errorf("redis nonce check: %w", err)
	}
	return result == "OK", nil
}
