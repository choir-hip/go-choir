package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// texture_sendback.go — Texture never settles a rejected round as complete
// (owner, 2026-11-04: "Rejected rounds don't complete but get sent back";
// docs/problems/texture-settles-rejected-round-as-complete-2026-11-04.md).
//
// A Done on a trajectory whose last candidate was rejected, with nothing
// still working on it, is refused at commit and nothing is committed:
//   - attempts left: the refusal carries the rejection's reasons and tells
//     the desk to Cast engineering again (the send-back);
//   - the budget spent: the request's work item is refused with the visible
//     fate "rejected after N attempts: ...", the round ends rejected.
//
// The read is the operations' own state (agentcore.RejectedRoundOf), never
// the desk's account of the work. A crash close and an owner abandon are
// not rejections there, and nothing here dispatches work: a restart never
// re-dispatches a rejected round.

func (h *Handler) rejectedRound(ctx context.Context, ownerID, computerID, trajectoryID string) (agentcore.RejectedRound, bool) {
	if h == nil {
		return agentcore.RejectedRound{}, false
	}
	if h.rejectedRoundOf != nil {
		return h.rejectedRoundOf(ctx, ownerID, computerID, trajectoryID)
	}
	if h.Core == nil {
		return agentcore.RejectedRound{}, false
	}
	return h.Core.RejectedRoundOf(ctx, ownerID, computerID, trajectoryID)
}

func (h *Handler) refuseDoneOnRejectedRound(ctx context.Context, rec *types.RunRecord, doc types.Document, turn yaegikernel.TextureTurn) error {
	if h == nil || h.Store == nil || rec == nil || strings.TrimSpace(turn.Done) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil
	}
	if _, isRoot := h.textureRootFor(ctx, rec.OwnerID, doc.ComputerID, doc.DocID); isRoot {
		return nil
	}
	round, rejected := h.rejectedRound(ctx, rec.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if !rejected {
		return nil
	}
	if !round.Exhausted {
		return fmt.Errorf("choir: Done refused: %s", round.SendBackInstruction())
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, rec.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return fmt.Errorf("load the Texture request's trajectory: %w", err)
	}
	fate := round.Fate()
	if work, open := openTextureRequest(snapshot, rec.AgentID); open {
		sum := sha256.Sum256([]byte(strings.Join([]string{rec.OwnerID, doc.ComputerID, doc.TrajectoryID, work.WorkItemID, round.OperationID}, "\x00")))
		refuse := types.RefuseLifecycleWorkRequest{
			OwnerID: rec.OwnerID, ComputerID: doc.ComputerID, CommandID: "texture-round-rejected:" + hex.EncodeToString(sum[:16]),
			TrajectoryID: doc.TrajectoryID, WorkItemID: work.WorkItemID, ActingAgentID: rec.AgentID,
			RefusalRef: "rejected:" + round.OperationID, Reason: fate,
		}
		if refuse.CommandDigest, err = store.ComputeRefuseLifecycleWorkDigest(refuse); err != nil {
			return err
		}
		if _, err := h.Store.RefuseLifecycleWork(ctx, refuse); err != nil {
			return fmt.Errorf("end the rejected round %s: %w", work.WorkItemID, err)
		}
		h.NoteTextureRootChange(context.WithoutCancel(ctx), rec.OwnerID, doc.ComputerID, doc.DocID)
		log.Printf("texture: %s (document %s, work %s)", fate, doc.DocID, work.WorkItemID)
	}
	return fmt.Errorf("choir: Done refused: the round ended %s. It is not complete and is not settled as complete; the owner's next revise starts new work. Nothing was committed", fate)
}
