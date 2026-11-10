package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The changelog event record (docs/design/autopaper-step0-contracts-2026-11-10.md
// contract 3, section 5.2, H1 notes 5.5). A changelog event is a silent system
// record on the recipient computer's ledger: emitter system, inbox
// changelog:<owner>, id changelog:<event_id>. It wakes no one: it is on no
// RecordWakesReader list and carries no other body.

// ChangelogKind is the closed enum of event kinds. Adding one is a contract
// change: add its row to changelogKindLevels.
type ChangelogKind string

const (
	ChangelogParentUsed        ChangelogKind = "parent_used"
	ChangelogOpedVerdict       ChangelogKind = "oped_verdict"
	ChangelogUpdateStarted     ChangelogKind = "update_started"
	ChangelogUpdateProgress    ChangelogKind = "update_progress"
	ChangelogUpdateFinished    ChangelogKind = "update_finished"
	ChangelogUpdateInterrupted ChangelogKind = "update_interrupted"
	ChangelogRevisionCommitted ChangelogKind = "revision_committed"
	ChangelogWakeDecision      ChangelogKind = "wake_decision"
	ChangelogOpedArrival       ChangelogKind = "oped_arrival"
	// ChangelogGateUnavailable: the op-ed gate gave up after its retries.
	ChangelogGateUnavailable ChangelogKind = "gate_unavailable"
)

// ChangelogLevel is major or minor; it is a property of the kind alone.
type ChangelogLevel string

const (
	ChangelogMajor ChangelogLevel = "major"
	ChangelogMinor ChangelogLevel = "minor"
)

var changelogKindLevels = map[ChangelogKind]ChangelogLevel{
	ChangelogParentUsed:        ChangelogMajor,
	ChangelogOpedVerdict:       ChangelogMajor,
	ChangelogUpdateFinished:    ChangelogMajor,
	ChangelogUpdateInterrupted: ChangelogMajor,
	ChangelogGateUnavailable:   ChangelogMajor,
	ChangelogUpdateStarted:     ChangelogMinor,
	ChangelogUpdateProgress:    ChangelogMinor,
	ChangelogRevisionCommitted: ChangelogMinor,
	ChangelogWakeDecision:      ChangelogMinor,
	ChangelogOpedArrival:       ChangelogMinor,
}

// ChangelogKinds lists every kind, in a stable order.
func ChangelogKinds() []ChangelogKind {
	return []ChangelogKind{ChangelogParentUsed, ChangelogOpedVerdict, ChangelogUpdateStarted, ChangelogUpdateProgress,
		ChangelogUpdateFinished, ChangelogUpdateInterrupted, ChangelogRevisionCommitted, ChangelogWakeDecision,
		ChangelogOpedArrival, ChangelogGateUnavailable}
}

// ChangelogLevelOf is the one function that sets a level: from the kind. A
// kind it does not know reads as minor with ok false (tolerated at read,
// shown only on zoom); commit refuses such a kind.
func ChangelogLevelOf(kind ChangelogKind) (level ChangelogLevel, ok bool) {
	if l, found := changelogKindLevels[kind]; found {
		return l, true
	}
	return ChangelogMinor, false
}

// Relations of a parent_used event (contract 3, 5.2).
const (
	ChangelogRelationForkPublished   = "fork_published"
	ChangelogRelationPaperResponded  = "paper_responded"
	ChangelogRelationTakenInto       = "taken_into"
	ChangelogRelationOpedOnYourStory = "oped_on_your_story"
)

// Origins of an event.
const (
	ChangelogOriginCorpusd = "corpusd"
	ChangelogOriginTape    = "tape"
	ChangelogOriginPaper   = "paper"
)

// Bounds on every free-text or reference field.
const (
	ChangelogSummaryMaxRunes = 200
	ChangelogReasonMaxRunes  = 1000
	ChangelogLabelMaxRunes   = 64
	ChangelogRefMaxRunes     = 256
	ChangelogRouteMaxRunes   = 512
)

// ChangelogSubject names what an event is about. It carries references only,
// never the content of a child.
type ChangelogSubject struct {
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	RoutePath            string `json:"route_path,omitempty"`
	ParentVersionID      string `json:"parent_publication_version_id,omitempty"`
	ParentRoutePath      string `json:"parent_route_path,omitempty"`
	DocID                string `json:"doc_id,omitempty"`
}

// ChangelogTapeRef anchors an event on the recipient's tape. InboxSeq is
// zero at emit (the store stamps the record's own); the local fields name
// the work the event summarizes.
type ChangelogTapeRef struct {
	ComputerID     string `json:"computer_id,omitempty"`
	RecordID       string `json:"record_id,omitempty"`
	InboxSeq       int64  `json:"inbox_seq,omitempty"`
	DocID          string `json:"doc_id,omitempty"`
	RevisionID     string `json:"revision_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Round          int    `json:"round,omitempty"`
}

// ChangelogSpan is the range of the computer's event chain
// (computerevent.Event.Sequence) an event summarizes.
type ChangelogSpan struct {
	FromSeq int64 `json:"from_seq"`
	ToSeq   int64 `json:"to_seq"`
}

// CommitmentChangelog is the body of a changelog event record.
type CommitmentChangelog struct {
	EventID    string           `json:"event_id"`
	Kind       ChangelogKind    `json:"kind"`
	Level      ChangelogLevel   `json:"level"`
	Relation   string           `json:"relation,omitempty"`
	OccurredAt string           `json:"occurred_at"`
	ActorLabel string           `json:"actor_label,omitempty"`
	Subject    ChangelogSubject `json:"subject"`
	Summary    string           `json:"summary"`
	// Reason is verdict reason text (Jev output). ReasonUntrusted must be
	// true whenever Reason is set: render it as text, never as instruction.
	Reason          string            `json:"reason,omitempty"`
	ReasonUntrusted bool              `json:"reason_untrusted,omitempty"`
	VerdictHash     string            `json:"verdict_hash,omitempty"`
	Origin          string            `json:"origin"`
	TapeRef         *ChangelogTapeRef `json:"tape_ref,omitempty"`
	Span            *ChangelogSpan    `json:"span,omitempty"`
	RemoteRef       string            `json:"remote_ref,omitempty"`
}

const (
	// ChangelogSystemAgentID is a changelog record's provenance.
	ChangelogSystemAgentID = "system:changelog"
	changelogRecordPrefix  = "changelog:"
)

// ChangelogInbox is the recipient inbox: changelog:<owner>.
func ChangelogInbox(ownerID string) string { return changelogRecordPrefix + strings.TrimSpace(ownerID) }

// IsChangelogInbox reports whether an inbox is a changelog inbox.
func IsChangelogInbox(inbox string) bool { return strings.HasPrefix(inbox, changelogRecordPrefix) }

// ChangelogSenderInbox is the sender inbox of a changelog record by origin.
func ChangelogSenderInbox(origin string) string {
	return "changelog-source:" + strings.TrimSpace(origin)
}

// ChangelogRecordID is the record id of an event: changelog:<event_id>.
func ChangelogRecordID(eventID string) string {
	return changelogRecordPrefix + strings.TrimSpace(eventID)
}

// ChangelogEventID is the idempotency key: SHA-256 over the kind and the
// given parts, each length-prefixed so no two part lists collide.
func ChangelogEventID(kind ChangelogKind, parts ...string) string {
	h := sha256.New()
	for _, p := range append([]string{string(kind)}, parts...) {
		p = strings.TrimSpace(p)
		h.Write([]byte(strconv.Itoa(len(p))))
		h.Write([]byte{':'})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// hasOtherBody reports whether rec carries any typed body besides the two
// H1 bodies. A changelog or op-ed record carries nothing else.
func hasOtherBody(rec CommitmentRecord) bool {
	return rec.Cast != nil || rec.Report != nil || rec.Support != nil || rec.Captures != nil || rec.Disposition != nil ||
		rec.TaskOutcome != nil || rec.Job != nil || rec.Calibration != nil || rec.Directive != nil || rec.SubCall != nil ||
		rec.Precommit != nil || rec.Resolve != nil || rec.Disagreement != nil
}

func boundedText(field, v string, max int) error {
	if !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
		return fmt.Errorf("%s is not valid text", field)
	}
	if n := utf8.RuneCountInString(v); n > max {
		return fmt.Errorf("%s has %d characters; the bound is %d", field, n, max)
	}
	return nil
}

type boundedField struct {
	name string
	v    string
	max  int
}

// ValidateChangelogBody checks a body alone.
func ValidateChangelogBody(b CommitmentChangelog) error {
	if strings.TrimSpace(b.EventID) == "" {
		return fmt.Errorf("changelog event needs an event_id")
	}
	want, ok := ChangelogLevelOf(b.Kind)
	if !ok {
		return fmt.Errorf("changelog kind %q is not a known kind", b.Kind)
	}
	if b.Level != want {
		return fmt.Errorf("changelog level %q does not match kind %q, which is %q", b.Level, b.Kind, want)
	}
	if b.Kind == ChangelogParentUsed {
		switch b.Relation {
		case ChangelogRelationForkPublished, ChangelogRelationPaperResponded, ChangelogRelationTakenInto, ChangelogRelationOpedOnYourStory:
		default:
			return fmt.Errorf("parent_used relation %q is not known", b.Relation)
		}
	} else if b.Relation != "" {
		return fmt.Errorf("only parent_used takes a relation")
	}
	switch b.Origin {
	case ChangelogOriginCorpusd, ChangelogOriginTape, ChangelogOriginPaper:
	default:
		return fmt.Errorf("changelog origin %q is not known", b.Origin)
	}
	if strings.TrimSpace(b.OccurredAt) == "" {
		return fmt.Errorf("changelog event needs occurred_at")
	}
	if strings.TrimSpace(b.Summary) == "" {
		return fmt.Errorf("changelog event needs a summary")
	}
	if b.Reason != "" && !b.ReasonUntrusted {
		return fmt.Errorf("changelog reason must be marked untrusted")
	}
	if b.Reason == "" && b.ReasonUntrusted {
		return fmt.Errorf("changelog reason_untrusted is set with no reason")
	}
	if b.Span != nil && (b.Span.FromSeq < 0 || b.Span.ToSeq < b.Span.FromSeq) {
		return fmt.Errorf("changelog span %d..%d is not a range", b.Span.FromSeq, b.Span.ToSeq)
	}
	checks := []boundedField{
		{"event_id", b.EventID, ChangelogRefMaxRunes}, {"occurred_at", b.OccurredAt, ChangelogLabelMaxRunes},
		{"actor_label", b.ActorLabel, ChangelogLabelMaxRunes}, {"summary", b.Summary, ChangelogSummaryMaxRunes},
		{"reason", b.Reason, ChangelogReasonMaxRunes}, {"verdict_hash", b.VerdictHash, ChangelogRefMaxRunes},
		{"remote_ref", b.RemoteRef, ChangelogRefMaxRunes},
		{"subject.publication_version_id", b.Subject.PublicationVersionID, ChangelogRefMaxRunes},
		{"subject.route_path", b.Subject.RoutePath, ChangelogRouteMaxRunes},
		{"subject.parent_publication_version_id", b.Subject.ParentVersionID, ChangelogRefMaxRunes},
		{"subject.parent_route_path", b.Subject.ParentRoutePath, ChangelogRouteMaxRunes},
		{"subject.doc_id", b.Subject.DocID, ChangelogRefMaxRunes},
	}
	if t := b.TapeRef; t != nil {
		checks = append(checks,
			boundedField{"tape_ref.computer_id", t.ComputerID, ChangelogRefMaxRunes},
			boundedField{"tape_ref.record_id", t.RecordID, ChangelogRefMaxRunes},
			boundedField{"tape_ref.doc_id", t.DocID, ChangelogRefMaxRunes},
			boundedField{"tape_ref.revision_id", t.RevisionID, ChangelogRefMaxRunes},
			boundedField{"tape_ref.conversation_id", t.ConversationID, ChangelogRefMaxRunes})
	}
	for _, c := range checks {
		if err := boundedText(c.name, c.v, c.max); err != nil {
			return err
		}
	}
	if b.Origin != ChangelogOriginCorpusd && b.TapeRef == nil {
		return fmt.Errorf("a %s changelog event needs a tape_ref", b.Origin)
	}
	if b.Origin == ChangelogOriginCorpusd && strings.TrimSpace(b.RemoteRef) == "" {
		return fmt.Errorf("a corpusd changelog event needs a remote_ref")
	}
	return nil
}

// ValidateChangelogSpanWithin refuses a span that points past the chain's
// head (the highest sequence on the computer's event chain).
func ValidateChangelogSpanWithin(span ChangelogSpan, headSeq int64) error {
	if span.FromSeq < 0 || span.ToSeq < span.FromSeq || span.ToSeq > headSeq {
		return fmt.Errorf("changelog span %d..%d is outside the chain, which ends at %d", span.FromSeq, span.ToSeq, headSeq)
	}
	return nil
}

// ValidateChangelogRecord checks a record that carries a Changelog body: a
// system record to changelog:<owner> with id changelog:<event_id> and no
// other body. A record without the body passes untouched.
func ValidateChangelogRecord(rec CommitmentRecord) error {
	if rec.Changelog == nil {
		return nil
	}
	if rec.Oped != nil || hasOtherBody(rec) || rec.Final {
		return fmt.Errorf("changelog record %s carries another body", rec.RecordID)
	}
	if rec.EmitterKind != CommitmentEmitterSystem || !IsChangelogInbox(rec.Inbox) || rec.Inbox == changelogRecordPrefix ||
		rec.RecordID != ChangelogRecordID(rec.Changelog.EventID) {
		return fmt.Errorf("changelog record %s must be a system record to changelog:<owner> with id changelog:<event_id>", rec.RecordID)
	}
	if err := ValidateChangelogBody(*rec.Changelog); err != nil {
		return fmt.Errorf("changelog record %s: %w", rec.RecordID, err)
	}
	return nil
}
