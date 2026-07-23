package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type lateCompletionProvider struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
	once     sync.Once
}

func newLateCompletionProvider() *lateCompletionProvider {
	return &lateCompletionProvider{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (p *lateCompletionProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	_ = ctx
	_ = emit
	close(p.started)
	<-p.release
	task.Result = "late completion"
	close(p.finished)
	return nil
}

func (p *lateCompletionProvider) ProviderName() string { return "late-completion-test" }

func (p *lateCompletionProvider) unblock() {
	p.once.Do(func() { close(p.release) })
}

func waitForTerminalRun(t *testing.T, rt *Runtime, runID string) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec, err := rt.store.GetRun(context.Background(), runID)
		if err == nil && rec.State.Terminal() {
			return rec
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %s did not become terminal", runID)
	return types.RunRecord{}
}

func TestCancelResidentRunReleasesImmediatelyAndRejectsLateCompletion(t *testing.T) {
	rt, _ := testRuntime(t)
	provider := newLateCompletionProvider()
	rt.provider = provider
	rt.cfg.ActivationBudget = time.Hour
	t.Cleanup(provider.unblock)

	rec, err := rt.StartRun(context.Background(), "block until cancelled", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	<-provider.started
	if got := rt.RunningCount(); got != 1 {
		t.Fatalf("running count before cancel = %d, want 1", got)
	}

	if err := rt.CancelRun(context.Background(), rec.RunID, "user-alice"); err != nil {
		t.Fatalf("cancel resident run: %v", err)
	}
	stored, err := rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get cancelled run: %v", err)
	}
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("cancelled run = state %q finished_at %v", stored.State, stored.FinishedAt)
	}
	if got := rt.RunningCount(); got != 0 {
		t.Fatalf("running count after cancel = %d, want 0", got)
	}

	provider.unblock()
	<-provider.finished
	rt.wg.Wait()
	stored, err = rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get run after late completion: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("state after late completion = %q, want cancelled", stored.State)
	}
}

func TestIdlePassivationCannotOverwriteCancelledRun(t *testing.T) {
	rt, _ := testRuntime(t)
	stale, err := rt.createRunWithMetadata(context.Background(), "stale passivation", "user-alice", nil)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := rt.CancelRun(context.Background(), stale.RunID, "user-alice"); err != nil {
		t.Fatalf("cancel run: %v", err)
	}

	rt.passivateIdleToolLoopRun(
		context.Background(),
		stale,
		"late passivation",
		provideriface.TokenUsage{InputTokens: 3, OutputTokens: 5},
		&toolregistry.ToolLoopPassivatedError{Reason: "idle"},
	)

	stored, err := rt.store.GetRun(context.Background(), stale.RunID)
	if err != nil {
		t.Fatalf("get run after late passivation: %v", err)
	}
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("run after late passivation = state %q finished_at %v, want cancelled terminal state", stored.State, stored.FinishedAt)
	}
}

func TestActivationBudgetProgressDeadlineTerminalizesAndReleases(t *testing.T) {
	rt, _ := testRuntime(t)
	provider := newLateCompletionProvider()
	rt.provider = provider
	rt.cfg.ActivationBudget = 25 * time.Millisecond
	t.Cleanup(provider.unblock)

	rec, err := rt.StartRun(context.Background(), "outlive activation budget", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	<-provider.started
	stored := waitForTerminalRun(t, rt, rec.RunID)
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("progress deadline run = state %q finished_at %v", stored.State, stored.FinishedAt)
	}
	if !strings.Contains(stored.Error, "activation budget") || !strings.Contains(stored.Error, "progress deadline") {
		t.Fatalf("deadline error = %q, want activation budget and progress deadline", stored.Error)
	}
	if got := rt.RunningCount(); got != 0 {
		t.Fatalf("running count after progress deadline = %d, want 0", got)
	}

	provider.unblock()
	<-provider.finished
	rt.wg.Wait()
	stored, err = rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		t.Fatalf("get deadline run after late completion: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("deadline state after late completion = %q, want cancelled", stored.State)
	}
}

func TestRunListAndCancelRoutesAreWiredAndOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)
	alice, err := rt.createRunWithMetadata(context.Background(), "alice pending", "user-alice", nil)
	if err != nil {
		t.Fatalf("create alice run: %v", err)
	}
	bob, err := rt.createRunWithMetadata(context.Background(), "bob pending", "user-bob", nil)
	if err != nil {
		t.Fatalf("create bob run: %v", err)
	}

	listW := runtimeHandlerRequest(t, handler.HandleRunList, http.MethodGet, "/api/agent/loops?limit=20", "", "user-alice")
	if listW.Code != http.StatusOK {
		t.Fatalf("run list status = %d, want 200; body=%s", listW.Code, listW.Body.String())
	}
	var listResp runListResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode run list: %v", err)
	}
	if len(listResp.Runs) != 1 || listResp.Runs[0].RunID != alice.RunID {
		t.Fatalf("owner-scoped run list = %+v, want only %s", listResp.Runs, alice.RunID)
	}

	foreignW := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+bob.RunID+"/cancel", ``, "user-alice")
	if foreignW.Code != http.StatusNotFound {
		t.Fatalf("foreign cancel status = %d, want 404; body=%s", foreignW.Code, foreignW.Body.String())
	}

	cancelW := runtimeHandlerRequest(t, handler.HandleRunResource, http.MethodPost, "/api/runs/"+alice.RunID+"/cancel", ``, "user-alice")
	if cancelW.Code != http.StatusOK {
		t.Fatalf("owner cancel status = %d, want 200; body=%s", cancelW.Code, cancelW.Body.String())
	}
	cancelled, err := rt.store.GetRun(context.Background(), alice.RunID)
	if err != nil {
		t.Fatalf("get route-cancelled run: %v", err)
	}
	if cancelled.State != types.RunCancelled || cancelled.FinishedAt == nil {
		t.Fatalf("route-cancelled run = state %q finished_at %v", cancelled.State, cancelled.FinishedAt)
	}
}

func TestCancelLifecycleTrajectoryPersistsCancelledActivationProjection(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-lifecycle-cancel"
		docID   = "doc-lifecycle-cancel"
		runID   = "run-lifecycle-cancel"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: runID, AgentID: currentTextureAgentID(docID), OwnerID: ownerID,
		SandboxID: "sandbox-test", TrajectoryID: trajectoryID,
		State: types.RunRunning, Prompt: "durable cancellation projection",
		AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: "texture", runMetadataAgentRole: "texture",
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create lifecycle activation: %v", err)
	}
	before, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
	if err != nil {
		t.Fatalf("snapshot before cancellation: %v", err)
	}
	result, cancelled, err := rt.CancelTrajectoryCommand(
		ctx, trajectoryID, ownerID, "command-cancel-lifecycle-activation", "owner cancelled",
		before.Trajectory.LifecycleVersion, before.HeadRevision.RevisionID,
	)
	if err != nil {
		t.Fatalf("cancel lifecycle trajectory: %v", err)
	}
	if result.Trajectory.Status != types.TrajectoryCancelled || len(cancelled) != 1 || cancelled[0] != runID {
		t.Fatalf("cancellation result = %+v, runs %v", result.Trajectory, cancelled)
	}
	stored, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", runID)
	if err != nil {
		t.Fatalf("get cancelled activation: %v", err)
	}
	if stored.State != types.RunCancelled || stored.FinishedAt == nil {
		t.Fatalf("cancelled activation = %+v", stored)
	}
	after, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
	if err != nil {
		t.Fatalf("snapshot after cancellation: %v", err)
	}
	if after.Activation.RunID != runID || after.Activation.State != types.RunCancelled {
		t.Fatalf("cancelled lifecycle projection = %+v", after.Activation)
	}
	if after.Trajectory.ReducerSeq != result.Trajectory.ReducerSeq {
		t.Fatalf("activation projection advanced cancelled reducer seq: result=%d snapshot=%d", result.Trajectory.ReducerSeq, after.Trajectory.ReducerSeq)
	}
	lateCompletion := stored
	lateCompletion.State = types.RunCompleted
	lateCompletion.Result = "provider completed after cancellation"
	lateCompletion.UpdatedAt = time.Now().UTC()
	lateCompletion.FinishedAt = &lateCompletion.UpdatedAt
	if err := s.UpdateRun(ctx, lateCompletion); err == nil {
		t.Fatal("late provider completion overwrote cancelled lifecycle activation")
	}
	stillCancelled, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", runID)
	if err != nil || stillCancelled.State != types.RunCancelled {
		t.Fatalf("activation after late completion = %+v, %v", stillCancelled, err)
	}
}

func TestLifecycleCancellationWinsActivationCompletionRace(t *testing.T) {
	for round := range 10 {
		rt, s := testRuntime(t)
		ctx := context.Background()
		suffix := fmt.Sprintf("%02d", round)
		ownerID, docID, runID := "user-cancel-race-"+suffix, "doc-cancel-race-"+suffix, "run-cancel-race-"+suffix
		trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
		now := time.Now().UTC()
		running := types.RunRecord{
			RunID: runID, AgentID: currentTextureAgentID(docID), OwnerID: ownerID,
			SandboxID: "sandbox-test", TrajectoryID: trajectoryID,
			State: types.RunRunning, Prompt: "race cancellation with completion",
			AgentProfile: "texture", AgentRole: "texture", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{
				runMetadataAgentProfile: "texture", runMetadataAgentRole: "texture",
				runMetadataTrajectoryID: trajectoryID,
			},
		}
		if err := s.CreateRun(ctx, running); err != nil {
			t.Fatalf("round %d create activation: %v", round, err)
		}
		snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
		if err != nil {
			t.Fatalf("round %d snapshot: %v", round, err)
		}
		completed := running
		completed.State, completed.Result = types.RunCompleted, "provider completed"
		completed.UpdatedAt = time.Now().UTC()
		completed.FinishedAt = &completed.UpdatedAt
		startRace := make(chan struct{})
		var cancelErr, completionErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-startRace
			_, _, cancelErr = rt.CancelTrajectoryCommand(
				ctx, trajectoryID, ownerID, "command-cancel-race-"+suffix, "owner cancelled",
				snapshot.Trajectory.LifecycleVersion, snapshot.HeadRevision.RevisionID,
			)
		}()
		go func() {
			defer wg.Done()
			<-startRace
			completionErr = s.UpdateRun(ctx, completed)
		}()
		close(startRace)
		wg.Wait()
		if cancelErr != nil {
			t.Fatalf("round %d cancellation: %v (completion=%v)", round, cancelErr, completionErr)
		}
		stored, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", runID)
		if err != nil || stored.State != types.RunCancelled {
			t.Fatalf("round %d terminal projection = %+v, %v (completion=%v)", round, stored, err, completionErr)
		}
	}
}

func TestRuntimeRunListsIncludeLifecycleProjectionWithinComputerScope(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-lifecycle-list"
		docID   = "doc-lifecycle-list"
		runID   = "run-lifecycle-list"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: runID, AgentID: currentTextureAgentID(docID), OwnerID: ownerID,
		SandboxID: "sandbox-test", TrajectoryID: trajectoryID, ChannelID: docID,
		State: types.RunRunning, Prompt: "list canonical lifecycle activation",
		AgentProfile: "texture", AgentRole: "texture", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: "texture", runMetadataAgentRole: "texture",
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create lifecycle activation: %v", err)
	}

	assertListed := func(label string, runs []types.RunRecord, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		for _, run := range runs {
			if run.RunID == runID {
				return
			}
		}
		t.Fatalf("%s omitted lifecycle run %q: %+v", label, runID, runs)
	}
	runs, err := rt.ListRunsByOwner(ctx, ownerID, 10)
	assertListed("list by owner", runs, err)
	runs, err = rt.ListRunsByChannel(ctx, ownerID, docID, 10)
	assertListed("list by channel", runs, err)
}

func TestCancelAgentDoesNotCrossComputerLifecycleScope(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "owner-cancel-scope"
		docID   = "doc-cancel-scope"
		agentID = "texture:" + docID
	)
	seed := func(computerID, suffix string) types.RunRecord {
		t.Helper()
		now := time.Now().UTC()
		trajectoryID := "trajectory-cancel-scope-" + suffix
		req := types.StartLifecycleRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "start-cancel-scope-" + suffix,
			TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
			SettlementRule: types.SettlementRule{
				Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
				RequiredSubjectRefs: []string{"artifact"},
			},
			SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID},
			InitialWork:     types.WorkItemRecord{WorkItemID: "work-cancel-scope-" + suffix, Objective: "remain scoped", AssignedAgentID: agentID},
			InitialDocument: types.Document{DocID: docID, Title: "Scoped cancellation " + suffix},
			InitialRevision: types.Revision{
				RevisionID: "revision-cancel-scope-" + suffix, AuthorKind: types.AuthorUser,
				AuthorLabel: ownerID, Content: "Scoped cancellation fixture",
			},
			Agent: types.AgentRecord{
				AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID,
				Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
			},
		}
		req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
		if _, err := s.StartLifecycle(ctx, req); err != nil {
			t.Fatalf("start %s lifecycle: %v", suffix, err)
		}
		run := types.RunRecord{
			RunID: "run-cancel-scope-" + suffix, AgentID: agentID, OwnerID: ownerID,
			SandboxID: computerID, TrajectoryID: trajectoryID, ChannelID: docID,
			State: types.RunPending, Prompt: "scoped cancellation fixture",
			AgentProfile: "texture", AgentRole: "texture", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, "lifecycle_work_item_id": req.InitialWork.WorkItemID},
		}
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatalf("create %s lifecycle activation: %v", suffix, err)
		}
		return run
	}

	local := seed("sandbox-test", "local")
	foreign := seed("computer-foreign", "foreign")
	trajectories, err := rt.ListTrajectoriesByOwner(ctx, ownerID, 20)
	if err != nil {
		t.Fatalf("list computer-scoped trajectories: %v", err)
	}
	foundLocal := false
	for _, trajectory := range trajectories {
		if trajectory.TrajectoryID == foreign.TrajectoryID {
			t.Fatalf("foreign trajectory leaked into runtime list: %+v", trajectory)
		}
		if trajectory.TrajectoryID == local.TrajectoryID {
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Fatalf("local trajectory omitted from runtime list: %+v", trajectories)
	}
	if err := rt.CancelAgent(ctx, agentID, ownerID); err != nil {
		t.Fatalf("cancel local lifecycle agent: %v", err)
	}
	localStored, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", local.RunID)
	if err != nil || localStored.State != types.RunCancelled {
		t.Fatalf("local cancellation = %+v, %v", localStored, err)
	}
	foreignStored, err := s.GetLifecycleRun(ctx, ownerID, "computer-foreign", foreign.RunID)
	if err != nil || foreignStored.State != types.RunPending {
		t.Fatalf("foreign activation changed = %+v, %v", foreignStored, err)
	}
}
