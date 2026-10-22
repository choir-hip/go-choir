package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The post-commit observer can finalize an owner's decision between the
// handler's decision append and its own operation Transition. The decision
// landed, so the handler must answer 200 with the decided operation, and a
// replay of the same key must answer 200 too. Staging answered 409 for a
// landed reject (docs/problems/selfdev-reject-lands-but-answers-409-2026-10-22.md).
//
// Ways this could pass while the product is still wrong, and what the test
// does about each:
//   - the observer never ran inside the window: the seam asserts the
//     operation already left awaiting_approval before the handler moves it;
//   - the answer is 200 but for some other operation state: the decided
//     operation must carry the decision event the tape holds;
//   - a replay passes because it was a fresh decision: the replay must not
//     add a second decision event;
//   - the fix loosens the replay: a different key and a changed reason on
//     the same key must still be refused.
func TestSelfDevDecisionObserverWinsRaceAnswersDecidedOperation(t *testing.T) {
	for _, decision := range []string{"reject", "approve"} {
		t.Run(decision, func(t *testing.T) {
			f := newP4Fixture(t)
			ctx := context.Background()
			operation := f.verify(t, f.freeze(t, "race-"+decision, "observer race "+decision))
			head := f.waitForStableHead(t)
			pending := head.PendingTransitionRef
			request := selfDevelopmentDecisionRequest{
				Decision: decision, IdempotencyKey: "race-key-" + decision,
				BundleDigest: operation.BundleDigest, VerifierRef: operation.VerifierRefs[0],
				ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
				ExpectedPendingTransitionRef:   &pending,
				ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
			}
			wantState := selfdev.StateRejected
			if decision == "reject" {
				request.Reason = "observer race drill"
				arm := platform.SetSelfDevelopmentModeRequest{
					Mode: platform.SelfDevelopmentModeProposeOnly, ExpectedGeneration: f.currentMode(t).Generation,
					IdempotencyKey: "race-propose-only",
				}
				if code, _, body := f.ownerWrite(t, arm, platform.ComputerTierTest); code != http.StatusOK {
					t.Fatalf("owner propose_only: %d %s", code, body)
				}
			} else {
				wantState = selfdev.StateApplied
				arm := platform.SetSelfDevelopmentModeRequest{
					Mode: platform.SelfDevelopmentModeAcceptOnce, ExpectedGeneration: f.currentMode(t).Generation,
					OperationID: operation.OperationID, BundleDigest: operation.BundleDigest,
					ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
					ExpectedPendingTransitionRef:   &pending,
					ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
					ExpiresAt:      time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond).Format(time.RFC3339Nano),
					IdempotencyKey: "race-accept-once",
				}
				if code, _, body := f.ownerWrite(t, arm, platform.ComputerTierTest); code != http.StatusOK {
					t.Fatalf("owner accept_once: %d %s", code, body)
				}
			}

			// The observer wins: it finalizes (and, for an approve, applies)
			// before the handler moves the operation.
			raced := false
			f.rt.selfdevDecisionBeforeTransition = func(ctx context.Context, operationID string) {
				f.rt.reconcileSelfDevelopmentMaterialization(ctx)
				current, err := f.operations.Get(ctx, f.computerID, operationID)
				raced = err == nil && current.State == wantState
			}
			handler := &APIHandler{rt: f.rt}
			decided, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request)
			f.rt.selfdevDecisionBeforeTransition = nil
			if !raced {
				t.Fatal("the observer did not finalize the decision inside the handler's window")
			}
			if status != http.StatusOK || err != nil {
				t.Fatalf("landed %s answered %d: %v", decision, status, err)
			}
			event := f.decisionEvent(t, operation.OperationID)
			eventDigest, _ := event.Digest()
			if decided.OperationID != operation.OperationID || decided.State != wantState || decided.DecisionEvent != eventDigest || decided.DecisionReceipt == "" {
				t.Fatalf("decided operation = %+v, want %s carrying %s", decided, wantState, eventDigest)
			}
			decisionEvents := f.countEventKinds()[event.EventKind]

			// A replay of the same key learns that the first call succeeded.
			// The caller no longer has the decision-time head (the decision
			// moved it), so it rebuilds the binding from the release
			// witness, as the CLI does.
			raw, err := json.Marshal(f.rt.readReleaseWitness(ctx, decided))
			if err != nil {
				t.Fatal(err)
			}
			var witness struct {
				Decision map[string]any `json:"decision"`
			}
			if err := json.Unmarshal(raw, &witness); err != nil {
				t.Fatal(err)
			}
			field := func(name string) string { value, _ := witness.Decision[name].(string); return value }
			replay := request
			replayPending := field("expected_pending_transition_ref")
			replay.ExpectedPendingTransitionRef = &replayPending
			replay.ExpectedDesiredEventHead, replay.ExpectedEffectiveEventHead = field("expected_desired_event_head"), field("expected_effective_event_head")
			replay.ExpectedDesiredStateCommitment, replay.ExpectedEffectiveStateCommitment = field("expected_desired_state_commitment"), field("expected_effective_state_commitment")
			replayed, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, replay)
			if status != http.StatusOK || err != nil || replayed.DecisionEvent != eventDigest || replayed.State != wantState {
				t.Fatalf("same-key replay from the witness binding answered %d %v: %+v (witness decision %v)", status, err, replayed, witness.Decision)
			}
			if got := f.countEventKinds()[event.EventKind]; got != decisionEvents {
				t.Fatalf("replay appended a decision: %d -> %d", decisionEvents, got)
			}
			// A replay built from the current head is a different binding.
			if moved := f.waitForStableHead(t); moved.DesiredEventHead != request.ExpectedDesiredEventHead || moved.DesiredStateCommitment != request.ExpectedDesiredStateCommitment {
				stale := request
				stale.ExpectedDesiredEventHead, stale.ExpectedDesiredStateCommitment = moved.DesiredEventHead, moved.DesiredStateCommitment
				if _, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, stale); status != http.StatusConflict || err == nil {
					t.Fatalf("a same-key replay with other heads answered %d %v", status, err)
				}
			}

			// Nothing else succeeds: another key, or the same key with a
			// changed binding.
			other := request
			other.IdempotencyKey = "race-other-key-" + decision
			if _, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, other); status != http.StatusConflict || err == nil {
				t.Fatalf("a different key after the decision answered %d %v", status, err)
			}
			changed := request
			if decision == "reject" {
				changed.Reason = "a different reason"
			} else {
				changed.Decision, changed.Reason = "reject", "flip to reject"
			}
			if _, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, changed); status != http.StatusConflict || err == nil {
				t.Fatalf("the same key with a changed binding answered %d %v", status, err)
			}
		})
	}
}
