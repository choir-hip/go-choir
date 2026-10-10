package agentcore

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// docs/problems/research-fetch-url-has-no-address-guard-2026-10-10.md (D1):
// research's fetch_url used a plain http.Client, so a research cell (or a
// page that prompt-injects one) could GET the guest runtime on loopback or
// any private address the guest can route to. Failure modes pinned: the
// installed research client reaches a loopback server; the refusal is not
// the address guard's.
func TestResearchFetchClientRefusesLoopback(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.Write([]byte("guest-internal"))
	}))
	defer server.Close()
	if rt.researchDeps == nil || rt.researchDeps.HTTP == nil {
		t.Fatal("research dependencies not installed")
	}
	resp, err := rt.researchDeps.HTTP.Get(server.URL + "/api/runs")
	if err == nil {
		resp.Body.Close()
		t.Fatal("research fetch client reached a loopback server")
	}
	if reached || !strings.Contains(err.Error(), "forbidden address") {
		t.Fatalf("reached=%v err=%v, want the address guard's refusal", reached, err)
	}
}
