package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The corpus brief's objective (docs/problems/web-searches-are-sent-without-asking-what-is-held-2026-11-19.md,
// fix 4). On the paper 51 of 77 briefs were skipped with "no objective in the
// unread records": a run reads its inbox from the reader's cursor, and a later
// run of the same research agent starts with the Cast already read.
//
// Failure list, written before the code:
//  1. a run whose cursor is past its conversation's Cast (nothing unread) is
//     skipped, though the conversation's last Cast names the objective;
//  2. a run with unread records that name no objective is skipped for the
//     same reason;
//  3. an objective in the unread records loses to an older one, or the log
//     does not say it came from the unread records;
//  4. the fallback takes the objective of another conversation's Cast;
//  5. the fallback takes the oldest objective of the conversation, not the
//     most recent;
//  6. a run with no Cast anywhere but an open work item assigned to it is
//     skipped, though the assignment names the objective;
//  7. a run with an objective nowhere is briefed, or its line does not say
//     objective_from=none and how many records were unread;
//  8. the log line has no objective_from field on some outcome;
//  9. the brief searches more threads at once than one corpusd answers in
//     the deadline (five at once took 4 s to 8 s each on the paper, one
//     alone 1.3 s).

func seedNumberedCast(t *testing.T, s *store.Store, owner, computer, doc, sender, id, objective string) {
	t.Helper()
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:" + id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: objective},
		Cast:      &types.CommitmentCast{}, Inbox: "research:" + doc, SenderInbox: sender, Addressee: "research:" + doc,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: sender},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-" + id, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("seed cast %s: %v", id, err)
	}
}

// readThrough moves the research agent's cursor on its own inbox.
func readThrough(t *testing.T, s *store.Store, owner, computer, doc string, from, to int64) {
	t.Helper()
	inbox := "research:" + doc
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "read-" + doc + "-" + string(rune('a'+to)),
		Cursor: &types.InboxCursorAdvance{ReaderAgentID: inbox, Inbox: inbox, ExpectedSeq: from, Seq: to}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("advance cursor: %v", err)
	}
}

func briefFromLine(logs string) string {
	for _, f := range strings.Fields(logs) {
		if strings.HasPrefix(f, "objective_from=") {
			return strings.TrimPrefix(f, "objective_from=")
		}
	}
	return ""
}

func TestBriefOfALaterRunWithNothingUnreadUsesTheConversationsLastObjective(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-1", rt.cfg.ComputerID, "doc-obj-1"
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj1a", "Did the harbor strike end?")
	readThrough(t, s, owner, computer, doc, 0, 1)
	probe := &briefProbe{result: briefResult([]map[string]any{briefCapture(1, "workers voted")}, 0)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := briefRunRecord(owner, computer, doc, "run-obj-1")
	b := rt.researchBriefOpening(context.Background(), rec)
	briefCommitFresh(t, rt, s, rec, b)
	if len(probe.queries) != 1 || !strings.Contains(probe.queries[0], "harbor strike") {
		t.Fatalf("queries = %v, want the conversation's objective", probe.queries)
	}
	if !strings.Contains(b.openingText(), "workers voted") || briefFromLine(logs.String()) != "conversation" || !strings.Contains(logs.String(), "outcome=delivered") {
		t.Fatalf("text=%q log=%s", b.openingText(), logs.String())
	}
}

func TestBriefTakesTheMostRecentObjectiveOfTheConversationNotTheOldest(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-2", rt.cfg.ComputerID, "doc-obj-2"
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj2a", "Old question about ferries")
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj2b", "New question about the harbor")
	readThrough(t, s, owner, computer, doc, 0, 2)
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	rt.researchBriefOpening(context.Background(), briefRunRecord(owner, computer, doc, "run-obj-2"))
	if len(probe.queries) != 1 || !strings.Contains(probe.queries[0], "harbor") {
		t.Fatalf("queries = %v, want the newest objective", probe.queries)
	}
}

func TestBriefDoesNotTakeAnObjectiveOfAnotherConversation(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-3", rt.cfg.ComputerID, "doc-obj-3"
	// A Cast into this inbox from a different sender is another conversation.
	seedNumberedCast(t, s, owner, computer, doc, "texture:another-doc", "obj3a", "Someone else's objective")
	readThrough(t, s, owner, computer, doc, 0, 1)
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := briefRunRecord(owner, computer, doc, "run-obj-3")
	b := rt.researchBriefOpening(context.Background(), rec)
	briefCommitFresh(t, rt, s, rec, b)
	if len(probe.queries) != 0 || briefFromLine(logs.String()) != "none" || !strings.Contains(logs.String(), "outcome=skipped") {
		t.Fatalf("queries=%v log=%s", probe.queries, logs.String())
	}
}

func TestBriefUnreadObjectiveWinsAndTheLogSaysSo(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-4", rt.cfg.ComputerID, "doc-obj-4"
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj4a", "Old question about ferries")
	readThrough(t, s, owner, computer, doc, 0, 1)
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj4b", "New question about the harbor")
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := briefRunRecord(owner, computer, doc, "run-obj-4")
	b := rt.researchBriefOpening(context.Background(), rec)
	briefCommitFresh(t, rt, s, rec, b)
	if len(probe.queries) != 1 || !strings.Contains(probe.queries[0], "harbor") || briefFromLine(logs.String()) != "unread" {
		t.Fatalf("queries=%v log=%s", probe.queries, logs.String())
	}
}

func TestBriefOfARunWithNoObjectiveAnywhereIsSkippedAndSaysWhichShape(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-5", rt.cfg.ComputerID, "doc-obj-5"
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := briefRunRecord(owner, computer, doc, "run-obj-5")
	b := rt.researchBriefOpening(context.Background(), rec)
	briefCommitFresh(t, rt, s, rec, b)
	if len(probe.queries) != 0 || briefFromLine(logs.String()) != "none" || !strings.Contains(logs.String(), "unread=0") || !strings.Contains(logs.String(), "outcome=skipped") {
		t.Fatalf("queries=%v log=%s", probe.queries, logs.String())
	}
	if !strings.Contains(b.openingText(), "the unread records name no objective") {
		t.Fatalf("text=%q", b.openingText())
	}
}

func TestBriefOfARunWithNoCastButAnAssignedWorkItemUsesTheAssignmentsObjective(t *testing.T) {
	rt, s := testRuntime(t)
	owner := "owner-obj-6"
	fixture := seedTextureLifecycleControl(t, s, owner, "obj6", agentprofile.Research+":control-obj6", agentprofile.Research)
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := fixture.run
	rec.State = types.RunRunning
	b := rt.researchBriefOpening(context.Background(), &rec)
	briefCommitFresh(t, rt, s, &rec, b)
	if len(probe.queries) != 1 || probe.queries[0] != "research obj6" || briefFromLine(logs.String()) != "assignment" {
		t.Fatalf("queries=%v log=%s", probe.queries, logs.String())
	}
}

func TestBriefLogLineSaysWhereTheObjectiveCameFromOnEveryOutcome(t *testing.T) {
	for _, res := range []researchBriefResult{
		{outcome: briefDelivered, from: briefFromUnread},
		{outcome: briefEmpty, from: briefFromConversation},
		{outcome: briefFailed, from: briefFromAssignment},
		{outcome: briefSkipped, from: briefFromNone, unread: 2},
		{outcome: briefFailed}, // a failure before any plan was read
	} {
		line := researchBriefLogLine("r", "research", 0, res)
		want := res.from
		if want == "" {
			want = briefFromNone
		}
		if !strings.Contains(line, " objective_from="+want+" ") || !strings.Contains(line, " unread=") {
			t.Fatalf("%s outcome: %s", res.outcome, line)
		}
	}
}

func TestBriefSearchesAtMostThreeThreadsAndNamesTheRest(t *testing.T) {
	cast := types.CommitmentRecord{Kind: types.CommitmentKindDirective, Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Did the harbor strike end?"},
		Cast: &types.CommitmentCast{
			Hypotheses:    []types.CastHypothesis{{Statement: "h1"}, {Statement: "h2"}, {Statement: "h3"}},
			Uncertainties: []types.CastUncertainty{{Question: "u1"}},
		}}
	plan := researchBriefThreads([]types.CommitmentRecord{cast}, "")
	if len(plan.threads) != 3 || len(plan.left) != 2 || plan.from != briefFromUnread {
		t.Fatalf("threads=%d left=%d from=%q", len(plan.threads), len(plan.left), plan.from)
	}
}

// Review follow-ups (docs/problems/web-searches-are-sent-without-asking-what-is-held-2026-11-19.md).
//
// Failure list, written before the code:
//  10. a work item of another computer of the same owner, named by the run's
//      metadata, is searched for the run;
//  11. a fallback brief says "your objective" as if the run had written it,
//      so the run cannot tell a record it did not write, or an item, from its
//      own prompt;
//  12. a later wake on a record with no objective is briefed with the Cast of
//      a round that was already answered;
//  13. a run that is assigned an open work item is briefed on the
//      conversation's older Cast instead.

func closeRoundOfDoc(t *testing.T, s *store.Store, owner, computer, doc, id string) {
	t.Helper()
	final := finalReportRecord("final-"+id, "research:"+doc, "texture:"+doc, time.Now())
	if _, err := commitUnreadCast(t, s, owner, computer, "final-cmd-"+id, nil, final); err != nil {
		t.Fatalf("close round: %v", err)
	}
}

func TestBriefNeverUsesAWorkItemOfAnotherComputerOfTheSameOwner(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-7", rt.cfg.ComputerID, "doc-obj-7"
	other, err := s.CreateWorkItem(context.Background(), types.WorkItemRecord{OwnerID: owner, ComputerID: "another-computer", TrajectoryID: "traj-obj-7",
		Objective: "Another computer's objective", AssignedAgentID: "research:" + doc})
	if err != nil {
		t.Fatal(err)
	}
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	rec := briefRunRecord(owner, computer, doc, "run-obj-7")
	rec.Metadata = map[string]any{"work_item_ids": []string{other.WorkItemID}}
	rt.researchBriefOpening(context.Background(), rec)
	if len(probe.queries) != 0 {
		t.Fatalf("queries = %v, want none: the item belongs to another computer", probe.queries)
	}
}

func TestBriefNamesWhereTheObjectiveCameFromInTheText(t *testing.T) {
	// unread: the run's own record, "your objective".
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-8", rt.cfg.ComputerID, "doc-obj-8"
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj8a", "Did the harbor strike end?")
	rt.researchBriefSearch = (&briefProbe{result: briefResult(nil, 1)}).search
	text := rt.researchBriefOpening(context.Background(), briefRunRecord(owner, computer, doc, "run-obj-8")).openingText()
	if !strings.Contains(text, "for your objective") || strings.Contains(text, "earlier record") || strings.Contains(text, "assigned work item") {
		t.Fatalf("unread wording:\n%s", text)
	}

	// conversation: an earlier record of this conversation.
	readThrough(t, s, owner, computer, doc, 0, 1)
	text = rt.researchBriefOpening(context.Background(), briefRunRecord(owner, computer, doc, "run-obj-8b")).openingText()
	if !strings.Contains(text, "an earlier record of this conversation") || strings.Contains(text, "for your objective") || !strings.Contains(text, "(objective of an earlier record of this conversation)") {
		t.Fatalf("conversation wording:\n%s", text)
	}

	// assignment: the assigned work item.
	rt2, s2 := testRuntime(t)
	fixture := seedTextureLifecycleControl(t, s2, "owner-obj-8c", "obj8c", agentprofile.Research+":control-obj8c", agentprofile.Research)
	rt2.researchBriefSearch = (&briefProbe{result: briefResult(nil, 1)}).search
	rec := fixture.run
	rec.State = types.RunRunning
	text = rt2.researchBriefOpening(context.Background(), &rec).openingText()
	if !strings.Contains(text, "your assigned work item") || strings.Contains(text, "for your objective") || !strings.Contains(text, "(objective of your assigned work item)") {
		t.Fatalf("assignment wording:\n%s", text)
	}
}

func TestBriefDoesNotBriefAnAnsweredRoundsCastOnALaterWake(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer, doc := "owner-obj-9", rt.cfg.ComputerID, "doc-obj-9"
	seedNumberedCast(t, s, owner, computer, doc, "texture:"+doc, "obj9a", "Did the harbor strike end?")
	readThrough(t, s, owner, computer, doc, 0, 1)
	closeRoundOfDoc(t, s, owner, computer, doc, "obj9")
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	rec := briefRunRecord(owner, computer, doc, "run-obj-9")
	b := rt.researchBriefOpening(context.Background(), rec)
	briefCommitFresh(t, rt, s, rec, b)
	if len(probe.queries) != 0 || briefFromLine(logs.String()) != "none" {
		t.Fatalf("queries=%v log=%s: the round was answered", probe.queries, logs.String())
	}
}

func TestBriefPrefersTheRunsOpenAssignmentToTheConversationsCast(t *testing.T) {
	rt, s := testRuntime(t)
	owner := "owner-obj-10"
	fixture := seedTextureLifecycleControl(t, s, owner, "obj10", agentprofile.Research+":control-obj10", agentprofile.Research)
	rec := fixture.run
	rec.State = types.RunRunning
	// The fixture's research agent is research:control-obj10; its conversation
	// is with the document's Texture desk.
	agent, computer := strings.TrimPrefix(rec.AgentID, "research:"), rec.ComputerID
	seedNumberedCast(t, s, owner, computer, agent, "texture:"+rec.ChannelID, "obj10a", "An older question about ferries")
	readThrough(t, s, owner, computer, agent, 0, 1)
	probe := &briefProbe{result: briefResult(nil, 1)}
	rt.researchBriefSearch = probe.search
	logs := briefCaptureLog(t)
	b := rt.researchBriefOpening(context.Background(), &rec)
	briefCommitFresh(t, rt, s, &rec, b)
	if len(probe.queries) != 1 || probe.queries[0] != "research obj10" || briefFromLine(logs.String()) != "assignment" {
		t.Fatalf("queries=%v log=%s", probe.queries, logs.String())
	}
}
