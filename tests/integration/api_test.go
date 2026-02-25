package integration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpHandler "secure-payment-gateway/internal/adapter/http/handler"
	redisStorage "secure-payment-gateway/internal/adapter/storage/redis"
	"secure-payment-gateway/internal/service"
	"secure-payment-gateway/pkg/logger"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testApp builds a full application stack with mocked storage connected via
// in-memory Redis (miniredis) and mock-based postgres repos. This exercises
// the real HTTP layer, middleware, handlers, services, and Redis stores end-to-end.

type testApp struct {
	server *httptest.Server
	redis  *miniredis.Miniredis
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()

	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)

	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	// Redis stores
	idempotencyCache := redisStorage.NewIdempotencyCache(rdb)
	nonceStore := redisStorage.NewNonceStore(rdb)

	// Core services with real implementations
	encSvc, err := service.NewAESEncryptionService("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	sigSvc := service.NewHMACSignatureService()
	hashSvc := service.NewArgon2HashService()
	tokenSvc := service.NewJWTTokenService("test-jwt-secret-key-32bytes!!", 24*time.Hour, "test-issuer")

	// In-memory repos
	merchantRepo := newInMemoryMerchantRepo()
	walletRepo := newInMemoryWalletRepo()
	txRepo := newInMemoryTransactionRepo()
	idempotencyRepo := newInMemoryIdempotencyRepo()
	transactor := newInMemoryTransactor()

	// Business services
	authSvc := service.NewAuthService(merchantRepo, walletRepo, hashSvc, encSvc, tokenSvc)
	log := logger.New("debug", false)
	paymentSvc := service.NewPaymentService(txRepo, walletRepo, idempotencyRepo, idempotencyCache, encSvc, transactor, log)
	reportingSvc := service.NewReportingService(txRepo, walletRepo, encSvc)

	router := httpHandler.SetupRouter(httpHandler.RouterDeps{
		AuthSvc:      authSvc,
		PaymentSvc:   paymentSvc,
		ReportingSvc: reportingSvc,
		MerchantRepo: merchantRepo,
		EncSvc:       encSvc,
		SigSvc:       sigSvc,
		NonceStore:   nonceStore,
		TokenSvc:     tokenSvc,
		Logger:       log,
	})

	server := httptest.NewServer(router)

	return &testApp{
		server: server,
		redis:  mr,
	}
}

func (a *testApp) close() {
	a.server.Close()
	a.redis.Close()
}

// --- Integration Tests ---

func TestIntegration_HealthCheck(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	resp, err := http.Get(app.server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "healthy", body["status"])
}

func TestIntegration_RegisterAndLogin(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	// Register
	regBody, _ := json.Marshal(map[string]string{
		"username":      "merchant1",
		"password":      "StrongPass123!",
		"merchant_name": "Test Merchant",
	})
	resp, err := http.Post(app.server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var regResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&regResp))
	data := regResp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["merchant_id"])
	assert.NotEmpty(t, data["access_key"])
	assert.NotEmpty(t, data["secret_key"])

	// Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": "merchant1",
		"password": "StrongPass123!",
	})
	resp2, err := http.Post(app.server.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var loginResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&loginResp))
	loginData := loginResp["data"].(map[string]interface{})
	assert.NotEmpty(t, loginData["token"])
}

func TestIntegration_LoginWrongCredentials(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	loginBody, _ := json.Marshal(map[string]string{
		"username": "nobody",
		"password": "wrong",
	})
	resp, err := http.Post(app.server.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_DuplicateUsername(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	regBody, _ := json.Marshal(map[string]string{
		"username":      "merchant1",
		"password":      "StrongPass123!",
		"merchant_name": "Test",
	})

	resp, err := http.Post(app.server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Try again with same username
	resp2, err := http.Post(app.server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)
}

func TestIntegration_JWT_Dashboard(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	// Register + login
	token := registerAndLogin(t, app)

	// Get wallet balance
	req, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/v1/wallets/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["balance"])
	assert.Equal(t, "VND", data["currency"])
}

func TestIntegration_JWT_DashboardStats(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	token := registerAndLogin(t, app)

	req, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/v1/dashboard/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestIntegration_JWT_ListTransactions(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	token := registerAndLogin(t, app)

	req, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/v1/transactions?page=1&page_size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, float64(0), data["total"])
}

func TestIntegration_HMAC_PaymentEndToEnd(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	// Register and get keys
	accessKey, secretKey := registerAndGetKeys(t, app)
	token := loginAndGetToken(t, app, "hmac_merchant", "StrongPass123!")

	// First topup via JWT
	topupBody, _ := json.Marshal(map[string]interface{}{
		"amount":   int64(1000000),
		"currency": "VND",
	})
	reqTopup, _ := http.NewRequest(http.MethodPost, app.server.URL+"/api/v1/wallets/topup", bytes.NewReader(topupBody))
	reqTopup.Header.Set("Content-Type", "application/json")
	reqTopup.Header.Set("Authorization", "Bearer "+token)
	respTopup, err := http.DefaultClient.Do(reqTopup)
	require.NoError(t, err)
	defer respTopup.Body.Close()
	assert.Equal(t, http.StatusCreated, respTopup.StatusCode)

	// Payment via HMAC
	payBody, _ := json.Marshal(map[string]interface{}{
		"reference_id": "order-001",
		"amount":       int64(50000),
		"currency":     "VND",
	})
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := "unique-nonce-001"

	// Build canonical string
	canonical := fmt.Sprintf("POST|/api/v1/payments|%s|%s|%s", timestamp, nonce, string(payBody))
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))

	reqPay, _ := http.NewRequest(http.MethodPost, app.server.URL+"/api/v1/payments", bytes.NewReader(payBody))
	reqPay.Header.Set("Content-Type", "application/json")
	reqPay.Header.Set("X-Merchant-Access-Key", accessKey)
	reqPay.Header.Set("X-Signature", signature)
	reqPay.Header.Set("X-Timestamp", timestamp)
	reqPay.Header.Set("X-Nonce", nonce)

	respPay, err := http.DefaultClient.Do(reqPay)
	require.NoError(t, err)
	defer respPay.Body.Close()

	payBodyResp, _ := io.ReadAll(respPay.Body)
	require.Equal(t, http.StatusCreated, respPay.StatusCode, "payment response: %s", string(payBodyResp))

	var payResp map[string]interface{}
	require.NoError(t, json.Unmarshal(payBodyResp, &payResp))
	payData := payResp["data"].(map[string]interface{})
	assert.Equal(t, "PAYMENT", payData["transaction_type"])
	assert.Equal(t, "SUCCESS", payData["status"])
	assert.Equal(t, float64(50000), payData["amount"])

	// Check balance reduced
	reqBal, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/v1/wallets/balance", nil)
	reqBal.Header.Set("Authorization", "Bearer "+token)
	respBal, err := http.DefaultClient.Do(reqBal)
	require.NoError(t, err)
	defer respBal.Body.Close()

	var balResp map[string]interface{}
	require.NoError(t, json.NewDecoder(respBal.Body).Decode(&balResp))
	balData := balResp["data"].(map[string]interface{})
	assert.Equal(t, float64(950000), balData["balance"])
}

func TestIntegration_HMAC_MissingHeaders(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	resp, err := http.Post(app.server.URL+"/api/v1/payments", "application/json", bytes.NewReader([]byte("{}")))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_JWT_Unauthorized(t *testing.T) {
	app := newTestApp(t)
	defer app.close()

	req, _ := http.NewRequest(http.MethodGet, app.server.URL+"/api/v1/wallets/balance", nil)
	// No Authorization header
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Helpers ---

func registerAndLogin(t *testing.T, app *testApp) string {
	t.Helper()
	regBody, _ := json.Marshal(map[string]string{
		"username":      "testmerchant",
		"password":      "StrongPass123!",
		"merchant_name": "Test",
	})
	resp, err := http.Post(app.server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	resp.Body.Close()

	return loginAndGetToken(t, app, "testmerchant", "StrongPass123!")
}

func loginAndGetToken(t *testing.T, app *testApp, username, password string) string {
	t.Helper()
	loginBody, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := http.Post(app.server.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var loginResp map[string]interface{}
	require.NoError(t, json.Unmarshal(bodyBytes, &loginResp))
	data := loginResp["data"].(map[string]interface{})
	return data["token"].(string)
}

func registerAndGetKeys(t *testing.T, app *testApp) (accessKey, secretKey string) {
	t.Helper()
	regBody, _ := json.Marshal(map[string]string{
		"username":      "hmac_merchant",
		"password":      "StrongPass123!",
		"merchant_name": "HMAC Test",
	})
	resp, err := http.Post(app.server.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var regResp map[string]interface{}
	require.NoError(t, json.Unmarshal(bodyBytes, &regResp))
	data := regResp["data"].(map[string]interface{})
	return data["access_key"].(string), data["secret_key"].(string)
}
