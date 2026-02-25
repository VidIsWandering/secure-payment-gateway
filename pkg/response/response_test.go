package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-payment-gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "test-req-123")

	OK(c, map[string]string{"status": "healthy"})

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SuccessResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test-req-123", resp.RequestID)
	assert.NotEmpty(t, resp.Timestamp)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", data["status"])
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "test-req-456")

	Created(c, map[string]string{"id": "abc"})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp SuccessResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "test-req-456", resp.RequestID)
}

func TestError_AppError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "test-req-789")

	Error(c, apperror.ErrInsufficientFunds())

	assert.Equal(t, http.StatusPaymentRequired, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "PAY_001", resp.ErrorCode)
	assert.Equal(t, "Insufficient balance in wallet", resp.Message)
	assert.Equal(t, "test-req-789", resp.RequestID)
	assert.NotEmpty(t, resp.Timestamp)
}

func TestError_WrappedAppError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	wrappedErr := fmt.Errorf("outer: %w", apperror.ErrInvalidSignature())
	Error(c, wrappedErr)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "SEC_002", resp.ErrorCode)
}

func TestError_UnknownError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Error(c, fmt.Errorf("something unexpected"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "SYS_000", resp.ErrorCode)
	assert.Equal(t, "Internal server error", resp.Message)
}

func TestOK_GeneratesRequestID_WhenMissing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// No request_id set in context.

	OK(c, nil)

	var resp SuccessResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.RequestID, "should generate a UUID when request_id is missing")
}
