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

// Failure modes F1-F15 of docs/design/tape-group-commit-2026-10-31.md, one
// test each where a test can show it. The cases were listed before the code.

// casGate holds the next CAS on the recording tape until release is closed,
// so writers that arrive meanwhile queue behind it, as they do behind a slow
// host append. entered is closed once the held CAS has started.
type casGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *tapeHarness) holdNextCAS() *casGate {
	h.t.Helper()
	g := &casGate{entered: make(chan struct{}), release: make(chan struct{})}
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error {
		held := false
		g.once.Do(func() { held = true })
		if held {
			close(g.entered)
			select {
			case <-g.release:
			case <-time.After(30 * time.Second):
			}
		}
		return nil
	}
	h.tape.mu.Unlock()
	return g
}

func (h *tapeHarness) pendingMembers() int {
	g := &h.store.tapeGroup
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.pending)
}

// warm writes one acknowledged event so the stream_seq lease and the boot
// watermark are already on the tape before a test shapes its own traffic.
func (h *tapeHarness) warm() {
	h.t.Helper()
	if err := h.store.AppendEvent(context.Background(), h.event("warm-"+fmt.Sprint(time.Now().UnixNano()), "run-warm", "owner-1", types.EventRunSubmitted, telemetryBase)); err != nil {
		h.t.Fatal(err)
	}
}

// eventsHolding returns the indexes of tape events holding item.
func (h *tapeHarness) eventsHolding(item string) []int {
	var out []int
	for i, e := range h.shape() {
		for _, it := range e.Items {
			if it == item {
				out = append(out, i)
			}
		}
	}
	return out
}

type writeResult struct {
	id  string
	err error
}

// startAcked starts an acknowledged event write on its own goroutine and
// waits until it is pending behind the held CAS.
func (h *tapeHarness) startAcked(results chan<- writeResult, id, run string, wantPending int) {
	h.t.Helper()
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event(id, run, "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{id, err}
	}()
	waitFor(h.t, "member "+id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= wantPending })
}

// F1 and the point of the change: writes that arrive while a commit is in
// flight share the next event, and none is answered before that event's CAS.
func TestGroupCommitMembersAckAfterSharedCommit(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	for i := 0; i < 5; i++ {
		h.startAcked(results, fmt.Sprintf("m%d", i), fmt.Sprintf("run-m%d", i), i+1)
	}
	select {
	case r := <-results:
		t.Fatalf("%s returned before the held CAS was released (err %v): acknowledged before durable", r.id, r.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gate.release)
	for i := 0; i < 6; i++ {
		if r := <-results; r.err != nil {
			t.Fatalf("%s: %v", r.id, r.err)
		}
	}
	shared := h.eventsHolding("loop.completed:m0")
	if len(shared) != 1 {
		t.Fatalf("m0 is in events %v, want exactly one", shared)
	}
	for i := 1; i < 5; i++ {
		if got := h.eventsHolding(fmt.Sprintf("loop.completed:m%d", i)); len(got) != 1 || got[0] != shared[0] {
			t.Fatalf("m%d is in events %v, want the shared event %d", i, got, shared[0])
		}
	}
	if lead := h.eventsHolding("loop.completed:lead"); len(lead) != 1 || lead[0] >= shared[0] {
		t.Fatalf("lead event %v, shared %d: the held write must land first and alone", lead, shared[0])
	}
	st := h.store.TelemetryStats().GroupCommit
	if st.Members < 6 || st.UnknownOutcome != 0 || st.RefusedSplits != 0 {
		t.Fatalf("group commit stats = %+v", st)
	}
	for _, ev := range []string{"m0", "m4"} {
		if got := h.storedEvents("run-" + ev); len(got) != 1 {
			t.Fatalf("run-%s stored %d events, want 1 (read after ack)", ev, len(got))
		}
	}
}

// distinctRunMemoryRuns returns n run ids whose run-memory stripes differ, so
// their appends can be pending at once.
func distinctRunMemoryRuns(s *Store, n int) []string {
	var out []string
	seen := map[any]bool{}
	for i := 0; len(out) < n; i++ {
		id := fmt.Sprintf("run-rm-%d", i)
		mu := s.runMemoryLock(id)
		if seen[mu] {
			continue
		}
		seen[mu] = true
		out = append(out, id)
	}
	return out
}

// F2: a refused group (nothing pinned) splits; each member gets its own
// answer, in arrival order; the refused one alone fails.
func TestGroupCommitRefusalSplitsToSingles(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	h.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if op.CanonicalID == "rm-poison" {
				return errors.New("poison refused by the dry run")
			}
		}
		return nil
	}
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	runs := distinctRunMemoryRuns(h.store, 3)
	for i, id := range []string{"rm-ok-1", "rm-poison", "rm-ok-2"} {
		id, run := id, runs[i]
		go func() {
			_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
			results <- writeResult{id, err}
		}()
		waitFor(t, id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+1 })
	}
	close(gate.release)
	got := map[string]error{}
	for i := 0; i < 4; i++ {
		r := <-results
		got[r.id] = r.err
	}
	if got["lead"] != nil || got["rm-ok-1"] != nil || got["rm-ok-2"] != nil {
		t.Fatalf("results %v: only the poison may fail", got)
	}
	if !errors.Is(got["rm-poison"], computerevent.ErrBatchRefused) {
		t.Fatalf("poison: %v, want ErrBatchRefused", got["rm-poison"])
	}
	if len(h.eventsHolding("run_memory:rm-poison")) != 0 {
		t.Fatal("the refused entry reached the tape")
	}
	a, b := h.eventsHolding("run_memory:rm-ok-1"), h.eventsHolding("run_memory:rm-ok-2")
	if len(a) != 1 || len(b) != 1 || a[0] >= b[0] {
		t.Fatalf("ok-1 in %v, ok-2 in %v: want each once, ok-1 first", a, b)
	}
	if st := h.store.TelemetryStats().GroupCommit; st.RefusedSplits != 1 {
		t.Fatalf("refused_splits = %d, want 1", st.RefusedSplits)
	}
}

// F3: an unknown outcome is never retried; every member is told.
func TestGroupCommitUnknownOutcomeIsNotRetried(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	for i := 0; i < 3; i++ {
		h.startAcked(results, fmt.Sprintf("u%d", i), fmt.Sprintf("run-u%d", i), i+1)
	}
	var mu sync.Mutex
	attempts := 0
	// The held lead CAS is already inside the gate's hook; every later CAS
	// (the group's) fails with an unknown outcome.
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		return errors.New("cas outcome unknown")
	}
	h.tape.mu.Unlock()
	close(gate.release)
	for i := 0; i < 4; i++ {
		r := <-results
		if r.id == "lead" {
			if r.err != nil {
				t.Fatalf("lead: %v", r.err)
			}
			continue
		}
		if r.err == nil || !strings.Contains(r.err.Error(), "cas outcome unknown") {
			t.Fatalf("%s: %v, want the shared CAS error", r.id, r.err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	// The store does not retry; the appender re-sends the SAME request once
	// when an outcome is unknown (idempotent at the host), so two calls reach
	// the CAS and still only one tape event could result.
	if attempts != 2 {
		t.Fatalf("CAS attempts after the lead = %d, want 2 (the group once, plus the appender's same-request re-send)", attempts)
	}
	if st := h.store.TelemetryStats().GroupCommit; st.UnknownOutcome != 1 {
		t.Fatalf("unknown_outcome = %d, want 1", st.UnknownOutcome)
	}
}

// F4: only unconditional upserts group; every guarded or unknown kind goes
// direct.
func TestGroupCommitOnlyUnconditionalOpsGroup(t *testing.T) {
	yes := []string{computerevent.ProjectionOpObject, computerevent.ProjectionOpObjectEdge, computerevent.ProjectionOpRunMemoryEntry}
	no := []string{
		computerevent.ProjectionOpDesktopState, computerevent.ProjectionOpSelfDevelopmentStartIntent,
		computerevent.ProjectionOpSelfDevelopmentOperation, computerevent.ProjectionOpTextureAgentMutation,
		computerevent.ProjectionOpTextureDocumentAlias, computerevent.ProjectionOpTextureDocumentAliasDelete,
		computerevent.ProjectionOpMediaProgress, computerevent.ProjectionOpMediaRecent, computerevent.ProjectionOpMediaRecentDelete,
		computerevent.ProjectionOpUserPreference, computerevent.ProjectionOpObjectRetire, computerevent.ProjectionOpRunMemoryRetire,
		"lifecycle_run_recorded", "made_up_kind",
	}
	for _, k := range yes {
		if !groupableOps([]computerevent.ProjectionOp{{Kind: k}}) {
			t.Errorf("%s does not group, want it to", k)
		}
	}
	for _, k := range no {
		if groupableOps([]computerevent.ProjectionOp{{Kind: k}}) {
			t.Errorf("%s groups; a guarded or unknown op must go direct", k)
		}
		if groupableOps([]computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObject}, {Kind: k}}) {
			t.Errorf("a write mixing %s with an object op groups", k)
		}
	}
	if groupableOps(nil) {
		t.Error("an empty write groups")
	}
}

// F5: pending telemetry rides ahead of the members in the same event; a queue
// longer than one batch seals its older chunks first.
func TestGroupCommitTelemetryRidesAheadOfMembers(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	h.warm()
	ctx := context.Background()
	for _, id := range []string{"t1", "t2"} {
		if err := h.store.AppendEvent(ctx, h.event(id, "run-r", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
			t.Fatal(err)
		}
	}
	if d := sl.depth(); d != 2 {
		t.Fatalf("queue depth = %d, want 2", d)
	}
	if err := h.store.AppendEvent(ctx, h.event("a1", "run-r", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	shape := h.shape()
	last := shape[len(shape)-1]
	if last.Authority != ordinaryAuthority || strings.Join(last.Items, ",") != "loop.progress:t1,loop.progress:t2,loop.completed:a1" {
		t.Fatalf("last event = %+v, want t1,t2 riding ahead of a1 under the ordinary authority", last)
	}
	for _, e := range shape {
		if e.Authority == telemetryAuthority {
			t.Fatalf("telemetry sealed as its own event: %+v", e)
		}
	}
	if d := sl.depth(); d != 0 {
		t.Fatalf("queue depth after the ride = %d", d)
	}
	b := h.store.TelemetryStats().Batching
	if b.SealReasons["ride"] != 1 || b.RecordsDropped != 0 {
		t.Fatalf("seal reasons %v dropped %d, want one ride", b.SealReasons, b.RecordsDropped)
	}

	// 70 queued: the first 64 are sealed on their own first, the last 6 ride.
	for i := 0; i < 70; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("q%d", i), "run-q", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
			t.Fatal(err)
		}
	}
	before := len(h.shape())
	if err := h.store.AppendEvent(ctx, h.event("a2", "run-q", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	tail := h.shape()[before:]
	var lens []int
	for _, e := range tail {
		lens = append(lens, len(e.Items))
	}
	if len(tail) < 2 || tail[len(tail)-1].Items[len(tail[len(tail)-1].Items)-1] != "loop.completed:a2" {
		t.Fatalf("tail sizes %v: a2 must close the last event", lens)
	}
	seen := 0
	for _, e := range tail {
		for _, it := range e.Items {
			if strings.HasPrefix(it, "loop.progress:q") {
				if it != fmt.Sprintf("loop.progress:q%d", seen) {
					t.Fatalf("telemetry out of order: got %s at position %d", it, seen)
				}
				seen++
			}
		}
	}
	if seen != 70 {
		t.Fatalf("%d of 70 telemetry records on the tape", seen)
	}
}

// F6: a write issued after another's ack lands in a later event.
func TestGroupCommitCausalOrderAcrossGoroutines(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 4)
	go func() {
		results <- writeResult{"lead", h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	<-gate.entered
	h.startAcked(results, "cause", "run-cause", 1)
	close(gate.release)
	for i := 0; i < 2; i++ {
		if r := <-results; r.err != nil {
			t.Fatal(r.err)
		}
	}
	if err := h.store.AppendEvent(context.Background(), h.event("effect", "run-effect", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	c, e := h.eventsHolding("loop.completed:cause"), h.eventsHolding("loop.completed:effect")
	if len(c) != 1 || len(e) != 1 || c[0] >= e[0] {
		t.Fatalf("cause in %v, effect in %v: the effect must land in a later event", c, e)
	}
}

// F7: no deadlock between leaders, the sealer's deadline flush, barriers of
// direct writers and the capacity flush, under load with the race detector;
// every write lands once and no number is issued twice.
func TestGroupCommitNoDeadlockUnderLoad(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(5 * time.Millisecond)
	ctx := context.Background()
	const workers, each = 32, 30
	n := newIssued()
	var mu sync.Mutex
	var wg sync.WaitGroup
	done := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			run := fmt.Sprintf("run-load-%d", w)
			for i := 0; i < each; i++ {
				switch i % 3 {
				case 0, 1:
					kind := types.EventRunProgress
					if i%3 == 1 {
						kind = types.EventRunStarted
					}
					rec := h.event(fmt.Sprintf("ld-%d-%d", w, i), run, "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Millisecond))
					if err := h.store.AppendEvent(ctx, rec); err != nil {
						t.Errorf("append: %v", err)
						return
					}
					mu.Lock()
					n.note(t, rec)
					mu.Unlock()
				case 2:
					if _, err := h.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)}); err != nil {
						t.Errorf("run memory: %v", err)
						return
					}
				}
			}
		}(w)
	}
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Fatal("deadlock: writers did not finish")
	}
	sl.drain()
	writes := 0
	for w := 0; w < workers; w++ {
		run := fmt.Sprintf("run-load-%d", w)
		if got := len(h.storedEvents(run)); got != 20 {
			t.Fatalf("%s stored %d events, want 20", run, got)
		}
		entries, err := h.store.ListRunMemoryEntries(ctx, "owner-1", run)
		if err != nil || len(entries) != 10 {
			t.Fatalf("%s run memory %d entries (%v), want 10", run, len(entries), err)
		}
		for i, e := range entries {
			if e.Seq != int64(i+1) {
				t.Fatalf("%s entry %d has seq %d: run memory seq must stay dense per run", run, i, e.Seq)
			}
		}
		writes += 30
	}
	st := h.store.TelemetryStats()
	if st.Batching.RecordsDropped != 0 || st.GroupCommit.UnknownOutcome != 0 {
		t.Fatalf("lost records: %+v %+v", st.Batching, st.GroupCommit)
	}
	t.Logf("%d writes -> %d tape events; groups %d, mean members %.2f, telemetry ridden %d",
		writes, len(h.tape.snapshot()), st.GroupCommit.Groups, st.GroupCommit.MeanMembers, st.GroupCommit.TelemetryRidden)
}

// F8: a tape with grouped events replays to the same rows.
func TestGroupCommitReplayMatchesLive(t *testing.T) {
	ctx := context.Background()
	live := openProjectStore(t)
	computerID := "computer-group-replay"
	genesis := prepareGenesisRequest(t, live, computerID, "genesis-group-replay")
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}
	cas := &slowRecorder{tapeRecorder: tapeRecorder{liveTapeCAS: liveTapeCAS{signer: signer, projection: live}}, delay: 20 * time.Millisecond}
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
	if err := live.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: true, Window: time.Second}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	const workers = 8
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			run := fmt.Sprintf("run-rp-%d", w)
			for i := 0; i < 6; i++ {
				kind := types.EventRunProgress
				if i%2 == 1 {
					kind = types.EventRunCompleted
				}
				rec := &types.EventRecord{EventID: fmt.Sprintf("rp-%d-%d", w, i), RunID: run, OwnerID: "owner-1", Timestamp: telemetryBase, Kind: kind, Payload: json.RawMessage(`{}`)}
				if err := live.AppendEvent(ctx, rec); err != nil {
					t.Errorf("append: %v", err)
					return
				}
				if _, err := live.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)}); err != nil {
					t.Errorf("run memory: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if err := live.DrainTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	st := live.TelemetryStats().GroupCommit
	if st.Members <= st.Groups {
		t.Fatalf("no write shared an event (groups %d, members %d): the replay proves nothing", st.Groups, st.Members)
	}
	replayed := openProjectStore(t)
	if err := appender.ReconstructInto(ctx, replayed); err != nil {
		t.Fatalf("replay from zero: %v", err)
	}
	for w := 0; w < workers; w++ {
		run := fmt.Sprintf("run-rp-%d", w)
		a, err := live.ListEventsAfter(ctx, run, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		b, err := replayed.ListEventsAfter(ctx, run, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) != 6 || len(b) != len(a) {
			t.Fatalf("%s: live %d events, replayed %d", run, len(a), len(b))
		}
		for i := range a {
			if a[i].EventID != b[i].EventID || a[i].Seq != b[i].Seq || a[i].StreamSeq != b[i].StreamSeq {
				t.Fatalf("%s event %d: live %+v replayed %+v", run, i, a[i], b[i])
			}
		}
		ma, _ := live.ListRunMemoryEntries(ctx, "owner-1", run)
		mb, _ := replayed.ListRunMemoryEntries(ctx, "owner-1", run)
		if len(ma) != 6 || len(mb) != len(ma) {
			t.Fatalf("%s: live %d memory entries, replayed %d", run, len(ma), len(mb))
		}
		for i := range ma {
			if ma[i].EntryID != mb[i].EntryID || ma[i].Seq != mb[i].Seq || ma[i].ParentEntryID != mb[i].ParentEntryID {
				t.Fatalf("%s entry %d: live %+v replayed %+v", run, i, ma[i], mb[i])
			}
		}
	}
}

// slowRecorder is a tapeRecorder whose CAS takes delay, like a host append,
// so concurrent writers overlap.
type slowRecorder struct {
	tapeRecorder
	delay time.Duration
}

func (c *slowRecorder) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	time.Sleep(c.delay)
	return c.tapeRecorder.CompareAndSwap(ctx, request)
}

// F9: a group is capped; a member over the byte cap goes alone; pending
// members are taken oldest first.
func TestGroupCommitCapsSplitGroups(t *testing.T) {
	var g groupCommit
	small := func() *groupMember {
		return &groupMember{ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObject}}, bytes: 100, done: make(chan struct{})}
	}
	big := &groupMember{ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObject}}, bytes: groupMaxBytes + 1, done: make(chan struct{})}
	a, c := small(), small()
	g.pending = []*groupMember{a, big, c}
	if got := g.takeGroup(); len(got) != 1 || got[0] != a {
		t.Fatalf("first group %v, want [a]: the big member must not join", got)
	}
	if got := g.takeGroup(); len(got) != 1 || got[0] != big {
		t.Fatalf("second group %v, want [big] alone", got)
	}
	if got := g.takeGroup(); len(got) != 1 || got[0] != c {
		t.Fatalf("third group %v, want [c]", got)
	}
	g.pending = nil
	for i := 0; i < groupMaxOps+10; i++ {
		g.pending = append(g.pending, small())
	}
	if got := g.takeGroup(); len(got) != groupMaxOps {
		t.Fatalf("group of %d single-op members, want the cap %d", len(got), groupMaxOps)
	}
	if got := g.takeGroup(); len(got) != 10 {
		t.Fatalf("remainder %d, want 10", len(got))
	}
}

// F10: a caller cancelled before a leader took its write gets ctx.Err() and
// nothing of it reaches the tape.
func TestGroupCommitCancelledBeforeTakenWritesNothing(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 2)
	go func() {
		results <- writeResult{"lead", h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	<-gate.entered
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		results <- writeResult{"gone", h.store.AppendEvent(ctx, h.event("gone", "run-gone", "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	waitFor(t, "gone pending", 5*time.Second, func() bool { return h.pendingMembers() == 1 })
	cancel()
	r := <-results
	if r.id != "gone" || !errors.Is(r.err, context.Canceled) {
		t.Fatalf("got %s %v, want gone: context.Canceled", r.id, r.err)
	}
	close(gate.release)
	if r := <-results; r.err != nil {
		t.Fatal(r.err)
	}
	if len(h.eventsHolding("loop.completed:gone")) != 0 {
		t.Fatal("a cancelled, untaken write reached the tape")
	}
	if st := h.store.TelemetryStats().GroupCommit; st.Cancelled != 1 {
		t.Fatalf("cancelled = %d, want 1", st.Cancelled)
	}
}
