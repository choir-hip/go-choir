package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Review round 2 should-fixes: skew and identity of the persisted cost, the
// floor after a failed-then-retried capture, and the producer after a restore.
func TestPersistedCostSkewAndIdentity(t *testing.T) {
	start := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	rec := snapshotCaptureRecord{StartedAt: start, DurationSeconds: 60, SealedBytes: 1}
	if !rec.belongsTo(start.Add(-30*time.Second)) || rec.belongsTo(start.Add(-5*time.Minute)) || !rec.belongsTo(start.Add(2*time.Minute)) || rec.belongsTo(start.Add(2*time.Hour)) {
		t.Fatal("belongsTo: a minute of skew before the start, and the capture's own window after it")
	}
	h, clock := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-13*time.Hour), 1_000_000
	h.head = 1_000_000 + 150_000
	if err := saveSnapshotCapture(h.deps.DataDir, snapshotCaptureRecord{ComputerID: "another-computer", StartedAt: h.newestAt.Add(-time.Minute), DurationSeconds: 60, SealedBytes: 1}); err != nil {
		t.Fatal(err)
	}
	var p snapshotProducer
	h.run(&p, clock.now)
	if p.status(clock.now).CaptureKnown {
		t.Fatal("another computer's capture cost was trusted")
	}
}

// A failed attempt uploads chunks the retry then finds held: the retry's own
// bytes understate the cost, so the floor keeps what was known.
func TestFloorKeepsTheKnownCostAfterAFailedThenRetriedCapture(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 0)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-8*24*time.Hour), 1000
	h.head = 5000
	h.seedCapture(37*time.Minute, 4_800_000_000)
	var p snapshotProducer
	failing := true
	inner := h.deps.Take
	h.deps.Take = func(ctx context.Context, d time.Duration) (snapshot.Ref, error) {
		if failing {
			h.takes++
			return snapshot.Ref{}, errors.New("snapshot: witness: exceeded the time budget")
		}
		return inner(ctx, d)
	}
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotFailed {
		t.Fatalf("outcome=%s", outcome)
	}
	failing = false
	clock.now = clock.now.Add(snapshotBackoffWait(1) + time.Minute)
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("retry: outcome=%s detail=%q", outcome, detail)
	}
	if st := p.status(clock.now); st.LastCaptureBytes < 4_800_000_000 {
		t.Fatalf("the retry's 0 uploaded bytes became the floor's input: %+v", st)
	}
}

// After a restore replaces the store the producer reads the newest snapshot
// and head again.
func TestProducerForgetsAfterARestore(t *testing.T) {
	h, clock := capturingHarness(t, 11*time.Second, 1<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-time.Hour), 1000
	h.head = 1100
	var p snapshotProducer
	h.run(&p, clock.now)
	h.newestSeq, h.head = 100, 150 // the restored store is behind
	h.run(&p, clock.now)
	if p.status(clock.now).NewestSequence != 1000 {
		t.Fatal("test premise: the producer remembers the pre-restore snapshot")
	}
	p.forget()
	h.run(&p, clock.now)
	if st := p.status(clock.now); st.NewestSequence != 100 || st.HeadSequence != 150 {
		t.Fatalf("after forget: %+v, want the restored store's snapshot 100 and head 150", st)
	}
}

// Review round 1 of lane/snapshot-by-replay-time, finding 1 and its
// should-fixes. See the failure list in snapshot_producer_replay_test.go
// (i and l).

// A new producer (a restart) on a large store whose newest snapshot is 13
// hours old and whose capture cost was persisted: the first tick is
// floor-limited, not taken.
func TestFloorSurvivesARestart(t *testing.T) {
	capture := 37*time.Minute + 40*time.Second
	h, clock := capturingHarness(t, capture, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-30*time.Hour), 1_000_000
	h.head = 1_000_000 + 282_000
	var before snapshotProducer
	// The ceiling-or-interval rule takes one capture and measures it.
	h.set.Interval = time.Hour
	if outcome, detail := h.run(&before, clock.now); outcome != snapshotTaken {
		t.Fatalf("first: outcome=%s detail=%q", outcome, detail)
	}
	h.set.Interval = 7 * 24 * time.Hour
	// The process restarts: a new producer, the snapshot event now on the
	// tape 13 hours back, 154,000 events of tail.
	h.newestAt, h.newestSeq = clock.now, h.head
	clock.now = clock.now.Add(13 * time.Hour)
	h.head += 154_000
	var after snapshotProducer
	takes := h.takes
	outcome, detail := h.run(&after, clock.now)
	if outcome != snapshotNotDue || h.takes != takes {
		t.Fatalf("restart: outcome=%s detail=%q: the persisted floor must hold the capture back", outcome, detail)
	}
	st := after.status(clock.now)
	if !st.FloorLimited || !st.CaptureKnown || st.Due {
		t.Fatalf("restart: status %+v", st)
	}
	// Past the floor (13.4 h of bytes) the capture is due.
	clock.now = clock.now.Add(time.Hour)
	h.head += 12_000
	if outcome, detail := h.run(&after, clock.now); outcome != snapshotTaken {
		t.Fatalf("past the floor: outcome=%s detail=%q", outcome, detail)
	}
}

// An existing snapshot whose capture cost is unknown (an old snapshot, no
// persisted figures, or figures of another snapshot): the floor is unknown and
// binds replay-time captures until the ceiling or the interval takes one.
func TestUnknownCaptureCostBindsReplayTimeCaptures(t *testing.T) {
	h, clock := capturingHarness(t, 37*time.Minute, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-13*time.Hour), 1_000_000
	h.head = 1_000_000 + 150_000
	var p snapshotProducer
	outcome, detail := h.run(&p, clock.now)
	if outcome != snapshotNotDue || h.takes != 0 {
		t.Fatalf("outcome=%s detail=%q: an unknown floor must not allow a replay-time capture", outcome, detail)
	}
	st := p.status(clock.now)
	if st.CaptureKnown || !st.FloorLimited || !strings.Contains(st.FloorWhy, "not recorded") {
		t.Fatalf("status %+v", st)
	}
	// Figures that belong to another snapshot are not this one's.
	if err := saveSnapshotCapture(h.deps.DataDir, snapshotCaptureRecord{StartedAt: clock.now.Add(-90 * 24 * time.Hour), DurationSeconds: 5, SealedBytes: 1}); err != nil {
		t.Fatal(err)
	}
	p = snapshotProducer{}
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotNotDue || h.takes != 0 {
		t.Fatalf("outcome=%s: figures of another snapshot were trusted", outcome)
	}
	// The event ceiling takes one, measures it, and the floor is then known.
	h.head = 1_000_000 + snapshotDefaultTailEvents
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken || !strings.Contains(detail, "tail of") {
		t.Fatalf("ceiling: outcome=%s detail=%q", outcome, detail)
	}
	if st := p.status(clock.now); !st.CaptureKnown || st.LastCaptureBytes != 4_800_000_000 {
		t.Fatalf("after the capture: %+v", st)
	}
}

// A replay-time capture needs room for the WAL its read transaction pins
// (about 2 MiB/s of the last capture's duration) above the free-space floor;
// the ceiling and the interval do not.
func TestReplayTimeCaptureNeedsWALHeadroom(t *testing.T) {
	h, clock := capturingHarness(t, 37*time.Minute+40*time.Second, 4_800_000_000)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-14*time.Hour), 1_000_000
	h.seedCapture(37*time.Minute+40*time.Second, 4_800_000_000)
	h.head = 1_000_000 + 160_000
	h.free = snapshotFreeFloor + 1<<30 // above the floor, under floor + 4.4 GiB
	var p snapshotProducer
	outcome, detail := h.run(&p, clock.now)
	if outcome != snapshotRefused || !strings.Contains(detail, "headroom") || h.takes != 0 {
		t.Fatalf("outcome=%s detail=%q takes=%d", outcome, detail, h.takes)
	}
	h.free = snapshotFreeFloor + 8<<30
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("with headroom: outcome=%s detail=%q", outcome, detail)
	}
	// The ceiling takes the same low room (the floor of 2 GiB is the only
	// rule there).
	h.newestAt, h.newestSeq = clock.now, h.head
	h.seedCapture(37*time.Minute+40*time.Second, 4_800_000_000)
	h.free = snapshotFreeFloor + 1<<30
	h.head += snapshotDefaultTailEvents
	if outcome, detail := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("ceiling: outcome=%s detail=%q", outcome, detail)
	}
}

// The block is current right after a capture, and reports the rate used.
func TestStatusIsCurrentAfterACaptureAndReportsTheRateUsed(t *testing.T) {
	var fresh snapshotProducer
	if st := fresh.status(time.Now()); st.ReplayRate != snapshotAssumedReplayRate || st.ReplayRateSource != "assumed" {
		t.Fatalf("a fresh producer reports rate %v (%s)", st.ReplayRate, st.ReplayRateSource)
	}
	h, clock := capturingHarness(t, 11*time.Second, 1<<20)
	h.newestFound, h.newestAt, h.newestSeq = true, clock.now.Add(-8*24*time.Hour), 1000
	h.head = 2000
	var p snapshotProducer
	if outcome, _ := h.run(&p, clock.now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s", outcome)
	}
	st := p.status(clock.now)
	if st.Due || st.TailEvents != 0 || st.NewestSequence != 2000 || st.DueWhy != "" || st.LastOutcome != string(snapshotTaken) {
		t.Fatalf("stale block after a capture: %+v", st)
	}
	h.deps.ReplayRate = func() (float64, bool) { return 40, true }
	h.head = 2100
	h.run(&p, clock.now.Add(snapshotTickEvery))
	if st := p.status(clock.now); st.ReplayRate != 40 || st.ReplayRateSource != "measured" {
		t.Fatalf("rate used %v (%s), want the slower measured 40", st.ReplayRate, st.ReplayRateSource)
	}
}

// The floor message is logged when the capture facts change, not when its
// countdown does.
func TestFloorKeyDoesNotChangeWithTheCountdown(t *testing.T) {
	in := snapshotDueInput{Tail: 150_000, Since: 5 * time.Hour, Interval: 7 * 24 * time.Hour, HaveCapture: true, CaptureDur: 37 * time.Minute, CaptureBytes: 4_800_000_000}
	a := snapshotDue(in)
	in.Since = 5*time.Hour + 10*time.Minute
	in.Tail += 2000
	b := snapshotDue(in)
	if !a.FloorLimited || !b.FloorLimited || a.FloorKey == "" || a.FloorKey != b.FloorKey || a.FloorWhy == b.FloorWhy {
		t.Fatalf("keys %q %q; the key must hold while the message's countdown moves", a.FloorKey, b.FloorKey)
	}
}
