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

// limiterInfo holds a reference to an IPRateLimiter and its name.
type limiterInfo struct {
	name    string
	limiter *IPRateLimiter
}

// UniqueIPsCollector collects metrics about unique IP addresses in rate limiters.
type UniqueIPsCollector struct {
	limiters    []limiterInfo // Slice of rate limiters to monitor
	ipCountDesc *prometheus.Desc
}

// NewUniqueIPsCollector creates a new UniqueIPsCollector.
func NewUniqueIPsCollector() *UniqueIPsCollector {
	return &UniqueIPsCollector{
		ipCountDesc: prometheus.NewDesc(
			"rate_limiter_unique_ips_total",
			"Total number of unique IP addresses currently tracked by the rate limiter.",
			[]string{"limiter_name"}, // Label to distinguish different limiters (e.g., "api", "webhook")
			nil,
		),
	}
}

// AddLimiter adds an IPRateLimiter to the collector with a given name.
func (c *UniqueIPsCollector) AddLimiter(limiter *IPRateLimiter, name string) {
	c.limiters = append(c.limiters, limiterInfo{name: name, limiter: limiter})
}

// RequestMetrics holds Prometheus metrics for HTTP requests.
type RequestMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	droppedRequestsTotal *prometheus.CounterVec
	uniqueIPsCollector   *UniqueIPsCollector // Reference to the custom collector
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
		droppedRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_dropped_total",
				Help: "Total number of HTTP requests dropped by the rate limiter.",
			},
			[]string{"path"},
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