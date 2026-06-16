package runtime

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureActorIdentityCompatibility(t *testing.T) {
	rec := &types.RunRecord{
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
		},
	}
	if got := configuredAgentProfileForRun(rec); got != AgentProfileVText {
		t.Fatalf("configured profile = %q, want internal %q", got, AgentProfileVText)
	}
	if got := agentProfileForRun(rec); got != AgentProfileVText {
		t.Fatalf("agent profile = %q, want internal %q", got, AgentProfileVText)
	}
	if got := agentRoleForRun(rec); got != AgentProfileVText {
		t.Fatalf("agent role = %q, want internal %q", got, AgentProfileVText)
	}

	agent, metadata := resolveRunIdentity("owner-1", "sandbox-1", map[string]any{
		runMetadataAgentProfile: AgentProfileTexture,
		runMetadataAgentRole:    AgentProfileTexture,
		"doc_id":                "doc-1",
	}, nil)
	if agent.AgentID != "texture:doc-1" || agent.Profile != AgentProfileTexture || agent.Role != AgentProfileTexture || agent.ChannelID != "doc-1" {
		t.Fatalf("resolved texture identity = %+v", agent)
	}
	if metadataStringValue(metadata, runMetadataAgentProfile) != AgentProfileTexture || metadataStringValue(metadata, runMetadataAgentRole) != AgentProfileTexture {
		t.Fatalf("resolved metadata = %+v, want texture profile/role", metadata)
	}

	legacy, legacyMetadata := resolveRunIdentity("owner-1", "sandbox-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		"doc_id":                "doc-1",
	}, nil)
	if legacy.AgentID != "vtext:doc-1" || legacy.Profile != AgentProfileVText || legacy.Role != AgentProfileVText || legacy.ChannelID != "doc-1" {
		t.Fatalf("resolved legacy identity = %+v", legacy)
	}
	if metadataStringValue(legacyMetadata, runMetadataAgentProfile) != AgentProfileVText || metadataStringValue(legacyMetadata, runMetadataAgentRole) != AgentProfileVText {
		t.Fatalf("legacy metadata = %+v, want vtext profile/role", legacyMetadata)
	}

	if !textureAgentIDMatchesDoc("texture:doc-1", "doc-1") {
		t.Fatal("texture agent id did not match doc")
	}
	if !textureAgentIDMatchesDoc("vtext:doc-1", "doc-1") {
		t.Fatal("legacy vtext agent id did not match doc")
	}
	if textureAgentIDMatchesDoc("texture:doc-2", "doc-1") {
		t.Fatal("wrong doc id matched")
	}
	if got := docIDFromTextureAgentID("texture:doc-1"); got != "doc-1" {
		t.Fatalf("texture doc id = %q, want doc-1", got)
	}
	if got := docIDFromTextureAgentID("vtext:doc-1"); got != "doc-1" {
		t.Fatalf("legacy doc id = %q, want doc-1", got)
	}
}

func TestTextureAgentRevisionTaskTypeCompatibility(t *testing.T) {
	if !isTextureAgentRevisionTaskType(textureAgentRevisionTaskType) {
		t.Fatalf("%q should be recognized as current Texture revision task type", textureAgentRevisionTaskType)
	}
	if !isTextureAgentRevisionTaskType(legacyVTextAgentRevisionTaskType) {
		t.Fatalf("%q should remain recognized as legacy Texture revision task type", legacyVTextAgentRevisionTaskType)
	}
	if isTextureAgentRevisionTaskType("researcher") {
		t.Fatal("unrelated task type should not be recognized as Texture revision task type")
	}
}

func TestTextureModelPolicyRoleUsesLegacySelectionKey(t *testing.T) {
	raw := `
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"
reasoning = "low"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
`
	policy, err := parseModelPolicy(raw, "/System/model-policy.toml")
	if err != nil {
		t.Fatalf("parseModelPolicy: %v", err)
	}
	texture := policy.Resolve(AgentProfileTexture)
	if texture.Provider != "fireworks" || texture.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("texture selection = %+v, want legacy vtext role selection", texture)
	}
	if got := normalizeModelPolicyRole("texture"); got != AgentProfileVText {
		t.Fatalf("texture normalized to %q, want %q", got, AgentProfileVText)
	}
}
