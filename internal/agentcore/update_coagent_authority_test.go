package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type coagentAuthorityFakeStore struct {
	agents        map[string]types.AgentRecord
	lifecycleRuns map[string]types.RunRecord
	legacyRuns    map[string]types.RunRecord
	lifecycleTraj map[string]types.TrajectoryRecord
	legacyTraj    map[string]types.TrajectoryRecord
	lifecycleWork map[string]types.WorkItemRecord
	legacyWork    map[string]types.WorkItemRecord
	slots         map[string]store.CoSuperSlotRecord
	errors        map[string]error
}

func (f *coagentAuthorityFakeStore) fail(op string) error { return f.errors[op] }
func (f *coagentAuthorityFakeStore) GetAgentByScope(_ context.Context, _, _, id string) (types.AgentRecord, error) {
	if err := f.fail("agent:" + id); err != nil {
		return types.AgentRecord{}, err
	}
	v, ok := f.agents[id]
	if !ok {
		return types.AgentRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetLifecycleRun(_ context.Context, _, _, id string) (types.RunRecord, error) {
	if err := f.fail("lifecycle-run:" + id); err != nil {
		return types.RunRecord{}, err
	}
	v, ok := f.lifecycleRuns[id]
	if !ok {
		return types.RunRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetRunByOwner(_ context.Context, _, id string) (types.RunRecord, error) {
	if err := f.fail("legacy-run:" + id); err != nil {
		return types.RunRecord{}, err
	}
	v, ok := f.legacyRuns[id]
	if !ok {
		return types.RunRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetLifecycleTrajectory(_ context.Context, _, _, id string) (types.TrajectoryRecord, error) {
	if err := f.fail("lifecycle-trajectory:" + id); err != nil {
		return types.TrajectoryRecord{}, err
	}
	v, ok := f.lifecycleTraj[id]
	if !ok {
		return types.TrajectoryRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetLifecycleWorkItem(_ context.Context, _, _, id string) (types.WorkItemRecord, error) {
	if err := f.fail("lifecycle-work:" + id); err != nil {
		return types.WorkItemRecord{}, err
	}
	v, ok := f.lifecycleWork[id]
	if !ok {
		return types.WorkItemRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetTrajectory(_ context.Context, _, id string) (types.TrajectoryRecord, error) {
	if err := f.fail("legacy-trajectory:" + id); err != nil {
		return types.TrajectoryRecord{}, err
	}
	v, ok := f.legacyTraj[id]
	if !ok {
		return types.TrajectoryRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) GetWorkItem(_ context.Context, _, id string) (types.WorkItemRecord, error) {
	if err := f.fail("legacy-work:" + id); err != nil {
		return types.WorkItemRecord{}, err
	}
	v, ok := f.legacyWork[id]
	if !ok {
		return types.WorkItemRecord{}, store.ErrNotFound
	}
	return v, nil
}
func (f *coagentAuthorityFakeStore) CoSuperSlotByAgentAndTrajectory(_ context.Context, _, trajectory, agent string) (store.CoSuperSlotRecord, bool, error) {
	key := trajectory + "\x00" + agent
	if err := f.fail("slot:" + key); err != nil {
		return store.CoSuperSlotRecord{}, false, err
	}
	v, ok := f.slots[key]
	return v, ok, nil
}

func lifecycleAuthorityFixture(profile string) (*Runtime, *coagentAuthorityFakeStore, context.Context, string) {
	const owner, computer, doc, trajectory = "owner-a", "computer-a", "doc-a", "trajectory-a"
	targetID, callerID, callerRunID, parentRunID, workID := "texture:"+doc, profile+":producer-a", "run-producer-a", "run-texture-a", "work-producer-a"
	target := types.AgentRecord{AgentID: targetID, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: doc, LifecycleVersion: 2}
	callerAgent := types.AgentRecord{AgentID: callerID, OwnerID: owner, ComputerID: computer, Profile: profile, Role: profile, ChannelID: doc}
	parent := types.RunRecord{RunID: parentRunID, AgentID: targetID, OwnerID: owner, ComputerID: computer, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: doc, TrajectoryID: trajectory}
	caller := types.RunRecord{RunID: callerRunID, AgentID: callerID, OwnerID: owner, ComputerID: computer, AgentProfile: profile, AgentRole: profile, ChannelID: doc, TrajectoryID: trajectory, RequestedByRunID: parentRunID, Metadata: map[string]any{
		"requested_by_run_id": parentRunID, "requested_by_agent_id": targetID, "requested_by_profile": agentprofile.Texture,
		"trajectory_id": trajectory, "lifecycle_work_item_id": workID, "work_item_ids": []string{workID},
	}}
	f := &coagentAuthorityFakeStore{
		agents:        map[string]types.AgentRecord{targetID: target, callerID: callerAgent},
		lifecycleRuns: map[string]types.RunRecord{callerRunID: caller, parentRunID: parent}, legacyRuns: map[string]types.RunRecord{},
		lifecycleTraj: map[string]types.TrajectoryRecord{trajectory: {TrajectoryID: trajectory, OwnerID: owner, ComputerID: computer, SubjectRefs: map[string]string{"doc_id": doc}}},
		legacyTraj:    map[string]types.TrajectoryRecord{},
		lifecycleWork: map[string]types.WorkItemRecord{workID: {WorkItemID: workID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectory, AssignedAgentID: callerID, AuthorityProfile: profile, Status: types.WorkItemOpen, Details: map[string]any{
			"requested_by_run_id": parentRunID, "requested_by_agent_id": targetID, "requested_by_profile": agentprofile.Texture,
		}}}, legacyWork: map[string]types.WorkItemRecord{}, slots: map[string]store.CoSuperSlotRecord{}, errors: map[string]error{},
	}
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computer}}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&caller))
	return rt, f, ctx, targetID
}

func TestResolveCoagentUpdateAuthorityLifecycleRoleAndBindingMatrix(t *testing.T) {
	for _, profile := range []string{agentprofile.Researcher, agentprofile.Processor, agentprofile.Reconciler} {
		t.Run(profile, func(t *testing.T) {
			rt, f, ctx, target := lifecycleAuthorityFixture(profile)
			a, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, "")
			if err != nil || !a.lifecycle || a.workItemID == "" || a.target.AgentID != target {
				t.Fatalf("authority=%+v err=%v", a, err)
			}
		})
	}
	for _, profile := range []string{agentprofile.Email, agentprofile.Super, agentprofile.CoSuper, agentprofile.Conductor} {
		t.Run("refuse_"+profile, func(t *testing.T) {
			rt, f, ctx, target := lifecycleAuthorityFixture(profile)
			if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); err == nil {
				t.Fatal("forbidden lifecycle role accepted")
			}
		})
	}
}

func TestResolveCoagentUpdateAuthorityRequiresExplicitScopedTargetAndEveryLookup(t *testing.T) {
	rt, f, ctx, target := lifecycleAuthorityFixture(agentprofile.Researcher)
	if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, "", ""); err == nil || !strings.Contains(err.Error(), "explicit agent_id") {
		t.Fatalf("missing target err=%v", err)
	}
	for _, op := range []string{"agent:" + target, "agent:researcher:producer-a", "lifecycle-run:run-producer-a", "lifecycle-trajectory:trajectory-a", "lifecycle-run:run-texture-a", "lifecycle-work:work-producer-a"} {
		t.Run(op, func(t *testing.T) {
			rt, f, ctx, target := lifecycleAuthorityFixture(agentprofile.Researcher)
			f.errors[op] = errors.New("injected lookup failure")
			if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); err == nil {
				t.Fatal("lookup error accepted")
			}
		})
	}
	delete(f.agents, target)
	if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing target err=%v", err)
	}
}

func TestResolveCoagentUpdateAuthorityRefusesScopeDocumentTrajectoryRequesterAndWorkDrift(t *testing.T) {
	cases := map[string]func(*Runtime, *coagentAuthorityFakeStore, *toolregistry.ExecutionContext, string){
		"absent autoputer": func(_ *Runtime, _ *coagentAuthorityFakeStore, e *toolregistry.ExecutionContext, _ string) {
			e.ComputerID = ""
		},
		"runtime computer mismatch": func(rt *Runtime, _ *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			rt.cfg.ComputerID = "computer-b"
		},
		"target computer": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, target string) {
			v := f.agents[target]
			v.ComputerID = "computer-b"
			f.agents[target] = v
		},
		"target owner": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, target string) {
			v := f.agents[target]
			v.OwnerID = "owner-b"
			f.agents[target] = v
		},
		"arbitrary Super": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, target string) {
			v := f.agents[target]
			v.Profile, v.Role = agentprofile.Super, agentprofile.Super
			f.agents[target] = v
		},
		"direct CoSuper": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, target string) {
			v := f.agents[target]
			v.Profile, v.Role = agentprofile.CoSuper, agentprofile.CoSuper
			f.agents[target] = v
		},
		"target document": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			v := f.lifecycleTraj["trajectory-a"]
			v.SubjectRefs["doc_id"] = "doc-b"
			f.lifecycleTraj["trajectory-a"] = v
		},
		"non lifecycle texture": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, target string) {
			v := f.agents[target]
			v.LifecycleVersion = 0
			f.agents[target] = v
		},
		"requester agent": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			v := f.lifecycleRuns["run-producer-a"]
			v.Metadata["requested_by_agent_id"] = "texture:other"
			f.lifecycleRuns[v.RunID] = v
		},
		"parent trajectory": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			v := f.lifecycleRuns["run-texture-a"]
			v.TrajectoryID = "trajectory-b"
			f.lifecycleRuns[v.RunID] = v
		},
		"work trajectory": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			v := f.lifecycleWork["work-producer-a"]
			v.TrajectoryID = "trajectory-b"
			f.lifecycleWork[v.WorkItemID] = v
		},
		"work requester": func(_ *Runtime, f *coagentAuthorityFakeStore, _ *toolregistry.ExecutionContext, _ string) {
			v := f.lifecycleWork["work-producer-a"]
			v.Details["requested_by_agent_id"] = "texture:other"
			f.lifecycleWork[v.WorkItemID] = v
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rt, f, ctx, target := lifecycleAuthorityFixture(agentprofile.Researcher)
			e := toolregistry.ExecutionContextFrom(ctx)
			mutate(rt, f, &e, target)
			ctx = toolregistry.WithExecutionContext(context.Background(), e)
			if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); err == nil {
				t.Fatal("drift accepted")
			}
		})
	}
}

func legacyRequesterFixture(profile, targetProfile string) (*Runtime, *coagentAuthorityFakeStore, context.Context, string) {
	const owner, computer, trajectory = "legacy-owner", "legacy-computer", "legacy-trajectory"
	targetID, callerID, parentRunID, callerRunID := targetProfile+":legacy-parent", profile+":legacy-child", "legacy-parent-run", "legacy-child-run"
	channel := "legacy-doc"
	target := types.AgentRecord{AgentID: targetID, OwnerID: owner, ComputerID: computer, Profile: targetProfile, Role: targetProfile, ChannelID: channel}
	callerAgent := types.AgentRecord{AgentID: callerID, OwnerID: owner, ComputerID: computer, Profile: profile, Role: profile, ChannelID: channel}
	parent := types.RunRecord{RunID: parentRunID, AgentID: targetID, OwnerID: owner, ComputerID: computer, AgentProfile: targetProfile, AgentRole: targetProfile, ChannelID: channel, TrajectoryID: trajectory}
	caller := types.RunRecord{RunID: callerRunID, AgentID: callerID, OwnerID: owner, ComputerID: computer, AgentProfile: profile, AgentRole: profile, ChannelID: channel, TrajectoryID: trajectory, RequestedByRunID: parentRunID, Metadata: map[string]any{"requested_by_run_id": parentRunID, "requested_by_agent_id": targetID, "requested_by_profile": targetProfile, "trajectory_id": trajectory}}
	f := &coagentAuthorityFakeStore{agents: map[string]types.AgentRecord{targetID: target, callerID: callerAgent}, lifecycleRuns: map[string]types.RunRecord{}, legacyRuns: map[string]types.RunRecord{parentRunID: parent, callerRunID: caller}, lifecycleTraj: map[string]types.TrajectoryRecord{}, legacyTraj: map[string]types.TrajectoryRecord{trajectory: {TrajectoryID: trajectory, OwnerID: owner, ComputerID: computer, SubjectRefs: map[string]string{"channel_id": channel}}}, lifecycleWork: map[string]types.WorkItemRecord{}, legacyWork: map[string]types.WorkItemRecord{}, slots: map[string]store.CoSuperSlotRecord{}, errors: map[string]error{}}
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computer}}
	return rt, f, toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&caller)), targetID
}

func TestResolveCoagentUpdateAuthorityRefusesLegacySuperCoSuperMessaging(t *testing.T) {
	rt, f, _, _ := legacyRequesterFixture(agentprofile.CoSuper, agentprofile.Super)
	superRun := f.legacyRuns["legacy-parent-run"]
	child := f.legacyRuns["legacy-child-run"]
	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&superRun))
	if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, child.AgentID, ""); err == nil {
		t.Fatal("persistent Super retained legacy update_coagent path to CoSuper")
	}
	rt, f, ctx, target := legacyRequesterFixture(agentprofile.CoSuper, agentprofile.Super)
	if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); err == nil {
		t.Fatal("CoSuper retained legacy update_coagent path to Super")
	}
}

func TestResolveCoagentUpdateAuthorityPreCutoverCompatibilityAndRefusals(t *testing.T) {
	rt, f, ctx, target := legacyRequesterFixture(agentprofile.Researcher, agentprofile.Texture)
	a, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, "")
	if err != nil || a.lifecycle {
		t.Fatalf("legacy authority=%+v err=%v", a, err)
	}
	for name, mutate := range map[string]func(*coagentAuthorityFakeStore){
		"arbitrary super": func(f *coagentAuthorityFakeStore) {
			v := f.agents[target]
			v.AgentID = "super:arbitrary"
			v.Profile = agentprofile.Super
			v.Role = agentprofile.Super
			delete(f.agents, target)
			f.agents[v.AgentID] = v
		},
		"known lifecycle trajectory": func(f *coagentAuthorityFakeStore) {
			f.lifecycleTraj["legacy-trajectory"] = types.TrajectoryRecord{TrajectoryID: "legacy-trajectory"}
		},
		"missing parent": func(f *coagentAuthorityFakeStore) { delete(f.legacyRuns, "legacy-parent-run") },
	} {
		t.Run(name, func(t *testing.T) {
			rt, f, ctx, target := legacyRequesterFixture(agentprofile.Researcher, agentprofile.Texture)
			mutate(f)
			if _, err := resolveCoagentUpdateAuthorityWithStore(ctx, rt, f, target, ""); err == nil {
				t.Fatal("legacy authority drift accepted")
			}
		})
	}
	if err := enforceCoagentUpdateAuthority(context.Background(), nil, types.AgentRecord{}, ""); err == nil {
		t.Fatal("nil runtime accepted")
	}
	emptyRT := &Runtime{}
	if err := enforceCoagentUpdateAuthority(context.Background(), emptyRT, types.AgentRecord{}, ""); err == nil {
		t.Fatal("nil store accepted")
	}
}

func TestUpdateCoagentRefusalDoesNotWakeOrEmitBeforeCommit(t *testing.T) {
	rt, s := testRuntime(t)
	var wakes atomic.Int32
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		wakes.Add(1)
		return nil
	})
	tool := newUpdateCoagentTool(rt)
	_, err := tool.Func(context.Background(), json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"channel-shaped hint only","channel_id":"doc-hint"}`))
	if err == nil {
		t.Fatal("channel-shaped missing target accepted")
	}
	if wakes.Load() != 0 {
		t.Fatalf("refused update woke target %d times", wakes.Load())
	}
	events, listErr := s.ListEvents(context.Background(), "", 10)
	if listErr != nil {
		t.Fatalf("list refused update events: %v", listErr)
	}
	if len(events) != 0 {
		t.Fatalf("refused update emitted events: %+v", events)
	}
}

func TestUpdateCoagentLifecycleExactToolCallReplayWakesOnce(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	const ownerID, docID = "owner-call-replay", "doc-call-replay"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	parent := types.RunRecord{RunID: "run-texture-call-replay", AgentID: "texture:" + docID, ChannelID: docID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, OwnerID: ownerID, ComputerID: "autoputer-test", State: types.RunRunning, TrajectoryID: trajectoryID, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataChannelID: docID}}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create lifecycle parent: %v", err)
	}
	child, err := rt.StartCoagentRun(ctx, parent.RunID, "research replay identity", ownerID, map[string]any{runMetadataAgentProfile: agentprofile.Researcher, runMetadataAgentRole: agentprofile.Researcher, runMetadataChannelID: docID})
	if err != nil {
		t.Fatalf("spawn lifecycle researcher: %v", err)
	}
	var wakes atomic.Int32
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		wakes.Add(1)
		return nil
	})
	args := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"one logical checkpoint","agent_id":"texture:` + docID + `","work_disposition":"open","claims":[{"text":"checkpoint"}]}`)
	execution := toolExecutionContextForRun(child)
	execution.ToolCallID = "provider-call-stable-1"
	callCtx := toolregistry.WithExecutionContext(ctx, execution)
	registry := rt.ToolRegistryForProfile(agentprofile.Researcher)
	first, err := registry.Execute(callCtx, "update_coagent", args)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	second, err := registry.Execute(callCtx, "update_coagent", args)
	if err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if !strings.Contains(first, `"status":"submitted"`) || !strings.Contains(second, `"status":"existing"`) {
		t.Fatalf("unexpected replay outputs: first=%q second=%q", first, second)
	}
	if wakes.Load() != 1 {
		t.Fatalf("exact tool-call retry wakes=%d, want 1", wakes.Load())
	}
	execution.ToolCallID = "provider-call-later-2"
	if _, err := registry.Execute(toolregistry.WithExecutionContext(ctx, execution), "update_coagent", args); err != nil {
		t.Fatalf("later call: %v", err)
	}
	if wakes.Load() != 2 {
		t.Fatalf("later tool call wakes=%d, want 2", wakes.Load())
	}
}

func TestDeriveLifecycleProducerUpdateIDIsRuntimeBoundUUIDv4(t *testing.T) {
	run := types.RunRecord{RunID: "run-a", OwnerID: "owner-a", ComputerID: "computer-a"}
	execution := toolregistry.ExecutionContext{RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, ToolCallID: "call-a"}
	first, err := deriveLifecycleProducerUpdateID(execution, run)
	if err != nil {
		t.Fatal(err)
	}
	second, err := deriveLifecycleProducerUpdateID(execution, run)
	if err != nil || second != first {
		t.Fatalf("stable derive=%q/%q err=%v", first, second, err)
	}
	parsed, err := uuid.Parse(first)
	if err != nil || parsed.Version() != uuid.Version(4) {
		t.Fatalf("derived id=%q err=%v version=%v", first, err, parsed.Version())
	}
	execution.ToolCallID = "call-b"
	later, _ := deriveLifecycleProducerUpdateID(execution, run)
	if later == first {
		t.Fatal("later tool call reused producer identity")
	}
	execution.ToolCallID = ""
	if _, err := deriveLifecycleProducerUpdateID(execution, run); err == nil {
		t.Fatal("missing tool call id accepted")
	}
}
