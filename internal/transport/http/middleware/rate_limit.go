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

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	ips map[string]*client
	mu  sync.RWMutex
	r   rate.Limit
	b   int
	rm  *RequestMetrics

	cleanupInterval time.Duration
	cleanupTTL      time.Duration
}

func NewIPRateLimiter(ctx context.Context, r rate.Limit, b int, rm *RequestMetrics) *IPRateLimiter {
	return NewIPRateLimiterWithCustomCleanup(ctx, r, b, rm, time.Minute, 3*time.Minute)
}

func NewIPRateLimiterWithCustomCleanup(ctx context.Context, r rate.Limit, b int, rm *RequestMetrics, interval, ttl time.Duration) *IPRateLimiter {
	i := &IPRateLimiter{
		ips:             make(map[string]*client),
		r:               r,
		b:               b,
		rm:              rm,
		cleanupInterval: interval,
		cleanupTTL:      ttl,
	}

	go i.cleanup(ctx)

	return i
}

func (i *IPRateLimiter) cleanup(ctx context.Context) {
	for {
		select {
		case <-time.After(i.cleanupInterval):
		case <-ctx.Done():
			return
		}

		i.mu.Lock()
		for ip, c := range i.ips {
			if time.Since(c.lastSeen) > i.cleanupTTL {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

func (i *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	c, exists := i.ips[ip]
	if !exists {
		c = &client{limiter: rate.NewLimiter(i.r, i.b)}
		i.ips[ip] = c
	}

	c.lastSeen = time.Now()
	return c.limiter
}

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

func (i *IPRateLimiter) GetIPCount() float64 {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return float64(len(i.ips))
}
