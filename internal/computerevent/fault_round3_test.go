package computerevent

import (
	"context"
	"fmt"
	"testing"
)

// Review round 3 of lane/appender-wedge.
//
//	K1. A kept row AT the host head whose re-send is refused (the host runs
//	    authorization and pin validation before its idempotency lookup) wedged
//	    until a boot: RecoverPrepared re-sent, failed, and the catch-up never
//	    ran because the heads' sequences were equal to the row's.
//	S-a. A row the host has contradicted kept its payload plaintext.

// K1: the first send commits at N and the reply is lost; the re-send and every
// recovery re-send are refused before the idempotency lookup; no later host
// event exists. One recovery pass must converge.
func TestRepairKeptRowAtTheHostHeadWhoseResendIsRefused(t *testing.T) {
	for _, status := range []int{400, 403} {
		t.Run(fmt.Sprintf("refused with %d", status), func(t *testing.T) {
			rig := newFaultRig(t)
			rig.proj.uniqueSequence = true
			rig.genesis(rig.app)
			refusal := func() casStep {
				return casStep{err: &CASError{Status: status, Definite: true, Err: fmt.Errorf("computer event client: corpusd returned %d: refused before the idempotency lookup", status)}}
			}
			rig.cas.script = []casStep{{commit: true, err: errReset}, refusal(), refusal(), refusal()}
			if _, err := rig.appendOne(rig.app); err == nil {
				t.Fatal("fixture: an append with an unknown outcome reported success")
			}
			if len(rig.proj.prepared) != 1 || rig.cas.head.Sequence != 2 || rig.proj.head.Sequence != 1 {
				t.Fatalf("fixture: prepared=%d host=%d local=%d, want 1/2/1", len(rig.proj.prepared), rig.cas.head.Sequence, rig.proj.head.Sequence)
			}
			// One recovery pass: the next append recovers the row, then appends.
			if _, err := rig.appendOne(rig.app); err != nil {
				t.Fatalf("the recovery pass did not converge: %v", err)
			}
			rig.requireAligned("after one pass", 3)
			if st := rig.app.WedgeStatus(); st.Active {
				t.Fatalf("wedge still active: %+v", st)
			}
		})
	}
}

// S-a: the host holds a DIFFERENT event at the row's sequence; the row's
// plaintext is released once the pass ends.
func TestRepairContradictedRowReleasesItsPayload(t *testing.T) {
	rig := newFaultRig(t)
	guest := &batchProjection{memoryProjection: rig.proj}
	rig.app = rig.newAppender(guest)
	rig.genesis(rig.app)

	stale := testEvent(t, nil, EventProjectionBatchRecorded)
	rig.cas.script = []casStep{{err: errReset}, {err: errReset}} // never reaches the host
	if _, _, err := rig.app.AppendNewPayload(context.Background(), stale, TransitionInput{}, testBatchPayloadFor(t, stale.EventID), ProjectionBatchMediaType, "owner"); err == nil {
		t.Fatal("append with an unknown outcome reported success")
	}
	if n := len(rig.app.livePayloads); n != 1 {
		t.Fatalf("fixture: cache holds %d entries, want 1", n)
	}
	rig.hostAhead(1) // the host commits another event at the row's sequence
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("append behind the contradicted row: %v", err)
	}
	if n := len(rig.app.livePayloads); n != 0 {
		t.Fatalf("cache holds %d entries after the host contradicted the row, want 0", n)
	}
	rig.requireAligned("after the pass", 3)
}
