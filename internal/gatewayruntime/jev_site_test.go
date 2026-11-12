package gatewayruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Ways this can fail, written before the code:
//
//	S1. A guest's Jev call carries no X-Choir-Jev-Site header, so the gateway
//	    log says site=unknown for every guest call (the 2026-11-12 usage
//	    review could not tell intake from the event layer).
//	S2. A call with no site in its context sends an empty header, or one the
//	    gateway would sanitise to "unknown" anyway.
//	S3. The site header reaches the request body or the vendor (it is a log
//	    label; the gateway never sends it upstream).

func TestJudgmentClientSendsTheCallSite(t *testing.T) {
	var gotSite, gotAuth string
	var body map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSite, gotAuth = r.Header.Get(JevSiteHeader), r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	}))
	defer srv.Close()
	c := NewJudgmentClient(srv.URL, "tok")
	req := JudgmentRequest{State: json.RawMessage(`"s"`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{}`)}}

	if _, err := c.Decide(WithJevSite(context.Background(), "intake_labels"), req); err != nil {
		t.Fatal(err)
	}
	if gotSite != "intake_labels" {
		t.Errorf("S1: header %q, want intake_labels", gotSite)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("the bearer is unchanged, got %q", gotAuth)
	}
	for k := range body {
		if k == "site" {
			t.Errorf("S3: the site rides the header only, not the body")
		}
	}

	if _, err := c.Decide(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if gotSite != "" {
		t.Errorf("S2: header %q sent with no site in the context", gotSite)
	}
}

// The gateway reads the same header name; the two must not drift.
func TestJevSiteHeaderMatchesTheGatewayConstant(t *testing.T) {
	if JevSiteHeader != "X-Choir-Jev-Site" {
		t.Fatalf("header %q; the gateway reads X-Choir-Jev-Site (internal/gateway/jev_limiter.go)", JevSiteHeader)
	}
}
