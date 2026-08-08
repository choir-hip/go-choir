package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func ownerInstructionRequest(t *testing.T, s *Store, start types.StartLifecycleRequest, suffix, content string) types.QueueLifecycleOwnerInstructionRequest {
	t.Helper()
	snapshot, err := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	req := types.QueueLifecycleOwnerInstructionRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "owner-command-" + suffix,
		RequestID: "owner-request-" + suffix, InstructionID: "owner-instruction-" + suffix,
		DocumentID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		TargetAgentID: start.Agent.AgentID, TargetWorkItemID: start.InitialWork.WorkItemID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
		Kind: types.LifecycleOwnerTell, Content: content,
	}
	req.CommandDigest, _ = ComputeQueueLifecycleOwnerInstructionDigest(req)
	return req
}

func TestQueueLifecycleOwnerInstructionReplayConflictOccurrenceAndAtomicTurnConsume(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	first := ownerInstructionRequest(t, s, start, "one", "same prose")
	result, err := s.QueueLifecycleOwnerInstruction(ctx, first)
	if err != nil || result.OwnerInstruction == nil || result.Replay || len(result.Events) != 1 ||
		result.Events[0].Kind != types.LifecycleOwnerInstructionQueued || result.Events[0].RequestID != first.RequestID {
		t.Fatalf("queue owner instruction = %+v, %v", result, err)
	}
	replay, err := s.QueueLifecycleOwnerInstruction(ctx, first)
	if err != nil || !replay.Replay || replay.OwnerInstruction == nil || replay.OwnerInstruction.InstructionID != first.InstructionID {
		t.Fatalf("owner instruction replay = %+v, %v", replay, err)
	}
	conflict := first
	conflict.Content = "changed prose"
	conflict.CommandDigest, _ = ComputeQueueLifecycleOwnerInstructionDigest(conflict)
	if _, err := s.QueueLifecycleOwnerInstruction(ctx, conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("same occurrence conflict err=%v", err)
	}
	second := ownerInstructionRequest(t, s, start, "two", "same prose")
	if _, err := s.QueueLifecycleOwnerInstruction(ctx, second); err != nil {
		t.Fatalf("same prose distinct occurrence: %v", err)
	}
	pending, err := s.ListPendingLifecycleOwnerInstructions(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pending) != 2 || pending[0].InstructionID != first.InstructionID || pending[1].InstructionID != second.InstructionID {
		t.Fatalf("ordered pending = %+v, %v", pending, err)
	}
	turn := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnNoSemanticChange)
	turn.CommandID = "turn-consuming-owner-one"
	turn.OwnerInstructions = []types.TextureTurnOwnerInstruction{
		{InstructionID: first.InstructionID, RequestID: first.RequestID},
		{InstructionID: second.InstructionID, RequestID: second.RequestID},
	}
	setTextureTurnDigest(t, &turn, TextureSourceGraphWriteSet{})
	turnResult, err := s.ApplyTextureTurn(ctx, turn)
	if err != nil || len(turnResult.Events) == 0 || len(turnResult.Events[0].RequestIDs) != 2 || turnResult.Events[0].RequestIDs[0] != first.RequestID || turnResult.Events[0].RequestIDs[1] != second.RequestID {
		t.Fatalf("turn consuming owner instruction = %+v, %v", turnResult, err)
	}
	turnReplay, err := s.ApplyTextureTurn(ctx, turn)
	if err != nil || !turnReplay.Replay {
		t.Fatalf("turn replay=%+v err=%v", turnReplay, err)
	}
	reordered := turn
	reordered.OwnerInstructions = []types.TextureTurnOwnerInstruction{turn.OwnerInstructions[1], turn.OwnerInstructions[0]}
	setTextureTurnDigest(t, &reordered, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, reordered); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("reordered occurrence set err=%v", err)
	}
	pending, err = s.ListPendingLifecycleOwnerInstructions(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after atomic multi-consume = %+v, %v", pending, err)
	}
	_, consumed, err := s.lifecycleOwnerInstructionObject(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, first.InstructionID)
	if err != nil || consumed.Status != types.LifecycleOwnerInstructionConsumed || consumed.ConsumedAt == nil {
		t.Fatalf("consumed record = %+v, %v", consumed, err)
	}
	_, consumedSecond, err := s.lifecycleOwnerInstructionObject(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, second.InstructionID)
	if err != nil || consumedSecond.Status != types.LifecycleOwnerInstructionConsumed || consumedSecond.ConsumedAt == nil {
		t.Fatalf("second consumed record = %+v, %v", consumedSecond, err)
	}
}

func TestLifecycleOwnerInstructionScopeHeadTargetAndRestartDurability(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owner-instruction.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := first.StartLifecycle(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	req := ownerInstructionRequest(t, first, start, "restart", "survive restart")
	before, _ := first.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	for name, mutate := range map[string]func(*types.QueueLifecycleOwnerInstructionRequest){
		"wrong head":         func(r *types.QueueLifecycleOwnerInstructionRequest) { r.ExpectedHeadRevisionID = "wrong" },
		"wrong target work":  func(r *types.QueueLifecycleOwnerInstructionRequest) { r.TargetWorkItemID = "wrong" },
		"wrong target agent": func(r *types.QueueLifecycleOwnerInstructionRequest) { r.TargetAgentID = "texture:wrong" },
	} {
		candidate := req
		candidate.CommandID += "-" + name
		mutate(&candidate)
		candidate.CommandDigest, _ = ComputeQueueLifecycleOwnerInstructionDigest(candidate)
		if _, err := first.QueueLifecycleOwnerInstruction(context.Background(), candidate); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	after, _ := first.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if after.Trajectory.ReducerSeq != before.Trajectory.ReducerSeq {
		t.Fatalf("refusals mutated trajectory")
	}
	if _, err := first.QueueLifecycleOwnerInstruction(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	pending, err := second.ListPendingLifecycleOwnerInstructions(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pending) != 1 || pending[0].Content != "survive restart" {
		t.Fatalf("restart pending=%+v err=%v", pending, err)
	}
	if cross, err := second.ListPendingLifecycleOwnerInstructions(context.Background(), "other-owner", start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10); err != nil || len(cross) != 0 {
		t.Fatalf("cross owner pending=%+v err=%v", cross, err)
	}
}
