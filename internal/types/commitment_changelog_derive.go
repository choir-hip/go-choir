package types

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The local update_* derivation (contract 3, 5.2): a pure function from what
// a computer already has (the open round, its revisions, the final Report or
// the crash closure) to changelog events. It reads; it never closes a round.

// UpdateRevision is one revision committed during a round.
type UpdateRevision struct {
	RevisionID  string
	CommittedAt string
	// Seq is the revision's sequence on the computer's event chain, 0 if
	// unknown (the event then carries no span).
	Seq int64
}

// UpdateClose is how a round ended: its final Report (or the crash closure).
type UpdateClose struct {
	RecordID   string
	StopReason string
	ClosedAt   string
	Seq        int64
}

// UpdateRound is one document update: a round of the document's inbox.
type UpdateRound struct {
	DocID          string
	ConversationID string
	Round          int
	OpenedAt       string
	OpenSeq        int64
	Revisions      []UpdateRevision
	Close          *UpdateClose
}

func updateSpan(from, to int64) *ChangelogSpan {
	if from <= 0 || to < from {
		return nil
	}
	return &ChangelogSpan{FromSeq: from, ToSeq: to}
}

// DeriveUpdateEvents derives the events of one round: update_started, one
// revision_committed per revision, and, once the round has closed, exactly
// one of update_finished or update_interrupted (a final Report whose stop
// reason is interrupted_by_restart is interrupted, never finished). The ids
// are deterministic in (kind, doc, conversation, round[, revision]).
func DeriveUpdateEvents(r UpdateRound) []CommitmentChangelog {
	round := strconv.Itoa(r.Round)
	ref := func(revision string) *ChangelogTapeRef {
		return &ChangelogTapeRef{DocID: r.DocID, RevisionID: revision, ConversationID: r.ConversationID, Round: r.Round}
	}
	event := func(kind ChangelogKind, at, summary, revision string, span *ChangelogSpan, parts ...string) CommitmentChangelog {
		level, _ := ChangelogLevelOf(kind)
		return CommitmentChangelog{
			EventID: ChangelogEventID(kind, append([]string{r.DocID, r.ConversationID, round}, parts...)...), Kind: kind, Level: level,
			OccurredAt: at, Subject: ChangelogSubject{DocID: r.DocID}, Summary: summary, Origin: ChangelogOriginTape,
			TapeRef: ref(revision), Span: span,
		}
	}
	var out []CommitmentChangelog
	out = append(out, event(ChangelogUpdateStarted, r.OpenedAt, "An update started.", "", updateSpan(r.OpenSeq, r.OpenSeq)))
	revs := append([]UpdateRevision(nil), r.Revisions...)
	sort.SliceStable(revs, func(i, j int) bool { return revs[i].CommittedAt < revs[j].CommittedAt })
	last := r.OpenSeq
	for _, rv := range revs {
		if strings.TrimSpace(rv.RevisionID) == "" {
			continue
		}
		out = append(out, event(ChangelogRevisionCommitted, rv.CommittedAt, "A revision was committed.", rv.RevisionID,
			updateSpan(rv.Seq, rv.Seq), rv.RevisionID))
		if rv.Seq > last {
			last = rv.Seq
		}
	}
	if c := r.Close; c != nil {
		to := c.Seq
		if to < last {
			to = last
		}
		if c.StopReason == ClosureReasonInterruptedByRestart {
			out = append(out, event(ChangelogUpdateInterrupted, c.ClosedAt, "An update was interrupted by a restart.", "", updateSpan(r.OpenSeq, to)))
		} else {
			out = append(out, event(ChangelogUpdateFinished, c.ClosedAt, "An update finished.", "", updateSpan(r.OpenSeq, to)))
		}
	}
	return out
}

// UpdateRoundsFromRecords reads the rounds of a document's inbox
// (texture:<doc>) from records: a Cast addressed to the document opens round
// n, and a Final Report sent from the document (it lands in the caster's
// inbox) ends it, so the caller passes both inboxes' records. Chain sequences are not on the records, so the caller fills OpenSeq,
// Revisions and Close.Seq when it has them. Rounds come back in round order.
func UpdateRoundsFromRecords(docID string, records []CommitmentRecord) []UpdateRound {
	byKey := map[string]*UpdateRound{}
	var keys []string
	round := func(rec CommitmentRecord) *UpdateRound {
		key := rec.ConversationID + "#" + strconv.Itoa(rec.Round)
		r := byKey[key]
		if r == nil {
			r = &UpdateRound{DocID: docID, ConversationID: rec.ConversationID, Round: rec.Round}
			byKey[key] = r
			keys = append(keys, key)
		}
		return r
	}
	for _, rec := range records {
		if rec.Round <= 0 {
			continue
		}
		switch {
		case rec.IsCast() && rec.Inbox == deskmanifest.DocumentInbox(docID):
			r := round(rec)
			if r.OpenedAt == "" {
				r.OpenedAt = rec.Provenance.CommittedAt
			}
		case rec.Final && rec.Report != nil && rec.SenderInbox == deskmanifest.DocumentInbox(docID):
			round(rec).Close = &UpdateClose{RecordID: rec.RecordID, StopReason: rec.Report.StopReason, ClosedAt: rec.Provenance.CommittedAt}
		}
	}
	out := make([]UpdateRound, 0, len(keys))
	for _, k := range keys {
		r := byKey[k]
		if r.OpenedAt == "" { // a close with no cast in the list: not a round we can describe
			continue
		}
		if _, err := time.Parse(time.RFC3339Nano, r.OpenedAt); err != nil {
			continue
		}
		out = append(out, *r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ConversationID != out[j].ConversationID {
			return out[i].ConversationID < out[j].ConversationID
		}
		return out[i].Round < out[j].Round
	})
	return out
}

// ChangelogQuery selects changelog events for a reader. The default shows
// major events only; IncludeMinor is the zoom. Since and Until are RFC 3339
// bounds on occurred_at (Since inclusive, Until exclusive); AfterSeq is the
// inbox sequence cursor.
type ChangelogQuery struct {
	IncludeMinor bool
	Since, Until string
	AfterSeq     int64
	Limit        int
}

// ChangelogEvent is a changelog record as a reader sees it.
type ChangelogEvent struct {
	InboxSeq  int64
	Level     ChangelogLevel
	Changelog CommitmentChangelog
}

// FilterChangelogRecords keeps the changelog records the query selects, in
// inbox order. A record with an unknown kind reads as minor (shown on zoom
// only); a record without a changelog body is skipped. The level always comes
// from the kind, never from the stored field.
func FilterChangelogRecords(records []CommitmentRecord, q ChangelogQuery) []ChangelogEvent {
	var out []ChangelogEvent
	for _, rec := range records {
		if rec.Changelog == nil || rec.InboxSeq <= q.AfterSeq {
			continue
		}
		level, _ := ChangelogLevelOf(rec.Changelog.Kind)
		if level != ChangelogMajor && !q.IncludeMinor {
			continue
		}
		if at, err := time.Parse(time.RFC3339Nano, rec.Changelog.OccurredAt); err == nil {
			if since, err := time.Parse(time.RFC3339Nano, q.Since); err == nil && at.Before(since) {
				continue
			}
			if until, err := time.Parse(time.RFC3339Nano, q.Until); err == nil && !at.Before(until) {
				continue
			}
		}
		ev := ChangelogEvent{InboxSeq: rec.InboxSeq, Level: level, Changelog: *rec.Changelog}
		if ev.Changelog.TapeRef != nil {
			ref := *ev.Changelog.TapeRef
			ref.InboxSeq = rec.InboxSeq
			ev.Changelog.TapeRef = &ref
		}
		out = append(out, ev)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out
}
