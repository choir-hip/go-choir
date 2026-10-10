package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// A crash boot closes every pre-decision self-development operation
// (docs/problems/selfdev-zombie-operations-pin-owner-computer-2026-10-10.md).
//
// Ways the closer can fail:
//  1. An executing operation survives a crash boot and pins the busy probe.
//  2. A planned boot closes work it should resume.
//  3. It closes awaiting_approval (the owner's decision) or a state the
//     materializer recovers from the updater journal.
//  4. A closed operation carries no visible fate.
//  5. It touches another computer's operations.

func seedSelfdevOperationForCrashTest(t *testing.T, rt *Runtime, computerID, operationID, state string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := rt.store.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, verifier_refs_json, desired_head, effective_head, state, created_at, updated_at) VALUES (?, ?, ?, '', ?, '', '', '[]', '', '', ?, ?, ?)`,
		operationID, computerID, "key-"+operationID, "trajectory-"+operationID, state, now, now); err != nil {
		t.Fatal(err)
	}
}

func crashCloseFixture(t *testing.T) (*Runtime, *selfdev.Store, map[string]string) {
	t.Helper()
	rt, productStore := testRuntime(t)
	rt.cfg.ComputerID = "computer-crash-close"
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	states := map[string]string{
		"op-requested":  selfdev.StateRequested,
		"op-executing":  selfdev.StateExecuting,
		"op-frozen":     selfdev.StateFrozen,
		"op-verified":   selfdev.StateVerified,
		"op-awaiting":   selfdev.StateAwaitingApproval,
		"op-accepted":   selfdev.StateAccepted,
		"op-material":   selfdev.StateMaterializing,
		"op-rollback":   selfdev.StateRollbackPending,
		"op-degraded":   selfdev.StateDegraded,
		"op-applied":    selfdev.StateApplied,
		"op-failed-old": selfdev.StateFailed,
	}
	for id, state := range states {
		seedSelfdevOperationForCrashTest(t, rt, rt.cfg.ComputerID, id, state)
	}
	seedSelfdevOperationForCrashTest(t, rt, "computer-other", "op-other-executing", selfdev.StateExecuting)
	return rt, operations, states
}

func TestCrashBootClosesPreDecisionSelfDevelopmentOperations(t *testing.T) {
	ctx := context.Background()
	rt, operations, states := crashCloseFixture(t)

	rt.closeSelfDevelopmentOperationsAfterCrash(ctx)

	closed := map[string]bool{"op-requested": true, "op-executing": true, "op-frozen": true, "op-verified": true}
	for id, before := range states {
		operation, err := operations.Get(ctx, rt.cfg.ComputerID, id)
		if err != nil {
			t.Fatal(err)
		}
		if closed[id] {
			if operation.State != selfdev.StateFailed || operation.TerminalError != selfdevInterruptedByRestart {
				t.Errorf("%s (%s) after crash boot = %s %q, want failed %q", id, before, operation.State, operation.TerminalError, selfdevInterruptedByRestart)
			}
			continue
		}
		if operation.State != before {
			t.Errorf("%s moved from %s to %s; the crash closer only closes pre-decision states", id, before, operation.State)
		}
	}
	other, err := operations.Get(ctx, "computer-other", "op-other-executing")
	if err != nil || other.State != selfdev.StateExecuting {
		t.Fatalf("another computer's operation = %+v err=%v, want untouched", other, err)
	}
	if n := rt.activeSelfdevOperations(ctx); n != 3 {
		t.Fatalf("busy count after crash boot = %d, want 3 (accepted, materializing, rollback_pending)", n)
	}
}

func TestPlannedBootKeepsSelfDevelopmentOperations(t *testing.T) {
	ctx := context.Background()
	rt, operations, states := crashCloseFixture(t)
	WithBootRestart(PlannedRestart{Reason: PlannedRestartSelfDevelopmentApply, Target: "op-material"}, true)(rt)

	rt.closeSelfDevelopmentOperationsAfterCrash(ctx)

	for id, before := range states {
		operation, err := operations.Get(ctx, rt.cfg.ComputerID, id)
		if err != nil {
			t.Fatal(err)
		}
		if operation.State != before {
			t.Errorf("planned boot moved %s from %s to %s", id, before, operation.State)
		}
	}
}
