package auth

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// statusRecorder remembers the status a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Instrument is the one wrapper every /auth route is registered through. It
// applies the class limiter, then writes exactly one access-log line per
// request, whether the limiter refused it, the handler served it, or the
// handler panicked:
//
//	[auth] operation=http_access method=GET route="/auth/session" status=200
//	  ms=3 class=read count=4 limit=300 addr=203.0.x.x user=c061abcd sid=1a2b3c4d outcome=ok
//
// route is the registered pattern, never the URL. The line never carries an
// email, full address, cookie value, token, code, challenge or credential
// bytes, body, query string or User-Agent. user is the first 8 characters of
// the user id named by a valid access token; sid is the first 8 hex of the
// SHA-256 of the refresh cookie (non-reversible); both are "-" when absent.
func (h *Handler) Instrument(rl *RateLimiters, pattern string, c RateClass, next http.HandlerFunc) http.HandlerFunc {
	l, ok := rl.byClass[c]
	if !ok {
		panic("auth: route registered without a rate class")
	}
	route := pattern
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		route = pattern[i+1:]
	}
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := clientIP(r)
		rec := &statusRecorder{ResponseWriter: w}
		allowed, retry, count := l.allow(ip, start.UTC())
		defer func() {
			if p := recover(); p != nil {
				rec.status = http.StatusInternalServerError
				h.writeAccessLog(r, route, rec.status, start, l, count, ip)
				panic(p)
			}
			h.writeAccessLog(r, route, rec.status, start, l, count, ip)
		}()
		if !allowed {
			writeRateLimited(rec, retry)
			return
		}
		next(rec, r)
	}
}

func (h *Handler) writeAccessLog(r *http.Request, route string, status int, start time.Time, l *IPRateLimiter, count int, ip string) {
	if status == 0 {
		status = http.StatusOK
	}
	outcome := "ok"
	switch {
	case status == http.StatusTooManyRequests:
		outcome = "rate_limited"
	case status >= 500:
		outcome = "server_error"
	case status >= 400:
		outcome = "denied"
	}
	log.Printf("[auth] operation=http_access method=%s route=%q status=%d ms=%d class=%s count=%d limit=%d addr=%s user=%s sid=%s outcome=%s",
		r.Method, route, status, time.Since(start).Milliseconds(), l.class, count, l.limit,
		addrPrefix(ip), h.logUser(r), logSession(r), outcome)
}

// logUser is the first 8 characters of the user id in a valid access token.
func (h *Handler) logUser(r *http.Request) string {
	cookie, err := r.Cookie(AccessTokenCookieName)
	if err != nil || cookie.Value == "" {
		return "-"
	}
	id, _, err := h.validateAccessTokenMethod(cookie.Value)
	if err != nil || id == "" {
		return "-"
	}
	if len(id) > 8 {
		id = id[:8]
	}
	return id
}

// logSession is a short hash of the refresh cookie: it ties a user's requests
// to one session without revealing the token.
func logSession(r *http.Request) string {
	cookie, err := r.Cookie(RefreshTokenCookieName)
	if err != nil || cookie.Value == "" {
		return "-"
	}
	sum := sha256.Sum256([]byte(cookie.Value))
	return fmt.Sprintf("%x", sum)[:8]
}
