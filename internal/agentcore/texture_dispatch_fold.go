package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// foldTextureDispatch is how a Texture cell dispatches (protocol P2a,
// docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md). Texture's
// staged Cast, Ask and Note to research or management become controls on the
// cell's one apply or decide ApplyTexture body, so each dispatch commits with
// the turn through textureTurnControls and the control accumulator: the round
// bound, opener merge and the research chain stay exactly as they are, and the
// accumulator mints each dispatch's record (P1). The returned intents exclude
// the folded ones; folded lists them for the receipt.
//
// The fold is a pure function of the staged intents, so a replayed cell folds
// to the same bytes. A cell with no dispatch keeps its body byte-identical.
func foldTextureDispatch(intents []yaegikernel.StagedIntent) (out, folded []yaegikernel.StagedIntent, err error) {
	applyAt := -1
	applies := 0
	for i, in := range intents {
		if in.Kind != yaegikernel.IntentTextureApply {
			continue
		}
		applies++
		applyAt = i
		var probe map[string]json.RawMessage
		if json.Unmarshal([]byte(in.Body), &probe) == nil {
			var authored []json.RawMessage
			if raw, ok := probe["controls"]; ok && json.Unmarshal(raw, &authored) == nil && len(authored) > 0 {
				return nil, nil, fmt.Errorf("reduce: Texture %s: ApplyTexture no longer takes controls; dispatch with choir.Cast or choir.Ask to research or management in the same cell", in.LocalID)
			}
		}
	}

	var controls []map[string]any
	managementCasts := 0
	for _, in := range intents {
		if in.Kind != yaegikernel.IntentCast && in.Kind != yaegikernel.IntentAsk && in.Kind != yaegikernel.IntentNote {
			out = append(out, in)
			continue
		}
		desk, _ := agentprofile.Canonical(strings.TrimSpace(in.ToDesk))
		var control map[string]any
		switch {
		case desk == agentprofile.Research:
			control, err = foldResearchDispatch(in)
		case desk == agentprofile.Management && in.Kind == yaegikernel.IntentCast:
			managementCasts++
			if managementCasts > 1 {
				return nil, nil, fmt.Errorf("reduce: Texture %s: a turn takes one management Cast; put every action in one execution_request", in.LocalID)
			}
			control, err = foldManagementCast(in)
		case desk == agentprofile.Management:
			return nil, nil, fmt.Errorf("reduce: Texture %s: management takes work only as choir.Cast with an execution_request packet", in.LocalID)
		default:
			return nil, nil, fmt.Errorf("reduce: Texture %s: Texture dispatches only to research or management, not %q", in.LocalID, in.ToDesk)
		}
		if err != nil {
			return nil, nil, err
		}
		controls = append(controls, control)
		folded = append(folded, in)
	}
	if len(controls) == 0 {
		return intents, nil, nil
	}
	if applies > 1 {
		return nil, nil, fmt.Errorf("reduce: a Texture cell that dispatches stages one ApplyTexture, not %d", applies)
	}
	var body map[string]any
	if applyAt >= 0 {
		if err := json.Unmarshal([]byte(intents[applyAt].Body), &body); err != nil {
			return nil, nil, fmt.Errorf("reduce: Texture %s: decode ApplyTexture body: %w", intents[applyAt].LocalID, err)
		}
	}
	if op, _ := body["op"].(string); op != "apply" && op != "decide" {
		return nil, nil, fmt.Errorf("reduce: Texture %s: a dispatch commits with an apply or decide ApplyTexture in the same cell", folded[0].LocalID)
	}
	body["controls"] = controls
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("reduce: encode folded ApplyTexture body: %w", err)
	}
	for i := range out {
		if out[i].LocalID == intents[applyAt].LocalID && out[i].Kind == yaegikernel.IntentTextureApply {
			out[i].Body = string(raw)
		}
	}
	return out, folded, nil
}

func foldResearchDispatch(in yaegikernel.StagedIntent) (map[string]any, error) {
	switch in.Kind {
	case yaegikernel.IntentCast:
		packet, err := foldPacket(in, in.Statement)
		if err != nil {
			return nil, err
		}
		if packet == nil {
			packet = map[string]any{"kind": "question", "summary": in.Objective, "questions": []string{in.Objective}}
		}
		return map[string]any{"open_researcher": true, "objective": in.Objective, "packet": packet, "staged_act": "cast"}, nil
	case yaegikernel.IntentAsk:
		q := strings.TrimSpace(in.Question)
		return map[string]any{"open_researcher": true, "objective": q, "staged_act": "ask",
			"packet": map[string]any{"kind": "question", "summary": q, "questions": []string{q}}}, nil
	default: // Note
		body := strings.TrimSpace(in.Body)
		if body == "" {
			return nil, fmt.Errorf("reduce: Texture %s: a note to research needs a body", in.LocalID)
		}
		return map[string]any{"open_researcher": true, "objective": body, "staged_act": "note",
			"packet": map[string]any{"kind": "evidence_update", "summary": body, "notes": []string{body}}}, nil
	}
}

func foldManagementCast(in yaegikernel.StagedIntent) (map[string]any, error) {
	packet, err := foldPacket(in, in.Statement)
	if err != nil {
		return nil, err
	}
	if kind, _ := packet["kind"].(string); kind != "execution_request" {
		return nil, fmt.Errorf("reduce: Texture %s: a management Cast carries an execution_request packet with actions", in.LocalID)
	}
	return map[string]any{"open_persistent_super": true, "objective": in.Objective, "packet": packet, "staged_act": "cast"}, nil
}

// foldPacket decodes a Texture Cast's third argument, which on Texture is the
// typed packet (on other desks it is the cast spec). Empty means no packet.
func foldPacket(in yaegikernel.StagedIntent, raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var packet map[string]any
	if err := json.Unmarshal([]byte(raw), &packet); err != nil {
		return nil, fmt.Errorf("reduce: Texture %s: a Cast's third argument is the packet (kind, summary, questions, actions ...): %w", in.LocalID, err)
	}
	if kind, _ := packet["kind"].(string); strings.TrimSpace(kind) == "" {
		return nil, fmt.Errorf("reduce: Texture %s: on Texture a Cast's third argument is the packet, which needs a kind (question, evidence_update, execution_request ...) and a summary, not a cast spec", in.LocalID)
	}
	return packet, nil
}
