package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The group is taken late (the commit group's take moved inside sealRiding,
// after flushMu is held): writers that arrive while the leader waits behind a
// telemetry seal join its group. These tests hold flushMu in the test to be
// that seal, and count pending members as the new design does: the leader's own
// member stays pending until the take.

// lateHarness holds flushMu, starts a leader, and returns once that leader has
// won the token (so it is, or is about to be, waiting on flushMu).
type lateHarness struct {
	*tapeHarness
	sl      *telemetrySealer
	results chan writeResult
	held    bool
}

func newLateHarness(t *testing.T) *lateHarness {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	h.warm()
	return &lateHarness{tapeHarness: h, sl: sl, results: make(chan writeResult, 16)}
}

// holdFlush stands in for an in-flight telemetry seal.
func (l *lateHarness) holdFlush() {
	l.sl.flushMu.Lock()
	l.held = true
}

func (l *lateHarness) releaseFlush() {
	if l.held {
		l.held = false
		l.sl.flushMu.Unlock()
	}
}

func (l *lateHarness) startLeader(id string) {
	l.t.Helper()
	won := make(chan struct{}, 1)
	hook := func() {
		select {
		case won <- struct{}{}:
		default:
		}
	}
	l.store.tapeGroup.afterToken.Store(&hook)
	go func() {
		l.results <- writeResult{id, l.store.AppendEvent(context.Background(), l.event(id, "run-"+id, "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	select {
	case <-won:
	case <-time.After(10 * time.Second):
		l.t.Fatalf("leader %s never won the token", id)
	}
	l.store.tapeGroup.afterToken.Store(nil)
}

func (l *lateHarness) startMember(ctx context.Context, id string, wantPending int) {
	l.t.Helper()
	go func() {
		l.results <- writeResult{id, l.store.AppendEvent(ctx, l.event(id, "run-"+id, "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	waitFor(l.t, "member "+id+" pending", 5*time.Second, func() bool { return l.pendingMembers() >= wantPending })
}

func (l *lateHarness) collect(n int) map[string]error {
	l.t.Helper()
	got := map[string]error{}
	for i := 0; i < n; i++ {
		select {
		case r := <-l.results:
			got[r.id] = r.err
		case <-time.After(30 * time.Second):
			l.t.Fatalf("only %d of %d results", len(got), n)
		}
	}
	return got
}

func (l *lateHarness) itemsOf(id string) (int, []string) {
	l.t.Helper()
	at := l.eventsHolding("loop.completed:" + id)
	if len(at) != 1 {
		return -1, nil
	}
	return at[0], l.shape()[at[0]].Items
}

// Invariants 1 and 5: a leader waiting on flushMu, two writers arriving in
// the meantime: all three commit in ONE event, in arrival order.
func TestGroupCommitLateTakeJoinsWritersArrivingBehindFlushMu(t *testing.T) {
	l := newLateHarness(t)
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("a")
	l.startMember(context.Background(), "b", 2)
	l.startMember(context.Background(), "c", 3)
	select {
	case r := <-l.results:
		t.Fatalf("%s returned while the seal held flushMu (err %v)", r.id, r.err)
	case <-time.After(50 * time.Millisecond):
	}
	l.releaseFlush()
	for id, err := range l.collect(3) {
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	at, items := l.itemsOf("a")
	if at < 0 || strings.Join(items, ",") != "loop.completed:a,loop.completed:b,loop.completed:c" {
		t.Fatalf("a, b and c are not one event in arrival order: event %d items %v", at, items)
	}
}

// Invariant 1 across groups: an earlier member is never reordered after a later
// one, even when the caps cut the late group short.
func TestGroupCommitLateTakeKeepsArrivalOrderAcrossCappedGroups(t *testing.T) {
	l := newLateHarness(t)
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("m0")
	const n = groupMaxOps + 20 // one op per member: the cap cuts the take
	for i := 1; i < n; i++ {
		l.startMember(context.Background(), fmt.Sprintf("m%d", i), i+1)
	}
	l.releaseFlush()
	for id, err := range l.collect(n) {
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	last := -1
	for i := 0; i < n; i++ {
		at := l.eventsHolding(fmt.Sprintf("loop.completed:m%d", i))
		if len(at) != 1 {
			t.Fatalf("m%d is in events %v, want exactly one", i, at)
		}
		if at[0] < last {
			t.Fatalf("m%d landed in event %d, before an earlier member's event %d", i, at[0], last)
		}
		last = at[0]
	}
	first, _ := l.itemsOf("m0")
	lastAt, _ := l.itemsOf(fmt.Sprintf("m%d", n-1))
	if first == lastAt {
		t.Fatal("all members shared one event: the cap did not split the group")
	}
}

// Invariant 3, before the take: a member cancelled while the leader waits is
// withdrawn, never written; the others are unaffected.
func TestGroupCommitLateTakeCancelBeforeTake(t *testing.T) {
	l := newLateHarness(t)
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("a")
	ctx, cancel := context.WithCancel(context.Background())
	l.startMember(ctx, "b", 2)
	l.startMember(context.Background(), "c", 3)
	cancel()
	r := <-l.results
	if r.id != "b" || !errors.Is(r.err, context.Canceled) {
		t.Fatalf("got %s %v, want b: context.Canceled", r.id, r.err)
	}
	l.releaseFlush()
	for id, err := range l.collect(2) {
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if got := l.eventsHolding("loop.completed:b"); len(got) != 0 {
		t.Fatalf("a member cancelled before the take reached the tape: %v", got)
	}
	if at, items := l.itemsOf("a"); at < 0 || strings.Join(items, ",") != "loop.completed:a,loop.completed:c" {
		t.Fatalf("a and c: event %d items %v, want one event of a then c", at, items)
	}
}

// Invariant 3, after the take: a member cancelled once a leader took it waits
// for the group's outcome and is reported committed, never cancelled.
func TestGroupCommitLateTakeCancelAfterTake(t *testing.T) {
	l := newLateHarness(t)
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("a")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.startMember(ctx, "b", 2)
	l.startMember(context.Background(), "c", 3)
	gate := l.holdNextCAS()
	l.releaseFlush()
	select {
	case <-gate.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the group's CAS never started")
	}
	cancel()
	select {
	case r := <-l.results:
		t.Fatalf("%s returned (%v) before the group's CAS was released: b, taken, must wait for the outcome", r.id, r.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gate.release)
	for id, err := range l.collect(3) {
		if err != nil {
			t.Fatalf("%s: %v (a taken member must be reported committed)", id, err)
		}
	}
	if at, items := l.itemsOf("a"); at < 0 || strings.Join(items, ",") != "loop.completed:a,loop.completed:b,loop.completed:c" {
		t.Fatalf("event %d items %v, want a, b, c in one event", at, items)
	}
}

// Invariant 4: a refused late group splits to singles in arrival order; the
// poison member alone fails.
func TestGroupCommitLateTakeRefusalSplit(t *testing.T) {
	l := newLateHarness(t)
	l.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if op.CanonicalID == "rm-poison" {
				return errors.New("poison refused by the dry run")
			}
		}
		return nil
	}
	before := l.store.TelemetryStats().GroupCommit
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("a")
	runs := distinctRunMemoryRuns(l.store, 2)
	for i, id := range []string{"rm-poison", "rm-ok"} {
		id, run := id, runs[i]
		go func() {
			_, err := l.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
			l.results <- writeResult{id, err}
		}()
		waitFor(t, id+" pending", 5*time.Second, func() bool { return l.pendingMembers() >= i+2 })
	}
	l.releaseFlush()
	got := l.collect(3)
	if got["a"] != nil || got["rm-ok"] != nil {
		t.Fatalf("results %v: only the poison may fail", got)
	}
	if !errors.Is(got["rm-poison"], computerevent.ErrBatchRefused) {
		t.Fatalf("poison: %v, want ErrBatchRefused", got["rm-poison"])
	}
	if len(l.eventsHolding("run_memory:rm-poison")) != 0 {
		t.Fatal("the refused entry reached the tape")
	}
	a, ok := l.eventsHolding("loop.completed:a"), l.eventsHolding("run_memory:rm-ok")
	if len(a) != 1 || len(ok) != 1 || a[0] >= ok[0] {
		t.Fatalf("a in %v, rm-ok in %v: want each once, a first", a, ok)
	}
	after := l.store.TelemetryStats().GroupCommit
	if after.RefusedSplits-before.RefusedSplits != 1 || after.Groups-before.Groups != 1 || after.Members-before.Members != 3 {
		t.Fatalf("before %+v after %+v: want one refused group of three members", before, after)
	}
}

// Invariants 2 and 6, and the replay half of the stress: many concurrent
// writers with telemetry timer seals racing, every write acknowledged once,
// and the tape replays to the live rows.
func TestGroupCommitLateTakeStressAckedOnceAndReplays(t *testing.T) {
	ctx := context.Background()
	live := openProjectStore(t)
	computerID := "computer-late-stress"
	genesis := prepareGenesisRequest(t, live, computerID, "genesis-late-stress")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	cas := &slowRecorder{tapeRecorder: tapeRecorder{liveTapeCAS: liveTapeCAS{signer: signer, projection: live}}, delay: 3 * time.Millisecond}
	cas.events = append(cas.events, genesis)
	reader := computerevent.NewMemoryArtifactReader()
	appender, err := computerevent.NewComputerEventAppender(computerID, payloadKeepingPinner{liveTapePinner{signer: signer}, reader}, live, cas, liveTapeVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	appender.SetPayloadResolver(reader, nil)
	if err := live.BindProjectionTape(computerID, appender); err != nil {
		t.Fatal(err)
	}
	if err := live.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: true, Window: 2 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	const workers, rounds = 48, 12
	var acks sync.Map
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			run := fmt.Sprintf("run-ls-%d", w)
			for i := 0; i < rounds; i++ {
				kind := types.EventRunProgress
				if i%2 == 1 {
					kind = types.EventRunCompleted
				}
				id := fmt.Sprintf("ls-%d-%d", w, i)
				rec := &types.EventRecord{EventID: id, RunID: run, OwnerID: "owner-1", Timestamp: telemetryBase, Kind: kind, Payload: json.RawMessage(`{}`)}
				if err := live.AppendEvent(ctx, rec); err != nil {
					t.Errorf("append %s: %v", id, err)
					return
				}
				if _, dup := acks.LoadOrStore(id, true); dup {
					t.Errorf("%s acknowledged twice", id)
				}
				entry, err := live.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{EntryID: "e-" + id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
				if err != nil {
					t.Errorf("run memory %s: %v", id, err)
					return
				}
				if _, dup := acks.LoadOrStore(entry.EntryID, true); dup {
					t.Errorf("%s acknowledged twice", entry.EntryID)
				}
			}
		}(w)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(120 * time.Second):
		t.Fatal("writers did not finish")
	}
	if err := live.DrainTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	st := live.TelemetryStats().GroupCommit
	if st.Members <= st.Groups || st.UnknownOutcome != 0 {
		t.Fatalf("group stats %+v: want shared events and no unknown outcome", st)
	}
	replayed := openProjectStore(t)
	if err := appender.ReconstructInto(ctx, replayed); err != nil {
		t.Fatalf("replay from zero: %v", err)
	}
	for w := 0; w < workers; w++ {
		run := fmt.Sprintf("run-ls-%d", w)
		a, err := live.ListEventsAfter(ctx, run, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		b, err := replayed.ListEventsAfter(ctx, run, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) != rounds || len(b) != rounds {
			t.Fatalf("%s: live %d events, replayed %d, want %d", run, len(a), len(b), rounds)
		}
		for i := range a {
			if a[i].EventID != b[i].EventID || a[i].Seq != b[i].Seq || a[i].StreamSeq != b[i].StreamSeq {
				t.Fatalf("%s event %d: live %+v replayed %+v", run, i, a[i], b[i])
			}
			if a[i].EventID != fmt.Sprintf("ls-%d-%d", w, i) {
				t.Fatalf("%s event %d is %s: a run's order was changed", run, i, a[i].EventID)
			}
		}
		ma, _ := live.ListRunMemoryEntries(ctx, "owner-1", run)
		mb, _ := replayed.ListRunMemoryEntries(ctx, "owner-1", run)
		if len(ma) != rounds || len(mb) != rounds {
			t.Fatalf("%s: live %d memory entries, replayed %d, want %d", run, len(ma), len(mb), rounds)
		}
		for i := range ma {
			if ma[i].EntryID != mb[i].EntryID || ma[i].Seq != mb[i].Seq || ma[i].EntryID != fmt.Sprintf("e-ls-%d-%d", w, i) {
				t.Fatalf("%s entry %d: live %+v replayed %+v", run, i, ma[i], mb[i])
			}
		}
	}
}
