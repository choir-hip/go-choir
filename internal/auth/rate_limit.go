package auth

import (
	"net/http"
	"sync"
	"time"
)

const (
	// AuthEndpointRateLimit is the maximum number of auth requests allowed per IP in one window.
	AuthEndpointRateLimit = 10

	// AuthEndpointRateWindow is the window used by the global auth endpoint limiter.
	AuthEndpointRateWindow = time.Minute
)

type authRateBucket struct {
	resetAt time.Time
	count   int
}

// IPRateLimiter limits auth endpoint traffic by client IP.
type IPRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	buckets map[string]authRateBucket
}

// NewIPRateLimiter creates an in-memory per-IP fixed-window rate limiter.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	if limit <= 0 {
		limit = AuthEndpointRateLimit
	}
	if window <= 0 {
		window = AuthEndpointRateWindow
	}
	return &IPRateLimiter{
		limit:   limit,
		window:  window,
		buckets: make(map[string]authRateBucket),
	}
}

// Wrap applies the limiter before invoking next.
func (l *IPRateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now().UTC()) {
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many auth requests from this address"})
			return
		}
		next(w, r)
	}
}

func (l *IPRateLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[ip]
	if !ok || !now.Before(bucket.resetAt) {
		l.buckets[ip] = authRateBucket{
			resetAt: now.Add(l.window),
			count:   1,
		}
		l.prune(now)
		return true
	}
	if bucket.count >= l.limit {
		return false
	}
	bucket.count++
	l.buckets[ip] = bucket
	return true
}

func (l *IPRateLimiter) prune(now time.Time) {
	for ip, bucket := range l.buckets {
		if !now.Before(bucket.resetAt) {
			delete(l.buckets, ip)
		}
	}
}
