package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VidIsWandering/secure-payment-gateway/internal/adapter/http/middleware"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports/mocks"
	"github.com/VidIsWandering/secure-payment-gateway/pkg/apperror"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// authedContext returns a gin context whose JWT middleware has already run.
func authedContext(method, target string, body []byte, merchantID *uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if merchantID != nil {
		c.Set(middleware.CtxMerchantID, *merchantID)
	}
	return c, w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

// --- Merchant Handler ---

func TestMerchantHandler_RequiresAuthenticatedMerchant(t *testing.T) {
	h := NewMerchantHandler(mocks.NewMockMerchantManagementService(gomock.NewController(t)))
	for name, fn := range map[string]gin.HandlerFunc{
		"GetProfile": h.GetProfile, "UpdateWebhookURL": h.UpdateWebhookURL, "RotateKeys": h.RotateKeys,
	} {
		t.Run(name, func(t *testing.T) {
			c, w := authedContext(http.MethodGet, "/", nil, nil)
			fn(c)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestMerchantHandler_GetProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	h := NewMerchantHandler(svc)
	merchantID := uuid.New()
	url := "https://shop.example.com/hook"

	svc.EXPECT().GetProfile(gomock.Any(), merchantID).Return(&ports.MerchantProfile{
		ID: merchantID, Username: "shop", MerchantName: "Shop", WebhookURL: &url, Status: domain.MerchantStatusActive,
	}, nil)

	c, w := authedContext(http.MethodGet, "/api/v1/merchants/me", nil, &merchantID)
	h.GetProfile(c)

	require.Equal(t, http.StatusOK, w.Code)
	data := decodeBody(t, w)["data"].(map[string]any)
	assert.Equal(t, merchantID.String(), data["id"])
	assert.Equal(t, url, data["webhook_url"])
	assert.Equal(t, "ACTIVE", data["status"])
	assert.NotContains(t, data, "secret_key", "profile must never expose secrets")
}

func TestMerchantHandler_GetProfile_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	merchantID := uuid.New()
	svc.EXPECT().GetProfile(gomock.Any(), merchantID).Return(nil, apperror.ErrNotFound("merchant"))

	c, w := authedContext(http.MethodGet, "/api/v1/merchants/me", nil, &merchantID)
	NewMerchantHandler(svc).GetProfile(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMerchantHandler_UpdateWebhookURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	merchantID := uuid.New()

	svc.EXPECT().UpdateWebhookURL(gomock.Any(), merchantID, gomock.Any()).
		DoAndReturn(func(_ any, _ uuid.UUID, u *string) error {
			require.NotNil(t, u)
			assert.Equal(t, "https://shop.example.com/hook", *u)
			return nil
		})

	c, w := authedContext(http.MethodPut, "/api/v1/merchants/me/webhook", []byte(`{"webhook_url":"https://shop.example.com/hook"}`), &merchantID)
	NewMerchantHandler(svc).UpdateWebhookURL(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMerchantHandler_UpdateWebhookURL_RejectsNonHTTPScheme(t *testing.T) {
	merchantID := uuid.New()
	// No service call expected: validation fails first.
	svc := mocks.NewMockMerchantManagementService(gomock.NewController(t))

	c, w := authedContext(http.MethodPut, "/api/v1/merchants/me/webhook", []byte(`{"webhook_url":"file:///etc/passwd"}`), &merchantID)
	NewMerchantHandler(svc).UpdateWebhookURL(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMerchantHandler_UpdateWebhookURL_SSRFBlockedByService(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	merchantID := uuid.New()
	svc.EXPECT().UpdateWebhookURL(gomock.Any(), merchantID, gomock.Any()).Return(apperror.Validation("webhook URL must not target a private address"))

	c, w := authedContext(http.MethodPut, "/api/v1/merchants/me/webhook", []byte(`{"webhook_url":"https://10.0.0.1/hook"}`), &merchantID)
	NewMerchantHandler(svc).UpdateWebhookURL(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMerchantHandler_RotateKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	merchantID := uuid.New()
	svc.EXPECT().RotateKeys(gomock.Any(), merchantID).Return(&ports.RotateKeysResponse{AccessKey: "ak_new", SecretKey: "sk_new"}, nil)

	c, w := authedContext(http.MethodPost, "/api/v1/merchants/me/rotate-keys", nil, &merchantID)
	NewMerchantHandler(svc).RotateKeys(c)

	require.Equal(t, http.StatusOK, w.Code)
	data := decodeBody(t, w)["data"].(map[string]any)
	assert.Equal(t, "ak_new", data["access_key"])
	assert.Equal(t, "sk_new", data["secret_key"])
}

func TestMerchantHandler_RotateKeys_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMerchantManagementService(ctrl)
	merchantID := uuid.New()
	svc.EXPECT().RotateKeys(gomock.Any(), merchantID).Return(nil, apperror.InternalError(errors.New("db down")))

	c, w := authedContext(http.MethodPost, "/api/v1/merchants/me/rotate-keys", nil, &merchantID)
	NewMerchantHandler(svc).RotateKeys(c)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Payment status ---

func TestGetPaymentStatus(t *testing.T) {
	merchantID, otherMerchant, txID := uuid.New(), uuid.New(), uuid.New()
	owned := &domain.Transaction{ID: txID, MerchantID: merchantID, ReferenceID: "ORDER-9", Amount: 1000,
		TransactionType: domain.TransactionTypePayment, Status: domain.TransactionStatusSuccess}

	tests := []struct {
		name       string
		id         string
		repoResult *domain.Transaction
		repoErr    error
		callRepo   bool
		wantStatus int
	}{
		{"owner sees transaction", txID.String(), owned, nil, true, http.StatusOK},
		{"other merchant gets 404", txID.String(), &domain.Transaction{ID: txID, MerchantID: otherMerchant}, nil, true, http.StatusNotFound},
		{"unknown id gets 404", txID.String(), nil, nil, true, http.StatusNotFound},
		{"repository error", txID.String(), nil, errors.New("db down"), true, http.StatusInternalServerError},
		{"malformed id", "not-a-uuid", nil, nil, false, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockTransactionRepository(ctrl)
			if tt.callRepo {
				repo.EXPECT().GetByID(gomock.Any(), txID).Return(tt.repoResult, tt.repoErr)
			}
			h := NewPaymentHandler(mocks.NewMockPaymentService(ctrl), repo)

			c, w := authedContext(http.MethodGet, "/api/v1/payments/"+tt.id+"/status", nil, &merchantID)
			c.Params = gin.Params{{Key: "id", Value: tt.id}}
			h.GetPaymentStatus(c)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				data := decodeBody(t, w)["data"].(map[string]any)
				assert.Equal(t, "ORDER-9", data["reference_id"])
			}
		})
	}
}
