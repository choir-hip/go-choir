package agentcore

// Slice D1: the server run sheet (docs: autoradio desk slices, "Slice D1").
// The failure list, written before the code:
//
//	F1  an op replayed by op_id writes a second record, bumps the version, or
//	    returns another result (applied or dropped), also after the reducer's
//	    own window of 512 and after a restart;
//	F2  two batches arriving in either order, or at once, give anything but
//	    one history of versions 1..n with one record each;
//	F3  an aired from a desk is accepted, from the route or in process;
//	F4  the stream does not start with ": autoradio", does not send a full
//	    sheet when after is absent, older than kept or ahead of the server,
//	    or does not echo the phone's own ops with the server's versions;
//	F5  a restart (a fresh store over the same tape) rebuilds another sheet,
//	    also across a snapshot, or resumes anything;
//	F6  a batch over 64 ops is not 400, a body over 256 KB is not 413, a
//	    malformed field is not named, an unknown op is accepted;
//	F7  a place beyond 100 ahead is silently trimmed instead of dropped with
//	    a named reason;
//	F8  a stranger can place an entry as the desk, send by:desk over the
//	    route, or send more than the hourly bound;
//	F9  an unauthenticated call is served, a wrong method is not 405;
//	F10 a record of the tape is not owner-private, not a notice, or lacks the
//	    record kind and labels the table demands;
//	F11 the tape is written for a dropped op, or the memory moves when the
//	    tape write fails;
//	F12 (superseded by R3: a replay never refuses; see below);
//	F13 the reserved run sheet kind (the name the test below builds) is still cited anywhere.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/runsheet"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func sheetRT(t *testing.T) (*Runtime, *APIHandler) {
	t.Helper()
	rt, _ := testRuntime(t)
	return rt, NewAPIHandler(rt)
}

func storyEntry(id, by string) map[string]any {
	return map[string]any{"id": id, "kind": "story", "placed_by": by, "state": "ahead", "title": "T " + id, "text": "Words of " + id + ".", "est_seconds": 30}
}

func placeOp(opID, by, at string, entry map[string]any) map[string]any {
	return map[string]any{"op_id": opID, "by": by, "op": "place", "at": at, "entry": entry, "base_version": 0}
}

func sheetDo(h *APIHandler, method, path, body string, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", progOwner)
	for _, m := range mutate {
		m(req)
	}
	rec := httptest.NewRecorder()
	switch {
	case strings.HasPrefix(path, "/api/autoradio/sheet/ops"):
		h.HandleAutoradioSheetOps(rec, req)
	case strings.HasPrefix(path, "/api/autoradio/sheet/stream"):
		h.HandleAutoradioSheetStream(rec, req)
	default:
		h.HandleAutoradioSheet(rec, req)
	}
	return rec
}

type opsAnswer struct {
	Version int64             `json:"version"`
	Results []runsheet.Result `json:"results"`
	Error   string            `json:"error"`
}

func postOps(t *testing.T, h *APIHandler, ops ...map[string]any) (*httptest.ResponseRecorder, opsAnswer) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"ops": ops})
	rec := sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", string(b))
	var a opsAnswer
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	return rec, a
}

func mustOps(t *testing.T, h *APIHandler, ops ...map[string]any) opsAnswer {
	t.Helper()
	rec, a := postOps(t, h, ops...)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST ops: %d %s", rec.Code, rec.Body.String())
	}
	return a
}

func getSheet(t *testing.T, h *APIHandler) runsheet.RunSheet {
	t.Helper()
	rec := sheetDo(h, http.MethodGet, "/api/autoradio/sheet", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET sheet: %d %s", rec.Code, rec.Body.String())
	}
	var s runsheet.RunSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func opRecords(t *testing.T, rt *Runtime) []types.CommitmentRecord {
	t.Helper()
	recs, err := rt.store.ListCommitmentRecordsBySubjectRef(context.Background(), progOwner, rt.cfg.ComputerID, "sheet_ops", sheetOwnerID, 100000)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func restart(rt *Runtime) *APIHandler {
	rt.autoradioSheets = nil // a new process: nothing in memory
	return NewAPIHandler(rt)
}

func ids(s runsheet.RunSheet) string {
	var out []string
	for _, e := range s.Entries {
		out = append(out, e.ID+":"+string(e.State))
	}
	return strings.Join(out, ",")
}

// F1
func TestSheetReplayOfAnOpIDWritesNothingAndReturnsTheSameResult(t *testing.T) {
	rt, h := sheetRT(t)
	batch := []map[string]any{
		placeOp("p-1", "phone", "end", storyEntry("e1", "phone")),
		placeOp("p-2", "phone", "end", storyEntry("e1", "phone")),                         // duplicate_id: dropped
		{"op_id": "p-3", "by": "phone", "op": "remove", "id": "ghost", "base_version": 0}, // not_ahead: dropped
	}
	first := mustOps(t, h, batch...)
	if len(first.Results) != 3 || !first.Results[0].Applied || first.Results[1].Dropped != runsheet.DroppedDuplicateID || first.Results[2].Dropped != runsheet.DroppedNotAhead {
		t.Fatalf("first results: %+v", first.Results)
	}
	if got := len(opRecords(t, rt)); got != 1 {
		t.Fatalf("records after the first batch = %d, want 1 (a dropped op writes nothing)", got)
	}
	second := mustOps(t, h, batch...)
	if fmt.Sprint(second) != fmt.Sprint(first) {
		t.Fatalf("replay answered %+v, first answered %+v", second, first)
	}
	if got := len(opRecords(t, rt)); got != 1 {
		t.Fatalf("records after the replay = %d, want 1", got)
	}
	// After a restart the applied op is still the same answer: the record
	// index, not the reducer's window, is the memory of an applied op.
	h = restart(rt)
	third := mustOps(t, h, batch[0])
	if len(third.Results) != 1 || !third.Results[0].Applied || third.Results[0].Version != 1 {
		t.Fatalf("replay after restart: %+v", third)
	}
	if got := len(opRecords(t, rt)); got != 1 || getSheet(t, h).Version != 1 {
		t.Fatalf("restart replay wrote: records %d", got)
	}
}

func TestSheetReplayBeyondTheReducersWindowStillWritesNothing(t *testing.T) {
	rt, h := sheetRT(t)
	// 520 applied ops: the reducer remembers the last 512 and no more.
	for i := 0; i < 520; i += 40 {
		var ops []map[string]any
		for j := i; j < i+40; j++ {
			ops = append(ops, placeOp(fmt.Sprintf("p-%d", j), "phone", "end", map[string]any{
				"id": fmt.Sprintf("e%d", j), "kind": "page", "placed_by": "phone", "state": "ahead", "title": "t", "est_seconds": 1}))
		}
		// stay under the ahead bound by clearing between rounds
		mustOps(t, h, ops...)
		cur := getSheet(t, h)
		mustOps(t, h, map[string]any{"op_id": fmt.Sprintf("c-%d", i), "by": "listener", "op": "clear", "base_version": cur.Version})
	}
	before := len(opRecords(t, rt))
	h = restart(rt)
	a := mustOps(t, h, placeOp("p-0", "phone", "end", map[string]any{"id": "e0", "kind": "page", "placed_by": "phone", "state": "ahead", "title": "t", "est_seconds": 1}))
	if !a.Results[0].Applied || a.Results[0].Version != 1 {
		t.Fatalf("an op older than the window came back as %+v, want its original result", a.Results[0])
	}
	if len(opRecords(t, rt)) != before {
		t.Fatal("the replay wrote a record")
	}
}

// F2
func TestSheetTwoBatchesGiveOneSequencedHistoryInEitherOrder(t *testing.T) {
	for _, order := range []string{"ab", "ba"} {
		t.Run(order, func(t *testing.T) {
			rt, h := sheetRT(t)
			a := []map[string]any{placeOp("a-1", "phone", "end", storyEntry("a1", "phone")), placeOp("a-2", "phone", "end", storyEntry("a2", "phone"))}
			b := []map[string]any{placeOp("b-1", "listener", "end", storyEntry("b1", "listener"))}
			batches := map[byte][]map[string]any{'a': a, 'b': b}
			var versions []int64
			for _, c := range []byte(order) {
				ans := mustOps(t, h, batches[c]...)
				for _, r := range ans.Results {
					versions = append(versions, r.Version)
				}
			}
			if fmt.Sprint(versions) != "[1 2 3]" {
				t.Fatalf("versions %v", versions)
			}
			if len(opRecords(t, rt)) != 3 {
				t.Fatal("want 3 records")
			}
		})
	}
}

func TestSheetConcurrentBatchesAreSequenced(t *testing.T) {
	rt, h := sheetRT(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b, _ := json.Marshal(map[string]any{"ops": []any{
				placeOp(fmt.Sprintf("g-%d-a", i), "phone", "end", storyEntry(fmt.Sprintf("g%da", i), "phone")),
				placeOp(fmt.Sprintf("g-%d-b", i), "phone", "end", storyEntry(fmt.Sprintf("g%db", i), "phone")),
			}})
			sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", string(b))
		}(i)
	}
	wg.Wait()
	recs := opRecords(t, rt)
	seen := map[string]bool{}
	for _, r := range recs {
		seen[r.SubjectRefs["version"]] = true
	}
	for v := 1; v <= 16; v++ {
		if !seen[fmt.Sprint(v)] {
			t.Fatalf("version %d missing from the tape (%d records)", v, len(recs))
		}
	}
	if len(recs) != 16 || getSheet(t, h).Version != 16 {
		t.Fatalf("records %d version %d", len(recs), getSheet(t, h).Version)
	}
}

// F3
func TestSheetAiredFromADeskIsRefused(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	aired := map[string]any{"op_id": "x-1", "by": "desk", "op": "aired", "id": "e1", "state": "on_air", "reason": "r", "base_version": 1}
	a := mustOps(t, h, aired)
	if a.Results[0].Applied || a.Results[0].Dropped != droppedByDeskRefused {
		t.Fatalf("route: %+v", a.Results[0])
	}
	// In process, where a desk may send ops, the reducer's own rule holds.
	var op runsheet.Op
	b, _ := json.Marshal(aired)
	_ = json.Unmarshal(b, &op)
	op.OpID = "x-2"
	res, err := h.applySheetOps(context.Background(), progOwner, sheetOwnerID, []runsheet.Op{op}, true)
	if err != nil || res[0].Applied || res[0].Dropped != runsheet.DroppedNotPhone {
		t.Fatalf("in process: %+v %v", res, err)
	}
	if len(opRecords(t, rt)) != 1 {
		t.Fatal("a refused aired wrote a record")
	}
	// The phone's aired applies.
	a = mustOps(t, h, map[string]any{"op_id": "x-3", "by": "phone", "op": "aired", "id": "e1", "state": "on_air", "base_version": 1})
	if !a.Results[0].Applied || a.Results[0].Version != 2 {
		t.Fatalf("phone aired: %+v", a.Results[0])
	}
}

func TestSheetADeskMayPlaceInProcessWithAReason(t *testing.T) {
	_, h := sheetRT(t)
	var op runsheet.Op
	b, _ := json.Marshal(placeOp("d-1", "desk", "end", storyEntry("d-t1-1", "desk")))
	_ = json.Unmarshal(b, &op)
	res, err := h.applySheetOps(context.Background(), progOwner, sheetOwnerID, []runsheet.Op{op}, true)
	if err != nil || res[0].Dropped != runsheet.DroppedReasonNeeded {
		t.Fatalf("no reason: %+v %v", res, err)
	}
	op.OpID, op.Reason = "d-2", "the listener asked"
	res, err = h.applySheetOps(context.Background(), progOwner, sheetOwnerID, []runsheet.Op{op}, true)
	if err != nil || !res[0].Applied {
		t.Fatalf("with reason: %+v %v", res, err)
	}
}

// F8
func TestSheetAStrangerCannotPlaceAsTheDeskOrLieAboutWhoPlacedAnEntry(t *testing.T) {
	_, h := sheetRT(t)
	deskPlace := placeOp("s-1", "desk", "end", storyEntry("e1", "desk"))
	deskPlace["reason"] = "I am the desk"
	liar := placeOp("s-2", "phone", "end", storyEntry("e2", "desk"))
	a := mustOps(t, h, deskPlace, liar)
	if a.Results[0].Dropped != droppedByDeskRefused || a.Results[1].Dropped != droppedPlacedByMismatch {
		t.Fatalf("%+v", a.Results)
	}
	if len(getSheet(t, h).Entries) != 0 {
		t.Fatal("a refused place reached the sheet")
	}
}

// F7
func TestSheetAheadIsBoundedAndTheDropNamesItsReason(t *testing.T) {
	_, h := sheetRT(t)
	var ops []map[string]any
	for i := 0; i < 64; i++ {
		ops = append(ops, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	mustOps(t, h, ops...)
	ops = ops[:0]
	for i := 64; i < 104; i++ {
		ops = append(ops, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	a := mustOps(t, h, ops...)
	applied, full := 0, 0
	for _, r := range a.Results {
		switch {
		case r.Applied:
			applied++
		case r.Dropped == droppedAheadFull:
			full++
		default:
			t.Fatalf("unexpected result %+v", r)
		}
	}
	if applied != 36 || full != 4 {
		t.Fatalf("applied %d, ahead_full %d", applied, full)
	}
	s := getSheet(t, h)
	if len(s.Entries) != 100 {
		t.Fatalf("ahead %d, want exactly 100", len(s.Entries))
	}
	// A skip-then-place frees nothing here; a remove does.
	a = mustOps(t, h, map[string]any{"op_id": "r-1", "by": "listener", "op": "remove", "id": "e0", "base_version": 0},
		placeOp("p-200", "phone", "end", storyEntry("late", "phone")))
	if !a.Results[0].Applied || !a.Results[1].Applied {
		t.Fatalf("%+v", a.Results)
	}
}

// F6
func TestSheetOpsRouteChecksShapeAndBounds(t *testing.T) {
	_, h := sheetRT(t)
	good := `{"op_id":"p-1","by":"phone","op":"place","at":"end","entry":` + jsonStr(storyEntry("e1", "phone")) + `,"base_version":0}`
	cases := []struct {
		name, body string
		code       int
		want       string
	}{
		{"not json", `{`, 400, "body"},
		{"no ops", `{}`, 400, "ops"},
		{"empty ops", `{"ops":[]}`, 400, "ops"},
		{"ops not an array", `{"ops":3}`, 400, "ops"},
		{"unknown op", `{"ops":[{"op_id":"p-1","by":"phone","op":"teleport","base_version":0}]}`, 400, "ops[0]"},
		{"missing op", `{"ops":[{"op_id":"p-1","by":"phone","base_version":0}]}`, 400, "ops[0].op"},
		{"bad op_id", `{"ops":[{"op_id":"no good","by":"phone","op":"skip","id":"x","base_version":0}]}`, 400, "ops[0].op_id"},
		{"missing by", `{"ops":[{"op_id":"p-1","op":"skip","id":"x","base_version":0}]}`, 400, "ops[0].by"},
		{"bad kind", `{"ops":[` + good + `,{"op_id":"p-2","by":"phone","op":"place","at":"end","entry":{"id":"e2","kind":"banana","placed_by":"phone","state":"ahead","title":"","est_seconds":1},"base_version":0}]}`, 400, "ops[1]"},
		{"ok", `{"ops":[` + good + `]}`, 200, ""},
	}
	for _, c := range cases {
		rec := sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", c.body)
		if rec.Code != c.code || (c.want != "" && !strings.Contains(rec.Body.String(), c.want)) {
			t.Errorf("%s: %d %s (want %d naming %q)", c.name, rec.Code, rec.Body.String(), c.code, c.want)
		}
	}
	// 65 ops
	var many []string
	for i := 0; i < 65; i++ {
		many = append(many, fmt.Sprintf(`{"op_id":"m-%d","by":"phone","op":"skip","id":"x","base_version":0}`, i))
	}
	rec := sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", `{"ops":[`+strings.Join(many, ",")+`]}`)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "64") {
		t.Errorf("65 ops: %d %s", rec.Code, rec.Body.String())
	}
	// 64 ops pass the count.
	rec = sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", `{"ops":[`+strings.Join(many[:64], ",")+`]}`)
	if rec.Code != 200 {
		t.Errorf("64 ops: %d %s", rec.Code, rec.Body.String())
	}
	// a body over 256 KB
	big := `{"ops":[],"pad":"` + strings.Repeat("x", 256*1024) + `"}`
	rec = sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("big body: %d", rec.Code)
	}
}

func jsonStr(v any) string { b, _ := json.Marshal(v); return string(b) }

// F9
func TestSheetRoutesNeedAnOwnerAndTheRightMethod(t *testing.T) {
	_, h := sheetRT(t)
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/autoradio/sheet"}, {"POST", "/api/autoradio/sheet/ops"}, {"GET", "/api/autoradio/sheet/stream"},
	} {
		rec := sheetDo(h, c.method, c.path, `{"ops":[]}`, func(r *http.Request) { r.Header.Del("X-Authenticated-User") })
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s unauthenticated: %d", c.method, c.path, rec.Code)
		}
	}
	if rec := sheetDo(h, "POST", "/api/autoradio/sheet", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST sheet: %d", rec.Code)
	}
	if rec := sheetDo(h, "GET", "/api/autoradio/sheet/ops", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET ops: %d", rec.Code)
	}
	if rec := sheetDo(h, "POST", "/api/autoradio/sheet/stream", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST stream: %d", rec.Code)
	}
	if rec := sheetDo(h, "GET", "/api/autoradio/sheet/stream?after=abc", ""); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "after") {
		t.Errorf("bad after: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSheetOneOwnerNeverSeesAnothers(t *testing.T) {
	_, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	rec := sheetDo(h, "GET", "/api/autoradio/sheet", "", func(r *http.Request) { r.Header.Set("X-Authenticated-User", "owner-2") })
	var s runsheet.RunSheet
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if rec.Code != 200 || s.Version != 0 || len(s.Entries) != 0 {
		t.Fatalf("owner-2 sees %+v", s)
	}
}

func TestSheetEmptyAnswerHasAnEntriesArrayAndNoReducerMemory(t *testing.T) {
	_, h := sheetRT(t)
	rec := sheetDo(h, "GET", "/api/autoradio/sheet", "")
	if !strings.Contains(rec.Body.String(), `"entries":[]`) {
		t.Fatalf("%s", rec.Body.String())
	}
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	if strings.Contains(sheetDo(h, "GET", "/api/autoradio/sheet", "").Body.String(), "applied_ops") {
		t.Fatal("the reducer's memory leaks into the wire sheet")
	}
}

// F10
func TestSheetOpRecordIsAnOwnerPrivateNotice(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	rec, err := rt.store.GetCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, "autoradio-sheet-op:p-1")
	if err != nil || rec == nil {
		t.Fatalf("record: %v %v", rec, err)
	}
	label, _ := types.AutoradioKindLabel("autoradio.sheet_op")
	if rec.SubjectRefs["desk"] != autoradioDesk || rec.SubjectRefs["record_kind"] != "autoradio.sheet_op" ||
		rec.SubjectRefs["sheet_ops"] != sheetOwnerID || rec.SubjectRefs["version"] != "1" || rec.SubjectRefs["op_id"] != "p-1" {
		t.Fatalf("refs %+v", rec.SubjectRefs)
	}
	if !types.NewLabelSet(rec.Provenance.Labels...).Has(label) {
		t.Fatalf("labels %v lack %s", rec.Provenance.Labels, label)
	}
	if rec.Directive == nil || rec.Directive.Subtype != types.CommitmentDirectiveNotice || rec.Directive.Body == "" {
		t.Fatalf("directive %+v", rec.Directive)
	}
	if strings.Contains(rec.Directive.Body, "applied_ops") {
		t.Fatal("the op record carries reducer memory")
	}
}

// F11
type failingLedger struct {
	autoradioLedger
	fail bool
}

func (f *failingLedger) AppendCommitmentRecord(ctx context.Context, o, c string, rec types.CommitmentRecord) (string, error) {
	if f.fail {
		return "", errors.New("tape down")
	}
	return f.autoradioLedger.AppendCommitmentRecord(ctx, o, c, rec)
}

func TestSheetTapeFailureBeforeTheWriteDoesNotMoveTheSheet(t *testing.T) {
	rt, h := sheetRT(t)
	led := &failingLedger{autoradioLedger: rt.store}
	h.autoradioLedgerOverride = led
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	led.fail = true
	rec, _ := postOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), placeOp("p-3", "phone", "end", storyEntry("e3", "phone")))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("tape down: %d %s", rec.Code, rec.Body.String())
	}
	if s := getSheet(t, h); s.Version != 1 || ids(s) != "e1:ahead" {
		t.Fatalf("memory moved without the tape: %s v%d", ids(s), s.Version)
	}
	led.fail = false
	a := mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	if !a.Results[0].Applied || a.Results[0].Version != 2 {
		t.Fatalf("retry after the tape came back: %+v", a.Results[0])
	}
}

// F5
func TestSheetRestartRebuildsTheSameSheetFromTheTape(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h,
		placeOp("p-1", "phone", "end", storyEntry("e1", "phone")),
		placeOp("p-2", "listener", "end", storyEntry("e2", "listener")),
		placeOp("p-3", "phone", "end", storyEntry("e3", "phone")),
		map[string]any{"op_id": "a-1", "by": "phone", "op": "aired", "id": "e1", "state": "on_air", "position_seconds": 3, "base_version": 0},
		map[string]any{"op_id": "m-1", "by": "listener", "op": "move", "id": "e3", "at": "next", "base_version": 0},
		map[string]any{"op_id": "r-1", "by": "listener", "op": "rewrite", "id": "e2", "text": "New words.", "base_version": 0},
	)
	before := getSheet(t, h)
	h = restart(rt)
	after := getSheet(t, h)
	if jsonStr(before) != jsonStr(after) || after.Version != 6 || after.OnAir != "e1" {
		t.Fatalf("before %s\nafter  %s", jsonStr(before), jsonStr(after))
	}
	// New ops continue the numbering.
	a := mustOps(t, h, placeOp("p-9", "phone", "end", storyEntry("e9", "phone")))
	if a.Results[0].Version != 7 {
		t.Fatalf("%+v", a.Results)
	}
}

func TestSheetRestartAcrossASnapshotIsTheSameSheet(t *testing.T) {
	rt, h := sheetRT(t)
	for i := 0; i < 3*sheetSnapshotEvery/2; i++ {
		var op map[string]any
		if i%2 == 0 {
			op = placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone"))
		} else {
			op = map[string]any{"op_id": fmt.Sprintf("r-%d", i), "by": "listener", "op": "remove", "id": fmt.Sprintf("e%d", i-1), "base_version": 0}
		}
		if a := mustOps(t, h, op); !a.Results[0].Applied {
			t.Fatalf("op %d: %+v", i, a.Results[0])
		}
	}
	snaps, err := rt.store.ListCommitmentRecordsBySubjectRef(context.Background(), progOwner, rt.cfg.ComputerID, "sheet_snap", sheetOwnerID, 100)
	if err != nil || len(snaps) != 1 || snaps[0].SubjectRefs["version"] != fmt.Sprint(sheetSnapshotEvery) {
		t.Fatalf("snapshots %d %v", len(snaps), err)
	}
	before := getSheet(t, h)
	h = restart(rt)
	if after := getSheet(t, h); jsonStr(before) != jsonStr(after) {
		t.Fatalf("before %s\nafter  %s", jsonStr(before), jsonStr(after))
	}
	// A replay that read the snapshot read fewer op records than the tape holds.
	ls, err := rt.sheetStore().get(context.Background(), h, progOwner, sheetOwnerID)
	if err != nil || ls.replayed != 3*sheetSnapshotEvery/2-sheetSnapshotEvery {
		t.Fatalf("replayed %d ops (%v), want only those after the snapshot", ls.replayed, err)
	}
}

func TestSheetRestartResumesNothing(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	before := allRecordCount(t, rt)
	h = restart(rt)
	_ = getSheet(t, h)
	if allRecordCount(t, rt) != before {
		t.Fatal("a restart wrote records")
	}
}

// ---- review round 1 (2026-11-16): the failure list added before the code ----
//
//	R1  a tape append that errors after the write landed leaves memory behind
//	    the tape: the sheet must be dropped and replayed, the retry answered
//	    from a sheet that holds the record, and no second record may take a
//	    version;
//	R2  a tape hit ahead of memory is answered only after a replay;
//	R3  replay never refuses: a record that does not decode, a record today's
//	    reducer drops, a version gap and a duplicate version are skipped,
//	    stepped over or deduplicated, counted, logged, and a snapshot is
//	    written at once; records at or below the snapshot are not decoded;
//	R4  only a store read error is a 503, and it is not retried against the
//	    tape for a few seconds;
//	R5  replay pages until the snapshot is reached and another computer's rows
//	    cannot shorten the window;
//	R6  place now at 100 ahead applies; previous cannot push ahead past 100;
//	R7  a same-batch repeat of an op_id is a duplicate; a cached drop never
//	    reports a version below the sheet's;
//	R8  an answer the phone already said lands heard and never on air;
//	R9  idle sheets leave memory and come back by replay.

// landedThenErrored writes the record to the real store and reports an error.
type landedThenErrored struct {
	autoradioLedger
	armed bool
}

func (f *landedThenErrored) AppendCommitmentRecord(ctx context.Context, o, c string, rec types.CommitmentRecord) (string, error) {
	id, err := f.autoradioLedger.AppendCommitmentRecord(ctx, o, c, rec)
	if f.armed && err == nil && rec.SubjectRefs["record_kind"] == kindSheetOp {
		f.armed = false
		return "", errors.New("deadline exceeded after commit")
	}
	return id, err
}

func TestSheetAppendErrorAfterTheWriteLandedDropsTheSheetAndReplays(t *testing.T) {
	rt, h := sheetRT(t)
	led := &landedThenErrored{autoradioLedger: rt.store}
	h.autoradioLedgerOverride = led
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	led.armed = true
	rec, _ := postOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	// The retry is answered from a sheet that holds the record.
	a := mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	if !a.Results[0].Applied || a.Results[0].Version != 2 || a.Version != 2 {
		t.Fatalf("retry: %+v", a)
	}
	if s := getSheet(t, h); ids(s) != "e1:ahead,e2:ahead" || s.Version != 2 {
		t.Fatalf("sheet %s v%d", ids(s), s.Version)
	}
	// A different op takes version 3, not 2 again; the tape is one record per version.
	a = mustOps(t, h, placeOp("p-3", "phone", "end", storyEntry("e3", "phone")))
	if a.Results[0].Version != 3 {
		t.Fatalf("%+v", a.Results[0])
	}
	h = restart(rt)
	if s := getSheet(t, h); s.Version != 3 || ids(s) != "e1:ahead,e2:ahead,e3:ahead" {
		t.Fatalf("after restart %s v%d", ids(s), s.Version)
	}
}

func TestSheetATapeHitAheadOfMemoryIsAnsweredOnlyAfterAReplay(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	// A record lands that this process never folded (another process, or an
	// append whose acknowledgement was lost).
	ls, _ := rt.sheetStore().get(context.Background(), h, progOwner, sheetOwnerID)
	var op runsheet.Op
	_ = json.Unmarshal([]byte(jsonStr(placeOp("x-2", "phone", "end", storyEntry("ex", "phone")))), &op)
	rec, err := ls.opRecord(rt, progOwner, op, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, rec); err != nil {
		t.Fatal(err)
	}
	a := mustOps(t, h, placeOp("x-2", "phone", "end", storyEntry("ex", "phone")))
	if !a.Results[0].Applied || a.Results[0].Version != 2 || a.Version != 2 {
		t.Fatalf("%+v", a)
	}
	if s := getSheet(t, h); ids(s) != "e1:ahead,ex:ahead" {
		t.Fatalf("sheet %s", ids(s))
	}
}

// putOp writes an op record straight to the tape at a version.
func putOp(t *testing.T, rt *Runtime, h *APIHandler, op map[string]any, version int64, mutate ...func(*types.CommitmentRecord)) {
	t.Helper()
	var o runsheet.Op
	if err := json.Unmarshal([]byte(jsonStr(op)), &o); err != nil {
		t.Fatal(err)
	}
	ls := &liveSheet{ownerID: progOwner, sheetID: sheetOwnerID}
	rec, err := ls.opRecord(rt, progOwner, o, version)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mutate {
		m(&rec)
	}
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, rec); err != nil {
		t.Fatal(err)
	}
}

func snapshots(t *testing.T, rt *Runtime) []types.CommitmentRecord {
	t.Helper()
	recs, err := rt.store.ListCommitmentRecordsBySubjectRef(context.Background(), progOwner, rt.cfg.ComputerID, "sheet_snap", sheetOwnerID, 100)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func sheetLogCapture(t *testing.T) *strings.Builder {
	t.Helper()
	var b strings.Builder
	old := log.Writer()
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(old) })
	return &b
}

func TestSheetReplaySkipsWhatItCannotFoldCountsItAndSnapshotsAtOnce(t *testing.T) {
	rt, h := sheetRT(t)
	// version 1 good; 2 does not decode; 3 good; (4 missing); 5 a remove of a
	// ghost that today's reducer drops; 6 good; 6 again (a duplicate version).
	putOp(t, rt, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")), 1)
	putOp(t, rt, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), 2, func(r *types.CommitmentRecord) { r.Directive.Body = "{not json" })
	putOp(t, rt, h, placeOp("p-3", "phone", "end", storyEntry("e3", "phone")), 3)
	putOp(t, rt, h, map[string]any{"op_id": "r-5", "by": "listener", "op": "remove", "id": "ghost", "base_version": 0}, 5)
	putOp(t, rt, h, placeOp("p-6", "phone", "end", storyEntry("e6", "phone")), 6)
	time.Sleep(5 * time.Millisecond)
	putOp(t, rt, h, placeOp("p-6b", "phone", "end", storyEntry("e6b", "phone")), 6)
	out := sheetLogCapture(t)
	h = restart(rt)
	s := getSheet(t, h)
	if s.Version != 6 || ids(s) != "e1:ahead,e3:ahead,e6:ahead" {
		t.Fatalf("sheet %s v%d", ids(s), s.Version)
	}
	if !strings.Contains(out.String(), "autoradio sheet replay repaired owner=owner-1 skipped=2 gaps=1 dup=1 unreadable=0 truncated=false") {
		t.Fatalf("log %q", out.String())
	}
	if snaps := snapshots(t, rt); len(snaps) != 1 || snaps[0].SubjectRefs["version"] != "6" {
		t.Fatalf("snapshots %d", len(snaps))
	}
	// The damage is paid once: the next replay starts from the snapshot.
	out.Reset()
	h = restart(rt)
	if s2 := getSheet(t, h); jsonStr(s2) != jsonStr(s) {
		t.Fatalf("second replay differs")
	}
	if strings.Contains(out.String(), "repaired") {
		t.Fatalf("the second replay repaired again: %q", out.String())
	}
	// And the sheet keeps taking ops at the next version.
	if a := mustOps(t, h, placeOp("p-7", "phone", "end", storyEntry("e7", "phone"))); a.Results[0].Version != 7 {
		t.Fatalf("%+v", a.Results[0])
	}
}

func TestSheetReplayDoesNotDecodeRecordsAtOrBelowTheSnapshot(t *testing.T) {
	rt, h := sheetRT(t)
	for i := 0; i < sheetSnapshotEvery+4; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", map[string]any{
			"id": fmt.Sprintf("e%d", i), "kind": "page", "placed_by": "phone", "state": "ahead", "title": "t", "est_seconds": 1}),
			map[string]any{"op_id": fmt.Sprintf("c-%d", i), "by": "listener", "op": "clear", "base_version": 2*int64(i) + 1})
	}
	// A corrupt record below the snapshot must not matter: it is not read.
	putOp(t, rt, h, placeOp("zz-1", "phone", "end", storyEntry("zz", "phone")), 3, func(r *types.CommitmentRecord) { r.Directive.Body = "{not json" })
	out := sheetLogCapture(t)
	before := getSheet(t, h)
	h = restart(rt)
	_ = rt.sheetStore()
	after := getSheet(t, h)
	if jsonStr(before) != jsonStr(after) {
		t.Fatal("restart changed the sheet")
	}
	if strings.Contains(out.String(), "repaired") {
		t.Fatalf("%q", out.String())
	}
}

func TestSheetReplayFallsBackWhenTheNewestSnapshotDoesNotDecode(t *testing.T) {
	rt, h := sheetRT(t)
	for i := 0; i < sheetSnapshotEvery+3; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", map[string]any{
			"id": fmt.Sprintf("e%d", i), "kind": "page", "placed_by": "phone", "state": "ahead", "title": "t", "est_seconds": 1}),
			map[string]any{"op_id": fmt.Sprintf("c-%d", i), "by": "listener", "op": "clear", "base_version": 2*int64(i) + 1})
	}
	before := getSheet(t, h)
	ls := &liveSheet{ownerID: progOwner, sheetID: sheetOwnerID}
	ls.sheet = runsheet.RunSheet{Version: 9999}
	bad, _ := ls.snapshotRecord(rt.cfg.ComputerID, false)
	bad.RecordID = "autoradio-sheet-snap:bad"
	bad.Directive.Body = "[]"
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, bad); err != nil {
		t.Fatal(err)
	}
	h = restart(rt)
	if after := getSheet(t, h); jsonStr(before) != jsonStr(after) {
		t.Fatal("the sheet differs after falling back")
	}
}

type countingLedger struct {
	autoradioLedger
	mu      sync.Mutex
	lists   int
	opLists int
	fail    bool
}

func (c *countingLedger) ListCommitmentRecordsBySubjectRefOfComputer(ctx context.Context, o, comp, k, v string, limit int) ([]types.CommitmentRecord, int, int, error) {
	c.mu.Lock()
	c.lists++
	if k == "sheet_ops" {
		c.opLists++
	}
	fail := c.fail
	c.mu.Unlock()
	if fail {
		return nil, 0, 0, errors.New("disk read failed")
	}
	return c.autoradioLedger.ListCommitmentRecordsBySubjectRefOfComputer(ctx, o, comp, k, v, limit)
}

func TestSheetOnlyAStoreReadErrorIs503AndIsNotRescannedForAFewSeconds(t *testing.T) {
	rt, h := sheetRT(t)
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	rt.sheetStore().now = func() time.Time { return now }
	led := &countingLedger{autoradioLedger: rt.store, fail: true}
	h.autoradioLedgerOverride = led
	for i := 0; i < 5; i++ {
		if rec := sheetDo(h, "GET", "/api/autoradio/sheet", ""); rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%d", rec.Code)
		}
	}
	if led.lists != 1 {
		t.Fatalf("%d tape reads for 5 failing requests, want 1 (backoff)", led.lists)
	}
	now = now.Add(sheetReplayBackoff + time.Second)
	led.mu.Lock()
	led.fail = false
	led.mu.Unlock()
	if rec := sheetDo(h, "GET", "/api/autoradio/sheet", ""); rec.Code != http.StatusOK {
		t.Fatalf("after the backoff: %d", rec.Code)
	}
}

func TestSheetReplayPagesToTheSnapshotAndOtherComputersCannotShortenTheWindow(t *testing.T) {
	old := sheetReplayPage
	sheetReplayPage = 4
	t.Cleanup(func() { sheetReplayPage = old })
	rt, h := sheetRT(t)
	for i := 0; i < 30; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	// Rows of another computer, newer than ours, for the same owner and sheet.
	for i := 0; i < 20; i++ {
		other := &liveSheet{ownerID: progOwner, sheetID: sheetOwnerID}
		var o runsheet.Op
		_ = json.Unmarshal([]byte(jsonStr(placeOp(fmt.Sprintf("z-%d", i), "phone", "end", storyEntry(fmt.Sprintf("z%d", i), "phone")))), &o)
		rec, _ := other.opRecord(rt, progOwner, o, int64(100+i))
		rec.ComputerID = "other-computer"
		if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, "other-computer", rec); err != nil {
			t.Fatal(err)
		}
	}
	before := getSheet(t, h)
	h = restart(rt)
	after := getSheet(t, h)
	if before.Version != 30 || jsonStr(before) != jsonStr(after) {
		t.Fatalf("paged replay: v%d vs v%d", before.Version, after.Version)
	}
}

// R6
func TestSheetPlaceNowIsNotSubjectToTheAheadBound(t *testing.T) {
	_, h := sheetRT(t)
	var ops []map[string]any
	for i := 0; i < 64; i++ {
		ops = append(ops, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	mustOps(t, h, ops...)
	ops = ops[:0]
	for i := 64; i < 100; i++ {
		ops = append(ops, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	mustOps(t, h, ops...)
	full := mustOps(t, h, placeOp("p-x", "phone", "end", storyEntry("x", "phone")))
	if full.Results[0].Dropped != droppedAheadFull {
		t.Fatalf("101st: %+v", full.Results[0])
	}
	now := placeOp("p-now", "listener", "now", storyEntry("nowe", "listener"))
	a := mustOps(t, h, now)
	if !a.Results[0].Applied {
		t.Fatalf("place now at 100 ahead: %+v", a.Results[0])
	}
	if s := getSheet(t, h); s.OnAir != "nowe" || len(s.Entries) != 101 {
		t.Fatalf("on air %q entries %d", s.OnAir, len(s.Entries))
	}
}

// R7
func TestSheetSameBatchRepeatIsADuplicateAndACachedDropNeverReportsAnOldVersion(t *testing.T) {
	_, h := sheetRT(t)
	first := placeOp("p-1", "phone", "end", storyEntry("e1", "phone"))
	a := mustOps(t, h, first, first)
	if !a.Results[0].Applied || a.Results[1].Dropped != runsheet.DroppedDuplicate || a.Results[1].Version != 1 {
		t.Fatalf("%+v", a.Results)
	}
	drop := map[string]any{"op_id": "r-1", "by": "phone", "op": "remove", "id": "ghost", "base_version": 0}
	d := mustOps(t, h, drop)
	if d.Results[0].Dropped != runsheet.DroppedNotAhead || d.Results[0].Version != 1 {
		t.Fatalf("%+v", d.Results[0])
	}
	mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), placeOp("p-3", "phone", "end", storyEntry("e3", "phone")))
	d = mustOps(t, h, drop)
	if d.Results[0].Version != 3 {
		t.Fatalf("a cached drop reports version %d, the sheet is at 3", d.Results[0].Version)
	}
}

// R8
func TestSheetAnAnswerTheListenerAlreadyHeardLandsHeardAndNeverOnAir(t *testing.T) {
	_, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")),
		map[string]any{"op_id": "a-1", "by": "phone", "op": "aired", "id": "e1", "state": "on_air", "base_version": 0})
	ans := map[string]any{"id": "p-ans", "kind": "answer", "placed_by": "phone", "state": "heard", "title": "a", "text": "It is the harbour.", "est_seconds": 3}
	a := mustOps(t, h, map[string]any{"op_id": "p-2", "by": "phone", "op": "place", "at": "end", "entry": ans, "base_version": 0})
	if !a.Results[0].Applied {
		t.Fatalf("%+v", a.Results[0])
	}
	s := getSheet(t, h)
	if ids(s) != "p-ans:heard,e1:on_air" || s.OnAir != "e1" {
		t.Fatalf("sheet %s on air %q", ids(s), s.OnAir)
	}
	// Not for the desk or the listener, not any other kind in another state.
	bad := mustOps(t, h, map[string]any{"op_id": "p-3", "by": "listener", "op": "place", "at": "end", "entry": dictWith(ans, "id", "l-ans", "placed_by", "listener"), "base_version": 0})
	if bad.Results[0].Applied {
		t.Fatalf("a listener's heard answer applied")
	}
}

func dictWith(m map[string]any, kv ...string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

// R9
func TestSheetIdleSheetsLeaveMemoryAndReturnByReplay(t *testing.T) {
	rt, h := sheetRT(t)
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	rt.sheetStore().now = func() time.Time { return now }
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	st := rt.sheetStore()
	if ls, _ := st.get(context.Background(), h, progOwner, sheetOwnerID); ls == nil {
		t.Fatal("no sheet")
	}
	// A stream pins its sheet.
	stream := openStream(h, "")
	stream.waitFor(t, "event: sheet\n")
	now = now.Add(sheetIdle + 2*time.Minute)
	st.mu.Lock()
	st.sweepLocked(now)
	st.mu.Unlock()
	e := st.entries[progOwner+"\x00"+sheetOwnerID]
	e.mu.Lock()
	pinned := e.ls != nil
	e.mu.Unlock()
	if !pinned {
		t.Fatal("a sheet with a stream was evicted")
	}
	stream.close()
	now = now.Add(2 * time.Minute)
	st.mu.Lock()
	st.sweepLocked(now)
	st.mu.Unlock()
	e.mu.Lock()
	gone := e.ls == nil
	e.mu.Unlock()
	if !gone {
		t.Fatal("an idle sheet stayed in memory")
	}
	if s := getSheet(t, h); s.Version != 1 || ids(s) != "e1:ahead" {
		t.Fatalf("after eviction %s v%d", ids(s), s.Version)
	}
}

// F8, the hourly bound
func TestSheetHourlyBoundRefusesTheWholeBatch(t *testing.T) {
	_, h := sheetRT(t)
	now := time.Date(2026, 11, 16, 10, 20, 0, 0, time.UTC)
	h.rt.sheetStore().now = func() time.Time { return now }
	mkBatch := func(prefix string, n int) []map[string]any {
		var ops []map[string]any
		for i := 0; i < n; i++ {
			ops = append(ops, map[string]any{"op_id": fmt.Sprintf("%s-%d", prefix, i), "by": "phone", "op": "skip", "id": "none", "base_version": 0})
		}
		return ops
	}
	for i := 0; i < sheetOpsPerHour/sheetMaxOps; i++ {
		mustOps(t, h, mkBatch(fmt.Sprintf("a%d", i), sheetMaxOps)...)
	}
	mustOps(t, h, mkBatch("tail", sheetOpsPerHour%sheetMaxOps)...)
	rec, _ := postOps(t, h, mkBatch("over", 1)...)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over the bound: %d retry-after %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	// A replay of ops already answered is free.
	if rec, _ := postOps(t, h, mkBatch("a0", sheetMaxOps)...); rec.Code != 200 {
		t.Fatalf("replay over the bound: %d", rec.Code)
	}
	now = now.Add(time.Hour)
	if rec, _ := postOps(t, h, mkBatch("over", 1)...); rec.Code != 200 {
		t.Fatalf("next hour: %d", rec.Code)
	}
}

// ---- the stream ----

type sseBuf struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	hdr  http.Header
	code int
}

func (b *sseBuf) Header() http.Header {
	if b.hdr == nil {
		b.hdr = http.Header{}
	}
	return b.hdr
}
func (b *sseBuf) WriteHeader(c int) { b.mu.Lock(); b.code = c; b.mu.Unlock() }
func (b *sseBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *sseBuf) Flush()       {}
func (b *sseBuf) text() string { b.mu.Lock(); defer b.mu.Unlock(); return b.buf.String() }

type streamRun struct {
	buf    *sseBuf
	cancel context.CancelFunc
	done   chan struct{}
}

func openStream(h *APIHandler, query string) *streamRun {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/autoradio/sheet/stream"+query, nil).WithContext(ctx)
	req.Header.Set("X-Authenticated-User", progOwner)
	s := &streamRun{buf: &sseBuf{}, cancel: cancel, done: make(chan struct{})}
	go func() { defer close(s.done); h.HandleAutoradioSheetStream(s.buf, req) }()
	return s
}

func (s *streamRun) close() {
	s.cancel()
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		panic("the stream handler did not return after its request was cancelled")
	}
}

func (s *streamRun) waitFor(t *testing.T, substr string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if txt := s.buf.text(); strings.Contains(txt, substr) {
			return txt
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("stream never showed %q; got %q", substr, s.buf.text())
	return ""
}

func streamEvents(t *testing.T, text string) []sseEvent {
	t.Helper()
	var out []sseEvent
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	var cur sseEvent
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur = sseEvent{Name: strings.TrimPrefix(line, "event: ")}
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Data); err != nil {
				t.Fatalf("event data: %v", err)
			}
			out = append(out, cur)
		}
	}
	return out
}

// F4
func TestSheetStreamStartsWithTheCommentThenAFullSheetThenEchoesOps(t *testing.T) {
	_, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	st := openStream(h, "")
	defer st.close()
	txt := st.waitFor(t, "event: sheet\n")
	if !strings.HasPrefix(txt, ": autoradio\n\n") {
		t.Fatalf("the stream begins %q", txt[:min(len(txt), 40)])
	}
	if st.buf.hdr.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type %q", st.buf.hdr.Get("Content-Type"))
	}
	evs := streamEvents(t, txt)
	if len(evs) != 1 || evs[0].Name != "sheet" || evs[0].Data["version"].(float64) != 1 {
		t.Fatalf("events %+v", evs)
	}
	// The phone's own op is echoed with the server's versions.
	mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), placeOp("p-3", "phone", "end", storyEntry("e3", "phone")))
	txt = st.waitFor(t, "event: sheet_ops")
	evs = streamEvents(t, txt)
	last := evs[len(evs)-1]
	if last.Name != "sheet_ops" || last.Data["from_version"].(float64) != 1 || last.Data["to_version"].(float64) != 3 {
		t.Fatalf("sheet_ops %+v", last)
	}
	ops := last.Data["ops"].([]any)
	if len(ops) != 2 || ops[0].(map[string]any)["op_id"] != "p-2" || ops[1].(map[string]any)["op_id"] != "p-3" {
		t.Fatalf("ops %+v", ops)
	}
	// A dropped op changes nothing and so is not sent.
	mustOps(t, h, map[string]any{"op_id": "r-x", "by": "phone", "op": "remove", "id": "ghost", "base_version": 0})
	time.Sleep(30 * time.Millisecond)
	if n := strings.Count(st.buf.text(), "event: sheet_ops"); n != 1 {
		t.Fatalf("%d sheet_ops events after a dropped op, want 1", n)
	}
}

func TestSheetStreamAfterKeptSendsOpsAndAfterOlderOrAheadSendsTheSheet(t *testing.T) {
	_, h := sheetRT(t)
	for i := 0; i < 5; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	st := openStream(h, "?after=3")
	txt := st.waitFor(t, "event: sheet_ops")
	st.close()
	evs := streamEvents(t, txt)
	if len(evs) != 1 || evs[0].Data["from_version"].(float64) != 3 || evs[0].Data["to_version"].(float64) != 5 || len(evs[0].Data["ops"].([]any)) != 2 {
		t.Fatalf("after=3: %+v", evs)
	}
	st = openStream(h, "?after=5")
	st.waitFor(t, ": autoradio")
	time.Sleep(30 * time.Millisecond)
	if strings.Contains(st.buf.text(), "event:") {
		t.Fatalf("after=current sent %q", st.buf.text())
	}
	st.close()
	st = openStream(h, "?after=99")
	txt = st.waitFor(t, "event: sheet\n")
	st.close()
	if evs := streamEvents(t, txt); evs[0].Data["version"].(float64) != 5 {
		t.Fatalf("after ahead: %+v", evs)
	}
}

func TestSheetStreamAfterOlderThanKeptSendsTheFullSheet(t *testing.T) {
	rt, h := sheetRT(t)
	// More applied ops than the stream keeps: place and remove in turn.
	n := sheetKeepOps + 20
	for i := 0; i < n; i += 2 {
		mustOps(t, h,
			placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")),
			map[string]any{"op_id": fmt.Sprintf("r-%d", i), "by": "listener", "op": "remove", "id": fmt.Sprintf("e%d", i), "base_version": 0})
	}
	_ = rt
	st := openStream(h, "?after=1")
	txt := st.waitFor(t, "event: sheet\n")
	st.close()
	if evs := streamEvents(t, txt); evs[0].Name != "sheet" || int(evs[0].Data["version"].(float64)) != n {
		t.Fatalf("%+v", evs[0])
	}
	// After a restart the replay keeps the recent ops again.
	h = restart(rt)
	st = openStream(h, fmt.Sprintf("?after=%d", n-4))
	txt = st.waitFor(t, "event: sheet_ops")
	st.close()
	if evs := streamEvents(t, txt); evs[0].Name != "sheet_ops" || len(evs[0].Data["ops"].([]any)) != 4 {
		t.Fatalf("after a restart: %+v", evs[0])
	}
}

func TestSheetStreamPerOwnerBound(t *testing.T) {
	_, h := sheetRT(t)
	var runs []*streamRun
	for i := 0; i < sheetMaxStreams; i++ {
		s := openStream(h, "")
		s.waitFor(t, "event: sheet\n")
		runs = append(runs, s)
	}
	rec := sheetDo(h, "GET", "/api/autoradio/sheet/stream", "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("stream over the bound: %d", rec.Code)
	}
	runs[0].close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := openStream(h, "")
		time.Sleep(20 * time.Millisecond)
		if strings.Contains(s.buf.text(), "event: sheet") {
			runs = append(runs, s)
			break
		}
		s.close()
	}
	for _, r := range runs[1:] {
		r.close()
	}
}

// ---- custody and deletion ----

// F13
func TestTheReservedRunSheetKindIsGoneFromCodeAndFixtures(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Skip("not a git checkout (the build VM carries no .git); run where there is one")
	}
	out, err := exec.Command("git", "-C", root, "grep", "-l", "choir.autoradio"+"_run_sheet", "--", "internal", "cmd", "scripts", "nix", "frontend/src", "mobile").CombinedOutput()
	if err == nil {
		t.Fatalf("the reserved kind is still cited:\n%s", out)
	}
	if len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("git grep: %v\n%s", err, out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for d := dir; d != "/" && d != "."; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("no go.mod above " + dir)
	return ""
}

// The air reports settle the desk's forecasts, so the tape keeps them exactly
// as sent: by, state, ended (outcome, at, last_block) and the position.
func TestSheetAirReportsAreRecordedExactlyAsSent(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")), placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	reports := []map[string]any{
		{"op_id": "a-1", "by": "phone", "op": "aired", "id": "e1", "state": "on_air", "position_seconds": 0, "base_version": 2},
		{"op_id": "a-2", "by": "phone", "op": "aired", "id": "e1", "state": "skipped", "position_seconds": 0,
			"ended": map[string]any{"outcome": "failed", "at": "2026-11-16T12:00:00Z", "last_block": 0}, "base_version": 3},
		{"op_id": "a-3", "by": "phone", "op": "aired", "id": "e2", "state": "on_air", "position_seconds": 0, "base_version": 4},
		{"op_id": "a-4", "by": "phone", "op": "aired", "id": "e2", "state": "heard", "position_seconds": 12.5,
			"ended": map[string]any{"outcome": "left", "at": "2026-11-16T12:01:00Z", "last_block": 4}, "base_version": 5},
	}
	a := mustOps(t, h, reports...)
	for _, r := range a.Results {
		if !r.Applied {
			t.Fatalf("%+v", a.Results)
		}
	}
	s := getSheet(t, h)
	if len(s.Entries) != 2 || s.Entries[0].Ended == nil || s.Entries[0].Ended.Outcome != runsheet.OutcomeFailed || s.Entries[1].Ended.Outcome != runsheet.OutcomeLeft || s.Entries[1].Ended.LastBlock != 4 {
		t.Fatalf("sheet %s", jsonStr(s))
	}
	for _, id := range []string{"a-2", "a-4"} {
		rec, err := rt.store.GetCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, "autoradio-sheet-op:"+id)
		if err != nil || rec == nil {
			t.Fatal(err)
		}
		var got struct {
			Op map[string]any `json:"op"`
		}
		if err := json.Unmarshal([]byte(rec.Directive.Body), &got); err != nil {
			t.Fatal(err)
		}
		want := reports[1]
		if id == "a-4" {
			want = reports[3]
		}
		var sent map[string]any
		_ = json.Unmarshal([]byte(jsonStr(want)), &sent)
		for _, k := range []string{"by", "state", "ended", "position_seconds"} {
			if jsonStr(got.Op[k]) != jsonStr(sent[k]) {
				t.Fatalf("%s: %s recorded as %s, sent %s", id, k, jsonStr(got.Op[k]), jsonStr(sent[k]))
			}
		}
	}
}

// ---- review round 2 (2026-11-16): the failure list added before the code ----
//
//	S1  a replay-skipped record consumes its version; its op_id is answered
//	    replay_skipped (never applied), with no further eviction, one replay,
//	    the stream left open, and the next op takes the next version;
//	S2  a repair is announced: the sheet carries an epoch, the POST answer and
//	    GET carry it, a reader that does not hold it gets the full sheet even
//	    at the current version, a reader that holds it does not;
//	S3  an undecodable row envelope is counted, not a 503;
//	S4  past the replay cap the sheet is rebuilt from the newest records;
//	S5  the largest valid op record is under the stated bound;
//	S6  previous names the entries it trimmed on its record;
//	S7  a stepped-over version is never served as a contiguous sheet_ops.

func TestSheetASkippedLastRecordConsumesItsVersionAndItsOpIsDropped(t *testing.T) {
	for name, mutate := range map[string]func(*types.CommitmentRecord){
		"undecodable":      func(r *types.CommitmentRecord) { r.Directive.Body = "{not json" },
		"reducer drops it": nil,
	} {
		t.Run(name, func(t *testing.T) {
			rt, h := sheetRT(t)
			mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
			op := placeOp("p-2", "phone", "end", storyEntry("e2", "phone"))
			if mutate == nil {
				op = map[string]any{"op_id": "p-2", "by": "phone", "op": "remove", "id": "ghost", "base_version": 0}
			}
			var m []func(*types.CommitmentRecord)
			if mutate != nil {
				m = append(m, mutate)
			}
			putOp(t, rt, h, op, 2, m...)
			led := &countingLedger{autoradioLedger: rt.store}
			h = restart(rt)
			h.autoradioLedgerOverride = led
			s := getSheet(t, h)
			if s.Version != 2 || ids(s) != "e1:ahead" || s.SheetEpoch == 0 {
				t.Fatalf("sheet %s v%d epoch %d", ids(s), s.Version, s.SheetEpoch)
			}
			st := openStream(h, fmt.Sprintf("?after=2&epoch=%d", s.SheetEpoch))
			st.waitFor(t, ": autoradio")
			// The retry of the skipped op: answered dropped, with no new replay.
			a := mustOps(t, h, op)
			if a.Results[0].Applied || a.Results[0].Dropped != runsheet.DroppedReplaySkipped {
				t.Fatalf("retry: %+v", a.Results[0])
			}
			select {
			case <-st.done:
				t.Fatal("the retry closed the stream")
			case <-time.After(50 * time.Millisecond):
			}
			if led.opLists != 1 {
				t.Fatalf("%d replays, want 1", led.opLists)
			}
			// The version is not reused.
			if a := mustOps(t, h, placeOp("p-3", "phone", "end", storyEntry("e3", "phone"))); a.Results[0].Version != 3 {
				t.Fatalf("next op took version %d, want 3", a.Results[0].Version)
			}
			st.close()
			// And after another restart the same answers hold.
			h = restart(rt)
			if s2 := getSheet(t, h); s2.Version != 3 || ids(s2) != "e1:ahead,e3:ahead" {
				t.Fatalf("after restart %s v%d", ids(s2), s2.Version)
			}
			if a := mustOps(t, h, op); a.Results[0].Applied {
				t.Fatalf("after restart: %+v", a.Results[0])
			}
		})
	}
}

func TestSheetARepairIsAnnouncedToReadersThatDoNotHoldItsEpoch(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	putOp(t, rt, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), 2, func(r *types.CommitmentRecord) { r.Directive.Body = "{" })
	h = restart(rt)
	s := getSheet(t, h)
	if s.SheetEpoch == 0 || s.Version != 2 {
		t.Fatalf("%+v", s)
	}
	// A reader at the current version without the epoch gets the full sheet.
	st := openStream(h, "?after=2")
	txt := st.waitFor(t, "event: sheet\n")
	st.close()
	if evs := streamEvents(t, txt); evs[0].Name != "sheet" || int64(evs[0].Data["sheet_epoch"].(float64)) != s.SheetEpoch {
		t.Fatalf("%+v", evs)
	}
	// A reader that holds it gets nothing.
	st = openStream(h, fmt.Sprintf("?after=2&epoch=%d", s.SheetEpoch))
	st.waitFor(t, ": autoradio")
	time.Sleep(30 * time.Millisecond)
	if strings.Contains(st.buf.text(), "event:") {
		t.Fatalf("sent %q", st.buf.text())
	}
	st.close()
	// The POST answer carries the epoch.
	a := mustOps(t, h, placeOp("p-3", "phone", "end", storyEntry("e3", "phone")))
	rec, _ := postOps(t, h, placeOp("p-4", "phone", "end", storyEntry("e4", "phone")))
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"sheet_epoch":%d`, s.SheetEpoch)) {
		t.Fatalf("POST answer %s (first %+v)", rec.Body.String(), a)
	}
	// A sender that says which epoch it holds is not told again.
	held := postBody(t, h, fmt.Sprintf(`{"sheet_epoch":%d,"ops":[%s]}`, s.SheetEpoch, jsonStr(placeOp("p-5", "phone", "end", storyEntry("e5", "phone")))))
	if strings.Contains(held, "sheet_epoch") {
		t.Fatalf("a sender that holds the epoch was told it again: %s", held)
	}
	// A sheet that never needed repair has no epoch at all.
	_, h2 := sheetRT(t)
	mustOps(t, h2, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	if strings.Contains(sheetDo(h2, "GET", "/api/autoradio/sheet", "").Body.String(), "sheet_epoch") {
		t.Fatal("an unrepaired sheet carries an epoch")
	}
	// The repair names the first versions and ops.
	out := sheetLogCapture(t)
	_ = restart(rt)
	_ = getSheet(t, NewAPIHandler(rt))
	if strings.Contains(out.String(), "repaired") {
		t.Fatalf("a repaired sheet repaired again: %q", out.String())
	}
}

// envelopeLedger adds unreadable rows to every page of op records.
type envelopeLedger struct {
	autoradioLedger
	bad int
}

func (e *envelopeLedger) ListCommitmentRecordsBySubjectRefOfComputer(ctx context.Context, o, c, k, v string, limit int) ([]types.CommitmentRecord, int, int, error) {
	recs, und, rows, err := e.autoradioLedger.ListCommitmentRecordsBySubjectRefOfComputer(ctx, o, c, k, v, limit)
	if k == "sheet_ops" {
		und += e.bad
		rows += e.bad
	}
	return recs, und, rows, err
}

func TestSheetAnUndecodableEnvelopeIsCountedNotA503(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")), placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	out := sheetLogCapture(t)
	h = restart(rt)
	h.autoradioLedgerOverride = &envelopeLedger{autoradioLedger: rt.store, bad: 3}
	rec := sheetDo(h, "GET", "/api/autoradio/sheet", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(out.String(), "unreadable=3") {
		t.Fatalf("log %q", out.String())
	}
	var s runsheet.RunSheet
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Version != 2 || s.SheetEpoch != 0 {
		t.Fatalf("an unreadable row alone must not repair (it cannot be paid for once): %+v", s)
	}
}

func TestSheetPastTheReplayCapTheSheetIsRebuiltFromTheNewestRecords(t *testing.T) {
	oldPage, oldMax := sheetReplayPage, sheetReplayMax
	sheetReplayPage, sheetReplayMax = 4, 8
	t.Cleanup(func() { sheetReplayPage, sheetReplayMax = oldPage, oldMax })
	rt, h := sheetRT(t)
	for i := 0; i < 20; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", storyEntry(fmt.Sprintf("e%d", i), "phone")))
	}
	out := sheetLogCapture(t)
	h = restart(rt)
	rec := sheetDo(h, "GET", "/api/autoradio/sheet", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var s runsheet.RunSheet
	_ = json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Version != 20 || s.SheetEpoch == 0 || !strings.Contains(out.String(), "truncated=true") {
		t.Fatalf("v%d epoch %d log %q", s.Version, s.SheetEpoch, out.String())
	}
	if a := mustOps(t, h, placeOp("p-20", "phone", "end", storyEntry("e20", "phone"))); a.Results[0].Version != 21 {
		t.Fatalf("%+v", a.Results[0])
	}
}

// S5: the stated bound on one op record, with the largest valid ops.
// tapeObjectBytes is what the projection tape writes for a record: the
// objectgraph object (envelope, metadata and the record as a base64 body),
// marshalled as projectionTape.interceptOG does (internal/store/projection_tape.go).
func tapeObjectBytes(t *testing.T, ownerID, computerID, recordID string, record []byte) int {
	t.Helper()
	const kind = objectgraph.ObjectKind("choir.commitment_record")
	id, err := objectgraph.BuildCanonicalID(kind, ownerID, objectgraph.StableSuffixFromKey(computerID+"/"+recordID))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.NormalizeMetadata(map[string]any{
		"record_id": recordID, "agent_id": autoradioDesk + ":" + ownerID, "model_id": autoradioDesk,
		"discrepancy": "unresolved", "schema_id": types.CommitmentRecordSchemaV1,
		"created_at": "2026-11-16T12:00:00.123456789Z", "updated_at": "2026-11-16T12:00:00.123456789Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 12, 0, 0, 123456789, time.UTC)
	raw, err := json.Marshal(objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: ownerID, ComputerID: computerID,
		ContentHash: objectgraph.ContentHash(kind, record, meta), Body: record, Metadata: meta, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	return len(raw)
}

func TestSheetTheLargestValidOpRecordIsUnderTheStatedBound(t *testing.T) {
	rt, _ := sheetRT(t)
	ls := &liveSheet{ownerID: progOwner, sheetID: sheetOwnerID}
	maxRec, maxSnap, maxWire, maxTapeRec, maxTapeSnap := 0, 0, 0, 0, 0
	// The largest entry the reducer accepts, filled with each character whose
	// stored cost differs from its typed cost: a control character, < > &, a
	// quote, a backslash, a multi-byte character and U+2028.
	for _, fill := range []string{"\u0001", "<", ">", "&", `"`, `\`, "日", "\u2028", "a"} {
		fill := fill
		t.Run(fmt.Sprintf("fill %q", fill), func(t *testing.T) {
			// rp is n bytes of the fill (a multi-byte fill gets fewer characters).
			rp := func(n int) string { return strings.Repeat(fill, n/len(fill)) }
			var big runsheet.Entry
			for n := 21; n <= runsheet.MaxTextBytes; n += 21 {
				e := runsheet.Entry{ID: "big", Kind: runsheet.KindStory, PlacedBy: runsheet.ByPhone, State: runsheet.StateAhead,
					Title: rp(100), Text: rp(n), EstSeconds: 1,
					Source: &runsheet.Source{Ref: rp(700), Label: rp(170), URL: rp(700), ClipID: rp(170)}}
				e.Cites = append(e.Cites, runsheet.Cite{Ref: rp(700), Role: runsheet.RoleQuoted, Label: rp(170)})
				_, res, _ := runsheet.Apply(runsheet.RunSheet{}, runsheet.Op{OpID: "x", By: runsheet.ByPhone, Op: runsheet.OpPlace, At: runsheet.AtEnd, Entry: &e})
				if !res.Applied {
					break
				}
				big = e
			}
			if big.ID == "" {
				t.Fatal("no valid large entry")
			}
			ops := []runsheet.Op{
				{OpID: "big-place", By: runsheet.ByPhone, Op: runsheet.OpPlace, At: runsheet.AtEnd, Entry: &big, Reason: strings.Repeat("r", 160)},
				{OpID: "big-rewrite", By: runsheet.ByDesk, Op: runsheet.OpRewrite, ID: "x", Text: rp(runsheet.MaxTextBytes), Reason: strings.Repeat("r", 160)},
			}
			for _, op := range ops {
				rec, err := ls.opRecord(rt, progOwner, op, 1<<40)
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := json.Marshal(rec)
				if len(raw) >= sheetMaxRecordBytes {
					t.Fatalf("%s: record is %d bytes, the stated bound is %d", op.OpID, len(raw), sheetMaxRecordBytes)
				}
				if len(raw) > maxRec {
					maxRec = len(raw)
				}
				if n := tapeObjectBytes(t, progOwner, rt.cfg.ComputerID, rec.RecordID, raw); n > maxTapeRec {
					maxTapeRec = n
				}
			}
			// The largest snapshot: 151 maximal entries (100 ahead, one on air,
			// 50 in the tail; the size does not depend on the state) and a full
			// applied_ops, as stored and as sent.
			var full runsheet.RunSheet
			for i := 0; i < 151; i++ {
				e := big
				e.ID = fmt.Sprintf("big%03d", i)
				full.Entries = append(full.Entries, e)
			}
			for i := 0; i < runsheet.MaxAppliedOps; i++ {
				full.AppliedOps = append(full.AppliedOps, strings.Repeat("o", 64))
			}
			full.Version = 1 << 40
			ls.sheet = full
			snap, err := ls.snapshotRecord(rt.cfg.ComputerID, true)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(snap)
			if len(raw) >= sheetMaxSnapshotBytes {
				t.Fatalf("snapshot is %d bytes, the stated bound is %d", len(raw), sheetMaxSnapshotBytes)
			}
			if len(raw) > maxSnap {
				maxSnap = len(raw)
			}
			if n := tapeObjectBytes(t, progOwner, rt.cfg.ComputerID, snap.RecordID, raw); n > maxTapeSnap {
				maxTapeSnap = n
			}
			wire, _ := json.Marshal(wireSheet(full))
			if len(wire) >= sheetMaxSnapshotBytes {
				t.Fatalf("the sheet on the wire is %d bytes", len(wire))
			}
			if len(wire) > maxWire {
				maxWire = len(wire)
			}
		})
	}
	t.Logf("MAXIMA over every fill, commitment-record bytes: op record %d (bound %d), snapshot record %d (bound %d), sheet on the wire %d",
		maxRec, sheetMaxRecordBytes, maxSnap, sheetMaxSnapshotBytes, maxWire)
	// On the tape the record is the base64 body of an objectgraph object.
	atOp := tapeObjectBytes(t, progOwner, rt.cfg.ComputerID, "autoradio-sheet-op:x", make([]byte, sheetMaxRecordBytes))
	atSnap := tapeObjectBytes(t, progOwner, rt.cfg.ComputerID, "autoradio-sheet-snap:1", make([]byte, sheetMaxSnapshotBytes))
	t.Logf("TAPE bytes (objectgraph object, base64 body): maximal op record %d, maximal snapshot %d; at the guard bounds: op record %d, snapshot %d; worst clock hour at the bounds %d bytes",
		maxTapeRec, maxTapeSnap, atOp, atSnap, 1200*atOp+10*atSnap)
}

// The write-site guard: whatever the reducer's model of size misses, a record
// over its bound is not written. The reducer's bounds keep every valid op
// inside the bound (the test above), so the guard is shown with a lowered one.
func TestSheetARecordOverItsBoundIsNotWritten(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	old := sheetMaxRecordBytes
	sheetMaxRecordBytes = 400
	a := mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")))
	sheetMaxRecordBytes = old
	if a.Results[0].Applied || a.Results[0].Dropped != droppedRecordTooLarge || a.Version != 1 {
		t.Fatalf("%+v", a)
	}
	if s := getSheet(t, h); ids(s) != "e1:ahead" || len(opRecords(t, rt)) != 1 {
		t.Fatalf("a refused op reached the sheet or the tape: %s, %d records", ids(s), len(opRecords(t, rt)))
	}
	if again := mustOps(t, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone"))); again.Results[0].Dropped != droppedRecordTooLarge {
		t.Fatalf("the answer changed on replay: %+v", again.Results[0])
	}
}

func TestSheetASnapshotOverItsBoundIsNotWrittenAndTheSheetStaysCorrect(t *testing.T) {
	rt, h := sheetRT(t)
	old := sheetMaxSnapshotBytes
	sheetMaxSnapshotBytes = 200
	out := sheetLogCapture(t)
	for i := 0; i < sheetSnapshotEvery+2; i++ {
		mustOps(t, h, placeOp(fmt.Sprintf("p-%d", i), "phone", "end", map[string]any{
			"id": fmt.Sprintf("e%d", i), "kind": "page", "placed_by": "phone", "state": "ahead", "title": "t", "est_seconds": 1}),
			map[string]any{"op_id": fmt.Sprintf("c-%d", i), "by": "listener", "op": "clear", "base_version": 2*int64(i) + 1})
	}
	sheetMaxSnapshotBytes = old
	if len(snapshots(t, rt)) != 0 || !strings.Contains(out.String(), "not written") {
		t.Fatalf("snapshots %d, log %q", len(snapshots(t, rt)), out.String())
	}
	before := getSheet(t, h)
	h = restart(rt)
	if after := getSheet(t, h); jsonStr(before) != jsonStr(after) {
		t.Fatal("without a snapshot the replay differs")
	}
}

// S6
func TestSheetPreviousNamesTheEntriesItTrimmed(t *testing.T) {
	rt, h := sheetRT(t)
	var desk []runsheet.Op
	for i := 0; i < 100; i++ {
		var op runsheet.Op
		_ = json.Unmarshal([]byte(jsonStr(placeOp(fmt.Sprintf("d-%d", i), "desk", "end", storyEntry(fmt.Sprintf("d%d", i), "desk")))), &op)
		op.Reason = "r"
		desk = append(desk, op)
	}
	if _, err := h.applySheetOps(context.Background(), progOwner, sheetOwnerID, desk, true); err != nil {
		t.Fatal(err)
	}
	mustOps(t, h,
		map[string]any{"op_id": "a-1", "by": "phone", "op": "aired", "id": "d0", "state": "on_air", "base_version": 0},
		map[string]any{"op_id": "a-2", "by": "phone", "op": "aired", "id": "d0", "state": "heard", "base_version": 0},
		map[string]any{"op_id": "a-3", "by": "phone", "op": "aired", "id": "d1", "state": "on_air", "base_version": 0},
		placeOp("l-1", "listener", "end", storyEntry("l1", "listener")), placeOp("l-2", "listener", "end", storyEntry("l2", "listener")))
	cur := getSheet(t, h).Version
	a := mustOps(t, h, map[string]any{"op_id": "prev-1", "by": "listener", "op": "previous", "base_version": cur})
	if !a.Results[0].Applied {
		t.Fatalf("%+v", a.Results[0])
	}
	rec, _ := rt.store.GetCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, "autoradio-sheet-op:prev-1")
	if rec == nil || rec.SubjectRefs["trimmed"] != "d98,d99" {
		t.Fatalf("trimmed ref %v", rec)
	}
}

// S7
func TestSheetAHoleInTheKeptOpsIsServedAsTheFullSheet(t *testing.T) {
	rt, h := sheetRT(t)
	mustOps(t, h, placeOp("p-1", "phone", "end", storyEntry("e1", "phone")))
	putOp(t, rt, h, placeOp("p-2", "phone", "end", storyEntry("e2", "phone")), 2, func(r *types.CommitmentRecord) { r.Directive.Body = "{" })
	putOp(t, rt, h, placeOp("p-3", "phone", "end", storyEntry("e3", "phone")), 3)
	h = restart(rt)
	s := getSheet(t, h)
	st := openStream(h, fmt.Sprintf("?after=1&epoch=%d", s.SheetEpoch))
	txt := st.waitFor(t, "event: ")
	st.close()
	if evs := streamEvents(t, txt); evs[0].Name != "sheet" {
		t.Fatalf("a hole in versions was served as %s", evs[0].Name)
	}
}

func postBody(t *testing.T, h *APIHandler, body string) string {
	t.Helper()
	rec := sheetDo(h, http.MethodPost, "/api/autoradio/sheet/ops", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
