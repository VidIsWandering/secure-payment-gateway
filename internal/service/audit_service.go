package service

import (
	"context"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"

	"github.com/rs/zerolog"
)

type auditService struct {
	repo ports.AuditRepository
	log  zerolog.Logger
}

// NewAuditService creates a new audit service.
// If repo is nil, audit logs are only written to the logger.
func NewAuditService(repo ports.AuditRepository, log zerolog.Logger) ports.AuditService {
	return &auditService{repo: repo, log: log}
}

// Log records an audit entry asynchronously (fire-and-forget).
// The write is detached from request cancellation so it outlives the response.
func (s *auditService) Log(ctx context.Context, entry *domain.AuditLog) {
	ctx = context.WithoutCancel(ctx)
	go func() {
		s.log.Info().
			Str("action", string(entry.Action)).
			Str("resource_type", entry.ResourceType).
			Str("resource_id", entry.ResourceID).
			Str("ip", entry.IPAddress).
			Msg("audit")

		if s.repo != nil {
			if err := s.repo.Create(ctx, entry); err != nil {
				s.log.Warn().Err(err).Str("action", string(entry.Action)).Msg("failed to persist audit log")
			}
		}
	}()
}
