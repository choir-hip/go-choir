package types

import "time"

// Inbox work state (Cut 1, definitions/choir-reeng-cut1-authority artifacts
// 1 and 2). Records are the only authority. InboxHead and ConversationHead
// are caches of the fold over records, written only by the inbox record
// commit and rebuilt by the checker; a reader's InboxCursor is the one
// mutable fact a reader owns ("read through #57").

// InboxCursorAdvance moves one reader's cursor on one inbox by
// compare-and-set from ExpectedSeq to Seq, in the same transaction as the
// reader's own effects. Seq may not pass the inbox head after the commit.
type InboxCursorAdvance struct {
	ReaderAgentID string `json:"reader_agent_id"`
	Inbox         string `json:"inbox"`
	ExpectedSeq   int64  `json:"expected_seq"`
	Seq           int64  `json:"seq"`
}

// CommitInboxRecordsRequest commits addressed records and an optional
// cursor advance as one idempotent command.
type CommitInboxRecordsRequest struct {
	OwnerID       string              `json:"owner_id"`
	ComputerID    string              `json:"computer_id"`
	CommandID     string              `json:"command_id"`
	CommandDigest string              `json:"command_digest"`
	Records       []CommitmentRecord  `json:"records,omitempty"`
	Cursor        *InboxCursorAdvance `json:"cursor,omitempty"`
	// Judge and Verdicts are support verdicts, recorded by the runtime's
	// Jev job as a system act (support-judgement-to-jev-design-2026-10-14):
	// Judge is SupportSystemAgentID, the only judge the store accepts. Each
	// verdict is memoized by its pair key; a failing one reaches the pair's
	// texture:<doc> as a system record from Judge, committed with this
	// command.
	Judge    string           `json:"judge,omitempty"`
	Verdicts []SupportVerdict `json:"verdicts,omitempty"`
	// Tasks are the management task rows a delegation opens (management
	// delegation S2a): each commits with the Cast into its inbox, which
	// must be in Records and name the row's leased captures.
	Tasks []ManagementTask `json:"tasks,omitempty"`
	// StoryLibraryHead is the story-library head the committing run last
	// read with choir.Stories() (R2-5). When set, a Cast to texture:new is
	// refused if the library moved past it; nil skips the check (callers
	// that are not a management cell).
	StoryLibraryHead *int64 `json:"story_library_head,omitempty"`
}

// InboxRecordReceipt is what the store stamped on one committed record.
type InboxRecordReceipt struct {
	RecordID       string `json:"record_id"`
	Inbox          string `json:"inbox"`
	InboxSeq       int64  `json:"inbox_seq"`
	ConversationID string `json:"conversation_id"`
	Round          int    `json:"round"`
	OpenedRound    bool   `json:"opened_round,omitempty"`
	ClosedRound    bool   `json:"closed_round,omitempty"`
	// HypothesisIDs are the ids assigned to the Cast's new hypotheses, in
	// the order the Cast listed them.
	HypothesisIDs []string `json:"hypothesis_ids,omitempty"`
	// WakeID is the actor wake outbox row minted for the recipient; empty
	// for system records, which wake no one, except a support failure or a
	// capture arrival.
	WakeID string `json:"wake_id,omitempty"`
	// OpenedStory marks a Cast to StoryOpenInbox that opened a new story;
	// Inbox names the story's texture:<doc> (Cut 2 2b).
	OpenedStory bool `json:"opened_story,omitempty"`
}

// InboxCommitReceipt is the durable receipt of one inbox command; a replay
// of the same command id and digest returns it with Replay set.
type InboxCommitReceipt struct {
	CommandID     string               `json:"command_id,omitempty"`
	CommandDigest string               `json:"command_digest,omitempty"`
	Records       []InboxRecordReceipt `json:"records,omitempty"`
	Cursor        *InboxCursorAdvance  `json:"cursor,omitempty"`
	// Verdicts are the verdicts this command recorded, each with its pair
	// and, for a failure, the record it sent; a memo hit is not listed.
	Verdicts []SupportVerdict `json:"verdicts,omitempty"`
	// StoryLibraryHead is the story-library head after this command's
	// opens, set only when the command carried a head and the check passed
	// (R2-5): the committing run has then seen every open up to it, its own
	// included (management delegation S2b, FM28).
	StoryLibraryHead int64     `json:"story_library_head,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	Replay           bool      `json:"replay,omitempty"`
}

// InboxHead is the cached last sequence number taken in one inbox.
type InboxHead struct {
	Inbox string `json:"inbox"`
	Seq   int64  `json:"seq"`
}

// InboxCursor is one reader's cursors, a map from inbox to the sequence
// read through.
type InboxCursor struct {
	ReaderAgentID string           `json:"reader_agent_id"`
	Cursors       map[string]int64 `json:"cursors"`
}

// HypothesisStatus is a hypothesis's state in the fold.
type HypothesisStatus string

const (
	HypothesisOpen      HypothesisStatus = "open"
	HypothesisSettled   HypothesisStatus = "settled"
	HypothesisDropped   HypothesisStatus = "dropped"
	HypothesisWithdrawn HypothesisStatus = "withdrawn"
)

// HypothesisState is one hypothesis as the fold over its conversation
// sees it. OpenedBy is the inbox that opened it; only that side revises
// or withdraws it, and only the other side settles or drops it.
type HypothesisState struct {
	ID          string           `json:"id"`
	OpenedBy    string           `json:"opened_by"`
	OpenedRound int              `json:"opened_round"`
	Statement   string           `json:"statement"`
	Probability float64          `json:"probability"`
	General     string           `json:"general,omitempty"`
	Status      HypothesisStatus `json:"status"`
	Outcome     string           `json:"outcome,omitempty"`
	Reason      string           `json:"reason,omitempty"`
}

// ConversationHead is the cached round state of one conversation. Round is
// the latest round number (0 before the first Cast), Open whether it is
// still open, and CasterInbox who opened it.
type ConversationHead struct {
	ConversationID  string                     `json:"conversation_id"`
	Round           int                        `json:"round"`
	Open            bool                       `json:"open"`
	CasterInbox     string                     `json:"caster_inbox,omitempty"`
	RecipientInbox  string                     `json:"recipient_inbox,omitempty"`
	HypothesisCount int                        `json:"hypothesis_count"`
	Hypotheses      map[string]HypothesisState `json:"hypotheses,omitempty"`
}

// InboxUnread is one inbox a reader has not read through.
type InboxUnread struct {
	Inbox  string `json:"inbox"`
	Cursor int64  `json:"cursor"`
	Head   int64  `json:"head"`
}
