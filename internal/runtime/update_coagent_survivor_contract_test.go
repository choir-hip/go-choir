package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
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
//   - Super executes ONLY kind=execution_request packets (privilege gate);
//   - non-execution packets addressed to persistent Super are settled instead
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
func TestSurvivorContract_AcceptsCanonicalSurface(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-survivor-canonical"
	docID := "doc-survivor"
	now := mustNow(t)
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   currentTextureAgentID(docID),
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileTexture,
		Role:      AgentProfileTexture,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert texture agent: %v", err)
	}
	researcherRun := d9CoagentRun("run-survivor-canonical", ownerID, "researcher:survivor", AgentProfileResearcher, docID, "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(researcherRun)), "update_coagent", json.RawMessage(validEvidenceUpdatePacket))
	if err != nil {
		t.Fatalf("update_coagent canonical surface rejected: %v", err)
	}
	stored, err := s.GetWorkerUpdate(ctx, ownerID, d9UpdateID(t, raw))
	if err != nil {
		t.Fatalf("get stored packet: %v", err)
	}
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
	superRun := d9CoagentRun("run-survivor-reject", "user-survivor-reject", "super:survivor-reject", AgentProfileSuper, "doc-survivor-reject", currentTextureAgentID("doc-survivor-reject"))
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
		_, err := rt.ToolRegistryForProfile(AgentProfileSuper).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun)), "update_coagent", raw)
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
	superRun := d9CoagentRun("run-survivor-unknown", "user-survivor-unknown", "super:survivor-unknown", AgentProfileSuper, "doc-survivor-unknown", currentTextureAgentID("doc-survivor-unknown"))
	raw := json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"unknown field injection",
		"agent_id":"texture:doc-survivor-unknown",
		"channel_id":"doc-survivor-unknown",
		"secret平行surface":["should be rejected"]
	}`)
	if _, err := rt.ToolRegistryForProfile(AgentProfileSuper).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun)), "update_coagent", raw); err == nil {
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
	ctx := context.Background()
	ownerID := "user-survivor-collation"
	docID := "doc-survivor-collation"
	now := mustNow(t)
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   currentTextureAgentID(docID),
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileTexture,
		Role:      AgentProfileTexture,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert texture agent: %v", err)
	}
	researcherRun := d9CoagentRun("run-survivor-collation", ownerID, "researcher:collation", AgentProfileResearcher, docID, "")
	// Deliberately embed source-shaped text in notes and summary prose that
	// must NOT be scraped: an http URL in notes, a "[Source: foo]" style
	// label in summary, and a bare command_output: URI in claims.text. Only
	// the single typed packet.sources entry may become an entity.
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(researcherRun)), "update_coagent", json.RawMessage(`{
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
	stored, err := s.GetWorkerUpdate(ctx, ownerID, d9UpdateID(t, raw))
	if err != nil {
		t.Fatalf("get stored packet: %v", err)
	}
	entities := rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, []types.CoagentSourcePacket{stored})
	if len(entities) == 0 {
		t.Fatalf("no entities collated; the typed packet.source must produce exactly one entity")
	}
	if len(entities) != 1 {
		t.Fatalf("expected exactly 1 entity from packet.sources, got %d: %#v", len(entities), entities)
	}
	for _, forbidden := range []string{"prose-only", "note-only", "should-not-scrape"} {
		for _, entity := range entities {
			identity := firstNonEmpty(entity.Target.PublicRecordID, entity.Target.FilePath, entity.Target.ContentID, entity.Target.ItemID, entity.Target.URL, entity.Target.CanonicalURL)
			if strings.Contains(identity, forbidden) || strings.Contains(entity.Label, forbidden) {
				t.Fatalf("Texture collation scraped prose-only reference %q into entity %#v; only packet.sources may produce entities", forbidden, entity)
			}
		}
	}
}

// TestSurvivorContract_SuperExecutesOnlyExecutionRequestPackets pins the
// privilege gate: persistent Super must not start privileged execution from
// a non-execution_request packet. This complements
// TestPersistentSuperIgnoresNonExecutionRequestUpdatePackets by also
// asserting the deliverable-for-run filter from the run side, so a later
// change cannot weaken one path while leaving the other intact.
func TestSurvivorContract_SuperExecutesOnlyExecutionRequestPackets(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-survivor-super-gate"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}
	// evidence_update addressed to persistent Super: must NOT be executable.
	coSuperRun := d9CoagentRun("run-survivor-super-gate", ownerID, "cosuper:survivor-gate", AgentProfileCoSuper, "", "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileCoSuper).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(coSuperRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"non-execution packet for Super",
		"agent_id":"`+superAgent.AgentID+`",
		"channel_id":"`+superAgent.ChannelID+`",
		"claims":[{"text":"This packet must not start privileged execution."}]
	}`))
	if err != nil {
		t.Fatalf("update_coagent evidence_update to super: %v", err)
	}
	// The privilege filter must reject this packet.
	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list super backlog: %v", err)
	}
	for _, pkt := range backlog {
		if persistentSuperExecutableUpdate(pkt) {
			t.Fatalf("persistentSuperExecutableUpdate admitted non-execution packet %#v", pkt)
		}
	}
	// Construct a persistent-Super-shaped RunRecord the same way the runtime
	// would so isPersistentSuperAgentRun recognizes it; then prove the
	// deliverable filter rejects the non-execution packet from the run side.
	persistentSuperRun := &types.RunRecord{
		RunID:        "run-survivor-super-gate-persistent",
		OwnerID:      ownerID,
		AgentID:      superAgent.AgentID,
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		ChannelID:    superAgent.ChannelID,
		SandboxID:    "sandbox-test",
	}
	if !isPersistentSuperAgentRun(persistentSuperRun) {
		t.Fatalf("test fixture: persistentSuperRun was not recognized as a persistent super run; agentID=%q ownerID=%q", superAgent.AgentID, ownerID)
	}
	// E3.2 obligation: the non-execution packet must not linger as live
	// pending backlog forever. The current behavior leaves it pending (pinned
	// by TestPersistentSuperIgnoresNonExecutionRequestUpdatePackets). This
	// assertion records the survivor contract that E3.2 must make true:
	// after settlement, no backlog row for a non-execution packet addressed
	// to persistent Super remains deliverable for a persistent-Super run.
	if updateID := d9UpdateID(t, raw); updateID != "" {
		for _, pkt := range backlog {
			if pkt.UpdateID == updateID && coagentUpdateDeliverableForRun(persistentSuperRun, pkt) {
				t.Fatalf("non-execution Super packet %s is still deliverable for a persistent-Super run; E3.2 settlement not yet landed", updateID)
			}
		}
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
	ctx := context.Background()
	ownerID := "user-survivor-reported"
	docID := "doc-survivor-reported"
	now := mustNow(t)
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   currentTextureAgentID(docID),
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileTexture,
		Role:      AgentProfileTexture,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert texture agent: %v", err)
	}
	researcherRun := d9CoagentRun("run-survivor-reported", ownerID, "researcher:reported", AgentProfileResearcher, docID, "")
	// A packet.source with an unsupported kind that cannot materialize. The
	// current behavior silently drops it. The survivor contract requires the
	// drop be reported.
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(researcherRun)), "update_coagent", json.RawMessage(`{
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
	stored, err := s.GetWorkerUpdate(ctx, ownerID, d9UpdateID(t, raw))
	if err != nil {
		t.Fatalf("get stored packet: %v", err)
	}
	entities, rejections := rt.evidenceSourceEntitiesAndRejectionsFromWorkerUpdates(ctx, ownerID, []types.CoagentSourcePacket{stored})
	if len(entities) != 0 {
		t.Fatalf("expected zero materialized entities for an unsupported source kind, got %#v", entities)
	}
	if len(rejections) != 1 || rejections[0].SourceID != "src-unsupported" || !strings.Contains(rejections[0].Reason, "did not materialize") {
		t.Fatalf("source rejection = %#v, want visible rejection for src-unsupported", rejections)
	}
}

func mustNow(t *testing.T) time.Time {
	t.Helper()
	return time.Now().UTC()
}

// TestSurvivorContract_SuperSettlesNonExecutionRequestPackets proves the E3.2
// obligation: non-execution packets addressed to persistent Super are
// automatically settled (marked delivered/settled) during reconciliation
// so they do not linger in the mailbox backlog forever.
func TestSurvivorContract_SuperSettlesNonExecutionRequestPackets(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-survivor-settle"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}

	coSuperRun := d9CoagentRun("run-survivor-settle-cosuper", ownerID, "cosuper:survivor-settle", AgentProfileCoSuper, "", "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileCoSuper).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(coSuperRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"non-execution packet to be settled",
		"agent_id":"`+superAgent.AgentID+`",
		"channel_id":"`+superAgent.ChannelID+`",
		"claims":[{"text":"This packet is non-executable and must be settled."}]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	updateID := d9UpdateID(t, raw)

	run, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run != nil {
		t.Fatalf("expected no run to start, got %+v", run)
	}

	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list backlog after: %v", err)
	}
	if len(backlog) != 0 {
		t.Fatalf("backlog = %+v, want 0 (settled)", backlog)
	}

	stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if stored.DeliveredToRunID != "settled_non_executable" {
		t.Fatalf("delivered_to_loop_id = %q, want settled_non_executable", stored.DeliveredToRunID)
	}
	if stored.DeliveredAt == nil {
		t.Fatal("expected delivered_at to be non-nil")
	}
}

func TestSurvivorContract_SuperSettlesNonExecutionBeforeExecutionBacklog(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-survivor-settle-mixed"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}

	now := mustNow(t)
	nonExec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "cosuper:survivor-settle-mixed",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		Role:          AgentProfileCoSuper,
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
	exec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "cosuper:survivor-settle-mixed",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		Role:          AgentProfileCoSuper,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "execution_request",
			Summary:       "executable work after non-execution packet",
			Claims:        []types.CoagentPacketClaim{{Text: "A valid execution request follows the evidence update."}},
			Actions: []types.CoagentPacketAction{{
				Type:      "run_command",
				Objective: "Run a harmless inspection command.",
				Safety: types.CoagentPacketActionSafety{
					MutationClass: "green",
					Network:       "forbidden",
					FileMutation:  "forbidden",
				},
			}},
		},
		CreatedAt: now.Add(time.Millisecond),
	}
	exec.UpdateID = deriveWorkerUpdateID(exec)
	exec.Content = buildWorkerUpdateMessage(exec)

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

	run, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run == nil {
		t.Fatal("expected execution_request to start a persistent Super run")
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
	backlog, err := s.ListCoagentMailboxBacklog(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list backlog: %v", err)
	}
	if len(backlog) != 1 || backlog[0].UpdateID != exec.UpdateID {
		t.Fatalf("backlog = %+v, want only executable update %s", backlog, exec.UpdateID)
	}
}

func TestSurvivorContract_SuperExecutesBeforeSettledNonExecutionBacklog(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-survivor-settle-reversed"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}

	now := mustNow(t)
	exec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "cosuper:survivor-settle-reversed",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		Role:          AgentProfileCoSuper,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "execution_request",
			Summary:       "executable work before non-execution packet",
			Claims:        []types.CoagentPacketClaim{{Text: "A valid execution request is first in the mailbox."}},
			Actions: []types.CoagentPacketAction{{
				Type:      "run_command",
				Objective: "Run a harmless inspection command.",
				Safety: types.CoagentPacketActionSafety{
					MutationClass: "green",
					Network:       "forbidden",
					FileMutation:  "forbidden",
				},
			}},
		},
		CreatedAt: now,
	}
	exec.UpdateID = deriveWorkerUpdateID(exec)
	exec.Content = buildWorkerUpdateMessage(exec)
	nonExec := types.CoagentSourcePacket{
		OwnerID:       ownerID,
		AgentID:       "cosuper:survivor-settle-reversed",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		Role:          AgentProfileCoSuper,
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

	run, err := rt.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile persistent super: %v", err)
	}
	if run == nil {
		t.Fatal("expected execution_request to start a persistent Super run")
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
