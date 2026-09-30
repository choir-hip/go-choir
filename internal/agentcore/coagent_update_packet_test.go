package agentcore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func TestBuildCoagentUpdateUserMessagesTypedPacket(t *testing.T) {
	t.Parallel()
	updates := []types.CoagentSourcePacket{{
		UpdateID:      "upd-1",
		AgentID:       "research:doc-1",
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
	sourceEntities := []types.SourceEntity{{
		EntityID: "src-source-service-demo",
		Kind:     "source_service_item",
		Label:    "Demo source",
		Target: types.SourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_demo",
		},
	}}
	msgs, ids, err := buildCoagentUpdateUserMessages(updates, coagentPacketDeliveryMid, "texture:doc-1", sourceEntities, nil)
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
	if !strings.Contains(text, `"update_id":"upd-1"`) {
		t.Fatalf("packet text missing update id pointer: %q", text)
	}
	// RLM prompt-as-variable: the payload must NOT be in chat — the cell
	// reads it through choir.Updates(). Assert body text and packet claims
	// never inline.
	if strings.Contains(text, "A sourced update arrived.") || strings.Contains(text, "human_projection") || strings.Contains(text, "grounded fact") {
		t.Fatalf("packet text must not inline the payload: %q", text)
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

func TestBuildEmitNoticeUserMessagesPointerNotPayload(t *testing.T) {
	t.Parallel()
	longBody := strings.Repeat("x", 500)
	emits := []yaegikernel.PendingEmit{{
		ChannelID:   "ch-sender",
		MessageSeq:  7,
		FromAgentID: "research:doc-1",
		FromRole:    "research",
		Kind:        "emit",
		Body:        longBody,
	}}
	msgs, err := buildEmitNoticeUserMessages(emits, "texture:doc-1")
	if err != nil {
		t.Fatalf("build emit notice: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notice message, got %d", len(msgs))
	}
	var userMsg struct {
		Role    string `json:"role"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(msgs[0], &userMsg); err != nil {
		t.Fatalf("decode notice: %v", err)
	}
	text := userMsg.Content[0].Text
	// Pointer-not-payload: the full 500-char body must NOT appear in chat —
	// only a bounded snippet plus routing metadata.
	if strings.Contains(text, longBody) {
		t.Fatalf("notice leaked full emission body into chat turn")
	}
	if !strings.Contains(text, "choir.Emits()") {
		t.Fatalf("notice must direct the desk to choir.Emits() for bodies")
	}
	if !strings.Contains(text, "emit_notice") {
		t.Fatalf("notice must carry packet_type emit_notice")
	}
	if !strings.Contains(text, "research:doc-1") || !strings.Contains(text, "7") {
		t.Fatalf("notice missing sender identity or seq: %s", text)
	}
}

func TestBuildEmitNoticeUserMessagesEmpty(t *testing.T) {
	t.Parallel()
	msgs, err := buildEmitNoticeUserMessages(nil, "texture:doc-1")
	if err != nil {
		t.Fatalf("build empty notice: %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil messages for no emits, got %d", len(msgs))
	}
}
