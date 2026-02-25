package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name:     "without wrapped error",
			appErr:   New("PAY_001", "Insufficient funds", http.StatusPaymentRequired),
			expected: "[PAY_001] Insufficient funds",
		},
		{
			name:     "with wrapped error",
			appErr:   Wrap("SYS_001", "DB error", http.StatusInternalServerError, fmt.Errorf("connection refused")),
			expected: "[SYS_001] DB error: connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.appErr.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	appErr := Wrap("SYS_001", "wrapped", http.StatusInternalServerError, inner)

	assert.True(t, errors.Is(appErr, inner))
}

func TestAppError_IsNilUnwrap(t *testing.T) {
	appErr := New("PAY_001", "test", http.StatusBadRequest)
	assert.Nil(t, appErr.Unwrap())
}

func TestSecurityErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		code       string
		httpStatus int
	}{
		{"InvalidAccessKey", ErrInvalidAccessKey(), "SEC_001", 401},
		{"InvalidSignature", ErrInvalidSignature(), "SEC_002", 401},
		{"TimestampExpired", ErrTimestampExpired(), "SEC_003", 403},
		{"NonceUsed", ErrNonceUsed(), "SEC_004", 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
		})
	}
}

func TestPaymentErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		code       string
		httpStatus int
	}{
		{"InsufficientFunds", ErrInsufficientFunds(), "PAY_001", 402},
		{"InvalidAmount", ErrInvalidAmount(), "PAY_002", 400},
		{"DuplicateTransaction", ErrDuplicateTransaction(), "PAY_003", 409},
		{"NotFound", ErrNotFound("Wallet"), "PAY_004", 404},
		{"TransactionLimitExceeded", ErrTransactionLimitExceeded(), "PAY_005", 422},
		{"InvalidRefund", ErrInvalidRefund(), "PAY_006", 400},
		{"RefundAmountExceeds", ErrRefundAmountExceedsOriginal(), "PAY_007", 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
		})
	}
}

func TestAuthErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		code       string
		httpStatus int
	}{
		{"InvalidCredentials", ErrInvalidCredentials(), "AUTH_001", 401},
		{"UsernameExists", ErrUsernameExists(), "AUTH_002", 409},
		{"InvalidToken", ErrInvalidToken(), "AUTH_003", 401},
		{"MerchantSuspended", ErrMerchantSuspended(), "AUTH_004", 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
		})
	}
}

func TestSystemErrors(t *testing.T) {
	inner := fmt.Errorf("pg: connection closed")
	dbErr := ErrDatabaseError(inner)
	assert.Equal(t, "SYS_001", dbErr.Code)
	assert.Equal(t, 500, dbErr.HTTPStatus)
	assert.True(t, errors.Is(dbErr, inner))

	lockErr := ErrLockTimeout(inner)
	assert.Equal(t, "SYS_002", lockErr.Code)
	assert.Equal(t, 503, lockErr.HTTPStatus)

	encErr := ErrEncryptionFailure(inner)
	assert.Equal(t, "SYS_003", encErr.Code)
	assert.Equal(t, 500, encErr.HTTPStatus)
}

func TestRateLimitError(t *testing.T) {
	err := ErrRateLimitExceeded()
	assert.Equal(t, "RATE_001", err.Code)
	assert.Equal(t, 429, err.HTTPStatus)
}

func TestNotFoundEntity(t *testing.T) {
	err := ErrNotFound("Merchant")
	assert.Contains(t, err.Message, "Merchant")
	assert.Equal(t, "PAY_004", err.Code)
}
