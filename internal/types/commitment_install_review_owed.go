package types

import (
	"fmt"
	"strings"
)

// An install-review-owed record is the runtime asking management to judge a
// widened-ceiling install (docs/design/desk-install-review-2026-11-08.md 2.1,
// 2.2). It is committed once under a deterministic id, so a repeat is a
// replay, and the commit itself mints its re-entry wakes (the clock: notices
// at most InstallReviewOwedMaxNotices times, then the 45 minute close). A
// content-identical record dedups into a consumed wake, so the capped notices
// each carry a fresh id. All are system Notes from install-review-owed:<release>,
// so none opens or closes a round. The runtime mints them only from a live
// request or its own continuation, never at a boot.

const (
	// InstallReviewOwedAgentID is an owed record's and notice's provenance.
	InstallReviewOwedAgentID = "system:install-review-owed"
	// InstallReviewOwedMaxNotices bounds the notices, and so the re-wakes,
	// per owed review.
	InstallReviewOwedMaxNotices = 2

	installReviewOwedPrefix    = "install-review-owed:"
	installReviewOwedNoticeMid = ":notice:"

	// InstallReviewPathInstall is an owner or update install (a pending row
	// holds the request); InstallReviewPathOperation is a package
	// self-development operation (the operation is the pending act).
	InstallReviewPathInstall   = "install"
	InstallReviewPathOperation = "operation"
)

// InstallReviewOwedSenderInbox is the sender inbox of the owed records about
// one release; InstallPacket finds an owed review by it.
func InstallReviewOwedSenderInbox(releaseID string) string {
	return installReviewOwedPrefix + strings.TrimSpace(releaseID)
}

// InstallReviewOwedRecordID is the id of the owed record of one review: the
// release, the subject digest, and the attempt (the first attempt carries no
// suffix; a review opened again after a timeout is attempt 2, and so on).
//
// The id is scoped by path: an owner or update review (operationID "") and a
// package self-development operation's review of the same release and subject
// are different reviews with different clocks, so the operation's id carries
// ":op:<operation id>" and the two can never collide (build review r1, item 1).
func InstallReviewOwedRecordID(releaseID, subjectDigest, operationID string, attempt int) string {
	id := fmt.Sprintf("%s%s:%s", installReviewOwedPrefix, strings.TrimSpace(releaseID), strings.TrimSpace(subjectDigest))
	if op := strings.TrimSpace(operationID); op != "" {
		id += ":op:" + op
	}
	if attempt > 1 {
		id += fmt.Sprintf(":a%d", attempt)
	}
	return id
}

// InstallReviewOwedNoticeID is the id of the nth notice of an owed review.
func InstallReviewOwedNoticeID(releaseID, subjectDigest, operationID string, attempt, n int) string {
	return fmt.Sprintf("%s%s%d", InstallReviewOwedRecordID(releaseID, subjectDigest, operationID, attempt), installReviewOwedNoticeMid, n)
}

// IsInstallReviewOwedRecord reports whether rec is an install-review-owed
// record or notice.
func IsInstallReviewOwedRecord(rec CommitmentRecord) bool {
	sender := strings.TrimSpace(rec.SenderInbox)
	return rec.EmitterKind == CommitmentEmitterSystem && strings.TrimSpace(rec.Provenance.AgentID) == InstallReviewOwedAgentID &&
		strings.HasPrefix(sender, installReviewOwedPrefix) && sender != installReviewOwedPrefix &&
		strings.HasPrefix(rec.RecordID, installReviewOwedPrefix) && !rec.Final && rec.Round == 0 &&
		rec.Directive != nil && rec.Directive.Subtype.IsNotice()
}

// InstallReviewOwedFacts are the SubjectRefs an owed record carries.
type InstallReviewOwedFacts struct {
	ReleaseID, SubjectDigest, Package, Path, OperationID string
	// PatchSHA256, ManifestSHA256 and Surface are the subject's other bindings,
	// so a verdict's replay can be matched without rebuilding the release.
	PatchSHA256, ManifestSHA256, Surface string
	Attempt, Notice                      int
}

func newInstallReviewOwedRecord(id, managementInbox, computerID string, f InstallReviewOwedFacts, body string) CommitmentRecord {
	refs := map[string]string{
		"release_id": strings.TrimSpace(f.ReleaseID), "subject_digest": strings.TrimSpace(f.SubjectDigest), "package": strings.TrimSpace(f.Package),
		"path": f.Path, "attempt": fmt.Sprint(f.Attempt),
		"patch_sha256": f.PatchSHA256, "manifest_sha256": f.ManifestSHA256, "behaviour_surface": f.Surface,
	}
	if f.OperationID != "" {
		refs["operation_id"] = strings.TrimSpace(f.OperationID)
	}
	if f.Notice > 0 {
		refs["notice"] = fmt.Sprint(f.Notice)
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    id,
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: InstallReviewOwedAgentID},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  computerID,
		Inbox:       strings.TrimSpace(managementInbox), SenderInbox: InstallReviewOwedSenderInbox(f.ReleaseID), Addressee: strings.TrimSpace(managementInbox),
		Directive:   &CommitmentDirective{Subtype: CommitmentDirectiveNotice, Body: strings.TrimSpace(body)},
		SubjectRefs: refs,
	}
}

// NewInstallReviewOwedRecord builds the record committed once per review.
func NewInstallReviewOwedRecord(managementInbox, computerID string, f InstallReviewOwedFacts, body string) CommitmentRecord {
	f.Notice = 0
	return newInstallReviewOwedRecord(InstallReviewOwedRecordID(f.ReleaseID, f.SubjectDigest, f.OperationID, f.Attempt), managementInbox, computerID, f, body)
}

// NewInstallReviewOwedNotice builds the nth notice (f.Notice = n). Its id is
// deterministic, so a retried commit replays, and differs from the record's,
// so it is a fresh wake.
func NewInstallReviewOwedNotice(managementInbox, computerID string, f InstallReviewOwedFacts, body string) CommitmentRecord {
	return newInstallReviewOwedRecord(InstallReviewOwedNoticeID(f.ReleaseID, f.SubjectDigest, f.OperationID, f.Attempt, f.Notice), managementInbox, computerID, f, body)
}
