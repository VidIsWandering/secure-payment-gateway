package postgres

import (
	"context"
	"errors"
	"fmt"

	"secure-payment-gateway/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// MerchantRepo implements ports.MerchantRepository.
type MerchantRepo struct {
	pool Pool
}

// NewMerchantRepo creates a new MerchantRepo.
func NewMerchantRepo(pool Pool) *MerchantRepo {
	return &MerchantRepo{pool: pool}
}

// Create inserts a new merchant into the database.
func (r *MerchantRepo) Create(ctx context.Context, m *domain.Merchant) error {
	query := `INSERT INTO merchants (id, username, password_hash, merchant_name, access_key, secret_key_enc, webhook_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		m.ID, m.Username, m.PasswordHash, m.MerchantName,
		m.AccessKey, m.SecretKeyEnc, m.WebhookURL, m.Status,
		m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert merchant: %w", err)
	}
	return nil
}

// GetByID fetches a merchant by its UUID.
func (r *MerchantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Merchant, error) {
	query := `SELECT id, username, password_hash, merchant_name, access_key, secret_key_enc, webhook_url, status, created_at, updated_at
		FROM merchants WHERE id = $1`

	m := &domain.Merchant{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID, &m.Username, &m.PasswordHash, &m.MerchantName,
		&m.AccessKey, &m.SecretKeyEnc, &m.WebhookURL, &m.Status,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get merchant by id: %w", err)
	}
	return m, nil
}

// GetByAccessKey fetches a merchant by its public access key.
func (r *MerchantRepo) GetByAccessKey(ctx context.Context, accessKey string) (*domain.Merchant, error) {
	query := `SELECT id, username, password_hash, merchant_name, access_key, secret_key_enc, webhook_url, status, created_at, updated_at
		FROM merchants WHERE access_key = $1`

	m := &domain.Merchant{}
	err := r.pool.QueryRow(ctx, query, accessKey).Scan(
		&m.ID, &m.Username, &m.PasswordHash, &m.MerchantName,
		&m.AccessKey, &m.SecretKeyEnc, &m.WebhookURL, &m.Status,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get merchant by access_key: %w", err)
	}
	return m, nil
}

// GetByUsername fetches a merchant by username.
func (r *MerchantRepo) GetByUsername(ctx context.Context, username string) (*domain.Merchant, error) {
	query := `SELECT id, username, password_hash, merchant_name, access_key, secret_key_enc, webhook_url, status, created_at, updated_at
		FROM merchants WHERE username = $1`

	m := &domain.Merchant{}
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&m.ID, &m.Username, &m.PasswordHash, &m.MerchantName,
		&m.AccessKey, &m.SecretKeyEnc, &m.WebhookURL, &m.Status,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get merchant by username: %w", err)
	}
	return m, nil
}

// Update updates a merchant record.
func (r *MerchantRepo) Update(ctx context.Context, m *domain.Merchant) error {
	query := `UPDATE merchants
		SET merchant_name=$1, webhook_url=$2, access_key=$3, secret_key_enc=$4, status=$5, updated_at=NOW()
		WHERE id=$6`
	_, err := r.pool.Exec(ctx, query,
		m.MerchantName, m.WebhookURL, m.AccessKey, m.SecretKeyEnc, m.Status, m.ID,
	)
	if err != nil {
		return fmt.Errorf("update merchant: %w", err)
	}
	return nil
}
