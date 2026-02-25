package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-jwt-secret-key-for-unit-tests"

func TestJWTTokenService_GenerateAndValidate(t *testing.T) {
	svc := NewJWTTokenService(testJWTSecret, 24*time.Hour, "test-issuer")
	merchantID := uuid.New()
	accessKey := "test-access-key"

	tokenStr, expiresAt, err := svc.Generate(merchantID, accessKey)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)
	assert.True(t, expiresAt.After(time.Now()))

	claims, err := svc.Validate(tokenStr)
	require.NoError(t, err)
	assert.Equal(t, merchantID, claims.MerchantID)
	assert.Equal(t, accessKey, claims.AccessKey)
}

func TestJWTTokenService_ExpiredToken(t *testing.T) {
	// Token with -1 hour expiry = already expired
	svc := NewJWTTokenService(testJWTSecret, -1*time.Hour, "test-issuer")
	merchantID := uuid.New()

	tokenStr, _, err := svc.Generate(merchantID, "key")
	require.NoError(t, err)

	_, err = svc.Validate(tokenStr)
	assert.Error(t, err, "expired token should fail validation")
}

func TestJWTTokenService_InvalidSignature(t *testing.T) {
	svc1 := NewJWTTokenService("secret-1", 24*time.Hour, "issuer")
	svc2 := NewJWTTokenService("secret-2", 24*time.Hour, "issuer")

	tokenStr, _, err := svc1.Generate(uuid.New(), "key")
	require.NoError(t, err)

	_, err = svc2.Validate(tokenStr)
	assert.Error(t, err, "token signed with different secret should fail")
}

func TestJWTTokenService_InvalidTokenString(t *testing.T) {
	svc := NewJWTTokenService(testJWTSecret, 24*time.Hour, "issuer")

	_, err := svc.Validate("not.a.valid.jwt")
	assert.Error(t, err)
}

func TestJWTTokenService_EmptyToken(t *testing.T) {
	svc := NewJWTTokenService(testJWTSecret, 24*time.Hour, "issuer")

	_, err := svc.Validate("")
	assert.Error(t, err)
}
