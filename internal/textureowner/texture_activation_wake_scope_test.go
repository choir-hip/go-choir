package textureowner

// Regression for docs/problems/texture-raced-activation-refused-first-turn-2026-10-11.md
// (Root cause, 2026-10-15): the conductor handoff activated a fresh
// document's Texture desk outside the document's wake scope, so a concurrent
// lifecycle wake that had already checked for an active run staled the
// handoff run's pending mutation, lost its own activation, and recovered the
// handoff run — leaving the doc's active run with a stale_activation
// mutation that both activation validators refuse.
//
// Ways the fix (serialize every Texture activator on lockTextureWakeScope)
// could fail, each covered below or named:
//
//  F1. The handoff still writes its mutation or activation while a wake holds
//      the document's wake scope; the wake's stale loop stales it (the bug).
//      Covered: the handoff must park at the lock with nothing written.
//  F2. The lock is taken inside submitTextureAgentRevisionRun, which the wake
//      already calls under the (non-reentrant) lock: the wake deadlocks.
//      Covered: the test runs the real wake body under the held lock.
//  F3. The lock is held across StartLifecycle, whose work assignment wakes
//      the desk; a synchronous wake would deadlock. Not taken there by design;
//      production wakes are asynchronous actor deliveries.
//  F4. The handoff locks a key the wake does not (e.g. a different computer
//      id), so nothing is excluded. Covered: the test holds the wake's own key
//      derived from the document.
//  F5. The serialized loser (handoff after wake) errors instead of returning
//      the wake's run. Covered: InitialLoopID must be the wake's run.
//  F6. The loser's rollback stales the winner's mutation. Covered: the
//      winner's mutation stays pending and passes ValidateActivationAuthority.
//  F7. Handoff-first order: the later wake must stand down on the valid active
//      run without staling it. Covered by the second test.
//  F8. The frozen coagent route (processor/reconciler) has the same unlocked
//      submit; it takes the same lock. Not separately exercised (frozen path).

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func setupTextureActivationRace(t *testing.T) (*agentcore.Runtime, *Handler, *types.RunRecord, string) {
	t.Helper()
	core, handler := testAPISetup(t)
	// Keep every run pending: the test reads mutation and activation state
	// at each step, so nothing may execute behind its back.
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	ownerID := "user-alice"
	text := "Answer my first prompt."
	rec, err := core.CompletePromptBarDecision(context.Background(), text, ownerID, map[string]any{
		"agent_profile": agentprofile.Conductor, "agent_role": agentprofile.Conductor,
		"input_source": "prompt_bar", "requested_app": agentprofile.Texture,
		"seed_prompt": text, "submission_surface": "prompt_bar",
	}, agentcore.PromptBarDecisionSpec{Action: "open_app", App: agentprofile.Texture, Title: "Race"})
	if err != nil {
		t.Fatalf("CompletePromptBarDecision: %v", err)
	}
	commandID := metadataStringValue(rec.Metadata, "lifecycle_command_id")
	if commandID == "" {
		t.Fatal("conductor run has no lifecycle_command_id")
	}
	lifecycleKey := strings.Join([]string{"choir:texture:lifecycle", ownerID, core.TextureComputerID(), commandID}, ":")
	docID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(lifecycleKey+":document")).String()
	return core, handler, rec, docID
}

func runConductorHandoff(handler *Handler, rec *types.RunRecord) chan struct {
	decision HandoffDecision
	err      error
} {
	done := make(chan struct {
		decision HandoffDecision
		err      error
	}, 1)
	go func() {
		decision, err := handler.EnsureTextureHandoff(context.Background(), rec, HandoffRequest{
			Kind: HandoffKindUserPrompt, CallerProfile: agentprofile.Conductor, Objective: "Answer my first prompt.",
		})
		done <- struct {
			decision HandoffDecision
			err      error
		}{decision, err}
	}()
	return done
}

func textureWakeScopeWaiters(handler *Handler, ownerID, computerID, docID string) int {
	key := strings.Join([]string{ownerID, computerID, docID}, "\x00")
	handler.textureWakeLocksMu.Lock()
	defer handler.textureWakeLocksMu.Unlock()
	if lock := handler.textureWakeLocks[key]; lock != nil {
		return lock.refs
	}
	return 0
}

// TestConductorHandoffWaitsForTextureWakeScope: the wake holds the doc's wake
// scope and has seen no active run; the handoff must not activate until the
// wake is done.
func TestConductorHandoffWaitsForTextureWakeScope(t *testing.T) {
	core, handler, rec, docID := setupTextureActivationRace(t)
	ctx := context.Background()
	ownerID, computerID := rec.OwnerID, core.TextureComputerID()
	agentID := currentTextureAgentID(docID)

	unlock := handler.lockTextureWakeScope(ownerID, computerID, docID)
	unlocked := false
	defer func() {
		if !unlocked {
			unlock()
		}
	}()
	done := runConductorHandoff(handler, rec)

	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case res := <-done:
			// The handoff activated Texture inside the wake's window. Play the
			// rest of the wake (reconcileAgentWakeLocked after its active-run
			// check) to show the staging failure it causes.
			if res.err != nil {
				t.Fatalf("handoff: %v", res.err)
			}
			doc, err := handler.getTextureDocument(ctx, ownerID, docID)
			if err != nil {
				t.Fatalf("load doc: %v", err)
			}
			for {
				m, err := handler.Store.GetPendingAgentMutationByDoc(ctx, ownerID, computerID, docID)
				if err != nil || m == nil {
					break
				}
				_ = handler.Store.MarkAgentMutationStale(ctx, ownerID, computerID, m.RunID)
			}
			recovered, err := handler.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{Intent: "initial_owner_work"}, 0)
			if err != nil {
				t.Fatalf("handoff activated outside the wake scope (run %s); wake submit: %v", res.decision.InitialLoopID, err)
			}
			t.Fatalf("handoff activated outside the wake scope: wake recovered run %s (handoff run %s); activation authority: %v",
				recovered.RunID, res.decision.InitialLoopID, handler.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, recovered.RunID))
		default:
		}
		if textureWakeScopeWaiters(handler, ownerID, computerID, docID) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("handoff neither finished nor waited on the wake scope")
		}
		time.Sleep(2 * time.Millisecond)
	}

	// F1: parked at the lock with nothing written.
	if m, err := handler.Store.GetPendingAgentMutationByDoc(ctx, ownerID, computerID, docID); err != nil || m != nil {
		t.Fatalf("handoff wrote a pending mutation before taking the wake scope: %+v err=%v", m, err)
	}
	if active, found, err := core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentID); err != nil || found {
		t.Fatalf("handoff activated before taking the wake scope: run=%s err=%v", active.RunID, err)
	}

	// The wake (F2: real body, under the held lock) wins the activation.
	doc, err := handler.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		t.Fatalf("load doc: %v", err)
	}
	wakeRun, err := handler.reconcileAgentWakeLocked(ctx, doc, agentID)
	if err != nil || wakeRun == nil {
		t.Fatalf("wake: run=%v err=%v", wakeRun, err)
	}
	unlock()
	unlocked = true

	var res struct {
		decision HandoffDecision
		err      error
	}
	select {
	case res = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handoff did not finish after the wake released the scope")
	}
	// F5: the handoff loses and returns the wake's run.
	if res.err != nil || res.decision.InitialLoopID != wakeRun.RunID {
		t.Fatalf("handoff = %q err=%v, want recovered wake run %s", res.decision.InitialLoopID, res.err, wakeRun.RunID)
	}
	// F6: the winner's mutation is pending and passes activation authority.
	if err := handler.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, wakeRun.RunID); err != nil {
		t.Fatalf("winner activation authority: %v", err)
	}
}

// TestTextureWakeStandsDownOnHandoffActivation (F7): handoff first, then the
// wake; the wake keeps the handoff's run and its pending mutation.
func TestTextureWakeStandsDownOnHandoffActivation(t *testing.T) {
	core, handler, rec, docID := setupTextureActivationRace(t)
	ctx := context.Background()
	ownerID, computerID := rec.OwnerID, core.TextureComputerID()
	agentID := currentTextureAgentID(docID)

	res := <-runConductorHandoff(handler, rec)
	if res.err != nil || res.decision.InitialLoopID == "" {
		t.Fatalf("handoff: %+v err=%v", res.decision, res.err)
	}
	if wakeRun, err := handler.ReconcileAgentWake(ctx, ownerID, docID); err != nil || wakeRun != nil {
		t.Fatalf("wake after handoff = %v err=%v, want stand-down", wakeRun, err)
	}
	if err := handler.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, res.decision.InitialLoopID); err != nil {
		t.Fatalf("handoff run activation authority after wake: %v", err)
	}
}
