package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management delegation S2a through the real reduce and commit paths
// (docs/archive/reports/management-delegation-design-2026-10-14.md §11, round 2).
// Ways it can fail, written before the code (numbers are the note's):
//
//	FM1  the parent waits on a task (a barrier, or a commit that waits for
//	     the task's run);
//	FM2  task Reports wake the parent on every task; FM21 the post-turn
//	     re-check re-arms the parent on clean Reports;
//	FM6  a task's acts look like the parent's, with no task named;
//	FM9  a crash boot resumes or restarts a task; AC6 a crash leaves task
//	     runs or rounds open, or wakes anyone;
//	FM11 a hung task holds its lease past its deadline;
//	FM19 a task run misses a profile-keyed switch: no route, no wake, no
//	     closure, no re-check;
//	FM23 a burst starts unbounded runs (K = 1 in S2a);
//	FM24 a running task is left passivated with its round open at a
//	     planned boot;
//	FM27 the task runs on the parent's pool rather than the pinned model;
//	R2-20 CancelTask leaves the captures leased, or cannot be followed by
//	     a re-delegation;
//	N5   a task's disposition against Jev's label records no override.

// delegationModel scripts both desks: the parent's turn runs one cell (its
// parentCell) and ends; a task's turn runs taskCell, whose Done ends the
// run. A task's calls block on gate while it is set.
type delegationModel struct {
	*provider.StubProvider
	mu         sync.Mutex
	parentCell string
	taskCell   string
	gate       chan struct{}
	taskModels []string
	calls      int
	outputs    []string
}

func (m *delegationModel) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	task := strings.Contains(req.System, "You are a management task")
	m.mu.Lock()
	cell, gate := m.parentCell, m.gate
	who := "parent"
	if task {
		cell, who = m.taskCell, "task"
		m.taskModels = append(m.taskModels, req.Provider+"/"+req.Model)
	}
	m.calls++
	callID := fmt.Sprintf("call-%s-%d", who, m.calls)
	m.mu.Unlock()
	if task && gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	// A turn's first call (its last message is not a tool result) runs
	// the cell; the call after the cell's result ends the turn. The
	// parent's memory spans its turns, so the history is not a guide.
	last := ""
	if n := len(req.Messages); n > 0 {
		last = string(req.Messages[n-1])
	}
	if !strings.Contains(last, "tool_result") {
		args, _ := json.Marshal(map[string]any{"source": cell})
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "test-model",
			ToolCalls: []types.ToolCall{{ID: callID, Name: "desk_go_eval", Arguments: args}}}, nil
	}
	m.mu.Lock()
	m.outputs = append(m.outputs, last)
	m.mu.Unlock()
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
}

func (m *delegationModel) lastOutput() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.outputs) == 0 {
		return ""
	}
	return m.outputs[len(m.outputs)-1]
}

const delegateAllCell = `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { ids = append(ids, r.RecordID) } }
choir.Delegate("intake", "", ids, "per_record")`

// taskCellFor disposes by title: a "Pope" capture feeds the pope story, a
// "leave" capture is left for management, the rest are declined.
func taskCellFor(popeDoc string) string {
	return `var decline []string
for _, r := range choir.Records() {
	if r.Captures == nil { continue }
	for _, c := range r.Captures.Items {
		if strings.Contains(c.Title, "Pope") {
			choir.Feed("texture:` + popeDoc + `", []string{c.CaptureID}, "the pope's recovery, not the label's story")
		} else if !strings.Contains(c.Title, "leave") {
			decline = append(decline, c.CaptureID)
		}
	}
}
if len(decline) > 0 { choir.Decline(decline, "no news event") }
choir.Done("the capture titled leave needs management")`
}

type delegationFixture struct {
	t        *testing.T
	rt       *Runtime
	s        *store.Store
	model    *delegationModel
	owner    string
	computer string
	pope     string
	mu       sync.Mutex
	wakes    []string
	// deliverTaskWakes delivers inbox_record wakes to task inboxes (the
	// slot starter's durable starts) when the outbox is swept.
	deliverTaskWakes bool
}

// newDelegationFixture runs at K = 1, S2a's slots, unless a test sets
// rt.cfg.ManagementTaskSlots before it delegates (S2b).
func newDelegationFixture(t *testing.T, owner string) *delegationFixture {
	t.Helper()
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	rt.cfg.ManagementTaskSlots = 1
	// Task model calls are spaced on the sub-RLM spacer (S2b, FM14); a
	// short interval keeps the fixture fast.
	rt.researchChildCalls = &callSpacer{interval: time.Millisecond}
	f := &delegationFixture{t: t, rt: rt, s: s, owner: owner, computer: rt.cfg.ComputerID}
	f.model = &delegationModel{StubProvider: provider.NewStubProvider(0), parentCell: delegateAllCell}
	rt.provider = f.model
	if err := rt.InstallDefaultAgentTools(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: f.computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	f.pope = openTestStory(t, s, owner, f.computer, "Pope has lung surgery")
	f.model.taskCell = taskCellFor(f.pope)
	rt.SetDispatchActor(func(dctx context.Context, ownerID, computerID, to, kind, content, _, _ string) error {
		switch kind {
		case "initial_dispatch":
			go func() {
				rec, err := s.GetRunByOwner(dctx, ownerID, strings.TrimSpace(content))
				if err == nil {
					rt.ExecuteActivationSync(dctx, &rec)
				}
			}()
		case "inbox_record":
			f.mu.Lock()
			f.wakes = append(f.wakes, to)
			deliver := f.deliverTaskWakes
			f.mu.Unlock()
			if _, task := types.ManagementTaskIDOfInbox(to); task && deliver {
				go func() { _, _ = rt.WakeRecordsInbox(context.WithoutCancel(dctx), ownerID, computerID, to) }()
			}
		}
		return nil
	})
	rt.SetScheduleActor(func(context.Context, string, string, string, string, string, string, string, time.Time) error {
		return nil
	})
	return f
}

func (f *delegationFixture) management() string { return persistentManagementAgentID(f.owner) }

// parentTurn wakes management and waits for its turn to end.
func (f *delegationFixture) parentTurn() types.RunRecord {
	f.t.Helper()
	rec, err := f.rt.WakeRecordsInbox(context.Background(), f.owner, f.computer, f.management())
	if err != nil || rec == nil {
		f.t.Fatalf("wake management: %v, %v", rec, err)
	}
	done := waitForStoredRunTerminalState(f.t, f.s, rec.RunID, 60*time.Second)
	if done.State != types.RunCompleted {
		f.t.Fatalf("parent turn: %s (%s)", done.State, done.Error)
	}
	return done
}

func (f *delegationFixture) tasks(state types.ManagementTaskState) []types.ManagementTask {
	f.t.Helper()
	rows, err := f.s.ListManagementTasks(context.Background(), f.owner, f.computer, state)
	if err != nil {
		f.t.Fatal(err)
	}
	return rows
}

func (f *delegationFixture) task(id string) types.ManagementTask {
	f.t.Helper()
	row, err := f.s.GetManagementTask(context.Background(), f.owner, f.computer, id)
	if err != nil {
		f.t.Fatalf("task %s: %v", id, err)
	}
	return row
}

// closingReport is the task's final Report in management's inbox.
func (f *delegationFixture) closingReport(task types.ManagementTask) types.CommitmentRecord {
	f.t.Helper()
	recs, _ := f.s.ListInboxRecords(context.Background(), f.owner, f.computer, f.management(), 0, 0)
	for _, rec := range recs {
		if rec.SenderInbox == task.TaskInbox && rec.Final {
			return rec
		}
	}
	f.t.Fatalf("no final Report from %s in %s", task.TaskInbox, f.management())
	return types.CommitmentRecord{}
}

// outboxWakes lists unprojected wakes to target whose content names
// needle.
func (f *delegationFixture) outboxWakes(target, needle string) int {
	f.t.Helper()
	wakes, err := f.s.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	n := 0
	for _, w := range wakes {
		if w.TargetAgentID == target && strings.Contains(w.Content, needle) {
			n++
		}
	}
	return n
}

// recheckWakes runs the post-turn re-check for a parent turn that began at
// began and returns how many inbox_record wakes it dispatched to the
// parent.
func (f *delegationFixture) recheckWakes(began time.Time) int {
	f.t.Helper()
	f.mu.Lock()
	f.wakes = nil
	f.mu.Unlock()
	now := time.Now().UTC()
	f.rt.recheckRecordsInbox(context.Background(), &types.RunRecord{RunID: fmt.Sprintf("parent-turn-%d", now.UnixNano()), AgentID: f.management(),
		OwnerID: f.owner, ComputerID: f.computer, AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		State: types.RunCompleted, CreatedAt: began, FinishedAt: &now})
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, to := range f.wakes {
		if to == f.management() {
			n++
		}
	}
	return n
}

func TestManagementDelegatesIntakeToTasks(t *testing.T) {
	f := newDelegationFixture(t, "owner-delegate")
	ctx := context.Background()
	seedArrival(t, f.s, f.owner, f.computer, "rss:a", []types.CaptureArrival{
		{CaptureID: "cap-a1", Title: "Pope recovers", URL: "https://a.example/1"},
		{CaptureID: "cap-a2", Title: "Newsletter", URL: "https://a.example/2"},
	})
	seedArrival(t, f.s, f.owner, f.computer, "rss:b", []types.CaptureArrival{
		{CaptureID: "cap-b1", Title: "please leave this", URL: "https://b.example/1"},
		{CaptureID: "cap-b2", Title: "Listing", URL: "https://b.example/2"},
	})
	// Jev labels cap-a1 as another story's event; the task feeds the pope
	// story, which is an override (N5).
	trial := openTestStory(t, f.s, f.owner, f.computer, "Nord Stream trial opens")
	if _, err := f.s.AppendCaptureLabel(ctx, store.CaptureLabel{OwnerID: f.owner, ComputerID: f.computer, CaptureID: "cap-a1", Title: "Pope recovers",
		Event: store.CaptureLabelEventStory, EventDocID: trial, EventHeadline: "Nord Stream trial opens", EventP: 0.6, Model: "jev"}); err != nil {
		t.Fatal(err)
	}

	// The parent's turn delegates both arrivals in one cell and ends: it
	// starts no task and waits for none (FM1).
	f.parentTurn()
	if out := f.model.lastOutput(); strings.Count(out, "rlm:delegate:") != 2 || strings.Count(out, "to management-task:") != 2 {
		t.Fatalf("parent cell receipts: %s", out)
	}
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 || len(f.tasks(types.ManagementTaskRunning)) != 0 {
		t.Fatalf("after delegation: queued %+v, running %+v", queued, f.tasks(types.ManagementTaskRunning))
	}
	taskA, taskB := queued[0], queued[1]
	if strings.Join(taskA.LeasedCaptures, ",") != "cap-a1,cap-a2" || strings.Join(taskB.LeasedCaptures, ",") != "cap-b1,cap-b2" {
		t.Fatalf("leases: %v, %v", taskA.LeasedCaptures, taskB.LeasedCaptures)
	}
	// The opening Casts minted each task inbox's durable start.
	if f.outboxWakes(taskA.TaskInbox, taskA.TaskInbox) != 1 || f.outboxWakes(taskB.TaskInbox, taskB.TaskInbox) != 1 {
		t.Fatal("a delegation Cast minted no wake for its task inbox")
	}

	// K = 1 (FM23): the first wake starts task A; the second finds the slot
	// taken and leaves B queued.
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	runA, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, taskA.TaskInbox)
	if err != nil || runA == nil {
		t.Fatalf("start task A: %v, %v", runA, err)
	}
	if again, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, taskB.TaskInbox); err != nil || again != nil {
		t.Fatalf("FM23: a second task started at K=1: %v, %v", again, err)
	}
	if running := f.tasks(types.ManagementTaskRunning); len(running) != 1 || running[0].TaskID != taskA.TaskID || running[0].RunID != runA.RunID {
		t.Fatalf("running tasks = %+v", running)
	}
	// FM27, R2-1: the task's run is management's profile in the task role,
	// on the pinned model, reading its own inbox.
	if runA.AgentProfile != agentprofile.Management || runA.AgentRole != agentprofile.ManagementTask || runA.AgentID != taskA.TaskInbox ||
		metadataStringValue(runA.Metadata, "llm_model") != modelpolicy.ManagementTaskModel || metadataStringValue(runA.Metadata, "llm_provider") != modelpolicy.ManagementTaskProvider {
		t.Fatalf("task run = profile %s role %s agent %s model %s/%s", runA.AgentProfile, runA.AgentRole, runA.AgentID,
			metadataStringValue(runA.Metadata, "llm_provider"), metadataStringValue(runA.Metadata, "llm_model"))
	}

	// FM1: while task A's model is stuck, the parent runs a whole turn for
	// a new arrival; the new task queues behind A.
	seedArrival(t, f.s, f.owner, f.computer, "rss:c", []types.CaptureArrival{{CaptureID: "cap-c1", Title: "Another listing", URL: "https://c.example/1"}})
	f.parentTurn()
	if q := f.tasks(types.ManagementTaskQueued); len(q) != 2 {
		t.Fatalf("queued after the second parent turn = %+v; parent output %s", q, f.model.lastOutput())
	}

	// A clean task Report does not re-arm the parent (FM2, FM21).
	began := time.Now().UTC()
	close(gate)
	doneA := waitForStoredRunTerminalState(t, f.s, runA.RunID, 60*time.Second)
	if doneA.State != types.RunCompleted {
		t.Fatalf("task A run: %s (%s)", doneA.State, doneA.Error)
	}
	eventually(t, func() bool { return f.task(taskA.TaskID).State == types.ManagementTaskClosed })
	reportA := f.closingReport(taskA)
	if o := reportA.TaskOutcome; o == nil || !o.Clean || o.Reason != types.TaskClosureDone || len(o.Dispositions) != 2 || len(o.Residuals) != 0 {
		t.Fatalf("task A outcome = %+v", reportA.TaskOutcome)
	}
	if store.RecordWakesReader(reportA) || f.outboxWakes(f.management(), reportA.RecordID) != 0 {
		t.Fatal("FM2: task A's clean Report woke the parent")
	}
	if n := f.recheckWakes(began); n != 0 {
		t.Fatalf("FM21: the re-check started %d parent turns for a clean Report", n)
	}
	// FM6: the task's acts are management's, with the task named.
	declines, _ := f.s.ListInboxRecords(ctx, f.owner, f.computer, types.CaptureArrivalSenderInbox("rss:a"), 0, 0)
	if len(declines) != 1 || declines[0].SenderInbox != f.management() || declines[0].Provenance.ActingTask != taskA.TaskID ||
		declines[0].Provenance.ActingRunID != runA.RunID || declines[0].Disposition == nil {
		t.Fatalf("task decline = %+v", declines)
	}
	if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-a1"); d.Kind != types.DispositionFeed || d.ActingTask != taskA.TaskID || d.Inbox != "texture:"+f.pope {
		t.Fatalf("cap-a1 index = %+v", d)
	}
	if o, _ := f.s.CaptureLabelOverrides(ctx, f.owner, f.computer, "cap-a1"); len(o) != 1 || o[0].DocID != f.pope {
		t.Fatalf("N5: a task's disposition against its label recorded overrides %+v", o)
	}
	// The task's session ended with it.
	workers := f.rt.deskSessionWorkers()
	workers.mu.Lock()
	_, kept := workers.sessions["agent:"+taskA.TaskInbox]
	_, parentKept := workers.sessions["agent:"+f.management()]
	workers.mu.Unlock()
	if kept || !parentKept {
		t.Fatalf("sessions after task A: task kept %v, parent kept %v; want the task's released and the parent's kept", kept, parentKept)
	}
	// Its closure minted the next queued task's start.
	if f.outboxWakes(taskB.TaskInbox, "start_after") != 1 {
		t.Fatal("task A's closure minted no start for task B")
	}

	// Task B leaves one capture: its Report wakes the parent once.
	f.model.mu.Lock()
	f.model.gate = nil
	f.model.mu.Unlock()
	runB, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, taskB.TaskInbox)
	if err != nil || runB == nil {
		t.Fatalf("start task B: %v, %v", runB, err)
	}
	waitForStoredRunTerminalState(t, f.s, runB.RunID, 60*time.Second)
	eventually(t, func() bool { return f.task(taskB.TaskID).State == types.ManagementTaskClosed })
	reportB := f.closingReport(taskB)
	if o := reportB.TaskOutcome; o == nil || o.Clean || len(o.Residuals) != 1 || o.Residuals[0].CaptureID != "cap-b1" || !strings.Contains(o.Residuals[0].Reason, "needs management") {
		t.Fatalf("task B outcome = %+v", reportB.TaskOutcome)
	}
	if !store.RecordWakesReader(reportB) || f.outboxWakes(f.management(), reportB.RecordID) != 1 {
		t.Fatal("a residual Report did not wake the parent")
	}
	if n := f.recheckWakes(began); n != 1 {
		t.Fatalf("the re-check after a residual Report started %d parent turns, want 1", n)
	}
	for _, model := range f.model.taskModels {
		if model != modelpolicy.ManagementTaskProvider+"/"+modelpolicy.ManagementTaskModel {
			t.Fatalf("FM27: a task call went to %s", model)
		}
	}
}

func TestManagementTaskDeadlineAndCancelReturnResiduals(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-deadline")
	ctx := context.Background()
	var mu sync.Mutex
	var budgets []time.Duration
	f.rt.SetScheduleActor(func(_ context.Context, _, _, _, kind, _, _, _ string, notBefore time.Time) error {
		if kind == activationBudgetDeadlineUpdateKind {
			mu.Lock()
			budgets = append(budgets, time.Until(notBefore))
			mu.Unlock()
		}
		return nil
	})
	seedArrival(t, f.s, f.owner, f.computer, "rss:d1", []types.CaptureArrival{{CaptureID: "cap-d1", Title: "Pope recovers", URL: "https://d.example/1"}})
	seedArrival(t, f.s, f.owner, f.computer, "rss:d2", []types.CaptureArrival{{CaptureID: "cap-d2", Title: "Listing", URL: "https://d.example/2"}})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("queued = %+v", queued)
	}
	stuck, waiting := queued[0], queued[1]

	// A task whose model never answers holds its lease only until its
	// deadline (FM11): the activation budget is the 10-minute deadline,
	// and when it fires the round closes with every capture returned.
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	f.model.mu.Unlock()
	run, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, stuck.TaskInbox)
	if err != nil || run == nil {
		t.Fatalf("start: %v, %v", run, err)
	}
	if budget := f.rt.activationBudgetForRun(run); budget != managementTaskDeadline {
		t.Fatalf("task activation budget %s, want %s", budget, managementTaskDeadline)
	}
	if budget := f.rt.activationBudgetForRun(&types.RunRecord{AgentID: f.management(), AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management}); budget == managementTaskDeadline {
		t.Fatal("the parent's activation budget is the task deadline")
	}
	eventually(t, func() bool {
		rec, err := f.s.GetRunByOwner(ctx, f.owner, run.RunID)
		return err == nil && rec.State == types.RunRunning
	})
	if err := f.rt.HandleActivationBudgetDeadline(ctx, f.owner, f.computer, stuck.TaskInbox, run.RunID); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	closed := f.task(stuck.TaskID)
	if closed.State != types.ManagementTaskClosed || closed.ClosureReason != types.TaskClosureDeadline {
		t.Fatalf("deadline-closed row = %+v", closed)
	}
	report := f.closingReport(stuck)
	if o := report.TaskOutcome; o == nil || o.Clean || len(o.Residuals) != 1 || o.Residuals[0].CaptureID != "cap-d1" || !strings.Contains(o.Error, "deadline") {
		t.Fatalf("deadline outcome = %+v", report.TaskOutcome)
	}
	if f.outboxWakes(f.management(), report.RecordID) != 1 {
		t.Fatal("a deadline closure did not wake the parent")
	}
	if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-d1"); d.Kind != "" {
		t.Fatalf("cap-d1 was disposed by a stuck task: %+v", d)
	}

	// CancelTask (R2-20) closes the queued task's round with its capture
	// as a residual.
	cell := managementCell(t, f.rt, f.owner, f.computer, "run-cancel-task")
	if out := cell(`choir.CancelTask("` + waiting.TaskID + `", "the parent takes it back")`); !strings.Contains(out, "rlm:cancel_task:") {
		t.Fatalf("cancel: %s", out)
	}
	cancelled := f.task(waiting.TaskID)
	if cancelled.State != types.ManagementTaskClosed || cancelled.ClosureReason != types.TaskClosureCancelled {
		t.Fatalf("cancelled row = %+v", cancelled)
	}
	if o := f.closingReport(waiting).TaskOutcome; o == nil || len(o.Residuals) != 1 || o.Residuals[0].CaptureID != "cap-d2" {
		t.Fatalf("cancel outcome = %+v", o)
	}
	// The parent's next turn reads both residual Reports and delegates
	// their captures again by the Reports' ids, in one task.
	next := managementCell(t, f.rt, f.owner, f.computer, "run-redelegate")
	out := next(`var reports []string
for _, r := range choir.Records() { if r.TaskOutcome != nil { fmt.Println(r.Kind, len(r.TaskOutcome.Residuals)); reports = append(reports, r.RecordID) } }
choir.Delegate("intake", "", reports, []any{reports})`)
	if strings.Count(out, "task_report 1") != 2 || strings.Count(out, "rlm:delegate:") != 1 {
		t.Fatalf("re-delegation: %s", out)
	}
	again := f.tasks(types.ManagementTaskQueued)
	if len(again) != 1 || strings.Join(again[0].LeasedCaptures, ",") != "cap-d1,cap-d2" || len(again[0].DelegatedRecordIDs) != 2 {
		t.Fatalf("re-delegated tasks = %+v", again)
	}
	// The new task is shown both captures records, cut to its lease.
	view, err := f.rt.loadCellRecords(ctx, f.owner, f.computer, again[0].TaskInbox, "")
	if err != nil {
		t.Fatal(err)
	}
	shown := 0
	for _, r := range view.view {
		if r.Captures != nil {
			shown += len(r.Captures.Items)
		}
	}
	if shown != 2 {
		t.Fatalf("the re-delegated task is shown %d captures: %+v", shown, view.view)
	}
}

func TestManagementTasksAtACrashBootAndAPlannedBoot(t *testing.T) {
	for _, planned := range []bool{false, true} {
		t.Run(map[bool]string{false: "crash", true: "planned"}[planned], func(t *testing.T) {
			owner := "owner-task-boot"
			t.Setenv("CHOIR_OWNER_ID", owner)
			f := newDelegationFixture(t, owner)
			// S2b (AC6 at K > 1): two tasks running and one queued.
			f.rt.cfg.ManagementTaskSlots = 2
			ctx := context.Background()
			seedArrival(t, f.s, f.owner, f.computer, "rss:k1", []types.CaptureArrival{{CaptureID: "cap-k1", Title: "Pope recovers", URL: "https://k.example/1"}})
			seedArrival(t, f.s, f.owner, f.computer, "rss:k2", []types.CaptureArrival{{CaptureID: "cap-k2", Title: "Pope leaves hospital", URL: "https://k.example/2"}})
			seedArrival(t, f.s, f.owner, f.computer, "rss:k3", []types.CaptureArrival{{CaptureID: "cap-k3", Title: "Listing", URL: "https://k.example/3"}})
			f.parentTurn()
			queued := f.tasks(types.ManagementTaskQueued)
			if len(queued) != 3 {
				t.Fatalf("queued = %+v", queued)
			}
			waiting := queued[2]
			f.model.mu.Lock()
			f.model.gate = make(chan struct{})
			f.model.mu.Unlock()
			// One wake fills both free slots, oldest first.
			if first, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, queued[0].TaskInbox); err != nil || first == nil {
				t.Fatalf("start: %v, %v", first, err)
			}
			var runs []string
			for _, task := range queued[:2] {
				row := f.task(task.TaskID)
				if row.State != types.ManagementTaskRunning || row.RunID == "" {
					t.Fatalf("task %s after one wake at K=2 = %+v", task.TaskID, row)
				}
				runs = append(runs, row.RunID)
			}
			if q := f.task(waiting.TaskID); q.State != types.ManagementTaskQueued {
				t.Fatalf("the third task at K=2 = %+v", q)
			}
			for _, runID := range runs {
				eventually(t, func() bool {
					rec, err := f.s.GetRunByOwner(ctx, f.owner, runID)
					return err == nil && rec.State == types.RunRunning
				})
			}

			// The guest goes down with two tasks running and one queued.
			boot := testPeerRuntime(t, f.rt, f.s)
			if planned {
				WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "t"}, true)(boot)
			}
			boot.Start(ctx)
			for i, task := range queued[:2] {
				after, _ := f.s.GetRunByOwner(ctx, f.owner, runs[i])
				if after.State == types.RunPassivated || after.State.Active() {
					t.Fatalf("the boot left task run %s %s: tasks are not resumed", runs[i], after.State)
				}
				closed := f.task(task.TaskID)
				if closed.State != types.ManagementTaskClosed {
					t.Fatalf("running task %s's row after the boot = %+v", task.TaskID, closed)
				}
				report := f.closingReport(task)
				if !planned {
					// AC6: the crash boot closes the runs and the rounds as
					// interrupted, frees the leases, and wakes no one.
					if after.Error != runFateInterruptedByRestart || closed.ClosureReason != types.ClosureReasonInterruptedByRestart {
						t.Fatalf("crash: run %s (%s), row %s", after.State, after.Error, closed.ClosureReason)
					}
					if store.RecordWakesReader(report) || f.outboxWakes(f.management(), report.RecordID) != 0 {
						t.Fatal("crash: a closure woke the parent")
					}
					if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, task.LeasedCaptures[0]); d.CrashCount != 1 {
						t.Fatalf("crash: %s's crash budget = %+v", task.LeasedCaptures[0], d)
					}
					continue
				}
				// FM24: the planned boot closes each running task
				// run_passivated with its residuals, which wakes the parent.
				if closed.ClosureReason != types.TaskClosureRunPassivated || report.TaskOutcome == nil || len(report.TaskOutcome.Residuals) != 1 {
					t.Fatalf("planned: row %+v, outcome %+v", closed, report.TaskOutcome)
				}
				if !store.RecordWakesReader(report) || f.outboxWakes(f.management(), report.RecordID) != 1 {
					t.Fatal("planned: a residual Report did not wake the parent")
				}
			}
			if !planned {
				if q := f.task(waiting.TaskID); q.State != types.ManagementTaskClosed || q.ClosureReason != types.ClosureReasonInterruptedByRestart {
					t.Fatalf("crash: the queued task = %+v", q)
				}
				if f.outboxWakes(waiting.TaskInbox, "start_after") != 0 {
					t.Fatal("crash: a closure started the queued task")
				}
				// FM9: a task inbox wake after the boot starts nothing.
				if rec, err := boot.WakeRecordsInbox(ctx, f.owner, f.computer, waiting.TaskInbox); err != nil || rec != nil {
					t.Fatalf("crash: a task started after the boot: %v, %v", rec, err)
				}
				if running := f.tasks(types.ManagementTaskRunning); len(running) != 0 {
					t.Fatalf("crash: tasks running after the boot: %+v", running)
				}
				return
			}
			// The queued task starts through the start wakes the closures
			// minted (one slot each; the starter fills free slots once).
			if q := f.task(waiting.TaskID); q.State != types.ManagementTaskQueued {
				t.Fatalf("planned: the queued task = %+v", q)
			}
			if f.outboxWakes(waiting.TaskInbox, "start_after") < 1 {
				t.Fatal("planned: no start for the queued task")
			}
		})
	}
}

// FM3 through a task cell (R2-5): a task that opens a story without
// reading the library is refused, with the story opened since named, and
// its cell commits nothing; one that read choir.Stories() first opens.
func TestTaskStoryOpenNeedsTheLibraryHead(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-library")
	ctx := context.Background()
	seedArrival(t, f.s, f.owner, f.computer, "rss:l1", []types.CaptureArrival{{CaptureID: "cap-l1", Title: "Harbor strike begins", URL: "https://l.example/1"}})
	seedArrival(t, f.s, f.owner, f.computer, "rss:l2", []types.CaptureArrival{{CaptureID: "cap-l2", Title: "Harbor strike begins", URL: "https://l.example/2"}})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("queued = %+v", queued)
	}
	open := func(readFirst bool) string {
		cell := "var ids []string\nfor _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }\n"
		if readFirst {
			cell += "p, _ := choir.Stories(50, \"\")\nfmt.Println(\"stories\", len(p.Stories))\n"
		}
		return cell + `choir.Cast("texture:new", "Harbor strike begins", map[string]any{"captures": ids, "publish": "auto", "hypotheses": []any{map[string]any{"statement": "the strike lasts a week", "probability": 0.5}}})
choir.Done("opened the strike story")`
	}
	run := func(task types.ManagementTask, cell string) types.ManagementTask {
		t.Helper()
		f.model.mu.Lock()
		f.model.taskCell = cell
		f.model.mu.Unlock()
		rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox)
		if err != nil || rec == nil {
			t.Fatalf("start %s: %v, %v", task.TaskID, rec, err)
		}
		waitForStoredRunTerminalState(t, f.s, rec.RunID, 60*time.Second)
		eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
		return f.task(task.TaskID)
	}
	// The pope story the fixture opened is past head 0.
	run(queued[0], open(false))
	if out := f.model.lastOutput(); !strings.Contains(out, "story library moved") || !strings.Contains(out, "Pope has lung surgery") {
		t.Fatalf("a task's open without the library was not refused naming the story since: %s", out)
	}
	if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-l1"); d.Kind != "" {
		t.Fatalf("the refused cell disposed of cap-l1: %+v", d)
	}
	if o := f.closingReport(queued[0]).TaskOutcome; o == nil || len(o.Residuals) != 1 {
		t.Fatalf("refused task outcome = %+v", o)
	}
	closed := run(queued[1], open(true))
	if o := f.closingReport(closed).TaskOutcome; o == nil || !o.Clean || len(o.Dispositions) != 1 || o.Dispositions[0].Kind != types.DispositionOpen {
		t.Fatalf("an open after reading the library = %+v (%s)", o, f.model.lastOutput())
	}
}
