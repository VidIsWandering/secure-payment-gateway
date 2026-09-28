package postgres

import (
	"context"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"
)

type auditRepo struct {
	pool Pool
}

// NewAuditRepository creates a PostgreSQL-backed AuditRepository.
func NewAuditRepository(pool Pool) ports.AuditRepository {
	return &auditRepo{pool: pool}
}

func (r *auditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs (id, merchant_id, action, resource_type, resource_id, details, ip_address, created_at)
 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		log.ID, log.MerchantID, string(log.Action), log.ResourceType,
		log.ResourceID, log.Details, log.IPAddress, log.CreatedAt,
	)
	return err
}
