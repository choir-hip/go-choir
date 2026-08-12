package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestUpdateCoagentAcceptsResearcherEvidenceUpdateSourcePacket(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-d9-researcher"
	docID := "doc-d9-researcher"
	researcherRun, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "d9-researcher", agentprofile.Researcher)
	raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolContextForTestCall(researcherRun, "call-d9-researcher"), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"official source is ready",
		"agent_id":"texture:doc-d9-researcher",
		"channel_id":"doc-d9-researcher",
		"claims":[{"text":"The official source confirms the update.","source_ids":["src-official"],"stance":"supports","recommended_surface":"inline_ref"}],
		"sources":[{"source_id":"src-official","kind":"content_item","target":{"uri":"https://example.test/official","title":"Official source"},"selectors":[{"kind":"whole_resource"}],"excerpt":"Official source excerpt for inline transclusion.","reader_snapshot":{"text_content":"Official source excerpt for inline transclusion.\n\nFuller cleaned reader text for the Source Viewer.","snapshot_kind":"cleaned_reader_markdown","media_type":"text/markdown","source_url":"https://example.test/official","access_scope":"private_user_source"},"evidence":{"state":"available","confidence":"high","rights_scope":"private_user_source"}}],
		"notes":["Delivered as a source packet."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, researcherRun, raw)
	if stored.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 || stored.Packet.Kind != "evidence_update" {
		t.Fatalf("packet identity = %#v", stored.Packet)
	}
	if len(stored.Packet.Claims) != 1 || len(stored.Packet.Sources) != 1 {
		t.Fatalf("packet claims/sources = %#v", stored.Packet)
	}
	if stored.Packet.Sources[0].Excerpt != "Official source excerpt for inline transclusion." {
		t.Fatalf("source excerpt not preserved: %#v", stored.Packet.Sources[0])
	}
	if stored.Packet.Sources[0].ReaderSnapshot == nil || !strings.Contains(stored.Packet.Sources[0].ReaderSnapshot.TextContent, "Fuller cleaned reader text") {
		t.Fatalf("source reader snapshot not preserved: %#v", stored.Packet.Sources[0].ReaderSnapshot)
	}
	if !strings.Contains(stored.Content, "Official source excerpt for inline transclusion.") {
		t.Fatalf("human projection omitted source excerpt: %q", stored.Content)
	}
	if strings.Contains(stored.Content, "Findings:") || strings.Contains(stored.Content, "Evidence IDs:") {
		t.Fatalf("human projection retained legacy sections: %q", stored.Content)
	}
}

func TestUpdateCoagentPersistsExplicitProducerWorkDisposition(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	const ownerID, docID = "user-producer-work-disposition", "doc-producer-work-disposition"
	run, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "producer-work-disposition", agentprofile.Researcher)
	workID := metadataStringValue(run.Metadata, "lifecycle_work_item_id")
	execute := func(callID, disposition, summary string) types.CoagentSourcePacket {
		t.Helper()
		dispositionField := ""
		if disposition != "" {
			dispositionField = `,"work_disposition":"` + disposition + `"`
		}
		raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolContextForTestCall(run, callID), "update_coagent", json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"`+summary+`","agent_id":"texture:`+docID+`"`+dispositionField+`,"claims":[{"text":"`+summary+`"}]}`))
		if err != nil {
			t.Fatalf("update_coagent %s: %v", disposition, err)
		}
		return lifecycleUpdateFromToolOutput(t, s, run, raw)
	}
	omitted := execute("call-producer-omitted", "", "omitted remains open")
	if omitted.WorkDisposition != types.WorkItemOpen || omitted.WorkItemID != workID {
		t.Fatalf("omitted disposition = %+v", omitted)
	}
	open := execute("call-producer-open", "open", "interim remains open")
	completed := execute("call-producer-completed", "completed", "work complete")
	if open.WorkDisposition != types.WorkItemOpen || completed.WorkDisposition != types.WorkItemCompleted {
		t.Fatalf("work consequences open=%+v completed=%+v", open, completed)
	}
	if open.ProducerUpdateID == completed.ProducerUpdateID || open.UpdateID == completed.UpdateID {
		t.Fatal("distinct runtime call identities deduped")
	}
	for _, update := range []types.CoagentSourcePacket{omitted, open, completed} {
		parsed, err := uuid.Parse(update.ProducerUpdateID)
		if err != nil || parsed.Version() != uuid.Version(4) {
			t.Fatalf("runtime producer id %q: %v", update.ProducerUpdateID, err)
		}
	}
}

func TestUpdateCoagentRefusesPresentInvalidWorkDisposition(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	run := d9CoagentRun("run-invalid-producer-disposition", "owner-invalid-producer-disposition", "researcher:invalid", agentprofile.Researcher, "doc-invalid", "")
	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run))
	for name, value := range map[string]string{"null": "null", "blank": `" "`, "unknown": `"done"`} {
		t.Run(name, func(t *testing.T) {
			raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"invalid","agent_id":"texture:doc-invalid","work_disposition":` + value + `}`)
			if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(ctx, "update_coagent", raw); err == nil {
				t.Fatalf("update_coagent accepted invalid work disposition: %s", raw)
			}
		})
	}
}

func TestSpawnedLifecycleResearcherQueuesOpenAndCompletedUpdates(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	const (
		ownerID = "user-spawned-lifecycle-researcher"
		docID   = "doc-spawned-lifecycle-researcher"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID: "run-spawned-lifecycle-parent", AgentID: "texture:" + docID, ChannelID: docID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		OwnerID: ownerID, ComputerID: "autoputer-test", State: types.RunRunning,
		TrajectoryID: trajectoryID, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataChannelID: docID},
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create lifecycle parent activation: %v", err)
	}
	child, err := rt.StartCoagentRun(ctx, parent.RunID, "research the durable subject", ownerID, map[string]any{
		runMetadataAgentProfile: agentprofile.Researcher,
		runMetadataAgentRole:    agentprofile.Researcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("spawn lifecycle researcher: %v", err)
	}
	workItemID := metadataStringValue(child.Metadata, "lifecycle_work_item_id")
	if workItemID == "" || !containsString(metadataStringSlice(child.Metadata["work_item_ids"]), workItemID) {
		t.Fatalf("spawned lifecycle work binding missing: %+v", child.Metadata)
	}
	execute := func(producerUpdateID, disposition string) {
		t.Helper()
		raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"` + disposition + ` lifecycle checkpoint","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","work_disposition":"` + disposition + `","claims":[{"text":"` + disposition + ` lifecycle checkpoint"}]}`)
		execution := toolExecutionContextForRun(child)
		execution.ToolCallID = producerUpdateID
		if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
			toolregistry.WithExecutionContext(ctx, execution),
			"update_coagent", raw,
		); err != nil {
			t.Fatalf("queue %s spawned lifecycle update: %v", disposition, err)
		}
	}
	execute("33333333-3333-4333-8333-333333333333", "open")
	execute("44444444-4444-4444-8444-444444444444", "completed")
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "autoputer-test", trajectoryID)
	if err != nil {
		t.Fatalf("snapshot spawned lifecycle updates: %v", err)
	}
	if len(snapshot.Updates) != 2 {
		t.Fatalf("spawned lifecycle updates = %+v, want two", snapshot.Updates)
	}
	for _, update := range snapshot.Updates {
		if update.AgentID != child.AgentID || update.WorkItemID != workItemID ||
			(update.WorkDisposition != types.WorkItemOpen && update.WorkDisposition != types.WorkItemCompleted) {
			t.Fatalf("spawned lifecycle update lost producer work binding: %+v", update)
		}
	}
	var assignedWork types.WorkItemRecord
	for _, work := range snapshot.WorkItems {
		if work.WorkItemID == workItemID {
			assignedWork = work
		}
	}
	if assignedWork.Status != types.WorkItemOpen || assignedWork.AssignedAgentID != child.AgentID {
		t.Fatalf("spawned lifecycle work changed before Texture disposition: %+v", assignedWork)
	}
	terminal := *child
	terminal.State = types.RunCompleted
	terminal.Result = "research complete"
	finishedAt := time.Now().UTC()
	terminal.UpdatedAt, terminal.FinishedAt = finishedAt, &finishedAt
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "project-terminal-spawned-researcher",
		TrajectoryID: trajectoryID, AgentID: child.AgentID, Run: terminal,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ProjectTerminalLifecycleRun(ctx, project); err != nil {
		t.Fatalf("project terminal lifecycle researcher: %v", err)
	}
	persisted, err := s.GetLifecycleRun(ctx, ownerID, "autoputer-test", child.RunID)
	if err != nil {
		t.Fatalf("reload terminal lifecycle researcher: %v", err)
	}
	binding, err := rt.ensurePersistedTerminalRunOutcome(ctx, &persisted)
	if err != nil || binding.Present || binding.Wake {
		t.Fatalf("terminal lifecycle projection synthesized update authority: %+v, %v", binding, err)
	}
	legacyUpdates, err := s.ListWorkerUpdatesBySourceRun(ctx, ownerID, child.RunID)
	if err != nil || len(legacyUpdates) != 0 {
		t.Fatalf("terminal lifecycle projection emitted legacy updates: %+v, %v", legacyUpdates, err)
	}
	rt.sweepOpenWorkItemActors(ctx)
	if active, err := s.GetLatestActiveRunByAgent(ctx, ownerID, child.AgentID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("boot sweep created activation despite pending terminal disposition: %+v, %v", active, err)
	}
}

type researcherAdmissionCountingProvider struct {
	mu    sync.Mutex
	calls int
	stub  *provider.StubProvider
}

func newResearcherAdmissionCountingProvider() *researcherAdmissionCountingProvider {
	return &researcherAdmissionCountingProvider{stub: provider.NewStubProvider(0)}
}

func (p *researcherAdmissionCountingProvider) Execute(ctx context.Context, rec *types.RunRecord, emit provideriface.EventEmitFunc) error {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return p.stub.Execute(ctx, rec, emit)
}

func (p *researcherAdmissionCountingProvider) ProviderName() string {
	return "researcher-admission-counting"
}

func (p *researcherAdmissionCountingProvider) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func setSynchronousResearcherAdmissionDispatch(rt *Runtime, s *store.Store) {
	rt.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		if kind != "initial_dispatch" {
			return nil
		}
		rec, err := s.GetLifecycleRun(ctx, ownerID, computerID, strings.TrimSpace(content))
		if err != nil {
			rec, err = s.GetRunByOwner(ctx, ownerID, strings.TrimSpace(content))
		}
		if err == nil {
			rt.ExecuteActivationSync(ctx, &rec)
		}
		return nil
	})
}

func TestLifecycleResearcherOpenWorkNeedsExactControlAndDoesNotMintSuccessors(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	counting := newResearcherAdmissionCountingProvider()
	rt.provider = counting
	setSynchronousResearcherAdmissionDispatch(rt, s)
	ctx := context.Background()
	const ownerID, docID = "user-researcher-admission", "doc-researcher-admission"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	initialSnapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "autoputer-test", trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureWorkID := ""
	for _, item := range initialSnapshot.WorkItems {
		if item.AssignedAgentID == currentTextureAgentID(docID) {
			textureWorkID = item.WorkItemID
			break
		}
	}
	if textureWorkID == "" {
		t.Fatal("initial Texture work is missing")
	}
	now := time.Now().UTC()
	textureAgentID := currentTextureAgentID(docID)
	parent := types.RunRecord{
		RunID: "run-researcher-admission-texture", AgentID: textureAgentID, ChannelID: docID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		OwnerID: ownerID, ComputerID: "autoputer-test", State: types.RunRunning,
		TrajectoryID: trajectoryID, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataChannelID: docID, "lifecycle_work_item_id": textureWorkID, "work_item_ids": []string{textureWorkID}},
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatal(err)
	}
	child, err := rt.StartCoagentRun(ctx, parent.RunID, "inspect the exact source", ownerID, map[string]any{
		runMetadataAgentProfile: agentprofile.Researcher,
		runMetadataAgentRole:    agentprofile.Researcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatal(err)
	}
	workID := metadataStringValue(child.Metadata, "lifecycle_work_item_id")
	if counting.Count() != 0 {
		t.Fatalf("open work alone made %d provider calls", counting.Count())
	}
	idle, err := s.GetLifecycleRun(ctx, ownerID, "autoputer-test", child.RunID)
	if err != nil || idle.State != types.RunPassivated || metadataStringValue(idle.Metadata, "passivated_reason") != "lifecycle_researcher_provider_admission_refused" {
		t.Fatalf("open-work Researcher is not durably idle: %+v err=%v", idle, err)
	}
	work, err := s.GetLifecycleWorkItem(ctx, ownerID, "autoputer-test", workID)
	if err != nil || work.Status != types.WorkItemOpen {
		t.Fatalf("open work was changed by admission refusal: %+v err=%v", work, err)
	}
	if generic, err := rt.reconcileAssignedWorkItemActor(ctx, []types.WorkItemRecord{work}); err != nil || generic != nil {
		t.Fatalf("generic reconciliation minted lifecycle Researcher run: %+v err=%v", generic, err)
	}
	for range 2 {
		rt.sweepOpenWorkItemActors(ctx)
		rt.sweepPassivatedSpawnedCoagentWork(ctx)
	}
	if counting.Count() != 0 {
		t.Fatalf("repeated no-control reconciliation made %d provider calls", counting.Count())
	}

	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "autoputer-test", trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureAgent, err := s.GetAgentByScope(ctx, ownerID, "autoputer-test", textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "inspect one exact follow-up"}
	content := "inspect one exact follow-up"
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	turn := types.ApplyTextureTurnRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "turn-researcher-admission-control", DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: textureAgentID, CallerRunID: parent.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion, ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID,
		CallerWorkItemID: textureWorkID, CallerWorkDisposition: types.WorkItemOpen,
		Outcome: types.TextureTurnWait, Reason: "issue one bounded follow-up",
		Controls: []types.TextureTurnControl{{ControlID: "control-researcher-admission", TargetAgentID: child.AgentID, TargetWorkItemID: workID, Packet: packet, Content: content, PayloadDigest: payloadDigest}},
	}
	turn.CommandDigest, _ = store.ComputeApplyTextureTurnDigest(turn)
	if _, err := s.ApplyTextureTurn(ctx, turn); err != nil {
		t.Fatal(err)
	}
	activated, err := rt.reconcileUpdatedCoagentActor(ctx, ownerID, child.AgentID)
	if err != nil || activated == nil {
		t.Fatalf("later exact control did not activate: %+v err=%v", activated, err)
	}
	if counting.Count() != 1 {
		t.Fatalf("one exact control made %d provider calls, want 1", counting.Count())
	}
	completed, err := s.GetLifecycleRun(ctx, ownerID, "autoputer-test", activated.RunID)
	if err != nil || completed.State != types.RunCompleted {
		t.Fatalf("controlled Researcher did not finish one bounded activation: %+v err=%v", completed, err)
	}
	reportPacket := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "bounded report remains open", Claims: []types.CoagentPacketClaim{{Text: "bounded report remains open"}}}
	reportContent := "bounded report remains open"
	reportDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(reportPacket, reportContent)
	report := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "queue-researcher-admission-open-report",
		TrajectoryID: trajectoryID, UpdateID: "update-researcher-admission-open-report", TargetAgentID: textureAgentID,
		ProducerAgentID: child.AgentID, ProducerUpdateID: "producer-researcher-admission-open-report", SourceRunID: completed.RunID,
		ChannelID: docID, Role: agentprofile.Researcher, WorkItemID: workID, WorkDisposition: types.WorkItemOpen,
		PayloadDigest: reportDigest, Packet: reportPacket, Content: reportContent,
	}
	report.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(report)
	if _, err := s.QueueLifecycleUpdate(ctx, report); err != nil {
		t.Fatal(err)
	}
	parent.State = types.RunPassivated
	parent.UpdatedAt = time.Now().UTC()
	parent.FinishedAt = nil
	if err := s.UpdateRun(ctx, parent); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if next, err := rt.reconcileUpdatedCoagentActor(ctx, ownerID, child.AgentID); err != nil || next != nil {
			t.Fatalf("open report minted generic successor: %+v err=%v", next, err)
		}
		rt.sweepOpenWorkItemActors(ctx)
		rt.sweepPassivatedSpawnedCoagentWork(ctx)
	}
	rt.Start(ctx)
	if counting.Count() != 1 {
		t.Fatalf("reconcile/restart amplified one control to %d provider calls", counting.Count())
	}
	runs, err := s.ListLifecycleRunsByTrajectory(ctx, ownerID, "autoputer-test", trajectoryID, 0)
	if err != nil {
		t.Fatal(err)
	}
	researcherRuns := 0
	for _, run := range runs {
		if run.AgentID == child.AgentID {
			researcherRuns++
		}
	}
	if researcherRuns != 2 {
		t.Fatalf("Researcher run count=%d want idle initial plus one controlled activation; runs=%+v", researcherRuns, runs)
	}
}

func TestLifecycleResearcherAdmissionErrorRecoversWithDistinctOccurrenceOnce(t *testing.T) {
	rt, s := testRuntime(t)
	counting := newResearcherAdmissionCountingProvider()
	rt.provider = counting
	fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-retry", "admission-retry")
	rt.passivateLifecycleResearcherAfterAdmissionError(context.Background(), &fixture.run, errors.New("injected admission store failure after bind"))
	parked, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, fixture.run.RunID)
	if err != nil || parked.State != types.RunPassivated || metadataStringValue(parked.Metadata, "passivated_reason") != lifecycleResearcherAdmissionRetryReason {
		t.Fatalf("admission failure was not durably retryable: %+v err=%v", parked, err)
	}
	var recoveryContents []string
	rt.SetDispatchActor(func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		if kind != "coagent_result" {
			t.Fatalf("admission retry reused %q instead of a distinct recovery occurrence", kind)
		}
		recoveryContents = append(recoveryContents, content)
		return nil // boot delivery is paused: append first, execute after projection
	})
	rt.Start(context.Background())
	if len(recoveryContents) == 1 {
		rec, terminal, resolveErr := rt.ResolveLifecycleResearcherAdmissionRecovery(context.Background(), parked.OwnerID, parked.ComputerID, parked.AgentID, recoveryContents[0], parked.TrajectoryID, fixture.control.AgentID)
		if resolveErr != nil || terminal || rec == nil {
			t.Fatalf("resolve appended recovery rec=%+v terminal=%v err=%v", rec, terminal, resolveErr)
		}
		if err := rt.ExecuteActivationSyncChecked(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	if len(recoveryContents) != 1 || !strings.HasPrefix(recoveryContents[0], "lifecycle-researcher-admission-recovery:v1:") {
		t.Fatalf("admission recovery occurrences=%v", recoveryContents)
	}
	if counting.Count() != 1 {
		t.Fatalf("admission restart provider calls=%d want 1", counting.Count())
	}
	completed, err := s.GetLifecycleRun(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, fixture.run.RunID)
	if err != nil || completed.State != types.RunCompleted {
		t.Fatalf("admission recovery run=%+v err=%v", completed, err)
	}
	rt.Start(context.Background())
	if len(recoveryContents) != 1 || counting.Count() != 1 {
		t.Fatalf("repeated restart duplicated recovery: occurrences=%v calls=%d", recoveryContents, counting.Count())
	}
}

func TestLifecycleResearcherProviderAdmissionFailsClosedForStaleAndCancelledRuns(t *testing.T) {
	t.Run("stale pending projection", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-stale-pending", "admission-stale-pending")
		stale := fixture.run
		stale.State = types.RunPending
		rt.ExecuteActivationSync(context.Background(), &stale)
		if counting.Count() != 0 || stale.State != types.RunPassivated {
			t.Fatalf("stale pending projection calls=%d state=%s", counting.Count(), stale.State)
		}
	})
	t.Run("stale running projection", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-stale-running", "admission-stale-running")
		canonical := fixture.run
		canonical.State = types.RunPending
		canonical.UpdatedAt = time.Now().UTC()
		if err := s.UpdateRun(context.Background(), canonical); err != nil {
			t.Fatal(err)
		}
		// The actor still holds the older running projection. Exact canonical
		// state must win before provider entry.
		rt.ExecuteActivationSync(context.Background(), &fixture.run)
		if counting.Count() != 0 || fixture.run.State != types.RunPassivated {
			t.Fatalf("stale running projection calls=%d state=%s", counting.Count(), fixture.run.State)
		}
	})
	t.Run("legacy version-zero Researcher remains executable", func(t *testing.T) {
		rt, _ := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		rec, err := rt.createRunWithMetadata(context.Background(), "legacy Researcher work", "owner-admission-legacy", map[string]any{
			runMetadataAgentProfile: agentprofile.Researcher,
			runMetadataAgentRole:    agentprofile.Researcher,
			runMetadataAgentID:      "researcher:admission-legacy",
		})
		if err != nil {
			t.Fatal(err)
		}
		rt.ExecuteActivationSync(context.Background(), rec)
		if counting.Count() != 1 || rec.State != types.RunCompleted {
			t.Fatalf("legacy Researcher calls=%d state=%s", counting.Count(), rec.State)
		}
	})
	t.Run("declared lifecycle missing canonical trajectory", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		rec, err := rt.createRunWithMetadata(context.Background(), "malformed declared lifecycle control", "owner-admission-missing-trajectory", map[string]any{
			runMetadataAgentProfile:               agentprofile.Researcher,
			runMetadataAgentRole:                  agentprofile.Researcher,
			runMetadataAgentID:                    "researcher:admission-missing-trajectory",
			runMetadataTrajectoryID:               "missing-canonical-lifecycle-trajectory",
			"request_source":                      "lifecycle_texture_control",
			lifecycleLogicalActivationKeyMetadata: "sha256:declared-logical",
			lifecycleFailedAttemptKeyMetadata:     "sha256:declared-attempt",
			lifecycleActivationBuildMetadata:      "declared-build",
		})
		if err != nil {
			t.Fatal(err)
		}
		rt.ExecuteActivationSync(context.Background(), rec)
		stored, err := s.GetRunByOwner(context.Background(), rec.OwnerID, rec.RunID)
		if err != nil || counting.Count() != 0 || stored.State != types.RunPassivated {
			t.Fatalf("missing lifecycle authority calls=%d run=%+v err=%v", counting.Count(), stored, err)
		}
	})
	t.Run("cancelled trajectory", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-cancelled", "admission-cancelled")
		snapshot, err := s.GetLifecycleSnapshot(context.Background(), fixture.run.OwnerID, fixture.run.ComputerID, fixture.trajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := rt.CancelTrajectoryCommand(context.Background(), fixture.trajectoryID, fixture.run.OwnerID, "cancel-admission-before-provider", "owner cancelled before provider", snapshot.Trajectory.LifecycleVersion, snapshot.HeadRevision.RevisionID); err != nil {
			t.Fatal(err)
		}
		rt.ExecuteActivationSync(context.Background(), &fixture.run)
		rt.sweepOpenWorkItemActors(context.Background())
		rt.sweepPassivatedSpawnedCoagentWork(context.Background())
		if counting.Count() != 0 {
			t.Fatalf("cancelled trajectory made %d provider calls", counting.Count())
		}
	})

	t.Run("admission and passivation store failure is returned to actor", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-store-cut", "admission-store-cut")
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := rt.ExecuteActivationSyncChecked(context.Background(), &fixture.run); err == nil {
			t.Fatal("admission plus retry-passivation failure was acknowledged")
		}
		if counting.Count() != 0 {
			t.Fatalf("provider calls=%d", counting.Count())
		}
	})

	t.Run("legacy version-zero Researcher attached to lifecycle trajectory remains executable", func(t *testing.T) {
		rt, s := testRuntime(t)
		counting := newResearcherAdmissionCountingProvider()
		rt.provider = counting
		const ownerID, docID, agentID = "owner-admission-legacy-trajectory", "doc-admission-legacy-trajectory", "researcher:legacy-trajectory"
		trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
		now := time.Now().UTC()
		if err := s.UpsertAgent(context.Background(), types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: "autoputer-test", Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: docID, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		rec := types.RunRecord{RunID: "run-admission-legacy-trajectory", AgentID: agentID, AgentProfile: agentprofile.Researcher, AgentRole: agentprofile.Researcher, OwnerID: ownerID, ComputerID: "autoputer-test", ChannelID: docID, TrajectoryID: trajectoryID, State: types.RunPending, CreatedAt: now, UpdatedAt: now}
		if err := s.CreateRun(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
		if err := rt.ExecuteActivationSyncChecked(context.Background(), &rec); err != nil {
			t.Fatal(err)
		}
		if counting.Count() != 1 || rec.State != types.RunCompleted {
			t.Fatalf("legacy lifecycle-trajectory calls=%d state=%s", counting.Count(), rec.State)
		}
	})
}

func TestGenericAssignedWorkRecoveryDefersTextureToDocumentOwner(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-texture-recovery-authority"
	docID := "doc-texture-recovery-authority"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "autoputer-test", trajectoryID)
	if err != nil {
		t.Fatalf("load durable Texture lifecycle: %v", err)
	}
	if len(snapshot.WorkItems) != 1 {
		t.Fatalf("Texture lifecycle work items = %d, want 1", len(snapshot.WorkItems))
	}

	replacement, err := rt.reconcileAssignedWorkItemActorWithSource(ctx, snapshot.WorkItems, "trajectory_work_item_sweep")
	if err != nil {
		t.Fatalf("reconcile generic assigned Texture work: %v", err)
	}
	if replacement != nil {
		t.Fatalf("generic recovery claimed Texture revision authority: %+v", replacement)
	}
	if _, found, err := rt.activeRunByAgent(ctx, ownerID, currentTextureAgentID(docID)); err != nil || found {
		t.Fatalf("generic recovery created Texture activation: found=%t err=%v", found, err)
	}
}

func TestUpdateCoagentRejectsLegacyFieldsAndExecutionRequestWithoutActions(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-reject"
	docID := "doc-d9-reject"
	superRun := d9CoagentRun("run-d9-reject", ownerID, "super:d9", agentprofile.Super, docID, currentTextureAgentID(docID))
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy","findings":["old shape"]}`),
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy","evidence_ids":["ev-old"]}`),
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"missing actions","notes":["not executable"]}`),
	} {
		if _, err := rt.ToolRegistryForProfile(agentprofile.Super).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun)), "update_coagent", raw); err == nil {
			t.Fatalf("update_coagent unexpectedly accepted %s", string(raw))
		}
	}
}

func TestUpdateCoagentRejectsMalformedExecutionRequestPackets(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	superRun := d9CoagentRun("run-d9-malformed", "user-d9-malformed", "super:d9-malformed", agentprofile.Super, "doc-d9-malformed", currentTextureAgentID("doc-d9-malformed"))
	validSafety := `"safety":{"mutation_class":"red","network":"allowed","file_mutation":"allowed"}`
	for name, raw := range map[string]json.RawMessage{
		"missing action type": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"missing action type",
			"actions":[{"objective":"Run the requested command.",` + validSafety + `}]
		}`),
		"unsupported action type": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"unsupported action type",
			"actions":[{"type":"shell_out","objective":"Run the requested command.",` + validSafety + `}]
		}`),
		"empty safety": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"empty safety",
			"actions":[{"type":"run_command","objective":"Run the requested command."}]
		}`),
		"unsupported safety enum": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"bad safety enum",
			"actions":[{"type":"run_command","objective":"Run the requested command.","safety":{"mutation_class":"purple","network":"allowed","file_mutation":"allowed"}}]
		}`),
		"malformed source target": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"bad source target",
			"sources":[{"source_id":"src-bad","kind":"test_run","target":{"title":"missing uri"}}],
			"actions":[{"type":"run_command","objective":"Run the requested command.",` + validSafety + `}]
		}`),
		"claim cites missing source": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"missing claim source",
			"claims":[{"text":"The claim cites an absent source.","source_ids":["src-missing"]}],
			"actions":[{"type":"run_command","objective":"Run the requested command.",` + validSafety + `}]
		}`),
	} {
		if _, err := rt.ToolRegistryForProfile(agentprofile.Super).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun)), "update_coagent", raw); err == nil {
			t.Fatalf("%s: update_coagent unexpectedly accepted malformed execution_request", name)
		}
	}
}

func TestUpdateCoagentRejectsUnsupportedSourceAndSelectorKinds(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	run := d9CoagentRun("run-d9-source-vocab", "user-d9-source-vocab", "researcher:d9-source-vocab", agentprofile.Researcher, "doc-d9-source-vocab", "")
	for name, raw := range map[string]json.RawMessage{
		"unsupported source kind": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"evidence_update",
			"summary":"unsupported source kind",
			"claims":[{"text":"The claim cites a source with an unsupported kind.","source_ids":["src-bad-kind"]}],
			"sources":[{"source_id":"src-bad-kind","kind":"magic_oracle","target":{"uri":"https://example.test/source","title":"Bad source"},"selectors":[{"kind":"whole_resource"}]}]
		}`),
		"unsupported selector kind": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"evidence_update",
			"summary":"unsupported selector kind",
			"claims":[{"text":"The claim cites a source with an unsupported selector.","source_ids":["src-bad-selector"]}],
			"sources":[{"source_id":"src-bad-selector","kind":"content_item","target":{"uri":"https://example.test/source","title":"Bad selector"},"selectors":[{"kind":"css_selector","quote":"main article"}]}]
		}`),
		"unsupported expected source kind": json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"execution_request",
			"summary":"unsupported expected source kind",
			"actions":[{"type":"run_command","objective":"Return impossible evidence.","expected_sources":[{"kind":"magic_oracle","required":true}],"safety":{"mutation_class":"red","network":"allowed","file_mutation":"allowed"}}]
		}`),
	} {
		if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "update_coagent", raw); err == nil {
			t.Fatalf("%s: update_coagent unexpectedly accepted unsupported source vocabulary", name)
		}
	}
}

func TestUpdateCoagentCanonicalizesSourceContractAliases(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-d9-source-alias"
	docID := "doc-d9-source-alias"
	run, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "d9-source-alias", agentprofile.Researcher)
	raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolContextForTestCall(run, "call-d9-source-alias"), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"source aliases normalize",
		"agent_id":"texture:doc-d9-source-alias",
		"channel_id":"doc-d9-source-alias",
		"claims":[{"text":"The source and selector aliases should be canonicalized.","source_ids":["src-alias"]}],
		"sources":[{"source_id":"src-alias","kind":"web_page","target":{"uri":"https://example.test/source","title":"Alias source"},"selectors":[{"kind":"text quote","quote":"Alias source"}]}]
	}`))
	if err != nil {
		t.Fatalf("update_coagent alias packet: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, run, raw)
	if got := stored.Packet.Sources[0].Kind; got != "web_source" {
		t.Fatalf("source kind = %q, want web_source", got)
	}
	if got := stored.Packet.Sources[0].Selectors[0].Kind; got != "text_quote" {
		t.Fatalf("selector kind = %q, want text_quote", got)
	}
}

func TestUpdateCoagentToolSchemaRequiresSourceTargetURIAndVocabularyEnums(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	tool, ok := rt.ToolRegistryForProfile(agentprofile.Researcher).Lookup("update_coagent")
	if !ok {
		t.Fatal("update_coagent tool missing")
	}
	if !schemaRequiredContains(tool.Parameters, "agent_id") {
		t.Fatalf("update_coagent schema required = %#v, want explicit agent_id", tool.Parameters["required"])
	}
	props := schemaObject(t, tool.Parameters, "properties")
	if _, exposed := props["producer_update_id"]; exposed {
		t.Fatal("update_coagent schema exposes model-authored producer_update_id")
	}
	sources := schemaObject(t, props, "sources")
	sourceItems := schemaObject(t, sources, "items")
	sourceProps := schemaObject(t, sourceItems, "properties")
	sourceKind := schemaObject(t, sourceProps, "kind")
	if !schemaEnumContains(sourceKind, "content_item") || !schemaEnumContains(sourceKind, "test_run") || schemaEnumContains(sourceKind, "web_page") {
		t.Fatalf("source kind enum = %#v, want canonical source contract kinds", sourceKind["enum"])
	}
	target := schemaObject(t, sourceProps, "target")
	if !schemaRequiredContains(target, "uri") {
		t.Fatalf("target schema required = %#v, want uri", target["required"])
	}
	selectors := schemaObject(t, sourceProps, "selectors")
	selectorItems := schemaObject(t, selectors, "items")
	selectorProps := schemaObject(t, selectorItems, "properties")
	selectorKind := schemaObject(t, selectorProps, "kind")
	if !schemaEnumContains(selectorKind, "whole_resource") || !schemaEnumContains(selectorKind, "text_quote") || schemaEnumContains(selectorKind, "css_selector") {
		t.Fatalf("selector kind enum = %#v, want canonical source contract selector kinds", selectorKind["enum"])
	}
}

func TestUpdateCoagentAcceptsSuperExecutionResultSourcesAndTextureCollatesPacketSourcesOnly(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-d9-super-result"
	docID := "doc-d9-super-result"
	producerRun, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "d9-result", agentprofile.Researcher)
	raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolContextForTestCall(producerRun, "call-d9-result"), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"execution_result",
		"summary":"command, diff, and tests completed",
		"agent_id":"texture:doc-d9-super-result",
		"channel_id":"doc-d9-super-result",
		"claims":[{"text":"The requested verification completed.","source_ids":["src-command","src-diff","src-test"]}],
		"sources":[
			{"source_id":"src-command","kind":"command_output","target":{"uri":"command_output:cmd-d9","title":"nix develop -c go test ./internal/runtime -run TestD9"}},
			{"source_id":"src-diff","kind":"diff_hunk","target":{"uri":"diff_hunk:d9-update-coagent","title":"update_coagent packet diff"}},
			{"source_id":"src-test","kind":"test_run","target":{"uri":"test_run:runtime-d9-focused","title":"focused runtime tests passed"}}
		],
		"notes":["Do not scrape command_output:prose-only or diff_hunk:prose-only from this note."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent execution_result: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, producerRun, raw)
	if len(stored.Packet.Sources) != 3 {
		t.Fatalf("stored execution sources = %#v, want three typed sources", stored.Packet.Sources)
	}
	for _, source := range stored.Packet.Sources {
		if strings.Contains(source.Target.URI, "prose-only") {
			t.Fatalf("packet source was scraped from prose: %#v", stored.Packet.Sources)
		}
	}
}

func TestLifecycleRunInjectorReadsComputerScopedPendingUpdates(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-lifecycle-injector"
		docID   = "doc-lifecycle-injector"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	targetAgentID := currentTextureAgentID(docID)
	producerAgentID, producerWorkID, producerRunID := projectTestLifecycleProducer(t, s, ownerID, "autoputer-test", trajectoryID, docID, "lifecycle-injector")
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "evidence_update",
		Summary:       "computer-scoped lifecycle update",
	}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "queue-lifecycle-injector",
		TrajectoryID: trajectoryID, TargetAgentID: targetAgentID, ProducerAgentID: producerAgentID,
		ProducerUpdateID: "producer-lifecycle-injector", UpdateID: "update-lifecycle-injector",
		ChannelID: docID, Role: agentprofile.Researcher, SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "scoped lifecycle content", Disposition: types.UpdatePending,
	}
	req.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	req.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(req)
	if _, err := s.QueueLifecycleUpdate(ctx, req); err != nil {
		t.Fatalf("queue lifecycle update: %v", err)
	}
	now := time.Now().UTC()
	rec := &types.RunRecord{
		RunID: "run-lifecycle-injector", OwnerID: ownerID, AgentID: targetAgentID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		ComputerID: "autoputer-test", ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunPending, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentID:      targetAgentID,
		},
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "activation:" + rec.RunID,
		TrajectoryID: trajectoryID, AgentID: targetAgentID, Run: *rec,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project lifecycle activation: %v", err)
	}
	if legacy, err := s.ListCoagentMailboxBacklog(ctx, ownerID, targetAgentID, 10); err != nil || len(legacy) != 0 {
		t.Fatalf("legacy mailbox exposed lifecycle update: %+v, %v", legacy, err)
	}
	inject := rt.coagentUpdateTurnInjector(rec)
	if inject == nil {
		t.Fatal("lifecycle coagent update injector is nil")
	}
	messages, err := inject(false)
	if err != nil {
		t.Fatalf("inject lifecycle update: %v", err)
	}
	if len(messages) != 1 || !strings.Contains(string(messages[0]), "scoped lifecycle content") {
		t.Fatalf("lifecycle update messages = %s", messages)
	}
}

func TestPendingCoagentUpdatesRejectsLifecycleMarkerAsAuthority(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID       = "user-legacy-marker-injector"
		targetAgentID = "texture:legacy-marker-injector"
	)
	update := types.CoagentSourcePacket{
		UpdateID: "update-legacy-marker-injector", OwnerID: ownerID,
		AgentID: "researcher:legacy-marker-injector", TargetAgentID: targetAgentID,
		ChannelID: "doc-legacy-marker-injector", TrajectoryID: "legacy-trajectory-marker-injector",
		Role: agentprofile.Researcher,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update", Summary: "legacy marker update",
		},
		Content: "legacy marker content", CreatedAt: time.Now().UTC(),
	}
	message := &types.ChannelMessage{
		ChannelID: update.ChannelID, FromAgentID: update.AgentID, ToAgentID: update.TargetAgentID,
		TrajectoryID: update.TrajectoryID, Role: update.Role, Content: update.Content, Timestamp: update.CreatedAt,
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, update, message); err != nil || !created {
		t.Fatalf("dispatch legacy marker update: created=%t err=%v", created, err)
	}
	rec := d9CoagentRun("run-legacy-marker-injector", ownerID, targetAgentID, agentprofile.Texture, update.ChannelID, "")
	rec.Metadata["lifecycle_work_item_id"] = "legacy-work-item"
	pending, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, targetAgentID, 10)
	if err != nil {
		t.Fatalf("list marker-only legacy updates: %v", err)
	}
	if len(pending) != 1 || pending[0].UpdateID != update.UpdateID {
		t.Fatalf("marker-only run selected lifecycle authority: %+v", pending)
	}
}

func TestUpdateCoagentRejectsTrajectoryMarkerAsLifecycleAuthority(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	const (
		ownerID = "user-legacy-producer-lifecycle-collision"
		docID   = "doc-legacy-producer-lifecycle-collision"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	legacy := d9CoagentRun(
		"run-legacy-producer-lifecycle-collision", ownerID,
		"researcher:legacy-producer-lifecycle-collision", agentprofile.Researcher, docID, "",
	)
	legacy.ComputerID, legacy.TrajectoryID = "autoputer-test", trajectoryID
	legacy.Metadata[runMetadataTrajectoryID] = trajectoryID
	now := time.Now().UTC()
	legacy.State, legacy.CreatedAt, legacy.UpdatedAt = types.RunRunning, now, now
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: legacy.AgentID, OwnerID: ownerID, ComputerID: "autoputer-test", Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: docID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("upsert durable legacy producer: %v", err)
	}
	if err := s.CreateRunOG(ctx, *legacy); err != nil {
		t.Fatalf("create durable pre-cutover producer row: %v", err)
	}
	raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy producer remains legacy","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","claims":[{"text":"legacy producer remains legacy"}]}`)
	_, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
		toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(legacy)),
		"update_coagent", raw,
	)
	if err == nil {
		t.Fatal("marker-only legacy producer bypassed lifecycle authority")
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "autoputer-test", trajectoryID)
	if err != nil || len(snapshot.Updates) != 0 {
		t.Fatalf("marker-only legacy producer queued lifecycle update: %+v, %v", snapshot.Updates, err)
	}
}

func d9InstallTools(t *testing.T, rt *Runtime) {
	t.Helper()
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install default tools: %v", err)
	}
}

func d9CoagentRun(runID, ownerID, agentID, profile, channelID, requestedTextureAgentID string) *types.RunRecord {
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    profile,
		runMetadataAgentID:      agentID,
		runMetadataChannelID:    channelID,
	}
	if requestedTextureAgentID != "" {
		metadata["requested_by_profile"] = agentprofile.Texture
		metadata["requested_by_agent_id"] = requestedTextureAgentID
	}
	return &types.RunRecord{
		RunID:        runID,
		OwnerID:      ownerID,
		AgentID:      agentID,
		AgentProfile: profile,
		AgentRole:    profile,
		ChannelID:    channelID,
		ComputerID:   "autoputer-test",
		Metadata:     metadata,
	}
}

func schemaObject(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("schema key %q = %#v, want object", key, parent[key])
	}
	return child
}

func TestLifecycleRuntimeSubmissionPreservesCanonicalActivationAdmission(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const ownerID = "user-runtime-active-run-cas"
	const docID = "doc-runtime-active-run-cas"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	const agentID = "researcher-runtime-active-run-cas"
	const channelID = "researcher-runtime-active-run-channel"
	const workItemID = "work-runtime-active-run-cas"
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: agentID, OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: channelID,
	}); err != nil {
		t.Fatalf("seed researcher agent: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		CommandID: "command-open-runtime-active-run-cas", TrajectoryID: trajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: workItemID, Objective: "admit exactly one runtime activation",
			AssignedAgentID: agentID, AuthorityProfile: agentprofile.Researcher,
		},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open researcher work: %v", err)
	}
	peer := testPeerRuntime(t, rt, s)
	runtimes := []*Runtime{rt, peer}
	baseMetadata := map[string]any{
		runMetadataAgentID: agentID, runMetadataAgentProfile: agentprofile.Researcher,
		runMetadataAgentRole: agentprofile.Researcher, runMetadataChannelID: channelID,
		runMetadataTrajectoryID: trajectoryID, "lifecycle_work_item_id": workItemID,
	}
	type result struct {
		run *types.RunRecord
		err error
	}
	results := make(chan result, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	start := make(chan struct{})
	for i := range runtimes {
		go func(candidate *Runtime) {
			metadata := make(map[string]any, len(baseMetadata))
			for key, value := range baseMetadata {
				metadata[key] = value
			}
			ready.Done()
			<-start
			run, err := candidate.createRunWithMetadata(ctx, "resume overlapping durable research", ownerID, metadata)
			results <- result{run: run, err: err}
		}(runtimes[i])
	}
	ready.Wait()
	close(start)
	first, second := <-results, <-results
	outcomes := []result{first, second}
	var winner *types.RunRecord
	for _, outcome := range outcomes {
		if outcome.err == nil {
			if winner != nil {
				t.Fatalf("runtime submissions admitted duplicate activations: %+v", outcomes)
			}
			winner = outcome.run
			continue
		}
		if !errors.Is(outcome.err, store.ErrConcurrentStateChange) &&
			!errors.Is(outcome.err, store.ErrLifecycleInvalidTransition) {
			t.Fatalf("runtime activation conflict = %v", outcome.err)
		}
	}
	if winner == nil {
		t.Fatalf("runtime submissions admitted no activation: %+v", outcomes)
	}
	agent, err := s.GetAgentByScope(ctx, ownerID, rt.TextureComputerID(), agentID)
	if err != nil || agent.ActiveRunID != winner.RunID {
		t.Fatalf("canonical active_run_id = %q, %v; want %q", agent.ActiveRunID, err, winner.RunID)
	}
}

func TestGenericRestartRewarmDefersTexturePendingLifecycleBindingsToDocumentOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		multiple bool
		terminal bool
	}{
		{name: "pending_evidence_update"},
		{name: "single_terminal", terminal: true},
		{name: "multi_terminal_retains_other_open_item", multiple: true, terminal: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, s := testRuntime(t)
			ctx := context.Background()
			ownerID := "user-lifecycle-rewarm-terminal-" + tc.name
			docID := "doc-lifecycle-rewarm-terminal-" + tc.name
			trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
			agentID := currentTextureAgentID(docID)
			firstWorkItemID := "test-work:" + ownerID + ":" + docID
			workItemIDs := []string{firstWorkItemID}
			secondWorkItemID := ""
			if tc.multiple {
				secondWorkItemID = "work-lifecycle-rewarm-second-" + tc.name
				open := types.OpenLifecycleWorkRequest{
					OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
					CommandID: "command-open-lifecycle-rewarm-second-" + tc.name, TrajectoryID: trajectoryID,
					WorkItem: types.WorkItemRecord{
						WorkItemID: secondWorkItemID, Objective: "retain the still-open restart binding",
						AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture,
					},
				}
				open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
				if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
					t.Fatalf("open second lifecycle work: %v", err)
				}
				workItemIDs = append(workItemIDs, secondWorkItemID)
			}
			now := time.Now().UTC()
			run := types.RunRecord{
				RunID:   "run-lifecycle-rewarm-terminal-" + tc.name,
				AgentID: agentID, OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
				ChannelID: docID, TrajectoryID: trajectoryID,
				State: types.RunRunning, Prompt: "interrupted lifecycle producer",
				AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
				CreatedAt: now, UpdatedAt: now,
				Metadata: map[string]any{
					runMetadataAgentID: agentID, runMetadataAgentProfile: agentprofile.Texture,
					runMetadataAgentRole: agentprofile.Texture, runMetadataTrajectoryID: trajectoryID,
					"work_item_ids": workItemIDs,
				},
			}
			if !tc.multiple {
				run.Metadata["lifecycle_work_item_id"] = firstWorkItemID
			}
			if err := s.CreateRun(ctx, run); err != nil {
				t.Fatalf("project interrupted lifecycle activation: %v", err)
			}
			packet := types.CoagentSourcePacketPayload{
				SchemaVersion: types.CoagentSourcePacketSchemaV1,
				Kind:          "evidence_update",
				Summary:       "terminal typed disposition already queued",
			}
			content := "terminal typed disposition already queued"
			payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
			queue := types.QueueLifecycleUpdateRequest{
				OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
				CommandID:    "command-queue-lifecycle-rewarm-terminal-" + tc.name,
				TrajectoryID: trajectoryID, TargetAgentID: agentID, ProducerAgentID: agentID,
				ProducerUpdateID: "producer-lifecycle-rewarm-terminal-" + tc.name,
				UpdateID:         "update-lifecycle-rewarm-terminal-" + tc.name,
				ChannelID:        docID, Role: agentprofile.Texture, SourceRunID: run.RunID,
				Packet: packet, Content: content, PayloadDigest: payloadDigest,
			}
			queue.WorkDisposition = types.WorkItemOpen
			queue.WorkItemID = firstWorkItemID
			if tc.terminal {
				queue.WorkDisposition = types.WorkItemCompleted
			}
			queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
			if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
				t.Fatalf("queue terminal lifecycle update: %v", err)
			}
			var dispatched []string
			rt.SetDispatchActor(func(_ context.Context, gotOwnerID, gotComputerID, toAgentID, kind, content, gotTrajectoryID, _ string) error {
				if kind == "initial_dispatch" && gotOwnerID == ownerID && gotComputerID == rt.TextureComputerID() &&
					gotTrajectoryID == trajectoryID && toAgentID == agentID {
					dispatched = append(dispatched, content)
				}
				return nil
			})
			rt.Start(ctx)
			if len(dispatched) != 0 {
				t.Fatalf("generic restart claimed Texture dispatch authority: %v", dispatched)
			}
			stale, err := s.GetLifecycleRun(ctx, ownerID, rt.TextureComputerID(), run.RunID)
			if err != nil || stale.State != types.RunPassivated {
				t.Fatalf("stale restart activation = %+v, %v; want passivated", stale, err)
			}
			if active, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil || found {
				t.Fatalf("generic restart created Texture replacement: found=%t run=%+v err=%v", found, active, err)
			}
		})
	}
}

func schemaEnumContains(schema map[string]any, want string) bool {
	values, ok := schema["enum"].([]string)
	if ok {
		for _, value := range values {
			if value == want {
				return true
			}
		}
		return false
	}
	anyValues, ok := schema["enum"].([]any)
	if !ok {
		return false
	}
	for _, value := range anyValues {
		if value == want {
			return true
		}
	}
	return false
}

func schemaRequiredContains(schema map[string]any, want string) bool {
	values, ok := schema["required"].([]string)
	if ok {
		for _, value := range values {
			if value == want {
				return true
			}
		}
		return false
	}
	anyValues, ok := schema["required"].([]any)
	if !ok {
		return false
	}
	for _, value := range anyValues {
		if value == want {
			return true
		}
	}
	return false
}

func d9UpdateID(t *testing.T, raw string) string {
	t.Helper()
	var resp struct {
		UpdateID string `json:"update_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode update response: %v\n%s", err, raw)
	}
	if strings.TrimSpace(resp.UpdateID) == "" {
		t.Fatalf("response missing update_id: %s", raw)
	}
	return resp.UpdateID
}

func TestLifecycleResearcherAdmissionErrorPassivationRecoversOnce(t *testing.T) {
	rt, s := testRuntime(t)
	counting := newResearcherAdmissionCountingProvider()
	rt.provider = counting
	fixture := bindResearcherControlFixture(t, rt, s, "owner-admission-recovery", "admission-recovery")
	ctx := context.Background()

	// Model the retryable provider-admission read/CAS failure after the exact
	// control bind but before provider entry. The one-shot initial_dispatch may
	// already be processed, so restart must use a distinct exact occurrence.
	rt.passivateLifecycleResearcherAfterAdmissionError(ctx, &fixture.run, errors.New("transient admission store outage"))
	if fixture.run.State != types.RunPassivated || metadataStringValue(fixture.run.Metadata, "passivated_reason") != lifecycleResearcherAdmissionRetryReason {
		t.Fatalf("retryable admission failure state=%s reason=%q", fixture.run.State, metadataStringValue(fixture.run.Metadata, "passivated_reason"))
	}

	type dispatchRecord struct{ ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string }
	var dispatches []dispatchRecord
	rt.SetDispatchActor(func(_ context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
		dispatches = append(dispatches, dispatchRecord{ownerID: ownerID, computerID: computerID, toAgentID: toAgentID, kind: kind, content: content, trajectoryID: trajectoryID, fromAgentID: fromAgentID})
		return nil
	})
	rt.reactivateRetryableLifecycleInjectionRuns(ctx, fixture.run.ComputerID)
	if len(dispatches) != 1 || dispatches[0].kind != "coagent_result" || !strings.HasPrefix(dispatches[0].content, "lifecycle-researcher-admission-recovery:v1:") {
		t.Fatalf("recovery dispatches=%+v", dispatches)
	}
	if dispatches[0].ownerID != fixture.run.OwnerID || dispatches[0].computerID != fixture.run.ComputerID || dispatches[0].toAgentID != fixture.run.AgentID || dispatches[0].trajectoryID != fixture.run.TrajectoryID {
		t.Fatalf("recovery dispatch scope=%+v", dispatches[0])
	}
	recovered, err := s.GetLifecycleRun(ctx, fixture.run.OwnerID, fixture.run.ComputerID, fixture.run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State != types.RunPending {
		t.Fatalf("recovered state=%s", recovered.State)
	}

	// The recovered exact run passes the central admission boundary once. Open
	// work remains open, but completion does not mint a generic successor.
	rt.ExecuteActivationSync(ctx, &recovered)
	if counting.Count() != 1 || recovered.State != types.RunCompleted {
		t.Fatalf("recovered provider calls=%d state=%s", counting.Count(), recovered.State)
	}
	rt.reactivateRetryableLifecycleInjectionRuns(ctx, fixture.run.ComputerID)
	if len(dispatches) != 1 || counting.Count() != 1 {
		t.Fatalf("repeated recovery dispatches=%d provider calls=%d", len(dispatches), counting.Count())
	}
	work, err := s.GetLifecycleWorkItem(ctx, fixture.run.OwnerID, fixture.run.ComputerID, fixture.workID)
	if err != nil {
		t.Fatal(err)
	}
	if work.Status != types.WorkItemOpen {
		t.Fatalf("recovery silently settled open responsibility: %s", work.Status)
	}
}
