package actorruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
)

// Work resumes after an update restart (owner rule), but not while the
// apply's checkpoint still needs a quiet chain
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md).
// Failure modes pinned: work runs while an apply is materializing; work is
// held when nothing is materializing; work is held past the bound; a cancel,
// deadline or the materializer's retry is held.
func TestApplyHoldDefersWorkWhileMaterializing(t *testing.T) {
	boot := time.Date(2026, 10, 10, 1, 13, 53, 0, time.UTC)
	materializing := true
	h := &actorHandler{
		bootAt: boot, plannedBoot: true, applyHoldUntil: boot.Add(applyHoldWindow),
		applyMaterializing: func(context.Context) bool { return materializing },
		now:                func() time.Time { return boot.Add(time.Minute) },
	}
	mailbox := scopedActorMailboxID("owner-1", "computer-1", "texture:doc-1")
	for kind := range restartWorkKinds {
		if !h.applyHoldsWork(context.Background(), actor.Update{ToAgentID: mailbox, Kind: kind}) {
			t.Fatalf("%s not held while an apply is materializing", kind)
		}
	}
	_, err := h.HandleUpdate(context.Background(), mailbox, actor.Update{UpdateID: "u1", ToAgentID: mailbox, Kind: "coagent_result", CreatedAt: boot.Add(time.Second)}, nil)
	if !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("HandleUpdate while materializing: err=%v, want deferred", err)
	}
	for _, kind := range []string{"cancel", "lifecycle_cancellation", "activation_budget_deadline", "cell_terminal_deadline", "selfdev_materialization_retry"} {
		if h.applyHoldsWork(context.Background(), actor.Update{ToAgentID: mailbox, Kind: kind}) {
			t.Fatalf("%s held; closing work and the materializer's retry must still run", kind)
		}
	}

	materializing = false
	if h.applyHoldsWork(context.Background(), actor.Update{ToAgentID: mailbox, Kind: "coagent_result"}) {
		t.Fatal("work held with nothing materializing")
	}

	materializing = true
	h.now = func() time.Time { return boot.Add(applyHoldWindow + time.Second) }
	if h.applyHoldsWork(context.Background(), actor.Update{ToAgentID: mailbox, Kind: "coagent_result"}) {
		t.Fatal("work held past the bound; a stalled apply must delay work, never destroy it")
	}

	other := &actorHandler{bootAt: boot, plannedBoot: true, applyMaterializing: func(context.Context) bool { return true }, now: func() time.Time { return boot }}
	if other.applyHoldsWork(context.Background(), actor.Update{ToAgentID: mailbox, Kind: "coagent_result"}) {
		t.Fatal("a boot that was not a self-development apply held work")
	}
}
