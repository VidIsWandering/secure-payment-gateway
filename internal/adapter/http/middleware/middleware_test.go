package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports/mocks"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHMACAuth_MissingHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	merchantRepo := mocks.NewMockMerchantRepository(ctrl)
	encSvc := mocks.NewMockEncryptionService(ctrl)
	sigSvc := mocks.NewMockSignatureService(ctrl)
	nonceStore := mocks.NewMockNonceStore(ctrl)
	log := zerolog.Nop()

	router := gin.New()
	router.POST("/test", HMACAuth(merchantRepo, encSvc, sigSvc, nonceStore, false, log), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_ExpiredTimestamp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	merchantRepo := mocks.NewMockMerchantRepository(ctrl)
	encSvc := mocks.NewMockEncryptionService(ctrl)
	sigSvc := mocks.NewMockSignatureService(ctrl)
	nonceStore := mocks.NewMockNonceStore(ctrl)
	log := zerolog.Nop()

	router := gin.New()
	router.POST("/test", HMACAuth(merchantRepo, encSvc, sigSvc, nonceStore, false, log), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(HeaderAccessKey, "ak_test")
	req.Header.Set(HeaderSignature, "sig")
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(time.Now().Add(-120*time.Second).Unix(), 10))
	req.Header.Set(HeaderNonce, "nonce123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHMACAuth_InvalidAccessKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	merchantRepo := mocks.NewMockMerchantRepository(ctrl)
	encSvc := mocks.NewMockEncryptionService(ctrl)
	sigSvc := mocks.NewMockSignatureService(ctrl)
	nonceStore := mocks.NewMockNonceStore(ctrl)
	log := zerolog.Nop()

	merchantRepo.EXPECT().GetByAccessKey(gomock.Any(), "invalid_key").Return(nil, nil)

	router := gin.New()
	router.POST("/test", HMACAuth(merchantRepo, encSvc, sigSvc, nonceStore, false, log), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(HeaderAccessKey, "invalid_key")
	req.Header.Set(HeaderSignature, "sig")
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set(HeaderNonce, "nonce123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	merchantRepo := mocks.NewMockMerchantRepository(ctrl)
	encSvc := mocks.NewMockEncryptionService(ctrl)
	sigSvc := mocks.NewMockSignatureService(ctrl)
	nonceStore := mocks.NewMockNonceStore(ctrl)
	log := zerolog.Nop()

	merchantID := uuid.New()
	merchant := &domain.Merchant{
		ID:           merchantID,
		AccessKey:    "ak_valid",
		SecretKeyEnc: "enc_secret",
		Status:       domain.MerchantStatusActive,
	}

	nowTs := time.Now().Unix()
	body := `{"amount":50000}`

	merchantRepo.EXPECT().GetByAccessKey(gomock.Any(), "ak_valid").Return(merchant, nil)
	nonceStore.EXPECT().CheckAndSet(gomock.Any(), merchantID.String(), "nonce-ok", nonceTTL).Return(true, nil)
	encSvc.EXPECT().Decrypt("enc_secret").Return("raw_secret", nil)
	sigSvc.EXPECT().BuildCanonicalString("POST", "/test", nowTs, "nonce-ok", body).Return("canonical")
	sigSvc.EXPECT().Verify("raw_secret", "canonical", "valid_sig").Return(true)

	var capturedID uuid.UUID
	router := gin.New()
	router.POST("/test", HMACAuth(merchantRepo, encSvc, sigSvc, nonceStore, false, log), func(c *gin.Context) {
		id, _ := c.Get(CtxMerchantID)
		capturedID = id.(uuid.UUID)
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set(HeaderAccessKey, "ak_valid")
	req.Header.Set(HeaderSignature, "valid_sig")
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(nowTs, 10))
	req.Header.Set(HeaderNonce, "nonce-ok")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, merchantID, capturedID)
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenSvc := mocks.NewMockTokenService(ctrl)
	log := zerolog.Nop()

	router := gin.New()
	router.GET("/test", JWTAuth(tokenSvc, log), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenSvc := mocks.NewMockTokenService(ctrl)
	log := zerolog.Nop()

	tokenSvc.EXPECT().Validate("bad_token").Return(nil, assert.AnError)

	router := gin.New()
	router.GET("/test", JWTAuth(tokenSvc, log), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer bad_token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenSvc := mocks.NewMockTokenService(ctrl)
	log := zerolog.Nop()

	merchantID := uuid.New()
	tokenSvc.EXPECT().Validate("good_token").Return(&ports.TokenClaims{
		MerchantID: merchantID,
		AccessKey:  "ak_test",
	}, nil)

	var capturedID uuid.UUID
	router := gin.New()
	router.GET("/test", JWTAuth(tokenSvc, log), func(c *gin.Context) {
		id, _ := c.Get(CtxMerchantID)
		capturedID = id.(uuid.UUID)
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer good_token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, merchantID, capturedID)
}

func TestRecovery_PanicRecovered(t *testing.T) {
	log := zerolog.Nop()

	router := gin.New()
	router.Use(Recovery(log))
	router.GET("/panic", func(c *gin.Context) {
		panic("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "SYS_001", resp["error_code"])
}

func TestHMACAuth_NonceStoreDown(t *testing.T) {
	tests := []struct {
		name       string
		failOpen   bool
		wantStatus int
	}{
		{"fail closed rejects request", false, http.StatusServiceUnavailable},
		{"fail open verifies signature and continues", true, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			merchantRepo := mocks.NewMockMerchantRepository(ctrl)
			encSvc := mocks.NewMockEncryptionService(ctrl)
			sigSvc := mocks.NewMockSignatureService(ctrl)
			nonceStore := mocks.NewMockNonceStore(ctrl)

			merchantID := uuid.New()
			merchant := &domain.Merchant{
				ID:           merchantID,
				AccessKey:    "ak_valid",
				SecretKeyEnc: "enc_secret",
				Status:       domain.MerchantStatusActive,
			}
			nowTs := time.Now().Unix()
			body := `{"amount":50000}`

			merchantRepo.EXPECT().GetByAccessKey(gomock.Any(), "ak_valid").Return(merchant, nil)
			nonceStore.EXPECT().CheckAndSet(gomock.Any(), merchantID.String(), "nonce-1", nonceTTL).
				Return(false, errors.New("redis: connection refused"))
			if tt.failOpen {
				encSvc.EXPECT().Decrypt("enc_secret").Return("raw_secret", nil)
				sigSvc.EXPECT().BuildCanonicalString("POST", "/test", nowTs, "nonce-1", body).Return("canonical")
				sigSvc.EXPECT().Verify("raw_secret", "canonical", "valid_sig").Return(true)
			}

			router := gin.New()
			router.POST("/test", HMACAuth(merchantRepo, encSvc, sigSvc, nonceStore, tt.failOpen, zerolog.Nop()), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
			req.Header.Set(HeaderAccessKey, "ak_valid")
			req.Header.Set(HeaderSignature, "valid_sig")
			req.Header.Set(HeaderTimestamp, strconv.FormatInt(nowTs, 10))
			req.Header.Set(HeaderNonce, "nonce-1")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if !tt.failOpen {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "SYS_004", resp["error_code"])
			}
		})
	}
}
