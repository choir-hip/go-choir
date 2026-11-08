package agentcore

import (
	"context"
	"errors"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
// operation's trajectory names, wherever that document is now (problem 21
// follow-up: an owner revision on a settled document continues it onto a new
// trajectory, and the records for later acts on the operation went nowhere),
// or false when there is none: a trajectory that is not a lifecycle
// trajectory (rollback and genesis operations generate theirs), one that
// names no document, or a document gone, archived or bound to no trajectory.
func (rt *Runtime) ownerActTextureInbox(ctx context.Context, ownerID, computerID string, operation selfdev.Operation) (string, bool, error) {
	doc, why, err := rt.operationDocument(ctx, ownerID, computerID, operation)
	if err != nil || why != "" {
		return "", false, err
	}
	return currentTextureAgentID(doc.DocID), true, nil
}

// operationDocument is the document the operation's trajectory names, read as
// it is now: possibly on a newer trajectory than the operation's own. why is
// non-empty, and says what is missing, when there is no usable document. The
// rebase notice's fallback and the owner-act records both resolve through it.
func (rt *Runtime) operationDocument(ctx context.Context, ownerID, computerID string, operation selfdev.Operation) (types.Document, string, error) {
	docID, err := rt.operationDocumentID(ctx, ownerID, computerID, operation)
	if err != nil {
		return types.Document{}, "", err
	}
	if docID == "" {
		return types.Document{}, "its trajectory " + strings.TrimSpace(operation.TrajectoryID) + " names no document", nil
	}
	// The owner act names the operation and the operation names this
	// document: the read is the document custodian's (enforce flip 6.4), for
	// this one read.
	doc, err := rt.store.GetLifecycleDocument(objectgraph.WithCustodianReader(ctx, "choir.texture_document"), ownerID, computerID, docID)
	if errors.Is(err, store.ErrNotFound) {
		return types.Document{}, "its document " + docID + " is not on the tape", nil
	}
	if err != nil {
		return types.Document{}, "", fmt.Errorf("document %s: %w", docID, err)
	}
	if doc.ArchivedAt != nil {
		return types.Document{}, "its document " + docID + " is archived", nil
	}
	if strings.TrimSpace(doc.TrajectoryID) == "" {
		return types.Document{}, "its document " + docID + " is bound to no trajectory", nil
	}
	return doc, "", nil
}

// operationDocumentID is the id of the document the operation's trajectory
// names in its subject refs (doc_id, or a texture://documents/ artifact), or ""
// when the trajectory is not a lifecycle trajectory or names none. It does not
// read the document: the document may since have moved to a newer trajectory.
func (rt *Runtime) operationDocumentID(ctx context.Context, ownerID, computerID string, operation selfdev.Operation) (string, error) {
	trajectoryID := strings.TrimSpace(operation.TrajectoryID)
	if trajectoryID == "" {
		return "", nil
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("trajectory %s: %w", trajectoryID, err)
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		if artifact := strings.TrimSpace(trajectory.SubjectRefs["artifact"]); strings.HasPrefix(artifact, "texture://documents/") {
			docID = strings.TrimSpace(strings.TrimPrefix(artifact, "texture://documents/"))
		}
	}
	return docID, nil
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
		if who != "The owner" {
			// R3: a consensus apply has no owner approval to answer for it, so
			// the record says what decided it and how the owner undoes it. The
			// Revert action on the candidate's embed calls the same route.
			return fmt.Sprintf("Self-development candidate %s is applied on a consensus decision (%s), with no owner approval: the computer now runs it. The owner can undo it with Revert on the candidate in this document (POST /api/computers/%s/self-development/operations/%s/revert); reverting an earlier candidate also undoes every later one.", id, strings.TrimSpace(operation.DecisionEvent), strings.TrimSpace(operation.ComputerID), id)
		}
		return fmt.Sprintf("Self-development candidate %s is applied: the computer now runs it, after it was approved.", id)
	case types.OwnerActReject:
		body := fmt.Sprintf("%s rejected self-development candidate %s. It will not be applied.", who, id)
		if reason = strings.TrimSpace(reason); reason != "" {
			body += " The reason, verbatim:\n\n" + reason
		}
		return body
	case types.OwnerActFail:
		after := " after its decision"
		if strings.TrimSpace(operation.DecisionEvent) == "" {
			after = " before a decision"
		}
		body := fmt.Sprintf("Self-development candidate %s failed%s: it was not applied, and the computer keeps the release it had.", id, after)
		if cause := strings.TrimSpace(operation.TerminalError); cause != "" {
			body += " The cause:\n\n" + cause
		}
		return body
	case types.OwnerActAbandon:
		// Abandon is only ever the owner's act. A degraded candidate had
		// installed a release: that release stays live, and only the
		// unsettled tail is closed.
		body := fmt.Sprintf("The owner abandoned self-development candidate %s; it is now %s. Nothing further is applied from it; a release it had already installed stays live.", id, operation.State)
		if reason = strings.TrimSpace(reason); reason != "" {
			body += " The reason, verbatim:\n\n" + reason
		}
		return body
	}
	return ""
}

// wakeTextureForOwnerAct mints the owner-act record of act on operation
// into its document's Texture inbox. It reports whether a record was
// committed or replayed; an operation with no bound document, or a runtime
// without a store, mints nothing and is no error. There is no switch (fix 13,
// docs/problems/selfdev-round-blockers-cluster-2026-11-03.md: with the old
// texture.owner_act_wakes off by default, Texture told the owner to apply a
// candidate already applied and called a rejected one verified). The command
// id is the record's, so the same act commits once.
func (rt *Runtime) wakeTextureForOwnerAct(ctx context.Context, ownerID string, operation selfdev.Operation, act, reason string) (bool, error) {
	if rt == nil || rt.store == nil || !types.IsOwnerAct(act) || strings.TrimSpace(operation.OperationID) == "" {
		return false, nil
	}
	ownerID = firstNonEmpty(strings.TrimSpace(ownerID), strings.TrimSpace(rt.selfdevRouteOwnerID))
	computerID := firstNonEmpty(strings.TrimSpace(rt.TextureComputerID()), strings.TrimSpace(operation.ComputerID))
	if ownerID == "" || computerID == "" {
		return false, nil
	}
	inbox, bound, err := rt.ownerActTextureInbox(ctx, ownerID, computerID, operation)
	if err != nil || !bound {
		return false, err
	}
	rec, ok := types.NewSelfDevelopmentOwnerAct(types.SelfDevelopmentOwnerAct{
		TextureInbox: inbox, ComputerID: computerID, OperationID: operation.OperationID,
		TrajectoryID: operation.TrajectoryID, Act: act, DecisionEvent: operation.DecisionEvent,
		Body: ownerActBody(ownerID, operation, act, reason) + rt.rebaseOwnerActSentence(operation, act) + rt.roundLineSentence(ctx, ownerID, computerID, operation, act),
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
	// The round's predictions the act decides first
	// (engineering_round_settlement.go), so the receipt states them against
	// what happened; then the receipt, so Texture's record carries its line
	// (round_receipt.go).
	rt.settleOwnerActPrecommits(ctx, ownerID, operation, act)
	rt.recordRoundReceipt(ctx, ownerID, operation, act)
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

// notifyTextureOfApplyFailure wakes the candidate's Texture when a live apply
// left the operation failed. It reads the operation back, so it mints only
// for the outcome this pass recorded; the crash boot's journal-only
// settlement never calls it (a restart never starts a Texture turn).
func (rt *Runtime) notifyTextureOfApplyFailure(ctx context.Context, operation selfdev.Operation) {
	if rt == nil || rt.selfdevOperations == nil || operation.State == selfdev.StateFailed {
		return
	}
	// The outcome is already durable: a cancel of the pass's context must
	// not drop the record that says so.
	ctx = context.WithoutCancel(ctx)
	current, err := rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
	if err != nil {
		log.Printf("selfdev owner act: fail %s: read the operation back: %v", operation.OperationID, err)
		return
	}
	if current.State != selfdev.StateFailed {
		return
	}
	rt.notifyTextureOfOwnerAct(ctx, rt.selfdevRouteOwnerID, current, types.OwnerActFail, "")
}
