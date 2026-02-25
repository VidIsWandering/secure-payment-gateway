package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/pkg/apperror"

	"github.com/google/uuid"
)

// AuthServiceImpl implements ports.AuthService.
type AuthServiceImpl struct {
	merchantRepo ports.MerchantRepository
	walletRepo   ports.WalletRepository
	hashSvc      ports.HashService
	encSvc       ports.EncryptionService
	tokenSvc     ports.TokenService
}

// NewAuthService creates a new AuthServiceImpl.
func NewAuthService(
	merchantRepo ports.MerchantRepository,
	walletRepo ports.WalletRepository,
	hashSvc ports.HashService,
	encSvc ports.EncryptionService,
	tokenSvc ports.TokenService,
) *AuthServiceImpl {
	return &AuthServiceImpl{
		merchantRepo: merchantRepo,
		walletRepo:   walletRepo,
		hashSvc:      hashSvc,
		encSvc:       encSvc,
		tokenSvc:     tokenSvc,
	}
}

// Register creates a new merchant account with a wallet.
// Returns the access_key and secret_key (plaintext shown only once).
func (s *AuthServiceImpl) Register(ctx context.Context, req ports.RegisterRequest) (*ports.RegisterResponse, error) {
	// Check username uniqueness
	existing, err := s.merchantRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("check username: %w", err))
	}
	if existing != nil {
		return nil, apperror.Validation("username already exists")
	}

	// Generate key pair
	accessKey, err := generateRandomHex(32) // 64 hex chars
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("generate access key: %w", err))
	}

	secretKey, err := generateRandomHex(32) // 64 hex chars
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("generate secret key: %w", err))
	}

	// Hash password with Argon2id
	passwordHash, err := s.hashSvc.Hash(req.Password)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("hash password: %w", err))
	}

	// Encrypt secret key with AES-256
	secretKeyEnc, err := s.encSvc.Encrypt(secretKey)
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("encrypt secret key: %w", err))
	}

	now := time.Now().UTC()
	merchant := &domain.Merchant{
		ID:           uuid.New(),
		Username:     req.Username,
		PasswordHash: passwordHash,
		MerchantName: req.MerchantName,
		AccessKey:    accessKey,
		SecretKeyEnc: secretKeyEnc,
		WebhookURL:   req.WebhookURL,
		Status:       domain.MerchantStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Create merchant
	if err := s.merchantRepo.Create(ctx, merchant); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("create merchant: %w", err))
	}

	// Encrypt initial balance (0)
	encryptedBalance, err := s.encSvc.Encrypt("0")
	if err != nil {
		return nil, apperror.InternalError(fmt.Errorf("encrypt initial balance: %w", err))
	}

	// Create default wallet
	wallet := &domain.Wallet{
		ID:               uuid.New(),
		MerchantID:       merchant.ID,
		Currency:         "VND",
		EncryptedBalance: encryptedBalance,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.walletRepo.Create(ctx, wallet); err != nil {
		return nil, apperror.InternalError(fmt.Errorf("create wallet: %w", err))
	}

	return &ports.RegisterResponse{
		MerchantID: merchant.ID,
		AccessKey:  accessKey,
		SecretKey:  secretKey,
	}, nil
}

// Login validates credentials and returns a JWT token.
func (s *AuthServiceImpl) Login(ctx context.Context, username, password string) (string, time.Time, error) {
	merchant, err := s.merchantRepo.GetByUsername(ctx, username)
	if err != nil {
		return "", time.Time{}, apperror.InternalError(fmt.Errorf("find merchant: %w", err))
	}
	if merchant == nil {
		return "", time.Time{}, apperror.ErrInvalidCredentials()
	}

	// Verify password
	valid, err := s.hashSvc.Verify(password, merchant.PasswordHash)
	if err != nil {
		return "", time.Time{}, apperror.InternalError(fmt.Errorf("verify password: %w", err))
	}
	if !valid {
		return "", time.Time{}, apperror.ErrInvalidCredentials()
	}

	// Check merchant status
	if !merchant.IsActive() {
		return "", time.Time{}, apperror.ErrMerchantSuspended()
	}

	// Generate JWT
	token, expiry, err := s.tokenSvc.Generate(merchant.ID, merchant.AccessKey)
	if err != nil {
		return "", time.Time{}, apperror.InternalError(fmt.Errorf("generate token: %w", err))
	}

	return token, expiry, nil
}

// generateRandomHex generates a random hex string of n bytes.
func generateRandomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
