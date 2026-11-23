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
		if method == http.MethodHead {
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
		if !reflect.DeepEqual(appleWebCredentialsApps, want) {
			t.Fatalf("list constant drifted from the served body: %v", appleWebCredentialsApps)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/.well-known/apple-app-site-association", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}
