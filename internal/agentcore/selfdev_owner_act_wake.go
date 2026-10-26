package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Owner acts on a self-development candidate wake the Texture of the
// document that supervises it
// (docs/problems/owner-acts-on-a-candidate-do-not-wake-texture-2026-10-25.md).
// The act is already durable (the decision event, the applied event, the
// operation row); this mints one owner-act record into texture:<doc>, the
// engineering progress relay's shape, so the inbox commit's own wake row
// starts a Texture records turn with the act as its cause. The decision
// path never writes document text: Texture stays the only writer.
//
// Callers are live acts only: the decision request (owner or consensus)
// and a live apply. A crash boot's settlement from the updater journal and
// the reconciler's boot-time decision recovery mint nothing, so a restart
// never starts a Texture turn. A record minted before a crash whose wake
// was still pending is closed by the generic crash-boot wake fate.

// ownerActTextureInbox resolves the Texture inbox of the document the
// operation's trajectory binds, or false when it binds none: a trajectory
// that is not a lifecycle trajectory (rollback and genesis operations
// generate theirs), one without a document, a document bound to another
// trajectory, or an archived document.
func (rt *Runtime) ownerActTextureInbox(ctx context.Context, ownerID, computerID string, operation selfdev.Operation) (string, bool, error) {
	trajectoryID := strings.TrimSpace(operation.TrajectoryID)
	if trajectoryID == "" {
		return "", false, nil
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(err, store.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("trajectory %s: %w", trajectoryID, err)
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		if artifact := strings.TrimSpace(trajectory.SubjectRefs["artifact"]); strings.HasPrefix(artifact, "texture://documents/") {
			docID = strings.TrimSpace(strings.TrimPrefix(artifact, "texture://documents/"))
		}
	}
	if docID == "" {
		return "", false, nil
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	if errors.Is(err, store.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("document %s: %w", docID, err)
	}
	if strings.TrimSpace(doc.TrajectoryID) != trajectoryID || doc.ArchivedAt != nil {
		return "", false, nil
	}
	return currentTextureAgentID(docID), true, nil
}

// ownerActBody is the text Texture reads. It is derived only from the act
// and the operation's settled fields, so a replay commits the same record.
func ownerActBody(ownerID string, operation selfdev.Operation, act, reason string) string {
	who := "The owner"
	if actor := strings.TrimSpace(operation.DecisionActor); actor != "" && actor != strings.TrimSpace(ownerID) {
		who = "A consensus decision"
	}
	id := strings.TrimSpace(operation.OperationID)
	switch act {
	case types.OwnerActApprove:
		return fmt.Sprintf("%s approved self-development candidate %s. It is accepted and the computer applies it; a separate record says when it is applied or fails.", who, id)
	case types.OwnerActApply:
		return fmt.Sprintf("Self-development candidate %s is applied: the computer now runs it, after it was approved.", id)
	case types.OwnerActReject:
		body := fmt.Sprintf("%s rejected self-development candidate %s. It will not be applied.", who, id)
		if reason = strings.TrimSpace(reason); reason != "" {
			body += " The reason, verbatim:\n\n" + reason
		}
		return body
	}
	return ""
}

// wakeTextureForOwnerAct mints the owner-act record of act on operation
// into its document's Texture inbox. It reports whether a record was
// committed or replayed; an operation with no bound document, an off
// switch, or a runtime without a store mints nothing and is no error. The
// command id is the record's, so the same act commits once.
func (rt *Runtime) wakeTextureForOwnerAct(ctx context.Context, ownerID string, operation selfdev.Operation, act, reason string) (bool, error) {
	if rt == nil || rt.store == nil || !types.IsOwnerAct(act) || strings.TrimSpace(operation.OperationID) == "" {
		return false, nil
	}
	ownerID = firstNonEmpty(strings.TrimSpace(ownerID), strings.TrimSpace(rt.selfdevRouteOwnerID))
	computerID := firstNonEmpty(strings.TrimSpace(rt.TextureComputerID()), strings.TrimSpace(operation.ComputerID))
	if ownerID == "" || computerID == "" {
		return false, nil
	}
	if !rt.computerSettingOn(ctx, ownerID, computerID, store.SettingTextureOwnerActWakes) {
		return false, nil
	}
	inbox, bound, err := rt.ownerActTextureInbox(ctx, ownerID, computerID, operation)
	if err != nil || !bound {
		return false, err
	}
	rec, ok := types.NewSelfDevelopmentOwnerAct(types.SelfDevelopmentOwnerAct{
		TextureInbox: inbox, ComputerID: computerID, OperationID: operation.OperationID,
		TrajectoryID: operation.TrajectoryID, Act: act, DecisionEvent: operation.DecisionEvent,
		Body: ownerActBody(ownerID, operation, act, reason) + rt.rebaseOwnerActSentence(operation, act),
	})
	if !ok {
		return false, nil
	}
	req := types.CommitInboxRecordsRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec},
	}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return false, err
	}
	req.CommandDigest = digest
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return false, fmt.Errorf("commit %s: %w", rec.RecordID, err)
	}
	return true, nil
}

// notifyTextureOfOwnerAct is the call sites' form: the act already
// happened, so a failure to wake is logged and never fails the act.
func (rt *Runtime) notifyTextureOfOwnerAct(ctx context.Context, ownerID string, operation selfdev.Operation, act, reason string) {
	if _, err := rt.wakeTextureForOwnerAct(ctx, ownerID, operation, act, reason); err != nil {
		log.Printf("selfdev owner act: %s %s: Texture not woken: %v", act, operation.OperationID, err)
	}
}

// rebaseOwnerActSentence adds the lineage receipt's digest to the apply record
// of a rebase operation (U4); nothing for any other act or operation.
func (rt *Runtime) rebaseOwnerActSentence(operation selfdev.Operation, act string) string {
	if act != types.OwnerActApply {
		return ""
	}
	return rt.rebaseReceiptSentence(operation.OperationID)
}
