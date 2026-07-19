package selfdev

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestOperationStartIsDurableIdempotentAndHeadBound(t *testing.T) {
	store, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 18, 23, 30, 0, 0, time.UTC)
	digest := strings.Repeat("a", 64)
	if _, err := store.DB().Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, 1, ?, ?, ?, ?, ?, 1, 0, ?)`, "computer-test", digest, digest, digest, strings.Repeat("b", 64), strings.Repeat("b", 64), now); err != nil {
		t.Fatal(err)
	}
	operations, err := NewStore(store, store)
	if err != nil {
		t.Fatal(err)
	}
	operations.now = func() time.Time { return now }
	request := StartRequest{ComputerID: "computer-test", IdempotencyKey: "start-1", PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64)}
	first, err := operations.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := operations.Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.OperationID != retry.OperationID || first.RequestCommitment != retry.RequestCommitment || first.BaseHead != digest || first.State != StateRequested {
		t.Fatalf("idempotent operation mismatch: first=%+v retry=%+v", first, retry)
	}
	changed := request
	changed.PromptArtifactRef = "artifact:sha256:" + strings.Repeat("d", 64)
	if _, err := operations.Start(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed idempotent request error = %v", err)
	}
}

func TestOperationTransitionsRefuseSkippedAndStaleStates(t *testing.T) {
	if allowedTransition(StateRequested, StateApplied) {
		t.Fatal("requested operation skipped directly to applied")
	}
	if !allowedTransition(StateAwaitingApproval, StateRejected) || allowedTransition(StateRejected, StateExecuting) {
		t.Fatal("terminal rejection transition matrix is invalid")
	}
}

func TestRollbackStartBindsPriorAppliedReceiptsAndReplays(t *testing.T) {
	store, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	head := strings.Repeat("a", 64)
	if _, err := store.DB().Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, 1, ?, ?, ?, ?, ?, 1, 0, ?)`, "computer-test", head, head, head, strings.Repeat("b", 64), strings.Repeat("b", 64), now); err != nil {
		t.Fatal(err)
	}
	operations, err := NewStore(store, store)
	if err != nil {
		t.Fatal(err)
	}
	operations.now = func() time.Time { return now }
	target, err := operations.Start(context.Background(), StartRequest{ComputerID: "computer-test", IdempotencyKey: "target", PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64)})
	if err != nil {
		t.Fatal(err)
	}
	for _, transition := range [][2]string{
		{StateRequested, StateExecuting}, {StateExecuting, StateFrozen}, {StateFrozen, StateVerified},
		{StateVerified, StateAwaitingApproval}, {StateAwaitingApproval, StateAccepted}, {StateAccepted, StateMaterializing},
	} {
		target, err = operations.Transition(context.Background(), target.ComputerID, target.OperationID, transition[0], transition[1], nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	targetHead := strings.Repeat("d", 64)
	target, err = operations.Transition(context.Background(), target.ComputerID, target.OperationID, StateMaterializing, StateApplied, func(next *Operation) error {
		next.EffectiveHead = targetHead
		next.BundleDigest = strings.Repeat("e", 64)
		next.MaterializationReceipt = strings.Repeat("f", 64)
		next.ReleaseDigest = strings.Repeat("7", 64)
		next.CodeRef = "code:sha256:" + strings.Repeat("8", 64)
		next.ArtifactProgramRef = "artifact-program:sha256:" + strings.Repeat("9", 64)
		next.VerifierRefs = []string{strings.Repeat("a", 64)}
		next.CheckpointRef = "checkpoint:sha256:" + strings.Repeat("1", 64)
		next.RouteReceipt = "route-receipt-prior"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := RollbackStartRequest{
		ComputerID: "computer-test", IdempotencyKey: "rollback-1", RequestCommitment: strings.Repeat("2", 64),
		RollbackEvent: strings.Repeat("3", 64), DecisionActor: "owner",
		CurrentDesired: strings.Repeat("4", 64), CurrentEffective: strings.Repeat("5", 64),
		Target: target, RouteGeneration: 7,
	}
	first, err := operations.StartRollback(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := operations.StartRollback(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.OperationID != replay.OperationID || first.State != StateRollbackPending || first.RouteReceipt != target.RouteReceipt || first.BundleDigest != target.BundleDigest || first.CheckpointRef != target.CheckpointRef {
		t.Fatalf("rollback bindings mismatch: first=%+v replay=%+v", first, replay)
	}
	resolved, err := operations.GetByEffectiveHead(context.Background(), "computer-test", targetHead)
	if err != nil || resolved.OperationID != target.OperationID {
		t.Fatalf("resolve target: operation=%+v err=%v", resolved, err)
	}
	changed := request
	changed.RequestCommitment = strings.Repeat("6", 64)
	if _, err := operations.StartRollback(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed rollback commitment error = %v", err)
	}
}
