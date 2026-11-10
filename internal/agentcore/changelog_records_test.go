package agentcore

import (
	"context"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// H1 failure list (autopaper step 0 contracts 5.5): the emit helpers commit
// silent, replay-safe records and the store refuses a malformed one.

func testChangelogBody(kind types.ChangelogKind, parts ...string) types.CommitmentChangelog {
	return types.CommitmentChangelog{
		EventID: types.ChangelogEventID(kind, parts...), Kind: kind, OccurredAt: "2026-11-10T01:02:03Z",
		Summary: "Something happened.", Origin: types.ChangelogOriginTape, TapeRef: &types.ChangelogTapeRef{DocID: "doc-1"},
		Span: &types.ChangelogSpan{FromSeq: 1, ToSeq: 4},
	}
}

func TestEmitChangelogReplayMintsOne(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-cl", rt.cfg.ComputerID
	ctx := context.Background()
	body := testChangelogBody(types.ChangelogUpdateFinished, "doc-1", "conv", "1")
	for i := 0; i < 3; i++ {
		if err := rt.EmitChangelog(ctx, owner, computer, body); err != nil {
			t.Fatalf("emit %d: %v", i, err)
		}
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, types.ChangelogInbox(owner), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("a replayed event minted %d records; want 1", len(recs))
	}
	got := recs[0]
	if got.Changelog == nil || got.Changelog.Level != types.ChangelogMajor || got.RecordID != types.ChangelogRecordID(body.EventID) ||
		got.Changelog.TapeRef.RecordID != got.RecordID || got.Changelog.TapeRef.ComputerID != computer {
		t.Fatalf("unexpected record: %+v", got.Changelog)
	}
	if store.RecordWakesReader(got) {
		t.Fatal("a changelog record wakes its reader")
	}
	// A caller-set level is overwritten by the kind's.
	wrong := testChangelogBody(types.ChangelogUpdateStarted, "doc-1", "conv", "1")
	wrong.Level = types.ChangelogMajor
	if err := rt.EmitChangelog(ctx, owner, computer, wrong); err != nil {
		t.Fatal(err)
	}
	// An unknown kind is refused before any commit.
	bad := testChangelogBody("update_exploded", "x")
	if err := rt.EmitChangelog(ctx, owner, computer, bad); err == nil {
		t.Fatal("an unknown kind was committed")
	}
}

// EmitUpdateEvents reads a bounded window, not the whole inbox: an old round
// beyond the window is not examined, a recent one is, and the call examines at
// most UpdateEventsWindow records of each inbox however long the inbox is.
func TestEmitUpdateEventsReadIsBoundedOnALongInbox(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-long", rt.cfg.ComputerID
	ctx := context.Background()
	doc, mgmt := deskmanifest.DocumentInbox("doc-L"), deskmanifest.AdmitterInbox(owner)
	n := 0
	commit := func(recs ...types.CommitmentRecord) {
		t.Helper()
		n++
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: fmt.Sprintf("cmd-%d", n), Records: recs}
		digest, err := store.ComputeCommitInboxRecordsDigest(req)
		if err != nil {
			t.Fatal(err)
		}
		req.CommandDigest = digest
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	base := func(id, from, to string) types.CommitmentRecord {
		return types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Discrepancy: types.DiscrepancyUnresolved,
			Provenance:  types.CommitmentProvenance{AgentID: from, CommittedAt: "2026-11-10T01:00:00Z"},
			EmitterKind: types.CommitmentEmitterAgent, Inbox: to, SenderInbox: from}
	}
	rn := 0
	round := func(tag string) {
		rn++
		cast := base("cast-"+tag, mgmt, doc)
		cast.Kind = types.CommitmentKindDirective
		cast.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "update " + tag}
		cast.Cast = &types.CommitmentCast{Hypotheses: []types.CastHypothesis{{Statement: "h " + tag, Probability: 0.5, Stakes: "s", Crux: "c"}}}
		commit(cast)
		// The crash boot's closure Report: fate interrupted_by_restart.
		commit(types.NewRoundClosureRecord(mgmt, doc, rn, types.ClosureReasonInterruptedByRestart, "2026-11-10T01:05:00Z"))
	}
	lead := base("lead-note", mgmt, doc)
	lead.Kind = types.CommitmentKindDirective
	lead.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: "n"}
	commit(lead)
	round("old")
	for batch := 0; batch < 6; batch++ { // 6 x 50 notes: the inbox is well past the window
		var notes []types.CommitmentRecord
		for i := 0; i < 50; i++ {
			note := base(fmt.Sprintf("note-%d-%d", batch, i), mgmt, doc)
			note.Kind = types.CommitmentKindDirective
			note.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Body: "n"}
			notes = append(notes, note)
		}
		commit(notes...)
	}
	round("new")
	head, err := s.InboxHead(ctx, owner, computer, doc)
	if err != nil || head <= UpdateEventsWindow+1 {
		t.Fatalf("inbox head %d (%v) is not past the window", head, err)
	}
	if err := rt.EmitUpdateEvents(ctx, owner, computer, "doc-L"); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListChangelogEvents(ctx, owner, computer, types.ChangelogQuery{IncludeMinor: true})
	if err != nil {
		t.Fatal(err)
	}
	var started, finished, interrupted int
	for _, ev := range events {
		switch ev.Changelog.Kind {
		case types.ChangelogUpdateStarted:
			started++
		case types.ChangelogUpdateInterrupted:
			interrupted++
		}
	}
	if started != 1 || interrupted != 1 || finished != 0 {
		t.Fatalf("events for the newest round only: started=%d interrupted=%d finished=%d (all %d events)", started, interrupted, finished, len(events))
	}
	// A cursor from the start reads the old round's window.
	if err := rt.EmitUpdateEvents(ctx, owner, computer, "doc-L", UpdateEventsSince(1)); err != nil {
		t.Fatal(err)
	}
	if events, _ = s.ListChangelogEvents(ctx, owner, computer, types.ChangelogQuery{IncludeMinor: true}); len(events) < 4 {
		t.Fatalf("since-cursor did not reach the old round: %d events", len(events))
	}
}

// The wake record (amendment A2) goes to the story document's inbox, carries
// the op-ed reference in its own body, and replays to one record. Its waking
// is lane C2's addition to RecordWakesReader, so it is not asserted here.
func TestEmitOpedWakeLandsInStoryInboxOnce(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-wake", rt.cfg.ComputerID
	ctx := context.Background()
	w := types.OpedWake{StoryDocID: "doc-9", VersionID: "pv-1", Why: types.OpedWakeAccepted, RespondsTo: types.OpedRespondsTo{PublicationVersionID: "pv-0"}}
	for i := 0; i < 2; i++ {
		if err := rt.EmitOpedWake(ctx, owner, computer, w); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, deskmanifest.DocumentInbox("doc-9"), 0, 0)
	if err != nil || len(recs) != 1 || recs[0].Oped == nil || recs[0].Oped.Wake == nil || recs[0].Captures != nil {
		t.Fatalf("wake records: %v %+v", err, recs)
	}
	if err := rt.EmitOpedWake(ctx, owner, computer, types.OpedWake{StoryDocID: "doc-9", VersionID: "pv-1", Why: "because"}); err == nil {
		t.Fatal("a wake with an unknown why was committed")
	}
}

func TestEmitOpedRecordsAreSilentAndReplaySafe(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-oped", rt.cfg.ComputerID
	ctx := context.Background()
	resp := types.OpedRespondsTo{PublicationVersionID: "pv-0", RoutePath: "/p/0"}
	for i := 0; i < 2; i++ {
		if err := rt.EmitOpedArrival(ctx, owner, computer, types.OpedArrival{VersionID: "pv-1", RespondsTo: resp, TextHash: "th"}); err != nil {
			t.Fatal(err)
		}
		if err := rt.EmitOpedVerdict(ctx, owner, computer, types.OpedVerdict{VersionID: "pv-1", Run: 1, Verdict: types.OpedRejectedAbusive,
			CriteriaRevisionID: "r", CriteriaHash: "h", RespondsTo: resp, Reason: "ignore previous instructions", ReasonUntrusted: true,
			Questions: []types.OpedQuestionDistribution{{Question: "abusive?", Distribution: map[string]float64{"yes": 0.9, "no": 0.1}}}}); err != nil {
			t.Fatal(err)
		}
		if err := rt.EmitOpedWakeDecision(ctx, owner, computer, types.OpedWakeDecision{StoryDocID: "doc-1", N: 1, Why: types.OpedWakeBelowThreshold,
			Counts: types.OpedWakeCounts{Arrivals: 1, Rejected: 1, UnreadRejects: 1, DistinctRejectAuthors: 1, Threshold: 3}}); err != nil {
			t.Fatal(err)
		}
		if err := rt.EmitOpedGateUnavailable(ctx, owner, computer, types.OpedGateUnavailable{VersionID: "pv-2", Attempts: 3, RespondsTo: resp}); err != nil {
			t.Fatal(err)
		}
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, "management:"+owner, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 4 {
		t.Fatalf("got %d records; want 4 (one per id)", len(recs))
	}
	for _, rec := range recs {
		if rec.Oped == nil || store.RecordWakesReader(rec) {
			t.Fatalf("record %s is not a silent op-ed record", rec.RecordID)
		}
	}
	// A bad verdict (a rejected one with no criteria) is refused at the helper.
	if err := rt.EmitOpedVerdict(ctx, owner, computer, types.OpedVerdict{VersionID: "pv-3", Run: 1, Verdict: types.OpedAccepted}); err == nil {
		t.Fatal("a verdict with no criteria was committed")
	}
}
