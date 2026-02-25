package apperror

import (
	"fmt"
	"net/http"
)

// AppError is a structured error that maps to HTTP responses.
type AppError struct {
	Code       string `json:"error_code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"` // Wrapped internal error (not exposed to client)
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError.
func New(code string, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps an internal error with an AppError.
func Wrap(code string, message string, httpStatus int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

// ---- Security & Authentication (SEC) ----

func ErrInvalidAccessKey() *AppError {
	return New("SEC_001", "Invalid access key", http.StatusUnauthorized)
}

func ErrInvalidSignature() *AppError {
	return New("SEC_002", "Invalid signature", http.StatusUnauthorized)
}

func ErrTimestampExpired() *AppError {
	return New("SEC_003", "Request timestamp expired", http.StatusForbidden)
}

func ErrNonceUsed() *AppError {
	return New("SEC_004", "Nonce has already been used", http.StatusForbidden)
}

// ---- Payment Business Logic (PAY) ----

func ErrInsufficientFunds() *AppError {
	return New("PAY_001", "Insufficient balance in wallet", http.StatusPaymentRequired)
}

func ErrInvalidAmount() *AppError {
	return New("PAY_002", "Invalid amount", http.StatusBadRequest)
}

func ErrDuplicateTransaction() *AppError {
	return New("PAY_003", "Duplicate transaction", http.StatusConflict)
}

func ErrNotFound(entity string) *AppError {
	return New("PAY_004", fmt.Sprintf("%s not found", entity), http.StatusNotFound)
}

func ErrTransactionLimitExceeded() *AppError {
	return New("PAY_005", "Transaction limit exceeded", http.StatusUnprocessableEntity)
}

func ErrInvalidRefund() *AppError {
	return New("PAY_006", "Original transaction not eligible for refund", http.StatusBadRequest)
}

func ErrRefundAmountExceedsOriginal() *AppError {
	return New("PAY_007", "Refund amount exceeds original transaction amount", http.StatusBadRequest)
}

// ---- Authentication (AUTH) ----

func ErrInvalidCredentials() *AppError {
	return New("AUTH_001", "Invalid credentials", http.StatusUnauthorized)
}

func ErrUsernameExists() *AppError {
	return New("AUTH_002", "Username already exists", http.StatusConflict)
}

func ErrInvalidToken() *AppError {
	return New("AUTH_003", "Invalid or expired token", http.StatusUnauthorized)
}

func ErrMerchantSuspended() *AppError {
	return New("AUTH_004", "Merchant account is suspended", http.StatusForbidden)
}

// ---- Rate Limiting (RATE) ----

func ErrRateLimitExceeded() *AppError {
	return New("RATE_001", "Rate limit exceeded", http.StatusTooManyRequests)
}

// ---- System & Infrastructure (SYS) ----

func ErrDatabaseError(err error) *AppError {
	return Wrap("SYS_001", "Internal database error", http.StatusInternalServerError, err)
}

func ErrLockTimeout(err error) *AppError {
	return Wrap("SYS_002", "Lock acquisition timeout", http.StatusServiceUnavailable, err)
}

func ErrEncryptionFailure(err error) *AppError {
	return Wrap("SYS_003", "Encryption service failure", http.StatusInternalServerError, err)
}

// InternalError wraps an internal error as a SYS_001 error.
func InternalError(err error) *AppError {
	return Wrap("SYS_001", "Internal server error", http.StatusInternalServerError, err)
}

// Validation returns a PAY_002-style validation error.
func Validation(message string) *AppError {
	return New("PAY_002", message, http.StatusBadRequest)
}
