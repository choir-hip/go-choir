package types

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

func kindsOf(evs []CommitmentChangelog) []ChangelogKind {
	var out []ChangelogKind
	for _, e := range evs {
		out = append(out, e.Kind)
	}
	return out
}

func TestDeriveUpdateEventsFinished(t *testing.T) {
	r := UpdateRound{DocID: "d", ConversationID: "c", Round: 2, OpenedAt: "2026-11-10T01:00:00Z",
		Close: &UpdateClose{RecordID: "rep", StopReason: "done", ClosedAt: "2026-11-10T01:03:00Z"}}
	evs := DeriveUpdateEvents(r)
	got := kindsOf(evs)
	if len(got) != 2 || got[0] != ChangelogUpdateStarted || got[1] != ChangelogUpdateFinished || evs[1].Level != ChangelogMajor {
		t.Fatalf("kinds %v", got)
	}
	for _, e := range evs {
		if err := ValidateChangelogBody(e); err != nil {
			t.Fatalf("%s: %v", e.Kind, err)
		}
		if e.Span != nil {
			t.Fatal("a derived event carries no span: the chain sequence is not on the records")
		}
	}
	again := DeriveUpdateEvents(r)
	for i := range evs {
		if evs[i].EventID != again[i].EventID {
			t.Fatal("event ids are not deterministic")
		}
	}
}

func TestDeriveUpdateEventsInterruptedIsNotFinished(t *testing.T) {
	r := UpdateRound{DocID: "d", ConversationID: "c", Round: 1, OpenedAt: "2026-11-10T01:00:00Z",
		Close: &UpdateClose{StopReason: ClosureReasonInterruptedByRestart, ClosedAt: "2026-11-10T01:05:00Z"}}
	got := kindsOf(DeriveUpdateEvents(r))
	if len(got) != 2 || got[1] != ChangelogUpdateInterrupted {
		t.Fatalf("kinds %v; want started then interrupted", got)
	}
	for _, k := range got {
		if k == ChangelogUpdateFinished {
			t.Fatal("an interrupted update was written as finished")
		}
	}
}

func TestDeriveUpdateEventsOpenRoundIsStartedOnly(t *testing.T) {
	got := kindsOf(DeriveUpdateEvents(UpdateRound{DocID: "d", ConversationID: "c", Round: 1, OpenedAt: "2026-11-10T01:00:00Z"}))
	if len(got) != 1 || got[0] != ChangelogUpdateStarted {
		t.Fatalf("kinds %v", got)
	}
}

func TestUpdateRoundsFromRecords(t *testing.T) {
	inbox := deskmanifest.DocumentInbox("d")
	cast := CommitmentRecord{RecordID: "k1", Inbox: inbox, SenderInbox: "management:o", ConversationID: "c", Round: 1,
		Provenance: CommitmentProvenance{CommittedAt: "2026-11-10T01:00:00Z"}, Kind: CommitmentKindDirective,
		Directive: &CommitmentDirective{Subtype: CommitmentDirectiveCast}}
	final := CommitmentRecord{RecordID: "f1", Inbox: "management:o", SenderInbox: inbox, ConversationID: "c", Round: 1, Final: true, Kind: CommitmentKindReport,
		Provenance: CommitmentProvenance{CommittedAt: "2026-11-10T01:09:00Z"}, Report: &CommitmentReport{StopReason: ClosureReasonInterruptedByRestart}}
	rounds := UpdateRoundsFromRecords("d", []CommitmentRecord{cast, final})
	if len(rounds) != 1 || rounds[0].Close == nil || rounds[0].Close.StopReason != ClosureReasonInterruptedByRestart || rounds[0].OpenedAt == "" {
		t.Fatalf("rounds: %+v", rounds)
	}
	if got := kindsOf(DeriveUpdateEvents(rounds[0])); got[len(got)-1] != ChangelogUpdateInterrupted {
		t.Fatalf("kinds %v", got)
	}
	// A close with no cast in the list is not described.
	if rs := UpdateRoundsFromRecords("d", []CommitmentRecord{final}); len(rs) != 0 {
		t.Fatalf("rounds from a lone close: %+v", rs)
	}
}

func TestFilterChangelogRecordsLevelRangeAndUnknownKind(t *testing.T) {
	mk := func(seq int64, kind ChangelogKind, at string) CommitmentRecord {
		return CommitmentRecord{InboxSeq: seq, Changelog: &CommitmentChangelog{EventID: kind2id(kind, seq), Kind: kind, Level: ChangelogMajor, OccurredAt: at,
			TapeRef: &ChangelogTapeRef{DocID: "d"}}}
	}
	recs := []CommitmentRecord{
		mk(1, ChangelogUpdateStarted, "2026-11-10T01:00:00Z"),
		mk(2, ChangelogUpdateFinished, "2026-11-10T02:00:00Z"),
		mk(3, "update_exploded", "2026-11-10T03:00:00Z"), // stored level lies; the kind decides
		mk(4, ChangelogParentUsed, "2026-11-10T04:00:00Z"),
		{InboxSeq: 5},
	}
	major := FilterChangelogRecords(recs, ChangelogQuery{})
	if len(major) != 2 || major[0].InboxSeq != 2 || major[1].InboxSeq != 4 {
		t.Fatalf("major: %+v", major)
	}
	if major[0].Changelog.TapeRef.InboxSeq != 2 {
		t.Fatal("the tape ref does not carry the record's inbox seq")
	}
	all := FilterChangelogRecords(recs, ChangelogQuery{IncludeMinor: true})
	if len(all) != 4 {
		t.Fatalf("zoomed: %d events; want 4 (the unknown kind shows as minor)", len(all))
	}
	ranged := FilterChangelogRecords(recs, ChangelogQuery{IncludeMinor: true, Since: "2026-11-10T02:00:00Z", Until: "2026-11-10T04:00:00Z"})
	if len(ranged) != 2 || ranged[0].InboxSeq != 2 || ranged[1].InboxSeq != 3 {
		t.Fatalf("ranged: %+v", ranged)
	}
	if got := FilterChangelogRecords(recs, ChangelogQuery{IncludeMinor: true, AfterSeq: 2, Limit: 1}); len(got) != 1 || got[0].InboxSeq != 3 {
		t.Fatalf("cursor and limit: %+v", got)
	}
}

func kind2id(k ChangelogKind, seq int64) string { return ChangelogEventID(k, string(rune('a'+seq))) }
