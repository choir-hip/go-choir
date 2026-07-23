package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestUpdateCoagentAcceptsResearcherEvidenceUpdateSourcePacket(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-researcher"
	docID := "doc-d9-researcher"
	seedDurableTextureSubject(t, s, ownerID, docID)
	researcherRun := d9CoagentRun("run-d9-researcher", ownerID, "researcher:d9", agentprofile.Researcher, docID, "")
	researcherRun.Metadata[runMetadataTrajectoryID] = "legacy-trajectory-d9-researcher"
	raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(researcherRun)), "update_coagent", json.RawMessage(`{
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
	updateID := d9UpdateID(t, raw)
	stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
	if err != nil {
		t.Fatalf("get stored packet: %v", err)
	}
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
	ctx := context.Background()
	const (
		ownerID = "user-producer-work-disposition"
		docID   = "doc-producer-work-disposition"
		workID  = "work-producer-work-disposition"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	producerAgentID := "researcher:producer-work-disposition"
	producerWork := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: "sandbox-test",
		CommandID: "command-open-producer-work-disposition", TrajectoryID: trajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: workID, Objective: "produce lifecycle evidence",
			AssignedAgentID: producerAgentID, AuthorityProfile: agentprofile.Researcher,
		},
	}
	producerWork.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(producerWork)
	if _, err := s.OpenLifecycleWork(ctx, producerWork); err != nil {
		t.Fatalf("open producer lifecycle work: %v", err)
	}
	var activeRun *types.RunRecord
	projectRun := func(run *types.RunRecord) {
		now := time.Now().UTC()
		if activeRun != nil {
			terminal := *activeRun
			terminal.State = types.RunCompleted
			terminal.UpdatedAt, terminal.FinishedAt = now, &now
			release := types.ReplaceLifecycleActivationRequest{
				OwnerID: ownerID, ComputerID: "sandbox-test",
				CommandID:    "release-producer-run:" + terminal.RunID,
				TrajectoryID: trajectoryID, AgentID: terminal.AgentID, Run: terminal,
			}
			release.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(release)
			if _, err := s.ProjectTerminalLifecycleRun(ctx, release); err != nil {
				t.Fatalf("release lifecycle producer run %s: %v", terminal.RunID, err)
			}
		}
		run.State = types.RunPending
		run.CreatedAt, run.UpdatedAt = now, now
		t.Helper()
		run.TrajectoryID = trajectoryID
		run.Metadata[runMetadataTrajectoryID] = trajectoryID
		run.Metadata["lifecycle_work_item_id"] = workID
		if err := s.UpsertAgent(ctx, types.AgentRecord{
			AgentID: run.AgentID, OwnerID: ownerID, ComputerID: "sandbox-test", SandboxID: "sandbox-test",
			Profile: run.AgentProfile, Role: run.AgentRole, ChannelID: run.ChannelID,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("upsert lifecycle producer agent %s: %v", run.AgentID, err)
		}
		project := types.ReplaceLifecycleActivationRequest{
			OwnerID: ownerID, ComputerID: "sandbox-test",
			CommandID:    "project-producer-run:" + run.RunID,
			TrajectoryID: trajectoryID, AgentID: run.AgentID, Run: *run,
		}
		project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
		if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
			t.Fatalf("project lifecycle producer run %s: %v", run.RunID, err)
		}
		activeRun = run
	}
	missing := d9CoagentRun("run-producer-missing-authority", ownerID, producerAgentID, agentprofile.Researcher, docID, "")
	projectRun(missing)
	if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
		toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(missing)),
		"update_coagent",
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"missing authority fields","agent_id":"texture:`+docID+`"}`),
	); err == nil {
		t.Fatal("lifecycle update accepted omitted producer_update_id")
	}
	for name, producerUpdateID := range map[string]string{
		"run": missing.RunID, "timestamp": "2026-07-22T13:00:00Z",
		"uuid_v1": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "opaque": "producer-open",
	} {
		t.Run("producer_identity_"+name, func(t *testing.T) {
			raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"invalid producer identity","agent_id":"texture:` + docID + `","producer_update_id":"` + producerUpdateID + `","work_disposition":"open"}`)
			if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(missing)), "update_coagent", raw); err == nil {
				t.Fatalf("lifecycle update accepted forbidden producer identity %q", producerUpdateID)
			}
		})
	}
	execute := func(runID, disposition, summary string) types.CoagentSourcePacket {
		t.Helper()
		run := d9CoagentRun(runID, ownerID, producerAgentID, agentprofile.Researcher, docID, "")
		projectRun(run)
		producerUpdateID := map[string]string{
			"open": "11111111-1111-4111-8111-111111111111", "completed": "22222222-2222-4222-8222-222222222222",
		}[disposition]
		_, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
			toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)),
			"update_coagent",
			json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"`+summary+`","agent_id":"texture:`+docID+`","channel_id":"`+docID+`","producer_update_id":"`+producerUpdateID+`","work_disposition":"`+disposition+`","claims":[{"text":"`+summary+`"}]}`),
		)
		if err != nil {
			t.Fatalf("update_coagent %s: %v", disposition, err)
		}
		stored, err := s.GetLifecycleUpdate(ctx, ownerID, "sandbox-test", trajectoryID, currentTextureAgentID(docID), producerAgentID, producerUpdateID)
		if err != nil {
			t.Fatalf("get %s update: %v", disposition, err)
		}
		return stored
	}
	omittedRun := d9CoagentRun("run-producer-omitted", ownerID, producerAgentID, agentprofile.Researcher, docID, "")
	omittedRun.Metadata[runMetadataTrajectoryID] = trajectoryID
	omittedRun.Metadata["lifecycle_work_item_id"] = workID
	projectRun(omittedRun)
	_, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
		toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(omittedRun)),
		"update_coagent",
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"omitted disposition remains open","agent_id":"texture:`+docID+`","channel_id":"`+docID+`","producer_update_id":"33333333-3333-4333-8333-333333333333","claims":[{"text":"omitted disposition remains open"}]}`),
	)
	if err != nil {
		t.Fatalf("update_coagent omitted disposition: %v", err)
	}
	omitted, err := s.GetLifecycleUpdate(ctx, ownerID, "sandbox-test", trajectoryID, currentTextureAgentID(docID), producerAgentID, "33333333-3333-4333-8333-333333333333")
	if err != nil || omitted.WorkDisposition != types.WorkItemOpen || omitted.WorkItemID != workID {
		t.Fatalf("omitted disposition did not preserve assigned open work: %+v, %v", omitted, err)
	}
	work, err := s.GetLifecycleWorkItem(ctx, ownerID, "sandbox-test", workID)
	if err != nil || work.Status != types.WorkItemOpen {
		t.Fatalf("omitted disposition settled assigned work: %+v, %v", work, err)
	}
	open := execute("run-producer-open", "open", "interim evidence remains incomplete")
	if open.WorkDisposition != types.WorkItemOpen || open.WorkItemID != workID {
		t.Fatalf("open checkpoint omitted assigned open work consequence: %+v", open)
	}
	completed := execute("run-producer-completed", "completed", "assigned evidence work is complete")
	if completed.WorkDisposition != types.WorkItemCompleted || completed.WorkItemID != workID {
		t.Fatalf("completed checkpoint omitted explicit work consequence: %+v", completed)
	}
	if completed.UpdateID == open.UpdateID || completed.ProducerUpdateID == open.ProducerUpdateID {
		t.Fatalf("distinct producer commands reused update identity: open=%+v completed=%+v", open, completed)
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
		OwnerID: ownerID, SandboxID: "sandbox-test", State: types.RunRunning,
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
		raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"` + disposition + ` lifecycle checkpoint","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","producer_update_id":"` + producerUpdateID + `","work_disposition":"` + disposition + `","claims":[{"text":"` + disposition + ` lifecycle checkpoint"}]}`)
		if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
			toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(child)),
			"update_coagent", raw,
		); err != nil {
			t.Fatalf("queue %s spawned lifecycle update: %v", disposition, err)
		}
	}
	execute("33333333-3333-4333-8333-333333333333", "open")
	execute("44444444-4444-4444-8444-444444444444", "completed")
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
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
		OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "project-terminal-spawned-researcher",
		TrajectoryID: trajectoryID, AgentID: child.AgentID, Run: terminal,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ProjectTerminalLifecycleRun(ctx, project); err != nil {
		t.Fatalf("project terminal lifecycle researcher: %v", err)
	}
	persisted, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", child.RunID)
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
	if replacement, err := rt.continueOpenLifecycleWorkAfterTerminal(ctx, &persisted); err != nil || replacement != nil {
		t.Fatalf("terminal producer disposition created redundant activation: %+v, %v", replacement, err)
	}
}

func TestCompletedLifecycleActivationReactivatesOpenWorkWithoutSettlingFromRunResult(t *testing.T) {
	tests := []struct {
		name             string
		producerUpdateID string
		requestSource    string
		boot             bool
		multiple         bool
		concurrent       bool
		settleBefore     bool
	}{
		{name: "immediate", producerUpdateID: "55555555-5555-4555-8555-555555555555", requestSource: "terminal_activation_work_recovery"},
		{name: "boot", producerUpdateID: "66666666-6666-4666-8666-666666666666", requestSource: "trajectory_work_item_sweep", boot: true},
		{name: "boot_multiple", producerUpdateID: "77777777-7777-4777-8777-777777777777", requestSource: "trajectory_work_item_sweep", boot: true, multiple: true},
		{name: "immediate_concurrent", producerUpdateID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", requestSource: "terminal_activation_work_recovery", concurrent: true},
		{name: "immediate_settled_before_reconcile", producerUpdateID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", requestSource: "terminal_activation_work_recovery", settleBefore: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rt, s := testRuntime(t)
			d9InstallTools(t, rt)
			rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
				return nil
			})
			ctx := context.Background()
			ownerID := "user-terminal-lifecycle-recovery-" + tc.name
			docID := "doc-terminal-lifecycle-recovery-" + tc.name
			trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
			now := time.Now().UTC()
			parent := types.RunRecord{
				RunID: "run-terminal-lifecycle-parent-" + tc.name, AgentID: "texture:" + docID, ChannelID: docID,
				AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
				OwnerID: ownerID, SandboxID: "sandbox-test", State: types.RunRunning,
				TrajectoryID: trajectoryID, CreatedAt: now, UpdatedAt: now,
				Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataChannelID: docID},
			}
			if err := s.CreateRun(ctx, parent); err != nil {
				t.Fatalf("create lifecycle parent activation: %v", err)
			}
			child, err := rt.StartCoagentRun(ctx, parent.RunID, "finish the assigned evidence work", ownerID, map[string]any{
				runMetadataAgentProfile: agentprofile.Researcher,
				runMetadataAgentRole:    agentprofile.Researcher,
				runMetadataChannelID:    docID,
			})
			if err != nil {
				t.Fatalf("start lifecycle researcher: %v", err)
			}
			workItemID := metadataStringValue(child.Metadata, "lifecycle_work_item_id")
			if workItemID == "" {
				t.Fatal("spawned lifecycle work binding is missing")
			}
			secondWorkItemID := ""
			if tc.multiple {
				secondWorkItemID = "work-terminal-lifecycle-recovery-second-" + tc.name
				openSecond := types.OpenLifecycleWorkRequest{
					OwnerID: ownerID, ComputerID: "sandbox-test",
					CommandID: "command-open-terminal-lifecycle-recovery-second-" + tc.name, TrajectoryID: trajectoryID,
					WorkItem: types.WorkItemRecord{
						WorkItemID: secondWorkItemID, Objective: "verify the second assigned evidence source",
						AssignedAgentID: child.AgentID, AuthorityProfile: agentprofile.Researcher,
					},
				}
				openSecond.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(openSecond)
				if _, err := s.OpenLifecycleWork(ctx, openSecond); err != nil {
					t.Fatalf("open second assigned lifecycle work: %v", err)
				}
			}
			if _, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
				toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(child)),
				"update_coagent",
				json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"interim evidence only","agent_id":"texture:`+docID+`","channel_id":"`+docID+`","producer_update_id":"`+tc.producerUpdateID+`","work_disposition":"open","claims":[{"text":"interim evidence only"}]}`),
			); err != nil {
				t.Fatalf("queue open lifecycle checkpoint: %v", err)
			}

			terminal := *child
			terminal.State = types.RunCompleted
			terminal.Result = "final prose that is not lifecycle authority"
			finishedAt := time.Now().UTC()
			terminal.UpdatedAt, terminal.FinishedAt = finishedAt, &finishedAt
			project := types.ReplaceLifecycleActivationRequest{
				OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "project-terminal-open-work-recovery-" + tc.name,
				TrajectoryID: trajectoryID, AgentID: child.AgentID, Run: terminal,
			}
			project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
			if _, err := s.ProjectTerminalLifecycleRun(ctx, project); err != nil {
				t.Fatalf("project terminal lifecycle researcher: %v", err)
			}
			persisted, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", child.RunID)
			if err != nil {
				t.Fatalf("reload terminal lifecycle researcher: %v", err)
			}
			openItems, err := s.ListOpenAssignedLifecycleWorkItems(ctx, "sandbox-test", 0)
			if err != nil {
				t.Fatalf("list boot-recoverable lifecycle work: %v", err)
			}
			foundOpenWork := false
			for _, item := range openItems {
				if item.WorkItemID == workItemID {
					foundOpenWork = true
					break
				}
			}
			if !foundOpenWork {
				t.Fatalf("boot lifecycle work inventory omitted %q: %+v", workItemID, openItems)
			}

			if tc.settleBefore {
				settle := types.SettleLifecycleWorkRequest{
					OwnerID: ownerID, ComputerID: "sandbox-test",
					CommandID: "command-settle-before-reconcile-" + tc.name, TrajectoryID: trajectoryID,
					WorkItemID: workItemID, ResultRef: "artifact://terminal-recovery/already-settled",
					ActingAgentID: child.AgentID,
				}
				settle.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settle)
				if _, err := s.SettleLifecycleWork(ctx, settle); err != nil {
					t.Fatalf("settle work before reconcile: %v", err)
				}
				replacement, err := rt.continueOpenLifecycleWorkAfterTerminal(ctx, &persisted)
				if err != nil {
					t.Fatalf("reconcile settled lifecycle work: %v", err)
				}
				if replacement != nil {
					t.Fatalf("settled lifecycle work reactivated from stale inventory: %+v", replacement)
				}
				return
			}
			var replacement *types.RunRecord
			if tc.boot {
				rt.sweepOpenWorkItemActors(ctx)
				active, found, activeErr := rt.activeRunByAgent(ctx, ownerID, child.AgentID)
				if activeErr != nil || !found {
					t.Fatalf("load boot replacement activation: found=%t run=%+v err=%v", found, active, activeErr)
				}
				replacement = &active
			} else if tc.concurrent {
				type continuationResult struct {
					run *types.RunRecord
					err error
				}
				results := make(chan continuationResult, 2)
				for range 2 {
					go func() {
						run, continueErr := rt.continueOpenLifecycleWorkAfterTerminal(ctx, &persisted)
						results <- continuationResult{run: run, err: continueErr}
					}()
				}
				firstResult, secondResult := <-results, <-results
				if firstResult.err != nil || secondResult.err != nil ||
					firstResult.run == nil || secondResult.run == nil ||
					firstResult.run.RunID != secondResult.run.RunID {
					t.Fatalf("concurrent terminal continuations = (%+v, %v), (%+v, %v); want one activation", firstResult.run, firstResult.err, secondResult.run, secondResult.err)
				}
				replacement = firstResult.run
			} else {
				replacement, err = rt.continueOpenLifecycleWorkAfterTerminal(ctx, &persisted)
				if err != nil {
					t.Fatalf("continue open lifecycle work: %v", err)
				}
			}
			if replacement == nil || replacement.RunID == child.RunID || replacement.State != types.RunPending {
				t.Fatalf("replacement activation = %+v, want a distinct pending run", replacement)
			}
			if tc.multiple {
				if got := metadataStringValue(replacement.Metadata, "lifecycle_work_item_id"); got != "" {
					t.Fatalf("multi-item replacement lifecycle_work_item_id = %q, want empty", got)
				}
				ids := metadataStringSlice(replacement.Metadata["work_item_ids"])
				if len(ids) != 2 || !containsString(ids, workItemID) || !containsString(ids, secondWorkItemID) {
					t.Fatalf("multi-item replacement work_item_ids = %v, want %q and %q", ids, workItemID, secondWorkItemID)
				}
			} else if got := metadataStringValue(replacement.Metadata, "lifecycle_work_item_id"); got != workItemID {
				t.Fatalf("replacement lifecycle_work_item_id = %q, want %q", got, workItemID)
			}
			if got := metadataStringValue(replacement.Metadata, "request_source"); got != tc.requestSource {
				t.Fatalf("replacement request_source = %q, want %q", got, tc.requestSource)
			}
			if !strings.Contains(replacement.Prompt, "RunRecord completion do not settle work") ||
				!strings.Contains(replacement.Prompt, "finish the assigned evidence work") {
				t.Fatalf("replacement prompt does not require native terminal disposition: %q", replacement.Prompt)
			}
			if tc.multiple {
				updateTool := rt.ToolRegistryForProfile(agentprofile.Researcher)
				combinedMarkerRun := *replacement
				combinedMarkerRun.Metadata = make(map[string]any, len(replacement.Metadata)+1)
				for key, value := range replacement.Metadata {
					combinedMarkerRun.Metadata[key] = value
				}
				combinedMarkerRun.Metadata["lifecycle_work_item_id"] = workItemID
				updateCtx := toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&combinedMarkerRun))
				missingWorkID := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"ambiguous multi-item update","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","producer_update_id":"88888888-8888-4888-8888-888888888888","work_disposition":"open","claims":[{"text":"must select one item"}]}`)
				if _, err := updateTool.Execute(updateCtx, "update_coagent", missingWorkID); err == nil || !strings.Contains(err.Error(), "requires work_item_id") {
					t.Fatalf("multi-item update without work_item_id error = %v", err)
				}
				unassignedWorkID := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"unassigned item update","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","producer_update_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","work_item_id":"work-not-assigned","work_disposition":"open","claims":[{"text":"must reject foreign work"}]}`)
				if _, err := updateTool.Execute(updateCtx, "update_coagent", unassignedWorkID); err == nil || !strings.Contains(err.Error(), "not assigned") {
					t.Fatalf("multi-item update for unassigned work error = %v", err)
				}
				selectedUpdate := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"second item checkpoint","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","producer_update_id":"99999999-9999-4999-8999-999999999999","work_item_id":"` + secondWorkItemID + `","work_disposition":"open","claims":[{"text":"second item remains open"}]}`)
				if _, err := updateTool.Execute(updateCtx, "update_coagent", selectedUpdate); err != nil {
					t.Fatalf("queue selected multi-item lifecycle update: %v", err)
				}
				snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
				if err != nil {
					t.Fatalf("load multi-item lifecycle snapshot: %v", err)
				}
				foundSelectedUpdate := false
				for _, update := range snapshot.Updates {
					if update.ProducerUpdateID == "99999999-9999-4999-8999-999999999999" && update.WorkItemID == secondWorkItemID {
						foundSelectedUpdate = true
						break
					}
				}
				if !foundSelectedUpdate {
					t.Fatalf("selected multi-item update missing or misbound: %+v", snapshot.Updates)
				}

				replacementTerminal := *replacement
				replacementTerminal.State = types.RunCompleted
				replacementTerminal.Result = "multi-item prose is still not work authority"
				replacementFinishedAt := time.Now().UTC()
				replacementTerminal.UpdatedAt, replacementTerminal.FinishedAt = replacementFinishedAt, &replacementFinishedAt
				replaceTerminal := types.ReplaceLifecycleActivationRequest{
					OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "project-terminal-multi-item-recovery-" + tc.name,
					TrajectoryID: trajectoryID, AgentID: replacement.AgentID, Run: replacementTerminal,
				}
				replaceTerminal.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(replaceTerminal)
				if _, err := s.ProjectTerminalLifecycleRun(ctx, replaceTerminal); err != nil {
					t.Fatalf("project terminal multi-item replacement: %v", err)
				}
				persistedReplacement, err := s.GetLifecycleRun(ctx, ownerID, "sandbox-test", replacement.RunID)
				if err != nil {
					t.Fatalf("reload terminal multi-item replacement: %v", err)
				}
				if err := rt.bindTerminalRunOutcome(ctx, &persistedReplacement, false); err != nil {
					t.Fatalf("bind terminal multi-item lifecycle outcome: %v", err)
				}
				legacyUpdates, err := s.ListWorkerUpdatesBySourceRun(ctx, ownerID, replacement.RunID)
				if err != nil {
					t.Fatalf("list legacy terminal updates for multi-item lifecycle run: %v", err)
				}
				if len(legacyUpdates) != 0 {
					t.Fatalf("multi-item lifecycle terminal prose synthesized legacy updates: %+v", legacyUpdates)
				}
				next, err := rt.continueOpenLifecycleWorkAfterTerminal(ctx, &persistedReplacement)
				if err != nil {
					t.Fatalf("continue multi-item lifecycle work: %v", err)
				}
				if next == nil || next.RunID == replacement.RunID {
					t.Fatalf("multi-item terminal continuation = %+v, want a distinct activation", next)
				}
				nextIDs := metadataStringSlice(next.Metadata["work_item_ids"])
				if len(nextIDs) != 2 || !containsString(nextIDs, workItemID) || !containsString(nextIDs, secondWorkItemID) {
					t.Fatalf("continued multi-item work_item_ids = %v, want both open items", nextIDs)
				}
			}
			work, err := s.GetLifecycleWorkItem(ctx, ownerID, "sandbox-test", workItemID)
			if err != nil || work.Status != types.WorkItemOpen || work.ResultRef != "" {
				t.Fatalf("RunRecord result settled lifecycle work: %+v, %v", work, err)
			}
		})
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
	ctx := context.Background()
	ownerID := "user-d9-source-alias"
	docID := "doc-d9-source-alias"
	seedDurableTextureSubject(t, s, ownerID, docID)
	run := d9CoagentRun("run-d9-source-alias", ownerID, "researcher:d9-source-alias", agentprofile.Researcher, docID, "")
	raw, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "update_coagent", json.RawMessage(`{
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
	stored, err := s.GetWorkerUpdate(ctx, ownerID, d9UpdateID(t, raw))
	if err != nil {
		t.Fatalf("get stored alias packet: %v", err)
	}
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
	props := schemaObject(t, tool.Parameters, "properties")
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

func TestPersistentSuperIgnoresNonExecutionRequestUpdatePackets(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-super-ignore"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}
	coSuperRun := d9CoagentRun("run-d9-super-ignore", ownerID, "cosuper:d9-super-ignore", agentprofile.CoSuper, "", "")
	raw, err := rt.ToolRegistryForProfile(agentprofile.CoSuper).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(coSuperRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"status evidence for Super, not an execution request",
		"agent_id":"`+superAgent.AgentID+`",
		"channel_id":"`+superAgent.ChannelID+`",
		"claims":[{"text":"A non-execution evidence packet should stay pending for Super review."}],
		"notes":["This packet must not start privileged execution."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent evidence_update to super: %v", err)
	}
	updateID := d9UpdateID(t, raw)
	if run, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID); err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	} else if run != nil {
		t.Fatalf("evidence_update started persistent Super run: %+v", run)
	}
	runs, err := s.ListRunsByOwner(ctx, ownerID, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	for _, run := range runs {
		if run.AgentID == superAgent.AgentID && metadataStringValue(run.Metadata, "request_source") == "update_coagent" {
			t.Fatalf("persistent Super run was created for non-execution packet: %+v", run)
		}
	}
	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list super backlog: %v", err)
	}
	if len(backlog) != 0 {
		t.Fatalf("super backlog = %+v, want 0 pending updates (settled non-execution packet)", backlog)
	}
	stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
	if err != nil {
		t.Fatalf("get settled non-execution update: %v", err)
	}
	if stored.DeliveredToRunID != "settled_non_executable" {
		t.Fatalf("delivered_to_loop_id = %q, want settled_non_executable", stored.DeliveredToRunID)
	}
	if stored.DeliveredAt == nil {
		t.Fatal("non-execution update delivered_at is nil")
	}
	rec := &types.RunRecord{
		RunID:        "run-d9-super-ignore-inject",
		OwnerID:      ownerID,
		AgentID:      superAgent.AgentID,
		AgentProfile: agentprofile.Super,
		AgentRole:    agentprofile.Super,
		ChannelID:    superAgent.ChannelID,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataAgentID:      superAgent.AgentID,
			"request_source":        "update_coagent",
		},
	}
	inject := rt.coagentUpdateTurnInjector(rec)
	if inject == nil {
		t.Fatal("persistent Super coagent update injector is nil")
	}
	msgs, err := inject(false)
	if err != nil {
		t.Fatalf("inject pending Super update: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("non-execution Super update was injected: %s", string(msgs[0]))
	}
	seed := []json.RawMessage{json.RawMessage(`{"role":"user","content":"base"}`)}
	prepended, err := rt.prependInitialCoagentUpdatePackets(ctx, rec, seed)
	if err != nil {
		t.Fatalf("cold inject pending Super update: %v", err)
	}
	if len(prepended) != len(seed) || string(prepended[0]) != string(seed[0]) {
		t.Fatalf("non-execution Super update was cold-injected: %v", prepended)
	}
}

func TestUpdateCoagentAcceptsSuperExecutionResultSourcesAndTextureCollatesPacketSourcesOnly(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-super-result"
	docID := "doc-d9-super-result"
	textureAgentID := currentTextureAgentID(docID)
	superRun := d9CoagentRun("run-d9-super-result", ownerID, "super:d9-result", agentprofile.Super, docID, textureAgentID)
	raw, err := rt.ToolRegistryForProfile(agentprofile.Super).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun)), "update_coagent", json.RawMessage(`{
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
	updateID := d9UpdateID(t, raw)
	stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
	if err != nil {
		t.Fatalf("get stored packet: %v", err)
	}
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
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "evidence_update",
		Summary:       "computer-scoped lifecycle update",
	}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "queue-lifecycle-injector",
		TrajectoryID: trajectoryID, TargetAgentID: targetAgentID, ProducerAgentID: "researcher:lifecycle-injector",
		ProducerUpdateID: "producer-lifecycle-injector", UpdateID: "update-lifecycle-injector",
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
		SandboxID: "sandbox-test", ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunPending, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentID:      targetAgentID,
		},
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: "sandbox-test", CommandID: "activation:" + rec.RunID,
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
	legacy.Metadata[runMetadataTrajectoryID] = trajectoryID
	raw := json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy producer remains legacy","agent_id":"texture:` + docID + `","channel_id":"` + docID + `","claims":[{"text":"legacy producer remains legacy"}]}`)
	_, err := rt.ToolRegistryForProfile(agentprofile.Researcher).Execute(
		toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(legacy)),
		"update_coagent", raw,
	)
	if !errors.Is(err, store.ErrLifecycleAuthorityRequired) {
		t.Fatalf("marker-only legacy producer error = %v, want legacy writer authority refusal", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "sandbox-test", trajectoryID)
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
		SandboxID:    "sandbox-test",
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
		AgentID: agentID, OwnerID: ownerID, ComputerID: rt.TextureSandboxID(), SandboxID: rt.TextureSandboxID(),
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: channelID,
	}); err != nil {
		t.Fatalf("seed researcher agent: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: rt.TextureSandboxID(),
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
	agent, err := s.GetAgentByScope(ctx, ownerID, rt.TextureSandboxID(), agentID)
	if err != nil || agent.ActiveRunID != winner.RunID {
		t.Fatalf("canonical active_run_id = %q, %v; want %q", agent.ActiveRunID, err, winner.RunID)
	}
}

func TestRestartRewarmSuppressesTerminalPendingLifecycleBindings(t *testing.T) {
	for _, tc := range []struct {
		name         string
		multiple     bool
		wantDispatch int
	}{
		{name: "single"},
		{name: "multi_retains_other_open_item", multiple: true, wantDispatch: 1},
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
					OwnerID: ownerID, ComputerID: rt.TextureSandboxID(),
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
				AgentID: agentID, OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
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
				OwnerID: ownerID, ComputerID: rt.TextureSandboxID(),
				CommandID:    "command-queue-lifecycle-rewarm-terminal-" + tc.name,
				TrajectoryID: trajectoryID, TargetAgentID: agentID, ProducerAgentID: agentID,
				ProducerUpdateID: "producer-lifecycle-rewarm-terminal-" + tc.name,
				UpdateID:         "update-lifecycle-rewarm-terminal-" + tc.name,
				ChannelID:        docID, Role: agentprofile.Texture, SourceRunID: run.RunID,
				Packet: packet, Content: content, PayloadDigest: payloadDigest,
				WorkDisposition: types.WorkItemCompleted, WorkItemID: firstWorkItemID,
			}
			queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
			if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
				t.Fatalf("queue terminal lifecycle update: %v", err)
			}
			var dispatched []string
			rt.SetDispatchActor(func(_ context.Context, gotOwnerID, gotComputerID, toAgentID, kind, content, gotTrajectoryID, _ string) error {
				if kind == "initial_dispatch" && gotOwnerID == ownerID && gotComputerID == rt.TextureSandboxID() &&
					gotTrajectoryID == trajectoryID && toAgentID == agentID {
					dispatched = append(dispatched, content)
				}
				return nil
			})
			rt.Start(ctx)
			if len(dispatched) != tc.wantDispatch {
				t.Fatalf("restart lifecycle dispatches = %v, want %d", dispatched, tc.wantDispatch)
			}
			stale, err := s.GetLifecycleRun(ctx, ownerID, rt.TextureSandboxID(), run.RunID)
			if err != nil || stale.State != types.RunPassivated {
				t.Fatalf("stale restart activation = %+v, %v; want passivated", stale, err)
			}
			if tc.multiple {
				active, found, err := rt.activeRunByAgent(ctx, ownerID, agentID)
				if err != nil || !found || active.RunID == run.RunID {
					t.Fatalf("replacement restart activation = found=%t run=%+v err=%v", found, active, err)
				}
				ids := metadataStringSlice(active.Metadata["work_item_ids"])
				if len(ids) != 1 || ids[0] != secondWorkItemID {
					t.Fatalf("replacement restart work_item_ids = %v, want [%s]", ids, secondWorkItemID)
				}
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
