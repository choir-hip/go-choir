package agentcore

// The heard index: the DJ is told what the listener heard. The failure list,
// written before the code (problem record
// docs/problems/autoradio-repeats-what-the-listener-already-heard-2026-11-18.md):
//
//	H1  a story the phone settled heard yesterday, at this version, is marked
//	    heard at this version, with its day and what was said, in the DJ's input;
//	H2  the same route at a higher version is marked heard at the earlier version;
//	H3  a heard story that is not a candidate is in the recent list, with day
//	    and title (and a candidate that was heard is not);
//	H4  a skipped or cut segment is not heard;
//	H5  a segment with no settlement, or a left or failed no-verdict record, is
//	    not heard;
//	H6  only a settlement the phone's outcome wrote counts: a segment record
//	    alone, a settlement by another method and a DJ line that says it was
//	    told change nothing;
//	H7  a request from a second device gets the same marks (the index is the
//	    computer's records);
//	H8  an entry the DJ marks a follow-up has the telling:followup tag on its
//	    claim; a missing or unknown value records full;
//	H9  heard text older than the window is not in the input, and the candidate
//	    is still marked heard (the key outlives the text);
//	H10 with 500 heard segments the recent list is its bound and every row and
//	    matched text is cut;
//	H11 another listener's records never appear;
//	H12 one log line per call carries counts and no words.
//
// The model is the fake of the programme tests; the assertions are on the
// input it was given and on the records written, never on its prose.

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type heardSeed struct {
	owner, stretch string
	route          string
	version        int
	title, text    string
	// outcome: heard, skipped (settled by the phone's rule), left or failed (a
	// no-verdict record), none (no settlement) or self (a settlement by
	// another method).
	outcome string
	// at is when the phone settled it; the stretch began ten minutes earlier.
	at   time.Time
	kind string
	// role is the cite's role; empty is subject.
	role string
}

// seedHeard writes, on a listener's computer, what a programme call and the
// phone's op path leave behind for one segment: the segment record, its
// forecast and the settlement or no-verdict record. It returns the claim id.
func seedHeard(t *testing.T, rt *Runtime, s heardSeed) string {
	t.Helper()
	if s.owner == "" {
		s.owner = progOwner
	}
	if s.kind == "" {
		s.kind = "story"
	}
	if s.outcome == "" {
		s.outcome = "heard"
	}
	ctx := context.Background()
	start := s.at.Add(-10 * time.Minute)
	c := &programmeCall{rt: rt, ledger: rt.store, ownerID: s.owner, stretchID: s.stretch, startedAt: start}
	seg := &programmeSegment{
		ID: s.stretch + ".s0", Kind: s.kind, Title: s.title, Text: s.text, Content: true, Blocks: len(calibration.SplitBlocks(s.text)),
		Cites: []programmeCite{{Ref: fmt.Sprintf("pub:%s@v%d", s.route, s.version), Role: cmpRole(s.role), Label: s.title}},
		P:     0.7, Cause: "importance", CausedBy: "importance",
	}
	seg.SHA256 = sha256Hex(seg.Text)
	if err := c.writeSegment(ctx, seg); err != nil {
		t.Fatal(err)
	}
	claim, err := calibration.NewAutoradioForecast(c.forecastInput(seg), start)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.writeClaim(ctx, claim); err != nil {
		t.Fatal(err)
	}
	computer := rt.cfg.ComputerID
	switch s.outcome {
	case "heard", "skipped":
		if _, err := calibration.SettleAutoradio(ctx, rt.store, s.owner, computer, claim.RecordID, calibration.AutoradioHeardRule, s.outcome == "heard", "op-"+s.stretch, s.at); err != nil {
			t.Fatal(err)
		}
	case "left", "failed":
		nv := ownerRecord(kindNoVerdict, "autoradio-noverdict:"+s.stretch, s.owner, computer,
			map[string]string{"claim_id": claim.RecordID, "reason": s.outcome, "op_id": "op-" + s.stretch}, "no verdict", "{}", s.at)
		if _, err := rt.store.AppendCommitmentRecord(ctx, s.owner, computer, nv); err != nil {
			t.Fatal(err)
		}
	case "self":
		rec, err := calibration.NewSettlementRecord(claim, calibration.SettlementRequest{
			Outcome: "true", Evidence: []string{"air:op-" + s.stretch}, Scorer: calibration.ScorerIdentity{Method: types.CalibrationScorerSelf, Name: "autoradio-dj"},
		}, s.at)
		if err != nil {
			t.Fatal(err)
		}
		rec.ComputerID = computer
		if _, err := rt.store.AppendCommitmentRecord(ctx, s.owner, computer, rec); err != nil {
			t.Fatal(err)
		}
	case "none":
	default:
		t.Fatalf("outcome %q", s.outcome)
	}
	return claim.RecordID
}

var heardReq int

// djInput posts a programme request and returns what the DJ was given (the
// stable and the volatile part of the user message).
func djInput(t *testing.T, rt *Runtime, fake *fakeRadio, mutate func(map[string]any), reqMut ...func(*http.Request)) string {
	t.Helper()
	heardReq++
	n := fake.callCount()
	progRun(t, rt, progBody(withRequestID(fmt.Sprintf("heard-%d", heardReq)), func(m map[string]any) {
		if mutate != nil {
			mutate(m)
		}
	}), reqMut...)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) <= n {
		t.Fatalf("the model was not called")
	}
	r := fake.calls[len(fake.calls)-1].req
	return r.StableUser + "\n" + r.User
}

// heardLine is the heard: line under the candidate with the given ref.
func heardLine(in, ref string) string {
	lines := strings.Split(in, "\n")
	for i, l := range lines {
		if l == "ref: "+ref {
			for _, m := range lines[i+1:] {
				if strings.HasPrefix(m, "heard: ") {
					return m
				}
				if strings.HasPrefix(m, "ref: ") {
					break
				}
			}
		}
	}
	return ""
}

func recentLines(in string) []string {
	var out []string
	for _, l := range strings.Split(in, "\n") {
		if strings.HasPrefix(l, "recent: ") {
			out = append(out, l)
		}
	}
	return out
}

func cmpRole(r string) string {
	if r == "" {
		return "subject"
	}
	return r
}

// dayOf is the listener's day: the default request's zone is Europe/Paris.
func dayOf(at time.Time) string {
	loc, _ := time.LoadLocation("Europe/Paris")
	return at.In(loc).Format("2006-01-02")
}

func oneStory() radioScript { return linesScript(titled("Only", nil)) }

func TestH1_AStoryHeardYesterdayIsMarkedHeardAtThisVersionWithWhatWasSaid(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	yesterday := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened after repairs. Ferries run again.", at: yesterday})
	in := djInput(t, rt, fake, nil)
	want := "heard: at this version (v1) on " + dayOf(yesterday) + ". Said then: The harbour reopened after repairs. Ferries run again."
	if got := heardLine(in, "pub:harbour-1@v1"); got != want {
		t.Errorf("(H1) harbour-1 line %q, want %q", got, want)
	}
	for _, ref := range []string{"pub:harbour-2@v1", "pub:budget-3@v1"} {
		if got := heardLine(in, ref); got != heardNoRecord {
			t.Errorf("(H1) %s line %q, want never", ref, got)
		}
	}
}

func TestH2_ALaterVersionIsMarkedHeardAtTheEarlierVersion(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	yesterday := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened after repairs.", at: yesterday})
	in := djInput(t, rt, fake, func(m map[string]any) {
		c := progCandidates()
		c[0]["version"] = 2
		m["candidates"] = c
	})
	want := "heard: at an earlier version (v1, now v2) on " + dayOf(yesterday) + ". Said then: The harbour reopened after repairs."
	if got := heardLine(in, "pub:harbour-1@v2"); got != want {
		t.Errorf("(H2) line %q, want %q", got, want)
	}
}

func TestH3_AHeardStoryThatIsNotACandidateIsInTheRecentList(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	two := time.Now().Add(-50 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "quarry-9", version: 1, title: "Quarry fire", text: "A fire at the quarry was put out. No one was hurt.", at: two})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	in := djInput(t, rt, fake, nil)
	rows := recentLines(in)
	if len(rows) != 1 || !strings.Contains(rows[0], dayOf(two)) || !strings.Contains(rows[0], "Quarry fire") || !strings.Contains(rows[0], "A fire at the quarry was put out.") {
		t.Errorf("(H3) recent list: %q", rows)
	}
	if strings.Contains(strings.Join(rows, "\n"), "Harbour reopens") {
		t.Errorf("(H3) a candidate is also listed as recent: %q", rows)
	}
	if got := heardLine(in, "pub:harbour-1@v1"); !strings.HasPrefix(got, "heard: at this version (v1)") {
		t.Errorf("(H3) candidate line %q", got)
	}
}

func TestH4toH5_OnlyAFinishedSegmentIsHeard(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "Skipped text.", outcome: "skipped", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "harbour-2", version: 1, title: "Ferry timetable", text: "Left text.", outcome: "left", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-3", route: "budget-3", version: 1, title: "Budget vote", text: "Failed text.", outcome: "failed", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-4", route: "quarry-9", version: 1, title: "Quarry fire", text: "Unsettled text.", outcome: "none", at: at})
	in := djInput(t, rt, fake, nil)
	for _, ref := range []string{"pub:harbour-1@v1", "pub:harbour-2@v1", "pub:budget-3@v1"} {
		if got := heardLine(in, ref); got != heardNoRecord {
			t.Errorf("(H4/H5) %s line %q, want never", ref, got)
		}
	}
	if rows := recentLines(in); len(rows) != 0 {
		t.Errorf("(H4/H5) recent list: %q", rows)
	}
	for _, w := range []string{"Skipped text", "Left text", "Failed text", "Unsettled text"} {
		if strings.Contains(in, w) {
			t.Errorf("(H4/H5) %q reached the input", w)
		}
	}
}

func TestH6_OnlyThePhonesSettlementCounts(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	at := time.Now().Add(-26 * time.Hour)
	// A segment and its forecast with no settlement; a settlement by another
	// method (the DJ's own, say).
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "Alone text.", outcome: "none", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "harbour-2", version: 1, title: "Ferry timetable", text: "Self text.", outcome: "self", at: at})
	// A DJ line that says it was told: written by the call, settled by nothing.
	fake.script = linesScript(titled("Told", map[string]any{
		"text": "Told yesterday, you heard this already.\n\nHeard in full.", "heard": true, "settled": "true", "cites": []any{"pub:budget-3@v1"},
	}))
	djInput(t, rt, fake, nil)
	in := djInput(t, rt, fake, nil)
	for _, ref := range []string{"pub:harbour-1@v1", "pub:harbour-2@v1", "pub:budget-3@v1"} {
		if got := heardLine(in, ref); got != heardNoRecord {
			t.Errorf("(H6) %s line %q, want never", ref, got)
		}
	}
	if rows := recentLines(in); len(rows) != 0 {
		t.Errorf("(H6) recent list: %q", rows)
	}
}

func TestH7_ASecondDeviceGetsTheSameMarks(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: at})
	a := djInput(t, rt, fake, nil, func(r *http.Request) { r.Header.Set("X-Device-Id", "device-one") })
	b := djInput(t, rt, fake, nil, func(r *http.Request) { r.Header.Set("X-Device-Id", "device-two") })
	la, lb := heardLine(a, "pub:harbour-1@v1"), heardLine(b, "pub:harbour-1@v1")
	if !strings.HasPrefix(la, "heard: at this version (v1) on "+dayOf(at)) || la != lb {
		t.Errorf("(H7) device one %q, device two %q", la, lb)
	}
}

func TestH8_TheTellingIsATagOnTheClaim(t *testing.T) {
	rt, _ := radioRuntime(t, linesScript(
		titled("One", map[string]any{"telling": "followup"}),
		titled("Two", map[string]any{"kind": "update", "cites": []any{"pub:harbour-2@v1"}}),
		titled("Three", map[string]any{"cites": []any{"pub:budget-3@v1"}, "telling": "banana"}),
	))
	_, evs := progRun(t, rt, progBody())
	sid := stretchIDOf(t, evs)
	got := map[string]string{}
	for _, c := range stretchClaims(t, rt, sid) {
		got[tagValue(c, "seg:")] = tagValue(c, "telling:")
	}
	want := map[string]string{sid + ".s0": "followup", sid + ".s1": "full", sid + ".s2": "full"}
	if len(got) != 3 {
		t.Fatalf("(H8) %d claims: %v", len(got), got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("(H8) %s telling %q, want %q (all: %v)", k, got[k], w, got)
		}
	}
}

func TestH9_TheKeyOutlivesTheText(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	old := time.Now().Add(-20 * 24 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Old harbour", text: "ZEBRA-OLD words from long ago.", at: old})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "quarry-9", version: 1, title: "Old quarry", text: "ZEBRA-QUARRY words from long ago.", at: old})
	in := djInput(t, rt, fake, nil)
	got := heardLine(in, "pub:harbour-1@v1")
	if !strings.HasPrefix(got, "heard: at this version (v1) on "+dayOf(old)) || strings.Contains(in, "ZEBRA") {
		t.Errorf("(H9) line %q; old words in the input: %v", got, strings.Contains(in, "ZEBRA"))
	}
	if rows := recentLines(in); len(rows) != 0 {
		t.Errorf("(H9) recent list holds a story older than the window: %q", rows)
	}
}

func TestH10_FiveHundredHeardSegmentsStayBounded(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	long := strings.Repeat("The council met and talked about the long budget. ", 80)
	now := time.Now()
	seedHeard(t, rt, heardSeed{stretch: "st-h1", route: "harbour-1", version: 1, title: "Harbour reopens", text: long, at: now.Add(-time.Hour)})
	for i := 0; i < 500; i++ {
		seedHeard(t, rt, heardSeed{stretch: fmt.Sprintf("st-%d", i), route: fmt.Sprintf("r-%d", i), version: 1, title: fmt.Sprintf("Story %d", i), text: long, at: now.Add(-time.Duration(i+2) * 30 * time.Minute)})
	}
	in := djInput(t, rt, fake, nil)
	rows := recentLines(in)
	if len(rows) != 12 { // the stated bound: 12 stories
		t.Fatalf("(H10) %d recent rows, want 12", len(rows))
	}
	for i, r := range rows {
		if len(r) > 400 {
			t.Errorf("(H10) row %d is %d bytes", i, len(r))
		}
	}
	if !strings.Contains(rows[0], "Story 0") || !strings.Contains(rows[11], "Story 11") {
		t.Errorf("(H10) not newest first: %q ... %q", rows[0], rows[11])
	}
	m := heardLine(in, "pub:harbour-1@v1")
	if len(m) > 700 || !strings.HasSuffix(m, ".") || !strings.HasPrefix(m, "heard: at this version (v1)") {
		t.Errorf("(H10) matched line is %d bytes: %q", len(m), m)
	}
	if len(in) > 30000 {
		t.Errorf("(H10) input is %d bytes", len(in))
	}
}

func TestH11_AnotherListenersRecordsNeverAppear(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{owner: "owner-2", stretch: "st-1", route: "harbour-1", version: 1, title: "Their harbour", text: "THEIRS words.", at: at})
	seedHeard(t, rt, heardSeed{owner: "owner-2", stretch: "st-2", route: "quarry-9", version: 1, title: "Their quarry", text: "THEIRS quarry words.", at: at})
	in := djInput(t, rt, fake, nil)
	if got := heardLine(in, "pub:harbour-1@v1"); got != heardNoRecord {
		t.Errorf("(H11) line %q", got)
	}
	if strings.Contains(in, "THEIRS") || strings.Contains(in, "Their ") || len(recentLines(in)) != 0 {
		t.Errorf("(H11) another listener's words reached the input")
	}
}

func TestH12_OneLogLineWithCountsAndNoWords(t *testing.T) {
	rt, fake := radioRuntime(t, linesScript(titled("Secret title", map[string]any{"telling": "followup"})))
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "Hidden words here.", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "budget-3", version: 1, title: "Old budget", text: "Older hidden words.", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-3", route: "quarry-9", version: 1, title: "Quarry fire", text: "Quarry hidden words.", at: at})
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	djInput(t, rt, fake, func(m map[string]any) {
		c := progCandidates()
		c[2]["version"] = 2
		m["candidates"] = c
	})
	var lines []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "autoradio heard:") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("(H12) %d heard log lines in %q", len(lines), buf.String())
	}
	for _, w := range []string{"candidates=3", "never=1", "same=1", "earlier=1", "recent=1", "followups=1"} {
		if !strings.Contains(lines[0], w) {
			t.Errorf("(H12) %q missing from %q", w, lines[0])
		}
	}
	for _, w := range []string{"Hidden", "Harbour", "Quarry", "budget", "Secret"} {
		if strings.Contains(lines[0], w) {
			t.Errorf("(H12) a word %q in the log line", w)
		}
	}
}

// H1b: the real path. A stretch is written by the programme route, the phone
// posts its ops, and the next programme call is told what was finished and
// not what was skipped or left.
func TestH1b_TheOpPathMakesAStoryHeard(t *testing.T) {
	rt, fake := radioRuntime(t, linesScript(
		titled("Harbour told", map[string]any{"text": "The harbour reopened today.\n\nFerries run again.", "cites": []any{"pub:harbour-1@v1"}}),
		titled("Ferry skipped", map[string]any{"cites": []any{"pub:harbour-2@v1"}}),
		titled("Budget left", map[string]any{"cites": []any{"pub:budget-3@v1"}}),
	))
	h := NewAPIHandler(rt)
	_, entries, _, _ := runStretch(t, rt)
	if len(entries) != 3 {
		t.Fatalf("entries %v", entries)
	}
	var ops []map[string]any
	ops = append(ops, phoneEnd(entries[0], "a", "heard", "finished", 1)...)
	ops = append(ops, phoneEnd(entries[1], "b", "skipped", "skipped", 0)...)
	ops = append(ops, phoneEnd(entries[2], "c", "heard", "left", 0)...)
	for _, r := range mustOps(t, h, ops...).Results {
		if !r.Applied {
			t.Fatalf("op dropped: %+v", r)
		}
	}
	fake.script = oneStory()
	in := djInput(t, rt, fake, nil)
	today := dayOf(time.Now())
	if got := heardLine(in, "pub:harbour-1@v1"); !strings.HasPrefix(got, "heard: at this version (v1) on "+today+". Said then: The harbour reopened today.") {
		t.Errorf("(H1b) finished: %q", got)
	}
	for _, ref := range []string{"pub:harbour-2@v1", "pub:budget-3@v1"} {
		if got := heardLine(in, ref); got != heardNoRecord {
			t.Errorf("(H1b) %s: %q", ref, got)
		}
	}
}
