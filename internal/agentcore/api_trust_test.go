package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustRouteDerivesTheViewAndNeverStoresIt(t *testing.T) {
	f := newSubCallFixture(t, nil)
	h := &APIHandler{rt: f.rt}
	get := func(owner, query string) (int, trustResponse) {
		req := httptest.NewRequest(http.MethodGet, "/api/trust"+query, nil)
		if owner != "" {
			req.Header.Set("X-Authenticated-User", owner)
		}
		rec := httptest.NewRecorder()
		h.HandleTrust(rec, req)
		var resp trustResponse
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v %s", err, rec.Body)
			}
		}
		return rec.Code, resp
	}
	if code, _ := get("", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", code)
	}
	code, resp := get(f.owner, "?replay=1")
	if code != http.StatusOK || resp.View.TableDigest == "" || resp.Replay == nil || len(resp.Replay.Candidates) != 2 {
		t.Fatalf("empty ledger: %d %+v", code, resp)
	}
	// MA7.9: the replay serves the pre-registered units and the measured
	// taps, empty and present (an absent field means an older build).
	if resp.PreregUnits == nil || resp.PreregUnits.Units == nil || resp.PreregUnits.Window != "336h0m0s" {
		t.Fatalf("prereg units: %+v", resp.PreregUnits)
	}
	if _, plain := get(f.owner, ""); plain.PreregUnits != nil || plain.GatedActs != nil {
		t.Fatal("the plain view carried the replay's units")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trust", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	rec := httptest.NewRecorder()
	h.HandleTrust(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}
}
