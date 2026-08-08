package agentcore

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

func TestRolePolicyFromSpecSeparatesSpawnAndMessageTargets(t *testing.T) {
	t.Parallel()

	got := rolePolicyFromSpec(agentprofile.PolicyFor(agentprofile.Texture))
	if want := []string{agentprofile.Researcher}; !reflect.DeepEqual(got.AllowedSpawnTargets, want) {
		t.Fatalf("Texture allowed spawn targets = %v, want %v", got.AllowedSpawnTargets, want)
	}
	if want := []string{agentprofile.Researcher, agentprofile.Super}; !reflect.DeepEqual(got.AllowedMessageTargets, want) {
		t.Fatalf("Texture allowed message targets = %v, want %v", got.AllowedMessageTargets, want)
	}
	if agentprofile.CanSpawn(agentprofile.Texture, agentprofile.Super) || !agentprofile.CanMessage(agentprofile.Texture, agentprofile.Super) {
		t.Fatal("prompt policy must expose Super as Texture message-only authority")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal role policy: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode role policy JSON: %v", err)
	}
	if _, ok := fields["allowed_spawn_targets"]; !ok {
		t.Fatal("role policy JSON omitted allowed_spawn_targets")
	}
	if _, ok := fields["allowed_message_targets"]; !ok {
		t.Fatal("role policy JSON omitted allowed_message_targets")
	}
	if _, stale := fields["allowed_delegate_targets"]; stale {
		t.Fatal("role policy JSON retained allowed_delegate_targets")
	}
}
