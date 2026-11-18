package agentcore

// Helpers for the station L1 tests (docs/design/autoradio-l1-failure-list-2026-11-15.md).
// The provider is the only fake: it implements provideriface.TextStreamProvider
// and nothing else (the same shape the gateway's Provider has, asserted at
// compile time in the wiring test). The store, the handler, the claims
// constructor and the record writers are the real ones.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const progOwner = "owner-1"

var regexpStretch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,31}$`)

func parseTS(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("timestamp %q: %v", s, err)
	}
	return ts
}

// progCandidates are the three candidates of the default request. Their refs
// are pub:harbour-1@v1, pub:harbour-2@v1 and pub:budget-3@v1.
func progCandidates() []map[string]any {
	return []map[string]any{
		{"route": "harbour-1", "version": 1, "title": "Harbour reopens", "dek": "Repairs are done.", "text": "The harbour reopened today after repairs."},
		{"route": "harbour-2", "version": 1, "title": "Ferry timetable", "dek": "New sailings.", "text": "The ferry adds two sailings."},
		{"route": "budget-3", "version": 1, "title": "Budget vote", "dek": "Council votes.", "text": "The council votes on the budget on Monday."},
	}
}

const progEdition = "The edition paragraph about the harbour.\n\nThe edition paragraph about the budget.\n\nThe edition paragraph about music."

// progBody is the default request body, with the mutations applied.
func progBody(mutate ...func(map[string]any)) string {
	m := map[string]any{
		"request_id": "r-1", "cause": "extend", "local_time": "2026-11-15T07:40:00+01:00", "tz": "Europe/Paris",
		"candidates": progCandidates(), "edition_text": progEdition,
	}
	for _, f := range mutate {
		f(m)
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func withRequestID(id string) func(map[string]any) {
	return func(m map[string]any) { m["request_id"] = id }
}

// progLine is a valid content line with the overrides applied; a nil override
// deletes the field.
func progLine(over map[string]any) string {
	m := map[string]any{
		"kind": "story", "title": "ok", "text": "The harbour reopened today.\n\nThe ferries run again.",
		"cites": []any{"pub:harbour-1@v1"}, "p_heard": 0.7, "caused_by": "importance",
	}
	for k, v := range over {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func titled(title string, over map[string]any) string {
	if over == nil {
		over = map[string]any{}
	}
	over["title"] = title
	return progLine(over)
}

// linesScript emits each line, with its newline, as its own delta.
func linesScript(lines ...string) radioScript {
	return func(_ int, _ context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		var all strings.Builder
		for _, l := range lines {
			all.WriteString(l + "\n")
			onDelta(l + "\n")
		}
		return &provideriface.TextStreamResponse{Text: all.String(), Model: req.Model, Usage: provideriface.TokenUsage{InputTokens: 100, OutputTokens: 50}}, nil
	}
}

// deltaScript emits the given raw deltas.
func deltaScript(deltas ...string) radioScript {
	return okAnswer(deltas...)
}

func progPost(rt *Runtime, body string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, []sseEvent) {
	rec := httptest.NewRecorder()
	progServe(NewAPIHandler(rt), rec, body, mutate...)
	return rec, nil
}

func progServe(h *APIHandler, w http.ResponseWriter, body string, mutate ...func(*http.Request)) {
	req := httptest.NewRequest(http.MethodPost, "/api/autoradio/programme", strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", progOwner)
	for _, m := range mutate {
		m(req)
	}
	h.HandleAutoradioProgramme(w, req)
}

// progRun posts a body and parses the stream.
func progRun(t *testing.T, rt *Runtime, body string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, []sseEvent) {
	t.Helper()
	rec, _ := progPost(rt, body, mutate...)
	return rec, parseSSE(t, rec.Body.String())
}

func segmentEvents(evs []sseEvent) []map[string]any {
	var out []map[string]any
	for _, e := range eventsNamed(evs, "segment") {
		out = append(out, e.Data)
	}
	return out
}

func segmentTitles(evs []sseEvent) []string {
	var out []string
	for _, s := range segmentEvents(evs) {
		out = append(out, s["title"].(string))
	}
	return out
}

func stretchIDOf(t *testing.T, evs []sseEvent) string {
	t.Helper()
	st := eventsNamed(evs, "stretch")
	if len(st) != 1 {
		t.Fatalf("want one stretch event, got %d: %+v", len(st), evs)
	}
	return st[0].Data["stretch_id"].(string)
}

// tape reads every record of one Autoradio record kind through the real
// store's subject-ref listing.
func tape(t *testing.T, rt *Runtime, kind string) []types.CommitmentRecord {
	t.Helper()
	recs, err := rt.store.ListCommitmentRecordsBySubjectRef(context.Background(), progOwner, rt.cfg.ComputerID, "record_kind", kind, 100000)
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func getRecord(t *testing.T, rt *Runtime, id string) *types.CommitmentRecord {
	t.Helper()
	rec, err := rt.store.GetCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, id)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// stretchClaims lists a stretch's forecasts through the real tag query.
func stretchClaims(t *testing.T, rt *Runtime, stretchID string) []types.CommitmentRecord {
	t.Helper()
	claims, truncated, err := calibration.Reader{Ledger: rt.store}.StretchForecasts(context.Background(), progOwner, rt.cfg.ComputerID, stretchID)
	if err != nil || truncated {
		t.Fatalf("stretch claims: %v truncated=%v", err, truncated)
	}
	return claims
}

func allRecordCount(t *testing.T, rt *Runtime) int {
	t.Helper()
	recs, err := rt.store.ListCommitmentRecords(context.Background(), progOwner, rt.cfg.ComputerID, "", 100000)
	if err != nil {
		t.Fatal(err)
	}
	return len(recs)
}

func tagValue(claim types.CommitmentRecord, prefix string) string {
	for _, tag := range claim.Calibration.Claim.Tags {
		if v, ok := strings.CutPrefix(tag, prefix); ok {
			return v
		}
	}
	return ""
}

// seedInputs puts into the store what a call's input can list: a turn the
// listener spoke (through the real turn route), a notebook note and an active
// lesson. The turn is "turn-1", the note "ab12" and the lesson "L1".
func seedInputs(t *testing.T, rt *Runtime, fake *fakeRadio) {
	t.Helper()
	prev := fake.script
	fake.script = okAnswer("Fine.")
	if rec := radioPost(rt, `{"turn_id":"turn-1","words":"Less sport, more harbour news please.","page":{"text":"p"}}`); rec.Code != http.StatusOK {
		t.Fatalf("seed turn: %d %s", rec.Code, rec.Body.String())
	}
	fake.script = prev
	fake.mu.Lock()
	fake.calls = nil
	fake.mu.Unlock()
	for _, s := range []struct{ id, kind, body string }{
		{"autoradio-note:ab12", "autoradio.note", "Keep the harbour stories short."},
		{"autoradio-lesson:L1", "autoradio.lesson", "Harbour stories are heard through in the morning."},
	} {
		label, ok := types.AutoradioKindLabel(s.kind)
		if !ok {
			t.Fatalf("kind %s", s.kind)
		}
		rec := types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: s.id, Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: s.body, Body: s.body},
			Provenance: types.CommitmentProvenance{AgentID: progOwner, ModelID: "owner", Origin: types.OriginOwner, CommittedAt: "2026-11-14T10:00:00Z",
				Labels: types.NewLabelSet(label).Strings()},
			EmitterKind: types.CommitmentEmitterOwner, ComputerID: rt.cfg.ComputerID,
			SubjectRefs: map[string]string{"desk": autoradioDesk, "record_kind": s.kind},
		}
		if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, rec); err != nil {
			t.Fatal(err)
		}
	}
}

// hookWriter is a ResponseWriter that calls onEvent, with the event's name and
// data, at the moment an event's bytes reach it.
type hookWriter struct {
	*httptest.ResponseRecorder
	mu      sync.Mutex
	onEvent func(name string, data map[string]any)
}

func newHookWriter(onEvent func(name string, data map[string]any)) *hookWriter {
	return &hookWriter{ResponseRecorder: httptest.NewRecorder(), onEvent: onEvent}
}

func (w *hookWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.ResponseRecorder.Write(p)
	w.mu.Unlock()
	s := string(p)
	if strings.HasPrefix(s, "event: ") && w.onEvent != nil {
		name, rest, _ := strings.Cut(strings.TrimPrefix(s, "event: "), "\n")
		var data map[string]any
		_ = json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(rest, "data: "))), &data)
		w.onEvent(name, data)
	}
	return n, err
}

// WriteString is promoted from the recorder and would bypass Write.
func (w *hookWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *hookWriter) body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Body.String()
}

// wrapLedger is a store decorator for the failures a real store does not
// produce on demand: it can refuse the append of a claim, or read a claim
// back changed. It adds no capability the store lacks.
type wrapLedger struct {
	autoradioLedger
	failClaimAppend bool
	changeClaimRead bool
	// failClaimFrom > 0 refuses the claim appends from the nth on (1-based).
	failClaimFrom int
	claimAppends  int
	// panicClaimAppend panics instead of appending a claim: the process dies
	// between the segment record and its claim.
	panicClaimAppend bool
	// failListKind makes the list of that record_kind fail; listCalls counts
	// the lists by record_kind.
	failListKind string
	listCalls    map[string]int
	// listDelay makes each record_kind list take that long; failPaged makes a
	// scorecard page read fail.
	listDelay time.Duration
	failPaged bool
	// panicKind makes the predicate list of that record_kind panic; barrierN
	// (with barrier made by newBarrierLedger) holds each predicate list until
	// that many have started; queries keeps the predicate queries.
	panicKind string
	barrierN  int
	barrier   chan struct{}
	started   int
	mu        sync.Mutex
	queries   []types.BodyListQuery
}

func newBarrierLedger(inner autoradioLedger, n int) *wrapLedger {
	return &wrapLedger{autoradioLedger: inner, barrierN: n, barrier: make(chan struct{})}
}

func (l *wrapLedger) noteList(key, value string) error {
	if key != "record_kind" {
		return nil
	}
	l.mu.Lock()
	if l.listCalls == nil {
		l.listCalls = map[string]int{}
	}
	l.listCalls[value]++
	l.mu.Unlock()
	if l.listDelay > 0 {
		time.Sleep(l.listDelay)
	}
	if l.failListKind != "" && l.failListKind == value {
		return context.DeadlineExceeded
	}
	return nil
}

// noteBody counts a predicate list by its record_kind, keeps the query, and
// applies the same delay, failure, barrier and panic as a subject-ref list.
func (l *wrapLedger) noteBody(q types.BodyListQuery) error {
	l.mu.Lock()
	l.queries = append(l.queries, q)
	l.mu.Unlock()
	if l.panicKind != "" && l.panicKind == q.Match["$.subject_refs.record_kind"] {
		panic(wrapLedgerPanicWords)
	}
	if err := l.noteList("record_kind", q.Match["$.subject_refs.record_kind"]); err != nil {
		return err
	}
	return l.waitBarrier()
}

// waitBarrier holds a list until barrierN lists have started: a read made one
// after the other never gets past it (it times out and fails).
func (l *wrapLedger) waitBarrier() error {
	if l.barrierN == 0 {
		return nil
	}
	l.mu.Lock()
	l.started++
	if l.started == l.barrierN {
		close(l.barrier)
	}
	ch := l.barrier
	l.mu.Unlock()
	select {
	case <-ch:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("the lists were not started together")
	}
}

func (l *wrapLedger) ListCommitmentRecordsBySubjectRef(ctx context.Context, ownerID, computerID, key, value string, limit int) ([]types.CommitmentRecord, error) {
	if err := l.noteList(key, value); err != nil {
		return nil, err
	}
	return l.autoradioLedger.ListCommitmentRecordsBySubjectRef(ctx, ownerID, computerID, key, value, limit)
}

func (l *wrapLedger) ListCalibrationRecordsPage(ctx context.Context, ownerID, computerID, agent, after string, limit int) ([]types.CommitmentRecord, string, error) {
	if l.failPaged {
		return nil, "", context.DeadlineExceeded
	}
	return l.autoradioLedger.(interface {
		ListCalibrationRecordsPage(ctx context.Context, ownerID, computerID, agent, after string, limit int) ([]types.CommitmentRecord, string, error)
	}).ListCalibrationRecordsPage(ctx, ownerID, computerID, agent, after, limit)
}

// ListCommitmentRecordsByBodyOfComputer is the predicate list: the newest rows
// of one computer whose body fields equal match.
func (l *wrapLedger) ListCommitmentRecordsByBodyOfComputer(ctx context.Context, ownerID, computerID string, q types.BodyListQuery, limit int) ([]types.CommitmentRecord, int, int, error) {
	if err := l.noteBody(q); err != nil {
		return nil, 0, 0, err
	}
	return l.autoradioLedger.(interface {
		ListCommitmentRecordsByBodyOfComputer(ctx context.Context, ownerID, computerID string, q types.BodyListQuery, limit int) ([]types.CommitmentRecord, int, int, error)
	}).ListCommitmentRecordsByBodyOfComputer(ctx, ownerID, computerID, q, limit)
}

func (l *wrapLedger) ListCommitmentRecordsBySubjectRefOfComputer(ctx context.Context, ownerID, computerID, key, value string, limit int) ([]types.CommitmentRecord, int, int, error) {
	if err := l.noteList(key, value); err != nil {
		return nil, 0, 0, err
	}
	return l.autoradioLedger.ListCommitmentRecordsBySubjectRefOfComputer(ctx, ownerID, computerID, key, value, limit)
}

// errWriter is a ResponseWriter whose writes fail after the first okWrites
// (the client went away).
type errWriter struct {
	*httptest.ResponseRecorder
	okWrites, writes int
}

func (w *errWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.okWrites {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(p)
}

func (w *errWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (l *wrapLedger) AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error) {
	if rec.Calibration != nil && rec.Calibration.Claim != nil {
		l.claimAppends++
		if l.panicClaimAppend {
			panic("simulated crash between the segment record and its claim")
		}
		if l.failClaimAppend || (l.failClaimFrom > 0 && l.claimAppends >= l.failClaimFrom) {
			return "", context.DeadlineExceeded
		}
	}
	return l.autoradioLedger.AppendCommitmentRecord(ctx, ownerID, computerID, rec)
}

func (l *wrapLedger) GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error) {
	rec, err := l.autoradioLedger.GetCommitmentRecord(ctx, ownerID, computerID, recordID)
	if l.changeClaimRead && rec != nil && rec.Calibration != nil && rec.Calibration.Claim != nil {
		cal := *rec.Calibration
		claim := *cal.Claim
		claim.Criterion = claim.Criterion + " (changed)"
		cal.Claim = &claim
		cp := *rec
		cp.Calibration = &cal
		return &cp, err
	}
	return rec, err
}

// wrapLedgerPanicWords is what the injected panic says; the log must not
// carry it (a panic value can hold a listener's words).
const wrapLedgerPanicWords = "injected panic in a heard read ZEBRA-PANIC"
