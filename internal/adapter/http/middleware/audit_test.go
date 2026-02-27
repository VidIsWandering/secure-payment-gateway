package middleware

import (
"net/http"
"net/http/httptest"
"testing"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports/mocks"

"github.com/gin-gonic/gin"
"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"go.uber.org/mock/gomock"
"context"
)

func TestAuditLog_PaymentSuccess(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockAudit := mocks.NewMockAuditService(ctrl)

done := make(chan struct{})
mockAudit.EXPECT().Log(gomock.Any(), gomock.Any()).DoAndReturn(
func(ctx context.Context, log *domain.AuditLog) {
assert.Equal(t, domain.AuditActionPayment, log.Action)
assert.Equal(t, "transaction", log.ResourceType)
close(done)
},
)

r := gin.New()
r.Use(AuditLog(mockAudit))
r.POST("/api/v1/payments", func(c *gin.Context) {
c.Set(CtxMerchantID, uuid.New())
c.JSON(http.StatusCreated, gin.H{"ok": true})
})

w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", nil)
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusCreated, w.Code)

select {
case <-done:
case <-time.After(time.Second):
t.Fatal("audit not called")
}
}

func TestAuditLog_SkipsGET(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockAudit := mocks.NewMockAuditService(ctrl)
// No expectations - Log should NOT be called for GET

r := gin.New()
r.Use(AuditLog(mockAudit))
r.GET("/api/v1/wallets/balance", func(c *gin.Context) {
c.JSON(http.StatusOK, gin.H{"balance": 100})
})

w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/api/v1/wallets/balance", nil)
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuditLog_SkipsFailedRequests(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockAudit := mocks.NewMockAuditService(ctrl)
// No expectations - Log should NOT be called for 4xx

r := gin.New()
r.Use(AuditLog(mockAudit))
r.POST("/api/v1/payments", func(c *gin.Context) {
c.JSON(http.StatusBadRequest, gin.H{"error": "bad"})
})

w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", nil)
r.ServeHTTP(w, req)

assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMapPathToAction(t *testing.T) {
tests := []struct {
path     string
method   string
action   domain.AuditAction
resource string
}{
{"/api/v1/auth/register", "POST", domain.AuditActionRegister, "merchant"},
{"/api/v1/auth/login", "POST", domain.AuditActionLogin, "session"},
{"/api/v1/payments", "POST", domain.AuditActionPayment, "transaction"},
{"/api/v1/payments/refund", "POST", domain.AuditActionRefund, "transaction"},
{"/api/v1/wallets/topup", "POST", domain.AuditActionTopup, "wallet"},
{"/api/v1/merchants/me/webhook", "PUT", domain.AuditActionUpdateWebhook, "merchant"},
{"/api/v1/merchants/me/rotate-keys", "POST", domain.AuditActionRotateKeys, "merchant"},
{"/unknown", "POST", "", ""},
}

for _, tc := range tests {
action, resource := mapPathToAction(tc.path, tc.method)
assert.Equal(t, tc.action, action, "path=%s method=%s", tc.path, tc.method)
assert.Equal(t, tc.resource, resource, "path=%s method=%s", tc.path, tc.method)
}
}
