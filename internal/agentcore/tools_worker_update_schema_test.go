package agentcore

import (
	"reflect"
	"slices"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestUpdateCoagentUsesCanonicalClosedPacketPayloadSchema(t *testing.T) {
	canonical := CoagentSourcePacketPayloadSchema()
	tool := newUpdateCoagentTool(nil)
	gotProperties := toolregistry.CloneSchemaMap(tool.Parameters)["properties"].(map[string]any)
	for _, envelopeField := range []string{"agent_id", "channel_id", "work_item_id", "work_disposition"} {
		delete(gotProperties, envelopeField)
	}
	got := toolregistry.JSONSchemaObject(gotProperties, []string{"schema_version", "kind", "summary"}, false)
	if !reflect.DeepEqual(got, canonical) {
		t.Fatalf("update_coagent packet payload schema drifted from canonical helper:\n got=%#v\nwant=%#v", got, canonical)
	}

	if additional, ok := canonical["additionalProperties"].(bool); !ok || additional {
		t.Fatalf("canonical payload additionalProperties = %#v, want false", canonical["additionalProperties"])
	}
	properties := canonical["properties"].(map[string]any)
	kind := properties["kind"].(map[string]any)
	kinds := kind["enum"].([]string)
	if !slices.Contains(kinds, "question") || slices.Contains(kinds, "research_request") {
		t.Fatalf("canonical packet kinds = %v", kinds)
	}
	for _, envelopeField := range []string{"agent_id", "target_agent_id", "channel_id", "work_item_id", "work_disposition", "direction", "command_id", "update_id"} {
		if _, present := properties[envelopeField]; present {
			t.Fatalf("canonical payload exposes envelope authority field %q", envelopeField)
		}
	}

	claims := properties["claims"].(map[string]any)["items"].(map[string]any)
	sources := properties["sources"].(map[string]any)["items"].(map[string]any)
	actions := properties["actions"].(map[string]any)["items"].(map[string]any)
	for name, nested := range map[string]map[string]any{"claims.items": claims, "sources.items": sources, "actions.items": actions} {
		if additional, ok := nested["additionalProperties"].(bool); !ok || additional {
			t.Fatalf("%s additionalProperties = %#v, want false", name, nested["additionalProperties"])
		}
	}
}
