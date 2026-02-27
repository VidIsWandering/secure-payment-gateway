package middleware

import (
"encoding/json"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"

"github.com/gin-gonic/gin"
"github.com/google/uuid"
)

// AuditLog creates an audit middleware that logs successful write operations.
// It maps HTTP methods and paths to audit actions.
func AuditLog(auditSvc ports.AuditService) gin.HandlerFunc {
return func(c *gin.Context) {
c.Next()

// Only audit successful write operations (status 2xx)
if c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
return
}
if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
return
}

action, resourceType := mapPathToAction(c.Request.URL.Path, c.Request.Method)
if action == "" {
return
}

var merchantID *uuid.UUID
if mid, exists := c.Get(CtxMerchantID); exists {
if id, ok := mid.(uuid.UUID); ok {
merchantID = &id
}
}

details, _ := json.Marshal(map[string]interface{}{
"method": c.Request.Method,
"path":   c.Request.URL.Path,
"status": c.Writer.Status(),
})

auditSvc.Log(c.Request.Context(), &domain.AuditLog{
ID:           uuid.New(),
MerchantID:   merchantID,
Action:       action,
ResourceType: resourceType,
IPAddress:    c.ClientIP(),
Details:      string(details),
CreatedAt:    time.Now(),
})
}
}

func mapPathToAction(path, method string) (domain.AuditAction, string) {
switch {
case path == "/api/v1/auth/register" && method == "POST":
return domain.AuditActionRegister, "merchant"
case path == "/api/v1/auth/login" && method == "POST":
return domain.AuditActionLogin, "session"
case path == "/api/v1/payments" && method == "POST":
return domain.AuditActionPayment, "transaction"
case path == "/api/v1/payments/refund" && method == "POST":
return domain.AuditActionRefund, "transaction"
case path == "/api/v1/wallets/topup" && method == "POST":
return domain.AuditActionTopup, "wallet"
case path == "/api/v1/merchants/me/webhook" && method == "PUT":
return domain.AuditActionUpdateWebhook, "merchant"
case path == "/api/v1/merchants/me/rotate-keys" && method == "POST":
return domain.AuditActionRotateKeys, "merchant"
}
return "", ""
}
