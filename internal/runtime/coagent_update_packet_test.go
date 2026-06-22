package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestBuildCoagentUpdateUserMessagesTypedPacket(t *testing.T) {
	t.Parallel()
	updates := []types.CoagentSourcePacket{{
		UpdateID:      "upd-1",
		AgentID:       "researcher:doc-1",
		TargetAgentID: "texture:doc-1",
		ChannelID:     "doc-1",
		Packet: newCoagentPacket(
			"evidence_update",
			"grounded fact",
			[]types.CoagentPacketClaim{coagentClaim("A sourced update arrived.", "src-demo")},
			[]types.CoagentPacketSource{coagentSourceFromURI("src-demo", "source_service_item", "source_service_item:srcitem_demo", "Demo source")},
			nil,
			nil,
			nil,
		),
		Content:    "A sourced update arrived.",
		MessageSeq: 3,
	}}
	sourceEntities := []textureSourceEntity{{
		EntityID: "src-source-service-demo",
		Kind:     "source_service_item",
		Label:    "Demo source",
		Target: textureSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_demo",
		},
	}}
	msgs, ids, err := buildCoagentUpdateUserMessages(updates, coagentPacketDeliveryMid, "texture:doc-1", sourceEntities)
	if err != nil {
		t.Fatalf("build messages: %v", err)
	}
	if len(ids) != 1 || ids[0] != "upd-1" {
		t.Fatalf("update ids = %+v, want upd-1", ids)
	}
	if len(msgs) != 1 {
		t.Fatalf("message count = %d, want 1", len(msgs))
	}
	var msg map[string]any
	if err := json.Unmarshal(msgs[0], &msg); err != nil {
		t.Fatalf("decode message: %v", err)
	}
	content, _ := msg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content blocks = %+v", content)
	}
	block, _ := content[0].(map[string]any)
	text, _ := block["text"].(string)
	if !strings.Contains(text, "coagent_update") {
		t.Fatalf("packet text missing typed metadata: %q", text)
	}
	if !strings.Contains(text, "A sourced update arrived.") {
		t.Fatalf("packet text missing update body: %q", text)
	}
	if !strings.Contains(text, `"source_entities"`) ||
		!strings.Contains(text, "src-source-service-demo") ||
		!strings.Contains(text, "Texture source entities/transclusion refs") ||
		!strings.Contains(text, "Do not write ordinary URL links") {
		t.Fatalf("packet text missing native source entity instruction: %q", text)
	}
	if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
		t.Fatalf("packet text should not instruct ordinary clickable links: %q", text)
	}
}

func TestRunSupportsCoagentUpdateInjectionIncludesTexture(t *testing.T) {
	t.Parallel()
	rec := &types.RunRecord{
		AgentID: "texture:doc-1",
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
		},
	}
	if !runSupportsCoagentUpdateInjection(rec) {
		t.Fatal("texture runs should support coagent update injection")
	}
}

func TestResolveResearcherFindingsTargetRequiresExplicitTextureAgent(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntimeWithProviderAndRegistry(t, NewStubProvider(0), nil)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			"requested_by_profile":  AgentProfileTexture,
			"requested_by_agent_id": "texture:doc-target",
			runMetadataChannelID:    "doc-target",
		},
	})
	if _, _, err := resolveFindingsTarget(ctx, rt, ""); err == nil || !strings.Contains(err.Error(), "requires agent_id") {
		t.Fatalf("missing agent_id err = %v, want requires agent_id", err)
	}
	target, channel, err := resolveFindingsTarget(ctx, rt, "texture:doc-target")
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if target != "texture:doc-target" || channel != "doc-target" {
		t.Fatalf("target/channel = %q/%q, want texture:doc-target/doc-target", target, channel)
	}
	if _, _, err := resolveFindingsTarget(ctx, rt, "super:primary"); err == nil || !strings.Contains(err.Error(), "Texture coagent") {
		t.Fatalf("non-texture agent_id err = %v, want Texture coagent requirement", err)
	}
}
