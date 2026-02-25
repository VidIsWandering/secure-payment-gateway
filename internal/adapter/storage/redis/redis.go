package redis

import (
	"context"
	"fmt"

	"secure-payment-gateway/config"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// NewClient creates a Redis client and verifies connectivity.
func NewClient(ctx context.Context, cfg config.RedisConfig, log zerolog.Logger) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Verify connectivity
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	log.Info().
		Str("addr", cfg.Addr()).
		Int("db", cfg.DB).
		Msg("Redis connection established")

	return client, nil
}
