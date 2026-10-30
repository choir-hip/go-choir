package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways the snapshot producer could fail (written before the code; the list
// above snapshotDefaultInterval in snapshot_producer.go is the full one):
//
//  a. no snapshot exists and none is taken, so no tape ever holds one;
//  b. a snapshot exists and another is taken at every tick;
//  c. a due snapshot is not retaken after the interval;
//  d. it runs beside a live capsule or a self-development apply, in too little
//     free space, or over the size cap, and does not say why;
//  e. two ticks run two snapshots at once;
//  f. a snapshot that keeps crashing the computer is retried forever;
//  g. the kill switch is ignored, or an unset environment is read as off.

type producerHarness struct {
	t     *testing.T
	deps  snapshotDeps
	set   snapshotSettings
	takes int

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
	db := dbxtest.SQLite(t)
	if _, err := db.Write.Exec(`CREATE TABLE filler (id INTEGER PRIMARY KEY, body BLOB)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 64; i++ {
		if _, err := db.Write.Exec(`INSERT INTO filler (id, body) VALUES (?, zeroblob(8192))`, i+1); err != nil {
			t.Fatal(err)
		}
	}
	h := &producerHarness{t: t, head: 50, free: 1 << 40}
	h.set = snapshotSettings{Interval: 24 * time.Hour, MaxBytes: 8 << 30}
	h.deps = snapshotDeps{
		ComputerID: "computer-producer",
		TempDir:    filepath.Join(t.TempDir(), snapshotTempDirName),
		ReadDB:     func() *sql.DB { return db.Read },
		Newest: func(context.Context) (uint64, time.Time, bool, error) {
			return h.newestSeq, h.newestAt, h.newestFound, nil
		},
		HeadSequence: func(context.Context) (uint64, bool, error) { return h.head, h.head > 0, nil },
		Busy:         func(context.Context) (string, error) { return h.busy, nil },
		FreeBytes:    func(string) (uint64, error) { return h.free, nil },
		Ready:        func() bool { return true },
		Take: func(ctx context.Context, tempDir string, maxBytes int64) (snapshot.Ref, error) {
			h.takes++
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
		{"over the size cap", func(h *producerHarness) { h.set.MaxBytes = 1 << 10 }, "cap"},
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

func TestProducerFreeSpaceBoundaryIsOnePointTwoFiveTimesTheStore(t *testing.T) {
	h := newProducerHarness(t)
	size, err := sqliteLogicalSize(context.Background(), h.deps.ReadDB())
	if err != nil {
		t.Fatal(err)
	}
	need := uint64(size) / 4 * 5
	var p snapshotProducer
	h.free = need - 1
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotRefused {
		t.Fatalf("one byte short must refuse, got %s", outcome)
	}
	h.free = need
	if outcome, _ := h.run(&p, time.Now()); outcome != snapshotTaken {
		t.Fatalf("exactly 1.25x must take, got %s", outcome)
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
	if s := snapshotSettingsFromEnv(get); s.Disabled || s.Interval != 24*time.Hour || s.MaxBytes != 8<<30 {
		t.Fatalf("defaults (on, 24h, 8 GiB): %+v", s)
	}
	env[envSnapshotInterval], env[envSnapshotMaxBytes], env[envSnapshotDisable] = "6h", "1000", "1"
	if s := snapshotSettingsFromEnv(get); !s.Disabled || s.Interval != 6*time.Hour || s.MaxBytes != 1000 {
		t.Fatalf("overrides: %+v", s)
	}
	env[envSnapshotInterval], env[envSnapshotMaxBytes], env[envSnapshotDisable] = "soon", "-5", "0"
	if s := snapshotSettingsFromEnv(get); s.Disabled || s.Interval != 24*time.Hour || s.MaxBytes != 8<<30 {
		t.Fatalf("garbage falls back to the defaults, never to off: %+v", s)
	}
}
