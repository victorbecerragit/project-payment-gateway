package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{w, http.StatusOK}
}

func (lrw *responseWriterWrapper) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

type limiterInfo struct {
	name    string
	limiter *IPRateLimiter
}

type UniqueIPsCollector struct {
	limiters    []limiterInfo
	ipCountDesc *prometheus.Desc
}

func NewUniqueIPsCollector() *UniqueIPsCollector {
	return &UniqueIPsCollector{
		ipCountDesc: prometheus.NewDesc(
			"rate_limiter_unique_ips_total",
			"Total number of unique IP addresses currently tracked by the rate limiter.",
			[]string{"limiter_name"},
			nil,
		),
	}
}

func (c *UniqueIPsCollector) AddLimiter(limiter *IPRateLimiter, name string) {
	c.limiters = append(c.limiters, limiterInfo{name: name, limiter: limiter})
}

type RequestMetrics struct {
	requestsTotal        *prometheus.CounterVec
	requestDuration      *prometheus.HistogramVec
	droppedRequestsTotal *prometheus.CounterVec
	uniqueIPsCollector   *UniqueIPsCollector
}

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
				Buckets: prometheus.DefBuckets,
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

func (rm *RequestMetrics) AddLimiter(limiter *IPRateLimiter, name string) {
	if rm.uniqueIPsCollector == nil {
		rm.uniqueIPsCollector = NewUniqueIPsCollector()
	}
	rm.uniqueIPsCollector.AddLimiter(limiter, name)
}

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
