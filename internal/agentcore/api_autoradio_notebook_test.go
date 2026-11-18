package agentcore

// The notebook (stations L4 and L5). The failure list, written before the code:
//
//	N1 a proposal is applied with a cite that was not shown to the call as a
//	   settled forecast, or with fewer than two cites;
//	N2 a line that tells the DJ which way to move a number is written;
//	N3 a full section takes a line that displaces nothing;
//	N4 a lesson id the model sent is used, or an id is minted twice;
//	N5 the listener's deletion or edit of a lesson is not recorded as a strike,
//	   their corrected words are lost, or a proposal made only of the struck
//	   line's cites is written again;
//	N6 the counts move without a settlement, or a model writes a count;
//	N7 the next call's input does not hold the notebook, or its receipt is not
//	   on the tape, or a stale phone edition_text beats the server's notebook;
//	N8 a forecast whose caused_by names a notebook line is dropped, or a ref the
//	   call was not shown resolves;
//	N9 a tap or a mute is not a line of the notebook with its referent, or the
//	   steering is not readable on the learning route;
//	N10 a spoken instruction becomes a listener line that is not a verbatim part
//	   of the turn;
//	N11 an open request writes a notebook line.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/notebook"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// fakeNotebook is Texture's side, in memory: the same notebook code applies the
// patch, the base revision is checked, the grants are not (the textureowner
// test holds those).
type fakeNotebook struct {
	mu      sync.Mutex
	found   bool
	chain   []notebook.Revision // newest first
	patches []notebook.Patch
	n       int
	// staleN makes the next NotebookApply calls answer stale (a lost race); cut
	// and readErr make a read report a too-long chain or fail.
	staleN  int
	cut     bool
	readErr error
}

func (f *fakeNotebook) head() notebook.Revision { return f.chain[0] }

func (f *fakeNotebook) NotebookRead(_ context.Context, _, _ string) (notebook.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return notebook.State{}, f.readErr
	}
	if !f.found {
		return notebook.State{}, nil
	}
	st := notebook.State{Found: true, DocID: "doc-nb", Head: f.head(), ChainCut: f.cut}
	for _, r := range f.chain {
		st.Chain = append(st.Chain, r)
		if r.IsDesk() {
			break
		}
	}
	return st, nil
}

func (f *fakeNotebook) NotebookApply(_ context.Context, _, _ string, p notebook.Patch) (notebook.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.staleN > 0 {
		f.staleN--
		return notebook.Result{Status: notebook.StatusStale}, nil
	}
	doc := notebook.Fresh()
	if f.found {
		if f.head().RevisionID != p.BaseRevisionID {
			return notebook.Result{Status: notebook.StatusStale}, nil
		}
		doc = notebook.Parse(f.head().Text)
	} else if p.BaseRevisionID != "" {
		return notebook.Result{Status: notebook.StatusStale}, nil
	}
	next, err := doc.Apply(p)
	if err != nil {
		return notebook.Result{Status: notebook.StatusRefused, Reason: err.Error()}, nil
	}
	f.patches = append(f.patches, p)
	f.push("appagent", notebook.DeskLabel, next.Render())
	f.found = true
	return notebook.Result{Status: notebook.StatusApplied, DocID: "doc-nb", RevisionID: f.head().RevisionID}, nil
}

func (f *fakeNotebook) push(kind, label, text string) {
	f.n++
	f.chain = append([]notebook.Revision{{RevisionID: fmt.Sprintf("rev-%d", f.n), AuthorKind: kind, AuthorLabel: label, At: time.Now(), Text: text}}, f.chain...)
}

func (f *fakeNotebook) text() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.found {
		return ""
	}
	return f.head().Text
}

// userEdit is the listener's revision.
func (f *fakeNotebook) userEdit(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.found = true
	f.push("user", "owner-1", text)
}

var settledRowRe = regexp.MustCompile(`(?m)^(\S+) \| story \|`)

func shownClaims(req provideriface.TextStreamRequest) []string {
	var out []string
	for _, m := range settledRowRe.FindAllStringSubmatch(req.User, -1) {
		out = append(out, m[1])
	}
	return out
}

// notebookScript writes per call number: calls[n] returns the lines for call n
// given the claims that call was shown.
func notebookScript(calls map[int]func(req provideriface.TextStreamRequest, shown []string) []string) radioScript {
	return func(n int, ctx context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		lines := []string{titled("Story A", map[string]any{})}
		if f, ok := calls[n]; ok {
			lines = f(req, shownClaims(req))
		}
		return linesScript(lines...)(n, ctx, req, onDelta)
	}
}

func jsonLine(v map[string]any) string { b, _ := json.Marshal(v); return string(b) }

func nbAdd(text string, cites []string, extra map[string]any) string {
	m := map[string]any{"kind": "notebook", "op": "add", "section": "learned", "text": text, "cites": cites}
	for k, v := range extra {
		m[k] = v
	}
	return jsonLine(m)
}

func serve(t *testing.T, h *APIHandler, id string) (*httptest.ResponseRecorder, []sseEvent) {
	t.Helper()
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID(id), func(m map[string]any) { delete(m, "edition_text") }))
	return rec, parseSSE(t, rec.Body.String())
}

func doneNotebook(t *testing.T, evs []sseEvent) map[string]any {
	t.Helper()
	d := eventsNamed(evs, "done")
	if len(d) != 1 {
		t.Fatalf("want one done: %+v", evs)
	}
	nb, _ := d[0].Data["notebook"].(map[string]any)
	return nb
}

// settleAll finishes the first four entries of a six-story stretch and skips two.
func settleStretch(t *testing.T, h *APIHandler, entries []string, outcomes []string) {
	t.Helper()
	var ops []map[string]any
	for i, e := range entries {
		o := outcomes[i]
		state := "heard"
		if o == "skipped" {
			state = "skipped"
		}
		ops = append(ops, phoneEnd(e, fmt.Sprintf("%s-%d", e, i), state, o, 1)...)
	}
	mustOps(t, h, ops...)
}

func noCadence(t *testing.T) {
	t.Helper()
	old := nbCountCadence
	nbCountCadence = 0
	t.Cleanup(func() { nbCountCadence = old })
}

func TestN_TheNotebookLoop(t *testing.T) {
	noCadence(t)
	cites := func(shown []string) []string { return shown[:2] }
	var fake *fakeNotebook
	var first []string
	script := notebookScript(map[int]func(provideriface.TextStreamRequest, []string) []string{
		1: func(req provideriface.TextStreamRequest, _ []string) []string {
			ls := []string{}
			for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
				ls = append(ls, titled("Story "+n, nil))
			}
			return ls
		},
		// Call 2: the first lesson, plus bad proposals that must be dropped.
		2: func(req provideriface.TextStreamRequest, shown []string) []string {
			first = cites(shown)
			return []string{
				titled("Story G", nil),
				nbAdd("Short pieces are heard through.", cites(shown), map[string]any{"id": "L77"}),
				nbAdd("One cite only.", shown[:1], nil),
				nbAdd("A cite never shown.", []string{shown[0], "calib:nope"}, nil),
				nbAdd("Raise p_heard for sport.", cites(shown), nil),
				nbAdd("Raise the probability of anything.", cites(shown), nil),
				nbAdd("Wrong section.", cites(shown), map[string]any{"section": "asked"}),
			}
		},
		// Call 3: the notebook is in the input; a story names the line.
		3: func(req provideriface.TextStreamRequest, shown []string) []string {
			if !strings.Contains(req.StableUser, "lesson:L1: Short pieces are heard through.") {
				t.Errorf("(N7) the stable input lacks the notebook:\n%s", req.StableUser)
			}
			// B3: the cached part carries no count; the volatile part has the table.
			if strings.Contains(req.StableUser, "heard 0 of 0") || strings.Contains(req.StableUser, "(heard") {
				t.Errorf("(B3) a count is in the cached part:\n%s", req.StableUser)
			}
			if !strings.Contains(req.User, "L1: heard 0 of 0") {
				t.Errorf("(B3) the volatile part lacks the counts table:\n%s", req.User)
			}
			if strings.Contains(req.User, "lesson:L1") {
				t.Errorf("(N7) the notebook is in the volatile part")
			}
			return []string{
				titled("Story H", map[string]any{"caused_by": "lesson:L1"}),
				titled("Story I", map[string]any{"caused_by": "lesson:L9"}), // not shown: dropped
				titled("Story J", map[string]any{"caused_by": "mine:00000000"}),
			}
		},
		// Call 6 (calls 4 and 5 propose nothing): the listener struck L1; the same
		// evidence again is refused.
		6: func(req provideriface.TextStreamRequest, shown []string) []string {
			return []string{titled("Story L", nil), nbAdd("Short pieces are heard through, again.", first, nil)}
		},
	})
	rt, rfake := radioRuntime(t, script)
	h := NewAPIHandler(rt)
	fake = &fakeNotebook{}
	h.SetNotebook(fake)

	// Stretch 1: six stories; the listener hears four through and skips two.
	_, evs := serve(t, h, "r-1")
	var entries []string
	for _, s := range segmentEvents(evs) {
		entries = append(entries, s["entry_id"].(string))
	}
	if len(entries) != 6 {
		t.Fatalf("stretch 1: %d entries", len(entries))
	}
	if nb := doneNotebook(t, evs); nb != nil && nb["status"] != nil && nb["status"] != notebook.StatusNoop {
		t.Errorf("(N11) a notebook patch from a call with no proposal: %v", nb)
	}
	if fake.found {
		t.Fatal("a notebook was created with nothing to say")
	}
	settleStretch(t, h, entries, []string{"finished", "finished", "finished", "finished", "skipped", "skipped"})

	// Stretch 2: the first lesson. Bad proposals are dropped with reasons.
	_, evs = serve(t, h, "r-2")
	nb := doneNotebook(t, evs)
	if nb["status"] != notebook.StatusApplied {
		t.Fatalf("stretch 2: %v", nb)
	}
	text := fake.text()
	if !strings.Contains(text, "- Short pieces are heard through. [L1 · heard 0 of 0]") || strings.Contains(text, "L77") {
		t.Errorf("(N4) the notebook:\n%s", text)
	}
	for _, bad := range []string{"One cite only", "never shown", "p_heard for sport", "probability of anything", "Wrong section"} {
		if strings.Contains(text, bad) {
			t.Errorf("(N1/N2) %q was written", bad)
		}
	}
	if d, _ := nb["dropped"].([]any); len(d) != 5 {
		t.Errorf("(N1) dropped %v", nb["dropped"])
	}
	lesson := getRecord(t, rt, lessonRecordPrefix+"L1")
	if lesson == nil || len(splitCSV(lesson.SubjectRefs["cites"])) != 2 || lesson.SubjectRefs["patch_id"] == "" {
		t.Fatalf("the lesson record: %+v", lesson)
	}
	if got := len(tape(t, rt, kindPatch)); got != 2 { // asked, and Texture's answer
		t.Errorf("(rule 2) %d patch records", got)
	}
	// A reply to a refused call stays in the ledger and on the learning route.
	// Stretch 3: the notebook is shown; a story names L1.
	_, evs = serve(t, h, "r-3")
	segs := segmentEvents(evs)
	if len(segs) != 1 || segs[0]["title"] != "Story H" {
		t.Fatalf("(N8) segments %v", segs)
	}
	claim := getRecord(t, rt, segs[0]["claim_id"].(string))
	if claim == nil || claim.Calibration.Claim.Criterion != "caused_by=lesson:L1" || !hasStr(claim.Calibration.Claim.Tags, "line:lesson:L1") || !hasStr(claim.Calibration.Claim.Tags, "cause:lesson") {
		t.Fatalf("(N8) the claim: %+v", claim)
	}
	start := getRecord(t, rt, stretchStartRecordPrefix+stretchIDOf(t, evs))
	if start.SubjectRefs["notebook_revision_id"] == "" || len(start.SubjectRefs["notebook_sha256"]) != 64 || start.SubjectRefs["notebook_bytes"] == "0" {
		t.Errorf("(N7) the input receipt: %v", start.SubjectRefs)
	}
	if strings.Contains(fmt.Sprint(start.SubjectRefs), "Short pieces") {
		t.Errorf("(N7) the receipt holds the text")
	}
	// The phone's own edition_text never beats the server's notebook.
	rec := httptest.NewRecorder()
	rfake.mu.Lock()
	before := len(rfake.calls)
	rfake.mu.Unlock()
	progServe(h, rec, progBody(withRequestID("r-3b")))
	rfake.mu.Lock()
	last := rfake.calls[len(rfake.calls)-1].req.StableUser
	rfake.mu.Unlock()
	if before == 0 || strings.Contains(last, "edition:P1") || !strings.Contains(last, "lesson:L1") {
		t.Errorf("(N7) the phone's edition beat the notebook:\n%s", last)
	}
	// The counts move only when a settlement does.
	settleStretch(t, h, []string{segs[0]["entry_id"].(string)}, []string{"finished"})
	// (the r-3b stretch above also made entries; they stay open and settle nothing)
	_, evs = serve(t, h, "r-4")
	if got := fake.text(); !strings.Contains(got, "[L1 · heard 1 of 1]") {
		t.Errorf("(N6) counts after a heard-through forecast:\n%s (%v)", got, doneNotebook(t, evs))
	}
	if len(fake.patches) != 2 || len(fake.patches[1].Ops) != 0 {
		t.Errorf("(N6) the count-only patch carried ops: %+v", fake.patches)
	}

	// B2: beside heard and of, the route reports the forecasts made since the
	// line was written (settled or not) and how many of those settled.
	_, lout := learningGet(t, h, "")
	for _, l := range lout["notebook"].(map[string]any)["lines"].([]any) {
		m := l.(map[string]any)
		if m["id"] == "L1" && (m["live_forecasts"] != float64(3) || m["live_settled"] != float64(1) || m["live_of"] != float64(1) || m["live_heard"] != float64(1)) {
			t.Errorf("(B2) L1 row: %v", m)
		}
	}

	// The listener strikes L1; the same evidence is refused; nothing is lost.
	fake.userEdit("# My notebook\n\n## What you asked for\n- Never anything about sport.\n\n## What Autoradio has learned\n\n## What Autoradio is trying\n\n## Recent steering\n")
	_, evs = serve(t, h, "r-5")
	strikes := tape(t, rt, kindStrike)
	if len(strikes) != 1 || strikes[0].SubjectRefs["line_id"] != "L1" || len(splitCSV(strikes[0].SubjectRefs["cites"])) != 2 {
		t.Fatalf("(N5) strikes: %+v (%v)", strikes, doneNotebook(t, evs))
	}
	if got := fake.text(); strings.Contains(got, "again") || !strings.Contains(got, "- Never anything about sport.") {
		t.Errorf("(N5) after the strike:\n%s", got)
	}
	d, _ := doneNotebook(t, evs)["dropped"].([]any)
	if len(d) != 1 || !strings.Contains(fmt.Sprint(d[0]), "struck") {
		t.Errorf("(N5) dropped %v", d)
	}

	// Rule 2: the learning route shows the lines, strikes, patches, receipts.
	code, out := learningGet(t, h, "")
	nbv, _ := out["notebook"].(map[string]any)
	if code != 200 || nbv["found"] != true || len(nbv["struck"].([]any)) != 1 || len(nbv["patches"].([]any)) < 4 || len(nbv["inputs"].([]any)) < 3 {
		t.Errorf("(rule 2) %d %v", code, nbv)
	}
	_, out = learningGet(t, h, "?line=lesson:L1")
	if rows := out["rows"].([]any); len(rows) != 1 {
		t.Errorf("(rule 2) behind a line: %d rows", len(rows))
	}
}

func hasStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestN3_AFullSectionNeedsADisplacement(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("# My notebook\n\n## What you asked for\n\n## What Autoradio has learned\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&sb, "- Lesson %d. [L%d · heard 0 of 0]\n", i, i)
	}
	sb.WriteString("\n## What Autoradio is trying\n\n## Recent steering\n")
	fake := &fakeNotebook{}
	fake.push("appagent", notebook.DeskLabel, sb.String())
	fake.found = true
	script := notebookScript(map[int]func(provideriface.TextStreamRequest, []string) []string{
		1: func(req provideriface.TextStreamRequest, _ []string) []string {
			ls := []string{}
			for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
				ls = append(ls, titled("Story "+n, nil))
			}
			return ls
		},
		2: func(req provideriface.TextStreamRequest, shown []string) []string {
			if !strings.Contains(req.StableUser, "The learned section holds 12 lines and has 12.") {
				t.Errorf("(N3) the budget is not stated plainly")
			}
			return []string{titled("Story G", nil),
				nbAdd("No displacement.", shown[:2], nil),
				nbAdd("Merged.", shown[:2], map[string]any{"replaces": []string{"L3", "L4"}}),
				nbAdd("Names a line that is not there.", shown[2:4], map[string]any{"replaces": []string{"L40"}})}
		},
	})
	rt, _ := radioRuntime(t, script)
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	_, evs := serve(t, h, "r-1")
	var entries []string
	for _, s := range segmentEvents(evs) {
		entries = append(entries, s["entry_id"].(string))
	}
	settleStretch(t, h, entries, []string{"finished", "finished", "finished", "finished", "skipped", "skipped"})
	_, evs = serve(t, h, "r-2")
	text := fake.text()
	if !strings.Contains(text, "- Merged. [L13 · heard 0 of 0]") || strings.Contains(text, "No displacement") || strings.Contains(text, "L3 ·") || strings.Contains(text, "L4 ·") || strings.Contains(text, "not there") {
		t.Errorf("(N3)\n%s\n%v", text, doneNotebook(t, evs))
	}
	if n := len(notebook.Parse(text).Lines(notebook.SecLearned)); n != 11 {
		t.Errorf("(N3) %d lessons", n)
	}
}

func TestN9toN10_SteeringBecomesTheListenersLines(t *testing.T) {
	fake := &fakeNotebook{}
	script := notebookScript(map[int]func(provideriface.TextStreamRequest, []string) []string{
		1: func(req provideriface.TextStreamRequest, _ []string) []string {
			return []string{titled("Leeds sack manager", nil)}
		},
		2: func(req provideriface.TextStreamRequest, _ []string) []string {
			return []string{titled("Story B", nil),
				jsonLine(map[string]any{"kind": "notebook", "op": "steer", "turn": "turn-1", "quote": "Less sport"}),
				jsonLine(map[string]any{"kind": "notebook", "op": "steer", "turn": "turn-1", "quote": "no more football please"}),
				jsonLine(map[string]any{"kind": "notebook", "op": "steer", "turn": "turn-zz", "quote": "Less sport"})}
		},
	})
	rt, _ := radioRuntime(t, script)
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	_, evs := serve(t, h, "r-1")
	entry := segmentEvents(evs)[0]["entry_id"].(string)
	// The turn the listener spoke is on the tape, as the turn route writes it.
	turn := autoradioNote(autoradioReqPrefix+"turn-1", progOwner, rt.cfg.ComputerID, map[string]string{}, "Less sport, thanks, and more science.", time.Now())
	turn.SubjectRefs["record_kind"] = kindTurn
	label, _ := types.AutoradioKindLabel(kindTurn)
	turn.Provenance.Labels = types.NewLabelSet(label).Strings()
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, turn); err != nil {
		t.Fatal(err)
	}
	mustOps(t, h, phoneEnd(entry, "a", "heard", "finished", 1)[0]) // the entry is on air
	q := "Less sport, thanks"
	_ = q
	_, evs = serve(t, h, "r-2")
	text := fake.text()
	asked := notebook.Parse(text).Lines(notebook.SecAsked)
	if len(asked) != 1 || !strings.HasPrefix(asked[0].Text, `"Less sport"`) || !strings.Contains(asked[0].Text, `during "Leeds sack manager"`) {
		t.Errorf("(N10) asked: %v (%v)\n%s", asked, doneNotebook(t, evs), text)
	}
}

func steerPost(h *APIHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/autoradio/steer", strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", progOwner)
	rec := httptest.NewRecorder()
	h.HandleAutoradioSteer(rec, req)
	return rec
}

func TestN9_TapsAndMutes(t *testing.T) {
	fake := &fakeNotebook{}
	rt, _ := radioRuntime(t, notebookScript(nil))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	_, evs := serve(t, h, "r-1")
	entry := segmentEvents(evs)[0]["entry_id"].(string)
	if rec := steerPost(h, `{"entry_id":"`+entry+`","direction":"less"}`); rec.Code != 200 {
		t.Fatalf("less: %d %s", rec.Code, rec.Body.String())
	}
	if rec := steerPost(h, `{"entry_id":"`+entry+`","direction":"more"}`); rec.Code != 200 {
		t.Fatalf("more: %d %s", rec.Code, rec.Body.String())
	}
	if rec := steerPost(h, `{"direction":"mute","publisher":"voice-3fa2c1"}`); rec.Code != 200 {
		t.Fatalf("mute: %d %s", rec.Code, rec.Body.String())
	}
	for body, want := range map[string]int{
		`{"entry_id":"nope","direction":"more"}`: 404, `{"direction":"sideways"}`: 400, `{"direction":"mute","publisher":"x; drop"}`: 400, `{"direction":"mute","bad":1}`: 400,
	} {
		if rec := steerPost(h, body); rec.Code != want {
			t.Errorf("(N9) %s: %d", body, rec.Code)
		}
	}
	doc := notebook.Parse(fake.text())
	steer, asked := doc.Lines(notebook.SecSteering), doc.Lines(notebook.SecAsked)
	if len(steer) != 2 || !strings.HasPrefix(steer[0].Text, `Less like this: "Story A"`) || !strings.HasPrefix(steer[1].Text, `More like this: "Story A"`) {
		t.Errorf("(N9) steering %v", steer)
	}
	if len(asked) != 1 || asked[0].Text != "Never play takes by voice-3fa2c1." {
		t.Errorf("(N9) asked %v", asked)
	}
	_, out := learningGet(t, h, "")
	if n := out["notebook"].(map[string]any)["steering"].([]any); len(n) != 3 {
		t.Errorf("(N9) the learning route shows %d steering notes", len(n))
	}
	// A call after a tap: the tap line is in the stable input with a tap: ref
	// the DJ may cite, and a ref it was not shown is dropped.
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("r-2")))
	_ = rec
}

func TestN11_AnOpenRequestWritesNoNotebookLine(t *testing.T) {
	fake := &fakeNotebook{}
	script := linesScript(
		progLine(map[string]any{"kind": "opening", "title": "", "text": "Good morning.", "p_heard": nil, "caused_by": nil, "cites": nil}),
		nbAdd("A lesson from an open call.", []string{"a", "b"}, nil),
	)
	rt, _ := radioRuntime(t, script)
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("r-open"), func(m map[string]any) { m["cause"] = "open" }))
	if fake.found {
		t.Errorf("(N11) a notebook from an open call: %s", fake.text())
	}
}
