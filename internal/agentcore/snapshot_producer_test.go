package agentcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways the snapshot producer could fail (written before the code; the list
// above snapshotDefaultInterval in snapshot_producer.go is the full one):
//
//  a. no snapshot exists and none is taken, so no tape ever holds one;
//  b. a snapshot exists and another is taken at every tick;
//  c. a due snapshot is not retaken after the interval;
//  d. it runs beside a live capsule or a self-development apply, or with the
//     volume under the WAL floor, and does not say why;
//  e. two ticks run two snapshots at once;
//  f. a snapshot that keeps crashing the computer is retried forever;
//  g. the kill switch is ignored, or an unset environment is read as off;
//  h. a store of any size is refused (there is no size cap and no 1.25x
//     free-space rule: the stream writes no copy);
//  i. the time budget is dropped on the way to the capture, or its abort is
//     read as success or logged without the reason;
//  j. a copy left by the VACUUM INTO era stays on the volume.

type producerHarness struct {
	t          *testing.T
	deps       snapshotDeps
	set        snapshotSettings
	takes      int
	tookWithin time.Duration

	newestFound bool
	newestAt    time.Time
	newestSeq   uint64
	head        uint64
	busy        string
	free        uint64
	takeErr     error
	block       chan struct{}
	entered     chan struct{}
}

func newProducerHarness(t *testing.T) *producerHarness {
	t.Helper()
	h := &producerHarness{t: t, head: 50, free: 1 << 40}
	h.set = snapshotSettings{Interval: 24 * time.Hour, MaxDuration: 30 * time.Minute}
	h.deps = snapshotDeps{
		ComputerID: "computer-producer",
		DataDir:    t.TempDir(),
		Newest: func(context.Context) (uint64, time.Time, bool, error) {
			return h.newestSeq, h.newestAt, h.newestFound, nil
		},
		HeadSequence: func(context.Context) (uint64, bool, error) { return h.head, h.head > 0, nil },
		Busy:         func(context.Context) (string, error) { return h.busy, nil },
		FreeBytes:    func(string) (uint64, error) { return h.free, nil },
		Ready:        func() bool { return true },
		Take: func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error) {
			h.takes++
			h.tookWithin = maxDuration
			if h.entered != nil {
				h.entered <- struct{}{}
				<-h.block
			}
			if h.takeErr != nil {
				return snapshot.Ref{}, h.takeErr
			}
			return snapshot.Ref{EventID: "event-snapshot", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 1}}, nil
		},
	}
	return h
}

func (h *producerHarness) run(p *snapshotProducer, now time.Time) (snapshotOutcome, string) {
	return p.run(context.Background(), h.deps, h.set, now)
}

func TestProducerTakesTheFirstSnapshotWhenNoneExists(t *testing.T) {
	h := newProducerHarness(t)
	var p snapshotProducer
	outcome, detail := h.run(&p, time.Now())
	if outcome != snapshotTaken || h.takes != 1 || !strings.Contains(detail, "first") {
		t.Fatalf("outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
	}
}

func TestProducerDoesNotRetakeWhenOneExistsAndTheIntervalHasNotPassed(t *testing.T) {
	h := newProducerHarness(t)
	now := time.Now()
	h.newestFound, h.newestAt, h.newestSeq = true, now.Add(-time.Hour), 40
	var p snapshotProducer
	if outcome, _ := h.run(&p, now); outcome != snapshotNotDue || h.takes != 0 {
		t.Fatalf("outcome=%s takes=%d: a snapshot an hour old must stand", outcome, h.takes)
	}
	// The interval passes: the next one is due.
	if outcome, detail := h.run(&p, now.Add(25*time.Hour)); outcome != snapshotTaken || h.takes != 1 || !strings.Contains(detail, "interval") {
		t.Fatalf("outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
	}
	// And not again at the next tick.
	if outcome, _ := h.run(&p, now.Add(25*time.Hour+10*time.Minute)); outcome != snapshotNotDue || h.takes != 1 {
		t.Fatalf("outcome=%s takes=%d: one snapshot per interval", outcome, h.takes)
	}
}

func TestProducerSkipsAnIntervalWithNoEventsSinceTheNewestSnapshot(t *testing.T) {
	h := newProducerHarness(t)
	now := time.Now()
	h.newestFound, h.newestAt, h.newestSeq = true, now.Add(-48*time.Hour), 49
	h.head = 50 // only the snapshot_recorded event itself
	var p snapshotProducer
	if outcome, _ := h.run(&p, now); outcome != snapshotNotDue || h.takes != 0 {
		t.Fatalf("outcome=%s takes=%d", outcome, h.takes)
	}
}

func TestProducerRefusesWithAReasonAndRetries(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(h *producerHarness)
		reason  string
	}{
		{"a self-development apply or live capsule", func(h *producerHarness) { h.busy = "1 engineering capsule(s) live" }, "capsule"},
		{"too little free space", func(h *producerHarness) { h.free = 1 << 10 }, "free space"},
		{"free space unmeasurable", func(h *producerHarness) {
			h.deps.FreeBytes = func(string) (uint64, error) { return 0, errors.New("statfs failed") }
		}, "free space"},
		{"busy state unreadable", func(h *producerHarness) {
			h.deps.Busy = func(context.Context) (string, error) { return "", errors.New("operations unreadable") }
		}, "fails closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newProducerHarness(t)
			tc.arrange(h)
			var p snapshotProducer
			outcome, detail := h.run(&p, time.Now())
			if outcome != snapshotRefused || h.takes != 0 || !strings.Contains(detail, tc.reason) {
				t.Fatalf("outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
			// The refusal clears: the next tick takes it.
			h2 := newProducerHarness(t)
			if outcome, _ := h2.run(&p, time.Now()); outcome != snapshotTaken {
				t.Fatalf("after the refusal cleared: outcome=%s", outcome)
			}
		})
	}
}

func TestProducerFreeSpaceBoundaryIsTheWALFloorNotTheStoreSize(t *testing.T) {
	h := newProducerHarness(t)
	var p snapshotProducer
	h.free = snapshotFreeFloor - 1
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotRefused {
		t.Fatalf("one byte under the floor must refuse, got %s", outcome)
	}
	h.free = snapshotFreeFloor
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotTaken {
		t.Fatalf("exactly the floor must take, got %s", outcome)
	}
}

// h: there is no cap: nothing in the producer reads the store's size, so a
// store of any size is taken as long as the floor holds.
func TestProducerHasNoSizeCap(t *testing.T) {
	h := newProducerHarness(t)
	h.free = 1 << 50
	var p snapshotProducer
	if outcome, detail := h.run(&p, time.Now()); outcome != snapshotTaken {
		t.Fatalf("outcome=%s detail=%q", outcome, detail)
	}
}

// i: the budget reaches the capture, and its abort is a logged failure that
// names the budget and the knob.
func TestProducerPassesTheBudgetAndReportsItsAbort(t *testing.T) {
	h := newProducerHarness(t)
	h.set.MaxDuration = 7 * time.Minute
	var p snapshotProducer
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotTaken || h.tookWithin != 7*time.Minute {
		t.Fatalf("the capture ran with budget %s", h.tookWithin)
	}
	h2 := newProducerHarness(t)
	h2.takeErr = fmt.Errorf("snapshot: read pages: %w (100 of 9000 pages read)", snapshot.ErrTimeBudget)
	var p2 snapshotProducer
	outcome, detail := h2.run(&p2, time.Now())
	if outcome != snapshotFailed || !strings.Contains(detail, envSnapshotMaxDuration) || !strings.Contains(detail, "pages read") || !strings.Contains(detail, "nothing recorded") {
		t.Fatalf("outcome=%s detail=%q", outcome, detail)
	}
	// An abort is retried at the next tick, not closed.
	h2.takeErr = nil
	if outcome, _ := h2.run(&p2, time.Now()); outcome != snapshotTaken {
		t.Fatalf("after an abort: %s", outcome)
	}
}

// j: a crashed VACUUM INTO copy is removed at the start of an attempt.
func TestProducerRemovesTheLegacyCopyDirectory(t *testing.T) {
	h := newProducerHarness(t)
	legacy := filepath.Join(h.deps.DataDir, snapshotLegacyTempDirName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "snapshot-123.db"), []byte("copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	var p snapshotProducer
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotTaken {
		t.Fatalf("outcome=%s", outcome)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("the legacy copy directory is still there: %v", err)
	}
}

func TestProducerRunsOneSnapshotAtATime(t *testing.T) {
	h := newProducerHarness(t)
	h.block, h.entered = make(chan struct{}), make(chan struct{})
	var p snapshotProducer
	done := make(chan snapshotOutcome, 1)
	go func() {
		outcome, _ := h.run(&p, time.Now())
		done <- outcome
	}()
	<-h.entered
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotSkipped {
		t.Fatalf("a second tick during a snapshot: %s, want skipped", outcome)
	}
	close(h.block)
	if outcome := <-done; outcome != snapshotTaken {
		t.Fatalf("first tick: %s", outcome)
	}
	if h.takes != 1 {
		t.Fatalf("%d snapshots, want 1", h.takes)
	}
}

func TestProducerWaitsForBootAndHonoursTheKillSwitch(t *testing.T) {
	h := newProducerHarness(t)
	var p snapshotProducer
	h.deps.Ready = func() bool { return false }
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotSkipped || h.takes != 0 {
		t.Fatalf("an unsettled boot took a snapshot: %s", outcome)
	}
	h.deps.Ready = func() bool { return true }
	h.set.Disabled = true
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotSkipped || h.takes != 0 {
		t.Fatalf("the kill switch was ignored: %s", outcome)
	}
}

func TestProducerClosesAfterCrashingTheComputer(t *testing.T) {
	h := newProducerHarness(t)
	h.deps.StorePath = filepath.Join(t.TempDir(), "runtime.db")
	for i := 0; i < snapshotAttemptLimit; i++ {
		if err := spendGuestAttempt(h.deps.StorePath, snapshotAttemptKey, h.deps.ComputerID, snapshotAttemptLimit); err != nil {
			t.Fatal(err)
		}
	}
	var p snapshotProducer
	outcome, detail := h.run(&p, time.Now())
	if outcome != snapshotRefused || h.takes != 0 || !strings.Contains(detail, "crashing") {
		t.Fatalf("outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
	}
}

func TestProducerClearsItsAttemptWhenTheSnapshotReturns(t *testing.T) {
	h := newProducerHarness(t)
	h.deps.StorePath = filepath.Join(t.TempDir(), "runtime.db")
	h.takeErr = errors.New("platform unreachable")
	var p snapshotProducer
	for i := 0; i < snapshotAttemptLimit+2; i++ {
		if outcome, _ := h.run(&p, time.Now()); outcome != snapshotFailed {
			t.Fatalf("attempt %d: %s, want failed (a returned failure is not a crash)", i, outcome)
		}
	}
	h.takeErr = nil
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotTaken {
		t.Fatalf("after the platform returned: %s", outcome)
	}
}

func TestSnapshotSettingsFromEnv(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if s := snapshotSettingsFromEnv(get); s.Disabled || s.Interval != 24*time.Hour || s.MaxDuration != 30*time.Minute {
		t.Fatalf("defaults (on, 24h, 30m): %+v", s)
	}
	env[envSnapshotInterval], env[envSnapshotMaxDuration], env[envSnapshotDisable] = "6h", "2h", "1"
	if s := snapshotSettingsFromEnv(get); !s.Disabled || s.Interval != 6*time.Hour || s.MaxDuration != 2*time.Hour {
		t.Fatalf("overrides: %+v", s)
	}
	env[envSnapshotInterval], env[envSnapshotMaxDuration], env[envSnapshotDisable] = "soon", "-5s", "0"
	if s := snapshotSettingsFromEnv(get); s.Disabled || s.Interval != 24*time.Hour || s.MaxDuration != 30*time.Minute {
		t.Fatalf("garbage falls back to the defaults, never to off: %+v", s)
	}
}
