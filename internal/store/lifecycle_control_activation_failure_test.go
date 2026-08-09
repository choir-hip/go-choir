package store

import (
	"context"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func lifecycleControlFailureFixture(t *testing.T, retainCanonicalWorkBinding bool) (*Store, types.StartLifecycleRequest, types.RunRecord, types.BindLifecycleControlDeliveryRequest, types.FailLifecycleControlActivationRequest) {
	t.Helper()
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	turnReq := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	turnReq.CommandID = "turn-for-activation-failure"
	turnReq.Controls = []types.TextureTurnControl{
		textureTurnControl(t, "control-activation-failure-a", researcherWork.AssignedAgentID, researcherWork.WorkItemID),
		textureTurnControl(t, "control-activation-failure-b", researcherWork.AssignedAgentID, researcherWork.WorkItemID),
	}
	setTextureTurnDigest(t, &turnReq, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(context.Background(), turnReq)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, "run-researcher-target")
	if err != nil {
		t.Fatal(err)
	}
	run.Metadata = lifecycleControlFailureMetadataCopy(run.Metadata)
	if !retainCanonicalWorkBinding {
		delete(run.Metadata, "lifecycle_work_item_id")
		delete(run.Metadata, "work_item_ids")
	}
	run.Metadata["request_source"] = "lifecycle_texture_control"
	run.Metadata["lifecycle_logical_activation_key"] = "logical-failure-key"
	run.Metadata["lifecycle_failed_attempt_key"] = "failed-attempt-key"
	bind := bindControlRequestForTest(t, s, start, run, turn.Controls)
	bind.CommandID = "bind-control-delivery:" + run.RunID + ":" + turn.Controls[0].UpdateID + "," + turn.Controls[1].UpdateID
	versions := make([]types.LifecycleControlActivationVersion, 0, len(bind.Controls))
	for _, item := range bind.Controls {
		versions = append(versions, types.LifecycleControlActivationVersion{UpdateID: item.UpdateID, TargetWorkItemID: item.TargetWorkItemID, ControlLifecycleVersion: item.ExpectedControlLifecycleVersion, WorkLifecycleVersion: item.ExpectedWorkLifecycleVersion})
	}
	run.Metadata["lifecycle_activation_build_commit"] = "build-test"
	run.Metadata["lifecycle_activation_versions"] = versions
	if err := s.UpdateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	bind.ActivationRefresh = &types.LifecycleControlActivationRefresh{Prompt: run.Prompt, LogicalActivationKey: "logical-failure-key", FailedAttemptKey: "failed-attempt-key", BuildCommit: "build-test", Versions: versions, WorkItemIDs: []string{researcherWork.WorkItemID}}
	bind.CommandDigest, _ = ComputeBindLifecycleControlDeliveryDigest(bind)
	failure := types.FailLifecycleControlActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: LifecycleControlActivationFailureCommandID("failed-attempt-key"),
		TrajectoryID: start.TrajectoryID, AgentID: run.AgentID, RunID: run.RunID, ExpectedLifecycleVersion: bind.ExpectedLifecycleVersion,
		LogicalActivationKey: "logical-failure-key", FailedAttemptKey: "failed-attempt-key", BindCommandID: bind.CommandID, BindCommandDigest: bind.CommandDigest,
		Controls: bind.Controls, ActivationRefresh: bind.ActivationRefresh, Failure: LifecycleControlActivationMissingRunWorkBindingFailure,
	}
	failure.CommandDigest, _ = ComputeFailLifecycleControlActivationDigest(failure)
	return s, start, run, bind, failure
}

func TestFailLifecycleControlActivationAtomicReceiptAndReplay(t *testing.T) {
	s, start, run, bind, failure := lifecycleControlFailureFixture(t, false)
	before, err := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindLifecycleControlDelivery(context.Background(), bind); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("bind error=%v", err)
	}
	result, err := s.FailLifecycleControlActivation(context.Background(), failure)
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.Kind != types.LifecycleFailControlActivation || len(result.Events) != 1 || result.Events[0].Kind != types.LifecycleControlActivationFailed || result.Events[0].FailedAttemptKey != failure.FailedAttemptKey {
		t.Fatalf("typed result=%+v", result)
	}
	stored, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
	if err != nil || stored.State != types.RunFailed || stored.Error != LifecycleControlActivationMissingRunWorkBindingFailure {
		t.Fatalf("failed run=%+v err=%v", stored, err)
	}
	versions := make([]types.LifecycleControlActivationVersion, 0, len(failure.Controls))
	for _, item := range failure.Controls {
		versions = append(versions, types.LifecycleControlActivationVersion{UpdateID: item.UpdateID, TargetWorkItemID: item.TargetWorkItemID, ControlLifecycleVersion: item.ExpectedControlLifecycleVersion, WorkLifecycleVersion: item.ExpectedWorkLifecycleVersion})
	}
	resolved, err := s.ResolveLifecycleControlActivation(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, failure.LogicalActivationKey, failure.FailedAttemptKey, versions)
	if err != nil || resolved.DurablyFailed == nil || resolved.DurablyFailed.RunID != run.RunID || resolved.Active != nil {
		t.Fatalf("typed resolver=%+v err=%v", resolved, err)
	}
	agent, err := s.GetAgentByScope(context.Background(), start.OwnerID, start.ComputerID, run.AgentID)
	if err != nil || agent.ActiveRunID != "" {
		t.Fatalf("cleared agent=%+v err=%v", agent, err)
	}
	after, err := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || after.Trajectory.ReducerSeq != before.Trajectory.ReducerSeq+1 || after.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion+1 {
		t.Fatalf("trajectory before=%+v after=%+v err=%v", before.Trajectory, after.Trajectory, err)
	}
	replay, err := s.FailLifecycleControlActivation(context.Background(), failure)
	if err != nil || !replay.Replay || replay.Receipt.Kind != types.LifecycleFailControlActivation {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

func TestFailLifecycleControlActivationRefusesNonMissingBindingAndVersionRace(t *testing.T) {
	t.Run("run binds work at initial canonical projection", func(t *testing.T) {
		s, start, run, _, failure := lifecycleControlFailureFixture(t, true)
		if _, err := s.FailLifecycleControlActivation(context.Background(), failure); !errors.Is(err, ErrLifecycleInvalidTransition) {
			t.Fatalf("error=%v", err)
		}
		stored, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
		if err != nil || stored.State != types.RunRunning {
			t.Fatalf("bound canonical run=%+v err=%v", stored, err)
		}
	})
	t.Run("generic update cannot mint missing work binding authority", func(t *testing.T) {
		s, start, run, _, failure := lifecycleControlFailureFixture(t, false)
		stored, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		workID := failure.Controls[0].TargetWorkItemID
		stored.Metadata["lifecycle_work_item_id"] = workID
		stored.Metadata["work_item_ids"] = []string{workID}
		if err := s.UpdateRun(context.Background(), stored); err != nil {
			t.Fatal(err)
		}
		reloaded, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if _, present := reloaded.Metadata["lifecycle_work_item_id"]; present {
			t.Fatalf("generic writer minted lifecycle_work_item_id: %+v", reloaded.Metadata)
		}
		if _, present := reloaded.Metadata["work_item_ids"]; present {
			t.Fatalf("generic writer minted work_item_ids: %+v", reloaded.Metadata)
		}
	})
	t.Run("canonical version changed", func(t *testing.T) {
		s, start, run, _, failure := lifecycleControlFailureFixture(t, false)
		work, err := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, failure.Controls[0].TargetWorkItemID)
		if err != nil {
			t.Fatal(err)
		}
		work.Reason = "canonical amendment raced failure persistence"
		amend := types.AmendLifecycleWorkRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "amend-before-control-activation-failure", TrajectoryID: start.TrajectoryID, WorkItemID: work.WorkItemID, ExpectedLifecycleVersion: work.LifecycleVersion, WorkItem: work}
		amend.CommandDigest, _ = ComputeAmendLifecycleWorkDigest(amend)
		if _, err := s.AmendLifecycleWork(context.Background(), amend); err != nil {
			t.Fatal(err)
		}
		if _, err := s.FailLifecycleControlActivation(context.Background(), failure); !errors.Is(err, ErrConcurrentStateChange) {
			t.Fatalf("error=%v", err)
		}
		stored, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
		if err != nil || stored.State != types.RunRunning {
			t.Fatalf("raced run=%+v err=%v", stored, err)
		}
		agent, err := s.GetAgentByScope(context.Background(), start.OwnerID, start.ComputerID, run.AgentID)
		if err != nil || agent.ActiveRunID != run.RunID {
			t.Fatalf("raced agent=%+v err=%v", agent, err)
		}
	})
}

func TestResolveLifecycleControlActivationRejectsMetadataOnlyFailure(t *testing.T) {
	s, start, run, bind, failure := lifecycleControlFailureFixture(t, false)
	now := run.UpdatedAt
	fake := run
	fake.RunID = "metadata-only-failed-run"
	fake.State, fake.Error, fake.FinishedAt = types.RunFailed, LifecycleControlActivationMissingRunWorkBindingFailure, &now
	fake.Metadata = lifecycleControlFailureMetadataCopy(fake.Metadata)
	fake.Metadata["lifecycle_control_bind_failed"] = true
	fake.Metadata["lifecycle_failed_attempt_key"] = failure.FailedAttemptKey
	fake.Metadata["lifecycle_control_activation_failure_command_id"] = LifecycleControlActivationFailureCommandID(failure.FailedAttemptKey)
	fake.Metadata["lifecycle_control_activation_failure_command_digest"] = failure.CommandDigest
	if err := s.CreateRun(context.Background(), fake); err != nil {
		t.Fatal(err)
	}
	versions := make([]types.LifecycleControlActivationVersion, 0, len(bind.Controls))
	for _, item := range bind.Controls {
		versions = append(versions, types.LifecycleControlActivationVersion{UpdateID: item.UpdateID, TargetWorkItemID: item.TargetWorkItemID, ControlLifecycleVersion: item.ExpectedControlLifecycleVersion, WorkLifecycleVersion: item.ExpectedWorkLifecycleVersion})
	}
	resolved, err := s.ResolveLifecycleControlActivation(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, failure.LogicalActivationKey, failure.FailedAttemptKey, versions)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.DurablyFailed != nil {
		t.Fatalf("metadata-only failure accepted: %+v", resolved.DurablyFailed)
	}
}
