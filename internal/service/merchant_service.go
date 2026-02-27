package service

import (
"context"
"crypto/rand"
"encoding/hex"
"fmt"
"time"

"secure-payment-gateway/internal/core/ports"
"secure-payment-gateway/pkg/apperror"

"github.com/google/uuid"
)

type merchantService struct {
merchantRepo ports.MerchantRepository
encSvc       ports.EncryptionService
}

// NewMerchantService creates a new merchant management service.
func NewMerchantService(
merchantRepo ports.MerchantRepository,
encSvc ports.EncryptionService,
) ports.MerchantManagementService {
return &merchantService{
merchantRepo: merchantRepo,
encSvc:       encSvc,
}
}

func (s *merchantService) GetProfile(ctx context.Context, merchantID uuid.UUID) (*ports.MerchantProfile, error) {
merchant, err := s.merchantRepo.GetByID(ctx, merchantID)
if err != nil {
return nil, apperror.InternalError(err)
}
if merchant == nil {
return nil, apperror.ErrNotFound("merchant")
}

return &ports.MerchantProfile{
ID:           merchant.ID,
Username:     merchant.Username,
MerchantName: merchant.MerchantName,
WebhookURL:   merchant.WebhookURL,
Status:       merchant.Status,
CreatedAt:    merchant.CreatedAt.Format(time.RFC3339),
}, nil
}

func (s *merchantService) UpdateWebhookURL(ctx context.Context, merchantID uuid.UUID, webhookURL *string) error {
merchant, err := s.merchantRepo.GetByID(ctx, merchantID)
if err != nil {
return apperror.InternalError(err)
}
if merchant == nil {
return apperror.ErrNotFound("merchant")
}

merchant.WebhookURL = webhookURL
merchant.UpdatedAt = time.Now()

if err := s.merchantRepo.Update(ctx, merchant); err != nil {
return apperror.InternalError(err)
}
return nil
}

func (s *merchantService) RotateKeys(ctx context.Context, merchantID uuid.UUID) (*ports.RotateKeysResponse, error) {
merchant, err := s.merchantRepo.GetByID(ctx, merchantID)
if err != nil {
return nil, apperror.InternalError(err)
}
if merchant == nil {
return nil, apperror.ErrNotFound("merchant")
}

// Generate new access key and secret key
newAccessKey, err := generateKey("ak_", 24)
if err != nil {
return nil, apperror.InternalError(fmt.Errorf("generate access key: %w", err))
}
newSecretKey, err := generateKey("sk_", 32)
if err != nil {
return nil, apperror.InternalError(fmt.Errorf("generate secret key: %w", err))
}

// Encrypt new secret key
encSecretKey, err := s.encSvc.Encrypt(newSecretKey)
if err != nil {
return nil, apperror.InternalError(fmt.Errorf("encrypt secret key: %w", err))
}

merchant.AccessKey = newAccessKey
merchant.SecretKeyEnc = encSecretKey
merchant.UpdatedAt = time.Now()

if err := s.merchantRepo.Update(ctx, merchant); err != nil {
return nil, apperror.InternalError(err)
}

return &ports.RotateKeysResponse{
AccessKey: newAccessKey,
SecretKey: newSecretKey,
}, nil
}

func generateKey(prefix string, length int) (string, error) {
b := make([]byte, length)
if _, err := rand.Read(b); err != nil {
return "", err
}
return prefix + hex.EncodeToString(b), nil
}
