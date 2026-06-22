package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestUpdateCoagentAcceptsResearcherEvidenceUpdateSourcePacket(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-researcher"
	docID := "doc-d9-researcher"
	now := time.Now().UTC()
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
	researcherRun := d9CoagentRun("run-d9-researcher", ownerID, "researcher:d9", AgentProfileResearcher, docID, "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(WithToolExecutionContext(ctx, researcherRun), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"official source is ready",
		"agent_id":"texture:doc-d9-researcher",
		"channel_id":"doc-d9-researcher",
		"claims":[{"text":"The official source confirms the update.","source_ids":["src-official"],"stance":"supports","recommended_surface":"inline_ref"}],
		"sources":[{"source_id":"src-official","kind":"content_item","target":{"uri":"https://example.test/official","title":"Official source"},"selectors":[{"kind":"whole_resource"}],"evidence":{"state":"available","confidence":"high","rights_scope":"private_user_source"}}],
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
	if strings.Contains(stored.Content, "Findings:") || strings.Contains(stored.Content, "Evidence IDs:") {
		t.Fatalf("human projection retained legacy sections: %q", stored.Content)
	}
}

func TestUpdateCoagentRejectsLegacyFieldsAndExecutionRequestWithoutActions(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-reject"
	docID := "doc-d9-reject"
	superRun := d9CoagentRun("run-d9-reject", ownerID, "super:d9", AgentProfileSuper, docID, currentTextureAgentID(docID))
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy","findings":["old shape"]}`),
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"legacy","evidence_ids":["ev-old"]}`),
		json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"missing actions","notes":["not executable"]}`),
	} {
		if _, err := rt.ToolRegistryForProfile(AgentProfileSuper).Execute(WithToolExecutionContext(ctx, superRun), "update_coagent", raw); err == nil {
			t.Fatalf("update_coagent unexpectedly accepted %s", string(raw))
		}
	}
}

func TestUpdateCoagentAcceptsSuperExecutionResultSourcesAndTextureCollatesPacketSourcesOnly(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-super-result"
	docID := "doc-d9-super-result"
	textureAgentID := currentTextureAgentID(docID)
	superRun := d9CoagentRun("run-d9-super-result", ownerID, "super:d9-result", AgentProfileSuper, docID, textureAgentID)
	raw, err := rt.ToolRegistryForProfile(AgentProfileSuper).Execute(WithToolExecutionContext(ctx, superRun), "update_coagent", json.RawMessage(`{
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
	entities := rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, []types.CoagentSourcePacket{stored})
	assertExecutionEntity(t, entities, "command_output", "cmd-d9", "")
	assertExecutionEntity(t, entities, "diff_hunk", "d9-update-coagent", "")
	assertExecutionEntity(t, entities, "test_run", "runtime-d9-focused", "")
	for _, entity := range entities {
		if entity.Target.PublicRecordID == "prose-only" {
			t.Fatalf("Texture collation scraped ordinary prose instead of packet.sources: %#v", entities)
		}
	}
}

func TestRequestSuperExecutionAppendsExecutionRequestPacketWithActions(t *testing.T) {
	rt, s := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	ownerID := "user-d9-super-request"
	docID := "doc-d9-super-request"
	textureRun := d9CoagentRun("run-d9-texture-request", ownerID, currentTextureAgentID(docID), AgentProfileTexture, docID, "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileTexture).Execute(WithToolExecutionContext(ctx, textureRun), "request_super_execution", json.RawMessage(`{
		"objective":"Run a bounded verification command and report command_output plus test_run sources back to Texture.",
		"channel_id":"doc-d9-super-request"
	}`))
	if err != nil {
		t.Fatalf("request_super_execution: %v", err)
	}
	updateID := d9UpdateID(t, raw)
	stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
	if err != nil {
		t.Fatalf("get super request packet: %v", err)
	}
	if stored.Packet.Kind != "execution_request" || len(stored.Packet.Actions) == 0 {
		t.Fatalf("super request packet = %#v, want execution_request with actions", stored.Packet)
	}
	if stored.Packet.Actions[0].Type != "request_worker" {
		t.Fatalf("super request action = %#v, want request_worker", stored.Packet.Actions[0])
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
		metadata["requested_by_profile"] = AgentProfileTexture
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
