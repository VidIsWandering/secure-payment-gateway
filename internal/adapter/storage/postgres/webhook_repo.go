package postgres

import (
	"context"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type webhookRepo struct {
	pool Pool
}

// NewWebhookRepository creates a PostgreSQL-backed WebhookRepository.
func NewWebhookRepository(pool Pool) ports.WebhookRepository {
	return &webhookRepo{pool: pool}
}

const webhookColumns = `id, transaction_id, merchant_id, webhook_url, payload,
http_status, attempt, status, next_retry_at, last_error, created_at, updated_at`

func (r *webhookRepo) CreateTx(ctx context.Context, tx ports.Tx, log *domain.WebhookDeliveryLog) error {
	_, err := UnwrapTx(tx).Exec(ctx,
		`INSERT INTO webhook_delivery_logs (`+webhookColumns+`)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		log.ID, log.TransactionID, log.MerchantID, log.WebhookURL,
		log.Payload, log.HTTPStatus, log.Attempt, string(log.Status),
		log.NextRetryAt, log.LastError, log.CreatedAt, log.UpdatedAt,
	)
	return err
}

// ClaimDue leases due deliveries in one statement. FOR UPDATE SKIP LOCKED lets
// several dispatchers (e.g. API replicas) poll concurrently without handing out
// the same row; the lease keeps a row invisible while it is being delivered and
// makes it due again if the dispatcher dies mid-delivery.
func (r *webhookRepo) ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]domain.WebhookDeliveryLog, error) {
	rows, err := r.pool.Query(ctx,
		`UPDATE webhook_delivery_logs
 SET next_retry_at = NOW() + make_interval(secs => $2), updated_at = NOW()
 WHERE id IN (
   SELECT id FROM webhook_delivery_logs
   WHERE status = 'PENDING' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
   ORDER BY next_retry_at NULLS FIRST, created_at
   LIMIT $1
   FOR UPDATE SKIP LOCKED
 )
 RETURNING `+webhookColumns,
		limit, lease.Seconds())
	if err != nil {
		return nil, err
	}
	return scanWebhookLogs(rows)
}

func (r *webhookRepo) Update(ctx context.Context, log *domain.WebhookDeliveryLog) error {
	log.UpdatedAt = time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE webhook_delivery_logs
 SET http_status=$1, attempt=$2, status=$3, next_retry_at=$4, last_error=$5, updated_at=$6
 WHERE id=$7`,
		log.HTTPStatus, log.Attempt, string(log.Status),
		log.NextRetryAt, log.LastError, log.UpdatedAt, log.ID,
	)
	return err
}

func (r *webhookRepo) GetByTransactionID(ctx context.Context, txID uuid.UUID) ([]domain.WebhookDeliveryLog, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+webhookColumns+`
 FROM webhook_delivery_logs
 WHERE transaction_id=$1
 ORDER BY created_at DESC`, txID)
	if err != nil {
		return nil, err
	}
	return scanWebhookLogs(rows)
}

func scanWebhookLogs(rows pgx.Rows) ([]domain.WebhookDeliveryLog, error) {
	defer rows.Close()

	var logs []domain.WebhookDeliveryLog
	for rows.Next() {
		var l domain.WebhookDeliveryLog
		var status string
		if err := rows.Scan(
			&l.ID, &l.TransactionID, &l.MerchantID, &l.WebhookURL, &l.Payload,
			&l.HTTPStatus, &l.Attempt, &status, &l.NextRetryAt, &l.LastError,
			&l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, err
		}
		l.Status = domain.WebhookStatus(status)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
