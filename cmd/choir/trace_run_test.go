package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// `choir trace run <run_id>` assembles one run from routes that already
// exist and prints its steps in order with the reasons the record holds.
// The failure list, written before the code (each is a test below):
//   - events of equal timestamp print out of their recorded order, or a
//     page served out of order is printed in that order;
//   - a model call's cell, or the cell's module calls, are attached to the
//     wrong model call or printed flat;
//   - a search with no recorded counts is shown with invented zeros, a
//     missing query is omitted instead of "query: not recorded", two
//     overlapping searches are crossed, or an unended search is paired
//     with the next one;
//   - a long source or output is printed whole by default, cut at a byte
//     inside a character, or clipped under --full;
//   - the reason of a call is invented, or the order turn commit, then the
//     model's words, then "not recorded" is not kept;
//   - the gaps section lists a gap the run's data does not show, or omits one it does;
//   - a restart and a resume are not shown as such;
//   - a run older than a kind's retention reads as a quiet run;
//   - events past the first page are lost, a failed second page is printed
//     as a whole run, or a cursor that does not advance loops;
//   - a 401 or 403 does not say which credential the route needs, or any
//     output carries the key;
//   - a run that is not there prints anything but "not found";
//   - --json does not round-trip or its field names drift;
//   - replayed events (same event_id) print twice;
//   - a live run is given an end;
//   - the command sends anything but GETs, or reads routes outside the list;
//   - `trace runs` sorts wrongly, ignores --desk or --limit, or offers a
//     column the list route cannot supply.

var traceT0 = time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)

func at(ms int64) time.Time { return traceT0.Add(time.Duration(ms) * time.Millisecond) }

func tsStr(ms int64) string { return at(ms).Format(time.RFC3339Nano) }

type fixEvent struct {
	seq     int64
	ms      int64
	kind    types.EventKind
	phase   string
	payload map[string]any
	id      string
}

func (e fixEvent) record(runID string) types.EventRecord {
	raw, _ := json.Marshal(e.payload)
	if e.payload == nil {
		raw = []byte(`{}`)
	}
	id := e.id
	if id == "" {
		id = "ev-" + strconv.FormatInt(e.seq, 10)
	}
	return types.EventRecord{EventID: id, Seq: e.seq, Timestamp: at(e.ms), RunID: runID, AgentID: "research:doc-1",
		Kind: e.kind, Phase: e.phase, Payload: raw}
}

type runFixture struct {
	run    types.RunRecord
	events []fixEvent
	// lifecycle events of the run's trajectory, served by reducer_seq.
	lifecycle []types.LifecycleEvent
	// inboxes by name.
	inboxes map[string][]types.CommitmentRecord
}

type fakeServer struct {
	mu        sync.Mutex
	runs      map[string]*runFixture
	listed    []map[string]any // GET /api/runs
	status    map[string]int   // path prefix -> forced status
	body      map[string]string
	failPage  int  // events page number (1-based) that answers 500; 0 none
	stuck     bool // events cursor does not advance
	reverse   bool // serve each events page in reverse order
	pageLimit int  // server page clamp for events
	requests  []string
	methods   []string
	auths     []string
}

func newFake(fixtures ...*runFixture) *fakeServer {
	f := &fakeServer{runs: map[string]*runFixture{}, status: map[string]int{}, body: map[string]string{}, pageLimit: 1000}
	for _, fx := range fixtures {
		f.runs[fx.run.RunID] = fx
	}
	return f
}

func writeFix(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.URL.RequestURI())
	f.methods = append(f.methods, r.Method)
	f.auths = append(f.auths, r.Header.Get("Authorization"))
	for prefix, st := range f.status {
		if strings.HasPrefix(r.URL.Path, prefix) {
			body := f.body[prefix]
			if body == "" {
				body = `{"error":"forced"}`
			}
			f.mu.Unlock()
			w.WriteHeader(st)
			_, _ = w.Write([]byte(body))
			return
		}
	}
	f.mu.Unlock()
	q := r.URL.Query()
	after, _ := strconv.ParseInt(q.Get("after"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	switch {
	case r.URL.Path == "/api/runs":
		writeFix(w, map[string]any{"runs": f.listed})
	case strings.HasPrefix(r.URL.Path, "/api/runs/") && strings.HasSuffix(r.URL.Path, "/events"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/events")
		fx := f.runs[id]
		if fx == nil {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"run not found"}`))
			return
		}
		if limit <= 0 {
			limit = 200
		}
		if limit > f.pageLimit {
			limit = f.pageLimit
		}
		f.mu.Lock()
		page := 0
		for _, rq := range f.requests {
			if strings.HasPrefix(rq, "/api/runs/"+id+"/events") {
				page++
			}
		}
		f.mu.Unlock()
		if f.failPage > 0 && page >= f.failPage {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"failed to list run events"}`))
			return
		}
		var evs []types.EventRecord
		for _, e := range fx.events {
			if e.seq > after {
				evs = append(evs, e.record(id))
			}
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].Seq < evs[j].Seq })
		resp := struct {
			Events    []types.EventRecord `json:"events"`
			NextAfter int64               `json:"next_after"`
		}{Events: []types.EventRecord{}}
		if len(evs) > limit {
			evs = evs[:limit]
			resp.NextAfter = evs[limit-1].Seq
			if f.stuck && after > 0 {
				resp.NextAfter = after
			}
		}
		if f.reverse {
			for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
				evs[i], evs[j] = evs[j], evs[i]
			}
		}
		resp.Events = append(resp.Events, evs...)
		writeFix(w, resp)
	case strings.HasPrefix(r.URL.Path, "/api/runs/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/runs/")
		fx := f.runs[id]
		if fx == nil {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"run not found"}`))
			return
		}
		writeFix(w, fx.run)
	case strings.HasPrefix(r.URL.Path, "/api/trajectories/") && strings.HasSuffix(r.URL.Path, "/events"):
		var all []types.LifecycleEvent
		for _, fx := range f.runs {
			all = append(all, fx.lifecycle...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].ReducerSeq < all[j].ReducerSeq })
		page := types.LifecycleEventPage{Schema: "choir.durable_work.v1", Events: []types.LifecycleEvent{}}
		if len(all) > 0 {
			page.Watermark = all[len(all)-1].ReducerSeq
		}
		if limit <= 0 {
			limit = 100
		}
		for _, e := range all {
			if e.ReducerSeq > after && len(page.Events) < limit {
				page.Events = append(page.Events, e)
				page.NextCursor = e.ReducerSeq
			}
		}
		if len(page.Events) == 0 {
			page.NextCursor = after
		}
		writeFix(w, page)
	case strings.HasPrefix(r.URL.Path, "/api/inboxes/"):
		inbox := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/inboxes/"), "/records")
		var recs []types.CommitmentRecord
		for _, fx := range f.runs {
			recs = append(recs, fx.inboxes[inbox]...)
		}
		sort.Slice(recs, func(i, j int) bool { return recs[i].InboxSeq < recs[j].InboxSeq })
		resp := map[string]any{"inbox": inbox, "head": int64(len(recs)), "cursor": 0, "open_rounds": []any{}, "next_after": 0}
		if limit <= 0 {
			limit = 200
		}
		out := []types.CommitmentRecord{}
		for _, rec := range recs {
			if rec.InboxSeq > after && len(out) < limit {
				out = append(out, rec)
			}
		}
		resp["records"] = out
		writeFix(w, resp)
	default:
		http.NotFound(w, r)
	}
}

func baseRun(id string) types.RunRecord {
	fin := at(6000)
	return types.RunRecord{RunID: id, AgentID: "research:doc-1", ChannelID: "doc-1", TrajectoryID: "traj-1",
		AgentProfile: "research", AgentRole: "research", OwnerID: "owner-1", ComputerID: "comp-1",
		State: types.RunCompleted, Prompt: "Find the filing for the story.", Result: "Found three filings.",
		CreatedAt: at(0), UpdatedAt: fin, FinishedAt: &fin, Metadata: map[string]any{}}
}

func toolLoop(seq, ms int64, words string, extra map[string]any) fixEvent {
	p := map[string]any{"desk": "research", "iteration": 1, "stop_reason": "tool_use", "tool_calls": 1,
		"tool_call_names": []string{"desk_go_eval"}, "response_text": words, "response_text_chars": len(words),
		"model": "swe-2-flash", "llm_provider": "devin", "llm_model": "swe-2-flash", "input_tokens": 9000,
		"output_tokens": 300, "cached_input_tokens": 8000, "provider_call_ms": 1000, "messages": 3, "tools": 4,
		// The runtime stores the phase inside the payload (payloadWithPhase);
		// the record's own phase field stays empty for run events.
		"phase": "tool_loop"}
	for k, v := range extra {
		p[k] = v
	}
	return fixEvent{seq: seq, ms: ms, kind: types.EventRunProgress, payload: p}
}

func cellEvent(seq, startMs, endMs int64, egress []map[string]any) fixEvent {
	p := map[string]any{"run_id": "run-1", "started_at": at(startMs), "finished_at": at(endMs), "duration_ms": endMs - startMs,
		"intents": 0, "error": ""}
	if egress != nil {
		p["egress"] = egress
		p["egress_calls"] = len(egress)
	}
	return fixEvent{seq: seq, ms: endMs, kind: types.EventDeskCell, payload: p}
}

func toolResult(seq, startMs, endMs int64, source, output string) fixEvent {
	return fixEvent{seq: seq, ms: endMs + 50, kind: types.EventToolResult, payload: map[string]any{
		"tool": "desk_go_eval", "call_id": "call-" + strconv.FormatInt(seq, 10), "is_error": false,
		"output": output, "output_len": len(output), "started_at": at(startMs), "duration_ms": endMs - startMs + 50,
		"arguments": map[string]any{"source": source}}}
}

func egressRow(verb string, startMs, endMs int64, extra map[string]any) map[string]any {
	row := map[string]any{"request_id": "r", "verb": verb, "run_id": "run-1", "started_at": at(startMs), "finished_at": at(endMs), "is_error": false}
	for k, v := range extra {
		row[k] = v
	}
	return row
}

func searchDone(seq, startMs, endMs int64, pages int, id string) fixEvent {
	return fixEvent{seq: seq, ms: endMs, kind: types.EventDeskEgress, payload: map[string]any{
		"verb": "search_done", "search_id": id, "run_id": "run-1", "started_at": at(startMs), "finished_at": at(endMs),
		"answered": 3, "arrived": 3, "total": 4, "pages_returned": pages}}
}

var decisionRow = map[string]any{"search_verb": "search", "earlier_searches": 2, "earlier_pages_new": 7, "pages_tried": 4, "pages_read": 3, "outlets_read": 2}

// mainFixture: two model calls; the first issues a cell that starts a
// search (with counts), reads one page (an error); the second issues a cell
// that starts a search with no counts. Its model text is empty.
func mainFixture() *runFixture {
	return &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted, payload: map[string]any{"prompt_length": 30}},
		{seq: 2, ms: 10, kind: types.EventRunStarted, payload: map[string]any{}},
		toolLoop(3, 1200, "I will search for the filing first.", nil),
		cellEvent(4, 1310, 1800, []map[string]any{
			egressRow("search_start", 1400, 1450, map[string]any{"decision": decisionRow}),
			egressRow("read", 1500, 1600, map[string]any{"is_error": true, "error": "timeout"}),
		}),
		toolResult(5, 1300, 1800, `choir.Search("the filing")`, `{"stdout":"ok"}`),
		searchDone(6, 1401, 4000, 12, "s1"),
		toolLoop(7, 5000, "", nil),
		cellEvent(8, 5100, 5600, []map[string]any{egressRow("search_start", 5200, 5250, nil)}),
		toolResult(9, 5090, 5600, `choir.Search("second query")`, `{"stdout":"done"}`),
		{seq: 10, ms: 6000, kind: types.EventRunCompleted, payload: map[string]any{}},
	}}
}

func runTraceCmd(t *testing.T, f *fakeServer, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	old, oldSleep := traceRunNow, traceRunSleep
	traceRunNow = func() time.Time { return at(3600 * 1000) }
	traceRunSleep = func(time.Duration) {}
	t.Cleanup(func() { traceRunNow, traceRunSleep = old, oldSleep })
	var out, errb bytes.Buffer
	full := append([]string{"trace", "run"}, args...)
	full = append(full, "--host", srv.URL)
	code = run(full, &out, &errb)
	return out.String(), errb.String(), code
}

func traceDoc(t *testing.T, f *fakeServer, id string, extra ...string) traceRunDoc {
	t.Helper()
	args := append([]string{id, "--json"}, extra...)
	out, errs, code := runTraceCmd(t, f, args...)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errs)
	}
	var doc traceRunDoc
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decode --json: %v\n%s", err, out)
	}
	return doc
}

func flatten(steps []traceRunStep) []traceRunStep {
	var out []traceRunStep
	for _, s := range steps {
		out = append(out, s)
		out = append(out, flatten(s.Steps)...)
	}
	return out
}

func ofKind(steps []traceRunStep, kind string) []traceRunStep {
	var out []traceRunStep
	for _, s := range flatten(steps) {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

func gapIDs(d traceRunDoc) map[string]bool {
	m := map[string]bool{}
	for _, g := range d.Gaps {
		m[g.ID] = true
	}
	return m
}

func TestTraceRunGroupsCellAndCallsUnderTheirModelCall(t *testing.T) {
	doc := traceDoc(t, newFake(mainFixture()), "run-1")
	if doc.Schema != traceRunSchema || doc.Run.RunID != "run-1" {
		t.Fatalf("header = %+v", doc.Run)
	}
	var models []traceRunStep
	for _, s := range doc.Steps {
		if s.Kind == "model_call" {
			models = append(models, s)
		}
	}
	if len(models) != 2 {
		t.Fatalf("model calls = %d, want 2: %+v", len(models), doc.Steps)
	}
	c1 := ofKind(models[0].Steps, "cell")
	c2 := ofKind(models[1].Steps, "cell")
	if len(c1) != 1 || len(c2) != 1 {
		t.Fatalf("cells under calls = %d and %d, want 1 and 1", len(c1), len(c2))
	}
	if len(c1[0].Steps) != 2 || c1[0].Steps[0].Call.Verb != "search_start" || c1[0].Steps[1].Call.Verb != "read" {
		t.Fatalf("cell 1 calls = %+v", c1[0].Steps)
	}
	if len(c2[0].Steps) != 1 {
		t.Fatalf("cell 2 calls = %+v", c2[0].Steps)
	}
	if !strings.Contains(c1[0].Cell.Source.Text, "the filing") || !strings.Contains(c2[0].Cell.Source.Text, "second query") {
		t.Fatalf("sources paired wrongly: %q / %q", c1[0].Cell.Source.Text, c2[0].Cell.Source.Text)
	}
	if doc.Totals.ModelCalls != 2 || doc.Totals.Cells != 2 || doc.Totals.InputTokens != 18000 || doc.Totals.CachedTokens != 16000 {
		t.Fatalf("totals = %+v", doc.Totals)
	}
	if doc.Totals.ModuleCalls["search_start"] != 2 || doc.Totals.ModuleCalls["read"] != 1 || doc.Totals.Searches != 2 || doc.Totals.Errors < 1 {
		t.Fatalf("module totals = %+v", doc.Totals)
	}
	if doc.Totals.PagesReturned != 12 || doc.Totals.PagesRead != 3 {
		t.Fatalf("pages = %d returned %d read", doc.Totals.PagesReturned, doc.Totals.PagesRead)
	}
}

func TestTraceRunOrdersEqualTimestampsByRecordedSeq(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 4, ms: 1000, kind: types.EventRunCompactionCompleted, payload: map[string]any{"tokens_before": 9}},
		{seq: 2, ms: 1000, kind: types.EventRunCompactionStarted},
		{seq: 3, ms: 1000, kind: types.EventRunRetry, payload: map[string]any{"reason": "overflow"}},
		{seq: 5, ms: 2000, kind: types.EventRunCompleted},
	}}
	f := newFake(fx)
	f.reverse = true // the page is served newest first; the view must not trust that
	doc := traceDoc(t, f, "run-1")
	var got []string
	for _, s := range doc.Steps {
		got = append(got, s.EventID)
	}
	want := []string{"ev-1", "ev-2", "ev-3", "ev-4", "ev-5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestTraceRunDedupesReplayedEvents(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 2, ms: 500, kind: types.EventRunRetry, id: "same", payload: map[string]any{"reason": "overflow"}},
		{seq: 3, ms: 500, kind: types.EventRunRetry, id: "same", payload: map[string]any{"reason": "overflow"}},
		{seq: 4, ms: 900, kind: types.EventRunCompleted},
	}}
	doc := traceDoc(t, newFake(fx), "run-1")
	if n := len(ofKind(doc.Steps, "retry")); n != 1 {
		t.Fatalf("retry steps = %d, want 1 (replayed event_id)", n)
	}
}

func TestTraceRunSearchWithAndWithoutRecordedCounts(t *testing.T) {
	doc := traceDoc(t, newFake(mainFixture()), "run-1")
	calls := ofKind(doc.Steps, "module_call")
	var searches []traceRunStep
	for _, c := range calls {
		if c.Call.Verb == "search_start" {
			searches = append(searches, c)
		}
	}
	if len(searches) != 2 {
		t.Fatalf("searches = %d", len(searches))
	}
	first, second := searches[0].Call, searches[1].Call
	if first.Decision == nil || first.Decision.EarlierSearches != 2 || first.Decision.PagesRead != 3 || first.Decision.OutletsRead != 2 {
		t.Fatalf("decision = %+v", first.Decision)
	}
	if first.End == nil || first.End.PagesReturned != 12 || first.End.PairedBy == "" {
		t.Fatalf("first end = %+v", first.End)
	}
	if second.Decision != nil {
		t.Fatalf("second search has invented counts: %+v", second.Decision)
	}
	if second.End != nil || !strings.Contains(second.EndNote, "no end recorded") {
		t.Fatalf("second end = %+v note %q", second.End, second.EndNote)
	}
	if first.Query != "not recorded" || second.Query != "not recorded" {
		t.Fatalf("queries = %q %q", first.Query, second.Query)
	}
	text, _, _ := runTraceCmd(t, newFake(mainFixture()), "run-1")
	for _, want := range []string{"no end recorded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text view lacks %q:\n%s", want, text)
		}
	}
}

func TestTraceRunOverlappingSearchesAreNotCrossed(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 500, "two at once", nil),
		cellEvent(3, 600, 900, []map[string]any{
			egressRow("search_start", 700, 710, map[string]any{"decision": decisionRow}),
			egressRow("search_start", 720, 730, map[string]any{"decision": decisionRow}),
		}),
		toolResult(4, 590, 900, "two searches", "{}"),
		searchDone(5, 701, 2000, 5, "sA"),
		searchDone(6, 721, 3000, 9, "sB"),
		{seq: 7, ms: 4000, kind: types.EventRunCompleted},
	}}
	doc := traceDoc(t, newFake(fx), "run-1")
	var searches []traceRunStep
	for _, c := range ofKind(doc.Steps, "module_call") {
		if c.Call.Verb == "search_start" {
			searches = append(searches, c)
		}
	}
	if len(searches) != 2 || searches[0].Call.End == nil || searches[1].Call.End == nil {
		t.Fatalf("searches = %+v", searches)
	}
	if searches[0].Call.End.PagesReturned != 5 || searches[1].Call.End.PagesReturned != 9 {
		t.Fatalf("crossed: %d and %d pages", searches[0].Call.End.PagesReturned, searches[1].Call.End.PagesReturned)
	}
}

func TestTraceRunClipsByDefaultAndFullDoesNot(t *testing.T) {
	source := strings.Repeat("é", 60000) // 120000 bytes
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 500, "go", nil),
		cellEvent(3, 600, 900, nil),
		toolResult(4, 590, 900, source, strings.Repeat("o", 5000)),
		{seq: 5, ms: 2000, kind: types.EventRunCompleted},
	}}
	out, _, code := runTraceCmd(t, newFake(fx), "run-1")
	if code != 0 || !utf8.ValidString(out) {
		t.Fatalf("exit=%d valid utf8=%v", code, utf8.ValidString(out))
	}
	if len(out) > 20000 || !strings.Contains(out, "clipped") || !strings.Contains(out, "60000") {
		t.Fatalf("default output not clipped with its size (len %d)", len(out))
	}
	fullOut, _, code := runTraceCmd(t, newFake(fx), "run-1", "--full")
	if code != 0 || len(fullOut) < 120000 || strings.Contains(fullOut, "[clipped") {
		t.Fatalf("--full clipped (len %d)", len(fullOut))
	}
	doc := traceDoc(t, newFake(fx), "run-1")
	cell := ofKind(doc.Steps, "cell")[0].Cell
	if !cell.Source.Cut || cell.Source.Chars != 60000 || !utf8.ValidString(cell.Source.Text) {
		t.Fatalf("json source = cut %v chars %d", cell.Source.Cut, cell.Source.Chars)
	}
}

func lifecycleCommit(seq int64, ms int64, reason string) types.LifecycleEvent {
	return types.LifecycleEvent{EventID: "le-" + strconv.FormatInt(seq, 10), TrajectoryID: "traj-1", RunID: "run-1",
		Kind: types.LifecycleTextureTurnCommitted, ReducerSeq: seq, Reason: reason, CreatedAt: at(ms)}
}

func TestTraceRunReasonOrderTurnCommitThenWordsThenNotRecorded(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 1000, "Draft v2 now.", nil),
		cellEvent(3, 1100, 1500, nil), toolResult(4, 1090, 1500, "A()", "{}"),
		toolLoop(5, 3000, "Let me read once more.", nil),
		cellEvent(6, 3100, 3500, nil), toolResult(7, 3090, 3500, "B()", "{}"),
		toolLoop(8, 5000, "", nil),
		cellEvent(9, 5100, 5500, nil), toolResult(10, 5090, 5500, "C()", "{}"),
		{seq: 11, ms: 6000, kind: types.EventRunCompleted},
	}, lifecycle: []types.LifecycleEvent{lifecycleCommit(1, 1700, "revised after the research report")}}
	doc := traceDoc(t, newFake(fx), "run-1")
	cells := ofKind(doc.Steps, "cell")
	if len(cells) != 3 {
		t.Fatalf("cells = %d", len(cells))
	}
	if r := cells[0].Reason; r == nil || r.Source != "turn_commit" || !strings.Contains(r.Text, "revised after the research report") {
		t.Fatalf("cell A reason = %+v", r)
	}
	if r := cells[1].Reason; r == nil || r.Source != "model_words" || r.Text != "Let me read once more." {
		t.Fatalf("cell B reason = %+v", r)
	}
	if r := cells[2].Reason; r == nil || r.Source != "not_recorded" || r.Text != "" {
		t.Fatalf("cell C reason = %+v", r)
	}
	if n := len(ofKind(doc.Steps, "turn_commit")); n != 1 {
		t.Fatalf("turn_commit steps = %d", n)
	}
	text, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	for _, want := range []string{"why: turn commit:", "the model said before this cell:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text lacks %q:\n%s", want, text)
		}
	}
	if !gapIDs(doc)["no_stated_reason"] {
		t.Fatalf("gaps = %+v", doc.Gaps)
	}
}

func TestTraceRunMarksTheModelTextTheWriterCut(t *testing.T) {
	words := strings.Repeat("w", 2003)
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 1000, words, map[string]any{"response_text_chars": 5200}),
		{seq: 3, ms: 2000, kind: types.EventRunCompleted},
	}}
	doc := traceDoc(t, newFake(fx), "run-1")
	m := ofKind(doc.Steps, "model_call")[0].Model
	if !m.Words.StoredCut || m.Words.Chars != 5200 {
		t.Fatalf("words = %+v", m.Words)
	}
	if !gapIDs(doc)["model_text_cut"] {
		t.Fatalf("gaps = %+v", doc.Gaps)
	}
}

func TestTraceRunGapsAreBuiltOnlyFromWhatIsAbsent(t *testing.T) {
	bare := traceDoc(t, newFake(mainFixture()), "run-1")
	for _, id := range []string{"no_stated_reason", "prompt_not_stored", "query_not_recorded", "wake_record_not_recorded",
		"digest_ids_absent", "tool_loop_no_call_ids", "search_start_no_search_id", "reasoning_text_absent"} {
		if !gapIDs(bare)[id] {
			t.Errorf("bare run lacks gap %q: %+v", id, bare.Gaps)
		}
	}
	// A run whose record holds a waking record, digest ids, call ids and a
	// reason on every call has none of those gaps.
	run := baseRun("run-1")
	run.Metadata = map[string]any{"request_source": "inbox_record", "woken_by_record": "rec-9"}
	fx := &runFixture{run: run, events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 2, ms: 10, kind: types.EventRunStarted, payload: map[string]any{"digest_captures": []string{"cap-1"}}},
		toolLoop(3, 1000, "plan", map[string]any{"tool_call_ids": []string{"call-4"}}),
		cellEvent(4, 1100, 1500, nil), toolResult(5, 1090, 1500, "A()", "{}"),
		{seq: 6, ms: 2000, kind: types.EventRunCompleted},
	}}
	full := traceDoc(t, newFake(fx), "run-1")
	for _, id := range []string{"no_stated_reason", "wake_record_not_recorded", "digest_ids_absent", "tool_loop_no_call_ids", "query_not_recorded", "search_start_no_search_id", "events_past_retention"} {
		if gapIDs(full)[id] {
			t.Errorf("complete run still lists gap %q", id)
		}
	}
	if !gapIDs(full)["prompt_not_stored"] {
		t.Errorf("prompt gap must stay: no provider_call event is stored")
	}
	if !full.Run.WokenBy.Recorded || full.Run.WokenBy.Fields["woken_by_record"] != "rec-9" {
		t.Fatalf("woken by = %+v", full.Run.WokenBy)
	}
}

func TestTraceRunWokenByIsRecordedOrNotRecorded(t *testing.T) {
	label := mainFixture()
	label.run.Metadata = map[string]any{"request_source": "inbox_record"}
	out, _, _ := runTraceCmd(t, newFake(label), "run-1")
	if !strings.Contains(out, "request_source=inbox_record") || !strings.Contains(out, "the waking record is not recorded") {
		t.Fatalf("label-only wake:\n%s", out)
	}
	none, _, _ := runTraceCmd(t, newFake(mainFixture()), "run-1")
	if !strings.Contains(none, "woken by: not recorded") {
		t.Fatalf("no wake info:\n%s", none)
	}
}

func TestTraceRunShowsARestartAndAResume(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 2, ms: 10, kind: types.EventRunStarted},
		toolLoop(3, 1000, "working", nil),
		{seq: 4, ms: 3000, kind: types.EventRunPassivated, payload: map[string]any{"recovery": "passivated_on_restart"}},
		{seq: 5, ms: 9000, kind: types.EventRunStarted},
		toolLoop(6, 10000, "continuing", nil),
		{seq: 7, ms: 11000, kind: types.EventRunCompleted},
	}}
	fx.run.FinishedAt = func() *time.Time { x := at(11000); return &x }()
	doc := traceDoc(t, newFake(fx), "run-1")
	var notes []string
	for _, s := range ofKind(doc.Steps, "lifecycle") {
		notes = append(notes, s.Title)
	}
	joined := strings.Join(notes, "|")
	if !strings.Contains(joined, "interrupted by a restart") || !strings.Contains(joined, "resumed") {
		t.Fatalf("lifecycle steps = %v", notes)
	}
	// A crash closure: the run ended cancelled with the restart error.
	crash := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 2, ms: 10, kind: types.EventRunStarted},
		{seq: 3, ms: 5000, kind: types.EventRunCancelled, payload: map[string]any{"error": "interrupted_by_restart"}},
	}}
	crash.run.State = types.RunCancelled
	crash.run.Error = "interrupted_by_restart"
	cd := traceDoc(t, newFake(crash), "run-1")
	var found bool
	for _, s := range ofKind(cd.Steps, "lifecycle") {
		found = found || strings.Contains(s.Title, "interrupted by a restart")
	}
	if !found || cd.Run.Error != "interrupted_by_restart" {
		t.Fatalf("crash closure not shown: %+v", cd.Steps)
	}
}

func TestTraceRunSaysWhichKindsRetirementHasTaken(t *testing.T) {
	// 10 days old: tool.result (7 d) is gone, desk.cell and tool_loop (14 d) are not.
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 1000, "words", nil),
		cellEvent(3, 1100, 1500, nil),
		{seq: 4, ms: 6000, kind: types.EventRunCompleted},
	}}
	f := newFake(fx)
	srv := httptest.NewServer(f)
	defer srv.Close()
	old := traceRunNow
	defer func() { traceRunNow = old }()
	traceRunNow = func() time.Time { return at(10 * 24 * 3600 * 1000) }
	var out, errb bytes.Buffer
	if code := run([]string{"trace", "run", "run-1", "--json", "--host", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	var doc traceRunDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Retention) != 1 || doc.Retention[0].Kind != "tool.result" || doc.Retention[0].Status != "gone" || doc.Retention[0].Days != 7 {
		t.Fatalf("retention = %+v", doc.Retention)
	}
	if !gapIDs(doc)["events_past_retention"] {
		t.Fatalf("gaps = %+v", doc.Gaps)
	}
	// 20 days old: every kind is gone.
	traceRunNow = func() time.Time { return at(20 * 24 * 3600 * 1000) }
	out.Reset()
	if code := run([]string{"trace", "run", "run-1", "--host", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"tool.result", "desk.cell", "desk.egress", "loop.progress"} {
		if !strings.Contains(out.String(), want+" ") {
			t.Fatalf("20-day-old run does not say %s is gone:\n%s", want, out.String())
		}
	}
	if !strings.Contains(out.String(), "gone") {
		t.Fatalf("no 'gone' in output")
	}
}

func TestTraceRunPagesThroughEveryEvent(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1")}
	fx.events = append(fx.events, fixEvent{seq: 1, ms: 0, kind: types.EventRunSubmitted})
	for i := int64(2); i <= 2499; i++ {
		fx.events = append(fx.events, fixEvent{seq: i, ms: i, kind: types.EventRunRetry, payload: map[string]any{"reason": "r"}})
	}
	fx.events = append(fx.events, fixEvent{seq: 2500, ms: 5000, kind: types.EventRunCompleted})
	f := newFake(fx)
	out, errs, code := runTraceCmd(t, f, "run-1", "--json")
	if code != 0 {
		t.Fatalf("exit %d %s", code, errs)
	}
	var doc traceRunDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Steps) != 2500 || doc.Partial {
		t.Fatalf("steps = %d partial = %v", len(doc.Steps), doc.Partial)
	}
	pages := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, "/api/runs/run-1/events") {
			pages++
		}
	}
	if pages != 3 {
		t.Fatalf("event pages requested = %d, want 3", pages)
	}
	if !strings.Contains(errs, "2000") {
		t.Fatalf("no progress on stderr: %q", errs)
	}
}

func TestTraceRunPrintsAPartialRunAsPartial(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1")}
	fx.events = append(fx.events, fixEvent{seq: 1, ms: 0, kind: types.EventRunSubmitted})
	for i := int64(2); i <= 1500; i++ {
		fx.events = append(fx.events, fixEvent{seq: i, ms: i, kind: types.EventRunRetry, payload: map[string]any{"reason": "r"}})
	}
	f := newFake(fx)
	f.failPage = 2
	out, _, code := runTraceCmd(t, f, "run-1")
	if code != 0 {
		t.Fatalf("a partial run still prints; exit %d", code)
	}
	for _, want := range []string{"PARTIAL", "seq 1000", "HTTP 500"} {
		if !strings.Contains(out, want) {
			t.Fatalf("partial run lacks %q:\n%s", want, out[:min(len(out), 1500)])
		}
	}
	doc := traceDoc(t, f, "run-1")
	if !doc.Partial || len(doc.PartialWhy) == 0 {
		t.Fatalf("json partial = %v %v", doc.Partial, doc.PartialWhy)
	}
	// A cursor that does not advance stops, and says so.
	stuck := newFake(fx)
	stuck.stuck = true
	stuck.pageLimit = 500
	sd := traceDoc(t, stuck, "run-1")
	if !sd.Partial || !strings.Contains(strings.Join(sd.PartialWhy, " "), "did not advance") {
		t.Fatalf("stuck cursor: %v", sd.PartialWhy)
	}
}

func TestTraceRunAuthRefusalsNameTheCredentialAndNeverPrintTheKey(t *testing.T) {
	const key = "choir_sk_TOPSECRET1234567890"
	t.Setenv(apiKeyEnvVar, key)
	cases := []struct {
		status int
		body   string
		want   []string
	}{
		{401, `{"error":"authentication required"}`, []string{"401", "owner API key", "read:runtime"}},
		{403, `{"error":"missing required scope: read:runtime"}`, []string{"403", "read:runtime", "scope"}},
		{403, `{"error":"echo ` + key + `"}`, []string{"403", "read:runtime"}},
	}
	for _, tc := range cases {
		f := newFake(mainFixture())
		f.status["/api/runs/"] = tc.status
		f.body["/api/runs/"] = tc.body
		out, errs, code := runTraceCmd(t, f, "run-1")
		if code != 1 || out != "" {
			t.Fatalf("status %d: exit %d stdout %q", tc.status, code, out)
		}
		for _, w := range tc.want {
			if !strings.Contains(errs, w) {
				t.Fatalf("status %d stderr lacks %q: %s", tc.status, w, errs)
			}
		}
		if strings.Contains(errs, "TOPSECRET") || strings.Contains(out, "TOPSECRET") {
			t.Fatalf("the key is printed: %s", errs)
		}
	}
	// A successful read never prints it either, in any format or on stderr.
	for _, args := range [][]string{{"run-1"}, {"run-1", "--json"}, {"run-1", "--full"}} {
		out, errs, _ := runTraceCmd(t, newFake(mainFixture()), args...)
		if strings.Contains(out+errs, "TOPSECRET") || strings.Contains(out+errs, "choir_sk_") {
			t.Fatalf("key in output of %v", args)
		}
	}
}

func TestTraceRunNotFoundPrintsOnlyThat(t *testing.T) {
	out, errs, code := runTraceCmd(t, newFake(), "run-gone")
	if code != 1 || out != "" || !strings.Contains(errs, "not found") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errs)
	}
}

func TestTraceRunJSONRoundTripsWithStableNames(t *testing.T) {
	out, _, code := runTraceCmd(t, newFake(mainFixture()), "run-1", "--json")
	if code != 0 {
		t.Fatal(code)
	}
	var doc traceRunDoc
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(doc)
	var a, b bytes.Buffer
	_ = json.Compact(&a, []byte(out))
	_ = json.Compact(&b, again)
	if a.String() != b.String() {
		t.Fatalf("round trip differs:\n%s\n%s", a.String(), b.String())
	}
	var top map[string]json.RawMessage
	_ = json.Unmarshal([]byte(out), &top)
	for _, k := range []string{"schema", "run", "totals", "steps", "retention", "reads", "partial", "gaps"} {
		if _, ok := top[k]; !ok {
			t.Fatalf("top-level field %q missing", k)
		}
	}
	// The text view carries the same steps.
	text, _, _ := runTraceCmd(t, newFake(mainFixture()), "run-1")
	if n := strings.Count(text, "MODEL CALL"); n != len(ofKind(doc.Steps, "model_call")) {
		t.Fatalf("text has %d model calls, json %d", n, len(ofKind(doc.Steps, "model_call")))
	}
	if !strings.Contains(text, "What this run's record does not contain") {
		t.Fatalf("text lacks the gaps section")
	}
}

func TestTraceRunLiveRunHasNoInventedEnd(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		{seq: 2, ms: 10, kind: types.EventRunStarted},
		toolLoop(3, 1000, "working", nil),
	}}
	fx.run.State = types.RunRunning
	fx.run.FinishedAt = nil
	out, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	if !strings.Contains(out, "live, as of seq 3") {
		t.Fatalf("live run:\n%s", out)
	}
	doc := traceDoc(t, newFake(fx), "run-1")
	if !doc.Run.Live || doc.Run.FinishedAt != "" {
		t.Fatalf("header = %+v", doc.Run)
	}
}

func TestTraceRunSendsOnlyGetsToTheNamedRoutes(t *testing.T) {
	f := newFake(mainFixture())
	fx := f.runs["run-1"]
	fx.lifecycle = []types.LifecycleEvent{lifecycleCommit(1, 1700, "x")}
	_, _, code := runTraceCmd(t, f, "run-1")
	if code != 0 {
		t.Fatal(code)
	}
	for i, m := range f.methods {
		if m != http.MethodGet {
			t.Fatalf("request %s used %s", f.requests[i], m)
		}
		p := f.requests[i]
		ok := strings.HasPrefix(p, "/api/runs/run-1") || strings.HasPrefix(p, "/api/runs?") ||
			strings.HasPrefix(p, "/api/trajectories/traj-1/events") || strings.HasPrefix(p, "/api/inboxes/")
		if !ok {
			t.Fatalf("unexpected route %s", p)
		}
		if strings.Contains(p, "/internal/") {
			t.Fatalf("internal route %s", p)
		}
	}
	for _, a := range f.auths {
		if !strings.HasPrefix(a, "Bearer ") {
			t.Fatalf("a request went without the key")
		}
	}
}

func TestTraceRunShowsRecordsSentAndReceived(t *testing.T) {
	p := 0.7
	cast := types.CommitmentRecord{RecordID: "rec-cast", Kind: types.CommitmentKindDirective, Inbox: "research:doc-1", SenderInbox: "texture:doc-1",
		InboxSeq: 1, Round: 1, PSuccess: &p,
		Directive:  &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "find the filing"},
		Cast:       &types.CommitmentCast{Hypotheses: []types.CastHypothesis{{ID: "H1", Statement: "the filing exists", Probability: 0.7}}},
		Provenance: types.CommitmentProvenance{AgentID: "texture:doc-1", CommittedAt: tsStr(-60000), RunID: "run-texture"}}
	report := types.CommitmentRecord{RecordID: "rec-report", Kind: types.CommitmentKindReport, Inbox: "texture:doc-1", SenderInbox: "research:doc-1",
		InboxSeq: 1, Round: 1, Final: true,
		Report:     &types.CommitmentReport{Summary: "found three filings", StopReason: "answered"},
		Provenance: types.CommitmentProvenance{AgentID: "research:doc-1", CommittedAt: tsStr(5900), RunID: "run-1"}}
	fx := mainFixture()
	fx.inboxes = map[string][]types.CommitmentRecord{"research:doc-1": {cast}, "texture:doc-1": {report}}
	doc := traceDoc(t, newFake(fx), "run-1")
	recs := ofKind(doc.Steps, "record")
	if len(recs) != 1 {
		t.Fatalf("records during the run = %d, want the one sent: %+v", len(recs), recs)
	}
	r := recs[0].Record
	if r.Direction != "sent" || r.Inbox != "texture:doc-1" || r.Report != "found three filings" || !r.Final {
		t.Fatalf("record = %+v", r)
	}
	if doc.Run.WokenBy.Candidate == "" || !strings.Contains(doc.Run.WokenBy.Candidate, "rec-cast") {
		t.Fatalf("candidate waker = %q (must be labelled inferred)", doc.Run.WokenBy.Candidate)
	}
	text, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	for _, want := range []string{"inferred", "rec-cast", "sent record", "found three filings"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text lacks %q:\n%s", want, text)
		}
	}
	// A forecast the record stores is shown when the run received it mid-run.
	cast.Provenance.CommittedAt = tsStr(2000)
	fx.inboxes["research:doc-1"] = []types.CommitmentRecord{cast}
	doc = traceDoc(t, newFake(fx), "run-1")
	var got *traceRunRecord
	for _, s := range ofKind(doc.Steps, "record") {
		if s.Record.Direction == "received" {
			got = s.Record
		}
	}
	if got == nil || len(got.Forecast) != 1 || !strings.Contains(got.Forecast[0], "0.7") || got.PSuccess == nil {
		t.Fatalf("received record = %+v", got)
	}
}

func TestTraceRunControlsAndWorkItemsComeFromLifecycleEvents(t *testing.T) {
	fx := mainFixture()
	fx.lifecycle = []types.LifecycleEvent{
		{EventID: "le-1", TrajectoryID: "traj-1", RunID: "run-1", Kind: types.LifecycleControlDelivered, ReducerSeq: 1, WorkItemID: "wi-1", Reason: "owner steer", CreatedAt: at(2000)},
		{EventID: "le-2", TrajectoryID: "traj-1", RunID: "run-1", Kind: types.LifecycleWorkOpened, ReducerSeq: 2, WorkItemID: "wi-2", CreatedAt: at(2500)},
		{EventID: "le-3", TrajectoryID: "traj-1", RunID: "run-other", Kind: types.LifecycleWorkOpened, ReducerSeq: 3, CreatedAt: at(2600)},
	}
	doc := traceDoc(t, newFake(fx), "run-1")
	if len(ofKind(doc.Steps, "control")) != 1 || len(ofKind(doc.Steps, "work_item")) != 1 {
		t.Fatalf("steps = %+v", doc.Steps)
	}
	if c := ofKind(doc.Steps, "control")[0]; !strings.Contains(c.Title, "owner steer") {
		t.Fatalf("control = %+v", c)
	}
}

func TestTraceRunsListsAndSorts(t *testing.T) {
	mk := func(id, agent string, startMs, endMs int64, state string) map[string]any {
		m := map[string]any{"run_id": id, "agent_id": agent, "agent_profile": strings.SplitN(agent, ":", 2)[0], "state": state,
			"created_at": tsStr(startMs), "channel_id": "doc-1"}
		if endMs > 0 {
			m["finished_at"] = tsStr(endMs)
		}
		return m
	}
	f := newFake()
	f.listed = []map[string]any{
		mk("run-a", "research:doc-1", 3000, 4000, "completed"),
		mk("run-b", "texture:doc-1", 2000, 22000, "completed"),
		mk("run-c", "research:doc-2", 1000, 9000, "failed"),
	}
	list := func(args ...string) (string, string, int) {
		srv := httptest.NewServer(f)
		defer srv.Close()
		var out, errb bytes.Buffer
		code := run(append(append([]string{"trace", "runs"}, args...), "--host", srv.URL), &out, &errb)
		return out.String(), errb.String(), code
	}
	order := func(s string) string {
		var ids []string
		for _, id := range []string{"run-a", "run-b", "run-c"} {
			if i := strings.Index(s, id); i >= 0 {
				ids = append(ids, fmt.Sprintf("%05d:%s", i, id))
			}
		}
		sort.Strings(ids)
		for i := range ids {
			ids[i] = ids[i][6:]
		}
		return strings.Join(ids, ",")
	}
	out, _, code := list()
	if code != 0 || order(out) != "run-a,run-b,run-c" {
		t.Fatalf("default (newest first): %d %s", code, out)
	}
	out, _, _ = list("--sort", "duration")
	if order(out) != "run-b,run-c,run-a" {
		t.Fatalf("by duration: %s", out)
	}
	out, _, _ = list("--desk", "research")
	if strings.Contains(out, "run-b") || !strings.Contains(out, "run-a") || !strings.Contains(out, "run-c") {
		t.Fatalf("--desk research: %s", out)
	}
	out, _, _ = list("--limit", "1")
	if strings.Contains(out, "run-b") || strings.Contains(out, "run-c") {
		t.Fatalf("--limit 1: %s", out)
	}
	_, errs, code := list("--sort", "searches")
	if code != 2 || !strings.Contains(errs, "trace run") || !strings.Contains(errs, "does not return") {
		t.Fatalf("--sort searches: exit %d %s", code, errs)
	}
	out, _, _ = list()
	if strings.Contains(strings.ToLower(out), "searches") {
		t.Fatalf("the list offers a searches column the route cannot supply:\n%s", out)
	}
	var help bytes.Buffer
	_ = run([]string{"trace", "runs", "-h"}, &help, &help)
	if !strings.Contains(help.String(), "not available") {
		t.Fatalf("help does not say which columns are left out:\n%s", help.String())
	}
	var jb bytes.Buffer
	srv := httptest.NewServer(f)
	defer srv.Close()
	if code := run([]string{"trace", "runs", "--json", "--host", srv.URL}, &jb, &jb); code != 0 || !json.Valid(jb.Bytes()) {
		t.Fatalf("--json: %d %s", code, jb.String())
	}
}

func TestTraceRunHelpNamesFlagsAndGroupListsBothCommands(t *testing.T) {
	var help bytes.Buffer
	if code := run([]string{"trace", "run", "-h"}, &help, &help); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"--json", "--full", "read:runtime", "no model call"} {
		if !strings.Contains(help.String(), want) {
			t.Fatalf("help lacks %q:\n%s", want, help.String())
		}
	}
	var group bytes.Buffer
	_ = run([]string{"trace", "help"}, &group, &group)
	if !strings.Contains(group.String(), "trace run") || !strings.Contains(group.String(), "trace runs") {
		t.Fatalf("group help:\n%s", group.String())
	}
}

func TestTraceRunSaysResearchChildrenEventsAreNotAttached(t *testing.T) {
	fx := &runFixture{run: baseRun("run-1"), events: []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 500, "cast a child", nil),
		cellEvent(3, 600, 900, []map[string]any{egressRow("research", 650, 890, nil)}),
		toolResult(4, 590, 900, "choir.Research(...)", "{}"),
		{seq: 5, ms: 4000, kind: types.EventRunCompleted},
	}}
	doc := traceDoc(t, newFake(fx), "run-1")
	if !gapIDs(doc)["research_children_events"] {
		t.Fatalf("gaps = %+v", doc.Gaps)
	}
	if gapIDs(traceDoc(t, newFake(mainFixture()), "run-1"))["research_children_events"] {
		t.Fatal("a run with no research call lists the children gap")
	}
}

// ---- model calls under each desk's real event shapes ----

// runAs makes a run of a given desk whose events carry the real stored shape:
// the record's phase field is empty and the phase rides in the payload.
func deskFixture(desk, profile string, events []fixEvent) *runFixture {
	r := baseRun("run-1")
	r.AgentProfile, r.AgentRole = profile, profile
	r.AgentID = profile + ":doc-1"
	for i := range events {
		if events[i].payload != nil && events[i].payload["desk"] != nil {
			events[i].payload["desk"] = desk
		}
	}
	return &runFixture{run: r, events: events}
}

func servedEvent(seq, ms int64, role string, calls, in, out int) fixEvent {
	p := map[string]any{"schema": "choir.model_served.v1", "purpose": "tool_loop",
		"models": []map[string]any{{"provider": "devin", "model": "swe-2-flash", "calls": calls, "input_tokens": in, "output_tokens": out}}}
	if role != "" {
		p["role"] = role
	}
	return fixEvent{seq: seq, ms: ms, kind: types.EventModelServed, payload: p}
}

func TestTraceRunCountsModelCallsForEveryDesk(t *testing.T) {
	for _, desk := range []string{"management", "texture", "research"} {
		fx := deskFixture(desk, desk, []fixEvent{
			{seq: 1, ms: 0, kind: types.EventRunSubmitted},
			toolLoop(2, 1200, "", nil),
			cellEvent(3, 1300, 1400, nil),
			toolResult(4, 1300, 1400, `choir.Records()`, `{"stdout":"2"}`),
			toolLoop(5, 3000, "Reading the second record.", map[string]any{"iteration": 2}),
			cellEvent(6, 3100, 3200, nil),
			toolResult(7, 3100, 3200, `choir.Records()`, `{"stdout":"3"}`),
			{seq: 8, ms: 6000, kind: types.EventRunCompleted},
		})
		doc := traceDoc(t, newFake(fx), "run-1")
		if doc.Totals.ModelCalls != 2 || doc.Totals.ModelCallsStatus != "rows" || doc.Totals.InputTokens != 18000 {
			t.Fatalf("%s: totals = %+v", desk, doc.Totals)
		}
		if len(doc.Run.Models) != 1 || doc.Run.Models[0].Calls != 2 {
			t.Fatalf("%s: models = %+v", desk, doc.Run.Models)
		}
		for _, c := range ofKind(flatten(doc.Steps), "cell") {
			if c.Reason != nil && strings.Contains(c.Reason.Note, "no model call row") {
				t.Fatalf("%s: a cell has no model call: %+v", desk, c.Reason)
			}
		}
		out, _, _ := runTraceCmd(t, newFake(fx), "run-1")
		if strings.Contains(out, "model calls 0") {
			t.Fatalf("%s: text says model calls 0:\n%s", desk, out)
		}
	}
}

func TestTraceRunProgressRowsOtherThanModelCallsAreNamed(t *testing.T) {
	fx := deskFixture("management", "management", []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 1200, "", nil),
		{seq: 3, ms: 1500, kind: types.EventRunProgress, payload: map[string]any{"phase": "terminal_tool_success", "tool": "report"}},
		{seq: 4, ms: 6000, kind: types.EventRunCompleted},
	})
	out, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	if !strings.Contains(out, "terminal_tool_success") {
		t.Fatalf("a progress row with a payload phase is unnamed:\n%s", out)
	}
	if strings.Contains(out, "loop.progress \n") {
		t.Fatalf("an unnamed loop.progress step:\n%s", out)
	}
}

func TestTraceRunNoModelRowsIsNotRecordedNeverZero(t *testing.T) {
	fx := deskFixture("management", "management", []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		cellEvent(2, 1300, 1400, nil),
		toolResult(3, 1300, 1400, `choir.Records()`, `{"stdout":"2"}`),
		{seq: 4, ms: 6000, kind: types.EventRunCompleted},
	})
	doc := traceDoc(t, newFake(fx), "run-1")
	if doc.Totals.ModelCallsStatus != "not_recorded" {
		t.Fatalf("status = %q", doc.Totals.ModelCallsStatus)
	}
	out, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	if !strings.Contains(out, "model calls not recorded") || strings.Contains(out, "model calls 0") || strings.Contains(out, "tokens in 0") {
		t.Fatalf("text:\n%s", out)
	}
}

func TestTraceRunAgedOutModelRowsSayAgedOut(t *testing.T) {
	fx := deskFixture("management", "management", []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		cellEvent(2, 1300, 1400, nil),
		{seq: 3, ms: 6000, kind: types.EventRunCompleted},
	})
	f := newFake(fx)
	srv := httptest.NewServer(f)
	defer srv.Close()
	old := traceRunNow
	defer func() { traceRunNow = old }()
	traceRunNow = func() time.Time { return at(20 * 24 * 3600 * 1000) }
	var out, errb bytes.Buffer
	if code := run([]string{"trace", "run", "run-1", "--host", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "model calls aged out") || strings.Contains(out.String(), "model calls 0") {
		t.Fatalf("text:\n%s", out.String())
	}
	out.Reset()
	run([]string{"trace", "run", "run-1", "--json", "--host", srv.URL}, &out, &errb)
	var doc traceRunDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil || doc.Totals.ModelCallsStatus != "aged_out" {
		t.Fatalf("status = %q err=%v", doc.Totals.ModelCallsStatus, err)
	}
}

func TestTraceRunFallsBackToTheServedRecordAndCountsChildren(t *testing.T) {
	// A run past the 14-day window keeps model.served (kept for good), which
	// also carries a research child's calls under the child's role.
	fx := deskFixture("research", "research", []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		cellEvent(2, 1300, 1400, nil),
		servedEvent(3, 5000, "", 4, 40000, 1200),
		servedEvent(4, 5500, "research_web", 3, 9000, 700),
		{seq: 5, ms: 6000, kind: types.EventRunCompleted},
	})
	doc := traceDoc(t, newFake(fx), "run-1")
	if doc.Totals.ModelCalls != 7 || doc.Totals.ModelCallsStatus != "served_only" {
		t.Fatalf("totals = %+v", doc.Totals)
	}
	if len(doc.Run.Served) != 2 || doc.Run.Served[1].Role != "research_web" || doc.Run.Served[1].Calls != 3 {
		t.Fatalf("served = %+v", doc.Run.Served)
	}
	out, _, _ := runTraceCmd(t, newFake(fx), "run-1")
	if !strings.Contains(out, "model calls 7 (from model.served; per-call rows not recorded)") || !strings.Contains(out, "research_web") {
		t.Fatalf("text:\n%s", out)
	}
}

func TestTraceRunRowsAndServedBothPresentCountsRowsAndShowsChildrenSeparately(t *testing.T) {
	fx := deskFixture("research", "research", []fixEvent{
		{seq: 1, ms: 0, kind: types.EventRunSubmitted},
		toolLoop(2, 1200, "", nil),
		servedEvent(3, 5000, "", 1, 9000, 300),
		servedEvent(4, 5500, "research_web", 3, 9000, 700),
		{seq: 5, ms: 6000, kind: types.EventRunCompleted},
	})
	doc := traceDoc(t, newFake(fx), "run-1")
	if doc.Totals.ModelCalls != 1 || doc.Totals.ModelCallsStatus != "rows" || doc.Totals.ChildModelCalls != 3 {
		t.Fatalf("totals = %+v", doc.Totals)
	}
}

func TestTraceRunStepsSayNothingAboutWhatTheRecordLacks(t *testing.T) {
	out, _, _ := runTraceCmd(t, newFake(mainFixture()), "run-1")
	i := strings.Index(out, "What this run's record does not contain")
	if i < 0 {
		t.Fatalf("no closing list:\n%s", out)
	}
	steps := out[strings.Index(out, "Steps ("):i]
	if strings.Contains(steps, "not recorded") {
		t.Fatalf("a step repeats what the closing list says once:\n%s", steps)
	}
}
