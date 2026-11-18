package agentcore

// Round 2 of the heard index (two review seats at 6e50c97bf). The failure
// list, written before the fixes:
//
//	R1  a failed history read never silences the radio: 200, a normal
//	    programme, the history-unavailable line, no heard: line, no recent rows;
//	R2  a day when every candidate was heard still produces content: the frame
//	    says so; one follow-up entry is placed; zero entries end as they do
//	    today (the spoken fallback, status failed) and the log counts the call;
//	R3  the open path reads two lists (segments, heard settlements) and no
//	    scorecard, and so does extend for the heard index;
//	R4  the day is the listener's: a story heard at 20:00 at UTC-8 is that
//	    local date, and with no zone the DJ is given no date at all;
//	R5  a cite whose role is mentioned is not a telling; quoted is;
//	R6  a later version heard than the candidate's prints the version heard;
//	R7  at the segment cap the newest are kept, the oldest drop, and the log
//	    says the read was full;
//	R8  (pin) a segment skipped and later finished is not heard: the forecast
//	    settles on the first outcome (a named residual);
//	R9  F41 scans the heard file, and the heard file reads no scorecard.

import (
	"bytes"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func heardCaptureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func heardLogLines(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "autoradio heard:") {
			out = append(out, l)
		}
	}
	return out
}

func lastInput(fake *fakeRadio) string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	r := fake.calls[len(fake.calls)-1].req
	return r.StableUser + "\n" + r.User
}

var datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

func TestR1_AFailedHistoryReadNeverSilencesTheRadio(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	buf := heardCaptureLog(t)
	h := NewAPIHandler(rt)
	h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, failListKind: kindSegment}
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("r1-req")))
	if rec.Code != 200 {
		t.Fatalf("(R1) status %d: %s", rec.Code, rec.Body.String())
	}
	evs := parseSSE(t, rec.Body.String())
	if len(segmentEvents(evs)) != 1 || len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("(R1) not a normal programme: %+v", evs)
	}
	in := lastInput(fake)
	if !strings.Contains(in, "History of what was heard is unavailable for this call") {
		t.Errorf("(R1) no unavailable line")
	}
	for _, l := range strings.Split(in, "\n") {
		if strings.HasPrefix(l, "heard: ") || strings.HasPrefix(l, "recent: ") {
			t.Errorf("(R1) a mark was shown with no history: %q", l)
		}
	}
	ls := heardLogLines(buf)
	if len(ls) < 1 || !strings.Contains(strings.Join(ls, "\n"), "unavailable") || !strings.Contains(strings.Join(ls, "\n"), "request=r1-req") {
		t.Errorf("(R1) log: %q", ls)
	}
}

func seedAllHeard(t *testing.T, rt *Runtime) {
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "harbour-2", version: 1, title: "Ferry timetable", text: "The ferry adds sailings.", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-3", route: "budget-3", version: 1, title: "Budget vote", text: "The council votes Monday.", at: at})
}

func TestR2_AllHeardStillProducesContent(t *testing.T) {
	// The frame says so.
	rt, fake := radioRuntime(t, oneStory())
	seedAllHeard(t, rt)
	in := djInput(t, rt, fake, nil)
	if !strings.Contains(in, "Every story candidate was already heard at its current version.") {
		t.Errorf("(R2) no all-heard line")
	}
	// A candidate not heard: no such line.
	rt2, fake2 := radioRuntime(t, oneStory())
	seedHeard(t, rt2, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	if in2 := djInput(t, rt2, fake2, nil); strings.Contains(in2, "Every story candidate was already heard") {
		t.Errorf("(R2) all-heard line with candidates never heard")
	}

	// One follow-up entry is placed, and the log counts the call.
	rt3, _ := radioRuntime(t, linesScript(titled("More on the harbour", map[string]any{"telling": "followup", "text": "You heard yesterday that the harbour reopened.\n\nNothing new is reported."})))
	seedAllHeard(t, rt3)
	buf := heardCaptureLog(t)
	_, evs := progRun(t, rt3, progBody())
	segs := segmentEvents(evs)
	done := eventsNamed(evs, "done")
	if len(segs) != 1 || segs[0]["entry_id"] == nil || len(done) != 1 || done[0].Data["placed"].(float64) != 1 {
		t.Fatalf("(R2) follow-up not placed: %+v", evs)
	}
	ls := heardLogLines(buf)
	if len(ls) != 1 || !strings.Contains(ls[0], "all_heard=1") || !strings.Contains(ls[0], "followups=1") {
		t.Errorf("(R2) log: %q", ls)
	}

	// Zero entries: today's ending. The listener gets the spoken fallback line
	// and an error event (status failed); nothing is placed.
	rt4, _ := radioRuntime(t, linesScript())
	seedAllHeard(t, rt4)
	buf4 := heardCaptureLog(t)
	_, evs4 := progRun(t, rt4, progBody())
	errs := eventsNamed(evs4, "error")
	if len(segmentEvents(evs4)) != 0 || len(errs) != 1 || errs[0].Data["spoken"] != programmeFallbackLine || errs[0].Data["status"] != "failed" {
		t.Fatalf("(R2) zero entries: %+v", evs4)
	}
	ls4 := heardLogLines(buf4)
	if len(ls4) != 1 || !strings.Contains(ls4[0], "all_heard=1") || !strings.Contains(ls4[0], "followups=0") {
		t.Errorf("(R2) zero-entry log: %q", ls4)
	}
}

func TestR3_TheHeardIndexReadsTwoListsAndNoScorecard(t *testing.T) {
	for _, cause := range []string{"open", "extend"} {
		rt, _ := radioRuntime(t, oneStory())
		seedAllHeard(t, rt)
		led := &wrapLedger{autoradioLedger: rt.store}
		h := NewAPIHandler(rt)
		h.autoradioLedgerOverride = led
		rec := httptest.NewRecorder()
		progServe(h, rec, progBody(withRequestID("r3-"+cause), func(m map[string]any) { m["cause"] = cause }))
		if led.listCalls[kindSegment] != 1 || led.listCalls["autoradio.settlement"] != 1 {
			t.Errorf("(R3) %s: lists %v, want one of segments and one of settlements", cause, led.listCalls)
		}
	}
}

func TestR4_TheDayIsTheListenersDay(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("no zoneinfo")
	}
	rt, fake := radioRuntime(t, oneStory())
	nowLA := time.Now().In(la)
	at := time.Date(nowLA.Year(), nowLA.Month(), nowLA.Day()-1, 20, 0, 0, 0, la)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: at})
	in := djInput(t, rt, fake, func(m map[string]any) {
		m["tz"] = "America/Los_Angeles"
		m["local_time"] = nowLA.Format(time.RFC3339)
	})
	want := "heard: at this version (v1) on " + at.In(la).Format("2006-01-02") + "."
	if got := heardLine(in, "pub:harbour-1@v1"); !strings.HasPrefix(got, want) || strings.Contains(got, at.UTC().Format("2006-01-02")) {
		t.Errorf("(R4) line %q, want prefix %q (UTC date %s)", got, want, at.UTC().Format("2006-01-02"))
	}

	// No zone: no date, only how many days ago.
	rt2, fake2 := radioRuntime(t, oneStory())
	seedHeard(t, rt2, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-72 * time.Hour)})
	seedHeard(t, rt2, heardSeed{stretch: "st-2", route: "quarry-9", version: 1, title: "Quarry fire", text: "A fire was put out.", at: time.Now().Add(-49 * time.Hour)})
	in2 := djInput(t, rt2, fake2, func(m map[string]any) { delete(m, "tz"); delete(m, "local_time") })
	got := heardLine(in2, "pub:harbour-1@v1")
	if !strings.HasPrefix(got, "heard: at this version (v1) 3 days ago.") {
		t.Errorf("(R4) no-zone line %q", got)
	}
	rows := recentLines(in2)
	if len(rows) != 1 || !strings.Contains(rows[0], "2 days ago") {
		t.Errorf("(R4) no-zone rows %q", rows)
	}
	for _, l := range append(rows, got) {
		if datePattern.MatchString(l) {
			t.Errorf("(R4) a date was given with no zone: %q", l)
		}
	}
}

func TestR5_OnlyAToldStoryIsHeard(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	at := time.Now().Add(-26 * time.Hour)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Mentioned only", text: "In passing, the harbour.", role: "mentioned", at: at})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "harbour-2", version: 1, title: "Quoted", text: "The ferry company said sailings rise.", role: "quoted", at: at})
	in := djInput(t, rt, fake, nil)
	if got := heardLine(in, "pub:harbour-1@v1"); got != "heard: never" {
		t.Errorf("(R5) mentioned: %q", got)
	}
	if got := heardLine(in, "pub:harbour-2@v1"); !strings.HasPrefix(got, "heard: at this version (v1)") {
		t.Errorf("(R5) quoted: %q", got)
	}
}

func TestR6_ALaterVersionHeardPrintsTheVersionHeard(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 3, title: "Harbour", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	in := djInput(t, rt, fake, func(m map[string]any) {
		c := progCandidates()
		c[0]["version"] = 2
		m["candidates"] = c
	})
	if got := heardLine(in, "pub:harbour-1@v2"); !strings.HasPrefix(got, "heard: at this version (v3) on ") {
		t.Errorf("(R6) %q", got)
	}
}

func TestR7_TheSegmentCapKeepsTheNewestAndSaysSo(t *testing.T) {
	prev := heardSegmentsRead
	heardSegmentsRead = 5
	defer func() { heardSegmentsRead = prev }()
	rt, fake := radioRuntime(t, oneStory())
	var cands []map[string]any
	for i := 0; i < 8; i++ {
		seedHeard(t, rt, heardSeed{stretch: fmt.Sprintf("st-%d", i), route: fmt.Sprintf("r-%d", i), version: 1, title: fmt.Sprintf("Story %d", i), text: "Said.", at: time.Now().Add(-26 * time.Hour)})
		time.Sleep(5 * time.Millisecond)
		cands = append(cands, map[string]any{"route": fmt.Sprintf("r-%d", i), "version": 1, "title": fmt.Sprintf("Cand %d", i), "dek": "d"})
	}
	buf := heardCaptureLog(t)
	in := djInput(t, rt, fake, func(m map[string]any) { m["candidates"] = cands })
	for i := 0; i < 8; i++ {
		got := heardLine(in, fmt.Sprintf("pub:r-%d@v1", i))
		if i >= 3 && !strings.HasPrefix(got, "heard: at this version") {
			t.Errorf("(R7) r-%d (among the newest 5): %q", i, got)
		}
		if i < 3 && got != "heard: never" {
			t.Errorf("(R7) r-%d (older than the cap): %q", i, got)
		}
	}
	all := buf.String()
	if !strings.Contains(all, "autoradio heard: read full") || !strings.Contains(all, "rows=5") || !strings.Contains(all, "limit=5") {
		t.Errorf("(R7) the full read is not logged: %q", heardLogLines(buf))
	}
}

func TestR8_PinASkippedSegmentLaterFinishedIsStillNotHeard(t *testing.T) {
	rt, fake := radioRuntime(t, linesScript(titled("Harbour told", map[string]any{"cites": []any{"pub:harbour-1@v1"}})))
	h := NewAPIHandler(rt)
	_, entries, _, _ := runStretch(t, rt)
	mustOps(t, h, phoneEnd(entries[0], "a", "skipped", "skipped", 0)...)
	postOps(t, h, phoneEnd(entries[0], "b", "heard", "finished", 1)...)
	fake.script = oneStory()
	in := djInput(t, rt, fake, nil)
	// The forecast settles on its first outcome; a replay to the end is not
	// read back. This pins today's behaviour so the gap is visible: it is a
	// residual in docs/reference/AUTORADIO-PROGRAMME.md.
	if got := heardLine(in, "pub:harbour-1@v1"); got != "heard: never" {
		t.Errorf("(R8) %q", got)
	}
}

func TestR9_F41ScansTheHeardFileAndItReadsNoScorecard(t *testing.T) {
	files, _ := filepath.Glob("api_autoradio_programme*.go")
	found := false
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if strings.Contains(f, "heard") {
			found = true
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "Scorecard(") {
				t.Errorf("(R9) %s reads the scorecard", f)
			}
		}
	}
	if !found {
		t.Errorf("(R9) the heard file is not covered by TestF41's glob: %v", files)
	}
}
