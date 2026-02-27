package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"secure-payment-gateway/internal/adapter/http/dto"
	"secure-payment-gateway/internal/core/domain"
	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/internal/core/ports/mocks"
	"secure-payment-gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Auth Handler Tests ---

func TestRegister_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthService(ctrl)
	h := NewAuthHandler(mockAuth)

	merchantID := uuid.New()
	mockAuth.EXPECT().Register(gomock.Any(), ports.RegisterRequest{
		Username:     "testuser",
		Password:     "password123",
		MerchantName: "Test Shop",
	}).Return(&ports.RegisterResponse{
		MerchantID: merchantID,
		AccessKey:  "ak_test",
		SecretKey:  "sk_test",
	}, nil)

	body, _ := json.Marshal(dto.RegisterRequest{
		Username:     "testuser",
		Password:     "password123",
		MerchantName: "Test Shop",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, merchantID.String(), data["merchant_id"])
	assert.Equal(t, "ak_test", data["access_key"])
	assert.Equal(t, "sk_test", data["secret_key"])
}

func TestRegister_ValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthService(ctrl)
	h := NewAuthHandler(mockAuth)

	// Empty body => binding error
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte("{}")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthService(ctrl)
	h := NewAuthHandler(mockAuth)

	mockAuth.EXPECT().Register(gomock.Any(), gomock.Any()).Return(nil, apperror.ErrUsernameExists())

	body, _ := json.Marshal(dto.RegisterRequest{
		Username:     "taken",
		Password:     "password123",
		MerchantName: "Shop",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthService(ctrl)
	h := NewAuthHandler(mockAuth)

	expiry := time.Now().Add(24 * time.Hour)
	mockAuth.EXPECT().Login(gomock.Any(), "testuser", "password123").Return("jwt-token-123", expiry, nil)

	body, _ := json.Marshal(dto.LoginRequest{
		Username: "testuser",
		Password: "password123",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "jwt-token-123", data["token"])
}

func TestLogin_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthService(ctrl)
	h := NewAuthHandler(mockAuth)

	mockAuth.EXPECT().Login(gomock.Any(), "bad", "bad").Return("", time.Time{}, apperror.ErrInvalidCredentials())

	body, _ := json.Marshal(dto.LoginRequest{
		Username: "bad",
		Password: "bad",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- Payment Handler Tests ---

func TestProcessPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	h := NewPaymentHandler(mockPayment, nil)

	merchantID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	mockPayment.EXPECT().ProcessPayment(gomock.Any(), gomock.Any()).Return(&domain.Transaction{
		ID:              txID,
		ReferenceID:     "ref-001",
		MerchantID:      merchantID,
		Amount:          50000,
		TransactionType: domain.TransactionTypePayment,
		Status:          domain.TransactionStatusSuccess,
		CreatedAt:       now,
		ProcessedAt:     &now,
	}, nil)

	body, _ := json.Marshal(dto.PaymentRequest{
		ReferenceID: "ref-001",
		Amount:      50000,
		Currency:    "VND",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("merchant_id", merchantID)

	h.ProcessPayment(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, txID.String(), data["id"])
	assert.Equal(t, "PAYMENT", data["transaction_type"])
}

func TestProcessPayment_MissingMerchantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	h := NewPaymentHandler(mockPayment, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	h.ProcessPayment(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProcessPayment_InsufficientFunds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	h := NewPaymentHandler(mockPayment, nil)

	merchantID := uuid.New()
	mockPayment.EXPECT().ProcessPayment(gomock.Any(), gomock.Any()).Return(nil, apperror.ErrInsufficientFunds())

	body, _ := json.Marshal(dto.PaymentRequest{
		ReferenceID: "ref-001",
		Amount:      9999999,
		Currency:    "VND",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("merchant_id", merchantID)

	h.ProcessPayment(c)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

func TestProcessRefund_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	h := NewPaymentHandler(mockPayment, nil)

	merchantID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	mockPayment.EXPECT().ProcessRefund(gomock.Any(), gomock.Any()).Return(&domain.Transaction{
		ID:              txID,
		ReferenceID:     "refund-001",
		MerchantID:      merchantID,
		Amount:          25000,
		TransactionType: domain.TransactionTypeRefund,
		Status:          domain.TransactionStatusSuccess,
		CreatedAt:       now,
	}, nil)

	body, _ := json.Marshal(dto.RefundRequest{
		OriginalReferenceID: "ref-001",
		Reason:              "Customer request",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("merchant_id", merchantID)

	h.ProcessRefund(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// --- Wallet Handler Tests ---

func TestGetBalance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	mockReporting := mocks.NewMockReportingService(ctrl)
	h := NewWalletHandler(mockPayment, mockReporting, nil)

	merchantID := uuid.New()
	mockReporting.EXPECT().GetWalletBalance(gomock.Any(), merchantID).Return(int64(100000), "VND", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("merchant_id", merchantID)

	h.GetBalance(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(100000), data["balance"])
	assert.Equal(t, "VND", data["currency"])
}

func TestTopup_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPayment := mocks.NewMockPaymentService(ctrl)
	mockReporting := mocks.NewMockReportingService(ctrl)
	h := NewWalletHandler(mockPayment, mockReporting, nil)

	merchantID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	mockPayment.EXPECT().ProcessTopup(gomock.Any(), ports.TopupRequest{
		MerchantID: merchantID,
		Amount:     500000,
		Currency:   "VND",
	}).Return(&domain.Transaction{
		ID:              txID,
		MerchantID:      merchantID,
		Amount:          500000,
		TransactionType: domain.TransactionTypeTopup,
		Status:          domain.TransactionStatusSuccess,
		CreatedAt:       now,
	}, nil)

	body, _ := json.Marshal(dto.TopupRequest{
		Amount:   500000,
		Currency: "VND",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("merchant_id", merchantID)

	h.Topup(c)

	assert.Equal(t, http.StatusCreated, w.Code)
}

// --- Dashboard Handler Tests ---

func TestGetStats_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporting := mocks.NewMockReportingService(ctrl)
	h := NewDashboardHandler(mockReporting)

	merchantID := uuid.New()
	mockReporting.EXPECT().GetDashboardStats(gomock.Any(), merchantID, "all").Return(&ports.TransactionStats{
		TotalTransactions: 100,
		Successful:        80,
		Failed:            15,
		Reversed:          5,
		TotalRevenue:      5000000,
		TotalRefunded:     200000,
		TotalTopup:        1000000,
	}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?period=all", nil)
	c.Set("merchant_id", merchantID)

	h.GetStats(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, float64(100), data["total_transactions"])
	assert.Equal(t, float64(5000000), data["total_revenue"])
}

func TestListTransactions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporting := mocks.NewMockReportingService(ctrl)
	h := NewDashboardHandler(mockReporting)

	merchantID := uuid.New()
	now := time.Now()

	mockReporting.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return([]domain.Transaction{
		{
			ID:              uuid.New(),
			ReferenceID:     "ref-001",
			MerchantID:      merchantID,
			Amount:          50000,
			TransactionType: domain.TransactionTypePayment,
			Status:          domain.TransactionStatusSuccess,
			CreatedAt:       now,
		},
	}, int64(1), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=1&page_size=20", nil)
	c.Set("merchant_id", merchantID)

	h.ListTransactions(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	assert.Len(t, items, 1)
	assert.Equal(t, float64(1), data["total"])
	assert.Equal(t, float64(1), data["total_pages"])
}

func TestListTransactions_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReporting := mocks.NewMockReportingService(ctrl)
	h := NewDashboardHandler(mockReporting)

	merchantID := uuid.New()
	mockReporting.EXPECT().ListTransactions(gomock.Any(), gomock.Any()).Return(nil, int64(0), errors.New("db down"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("merchant_id", merchantID)

	h.ListTransactions(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Health Check Test ---

func TestHealthCheck(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	HealthCheck()(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "healthy", resp["status"])
}

func TestSwaggerUI(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/swagger", nil)

	SwaggerUI(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "swagger-ui")
	assert.Contains(t, w.Body.String(), "/swagger/spec")
}

func TestSwaggerSpec_Loaded(t *testing.T) {
	SetSwaggerSpec([]byte("openapi: '3.0.0'\ninfo:\n  title: Test"))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/swagger/spec", nil)

	SwaggerSpec(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "openapi")
}

func TestSwaggerSpec_NotLoaded(t *testing.T) {
	SetSwaggerSpec(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/swagger/spec", nil)

	SwaggerSpec(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
