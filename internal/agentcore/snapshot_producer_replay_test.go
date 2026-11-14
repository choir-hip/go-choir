package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways the replay-time trigger could fail (lane/snapshot-by-replay-time,
// written before the code; docs/problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md):
//
//  a. a tail whose replay would not fit the restore request is not snapshotted
//     until 540,000 events or seven days, so a restore cannot finish;
//  b. the estimate ignores a rate measured on this computer, or trusts one
//     measured over a handful of events;
//  c. the trigger is a loop on a large store: the paper (37 minute capture,
//     4.8 GB written, 3.3 events/s) captures continuously;
//  d. the floor wins silently: the computer's restore tail exceeds the budget
//     and nothing says so;
//  e. the floor also delays the 540,000-event ceiling or the seven-day
//     interval (they must fire regardless);
//  f. a snapshot that is due and refused is visible only in the console log;
//  g. "refused since" resets at every tick, or survives a taken snapshot;
//  h. the observability route has no snapshot_producer block.

// capturingHarness makes every Take spend dur on a fake clock and report
// uploaded bytes.
func capturingHarness(t *testing.T, dur time.Duration, uploaded int64) (*producerHarness, *fakeClock) {
	t.Helper()
	h := newProducerHarness(t)
	clock := &fakeClock{now: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)}
	h.deps.Now = clock.Now
	h.set.Interval = 7 * 24 * time.Hour
	h.set.TailEvents = snapshotDefaultTailEvents
	h.deps.Take = func(ctx context.Context, _ time.Duration) (snapshot.Ref, error) {
		h.takes++
		clock.now = clock.now.Add(dur)
		return snapshot.Ref{EventID: "e", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 1}, Stats: snapshot.Stats{BytesUploaded: uploaded}}, nil
	}
	return h, clock
}

func TestReplayTimeBoundIsInsideTheRestoreRequest(t *testing.T) {
	// The restore request is ten minutes; scan and install took about four on
	// d@e. The replay bound must leave room for both, the witness and margin.
	if snapshotReplayBound <= 0 || snapshotReplayBound+4*time.Minute+time.Minute >= restoreRequestBudget {
		t.Fatalf("replay bound %s does not fit a %s request after 4 minutes of scan and install", snapshotReplayBound, restoreRequestBudget)
	}
}

func TestProducerSnapshotsWhenTheTailReplaysPastTheBound(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 1<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-time.Hour), 1000
	var p snapshotProducer
	atBound := uint64(float64(snapshotReplayBound/time.Second) * snapshotAssumedReplayRate)
	h.head = 1000 + atBound - 1
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotNotDue {
		t.Fatalf("outcome=%s: a tail that replays inside the bound is not due", outcome)
	}
	h.head = 1000 + atBound + 1000
	outcome, detail := h.run(&p, clock.now)
	if outcome != snapshotTaken || !strings.Contains(detail, "replay") {
		t.Fatalf("outcome=%s detail=%q: a tail past the replay bound is due", outcome, detail)
	}
}

func TestProducerUsesAMeasuredReplayRate(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 1<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-time.Hour), 1000
	h.head = 1000 + 20000
	var p snapshotProducer
	// 20,000 events at 400/s is 50 s: not due. At 40/s it is 500 s: due.
	h.deps.ReplayRate = func() (float64, bool) { return 400, true }
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotNotDue {
		t.Fatalf("outcome=%s: fast measured rate, not due", outcome)
	}
	h.deps.ReplayRate = func() (float64, bool) { return 40, true }
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s: slow measured rate, due", outcome)
	}
}

// d@e: 11 s capture, a small store, about 1.8 events/s. 27,120 events of tail
// (4 minutes at 113/s) is about four hours; the floor is under an hour.
func TestReplayTimeRuleOnDE(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 200<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-time.Hour), 100000
	var p snapshotProducer
	h.head = 100000 + 30000
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("first: outcome=%s detail=%q", outcome, detail)
	}
	// 4.5 hours later, 30,000 more events (1.8/s): due again; the floor
	// (110 s, and 200 MiB at 8 GiB a day is 35 minutes) has passed.
	clock.now = clock.now.Add(4*time.Hour + 30*time.Minute)
	h.head += 30000
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("d@e after 4.5 h: outcome=%s detail=%q", outcome, detail)
	}
}

// The paper: 37 min 40 s capture, 4.8 GB written, 3.3 events/s.
func TestReplayTimeRuleOnThePaperDoesNotCaptureContinuously(t *testing.T) {
	capture := 37*time.Minute + 40*time.Second
	h, clock := capturingHarness(t, capture, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-30*time.Hour), 1_000_000
	h.head = 1_000_000 + 282_000
	var p snapshotProducer
	start := clock.now
	// A day-old snapshot, 282,000 events of tail: well past the bound, and no
	// capture is known yet, so there is no floor: due.
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("paper first: outcome=%s detail=%q", outcome, detail)
	}
	// Every tick for the next 12 hours: the tail grows at 3.3/s, far past the
	// bound, but no second capture may start before the floor.
	for at := clock.now; at.Before(start.Add(12 * time.Hour)); at = at.Add(snapshotTickEvery) {
		clock.now = at
		h.head = 1_000_000 + 282_000 + uint64(at.Sub(start).Seconds()*3.3)
		before := h.takes
		outcome, detail := h.run(&p, clock.now)
		if h.takes != before || outcome != snapshotNotDue {
			t.Fatalf("at +%s: outcome=%s detail=%q: the floor must hold", at.Sub(start), outcome, detail)
		}
		st := p.status(at)
		if st.EstimatedReplaySeconds > snapshotReplayBound.Seconds() {
			if !st.FloorLimited || st.Due {
				t.Fatalf("at +%s: floor_limited=%v due=%v: the floor won over the replay bound and must say so", at.Sub(start), st.FloorLimited, st.Due)
			}
			if !strings.Contains(detail, "capture") {
				t.Fatalf("detail %q does not name the capture cost", detail)
			}
		} else if st.FloorLimited {
			t.Fatalf("at +%s: floor_limited with an estimate inside the bound", at.Sub(start))
		}
	}
	// The floor is the larger of ten times the capture (6.3 h) and the bytes
	// at 8 GiB a day (13.4 h). Past it the next capture is due.
	clock.now = start.Add(14 * time.Hour)
	h.head = 1_000_000 + 282_000 + uint64(14*3600*3.3)
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("paper at +14h: outcome=%s detail=%q", outcome, detail)
	}
	if frac := float64(capture) / float64(14*time.Hour); frac > 0.10 {
		t.Fatalf("capturing %.1f%% of wall time", frac*100)
	}
}

func TestFloorDoesNotDelayTheEventCeilingOrTheInterval(t *testing.T) {
	h, clock := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-2*time.Hour), 1000
	h.head = 5000
	h.set.TailEvents = 0
	h.set.Interval = time.Hour
	var p snapshotProducer
	// Seed a capture so a floor exists.
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("interval: outcome=%s", outcome)
	}
	// Immediately after, inside the floor, the event ceiling still fires.
	h.set.TailEvents = 100
	h.set.Interval = 7 * 24 * time.Hour
	h.head += 100
	clock.now = clock.now.Add(snapshotTickEvery)
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken || !strings.Contains(detail, "tail of") {
		t.Fatalf("ceiling inside the floor: outcome=%s detail=%q", outcome, detail)
	}
	// And so does the interval.
	h.set.TailEvents = 0
	h.set.Interval = time.Hour
	h.head += 10
	clock.now = clock.now.Add(snapshotTickEvery + time.Hour)
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken || !strings.Contains(detail, "interval") {
		t.Fatalf("interval inside the floor: outcome=%s detail=%q", outcome, detail)
	}
}

func TestStatusShowsADueSnapshotThatIsRefused(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 1<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-8*24*time.Hour), 1000
	h.head = 2000
	h.free = 1 << 20 // under the floor
	var p snapshotProducer
	first := clock.now
	for i := 0; i < 3; i++ {
		clock.now = first.Add(time.Duration(i) * snapshotTickEvery)
		if outcome, _ := h.run(&p, clock.now); outcome != snapshotRefused {
			t.Fatalf("outcome=%s", outcome)
		}
	}
	st := p.status(clock.now)
	if !st.Due || st.DueWhy == "" || st.Refusal == "" || st.ConsecutiveRefusals != 3 || st.LastOutcome != string(snapshotRefused) {
		t.Fatalf("status %+v", st)
	}
	if st.RefusedSince != first.UTC().Format(time.RFC3339) {
		t.Fatalf("refused since %q want %s: it must hold across ticks", st.RefusedSince, first.UTC().Format(time.RFC3339))
	}
	if st.NewestSequence != 1000 || st.TailEvents != 1000 || st.EstimatedReplaySeconds <= 0 {
		t.Fatalf("status %+v", st)
	}
	// The refusal clears and a snapshot is taken: the streak ends and the
	// capture is recorded.
	h.free = 1 << 40
	clock.now = clock.now.Add(snapshotTickEvery)
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s", outcome)
	}
	st = p.status(clock.now)
	if st.Refusal != "" || st.RefusedSince != "" || st.ConsecutiveRefusals != 0 || st.LastCaptureSeconds != 11 || st.LastCaptureBytes != 1<<20 {
		t.Fatalf("status after a capture %+v", st)
	}
}

func TestObservabilityHasTheSnapshotProducerBlock(t *testing.T) {
	rt := telemetryRuntime(t, "computer-snapshot-producer-obs")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	raw, ok := obs["snapshot_producer"]
	if !ok {
		t.Fatalf("observability has no snapshot_producer: %s", rec.Body.String())
	}
	var blk map[string]any
	if err := json.Unmarshal(raw, &blk); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"newest_sequence", "tail_events", "estimated_replay_seconds", "due", "consecutive_refusals", "floor_limited"} {
		if _, ok := blk[key]; !ok {
			t.Fatalf("snapshot_producer lacks %q: %s", key, raw)
		}
	}
}
