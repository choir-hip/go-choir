package textureowner

import (
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func changelogKinds(t *testing.T, s *store.Store, owner, computer string) []types.ChangelogKind {
	t.Helper()
	events, err := s.ListChangelogEvents(t.Context(), owner, computer, types.ChangelogQuery{IncludeMinor: true})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []types.ChangelogKind
	for _, e := range events {
		kinds = append(kinds, e.Changelog.Kind)
	}
	return kinds
}

func TestTakeEmitsOneRevisionCommittedAndWakesNothing(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	v := testVersion("pub-c-v1", "A taken body.")
	handler.PublicationVersions = &fakeVersions{versions: map[string]PublicationVersion{v.PublicationVersionID: v}}
	body := map[string]any{"client_request_id": "take-log", "sources": []any{sourceOf(v)}}
	if w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, body); w.Code != http.StatusAccepted {
		t.Fatalf("take status=%d body=%s", w.Code, w.Body.String())
	}
	kinds := changelogKinds(t, core.Store(), start.OwnerID, start.ComputerID)
	if len(kinds) != 1 || kinds[0] != types.ChangelogRevisionCommitted {
		t.Fatalf("changelog after a take = %v, want one revision_committed", kinds)
	}
	// A replay mints nothing more.
	if w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, body); w.Code != http.StatusAccepted {
		t.Fatalf("replay status=%d", w.Code)
	}
	if kinds := changelogKinds(t, core.Store(), start.OwnerID, start.ComputerID); len(kinds) != 1 {
		t.Fatalf("changelog after a replay = %v", kinds)
	}
	if wakes() != 0 {
		t.Fatalf("the take or its changelog record woke the desk %d times", wakes())
	}
}

func TestPlainOwnerRevisionAndBodySaveEmitNothing(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	wakes := countWakes(core)
	if w := postRevise(t, handler, start.InitialDocument.DocID, start.OwnerID, map[string]any{"client_request_id": "plain", "text": "tighten"}); w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if kinds := changelogKinds(t, core.Store(), start.OwnerID, start.ComputerID); len(kinds) != 0 {
		t.Fatalf("an owner instruction emitted %v; only the desk's turn and take/merge/fork do", kinds)
	}
	if wakes() != 1 {
		t.Fatalf("wakes=%d, want only the revise's own", wakes())
	}
}

// A round the desk answers: the owner's request opens it (a Cast to the
// document's desk) and the desk's final Report closes it; the desk's turn
// commit derives started, then finished, into the changelog inbox. Neither
// record wakes anyone.
func TestDeskTurnCommitDerivesStartedThenFinished(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	wakes := countWakes(core)
	docInbox := deskmanifest.DocumentInbox(start.InitialDocument.DocID)
	caster := "management:" + start.OwnerID
	now := time.Now().UTC()
	mk := func(id string, rec types.CommitmentRecord) types.CommitmentRecord {
		rec.SchemaID, rec.RecordID, rec.ComputerID = types.CommitmentRecordSchemaV1, id, start.ComputerID
		rec.Discrepancy = types.DiscrepancyUnresolved
		rec.Provenance = types.CommitmentProvenance{AgentID: "test", CommittedAt: now.Format(time.RFC3339Nano)}
		return rec
	}
	commit := func(rec types.CommitmentRecord, cursor *types.InboxCursorAdvance) {
		t.Helper()
		req := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: rec.RecordID, Records: []types.CommitmentRecord{rec}, Cursor: cursor}
		var err error
		if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
			t.Fatal(err)
		}
		if _, err := s.CommitInboxRecords(t.Context(), req); err != nil {
			t.Fatalf("commit %s: %v", rec.RecordID, err)
		}
	}
	commit(mk("cast-1", types.CommitmentRecord{Kind: types.CommitmentKindDirective, Inbox: docInbox, SenderInbox: caster, Addressee: docInbox,
		EmitterKind: types.CommitmentEmitterAgent, ConversationID: "conv:management:" + start.OwnerID + "|" + docInbox, Round: 1,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast}}), nil)
	handler.emitUpdateEvents(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if kinds := changelogKinds(t, s, start.OwnerID, start.ComputerID); len(kinds) != 1 || kinds[0] != types.ChangelogUpdateStarted {
		t.Fatalf("open round: changelog = %v, want update_started", kinds)
	}
	commit(mk("final-1", types.CommitmentRecord{Kind: types.CommitmentKindReport, Inbox: caster, SenderInbox: docInbox, Addressee: caster,
		EmitterKind: types.CommitmentEmitterAgent, ConversationID: "conv:management:" + start.OwnerID + "|" + docInbox, Round: 1, Final: true,
		Report: &types.CommitmentReport{StopReason: "done"}}), &types.InboxCursorAdvance{ReaderAgentID: docInbox, Inbox: docInbox, ExpectedSeq: 0, Seq: 1})
	handler.emitUpdateEvents(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	kinds := changelogKinds(t, s, start.OwnerID, start.ComputerID)
	if len(kinds) != 2 || kinds[0] != types.ChangelogUpdateStarted || kinds[1] != types.ChangelogUpdateFinished {
		t.Fatalf("closed round: changelog = %v, want started then finished", kinds)
	}
	handler.emitUpdateEvents(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if again := changelogKinds(t, s, start.OwnerID, start.ComputerID); len(again) != 2 {
		t.Fatalf("a repeat minted more: %v", again)
	}
	if wakes() != 0 {
		t.Fatalf("changelog records woke %d readers", wakes())
	}
}
