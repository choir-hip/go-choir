package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func getSubCalls(t *testing.T, h *APIHandler, owner, query string) (int, subCallsResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/subcalls"+query, nil)
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	rec := httptest.NewRecorder()
	h.HandleSubCalls(rec, req)
	var resp subCallsResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v %s", err, rec.Body)
		}
	}
	return rec.Code, resp
}

func TestSubCallsRouteReadsTheLedger(t *testing.T) {
	f := newSubCallFixture(t, nil)
	h := &APIHandler{rt: f.rt}

	// An empty ledger is 200 with empty arrays and maps.
	code, resp := getSubCalls(t, h, f.owner, "")
	if code != http.StatusOK || len(resp.Records) != 0 || resp.Summary.ByKind == nil || resp.Summary.ByClass == nil ||
		resp.SelfFavor.Groups == nil || resp.SelfFavor.Rows == nil || resp.Inert.Rates == nil || resp.Limit != 100 {
		t.Fatalf("empty ledger: %d %+v", code, resp)
	}
	if code, _ := getSubCalls(t, h, "", ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/subcalls", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	rec := httptest.NewRecorder()
	h.HandleSubCalls(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}

	// Seed: three Casts, two with Reports, one judge close, one settlement.
	f.writeCall(t, "fan-a", true)
	f.writeCall(t, "fan-b", true)
	castC := f.writeCall(t, "fan-c", false)
	f.rt.closeJudgeWork(context.Background(), f.owner, f.computer, types.SubCallJudgeWorkID(castC.RecordID, 0), "fan-c", castC.RecordID,
		types.SubCallPhaseInterrupted, "interrupted by a restart", "", 0, 0, 0)
	settle := castC
	settle.RecordID = "settle-c"
	sub := *castC.SubCall
	sub.Kind, sub.Class, sub.CastID = types.SubCallSettlement, types.DiscrepancyContradicted, castC.RecordID
	settle.SubCall = &sub
	if err := f.rt.appendSubCallRecord(context.Background(), f.owner, f.computer, settle); err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	code, resp = getSubCalls(t, h, f.owner, "")
	if code != http.StatusOK {
		t.Fatalf("seeded: %d", code)
	}
	if len(resp.Records) != 7 || resp.Summary.Window != 7 {
		t.Fatalf("records %d window %d", len(resp.Records), resp.Summary.Window)
	}
	want := map[string]int{types.SubCallCast: 3, types.SubCallReport: 2, types.SubCallJudgeWork: 0, types.SubCallJudgeClose: 1, types.SubCallSettlement: 1}
	for k, n := range want {
		if resp.Summary.ByKind[k] != n {
			t.Fatalf("kind %s = %d, want %d (%v)", k, resp.Summary.ByKind[k], n, resp.Summary.ByKind)
		}
	}
	if resp.Summary.ByClass[string(types.DiscrepancyContradicted)] != 1 {
		t.Fatalf("by class: %v", resp.Summary.ByClass)
	}

	// Limit: clamped to [1, 500], default 100; the page is the newest N.
	if _, r := getSubCalls(t, h, f.owner, "?limit=2"); len(r.Records) != 2 || r.Limit != 2 || r.Summary.Window != 7 {
		t.Fatalf("limit 2: %d %d %d", len(r.Records), r.Limit, r.Summary.Window)
	}
	for q, want := range map[string]int{"?limit=9999": 500, "?limit=0": 100, "?limit=-3": 100, "?limit=abc": 100, "?limit=500": 500} {
		if _, r := getSubCalls(t, h, f.owner, q); r.Limit != want {
			t.Fatalf("%s: limit %d, want %d", q, r.Limit, want)
		}
	}

	// Another owner sees none of it.
	code, other := getSubCalls(t, h, "owner-other", "")
	if code != http.StatusOK || len(other.Records) != 0 || len(other.Summary.ByKind) != 0 {
		t.Fatalf("owner B read owner A's ledger: %d %+v", code, other)
	}
}
