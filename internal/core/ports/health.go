package ports

import "context"

// HealthChecker checks external dependency health.
type HealthChecker interface {
// Ping verifies connectivity. Returns nil if healthy.
Ping(ctx context.Context) error
// Name returns the dependency name (e.g., "postgresql", "redis").
Name() string
}
