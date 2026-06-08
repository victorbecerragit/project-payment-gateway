package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// responseWriterWrapper wraps http.ResponseWriter to capture the status code.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{w, http.StatusOK} // Default status code
}

func (lrw *responseWriterWrapper) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// RequestMetrics holds Prometheus metrics for HTTP requests.
type RequestMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// NewRequestMetrics initializes and registers Prometheus metrics.
func NewRequestMetrics() *RequestMetrics {
	return &RequestMetrics{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status_code"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds.",
				Buckets: prometheus.DefBuckets, // Default buckets: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
			},
			[]string{"method", "path", "status_code"},
		),
	}
}

// MetricsMiddleware returns an http.Handler middleware that tracks request counts and latency.
// It takes a 'pattern' string to ensure low-cardinality labels for dynamic routes.
func (rm *RequestMetrics) MetricsMiddleware(pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrappedWriter := newResponseWriterWrapper(w)

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(wrappedWriter.statusCode)

		rm.requestsTotal.WithLabelValues(r.Method, pattern, statusCode).Inc()
		rm.requestDuration.WithLabelValues(r.Method, pattern, statusCode).Observe(duration)
	})
}