package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	desktopAuthRequestLimit  = 8 << 10
	desktopAuthResponseLimit = 64 << 10
	desktopAuthNetworkLimit  = 15 * time.Second
	accessCookieName         = "choir_access"
	refreshCookieName        = "choir_refresh"
)

var blockedDesktopAuthRoutes = map[string]struct{}{
	"/auth/desktop/exchange":          {},
	"/auth/desktop/exchange-redirect": {},
	"/auth/desktop/redeem":            {},
}

type desktopTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// desktopSession is the sole session authority for the Wails app. The native
// process owns the backend cookies; renderer requests can use them through the
// proxy but can neither read nor replace them.
type desktopSession struct {
	backend         *url.URL
	jar             http.CookieJar
	client          *http.Client
	openAuthSession func(string, string) (string, error)
	authMu          sync.Mutex
}

func newDesktopSession(rawBackend string) (*desktopSession, error) {
	backend, err := parseDesktopBackend(rawBackend)
	if err != nil {
		return nil, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create desktop cookie jar: %w", err)
	}
	return &desktopSession{
		backend: backend,
		jar:     jar,
		client: &http.Client{
			Jar:     jar,
			Timeout: desktopAuthNetworkLimit,
		},
		openAuthSession: startWebAuthSession,
	}, nil
}

func parseDesktopBackend(raw string) (*url.URL, error) {
	backend, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !backend.IsAbs() || backend.Host == "" {
		return nil, errors.New("desktop backend must be an absolute URL")
	}
	if backend.User != nil || backend.RawQuery != "" || backend.Fragment != "" {
		return nil, errors.New("desktop backend must not contain credentials, query, or fragment")
	}
	if backend.Path != "" && backend.Path != "/" {
		return nil, errors.New("desktop backend must not contain a path")
	}

	switch strings.ToLower(backend.Scheme) {
	case "https":
	case "http":
		if !isLoopbackHost(backend.Hostname()) {
			return nil, errors.New("desktop backend requires HTTPS outside loopback development")
		}
	default:
		return nil, errors.New("desktop backend requires HTTPS outside loopback development")
	}

	backend.Scheme = strings.ToLower(backend.Scheme)
	backend.Path = ""
	return backend, nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *desktopSession) endpoint(path string) *url.URL {
	return s.backend.ResolveReference(&url.URL{Path: path})
}

func (s *desktopSession) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if !s.authMu.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "desktop authentication already in progress"})
		return
	}
	defer s.authMu.Unlock()

	r.Body = http.MaxBytesReader(w, r.Body, desktopAuthRequestLimit)
	var input struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			status = http.StatusRequestEntityTooLarge
		}
		writeJSON(w, status, map[string]string{"error": "invalid desktop authentication request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid desktop authentication request"})
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	if input.Email == "" || len(input.Email) > 320 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid desktop authentication request"})
		return
	}

	bridgeURL := s.endpoint("/desktop-bridge.html")
	query := bridgeURL.Query()
	query.Set("email", input.Email)
	bridgeURL.RawQuery = query.Encode()

	callback, err := s.openAuthSession(bridgeURL.String(), "choir")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "desktop authentication failed"})
		return
	}
	code, err := desktopExchangeCode(callback)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "desktop authentication failed"})
		return
	}

	tokens, err := s.redeem(r, code)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "desktop authentication failed"})
		return
	}
	if err := s.seedSession(tokens); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "desktop authentication failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func desktopExchangeCode(callback string) (string, error) {
	callbackURL, err := url.Parse(callback)
	if err != nil || callbackURL.Scheme != "choir" || callbackURL.Host != "auth-complete" {
		return "", errors.New("invalid desktop authentication callback")
	}
	code := callbackURL.Query().Get("code")
	if code == "" || len(code) > 2048 {
		return "", errors.New("invalid desktop authentication callback")
	}
	return code, nil
}

func (s *desktopSession) redeem(r *http.Request, code string) (desktopTokenResponse, error) {
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return desktopTokenResponse{}, errors.New("encode desktop token redemption")
	}
	redeemRequest, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		s.endpoint("/auth/desktop/redeem").String(),
		bytes.NewReader(body),
	)
	if err != nil {
		return desktopTokenResponse{}, errors.New("create desktop token redemption")
	}
	redeemRequest.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(redeemRequest)
	if err != nil {
		return desktopTokenResponse{}, errors.New("perform desktop token redemption")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, desktopAuthResponseLimit))
		return desktopTokenResponse{}, errors.New("desktop token redemption rejected")
	}

	limited := io.LimitReader(response.Body, desktopAuthResponseLimit+1)
	payload, err := io.ReadAll(limited)
	if err != nil || len(payload) > desktopAuthResponseLimit {
		return desktopTokenResponse{}, errors.New("invalid desktop token redemption response")
	}
	var tokens desktopTokenResponse
	if err := json.Unmarshal(payload, &tokens); err != nil || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return desktopTokenResponse{}, errors.New("invalid desktop token redemption response")
	}
	return tokens, nil
}

func (s *desktopSession) seedSession(tokens desktopTokenResponse) error {
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return errors.New("desktop session tokens are required")
	}
	secure := s.backend.Scheme == "https"
	s.jar.SetCookies(s.endpoint("/"), []*http.Cookie{
		{
			Name:     accessCookieName,
			Value:    tokens.AccessToken,
			Path:     "/",
			Secure:   secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
		{
			Name:     refreshCookieName,
			Value:    tokens.RefreshToken,
			Path:     "/auth",
			Secure:   secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
	})
	return nil
}

func (s *desktopSession) proxy() *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(s.backend)
	proxy.Transport = s.client.Transport
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Host = s.backend.Host
		request.Header.Del("Cookie")
		for _, cookie := range s.jar.Cookies(request.URL) {
			request.AddCookie(cookie)
		}
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.Request != nil && response.Request.URL != nil {
			s.jar.SetCookies(response.Request.URL, response.Cookies())
		}
		response.Header.Del("Set-Cookie")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "backend request failed"})
	}
	return proxy
}

func isBlockedDesktopAuthRoute(path string) bool {
	for route := range blockedDesktopAuthRoutes {
		if path == route || strings.HasPrefix(path, route+"/") {
			return true
		}
	}
	return false
}
