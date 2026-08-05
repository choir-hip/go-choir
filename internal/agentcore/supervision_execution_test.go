package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type supervisionSweepTestTimer struct{}

func (supervisionSweepTestTimer) Stop() bool { return true }

func TestSupervisionAttemptShapeSecondAttemptIsCanonicalRetry(t *testing.T) {
	kind, ordinal, prior := supervisionAttemptShape([]store.SupervisionAttemptLineage{{
		AttemptID: "attempt-1", AttemptKind: "initial", Ordinal: 1, Status: "returned",
	}}, "attempt-2")
	if kind != "retry" || ordinal != 2 || prior == nil || *prior != "attempt-1" {
		t.Fatalf("second attempt = (%q, %d, %v)", kind, ordinal, prior)
	}
}

func TestSupervisionAttemptShapeUsesUnboundedLineage(t *testing.T) {
	attempts := make([]store.SupervisionAttemptLineage, 9)
	for i := range attempts {
		attempts[i] = store.SupervisionAttemptLineage{
			AttemptID: fmt.Sprintf("attempt-%d", i+1), AttemptKind: "retry", Ordinal: uint64(i + 1),
		}
	}
	attempts[0].AttemptKind = "initial"
	kind, ordinal, prior := supervisionAttemptShape(attempts, "attempt-10")
	if kind != "retry" || ordinal != 10 || prior == nil || *prior != "attempt-9" {
		t.Fatalf("unbounded retry = (%q, %d, %v)", kind, ordinal, prior)
	}
}

func TestStoredSupervisionObservedBasePreservesAttemptBase(t *testing.T) {
	fallback := computerevent.SupervisionObservedBase{CanonicalEventHead: "current", IntentRevisionID: "intent-current", ArtifactHeadRevisionID: "artifact-current"}
	base := storedSupervisionObservedBase(map[string]any{runMetadataObservedBase: map[string]string{"canonical_event_head": "start", "intent_revision_id": "intent-start", "artifact_head_revision_id": "artifact-start"}}, fallback)
	if base.CanonicalEventHead != "start" || base.IntentRevisionID != "intent-start" || base.ArtifactHeadRevisionID != "artifact-start" {
		t.Fatalf("observed base = %#v", base)
	}
}

func TestTextureExecutionRequestAppendsCanonicalSupervisionMessageAndTargetsSuper(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-texture-super-delivery"
	docID := "doc-texture-super-delivery"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	d9InstallTools(t, rt)

	textureRun := &types.RunRecord{
		RunID: "run-texture-super-delivery", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: currentTextureAgentID(docID), ChannelID: docID, State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			runMetadataTrajectoryID: trajectoryID,
			runMetadataChannelID:    docID,
			"doc_id":                docID,
		},
	}
	superAgentID := persistentSuperAgentID(ownerID)
	var dispatched []types.CoagentSourcePacket
	rt.dispatchActor = func(_ context.Context, gotOwnerID, gotComputerID, gotAgentID, kind, content, gotTrajectoryID, fromAgentID string) error {
		dispatched = append(dispatched, types.CoagentSourcePacket{
			UpdateID: content, AgentID: fromAgentID, TargetAgentID: gotAgentID,
			OwnerID: gotOwnerID, ComputerID: gotComputerID, TrajectoryID: gotTrajectoryID,
			Content: kind,
		})
		return nil
	}
	raw := json.RawMessage(fmt.Sprintf(`{
		"agent_id":%q,
		"schema_version":"coagent_source_packet.v1",
		"kind":"execution_request",
		"producer_update_id":"ae681b5b-14f6-412f-a153-7b5a0924c7f8",
		"summary":"Inspect the canonical delivery path.",
		"actions":[{
			"type":"run_command",
			"objective":"Run the focused delivery verification.",
			"safety":{"mutation_class":"yellow","network":"forbidden","file_mutation":"forbidden"}
		}]
	}`, superAgentID))
	execCtx := toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(textureRun))
	if _, err := rt.ToolRegistryForProfile(agentprofile.Texture).Execute(execCtx, "update_coagent", raw); err != nil {
		t.Fatalf("Texture update_coagent execution_request: %v", err)
	}
	if len(dispatched) != 1 {
		t.Fatalf("dispatches = %+v, want one", dispatched)
	}
	if got := dispatched[0]; got.TargetAgentID != superAgentID || got.AgentID != textureRun.AgentID || got.TrajectoryID != trajectoryID || got.Content != "coagent_result" {
		t.Fatalf("dispatch = %+v, want canonical Texture-to-Super wake", got)
	}

	updates, err := rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), superAgentID, trajectoryID, 10)
	if err != nil {
		t.Fatalf("load canonical Super updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("canonical updates = %+v, want one", updates)
	}
	if updates[0].UpdateID != dispatched[0].UpdateID || updates[0].Packet.Kind != "execution_request" ||
		updates[0].AgentID != textureRun.AgentID || updates[0].TargetAgentID != superAgentID {
		t.Fatalf("canonical update = %+v", updates[0])
	}
	if len(updates[0].Packet.Actions) != 1 || updates[0].Packet.Actions[0].Objective != "Run the focused delivery verification." {
		t.Fatalf("canonical packet lost typed action: %+v", updates[0].Packet)
	}

	now := time.Now().UTC()
	consumedRun := types.RunRecord{
		RunID: "run-super-consumed", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: superAgentID, State: types.RunCompleted, CreatedAt: now, UpdatedAt: now, FinishedAt: &now,
		Metadata: map[string]any{
			runMetadataAgentProfile:          agentprofile.Super,
			runMetadataAgentRole:             agentprofile.Super,
			runMetadataTrajectoryID:          trajectoryID,
			"request_source":                 "update_coagent",
			"worker_update_ids":              []string{updates[0].UpdateID},
			runMetadataWorkerUpdatesInjected: true,
		},
	}
	if err := s.CreateRun(ctx, consumedRun); err != nil {
		t.Fatalf("record completed canonical delivery run: %v", err)
	}
	updates, err = rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), superAgentID, trajectoryID, 10)
	if err != nil || len(updates) != 1 {
		t.Fatalf("run completion without tape acknowledgement consumed canonical delivery: updates=%+v err=%v", updates, err)
	}
	consumedRun.Metadata["supervision_delivery_authority"] = "canonical"
	consumedRun.Metadata["supervision_delivery_ids"] = []string{updates[0].UpdateID}
	if err := rt.acknowledgeCanonicalSupervisionDeliveries(ctx, &consumedRun); err != nil {
		t.Fatalf("append canonical delivery acknowledgement: %v", err)
	}
	updates, err = rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), superAgentID, trajectoryID, 10)
	if err != nil {
		t.Fatalf("reload consumed canonical Super updates: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("completed canonical update redelivered: %+v", updates)
	}
}

func TestCanonicalSupervisionDeliveryRecoversAfterDispatchFailure(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-super-delivery-restart"
	docID := "doc-super-delivery-restart"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	secondDocID := "doc-super-delivery-restart-second"
	secondTrajectoryID := seedDurableTextureSubject(t, s, ownerID, secondDocID)
	installTestSupervisionAppender(t, rt, s)
	textureRun := &types.RunRecord{
		RunID: "run-super-delivery-restart", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: currentTextureAgentID(docID), ChannelID: docID, State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			runMetadataTrajectoryID: trajectoryID,
			runMetadataChannelID:    docID,
		},
	}
	secondTextureRun := &types.RunRecord{
		RunID: "run-super-delivery-restart-second", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: currentTextureAgentID(secondDocID), ChannelID: secondDocID, State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			runMetadataTrajectoryID: secondTrajectoryID,
			runMetadataChannelID:    secondDocID,
		},
	}
	var retrySweep func()
	rt.textureWakeAfter = func(_ time.Duration, fn func()) textureWakeTimer {
		retrySweep = fn
		return supervisionSweepTestTimer{}
	}
	rt.dispatchActor = func(context.Context, string, string, string, string, string, string, string) error {
		return errors.New("simulated broker outage")
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "execution_request",
		Summary:       "Recover this request from the canonical tape.",
		Actions: []types.CoagentPacketAction{{
			Type: "run_command", Objective: "Run the recovered request.",
			Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "forbidden"},
		}},
	}
	if _, err := rt.appendSupervisedUpdate(ctx, textureRun, packet, "command-super-delivery-restart", persistentSuperAgentID(ownerID), docID); err == nil ||
		!strings.Contains(err.Error(), "simulated broker outage") {
		t.Fatalf("append dispatch error = %v", err)
	}
	if _, err := rt.appendSupervisedUpdate(ctx, secondTextureRun, packet, "command-super-delivery-restart-second", persistentSuperAgentID(ownerID), secondDocID); err == nil ||
		!strings.Contains(err.Error(), "simulated broker outage") {
		t.Fatalf("append second dispatch error = %v", err)
	}

	appender, cipher := rt.eventAppender, rt.privateArtifactCipher
	rt.eventAppender, rt.privateArtifactCipher = nil, nil
	if _, err := rt.reconcilePersistentSuperActor(ctx, ownerID, persistentSuperAgentID(ownerID)); !errors.Is(err, ErrSupervisionAuthorityRequired) {
		t.Fatalf("cold canonical reconcile without artifact authority = %v", err)
	}
	if retrySweep == nil {
		t.Fatal("post-commit dispatch failure did not schedule a canonical retry")
	}
	rt.eventAppender, rt.privateArtifactCipher = appender, cipher

	var recovered []types.CoagentSourcePacket
	rt.dispatchActor = func(_ context.Context, gotOwnerID, gotComputerID, gotAgentID, kind, content, gotTrajectoryID, fromAgentID string) error {
		recovered = append(recovered, types.CoagentSourcePacket{
			UpdateID: content, OwnerID: gotOwnerID, ComputerID: gotComputerID,
			TargetAgentID: gotAgentID, AgentID: fromAgentID, TrajectoryID: gotTrajectoryID, Content: kind,
		})
		return nil
	}
	retrySweep()
	if len(recovered) != 2 {
		t.Fatalf("recovered dispatches = %+v, want one per trajectory", recovered)
	}
	recoveredTrajectories := map[string]bool{}
	for _, got := range recovered {
		if got.TargetAgentID != persistentSuperAgentID(ownerID) || got.Content != "coagent_result" {
			t.Fatalf("recovered dispatch = %+v", got)
		}
		recoveredTrajectories[got.TrajectoryID] = true
	}
	if !recoveredTrajectories[trajectoryID] || !recoveredTrajectories[secondTrajectoryID] {
		t.Fatalf("recovered trajectory set = %+v", recoveredTrajectories)
	}
}

func TestSupervisedReturnsRejectCallerSelectedRecipients(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-fixed-supervision-recipients"
	docID := "doc-fixed-supervision-recipients"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "execution_result",
		Summary:       "A canonical supervised result.",
	}

	for _, tc := range []struct {
		name        string
		profile     string
		agentID     string
		targetID    string
		wantMessage string
	}{
		{
			name: "CoSuper must return to persistent Super", profile: agentprofile.CoSuper,
			agentID: "cosuper:fixed-recipient", targetID: currentTextureAgentID(docID),
			wantMessage: "must target persistent Super",
		},
		{
			name: "Researcher must return to trajectory Texture", profile: agentprofile.Researcher,
			agentID: "researcher:fixed-recipient", targetID: persistentSuperAgentID(ownerID),
			wantMessage: "must target trajectory Texture",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := &types.RunRecord{
				RunID: "run-" + tc.agentID, OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
				AgentID: tc.agentID, State: types.RunRunning,
				Metadata: map[string]any{
					runMetadataAgentProfile: tc.profile,
					runMetadataAgentRole:    tc.profile,
					runMetadataTrajectoryID: trajectoryID,
				},
			}
			_, err := rt.appendSupervisedUpdate(ctx, run, packet, "command-"+tc.agentID, tc.targetID, docID)
			if err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("append wrong-recipient %s result = %v", tc.profile, err)
			}
		})
	}
}

func TestCanonicalSupervisionConsumptionIgnoresCompletedRunHistoryWithoutTapeAcknowledgement(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-unbounded-canonical-consumption"
	computerID := rt.TextureSandboxID()
	targetAgentID := persistentSuperAgentID(ownerID)
	now := time.Now().UTC()
	for i := range 101 {
		updateID := fmt.Sprintf("canonical-update-%03d", i)
		if err := s.CreateRun(ctx, types.RunRecord{
			RunID: fmt.Sprintf("run-consumed-%03d", i), OwnerID: ownerID, SandboxID: computerID,
			AgentID: targetAgentID, State: types.RunCompleted,
			CreatedAt: now.Add(time.Duration(i) * time.Millisecond), UpdatedAt: now, FinishedAt: &now,
			Metadata: map[string]any{
				runMetadataAgentProfile:          agentprofile.Super,
				"worker_update_ids":              []string{updateID},
				runMetadataWorkerUpdatesInjected: true,
			},
		}); err != nil {
			t.Fatalf("create completed delivery run %d: %v", i, err)
		}
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, ownerID, computerID, targetAgentID)
	if err != nil {
		t.Fatalf("load consumed canonical IDs: %v", err)
	}
	if len(consumed) != 0 {
		t.Fatalf("derived run history consumed canonical IDs without tape acknowledgement: %+v", consumed)
	}
}

func TestColdSuperRunClaimsCanonicalUpdateOnlyAfterPacketPrepend(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-cold-super-claim"
	docID := "doc-cold-super-claim"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	rt.dispatchActor = func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "execution_request",
		Summary:       "Deliver this request before claiming it.",
		Actions: []types.CoagentPacketAction{{
			Type: "run_command", Objective: "Inspect the canonical request.",
			Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "forbidden"},
		}},
	}
	textureRun := &types.RunRecord{
		RunID: "run-cold-super-source", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: currentTextureAgentID(docID), State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	updateID, err := rt.appendSupervisedUpdate(ctx, textureRun, packet, "command-cold-super-claim", persistentSuperAgentID(ownerID), docID)
	if err != nil {
		t.Fatalf("append canonical request: %v", err)
	}
	superRun := &types.RunRecord{
		RunID: "run-cold-super-consumer", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: persistentSuperAgentID(ownerID), State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataTrajectoryID: trajectoryID,
			"request_source":        "update_coagent",
		},
	}
	superRun.Metadata["supervision_delivery_authority"] = "canonical"
	superRun.Metadata["supervision_delivery_ids"] = []string{updateID}
	if err := s.CreateRun(ctx, *superRun); err != nil {
		t.Fatalf("create cold Super run: %v", err)
	}
	if ids := coagentUpdateIDsForRun(superRun); len(ids) != 0 {
		t.Fatalf("cold Super run pre-claimed update IDs: %v", ids)
	}
	messages, err := rt.prependInitialCoagentUpdatePackets(ctx, superRun, nil)
	if err != nil {
		t.Fatalf("prepend canonical update: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("prepended messages = %d, want 1", len(messages))
	}
	if ids := coagentUpdateIDsForRun(superRun); len(ids) != 1 || ids[0] != updateID {
		t.Fatalf("claimed IDs after prepend = %v, want %q", ids, updateID)
	}
	superRun.State = types.RunCompleted
	superRun.UpdatedAt = time.Now().UTC()
	superRun.FinishedAt = &superRun.UpdatedAt
	if err := rt.acknowledgeCanonicalSupervisionDeliveries(ctx, superRun); err != nil {
		t.Fatalf("acknowledge canonical delivery: %v", err)
	}
	if err := rt.markPersistentSuperRunUpdatesDelivered(ctx, superRun); err != nil {
		t.Fatalf("complete canonical delivery run: %v", err)
	}
	pending, err := rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), superRun.AgentID, trajectoryID, 0)
	if err != nil {
		t.Fatalf("reload canonical request: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("completed canonical request redelivered: %+v", pending)
	}
}

func TestSupervisedPendingDeliveryFailsClosedWithoutArtifactAuthority(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-supervised-delivery-authority"
	docID := "doc-supervised-delivery-authority"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	rec := &types.RunRecord{
		RunID: "run-supervised-delivery-authority", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: persistentSuperAgentID(ownerID), State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if _, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, rec.AgentID, 100); !errors.Is(err, ErrSupervisionAuthorityRequired) {
		t.Fatalf("pending supervised delivery without artifact authority = %v", err)
	}
}

func TestCompatibilityDeliveryFiltersCompletedRunConsumption(t *testing.T) {
	updates := []types.CoagentSourcePacket{
		{UpdateID: "already-consumed"},
		{UpdateID: "still-pending"},
	}
	pending := filterConsumedCoagentUpdates(updates, map[string]bool{"already-consumed": true})
	if len(pending) != 1 || pending[0].UpdateID != "still-pending" {
		t.Fatalf("filtered compatibility updates = %+v", pending)
	}
}

func TestPersistentSuperExecutionFilterScansPastNonExecutionPrefix(t *testing.T) {
	updates := make([]types.CoagentSourcePacket, 101)
	for i := range 100 {
		updates[i] = types.CoagentSourcePacket{
			UpdateID: fmt.Sprintf("result-%03d", i),
			Packet: types.CoagentSourcePacketPayload{
				SchemaVersion: "coagent_source_packet.v1",
				Kind:          "execution_result",
				Summary:       "A non-executable result.",
			},
		}
	}
	updates[100] = types.CoagentSourcePacket{
		UpdateID: "request-101",
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: "coagent_source_packet.v1",
			Kind:          "execution_request",
			Summary:       "Executable request after results.",
			Actions: []types.CoagentPacketAction{{
				Type: "run_command", Objective: "Run after the non-execution prefix.",
				Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "forbidden"},
			}},
		},
	}
	executable := filterPersistentSuperRunnableUpdates(updates)
	if len(executable) != 1 || executable[0].UpdateID != "request-101" {
		t.Fatalf("executable canonical backlog = %+v", executable)
	}
}

func TestPersistentSuperRunnableFilterIncludesCanonicalResult(t *testing.T) {
	update := types.CoagentSourcePacket{
		UpdateID:         "canonical-result",
		LifecycleVersion: 1,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: "coagent_source_packet.v1",
			Kind:          "execution_result",
			Summary:       "Typed CoSuper result for reconciliation.",
			Notes:         []string{"Result evidence is available on the canonical tape."},
		},
	}
	runnable := filterPersistentSuperRunnableUpdates([]types.CoagentSourcePacket{update})
	if len(runnable) != 1 || runnable[0].UpdateID != update.UpdateID {
		t.Fatalf("canonical result backlog = %+v", runnable)
	}
}

func TestCanonicalResearcherRetryRecoversImplicitLegacyRecipient(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-researcher-implicit-retry"
	docID := "doc-researcher-implicit-retry"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	rt.dispatchActor = func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	}
	rec := &types.RunRecord{
		RunID: "run-researcher-implicit-retry", OwnerID: ownerID,
		SandboxID: rt.TextureSandboxID(), AgentID: "researcher:implicit-retry",
		TrajectoryID: trajectoryID,
		Metadata:     map[string]any{runMetadataAgentProfile: agentprofile.Researcher, runMetadataTrajectoryID: trajectoryID},
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1", Kind: "evidence_update",
		Summary: "Recovered sourced evidence.", Notes: []string{"The accepted recipient is on the tape."},
	}
	commandID := "c2c82155-8f87-4c87-a7a9-a1bc34c0a4db"
	targetAgentID := currentTextureAgentID(docID)
	recordID, err := rt.appendSupervisedUpdate(ctx, rec, packet, commandID, targetAgentID, docID)
	if err != nil {
		t.Fatalf("append researcher delivery: %v", err)
	}
	recovered, found, err := rt.recoverSupervisedUpdate(ctx, rec, packet, commandID, "", "")
	if err != nil || !found {
		t.Fatalf("recover implicit recipient: found=%v err=%v", found, err)
	}
	if recovered.UpdateID != recordID || recovered.TargetAgentID != targetAgentID || recovered.ChannelID != docID {
		t.Fatalf("recovered researcher delivery = %+v", recovered)
	}
}

func TestCanonicalDeliveryReadsLegacyCommandBoundPrivateArtifact(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-legacy-private-binding"
	docID := "doc-legacy-private-binding"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	snapshot, err := s.GetSupervisionProjectionSnapshot(ctx, ownerID, rt.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("load supervision snapshot: %v", err)
	}
	observedBase, err := supervisionObservedBase(snapshot)
	if err != nil {
		t.Fatalf("resolve supervision base: %v", err)
	}
	commandID := "legacy-command-private-binding"
	messageID := "7b4c9b29-c078-4e79-b267-2e4df123947a"
	bindingID := commandID + ":packet"
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "execution_request",
		Summary:       "Decrypt the legacy command-bound packet.",
		Actions: []types.CoagentPacketAction{{
			Type: "run_command", Objective: "Inspect legacy delivery compatibility.",
			Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "forbidden"},
		}},
	}
	packetBytes, err := computerevent.CanonicalJSON(packet)
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"message_id": messageID, "from_actor_id": currentTextureAgentID(docID),
		"to_role": agentprofile.Super, "to_actor_id": persistentSuperAgentID(ownerID),
		"channel_id": docID, "payload_artifact_ref": computerevent.SupervisionArtifactPlaceholder(bindingID),
		"material": false,
	})
	if err != nil {
		t.Fatalf("marshal legacy message body: %v", err)
	}
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe:  computerevent.SupervisionDigestRecipeV1,
		TransactionID: commandID, OwnerID: ownerID, ComputerID: rt.TextureSandboxID(),
		TrajectoryID: trajectoryID, CommandID: commandID,
		Expected: supervisionExpected(snapshot), ObservedBase: &observedBase,
		TransactionClass: "record_message",
		Actor: computerevent.SupervisionActor{
			ActorID: currentTextureAgentID(docID), Role: agentprofile.Texture,
			AuthorityRef: "run:legacy-private-binding",
		},
		Mutations: []computerevent.SupervisionMutation{{Kind: "actor_message_recorded", Body: body}},
	}
	if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{
		BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1,
	}}); err != nil {
		t.Fatalf("append legacy command-bound delivery: %v", err)
	}
	updates, err := rt.canonicalSupervisionUpdatesForAgent(ctx, ownerID, rt.TextureSandboxID(), persistentSuperAgentID(ownerID), trajectoryID, 0)
	if err != nil {
		t.Fatalf("read legacy command-bound delivery: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != messageID || updates[0].Packet.Summary != packet.Summary {
		t.Fatalf("legacy command-bound update = %+v", updates)
	}
	rt.dispatchActor = func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	}
	rec := &types.RunRecord{
		RunID: "legacy-private-binding", OwnerID: ownerID,
		SandboxID: rt.TextureSandboxID(), AgentID: currentTextureAgentID(docID),
		TrajectoryID: trajectoryID,
		Metadata:     map[string]any{runMetadataAgentProfile: agentprofile.Texture, runMetadataTrajectoryID: trajectoryID},
	}
	recoveredID, err := rt.appendSupervisedUpdate(ctx, rec, packet, commandID, persistentSuperAgentID(ownerID), docID)
	if err != nil {
		t.Fatalf("retry legacy command-bound delivery: %v", err)
	}
	if recoveredID != messageID {
		t.Fatalf("recovered legacy delivery ID = %q, want %q", recoveredID, messageID)
	}
	otherRun := *rec
	otherRun.RunID = "legacy-private-binding-other-run"
	if _, err := rt.appendSupervisedUpdate(ctx, &otherRun, packet, commandID, persistentSuperAgentID(ownerID), docID); !errors.Is(err, computerevent.ErrSupervisionIdempotencyConflict) {
		t.Fatalf("cross-run legacy retry error = %v, want idempotency conflict", err)
	}
	if _, err := rt.appendSupervisedUpdate(ctx, rec, packet, commandID, persistentSuperAgentID(ownerID), "other-channel"); !errors.Is(err, computerevent.ErrSupervisionIdempotencyConflict) {
		t.Fatalf("changed-channel legacy retry error = %v, want idempotency conflict", err)
	}
}
func TestCanonicalRoleAddressResolvesStoredGenericProfile(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-role-addressed-generic"
	docID := "doc-role-addressed-generic"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	targetAgentID := "researcher:role-addressed-generic"
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: targetAgentID, OwnerID: ownerID,
		ComputerID: rt.TextureSandboxID(), SandboxID: rt.TextureSandboxID(),
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("persist role-addressed agent: %v", err)
	}
	trajectory, err := s.GetLifecycleTrajectory(ctx, ownerID, rt.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("load trajectory: %v", err)
	}
	addressed, err := rt.canonicalSupervisionDeliveryBodyAddressed(
		ctx, trajectory, targetAgentID, "actor_message_recorded",
		canonicalSupervisionDeliveryBody{ToRole: agentprofile.Researcher},
	)
	if err != nil || !addressed {
		t.Fatalf("role-addressed generic actor = %v, err=%v", addressed, err)
	}
}

func TestCanonicalMessageColdStartsGenericSupervisedActor(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-generic-canonical-wake"
	docID := "doc-generic-canonical-wake"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	targetAgentID := "researcher:generic-canonical-wake"
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: targetAgentID, OwnerID: ownerID,
		ComputerID: rt.TextureSandboxID(), SandboxID: rt.TextureSandboxID(),
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher,
		ChannelID: docID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("persist canonical target agent: %v", err)
	}
	rt.dispatchActor = func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	}
	source := &types.RunRecord{
		RunID: "run-generic-canonical-source", OwnerID: ownerID, SandboxID: rt.TextureSandboxID(),
		AgentID: currentTextureAgentID(docID), State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1",
		Kind:          "evidence_update",
		Summary:       "A canonical message for a cold Researcher.",
		Claims:        []types.CoagentPacketClaim{{Text: "The packet must start the addressed actor."}},
	}
	if _, err := rt.appendSupervisedUpdate(ctx, source, packet, "command-generic-canonical-wake", targetAgentID, docID); err != nil {
		t.Fatalf("append canonical generic message: %v", err)
	}
	rec, err := rt.reconcileUpdatedCoagentActor(ctx, ownerID, targetAgentID)
	if err != nil {
		t.Fatalf("reconcile canonical target: %v", err)
	}
	if rec == nil || rec.AgentID != targetAgentID || trajectoryIDForRun(rec) != trajectoryID {
		t.Fatalf("canonical target run = %+v", rec)
	}
	if ids := coagentUpdateIDsForRun(rec); len(ids) != 0 {
		t.Fatalf("canonical target run pre-claimed packet IDs: %v", ids)
	}
	if strings.Contains(rec.Prompt, packet.Summary) || strings.Contains(rec.Prompt, packet.Claims[0].Text) {
		t.Fatalf("canonical private packet leaked into durable prompt: %q", rec.Prompt)
	}
	if metadataStringValue(rec.Metadata, "supervision_delivery_authority") != "canonical" {
		t.Fatalf("canonical activation authority = %#v", rec.Metadata)
	}
	inject := rt.coagentUpdateTurnInjectorWithInitialPhase(rec, "")
	if inject == nil {
		t.Fatal("canonical generic activation has no private turn injector")
	}
	turns, err := inject(false)
	if err != nil {
		t.Fatalf("inject canonical private turn: %v", err)
	}
	if len(turns) != 1 || !strings.Contains(string(turns[0]), packet.Summary) || !strings.Contains(string(turns[0]), packet.Claims[0].Text) {
		t.Fatalf("canonical private turn = %s", turns)
	}
	storedRun, err := s.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatalf("reload canonical generic run: %v", err)
	}
	if !metadataBoolValue(storedRun.Metadata, runMetadataPrivateTraceTainted) {
		t.Fatalf("canonical private delivery did not taint run metadata: %#v", storedRun.Metadata)
	}
	rt.eventAppender, rt.privateArtifactCipher = nil, nil
	if _, err := inject(false); !errors.Is(err, ErrSupervisionAuthorityRequired) {
		t.Fatalf("warm canonical injection without tape authority = %v", err)
	}
}

func TestCanonicalRoleAddressedGenericActorRecoversAfterRestart(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-role-addressed-restart"
	docID := "doc-role-addressed-restart"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	installTestSupervisionAppender(t, rt, s)
	targetAgentID := "researcher:role-addressed-restart"
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: targetAgentID, OwnerID: ownerID,
		ComputerID: rt.TextureSandboxID(), SandboxID: rt.TextureSandboxID(),
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("persist generic Researcher: %v", err)
	}
	snapshot, err := s.GetSupervisionProjectionSnapshot(ctx, ownerID, rt.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("load supervision snapshot: %v", err)
	}
	observedBase, err := supervisionObservedBase(snapshot)
	if err != nil {
		t.Fatalf("resolve supervision base: %v", err)
	}
	commandID := "role-addressed-restart-command"
	messageID := "role-addressed-restart-message"
	bindingID := messageID + ":packet"
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: "coagent_source_packet.v1", Kind: "execution_result",
		Summary: "Recover a role-addressed Researcher packet.",
		Sources: []types.CoagentPacketSource{{
			SourceID: "role-restart-source", Kind: "source_service_item",
			Target: types.CoagentPacketSourceTarget{URI: "source_service_item:role-restart"},
		}},
	}
	packetBytes, err := computerevent.CanonicalJSON(packet)
	if err != nil {
		t.Fatalf("marshal role-addressed packet: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"message_id": messageID, "from_actor_id": currentTextureAgentID(docID),
		"to_role": agentprofile.Researcher, "to_actor_id": nil,
		"channel_id": docID, "payload_artifact_ref": computerevent.SupervisionArtifactPlaceholder(bindingID),
		"material": false,
	})
	if err != nil {
		t.Fatalf("marshal role-addressed body: %v", err)
	}
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe:  computerevent.SupervisionDigestRecipeV1,
		TransactionID: commandID, OwnerID: ownerID, ComputerID: rt.TextureSandboxID(),
		TrajectoryID: trajectoryID, CommandID: commandID,
		Expected: supervisionExpected(snapshot), ObservedBase: &observedBase,
		TransactionClass: "record_message",
		Actor: computerevent.SupervisionActor{
			ActorID: currentTextureAgentID(docID), Role: agentprofile.Texture,
			AuthorityRef: "run:role-addressed-restart",
		},
		Mutations: []computerevent.SupervisionMutation{{Kind: "actor_message_recorded", Body: body}},
	}
	if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{
		BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1,
	}}); err != nil {
		t.Fatalf("append role-addressed delivery: %v", err)
	}
	var dispatchedAgentID string
	rt.dispatchActor = func(_ context.Context, _, _, agentID, _, _, _, _ string) error {
		dispatchedAgentID = agentID
		return nil
	}
	if err := rt.sweepPendingCanonicalSupervisionActors(ctx); err != nil {
		t.Fatalf("sweep role-addressed delivery: %v", err)
	}
	if dispatchedAgentID != targetAgentID {
		t.Fatalf("role-addressed restart dispatched to %q, want %q", dispatchedAgentID, targetAgentID)
	}
}
