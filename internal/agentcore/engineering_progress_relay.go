package agentcore

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// engineeringProgressRelays returns the progress records to mint beside an
// engineering cell's records: one platform-authored Note per progress Report
// to management, addressed to the Texture whose request the assignment
// serves (docs/problems/engineering-reports-never-reach-texture-2026-10-24.md).
//
// Engineering still addresses management only, and management still reads
// the Report; the relay is the ledger projected into the inbox of the one
// desk that renders it for the owner. Texture stays the only writer of the
// document: the record is evidence it may use, ignore or reject. The relay
// is a system Note from progress:<assignment>, so it opens and closes no
// round and is no desk's speech.
//
// It is off unless the computer's engineering.progress_to_texture setting is
// on (red: it wakes Texture). Anything that does not bind (no setting, no
// records assignment, a parent that is not a Texture) mints nothing and
// leaves the cell's own records exactly as they were.
func (r *rlmCallReduction) engineeringProgressRelays(ctx context.Context, records []types.CommitmentRecord) []types.CommitmentRecord {
	rt := r.rt()
	if rt == nil || rt.store == nil || r.rec == nil || strings.TrimSpace(r.rec.AgentProfile) != agentprofile.Engineering {
		return nil
	}
	var relays []types.CommitmentRecord
	switchOn, switchRead := false, false
	for _, rec := range records {
		if rec.Report == nil || rec.Final || rec.EmitterKind != types.CommitmentEmitterAgent ||
			!strings.HasPrefix(rec.SenderInbox, agentprofile.Engineering+":") || !strings.HasPrefix(rec.Inbox, agentprofile.Management+":") {
			continue
		}
		if !switchRead {
			switchOn, switchRead = rt.computerSettingOn(ctx, r.scope.OwnerID, r.scope.ComputerID, store.SettingEngineeringProgressToTexture), true
		}
		if !switchOn {
			return nil
		}
		assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, r.scope.OwnerID, r.scope.ComputerID, rec.SenderInbox)
		if err != nil {
			log.Printf("runtime: progress relay: assignment of %s: %v (no relay)", rec.SenderInbox, err)
			continue
		}
		textureInbox, ok := types.ConversationCounterpart(assignment.Binding.ParentConversationID, assignment.Binding.ParentAgentID)
		if !ok {
			continue
		}
		if relay, ok := types.NewEngineeringProgressRelay(rec, textureInbox); ok {
			relays = append(relays, relay)
		}
	}
	return relays
}
