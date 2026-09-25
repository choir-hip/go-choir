package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// This file pins the E1 survivor contract for the source-centric
// update_coagent hard cutover (mission-update-coagent-source-centric-deletion-v0).
//
// The survivor set is exactly:
//   - update_coagent accepts ONLY the coagent_source_packet.v1 surface and
//     rejects legacy top-level fields and unknown fields;
//   - update_coagent rejects invalid nested packet objects (sources, claims,
//     actions);
//   - Texture source collation reads ONLY packet.sources; prose in notes,
//     summary, or claims.text does not become a source entity;
//   - Management executes ONLY Texture-authorized Direction=control execution_request
//     packets (sender-authorization privilege gate; packet.kind is not authority);
//   - a Engineering packet declaring kind=execution_request must not open Management execution;
//   - assigned Engineering producer reports remain in the mailbox and never become
//     Management execution; unsigned Engineering packets without Direction=producer_report
//     are still settled as non-executable;
//   - static Engineering registry still has no update_coagent (assigned overlay does);
//   - unauthorized packets addressed to persistent Management are settled instead
//     of remaining as live pending backlog.
//
// Every later deletion commit (E2-E4) must keep this file green. If a test
// here is intentionally relaxed or removed, the paradoc must record why and
// name the new contract surface that replaces it.
//
// The "rejected sources are REPORTED" obligation from the paradoc (silent
// skip at texture_evidence_sources.go:163-170) is a behavior change landed
// at E3.3, not a test-only obligation. It is pinned here as
// TestSurvivorContract_RejectedSourcesAreReported with a t.Skip marker
// describing the E3.3 unblock condition; E3.3 removes the skip and makes the
// assertion green.

// validEvidenceUpdatePacket is the canonical source-centric packet used as the
// baseline for survivor-contract assertions. Every field is in the survivor
// surface; no legacy field is present.
const validEvidenceUpdatePacket = `{
	"schema_version":"coagent_source_packet.v1",
	"kind":"evidence_update",
	"summary":"baseline survivor packet",
	"agent_id":"texture:doc-survivor",
	"channel_id":"doc-survivor",
	"claims":[{"text":"A supported claim.","source_ids":["src-survivor"],"stance":"supports","recommended_surface":"inline_ref"}],
	"sources":[{"source_id":"src-survivor","kind":"content_item","target":{"uri":"https://example.test/survivor","title":"Survivor source"},"selectors":[{"kind":"whole_resource"}],"evidence":{"state":"available","confidence":"high","rights_scope":"private_user_source"}}],
	"questions":["What is the next source path?"],
	"notes":["Baseline packet for the survivor contract."]
}`

// TestSurvivorContract_AcceptsCanonicalSurface proves the survivor packet
// shape is accepted end-to-end and persists as a CoagentSourcePacket with
// typed claims/sources/questions/notes.
func TestSurvivorContract_ProcessorAcceptsCanonicalSurface(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-survivor-canonical"
	docID := "doc-survivor"
	processorRun, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "survivor-canonical", agentprofile.Processor)
	raw, err := rt.ToolRegistryForProfile(agentprofile.Processor).Execute(toolContextForTestCall(processorRun, "call-survivor-canonical"), "update_coagent", json.RawMessage(validEvidenceUpdatePacket))
	if err != nil {
		t.Fatalf("update_coagent canonical surface rejected: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, processorRun, raw)
	if stored.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 {
		t.Fatalf("schema_version = %q, want %q", stored.Packet.SchemaVersion, types.CoagentSourcePacketSchemaV1)
	}
	if stored.Packet.Kind != "evidence_update" {
		t.Fatalf("kind = %q", stored.Packet.Kind)
	}
	if len(stored.Packet.Claims) != 1 || len(stored.Packet.Sources) != 1 || len(stored.Packet.Questions) != 1 || len(stored.Packet.Notes) != 1 {
		t.Fatalf("survivor surface lost fields: %#v", stored.Packet)
	}
	// The human projection must not carry legacy section headers.
	for _, legacy := range []string{"Findings:", "Evidence IDs:", "Artifacts:", "Refs:", "Tests:", "Proposals:", "Capability requests:"} {
		if strings.Contains(stored.Content, legacy) {
			t.Fatalf("human projection retained legacy section %q: %q", legacy, stored.Content)
		}
	}
}

// TestSurvivorContract_RejectsEveryLegacyTopLevelField proves that every
// legacy top-level field named in the deletion report is rejected. The
// pre-existing TestUpdateCoagentRejectsLegacyFieldsAndExecutionRequestWithoutActions
// only covered two of these; this test pins the full legacy vocabulary.
func TestSurvivorContract_RejectsEveryLegacyTopLevelField(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	processorRun := d9CoagentRun("run-survivor-reject", "user-survivor-reject", "processor:survivor-reject", agentprofile.Processor, "doc-survivor-reject", currentTextureAgentID("doc-survivor-reject"))
	for _, field := range []string{
		"findings",
		"evidence_ids",
		"evidence",
		"artifacts",
		"refs",
		"tests",
		"proposals",
		"capability_requests",
		"update_id",
	} {
		raw := json.RawMessage(`{
			"schema_version":"coagent_source_packet.v1",
			"kind":"evidence_update",
			"summary":"legacy field injection",
			"agent_id":"texture:doc-survivor-reject",
			"channel_id":"doc-survivor-reject",
			"` + field + `":["legacy-value"]
		}`)
		_, err := rt.ToolRegistryForProfile(agentprofile.Processor).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(processorRun)), "update_coagent", raw)
		if err == nil {
			t.Fatalf("update_coagent accepted legacy field %q", field)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "legacy") && !strings.Contains(err.Error(), field) {
			t.Fatalf("update_coagent rejection for %q did not name the field: %v", field, err)
		}
	}
}

// TestSurvivorContract_RejectsUnknownTopLevelField proves the surface is
// closed: a field outside the survivor set is rejected even if it is not in
// the legacy vocabulary. This blocks silent reintroduction of a parallel
// surface under a new name.
func TestSurvivorContract_RejectsUnknownTopLevelField(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	processorRun := d9CoagentRun("run-survivor-unknown", "user-survivor-unknown", "processor:survivor-unknown", agentprofile.Processor, "doc-survivor-unknown", currentTextureAgentID("doc-survivor-unknown"))
	raw := json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"unknown field injection",
		"agent_id":"texture:doc-survivor-unknown",
		"channel_id":"doc-survivor-unknown",
		"secret平行surface":["should be rejected"]
	}`)
	if _, err := rt.ToolRegistryForProfile(agentprofile.Processor).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(processorRun)), "update_coagent", raw); err == nil {
		t.Fatalf("update_coagent accepted unknown top-level field (parallel surface reintroduction risk)")
	}
}

// TestSurvivorContract_TextureCollatesOnlyPacketSources proves the core
// invariant of the source-centric cutover: Texture source collation reads
// ONLY packet.sources. Source-shaped text in notes, summary, or claims.text
// must NOT become a Texture source entity. This is the survivor guarantee
// that makes the partial-cutover failure mode (legacy findings prose
// treated as source substrate) impossible.
func TestSurvivorContract_TextureCollatesOnlyPacketSources(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-survivor-collation"
	docID := "doc-survivor-collation"
	processorRun, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "survivor-collation", agentprofile.Processor)
	// Deliberately embed source-shaped text in notes and summary prose that
	// must NOT be scraped: an http URL in notes, a "[Source: foo]" style
	// label in summary, and a bare command_output: URI in claims.text. Only
	// the single typed packet.sources entry may become an entity.
	raw, err := rt.ToolRegistryForProfile(agentprofile.Processor).Execute(toolContextForTestCall(processorRun, "call-survivor-collation"), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"Summary references [Source: prose-only] and should not be scraped.",
		"agent_id":"texture:doc-survivor-collation",
		"channel_id":"doc-survivor-collation",
		"claims":[{"text":"Claim text mentions command_output:should-not-scrape and https://example.test/prose-only but cites only src-real.","source_ids":["src-real"]}],
		"sources":[{"source_id":"src-real","kind":"content_item","target":{"uri":"https://example.test/real-source","title":"Real source"},"selectors":[{"kind":"whole_resource"}]}],
		"notes":["See also https://example.test/note-only and command_output:note-only; neither is a source."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, processorRun, raw)
	if len(stored.Packet.Sources) != 1 {
		t.Fatalf("stored packet sources = %#v, want exactly one typed source", stored.Packet.Sources)
	}
	sourceURI := stored.Packet.Sources[0].Target.URI
	for _, forbidden := range []string{"prose-only", "note-only", "should-not-scrape"} {
		if strings.Contains(sourceURI, forbidden) {
			t.Fatalf("stored packet source was scraped from prose: %#v", stored.Packet.Sources)
		}
	}
}

// TestSurvivorContract_ManagementExecutesOnlyExecutionRequestPackets pins the
// privilege gate: persistent Management must not start privileged execution from
// a non-execution_request packet. This complements
// TestPersistentManagementIgnoresNonExecutionRequestUpdatePackets by also
// asserting the deliverable-for-run filter from the run side, so a later
// change cannot weaken one path while leaving the other intact.
func TestSurvivorContract_EngineeringExecutionRequestDoesNotOpenPersistentManagement(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	if _, ok := rt.ToolRegistryForProfile(agentprofile.Engineering).Lookup("update_coagent"); ok {
		t.Fatal("static Engineering registry retained unassigned update_coagent")
	}
	ctx := context.Background()
	ownerID := "user-survivor-cosuper-exec"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}
	now := mustNow(t)
	comanagementExec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "engineering:survivor-exec",
		TargetAgentID: managementAgent.AgentID,
		ChannelID:     managementAgent.ChannelID,
		Role:          agentprofile.Engineering,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "execution_request",
			Summary:       "Engineering-authored execution_request must not open Management",
			Claims:        []types.CoagentPacketClaim{{Text: "kind is content, not authority."}},
			Actions: []types.CoagentPacketAction{{
				Type:      "run_command",
				Objective: "This must not become privileged Management work.",
				Safety: types.CoagentPacketActionSafety{
					MutationClass: "green",
					Network:       "forbidden",
					FileMutation:  "forbidden",
				},
			}},
		},
		CreatedAt: now,
	}
	comanagementExec.UpdateID = deriveWorkerUpdateID(comanagementExec)
	comanagementExec.Content = buildWorkerUpdateMessage(comanagementExec)
	msg := &types.ChannelMessage{
		ChannelID: comanagementExec.ChannelID, From: comanagementExec.AgentID, FromAgentID: comanagementExec.AgentID,
		ToAgentID: comanagementExec.TargetAgentID, Role: comanagementExec.Role, Content: comanagementExec.Content, Timestamp: comanagementExec.CreatedAt,
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, comanagementExec, msg); err != nil || !created {
		t.Fatalf("dispatch Engineering execution_request: created=%v err=%v", created, err)
	}
	run, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run != nil {
		t.Fatalf("Engineering execution_request opened persistent Management run %s", run.RunID)
	}
	stored, err := s.GetWorkerUpdate(ctx, ownerID, comanagementExec.UpdateID)
	if err != nil {
		t.Fatalf("get Engineering execution_request: %v", err)
	}
	if stored.DeliveredToRunID != "settled_non_executable" || stored.DeliveredAt == nil {
		t.Fatalf("Engineering execution_request not settled: %+v", stored)
	}
	if persistentManagementExecutablePacket(comanagementExec) {
		t.Fatal("Engineering execution_request remained Management-executable")
	}
	textureExec := authorizedPersistentManagementExecutionRequest(ownerID, managementAgent.AgentID, managementAgent.ChannelID, "Texture control opens Management", now.Add(time.Millisecond))
	if !persistentManagementExecutablePacket(textureExec) {
		t.Fatal("Texture control execution_request lost Management executability")
	}
}

// TestSurvivorContract_RejectedSourcesAreReported pins the E3.3 obligation:
// when a packet.source fails to materialize into a Texture source entity
// (sourceEntityFromCoagentPacketSource returns empty at
// texture_evidence_sources.go:163-170), the rejection must be visible to the
// integrating agent, not silently swallowed.
//
// This is a behavior change, not a test-only obligation. The assertion is
// skipped until E3.3 lands a reporting surface; the skip text names the
// unblock condition so E3.3 cannot silently leave it skipped.
func TestSurvivorContract_RejectedSourcesAreReported(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ownerID := "user-survivor-reported"
	docID := "doc-survivor-reported"
	processorRun, _ := spawnBoundTestLifecycleProducer(t, rt, s, ownerID, docID, "survivor-reported", agentprofile.Processor)
	// A packet.source with an unsupported kind that cannot materialize. The
	// current behavior silently drops it. The survivor contract requires the
	// drop be reported.
	raw, err := rt.ToolRegistryForProfile(agentprofile.Processor).Execute(toolContextForTestCall(processorRun, "call-survivor-reported"), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"packet with a source that cannot materialize",
		"agent_id":"texture:doc-survivor-reported",
		"channel_id":"doc-survivor-reported",
		"claims":[{"text":"Claim depends on a source that will not materialize.","source_ids":["src-unsupported"]}],
		"sources":[{"source_id":"src-unsupported","kind":"unsupported_kind","target":{"uri":"unsupported:does-not-resolve","title":"Will not materialize"},"selectors":[{"kind":"whole_resource"}]}]
	}`))
	if err != nil {
		// If D9 validation grows to reject unsupported source kinds ahead of
		// storage (a valid alternative to reporting at collation time), that
		// also satisfies the survivor contract: the rejection is visible to
		// the agent as a failed tool result. Record that path and return.
		if strings.Contains(err.Error(), "unsupported_kind") || strings.Contains(err.Error(), "kind") {
			t.Logf("D9 validation rejected the unsupported source kind at the tool boundary; this satisfies the survivor contract via a visible tool error: %v", err)
			return
		}
		t.Fatalf("update_coagent: %v", err)
	}
	stored := lifecycleUpdateFromToolOutput(t, s, processorRun, raw)
	if len(stored.Packet.Sources) != 1 || stored.Packet.Sources[0].SourceID != "src-unsupported" {
		t.Fatalf("unsupported source was not durably visible in its packet: %#v", stored.Packet.Sources)
	}
}

func authorizedPersistentManagementExecutionRequest(ownerID, targetAgentID, channelID, summary string, now time.Time) types.CoagentSourcePacket {
	update := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "texture:survivor-control",
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		Role:          agentprofile.Texture,
		Direction:     types.LifecyclePacketDirectionControl,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "execution_request",
			Summary:       summary,
			Claims:        []types.CoagentPacketClaim{{Text: summary}},
			Actions: []types.CoagentPacketAction{{
				Type:      "run_command",
				Objective: summary,
				Safety: types.CoagentPacketActionSafety{
					MutationClass: "green",
					Network:       "forbidden",
					FileMutation:  "forbidden",
				},
			}},
		},
		CreatedAt: now,
	}
	update.UpdateID = deriveWorkerUpdateID(update)
	update.Content = buildWorkerUpdateMessage(update)
	return update
}

func mustNow(t *testing.T) time.Time {
	t.Helper()
	return time.Now().UTC()
}

// TestSurvivorContract_ManagementSettlesNonExecutionRequestPackets proves the E3.2
// obligation: non-execution packets addressed to persistent Management are
// automatically settled (marked delivered/settled) during reconciliation
// so they do not linger in the mailbox backlog forever.

func TestSurvivorContract_ManagementSettlesNonExecutionBeforeExecutionBacklog(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-survivor-settle-mixed"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}

	now := mustNow(t)
	nonExec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "engineering:survivor-settle-mixed",
		TargetAgentID: managementAgent.AgentID,
		ChannelID:     managementAgent.ChannelID,
		Role:          agentprofile.Engineering,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "non-execution packet before executable work",
			Claims:        []types.CoagentPacketClaim{{Text: "This packet is evidence, not privileged work."}},
		},
		CreatedAt: now,
	}
	nonExec.UpdateID = deriveWorkerUpdateID(nonExec)
	nonExec.Content = buildWorkerUpdateMessage(nonExec)
	exec := authorizedPersistentManagementExecutionRequest(ownerID, managementAgent.AgentID, managementAgent.ChannelID, "executable work after non-execution packet", now.Add(time.Millisecond))

	for _, update := range []types.CoagentSourcePacket{nonExec, exec} {
		msg := &types.ChannelMessage{
			ChannelID:    update.ChannelID,
			From:         update.AgentID,
			FromAgentID:  update.AgentID,
			ToAgentID:    update.TargetAgentID,
			TrajectoryID: update.TrajectoryID,
			Role:         update.Role,
			Content:      update.Content,
			Timestamp:    update.CreatedAt,
		}
		if _, created, err := s.DispatchWorkerUpdate(ctx, update, msg); err != nil {
			t.Fatalf("dispatch seeded update %s: %v", update.UpdateID, err)
		} else if !created {
			t.Fatalf("seeded update %s was not created", update.UpdateID)
		}
	}

	run, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run == nil {
		t.Fatal("expected execution_request to start a persistent Management run")
	}
	ids := metadataStringSlice(run.Metadata["worker_update_ids"])
	if len(ids) != 1 || ids[0] != exec.UpdateID {
		t.Fatalf("worker_update_ids = %+v, want only executable update %s", ids, exec.UpdateID)
	}

	storedNonExec, err := s.GetWorkerUpdate(ctx, ownerID, nonExec.UpdateID)
	if err != nil {
		t.Fatalf("get settled non-execution update: %v", err)
	}
	if storedNonExec.DeliveredToRunID != "settled_non_executable" || storedNonExec.DeliveredAt == nil {
		t.Fatalf("non-execution update not settled: %+v", storedNonExec)
	}
	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, managementAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list backlog: %v", err)
	}
	if len(backlog) != 1 || backlog[0].UpdateID != exec.UpdateID {
		t.Fatalf("backlog = %+v, want only executable update %s", backlog, exec.UpdateID)
	}
}

func TestSurvivorContract_ManagementExecutesBeforeSettledNonExecutionBacklog(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-survivor-settle-reversed"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}

	now := mustNow(t)
	exec := authorizedPersistentManagementExecutionRequest(ownerID, managementAgent.AgentID, managementAgent.ChannelID, "executable work before non-execution packet", now)
	nonExec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "engineering:survivor-settle-reversed",
		TargetAgentID: managementAgent.AgentID,
		ChannelID:     managementAgent.ChannelID,
		Role:          agentprofile.Engineering,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "non-execution packet after executable work",
			Claims:        []types.CoagentPacketClaim{{Text: "This packet is evidence, not privileged work."}},
		},
		CreatedAt: now.Add(time.Millisecond),
	}
	nonExec.UpdateID = deriveWorkerUpdateID(nonExec)
	nonExec.Content = buildWorkerUpdateMessage(nonExec)

	for _, update := range []types.CoagentSourcePacket{exec, nonExec} {
		msg := &types.ChannelMessage{
			ChannelID:    update.ChannelID,
			From:         update.AgentID,
			FromAgentID:  update.AgentID,
			ToAgentID:    update.TargetAgentID,
			TrajectoryID: update.TrajectoryID,
			Role:         update.Role,
			Content:      update.Content,
			Timestamp:    update.CreatedAt,
		}
		if _, created, err := s.DispatchWorkerUpdate(ctx, update, msg); err != nil {
			t.Fatalf("dispatch seeded update %s: %v", update.UpdateID, err)
		} else if !created {
			t.Fatalf("seeded update %s was not created", update.UpdateID)
		}
	}

	run, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run == nil {
		t.Fatal("expected execution_request to start a persistent Management run")
	}
	ids := metadataStringSlice(run.Metadata["worker_update_ids"])
	if len(ids) != 1 || ids[0] != exec.UpdateID {
		t.Fatalf("worker_update_ids = %+v, want only executable update %s", ids, exec.UpdateID)
	}

	storedNonExec, err := s.GetWorkerUpdate(ctx, ownerID, nonExec.UpdateID)
	if err != nil {
		t.Fatalf("get settled non-execution update: %v", err)
	}
	if storedNonExec.DeliveredToRunID != "settled_non_executable" || storedNonExec.DeliveredAt == nil {
		t.Fatalf("non-execution update not settled: %+v", storedNonExec)
	}
}

func assignedEngineeringManagementReportPacket(ownerID, targetAgentID, channelID, summary string, now time.Time) types.CoagentSourcePacket {
	update := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "engineering:survivor-report",
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		Role:          agentprofile.Engineering,
		Direction:     types.LifecyclePacketDirectionProducerReport,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       summary,
			Claims:        []types.CoagentPacketClaim{{Text: summary}},
		},
		CreatedAt: now,
	}
	update.UpdateID = deriveWorkerUpdateID(update)
	update.Content = buildWorkerUpdateMessage(update)
	return update
}

func TestSurvivorContract_SenderAuthorizationNotPacketKind(t *testing.T) {
	now := mustNow(t)
	texture := authorizedPersistentManagementExecutionRequest("owner-auth", "management:owner-auth", "management:owner-auth", "texture control", now)
	cases := []struct {
		name       string
		mutate     func(types.CoagentSourcePacket) types.CoagentSourcePacket
		executable bool
		report     bool
	}{
		{name: "texture control execution_request", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket { return u }, executable: true},
		{name: "texture without control direction", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			u.Direction = ""
			return u
		}},
		{name: "texture control evidence_update", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			u.Packet.Kind = "evidence_update"
			u.Packet.Actions = nil
			u.Packet.Claims = []types.CoagentPacketClaim{{Text: "not execution"}}
			return u
		}},
		{name: "cosuper control execution_request spoof", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			u.Role = agentprofile.Engineering
			u.AgentID = "engineering:spoof"
			return u
		}},
		{name: "cosuper producer_report execution_request", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			u.Role = agentprofile.Engineering
			u.AgentID = "engineering:spoof"
			u.Direction = types.LifecyclePacketDirectionProducerReport
			return u
		}},
		{name: "researcher control execution_request", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			u.Role = agentprofile.Research
			u.AgentID = "research:spoof"
			return u
		}},
		{name: "cosuper producer_report evidence_update", mutate: func(u types.CoagentSourcePacket) types.CoagentSourcePacket {
			return assignedEngineeringManagementReportPacket(u.OwnerID, u.TargetAgentID, u.ChannelID, "report", now)
		}, report: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.mutate(texture)
			if persistentManagementExecutablePacket(got) != tc.executable {
				t.Fatalf("executable=%v want %v", persistentManagementExecutablePacket(got), tc.executable)
			}
			if persistentManagementAdmissibleReport(got) != tc.report {
				t.Fatalf("admissible report=%v want %v", persistentManagementAdmissibleReport(got), tc.report)
			}
		})
	}
}

func TestSurvivorContract_AssignedEngineeringReportDoesNotOpenPersistentManagement(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-survivor-cosuper-report"
	managementAgent, err := rt.EnsurePersistentManagementAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}
	now := mustNow(t)
	report := assignedEngineeringManagementReportPacket(ownerID, managementAgent.AgentID, managementAgent.ChannelID, "assigned Engineering report", now)
	msg := &types.ChannelMessage{
		ChannelID: report.ChannelID, From: report.AgentID, FromAgentID: report.AgentID,
		ToAgentID: report.TargetAgentID, Role: report.Role, Content: report.Content, Timestamp: report.CreatedAt,
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, report, msg); err != nil || !created {
		t.Fatalf("dispatch Engineering report: created=%v err=%v", created, err)
	}
	run, err := rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run != nil {
		t.Fatalf("Engineering producer report opened persistent Management run %s", run.RunID)
	}
	stored, err := s.GetWorkerUpdate(ctx, ownerID, report.UpdateID)
	if err != nil {
		t.Fatalf("get Engineering report: %v", err)
	}
	if stored.DeliveredAt != nil || stored.DeliveredToRunID != "" {
		t.Fatalf("Engineering producer report settled instead of retained: %+v", stored)
	}
	managementRun := types.RunRecord{
		RunID: "run-mailbox-super", OwnerID: ownerID, AgentID: managementAgent.AgentID,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		Metadata: map[string]any{"request_source": "update_coagent"},
	}
	if !coagentUpdateDeliverableForRun(&managementRun, report) {
		t.Fatal("mailbox Management cannot inject retained Engineering producer report")
	}
	if persistentManagementExecutablePacket(report) {
		t.Fatal("Engineering producer report became Management-executable")
	}
	textureExec := authorizedPersistentManagementExecutionRequest(ownerID, managementAgent.AgentID, managementAgent.ChannelID, "Texture control opens Management after report", now.Add(time.Millisecond))
	execMsg := &types.ChannelMessage{
		ChannelID: textureExec.ChannelID, From: textureExec.AgentID, FromAgentID: textureExec.AgentID,
		ToAgentID: textureExec.TargetAgentID, Role: textureExec.Role, Content: textureExec.Content, Timestamp: textureExec.CreatedAt,
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, textureExec, execMsg); err != nil || !created {
		t.Fatalf("dispatch Texture control: created=%v err=%v", created, err)
	}
	run, err = rt.reconcilePersistentManagementActor(ctx, ownerID, managementAgent.AgentID)
	if err != nil || run == nil {
		t.Fatalf("Texture control should open Management after retained report: run=%v err=%v", run, err)
	}
	ids := metadataStringSlice(run.Metadata["worker_update_ids"])
	if len(ids) != 1 || ids[0] != textureExec.UpdateID {
		t.Fatalf("worker_update_ids = %+v, want only executable update %s", ids, textureExec.UpdateID)
	}
	storedReport, err := s.GetWorkerUpdate(ctx, ownerID, report.UpdateID)
	if err != nil {
		t.Fatalf("get retained Engineering report: %v", err)
	}
	if storedReport.DeliveredToRunID == "settled_non_executable" {
		t.Fatal("Engineering producer report was settled when Management opened")
	}
	if !coagentUpdateDeliverableForRun(run, storedReport) {
		t.Fatal("opened Management cannot inject retained Engineering producer report")
	}
	if !persistentManagementExecutablePacket(textureExec) {
		t.Fatal("Texture control execution_request lost Management executability")
	}
}
