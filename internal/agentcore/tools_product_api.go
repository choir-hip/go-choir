package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

// SMG: the management desk's product API call is an in-cell verb
// (choir.ProductAPI -> broker ActionProductAPI -> here), not a registered
// tool. The allowlist + owner-bound serving semantics are byte-identical to
// the deleted product_api_request tool.

const productAPIMaxBodyBytes = 1 << 20

// productAPIHandler is bound on the Runtime at autoputer boot; see
// SetProductAPIHandler on the Runtime.

// SetProductAPIHandler binds the canonical product API route table for the
// management desk's choir.ProductAPI verb, and the computer's own identity
// the calls are bound to. Called once at autoputer boot with the boot
// identity (CHOIR_COMPUTER_ID); an empty computerID sends no binding, so
// /api/computers/ routes refuse.
func (rt *Runtime) SetProductAPIHandler(h http.Handler, computerID string) {
	rt.productAPIMu.Lock()
	rt.productAPISrv = h
	rt.productAPIComputerID = strings.TrimSpace(computerID)
	rt.productAPIMu.Unlock()
}

func (rt *Runtime) productAPIServer() (http.Handler, string) {
	rt.productAPIMu.RLock()
	defer rt.productAPIMu.RUnlock()
	return rt.productAPISrv, rt.productAPIComputerID
}

// productAPIRequest serves one allowlisted product API call as the given
// owner. Called from the desk-cell egress dispatch; ownerID/ownerEmail come
// from the run's execution context, never from cell input.
func (rt *Runtime) productAPIRequest(ctx context.Context, ownerID, ownerEmail, method, path string, body json.RawMessage) (string, error) {
	s, computerID := rt.productAPIServer()
	if s == nil {
		return "", fmt.Errorf("choir.ProductAPI host unavailable")
	}
	if strings.TrimSpace(ownerID) == "" {
		return "", fmt.Errorf("product_api missing owner context")
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	norm, err := normalizeProductAPIPath(path)
	if err != nil {
		return "", err
	}
	if err := validateProductAPIRoute(method, norm); err != nil {
		return "", err
	}
	var rdr io.Reader
	if len(body) > 0 && string(body) != "null" {
		if len(body) > productAPIMaxBodyBytes {
			return "", fmt.Errorf("product_api body exceeds %d bytes", productAPIMaxBodyBytes)
		}
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, norm, rdr).WithContext(ctx)
	req.Header.Set("X-Authenticated-User", ownerID)
	if ownerEmail != "" {
		req.Header.Set("X-Authenticated-Email", ownerEmail)
	}
	// The computer routes require the caller's computer binding (the proxy
	// sets it for owner requests). An in-process ProductAPI call is this
	// computer acting on itself: the binding is its boot identity, so a path
	// naming any other computer is refused by the router.
	if computerID != "" {
		req.Header.Set("X-Authenticated-Computer", computerID)
	}
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, productAPIMaxBodyBytes+1))
	if readErr != nil {
		return "", fmt.Errorf("read product_api response: %w", readErr)
	}
	truncated := false
	if len(respBody) > productAPIMaxBodyBytes {
		respBody = respBody[:productAPIMaxBodyBytes]
		truncated = true
	}
	result := map[string]any{
		"method":       method,
		"path":         norm,
		"status_code":  resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"body":         strings.TrimSpace(string(respBody)),
		"allowed_by":   "product_api_allowlist",
	}
	if truncated {
		result["truncated"] = true
	}
	if resp.StatusCode >= 400 {
		result["error"] = "product API returned non-2xx status"
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// productAPIEgress decodes the cell's payload and calls productAPIRequest
// with the caller's owner identity. Bound into the desk egress dispatch.
func (rt *Runtime) productAPIEgress(ctx context.Context, ownerID, ownerEmail string, payload json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body,omitempty"`
	}
	if err := json.Unmarshal(payload, &in); err != nil {
		return nil, fmt.Errorf("decode product_api payload: %w", err)
	}
	out, err := rt.productAPIRequest(ctx, ownerID, ownerEmail, in.Method, in.Path, in.Body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

func normalizeProductAPIPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("path must not contain newlines")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("parse product API URL: %w", err)
		}
		raw = u.RequestURI()
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("path must be absolute")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return "", fmt.Errorf("parse product API path: %w", err)
	}
	if u.Path == "" {
		return "", fmt.Errorf("path must include an API route")
	}
	return u.RequestURI(), nil
}

func validateProductAPIRoute(method, requestURI string) error {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return fmt.Errorf("method %q is not allowed", method)
	}
	u, err := url.ParseRequestURI(requestURI)
	if err != nil {
		return err
	}
	path := u.Path
	for _, blocked := range []string{
		"/internal/",
		"/api/agent/",
		"/api/prompts",
		"/api/test/",
	} {
		if path == strings.TrimSuffix(blocked, "/") || strings.HasPrefix(path, blocked) {
			return fmt.Errorf("product_api refuses non-product route %s", path)
		}
	}
	// The computer routes are read-only to a desk. Their writes are owner
	// acts (operation decisions, restore, replace-workspace, bootstrap-chain,
	// rematerialize, residue import), fenced off agents in 5b17bb572; the
	// computer binding above must not hand them to the management desk.
	if method != http.MethodGet && strings.HasPrefix(path, "/api/computers/") {
		return fmt.Errorf("product_api refuses %s %s: computer routes are read-only to a desk; their writes are owner acts", method, path)
	}
	// Only the Texture desk writes a Texture (protocol P2b): a desk reads
	// Texture routes but never mutates one. A desk that needs a Texture
	// changed casts to it; a new Texture starts through /api/prompt-bar.
	if method != http.MethodGet && (path == "/api/texture" || strings.HasPrefix(path, "/api/texture/")) {
		return fmt.Errorf("product_api refuses %s %s: only the Texture desk writes a Texture; read it with GET, or address the Texture desk", method, path)
	}
	for _, allowed := range []string{
		"/api/prompt-bar",
		"/api/texture/",
		"/api/computers/",
		"/api/run-acceptances",
		"/api/run-acceptances/",
		"/api/autopaper/",
	} {
		if path == strings.TrimSuffix(allowed, "/") || strings.HasPrefix(path, allowed) {
			return nil
		}
	}
	return fmt.Errorf("product_api route %s is not in the product-path allowlist", path)
}
