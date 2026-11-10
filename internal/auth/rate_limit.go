package auth

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// AuthEndpointRateLimit is the maximum number of requests the recovery and
	// email routes allow per IP in one window. It is the original single
	// auth limit, kept exactly for the routes that send email or could
	// enumerate accounts.
	AuthEndpointRateLimit = 10

	// AuthEndpointRateWindow is the window used by every auth limiter.
	AuthEndpointRateWindow = time.Minute

	// AuthReadRateLimit covers cookie-authenticated reads of one's own state
	// (session, account list, credential and session lists, root status).
	// Measured cost: a page load is 1 to 2 session reads, a focus is 1, a
	// sign-in or switch is up to 5; several tabs and people behind one
	// address fit in 300 a minute.
	AuthReadRateLimit = 300

	// AuthActRateLimit covers authenticated session acts (switch, signout,
	// logout, credential and key changes, device approve). A worst-case act
	// is 4 requests, so 60 a minute is about 15 acts.
	AuthActRateLimit = 60

	// AuthBeginRateLimit covers /auth/login/begin and /auth/register/begin
	// only. They answer differently for a known and an unknown email (an
	// account-existence oracle) and register/begin for a new email writes
	// permanent rows, so they keep the original 10 a minute. A sign-in costs
	// 2 ceremony requests (begin, finish; session reads are in the read
	// class), so 10 a minute still allows about 5 sign-ins.
	AuthBeginRateLimit = 10

	// AuthCeremonyRateLimit covers the other unauthenticated ceremony routes
	// (login and register finish, desktop exchange-redirect and redeem, device
	// begin and lookup). The passkey challenge is the brute-force defense;
	// this only bounds the volume.
	AuthCeremonyRateLimit = 30
)

// RateClass names a cost class of auth routes. Each class has its own
// limiter, so exhausting one never refuses another.
type RateClass int

const (
	// RateClassUnset is the zero value; a route without a class is a bug.
	RateClassUnset RateClass = iota
	// RateClassRead is a cookie-authenticated read of one's own state.
	RateClassRead
	// RateClassAct is an authenticated session act.
	RateClassAct
	// RateClassBegin is login/begin and register/begin: account-existence
	// oracles, 10 a minute.
	RateClassBegin
	// RateClassCeremony is the other unauthenticated ceremony routes.
	RateClassCeremony
	// RateClassSensitive is the recovery and email routes: the original
	// 10-a-minute limiter, unchanged.
	RateClassSensitive
	// RateClassDeviceToken is the device-token poll (slow_down paces it).
	RateClassDeviceToken
)

func (c RateClass) String() string {
	switch c {
	case RateClassRead:
		return "read"
	case RateClassAct:
		return "act"
	case RateClassBegin:
		return "begin"
	case RateClassCeremony:
		return "ceremony"
	case RateClassSensitive:
		return "sensitive"
	case RateClassDeviceToken:
		return "device_token"
	}
	return "unset"
}

// RateLimiters holds one limiter per class.
type RateLimiters struct {
	byClass map[RateClass]*IPRateLimiter
}

// NewRateLimiters builds the production limiter set from the constants.
func NewRateLimiters() *RateLimiters {
	return NewRateLimitersWithLimits(map[RateClass]int{
		RateClassRead:        AuthReadRateLimit,
		RateClassAct:         AuthActRateLimit,
		RateClassBegin:       AuthBeginRateLimit,
		RateClassCeremony:    AuthCeremonyRateLimit,
		RateClassSensitive:   AuthEndpointRateLimit,
		RateClassDeviceToken: DeviceTokenRateLimit,
	})
}

// NewRateLimitersWithLimits builds a limiter set with the given per-class
// limits (tests use small ones). Every class must be present.
func NewRateLimitersWithLimits(limits map[RateClass]int) *RateLimiters {
	rl := &RateLimiters{byClass: make(map[RateClass]*IPRateLimiter)}
	for _, c := range []RateClass{RateClassRead, RateClassAct, RateClassBegin, RateClassCeremony, RateClassSensitive, RateClassDeviceToken} {
		n, ok := limits[c]
		if !ok {
			panic("auth: no rate limit for class " + c.String())
		}
		l := NewIPRateLimiter(n, AuthEndpointRateWindow)
		l.class = c.String()
		rl.byClass[c] = l
	}
	return rl
}

type authRateBucket struct {
	resetAt time.Time
	count   int
}

// IPRateLimiter limits auth endpoint traffic by client IP.
type IPRateLimiter struct {
	mu      sync.Mutex
	class   string
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
		class:   "unclassed",
		limit:   limit,
		window:  window,
		buckets: make(map[string]authRateBucket),
	}
}

// Wrap applies the limiter before invoking next. A refusal sets Retry-After
// and writes one log line. Production routes use Handler.Instrument, which
// also writes the access line.
func (l *IPRateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ok, retry, count := l.allow(ip, time.Now().UTC())
		if !ok {
			secs := writeRateLimited(w, retry)
			log.Printf("auth: rate_limited class=%s addr=%s count=%d retry_after=%d", l.class, addrPrefix(ip), count, secs)
			return
		}
		next(w, r)
	}
}

// writeRateLimited writes the 429 with Retry-After (whole seconds until the
// window frees) and returns those seconds.
func writeRateLimited(w http.ResponseWriter, retry time.Duration) int {
	secs := int((retry + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":       "too many auth requests from this address",
		"retry_after": secs,
	})
	return secs
}

// addrPrefix is the only form of a client address the logs carry: the first
// two IPv4 octets or the first 32 bits of an IPv6 address.
func addrPrefix(ip string) string {
	p := net.ParseIP(ip)
	if p == nil {
		return "unknown"
	}
	if v4 := p.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.x.x", v4[0], v4[1])
	}
	parts := strings.Split(p.String(), ":")
	if len(parts) < 2 {
		return "unknown"
	}
	return parts[0] + ":" + parts[1] + "::/32"
}

// allow reports whether the request fits the window. On refusal it returns
// the time until the window frees and the attempts counted so far.
func (l *IPRateLimiter) allow(ip string, now time.Time) (bool, time.Duration, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[ip]
	if !ok || !now.Before(bucket.resetAt) {
		l.buckets[ip] = authRateBucket{
			resetAt: now.Add(l.window),
			count:   1,
		}
		l.prune(now)
		return true, 0, 1
	}
	if bucket.count >= l.limit {
		bucket.count++
		l.buckets[ip] = bucket
		return false, bucket.resetAt.Sub(now), bucket.count
	}
	bucket.count++
	l.buckets[ip] = bucket
	return true, 0, bucket.count
}

func (l *IPRateLimiter) prune(now time.Time) {
	for ip, bucket := range l.buckets {
		if !now.Before(bucket.resetAt) {
			delete(l.buckets, ip)
		}
	}
}
