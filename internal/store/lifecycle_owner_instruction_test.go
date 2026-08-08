package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
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
		result.OwnerInstruction.Schema != types.LifecycleOwnerInstructionSchemaV1 || result.Events[0].Kind != types.LifecycleOwnerInstructionQueued || result.Events[0].RequestID != first.RequestID || len(result.Events[0].ArtifactRefs) != 1 || !strings.HasPrefix(result.Events[0].ArtifactRefs[0], "obj:choir.owner_instruction:") {
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

func TestDirectOwnerHeadAndConcurrentTellCASPreserveEveryLinearizedOccurrence(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	seeded := ownerInstructionRequest(t, s, start, "race-seeded", "intent pending before direct edit")
	if _, err := s.QueueLifecycleOwnerInstruction(ctx, seeded); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	racing := types.QueueLifecycleOwnerInstructionRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "owner-command-racing",
		RequestID: "owner-request-racing", InstructionID: "owner-instruction-racing",
		DocumentID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		TargetAgentID: start.Agent.AgentID, TargetWorkItemID: start.InitialWork.WorkItemID,
		ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: before.Document.CurrentRevisionID,
		Kind: types.LifecycleOwnerTell, Content: "intent racing direct edit",
	}
	racing.CommandDigest, _ = ComputeQueueLifecycleOwnerInstructionDigest(racing)
	edit := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "direct-owner-race",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: before.Document.CurrentRevisionID,
		Revision:               types.Revision{RevisionID: "direct-owner-race-v1", AuthorKind: types.AuthorUser, AuthorLabel: start.OwnerID, Content: "direct owner edit"},
		OwnerCorrection: &types.CommitLifecycleOwnerCorrection{
			RequestID: "direct-owner-race-request", InstructionID: "direct-owner-race-instruction",
			TargetAgentID: start.Agent.AgentID, TargetWorkItemID: start.InitialWork.WorkItemID,
			Content: "reconcile the exact direct owner edit",
		},
	}
	edit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(edit, TextureSourceGraphWriteSet{})
	type editResult struct {
		result types.LifecycleResult
		err    error
	}
	startRace := make(chan struct{})
	tellDone := make(chan error, 1)
	editDone := make(chan editResult, 1)
	go func() {
		<-startRace
		_, queueErr := s.QueueLifecycleOwnerInstruction(ctx, racing)
		tellDone <- queueErr
	}()
	go func() {
		<-startRace
		result, commitErr := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, edit, TextureSourceGraphWriteSet{})
		editDone <- editResult{result: result, err: commitErr}
	}()
	close(startRace)
	tellErr := <-tellDone
	committed := <-editDone
	if tellErr != nil && committed.err != nil {
		t.Fatalf("both racing commands failed: tell=%v edit=%v", tellErr, committed.err)
	}
	if tellErr == nil && committed.err == nil {
		t.Fatal("both stale-version racing commands committed")
	}
	if committed.err != nil {
		if !errors.Is(committed.err, ErrConcurrentStateChange) {
			t.Fatalf("unexpected edit race error: %v", committed.err)
		}
		latest, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		edit.ExpectedLifecycleVersion = latest.Trajectory.LifecycleVersion
		edit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(edit, TextureSourceGraphWriteSet{})
		committed.result, committed.err = s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, edit, TextureSourceGraphWriteSet{})
		if committed.err != nil {
			t.Fatalf("retry direct edit after linearized tell: %v", committed.err)
		}
	} else {
		if !errors.Is(tellErr, ErrConcurrentStateChange) {
			t.Fatalf("unexpected tell race error: %v", tellErr)
		}
		latest, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		racing.ExpectedLifecycleVersion = latest.Trajectory.LifecycleVersion
		racing.ExpectedHeadRevisionID = latest.Document.CurrentRevisionID
		racing.CommandDigest, _ = ComputeQueueLifecycleOwnerInstructionDigest(racing)
		if _, err := s.QueueLifecycleOwnerInstruction(ctx, racing); err != nil {
			t.Fatalf("retry tell against linearized direct head: %v", err)
		}
	}
	if committed.result.Revision == nil {
		t.Fatal("direct edit revision missing")
	}
	newHead := committed.result.Revision.RevisionID
	oldPending, err := s.ListPendingLifecycleOwnerInstructionsForHead(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, before.Document.CurrentRevisionID)
	if err != nil || len(oldPending) != 0 {
		t.Fatalf("race stranded old-head occurrences=%+v err=%v", oldPending, err)
	}
	pending, err := s.ListPendingLifecycleOwnerInstructionsForHead(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, newHead)
	if err != nil || len(pending) != 3 {
		t.Fatalf("race lost occurrence: %+v err=%v", pending, err)
	}
	seen := map[string]bool{}
	for _, occurrence := range pending {
		seen[occurrence.InstructionID] = true
	}
	for _, id := range []string{seeded.InstructionID, racing.InstructionID, edit.OwnerCorrection.InstructionID} {
		if !seen[id] {
			t.Fatalf("race lost owner occurrence %q: %+v", id, pending)
		}
	}
	replay, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, edit, TextureSourceGraphWriteSet{})
	if err != nil || !replay.Replay || replay.Revision == nil || replay.Revision.RevisionID != newHead {
		t.Fatalf("direct race replay=%+v err=%v", replay, err)
	}
	conflict := edit
	conflict.Revision.Content = "changed direct edit"
	conflict.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(conflict, TextureSourceGraphWriteSet{})
	if _, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, conflict, TextureSourceGraphWriteSet{}); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed direct race replay err=%v", err)
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
