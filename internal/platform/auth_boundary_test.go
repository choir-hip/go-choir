package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Failure modes pinned here
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F5):
//
//	a bound guest lists another owner's or computer's objects by choosing
//	the filter; a guest lists edges at all; a host caller loses the list;
//	the break-glass unwrap routes accept an operator token from a guest
//	tap, or from loopback without the host credential once it enforces.

func TestGuestObjectListIsScopedToItsOwnComputer(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := NewObjectGraphHandler(svc, mem)
	if err := og.ConfigureGuestBinding(fakeGuestLookup{
		"10.200.1.2": {Found: true, UserID: "paper-owner", ComputerID: "computer-paper"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []struct{ owner, computer string }{{"paper-owner", "computer-paper"}, {"reader-owner", "computer-reader"}} {
		if _, err := svc.CreateObject(t.Context(), objectgraph.CreateObjectRequest{
			Kind: objectgraph.ObjectKind("choir.agent"), OwnerID: owner.owner, ComputerID: owner.computer, IdentityKey: "agent-" + owner.owner,
			Metadata: map[string]any{},
		}); err != nil {
			t.Fatalf("create object for %s: %v", owner.owner, err)
		}
	}
	call := func(handler http.HandlerFunc, path, remote string, internal bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if remote != "" {
			req.RemoteAddr = remote
		}
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	for _, path := range []string{
		"/internal/platform/objects?owner=reader-owner&computer=computer-reader",
		"/internal/platform/objects?owner=paper-owner&computer=computer-reader",
		"/internal/platform/objects?owner=reader-owner",
		"/internal/platform/objects",
	} {
		if rec := call(og.HandleObjects, path, "10.200.1.2:40000", false); rec.Code != http.StatusForbidden {
			t.Errorf("guest %s = %d %s, want 403", path, rec.Code, rec.Body.String())
		}
	}
	if rec := call(og.HandleEdges, "/internal/platform/edges", "10.200.1.2:40000", false); rec.Code != http.StatusForbidden {
		t.Errorf("guest edge list = %d, want 403", rec.Code)
	}

	own := call(og.HandleObjects, "/internal/platform/objects?owner=paper-owner&computer=computer-paper", "10.200.1.2:40000", false)
	var objects []objectgraph.Object
	if own.Code != http.StatusOK || json.Unmarshal(own.Body.Bytes(), &objects) != nil || len(objects) != 1 || objects[0].OwnerID != "paper-owner" {
		t.Fatalf("guest own list = %d %s, want its one object", own.Code, own.Body.String())
	}
	// Host callers keep the unfiltered list.
	all := call(og.HandleObjects, "/internal/platform/objects", "", true)
	if all.Code != http.StatusOK || json.Unmarshal(all.Body.Bytes(), &objects) != nil || len(objects) != 2 {
		t.Fatalf("host list = %d %s, want both objects", all.Code, all.Body.String())
	}
	if rec := call(og.HandleEdges, "/internal/platform/edges", "", true); rec.Code != http.StatusOK {
		t.Fatalf("host edge list = %d %s", rec.Code, rec.Body.String())
	}
}

func TestKeyUnwrapOperatorRoutesNeedHostTransportAndCredential(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing-key")))
	privateKey, _, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureKeyEscrow(privateKey, "alice:token-a,bob:token-b"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(keyUnwrapRequestInput{ComputerID: "computer-x", Reason: "recovery", IdempotencyKey: "guest-attempt"})
	send := func(handlerFunc http.HandlerFunc, path, remote string, payload []byte, token string) int {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
		req.RemoteAddr = remote
		req.Header.Set("X-Choir-Operator", "alice")
		req.Header.Set("X-Choir-Operator-Token", "token-a")
		if token != "" {
			req.Header.Set(hostauth.Header, token)
		}
		rec := httptest.NewRecorder()
		hostauth.Middleware(handlerFunc).ServeHTTP(rec, req)
		return rec.Code
	}
	// A guest tap holding a valid operator token is refused on every
	// operator route.
	for _, route := range []struct {
		handler http.HandlerFunc
		path    string
	}{
		{handler.HandleKeyUnwrapRequests, "/internal/computers/keys/unwrap-requests"},
		{handler.HandleKeyUnwrapRequestAction, "/internal/computers/keys/unwrap-requests/req-1/approvals"},
		{handler.HandleKeyUnwrapRequestAction, "/internal/computers/keys/unwrap-requests/req-1/reveal"},
	} {
		if code := send(route.handler, route.path, "10.200.1.2:40000", body, ""); code != http.StatusForbidden {
			t.Errorf("guest-tap %s = %d, want 403", route.path, code)
		}
	}

	// With the host credential enforced, loopback without it is refused and
	// loopback with it reaches the operator check.
	const token = "0123456789abcdef0123456789abcdef0123456789abcdef"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	if code := send(handler.HandleKeyUnwrapRequests, "/internal/computers/keys/unwrap-requests", "127.0.0.1:40000", body, ""); code != http.StatusForbidden {
		t.Fatalf("loopback without the host credential = %d, want 403", code)
	}
	if code := send(handler.HandleKeyUnwrapRequests, "/internal/computers/keys/unwrap-requests", "127.0.0.1:40000", body, token); code == http.StatusForbidden {
		t.Fatalf("loopback with the host credential and operator token = %d, want past the gate", code)
	}
}
