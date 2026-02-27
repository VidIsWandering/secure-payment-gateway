package service

import (
"context"
"errors"
"testing"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports/mocks"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
"go.uber.org/mock/gomock"
)

func TestMerchantService_GetProfile_Success(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockMerchantRepository(ctrl)
mockEnc := mocks.NewMockEncryptionService(ctrl)
svc := NewMerchantService(mockRepo, mockEnc)

merchantID := uuid.New()
webhookURL := "https://example.com/webhook"
mockRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
ID:           merchantID,
Username:     "testuser",
MerchantName: "Test Shop",
WebhookURL:   &webhookURL,
Status:       domain.MerchantStatusActive,
}, nil)

profile, err := svc.GetProfile(context.Background(), merchantID)
require.NoError(t, err)
assert.Equal(t, merchantID, profile.ID)
assert.Equal(t, "testuser", profile.Username)
assert.Equal(t, "Test Shop", profile.MerchantName)
assert.Equal(t, &webhookURL, profile.WebhookURL)
}

func TestMerchantService_GetProfile_NotFound(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockMerchantRepository(ctrl)
mockEnc := mocks.NewMockEncryptionService(ctrl)
svc := NewMerchantService(mockRepo, mockEnc)

mockRepo.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(nil, nil)

_, err := svc.GetProfile(context.Background(), uuid.New())
assert.Error(t, err)
}

func TestMerchantService_UpdateWebhookURL(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockMerchantRepository(ctrl)
mockEnc := mocks.NewMockEncryptionService(ctrl)
svc := NewMerchantService(mockRepo, mockEnc)

merchantID := uuid.New()
mockRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
ID: merchantID,
}, nil)
mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

newURL := "https://new.example.com/hook"
err := svc.UpdateWebhookURL(context.Background(), merchantID, &newURL)
assert.NoError(t, err)
}

func TestMerchantService_RotateKeys_Success(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockMerchantRepository(ctrl)
mockEnc := mocks.NewMockEncryptionService(ctrl)
svc := NewMerchantService(mockRepo, mockEnc)

merchantID := uuid.New()
mockRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
ID: merchantID,
}, nil)
mockEnc.EXPECT().Encrypt(gomock.Any()).Return("encrypted-new-secret", nil)
mockRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

result, err := svc.RotateKeys(context.Background(), merchantID)
require.NoError(t, err)
assert.Contains(t, result.AccessKey, "ak_")
assert.Contains(t, result.SecretKey, "sk_")
assert.True(t, len(result.AccessKey) > 10)
assert.True(t, len(result.SecretKey) > 10)
}

func TestMerchantService_RotateKeys_EncryptError(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockMerchantRepository(ctrl)
mockEnc := mocks.NewMockEncryptionService(ctrl)
svc := NewMerchantService(mockRepo, mockEnc)

merchantID := uuid.New()
mockRepo.EXPECT().GetByID(gomock.Any(), merchantID).Return(&domain.Merchant{
ID: merchantID,
}, nil)
mockEnc.EXPECT().Encrypt(gomock.Any()).Return("", errors.New("encrypt failed"))

_, err := svc.RotateKeys(context.Background(), merchantID)
assert.Error(t, err)
}
