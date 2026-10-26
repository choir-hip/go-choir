package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Run-root trajectories (docs/problems/run-root-trajectories-never-settle-2026-10-26.md):
// the registry row minted for a run is created live with a rule that must be
// satisfiable, and it settles when its root run completes with nothing open.

func TestDefaultSettlementRuleRunRootRowsAreSatisfiable(t *testing.T) {
	for _, kind := range []types.TrajectoryKind{types.TrajectoryKindTask, types.TrajectoryKindDocument} {
		rule := defaultSettlementRuleForKind(kind)
		rec := types.TrajectoryRecord{
			Kind: kind, Status: types.TrajectoryLive,
			SubjectRefs:    map[string]string{"channel_id": "c", "root_loop_id": "r"},
			SettlementRule: rule,
		}
		if ready, waiting := EvaluateTrajectorySettlement(rec, 0); !ready {
			t.Fatalf("%s: a freshly minted run-root row can never validate: %v (rule %+v)", kind, waiting, rule)
		}
	}
	// A publication row keeps its own subject refs; the change must not touch it.
	pub := defaultSettlementRuleForKind(types.TrajectoryKindPublication)
	if len(pub.RequiredSubjectRefs) != 2 {
		t.Fatalf("publication rule changed: %+v", pub)
	}
}

func waitForTrajectoryStatus(t *testing.T, rt *Runtime, ownerID, trajectoryID string, want types.TrajectoryStatus) types.TrajectoryRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last types.TrajectoryRecord
	for time.Now().Before(deadline) {
		rec, err := rt.store.GetTrajectory(context.Background(), ownerID, trajectoryID)
		if err == nil {
			last = rec
			if rec.Status == want {
				return rec
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("trajectory %s did not reach %s; last=%+v", trajectoryID, want, last)
	return last
}

func TestRunRootTrajectorySettlesWhenItsRunCompletes(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	root, err := rt.StartRunWithMetadata(ctx, "build a document", "user-alice", map[string]any{
		runMetadataAgentProfile: agentprofile.Conductor,
		runMetadataAgentRole:    agentprofile.Conductor,
	})
	if err != nil {
		t.Fatalf("start root run: %v", err)
	}
	done := waitForRuntimeRunTerminal(t, rt, root.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Skipf("stub run ended %s, not completed", done.State)
	}
	settled := waitForTrajectoryStatus(t, rt, "user-alice", root.TrajectoryID, types.TrajectorySettled)
	if settled.SettledAt == nil {
		t.Fatalf("settled trajectory has no settled_at: %+v", settled)
	}
	got, err := s.GetTrajectory(ctx, "user-alice", root.TrajectoryID)
	if err != nil || got.Status != types.TrajectorySettled {
		t.Fatalf("stored trajectory = %+v, %v", got, err)
	}
}

func seedRunRoot(t *testing.T, rt *Runtime, trajectoryID string, state types.RunState, rule types.SettlementRule) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: trajectoryID, AgentID: "agent-" + trajectoryID, ChannelID: "chan-1",
		AgentProfile: agentprofile.Conductor, AgentRole: agentprofile.Conductor,
		OwnerID: "user-alice", ComputerID: "autoputer-test", State: state,
		Prompt: "root", CreatedAt: now, UpdatedAt: now, TrajectoryID: trajectoryID,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Conductor, runMetadataAgentRole: agentprofile.Conductor,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if state == types.RunCompleted {
		rec.FinishedAt = &now
	}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	if _, err := rt.store.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: trajectoryID, OwnerID: "user-alice", Kind: types.TrajectoryKindDocument,
		Status:         types.TrajectoryLive,
		SubjectRefs:    map[string]string{"channel_id": "chan-1", "root_loop_id": trajectoryID},
		SettlementRule: rule,
	}); err != nil {
		t.Fatalf("seed trajectory: %v", err)
	}
	return rec
}

func TestRunRootTrajectoryStaysLiveWhileWorkIsOpen(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := seedRunRoot(t, rt, "traj-root-open-work", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))

	work, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID: "user-alice", TrajectoryID: rec.TrajectoryID, Objective: "child objective",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	rt.settleRunRootTrajectory(ctx, &rec)
	got, _ := s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectoryLive {
		t.Fatalf("trajectory settled with an open work item: %+v", got)
	}

	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", work.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	rt.settleRunRootTrajectory(ctx, &rec)
	got, _ = s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectorySettled {
		t.Fatalf("trajectory did not settle once its work closed: %+v", got)
	}
}

func TestRunRootTrajectoryStaysLiveWhileAChildRunIsActive(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := seedRunRoot(t, rt, "traj-root-active-child", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))
	now := time.Now().UTC()
	child := types.RunRecord{
		RunID: "run-child-active", AgentID: "agent-child", ChannelID: "chan-1", RequestedByRunID: rec.RunID,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research,
		OwnerID: "user-alice", ComputerID: "autoputer-test", State: types.RunRunning,
		Prompt: "child", CreatedAt: now, UpdatedAt: now, TrajectoryID: rec.TrajectoryID,
		Metadata: map[string]any{runMetadataTrajectoryID: rec.TrajectoryID},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	rt.settleRunRootTrajectory(ctx, &rec)
	got, _ := s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectoryLive {
		t.Fatalf("trajectory settled under an active child run: %+v", got)
	}
}

func TestRunRootTrajectoryIgnoresARunThatIsNotItsRoot(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := seedRunRoot(t, rt, "traj-root-not-root", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))
	childRec := rec
	childRec.RunID, childRec.RequestedByRunID = "run-some-child", rec.RunID
	rt.settleRunRootTrajectory(ctx, &childRec)
	got, _ := s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectoryLive {
		t.Fatalf("a child run's completion settled the root's trajectory: %+v", got)
	}
}

// Rows minted before this change carry no required refs. The listing and
// `choir trajectory` must say what they are, not "invalid settlement rule".
func TestLegacyRunRootRowReportsARunRootVerdict(t *testing.T) {
	ctx := context.Background()
	rt, _ := testRuntime(t)
	oldRule := types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true}

	done := seedRunRoot(t, rt, "traj-legacy-done", types.RunCompleted, oldRule)
	got, err := rt.TrajectoryObligations(ctx, done.OwnerID, done.TrajectoryID)
	if err != nil {
		t.Fatalf("obligations: %v", err)
	}
	for _, w := range got.WaitingOn {
		if strings.Contains(w, "invalid settlement rule") {
			t.Fatalf("a completed run-root row still reads as an invalid rule: %v", got.WaitingOn)
		}
	}
	if !got.SettlementReady {
		t.Fatalf("a completed run-root row with nothing open should read ready: %+v", got)
	}
	raw, _ := json.Marshal(got)
	var asMap map[string]any
	_ = json.Unmarshal(raw, &asMap)
	verdict, ok := asMap["run_root"].(map[string]any)
	if !ok || verdict["run_id"] != done.RunID || verdict["run_state"] != string(types.RunCompleted) {
		t.Fatalf("no run_root verdict naming the run and its state: %s", raw)
	}

	live := seedRunRoot(t, rt, "traj-legacy-running", types.RunRunning, oldRule)
	got, err = rt.TrajectoryObligations(ctx, live.OwnerID, live.TrajectoryID)
	if err != nil {
		t.Fatalf("obligations: %v", err)
	}
	if got.SettlementReady || len(got.WaitingOn) != 1 || !strings.Contains(got.WaitingOn[0], "running") {
		t.Fatalf("a running root should wait on its run: %+v", got)
	}
}

// A row that is not a run root keeps the strict verdict: only rows that carry
// root_loop_id get the run-root reading.
func TestNonRunRootRowWithEmptyRuleStaysInvalid(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-odd", OwnerID: "user-alice", Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SubjectRefs:    map[string]string{"channel_id": "c"},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := rt.TrajectoryObligations(ctx, "user-alice", "traj-odd")
	if err != nil {
		t.Fatalf("obligations: %v", err)
	}
	if got.SettlementReady || len(got.WaitingOn) == 0 || !strings.Contains(got.WaitingOn[0], "invalid settlement rule") {
		t.Fatalf("unexpected verdict for a non-run-root row: %+v", got)
	}
}

// Review 330 (1): a passivated child is non-terminal and can be re-warmed, so
// it keeps the trajectory open.
func TestRunRootTrajectoryStaysLiveWhileAChildRunIsPassivated(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := seedRunRoot(t, rt, "traj-root-passivated-child", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))
	now := time.Now().UTC()
	child := types.RunRecord{
		RunID: "run-child-passivated", AgentID: "agent-child-p", ChannelID: "chan-1", RequestedByRunID: rec.RunID,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research,
		OwnerID: "user-alice", ComputerID: "autoputer-test", State: types.RunPassivated,
		Prompt: "child", CreatedAt: now, UpdatedAt: now, TrajectoryID: rec.TrajectoryID,
		Metadata: map[string]any{runMetadataTrajectoryID: rec.TrajectoryID},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	rt.settleRunRootTrajectory(ctx, &rec)
	got, _ := s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectoryLive {
		t.Fatalf("trajectory settled under a passivated child run: %+v", got)
	}
}

// Review 330 (2): the settle itself re-checks open work and non-terminal runs
// under the trajectory lock, so work that landed after the caller's reads keeps
// the row live.
func TestSettleTrajectoryIfIdleRechecksUnderTheLock(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := seedRunRoot(t, rt, "traj-idle-recheck", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))

	work, err := s.CreateWorkItem(ctx, types.WorkItemRecord{OwnerID: "user-alice", TrajectoryID: rec.TrajectoryID, Objective: "late work"})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if settled, err := s.SettleTrajectoryIfIdle(ctx, "user-alice", rec.TrajectoryID); err != nil || settled {
		t.Fatalf("settled over an open work item: settled=%v err=%v", settled, err)
	}
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", work.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work: %v", err)
	}

	now := time.Now().UTC()
	child := types.RunRecord{
		RunID: "run-late-child", AgentID: "agent-late", ChannelID: "chan-1", RequestedByRunID: rec.RunID,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research,
		OwnerID: "user-alice", ComputerID: "autoputer-test", State: types.RunPassivated,
		Prompt: "child", CreatedAt: now, UpdatedAt: now, TrajectoryID: rec.TrajectoryID,
		Metadata: map[string]any{runMetadataTrajectoryID: rec.TrajectoryID},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if settled, err := s.SettleTrajectoryIfIdle(ctx, "user-alice", rec.TrajectoryID); err != nil || settled {
		t.Fatalf("settled over a non-terminal run: settled=%v err=%v", settled, err)
	}
	child.State = types.RunCompleted
	fin := time.Now().UTC()
	child.FinishedAt = &fin
	if err := s.UpdateRun(ctx, child); err != nil {
		t.Fatalf("finish child: %v", err)
	}
	if settled, err := s.SettleTrajectoryIfIdle(ctx, "user-alice", rec.TrajectoryID); err != nil || !settled {
		t.Fatalf("idle trajectory did not settle: settled=%v err=%v", settled, err)
	}
	got, _ := s.GetTrajectory(ctx, "user-alice", rec.TrajectoryID)
	if got.Status != types.TrajectorySettled || got.SettledAt == nil {
		t.Fatalf("stored trajectory = %+v", got)
	}
}

// Review 330 (3): with a pending update the verdict must not say the row is
// ready to settle.
func TestRunRootVerdictDoesNotClaimReadyWithPendingUpdates(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	done := seedRunRoot(t, rt, "traj-root-pending-update", types.RunCompleted, defaultSettlementRuleForKind(types.TrajectoryKindDocument))
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, done.OwnerID)
	if err != nil {
		t.Fatalf("ensure management agent: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID: "update-root-pending", OwnerID: done.OwnerID, AgentID: "engineering:verifier",
		TargetAgentID: managementAgent.AgentID, ChannelID: managementAgent.ChannelID, TrajectoryID: done.TrajectoryID,
		Role: agentprofile.Engineering, Packet: testCoagentUpdatePacket("execution_result", "pending"),
		Content: "pending", CreatedAt: time.Now().UTC(),
	}
	message := types.ChannelMessage{
		ChannelID: update.ChannelID, FromAgentID: update.AgentID, ToAgentID: update.TargetAgentID,
		TrajectoryID: update.TrajectoryID, Role: update.Role, Content: update.Content, Timestamp: update.CreatedAt,
	}
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch update: %v", err)
	}
	got, err := rt.TrajectoryObligations(ctx, done.OwnerID, done.TrajectoryID)
	if err != nil {
		t.Fatalf("obligations: %v", err)
	}
	if got.SettlementReady || got.RunRoot == nil {
		t.Fatalf("obligations = %+v", got)
	}
	if strings.Contains(got.RunRoot.Verdict, "ready to settle") || strings.Contains(got.RunRoot.Verdict, "nothing is open") {
		t.Fatalf("verdict claims ready while an update is pending: %q", got.RunRoot.Verdict)
	}
}

// Review 330 (4): a run-root row names the run that is the trajectory's own
// root; a root_loop_id that is some other run is not a run-root row.
func TestRunRootRecordRequiresRootLoopToBeTheTrajectory(t *testing.T) {
	rec := types.TrajectoryRecord{
		TrajectoryID: "traj-a", Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SubjectRefs: map[string]string{"root_loop_id": "run-other"},
	}
	if isRunRootRecord(rec) {
		t.Fatalf("a row whose root_loop_id is another run read as a run-root row")
	}
	rec.SubjectRefs["root_loop_id"] = "traj-a"
	if !isRunRootRecord(rec) {
		t.Fatalf("a row whose root_loop_id is its own id is a run-root row")
	}
}
