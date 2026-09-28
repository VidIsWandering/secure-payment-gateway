package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/VidIsWandering/secure-payment-gateway/internal/core/domain"
	"github.com/VidIsWandering/secure-payment-gateway/internal/core/ports/mocks"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestInstrumentWebhookRepository_CountsTerminalStatusesOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	inner := mocks.NewMockWebhookRepository(ctrl)
	repo := InstrumentWebhookRepository(inner)

	delivered := webhookDeliveriesTotal.WithLabelValues(string(domain.WebhookStatusDelivered))
	failed := webhookDeliveriesTotal.WithLabelValues(string(domain.WebhookStatusFailed))
	pending := webhookDeliveriesTotal.WithLabelValues(string(domain.WebhookStatusPending))
	baseDelivered, baseFailed, basePending := testutil.ToFloat64(delivered), testutil.ToFloat64(failed), testutil.ToFloat64(pending)

	inner.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	inner.EXPECT().Update(gomock.Any(), gomock.Any()).Return(errors.New("db down"))

	ctx := context.Background()
	assert.NoError(t, repo.Update(ctx, &domain.WebhookDeliveryLog{Status: domain.WebhookStatusPending}))
	assert.NoError(t, repo.Update(ctx, &domain.WebhookDeliveryLog{Status: domain.WebhookStatusDelivered}))
	// Outcome is still counted when persisting the log fails.
	assert.Error(t, repo.Update(ctx, &domain.WebhookDeliveryLog{Status: domain.WebhookStatusFailed}))

	assert.Equal(t, basePending, testutil.ToFloat64(pending))
	assert.Equal(t, baseDelivered+1, testutil.ToFloat64(delivered))
	assert.Equal(t, baseFailed+1, testutil.ToFloat64(failed))
}
