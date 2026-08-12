package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func setupLifecycleTextureTargetFixture(t *testing.T) (*Store, types.StartLifecycleRequest, types.RunRecord, types.WorkItemRecord) {
	t.Helper()
	return setupLifecycleTextureTargetFixtureWithStore(t, openTestStore(t))
}

func setupLifecycleTextureTargetFixtureWithStore(t *testing.T, s *Store) (*Store, types.StartLifecycleRequest, types.RunRecord, types.WorkItemRecord) {
	t.Helper()
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-texture-target"
	start.TrajectoryID = "trajectory-texture-target"
	start.InitialWork.WorkItemID = "work-texture-caller"
	start.InitialDocument.DocID = "document-texture-target"
	start.InitialRevision.RevisionID = "revision-texture-target"
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.SubjectRefs["artifact"] = "texture://" + start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}

	caller := lifecycleRunFixture(start, "run-texture-target", types.RunRunning)
	caller.ChannelID = start.InitialDocument.DocID
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "project-texture-target-caller", TrajectoryID: start.TrajectoryID,
		AgentID: start.Agent.AgentID, Run: caller,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project caller activation: %v", err)
	}

	now := time.Now().UTC()
	researcher := types.AgentRecord{
		AgentID: "researcher:texture-target", OwnerID: start.OwnerID,
		ComputerID: start.ComputerID,
		Profile:    "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(ctx, researcher); err != nil {
		t.Fatalf("upsert Researcher: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "open-researcher-target-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-researcher-target", Objective: "continue bounded research",
			AuthorityProfile: "researcher", AssignedAgentID: researcher.AgentID,
			CreatedByRunID: caller.RunID,
			Details: map[string]any{
				"requested_by_profile":  "texture",
				"requested_by_agent_id": caller.AgentID,
				"requested_by_run_id":   caller.RunID,
			},
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	opened, err := s.OpenLifecycleWork(ctx, open)
	if err != nil || opened.WorkItem == nil {
		t.Fatalf("open Researcher target work: %+v, %v", opened.WorkItem, err)
	}
	researcherRun := lifecycleRunFixture(start, "run-researcher-target", types.RunRunning)
	researcherRun.AgentID = researcher.AgentID
	researcherRun.AgentProfile = "researcher"
	researcherRun.AgentRole = "researcher"
	researcherRun.Metadata = map[string]any{"lifecycle_work_item_id": opened.WorkItem.WorkItemID}
	researcherRun.RequestedByRunID = caller.RunID
	projectResearcher := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "project-researcher-target", TrajectoryID: start.TrajectoryID,
		AgentID: researcher.AgentID, Run: researcherRun,
	}
	projectResearcher.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(projectResearcher)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectResearcher); err != nil {
		t.Fatalf("project Researcher activation: %v", err)
	}
	return s, start, caller, *opened.WorkItem
}

func lifecycleTextureTargetRequest(start types.StartLifecycleRequest, caller types.RunRecord, targetAgentID, targetWorkItemID string) LifecycleTextureControlTargetRequest {
	return LifecycleTextureControlTargetRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		DocumentID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		CallerAgentID: caller.AgentID, CallerRunID: caller.RunID,
		TargetAgentID: targetAgentID, TargetWorkItemID: targetWorkItemID,
	}
}

func TestValidateLifecycleTextureControlTargetAcceptsBoundResearcherAndExactPersistentSuper(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	researcherBinding, err := s.ValidateLifecycleTextureControlTarget(ctx, lifecycleTextureTargetRequest(start, caller, researcherWork.AssignedAgentID, researcherWork.WorkItemID))
	if err != nil {
		t.Fatalf("validate Researcher: %v", err)
	}
	if researcherBinding.TargetProfile != "researcher" || researcherBinding.TargetWorkItem == nil || researcherBinding.TargetRun == nil ||
		researcherBinding.TargetWorkItem.WorkItemID != researcherWork.WorkItemID || researcherBinding.CallerRun.RunID != caller.RunID {
		t.Fatalf("Researcher binding = %+v", researcherBinding)
	}

	passivatedRun := *researcherBinding.TargetRun
	passivatedRun.State = types.RunPassivated
	passivatedRun.UpdatedAt = time.Now().UTC()
	passivate := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "project-researcher-passivated", TrajectoryID: start.TrajectoryID,
		AgentID: researcherWork.AssignedAgentID, Run: passivatedRun,
	}
	passivate.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(passivate)
	if _, err := s.ReplaceLifecycleActivation(ctx, passivate); err != nil {
		t.Fatalf("passivate lifecycle Researcher: %v", err)
	}
	passivatedBinding, err := s.ValidateLifecycleTextureControlTarget(ctx, lifecycleTextureTargetRequest(start, caller, researcherWork.AssignedAgentID, researcherWork.WorkItemID))
	if err != nil || passivatedBinding.TargetRun != nil || passivatedBinding.TargetAgent.LifecycleVersion <= 0 {
		t.Fatalf("validate reconstructible passivated Researcher = %+v, %v", passivatedBinding, err)
	}

	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert persistent Super: %v", err)
	}
	superBinding, err := s.ValidateLifecycleTextureControlTarget(ctx, lifecycleTextureTargetRequest(start, caller, superID, ""))
	if err != nil {
		t.Fatalf("validate persistent Super opener: %v", err)
	}
	if superBinding.TargetProfile != "super" || superBinding.TargetAgent.AgentID != superID || superBinding.TargetWorkItem != nil || superBinding.TargetAgent.LifecycleVersion != 0 {
		t.Fatalf("Super binding = %+v", superBinding)
	}
}

func TestValidateLifecycleTextureControlTargetRejectsScopeRoleAndWorkMismatches(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	base := lifecycleTextureTargetRequest(start, caller, researcherWork.AssignedAgentID, researcherWork.WorkItemID)
	now := time.Now().UTC()

	for _, agent := range []types.AgentRecord{
		{AgentID: "super:arbitrary", Profile: "super", Role: "super"},
		{AgentID: "co-super:direct", Profile: "co-super", Role: "co-super"},
		{AgentID: "researcher:unbound", Profile: "researcher", Role: "researcher"},
		{AgentID: "researcher:non-lifecycle", Profile: "researcher", Role: "researcher"},
	} {
		agent.OwnerID, agent.ComputerID, agent.ComputerID = start.OwnerID, start.ComputerID, start.ComputerID
		agent.ChannelID, agent.CreatedAt, agent.UpdatedAt = start.InitialDocument.DocID, now, now
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("upsert negative target %s: %v", agent.AgentID, err)
		}
	}
	channelShapeOpen := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "open-channel-shape-only-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-channel-shape-only", Objective: "same channel is not provenance",
			AuthorityProfile: "researcher", AssignedAgentID: "researcher:unbound",
			CreatedByRunID: "run-foreign-texture",
			Details: map[string]any{
				"requested_by_profile": "texture", "requested_by_agent_id": caller.AgentID,
				"requested_by_run_id": "run-foreign-texture",
			},
		},
	}
	channelShapeOpen.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(channelShapeOpen)
	if _, err := s.OpenLifecycleWork(ctx, channelShapeOpen); err != nil {
		t.Fatalf("open channel-shape-only work: %v", err)
	}
	nonLifecycleOpen := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "open-non-lifecycle-researcher-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-non-lifecycle-researcher", Objective: "generic actors cannot inherit lifecycle work",
			AuthorityProfile: "researcher", AssignedAgentID: "researcher:non-lifecycle",
			CreatedByRunID: caller.RunID,
			Details: map[string]any{
				"requested_by_profile": "texture", "requested_by_agent_id": caller.AgentID,
				"requested_by_run_id": caller.RunID,
			},
		},
	}
	nonLifecycleOpen.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(nonLifecycleOpen)
	if _, err := s.OpenLifecycleWork(ctx, nonLifecycleOpen); err != nil {
		t.Fatalf("open non-lifecycle Researcher work: %v", err)
	}

	legacyCaller := types.RunRecord{
		RunID: "run-non-lifecycle-texture", AgentID: "legacy:texture-caller", ChannelID: caller.ChannelID,
		AgentProfile: "texture", AgentRole: "texture",
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, State: types.RunRunning,
		CreatedAt: now, UpdatedAt: now,
	}
	// The generic run store is intentionally not lifecycle authority. The
	// validator must not fall back to it when the lifecycle lookup misses.
	if err := s.CreateRun(ctx, legacyCaller); err != nil {
		t.Fatalf("create non-lifecycle caller: %v", err)
	}

	tests := map[string]func(*LifecycleTextureControlTargetRequest){
		"cross document": func(req *LifecycleTextureControlTargetRequest) {
			req.DocumentID = "document-other"
			req.CallerAgentID = "texture:document-other"
		},
		"cross trajectory": func(req *LifecycleTextureControlTargetRequest) { req.TrajectoryID = "trajectory-other" },
		"cross computer":   func(req *LifecycleTextureControlTargetRequest) { req.ComputerID = "computer-other" },
		"non lifecycle caller": func(req *LifecycleTextureControlTargetRequest) {
			req.CallerRunID = legacyCaller.RunID
			req.CallerAgentID = legacyCaller.AgentID
		},
		"work missing":                   func(req *LifecycleTextureControlTargetRequest) { req.TargetWorkItemID = "" },
		"work assigned to another agent": func(req *LifecycleTextureControlTargetRequest) { req.TargetAgentID = "researcher:unbound" },
		"non-lifecycle Researcher despite exact work provenance": func(req *LifecycleTextureControlTargetRequest) {
			req.TargetAgentID = "researcher:non-lifecycle"
			req.TargetWorkItemID = "work-non-lifecycle-researcher"
		},
		"channel shape without caller provenance": func(req *LifecycleTextureControlTargetRequest) {
			req.TargetAgentID = "researcher:unbound"
			req.TargetWorkItemID = "work-channel-shape-only"
		},
		"arbitrary Super": func(req *LifecycleTextureControlTargetRequest) {
			req.TargetAgentID = "super:arbitrary"
			req.TargetWorkItemID = ""
		},
		"direct CoSuper": func(req *LifecycleTextureControlTargetRequest) {
			req.TargetAgentID = "co-super:direct"
			req.TargetWorkItemID = ""
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			req := base
			mutate(&req)
			if _, err := s.ValidateLifecycleTextureControlTarget(ctx, req); err == nil {
				t.Fatal("target validation unexpectedly succeeded")
			}
		})
	}
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "settle-target-before-control", TrajectoryID: start.TrajectoryID,
		WorkItemID: researcherWork.WorkItemID, ActingAgentID: researcherWork.AssignedAgentID,
		ResultRef: "artifact://closed-target-work",
	}
	settle.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settle)
	if _, err := s.SettleLifecycleWork(ctx, settle); err != nil {
		t.Fatalf("settle target work: %v", err)
	}
	if _, err := s.ValidateLifecycleTextureControlTarget(ctx, base); err == nil {
		t.Fatal("closed target work validation unexpectedly succeeded")
	}
}

type failingLifecycleTextureTargetReader struct {
	base        lifecycleTextureControlTargetReader
	operation   string
	targetID    string
	callerRunID string
	targetRunID string
	err         error
}

func (r failingLifecycleTextureTargetReader) GetLifecycleTrajectory(ctx context.Context, owner, computer, trajectory string) (types.TrajectoryRecord, error) {
	if r.operation == "trajectory" {
		return types.TrajectoryRecord{}, r.err
	}
	return r.base.GetLifecycleTrajectory(ctx, owner, computer, trajectory)
}
func (r failingLifecycleTextureTargetReader) GetLifecycleDocument(ctx context.Context, owner, computer, doc string) (types.Document, error) {
	if r.operation == "document" {
		return types.Document{}, r.err
	}
	return r.base.GetLifecycleDocument(ctx, owner, computer, doc)
}
func (r failingLifecycleTextureTargetReader) GetLifecycleRun(ctx context.Context, owner, computer, run string) (types.RunRecord, error) {
	if (r.operation == "caller run" && run == r.callerRunID) || (r.operation == "target run" && run == r.targetRunID) {
		return types.RunRecord{}, r.err
	}
	return r.base.GetLifecycleRun(ctx, owner, computer, run)
}
func (r failingLifecycleTextureTargetReader) GetLifecycleWorkItem(ctx context.Context, owner, computer, work string) (types.WorkItemRecord, error) {
	if r.operation == "work" {
		return types.WorkItemRecord{}, r.err
	}
	return r.base.GetLifecycleWorkItem(ctx, owner, computer, work)
}
func (r failingLifecycleTextureTargetReader) GetAgentByScope(ctx context.Context, owner, computer, agent string) (types.AgentRecord, error) {
	if r.operation == "caller agent" && agent != r.targetID {
		return types.AgentRecord{}, r.err
	}
	if r.operation == "target agent" && agent == r.targetID {
		return types.AgentRecord{}, r.err
	}
	return r.base.GetAgentByScope(ctx, owner, computer, agent)
}

func TestValidateLifecycleTextureControlTargetPropagatesEveryLookupError(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	req := lifecycleTextureTargetRequest(start, caller, researcherWork.AssignedAgentID, researcherWork.WorkItemID)
	injected := errors.New("injected target validation lookup failure")
	for _, operation := range []string{"trajectory", "document", "caller run", "caller agent", "target agent", "work", "target run"} {
		t.Run(operation, func(t *testing.T) {
			reader := failingLifecycleTextureTargetReader{
				base: s, operation: operation, targetID: req.TargetAgentID,
				callerRunID: req.CallerRunID, targetRunID: "run-researcher-target", err: injected,
			}
			if _, err := validateLifecycleTextureControlTarget(context.Background(), reader, req); !errors.Is(err, injected) {
				t.Fatalf("error = %v, want injected lookup failure", err)
			}
		})
	}
}

func TestLifecyclePacketDirectionRepresentationPreservesLegacyAndProducerReportSemantics(t *testing.T) {
	legacyJSON := `{"update_id":"legacy","owner_id":"owner","computer_id":"computer","agent_id":"producer","target_agent_id":"texture:doc","channel_id":"doc","message_seq":1,"work_item_id":"legacy-work","packet":{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy"},"content":"legacy","created_at":"2026-01-01T00:00:00Z"}`
	var legacy types.CoagentSourcePacket
	if err := json.Unmarshal([]byte(legacyJSON), &legacy); err != nil {
		t.Fatalf("decode legacy packet: %v", err)
	}
	if legacy.Direction != "" || legacy.ProducerWorkItemID != "" || legacy.TargetWorkItemID != "" || legacy.WorkItemID != "legacy-work" {
		t.Fatalf("legacy packet aliases changed: %+v", legacy)
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "direction") || strings.Contains(string(encoded), "producer_work_item_id") || strings.Contains(string(encoded), "target_work_item_id") {
		t.Fatalf("legacy packet emitted new empty fields: %s", encoded)
	}

	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "start-direction-producer-report"
	start.TrajectoryID = "trajectory-direction-producer-report"
	start.InitialWork.WorkItemID = "work-direction-producer"
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	queue := queueLifecycleUpdateFixture(t, s, start, "queue-direction-producer-report")
	queue.ProducerAgentID = start.Agent.AgentID
	queue.WorkItemID = start.InitialWork.WorkItemID
	queue.WorkDisposition = types.WorkItemOpen
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	queued, err := s.QueueLifecycleUpdate(ctx, queue)
	if err != nil || queued.Update == nil {
		t.Fatalf("queue producer report: %+v, %v", queued.Update, err)
	}
	if queued.Update.Direction != types.LifecyclePacketDirectionProducerReport ||
		queued.Update.ProducerWorkItemID != queue.WorkItemID || queued.Update.TargetWorkItemID != "" || queued.Update.WorkItemID != queue.WorkItemID {
		t.Fatalf("queued producer report representation = %+v", queued.Update)
	}
	producerWork, targetWork, err := ResolveLifecyclePacketWorkBindings(*queued.Update)
	if err != nil || producerWork != queue.WorkItemID || targetWork != "" {
		t.Fatalf("resolve producer report = producer %q target %q err %v", producerWork, targetWork, err)
	}

	for name, packet := range map[string]types.CoagentSourcePacket{
		"producer aliases disagree": {
			Direction:  types.LifecyclePacketDirectionProducerReport,
			WorkItemID: "legacy", ProducerWorkItemID: "different",
		},
		"producer report carries target": {
			Direction:          types.LifecyclePacketDirectionProducerReport,
			ProducerWorkItemID: "producer", TargetWorkItemID: "target",
		},
		"control carries producer alias": {
			Direction:  types.LifecyclePacketDirectionControl,
			WorkItemID: "producer", TargetWorkItemID: "target",
		},
		"control omits target":                     {Direction: types.LifecyclePacketDirectionControl},
		"unknown direction":                        {Direction: "sideways"},
		"legacy packet carries new producer field": {ProducerWorkItemID: "producer"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ResolveLifecyclePacketWorkBindings(packet); !errors.Is(err, ErrLifecycleInvalidTransition) {
				t.Fatalf("error = %v, want ErrLifecycleInvalidTransition", err)
			}
		})
	}
	producerWork, targetWork, err = ResolveLifecyclePacketWorkBindings(types.CoagentSourcePacket{
		Direction: types.LifecyclePacketDirectionControl, TargetWorkItemID: "target-work",
	})
	if err != nil || producerWork != "" || targetWork != "target-work" {
		t.Fatalf("resolve control = producer %q target %q err %v", producerWork, targetWork, err)
	}
}

func TestQueueLifecycleUpdateRequiresCanonicalTextureTargetAndProducerActivation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "start-queue-authority"
	start.TrajectoryID = "trajectory-queue-authority"
	start.InitialWork.WorkItemID = "work-queue-authority"
	start.InitialDocument.DocID = "document-queue-authority"
	start.InitialRevision.RevisionID = "revision-queue-authority"
	start.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(start.InitialDocument.DocID, start.InitialRevision.RevisionID, start.InitialRevision.Content)
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	valid := queueLifecycleUpdateFixture(t, s, start, "queue-authorized-producer")
	valid.UpdateID, valid.ProducerUpdateID = "update-authorized-producer", "producer-update-authorized-producer"
	valid.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(valid)
	if _, err := s.QueueLifecycleUpdate(ctx, valid); err != nil {
		t.Fatalf("queue exact producer report: %v", err)
	}

	second := lifecycleStartFixture()
	second.CommandID = "start-cross-document-target"
	second.TrajectoryID = "trajectory-cross-document-target"
	second.InitialWork.WorkItemID = "work-cross-document-target"
	second.InitialDocument.DocID = "document-cross-document-target"
	second.InitialRevision.RevisionID = "revision-cross-document-target"
	second.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(second.InitialDocument.DocID, second.InitialRevision.RevisionID, second.InitialRevision.Content)
	second.Agent.AgentID = "texture:" + second.InitialDocument.DocID
	second.Agent.ChannelID = second.InitialDocument.DocID
	second.SubjectRefs["doc_id"] = second.InitialDocument.DocID
	second.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(second)
	if _, err := s.StartLifecycle(ctx, second); err != nil {
		t.Fatalf("start second document: %v", err)
	}

	now := time.Now().UTC()
	researcher := types.AgentRecord{
		AgentID: "researcher:wrong-source-run", OwnerID: start.OwnerID,
		ComputerID: start.ComputerID,
		Profile:    "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(ctx, researcher); err != nil {
		t.Fatalf("upsert wrong producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "open-wrong-producer-run-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-wrong-producer-run", Objective: "provide a mismatched activation",
			AssignedAgentID: researcher.AgentID, AuthorityProfile: "researcher",
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open wrong producer work: %v", err)
	}
	wrongRun := lifecycleRunFixture(start, "run-wrong-producer", types.RunRunning)
	wrongRun.AgentID, wrongRun.AgentProfile, wrongRun.AgentRole = researcher.AgentID, "researcher", "researcher"
	wrongRun.Metadata = map[string]any{"lifecycle_work_item_id": open.WorkItem.WorkItemID}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "project-wrong-producer-run", TrajectoryID: start.TrajectoryID,
		AgentID: researcher.AgentID, Run: wrongRun,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project wrong producer run: %v", err)
	}

	tests := map[string]func(*types.QueueLifecycleUpdateRequest){
		"cross-document same-scope target": func(req *types.QueueLifecycleUpdateRequest) {
			req.TargetAgentID = second.Agent.AgentID
		},
		"missing producer run": func(req *types.QueueLifecycleUpdateRequest) {
			req.SourceRunID = ""
		},
		"unknown producer run": func(req *types.QueueLifecycleUpdateRequest) {
			req.SourceRunID = "run-does-not-exist"
		},
		"wrong producer run": func(req *types.QueueLifecycleUpdateRequest) {
			req.SourceRunID = wrongRun.RunID
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.CommandID = "refuse-" + strings.ReplaceAll(name, " ", "-")
			candidate.UpdateID = "update-" + candidate.CommandID
			candidate.ProducerUpdateID = "producer-" + candidate.CommandID
			mutate(&candidate)
			candidate.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(candidate)
			if _, err := s.QueueLifecycleUpdate(ctx, candidate); err == nil {
				t.Fatal("unsafe exported queue mutation unexpectedly succeeded")
			}
		})
	}
}
