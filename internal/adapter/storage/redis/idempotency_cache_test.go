package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdempotencyCache_SetAndGet(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	cache := NewIdempotencyCache(client)
	ctx := context.Background()

	key := "merchant-123:ORDER-001"
	value := []byte(`{"transaction_id":"abc","status":"SUCCESS"}`)

	// Get before set => nil
	result, err := cache.Get(ctx, key)
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Set
	err = cache.Set(ctx, key, value, 24*time.Hour)
	require.NoError(t, err)

	// Get after set
	result, err = cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, result)
}

func TestIdempotencyCache_TTLExpiry(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	cache := NewIdempotencyCache(client)
	ctx := context.Background()

	key := "merchant-456:ORDER-002"
	value := []byte(`{"data":"test"}`)

	err := cache.Set(ctx, key, value, 1*time.Second)
	require.NoError(t, err)

	// Fast-forward time in miniredis
	s.FastForward(2 * time.Second)

	result, err := cache.Get(ctx, key)
	assert.NoError(t, err)
	assert.Nil(t, result, "expired key should return nil")
}

func TestIdempotencyCache_OverwriteKey(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	cache := NewIdempotencyCache(client)
	ctx := context.Background()

	key := "merchant-789:ORDER-003"

	err := cache.Set(ctx, key, []byte("first"), 1*time.Hour)
	require.NoError(t, err)

	err = cache.Set(ctx, key, []byte("second"), 1*time.Hour)
	require.NoError(t, err)

	result, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("second"), result)
}
