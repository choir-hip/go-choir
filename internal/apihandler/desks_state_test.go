package apihandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

func TestDesksStateListsTheDerivedSet(t *testing.T) {
	h := NewHandler(nil)
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:statelist", Verbs: []string{"ReadCapture"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62904, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterAppAt(d, 12); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:statelist") })

	rec := httptest.NewRecorder()
	h.HandleDesksState(rec, authenticatedCostsRequest(http.MethodGet, "/api/desks/state", "owner-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var got desksState
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	has := func(list []string, s string) bool {
		for _, x := range list {
			if x == s {
				return true
			}
		}
		return false
	}
	if !has(got.AppDesks, "app:statelist@12") || !has(got.Desks, "management") || got.Required == nil {
		t.Fatalf("state = %+v", got)
	}
	// No platform source and no selftest in a test: the reading is closed and
	// says why.
	if got.DeskSandbox.Admits || got.DeskSandbox.Worker.Ready || got.DeskSandbox.Worker.Reason == "" || !got.DeskSandbox.IframeSandbox {
		t.Fatalf("desk_sandbox = %+v", got.DeskSandbox)
	}
	for _, c := range []struct {
		method, owner string
		want          int
	}{{http.MethodGet, "", http.StatusUnauthorized}, {http.MethodPost, "owner-1", http.StatusMethodNotAllowed}} {
		rec = httptest.NewRecorder()
		h.HandleDesksState(rec, authenticatedCostsRequest(c.method, "/api/desks/state", c.owner))
		if rec.Code != c.want {
			t.Errorf("%s as %q: %d, want %d", c.method, c.owner, rec.Code, c.want)
		}
	}
}
