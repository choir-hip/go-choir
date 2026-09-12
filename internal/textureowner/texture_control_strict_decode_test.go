package textureowner

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureControlPacketsStrictDecodeEveryNestedAuthorityBoundary(t *testing.T) {
	validPacket := `{
		"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"do exact work",
		"claims":[{"claim_id":"claim-1","text":"grounded","source_ids":["source-1"],"stance":"supports","recommended_surface":"inline_ref"}],
		"sources":[{"source_id":"source-1","kind":"web_url","target":{"uri":"https://example.com","title":"Example","media_type":"text/html"},"selectors":[{"kind":"text_quote","quote":"grounded","start":1,"end":9}],"excerpt":"grounded","reader_snapshot":{"text_content":"grounded","snapshot_kind":"reader","media_type":"text/plain","source_url":"https://example.com"},"evidence":{"state":"available","confidence":"high","rights_scope":"public"}}],
		"actions":[{"action_id":"action-1","type":"run_tests","objective":"run focused tests","inputs":{"suite":"focused","options":[{"shard":"one"}]},"expected_sources":[{"kind":"file","required":true}],"safety":{"mutation_class":"yellow","network":"forbidden","file_mutation":"allowed"}}],
		"questions":["what changed?"],"notes":["preserve evidence"]
	}`
	wrapEdit := func(packet string) json.RawMessage {
		return json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"body","controls":[{"target_work_item_id":"work","packet":` + packet + `}]}`)
	}
	wrapDecision := func(packet string) json.RawMessage {
		return json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","decision_kind":"wait_for_evidence","reason":"wait","controls":[{"target_work_item_id":"work","packet":` + packet + `}]}`)
	}
	if _, err := decodeTextureEditArgs("rewrite_texture", wrapEdit(validPacket)); err != nil {
		t.Fatalf("valid recursively typed control packet: %v", err)
	}
	if _, err := decodeRecordTextureDecisionArgs(wrapDecision(validPacket)); err != nil {
		t.Fatalf("valid recursively typed decision control packet: %v", err)
	}

	badPackets := map[string]string{
		"packet envelope authority":        `{"schema_version":"coagent_source_packet.v1","kind":"question","summary":"x","notes":["x"],"target_agent_id":"super:other"}`,
		"claim unknown":                    `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x","claims":[{"text":"x","owner_id":"other"}]}`,
		"source target unknown":            `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x","sources":[{"kind":"web_url","target":{"uri":"https://example.com","computer_id":"other"}}]}`,
		"selector unknown":                 `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x","sources":[{"kind":"web_url","target":{"uri":"https://example.com"},"selectors":[{"kind":"text_quote","quote":"x","direction":"control"}]}]}`,
		"reader snapshot unknown":          `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x","sources":[{"kind":"web_url","target":{"uri":"https://example.com"},"reader_snapshot":{"text_content":"x","target_agent_id":"super:other"}}]}`,
		"evidence unknown":                 `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"x","sources":[{"kind":"web_url","target":{"uri":"https://example.com"},"evidence":{"state":"available","lifecycle_version":9}}]}`,
		"action unknown":                   `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","target_work_item_id":"other"}]}`,
		"expected source unknown":          `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","expected_sources":[{"kind":"file","update_id":"forged"}]}]}`,
		"safety unknown":                   `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","safety":{"mutation_class":"yellow","network":"forbidden","file_mutation":"allowed","command_id":"forged"}}]}`,
		"questions object":                 `{"schema_version":"coagent_source_packet.v1","kind":"question","summary":"x","questions":[{"text":"silently dropped before"}]}`,
		"notes object":                     `{"schema_version":"coagent_source_packet.v1","kind":"question","summary":"x","notes":[{"text":"silently dropped before"}]}`,
		"recursive inputs authority":       `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"nested":[{"target_agent_id":"super:other"}]}}]}`,
		"recursive inputs camel authority": `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"nested":[{"requestedTargetAgentId":"super:other"}]}}]}`,
		"recursive control binding":        `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"control_binding_id":"forged"}}]}`,
		"recursive assignment identity":    `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"assignment_id":"forged","capsule_id":"forged"}}]}`,
		"recursive capability handle":      `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"capability_digest":"sha256:forged","execution_handle":"forged"}}]}`,
		"recursive outcome witness":        `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"source_outcome_sha256":"sha256:forged"}}]}`,
		"recursive assignment attempt":     `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"attempt":99,"parent_loop_id":"forged","parent_decision_id":"forged"}}]}`,
		"recursive capsule modes":          `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"network_mode":"allowed","filesystem_mode":"host","writable":true}}]}`,
		"recursive coordination binding":   `{"schema_version":"coagent_source_packet.v1","kind":"execution_request","summary":"x","actions":[{"type":"run_tests","objective":"x","inputs":{"coordination_contract_id":"forged","coordination_contract_digest":"sha256:forged"}}]}`,
		"null nested array member":         `{"schema_version":"coagent_source_packet.v1","kind":"question","summary":"x","claims":[null],"notes":["would otherwise hide null"]}`,
	}
	for name, packet := range badPackets {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeTextureEditArgs("rewrite_texture", wrapEdit(packet)); err == nil {
				t.Fatal("rewrite accepted nested packet")
			}
			if _, err := decodeRecordTextureDecisionArgs(wrapDecision(packet)); err == nil {
				t.Fatal("decision accepted nested packet")
			}
		})
	}
}

func TestTextureControlAuthorityRejectorCoversCompleteCoSuperAssignmentBinding(t *testing.T) {
	bindingType := reflect.TypeOf(types.CoSuperAssignmentBinding{})
	for i := 0; i < bindingType.NumField(); i++ {
		field := bindingType.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		t.Run(jsonName, func(t *testing.T) {
			value := map[string]any{"nested": []any{map[string]any{jsonName: "model-authored"}}}
			if err := rejectTextureControlAuthorityFields(value, "packet.actions[0].inputs"); err == nil {
				t.Fatalf("runtime-owned CoSuper assignment binding %q is not reserved", jsonName)
			}
		})
	}
}

func TestTextureControlToolsExposeCanonicalPayloadOnlySchema(t *testing.T) {
	want := agentcore.CoagentSourcePacketPayloadSchema()
	for _, tool := range []toolregistry.Tool{
		newPatchTextureTool(nil),
		newRewriteTextureTool(nil),
		newRecordTextureDecisionTool(nil),
	} {
		t.Run(tool.Name, func(t *testing.T) {
			properties := tool.Parameters["properties"].(map[string]any)
			controls := properties["controls"].(map[string]any)
			items := controls["items"].(map[string]any)
			controlProperties := items["properties"].(map[string]any)
			packet := toolregistry.CloneSchemaMap(controlProperties["packet"].(map[string]any))
			delete(packet, "description")
			if !reflect.DeepEqual(packet, want) {
				t.Fatalf("controls packet schema drifted from canonical payload schema:\n got=%#v\nwant=%#v", packet, want)
			}
			packetProperties := packet["properties"].(map[string]any)
			for _, authorityField := range []string{"agent_id", "target_agent_id", "channel_id", "work_item_id", "work_disposition", "direction", "control_id", "command_id", "update_id"} {
				if _, present := packetProperties[authorityField]; present {
					t.Fatalf("controls packet exposes runtime envelope authority %q", authorityField)
				}
			}
		})
	}
}
