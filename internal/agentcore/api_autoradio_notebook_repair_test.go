package agentcore

// Review repairs of the notebook (seat 1, 2026-11-18). The failure list,
// written before the code:
//
//	B1 the model erases a lesson the evidence contradicts with unrelated cites,
//	   the contradiction is stored nowhere, or a replacement loses its lineage;
//	B2 the route shows only the counts of the forecasts the DJ chose to name a
//	   line on, with nothing beside them;
//	B3 a count-only patch is written on every extend, or a count is in the cached
//	   part of the prompt;
//	N5 a re-sent tap writes two lines;
//	N6 a listener's untagged bullets make a patch refuse, or the steering cap
//	   cuts one of their lines;
//	N8 an unbounded listener line or notebook fills the prompt;
//	N9 a listener's reworded line stays tagged as the DJ's lesson;
//	N4 a model-made document titled My notebook is adopted, or an archived one
//	   is written to.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/notebook"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var settledCausedRe = regexp.MustCompile(`(?m)^(\S+) \| \S+ \| .* \| p_heard \S+ \| caused_by (\S+) \|`)

// shownCausedBy lists the claims of the settled fence whose caused_by is ref.
func shownCausedBy(req provideriface.TextStreamRequest, ref string) (named, others []string) {
	for _, m := range settledCausedRe.FindAllStringSubmatch(req.User, -1) {
		if m[2] == ref {
			named = append(named, m[1])
		} else {
			others = append(others, m[1])
		}
	}
	return
}

func firstStretch(t *testing.T, h *APIHandler, id string, outcomes []string) {
	t.Helper()
	_, evs := serve(t, h, id)
	var entries []string
	for _, s := range segmentEvents(evs) {
		entries = append(entries, s["entry_id"].(string))
	}
	if len(entries) != len(outcomes) {
		t.Fatalf("%d entries, want %d", len(entries), len(outcomes))
	}
	settleStretch(t, h, entries, outcomes)
}

func sixStoriesLines() []string {
	var ls []string
	for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
		ls = append(ls, titled("Story "+n, nil))
	}
	return ls
}

func routeLines(t *testing.T, h *APIHandler) map[string]map[string]any {
	t.Helper()
	_, out := learningGet(t, h, "")
	res := map[string]map[string]any{}
	for _, l := range out["notebook"].(map[string]any)["lines"].([]any) {
		m := l.(map[string]any)
		if id, _ := m["id"].(string); id != "" {
			res[id] = m
		}
	}
	return res
}

func TestB1_ARetiredLessonKeepsItsContradiction(t *testing.T) {
	noCadence(t)
	var unrelated, named []string
	script := notebookScript(map[int]func(provideriface.TextStreamRequest, []string) []string{
		1: func(provideriface.TextStreamRequest, []string) []string { return sixStoriesLines() },
		2: func(req provideriface.TextStreamRequest, shown []string) []string {
			return []string{titled("Story G", nil), nbAdd("Short pieces are heard through.", shown[:2], nil)}
		},
		// Call 3: one forecast names L1, and the listener will skip it.
		3: func(provideriface.TextStreamRequest, []string) []string {
			return []string{titled("Story H", map[string]any{"caused_by": "lesson:L1"})}
		},
		// Call 4: retire L1 on evidence that does not name it: refused.
		4: func(req provideriface.TextStreamRequest, shown []string) []string {
			named, unrelated = shownCausedBy(req, "lesson:L1")
			if len(named) != 1 || len(unrelated) < 2 {
				t.Errorf("shown: %v %v", named, unrelated)
				return []string{titled("Story I", nil)}
			}
			return []string{titled("Story I", nil),
				jsonLine(map[string]any{"kind": "notebook", "op": "retire", "line": "L1", "cites": unrelated[:1]}),
				nbAdd("Reworded: short pieces win.", unrelated[:2], map[string]any{"replaces": []string{"L1"}})}
		},
		// Call 5: the same, citing the forecast that named L1: applied.
		5: func(req provideriface.TextStreamRequest, shown []string) []string {
			return []string{titled("Story J", nil),
				nbAdd("Reworded: short pieces win.", append([]string{named[0]}, unrelated[0]), map[string]any{"replaces": []string{"L1"}})}
		},
	})
	rt, _ := radioRuntime(t, script)
	h := NewAPIHandler(rt)
	fake := &fakeNotebook{}
	h.SetNotebook(fake)
	firstStretch(t, h, "r-1", []string{"finished", "finished", "finished", "finished", "skipped", "skipped"})
	serve(t, h, "r-2")
	_, evs := serve(t, h, "r-3")
	settleStretch(t, h, []string{segmentEvents(evs)[0]["entry_id"].(string)}, []string{"skipped"})

	_, evs = serve(t, h, "r-4")
	if !strings.Contains(fake.text(), "L1 · heard 0 of 1") || strings.Contains(fake.text(), "Reworded") {
		t.Fatalf("(B1) the contradicted lesson was erased on unrelated cites:\n%s", fake.text())
	}
	d, _ := doneNotebook(t, evs)["dropped"].([]any)
	if len(d) != 2 || !strings.Contains(fmt.Sprint(d), "names the line") {
		t.Errorf("(B1) dropped %v", d)
	}
	_, evs = serve(t, h, "r-5")
	text := fake.text()
	if strings.Contains(text, "L1 ·") || !strings.Contains(text, "- Reworded: short pieces win. [L2 · heard 0 of 0]") {
		t.Fatalf("(B1) the replacement:\n%s\n%v", text, doneNotebook(t, evs))
	}
	_, out := learningGet(t, h, "")
	nb := out["notebook"].(map[string]any)
	ret, _ := nb["retired"].([]any)
	if len(ret) != 1 {
		t.Fatalf("(B1) retired: %v", nb["retired"])
	}
	r := ret[0].(map[string]any)
	if r["id"] != "L1" || r["text"] != "Short pieces are heard through." || r["heard"] != float64(0) || r["of"] != float64(1) || r["replaced_by"] != "L2" || r["age_seconds"] == nil {
		t.Errorf("(B1) retired row: %v", r)
	}
	l2 := routeLines(t, h)["L2"]
	if l2["replaces_text"] != "replaces L1 (heard 0 of 1)" {
		t.Errorf("(B1) lineage: %v", l2)
	}
}

func TestB3_CountsRideOnlyWhenSomethingElseIsWrittenOrAnHourHasPassed(t *testing.T) {
	// A notebook with L1; every story names it; the listener hears them all.
	fake := &fakeNotebook{}
	fake.push("appagent", notebook.DeskLabel, "# My notebook\n\n## What you asked for\n\n## What Autoradio has learned\n- Short pieces are heard. [L1 · heard 0 of 0]\n\n## What Autoradio is trying\n\n## Recent steering\n")
	fake.found = true
	var lines []string
	for _, n := range []string{"A", "B", "C"} {
		lines = append(lines, titled("Story "+n, map[string]any{"caused_by": "lesson:L1"}))
	}
	rt, _ := radioRuntime(t, linesScript(lines...))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	firstStretch(t, h, "r-1", []string{"finished", "finished", "skipped"})
	serve(t, h, "r-2") // counts changed, an hour has not passed
	if len(fake.patches) != 0 {
		t.Fatalf("(B3) a count-only patch inside the hour: %+v", fake.patches)
	}
	if routeLines(t, h)["L1"]["live_heard"] != float64(2) || routeLines(t, h)["L1"]["live_of"] != float64(3) {
		t.Errorf("(B3) the route is live: %v", routeLines(t, h)["L1"])
	}
	fake.chain[0].At = time.Now().Add(-2 * time.Hour)
	serve(t, h, "r-3")
	if len(fake.patches) != 1 || !strings.Contains(fake.text(), "[L1 · heard 2 of 3]") {
		t.Errorf("(B3) the count-only patch after the hour: %d\n%s", len(fake.patches), fake.text())
	}
	serve(t, h, "r-4") // nothing changed (the r-2, r-3 stories are open)
	if len(fake.patches) != 1 {
		t.Errorf("(B3) a patch with no change: %d", len(fake.patches))
	}
}

func TestN5_ATapSentTwiceIsOneLine(t *testing.T) {
	fake := &fakeNotebook{}
	rt, _ := radioRuntime(t, notebookScript(nil))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	_, evs := serve(t, h, "r-1")
	entry := segmentEvents(evs)[0]["entry_id"].(string)
	body := `{"entry_id":"` + entry + `","direction":"less","id":"tap-1"}`
	for i := 0; i < 3; i++ {
		if rec := steerPost(h, body); rec.Code != 200 {
			t.Fatalf("send %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	// Without an id, the same entry and direction within ten minutes is the same tap.
	for i := 0; i < 2; i++ {
		steerPost(h, `{"entry_id":"`+entry+`","direction":"more"}`)
	}
	if n := len(notebook.Parse(fake.text()).Lines(notebook.SecSteering)); n != 2 {
		t.Errorf("(N5) %d steering lines:\n%s", n, fake.text())
	}
	if n := len(tape(t, rt, kindNote)); n != 2 {
		t.Errorf("(N5) %d notes", n)
	}
}

func TestN8_ThePromptCopyOfTheNotebookIsBounded(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("# My notebook\n\n## What you asked for\n- " + strings.Repeat("long ", 400) + "\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "- Line number %d %s\n", i, strings.Repeat("x", 300))
	}
	sb.WriteString("\n## What Autoradio has learned\n\n## What Autoradio is trying\n\n## Recent steering\n")
	fake := &fakeNotebook{}
	fake.push("appagent", notebook.DeskLabel, sb.String())
	fake.found = true
	var stable string
	rt, _ := radioRuntime(t, func(n int, ctx context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		stable = req.StableUser
		return linesScript(titled("Story A", nil))(n, ctx, req, onDelta)
	})
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	_, evs := serve(t, h, "r-1")
	if len(stable) > nbPromptMax+16<<10 || !strings.Contains(stable, "more lines of the notebook are not shown here") || !strings.Contains(stable, "[truncated]") {
		t.Errorf("(N8) the stable part holds %d bytes:\n%.400s", len(stable), stable)
	}
	start := getRecord(t, rt, stretchStartRecordPrefix+stretchIDOf(t, evs))
	if start.SubjectRefs["notebook_lines_total"] != "61" || start.SubjectRefs["notebook_lines_shown"] == "61" {
		t.Errorf("(N8) the receipt: %v", start.SubjectRefs)
	}
	_, out := learningGet(t, h, "")
	in := out["notebook"].(map[string]any)["inputs"].([]any)
	if len(in) != 1 || in[0].(map[string]any)["notebook_lines_total"] != "61" {
		t.Errorf("(N8) the route: %v", in)
	}
}

func TestN9_AListenersRewordingThatCannotMoveStaysTheirOwnUntagged(t *testing.T) {
	noCadence(t)
	fake := &fakeNotebook{}
	fake.push("appagent", notebook.DeskLabel, "# My notebook\n\n## What you asked for\n\n## What Autoradio has learned\n- Short pieces are heard. [L1 · heard 0 of 0]\n\n## What Autoradio is trying\n\n## Recent steering\n")
	fake.found = true
	fake.userEdit("# My notebook\n\n## What you asked for\n\n## What Autoradio has learned\n- I say [L9 odd [L1 · heard 0 of 0]\n\n## What Autoradio is trying\n\n## Recent steering\n")
	rt, _ := radioRuntime(t, notebookScript(nil))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	serve(t, h, "r-1")
	text := fake.text()
	if strings.Contains(text, "L1 ·") || !strings.Contains(text, "- I say [L9 odd\n") {
		t.Errorf("(N9)\n%s", text)
	}
	strikes := tape(t, rt, kindStrike)
	if len(strikes) != 1 || !strings.Contains(strikes[0].SubjectRefs["note"], "untagged") {
		t.Errorf("(N9) strike record: %+v", strikes)
	}
}

// a ledger that refuses lesson records
type noLessonLedger struct{ autoradioLedger }

func (l noLessonLedger) AppendCommitmentRecord(ctx context.Context, o, c string, rec types.CommitmentRecord) (string, error) {
	if rec.SubjectRefs["record_kind"] == kindLesson {
		return "", errors.New("tape down")
	}
	return l.autoradioLedger.AppendCommitmentRecord(ctx, o, c, rec)
}

func TestN10_ALessonWhoseRecordCannotBeWrittenIsNotApplied(t *testing.T) {
	noCadence(t)
	fake := &fakeNotebook{}
	rt, _ := radioRuntime(t, notebookScript(map[int]func(provideriface.TextStreamRequest, []string) []string{
		1: func(provideriface.TextStreamRequest, []string) []string { return sixStoriesLines() },
		2: func(req provideriface.TextStreamRequest, shown []string) []string {
			return []string{titled("Story G", nil), nbAdd("Short pieces are heard through.", shown[:2], nil)}
		},
	}))
	h := NewAPIHandler(rt)
	h.SetNotebook(fake)
	firstStretch(t, h, "r-1", []string{"finished", "finished", "finished", "finished", "skipped", "skipped"})
	h.autoradioLedgerOverride = noLessonLedger{rt.store}
	_, evs := serve(t, h, "r-2")
	if fake.found || !strings.Contains(fmt.Sprint(doneNotebook(t, evs)["error"]), "lesson record") {
		t.Errorf("(N10) applied without its record: found=%v %v", fake.found, doneNotebook(t, evs))
	}
}
