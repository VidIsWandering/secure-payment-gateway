package redis

import (
	"testing"

	"secure-payment-gateway/config"

	"github.com/stretchr/testify/assert"
)

func TestRedisAddr(t *testing.T) {
	cfg := config.RedisConfig{
		Host: "redis.example.com",
		Port: 6380,
	}

	assert.Equal(t, "redis.example.com:6380", cfg.Addr())
}

func TestRedisDefaultConfig(t *testing.T) {
	cfg := config.RedisConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "",
		DB:       0,
	}

	assert.Equal(t, "localhost:6379", cfg.Addr())
	assert.Empty(t, cfg.Password)
	assert.Equal(t, 0, cfg.DB)
}
