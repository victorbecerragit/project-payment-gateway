package middleware

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/response"
	"golang.org/x/time/rate"
)

// client wraps the rate limiter with a timestamp to track activity.
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages a collection of rate limiters per IP address.
type IPRateLimiter struct {
	ips   map[string]*client
	mu    sync.RWMutex
	r     rate.Limit
	b     int
	rm    *RequestMetrics

	cleanupInterval time.Duration
	cleanupTTL      time.Duration
}

// NewIPRateLimiter creates a new limiter with a specific rate (req/s) and burst size.
func NewIPRateLimiter(ctx context.Context, r rate.Limit, b int, rm *RequestMetrics) *IPRateLimiter { // Corrected to use RequestMetrics
	return NewIPRateLimiterWithCustomCleanup(ctx, r, b, rm, time.Minute, 3*time.Minute)
}

// NewIPRateLimiterWithCustomCleanup creates a new limiter with custom cleanup durations.
// It is primarily used for testing purposes.
func NewIPRateLimiterWithCustomCleanup(ctx context.Context, r rate.Limit, b int, rm *RequestMetrics, interval, ttl time.Duration) *IPRateLimiter { // Corrected to use RequestMetrics
	i := &IPRateLimiter{
		ips:             make(map[string]*client),
		r:               r,
		b:               b,
		rm:              rm,
		cleanupInterval: interval,
		cleanupTTL:      ttl,
	}

	// Start background cleanup routine to prevent memory bloat
	go i.cleanup(ctx)

	return i
}

// cleanup periodically removes limiters that haven't been seen for a while.
func (i *IPRateLimiter) cleanup(ctx context.Context) {
	for {
		select {
		case <-time.After(i.cleanupInterval):
			// Continue with cleanup
		case <-ctx.Done():
			// Context cancelled, exit cleanup goroutine
			return
		}

		i.mu.Lock()
		for ip, c := range i.ips {
			// Remove limiters idle for more than the TTL
			if time.Since(c.lastSeen) > i.cleanupTTL {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

// getLimiter returns the rate limiter for the provided IP address, creating it if necessary.
func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	c, exists := i.ips[ip]
	if !exists {
		c = &client{
			limiter: rate.NewLimiter(i.r, i.b),
		}
		i.ips[ip] = c
	}

	c.lastSeen = time.Now()
	return c.limiter
}

// Handler returns a middleware that applies rate limiting to the provided next handler.
func (i *IPRateLimiter) Handler(pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if !i.getLimiter(ip).Allow() {
			if i.rm != nil {
				i.rm.droppedRequestsTotal.WithLabelValues(pattern).Inc()
			}
			response.RespondWithError(w, http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetIPCount returns the current number of unique IP addresses being tracked.
func (i *IPRateLimiter) GetIPCount() float64 {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return float64(len(i.ips))
}