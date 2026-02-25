package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/internal/core/ports/mocks"
	"secure-payment-gateway/pkg/apperror"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupAuthService(t *testing.T) (
	*AuthServiceImpl,
	*mocks.MockMerchantRepository,
	*mocks.MockWalletRepository,
	*mocks.MockHashService,
	*mocks.MockEncryptionService,
	*mocks.MockTokenService,
	*gomock.Controller,
) {
	ctrl := gomock.NewController(t)
	merchantRepo := mocks.NewMockMerchantRepository(ctrl)
	walletRepo := mocks.NewMockWalletRepository(ctrl)
	hashSvc := mocks.NewMockHashService(ctrl)
	encSvc := mocks.NewMockEncryptionService(ctrl)
	tokenSvc := mocks.NewMockTokenService(ctrl)

	svc := NewAuthService(merchantRepo, walletRepo, hashSvc, encSvc, tokenSvc)
	return svc, merchantRepo, walletRepo, hashSvc, encSvc, tokenSvc, ctrl
}

func TestAuthService_Register_Success(t *testing.T) {
	svc, merchantRepo, walletRepo, hashSvc, encSvc, _, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	req := ports.RegisterRequest{
		Username:     "new_merchant",
		Password:     "StrongP@ss123",
		MerchantName: "Test Shop",
	}

	// Expect: check username uniqueness
	merchantRepo.EXPECT().GetByUsername(ctx, req.Username).Return(nil, nil)
	// Expect: hash password
	hashSvc.EXPECT().Hash(req.Password).Return("$argon2id$hashed", nil)
	// Expect: encrypt secret key
	encSvc.EXPECT().Encrypt(gomock.Any()).Return("encrypted_secret", nil)
	// Expect: create merchant
	merchantRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)
	// Expect: encrypt initial balance
	encSvc.EXPECT().Encrypt("0").Return("encrypted_zero", nil)
	// Expect: create wallet
	walletRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

	resp, err := svc.Register(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.AccessKey)
	assert.NotEmpty(t, resp.SecretKey)
	assert.Len(t, resp.AccessKey, 64) // 32 bytes = 64 hex chars
	assert.Len(t, resp.SecretKey, 64)
	assert.NotEqual(t, uuid.Nil, resp.MerchantID)
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	svc, merchantRepo, _, _, _, _, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	req := ports.RegisterRequest{
		Username:     "existing_user",
		Password:     "password",
		MerchantName: "Shop",
	}

	existing := &domain.Merchant{Username: "existing_user"}
	merchantRepo.EXPECT().GetByUsername(ctx, req.Username).Return(existing, nil)

	resp, err := svc.Register(ctx, req)
	assert.Nil(t, resp)
	require.Error(t, err)

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "PAY_002", appErr.Code) // Validation error
}

func TestAuthService_Login_Success(t *testing.T) {
	svc, merchantRepo, _, hashSvc, _, tokenSvc, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	merchantID := uuid.New()
	accessKey := "ak_test123"

	merchant := &domain.Merchant{
		ID:           merchantID,
		Username:     "test_user",
		PasswordHash: "$argon2id$hashed",
		AccessKey:    accessKey,
		Status:       domain.MerchantStatusActive,
	}

	merchantRepo.EXPECT().GetByUsername(ctx, "test_user").Return(merchant, nil)
	hashSvc.EXPECT().Verify("correct_password", "$argon2id$hashed").Return(true, nil)
	tokenSvc.EXPECT().Generate(merchantID, accessKey).Return("jwt_token_here", time.Now().Add(24*time.Hour), nil)

	token, _, err := svc.Login(ctx, "test_user", "correct_password")
	require.NoError(t, err)
	assert.Equal(t, "jwt_token_here", token)
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	svc, merchantRepo, _, _, _, _, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	merchantRepo.EXPECT().GetByUsername(ctx, "nonexistent").Return(nil, nil)

	_, _, err := svc.Login(ctx, "nonexistent", "password")
	require.Error(t, err)

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "AUTH_001", appErr.Code)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	svc, merchantRepo, _, hashSvc, _, _, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	merchant := &domain.Merchant{
		ID:           uuid.New(),
		Username:     "test_user",
		PasswordHash: "$argon2id$hashed",
		Status:       domain.MerchantStatusActive,
	}

	merchantRepo.EXPECT().GetByUsername(ctx, "test_user").Return(merchant, nil)
	hashSvc.EXPECT().Verify("wrong_password", "$argon2id$hashed").Return(false, nil)

	_, _, err := svc.Login(ctx, "test_user", "wrong_password")
	require.Error(t, err)

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "AUTH_001", appErr.Code)
}

func TestAuthService_Login_MerchantSuspended(t *testing.T) {
	svc, merchantRepo, _, hashSvc, _, _, ctrl := setupAuthService(t)
	defer ctrl.Finish()

	ctx := context.Background()
	merchant := &domain.Merchant{
		ID:           uuid.New(),
		Username:     "test_user",
		PasswordHash: "$argon2id$hashed",
		Status:       domain.MerchantStatusSuspended,
	}

	merchantRepo.EXPECT().GetByUsername(ctx, "test_user").Return(merchant, nil)
	hashSvc.EXPECT().Verify("correct_password", "$argon2id$hashed").Return(true, nil)

	_, _, err := svc.Login(ctx, "test_user", "correct_password")
	require.Error(t, err)

	var appErr *apperror.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "AUTH_004", appErr.Code)
}
