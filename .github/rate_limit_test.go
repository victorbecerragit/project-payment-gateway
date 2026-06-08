package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"golang.org/x/time/rate"
)

func TestIPRateLimiter_Cleanup(t *testing.T) {
	// Use very short durations to speed up the test
	interval := 50 * time.Millisecond
	ttl := 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl := NewIPRateLimiterWithCustomCleanup(ctx, rate.Limit(100), 100, nil, interval, ttl)

	ip := "1.2.3.4"
	// Add an IP
	rl.getLimiter(ip)

	// Verify it exists in the map
	rl.mu.RLock()
	_, exists := rl.ips[ip]
	rl.mu.RUnlock()
	if !exists {
		t.Fatal("expected IP to exist in map after access")
	}

	// Wait for cleanup to run (interval + ttl + some buffer)
	time.Sleep(interval + ttl + 20*time.Millisecond)

	// Verify it has been removed
	rl.mu.RLock()
	_, exists = rl.ips[ip]
	rl.mu.RUnlock()
	if exists {
		t.Error("expected IP to be removed from map after TTL expired")
	}
}

func TestIPRateLimiter_ActivityResetsTTL(t *testing.T) {
	interval := 50 * time.Millisecond
	ttl := 150 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl := NewIPRateLimiterWithCustomCleanup(ctx, rate.Limit(100), 100, nil, interval, ttl)
	ip := "5.6.7.8"

	rl.getLimiter(ip)

	// Perform activity just before the first TTL would have expired
	time.Sleep(100 * time.Millisecond)
	rl.getLimiter(ip)

	// Wait for the original TTL to pass
	time.Sleep(70 * time.Millisecond)

	// Verify it still exists because the second getLimiter call updated lastSeen
	rl.mu.RLock()
	_, exists := rl.ips[ip]
	rl.mu.RUnlock()
	if !exists {
		t.Error("expected IP to still exist because activity should have reset TTL")
	}
}

func TestIPRateLimiter_GetLimiterUpdatesLastSeen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rl := NewIPRateLimiterWithCustomCleanup(ctx, rate.Limit(1), 1, nil, time.Hour, time.Hour)
	ip := "9.10.11.12"

	rl.getLimiter(ip)
	t1 := rl.ips[ip].lastSeen

	time.Sleep(10 * time.Millisecond)
	rl.getLimiter(ip)
	t2 := rl.ips[ip].lastSeen

	if !t2.After(t1) {
		t.Error("getLimiter should update lastSeen timestamp on every call")
	}
}

func TestIPRateLimiter_HandlerThrottling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rm := NewRequestMetrics()
	// Configure a limiter that allows exactly 1 request and never refills (Rate 0)
	rl := NewIPRateLimiterWithCustomCleanup(ctx, rate.Limit(0), 1, rm, time.Hour, time.Hour)

	pattern := "/api/v1/payments"
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rl.Handler(pattern, nextHandler)

	// Request 1: Should be allowed via the initial burst
	req1 := httptest.NewRequest(http.MethodPost, "http://localhost"+pattern, nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request should have been allowed, got %d", rr1.Code)
	}

	// Request 2: Same IP, should be throttled (429)
	req2 := httptest.NewRequest(http.MethodPost, "http://localhost"+pattern, nil)
	req2.RemoteAddr = "192.168.1.1:5678"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("second request should have been throttled (429), got %d", rr2.Code)
	}

	// Verify the 'droppedRequestsTotal' metric was incremented exactly once for this path
	droppedCount := testutil.ToFloat64(rm.droppedRequestsTotal.WithLabelValues(pattern))
	if droppedCount != 1 {
		t.Errorf("expected droppedRequestsTotal to be 1, got %f", droppedCount)
	}
}