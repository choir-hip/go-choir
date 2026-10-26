package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Density D3a (docs/problems/idle-hibernate-blind-to-live-capsules-2026-10-26.md):
// vmctl decides a computer is idle from /health's running_runs,
// desk_pending_mutations and selfdev_active_operations. Three states hold a
// live engineering capsule and read zero on all three, so /health also
// reports live_capsules, the count the planned-restart refusal already
// computes.
//
// Ways this could fail, written first:
//   - /health omits the count, or reports 0, for a parked run, a capsule
//     before its first Freeze, or a frozen candidate awaiting approval;
//   - /health computes the count a second way, and the two drift;
//   - the field is omitted when 0, so vmctl cannot tell "no live capsule"
//     from "a guest that predates the field";
//   - a count that cannot be read answers 0 (idle) instead of failing the
//     health read closed (503 store_stalled);
//   - the planned-restart refusal changes.

func healthLiveCapsules(t *testing.T, handler *APIHandler) (*int, map[string]json.RawMessage) {
	t.Helper()
	rec, _ := serveHealth(t, handler)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health status=%d body=%s", rec.Code, rec.Body.String())
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	raw, present := keys["live_capsules"]
	if !present {
		return nil, keys
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("live_capsules = %s: %v", raw, err)
	}
	return &n, keys
}

// seedLiveCapsule opens and binds one engineering assignment (capsule
// active, bound run pending) and returns what is needed to move it on.
func seedLiveCapsule(t *testing.T, handler *APIHandler, s *choirstore.Store) types.OpenEngineeringAssignmentRequest {
	t.Helper()
	ctx := context.Background()
	computerID := handler.rt.TextureComputerID()
	seed, err := choirstore.SeedEngineeringAssignmentAuthority(s, "owner-live-capsule", computerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	capability := "opaque-live-capsule"
	open := choirstore.SeedEngineeringOpenRequest(seed, 0, capability)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, choirstore.SeedEngineeringBindRequest(open, "run:"+seed.AssignmentIDs[0], capability)); err != nil {
		t.Fatal(err)
	}
	return open
}

func TestHealthReportsZeroLiveCapsulesWhenThereAreNone(t *testing.T) {
	handler, _ := newBoundedHealthFixture(t)
	got, _ := healthLiveCapsules(t, handler)
	if got == nil || *got != 0 {
		t.Fatalf("live_capsules = %v, want an explicit 0 (absent means a guest that predates the field)", got)
	}
}

// The three states of the problem doc: each reads running_runs 0,
// desk_pending_mutations 0 and selfdev_active_operations 0, and each reports
// a live capsule.
func TestHealthReportsLiveCapsuleInTheThreeBlindStates(t *testing.T) {
	ctx := context.Background()

	t.Run("capsule before its first Freeze, no operation", func(t *testing.T) {
		handler, s := newBoundedHealthFixture(t)
		seedLiveCapsule(t, handler, s)
		assertBlindToTheThreeFieldsButCounted(t, handler)
	})

	t.Run("parked (passivated) bound run keeps its capsule", func(t *testing.T) {
		handler, s := newBoundedHealthFixture(t)
		open := seedLiveCapsule(t, handler, s)
		run, err := s.GetRun(ctx, "run:"+open.AssignmentID)
		if err != nil {
			t.Fatal(err)
		}
		run.State = types.RunPassivated
		if err := s.UpdateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
		if handler.rt.RunningCount() != 0 {
			t.Fatalf("a parked run counts as running")
		}
		assertBlindToTheThreeFieldsButCounted(t, handler)
	})

	t.Run("frozen candidate awaiting approval", func(t *testing.T) {
		handler, s := newBoundedHealthFixture(t)
		open := seedLiveCapsule(t, handler, s)
		freezeCapsule(t, s, handler.rt.TextureComputerID(), open)
		operations, err := selfdev.NewStore(s, s)
		if err != nil {
			t.Fatal(err)
		}
		handler.rt.selfdevOperations = operations
		now := time.Now().UTC()
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,base_head,prompt_artifact_ref,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'[]',?,?,?, ?,?)`,
			"selfdev-awaiting", handler.rt.TextureComputerID(), "key-awaiting", strings.Repeat("d", 64), "trajectory-awaiting", strings.Repeat("a", 64),
			"artifact:sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateAwaitingApproval, now, now); err != nil {
			t.Fatal(err)
		}
		assertBlindToTheThreeFieldsButCounted(t, handler)
	})
}

func freezeCapsule(t *testing.T, s *choirstore.Store, computerID string, open types.OpenEngineeringAssignmentRequest) {
	t.Helper()
	ctx := context.Background()
	assignments, err := s.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil || len(assignments) != 1 {
		t.Fatalf("assignments = %v, %v", assignments, err)
	}
	version := assignments[0].LifecycleVersion
	intent := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-freeze-intent", OwnerID: open.Binding.OwnerID, ComputerID: computerID,
		AssignmentID: open.AssignmentID, Attempt: open.Binding.Attempt, ExpectedLifecycleVersion: version,
		Disposition: types.EngineeringCapsuleFreezeRequested, IntentRef: "intent:freeze",
	}
	intent.CommandDigest, _ = choirstore.ComputeSetEngineeringCapsuleDispositionDigest(intent)
	if _, err := s.SetEngineeringCapsuleDisposition(ctx, intent); err != nil {
		t.Fatalf("freeze intent: %v", err)
	}
	ack := intent
	ack.CommandID, ack.ExpectedLifecycleVersion = "command-freeze-ack", version+1
	ack.Disposition, ack.AckRef = types.EngineeringCapsuleFrozen, "receipt:freeze"
	ack.CommandDigest, _ = choirstore.ComputeSetEngineeringCapsuleDispositionDigest(ack)
	if _, err := s.SetEngineeringCapsuleDisposition(ctx, ack); err != nil {
		t.Fatalf("freeze ack: %v", err)
	}
}

func assertBlindToTheThreeFieldsButCounted(t *testing.T, handler *APIHandler) {
	t.Helper()
	rec, _ := serveHealth(t, handler)
	var body struct {
		RunningRuns          int  `json:"running_runs"`
		DeskPendingMutations int  `json:"desk_pending_mutations"`
		SelfdevActiveOps     int  `json:"selfdev_active_operations"`
		LiveCapsules         *int `json:"live_capsules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.RunningRuns != 0 || body.DeskPendingMutations != 0 || body.SelfdevActiveOps != 0 {
		t.Fatalf("the state is not blind to the old fields: %s", rec.Body.String())
	}
	if body.LiveCapsules == nil || *body.LiveCapsules != 1 {
		t.Fatalf("live_capsules = %v, want 1: %s", body.LiveCapsules, rec.Body.String())
	}
}

// One counter: /health and the planned-restart refusal read the same
// function, and a terminal assignment is not live in either.
func TestHealthAndPlannedRestartShareOneLiveCapsuleCount(t *testing.T) {
	handler, s := newBoundedHealthFixture(t)
	seedLiveCapsule(t, handler, s)
	ctx := context.Background()
	work, err := handler.rt.plannedRestartBusy(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	counted, err := handler.rt.liveCapsuleCount(ctx)
	if err != nil || counted != work.LiveCapsules || counted != 1 {
		t.Fatalf("liveCapsuleCount = %d, %v; plannedRestartBusy = %d; want both 1", counted, err, work.LiveCapsules)
	}
	if got, _ := healthLiveCapsules(t, handler); got == nil || *got != counted {
		t.Fatalf("/health live_capsules = %v, want %d", got, counted)
	}
}

// The count cannot be read: /health fails closed (503 store_stalled naming
// it), never an idle-looking 0. The stalled-store tests hold the connection;
// this one pins that live_capsules is among the reads that can be unanswered.
func TestHealthLiveCapsulesIsAFailClosedStoreRead(t *testing.T) {
	handler, productStore := newBoundedHealthFixture(t)
	release := holdStore(t, productStore)
	defer release()
	rec, _ := serveHealth(t, handler)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("held store: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"live_capsules"`) {
		t.Fatalf("503 does not name live_capsules among the unanswered reads: %s", rec.Body.String())
	}
}
