package types

import "strings"

// An owner-act record tells a Texture that the self-development candidate
// its document supervises was approved, applied or rejected
// (docs/problems/owner-acts-on-a-candidate-do-not-wake-texture-2026-10-25.md).
// The decision is the owner's (or a consensus decision's) and is already on
// the tape; this record is its projection into the inbox of the one desk
// that renders the document. It has the engineering progress relay's shape:
// a system Note from a non-desk sender, selfdev:<operation>, so it opens
// and closes no round and is no desk's speech, and it wakes its reader.
// Texture stays the only writer of the document and decides what the
// record changes.

const (
	// OwnerActAgentID is an owner-act record's provenance.
	OwnerActAgentID = "system:owner-act"

	// The acts an owner-act record names.
	OwnerActApprove = "approve"
	OwnerActApply   = "apply"
	OwnerActReject  = "reject"

	ownerActSenderDesk   = "selfdev"
	ownerActRecordPrefix = "owner-act:"
)

// OwnerActSenderInbox is the sender inbox of a candidate's owner-act
// records: selfdev:<operation id>.
func OwnerActSenderInbox(operationID string) string {
	return ownerActSenderDesk + ":" + strings.TrimSpace(operationID)
}

// OwnerActRecordID is the id of the one record of one act on one
// candidate, so a replayed act mints the same record.
func OwnerActRecordID(operationID, act string) string {
	return ownerActRecordPrefix + strings.TrimSpace(operationID) + ":" + strings.TrimSpace(act)
}

// IsOwnerAct reports whether act is one an owner-act record may name.
func IsOwnerAct(act string) bool {
	switch act {
	case OwnerActApprove, OwnerActApply, OwnerActReject:
		return true
	}
	return false
}

// IsSelfDevelopmentOwnerAct reports whether rec is an owner-act record: a
// system Note from selfdev:<operation> whose id and subject refs name the
// operation and the act.
func IsSelfDevelopmentOwnerAct(rec CommitmentRecord) bool {
	operationID := strings.TrimSpace(rec.SubjectRefs["operation_id"])
	act := strings.TrimSpace(rec.SubjectRefs["owner_act"])
	return rec.EmitterKind == CommitmentEmitterSystem && operationID != "" && IsOwnerAct(act) &&
		strings.TrimSpace(rec.SenderInbox) == OwnerActSenderInbox(operationID) &&
		strings.TrimSpace(rec.RecordID) == OwnerActRecordID(operationID, act) &&
		rec.Directive != nil && rec.Directive.Subtype == CommitmentDirectiveNote
}

// SelfDevelopmentOwnerAct is one act on one candidate, addressed to the
// Texture of the document its trajectory binds.
type SelfDevelopmentOwnerAct struct {
	TextureInbox  string
	ComputerID    string
	OperationID   string
	TrajectoryID  string
	Act           string
	DecisionEvent string
	Body          string
}

// NewSelfDevelopmentOwnerAct builds the owner-act record of act. It returns
// false when the act does not bind (no Texture inbox, no operation, an
// unknown act, no text).
func NewSelfDevelopmentOwnerAct(act SelfDevelopmentOwnerAct) (CommitmentRecord, bool) {
	inbox := strings.TrimSpace(act.TextureInbox)
	operationID := strings.TrimSpace(act.OperationID)
	kind := strings.TrimSpace(act.Act)
	body := strings.TrimSpace(act.Body)
	if !strings.HasPrefix(inbox, "texture:") || inbox == "texture:" || operationID == "" || !IsOwnerAct(kind) || body == "" {
		return CommitmentRecord{}, false
	}
	refs := map[string]string{"operation_id": operationID, "owner_act": kind}
	if trajectoryID := strings.TrimSpace(act.TrajectoryID); trajectoryID != "" {
		refs["trajectory_id"] = trajectoryID
	}
	if decision := strings.TrimSpace(act.DecisionEvent); decision != "" {
		refs["decision_event"] = decision
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    OwnerActRecordID(operationID, kind),
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: OwnerActAgentID},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  strings.TrimSpace(act.ComputerID),
		Inbox:       inbox, SenderInbox: OwnerActSenderInbox(operationID), Addressee: inbox,
		Directive:    &CommitmentDirective{Subtype: CommitmentDirectiveNote, Body: body},
		EvidenceRefs: []string{operationID},
		SubjectRefs:  refs,
	}, true
}
