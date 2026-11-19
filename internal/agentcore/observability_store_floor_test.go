package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// store_floor is the store's schema floor, read by an operator before a
// Store-Floor-Override (scripts/check-store-schema-floor.sh,
// scripts/rollback/fc-deploy1-rollback.sh). Omitted while there is none.
func TestObservabilityShowsTheStoreFloorOnlyWhenOne(t *testing.T) {
	rt := telemetryRuntime(t, "computer-store-floor")
	read := func() (uint64, bool) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
		req.Header.Set("X-Authenticated-User", "owner-telemetry")
		rec := httptest.NewRecorder()
		(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
		var obs map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
		}
		raw, ok := obs["store_floor"]
		if !ok {
			return 0, false
		}
		var v uint64
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("store_floor = %s: %v", raw, err)
		}
		return v, true
	}
	if v, ok := read(); ok {
		t.Fatalf("a store with no floor showed store_floor=%d", v)
	}
	if err := storeschema.WriteFloor(rt.store.TexturePath(), 2, "test floor"); err != nil {
		t.Fatal(err)
	}
	if v, ok := read(); !ok || v != 2 {
		t.Fatalf("a store floored at 2 showed store_floor=%d present=%v", v, ok)
	}
}
