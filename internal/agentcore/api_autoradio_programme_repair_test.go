package agentcore

// Station L1, review round 1: the repairs. Each test was written and seen to
// fail before its fix.

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func identLine() string {
	return progLine(map[string]any{"kind": "ident", "title": "On air", "text": "This is the radio.", "p_heard": nil, "caused_by": nil, "cites": nil})
}

// F45 (B1): a replay streams only the leading run of segments that have what
// they should: a content segment needs its claim.
func TestF45_AReplayNeverStreamsASegmentWhoseClaimWasNotWritten(t *testing.T) {
	// The claim write fails for the second claim.
	rt, _ := radioRuntime(t, linesScript(identLine(), titled("First", nil), titled("Second", nil)))
	h := NewAPIHandler(rt)
	h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, failClaimFrom: 2}
	w := httptest.NewRecorder()
	progServe(h, w, progBody(withRequestID("f45")))
	live := parseSSE(t, w.Body.String())
	if strings.Join(segmentTitles(live), "|") != "On air|First" {
		t.Fatalf("live stream %q", segmentTitles(live))
	}
	_, replay := progRun(t, rt, progBody(withRequestID("f45")))
	if got := strings.Join(segmentTitles(replay), "|"); got != "On air|First" {
		t.Fatalf("(F45) replay streamed %q: a segment without its claim, or too little", got)
	}
	if len(eventsNamed(replay, "error")) != 1 || len(eventsNamed(replay, "done")) != 0 {
		t.Fatalf("(F45) replay events %+v", replay)
	}
	if n := len(stretchClaims(t, rt, stretchIDOf(t, replay))); n != 1 {
		t.Fatalf("claims %d", n)
	}
}

func TestF45_APanicBetweenTheSegmentRecordAndItsClaimThenReplay(t *testing.T) {
	rt, _ := radioRuntime(t, linesScript(identLine(), titled("First", nil), titled("Second", nil)))
	h := NewAPIHandler(rt)
	h.autoradioLedgerOverride = &wrapLedger{autoradioLedger: rt.store, panicClaimAppend: true}
	w := httptest.NewRecorder()
	progServe(h, w, progBody(withRequestID("f45p")))
	sid := stretchIDOf(t, parseSSE(t, w.Body.String()))
	if getRecord(t, rt, "autoradio-seg:"+sid+"/"+sid+".s1") == nil {
		t.Fatal("the segment record of s1 is not on the tape: the panic did not land between the two writes")
	}
	_, replay := progRun(t, rt, progBody(withRequestID("f45p")))
	if got := strings.Join(segmentTitles(replay), "|"); got != "On air" {
		t.Fatalf("(F45) replay streamed %q", got)
	}
}

// F46 (B2): a wall-clock deadline for the walk and an idle deadline between
// deltas, both through the failed path; the request_id is not stuck.
func TestF46_TheWallClockAndIdleDeadlines(t *testing.T) {
	stall := func(lines ...string) radioScript {
		return func(n int, ctx context.Context, req provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
			for _, l := range lines {
				onDelta(l + "\n")
			}
			<-ctx.Done()
			return nil, ctx.Err()
		}
	}
	for name, c := range map[string]struct{ wall, idle time.Duration }{
		"wall clock": {300 * time.Millisecond, time.Hour},
		"idle":       {time.Hour, 300 * time.Millisecond},
	} {
		oldW, oldI := programmeWallDeadline, programmeIdleDeadline
		programmeWallDeadline, programmeIdleDeadline = c.wall, c.idle
		rt, fake := radioRuntime(t, stall(titled("First", nil)))
		done := make(chan struct{})
		var evs []sseEvent
		go func() {
			_, evs = progRun(t, rt, progBody(withRequestID("stall")))
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			programmeWallDeadline, programmeIdleDeadline = oldW, oldI
			t.Fatalf("(F46) %s: the call did not end", name)
		}
		programmeWallDeadline, programmeIdleDeadline = oldW, oldI
		sid := stretchIDOf(t, evs)
		if len(eventsNamed(evs, "error")) != 1 || len(eventsNamed(evs, "done")) != 0 || len(segmentEvents(evs)) != 1 {
			t.Fatalf("(F46) %s: events %+v", name, evs)
		}
		idx := getRecord(t, rt, "autoradio-stretch:"+sid)
		if idx == nil || idx.SubjectRefs["status"] != "deadline" {
			t.Fatalf("(F46) %s: index %+v", name, idx)
		}
		if fake.callCount() != 1 {
			t.Fatalf("(F46) %s: walked on after a deadline: %d calls", name, fake.callCount())
		}
		// Not 409 forever: the same request_id answers from the record.
		rec, replay := progRun(t, rt, progBody(withRequestID("stall")))
		if rec.Code != http.StatusOK || len(segmentEvents(replay)) != 1 {
			t.Fatalf("(F46) %s: the request_id is stuck: %d", name, rec.Code)
		}
	}
}

func TestF47_ASegmentCapEndsTheStretch(t *testing.T) {
	var lines []string
	for i := 0; i < 35; i++ {
		lines = append(lines, titled(fmt.Sprintf("S%d", i), nil))
	}
	rt, _ := radioRuntime(t, linesScript(lines...))
	_, evs := progRun(t, rt, progBody())
	sid := stretchIDOf(t, evs)
	if n := len(segmentEvents(evs)); n != 30 {
		t.Fatalf("(F47) %d segments", n)
	}
	if len(eventsNamed(evs, "done")) != 1 {
		t.Fatalf("(F47) events %+v", evs)
	}
	idx := getRecord(t, rt, "autoradio-stretch:"+sid)
	if idx == nil || !strings.Contains(idx.SubjectRefs["dropped"], "cap") || len(stretchClaims(t, rt, sid)) != 30 {
		t.Fatalf("(F47) index %+v", idx)
	}
}

// The replay reason says what is known (S2).
func TestF48_TheReplayReasonIsWhatIsKnown(t *testing.T) {
	rt, _ := radioRuntime(t, func(_ int, _ context.Context, _ provideriface.TextStreamRequest, onDelta func(string)) (*provideriface.TextStreamResponse, error) {
		onDelta(titled("One", nil) + "\n")
		panic("crash")
	})
	progRun(t, rt, progBody(withRequestID("rr")))
	_, replay := progRun(t, rt, progBody(withRequestID("rr")))
	r := eventsNamed(replay, "error")[0].Data["reason"].(string)
	if !strings.Contains(r, "without an index") || strings.Contains(r, "restart") {
		t.Fatalf("(F48) reason %q", r)
	}
}

// S3: only what L1 can legitimately produce.
func TestF49_KindsL1CannotProduceAreDroppedByName(t *testing.T) {
	rt, _ := radioRuntime(t, linesScript())
	for _, kind := range []string{"mail", "podcast_note", "answer", "result"} {
		line := progLine(map[string]any{"kind": kind, "title": "x"})
		rt.provider.(*fakeRadio).script = linesScript(line, titled("ok", nil))
		_, evs := progRun(t, rt, progBody(withRequestID("k-"+kind)))
		if strings.Join(segmentTitles(evs), "|") != "ok" {
			t.Errorf("(F49) %s streamed: %q", kind, segmentTitles(evs))
		}
		idx := getRecord(t, rt, "autoradio-stretch:"+stretchIDOf(t, evs))
		if !strings.Contains(idx.SubjectRefs["dropped"], kind) || !strings.Contains(idx.SubjectRefs["dropped"], "not produced") {
			t.Errorf("(F49) %s: reason %q", kind, idx.SubjectRefs["dropped"])
		}
	}
}

// S4: each instruction and note in the prompt is bounded.
func TestF50_InstructionAndNoteTextIsBounded(t *testing.T) {
	rt, fake := radioRuntime(t, twoStoryScript())
	fake.script = okAnswer("Fine.")
	words := strings.Repeat("é", 900) // 1800 bytes
	body := `{"turn_id":"turn-big","words":"` + words + `","page":{"text":"p"}}`
	if rec := radioPost(rt, body); rec.Code != http.StatusOK {
		t.Fatalf("seed %d", rec.Code)
	}
	label, _ := types.AutoradioKindLabel("autoradio.note")
	note := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "autoradio-note:big", Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: "n", Body: strings.Repeat("n", 5000)},
		Provenance:  types.CommitmentProvenance{AgentID: progOwner, CommittedAt: "2026-11-14T10:00:00Z", Labels: types.NewLabelSet(label).Strings()},
		EmitterKind: types.CommitmentEmitterOwner, ComputerID: rt.cfg.ComputerID, SubjectRefs: map[string]string{"desk": autoradioDesk, "record_kind": "autoradio.note"},
	}
	if _, err := rt.store.AppendCommitmentRecord(context.Background(), progOwner, rt.cfg.ComputerID, note); err != nil {
		t.Fatal(err)
	}
	fake.script = twoStoryScript()
	progRun(t, rt, progBody())
	in := fake.calls[len(fake.calls)-1].req.StableUser
	seen := 0
	for _, line := range strings.Split(in, "\n") {
		if strings.HasPrefix(line, "instruction:") {
			seen++
			_, text, _ := strings.Cut(line, ": ")
			if !utf8.ValidString(text) || len(text) > 1000+len(" [truncated]") || !strings.HasSuffix(text, "[truncated]") {
				t.Errorf("(F50) instruction line not bounded and marked: %d bytes valid=%v", len(text), utf8.ValidString(text))
			}
		}
	}
	if seen != 2 {
		t.Fatalf("(F50) %d instructions listed", seen)
	}
}

// S5: client_gone is recorded.
func TestF51_ClientGoneIsRecordedInTheIndex(t *testing.T) {
	rt, _ := radioRuntime(t, twoStoryScript())
	w := &errWriter{ResponseRecorder: httptest.NewRecorder(), okWrites: 3}
	progServe(NewAPIHandler(rt), w, progBody(withRequestID("gone")))
	req := getRecord(t, rt, "autoradio-request:gone")
	idx := getRecord(t, rt, "autoradio-stretch:"+req.SubjectRefs["stretch_id"])
	if idx == nil || idx.SubjectRefs["status"] != "completed" || idx.SubjectRefs["client_gone"] != "true" {
		t.Fatalf("(F51) index %+v", idx)
	}
	if len(stretchClaims(t, rt, req.SubjectRefs["stretch_id"])) != 2 {
		t.Fatalf("(F51) the call did not continue")
	}
	rt2, _ := radioRuntime(t, twoStoryScript())
	_, evs := progRun(t, rt2, progBody())
	if idx := getRecord(t, rt2, "autoradio-stretch:"+stretchIDOf(t, evs)); idx.SubjectRefs["client_gone"] != "false" {
		t.Fatalf("(F51) a delivered stretch says %q", idx.SubjectRefs["client_gone"])
	}
}

// S6: no prompt, no record.
func TestF52_AnEmptyPromptFailsBeforeTheFirstRecord(t *testing.T) {
	rt, fake := radioRuntime(t, twoStoryScript())
	old := programmePrompt
	programmePrompt = func() (string, bool) { return "", false }
	defer func() { programmePrompt = old }()
	before := allRecordCount(t, rt)
	rec, _ := progPost(rt, progBody())
	if rec.Code < 500 || allRecordCount(t, rt) != before || fake.callCount() != 0 {
		t.Fatalf("(F52) status %d, %d records written, %d calls", rec.Code, allRecordCount(t, rt)-before, fake.callCount())
	}
}

// S7: the dek, U+2028 and U+2029.
func TestF53_DekIsRequiredAndLineSeparatorsAreStripped(t *testing.T) {
	rt, fake := radioRuntime(t, twoStoryScript())
	for _, dek := range []any{"", "  ", nil} {
		rec, _ := progPost(rt, progBody(func(m map[string]any) {
			c := progCandidates()
			if dek == nil {
				delete(c[0], "dek")
			} else {
				c[0]["dek"] = dek
			}
			m["candidates"] = c
		}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("(F53) dek %v: %d", dek, rec.Code)
		}
	}
	progRun(t, rt, progBody(func(m map[string]any) {
		c := progCandidates()
		c[0]["title"] = "a b c"
		c[1]["dek"] = "d e"
		m["candidates"] = c
	}))
	in := fake.calls[0].req.StableUser
	if strings.ContainsAny(in, "  ") {
		t.Errorf("(F53) a line separator survived in the input")
	}
}

// Nit: -0 is 0.
func TestF54_NegativeZeroIsZero(t *testing.T) {
	rt, _ := radioRuntime(t, linesScript(`{"kind":"story","title":"z","text":"The harbour reopened today.","cites":["pub:harbour-1@v1"],"p_heard":-0,"caused_by":"importance"}`))
	_, evs := progRun(t, rt, progBody())
	c := stretchClaims(t, rt, stretchIDOf(t, evs))
	if len(c) != 1 || c[0].Calibration.Claim.Probability != 0 || math.Signbit(c[0].Calibration.Claim.Probability) {
		t.Fatalf("(F54) claims %+v", c)
	}
}
