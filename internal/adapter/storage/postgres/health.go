package postgres

import "context"

// HealthCheck implements ports.HealthChecker for PostgreSQL.
type HealthCheck struct {
pool Pool
}

// NewHealthCheck creates a PostgreSQL health checker.
func NewHealthCheck(pool Pool) *HealthCheck {
return &HealthCheck{pool: pool}
}

// Ping checks PostgreSQL connectivity.
func (h *HealthCheck) Ping(ctx context.Context) error {
_, err := h.pool.Exec(ctx, "SELECT 1")
return err
}

// Name returns the dependency name.
func (h *HealthCheck) Name() string {
return "postgresql"
}
