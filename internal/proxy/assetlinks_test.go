package proxy

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// Android Credential Manager verifies the installed package and signing
// certificate through this exact public document. Exercise the real route
// table and server chain: no cookie, no redirect and no HTML catch-all.
func TestAndroidAssetLinksThroughRealChain(t *testing.T) {
	handler, _, _ := testProxyEnv(t)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	want := `[
  {
    "relation": [
      "delegate_permission/common.get_login_creds"
    ],
    "target": {
      "namespace": "android_app",
      "package_name": "news.choir.autoradio.lab.one",
      "sha256_cert_fingerprints": [
        "DF:E9:76:7C:64:2A:2F:86:4B:29:70:49:BD:E9:7C:08:36:CB:58:D5:CE:51:6F:FE:7A:7D:A1:1F:6F:EC:9E:A7"
      ]
    }
  }
]
`
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "https://choir.news/.well-known/assetlinks.json", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 (location %q)", method, rec.Code, rec.Header().Get("Location"))
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s content type = %q, want application/json", method, ct)
		}
		if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(len(want)) {
			t.Fatalf("%s content length = %q, want %d", method, cl, len(want))
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
		if rec.Body.String() != want {
			t.Fatalf("assetlinks body = %q, want %q", rec.Body.String(), want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/.well-known/assetlinks.json", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST status = %d, Allow = %q", rec.Code, rec.Header().Get("Allow"))
	}

	for _, path := range []string{"/.well-known/other", "/.well-known/assetlinks.json/", "/.well-known/assetlinks.json/extra"} {
		req = httptest.NewRequest(http.MethodGet, "https://choir.news"+path, nil)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "delegate_permission/common.get_login_creds") {
			t.Fatalf("%s served assetlinks (status %d)", path, rec.Code)
		}
	}
}
