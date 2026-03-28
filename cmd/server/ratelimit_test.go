package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRateLimit_WithinBurst(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(10), 5)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(okHandler))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:9999"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimit_ExceedsBurst_Returns429(t *testing.T) {
	// Very low limit — burst of 1 and near-zero rate so any second request is denied
	limiter := NewIPRateLimiter(rate.Limit(0.001), 1)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(okHandler))

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	// Second request should be rate-limited
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "1.2.3.4:9999"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rr2.Code)
	}
}

func TestRateLimit_429_HasRetryAfterHeader(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(0.001), 1)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(okHandler))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.5.5.5:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if i == 1 {
			if rr.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d", rr.Code)
			}
			if rr.Header().Get("Retry-After") == "" {
				t.Error("expected Retry-After header on 429 response")
			}
			var body map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode 429 body: %v", err)
			}
			if body["error"] == "" {
				t.Error("expected error message in 429 response body")
			}
		}
	}
}

func TestRateLimit_DifferentIPs_IndependentLimits(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(0.001), 1)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(okHandler))

	ips := []string{"10.0.0.1:1", "10.0.0.2:1", "10.0.0.3:1"}
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("IP %s: expected 200, got %d", ip, rr.Code)
		}
	}
}

func TestRateLimit_XForwardedFor(t *testing.T) {
	limiter := NewIPRateLimiter(rate.Limit(0.001), 1)
	handler := RateLimitMiddleware(limiter)(http.HandlerFunc(okHandler))

	// First request via proxy
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	req1.RemoteAddr = "10.0.0.1:80"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr1.Code)
	}

	// Same real IP again → rate limited
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	req2.RemoteAddr = "10.0.0.1:80"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr2.Code)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:54321"
	if got := clientIP(req); got != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", got)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	req.RemoteAddr = "10.0.0.1:80"
	if got := clientIP(req); got != "203.0.113.5" {
		t.Errorf("expected 203.0.113.5, got %s", got)
	}
}
