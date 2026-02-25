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
	ok, err := s.client.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis nonce check: %w", err)
	}
	return ok, nil
}
