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

func TestNonceStore_CheckAndSet_NewNonce(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	store := NewNonceStore(client)
	ctx := context.Background()

	ok, err := store.CheckAndSet(ctx, "merchant-1", "nonce-abc", 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "new nonce should return true")
}

func TestNonceStore_CheckAndSet_ReplayNonce(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	store := NewNonceStore(client)
	ctx := context.Background()

	// First use
	ok, err := store.CheckAndSet(ctx, "merchant-1", "nonce-xyz", 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)

	// Replay
	ok, err = store.CheckAndSet(ctx, "merchant-1", "nonce-xyz", 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "replayed nonce should return false")
}

func TestNonceStore_CheckAndSet_DifferentMerchants(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	store := NewNonceStore(client)
	ctx := context.Background()

	// Same nonce, different merchants
	ok1, err := store.CheckAndSet(ctx, "merchant-A", "nonce-123", 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, ok1)

	ok2, err := store.CheckAndSet(ctx, "merchant-B", "nonce-123", 5*time.Minute)
	require.NoError(t, err)
	assert.True(t, ok2, "same nonce for different merchant should be valid")
}

func TestNonceStore_CheckAndSet_ExpiredNonce(t *testing.T) {
	s := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: s.Addr()})
	store := NewNonceStore(client)
	ctx := context.Background()

	ok, err := store.CheckAndSet(ctx, "merchant-1", "nonce-expire", 1*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)

	// Fast-forward past TTL
	s.FastForward(2 * time.Second)

	ok, err = store.CheckAndSet(ctx, "merchant-1", "nonce-expire", 1*time.Second)
	require.NoError(t, err)
	assert.True(t, ok, "expired nonce should be accepted again")
}
