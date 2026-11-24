package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/appids"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// The phone app's passkeys need this file at choir.news: 200, JSON, no
// redirect, no sign-in, and the two app identifiers. It goes through the
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
		if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(len(appleAppSiteAssociationBody)) {
			t.Fatalf("%s content length = %q, want %d", method, cl, len(appleAppSiteAssociationBody))
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
		if rec.Body.Len() != len(appleAppSiteAssociationBody) {
			t.Fatalf("body is %d bytes, want %d", rec.Body.Len(), len(appleAppSiteAssociationBody))
		}
		var top map[string]map[string][]string
		dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
		if err := dec.Decode(&top); err != nil {
			t.Fatalf("body is not the expected JSON shape: %v", err)
		}
		if len(top) != 1 || len(top["webcredentials"]) != 1 {
			t.Fatalf("body has extra keys: %s", rec.Body.String())
		}
		var body struct {
			WebCredentials struct {
				Apps []string `json:"apps"`
			} `json:"webcredentials"`
		}
		body.WebCredentials.Apps = top["webcredentials"]["apps"]
		want := []string{
			"S47GERGJZ3.news.choir.autoradio",
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

	// Only the exact path serves the file: neighbours and children fall to the catch-all.
	for _, path := range []string{"/.well-known/other", "/.well-known/apple-app-site-association/", "/.well-known/apple-app-site-association/extra"} {
		req = httptest.NewRequest(http.MethodGet, "https://choir.news"+path, nil)
		rec = httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "webcredentials") {
			t.Fatalf("%s served the association file (status %d)", path, rec.Code)
		}
	}
}

// Every app the manifest produces is in the list, except the first development app
// (".lab"), which is left out on purpose (see appleWebCredentialsApps).
func TestAppleWebCredentialsListCoversTheAppManifest(t *testing.T) {
	raw, err := os.ReadFile("../../mobile/AutoradioLab/app.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		App struct {
			ID string `json:"id"`
		} `json:"app"`
		Variants map[string]struct {
			IDSuffix string `json:"idSuffix"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.App.ID == "" || len(m.Variants) == 0 {
		t.Fatalf("app.json has no app id or no variants: %q %d", m.App.ID, len(m.Variants))
	}
	ids := []string{m.App.ID}
	for _, v := range m.Variants {
		if v.IDSuffix == "" {
			t.Fatalf("app.json has a variant with no id suffix")
		}
		if v.IDSuffix == ".lab" {
			continue
		}
		ids = append(ids, m.App.ID+v.IDSuffix)
	}
	for _, id := range ids {
		if team := "S47GERGJZ3." + id; !slices.Contains(appids.WebCredentialApps(), team) {
			t.Errorf("app.json produces %s, which the association file does not list", team)
		}
	}
}
