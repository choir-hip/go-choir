package types

import (
	"fmt"
	"strings"
)

// A shots-owed record is the platform asking management to shoot and judge
// the host-built candidate of an accepted operation
// (docs/design/desk-shots-and-review-2026-11-07.md 3.2, 3.4). The gate commits
// it once, with the deterministic id shots-owed:<operation>:<host build job>,
// so a repeat is a replay. A content-identical record dedups into a consumed
// wake, so the record alone never re-wakes management; the capped notices do,
// each under a fresh id. All are system Notes from shots-owed:<operation>, so
// none opens or closes a round. The runtime mints them only from its live
// continuation, never at a boot.

const (
	// ShotsOwedAgentID is a shots-owed record's and notice's provenance.
	ShotsOwedAgentID = "system:shots-owed"
	// ShotsOwedMaxNotices bounds the notices, and so the re-wakes, per
	// operation. The gate's deadline reads it.
	ShotsOwedMaxNotices = 2
	shotsOwedPrefix     = "shots-owed:"
	shotsOwedNoticeMid  = ":notice:"
)

// ShotsOwedSenderInbox is the sender inbox of the records about one operation.
func ShotsOwedSenderInbox(operationID string) string {
	return shotsOwedPrefix + strings.TrimSpace(operationID)
}

// ShotsOwedRecordID is the id of the shots-owed record of an operation's host
// build job.
func ShotsOwedRecordID(operationID, hostBuildJobID string) string {
	return fmt.Sprintf("%s%s:%s", shotsOwedPrefix, strings.TrimSpace(operationID), strings.TrimSpace(hostBuildJobID))
}

// ShotsOwedNoticeID is the id of the nth notice about an operation's host
// build job.
func ShotsOwedNoticeID(operationID, hostBuildJobID string, n int) string {
	return fmt.Sprintf("%s%s%d", ShotsOwedRecordID(operationID, hostBuildJobID), shotsOwedNoticeMid, n)
}

// IsShotsOwedRecord reports whether rec is a shots-owed record or notice.
func IsShotsOwedRecord(rec CommitmentRecord) bool {
	sender := strings.TrimSpace(rec.SenderInbox)
	return rec.EmitterKind == CommitmentEmitterSystem && strings.TrimSpace(rec.Provenance.AgentID) == ShotsOwedAgentID &&
		strings.HasPrefix(sender, shotsOwedPrefix) && sender != shotsOwedPrefix &&
		strings.HasPrefix(rec.RecordID, shotsOwedPrefix) && !rec.Final && rec.Round == 0 &&
		rec.Directive != nil && rec.Directive.Subtype.IsNotice()
}

func newShotsOwedRecord(id, managementInbox, computerID, trajectoryID, operationID, hostBuildJobID, bundleDigest, planDigest, body string, notice int) CommitmentRecord {
	operationID = strings.TrimSpace(operationID)
	refs := map[string]string{
		"trajectory_id": strings.TrimSpace(trajectoryID), "operation_id": operationID, "host_build_job_id": strings.TrimSpace(hostBuildJobID),
		"bundle_digest": strings.TrimSpace(bundleDigest), "plan_sha256": strings.TrimSpace(planDigest),
	}
	if notice > 0 {
		refs["notice"] = fmt.Sprint(notice)
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    id,
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: ShotsOwedAgentID},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  computerID,
		Inbox:       strings.TrimSpace(managementInbox), SenderInbox: ShotsOwedSenderInbox(operationID), Addressee: strings.TrimSpace(managementInbox),
		Directive:   &CommitmentDirective{Subtype: CommitmentDirectiveNotice, Body: strings.TrimSpace(body)},
		SubjectRefs: refs,
	}
}

// NewShotsOwedRecord builds the record the gate commits once.
func NewShotsOwedRecord(managementInbox, computerID, trajectoryID, operationID, hostBuildJobID, bundleDigest, planDigest, body string) CommitmentRecord {
	return newShotsOwedRecord(ShotsOwedRecordID(operationID, hostBuildJobID), managementInbox, computerID, trajectoryID, operationID, hostBuildJobID, bundleDigest, planDigest, body, 0)
}

// NewShotsOwedNotice builds the nth notice. The id is deterministic in the
// operation, the job and n, so a retried commit replays, and differs from the
// record's, so it is a fresh wake.
func NewShotsOwedNotice(managementInbox, computerID, trajectoryID, operationID, hostBuildJobID, bundleDigest, planDigest string, n int, body string) CommitmentRecord {
	return newShotsOwedRecord(ShotsOwedNoticeID(operationID, hostBuildJobID, n), managementInbox, computerID, trajectoryID, operationID, hostBuildJobID, bundleDigest, planDigest, body, n)
}
