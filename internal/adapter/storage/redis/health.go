package redis

import (
"context"

goredis "github.com/redis/go-redis/v9"
)

// HealthCheck implements ports.HealthChecker for Redis.
type HealthCheck struct {
client *goredis.Client
}

// NewHealthCheck creates a Redis health checker.
func NewHealthCheck(client *goredis.Client) *HealthCheck {
return &HealthCheck{client: client}
}

// Ping checks Redis connectivity.
func (h *HealthCheck) Ping(ctx context.Context) error {
return h.client.Ping(ctx).Err()
}

// Name returns the dependency name.
func (h *HealthCheck) Name() string {
return "redis"
}
