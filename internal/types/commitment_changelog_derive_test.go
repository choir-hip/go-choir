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
	r := UpdateRound{DocID: "d", ConversationID: "c", Round: 2, OpenedAt: "2026-11-10T01:00:00Z", OpenSeq: 10,
		Revisions: []UpdateRevision{{RevisionID: "r2", CommittedAt: "2026-11-10T01:02:00Z", Seq: 14}, {RevisionID: "r1", CommittedAt: "2026-11-10T01:01:00Z", Seq: 12}},
		Close:     &UpdateClose{RecordID: "rep", StopReason: "done", ClosedAt: "2026-11-10T01:03:00Z", Seq: 15}}
	evs := DeriveUpdateEvents(r)
	want := []ChangelogKind{ChangelogUpdateStarted, ChangelogRevisionCommitted, ChangelogRevisionCommitted, ChangelogUpdateFinished}
	got := kindsOf(evs)
	if len(got) != len(want) {
		t.Fatalf("kinds %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds %v; want %v", got, want)
		}
	}
	last := evs[len(evs)-1]
	if last.Span == nil || last.Span.FromSeq != 10 || last.Span.ToSeq != 15 || last.Level != ChangelogMajor {
		t.Fatalf("finished event span/level: %+v", last)
	}
	if evs[1].TapeRef.RevisionID != "r1" {
		t.Fatalf("revisions are not in commit order: %+v", evs[1].TapeRef)
	}
	for _, e := range evs {
		if err := ValidateChangelogBody(e); err != nil {
			t.Fatalf("%s: %v", e.Kind, err)
		}
	}
	// Deterministic: a second derivation yields the same ids.
	again := DeriveUpdateEvents(r)
	for i := range evs {
		if evs[i].EventID != again[i].EventID {
			t.Fatal("event ids are not deterministic")
		}
	}
}

func TestDeriveUpdateEventsInterruptedIsNotFinished(t *testing.T) {
	r := UpdateRound{DocID: "d", ConversationID: "c", Round: 1, OpenedAt: "2026-11-10T01:00:00Z", OpenSeq: 3,
		Close: &UpdateClose{StopReason: ClosureReasonInterruptedByRestart, ClosedAt: "2026-11-10T01:05:00Z", Seq: 9}}
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
	if ev := DeriveUpdateEvents(UpdateRound{DocID: "d", ConversationID: "c", Round: 1, OpenedAt: "2026-11-10T01:00:00Z"})[0]; ev.Span != nil {
		t.Fatal("an unknown sequence must carry no span")
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
