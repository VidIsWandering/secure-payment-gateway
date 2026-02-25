package handler

import (
"math"
"strconv"

"secure-payment-gateway/internal/adapter/http/dto"
"secure-payment-gateway/internal/adapter/http/middleware"
"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports"
"secure-payment-gateway/pkg/apperror"
"secure-payment-gateway/pkg/response"

"github.com/gin-gonic/gin"
"github.com/google/uuid"
)

// DashboardHandler handles dashboard & transaction list endpoints.
type DashboardHandler struct {
reportingSvc ports.ReportingService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(reportingSvc ports.ReportingService) *DashboardHandler {
return &DashboardHandler{reportingSvc: reportingSvc}
}

// GetStats handles GET /api/v1/dashboard/stats.
func (h *DashboardHandler) GetStats(c *gin.Context) {
merchantID, ok := c.Get(middleware.CtxMerchantID)
if !ok {
response.Error(c, apperror.ErrInvalidToken())
return
}

period := c.DefaultQuery("period", "all")
stats, err := h.reportingSvc.GetDashboardStats(c.Request.Context(), merchantID.(uuid.UUID), period)
if err != nil {
response.Error(c, err)
return
}

response.OK(c, dto.DashboardStatsResponse{
TotalTransactions: stats.TotalTransactions,
Successful:        stats.Successful,
Failed:            stats.Failed,
Reversed:          stats.Reversed,
TotalRevenue:      stats.TotalRevenue,
TotalRefunded:     stats.TotalRefunded,
TotalTopup:        stats.TotalTopup,
})
}

// ListTransactions handles GET /api/v1/transactions.
func (h *DashboardHandler) ListTransactions(c *gin.Context) {
merchantID, ok := c.Get(middleware.CtxMerchantID)
if !ok {
response.Error(c, apperror.ErrInvalidToken())
return
}

page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
if page < 1 {
page = 1
}
if pageSize < 1 || pageSize > 100 {
pageSize = 20
}

params := ports.TransactionListParams{
MerchantID: merchantID.(uuid.UUID),
Page:       page,
PageSize:   pageSize,
}

if s := c.Query("status"); s != "" {
status := domain.TransactionStatus(s)
params.Status = &status
}
if t := c.Query("type"); t != "" {
txType := domain.TransactionType(t)
params.Type = &txType
}
if f := c.Query("from"); f != "" {
if v, err := strconv.ParseInt(f, 10, 64); err == nil {
params.From = &v
}
}
if t := c.Query("to"); t != "" {
if v, err := strconv.ParseInt(t, 10, 64); err == nil {
params.To = &v
}
}

txns, total, err := h.reportingSvc.ListTransactions(c.Request.Context(), params)
if err != nil {
response.Error(c, err)
return
}

items := make([]dto.TransactionResponse, 0, len(txns))
for i := range txns {
items = append(items, toTransactionResponse(&txns[i]))
}

totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

response.OK(c, dto.TransactionListResponse{
Items:      items,
Total:      total,
Page:       page,
PageSize:   pageSize,
TotalPages: totalPages,
})
}
