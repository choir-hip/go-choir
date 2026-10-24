package types

import "strings"

// An engineering progress relay is the platform telling a Texture what
// engineering reported (docs/problems/engineering-reports-never-reach-texture-2026-10-24.md).
// Engineering addresses management only; its progress Report is a record in
// management's inbox. With the computer setting engineering.progress_to_texture
// on, the same transaction also mints this record into the inbox of the
// Texture whose request the assignment serves. It is a system Note, not a
// desk's speech: sent from progress:<assignment>, never from engineering or
// management, so it opens and closes no round, and it is a directive, so no
// open-claim projection counts it. Texture stays the only writer of the
// document and decides what the record changes.

const (
	// ProgressRelayAgentID is a relay record's provenance.
	ProgressRelayAgentID = "system:progress-relay"
	progressSenderDesk   = "progress"
	progressRecordPrefix = "progress:"
)

// ProgressSenderInbox is the sender inbox of the relay of one engineering
// assignment's progress: progress:<assignment id>.
func ProgressSenderInbox(engineeringInbox string) string {
	inbox := strings.TrimSpace(engineeringInbox)
	if _, subject, ok := strings.Cut(inbox, ":"); ok {
		inbox = subject
	}
	return progressSenderDesk + ":" + inbox
}

// ProgressRelayRecordID is the id of the relay of source record id: one
// relay per engineering Report, derived from the Report's own id so a
// replayed cell mints the same record.
func ProgressRelayRecordID(sourceRecordID string) string {
	return progressRecordPrefix + strings.TrimSpace(sourceRecordID)
}

// IsEngineeringProgressRelay reports whether rec is a progress relay: a
// system Note from progress:<assignment>.
func IsEngineeringProgressRelay(rec CommitmentRecord) bool {
	return rec.EmitterKind == CommitmentEmitterSystem &&
		strings.HasPrefix(strings.TrimSpace(rec.SenderInbox), progressRecordPrefix) &&
		strings.TrimSpace(rec.SenderInbox) != progressRecordPrefix &&
		rec.Directive != nil && rec.Directive.Subtype == CommitmentDirectiveNote
}

// NewEngineeringProgressRelay builds the relay of one engineering progress
// Report for the Texture inbox the assignment serves. It returns false when
// the Report carries nothing to relay (no packet and no claim text).
func NewEngineeringProgressRelay(source CommitmentRecord, textureInbox string) (CommitmentRecord, bool) {
	textureInbox = strings.TrimSpace(textureInbox)
	body := strings.TrimSpace(source.Prediction.Hypothesis)
	if body == "" || !strings.HasPrefix(textureInbox, "texture:") || strings.TrimSpace(source.RecordID) == "" ||
		!strings.HasPrefix(strings.TrimSpace(source.SenderInbox), "engineering:") {
		return CommitmentRecord{}, false
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    ProgressRelayRecordID(source.RecordID),
		Kind:        CommitmentKindDirective,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: ProgressRelayAgentID, CommittedAt: source.Provenance.CommittedAt},
		EmitterKind: CommitmentEmitterSystem,
		ComputerID:  source.ComputerID,
		Inbox:       textureInbox, SenderInbox: ProgressSenderInbox(source.SenderInbox), Addressee: textureInbox,
		Directive:    &CommitmentDirective{Subtype: CommitmentDirectiveNote, Body: body},
		RelatedIDs:   []string{strings.TrimSpace(source.RecordID)},
		EvidenceRefs: append([]string(nil), source.EvidenceRefs...),
		SubjectRefs:  map[string]string{"assignment_inbox": strings.TrimSpace(source.SenderInbox), "relay_of": strings.TrimSpace(source.RecordID)},
	}, true
}

// ConversationCounterpart returns the inbox other than self in a
// conversation id (InboxConversationID), or false when self is not one of
// its two inboxes. An engineering assignment's parent conversation is
// management's with the Texture whose request it serves, so the
// counterpart of the parent agent is that Texture's inbox.
func ConversationCounterpart(conversationID, self string) (string, bool) {
	return conversationCounterpart(strings.TrimSpace(conversationID), strings.TrimSpace(self))
}
