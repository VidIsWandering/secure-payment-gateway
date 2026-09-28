package redis

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/VidIsWandering/secure-payment-gateway/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func redisConfig(t *testing.T, addr string) config.RedisConfig {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return config.RedisConfig{Host: host, Port: port}
}

func TestNewClientAndHealthCheck(t *testing.T) {
	mr := miniredis.RunT(t)
	ctx := context.Background()

	client, err := NewClient(ctx, redisConfig(t, mr.Addr()), zerolog.Nop())
	require.NoError(t, err)
	defer client.Close()

	hc := NewHealthCheck(client)
	assert.Equal(t, "redis", hc.Name())
	assert.NoError(t, hc.Ping(ctx))

	mr.Close()
	assert.Error(t, hc.Ping(ctx), "health check must report an unreachable Redis")
}

func TestNewClient_Unreachable(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := redisConfig(t, mr.Addr())
	mr.Close()

	_, err := NewClient(context.Background(), cfg, zerolog.Nop())
	assert.ErrorContains(t, err, "pinging redis")
}
