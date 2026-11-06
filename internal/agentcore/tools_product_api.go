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
	pathpkg "path"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
	if err := validateProductAPIRouteFor(ctx, method, norm); err != nil {
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
	// In-process: an empty RemoteAddr is the guest's in-process marker
	// (server.GuestHostCaller). httptest's default TEST-NET-1 address counts
	// only inside test binaries.
	req.RemoteAddr = ""
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
// with the caller's owner identity. Bound into the desk egress dispatch. The
// host serves it only to a desk that holds the product.api grant (MA4.9): the
// daemon's frame check refuses the action first, and this is the host's own
// check, so no path that reaches the host without the frame check widens it.
func (rt *Runtime) productAPIEgress(ctx context.Context, deskRole, ownerID, ownerEmail string, payload json.RawMessage) (json.RawMessage, error) {
	if !deskmanifest.Holds(strings.TrimSpace(deskRole), "product.api") {
		return nil, fmt.Errorf("product_api is served under the product.api grant, which %q does not hold", deskRole)
	}
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
	// The route checks below read the decoded path; the router serves the
	// escaped one. A path the two could read differently (a dot segment, an
	// empty segment, an escaped slash) is refused, so the route a desk is
	// checked against is the route it is served (MA6.3: a cell-supplied
	// reference never reaches past the allowlist through the daemon).
	if err := canonicalProductAPIPath(u); err != nil {
		return "", err
	}
	return u.RequestURI(), nil
}

// canonicalProductAPIPath refuses a path that is not its own clean form.
func canonicalProductAPIPath(u *url.URL) error {
	p := u.Path
	clean := pathpkg.Clean(p)
	if strings.HasSuffix(p, "/") && clean != "/" {
		clean += "/"
	}
	if clean != p {
		return fmt.Errorf("path %q is not canonical (it reads as %q); send the route itself", p, clean)
	}
	if strings.Contains(strings.ToLower(u.EscapedPath()), "%2f") || strings.Contains(p, "\\") {
		return fmt.Errorf("path %q carries an escaped slash or a backslash; send the route itself", p)
	}
	return nil
}

func validateProductAPIRoute(method, requestURI string) error {
	return validateProductAPIRouteFor(context.Background(), method, requestURI)
}

// productAPIWriteKinds names, for each ProductAPI route a desk may write, the
// object kind the write lands in. A route absent here has no kind, hence no
// custodian that could be the caller, and is refused for a write (6.2-8).
var productAPIWriteKinds = map[string]objectgraph.ObjectKind{
	// Management is the custodian of source schedules (plan section 8.2).
	"/api/autopaper/sources/schedules": "choir.source_schedule",
}

// productAPICaller is the desk a ProductAPI call is made as: the declared
// reader of the call's context. A call that declares none has no caller and
// holds no authority (it fails closed, never the union of every desk's).
func productAPICaller(ctx context.Context) (objectgraph.Writer, bool) {
	return objectgraph.ReaderFrom(ctx)
}

func validateProductAPIRouteFor(ctx context.Context, method, requestURI string) error {
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
		// The owner's lease view and revoke (MA3.4): an owner act no desk reaches.
		"/api/leases",
		// The owner's tap on a disclosure (MA3.6): an owner act no desk reaches.
		"/api/disclosures",
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
	// A desk lists the stories and reads their summaries; a story's content is
	// Texture's, so it comes through a request to Texture (a Cast), never a
	// document or revision route (MA3.7a, X7). The list route answers a desk
	// from the story index Texture publishes.
	if path == "/api/texture" || strings.HasPrefix(path, "/api/texture/") {
		if path != "/api/texture/documents" {
			return fmt.Errorf("product_api refuses GET %s: a desk reads the story list (GET /api/texture/documents) and summaries; a story's content comes through a Cast to the Texture desk", path)
		}
		return nil
	}
	// A desk writes only the kinds it is custodian of: the Autopaper routes
	// that write (source schedules) are management's, and any other write
	// under /api/autopaper/ names no kind the caller is custodian of (6.2-8).
	if method != http.MethodGet && (path == "/api/autopaper" || strings.HasPrefix(path, "/api/autopaper/")) {
		kind, known := productAPIWriteKinds[strings.TrimSuffix(path, "/")]
		custodian, held := objectgraph.CustodianOf(kind)
		caller, declared := productAPICaller(ctx)
		if !(known && held && declared && caller == custodian) {
			return fmt.Errorf("product_api refuses %s %s: the route writes a kind the calling desk is not custodian of", method, path)
		}
		return nil
	}
	// Management scores a candidate's screens with one bounded model call that
	// may carry an image ref (problems/models-configured-text-only-
	// 2026-10-15.md, desk seam). Only the verify route: policy reads and
	// writes stay owner acts.
	if path == "/api/model-policy/verify" {
		if method != http.MethodPost {
			return fmt.Errorf("product_api refuses %s %s: the verify route takes POST", method, path)
		}
		return nil
	}
	for _, allowed := range []string{
		"/api/prompt-bar",
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
