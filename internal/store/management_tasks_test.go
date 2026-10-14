package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management tasks in the store (management delegation S2a, design round 2
// R2-3, R2-4, R2-5, R2-7, R2-8, R2-17). Ways it can fail, written before the
// code:
//   - a capture is delegated to two tasks at once (FM4), or the parent
//     disposes of a capture a task holds;
//   - a task disposes of a capture twice, or of one it does not lease
//     (FM5, FM12), because the check ran before the commit lock (B5);
//   - a late commit from a task lands after the closure that freed its
//     lease (FM22);
//   - a record is sent from management-task:<id> by the model, or a task's
//     record carries no acting_task, so its acts look like the parent's
//     (FM6);
//   - the task's final Report claims "clean" with captures undisposed, so a
//     task that dropped captures never wakes the parent (B9);
//   - a clean Report wakes the parent (FM2) or a residual one does not;
//   - a crash closure wakes anyone, or does not count toward the capture's
//     crash budget, so a capture that kills the guest is re-delegated
//     forever (FM26);
//   - a task ending leaves the next queued task with no durable start;
//   - two opens of one event: a Cast to texture:new from a run that has not
//     seen the latest story is not refused (FM3).

const taskParent = "management:" + inboxTestOwner

func taskArrival(t *testing.T, s *Store, source string, captures ...string) types.CommitmentRecord {
	t.Helper()
	var items []types.CaptureArrival
	for _, c := range captures {
		items = append(items, types.CaptureArrival{CaptureID: c, Title: "title " + c, URL: "https://x.example/" + c})
	}
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		RecordID:    types.CaptureArrivalRecordID(source, captures),
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, Inbox: taskParent, SenderInbox: types.CaptureArrivalSenderInbox(source),
		Captures: &types.CommitmentCaptures{SourceID: source, Items: items},
	}
	mustCommitInbox(t, s, "arrival:"+rec.RecordID, nil, rec)
	return rec
}

// delegation builds a parent's delegation Cast and its task row.
func delegation(taskID string, arrival types.CommitmentRecord, captures ...string) (types.CommitmentRecord, types.ManagementTask) {
	inbox := types.ManagementTaskInbox(taskID)
	cast := inboxCast("cell-parent:delegate:"+taskID, taskParent, inbox, types.CommitmentCast{Captures: captures})
	cast.RelatedIDs = []string{arrival.RecordID}
	now := time.Now().UTC()
	row := types.ManagementTask{
		TaskID: taskID, OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, Kind: types.ManagementTaskIntake,
		Objective: "dispose", ParentInbox: taskParent, ParentRunID: "run-parent", TaskInbox: inbox,
		ConversationID: types.InboxConversationID(taskParent, inbox), OpeningRecordID: cast.RecordID,
		DelegatedRecordIDs: []string{arrival.RecordID}, LeasedCaptures: captures, DeadlineSeconds: 600,
		State: types.ManagementTaskQueued, CreatedAt: now, UpdatedAt: now,
	}
	return cast, row
}

func commitTaskRequest(t *testing.T, s *Store, req types.CommitInboxRecordsRequest) (types.InboxCommitReceipt, error) {
	t.Helper()
	req.OwnerID, req.ComputerID = inboxTestOwner, inboxTestComputer
	digest, err := ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	return s.CommitInboxRecords(context.Background(), req)
}

func delegate(t *testing.T, s *Store, cast types.CommitmentRecord, row types.ManagementTask) error {
	t.Helper()
	_, err := commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "delegate:" + row.TaskID, Records: []types.CommitmentRecord{cast}, Tasks: []types.ManagementTask{row}})
	return err
}

// taskFeed is a task's feed Note, sent as management with the task named.
func taskFeed(id, taskID, runID, doc string, captures ...string) types.CommitmentRecord {
	rec := inboxNote(id, taskParent, "texture:"+doc)
	rec.Provenance.AgentID = types.ManagementTaskInbox(taskID)
	rec.Provenance.ActingTask, rec.Provenance.ActingRunID = taskID, runID
	rec.Disposition = &types.CommitmentDisposition{Kind: types.DispositionFeed, Captures: captures, Reason: "adds detail"}
	return rec
}

func parentFeed(id, doc string, captures ...string) types.CommitmentRecord {
	rec := inboxNote(id, taskParent, "texture:"+doc)
	rec.Disposition = &types.CommitmentDisposition{Kind: types.DispositionFeed, Captures: captures, Reason: "parent feed"}
	return rec
}

func taskClosing(row types.ManagementTask, reason, note string) types.CommitmentRecord {
	return types.NewTaskClosingReport(row, reason, note, "", "run-"+row.TaskID, time.Now().UTC().Format(time.RFC3339Nano))
}

func startedTask(t *testing.T, s *Store, taskID string) types.ManagementTask {
	t.Helper()
	ctx := context.Background()
	claimed, err := s.ClaimManagementTask(ctx, inboxTestOwner, inboxTestComputer, taskID, time.Now().UTC())
	if err != nil {
		t.Fatalf("claim %s: %v", taskID, err)
	}
	if claimed.State != types.ManagementTaskRunning {
		t.Fatalf("claimed row = %+v", claimed)
	}
	bound, err := s.BindManagementTaskRun(ctx, inboxTestOwner, inboxTestComputer, taskID, "run-"+taskID)
	if err != nil {
		t.Fatalf("bind %s: %v", taskID, err)
	}
	return bound
}

func wakesTo(t *testing.T, s *Store, target string) []ActorWakeOutbox {
	t.Helper()
	var out []ActorWakeOutbox
	for _, w := range storyWakes(t, s) {
		if w.TargetAgentID == target {
			out = append(out, w)
		}
	}
	return out
}

func TestTaskLeaseRefusesSecondDelegationAndParentDisposition(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	arrival := taskArrival(t, s, "rss:lease", "cap-l1", "cap-l2")
	cast, row := delegation("task-l1", arrival, "cap-l1", "cap-l2")
	if err := delegate(t, s, cast, row); err != nil {
		t.Fatalf("delegate: %v", err)
	}
	stored, err := s.GetManagementTask(ctx, inboxTestOwner, inboxTestComputer, "task-l1")
	if err != nil || stored.State != types.ManagementTaskQueued || stored.Round != 1 {
		t.Fatalf("task row = %+v, %v", stored, err)
	}
	// The opening Cast wakes the task inbox: the durable start.
	if w := wakesTo(t, s, row.TaskInbox); len(w) != 1 || w[0].Kind != InboxRecordWakeKind {
		t.Fatalf("task start wakes = %+v", w)
	}
	// FM4: a second task cannot lease a leased capture.
	cast2, row2 := delegation("task-l2", arrival, "cap-l2")
	if err := delegate(t, s, cast2, row2); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "task-l1") {
		t.Fatalf("second delegation of a leased capture: %v", err)
	}
	// FM4: nor can the parent dispose of it while the round is open.
	if _, err := commitInbox(t, s, "parent-feed-leased", nil, parentFeed("parent-feed-1", "doc-a", "cap-l1")); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "leased") {
		t.Fatalf("parent disposition of a leased capture: %v", err)
	}
	// A row whose Cast does not name its leased captures is refused.
	cast3, row3 := delegation("task-l3", arrival, "cap-l9")
	row3.LeasedCaptures = []string{"cap-l8"}
	if err := delegate(t, s, cast3, row3); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("a row whose captures differ from its Cast: %v", err)
	}
	// A Cast to a task inbox without its row is refused.
	stray := inboxCast("stray-cast", taskParent, types.ManagementTaskInbox("task-stray"), types.CommitmentCast{Captures: []string{"cap-x"}})
	mustRefuseInbox(t, s, "stray", stray)

	// After the round closes the lease is free: the parent's disposition
	// is the effective one.
	startedTask(t, s, "task-l1")
	mustCommitInbox(t, s, "close-l1", nil, taskClosing(stored, types.TaskClosureDone, "left both"))
	if _, err := commitInbox(t, s, "parent-feed-free", nil, parentFeed("parent-feed-2", "doc-a", "cap-l1")); err != nil {
		t.Fatalf("parent feed after the lease freed: %v", err)
	}
	d, found, err := s.CaptureDispositionOf(ctx, inboxTestOwner, inboxTestComputer, "cap-l1")
	if err != nil || !found || d.Kind != types.DispositionFeed || d.RecordID != "parent-feed-2" || d.ActingTask != "" {
		t.Fatalf("index of cap-l1 = %+v, %v, %v", d, found, err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestTaskDispositionIsLeasedOnceAndRefusedAfterClosure(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	arrival := taskArrival(t, s, "rss:once", "cap-o1", "cap-o2", "cap-o3")
	cast, row := delegation("task-o", arrival, "cap-o1", "cap-o2")
	if err := delegate(t, s, cast, row); err != nil {
		t.Fatal(err)
	}
	// A queued task commits nothing: its round has no run.
	if _, err := commitInbox(t, s, "queued-feed", nil, taskFeed("queued-feed", "task-o", "run-task-o", "doc-a", "cap-o1")); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("a queued task's feed: %v", err)
	}
	startedTask(t, s, "task-o")
	if _, err := commitInbox(t, s, "task-feed-1", nil, taskFeed("task-feed-1", "task-o", "run-task-o", "doc-a", "cap-o1")); err != nil {
		t.Fatalf("task feed of a leased capture: %v", err)
	}
	// FM12: a second disposition of the same capture by a task.
	if _, err := commitInbox(t, s, "task-feed-2", nil, taskFeed("task-feed-2", "task-o", "run-task-o", "doc-b", "cap-o1")); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "already") {
		t.Fatalf("second task disposition: %v", err)
	}
	// FM5: a capture the task does not lease.
	if _, err := commitInbox(t, s, "task-feed-3", nil, taskFeed("task-feed-3", "task-o", "run-task-o", "doc-a", "cap-o3")); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "not leased") {
		t.Fatalf("task disposition outside its lease: %v", err)
	}
	// FM6: a task's record without acting_task, or a record from the task
	// inbox the host did not mint.
	bare := taskFeed("task-bare", "task-o", "run-task-o", "doc-a", "cap-o2")
	bare.Provenance.ActingTask = ""
	mustRefuseInbox(t, s, "task-bare", bare)
	forged := inboxReport("forged-report", row.TaskInbox, taskParent, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "all clean"}}})
	mustRefuseInbox(t, s, "forged", forged)
	// The wrong run.
	if _, err := commitInbox(t, s, "task-wrong-run", nil, taskFeed("task-wrong-run", "task-o", "run-other", "doc-a", "cap-o2")); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("a record naming another run: %v", err)
	}

	// The host's closing Report: residuals are computed from the index,
	// not claimed. cap-o2 was left.
	stored, _ := s.GetManagementTask(ctx, inboxTestOwner, inboxTestComputer, "task-o")
	closing := taskClosing(stored, types.TaskClosureDone, "cap-o2 is ambiguous")
	closing.TaskOutcome.Clean = true // a forged claim the store must not believe
	receipt := mustCommitInbox(t, s, "close-o", nil, closing)
	if receipt.Records[0].WakeID == "" || !receipt.Records[0].ClosedRound {
		t.Fatalf("a residual Report must close the round and wake the parent: %+v", receipt.Records[0])
	}
	got, _ := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, taskParent, 0, 0)
	report := got[len(got)-1]
	o := report.TaskOutcome
	if o == nil || o.Clean || len(o.Residuals) != 1 || o.Residuals[0].CaptureID != "cap-o2" || o.Residuals[0].Reason != "cap-o2 is ambiguous" ||
		len(o.Dispositions) != 1 || o.Dispositions[0].CaptureID != "cap-o1" || o.Dispositions[0].RecordID != "task-feed-1" {
		t.Fatalf("task outcome = %+v", o)
	}
	if !RecordWakesReader(report) {
		t.Fatal("RecordWakesReader: a residual Report must wake")
	}
	closed, _ := s.GetManagementTask(ctx, inboxTestOwner, inboxTestComputer, "task-o")
	if closed.State != types.ManagementTaskClosed || closed.ClosureReason != types.TaskClosureDone || closed.ClosingRecordID != report.RecordID {
		t.Fatalf("closed row = %+v", closed)
	}
	// FM22: a late commit after the closure freed the lease.
	if _, err := commitInbox(t, s, "task-late", nil, taskFeed("task-late", "task-o", "run-task-o", "doc-a", "cap-o2")); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("late task commit after closure: %v", err)
	}
	// A second closure of the same round is refused.
	mustRefuseInbox(t, s, "close-o-again", taskClosing(stored, "run_completed", ""))
	assertInboxHeadsMatchRebuild(t, s)
}

func TestCleanTaskReportDoesNotWakeAndStartsTheNextQueuedTask(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	arrival := taskArrival(t, s, "rss:clean", "cap-c1", "cap-c2", "cap-c3")
	castA, rowA := delegation("task-a", arrival, "cap-c1")
	castB, rowB := delegation("task-b", arrival, "cap-c2")
	castC, rowC := delegation("task-c", arrival, "cap-c3")
	if _, err := commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "delegate-abc", Records: []types.CommitmentRecord{castA, castB, castC},
		Tasks: []types.ManagementTask{rowA, rowB, rowC}}); err != nil {
		t.Fatalf("batch delegation: %v", err)
	}
	queued, err := s.ListManagementTasks(ctx, inboxTestOwner, inboxTestComputer, types.ManagementTaskQueued)
	if err != nil || len(queued) != 3 {
		t.Fatalf("queued tasks = %+v, %v", queued, err)
	}
	startedTask(t, s, "task-a")
	if _, err := commitInbox(t, s, "a-open", nil, func() types.CommitmentRecord {
		rec := openingCast("a-open-cast")
		rec.Cast.Captures = []string{"cap-c1"}
		rec.Provenance.AgentID, rec.Provenance.ActingTask, rec.Provenance.ActingRunID = rowA.TaskInbox, "task-a", "run-task-a"
		return rec
	}()); err != nil {
		t.Fatalf("task open: %v", err)
	}
	if d, _, _ := s.CaptureDispositionOf(ctx, inboxTestOwner, inboxTestComputer, "cap-c1"); d.Kind != types.DispositionOpen || d.ActingTask != "task-a" {
		t.Fatalf("index after an open = %+v", d)
	}
	stored, _ := s.GetManagementTask(ctx, inboxTestOwner, inboxTestComputer, "task-a")
	receipt := mustCommitInbox(t, s, "close-a", nil, taskClosing(stored, types.TaskClosureDone, ""))
	if receipt.Records[0].WakeID != "" {
		t.Fatalf("FM2: a clean Report minted a wake for the parent: %+v", receipt.Records[0])
	}
	got, _ := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, taskParent, 0, 0)
	if clean := got[len(got)-1]; clean.TaskOutcome == nil || !clean.TaskOutcome.Clean || RecordWakesReader(clean) {
		t.Fatalf("clean Report = %+v", clean.TaskOutcome)
	}
	// The closure started the oldest queued task (task-b, delegated in
	// the same commit, ordered by row) with a durable wake.
	if w := wakesTo(t, s, rowB.TaskInbox); len(w) != 2 {
		t.Fatalf("task-b wakes = %+v; want its opening wake and the start wake", w)
	}
	if w := wakesTo(t, s, rowC.TaskInbox); len(w) != 1 {
		t.Fatalf("task-c wakes = %+v; want only its opening wake", w)
	}
}

func TestCrashClosureOfATaskWakesNoOneAndSpendsTheCrashBudget(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	arrival := taskArrival(t, s, "rss:crash", "cap-k1", "cap-k2")
	crash := func(taskID string, wantFate string) {
		t.Helper()
		cast, row := delegation(taskID, arrival, "cap-k1", "cap-k2")
		if err := delegate(t, s, cast, row); err != nil {
			t.Fatalf("delegate %s: %v", taskID, err)
		}
		if taskID == "task-k1" {
			startedTask(t, s, taskID)
			if _, err := commitInbox(t, s, "k1-feed", nil, taskFeed("k1-feed", taskID, "run-"+taskID, "doc-k", "cap-k2")); err != nil {
				t.Fatalf("feed: %v", err)
			}
		}
		closure := types.NewRoundClosureRecord(taskParent, row.TaskInbox, 1, types.ClosureReasonInterruptedByRestart, time.Now().UTC().Format(time.RFC3339Nano))
		closure.ComputerID = inboxTestComputer
		receipt := mustCommitInbox(t, s, "boot-closure:"+closure.RecordID, nil, closure)
		if receipt.Records[0].WakeID != "" {
			t.Fatalf("a crash closure woke the parent: %+v", receipt.Records[0])
		}
		closed, _ := s.GetManagementTask(ctx, inboxTestOwner, inboxTestComputer, taskID)
		if closed.State != types.ManagementTaskClosed || closed.ClosureReason != types.ClosureReasonInterruptedByRestart {
			t.Fatalf("crash-closed row = %+v", closed)
		}
		d, _, _ := s.CaptureDispositionOf(ctx, inboxTestOwner, inboxTestComputer, "cap-k1")
		if d.Fate != wantFate {
			t.Fatalf("cap-k1 after %s: %+v, want fate %q", taskID, d, wantFate)
		}
	}
	crash("task-k1", "")
	// cap-k2 was fed before the crash: it is disposed and spends nothing.
	if d, _, _ := s.CaptureDispositionOf(ctx, inboxTestOwner, inboxTestComputer, "cap-k2"); d.CrashCount != 0 || d.Kind != types.DispositionFeed {
		t.Fatalf("cap-k2 = %+v", d)
	}
	// cap-k2 is disposed, so it cannot be delegated again; cap-k1 can,
	// once more.
	cast, row := delegation("task-k-disposed", arrival, "cap-k2")
	if err := delegate(t, s, cast, row); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "disposed") {
		t.Fatalf("delegating a disposed capture: %v", err)
	}
	castK2, rowK2 := delegation("task-k2", arrival, "cap-k1")
	if err := delegate(t, s, castK2, rowK2); err != nil {
		t.Fatalf("second delegation after one crash: %v", err)
	}
	closure := types.NewRoundClosureRecord(taskParent, rowK2.TaskInbox, 1, types.ClosureReasonInterruptedByRestart, time.Now().UTC().Format(time.RFC3339Nano))
	mustCommitInbox(t, s, "boot-closure:"+closure.RecordID, nil, closure)
	if d, _, _ := s.CaptureDispositionOf(ctx, inboxTestOwner, inboxTestComputer, "cap-k1"); d.CrashCount != 2 || d.Fate != types.CaptureFateClosedAfterCrashes {
		t.Fatalf("cap-k1 after two crashes = %+v", d)
	}
	cast3, row3 := delegation("task-k3", arrival, "cap-k1")
	if err := delegate(t, s, cast3, row3); !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "two crashes") {
		t.Fatalf("FM26: delegating a capture closed after two crashes: %v", err)
	}
}

func TestRecordWakesReaderIsTheOnePredicate(t *testing.T) {
	agent := inboxNote("n", taskParent, "texture:doc")
	if !RecordWakesReader(agent) || !inboxRecordWakes(agent) {
		t.Fatal("an agent record wakes its reader")
	}
	decline := inboxNote("d", taskParent, "source:rss:x")
	if RecordWakesReader(decline) {
		t.Fatal("a source inbox has no reader")
	}
	crash := types.NewRoundClosureRecord("texture:doc", taskParent, 1, types.ClosureReasonInterruptedByRestart, "")
	if RecordWakesReader(crash) {
		t.Fatal("a crash closure wakes no one")
	}
	row := types.ManagementTask{TaskID: "t", OwnerID: inboxTestOwner, ParentInbox: taskParent, TaskInbox: types.ManagementTaskInbox("t"), Kind: types.ManagementTaskIntake}
	clean := types.NewTaskClosingReport(row, types.TaskClosureDone, "", "", "", "")
	clean.TaskOutcome.Clean = true
	if RecordWakesReader(clean) {
		t.Fatal("a clean task Report does not wake")
	}
	residual := types.NewTaskClosingReport(row, types.TaskClosureDeadline, "", "", "", "")
	if !RecordWakesReader(residual) {
		t.Fatal("a residual task Report wakes")
	}
	for id, rec := range map[string]types.CommitmentRecord{"agent": agent, "decline": decline, "crash": crash, "clean": clean, "residual": residual} {
		if RecordWakesReader(rec) != inboxRecordWakes(rec) {
			t.Fatalf("%s: the store's wake rule and RecordWakesReader disagree", id)
		}
	}
}

func TestStoryLibraryHeadRefusesAStaleOpen(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	head := func() int64 {
		h, err := s.StoryLibraryHeadOf(ctx, inboxTestOwner, inboxTestComputer)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	seen := int64(0)
	first := openingCast("lib-open-1")
	receipt, err := commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "lib-1", Records: []types.CommitmentRecord{first}, StoryLibraryHead: &seen})
	if err != nil {
		t.Fatalf("first open at head 0: %v", err)
	}
	if head() != 1 {
		t.Fatalf("head after one open = %d", head())
	}
	// FM28 (S2b): a checked open's receipt carries the head after it; the
	// run saw everything up to it, its own open included.
	if receipt.StoryLibraryHead != 1 {
		t.Fatalf("checked open receipt head = %d, want 1", receipt.StoryLibraryHead)
	}
	// FM3: a second run that saw head 0 opens the same event.
	second := openingCast("lib-open-2")
	_, err = commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "lib-2", Records: []types.CommitmentRecord{second}, StoryLibraryHead: &seen})
	if !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "Harbor reopens after the storm") || !strings.Contains(err.Error(), "capture-a") {
		t.Fatalf("stale open: %v; want a refusal naming the story opened since", err)
	}
	// Having seen head 1, it opens (a distinct event).
	current := head()
	if receipt, err = commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "lib-3", Records: []types.CommitmentRecord{second}, StoryLibraryHead: &current}); err != nil {
		t.Fatalf("open at the current head: %v", err)
	}
	if receipt.StoryLibraryHead != 2 {
		t.Fatalf("second checked open receipt head = %d, want 2", receipt.StoryLibraryHead)
	}
	// A caller that names no head (not a management cell) is unchecked,
	// and its receipt names no head: nothing was seen.
	third := openingCast("lib-open-3")
	if receipt, err = commitTaskRequest(t, s, types.CommitInboxRecordsRequest{CommandID: "lib-4", Records: []types.CommitmentRecord{third}}); err != nil {
		t.Fatalf("unchecked open: %v", err)
	}
	if receipt.StoryLibraryHead != 0 {
		t.Fatalf("unchecked open receipt head = %d, want 0", receipt.StoryLibraryHead)
	}
	if head() != 3 {
		t.Fatalf("head = %d, want 3", head())
	}
}
