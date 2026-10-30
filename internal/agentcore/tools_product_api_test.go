package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// SMG: the typed product_api_request tool is deleted; the management desk
// reaches the same allowlisted surface through choir.ProductAPI. These tests
// exercise the host-side carrier (rt.productAPIRequest) — the serving core
// is byte-identical to the deleted tool's Func.

func TestProductAPIRequestUsesCanonicalServerAndRunOwner(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-verb-test", "0")
	var gotRequest struct {
		method      string
		requestURI  string
		ownerID     string
		ownerEmail  string
		contentType string
		body        string
	}
	canonical.HandleFunc("/api/autopaper/sources/schedules", func(w http.ResponseWriter, r *http.Request) {
		gotRequest.method = r.Method
		gotRequest.requestURI = r.URL.RequestURI()
		gotRequest.ownerID = r.Header.Get("X-Authenticated-User")
		gotRequest.ownerEmail = r.Header.Get("X-Authenticated-Email")
		gotRequest.contentType = r.Header.Get("Content-Type")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		gotRequest.body, _ = body["title"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"owner_id":%q,"server":"canonical"}`, r.Header.Get("X-Authenticated-User"))
	})

	rt := &Runtime{}
	rt.SetProductAPIHandler(canonical, "")

	raw, err := rt.productAPIRequest(context.Background(), "user-product-api", "owner@example.com",
		" post ", "https://ignored.example/api/autopaper/sources/schedules?source=tool",
		json.RawMessage(`{"title":"Product API verb owner proof"}`))
	if err != nil {
		t.Fatalf("product api verb: %v", err)
	}
	if gotRequest.method != http.MethodPost || gotRequest.requestURI != "/api/autopaper/sources/schedules?source=tool" {
		t.Fatalf("canonical request method/URI = %q %q", gotRequest.method, gotRequest.requestURI)
	}
	if gotRequest.ownerID != "user-product-api" || gotRequest.ownerEmail != "owner@example.com" {
		t.Fatalf("canonical request owner headers = %q %q", gotRequest.ownerID, gotRequest.ownerEmail)
	}
	if gotRequest.contentType != "application/json" || gotRequest.body != "Product API verb owner proof" {
		t.Fatalf("canonical request content type/body = %q %q", gotRequest.contentType, gotRequest.body)
	}

	var result struct {
		Method      string `json:"method"`
		Path        string `json:"path"`
		StatusCode  int    `json:"status_code"`
		ContentType string `json:"content_type"`
		Body        string `json:"body"`
		AllowedBy   string `json:"allowed_by"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode product_api response: %v\n%s", err, raw)
	}
	if result.Method != http.MethodPost || result.Path != "/api/autopaper/sources/schedules?source=tool" || result.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected product API result: %+v", result)
	}
	if result.ContentType != "application/json" || result.AllowedBy != "product_api_allowlist" {
		t.Fatalf("unexpected product API metadata: %+v", result)
	}
	if result.Body != `{"owner_id":"user-product-api","server":"canonical"}` {
		t.Fatalf("result body did not come from canonical server: %s", result.Body)
	}
}

func TestProductAPIRequestRejectsDisallowedRoutes(t *testing.T) {
	t.Parallel()

	rt := &Runtime{}
	rt.SetProductAPIHandler(server.NewServer("product-api-rejection-test", "0"), "")

	for _, tc := range []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "internal", method: "GET", path: "/internal/runtime/runs/run-1", want: "refuses non-product route"},
		{name: "test", method: "GET", path: "/api/test/texture", want: "refuses non-product route"},
		{name: "agent", method: "GET", path: "/api/agent/loops", want: "refuses non-product route"},
		{name: "prompt config", method: "GET", path: "/api/prompts/super", want: "refuses non-product route"},
		{name: "raw event", method: "POST", path: "/api/events", want: "not in the product-path allowlist"},
		{name: "invalid method", method: "PATCH", path: "/api/texture/documents", want: `method "PATCH" is not allowed`},
		{name: "empty path", method: "GET", path: "", want: "path must not be empty"},
		{name: "relative path", method: "GET", path: "api/texture/documents", want: "path must be absolute"},
		{name: "newline path", method: "GET", path: "/api/texture/documents\nX-Evil: true", want: "path must not contain newlines"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rt.productAPIRequest(context.Background(), "user-product-api", "", tc.method, tc.path, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}

	if _, err := rt.productAPIRequest(context.Background(), "", "", "GET", "/api/run-acceptances", nil); err == nil || !strings.Contains(err.Error(), "missing owner context") {
		t.Fatalf("missing-owner error = %v", err)
	}

	oversized := json.RawMessage(`{"value":"` + strings.Repeat("x", productAPIMaxBodyBytes) + `"}`)
	if _, err := rt.productAPIRequest(context.Background(), "user-product-api", "", "POST", "/api/autopaper/sources/schedules", oversized); err == nil || !strings.Contains(err.Error(), "body exceeds 1048576 bytes") {
		t.Fatalf("oversized-body error = %v", err)
	}
}

func TestProductAPIRequestCapsResponseAndReportsHTTPError(t *testing.T) {
	t.Parallel()

	canonical := server.NewServer("product-api-response-cap-test", "0")
	canonical.HandleFunc("/api/run-acceptances/oversized", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", productAPIMaxBodyBytes+1)))
	})
	rt := &Runtime{}
	rt.SetProductAPIHandler(canonical, "")

	raw, err := rt.productAPIRequest(context.Background(), "user-product-api", "", "GET", "/api/run-acceptances/oversized", nil)
	if err != nil {
		t.Fatalf("product api verb: %v", err)
	}
	var result struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
		Truncated  bool   `json:"truncated"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode product_api response: %v", err)
	}
	if result.StatusCode != http.StatusBadGateway || !result.Truncated || result.Error != "product API returned non-2xx status" {
		t.Fatalf("unexpected capped error result: status=%d truncated=%t error=%q", result.StatusCode, result.Truncated, result.Error)
	}
	if len(result.Body) != productAPIMaxBodyBytes || strings.Trim(result.Body, "x") != "" {
		t.Fatalf("capped body length/content = %d/%q", len(result.Body), result.Body[:min(len(result.Body), 32)])
	}
}

func TestProductAPIRequestWithoutBoundServerFailsClosed(t *testing.T) {
	t.Parallel()
	rt := &Runtime{}
	if _, err := rt.productAPIRequest(context.Background(), "user-product-api", "", "GET", "/api/run-acceptances", nil); err == nil || !strings.Contains(err.Error(), "host unavailable") {
		t.Fatalf("unbound-server error = %v", err)
	}
}

// The guest router serves no /api/trace/, /api/continuations or
// /api/universal-wire/ route (the last was deleted with the Universal Wire
// app in Cut 2), so the allowlist no longer names them (D0 census
// desk-config 1.10).
func TestProductAPIAllowlistRefusesRoutesTheGuestDoesNotServe(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"/api/trace/events",
		"/api/continuations",
		"/api/continuations/c-1",
		"/api/universal-wire/stories",
	} {
		if err := validateProductAPIRoute(http.MethodGet, path); err == nil || !strings.Contains(err.Error(), "not in the product-path allowlist") {
			t.Errorf("GET %s: error = %v, want an allowlist refusal", path, err)
		}
	}
}

// P2b (docs/archive/definitions/choir-protocol-p2-one-surface-2026-10-11.md): only
// the Texture desk writes a Texture, so ProductAPI refuses every non-GET
// /api/texture/ route for every desk. The rule is by prefix and method, so a
// Texture route added later is refused without a list to update; the paths
// below are the routes registered today (internal/textureowner/router.go,
// internal/apihandler/routes.go). Reads stay open.
func TestProductAPIRefusesEveryTextureWrite(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/api/texture/documents",
		"/api/texture/documents/doc-1",
		"/api/texture/documents/doc-1/revisions",
		"/api/texture/documents/doc-1/revise",
		"/api/texture/documents/doc-1/restore",
		"/api/texture/documents/doc-1/tell",
		"/api/texture/documents/doc-1/correct",
		"/api/texture/documents/doc-1/agent-revision",
		"/api/texture/documents/doc-1/source-open",
		"/api/texture/revisions/rev-1",
		"/api/texture/markdown-lineage/import",
		"/api/texture/files/open",
		"/api/texture/lifecycle-documents",
		"/api/texture/some-route-added-later",
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		for _, path := range paths {
			err := validateProductAPIRoute(method, path)
			if err == nil || !strings.Contains(err.Error(), "only the Texture desk writes a Texture") {
				t.Errorf("%s %s: error = %v, want a Texture-write refusal", method, path, err)
			}
		}
	}
	for _, path := range []string{
		"/api/texture/documents",
		"/api/texture/documents/doc-1/revisions",
		"/api/texture/documents/doc-1/history",
		"/api/texture/documents/doc-1",
	} {
		if err := validateProductAPIRoute(http.MethodGet, path); err != nil {
			t.Errorf("GET %s refused: %v", path, err)
		}
	}
}
