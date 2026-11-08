package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// resolveTextureRebaseNotices binds each Texture Cast to management whose
// packet names a rebase_notice to the notice record it answers (problem 16,
// docs/problems/selfdev-round-blockers-cluster-2026-11-03.md, clustering
// assessment 2). Pool's Texture copied the id with its sender inbox as an
// extra segment, and both the timer and admission compared strings, so the
// rebase was refused and the answer went unseen. The naming is resolved
// (ResolveStackRebaseNoticeID) and must be a rebase notice on the tape sent to
// this Texture; the packet then carries that record's own id. A naming that
// resolves to none refuses the cell, so the desk learns at once instead of
// management's round failing later. Notice records are immutable, so a
// replayed cell resolves to the same bytes. The caller's intents are not
// modified.
func (r *rlmCallReduction) resolveTextureRebaseNotices(ctx context.Context, intents []yaegikernel.StagedIntent) ([]yaegikernel.StagedIntent, error) {
	desk := strings.TrimSpace(r.scope.DeskAgentID)
	var out []yaegikernel.StagedIntent
	for i, in := range intents {
		if in.Kind != yaegikernel.IntentCast || !deskmanifest.AdmitsAssignments(strings.TrimSpace(in.ToDesk)) {
			continue
		}
		packet, ok := castPacketObject(in.Statement)
		if !ok {
			continue
		}
		raw, named := packet[types.RebaseNoticeCastField]
		if !named {
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: rebase_notice is the notice's record id as a string", in.LocalID)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		id, ok := types.ResolveStackRebaseNoticeID(text)
		if !ok {
			return nil, fmt.Errorf("reduce: Texture cast %s: rebase_notice %s is not a rebase notice id (rebase-notice:<operation>:<base>:<attempt>); copy it from the notice record", in.LocalID, truncateForError(text))
		}
		if r.ledger == nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: no store to resolve rebase notice %s", in.LocalID, id)
		}
		notice, err := r.ledger.GetCommitmentRecord(ctx, strings.TrimSpace(r.scope.OwnerID), strings.TrimSpace(r.scope.ComputerID), id)
		if err != nil {
			return nil, fmt.Errorf("reduce: Texture cast %s: rebase notice %s: %w", in.LocalID, id, err)
		}
		if notice == nil || !types.IsStackRebaseNotice(*notice) || strings.TrimSpace(notice.Inbox) != desk {
			return nil, fmt.Errorf("reduce: Texture cast %s: rebase_notice %s names no rebase notice sent to %s; copy the id from the notice record", in.LocalID, truncateForError(text), desk)
		}
		if text == notice.RecordID {
			continue
		}
		canonical, err := json.Marshal(notice.RecordID)
		if err != nil {
			return nil, err
		}
		packet[types.RebaseNoticeCastField] = canonical
		statement, err := json.Marshal(packet)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = append([]yaegikernel.StagedIntent(nil), intents...)
		}
		out[i].Statement = string(statement)
	}
	if out == nil {
		return intents, nil
	}
	return out, nil
}
