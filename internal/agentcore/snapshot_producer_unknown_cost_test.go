package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways an unknown capture cost could still mute, or wrongly drive, a
// replay-time capture (docs/design/replay-time-capture-with-unknown-cost-2026-11-15.md,
// written before the code). Tests below pin each:
//
//  1. a tail whose replay is over the bound with no cost on record waits for
//     the 540,000-event ceiling or the interval (the largest computer went 44
//     hours without a snapshot): it must be due, kind replay;
//  2. the unmuting makes a sub-bound tail due: it must not;
//  3. the unknown state becomes due through floor = 0, so a negative Since (a
//     clock step back) leaves it floor-limited and never due: the unknown
//     branch is explicit;
//  4. a known cost inside the floor stops holding (a capture every tick), or a
//     known cost past the floor stops being due;
//  5. the ceiling or the interval change for either state;
//  6. an unknown-cost capture that succeeds does not write its cost, so the
//     floor stays unknown and the next tick captures again; a failed one writes
//     a cost or retries at the next tick instead of waiting out its backoff;
//  7. a record of another computer or another snapshot is trusted;
//  8. the status still says "wait for the ceiling" for a state that no longer
//     waits (floor_why, capture_known);
//  9. a planned restart lands between the end of the capture and the write of
//     its cost record (inFlight is cleared inside the Take closure), loses the
//     record and costs a second full capture on the next boot.
//
// TestUnknownCaptureCostBindsReplayTimeCaptures and the second half of
// TestPersistedCostSkewAndIdentity pinned the old behaviour; the first is
// replaced by TestUnknownCaptureCostTakesAReplayTimeCaptureAndMeasuresIt, the
// second keeps its intent (another computer's record is not trusted) with a
// sub-bound tail.

func TestSnapshotDueWithAnUnknownOrKnownCost(t *testing.T) {
	const paperDur, paperBytes = 37*time.Minute + 40*time.Second, int64(4_800_000_000)
	base := snapshotDueInput{Interval: 7 * 24 * time.Hour, TailEvents: snapshotDefaultTailEvents}
	cases := []struct {
		name        string
		mod         func(in *snapshotDueInput)
		due         bool
		kind        string
		floorLimit  bool
		whyContains string
	}{
		{"unknown cost, tail over the bound: due, replay", func(in *snapshotDueInput) { in.Tail, in.Since = 441_187, 44 * time.Hour }, true, "replay", false, ""},
		{"unknown cost, negative since (a clock step): due, replay", func(in *snapshotDueInput) { in.Tail, in.Since = 441_187, -time.Hour }, true, "replay", false, ""},
		{"unknown cost, tail under the bound: not due", func(in *snapshotDueInput) { in.Tail, in.Since = 20_000, 44 * time.Hour }, false, "", false, ""},
		{"unknown cost, tail exactly at the bound (27,120): not due", func(in *snapshotDueInput) { in.Tail, in.Since = 27_120, time.Hour }, false, "", false, ""},
		{"unknown cost, one event over the bound: due", func(in *snapshotDueInput) { in.Tail, in.Since = 27_121, time.Hour }, true, "replay", false, ""},
		{"known cost, inside the floor: not due, floor-limited", func(in *snapshotDueInput) {
			in.Tail, in.Since, in.HaveCapture, in.CaptureDur, in.CaptureBytes = 441_187, 5*time.Hour, true, paperDur, paperBytes
		}, false, "", true, "the last capture took"},
		{"known cost, past the floor: due, replay", func(in *snapshotDueInput) {
			in.Tail, in.Since, in.HaveCapture, in.CaptureDur, in.CaptureBytes = 441_187, 14*time.Hour, true, paperDur, paperBytes
		}, true, "replay", false, ""},
		{"unknown cost, ceiling", func(in *snapshotDueInput) { in.Tail, in.Since = snapshotDefaultTailEvents, time.Hour }, true, "ceiling", false, ""},
		{"known cost inside the floor, ceiling", func(in *snapshotDueInput) {
			in.Tail, in.Since, in.HaveCapture, in.CaptureDur, in.CaptureBytes = snapshotDefaultTailEvents, time.Hour, true, paperDur, paperBytes
		}, true, "ceiling", false, ""},
		{"unknown cost, interval", func(in *snapshotDueInput) { in.Tail, in.Since = 10, 8 * 24 * time.Hour }, true, "interval", false, ""},
		{"known cost inside the floor, interval", func(in *snapshotDueInput) {
			in.Tail, in.Since, in.Interval, in.HaveCapture, in.CaptureDur, in.CaptureBytes = 441_187, 2*time.Hour, time.Hour, true, paperDur, paperBytes
		}, true, "interval", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base
			c.mod(&in)
			ev := snapshotDue(in)
			if ev.Due != c.due || ev.Kind != c.kind || ev.FloorLimited != c.floorLimit {
				t.Fatalf("due=%v kind=%q floor_limited=%v; want %v %q %v (why %q, floor_why %q)", ev.Due, ev.Kind, ev.FloorLimited, c.due, c.kind, c.floorLimit, ev.Why, ev.FloorWhy)
			}
			if c.whyContains != "" && !strings.Contains(ev.FloorWhy, c.whyContains) {
				t.Fatalf("floor_why %q lacks %q", ev.FloorWhy, c.whyContains)
			}
			if strings.Contains(ev.FloorWhy, "not recorded") || ev.FloorKey == "unknown" {
				t.Fatalf("status still promises a wait for an unknown cost: %q %q", ev.FloorWhy, ev.FloorKey)
			}
		})
	}
}

// An existing snapshot whose capture cost is unknown (an old snapshot, no
// persisted figures, or figures of another snapshot): a tail over the replay
// bound takes a capture at the first tick, which measures the cost; the floor
// is known from then on. Replaces TestUnknownCaptureCostBindsReplayTimeCaptures.
func TestUnknownCaptureCostTakesAReplayTimeCaptureAndMeasuresIt(t *testing.T) {
	h, clock := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-13*time.Hour), 1_000_000
	h.head = 1_000_000 + 150_000
	var p snapshotProducer
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken || h.takes != 1 || !strings.Contains(detail, "restore replay") {
		t.Fatalf("outcome=%s detail=%q takes=%d: an unknown cost must not mute a replay-time capture", outcome, detail, h.takes)
	}
	st := p.status(clock.now)
	if !st.CaptureKnown || st.FloorLimited || st.LastCaptureBytes != 4_800_000_000 {
		t.Fatalf("status after the capture %+v", st)
	}
	rec, ok := loadSnapshotCapture(h.deps.DataDir)
	if !ok || rec.SealedBytes != 4_800_000_000 {
		t.Fatalf("the cost was not recorded beside the store: %+v %v", rec, ok)
	}
	// Past the capture the tail is over the bound again, but the floor is now
	// known and holds.
	clock.now = clock.now.Add(snapshotTickEvery)
	h.head += 30_000
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotNotDue || h.takes != 1 {
		t.Fatalf("outcome=%s takes=%d: the measured floor must hold", outcome, h.takes)
	}
	// A restart keeps it: a fresh producer reads the record and does not capture.
	var q snapshotProducer
	h.newestAt = clock.now.Add(-time.Minute)
	if outcome, _ := h.run(&q, clock.now); outcome != snapshotNotDue || h.takes != 1 || !q.status(clock.now).CaptureKnown {
		t.Fatalf("outcome=%s takes=%d known=%v after a restart", outcome, h.takes, q.status(clock.now).CaptureKnown)
	}

	// Figures that belong to another snapshot are not this one's: the cost is
	// unknown, a capture is taken and its own cost is the one used.
	h2, clock2 := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h2.newestFound, h2.newestAt, h2.newestSeq = true, clock2.now.Add(-13*time.Hour), 1_000_000
	h2.head = 1_000_000 + 150_000
	if err := saveSnapshotCapture(h2.deps.DataDir, snapshotCaptureRecord{StartedAt: clock2.now.Add(-90 * 24 * time.Hour), DurationSeconds: 5, SealedBytes: 1}); err != nil {
		t.Fatal(err)
	}
	var r snapshotProducer
	if outcome, _ := h2.run(&r, clock2.now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s: a stale record must read as unknown and capture", outcome)
	}
	if got := r.status(clock2.now).LastCaptureBytes; got != 4_800_000_000 {
		t.Fatalf("figures of another snapshot were used: %d", got)
	}
}

// A failed unknown-cost capture backs off and records nothing: the cost stays
// unknown and the retry waits out the backoff, not the next tick.
func TestAFailedUnknownCostCaptureBacksOffAndRecordsNothing(t *testing.T) {
	h, clock := capturingHarness(t, time.Minute, 1)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-13*time.Hour), 1_000_000
	h.head = 1_000_000 + 150_000
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) {
		h.takes++
		return snapshot.Ref{}, errors.New("upload failed")
	}
	var p snapshotProducer
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	if _, ok := loadSnapshotCapture(h.deps.DataDir); ok || p.status(clock.now).CaptureKnown {
		t.Fatal("a failed capture recorded a cost")
	}
	clock.now = clock.now.Add(snapshotTickEvery)
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotBackoff || h.takes != 1 {
		t.Fatalf("outcome=%s takes=%d: the retry must wait out the backoff", outcome, h.takes)
	}
}

// inFlight spans the record write: a planned restart refuses while it is set
// (plannedRestartWork.SnapshotInFlight), so it cannot land between the end of
// the capture and the write of its cost and lose it.
func TestInFlightSpansTheCostRecordWrite(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	// Now is read once before Take (the start) and once right after it, before
	// the record is written.
	calls, afterTake := 0, false
	h.deps.Now = func() time.Time {
		calls++
		if calls == 2 {
			afterTake = p.inFlight.Load()
		}
		return now
	}
	h.run(p, now)
	if calls < 2 || !afterTake {
		t.Fatalf("inFlight after Take returned and before the record write: %v (Now calls %d)", afterTake, calls)
	}
	if p.inFlight.Load() {
		t.Fatal("inFlight stays set after the attempt")
	}
}
