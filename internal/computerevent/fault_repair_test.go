package computerevent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Review round 2 of lane/appender-wedge. Ways the first fix can fail, written
// before the repair:
//
//	R1. A prepared row kept at N (the outcome was unknown twice) becomes
//	    unresolvable when the host moves past N: RecoverPrepared knows only the
//	    host head and head+1, so it errors before the catch-up runs, forever.
//	R2. A stale row (the host holds a different event at its sequence) blocks
//	    the catch-up's Prepare on the sequence key.
//	R3. A catch-up that stops between Prepare and Finalize leaves a row the
//	    next attempt cannot resolve.
//	R4. After a re-send settles the CAS, finalize runs on the cancelled
//	    caller's ctx and fails; the append reports a poison.
//	R5. A refusal of the RE-SEND deletes the only record that the first send
//	    may have committed.
//	R6. A catch-up over a projection-batch event whose payload fetch fails.
//	R7. A failed AppendNewPayload keeps the plaintext forever.
//	R8. A catch-up stopped by its bound reports the generic heads-differ kind.

// keptRow leaves a prepared row at the next sequence with an unknown outcome:
// the first send commits (or not) at the host and both sends return an error.
func (r *faultRig) keptRow(committed bool) {
	r.t.Helper()
	r.cas.script = []casStep{{commit: committed, err: errReset}, {err: errReset}}
	if _, err := r.appendOne(r.app); err == nil {
		r.t.Fatal("fixture: an append with an unknown outcome reported success")
	}
	if len(r.proj.prepared) != 1 {
		r.t.Fatalf("fixture: prepared rows=%d, want the one kept row", len(r.proj.prepared))
	}
}

// R1: the row is the event the host committed; the host then moves past it.
func TestRepairKeptRowResolvesWhenTheHostPassesIt(t *testing.T) {
	for _, k := range []int{1, 3} {
		t.Run(fmt.Sprintf("host+%d", k), func(t *testing.T) {
			rig := newFaultRig(t)
			rig.proj.uniqueSequence = true
			rig.genesis(rig.app)
			rig.keptRow(true) // host 2, local 1, row 2
			rig.hostAhead(k)  // host 2+k
			if _, err := rig.appendOne(rig.app); err != nil {
				t.Fatalf("append with a kept row and the host %d past it: %v", k, err)
			}
			rig.requireAligned("after one pass", uint64(2+k+1))
		})
	}
}

// R2: the row is an event the host never committed; the host holds another
// event at that sequence.
func TestRepairStaleRowWithADifferentHostEvent(t *testing.T) {
	for _, k := range []int{1, 3} {
		t.Run(fmt.Sprintf("host+%d", k), func(t *testing.T) {
			rig := newFaultRig(t)
			rig.proj.uniqueSequence = true
			rig.genesis(rig.app)
			rig.keptRow(false) // host 1, local 1, stale row 2
			rig.hostAhead(k)   // the host commits OTHER events at 2..
			if _, err := rig.appendOne(rig.app); err != nil {
				t.Fatalf("append behind a stale row: %v", err)
			}
			rig.requireAligned("after one pass", uint64(1+k+1))
		})
	}
}

// R3: the catch-up stops between Prepare and Finalize and the discard fails
// too (a crash): a later append still converges.
func TestRepairCatchUpLeftARowThenALaterAppend(t *testing.T) {
	rig := newFaultRig(t)
	rig.proj.uniqueSequence = true
	rig.genesis(rig.app)
	rig.hostAhead(5) // host 6
	failed := false
	rig.proj.finalizeHook = func(sequence uint64) error {
		if sequence == 4 && !failed {
			failed = true
			return errors.New("injected projection failure")
		}
		return nil
	}
	rig.proj.failDiscardN = 1
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append reported success while the catch-up failed")
	}
	if len(rig.proj.prepared) != 1 || rig.proj.head.Sequence != 3 {
		t.Fatalf("fixture: prepared=%d local=%d, want one stuck row and local 3", len(rig.proj.prepared), rig.proj.head.Sequence)
	}
	rig.app.recoveryFailedAt = time.Time{}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("a later append did not resolve the stuck row: %v", err)
	}
	rig.requireAligned("after the later append", 7)
}

// R4: the caller's ctx is cancelled while the request is in flight (the
// likeliest trigger of 2026-11-12). The re-send settles the commit; finalize
// must not run on the cancelled ctx.
func TestRepairCancelledCallerStillFinalizes(t *testing.T) {
	rig := newFaultRig(t)
	rig.proj.ctxAware = true
	rig.genesis(rig.app)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rig.cas.script = []casStep{{commit: true, hook: cancel, err: context.Canceled}}
	if _, err := rig.app.AppendNew(ctx, testEvent(t, nil, EventArtifactProduced), TransitionInput{}, nil); err != nil {
		t.Fatalf("append whose caller was cancelled after the commit: %v", err)
	}
	rig.requireAligned("after the append", 2)
	if st := rig.app.WedgeStatus(); st.Active {
		t.Fatalf("wedge tripped: %+v", st)
	}
}

// R5: the re-send is refused by a check that runs before the host's
// idempotency lookup (auth, pin validation), although the first send committed.
// Only a refusal of the first send proves nothing was committed.
func TestRepairResendRefusalKeepsTheRow(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{commit: true, err: errReset}, {err: fmt.Errorf("%w: refused before the idempotency lookup", ErrCASConflict)}}
	discards := rig.proj.discards
	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append reported success")
	}
	if rig.proj.discards != discards || len(rig.proj.prepared) != 1 {
		t.Fatalf("the row was dropped on a refusal of the re-send: discards+%d prepared=%d", rig.proj.discards-discards, len(rig.proj.prepared))
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the next append", 3)
}

// R5b: the re-send is refused and the host never committed: the next pass
// discards the row from the host's head.
func TestRepairResendRefusalWithoutACommitIsDiscardedLater(t *testing.T) {
	rig := newFaultRig(t)
	rig.proj.uniqueSequence = true
	rig.genesis(rig.app)
	rig.cas.script = []casStep{{err: errReset}, {err: fmt.Errorf("%w: refused", ErrCASConflict)}}
	_, _ = rig.appendOne(rig.app)
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("next append: %v", err)
	}
	rig.requireAligned("after the next append", 2)
}

// batchProjection finalizes projection batches (the store's role) and records
// them.
type batchProjection struct {
	*memoryProjection
	applied []string
}

func (p *batchProjection) FinalizeBatch(ctx context.Context, computerID, digest string, receipt Receipt, batch *ProjectionBatch) error {
	if batch != nil {
		p.applied = append(p.applied, batch.EventID)
	}
	return p.memoryProjection.Finalize(ctx, computerID, digest, receipt)
}

type flakyReader struct {
	inner ArtifactReader
	fails int
}

func (r *flakyReader) FetchPayload(ctx context.Context, computerID, digest string) ([]byte, error) {
	if r.fails > 0 {
		r.fails--
		return nil, errors.New("injected payload fetch failure")
	}
	return r.inner.FetchPayload(ctx, computerID, digest)
}

// R6: the host holds a projection_batch_recorded event the guest never
// applied; the payload fetch fails once during the catch-up.
func TestRepairCatchUpOverAProjectionBatchWhosePayloadFetchFails(t *testing.T) {
	rig := newFaultRig(t)
	guest := &batchProjection{memoryProjection: rig.proj}
	rig.app = rig.newAppender(guest)
	reader := NewMemoryArtifactReader()
	flaky := &flakyReader{inner: reader, fails: 1}
	rig.app.SetPayloadResolver(flaky, nil)
	rig.genesis(rig.app)

	writer := rig.boot(&batchProjection{memoryProjection: &memoryProjection{}})
	batchEvent := testEvent(t, nil, EventProjectionBatchRecorded)
	batchEvent.PrivacyClass = "public"
	payload := testBatchPayloadFor(t, batchEvent.EventID)
	reader.Put(payload)
	if _, _, err := writer.AppendNewPayload(context.Background(), batchEvent, TransitionInput{}, payload, ProjectionBatchMediaType, "owner"); err != nil {
		t.Fatalf("host-side batch append: %v", err)
	}
	if _, err := rig.appendOne(writer); err != nil {
		t.Fatal(err)
	}

	if _, err := rig.appendOne(rig.app); err == nil {
		t.Fatal("append reported success while the payload fetch failed")
	}
	if rig.proj.head.Sequence != 1 || len(rig.proj.prepared) != 0 {
		t.Fatalf("after the failed fetch: local=%d prepared=%d, want 1/0", rig.proj.head.Sequence, len(rig.proj.prepared))
	}
	rig.app.recoveryFailedAt = time.Time{}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("the next pass did not resume: %v", err)
	}
	rig.requireAligned("after resuming", 4)
	if len(guest.applied) != 1 || guest.applied[0] != batchEvent.EventID {
		t.Fatalf("applied batches = %v, want [%s]", guest.applied, batchEvent.EventID)
	}
}

// R7 / F23: a failed AppendNewPayload leaves its plaintext in livePayloads.
// It is needed only while a prepared row may still be finalized from it.
func TestRepairLivePayloadIsDroppedWhenNoRowNeedsIt(t *testing.T) {
	rig := newFaultRig(t)
	guest := &batchProjection{memoryProjection: rig.proj}
	rig.app = rig.newAppender(guest)
	rig.genesis(rig.app)

	refused := testEvent(t, nil, EventProjectionBatchRecorded)
	rig.cas.script = []casStep{{err: fmt.Errorf("%w: refused", ErrCASConflict)}}
	if _, _, err := rig.app.AppendNewPayload(context.Background(), refused, TransitionInput{}, testBatchPayloadFor(t, refused.EventID), ProjectionBatchMediaType, "owner"); err == nil {
		t.Fatal("refused append reported success")
	}
	if n := len(rig.app.livePayloads); n != 0 {
		t.Fatalf("a refused append left %d plaintexts in the cache", n)
	}

	// An unknown outcome keeps the row, and the plaintext with it, until the
	// next pass finalizes the row.
	unknown := testEvent(t, nil, EventProjectionBatchRecorded)
	rig.cas.script = []casStep{{commit: true, err: errReset}, {err: errReset}}
	if _, _, err := rig.app.AppendNewPayload(context.Background(), unknown, TransitionInput{}, testBatchPayloadFor(t, unknown.EventID), ProjectionBatchMediaType, "owner"); err == nil {
		t.Fatal("append with an unknown outcome reported success")
	}
	if n := len(rig.app.livePayloads); n != 1 {
		t.Fatalf("cache holds %d entries while the row is kept, want 1", n)
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatal(err)
	}
	if n := len(rig.app.livePayloads); n != 0 {
		t.Fatalf("cache holds %d entries after the row was finalized, want 0", n)
	}
}

// R8: a catch-up stopped by its bound is named, not the generic heads-differ.
func TestRepairIncompleteCatchUpHasANamedWedgeKind(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(EventReplayPageSize + 1)
	rig.app.catchUpPages = 1
	if _, err := rig.appendOne(rig.app); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append after a bounded attempt = %v, want projection repair", err)
	}
	if st := rig.app.WedgeStatus(); !st.Active || st.LastError != "catch-up incomplete" {
		t.Fatalf("wedge=%+v, want active with kind %q", st, "catch-up incomplete")
	}
}

// The harness checks the commitment on a replayed idempotency key, as the host
// does, so a re-send of a changed request is a conflict and not a replay.
func TestRepairHarnessRefusesAChangedCommitmentOnReplay(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	changed := rig.cas.records[0].Request
	changed.Event.RequestCommitment = testDigestB
	if _, err := rig.cas.CompareAndSwap(context.Background(), changed); !CASRefusedDefinitely(err) {
		t.Fatalf("replay with a changed commitment = %v, want a definite refusal", err)
	}
	same := rig.cas.records[0].Request
	if _, err := rig.cas.CompareAndSwap(context.Background(), same); err != nil {
		t.Fatalf("identical replay: %v", err)
	}
	if len(rig.cas.records) != 1 {
		t.Fatalf("a replay added a row: %d", len(rig.cas.records))
	}
}
