package textureowner

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// openedWork is a desk work item a revise opened (a reopen of settled work,
// or the continue of a settled document) before it committed its revision.
type openedWork struct {
	Doc        types.Document
	WorkItemID string
}

// closeOpenedWork refuses a work item a failed revise opened, with a visible
// fate. The reason carries UnattendedRequestReason, so the runtime's rule
// holds: the desk did not decide it, and the owner's next revise reopens
// work. Best effort and idempotent in the item.
func (h *Handler) closeOpenedWork(ctx context.Context, ownerID string, opened openedWork, cause error) {
	if h == nil || h.Store == nil || opened.WorkItemID == "" || cause == nil {
		return
	}
	reason := UnattendedRequestReason + ": the revise failed before it committed a revision (" + shortCause(cause) + "); revise again to continue"
	if err := h.refuseWork(ctx, ownerID, opened.Doc, opened.WorkItemID, "revise-failed", reason); err != nil {
		log.Printf("texture api: close work %s opened by a failed revise: %v", opened.WorkItemID, err)
	}
}

// refuseWork refuses one open Texture work item of the document's desk.
func (h *Handler) refuseWork(ctx context.Context, ownerID string, doc types.Document, workItemID, ref, reason string) error {
	req := types.RefuseLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: ref + ":" + workItemID,
		TrajectoryID: doc.TrajectoryID, WorkItemID: workItemID, ActingAgentID: currentTextureAgentID(doc.DocID),
		RefusalRef: ref + ":" + workItemID, Reason: reason,
	}
	var err error
	if req.CommandDigest, err = store.ComputeRefuseLifecycleWorkDigest(req); err != nil {
		return err
	}
	_, err = h.Store.RefuseLifecycleWork(asOwner(context.WithoutCancel(ctx)), req)
	return err
}

func shortCause(err error) string {
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(msg); len(r) > 160 {
		msg = string(r[:160]) + "..."
	}
	return msg
}
