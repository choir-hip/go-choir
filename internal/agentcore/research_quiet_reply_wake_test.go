package agentcore

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Research desks are woken by management replies that carry no objective
// (research-desks-woken-by-management-replies-loop-2026-11-20: 27 runs, 639
// model calls, 31 searches, nothing published). Ways it can fail, written
// before the code:
//  1. a reply with no objective, to a record the desk wrote and has since
//     run past (on 2026-11-20 every one of 38 replies sampled answered the
//     desk's own Escalate of 2026-10-26 to 2026-11-19), starts a research run;
//  2. the reply is skipped by moving the desk's cursor, so the desk never
//     reads it, or it is deleted;
//  2b. a reply to an Ask or Escalate the desk wrote and ended its turn on
//     (nothing it has read is newer) does not wake it, and the desk waits
//     forever; or a reply whose related record cannot be found does not wake;
//  3. a record that carries an objective, behind or beside a reply, does not
//     wake the desk (the rule swallows real work);
//  4. a wave of N replies to N desks starts any run;
//  5. the log line a deployed proof counts is missing.

func seedResearchDesk(t *testing.T, rt *Runtime, s *store.Store, owner, docID string) string {
	t.Helper()
	seedDurableTextureSubject(t, s, owner, docID)
	research := deskmanifest.ResearchInbox(docID)
	now := time.Now().UTC()
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{
		AgentID: research, OwnerID: owner, ComputerID: rt.cfg.ComputerID, Profile: agentprofile.Research, Role: agentprofile.Research,
		ChannelID: docID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	return research
}

func commitToResearch(t *testing.T, rt *Runtime, s *store.Store, owner, id string, rec types.CommitmentRecord) {
	t.Helper()
	rec.SchemaID, rec.RecordID = types.CommitmentRecordSchemaV1, id
	rec.Discrepancy = types.DiscrepancyUnresolved
	rec.Addressee = rec.Inbox
	rec.EmitterKind = types.CommitmentEmitterAgent
	rec.Provenance = types.CommitmentProvenance{AgentID: rec.SenderInbox}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: rt.cfg.ComputerID, CommandID: id, Records: []types.CommitmentRecord{rec}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("commit %s: %v", id, err)
	}
}

func managementReplyTo(research, owner string, related ...string) types.CommitmentRecord {
	if len(related) == 0 {
		related = []string{"the-desks-escalation"}
	}
	return types.CommitmentRecord{
		Kind: types.CommitmentKindReport, Inbox: research, SenderInbox: deskmanifest.AdmitterInbox(owner),
		RelatedIDs: related,
		Prediction: types.CommitmentPrediction{Hypothesis: "stop retrying; deliver your round's findings"},
	}
}

// deskEscalation is a record the research desk wrote to management.
func deskEscalation(research, owner string) types.CommitmentRecord {
	return types.CommitmentRecord{
		Kind: types.CommitmentKindDirective, Inbox: deskmanifest.AdmitterInbox(owner), SenderInbox: research,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveEscalate, Body: "Relate is refused"},
	}
}

func deskAsk(research, owner string) types.CommitmentRecord {
	return types.CommitmentRecord{
		Kind: types.CommitmentKindPrecommit, Inbox: deskmanifest.AdmitterInbox(owner), SenderInbox: research,
		Precommit: &types.CommitmentPrecommit{Question: "which source may I use?", Distribution: map[string]float64{"a": 1}, Resolver: deskmanifest.AdmitterInbox(owner)},
	}
}

func textureCast(research, docID string) types.CommitmentRecord {
	return types.CommitmentRecord{
		Kind: types.CommitmentKindDirective, Inbox: research, SenderInbox: "texture:" + docID,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "who owns the filing"},
		Cast:      &types.CommitmentCast{},
	}
}

// commitAs commits a record the way commitToResearch does, whatever its inbox.
func commitAs(t *testing.T, rt *Runtime, s *store.Store, owner, id string, rec types.CommitmentRecord) {
	t.Helper()
	commitToResearch(t, rt, s, owner, id, rec)
}

// deskReadsThrough moves the research desk's own cursor, as its run does.
func deskReadsThrough(t *testing.T, rt *Runtime, s *store.Store, owner, research string, from, to int64) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: rt.cfg.ComputerID, CommandID: fmt.Sprintf("read-%s-%d", research, to),
		Cursor: &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: from, Seq: to}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("cursor advance: %v", err)
	}
}

// staleEscalation seeds what the paper had: the desk escalated (seq 0 of the
// management inbox), then ran again and read a later record (research seq 1).
func staleEscalation(t *testing.T, rt *Runtime, s *store.Store, owner, docID, research string) string {
	t.Helper()
	esc := "esc-" + docID
	commitAs(t, rt, s, owner, esc, deskEscalation(research, owner))
	commitToResearch(t, rt, s, owner, "cast-"+docID, textureCast(research, docID))
	deskReadsThrough(t, rt, s, owner, research, 0, 1)
	return esc
}

func captureWakeLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestReplyToAStaleEscalationStartsNoResearchRunAndStaysReadable(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner, docID = "owner-quiet-reply", "doc-quiet-reply"
	computer := rt.cfg.ComputerID
	research := seedResearchDesk(t, rt, s, owner, docID)
	esc := staleEscalation(t, rt, s, owner, docID, research)
	commitToResearch(t, rt, s, owner, "reply-q1", managementReplyTo(research, owner, esc))
	logs := captureWakeLog(t)

	run, err := rt.WakeRecordsInbox(ctx, owner, computer, research)
	if err != nil || run != nil {
		t.Fatalf("wake = %v, %v; want no run", run, err)
	}
	want := "records inbox: wake declined desk=" + research + " kind=reply reason=management_reply_no_objective count=1"
	if !strings.Contains(logs.String(), want) {
		t.Fatalf("log %q lacks %q", logs.String(), want)
	}
	// The cursor did not move: the reply is still the desk's to read.
	cursor, _ := s.ReaderCursor(ctx, owner, computer, research, research)
	head, _ := s.InboxHead(ctx, owner, computer, research)
	if cursor != 1 || head != 2 {
		t.Fatalf("cursor=%d head=%d; the reply must stay unread (1, 2)", cursor, head)
	}
	recs, err := s.ListInboxRecords(ctx, owner, computer, research, cursor, head)
	if err != nil || len(recs) != 1 || recs[0].RecordID != "reply-q1" {
		t.Fatalf("the desk's next read sees %v, %v; want the reply", recs, err)
	}
	if run, err := rt.WakeRecordsInbox(ctx, owner, computer, research); err != nil || run != nil {
		t.Fatalf("second wake = %v, %v", run, err)
	}
}

func TestReplyToAnAskTheDeskEndedItsTurnOnWakesIt(t *testing.T) {
	for name, rec := range map[string]func(research, owner string) types.CommitmentRecord{"Ask": deskAsk, "Escalate": deskEscalation} {
		t.Run(name, func(t *testing.T) {
			rt, s := testRuntime(t)
			ctx := context.Background()
			const owner, docID = "owner-waits", "doc-waits"
			computer := rt.cfg.ComputerID
			research := seedResearchDesk(t, rt, s, owner, docID)
			// The desk read its cast (seq 1), then wrote its Ask, then its turn ended.
			commitToResearch(t, rt, s, owner, "cast-w", textureCast(research, docID))
			deskReadsThrough(t, rt, s, owner, research, 0, 1)
			commitAs(t, rt, s, owner, "ask-w", rec(research, owner))
			commitToResearch(t, rt, s, owner, "reply-w", managementReplyTo(research, owner, "ask-w"))
			rt.SetDispatchActor(func(_ context.Context, _, _, _, _, _, _, _ string) error { return nil })
			run, err := rt.WakeRecordsInbox(ctx, owner, computer, research)
			if err != nil || run == nil {
				t.Fatalf("a reply to what the desk waits on must wake it: %v, %v", run, err)
			}
		})
	}
}

func TestReplyWakesWhenItsRelatedRecordCannotBeFound(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner, docID = "owner-unfound", "doc-unfound"
	research := seedResearchDesk(t, rt, s, owner, docID)
	staleEscalation(t, rt, s, owner, docID, research)
	commitToResearch(t, rt, s, owner, "reply-u", managementReplyTo(research, owner, "no-such-record"))
	rt.SetDispatchActor(func(_ context.Context, _, _, _, _, _, _, _ string) error { return nil })
	if run, err := rt.WakeRecordsInbox(ctx, owner, rt.cfg.ComputerID, research); err != nil || run == nil {
		t.Fatalf("a reply that cannot be judged must wake: %v, %v", run, err)
	}
}

func TestResearchRecordWithAnObjectiveStillWakesBesideAReply(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner, docID = "owner-quiet-cast", "doc-quiet-cast"
	computer := rt.cfg.ComputerID
	research := seedResearchDesk(t, rt, s, owner, docID)
	esc := staleEscalation(t, rt, s, owner, docID, research)
	commitToResearch(t, rt, s, owner, "reply-c1", managementReplyTo(research, owner, esc))
	rt.SetDispatchActor(func(_ context.Context, _, _, _, _, _, _, _ string) error { return nil })
	if run, err := rt.WakeRecordsInbox(ctx, owner, computer, research); err != nil || run != nil {
		t.Fatalf("reply alone woke: %v, %v", run, err)
	}
	commitToResearch(t, rt, s, owner, "cast-c2", textureCast(research, docID))
	run, err := rt.WakeRecordsInbox(ctx, owner, computer, research)
	if err != nil || run == nil {
		t.Fatalf("a record with an objective must wake the desk: %v, %v", run, err)
	}
}

func TestAWaveOfRepliesToStaleEscalationsOfManyDesksStartsNoRun(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const owner = "owner-quiet-wave"
	computer := rt.cfg.ComputerID
	logs := captureWakeLog(t)
	const desks = 21
	for i := 0; i < desks; i++ {
		docID := fmt.Sprintf("doc-wave-%d", i)
		research := seedResearchDesk(t, rt, s, owner, docID)
		esc := staleEscalation(t, rt, s, owner, docID, research)
		commitToResearch(t, rt, s, owner, fmt.Sprintf("reply-w%d", i), managementReplyTo(research, owner, esc))
		if run, err := rt.WakeRecordsInbox(ctx, owner, computer, research); err != nil || run != nil {
			t.Fatalf("desk %d: wake = %v, %v; want no run", i, run, err)
		}
	}
	if n := strings.Count(logs.String(), "reason=management_reply_no_objective"); n != desks {
		t.Fatalf("declined-wake log lines = %d, want %d", n, desks)
	}
}
