package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// The management desk's choir.ProductAPI reaches /api/computers/ through the
// real computers router, which requires the caller's computer binding. Before
// the fix the in-process request carried none, so every call got 403
// (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 3).
//
// Failure modes pinned:
//   - no binding: every computer read is refused;
//   - a binding taken from the path or cell input: a desk could name another
//     computer;
//   - the binding unlocks writes: owner acts (decision, restore,
//     replace-workspace, ...) become desk acts;
//   - an unconfigured identity binds to something: it must fail closed.
func TestProductAPIComputerRoutesBindToTheBootComputer(t *testing.T) {
	t.Parallel()

	newRuntime := func(computerID string) *Runtime {
		canonical := server.NewServer("product-api-computer-binding-test", "0")
		canonical.HandleFunc("/api/computers/", NewAPIHandler(&Runtime{}).HandleComputersRouter)
		rt := &Runtime{}
		rt.SetProductAPIHandler(canonical, computerID)
		return rt
	}
	call := func(rt *Runtime, method, path string) (int, string, error) {
		t.Helper()
		raw, err := rt.productAPIRequest(context.Background(), "user-product-api", "", method, path, nil)
		if err != nil {
			return 0, "", err
		}
		var result struct {
			StatusCode int    `json:"status_code"`
			Body       string `json:"body"`
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatalf("decode product_api response: %v\n%s", err, raw)
		}
		return result.StatusCode, result.Body, nil
	}
	const binding = "authenticated computer binding required"

	rt := newRuntime("computer-own")
	// Own computer: the binding passes and the router reaches its handler (an
	// empty runtime has no operation store, so it answers 503 past the check).
	status, body, err := call(rt, http.MethodGet, "/api/computers/computer-own/self-development/operations")
	if err != nil {
		t.Fatalf("own computer read: %v", err)
	}
	if status == http.StatusForbidden || strings.Contains(body, binding) {
		t.Fatalf("own computer read refused on binding: status=%d body=%s", status, body)
	}
	if status != http.StatusServiceUnavailable {
		t.Fatalf("own computer read status = %d body=%s, want 503 from the handler past the binding", status, body)
	}

	// Another computer: refused by the router, never bound from the path.
	status, body, err = call(rt, http.MethodGet, "/api/computers/computer-other/self-development/operations")
	if err != nil || status != http.StatusForbidden || !strings.Contains(body, binding) {
		t.Fatalf("foreign computer read: status=%d body=%s err=%v, want 403 binding refusal", status, body, err)
	}

	// Writes are owner acts: refused before the router.
	for _, path := range []string{
		"/api/computers/computer-own/self-development/operations/op-1/decision",
		"/api/computers/computer-own/lifecycle/restore",
		"/api/computers/computer-own/lifecycle/replace-workspace",
		"/api/computers/computer-own/lifecycle/bootstrap-chain",
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if _, _, err := call(rt, method, path); err == nil || !strings.Contains(err.Error(), "read-only to a desk") {
				t.Errorf("%s %s: error = %v, want a read-only refusal", method, path, err)
			}
		}
	}

	// No boot identity: no binding, so the router refuses.
	status, body, err = call(newRuntime(""), http.MethodGet, "/api/computers/computer-own/self-development/operations")
	if err != nil || status != http.StatusForbidden || !strings.Contains(body, binding) {
		t.Fatalf("unbound runtime read: status=%d body=%s err=%v, want 403 binding refusal", status, body, err)
	}
}
