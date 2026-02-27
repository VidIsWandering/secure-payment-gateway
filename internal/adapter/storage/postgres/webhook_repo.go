package postgres

import (
"context"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"

"github.com/google/uuid"
"github.com/jackc/pgx/v5/pgxpool"
)

type webhookRepo struct {
pool *pgxpool.Pool
}

// NewWebhookRepository creates a PostgreSQL-backed WebhookRepository.
func NewWebhookRepository(pool *pgxpool.Pool) ports.WebhookRepository {
return &webhookRepo{pool: pool}
}

func (r *webhookRepo) Create(ctx context.Context, log *domain.WebhookDeliveryLog) error {
_, err := r.pool.Exec(ctx,
`INSERT INTO webhook_delivery_logs
(id, transaction_id, merchant_id, webhook_url, payload, http_status, attempt, status, next_retry_at, last_error, created_at, updated_at)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
log.ID, log.TransactionID, log.MerchantID, log.WebhookURL,
log.Payload, log.HTTPStatus, log.Attempt, string(log.Status),
log.NextRetryAt, log.LastError, log.CreatedAt, log.UpdatedAt,
)
return err
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
`SELECT id, transaction_id, merchant_id, webhook_url, payload,
http_status, attempt, status, next_retry_at, last_error,
created_at, updated_at
 FROM webhook_delivery_logs
 WHERE transaction_id=$1
 ORDER BY created_at DESC`, txID)
if err != nil {
return nil, err
}
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
