package ports

import (
	"context"
	"time"

	"secure-payment-gateway/internal/core/domain"

	"github.com/google/uuid"
)

// EncryptionService handles AES-256-GCM encryption/decryption.
type EncryptionService interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// SignatureService handles HMAC-SHA256 signing and verification.
type SignatureService interface {
	Sign(secretKey string, payload string) string
	Verify(secretKey string, payload string, signature string) bool
	BuildCanonicalString(method, path string, timestamp int64, nonce string, body string) string
}

// HashService handles password hashing (Argon2id).
type HashService interface {
	Hash(password string) (string, error)
	Verify(password string, hash string) (bool, error)
}

// TokenService handles JWT token operations.
type TokenService interface {
	Generate(merchantID uuid.UUID, accessKey string) (string, time.Time, error)
	Validate(tokenString string) (*TokenClaims, error)
}

// TokenClaims holds the parsed JWT claims.
type TokenClaims struct {
	MerchantID uuid.UUID
	AccessKey  string
}

// IdempotencyCache is the Redis-layer idempotency check (fast path).
type IdempotencyCache interface {
	Get(ctx context.Context, key string) ([]byte, error) // Returns cached response JSON or nil
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// NonceStore manages nonce uniqueness for replay attack prevention.
type NonceStore interface {
	// CheckAndSet atomically checks if nonce exists, sets it if not.
	// Returns true if nonce is new (valid), false if already used.
	CheckAndSet(ctx context.Context, merchantID string, nonce string, ttl time.Duration) (bool, error)
}

// --- Service Ports (Business Logic) ---

// PaymentService defines the core payment business logic.
type PaymentService interface {
	ProcessPayment(ctx context.Context, req PaymentRequest) (*domain.Transaction, error)
	ProcessRefund(ctx context.Context, req RefundRequest) (*domain.Transaction, error)
	ProcessTopup(ctx context.Context, req TopupRequest) (*domain.Transaction, error)
}

// PaymentRequest holds validated input for payment processing.
type PaymentRequest struct {
	MerchantID  uuid.UUID
	ReferenceID string
	Amount      int64
	Currency    string
	Signature   string
	ClientIP    string
	ExtraData   *string
}

// RefundRequest holds validated input for refund processing.
type RefundRequest struct {
	MerchantID          uuid.UUID
	OriginalReferenceID string
	Amount              *int64 // nil = full refund
	Reason              string
	Signature           string
	ClientIP            string
}

// TopupRequest holds validated input for wallet topup.
type TopupRequest struct {
	MerchantID uuid.UUID
	Amount     int64
	Currency   string
}

// AuthService defines authentication business logic.
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, username, password string) (string, time.Time, error) // token, expiry, error
}

// RegisterRequest holds input for merchant registration.
type RegisterRequest struct {
	Username     string
	Password     string
	MerchantName string
	WebhookURL   *string
}

// RegisterResponse holds the registration result shown once.
type RegisterResponse struct {
	MerchantID uuid.UUID
	AccessKey  string
	SecretKey  string // Plaintext, shown only at registration
}

// ReportingService defines dashboard/reporting business logic.
type ReportingService interface {
	GetDashboardStats(ctx context.Context, merchantID uuid.UUID, period string) (*TransactionStats, error)
	ListTransactions(ctx context.Context, params TransactionListParams) ([]domain.Transaction, int64, error)
	GetWalletBalance(ctx context.Context, merchantID uuid.UUID) (int64, string, error) // balance, currency, error
}

// WebhookService defines async webhook delivery.
type WebhookService interface {
	EnqueueWebhook(ctx context.Context, transaction *domain.Transaction) error
}
