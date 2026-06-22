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

func TestUpdateCoagentRejectsMalformedExecutionRequestPackets(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	superRun := d9CoagentRun("run-d9-malformed", "user-d9-malformed", "super:d9-malformed", AgentProfileSuper, "doc-d9-malformed", currentTextureAgentID("doc-d9-malformed"))
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
		if _, err := rt.ToolRegistryForProfile(AgentProfileSuper).Execute(WithToolExecutionContext(ctx, superRun), "update_coagent", raw); err == nil {
			t.Fatalf("%s: update_coagent unexpectedly accepted malformed execution_request", name)
		}
	}
}

func TestUpdateCoagentRejectsUnsupportedSourceAndSelectorKinds(t *testing.T) {
	rt, _ := testRuntime(t)
	d9InstallTools(t, rt)
	ctx := context.Background()
	run := d9CoagentRun("run-d9-source-vocab", "user-d9-source-vocab", "researcher:d9-source-vocab", AgentProfileResearcher, "doc-d9-source-vocab", "")
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
			"actions":[{"type":"request_worker","objective":"Return impossible evidence.","expected_sources":[{"kind":"magic_oracle","required":true}],"safety":{"mutation_class":"red","network":"allowed","file_mutation":"allowed"}}]
		}`),
	} {
		if _, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(WithToolExecutionContext(ctx, run), "update_coagent", raw); err == nil {
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
	run := d9CoagentRun("run-d9-source-alias", ownerID, "researcher:d9-source-alias", AgentProfileResearcher, docID, "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(WithToolExecutionContext(ctx, run), "update_coagent", json.RawMessage(`{
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
	tool, ok := rt.ToolRegistryForProfile(AgentProfileResearcher).Lookup("update_coagent")
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
	coSuperRun := d9CoagentRun("run-d9-super-ignore", ownerID, "cosuper:d9-super-ignore", AgentProfileCoSuper, "", "")
	raw, err := rt.ToolRegistryForProfile(AgentProfileCoSuper).Execute(WithToolExecutionContext(ctx, coSuperRun), "update_coagent", json.RawMessage(`{
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
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		ChannelID:    superAgent.ChannelID,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
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

func schemaObject(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("schema key %q = %#v, want object", key, parent[key])
	}
	return child
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
