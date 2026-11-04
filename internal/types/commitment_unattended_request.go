package types

import "strings"

// An unattended-request notice is the runtime telling management that a
// Texture desk left the owner's request open with no next actor
// (docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md):
// the desk's activation ended with no Cast, no Wait naming what it waits on
// and no Done, and nothing else (a round Texture cast, an engineering
// assignment, a self-development operation) works on the request. The
// runtime refuses the request's work item with UnattendedRequestReason, and
// this system Note puts the same fact in management's inbox. It is sent from
// unattended:<work item>, never from a desk, so it opens and closes no round,
// and it wakes no one: management reads it at its next turn.

const (
	// UnattendedRequestAgentID is a notice's provenance.
	UnattendedRequestAgentID = "system:unattended-request"
	unattendedSenderDesk     = "unattended"
	unattendedRecordPrefix   = "unattended:"
)

// UnattendedRequestSenderInbox is the sender inbox of the notice about one
// work item: unattended:<work item id>.
func UnattendedRequestSenderInbox(workItemID string) string {
	return unattendedSenderDesk + ":" + strings.TrimSpace(workItemID)
}

// IsUnattendedRequestNotice reports whether rec is an unattended-request
// notice: a system Note from unattended:<work item>.
func IsUnattendedRequestNotice(rec CommitmentRecord) bool {
	sender := strings.TrimSpace(rec.SenderInbox)
	return rec.EmitterKind == CommitmentEmitterSystem &&
		strings.HasPrefix(sender, unattendedRecordPrefix) && sender != unattendedRecordPrefix &&
		rec.Directive != nil && rec.Directive.Subtype.IsNotice()
}

// NewUnattendedRequestNotice builds the notice for managementInbox about the
// Texture work item workItemID on document docID, left by run runID. The
// record id is deterministic in the work item and the run, so a retried
// commit replays.
func NewUnattendedRequestNotice(managementInbox, computerID, docID, trajectoryID, workItemID, runID, body string) CommitmentRecord {
	workItemID, runID = strings.TrimSpace(workItemID), strings.TrimSpace(runID)
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    unattendedRecordPrefix + workItemID + ":" + runID,
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: UnattendedRequestAgentID},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  computerID,
		Inbox:       strings.TrimSpace(managementInbox), SenderInbox: UnattendedRequestSenderInbox(workItemID), Addressee: strings.TrimSpace(managementInbox),
		Directive:   &CommitmentDirective{Subtype: CommitmentDirectiveNotice, Body: strings.TrimSpace(body)},
		SubjectRefs: map[string]string{"doc_id": strings.TrimSpace(docID), "trajectory_id": strings.TrimSpace(trajectoryID), "work_item_id": workItemID, "run_id": runID},
	}
}
