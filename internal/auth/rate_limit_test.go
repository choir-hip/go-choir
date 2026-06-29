package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiterBlocksRequestsAfterLimit(t *testing.T) {
	// Given
	limiter := NewIPRateLimiter(2, time.Minute)
	calls := 0
	handler := limiter.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}))

	// When
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
		req.RemoteAddr = "203.0.113.7:4000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status: got %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", nil)
	req.RemoteAddr = "203.0.113.7:4000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Then
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status: got %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if calls != 2 {
		t.Fatalf("handler calls: got %d, want 2", calls)
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestClientIPUsesProxyAppendedForwardedAddress(t *testing.T) {
	// Given
	limiter := NewIPRateLimiter(1, time.Minute)
	calls := 0
	handler := limiter.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", nil)
	req1.RemoteAddr = "127.0.0.1:5000"
	req1.Header.Set("X-Forwarded-For", "198.51.100.10, 203.0.113.8")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status: got %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", nil)
	req2.RemoteAddr = "127.0.0.1:5001"
	req2.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.8")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Then
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second status: got %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
	if calls != 1 {
		t.Fatalf("handler calls: got %d, want 1", calls)
	}
}
