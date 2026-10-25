package agentcore

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// jobReceipt mints a job's receipt the way the executor does
// (capsule.MintJobReceipt), for a job that ended in state with exit.
func jobReceipt(t *testing.T, id, state string, exit int, startTree, endTree string) capsule.ExecutionReceipt {
	t.Helper()
	started := time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC)
	r, _, err := capsule.MintJobReceipt(capsule.JobReceiptInput{
		AgentRunID: "run-1", Handle: "h-1", CapsuleID: "capsule-1", SourceTreeDigest: "src",
		StartWorktreeDigest: startTree, EndWorktreeDigest: endTree, ObservedAt: started.Add(3 * time.Minute),
		Status: capsulejob.Status{ID: id, State: state, ExitCode: &exit, Argv: []string{"go", "build", "./cmd/autoputer"},
			Cwd: capsulejob.DefaultCwd, StartedAt: started, FinishedAt: started.Add(2 * time.Minute), OutputSHA256: "abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Freeze's evidence check (design F11, F12): a done job's receipt satisfies
// the build and test slots beside a toolchain cell; a failed, timed-out or
// stopped job's receipt is refused in every slot and named by slot; a job
// whose tree changed while it ran is refused for build and test only; a
// cell that panicked (exit 1) is refused.
func TestFreezeReceiptOutcomes(t *testing.T) {
	build := jobReceipt(t, "job-build", capsulejob.StateDone, 0, "tree", "tree")
	test := jobReceipt(t, "job-test", capsulejob.StateDone, 0, "tree", "tree")
	toolchain := capsule.ExecutionReceipt{ReceiptRef: "capsule-go-eval:sha256:tool", Command: "go_eval:x", ExitCode: 0}
	ok := []capsule.ExecutionReceipt{build, test, toolchain}
	if err := freezeReceiptOutcomesError(build.ReceiptRef, []string{test.ReceiptRef}, []string{toolchain.ReceiptRef}, ok); err != nil {
		t.Fatalf("done jobs refused: %v", err)
	}
	if strings.HasPrefix(build.ReceiptRef, "capsule-exec:sha256:") == false {
		t.Fatalf("job receipt prefix %q is not one Freeze's bundle validator accepts", build.ReceiptRef)
	}

	failed := jobReceipt(t, "job-build", capsulejob.StateFailed, 2, "tree", "tree")
	timeout := jobReceipt(t, "job-test", capsulejob.StateTimeout, -1, "tree", "tree")
	stopped := jobReceipt(t, "job-tool", capsulejob.StateStopped, 0, "tree", "tree")
	err := freezeReceiptOutcomesError(failed.ReceiptRef, []string{timeout.ReceiptRef}, []string{stopped.ReceiptRef}, []capsule.ExecutionReceipt{failed, timeout, stopped})
	if err == nil {
		t.Fatal("failed, timed-out and stopped jobs accepted")
	}
	for _, want := range []string{freezeFailedReceipts, "build_recipe_ref cites " + failed.ReceiptRef + ": job job-build ended failed (exit code 2)",
		"test_receipts[0] cites " + timeout.ReceiptRef + ": job job-test ended timeout", "dependency_toolchain_refs[0] cites " + stopped.ReceiptRef + ": job job-tool ended stopped"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q lacks %q", err, want)
		}
	}

	moved := jobReceipt(t, "job-test", capsulejob.StateDone, 0, "tree-before", "tree-after")
	if err := freezeReceiptOutcomesError(build.ReceiptRef, []string{moved.ReceiptRef}, []string{toolchain.ReceiptRef}, []capsule.ExecutionReceipt{build, moved, toolchain}); err == nil || !strings.Contains(err.Error(), "test_receipts[0]") || !strings.Contains(err.Error(), "workspace changed") {
		t.Fatalf("a test job on a moving tree: %v", err)
	}
	movedTool := jobReceipt(t, "job-tool", capsulejob.StateDone, 0, "tree-before", "tree-after")
	if err := freezeReceiptOutcomesError(build.ReceiptRef, []string{test.ReceiptRef}, []string{movedTool.ReceiptRef}, []capsule.ExecutionReceipt{build, test, movedTool}); err != nil {
		t.Fatalf("a toolchain job on a moving tree refused: %v", err)
	}
	panicked := capsule.ExecutionReceipt{ReceiptRef: "capsule-go-eval:sha256:panic", Command: "go_eval:y", ExitCode: 1}
	if err := freezeReceiptOutcomesError(build.ReceiptRef, []string{panicked.ReceiptRef}, []string{toolchain.ReceiptRef}, []capsule.ExecutionReceipt{build, panicked, toolchain}); err == nil || !strings.Contains(err.Error(), "exited 1") {
		t.Fatalf("a panicked test cell: %v", err)
	}
}

// Job intents are engineering's, and a start must name a runnable spec.
func TestValidateJobIntents(t *testing.T) {
	start := yaegikernel.StagedIntent{LocalID: "tray-1", Kind: yaegikernel.IntentJob, JobOp: yaegikernel.JobOpStart, JobID: "job-abc", Argv: []string{"go", "test", "./..."}}
	if err := validateCellIntents(ReductionScope{FromRole: "engineering"}, []yaegikernel.StagedIntent{start}); err != nil {
		t.Fatalf("engineering start refused: %v", err)
	}
	if err := validateCellIntents(ReductionScope{FromRole: "management"}, []yaegikernel.StagedIntent{start}); err == nil {
		t.Fatal("management staged a job")
	}
	bad := start
	bad.Argv = nil
	if err := validateCellIntents(ReductionScope{FromRole: "engineering"}, []yaegikernel.StagedIntent{bad}); err == nil {
		t.Fatal("a start without argv passed")
	}
	bad = start
	bad.JobID = "../x"
	if err := validateCellIntents(ReductionScope{FromRole: "engineering"}, []yaegikernel.StagedIntent{bad}); err == nil {
		t.Fatal("a bad job id passed")
	}
	// A declared memory need is checked again at the trust boundary (J13).
	bad = start
	bad.MemoryMiB = 1 << 20
	if err := validateCellIntents(ReductionScope{FromRole: "engineering"}, []yaegikernel.StagedIntent{bad}); err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("an out-of-range memory need passed: %v", err)
	}
	ok := start
	ok.MemoryMiB = 768
	if err := validateCellIntents(ReductionScope{FromRole: "engineering"}, []yaegikernel.StagedIntent{ok}); err != nil {
		t.Fatalf("a 768 MiB need refused: %v", err)
	}
	if req := jobStartRequest(ok, nil); req.MemoryMiB != 768 || req.ID != "job-abc" {
		t.Fatalf("job request = %+v, want the need carried", req)
	}
}

// A job's end commits one system record from job:<id> into the engineering
// inbox that started it (replay-safe), shown to the cell as kind "job" with
// its receipt and tail, and signals the run's wake.
func TestJobEndRecordReachesTheEngineeringInbox(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	receipt := jobReceipt(t, "job-build", capsulejob.StateDone, 0, "tree", "tree")
	exit := 0
	c := capsule.JobCompletion{AgentRunID: "run-1", CapsuleID: "capsule-1",
		Notify:  map[string]string{jobNotifyOwner: "owner-1", jobNotifyComputer: "computer-1", jobNotifyInbox: "engineering:asg-1", jobNotifyRun: "run-1"},
		View:    yaegikernel.JobView{ID: "job-build", Tail: "ok", LogRef: "capsule-job-log:sha256:abc"},
		Status:  capsulejob.Status{ID: "job-build", State: capsulejob.StateDone, ExitCode: &exit, Argv: []string{"go", "build", "./cmd/autoputer"}, Cwd: capsulejob.DefaultCwd},
		Receipt: &receipt}
	rt.handleCapsuleJobDone(c)
	rt.handleCapsuleJobDone(c) // a second delivery replays
	head, err := s.InboxHead(ctx, "owner-1", "computer-1", "engineering:asg-1")
	if err != nil || head != 1 {
		t.Fatalf("inbox head %d %v", head, err)
	}
	records, err := s.ListInboxRecords(ctx, "owner-1", "computer-1", "engineering:asg-1", 0, head)
	if err != nil || len(records) != 1 {
		t.Fatalf("records %+v %v", records, err)
	}
	view := inboxRecordView(records[0])
	if view.Kind != "job" || view.From != "job:job-build" || view.Job == nil || view.Job.ReceiptRef != receipt.ReceiptRef ||
		view.Job.State != capsulejob.StateDone || view.Job.Tail != "ok" || view.Job.LogRef == "" {
		t.Fatalf("view: %+v job %+v", view, view.Job)
	}
	select {
	case <-rt.jobWake("run-1"):
	default:
		t.Fatal("the run's wake was not signalled")
	}
	forged := records[0]
	forged.RecordID, forged.InboxSeq, forged.ConversationID = "job:job-x:ended", 0, ""
	forged.EmitterKind = types.CommitmentEmitterAgent
	req, _ := jobEndRecordRequest("owner-1", "computer-1", "engineering:asg-1", c)
	req.Records = []types.CommitmentRecord{forged}
	req.CommandID = "forged"
	if _, err := s.CommitInboxRecords(ctx, req); err == nil {
		t.Fatal("an agent-emitted job record was accepted")
	}
}

type fakeJobTable struct {
	mu      sync.Mutex
	running bool
	view    yaegikernel.JobView
}

func (f *fakeJobTable) RunningJob(string) (yaegikernel.JobView, time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.view, time.Now().Add(time.Minute), f.running
}

func (f *fakeJobTable) JobViews(string) []yaegikernel.JobView {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []yaegikernel.JobView{f.view}
}

// The completion guard holds a turn that ended while its job runs: a stale
// signal does not release it, the job's end does, and the turn resumes with
// the outcome as a wait, not a retry. With no running job it does nothing.
func TestEngineeringJobWaitResumesTheTurnWhenTheJobEnds(t *testing.T) {
	rt, _ := testRuntime(t)
	table := &fakeJobTable{running: true, view: yaegikernel.JobView{ID: "job-test", State: capsulejob.StateRunning, Argv: []string{"go", "test", "./..."}}}
	rt.jobTable = table
	rec := &types.RunRecord{RunID: "run-1"}
	deliveries := newJobDeliveries(nil)
	type result struct {
		res bool
		out string
	}
	done := make(chan result, 1)
	go func() {
		got, ok := rt.engineeringJobWait(context.Background(), rec, deliveries)
		done <- result{ok && got.Continue && got.Waited, got.Instruction}
	}()
	time.Sleep(50 * time.Millisecond)
	rt.signalJobWake("run-1") // stale: the job still runs
	select {
	case r := <-done:
		t.Fatalf("released by a stale signal: %+v", r)
	case <-time.After(100 * time.Millisecond):
	}
	exit := 0
	table.mu.Lock()
	table.running = false
	table.view = yaegikernel.JobView{ID: "job-test", State: capsulejob.StateDone, ExitCode: &exit, Argv: []string{"go", "test", "./..."}, ElapsedSec: 95, ReceiptRef: "capsule-exec:sha256:ok"}
	table.mu.Unlock()
	rt.signalJobWake("run-1")
	select {
	case r := <-done:
		if !r.res || !strings.Contains(r.out, "job-test") || !strings.Contains(r.out, "state done, exit code 0") || !strings.Contains(r.out, "capsule-exec:sha256:ok: cite it in choir.Freeze") {
			t.Fatalf("resume: %+v", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the job's end did not resume the turn")
	}
	if _, ok := rt.engineeringJobWait(context.Background(), rec, deliveries); ok {
		t.Fatal("waited with no running job")
	}
	ctx, cancel := context.WithCancel(context.Background())
	table.mu.Lock()
	table.running = true
	table.mu.Unlock()
	cancel()
	if _, ok := rt.engineeringJobWait(ctx, rec, deliveries); ok {
		t.Fatal("a cancelled run kept waiting")
	}
}
