package platform

// A capture write hands the vector worker the ids of the captures it wrote
// (station E1 of docs/design/entity-layer-2026-11-20.md). Failure modes
// guarded:
//
//   1. a capture written while the worker is idle waits the idle interval
//      (five minutes in production) for its vector, so the event flush, which
//      fires 60 s after the first queued capture, asks about captures that
//      have none;
//   2. the wake runs the full pass, whose count queries over every projected
//      capture (10 to 18 s at 228,000) would then run back to back;
//   3. a wake cuts a back-off short, or a stream of writes makes embedder
//      calls while the embedder is refusing;
//   4. a just-written capture waits behind an older backlog while the worker
//      is idle (the step reads no backlog; during a pass that is working the
//      next pass covers it);
//   5. a capture the ids wake skipped (set full, restart) is lost: the full
//      pass must still find it;
//   6. an owner-private or already-embedded capture is embedded by the wake.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

// startIdleWorker starts the worker with an idle interval of an hour and
// waits for its first pass (nothing to embed) to finish, so what happens next
// is the idle wait and only a wake ends it.
func startIdleWorker(t *testing.T, f *vecFixture, backoff time.Duration) {
	t.Helper()
	oStart, oIdle, oCheck, oBack, oMax := vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery, vectorBackoffStart, vectorBackoffMax
	vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery = 0, time.Hour, time.Hour
	vectorBackoffStart, vectorBackoffMax = backoff, backoff
	ctx, cancel := context.WithCancel(context.Background())
	f.svc.StartVectorWorker(ctx)
	// The worker read its timings; put them back now (see StartVectorWorker).
	vectorStartDelay, vectorIdleInterval, vectorInboxCheckEvery, vectorBackoffStart, vectorBackoffMax = oStart, oIdle, oCheck, oBack, oMax
	t.Cleanup(cancel)
	deadline := time.Now().Add(10 * time.Second)
	for !f.store.VectorsReady() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // the first pass is over; the worker is in its idle wait
}

func waitCount(t *testing.T, f *vecFixture, table string, want int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for f.count(t, table) < want {
		if time.Now().After(deadline) {
			t.Fatalf("%s has %d rows after %s, want %d", table, f.count(t, table), within, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCaptureWriteEmbedsTheCaptureWithoutWaitingTheIdleIntervalOrRunningTheFullPass(t *testing.T) {
	f := newVecFixture(t)
	logs := fullTextLog(t)
	startIdleWorker(t, f, 20*time.Millisecond)
	before := vectorWholeTableQueries.Load()
	mark := len(logs.String())
	f.capture(t, "o1", "c1", "k1", "Harbor reopens", "the harbor reopened today", time.Now().UTC(), false)
	waitCount(t, f, "vec_captures", 1, 10*time.Second)
	time.Sleep(200 * time.Millisecond)
	if n := vectorWholeTableQueries.Load() - before; n != 0 {
		t.Fatalf("a wake issued %d count or gap queries over the whole projection, want 0", n)
	}
	if strings.Contains(logs.String()[mark:], "op=embed_gap") {
		t.Fatalf("a full pass ran after the write:\n%s", logs.String()[mark:])
	}
}

func TestManyCaptureWritesWhileTheWorkerRunsAreEmbeddedByIdsWithNoFullPass(t *testing.T) {
	f := newVecFixture(t)
	startIdleWorker(t, f, 20*time.Millisecond)
	before := vectorWholeTableQueries.Load()
	for i := 0; i < 12; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("burst-%d", i), "Burst", fmt.Sprintf("burst text number%d", i), time.Now().UTC(), false)
	}
	waitCount(t, f, "vec_captures", 12, 20*time.Second)
	time.Sleep(200 * time.Millisecond)
	if n := vectorWholeTableQueries.Load() - before; n != 0 {
		t.Fatalf("%d whole-projection queries for 12 writes, want 0", n)
	}
	if got := f.emb.Inputs.Load(); got != 12 {
		t.Fatalf("%d inputs embedded for 12 captures", got)
	}
}

func TestTheIdsWakeEmbedsOnlyEligibleCapturesThatLackAVector(t *testing.T) {
	f := newVecFixture(t)
	startIdleWorker(t, f, 20*time.Millisecond)
	f.capture(t, "o1", "c1", "pub", "Public", "public text", time.Now().UTC(), false)
	waitCount(t, f, "vec_captures", 1, 10*time.Second)
	inputs := f.emb.Inputs.Load()
	// the same capture written again, and an owner-private one
	f.capture(t, "o1", "c1", "pub", "Public", "public text", time.Now().UTC(), false)
	f.capture(t, "o1", "c1", "priv", "Private", "private text", time.Now().UTC(), true)
	time.Sleep(500 * time.Millisecond)
	if got := f.emb.Inputs.Load(); got != inputs {
		t.Fatalf("the wake embedded %d more inputs, want none (one has its vector, one is private)", got-inputs)
	}
	if f.count(t, "vec_captures") != 1 {
		t.Fatalf("%d vector rows, want 1", f.count(t, "vec_captures"))
	}
}

func TestJustWrittenCapturesAreEmbeddedBeforeAnOlderBacklog(t *testing.T) {
	f := newVecFixture(t)
	f.captures(t, "o1", "c1", "old", 40, time.Now().UTC().Add(-100*time.Hour), func(i int) string { return fmt.Sprintf("old text number%d", i) }, time.Minute)
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The backlog was named when written; here it stands for captures the
	// worker has not reached (a restart forgot their ids), so the set is
	// emptied and only the new capture is named.
	f.store.takePendingCaptures()
	obj := f.capture(t, "o1", "c1", "new", "New", "new text", time.Now().UTC(), false)
	n, err := f.svc.runCaptureIDStep(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("step embedded %d, err %v", n, err)
	}
	if f.count(t, "vec_captures") != 1 {
		t.Fatalf("%d vectors; only the new capture should have one", f.count(t, "vec_captures"))
	}
	var id string
	_ = f.store.corpus().QueryRow(`SELECT capture_id FROM vec_captures`).Scan(&id)
	if id != obj.CanonicalID {
		t.Fatalf("embedded %s, want the new capture %s", id, obj.CanonicalID)
	}
}

func TestWhenTheIdSetIsFullNothingIsLostTheFullPassFindsIt(t *testing.T) {
	f := newVecFixture(t)
	old := vectorPendingMax
	vectorPendingMax = 3
	defer func() { vectorPendingMax = old }()
	for i := 0; i < 10; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("many-%d", i), "Many", fmt.Sprintf("many text number%d", i), time.Now().UTC(), false)
	}
	if n := f.store.pendingCaptureCount(); n > 3 {
		t.Fatalf("%d ids held, the set holds at most 3", n)
	}
	held := f.store.takePendingCaptures()
	for i := 7; i < 10; i++ {
		want := mustCaptureID(t, "o1", fmt.Sprintf("many-%d", i))
		found := false
		for _, id := range held {
			found = found || id == want
		}
		if !found {
			t.Fatalf("the newest capture many-%d was dropped; held %v", i, held)
		}
	}
	f.pass(t)
	if f.count(t, "vec_captures") != 10 {
		t.Fatalf("%d vectors after the full pass, want 10", f.count(t, "vec_captures"))
	}
}

// attemptEmbedder records the time of every call to the embedder, successful
// or not.
type attemptEmbedder struct {
	inner embedder.Embedder
	mu    sync.Mutex
	at    []time.Time
}

func (a *attemptEmbedder) note() { a.mu.Lock(); a.at = append(a.at, time.Now()); a.mu.Unlock() }
func (a *attemptEmbedder) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.at)
}
func (a *attemptEmbedder) Health(ctx context.Context) error { a.note(); return a.inner.Health(ctx) }
func (a *attemptEmbedder) Embed(ctx context.Context, in []string) ([][]float32, error) {
	a.note()
	return a.inner.Embed(ctx, in)
}

func TestWritesDuringABackOffMakeNoEmbedderCallAndAreEmbeddedWhenItEnds(t *testing.T) {
	f := newVecFixture(t)
	logs := fullTextLog(t)
	att := &attemptEmbedder{inner: f.emb}
	f.store.SetEmbedder(att)
	startIdleWorker(t, f, 3*time.Second)
	f.emb.Down.Store(true)
	f.capture(t, "o1", "c1", "first", "First", "first text", time.Now().UTC(), false)
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(logs.String(), "embedder down, backing off") {
		if time.Now().After(deadline) {
			t.Fatalf("no back-off:\n%s", logs.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	start := time.Now()
	calls := att.count()
	for i := 0; i < 6; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("during-%d", i), "During", fmt.Sprintf("during text number%d", i), time.Now().UTC(), false)
		time.Sleep(40 * time.Millisecond)
	}
	if time.Since(start) < 500*time.Millisecond {
		time.Sleep(500*time.Millisecond - time.Since(start))
	}
	// the back-off is 3 s; at about 500 ms after it began the embedder has
	// not been asked.
	if got := att.count(); got != calls {
		t.Fatalf("%d embedder calls during the back-off, want none", got-calls)
	}
	f.emb.Down.Store(false)
	waitCount(t, f, "vec_captures", 7, 20*time.Second)
}

func TestIdStepsWriteOneSummaryLineAndNoLinePerStep(t *testing.T) {
	f := newVecFixture(t)
	logs := fullTextLog(t)
	if err := f.store.ensureVectorTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := vectorStepLogEvery
	vectorStepLogEvery = time.Hour
	defer func() { vectorStepLogEvery = old }()
	f.store.takePendingCaptures()
	for i := 0; i < 3; i++ {
		f.capture(t, "o1", "c1", fmt.Sprintf("s-%d", i), "S", fmt.Sprintf("step text number%d", i), time.Now().UTC(), false)
		if n, err := f.svc.runCaptureIDStep(context.Background()); err != nil || n != 1 {
			t.Fatalf("step %d: %d, %v", i, n, err)
		}
	}
	if strings.Contains(logs.String(), "corpusd vectors ids") {
		t.Fatalf("a line per step:\n%s", logs.String())
	}
	vectorStepLogEvery = 0
	f.capture(t, "o1", "c1", "s-last", "S", "last text", time.Now().UTC(), false)
	if _, err := f.svc.runCaptureIDStep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "corpusd vectors ids steps=4 named=4 embedded=4") {
		t.Fatalf("no summary line:\n%s", logs.String())
	}
}
