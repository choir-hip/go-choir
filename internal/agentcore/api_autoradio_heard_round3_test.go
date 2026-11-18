package agentcore

// Round 3 of the heard index (seat 2's follow-up). The failure list, written
// before the fixes:
//
//	R10 the heard settlement page holds only heard settlements: skipped and cut
//	    entries write false settlements, and they must not use up the page;
//	R11 the two lists are read at the same time;
//	R12 (calibration) an unparsable settled time is counted, an undecodable row
//	    does not fail the read (see calibration/autoradio_heard_test.go);
//	R13 another computer's segments and settlements never appear;
//	R14 with no usable tz the local_time offset names the day;
//	R15 a failed scorecard read gives 200 and a programme with no cases;
//	R16 the all-heard line says every story candidate and names the follow-up
//	    tag.

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestR10_FalseSettlementsDoNotUseUpTheHeardPage(t *testing.T) {
	prev := heardSettlementsRead
	heardSettlementsRead = 10
	defer func() { heardSettlementsRead = prev }()
	rt, fake := radioRuntime(t, oneStory())
	var cands []map[string]any
	// Heard and skipped alternate, oldest first: the newest settlements are
	// half false. The newest 10 heard are i = 10, 12, ... 28.
	for i := 0; i < 30; i++ {
		outcome := "heard"
		if i%2 == 1 {
			outcome = "skipped"
		}
		seedHeard(t, rt, heardSeed{stretch: fmt.Sprintf("st-%d", i), route: fmt.Sprintf("r-%d", i), version: 1, title: fmt.Sprintf("Story %d", i), text: "Said.", outcome: outcome, at: time.Now().Add(-26 * time.Hour)})
		time.Sleep(5 * time.Millisecond)
		cands = append(cands, map[string]any{"route": fmt.Sprintf("r-%d", i), "version": 1, "title": fmt.Sprintf("Cand %d", i), "dek": "d"})
	}
	buf := heardCaptureLog(t)
	in := djInput(t, rt, fake, func(m map[string]any) { m["candidates"] = cands })
	for i := 10; i < 30; i += 2 {
		if got := heardLine(in, fmt.Sprintf("pub:r-%d@v1", i)); !strings.HasPrefix(got, "heard: at this version") {
			t.Errorf("(R10) r-%d heard, inside the newest 10 heard: %q", i, got)
		}
	}
	for i := 1; i < 30; i += 2 {
		if got := heardLine(in, fmt.Sprintf("pub:r-%d@v1", i)); got != heardNoRecord {
			t.Errorf("(R10) r-%d skipped: %q", i, got)
		}
	}
	if !strings.Contains(buf.String(), "autoradio heard: read full") || !strings.Contains(buf.String(), "kind=settlement rows=10") {
		t.Errorf("(R10) the full settlement read is not logged: %q", heardLogLines(buf))
	}
}

func TestR11_TheTwoListsAreReadAtTheSameTime(t *testing.T) {
	rt, _ := radioRuntime(t, oneStory())
	// Each list waits until both have started: read one after the other, the
	// first never returns and the read fails.
	led := newBarrierLedger(rt.store, 2)
	c := &programmeCall{rt: rt, ledger: led, ownerID: progOwner, req: programmeRequest{RequestID: "r11", Cause: "open"}}
	if _, _, err := c.readHeard(testCtx()); err != nil {
		t.Fatalf("(R11) the two lists were not in flight together: %v", err)
	}
}

func TestR17_APanicInAReadingGoroutineIsHistoryUnavailable(t *testing.T) {
	for _, kind := range []string{kindSegment, "autoradio.settlement"} {
		rt, fake := radioRuntime(t, oneStory())
		seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
		buf := heardCaptureLog(t)
		h := NewAPIHandler(rt)
		h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, panicKind: kind}
		rec := httptest.NewRecorder()
		progServe(h, rec, progBody(withRequestID("r17-"+kind)))
		if rec.Code != 200 || len(segmentEvents(parseSSE(t, rec.Body.String()))) != 1 {
			t.Fatalf("(R17) %s: status %d: %s", kind, rec.Code, rec.Body.String())
		}
		in := lastInput(fake)
		if !strings.Contains(in, "History of what was heard is unavailable for this call") || strings.Contains(in, "\nheard: ") {
			t.Errorf("(R17) %s: no unavailable line, or a mark shown", kind)
		}
		if !strings.Contains(buf.String(), "autoradio heard: read panicked") {
			t.Errorf("(R17) %s: the panic is not logged: %q", kind, heardLogLines(buf))
		}
		// The line carries the stack, so the fault is findable, and never
		// the panic's own words (it is not a runtime error).
		if out := buf.String(); !strings.Contains(out, "stack=") || !strings.Contains(out, "recoverHeardRead") || strings.Contains(out, wrapLedgerPanicWords) {
			t.Errorf("(R17) %s: the panic line lacks the stack or carries the panic's words: %q", kind, heardLogLines(buf))
		}
	}
}

func TestR19_BothListsAreBoundedByTheWindowAndTheSegmentListLeavesTakesOut(t *testing.T) {
	rt, _ := radioRuntime(t, oneStory())
	led := &wrapLedger{autoradioLedger: rt.store}
	c := &programmeCall{rt: rt, ledger: led, ownerID: progOwner, req: programmeRequest{RequestID: "r19", Cause: "open"}}
	if _, _, err := c.readHeard(testCtx()); err != nil {
		t.Fatal(err)
	}
	if len(led.queries) != 2 {
		t.Fatalf("(R19) %d queries", len(led.queries))
	}
	for _, q := range led.queries {
		// The settlement list is bounded by the window; the segment list
		// reaches one window further back (a segment heard late).
		lo, hi := 13*24*time.Hour, 15*24*time.Hour
		if q.Match["$.subject_refs.record_kind"] == kindSegment {
			lo, hi = 27*24*time.Hour, 29*24*time.Hour
		}
		if age := time.Since(q.Since); q.Since.IsZero() || age < lo || age > hi {
			t.Errorf("(R19) %v: since %v is %v ago, want between %v and %v", q.Match, q.Since, age, lo, hi)
		}
		if q.Match["$.subject_refs.record_kind"] == kindSegment && (len(q.Absent) != 1 || q.Absent[0] != "$.subject_refs.take_id") {
			t.Errorf("(R19) the segment list does not leave takes out: %v", q.Absent)
		}
	}
}

func TestR13_AnotherComputersRecordsNeverAppear(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	own := rt.cfg.ComputerID
	rt.cfg.ComputerID = "other-computer"
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Their harbour", text: "OTHERCOMPUTER words.", at: time.Now().Add(-26 * time.Hour)})
	seedHeard(t, rt, heardSeed{stretch: "st-2", route: "quarry-9", version: 1, title: "Their quarry", text: "OTHERCOMPUTER quarry.", at: time.Now().Add(-26 * time.Hour)})
	rt.cfg.ComputerID = own
	in := djInput(t, rt, fake, nil)
	if got := heardLine(in, "pub:harbour-1@v1"); got != heardNoRecord || strings.Contains(in, "OTHERCOMPUTER") || len(recentLines(in)) != 0 {
		t.Errorf("(R13) line %q, rows %q", got, recentLines(in))
	}
}

func TestR14_TheLocalTimeOffsetNamesTheDayWhenNoZoneResolves(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	minus8 := time.FixedZone("", -8*3600)
	nowL := time.Now().In(minus8)
	at := time.Date(nowL.Year(), nowL.Month(), nowL.Day()-1, 20, 0, 0, 0, minus8)
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: at})
	want := "heard: at this version (v1) on " + at.In(minus8).Format("2006-01-02") + "."
	for name, mut := range map[string]func(map[string]any){
		"tz absent":  func(m map[string]any) { delete(m, "tz"); m["local_time"] = nowL.Format(time.RFC3339) },
		"tz invalid": func(m map[string]any) { m["tz"] = "Not/AZone"; m["local_time"] = nowL.Format(time.RFC3339) },
	} {
		in := djInput(t, rt, fake, mut)
		if got := heardLine(in, "pub:harbour-1@v1"); !strings.HasPrefix(got, want) || strings.Contains(got, at.UTC().Format("2006-01-02")) {
			t.Errorf("(R14) %s: %q, want prefix %q", name, got, want)
		}
	}
}

func TestR15_AFailedScorecardReadGivesA200AndNoCases(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	seedHeard(t, rt, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	h := NewAPIHandler(rt)
	h.SetNotebook(&fakeNotebook{})
	h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, failPaged: true}
	rec := httptest.NewRecorder()
	progServe(h, rec, progBody(withRequestID("r15")))
	if rec.Code != 200 {
		t.Fatalf("(R15) status %d: %s", rec.Code, rec.Body.String())
	}
	evs := parseSSE(t, rec.Body.String())
	if len(segmentEvents(evs)) != 1 || len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("(R15) not a normal programme: %+v", evs)
	}
	if in := lastInput(fake); strings.Contains(in, " settled>>>") || strings.Contains(in, " counts>>>") {
		t.Errorf("(R15) the notebook's cases or counts were shown after a failed read")
	}
	// And the same request with the read working shows the cases.
	rt2, fake2 := radioRuntime(t, oneStory())
	seedHeard(t, rt2, heardSeed{stretch: "st-1", route: "harbour-1", version: 1, title: "Harbour reopens", text: "The harbour reopened.", at: time.Now().Add(-26 * time.Hour)})
	h2 := NewAPIHandler(rt2)
	h2.SetNotebook(&fakeNotebook{})
	progServe(h2, httptest.NewRecorder(), progBody(withRequestID("r15b")))
	if in := lastInput(fake2); !strings.Contains(in, " settled>>>") {
		t.Errorf("(R15) control: the cases are missing with a working read")
	}
}

func TestR16_TheAllHeardLineSaysStoryCandidatesAndTheFollowupTag(t *testing.T) {
	rt, fake := radioRuntime(t, oneStory())
	seedAllHeard(t, rt)
	in := djInput(t, rt, fake, nil)
	if !strings.Contains(in, "Every story candidate was already heard at its current version.") || !strings.Contains(in, `"telling":"followup"`) {
		t.Errorf("(R16) all-heard line:\n%s", in)
	}
}

func testCtx() context.Context { return context.Background() }

// heardNoRecord is the mark of a candidate with no heard record in the heard
// window: what is known, not "never".
const heardNoRecord = "heard: not in the last 14 days"
