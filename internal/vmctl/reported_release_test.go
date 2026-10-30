package vmctl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// P6 at vmctl (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md). Failure
// modes pinned:
//   I6 vmctl list probes a guest (list must read only what was reported);
//   I7 a guest reports another computer's identity, or an unbound caller
//      writes a report;
//   I8 a report overwrites vmctl's own authority (construction_version,
//      divergence_status);
//   a report does not survive a vmctl restart.

func reportedReleaseRegistry(t *testing.T, path string) (*OwnershipRegistry, *Handler, map[string]string) {
	t.Helper()
	registry := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := registry.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(registry)
	ids := map[string]string{}
	for user, addr := range map[string]string{"user-a": "10.200.1.2", "user-b": "10.200.1.3"} {
		req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", bytes.NewBufferString(`{"user_id":"`+user+`","desktop_id":"primary"}`))
		req.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		handler.HandleResolve(rec, req)
		var resolved resolveResponse
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resolved) != nil {
			t.Fatalf("resolve %s = %d %s", user, rec.Code, rec.Body.String())
		}
		ids[user] = resolved.ComputerID
		registry.mu.Lock()
		own := registry.ownerships[ownershipKey(user, PrimaryDesktopID)]
		own.ComputerURL = "http://" + addr + ":8085"
		own.State = VMStateActive
		registry.mu.Unlock()
	}
	return registry, handler, ids
}

func postReport(handler *Handler, remote, computerID string, report ReportedRelease) *httptest.ResponseRecorder {
	body, _ := json.Marshal(EffectiveReleaseReport{ComputerID: computerID, Release: report})
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computers/effective-release", bytes.NewReader(body))
	req.RemoteAddr = remote
	rec := httptest.NewRecorder()
	handler.HandleEffectiveReleaseReport(rec, req)
	return rec
}

func TestEffectiveReleaseReportIsTapBoundReportOnlyAndPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	registry, handler, ids := reportedReleaseRegistry(t, path)
	report := ReportedRelease{
		ReleaseDigest: strings.Repeat("a", 64), CodeRef: "code:sha256:" + strings.Repeat("b", 64), TreeHash: strings.Repeat("c", 40),
		BuildCommit: strings.Repeat("d", 40), OperationID: "op-1", AdoptedReleaseID: strings.Repeat("e", 64),
		EventHeadSequence: 9, CanonicalEventHead: strings.Repeat("f", 64), EffectiveEventHead: strings.Repeat("1", 64),
	}
	// I7: computer A's guest names B; an unbound address names A.
	if rec := postReport(handler, "10.200.1.2:40000", ids["user-b"], report); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-computer report = %d, want 403", rec.Code)
	}
	if rec := postReport(handler, "10.200.1.9:40000", ids["user-a"], report); rec.Code != http.StatusForbidden {
		t.Fatalf("unbound report = %d, want 403", rec.Code)
	}
	if rec := postReport(handler, "10.200.1.2:40000", ids["user-a"], ReportedRelease{ReleaseDigest: "not-a-digest"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed report = %d, want 400", rec.Code)
	}
	if rec := postReport(handler, "10.200.1.2:40000", ids["user-a"], report); rec.Code != http.StatusOK {
		t.Fatalf("bound report = %d %s", rec.Code, rec.Body.String())
	}

	list := func(h *Handler) []ownershipResponse {
		req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
		req.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		h.HandleList(rec, req)
		var out struct {
			Ownerships []ownershipResponse `json:"ownerships"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
		}
		return out.Ownerships
	}
	var a, b *ownershipResponse
	for _, own := range list(handler) {
		own := own
		switch own.ComputerID {
		case ids["user-a"]:
			a = &own
		case ids["user-b"]:
			b = &own
		}
	}
	if a == nil || a.ReportedRelease == nil || a.ReportedRelease.TreeHash != report.TreeHash || a.ReportedRelease.ReportedAt == "" {
		t.Fatalf("A's reported release = %+v", a)
	}
	if b == nil || b.ReportedRelease != nil {
		t.Fatalf("B carries a report it never made: %+v", b)
	}
	// I8: vmctl's own authority is untouched by a report.
	if a.ConstructionVersion != nil || a.DivergenceStatus != "tracking" {
		t.Fatalf("report moved vmctl authority: construction %v divergence %q", a.ConstructionVersion, a.DivergenceStatus)
	}

	// The report survives a vmctl restart.
	_ = registry
	reloaded := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reloaded.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, own := range list(NewHandler(reloaded)) {
		if own.ComputerID == ids["user-a"] && own.ReportedRelease != nil && own.ReportedRelease.ReleaseDigest == report.ReleaseDigest {
			found = true
		}
	}
	if !found {
		t.Fatal("the reported release did not survive a reload")
	}
}
