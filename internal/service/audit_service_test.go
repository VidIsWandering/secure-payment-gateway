package service

import (
"context"
"testing"
"time"

"secure-payment-gateway/internal/core/domain"
"secure-payment-gateway/internal/core/ports/mocks"

"github.com/google/uuid"
"go.uber.org/mock/gomock"
)

func TestAuditService_Log_PersistsToRepo(t *testing.T) {
ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockRepo := mocks.NewMockAuditRepository(ctrl)
svc := NewAuditService(mockRepo, newTestLogger())

done := make(chan struct{})
mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
func(ctx context.Context, log *domain.AuditLog) error {
if log.Action != domain.AuditActionPayment {
t.Errorf("expected PAYMENT, got %s", log.Action)
}
close(done)
return nil
},
)

merchantID := uuid.New()
svc.Log(context.Background(), &domain.AuditLog{
ID:           uuid.New(),
MerchantID:   &merchantID,
Action:       domain.AuditActionPayment,
ResourceType: "transaction",
ResourceID:   uuid.New().String(),
IPAddress:    "127.0.0.1",
CreatedAt:    time.Now(),
})

select {
case <-done:
// OK
case <-time.After(2 * time.Second):
t.Fatal("audit log not persisted in time")
}
}

func TestAuditService_Log_NilRepo(t *testing.T) {
svc := NewAuditService(nil, newTestLogger())

merchantID := uuid.New()
// Should not panic
svc.Log(context.Background(), &domain.AuditLog{
ID:           uuid.New(),
MerchantID:   &merchantID,
Action:       domain.AuditActionLogin,
ResourceType: "session",
IPAddress:    "127.0.0.1",
CreatedAt:    time.Now(),
})

time.Sleep(50 * time.Millisecond) // let goroutine run
}
