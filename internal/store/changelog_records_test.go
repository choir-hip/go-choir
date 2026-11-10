package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// H1 failure list (autopaper step 0 contracts 5.5): H1-3 and H1-7 through the
// real commit path.

func changelogTestRecord(owner string, mutate func(*types.CommitmentRecord)) types.CommitmentRecord {
	body := types.CommitmentChangelog{
		EventID: types.ChangelogEventID(types.ChangelogUpdateFinished, "d", "c"), Kind: types.ChangelogUpdateFinished, Level: types.ChangelogMajor,
		OccurredAt: "2026-11-10T00:00:00Z", Summary: "Done.", Origin: types.ChangelogOriginTape, TapeRef: &types.ChangelogTapeRef{DocID: "d"},
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.ChangelogRecordID(body.EventID), Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: types.ChangelogSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, Inbox: types.ChangelogInbox(owner), SenderInbox: types.ChangelogSenderInbox(body.Origin),
		Changelog: &body,
	}
	if mutate != nil {
		mutate(&rec)
	}
	return rec
}

func TestChangelogRecordRefusesOtherBodiesAtCommit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	commit := func(rec types.CommitmentRecord) error {
		req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
		digest, err := ComputeCommitInboxRecordsDigest(req)
		if err != nil {
			t.Fatal(err)
		}
		req.CommandDigest = digest
		_, err = s.CommitInboxRecords(ctx, req)
		return err
	}
	if err := commit(changelogTestRecord(inboxTestOwner, func(r *types.CommitmentRecord) {
		r.Captures = &types.CommitmentCaptures{SourceID: "rss:x", Items: []types.CaptureArrival{{CaptureID: "c1"}}}
	})); err == nil {
		t.Fatal("a changelog record that also carries captures was committed")
	}
	if err := commit(changelogTestRecord(inboxTestOwner, func(r *types.CommitmentRecord) { r.Changelog.Kind = "update_exploded" })); err == nil {
		t.Fatal("an unknown kind was committed")
	}
	if err := commit(changelogTestRecord(inboxTestOwner, nil)); err != nil {
		t.Fatalf("a good changelog record was refused: %v", err)
	}
}

func TestListChangelogEventsByLevel(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for i, kind := range []types.ChangelogKind{types.ChangelogUpdateStarted, types.ChangelogUpdateFinished, types.ChangelogOpedArrival} {
		rec := changelogTestRecord(inboxTestOwner, func(r *types.CommitmentRecord) {
			level, _ := types.ChangelogLevelOf(kind)
			r.Changelog.Kind, r.Changelog.Level = kind, level
			r.Changelog.EventID = types.ChangelogEventID(kind, string(rune('a'+i)))
			r.RecordID = types.ChangelogRecordID(r.Changelog.EventID)
		})
		req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}}
		digest, err := ComputeCommitInboxRecordsDigest(req)
		if err != nil {
			t.Fatal(err)
		}
		req.CommandDigest = digest
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	major, err := s.ListChangelogEvents(ctx, inboxTestOwner, inboxTestComputer, types.ChangelogQuery{})
	if err != nil || len(major) != 1 || major[0].Changelog.Kind != types.ChangelogUpdateFinished || major[0].Changelog.TapeRef.InboxSeq == 0 {
		t.Fatalf("major: %v %+v", err, major)
	}
	all, err := s.ListChangelogEvents(ctx, inboxTestOwner, inboxTestComputer, types.ChangelogQuery{IncludeMinor: true})
	if err != nil || len(all) != 3 {
		t.Fatalf("zoomed: %v %d", err, len(all))
	}
	if other, err := s.ListChangelogEvents(ctx, "someone-else", inboxTestComputer, types.ChangelogQuery{IncludeMinor: true}); err != nil || len(other) != 0 {
		t.Fatalf("another owner's changelog: %v %d", err, len(other))
	}
}

func TestChangelogAndOpedRecordsWakeNoReader(t *testing.T) {
	cl := changelogTestRecord("o", nil)
	if RecordWakesReader(cl) {
		t.Fatal("a changelog record wakes its reader")
	}
	for _, kind := range types.ChangelogKinds() {
		rec := changelogTestRecord("o", func(r *types.CommitmentRecord) { r.Changelog.Kind = kind })
		if RecordWakesReader(rec) {
			t.Fatalf("a %s changelog record wakes its reader", kind)
		}
	}
	for _, body := range []types.CommitmentOped{
		{Kind: types.OpedRecordArrival, Arrival: &types.OpedArrival{VersionID: "v"}},
		{Kind: types.OpedRecordVerdict, Verdict: &types.OpedVerdict{VersionID: "v", Run: 1, Verdict: types.OpedAccepted}},
		{Kind: types.OpedRecordWakeDecision, WakeDecision: &types.OpedWakeDecision{StoryDocID: "d", N: 1, Woken: true, Why: types.OpedWakeAccepted}},
		{Kind: types.OpedRecordGateUnavailable, GateUnavailable: &types.OpedGateUnavailable{VersionID: "v", Attempts: 3}},
		{Kind: types.OpedRecordVerdict, Verdict: &types.OpedVerdict{VersionID: "v", Run: 3, Verdict: types.OpedUnavailable}},
		// oped_wake is the one waking op-ed record (oped_wake_test.go).
	} {
		body := body
		rec := types.CommitmentRecord{EmitterKind: types.CommitmentEmitterSystem, Inbox: "management:o", SenderInbox: "oped:v", Oped: &body}
		if RecordWakesReader(rec) {
			t.Fatalf("an op-ed %s record wakes management", body.Kind)
		}
	}
}
