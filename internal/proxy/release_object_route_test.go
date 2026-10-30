package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Library S1 review 262, F20: corpusd sees a guest's relayed write as the
// proxy's (an internal caller), so the proxy must keep a bound guest off the
// generic object routes for choir.software_release, as it does for
// captures. A release is published only by the owner's act
// (POST /api/computers/<id>/releases); a guest-filed release object would
// satisfy metadata the release reads trust.
func TestGuestCannotFileReleaseObjectsThroughTheProxy(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := platform.NewObjectGraphHandler(svc, mem)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/platform/objects", og.HandleObjects)
	corpusd := httptest.NewServer(mux)
	defer corpusd.Close()
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.GuestOwnership{Found: true, UserID: "owner-a", ComputerID: "computer-a1"})
	}))
	defer vmctlServer.Close()
	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	send := func(method string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = "10.200.1.2:40000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
	meta := map[string]any{"patch_sha256": strings.Repeat("a", 64)}
	create := map[string]any{"kind": "choir.software_release", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": meta}
	if rec := send(http.MethodPost, create); rec.Code != http.StatusForbidden {
		t.Fatalf("guest created a release object through the proxy: %d %s", rec.Code, rec.Body)
	}
	id, _ := objectgraph.BuildCanonicalID(platform.SoftwareReleaseKind, "owner-a", "forged")
	normalized, _ := objectgraph.NormalizeMetadata(meta)
	put := objectgraph.Object{CanonicalID: id, ObjectKind: platform.SoftwareReleaseKind, OwnerID: "owner-a", ComputerID: "computer-a1",
		Body: []byte("x"), Metadata: normalized, ContentHash: objectgraph.ContentHash(platform.SoftwareReleaseKind, []byte("x"), normalized)}
	if rec := send(http.MethodPut, put); rec.Code != http.StatusForbidden {
		t.Fatalf("guest put a release object through the proxy: %d %s", rec.Code, rec.Body)
	}
	if _, err := svc.GetObject(t.Context(), id); err == nil {
		t.Fatal("a release object was written through the generic route")
	}
	// Every other kind is refused too: no guest writes the shared graph
	// through the generic route (guest_generic_write_test.go).
	other := map[string]any{"kind": "choir.source_entity", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k2", "metadata": map[string]any{}}
	if rec := send(http.MethodPost, other); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest wrote another kind through the generic route: %d %s", rec.Code, rec.Body)
	}
}
