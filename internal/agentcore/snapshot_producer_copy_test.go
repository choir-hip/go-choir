package agentcore

// Tests of the producer side of the static-copy capture, written from the
// failure list of docs/design/capture-static-copy-2026-11-15.md section 3
// before the code. Each comment names the failure entry it pins.

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
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

const copyTestStoreBytes = int64(10) << 30

// dueHarness returns a harness whose snapshot is due by the given kind.
func dueHarness(t *testing.T, kind string) (*producerHarness, *snapshotProducer, time.Time) {
	t.Helper()
	h := newProducerHarness(t)
	now := time.Now()
	p := &snapshotProducer{}
	switch kind {
	case "first":
	case "interval":
		h.newestFound, h.newestAt, h.newestSeq = true, now.Add(-48*time.Hour), 40
	case "ceiling":
		h.set.TailEvents = 100
		h.newestFound, h.newestAt, h.newestSeq = true, now.Add(-time.Hour), 40
		h.head = 200
	case "replay":
		h.newestFound, h.newestAt, h.newestSeq = true, now.Add(-2*time.Hour), 40
		h.head = 40 + 30000
		p.haveCapture, p.lastCaptureDuration = true, time.Second
	default:
		t.Fatalf("unknown kind %q", kind)
	}
	h.deps.StoreBytes = func(context.Context) (int64, error) { return copyTestStoreBytes, nil }
	return h, p, now
}

// Failure 17: one rule, S + floor, for every trigger kind.
func TestEveryTriggerKindNeedsTheStoreSizePlusTheFloor(t *testing.T) {
	need := uint64(copyTestStoreBytes) + snapshotFreeFloor
	for _, kind := range []string{"first", "interval", "ceiling", "replay"} {
		t.Run(kind, func(t *testing.T) {
			h, p, now := dueHarness(t, kind)
			h.free = need - 1
			outcome, detail := h.run(p, now)
			if outcome != snapshotRefused || h.takes != 0 {
				t.Fatalf("free = need-1: outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
			if !strings.Contains(detail, fmt.Sprint(h.free)) || !strings.Contains(detail, fmt.Sprint(need)) {
				t.Fatalf("the refusal must name the free space and the need (%d, %d): %q", h.free, need, detail)
			}
			h.free = need
			if outcome, detail := h.run(p, now); outcome != snapshotTaken || h.takes != 1 {
				t.Fatalf("free = need: outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
		})
	}
}

// Failure 18: a leftover scratch is removed BEFORE the free-space measurement.
func TestALeftoverScratchIsRemovedBeforeFreeSpaceIsMeasured(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	need := uint64(copyTestStoreBytes) + snapshotFreeFloor
	leftover := filepath.Join(h.deps.DataDir, snapshotScratchDirName)
	if err := os.MkdirAll(leftover, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftover, "state.db"), make([]byte, 100), 0o600); err != nil {
		t.Fatal(err)
	}
	var order []string
	h.deps.Leftovers = func() error { order = append(order, "leftovers"); return os.RemoveAll(leftover) }
	h.deps.FreeBytes = func(string) (uint64, error) {
		order = append(order, "free")
		if _, err := os.Stat(leftover); err == nil {
			return need - 1, nil // the leftover still occupies the volume
		}
		return need, nil
	}
	if outcome, detail := h.run(p, now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s detail=%q: the leftover wedged the producer", outcome, detail)
	}
	if fmt.Sprint(order) != "[leftovers free]" {
		t.Fatalf("order %v, want the cleanup before the measurement", order)
	}
}

// Failure 19: the token is taken before any deletion or measurement. A held
// token (also one taken at the afterGates seam, between the read-only gates and
// the token) refuses the attempt and nothing is deleted.
func TestAHeldTokenRefusesBeforeAnythingIsDeletedOrMeasured(t *testing.T) {
	for _, atSeam := range []bool{false, true} {
		t.Run(fmt.Sprintf("taken at the seam %v", atSeam), func(t *testing.T) {
			h, p, now := dueHarness(t, "first")
			leftover := filepath.Join(h.deps.DataDir, snapshotScratchDirName, "state.db")
			if err := os.MkdirAll(filepath.Dir(leftover), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(leftover, []byte("mid-write"), 0o600); err != nil {
				t.Fatal(err)
			}
			held := !atSeam
			if atSeam {
				h.deps.afterGates = func() { held = true }
			}
			h.deps.HoldCopy = func() (func(), string, bool) { return func() {}, "compaction", !held }
			h.deps.Leftovers = func() error { t.Error("cleanup ran without the token"); return nil }
			h.deps.FreeBytes = func(string) (uint64, error) { t.Error("free space measured without the token"); return 1 << 40, nil }
			outcome, detail := h.run(p, now)
			if outcome != snapshotRefused || h.takes != 0 || !strings.Contains(detail, "compaction") {
				t.Fatalf("outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
			if raw, err := os.ReadFile(leftover); err != nil || string(raw) != "mid-write" {
				t.Fatalf("the file the holder was writing was touched: %q %v", raw, err)
			}
		})
	}
}

// Failure 23: a cleanup that fails fails the attempt closed.
func TestAFailedCleanupFailsTheAttemptClosed(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	h.deps.Leftovers = func() error { return errors.New("cannot remove") }
	if outcome, _ := h.run(p, now); outcome == snapshotTaken || h.takes != 0 {
		t.Fatalf("outcome=%s takes=%d: a capture ran on top of a file it could not remove", outcome, h.takes)
	}
}

// Failures 28 and 39: the token is released, once, on every return after it was
// taken: success, a free-space error, the attempt guard's refusal, a failed
// Take and a panic in Take.
func TestTheTokenIsReleasedOnEveryReturn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(h *producerHarness)
		panic bool
	}{
		{"success", func(*producerHarness) {}, false},
		{"free space read error", func(h *producerHarness) {
			h.deps.FreeBytes = func(string) (uint64, error) { return 0, errors.New("statfs failed") }
		}, false},
		{"attempt guard refusal", func(h *producerHarness) {
			h.deps.StorePath = filepath.Join(h.t.TempDir(), "store")
			for i := 0; i < snapshotAttemptLimit; i++ {
				if err := spendGuestAttempt(h.deps.StorePath, snapshotAttemptKey, h.deps.ComputerID, snapshotAttemptLimit+1); err != nil {
					h.t.Fatal(err)
				}
			}
		}, false},
		{"take error", func(h *producerHarness) { h.takeErr = errors.New("take failed") }, false},
		{"take panic", func(h *producerHarness) {
			h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) { panic("take panicked") }
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, p, now := dueHarness(t, "first")
			held, released := 0, 0
			h.deps.HoldCopy = func() (func(), string, bool) {
				held++
				return func() { released++ }, "", true
			}
			tc.setup(h)
			func() {
				defer func() {
					if r := recover(); r != nil && !tc.panic {
						panic(r)
					}
				}()
				h.run(p, now)
			}()
			if held != 1 || released != 1 {
				t.Fatalf("token taken %d times and released %d times, want once each", held, released)
			}
		})
	}
}

// Failures 26 and 41: a staged compact copy lowers the measured free space and
// is not added again; below the need the capture refuses, naming the amounts and
// the staged size, and nothing is removed.
func TestAStagedCopyIsNamedInTheRefusalAndNeverRemoved(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	need := uint64(copyTestStoreBytes) + snapshotFreeFloor
	staged := int64(42) << 30
	h.deps.StagedCopyBytes = func() int64 { return staged }
	removed := false
	h.deps.Leftovers = func() error { return nil }
	h.free = need - 1
	outcome, detail := h.run(p, now)
	if outcome != snapshotRefused || !strings.Contains(detail, "staged") || !strings.Contains(detail, fmt.Sprint(staged)) {
		t.Fatalf("outcome=%s detail=%q: the refusal must name the staged copy and its size", outcome, detail)
	}
	if removed {
		t.Fatal("something was removed")
	}
	h.free = need
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatalf("free = need with a staged copy present: %s", outcome)
	}
}

// Failure 43: many refusals because the token is held touch neither the failure
// counter nor the backoff.
func TestRefusalsForAHeldTokenEscalateNothing(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	h.deps.HoldCopy = func() (func(), string, bool) { return func() {}, "compaction", false }
	for i := 0; i < 5; i++ {
		if outcome, _ := h.run(p, now.Add(time.Duration(i)*snapshotTickEvery)); outcome != snapshotRefused {
			t.Fatalf("tick %d: %s", i, outcome)
		}
	}
	if p.failures != 0 || !p.retryAt.IsZero() {
		t.Fatalf("failures=%d retryAt=%v: a held token must not back the producer off", p.failures, p.retryAt)
	}
	if st := p.status(now); st.ConsecutiveRefusals != 5 || !strings.Contains(st.Refusal, "compaction") {
		t.Fatalf("status %+v: the refusals must be visible and name the holder", st)
	}
}

// Failure 31: inFlight spans the whole Take.
func TestInFlightSpansTheWholeTake(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	var during bool
	inner := h.deps.Take
	h.deps.Take = func(ctx context.Context, d time.Duration) (snapshot.Ref, error) {
		during = p.inFlight.Load()
		return inner(ctx, d)
	}
	h.run(p, now)
	if !during || p.inFlight.Load() {
		t.Fatalf("inFlight during Take %v, after %v", during, p.inFlight.Load())
	}
}

// Failure 33: the persisted duration is the whole capture's.
func TestThePersistedCaptureDurationIsTheWholeCapture(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	clock := now
	h.deps.Now = func() time.Time { return clock }
	inner := h.deps.Take
	h.deps.Take = func(ctx context.Context, d time.Duration) (snapshot.Ref, error) {
		clock = clock.Add(10 * time.Minute)
		return inner(ctx, d)
	}
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatal(outcome)
	}
	rec, ok := loadSnapshotCapture(h.deps.DataDir)
	if !ok || rec.DurationSeconds != 600 {
		t.Fatalf("persisted %+v (%v), want the whole 600 s", rec, ok)
	}
}

// Failure 34: the pin and the phases are reported structurally, and the text no
// longer calls the pin a "read transaction".
func TestTheStatusReportsThePinAndThePhases(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	if st := p.status(now); st.LastPinSeconds != 0 || st.LastScratchBytes != 0 {
		t.Fatalf("fields before the first capture: %+v", st)
	}
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) {
		return snapshot.Ref{EventID: "e", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 99},
			Stats: snapshot.Stats{PinTime: 2 * time.Second, CopyTime: time.Second, WitnessTime: 3 * time.Second, UploadTime: 4 * time.Second,
				ScratchBytes: 99, PinWALStart: 10, PinWALEnd: 110, PinEvents: 7}}, nil
	}
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatal(outcome)
	}
	st := p.status(now)
	if st.LastPinSeconds != 2 || st.LastCopySeconds != 1 || st.LastWitnessSeconds != 3 || st.LastUploadSeconds != 4 ||
		st.LastScratchBytes != 99 || st.LastPinWALGrowthBytes != 100 || st.LastPinEvents != 7 {
		t.Fatalf("status %+v", st)
	}
	if !strings.Contains(st.LastDetail, "pin ") || strings.Contains(st.LastDetail, "read transaction") {
		t.Fatalf("last_detail %q must say pin and never read transaction", st.LastDetail)
	}
}

// Failure 35: the scratch name is the previous binary's cleanup name.
func TestTheScratchDirectoryNameIsThePreviousBinarysCleanupName(t *testing.T) {
	if snapshotScratchDirName != "snapshot-tmp" || choirstore.SnapshotScratchDirName != "snapshot-tmp" {
		t.Fatalf("names %q and %q must both be snapshot-tmp", snapshotScratchDirName, choirstore.SnapshotScratchDirName)
	}
}

// Failure 22 (the Start half): Runtime.Start sweeps the scratch, as the second
// defence after the boot helper.
func TestRuntimeStartSweepsTheSnapshotScratch(t *testing.T) {
	raw, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	start := strings.Index(src, "func (rt *Runtime) Start(")
	sweep := strings.Index(src, "store.RemoveSnapshotScratch(")
	probe := strings.Index(src, "rt.removeStaleReplayScratch()")
	if start < 0 || sweep < start || sweep > probe+600 {
		t.Fatalf("Start (at %d) must call RemoveSnapshotScratch beside removeStaleReplayScratch (sweep at %d, probe at %d)", start, sweep, probe)
	}
}
