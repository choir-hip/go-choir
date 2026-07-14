package runtime

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureActorIdentityCompatibility(t *testing.T) {
	rec := &types.RunRecord{
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
		},
	}
	if got := configuredAgentProfileForRun(rec); got != agentprofile.Texture {
		t.Fatalf("configured profile = %q, want internal %q", got, agentprofile.Texture)
	}
	if got := agentProfileForRun(rec); got != agentprofile.Texture {
		t.Fatalf("agent profile = %q, want internal %q", got, agentprofile.Texture)
	}
	if got := agentRoleForRun(rec); got != agentprofile.Texture {
		t.Fatalf("agent role = %q, want internal %q", got, agentprofile.Texture)
	}

	agent, metadata := resolveRunIdentity("owner-1", "sandbox-1", map[string]any{
		runMetadataAgentProfile: agentprofile.Texture,
		runMetadataAgentRole:    agentprofile.Texture,
		"doc_id":                "doc-1",
	}, nil)
	if agent.AgentID != "texture:doc-1" || agent.Profile != agentprofile.Texture || agent.Role != agentprofile.Texture || agent.ChannelID != "doc-1" {
		t.Fatalf("resolved texture identity = %+v", agent)
	}
	if metadataStringValue(metadata, runMetadataAgentProfile) != agentprofile.Texture || metadataStringValue(metadata, runMetadataAgentRole) != agentprofile.Texture {
		t.Fatalf("resolved metadata = %+v, want texture profile/role", metadata)
	}

	legacy, legacyMetadata := resolveRunIdentity("owner-1", "sandbox-1", map[string]any{
		runMetadataAgentProfile: agentprofile.Texture,
		runMetadataAgentRole:    agentprofile.Texture,
		"doc_id":                "doc-1",
	}, nil)
	if legacy.AgentID != "texture:doc-1" || legacy.Profile != agentprofile.Texture || legacy.Role != agentprofile.Texture || legacy.ChannelID != "doc-1" {
		t.Fatalf("resolved legacy identity = %+v", legacy)
	}
	if metadataStringValue(legacyMetadata, runMetadataAgentProfile) != agentprofile.Texture || metadataStringValue(legacyMetadata, runMetadataAgentRole) != agentprofile.Texture {
		t.Fatalf("legacy metadata = %+v, want texture profile/role", legacyMetadata)
	}

	if !textureAgentIDMatchesDoc("texture:doc-1", "doc-1") {
		t.Fatal("texture agent id did not match doc")
	}
	if !textureAgentIDMatchesDoc("texture:doc-1", "doc-1") {
		t.Fatal("legacy texture agent id did not match doc")
	}
	if textureAgentIDMatchesDoc("texture:doc-2", "doc-1") {
		t.Fatal("wrong doc id matched")
	}
	if got := docIDFromTextureAgentID("texture:doc-1"); got != "doc-1" {
		t.Fatalf("texture doc id = %q, want doc-1", got)
	}
	if got := docIDFromTextureAgentID("texture:doc-1"); got != "doc-1" {
		t.Fatalf("legacy doc id = %q, want doc-1", got)
	}
}

func TestTextureAgentRevisionTaskTypeCompatibility(t *testing.T) {
	if !isTextureAgentRevisionTaskType(textureAgentRevisionTaskType) {
		t.Fatalf("%q should be recognized as current Texture revision task type", textureAgentRevisionTaskType)
	}
	if isTextureAgentRevisionTaskType("researcher") {
		t.Fatal("unrelated task type should not be recognized as Texture revision task type")
	}
}
