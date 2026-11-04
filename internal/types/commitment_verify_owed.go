package types

import (
	"fmt"
	"strings"
)

// A verify-owed notice is the runtime telling management that it owes the
// Verify of a frozen candidate (docs/problems/management-ends-without-verify-2026-11-04.md):
// an assignment management cast completed (or was retained at its build's
// end, or asked for review) with its operation frozen, management's
// activation ended without choir.Verify, and nothing else would wake it. It
// is a system Note from verify-owed:<assignment>, so it opens and closes no
// round, and it wakes management. The runtime mints at most
// VerifyOwedMaxNotices per operation, only from its live continuation, never
// at a boot.

const (
	// VerifyOwedAgentID is a notice's provenance.
	VerifyOwedAgentID = "system:verify-owed"
	// VerifyOwedMaxNotices bounds the notices, and so the re-wakes, per
	// operation.
	VerifyOwedMaxNotices = 2
	verifyOwedPrefix     = "verify-owed:"
)

// VerifyOwedSenderInbox is the sender inbox of the notices about one
// assignment: verify-owed:<assignment>.
func VerifyOwedSenderInbox(assignmentID string) string {
	return verifyOwedPrefix + strings.TrimSpace(assignmentID)
}

// VerifyOwedNoticeID is the id of the nth notice about operationID.
func VerifyOwedNoticeID(operationID string, n int) string {
	return fmt.Sprintf("%s%s:%d", verifyOwedPrefix, strings.TrimSpace(operationID), n)
}

// IsVerifyOwedNotice reports whether rec is a verify-owed notice: a system
// Note from verify-owed:<assignment> with a verify-owed record id. An
// agent's record is always EmitterAgent from its own inbox, so none passes.
func IsVerifyOwedNotice(rec CommitmentRecord) bool {
	sender := strings.TrimSpace(rec.SenderInbox)
	return rec.EmitterKind == CommitmentEmitterSystem && strings.TrimSpace(rec.Provenance.AgentID) == VerifyOwedAgentID &&
		strings.HasPrefix(sender, verifyOwedPrefix) && sender != verifyOwedPrefix &&
		strings.HasPrefix(rec.RecordID, verifyOwedPrefix) && !rec.Final && rec.Round == 0 &&
		rec.Directive != nil && rec.Directive.Subtype.IsNotice()
}

// NewVerifyOwedNotice builds the nth notice for managementInbox about the
// frozen operation of assignmentID. The id is deterministic in the operation
// and n, so a retried commit replays.
func NewVerifyOwedNotice(managementInbox, computerID, trajectoryID, assignmentID, operationID, bundleDigest string, n int, body string) CommitmentRecord {
	assignmentID, operationID = strings.TrimSpace(assignmentID), strings.TrimSpace(operationID)
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    VerifyOwedNoticeID(operationID, n),
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: VerifyOwedAgentID},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  computerID,
		Inbox:       strings.TrimSpace(managementInbox), SenderInbox: VerifyOwedSenderInbox(assignmentID), Addressee: strings.TrimSpace(managementInbox),
		Directive: &CommitmentDirective{Subtype: CommitmentDirectiveNotice, Body: strings.TrimSpace(body)},
		SubjectRefs: map[string]string{
			"trajectory_id": strings.TrimSpace(trajectoryID), "assignment_id": assignmentID, "operation_id": operationID,
			"bundle_digest": strings.TrimSpace(bundleDigest), "notice": fmt.Sprint(n),
		},
	}
}
