package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// The phone app's passkeys need this file at choir.news: 200, JSON, no
// redirect, no sign-in, and the three app identifiers. It goes through the
// real route table and the real server handler chain, with no cookie.
func TestAppleAppSiteAssociationThroughRealChain(t *testing.T) {
	handler, _, _ := testProxyEnv(t)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "https://choir.news/.well-known/apple-app-site-association", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 (location %q)", method, rec.Code, rec.Header().Get("Location"))
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s content type = %q, want application/json", method, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
			t.Fatalf("%s cache control = %q", method, cc)
		}
		if method == http.MethodHead {
			if rec.Body.Len() != 0 {
				t.Fatalf("HEAD wrote a body of %d bytes", rec.Body.Len())
			}
			continue
		}
		var body struct {
			WebCredentials struct {
				Apps []string `json:"apps"`
			} `json:"webcredentials"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		want := []string{
			"S47GERGJZ3.news.choir.autoradio",
			"S47GERGJZ3.news.choir.autoradio.lab",
			"S47GERGJZ3.news.choir.autoradio.lab.one",
		}
		if !reflect.DeepEqual(body.WebCredentials.Apps, want) {
			t.Fatalf("webcredentials.apps = %v, want %v", body.WebCredentials.Apps, want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/.well-known/apple-app-site-association", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("POST Allow = %q", allow)
	}

	// Only the exact path serves the file: a neighbour falls to the catch-all.
	req = httptest.NewRequest(http.MethodGet, "https://choir.news/.well-known/other", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Type") == "application/json" && rec.Code == http.StatusOK && rec.Body.String() == string(appleAppSiteAssociationBody) {
		t.Fatalf("a neighbouring path served the association file")
	}
}
