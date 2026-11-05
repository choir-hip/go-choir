package agentcore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// textureFoldedTurnLocalID names the turn the fold makes for a cell that
// dispatched without Write, Done, Wait or Reject.
const textureFoldedTurnLocalID = "tray-turn"

// foldTextureDispatch is how a Texture cell dispatches (protocol P2a,
// docs/archive/definitions/choir-protocol-p2-one-surface-2026-10-11.md). Texture's
// staged Cast and Ask to a desk the address resolver reaches fold into the cell's
// one turn, so each dispatch commits with it; a cell that dispatched with
// no turn verb commits as a Wait. The returned intents exclude the folded
// ones; folded lists them for the receipt.
//
// The fold is a pure function of the staged intents and the records the
// cell read, so a replayed cell folds to the same bytes.
//
// Cut 1 L3a/L3b: Texture's conversations with research and management are on
// records. A dispatch becomes an addressed record committed with the turn
// (inbox_records), and the turn carries the cursor advance over the range
// the cell read (inbox_cursor) and the frame's source handles (Cut 2 2a).
func foldTextureDispatch(intents []yaegikernel.StagedIntent, scope ReductionScope, records cellRecords) (out, folded []yaegikernel.StagedIntent, err error) {
	turnAt := -1
	for i, in := range intents {
		if in.Kind != yaegikernel.IntentTextureTurn {
			continue
		}
		if turnAt >= 0 {
			return nil, nil, fmt.Errorf("reduce: a Texture cell stages one turn, not two (%s, %s)", intents[turnAt].LocalID, in.LocalID)
		}
		turnAt = i
	}
	var inboxRecords []types.CommitmentRecord
	var objectives []string
	for _, in := range intents {
		if in.Kind != yaegikernel.IntentCast && in.Kind != yaegikernel.IntentAsk {
			out = append(out, in)
			continue
		}
		route, routeErr := ResolveAddress(scope.DeskAgentID, scope.OwnerID, in.ToDesk)
		if routeErr != nil || !route.OnRoute {
			return nil, nil, fmt.Errorf("reduce: Texture %s: no inbox for %q from desk %q: %v", in.LocalID, in.ToDesk, scope.DeskAgentID, routeErr)
		}
		toInbox := route.Inbox
		desk, _, _ := strings.Cut(strings.TrimSpace(in.ToDesk), ":")
		if route.Forwarded && in.Kind == yaegikernel.IntentAsk {
			return nil, nil, fmt.Errorf("reduce: Texture %s: an Ask aimed at %s cannot be forwarded to its admission; Cast to it or ask management", in.LocalID, strings.TrimSpace(in.ToDesk))
		}
		if route.Forwarded {
			in = markForwarded(in, scope.DeskAgentID)
		}
		if err := types.ValidateRequestFields(in.Kind, in.Req); err != nil {
			return nil, nil, fmt.Errorf("reduce: Texture %s: %s to %s: %w", in.LocalID, in.Kind, desk, err)
		}
		rec, recErr := inboxRecordForIntent(scope, in, toInbox, false)
		if recErr != nil {
			return nil, nil, recErr
		}
		// The fold is a pure function of the staged intents: the commit
		// stamps the record's time, and the Texture author the model that
		// produced the turn (its identity must not depend on a failover).
		rec.Provenance.CommittedAt = ""
		rec.Prediction.CommittedAt = ""
		rec.Provenance.ModelID = ""
		inboxRecords = append(inboxRecords, rec)
		folded = append(folded, in)
		objectives = append(objectives, desk+": "+firstNonEmpty(in.Objective, in.Question, in.Body))
	}
	if turnAt < 0 && len(inboxRecords) == 0 {
		// No turn: the records stay unread for the turn that answers them.
		return intents, nil, nil
	}
	var turn yaegikernel.TextureTurn
	if turnAt >= 0 {
		if err := json.Unmarshal([]byte(intents[turnAt].Body), &turn); err != nil {
			return nil, nil, fmt.Errorf("reduce: Texture %s: decode turn: %w", intents[turnAt].LocalID, err)
		}
	} else {
		turn.Wait = clipReason("dispatched " + strings.Join(objectives, "; "))
	}
	if len(inboxRecords) > 0 {
		turn.InboxRecords = inboxRecords
	}
	turn.InboxCursor = records.cursorAdvance()
	if handles := texturewrite.Handles(texturewrite.RecordsFrom(textureRecordInputs(records.view))); len(handles) > 0 {
		turn.Handles = handles
	}
	raw, err := json.Marshal(turn)
	if err != nil {
		return nil, nil, fmt.Errorf("reduce: encode folded Texture turn: %w", err)
	}
	if turnAt < 0 {
		return append(out, yaegikernel.StagedIntent{LocalID: textureFoldedTurnLocalID, Kind: yaegikernel.IntentTextureTurn, Body: string(raw)}), folded, nil
	}
	for i := range out {
		if out[i].LocalID == intents[turnAt].LocalID && out[i].Kind == yaegikernel.IntentTextureTurn {
			out[i].Body = string(raw)
		}
	}
	return out, folded, nil
}

func clipReason(s string) string {
	if len(s) > 400 {
		return strings.TrimSpace(s[:400]) + "…"
	}
	return s
}
