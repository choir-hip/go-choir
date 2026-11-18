package agentcore

// The settlement slice. The failure list, written before the code:
//
//	L1  a committed segment is not on the server's run sheet with its claim id,
//	    or the segment event does not name that entry and claim;
//	L2  a replay of the request_id, or a phone's own place of the same id, makes
//	    a second entry or a second op record;
//	L3  a finished op does not settle true, a skipped or cut op false;
//	L4  a left or failed op settles anything, or writes no no-verdict record;
//	L5  a replayed op, a different op for an already settled entry, or a
//	    reordered batch writes a second or a changed settlement;
//	L6  an op later than the forecast's deadline settles, or is not recorded;
//	L7  an entry whose claim id is not its own segment's claim settles;
//	L8  the learning route is served unauthenticated, wrongly counts, or does
//	    not show the settling op and the no-verdict rows to the listener;
//	L9  the programme route has a path to a settlement (F41 holds that).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/runsheet"
)

func sixStories() radioScript {
	var lines []string
	for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
		lines = append(lines, titled("Story "+n, nil))
	}
	return linesScript(lines...)
}

// phoneEnd is the pair of ops a phone posts for an entry it plays and ends.
func phoneEnd(entryID, tag, state, outcome string, last int) []map[string]any {
	on := map[string]any{"op_id": "on-" + tag, "by": "phone", "op": "aired", "id": entryID, "state": "on_air", "base_version": 0, "position_seconds": 0}
	end := map[string]any{"op_id": "end-" + tag, "by": "phone", "op": "aired", "id": entryID, "state": state, "base_version": 0,
		"ended": map[string]any{"outcome": outcome, "at": time.Now().UTC().Format(time.RFC3339Nano), "last_block": last}, "position_seconds": 12.5}
	return []map[string]any{on, end}
}

func learningGet(t *testing.T, h *APIHandler, query string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/autoradio/learning"+query, nil)
	req.Header.Set("X-Authenticated-User", progOwner)
	rec := httptest.NewRecorder()
	h.HandleAutoradioLearning(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func settlementOf(t *testing.T, rt *Runtime, claimID string) string {
	t.Helper()
	rec := getRecord(t, rt, calibration.SettlementRecordID(claimID, "rule", calibration.AutoradioHeardRule))
	if rec == nil {
		return ""
	}
	return rec.Calibration.Settlement.Outcome
}

func noVerdicts(t *testing.T, rt *Runtime) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, r := range tape(t, rt, kindNoVerdict) {
		out[r.SubjectRefs["claim_id"]] = r.SubjectRefs["reason"]
	}
	return out
}

func runStretch(t *testing.T, rt *Runtime) (sid string, entries []string, claims []string, evs []sseEvent) {
	t.Helper()
	_, evs = progRun(t, rt, progBody())
	sid = stretchIDOf(t, evs)
	for _, s := range segmentEvents(evs) {
		entries = append(entries, s["entry_id"].(string))
		c, _ := s["claim_id"].(string)
		claims = append(claims, c)
	}
	return
}

func TestL1_TheSegmentIsOnTheServersSheetWithItsForecast(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	sid, entries, claims, evs := runStretch(t, rt)
	if len(entries) != 3 || entries[1] != "d-"+sid+".s1" || claims[0] != "" || claims[1] == "" || claims[2] == "" {
		t.Fatalf("(L1) event ids: %v %v", entries, claims)
	}
	sheet := getSheet(t, h)
	if len(sheet.Entries) != 3 {
		t.Fatalf("(L1) %d entries on the sheet", len(sheet.Entries))
	}
	for i, e := range sheet.Entries {
		if e.ID != entries[i] || e.PlacedBy != runsheet.ByDesk || e.State != runsheet.StateAhead {
			t.Errorf("(L1) entry %d: %+v", i, e)
		}
		if (claims[i] == "") != (e.Forecast == nil) || (e.Forecast != nil && e.Forecast.ClaimID != claims[i]) {
			t.Errorf("(L1) entry %d forecast %+v, event claim %q", i, e.Forecast, claims[i])
		}
	}
	if sheet.Entries[1].Forecast.PHeard != 0.7 || sheet.Entries[1].Forecast.CausedBy != "importance" {
		t.Errorf("(L1) forecast: %+v", sheet.Entries[1].Forecast)
	}
	if d := eventsNamed(evs, "done"); len(d) != 1 || d[0].Data["placed"].(float64) != 3 {
		t.Errorf("(L1) done: %+v", d)
	}
}

func TestL2_ReplayAndThePhonesOwnPlaceAreNoOps(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	_, entries, claims, _ := runStretch(t, rt)
	before := len(opRecords(t, rt))
	_, evs2 := progRun(t, rt, progBody()) // the same request_id: a replay
	for i, s := range segmentEvents(evs2) {
		if s["entry_id"] != entries[i] || (s["claim_id"] == nil) != (claims[i] == "") {
			t.Errorf("(L2) replay event %d: %v", i, s)
		}
	}
	if after := len(opRecords(t, rt)); after != before || len(getSheet(t, h).Entries) != 3 {
		t.Errorf("(L2) replay wrote ops: %d -> %d", before, after)
	}
	// A phone that posts a place of the same id: dropped, not a duplicate.
	_, a := postOps(t, h, placeOp("ph-1", "phone", "end", storyEntry(entries[1], "phone")))
	if a.Results[0].Applied || a.Results[0].Dropped != runsheet.DroppedDuplicateID {
		t.Errorf("(L2) phone place: %+v", a.Results[0])
	}
	if len(getSheet(t, h).Entries) != 3 {
		t.Errorf("(L2) duplicate entry")
	}
}

func TestL3toL5_TheOpsSettleTheForecasts(t *testing.T) {
	rt, _ := radioRuntime(t, sixStories())
	h := NewAPIHandler(rt)
	_, entries, claims, _ := runStretch(t, rt)
	var ops []map[string]any
	ops = append(ops, phoneEnd(entries[0], "a", "heard", "finished", 1)...)
	ops = append(ops, phoneEnd(entries[1], "b", "heard", "finished", 1)...)
	ops = append(ops, phoneEnd(entries[2], "c", "skipped", "skipped", 0)...)
	// D is cut: it goes on air, then the listener places another at now.
	ops = append(ops, phoneEnd(entries[3], "d", "heard", "finished", 0)[0])
	ops = append(ops, placeOp("cut-d", "listener", "now", storyEntry("L-cut", "listener")))
	ops = append(ops, phoneEnd("L-cut", "lc", "heard", "finished", 1)[1])
	ops = append(ops, phoneEnd(entries[4], "e", "heard", "left", 0)...)
	a := mustOps(t, h, ops...)
	for _, r := range a.Results {
		if !r.Applied {
			t.Fatalf("(L3) op dropped: %+v", r)
		}
	}
	want := []string{"true", "true", "false", "false", "", ""}
	for i, w := range want {
		if got := settlementOf(t, rt, claims[i]); got != w {
			t.Errorf("(L3) entry %d settlement %q, want %q", i, got, w)
		}
	}
	nv := noVerdicts(t, rt)
	if nv[claims[4]] != "left" || len(nv) != 1 {
		t.Errorf("(L4) no-verdict records: %v", nv)
	}
	// L5: the same ops again, a contrary op for a settled entry, a reorder.
	n := len(tape(t, rt, "autoradio.settlement"))
	mustOps(t, h, ops...)
	mustOps(t, h, append(phoneEnd(entries[0], "a2", "skipped", "skipped", 0), phoneEnd(entries[0], "a3", "heard", "finished", 1)...)...)
	if got := len(tape(t, rt, "autoradio.settlement")); got != n || settlementOf(t, rt, claims[0]) != "true" {
		t.Errorf("(L5) settlements %d -> %d, first %q", n, got, settlementOf(t, rt, claims[0]))
	}
	// failed: no verdict on entry F.
	mustOps(t, h, phoneEnd(entries[5], "f", "skipped", "failed", 0)...)
	if settlementOf(t, rt, claims[5]) != "" || noVerdicts(t, rt)[claims[5]] != "failed" {
		t.Errorf("(L4) failed: %q %v", settlementOf(t, rt, claims[5]), noVerdicts(t, rt))
	}

	// L8: the listener reads it.
	code, out := learningGet(t, h, "")
	if code != 200 {
		t.Fatalf("(L8) %d %v", code, out)
	}
	card := out["scorecard"].(map[string]any)
	if card["settled"].(float64) != 4 || card["heard_through"].(float64) != 2 || card["no_verdict"].(float64) != 2 || card["forecasts"].(float64) != 6 {
		t.Errorf("(L8) scorecard: %v", card)
	}
	rows := out["rows"].([]any)
	if len(rows) != 6 {
		t.Fatalf("(L8) %d rows", len(rows))
	}
	byClaim := map[string]map[string]any{}
	for _, r := range rows {
		m := r.(map[string]any)
		byClaim[m["claim_id"].(string)] = m
	}
	r0 := byClaim[claims[0]]
	if r0["status"] != "settled" || r0["outcome"] != "true" || r0["title"] != "Story A" || r0["op"].(map[string]any)["op_id"] != "end-a" {
		t.Errorf("(L8) row 0: %v", r0)
	}
	if r4 := byClaim[claims[4]]; r4["status"] != "no_verdict" || r4["no_verdict_reason"] != "left" || r4["op"].(map[string]any)["position_seconds"].(float64) != 12.5 {
		t.Errorf("(L8) row 4: %v", r4)
	}
	// Paging.
	_, p1 := learningGet(t, h, "?limit=4")
	if len(p1["rows"].([]any)) != 4 || p1["next"] == nil {
		t.Fatalf("(L8) page 1: %v", p1)
	}
	_, p2 := learningGet(t, h, "?limit=4&after="+p1["next"].(string))
	if len(p2["rows"].([]any)) != 2 || p2["next"] != nil {
		t.Errorf("(L8) page 2: %v", p2)
	}
}

func TestL6_ALateOpDoesNotSettle(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	_, entries, claims, _ := runStretch(t, rt)
	settleNow = func() time.Time { return time.Now().Add(calibration.AutoradioLapse + time.Hour) }
	defer func() { settleNow = time.Now }()
	late := phoneEnd(entries[1], "late", "heard", "finished", 1)
	late[1]["ended"].(map[string]any)["at"] = time.Now().Add(calibration.AutoradioLapse + 30*time.Minute).UTC().Format(time.RFC3339Nano)
	mustOps(t, h, late...)
	if settlementOf(t, rt, claims[1]) != "" || noVerdicts(t, rt)[claims[1]] != "late" {
		t.Errorf("(L6) %q %v", settlementOf(t, rt, claims[1]), noVerdicts(t, rt))
	}
}

func TestL7_AnEntryWithSomeoneElsesClaimSettlesNothing(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	_, _, claims, _ := runStretch(t, rt)
	forged := storyEntry("d-forged", "desk")
	forged["forecast"] = map[string]any{"p_heard": 0.5, "caused_by": "importance", "claim_id": claims[1]}
	_, err := h.applySheetOps(context.Background(), progOwner, sheetOwnerID, []runsheet.Op{{OpID: "o-forged", By: runsheet.ByDesk, Op: runsheet.OpPlace, At: runsheet.AtEnd, Reason: "test", Entry: entryOf(t, forged)}}, true)
	if err != nil {
		t.Fatal(err)
	}
	mustOps(t, h, phoneEnd("d-forged", "fg", "heard", "finished", 1)...)
	if settlementOf(t, rt, claims[1]) != "" {
		t.Errorf("(L7) a forged entry settled a forecast")
	}
}

func entryOf(t *testing.T, m map[string]any) *runsheet.Entry {
	t.Helper()
	b, _ := json.Marshal(m)
	var e runsheet.Entry
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	return &e
}

func TestL8_TheLearningRouteIsAuthenticatedAndReadOnly(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	h := NewAPIHandler(rt)
	req := httptest.NewRequest(http.MethodGet, "/api/autoradio/learning", nil)
	rec := httptest.NewRecorder()
	h.HandleAutoradioLearning(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("(L8) unauthenticated: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/autoradio/learning", strings.NewReader("{}"))
	req.Header.Set("X-Authenticated-User", progOwner)
	rec = httptest.NewRecorder()
	h.HandleAutoradioLearning(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("(L8) POST: %d", rec.Code)
	}
	if code, _ := learningGet(t, h, "?limit=0"); code != http.StatusBadRequest {
		t.Errorf("(L8) limit 0: %d", code)
	}
	if code, out := learningGet(t, h, ""); code != 200 || len(out["rows"].([]any)) != 0 {
		t.Errorf("(L8) empty: %d %v", code, out)
	}
}

func TestL9_ARecordOfTheMaximalForecastFitsTheBound(t *testing.T) {
	rt, _ := sheetRT(t)
	h := NewAPIHandler(rt)
	e := storyEntry("d-max", "desk")
	e["forecast"] = map[string]any{"p_heard": 1, "caused_by": strings.Repeat("c", 160), "claim_id": strings.Repeat("x", 160)}
	res, err := h.applySheetOps(context.Background(), progOwner, sheetOwnerID, []runsheet.Op{{OpID: "o-max", By: runsheet.ByDesk, Op: runsheet.OpPlace, At: runsheet.AtEnd, Reason: "t", Entry: entryOf(t, e)}}, true)
	if err != nil || !res[0].Applied {
		t.Fatalf("(L9) %v %+v", err, res)
	}
}
