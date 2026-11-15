package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Fault-injection tests for the append path
// (docs/problems/event-appender-wedges-clustering-assessment-2026-11-12.md,
// section 4). The paper stopped for four hours on 2026-11-12, and on
// 2026-11-02, because a CAS error discarded the prepared row of an append the
// host had already committed. Case names follow the assessment's table.

var (
	errReset      = errors.New("read tcp 10.0.0.2:41234->10.0.0.1:8081: connection reset by peer")
	errServer503  = errors.New("computer event client: corpusd returned 503: Service Unavailable")
	errPageFailed = errors.New("injected replay page failure")
)

type faultRig struct {
	t        *testing.T
	signer   SigningKey
	verifier EventHeadReceiptVerifier
	cas      *memoryCAS
	proj     *memoryProjection
	app      *ComputerEventAppender
}

func newFaultRig(t *testing.T) *faultRig {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	rig := &faultRig{
		t:        t,
		signer:   signer,
		verifier: EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}},
		cas:      &memoryCAS{signer: signer},
		proj:     &memoryProjection{},
	}
	rig.app = rig.newAppender(rig.proj)
	return rig
}

func (r *faultRig) newAppender(projection ProjectionStore) *ComputerEventAppender {
	r.t.Helper()
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: r.signer}, projection, r.cas, r.verifier)
	if err != nil {
		r.t.Fatal(err)
	}
	return appender
}

// boot builds a new appender over the SAME stores and runs the boot sequence
// (replay mode, Reconstruct), as the autoputer does.
func (r *faultRig) boot(projection ProjectionStore) *ComputerEventAppender {
	r.t.Helper()
	appender := r.newAppender(projection)
	appender.SetReplayMode(true)
	err := appender.Reconstruct(context.Background(), r.cas)
	appender.SetReplayMode(false)
	if err != nil {
		r.t.Fatalf("boot reconstruct: %v", err)
	}
	return appender
}

func (r *faultRig) genesis(appender *ComputerEventAppender) {
	r.t.Helper()
	event := testEvent(r.t, nil, EventGenesisImported)
	event.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), event, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		r.t.Fatalf("genesis: %v", err)
	}
}

func (r *faultRig) appendOne(appender *ComputerEventAppender) (Receipt, error) {
	r.t.Helper()
	return appender.AppendNew(context.Background(), testEvent(r.t, nil, EventArtifactProduced), TransitionInput{}, nil)
}

// hostAhead advances the host chain by k events through a second appender (a
// writer whose own projection is current), leaving rig.proj behind.
func (r *faultRig) hostAhead(k int) {
	r.t.Helper()
	other := r.boot(&memoryProjection{})
	for i := 0; i < k; i++ {
		if _, err := r.appendOne(other); err != nil {
			r.t.Fatalf("host-side append %d: %v", i, err)
		}
	}
}

func (r *faultRig) requireAligned(label string, wantSeq uint64) {
	r.t.Helper()
	if r.cas.head == nil || r.proj.head == nil || *r.cas.head != *r.proj.head {
		r.t.Fatalf("%s: heads differ: host=%s local=%s", label, replayHeadSummary(r.cas.head), replayHeadSummary(r.proj.head))
	}
	if r.cas.head.Sequence != wantSeq {
		r.t.Fatalf("%s: sequence=%d, want %d", label, r.cas.head.Sequence, wantSeq)
	}
	if len(r.proj.prepared) != 0 {
		r.t.Fatalf("%s: %d prepared rows left", label, len(r.proj.prepared))
	}
}

// F1, F1b, F1c: the host commits, the reply is lost. The caller's append must
// leave local == host with no prepared row, with no later pass needed.
func TestFaultCASCommitsThenReplyLost(t *testing.T) {
	cases := map[string]error{
		"F1 transport reset":      errReset,
		"F1b caller ctx canceled": context.Canceled,
		"F1c 503 after commit":    errServer503,
	}
	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			rig := newFaultRig(t)
			rig.genesis(rig.app)
			rig.cas.script = []casStep{{commit: true, err: cause}}
			_, appendErr := rig.appendOne(rig.app)
			if rig.cas.head == nil || rig.cas.head.Sequence != 2 {
				t.Fatalf("fixture: host did not commit (append err=%v)", appendErr)
			}
			rig.requireAligned("after the append that lost its reply", 2)
			if _, err := rig.appendOne(rig.app); err != nil {
				t.Fatalf("append after a lost reply: %v", err)
			}
			rig.requireAligned("after the next append", 3)
		})
	}
}

// F2: a definite refusal commits nothing and is not re-sent.
func TestFaultCASDefiniteRefusalDiscards(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{err: fmt.Errorf("%w: expected head changed", ErrCASConflict)}}
	before := rig.cas.casCalls
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("a refused CAS reported success")
	}
	if got := rig.cas.casCalls - before; got != 1 {
		t.Fatalf("a definite refusal was sent %d times, want 1", got)
	}
	rig.requireAligned("after a definite refusal", 1)
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("append after a definite refusal: %v", err)
	}
	rig.requireAligned("after the next append", 2)
}

// F2b: an unclear error where the host never saw the request. Either outcome
// (discarded, or committed by the re-send) is fine; a stuck row or a split is
// not.
func TestFaultCASUnclearErrorHostNeverSawIt(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{err: errReset}}
	_, _ = rig.appendOne(rig.app)
	if len(rig.proj.prepared) != 0 {
		t.Fatalf("a prepared row is stuck: %d", len(rig.proj.prepared))
	}
	if *rig.cas.head != *rig.proj.head {
		t.Fatalf("heads split: host=%s local=%s", replayHeadSummary(rig.cas.head), replayHeadSummary(rig.proj.head))
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the next append", rig.cas.head.Sequence)
}

// F3: an unclear error while the original request is still in flight at the
// host, which commits it only after the guest's call returned. The re-send
// settles it (the host answers a request it already holds, or commits it
// once); the late arrival of the original must not add a row. F3b below is
// the case where the guest never learns the outcome before the commit lands.
func TestFaultCASLateCommitIsNotDiscarded(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{late: true, err: errReset}}
	_, _ = rig.appendOne(rig.app)
	rig.cas.landLate()
	if rig.cas.head.Sequence != 2 || len(rig.cas.records) != 2 {
		t.Fatalf("host chain: seq=%d records=%d, want 2/2 (a re-send must not add a row)", rig.cas.head.Sequence, len(rig.cas.records))
	}
	rig.requireAligned("after the late commit landed", 2)
}

// F3b: the re-send is also unclear and the host commits only afterwards. The
// prepared row must survive until the commit can be reconciled.
func TestFaultCASLateCommitAfterTwoUnclearErrors(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{late: true, err: errReset}, {late: true, err: errReset}}
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append with an unknown outcome reported success")
	}
	if len(rig.proj.prepared) != 1 {
		t.Fatalf("prepared rows=%d, want the one row kept while the outcome is unknown", len(rig.proj.prepared))
	}
	rig.cas.landLate()
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append after the late commit: %v", err)
	}
	rig.requireAligned("after the next append", 3)
}

// F4: a definite refusal whose discard also fails leaves a stuck row; the next
// append clears it (the 11-02 11:42Z mechanism).
func TestFaultDefiniteRefusalAndDiscardFails(t *testing.T) {
	rig := newFaultRig(t)
	rig.proj.uniqueSequence = true
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{err: fmt.Errorf("%w: expected head changed", ErrCASConflict)}}
	rig.proj.failDiscardN = 1
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("a refused CAS reported success")
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("append behind a stuck row: %v", err)
	}
	rig.requireAligned("after the pass", 2)
}

// F5: the host commits, the reply is lost and the discard fails.
func TestFaultCommitReplyLostAndDiscardFails(t *testing.T) {
	rig := newFaultRig(t)
	rig.proj.uniqueSequence = true
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{commit: true, err: errReset}}
	rig.proj.failDiscardN = 1
	_, _ = rig.appendOne(rig.app)
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the pass", 3)
}

type failingVerifier struct {
	inner ReceiptVerifier
	fails int
}

func (v *failingVerifier) VerifyEventHeadReceipt(ctx context.Context, receipt Receipt, request CASRequest) error {
	if v.fails > 0 {
		v.fails--
		return errors.New("receipt key unknown")
	}
	return v.inner.VerifyEventHeadReceipt(ctx, receipt, request)
}

// F10: the receipt does not verify after the host committed. Nothing is
// discarded and the head does not advance on an unverified receipt; the next
// pass converges once the key resolves.
func TestFaultVerifyFailsAfterCommit(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.app.verifier = &failingVerifier{inner: rig.verifier, fails: 1}
	discardsBefore := rig.proj.discards
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append with an unverifiable receipt reported success")
	}
	if rig.proj.discards != discardsBefore {
		t.Fatal("a committed prepared row was discarded after a verify failure")
	}
	if rig.proj.head.Sequence != 1 || rig.cas.head.Sequence != 2 || len(rig.proj.prepared) != 1 {
		t.Fatalf("state: local=%d host=%d prepared=%d, want 1/2/1", rig.proj.head.Sequence, rig.cas.head.Sequence, len(rig.proj.prepared))
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the pass", 3)
}

// F22: the re-send is unclear and the host's head cannot be read either. The
// row is kept and the error returned; nothing is discarded.
func TestFaultHeadProbeFailsAfterUnclearCAS(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{commit: true, headDown: true, err: errReset}, {err: errReset}}
	discardsBefore := rig.proj.discards
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append with an unknown outcome reported success")
	}
	if rig.proj.discards != discardsBefore {
		t.Fatal("the prepared row was discarded although the host's state is unknown")
	}
	if len(rig.proj.prepared) != 1 {
		t.Fatalf("prepared rows=%d, want the one row kept", len(rig.proj.prepared))
	}
	rig.cas.mu.Lock()
	rig.cas.headErr = nil
	rig.cas.mu.Unlock()
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the pass", 3)
}

// F15: the host is ahead by k events and no prepared row exists (a wedge that
// already happened). One pass must catch the local projection up in place.
func TestFaultCatchUpInPlace(t *testing.T) {
	for name, k := range map[string]int{"k=1": 1, "k=3": 3, "k=page+1": EventReplayPageSize + 1} {
		t.Run(name, func(t *testing.T) {
			rig := newFaultRig(t)
			rig.genesis(rig.app)
			rig.hostAhead(k)
			if _, err := rig.appendOne(rig.app); err != nil {
				t.Fatalf("append with the host %d events ahead: %v", k, err)
			}
			rig.requireAligned("after one pass", uint64(1+k+1))
			if st := rig.app.WedgeStatus(); st.Active {
				t.Fatalf("wedge still active: %+v", st)
			}
		})
	}
}

// F15 control: a boot repairs the same state, so the in-place result must be
// the boot's result.
func TestFaultBootRepairsTheSameWedge(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(3)
	rig.boot(rig.proj)
	rig.requireAligned("after a boot", 4)
}

// F16: the catch-up fails halfway. Progress is kept, nothing is corrupted, and
// the next pass resumes.
func TestFaultCatchUpFailsHalfwayThenResumes(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(5) // host seq 6
	failed := false
	rig.proj.finalizeHook = func(sequence uint64) error {
		if sequence == 4 && !failed {
			failed = true
			return errors.New("injected projection failure")
		}
		return nil
	}
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append reported success while the catch-up failed")
	}
	if rig.proj.head.Sequence != 3 {
		t.Fatalf("local seq=%d, want 3 (the last event applied before the failure)", rig.proj.head.Sequence)
	}
	if len(rig.proj.prepared) != 0 {
		t.Fatalf("the failed catch-up left %d prepared rows", len(rig.proj.prepared))
	}
	if !rig.app.WedgeStatus().Active {
		t.Fatal("the wedge was hidden")
	}
	rig.app.recoveryFailedAt = time.Time{} // the 30 s cooldown
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("the next pass did not resume: %v", err)
	}
	rig.requireAligned("after resuming", 7)
}

// F16b: the page fetch fails after a full page was applied.
func TestFaultCatchUpPageFetchFailsThenResumes(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(EventReplayPageSize + 1)
	fail := true
	rig.cas.onEventsPage = func(_ context.Context, after uint64) error {
		if after > 1 && fail {
			fail = false
			return errPageFailed
		}
		return nil
	}
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append reported success while the page fetch failed")
	}
	if rig.proj.head.Sequence != uint64(1+EventReplayPageSize) {
		t.Fatalf("local seq=%d, want %d (the first page kept)", rig.proj.head.Sequence, 1+EventReplayPageSize)
	}
	rig.app.recoveryFailedAt = time.Time{}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("the next pass did not resume: %v", err)
	}
	rig.requireAligned("after resuming", uint64(1+EventReplayPageSize+1+1))
}

// F17: the host's chain does not extend the local head. Never auto-heal a fork.
func TestFaultCatchUpRefusesADivergentChain(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	// A different genesis on the host: the same sequence, another digest.
	rig.cas.mu.Lock()
	rig.cas.head, rig.cas.records, rig.cas.receipts = nil, nil, nil
	rig.cas.mu.Unlock()
	writer := rig.newAppender(&memoryProjection{})
	rig.genesis(writer)
	for i := 0; i < 2; i++ {
		if _, err := rig.appendOne(writer); err != nil {
			t.Fatal(err)
		}
	}
	localBefore := *rig.proj.head
	hostBefore := *rig.cas.head
	if _, err := rig.appendOne(rig.app); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append on a forked chain = %v, want projection repair", err)
	}
	if *rig.proj.head != localBefore || *rig.cas.head != hostBefore || len(rig.proj.prepared) != 0 {
		t.Fatalf("a refused fork changed state: local=%s host=%s prepared=%d", replayHeadSummary(rig.proj.head), replayHeadSummary(rig.cas.head), len(rig.proj.prepared))
	}
	if st := rig.app.WedgeStatus(); !st.Active || st.LastError != "divergent chain" {
		t.Fatalf("wedge=%+v, want active with kind %q", st, "divergent chain")
	}
}

// F18: the local head is ahead of the host. No catch-up, no change.
func TestFaultLocalAheadIsRefusedInPlace(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatal(err)
	}
	// The host rolls back to sequence 1.
	rig.cas.mu.Lock()
	rig.cas.head = cloneHead(&rig.cas.records[0].Request.Next)
	rig.cas.records = rig.cas.records[:1]
	rig.cas.mu.Unlock()
	pagesBefore := rig.cas.pageCalls
	localBefore := *rig.proj.head
	if _, err := rig.appendOne(rig.app); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append with the local head ahead = %v, want projection repair", err)
	}
	if rig.cas.pageCalls != pagesBefore {
		t.Fatal("a catch-up fetched events for a local head that is ahead")
	}
	if *rig.proj.head != localBefore || rig.cas.head.Sequence != 1 {
		t.Fatalf("state changed: local=%s host=%s", replayHeadSummary(rig.proj.head), replayHeadSummary(rig.cas.head))
	}
}

// F19: a live append that arrives while a catch-up runs waits for it and binds
// to the head the catch-up reached.
func TestFaultAppendWaitsForCatchUp(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(2) // host seq 3
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	rig.cas.onEventsPage = func(context.Context, uint64) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}
	results := make(chan error, 2)
	go func() { _, err := rig.appendOne(rig.app); results <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the catch-up never fetched events")
	}
	go func() { _, err := rig.appendOne(rig.app); results <- err }()
	select {
	case err := <-results:
		t.Fatalf("an append finished while the catch-up held the appender: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("append %d: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("appends did not finish after the catch-up")
		}
	}
	rig.requireAligned("after both appends", 5)
}
