package dto

// RegisterRequest is the request body for merchant registration.
type RegisterRequest struct {
Username     string  `json:"username" binding:"required,min=3,max=50"`
Password     string  `json:"password" binding:"required,min=8,max=128"`
MerchantName string  `json:"merchant_name" binding:"required,min=1,max=100"`
WebhookURL   *string `json:"webhook_url,omitempty"`
}

// LoginRequest is the request body for merchant login.
type LoginRequest struct {
Username string `json:"username" binding:"required"`
Password string `json:"password" binding:"required"`
}

// RegisterResponse is the response body for successful registration.
type RegisterResponse struct {
MerchantID string `json:"merchant_id"`
AccessKey  string `json:"access_key"`
SecretKey  string `json:"secret_key"`
}

// LoginResponse is the response body for successful login.
type LoginResponse struct {
Token  string `json:"token"`
Expiry int64  `json:"expiry"` // Unix timestamp
}

// PaymentRequest is the request body for payment processing.
type PaymentRequest struct {
ReferenceID string  `json:"reference_id" binding:"required,max=100"`
Amount      int64   `json:"amount" binding:"required,gt=0"`
Currency    string  `json:"currency" binding:"required,len=3"`
ExtraData   *string `json:"extra_data,omitempty"`
}

// RefundRequest is the request body for refund processing.
type RefundRequest struct {
OriginalReferenceID string `json:"original_reference_id" binding:"required"`
Amount              *int64 `json:"amount,omitempty"`
Reason              string `json:"reason" binding:"required"`
}

// TopupRequest is the request body for wallet topup.
type TopupRequest struct {
Amount   int64  `json:"amount" binding:"required,gt=0"`
Currency string `json:"currency" binding:"required,len=3"`
}

// TransactionResponse is the response body for transaction results.
type TransactionResponse struct {
ID              string  `json:"id"`
ReferenceID     string  `json:"reference_id"`
Amount          int64   `json:"amount"`
TransactionType string  `json:"transaction_type"`
Status          string  `json:"status"`
CreatedAt       string  `json:"created_at"`
ProcessedAt     *string `json:"processed_at,omitempty"`
}

// WalletBalanceResponse is the response for balance query.
type WalletBalanceResponse struct {
Balance  int64  `json:"balance"`
Currency string `json:"currency"`
}

// DashboardStatsResponse is the response for dashboard statistics.
type DashboardStatsResponse struct {
TotalTransactions int64 `json:"total_transactions"`
Successful        int64 `json:"successful"`
Failed            int64 `json:"failed"`
Reversed          int64 `json:"reversed"`
TotalRevenue      int64 `json:"total_revenue"`
TotalRefunded     int64 `json:"total_refunded"`
TotalTopup        int64 `json:"total_topup"`
}

// TransactionListResponse wraps paginated transaction list.
type TransactionListResponse struct {
Items      []TransactionResponse `json:"items"`
Total      int64                 `json:"total"`
Page       int                   `json:"page"`
PageSize   int                   `json:"page_size"`
TotalPages int                   `json:"total_pages"`
}
