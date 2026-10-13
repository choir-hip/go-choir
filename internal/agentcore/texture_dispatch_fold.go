package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// foldTextureDispatch is how a Texture cell dispatches (protocol P2a,
// docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md). Texture's
// staged Cast, Ask and Note to research or management fold into the cell's
// one apply or decide ApplyTexture body, so each dispatch commits with the
// turn. The returned intents exclude the folded ones; folded lists them for
// the receipt.
//
// The fold is a pure function of the staged intents, so a replayed cell folds
// to the same bytes. A cell with no dispatch keeps its body byte-identical.
//
// Cut 1 L3a/L3b: Texture's conversations with research and management are on
// records. A dispatch becomes an addressed record committed with the turn
// (inbox_records), and every apply or decide body carries the cursor advance
// over the range the cell read (inbox_cursor).
func foldTextureDispatch(intents []yaegikernel.StagedIntent, scope ReductionScope, records cellRecords) (out, folded []yaegikernel.StagedIntent, err error) {
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
			if _, ok := probe["inbox_records"]; ok {
				return nil, nil, fmt.Errorf("reduce: Texture %s: ApplyTexture does not take inbox_records; dispatch to research with choir.Cast, choir.Ask or choir.Note in the same cell", in.LocalID)
			}
			if _, ok := probe["inbox_cursor"]; ok {
				return nil, nil, fmt.Errorf("reduce: Texture %s: ApplyTexture does not take inbox_cursor; the runtime advances it", in.LocalID)
			}
		}
	}

	var inboxRecords []types.CommitmentRecord
	for _, in := range intents {
		if in.Kind != yaegikernel.IntentCast && in.Kind != yaegikernel.IntentAsk && in.Kind != yaegikernel.IntentNote {
			out = append(out, in)
			continue
		}
		desk, _ := agentprofile.Canonical(strings.TrimSpace(in.ToDesk))
		if desk != agentprofile.Research && desk != agentprofile.Management {
			return nil, nil, fmt.Errorf("reduce: Texture %s: Texture dispatches only to research or management, not %q", in.LocalID, in.ToDesk)
		}
		toInbox, onRoute, routeErr := recordsRoute(scope.DeskAgentID, scope.OwnerID, desk)
		if routeErr != nil || !onRoute {
			return nil, nil, fmt.Errorf("reduce: Texture %s: no %s inbox for desk %q: %v", in.LocalID, desk, scope.DeskAgentID, routeErr)
		}
		if in.Kind == yaegikernel.IntentNote && strings.TrimSpace(in.Body) == "" {
			return nil, nil, fmt.Errorf("reduce: Texture %s: a note to %s needs a body", in.LocalID, desk)
		}
		rec, recErr := inboxRecordForIntent(scope, in, toInbox, false)
		if recErr != nil {
			return nil, nil, recErr
		}
		// The fold is a pure function of the staged intents: the commit
		// stamps the record's time.
		rec.Provenance.CommittedAt = ""
		rec.Prediction.CommittedAt = ""
		inboxRecords = append(inboxRecords, rec)
		folded = append(folded, in)
	}
	cursor := records.cursorAdvance()
	if len(inboxRecords) == 0 {
		if cursor == nil || applyAt < 0 {
			return intents, nil, nil
		}
		// No dispatch, but an apply or decide turn marks its range read.
		return withFoldedBody(intents, applyAt, func(body map[string]any) {
			if op, _ := body["op"].(string); op == "apply" || op == "decide" {
				body["inbox_cursor"] = cursor
			}
		})
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
	if len(inboxRecords) > 0 {
		body["inbox_records"] = inboxRecords
	}
	if cursor != nil {
		body["inbox_cursor"] = cursor
	}
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




// withFoldedBody rewrites the one staged ApplyTexture body with set applied.
func withFoldedBody(intents []yaegikernel.StagedIntent, applyAt int, set func(map[string]any)) (out, folded []yaegikernel.StagedIntent, err error) {
	var body map[string]any
	if err := json.Unmarshal([]byte(intents[applyAt].Body), &body); err != nil {
		return nil, nil, fmt.Errorf("reduce: Texture %s: decode ApplyTexture body: %w", intents[applyAt].LocalID, err)
	}
	set(body)
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("reduce: encode folded ApplyTexture body: %w", err)
	}
	out = append([]yaegikernel.StagedIntent(nil), intents...)
	out[applyAt].Body = string(raw)
	return out, nil, nil
}
