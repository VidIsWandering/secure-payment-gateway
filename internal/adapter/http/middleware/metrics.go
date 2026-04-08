package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "spg_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "spg_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "spg_http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)

	paymentTransactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "spg_payment_transactions_total",
			Help: "Total number of payment transactions processed",
		},
		[]string{"type", "status"},
	)

	webhookDeliveriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "spg_webhook_deliveries_total",
			Help: "Total number of webhook delivery attempts",
		},
		[]string{"status"},
	)
)

// PrometheusMetrics returns a middleware that records HTTP metrics for Prometheus.
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpRequestsInFlight.Inc()
		start := time.Now()

		c.Next()

		httpRequestsInFlight.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// Normalize path to avoid high cardinality from path params
		path := normalizePath(c)

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// RecordTransaction records a payment transaction metric.
func RecordTransaction(txType, status string) {
	paymentTransactionsTotal.WithLabelValues(txType, status).Inc()
}

// RecordWebhookDelivery records a webhook delivery metric.
func RecordWebhookDelivery(status string) {
	webhookDeliveriesTotal.WithLabelValues(status).Inc()
}

// normalizePath returns a sanitized path pattern to prevent high-cardinality labels.
func normalizePath(c *gin.Context) string {
	// Use matched route template if available (e.g., "/api/v1/payments/:id/status")
	if route := c.FullPath(); route != "" {
		return route
	}
	return c.Request.URL.Path
}
