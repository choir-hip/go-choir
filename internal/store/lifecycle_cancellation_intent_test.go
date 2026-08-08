package store

import (
	"context"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestLifecycleCancellationIntentIsReceiptedReplayableAndPreservesOriginalCASIdentity(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	before, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	req := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-with-durable-intent",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion,
		RequestedLifecycleVersion: before.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:    before.HeadRevision.RevisionID, Reason: "owner cancelled exact assignment work",
	}
	req.CommandDigest, err = ComputeCancelLifecycleDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := s.PrepareLifecycleCancellation(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if intent.CommandDigest != req.CommandDigest || intent.RequestedLifecycleVersion != before.Trajectory.LifecycleVersion {
		t.Fatalf("intent=%+v", intent)
	}
	afterIntent, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if afterIntent.Trajectory.Status != types.TrajectoryLive || afterIntent.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion+1 {
		t.Fatalf("trajectory after intent=%+v", afterIntent.Trajectory)
	}
	if len(afterIntent.Events) == 0 || afterIntent.Events[len(afterIntent.Events)-1].Kind != types.LifecycleTrajectoryCancellationRequested {
		t.Fatalf("intent events=%+v", afterIntent.Events)
	}
	receiptObj, err := s.lifecycleGetObject(ctx, ogKindLifecycleCmd, start.OwnerID, start.ComputerID, req.CommandID+":intent")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](receiptObj)
	if err != nil || receipt.Kind != types.LifecyclePrepareCancelTrajectory || len(receipt.ResultEventRefs) != 1 {
		t.Fatalf("intent receipt=%+v err=%v", receipt, err)
	}
	if replay, err := s.PrepareLifecycleCancellation(ctx, req); err != nil || replay.CommandDigest != intent.CommandDigest {
		t.Fatalf("intent replay=%+v err=%v", replay, err)
	}
	afterReplay, _ := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if afterReplay.LifecycleVersion != afterIntent.Trajectory.LifecycleVersion {
		t.Fatalf("intent replay advanced lifecycle version: %d -> %d", afterIntent.Trajectory.LifecycleVersion, afterReplay.LifecycleVersion)
	}
	conflict := req
	conflict.Reason = "different cancellation authority"
	conflict.CommandDigest, _ = ComputeCancelLifecycleDigest(conflict)
	if _, err := s.PrepareLifecycleCancellation(ctx, conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed intent error=%v", err)
	}
	final := req
	final.ExpectedLifecycleVersion = afterIntent.Trajectory.LifecycleVersion
	result, err := s.CancelLifecycleTrajectory(ctx, final)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trajectory.Status != types.TrajectoryCancelled || result.Receipt.CommandDigest != req.CommandDigest || result.Receipt.Kind != types.LifecycleCancelTrajectory {
		t.Fatalf("final cancellation=%+v", result)
	}
}
