package textureowner

import (
	"encoding/json"
	"testing"
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
