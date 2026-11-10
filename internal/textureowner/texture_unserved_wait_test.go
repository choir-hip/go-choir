package textureowner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/problems/unsettled-obligations-cluster-2026-11-09.md, "Build: failure
// list". A Wait is accepted as a next actor for any text, nothing arms a wake
// for it, and six b@c trajectories sat live for hours on an open work item.
// Ways the Wait leg can fail, written before it:
//   - a Wait nothing serves leaves the request open (the class stays);
//   - a Wait something serves (a round, an unread waking record, a live
//     activation, a later head) is closed anyway;
//   - a Cast and a Wait in one turn close before the cast's round is seen;
//   - a replayed turn closes twice or notifies management twice;
//   - the notice wakes management, or the owner cannot reopen the work.

func unattendedNotices(t *testing.T, s *store.Store, rec types.RunRecord) []types.CommitmentRecord {
	t.Helper()
	records, err := s.ListInboxRecords(context.Background(), rec.OwnerID, rec.ComputerID, agentprofile.Management+":"+rec.OwnerID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.CommitmentRecord
	for _, r := range records {
		if types.IsUnattendedRequestNotice(r) {
			out = append(out, r)
		}
	}
	return out
}

func TestWaitWithNothingServingClosesTheRequestAndTellsManagement(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-wait")
	ctx := context.Background()
	base := headOf(t, s, rec)
	wait := turnBody(t, writeTurn(base.RevisionID, func(turn *yaegikernel.TextureTurn) { turn.Wait = "provider quota resets at 14:03Z" }))
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "unserved-1"); err != nil {
		t.Fatalf("Write with an unserved Wait: %v", err)
	}
	if headOf(t, s, rec).RevisionID == base.RevisionID {
		t.Fatal("the Write did not land")
	}
	w := textureWork(t, s, rec)
	if w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, UnattendedRequestReason) ||
		!strings.Contains(w.Reason, "provider quota resets at 14:03Z") || !strings.Contains(w.Reason, rec.RunID) {
		t.Fatalf("work = %s %q, want refused as unattended naming the Wait and the run", w.Status, w.Reason)
	}
	// A replayed turn (a retried activation end) closes nothing twice.
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "unserved-1"); err != nil {
		t.Fatalf("replayed Wait turn: %v", err)
	}
	notices := unattendedNotices(t, s, rec)
	if len(notices) != 1 || !strings.Contains(notices[0].Directive.Body, w.WorkItemID) {
		t.Fatalf("management notices = %+v, want exactly one for %s", notices, w.WorkItemID)
	}
	if store.RecordWakesReader(notices[0]) {
		t.Fatal("the notice wakes management: a Wait that closes would loop")
	}
	// The owner sees it on the document and the owner's next revise reopens.
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if got := textureRequestUnattended(snapshot, rec.ChannelID); !strings.HasPrefix(got, UnattendedRequestReason) {
		t.Fatalf("document unattended state = %q", got)
	}
	if !textureWorkSettledByDesk(snapshot, rec.ChannelID) {
		t.Fatal("a closed Wait does not let the owner's next revise reopen work")
	}
}

func TestWaitWithACastInTheSameTurnStaysOpen(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-wait-cast")
	ctx := context.Background()
	base := headOf(t, s, rec)
	// The Cast and the Wait commit in one transaction, before the cast's
	// child run exists; the round on the conversation head is the next actor.
	turn := writeTurn(base.RevisionID, func(turn *yaegikernel.TextureTurn) {
		turn.Wait = "research is checking the ruling"
		turn.InboxRecords = []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:same-turn")}
	})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, turn), "same-turn"); err != nil {
		t.Fatalf("Cast with Wait: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s %q, want open while research owes a Report", w.Status, w.Reason)
	}
	if n := len(unattendedNotices(t, s, rec)); n != 0 {
		t.Fatalf("management notices = %d, want none", n)
	}
	// A later Wait-only turn with the round still open is served too.
	v1 := headOf(t, s, rec)
	again := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Wait: "still checking"})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, again, "same-turn-2"); err != nil {
		t.Fatalf("Wait with the round open: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work after a Wait-only turn = %s, want open", w.Status)
	}
}

// The race the investigator could not rule out, the other way round: the
// Report that closes the round lands before the check. It is unread and
// wakes the desk, so the request stays open; once the turn has read it the
// same Wait is unserved.
func TestWaitStaysOpenWhileAnUnreadWakingRecordIsInTheInbox(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-wait-unread")
	ctx := context.Background()
	base := headOf(t, s, rec)
	texture, research := rec.AgentID, agentprofile.Research+":"+rec.ChannelID
	cast := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:unread")}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, cast, "unread-1"); err != nil {
		t.Fatalf("Wait with a Cast: %v", err)
	}
	report := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "seed-unread-report", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:report:unread", Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "the ruling is in"}}, StopReason: "answered"}, Inbox: texture, SenderInbox: research, Addressee: texture,
		ConversationID: types.InboxConversationID(texture, research), Round: 1, Final: true,
		ComputerID: rec.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research},
	}}, Cursor: &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 0, Seq: 1}}
	report.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(report)
	if _, err := s.CommitInboxRecords(ctx, report); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	// The round is closed now; only the unread Report serves the Wait.
	if rounds, err := s.OpenRoundsCastBy(ctx, rec.OwnerID, rec.ComputerID, texture); err != nil || len(rounds) != 0 {
		t.Fatalf("open rounds = %d, %v; want the Report to have closed the round", len(rounds), err)
	}
	unread := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "reading the report next"})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, unread, "unread-2"); err != nil {
		t.Fatalf("Wait with an unread Report: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s %q, want open while a waking Report is unread", w.Status, w.Reason)
	}
	// The turn that reads it and still only waits has nothing behind it.
	read := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "the report was off topic",
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, read, "unread-3"); err != nil {
		t.Fatalf("Wait after reading: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, UnattendedRequestReason) {
		t.Fatalf("work = %s %q, want refused once nothing is unread", w.Status, w.Reason)
	}
}

func TestWaitIsServedByAWakeTheRuntimeAlreadyHolds(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-wait-wake")
	ctx := context.Background()
	base := headOf(t, s, rec)
	doc, err := s.GetLifecycleDocument(ctx, rec.OwnerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	// An owner revision after a Wait-only turn moved the head: it opened work
	// and woke the desk.
	stale := yaegikernel.TextureTurn{BaseRevisionID: "revision-the-turn-answered", Wait: "waiting"}
	if served, err := h.waitServedByWake(ctx, &rec, doc, stale); err != nil || !served {
		t.Fatalf("a moved head: served=%v err=%v, want served", served, err)
	}
	// A Write turn whose head is not the desk's own was followed by an owner
	// edit (the seed head is the owner's).
	written := yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "waiting", Write: &yaegikernel.TextureWrite{Edits: "x", Why: "y"}}
	if served, err := h.waitServedByWake(ctx, &rec, doc, written); err != nil || !served {
		t.Fatalf("an owner head after a Write: served=%v err=%v, want served", served, err)
	}
	// Nothing else: the head is the turn's, no other activation, nothing unread.
	same := yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "waiting"}
	if served, err := h.waitServedByWake(ctx, &rec, doc, same); err != nil || served {
		t.Fatalf("an idle desk: served=%v err=%v, want unserved", served, err)
	}
	// Another live activation of the same desk is a wake already dispatched.
	ended := rec
	finished := time.Now().UTC()
	ended.State, ended.UpdatedAt, ended.FinishedAt = types.RunCompleted, finished, &finished
	end := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "end-this-activation", TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: ended}
	end.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(end)
	if _, err := s.ReplaceLifecycleActivation(ctx, end); err != nil {
		t.Fatalf("end this activation: %v", err)
	}
	other := rec
	other.FinishedAt = nil
	other.RunID = "texture-run-the-other-activation"
	other.State = types.RunRunning
	project := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "project-other-activation", TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: other}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project the other activation: %v", err)
	}
	if served, err := h.waitServedByWake(ctx, &rec, doc, same); err != nil || !served {
		t.Fatalf("another live activation: served=%v err=%v, want served", served, err)
	}
}

// What serves a request besides a round is read from the trajectory and the
// self-development store; the table pins every kind the predicate names.
func TestTextureRequestServerNamesWhatServes(t *testing.T) {
	live := types.LifecycleSnapshot{EngineeringAssignments: []types.EngineeringAssignment{{AssignmentID: "a1", Disposition: types.EngineeringAssignmentBound}}}
	done := types.LifecycleSnapshot{EngineeringAssignments: []types.EngineeringAssignment{{AssignmentID: "a1", Disposition: types.EngineeringAssignmentCompleted}}}
	for _, tc := range []struct {
		name   string
		snap   types.LifecycleSnapshot
		rounds int
		op     string
		want   string
	}{
		{"a live engineering assignment (its capsule job runs under it)", live, 0, "", "engineering assignment"},
		{"an open round Texture cast", done, 1, "", "open round"},
		{"a live self-development operation", done, 0, "selfdev-op-1", "self-development operation"},
		{"a finished assignment and nothing else", done, 0, "", ""},
		{"an empty trajectory", types.LifecycleSnapshot{}, 0, "", ""},
	} {
		if got := textureRequestServer(tc.snap, tc.rounds, tc.op); got != tc.want {
			t.Errorf("%s: server = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The crash boot's leg (agentcore closeUnservedTextureWorkAfterCrash hands
// each open Texture work item to closeUnservedAtBoot).
func TestCrashBootClosesTextureWorkNothingServes(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-boot")
	ctx := context.Background()
	work := textureWork(t, s, rec)
	if err := h.closeUnservedAtBoot(ctx, work); err != nil {
		t.Fatalf("boot close: %v", err)
	}
	w := textureWork(t, s, rec)
	if w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, UnattendedRequestReason) || !strings.Contains(w.Reason, "interrupted by a restart") {
		t.Fatalf("work = %s %q, want refused, interrupted by a restart", w.Status, w.Reason)
	}
	// A second boot finds it closed and writes nothing.
	if err := h.closeUnservedAtBoot(ctx, work); err != nil {
		t.Fatalf("second boot close: %v", err)
	}
	notices := unattendedNotices(t, s, rec)
	if len(notices) != 1 || store.RecordWakesReader(notices[0]) {
		t.Fatalf("management notices = %+v, want one that wakes no one", notices)
	}
}

func TestCrashBootLeavesServedAndForeignWorkAlone(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-boot-served")
	ctx := context.Background()
	base := headOf(t, s, rec)
	cast := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:boot-served")}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, cast, "boot-served-1"); err != nil {
		t.Fatalf("Wait with a Cast: %v", err)
	}
	work := textureWork(t, s, rec)
	if err := h.closeUnservedAtBoot(ctx, work); err != nil {
		t.Fatalf("boot close: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s, want open while a round Texture cast is open", w.Status)
	}
	// Another desk's work item is not Texture's to close.
	foreign := work
	foreign.AssignedAgentID = agentprofile.Research + ":" + rec.ChannelID
	foreign.AuthorityProfile = agentprofile.Research
	if err := h.closeUnservedAtBoot(ctx, foreign); err != nil {
		t.Fatalf("foreign boot close: %v", err)
	}
	if n := len(unattendedNotices(t, s, rec)); n != 0 {
		t.Fatalf("management notices = %d, want none", n)
	}
}

// Review of x3-unserved-work, finding 1: an Ask is a staked request (a
// precommit with a Resolver) and an Escalate a directive; neither opens a
// round, yet the desk they wake holds the question and the Reply wakes this
// desk. foldTextureDispatch turns a cell that only Asks into a Wait, so a
// Wait served by an Ask must stay open. Ways it can fail, written first:
//   - an Ask or Escalate in the same turn as the Wait is closed over;
//   - an answered Ask, a retracted one or a read Escalate keeps the request
//     open forever (the control).

func researchAsk(rec types.RunRecord, id string) types.CommitmentRecord {
	texture, research := rec.AgentID, agentprofile.Research+":"+rec.ChannelID
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindPrecommit,
		Precommit: &types.CommitmentPrecommit{Question: "who funded the merger?", Resolver: research},
		Inbox:     research, SenderInbox: texture, Addressee: research, ComputerID: rec.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
	}
}

func managementEscalate(rec types.RunRecord, id string) types.CommitmentRecord {
	texture, management := rec.AgentID, agentprofile.Management+":"+rec.OwnerID
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveEscalate, Body: "the request needs a decision"},
		Inbox:     management, SenderInbox: texture, Addressee: management, ComputerID: rec.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
	}
}

func commitRecords(t *testing.T, s *store.Store, rec types.RunRecord, command string, cursor *types.InboxCursorAdvance, records ...types.CommitmentRecord) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: command, Records: records, Cursor: cursor}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("%s: %v", command, err)
	}
}

func TestWaitStaysOpenWhileAnAskOrEscalateIsBeingWorked(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(types.RunRecord) types.CommitmentRecord
	}{
		{"an Ask", func(rec types.RunRecord) types.CommitmentRecord { return researchAsk(rec, "texture-cell:ask:fold") }},
		{"an Escalate", func(rec types.RunRecord) types.CommitmentRecord {
			return managementEscalate(rec, "texture-cell:escalate:fold")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, s, rec := cellAuthorSetup(t, "unserved-ask-"+strings.ReplaceAll(strings.ToLower(tc.name), " ", "-"))
			ctx := context.Background()
			base := headOf(t, s, rec)
			// The folded shape: the dispatch and the implicit Wait in one turn.
			turn := yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "dispatched: who funded the merger?", InboxRecords: []types.CommitmentRecord{tc.make(rec)}}
			if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, turn), "ask-fold"); err != nil {
				t.Fatalf("dispatch with a Wait: %v", err)
			}
			if rounds, _ := s.OpenRoundsCastBy(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID); len(rounds) != 0 {
				t.Fatalf("open rounds = %d; the premise is that %s opens none", len(rounds), tc.name)
			}
			if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
				t.Fatalf("work = %s %q, want open while %s is being worked", w.Status, w.Reason, tc.name)
			}
			if n := len(unattendedNotices(t, s, rec)); n != 0 {
				t.Fatalf("management notices = %d, want none", n)
			}
		})
	}
}

func TestWaitClosesOnceTheAskIsAnsweredRetractedOrTheEscalateRead(t *testing.T) {
	t.Run("a Reply that was read", func(t *testing.T) {
		h, s, rec := cellAuthorSetup(t, "unserved-ask-reply")
		ctx := context.Background()
		texture, research := rec.AgentID, agentprofile.Research+":"+rec.ChannelID
		base := headOf(t, s, rec)
		ask := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchAsk(rec, "texture-cell:ask:reply")}})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, ask, "reply-1"); err != nil {
			t.Fatal(err)
		}
		commitRecords(t, s, rec, "seed-reply", nil, types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:reply:1", Kind: types.CommitmentKindReport,
			Report:     &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "the fund"}}},
			RelatedIDs: []string{"texture-cell:ask:reply"}, Inbox: texture, SenderInbox: research, Addressee: texture,
			ComputerID: rec.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research},
		})
		if out, err := s.OutstandingDispatches(ctx, rec.OwnerID, rec.ComputerID, texture); err != nil || len(out) != 0 {
			t.Fatalf("outstanding after the Reply = %d, %v; want none", len(out), err)
		}
		// The turn that reads the Reply and only waits has nothing behind it.
		read := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "the reply was not enough",
			InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1}})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, read, "reply-2"); err != nil {
			t.Fatal(err)
		}
		if w := textureWork(t, s, rec); w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, UnattendedRequestReason) {
			t.Fatalf("work = %s %q, want refused once the Ask is answered and read", w.Status, w.Reason)
		}
	})
	t.Run("a retracted Ask", func(t *testing.T) {
		h, s, rec := cellAuthorSetup(t, "unserved-ask-retract")
		ctx := context.Background()
		texture := rec.AgentID
		base := headOf(t, s, rec)
		ask := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchAsk(rec, "texture-cell:ask:retract")}})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, ask, "retract-1"); err != nil {
			t.Fatal(err)
		}
		research := agentprofile.Research + ":" + rec.ChannelID
		commitRecords(t, s, rec, "seed-retract", nil, types.CommitmentRecord{
			SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cancel:1", Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveRetract, TargetRef: "texture-cell:ask:retract"},
			Inbox:     research, SenderInbox: texture, Addressee: research, ComputerID: rec.ComputerID,
			EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
		})
		if out, err := s.OutstandingDispatches(ctx, rec.OwnerID, rec.ComputerID, texture); err != nil || len(out) != 0 {
			t.Fatalf("outstanding after the retraction = %d, %v; want none", len(out), err)
		}
	})
	t.Run("an Escalate its recipient read", func(t *testing.T) {
		h, s, rec := cellAuthorSetup(t, "unserved-escalate-read")
		ctx := context.Background()
		texture, management := rec.AgentID, agentprofile.Management+":"+rec.OwnerID
		base := headOf(t, s, rec)
		esc := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "escalated", InboxRecords: []types.CommitmentRecord{managementEscalate(rec, "texture-cell:escalate:read")}})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, esc, "escalate-1"); err != nil {
			t.Fatal(err)
		}
		if out, err := s.OutstandingDispatches(ctx, rec.OwnerID, rec.ComputerID, texture); err != nil || len(out) != 1 {
			t.Fatalf("outstanding before management reads = %d, %v; want the Escalate", len(out), err)
		}
		// Management reads its inbox (an empty commit that only advances the cursor).
		commitRecords(t, s, rec, "management-reads", &types.InboxCursorAdvance{ReaderAgentID: management, Inbox: management, ExpectedSeq: 0, Seq: 1})
		if out, err := s.OutstandingDispatches(ctx, rec.OwnerID, rec.ComputerID, texture); err != nil || len(out) != 0 {
			t.Fatalf("outstanding after management read it = %d, %v; want none", len(out), err)
		}
	})
}

// At a crash boot the asked desk's run is closed and its wake interrupted, so
// an unanswered Ask serves nothing: the boot leg does not count it, and the
// request closes as interrupted by a restart.
func TestCrashBootClosesTextureWorkWhoseAskWasNeverAnswered(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-boot-ask")
	ctx := context.Background()
	base := headOf(t, s, rec)
	ask := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchAsk(rec, "texture-cell:ask:boot")}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, ask, "boot-ask-1"); err != nil {
		t.Fatal(err)
	}
	work := textureWork(t, s, rec)
	if work.Status != types.WorkItemOpen {
		t.Fatalf("work = %s, want open while the Ask is live", work.Status)
	}
	if err := h.closeUnservedAtBoot(ctx, work); err != nil {
		t.Fatal(err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemRefused || !strings.Contains(w.Reason, "interrupted by a restart") {
		t.Fatalf("work = %s %q, want refused, interrupted by a restart", w.Status, w.Reason)
	}
}

// Round 2 review, F1: the Idle backstop and the bare-Write guard read the same
// "is anything working on this request" predicate, so an Ask or Escalate still
// being worked must serve the request there too, not only after a Wait.
func TestIdleTurnAndBareWriteLeaveTheRequestOpenWhileAnAskIsOutstanding(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-idle-ask")
	ctx := context.Background()
	base := headOf(t, s, rec)
	ask := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchAsk(rec, "texture-cell:ask:idle")}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, ask, "idle-ask-1"); err != nil {
		t.Fatal(err)
	}
	// An unrelated wake ends in an Idle turn while the Ask is still worked.
	idle := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Idle: agentcore.TextureIdleNoActReason})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "idle-ask-2"); err != nil {
		t.Fatalf("Idle turn: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s %q, want open while the Ask is outstanding", w.Status, w.Reason)
	}
	// A bare Write (no Cast, Wait or Done) is a progress revision, not refused.
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, writeTurn(base.RevisionID, nil)), "idle-ask-3"); err != nil {
		t.Fatalf("bare Write while an Ask is outstanding: %v", err)
	}
}

// The boot leg keeps its exemption: the asked desk's run is closed there.
func TestIdleTurnClosesOnceNothingIsOutstanding(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-idle-noask")
	ctx := context.Background()
	base := headOf(t, s, rec)
	idle := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Idle: agentcore.TextureIdleNoActReason})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "noask-1"); err != nil {
		t.Fatal(err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemRefused {
		t.Fatalf("work = %s, want refused with nothing outstanding", w.Status)
	}
}

// Round 2 review, F3: an Escalate is a standalone intent committed in the
// cell's staged order, not folded into the turn. Committed before the Wait
// turn it serves the Wait. (Staged after the Wait it commits after the check
// and the Wait is closed over: a named residual in the cluster record.)
func TestWaitAfterAStandaloneEscalateStaysOpen(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unserved-escalate-standalone")
	ctx := context.Background()
	base := headOf(t, s, rec)
	commitRecords(t, s, rec, "standalone-escalate", nil, managementEscalate(rec, "texture-cell:escalate:standalone"))
	wait := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "escalated to management"})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "standalone-1"); err != nil {
		t.Fatal(err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s %q, want open while management holds the Escalate", w.Status, w.Reason)
	}
}
