package agentcore

// Tests of the producer side of the capture in one pass
// (docs/design/snapshot-one-pass-2026-11-22.md): the free-space rule is the
// floor plus WAL headroom, never the store's size; no store-copy token, no
// scratch directory and no leftover cleanup belong to an attempt.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

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
	return h, p, now
}

// Failure 17: one rule, the floor plus WAL headroom, for every trigger kind, and
// the store's size is not in it.
func TestEveryTriggerKindNeedsTheFloorPlusWALHeadroom(t *testing.T) {
	need := snapshotWALHeadroom(0) + snapshotFreeFloor
	for _, kind := range []string{"first", "interval", "ceiling", "replay"} {
		t.Run(kind, func(t *testing.T) {
			h, p, now := dueHarness(t, kind)
			h.free = need - 1
			outcome, detail := h.run(p, now)
			if outcome != snapshotRefused || h.takes != 0 {
				t.Fatalf("free = need-1: outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
			if !strings.Contains(detail, fmt.Sprint(h.free)) || !strings.Contains(detail, fmt.Sprint(need)) || !strings.Contains(detail, "WAL headroom") {
				t.Fatalf("the refusal must name the free space, the need and the WAL term (%d, %d): %q", h.free, need, detail)
			}
			if st := p.status(now); st.ConsecutiveRefusals != 1 || !strings.Contains(st.Refusal, fmt.Sprint(need)) {
				t.Fatalf("the refusal is not visible in status: %+v", st)
			}
			h.free = need
			if outcome, detail := h.run(p, now); outcome != snapshotTaken || h.takes != 1 {
				t.Fatalf("free = need: outcome=%s takes=%d detail=%q", outcome, h.takes, detail)
			}
		})
	}
}

// The headroom is the planned WAL rate over the pin deadline (a few GiB), far
// under a 51 GiB store, and twice the last capture's own growth when that is
// larger.
func TestTheWALHeadroomIsThePlannedRateOrTwiceTheLastGrowth(t *testing.T) {
	planned := uint64(snapshotWALBytesPerSecond) * uint64(snapshotPinDeadline/time.Second)
	if got := snapshotWALHeadroom(0); got != planned || got > uint64(8)<<30 {
		t.Fatalf("headroom %d, want the planned %d (under 8 GiB)", got, planned)
	}
	if got := snapshotWALHeadroom(1 << 20); got != planned {
		t.Fatalf("a small last growth changed the headroom to %d", got)
	}
	if got, want := snapshotWALHeadroom(int64(10)<<30), uint64(20)<<30; got != want {
		t.Fatalf("headroom %d after a 10 GiB growth, want %d", got, want)
	}
}

// A capture of a very large store with only the headroom free is taken: nothing
// in the rule scales with the store.
func TestAStoreFarLargerThanFreeSpaceIsStillCaptured(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	h.free = snapshotWALHeadroom(0) + snapshotFreeFloor
	if outcome, detail := h.run(p, now); outcome != snapshotTaken {
		t.Fatalf("outcome=%s detail=%q", outcome, detail)
	}
}

// Failures 26 and 41: a staged compact copy lowers the measured free space and is
// not added again; below the need the capture refuses, naming the amounts and the
// staged size. The size is the store's own helper on real files: a capture never
// touches the staged copy or its sidecar.
func TestAStagedCopyIsNamedInTheRefusalAndNeverRemoved(t *testing.T) {
	h, p, now := dueHarness(t, "first")
	need := snapshotWALHeadroom(0) + snapshotFreeFloor
	marker := filepath.Join(h.deps.DataDir, "store") // StateDBPath(marker) is DataDir/state.db
	staged := filepath.Join(h.deps.DataDir, "state.db.compact")
	sidecar := filepath.Join(h.deps.DataDir, "state.db.compact.json")
	if err := os.WriteFile(staged, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sidecar, []byte(`{"sequence":7}`), 0o600); err != nil {
		t.Fatal(err)
	}
	h.deps.StagedCopyBytes = func() int64 { return choirstore.CompactStagedBytes(marker) }
	h.free = need - 1
	outcome, detail := h.run(p, now)
	if outcome != snapshotRefused || !strings.Contains(detail, "staged") || !strings.Contains(detail, fmt.Sprint(1<<20)) {
		t.Fatalf("outcome=%s detail=%q: the refusal must name the staged copy and its size", outcome, detail)
	}
	if info, err := os.Stat(staged); err != nil || info.Size() != 1<<20 {
		t.Fatalf("the staged copy was touched: %v %v", info, err)
	}
	if raw, err := os.ReadFile(sidecar); err != nil || string(raw) != `{"sequence":7}` {
		t.Fatalf("the staged copy's sidecar was touched: %q %v", raw, err)
	}
	h.free = need
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatalf("free = need with a staged copy present: %s", outcome)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("the staged copy was removed by a capture that took: %v", err)
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
	if st := p.status(now); st.LastPinSeconds != 0 || st.LastStateBytes != 0 {
		t.Fatalf("fields before the first capture: %+v", st)
	}
	h.deps.Take = func(context.Context, time.Duration) (snapshot.Ref, error) {
		return snapshot.Ref{EventID: "e", Record: snapshot.Record{StoreHeadSequence: h.head, PlaintextSize: 99},
			Stats: snapshot.Stats{PinTime: 2 * time.Second, UploadTime: 4 * time.Second,
				StateBytes: 99, PinWALStart: 10, PinWALEnd: 110, PinEvents: 7}}, nil
	}
	if outcome, _ := h.run(p, now); outcome != snapshotTaken {
		t.Fatal(outcome)
	}
	st := p.status(now)
	if st.LastPinSeconds != 2 || st.LastUploadSeconds != 4 ||
		st.LastStateBytes != 99 || st.LastPinWALGrowthBytes != 100 || st.LastPinEvents != 7 {
		t.Fatalf("status %+v", st)
	}
	if !strings.Contains(st.LastDetail, "pin ") || strings.Contains(st.LastDetail, "read transaction") {
		t.Fatalf("last_detail %q must say pin and never read transaction", st.LastDetail)
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

// The production constructor sets the hook the producer requires, and the
// capture it builds has no scratch directory, no store-copy token and a pin
// deadline.
func TestTheProductionConstructorWiresEveryHook(t *testing.T) {
	st := newStreamStore(t, 20)
	client, err := computerevent.NewHTTPClient("http://127.0.0.1:1", http.DefaultClient,
		func(context.Context) (string, error) { return "capability", nil }, true)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rt := &Runtime{
		store:                 st,
		eventAppender:         &computerevent.ComputerEventAppender{},
		privateArtifactCipher: streamCipher(t),
		eventPayloadReader:    client,
		cfg:                   provideriface.Config{ComputerID: streamComputer, StorePath: filepath.Join(dir, "store")},
	}
	deps, ok := rt.snapshotDepsForRuntime()
	if !ok {
		t.Fatal("the constructor refused a runtime with a store, an appender, a cipher and an event client")
	}
	if deps.StagedCopyBytes == nil || deps.Take == nil || deps.FreeBytes == nil {
		t.Fatalf("a hook is not wired: StagedCopyBytes %v, Take %v, FreeBytes %v", deps.StagedCopyBytes != nil, deps.Take != nil, deps.FreeBytes != nil)
	}
	raw, err := os.ReadFile("snapshot_producer.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "PinDeadline: snapshotPinDeadline") {
		t.Fatal("the capture is built without its pin deadline")
	}
	for _, gone := range []string{"ScratchDir", "TryHoldStoreCopy", "RemoveCaptureLeftovers", "Witness:"} {
		if strings.Contains(src, gone) {
			t.Fatalf("the producer still names %q: the old capture path is not deleted", gone)
		}
	}
}
