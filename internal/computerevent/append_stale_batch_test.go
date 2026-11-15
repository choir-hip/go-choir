package computerevent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// A batch is numbered and dry-run against local state; under the appender
// mutex the in-place recovery or catch-up can then apply events the batch has
// not seen, and the stale batch used to go on to the CAS (a duplicate
// (run, seq) on the canonical tape, then a finalize wedge). These tests model
// that with a projection whose dry run refuses a batch numbered at a local
// head other than the current one. Review rounds 1 and 2 of lane/s5-coalesce.

// stalenessProjection is a batch projection over memoryProjection. A batch's
// single op body names the local head sequence it was numbered at; the dry run
// refuses it if the local head has moved since.
type stalenessProjection struct {
	*memoryProjection
	dryRuns int
	// onPrepare, when set, runs once at the next Prepare, which then fails.
	onPrepare func()
}

func (p *stalenessProjection) Prepare(ctx context.Context, request CASRequest) error {
	if hook := p.onPrepare; hook != nil {
		p.onPrepare = nil
		hook()
		return errors.New("injected prepare failure")
	}
	return p.memoryProjection.Prepare(ctx, request)
}

func (p *stalenessProjection) DryRunProjectionBatch(_ context.Context, _ string, batch *ProjectionBatch) error {
	p.dryRuns++
	var numberedAt uint64
	for _, op := range batch.Ops {
		var body struct {
			NumberedAt uint64 `json:"numbered_at"`
			Any        bool   `json:"any"`
		}
		_ = json.Unmarshal(op.Body, &body)
		if body.Any {
			return nil
		}
		numberedAt = body.NumberedAt
	}
	if numberedAt != headSeq(p.memoryProjection.head) {
		return errors.New("stale numbering: the local head moved since the batch was numbered")
	}
	return nil
}

func (p *stalenessProjection) DryRunProjectionBatchAt(ctx context.Context, computerID string, batch *ProjectionBatch) (uint64, error) {
	err := p.DryRunProjectionBatch(ctx, computerID, batch)
	return headSeq(p.memoryProjection.head), err
}

func (p *stalenessProjection) FinalizeBatch(ctx context.Context, computerID, digest string, receipt Receipt, _ *ProjectionBatch) error {
	return p.memoryProjection.Finalize(ctx, computerID, digest, receipt)
}

type staleRig struct {
	*faultRig
	proj2 *stalenessProjection
}

func newStaleRig(t *testing.T) *staleRig {
	t.Helper()
	rig := newFaultRig(t)
	sp := &stalenessProjection{memoryProjection: rig.proj}
	rig.app = rig.newAppender(sp)
	rig.genesis(rig.app)
	return &staleRig{faultRig: rig, proj2: sp}
}

// batchAt is a projection batch numbered at the local head as it is now.
func (r *staleRig) batchAt(eventID string) []byte {
	raw, err := json.Marshal(ProjectionBatch{
		Version: ProjectionBatchV2, ProjectorVersion: ProjectorVersionV2,
		ComputerID: testComputerID, EventID: eventID,
		Ops: []ProjectionOp{{Kind: ProjectionOpDesktopState, Body: json.RawMessage(`{"numbered_at":` + itoa(headSeq(r.proj.head)) + `}`)}},
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return raw
}

func itoa(n uint64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func (r *staleRig) appendBatch() (Receipt, error) {
	r.t.Helper()
	event := testEvent(r.t, nil, EventProjectionBatchRecorded)
	_, _, err := r.app.AppendNewPayload(context.Background(), event, TransitionInput{}, r.batchAt(event.EventID), ProjectionBatchMediaType, "owner")
	return Receipt{}, err
}

// landUnacknowledged leaves the host with one committed batch event the local
// projection has not applied, and its prepared row kept: the first send
// committed, both replies were lost.
func (r *staleRig) landUnacknowledged() {
	r.t.Helper()
	r.cas.script = []casStep{{commit: true, err: errReset}, {err: errReset}}
	if _, err := r.appendBatch(); err == nil {
		r.t.Fatal("fixture: the append with a lost reply reported success")
	}
	if len(r.proj.prepared) != 1 || r.cas.head.Sequence != 2 || r.proj.head.Sequence != 1 {
		r.t.Fatalf("fixture: prepared=%d host=%d local=%d, want 1, 2, 1", len(r.proj.prepared), r.cas.head.Sequence, r.proj.head.Sequence)
	}
}

// U1: the recovery in the append path finalizes the landed row. The batch,
// numbered before it, is re-validated under the mutex and refused: nothing of
// it reaches the host.
func TestAppendRevalidatesTheBatchAfterInPlaceRecovery(t *testing.T) {
	rig := newStaleRig(t)
	rig.landUnacknowledged()
	records := len(rig.cas.records)
	_, err := rig.appendBatch()
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("a batch numbered before the recovery applied a landed row returned %v, want ErrBatchRefused", err)
	}
	if got := len(rig.cas.records); got != records {
		t.Fatalf("the host holds %d events, want %d: the stale batch reached the CAS", got, records)
	}
	rig.requireAligned("after the refusal", 2)
	// The caller's next attempt numbers from the caught-up state and lands.
	if _, err := rig.appendBatch(); err != nil {
		t.Fatalf("the next attempt: %v", err)
	}
	rig.requireAligned("after the next attempt", 3)
}

// U2 (B1): the re-send of the committed row is refused, so the row is
// discarded and its application is left to the catch-up; the catch-up fails
// once. Nothing is left to settle by looking at prepared rows. The next write must still be
// refused, not committed over the landed entry, and must not wedge the tape.
func TestAppendAfterADiscardedRowAndAFailedCatchUpDoesNotCommitAStaleBatch(t *testing.T) {
	rig := newStaleRig(t)
	rig.landUnacknowledged()
	records := len(rig.cas.records)

	rig.cas.script = []casStep{{err: errReset}} // the recovery's re-send of the committed row is refused
	failed := true
	rig.cas.onEventsPage = func(context.Context, uint64) error {
		if failed {
			failed = false
			return errPageFailed
		}
		return nil
	}
	if _, err := rig.appendBatch(); err == nil {
		t.Fatal("a write whose catch-up failed reported success")
	}
	if len(rig.proj.prepared) != 0 {
		t.Fatalf("the discarded row is still prepared: %d", len(rig.proj.prepared))
	}
	rig.app.recoveryFailedAt = time.Time{} // the 30 s cooldown
	if _, err := rig.appendBatch(); !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("the write after the failed catch-up returned %v, want ErrBatchRefused (numbered before the catch-up applied the landed event)", err)
	}
	if got := len(rig.cas.records); got != records {
		t.Fatalf("the host holds %d events, want %d: a stale batch reached the CAS", got, records)
	}
	rig.requireAligned("after the catch-up", 2)
	if _, err := rig.appendBatch(); err != nil {
		t.Fatalf("the next attempt: %v", err)
	}
	rig.requireAligned("after the next attempt", 3)
	if st := rig.app.WedgeStatus(); st.Active {
		t.Fatalf("wedge active: %+v", st)
	}
}

// U3: with no recovery the guard costs nothing: one dry run, one head read.
func TestAppendWithoutRecoveryPaysNoExtraDryRunOrHeadRead(t *testing.T) {
	rig := newStaleRig(t)
	for i := 0; i < 2; i++ {
		rig.cas.mu.Lock()
		heads := rig.cas.headCalls
		rig.cas.mu.Unlock()
		dry := rig.proj2.dryRuns
		if _, err := rig.appendBatch(); err != nil {
			t.Fatal(err)
		}
		rig.cas.mu.Lock()
		heads = rig.cas.headCalls - heads
		rig.cas.mu.Unlock()
		if got := rig.proj2.dryRuns - dry; got != 1 {
			t.Fatalf("append %d ran the dry run %d times, want 1", i, got)
		}
		if heads != 1 {
			t.Fatalf("append %d read the host head %d times, want 1", i, heads)
		}
	}
}

// anyBatch is a batch no staleness can refuse.
func (r *staleRig) anyBatch(eventID string) []byte {
	raw, err := json.Marshal(ProjectionBatch{
		Version: ProjectionBatchV2, ProjectorVersion: ProjectorVersionV2,
		ComputerID: testComputerID, EventID: eventID,
		Ops: []ProjectionOp{{Kind: ProjectionOpDesktopState, Body: json.RawMessage(`{"any":true}`)}},
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return raw
}

func (r *staleRig) appendAny() error {
	r.t.Helper()
	event := testEvent(r.t, nil, EventProjectionBatchRecorded)
	_, _, err := r.app.AppendNewPayload(context.Background(), event, TransitionInput{}, r.anyBatch(event.EventID), ProjectionBatchMediaType, "owner")
	return err
}

// sealedRig is the production shape: the batch is sealed, and the store (here
// the test) dry-runs it OUTSIDE the appender mutex before the append. Round 3:
// a writer R dry-runs, another writer B takes the mutex and moves the local
// head (its own recovery, or an ordinary finalize), then R takes the mutex with
// the heads level: no recovery of its own, yet its batch is stale.
func (r *staleRig) sealedAppend(batch []byte, eventID string, validatedAt uint64) error {
	r.t.Helper()
	cipher, err := NewPrivateArtifactCipher(testComputerID, make([]byte, 32))
	if err != nil {
		r.t.Fatal(err)
	}
	reader := NewMemoryArtifactReader()
	r.app.pins = sealedPinner{memoryPinner: memoryPinner{signer: r.signer}, reader: reader}
	r.app.SetPayloadResolver(reader, cipher)
	event := testEvent(r.t, nil, EventProjectionBatchRecorded)
	event.EventID = eventID
	event.IdempotencyKey = "idem-" + eventID
	_, _, err = r.app.AppendNewPrivatePayload(WithValidatedHead(context.Background(), validatedAt), event, TransitionInput{}, batch, ProjectionBatchMediaType, cipher)
	return err
}

type sealedPinner struct {
	memoryPinner
	reader *MemoryArtifactReader
}

func (p sealedPinner) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, intent string) (PinResult, error) {
	p.reader.Put(envelope)
	return p.memoryPinner.PinPrivatePayload(ctx, cipher, computerID, eventID, envelope, intent)
}

// R3a: R's dry run happened before B recovered a landed row under the mutex.
func TestSealedAppendWhoseDryRunStraddledAnotherWritersRecoveryIsRefused(t *testing.T) {
	rig := newStaleRig(t)
	rig.landUnacknowledged() // host seq 2, local 1, one kept prepared row
	rID, _ := NewEventID()
	stale := rig.batchAt(rID)               // R numbered and dry-ran at local head 1
	if err := rig.appendAny(); err != nil { // B recovers the landed row and appends
		t.Fatalf("B: %v", err)
	}
	rig.requireAligned("after B", 3)
	records := len(rig.cas.records)
	err := rig.sealedAppend(stale, rID, 1)
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("R returned %v, want ErrBatchRefused: its batch was validated before B moved the local head", err)
	}
	if got := len(rig.cas.records); got != records {
		t.Fatalf("the host holds %d events, want %d: R's stale batch reached the CAS", got, records)
	}
}

// R3b: R's dry run happened before another writer's ordinary finalize.
func TestSealedAppendWhoseDryRunStraddledAnOrdinaryFinalizeIsRefused(t *testing.T) {
	rig := newStaleRig(t)
	rID, _ := NewEventID()
	stale := rig.batchAt(rID)
	if err := rig.appendAny(); err != nil { // B: an ordinary append, no recovery
		t.Fatalf("B: %v", err)
	}
	records := len(rig.cas.records)
	err := rig.sealedAppend(stale, rID, 1)
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("R returned %v, want ErrBatchRefused", err)
	}
	if got := len(rig.cas.records); got != records {
		t.Fatalf("the host holds %d events, want %d", got, records)
	}
}

// After a prepare failure the in-place recovery may catch the local head up
// past the sequence the event was bound at (the host moved). The retry used
// the stale request, and its refusal counted as a wedge. It is refused cleanly.
func TestPrepareRecoveryThatMovedTheHeadRefusesTheBoundEventCleanly(t *testing.T) {
	rig := newStaleRig(t)
	rig.proj2.onPrepare = func() { rig.hostAhead(1) }
	err := func() error { _, err := rig.appendBatch(); return err }()
	if err == nil {
		t.Fatal("an event bound before the host moved was appended")
	}
	if st := rig.app.WedgeStatus(); st.Active {
		t.Fatalf("a clean refusal tripped the wedge counter: %+v", st)
	}
	if _, err := rig.appendBatch(); err != nil {
		t.Fatalf("the next append binds to the head as it is now: %v", err)
	}
}
