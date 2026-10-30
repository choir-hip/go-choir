package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management delegation S2b, concurrency, through the real reduce and
// commit paths (docs/archive/reports/management-delegation-design-2026-10-14.md
// §18.1, written before the code):
//
//	FM23 at K > 1  a burst starts more than K task runs;
//	FM8            queued tasks wait while slots are free;
//	FM3            two running tasks open stories for one event;
//	FM28           a task's own open refuses its next open;
//	FM14           K tasks' model calls burst past the gateway's limit;
//	FM29           a refused claim takes a slot, or a failing start spins;
//	FM31           no rollback to K = 1 without a deploy.
//
// FM9 and AC6 at K = 2 are in TestManagementTasksAtACrashBootAndAPlannedBoot.

// TestManagementTasksRunKAtOnceAndTheQueueDrains: six single-capture tasks
// at K = 3 (from config). Concurrent wakes start exactly three, whose model
// calls are in flight together; the rest start only through the durable
// start wakes the closures mint, and the queue drains with at most K
// running. Task model calls are spaced on the shared sub-RLM spacer.
func TestManagementTasksRunKAtOnceAndTheQueueDrains(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-drain")
	ctx := context.Background()
	const k = 3
	f.rt.cfg.ManagementTaskSlots = k
	// FM14: a short interval so the test can see the spacing.
	const interval = 150 * time.Millisecond
	f.rt.researchChildCalls = &callSpacer{interval: interval}
	model := &concurrencyModel{StubProvider: provider.NewStubProvider(0), parentCell: delegateAllCell, gate: make(chan struct{})}
	model.taskCell = func(step int, _ string) (string, bool) {
		if step > 0 {
			return "", false
		}
		return `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }
choir.Decline(ids, "a listing, no news event")
choir.Done("declined")`, true
	}
	f.rt.provider = model
	for i := 1; i <= 6; i++ {
		seedArrival(t, f.s, f.owner, f.computer, fmt.Sprintf("rss:q%d", i), []types.CaptureArrival{
			{CaptureID: fmt.Sprintf("cap-q%d", i), Title: fmt.Sprintf("Listing %d", i), URL: fmt.Sprintf("https://q.example/%d", i)}})
	}
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 6 {
		t.Fatalf("queued = %d tasks, want 6", len(queued))
	}

	// Peak running, sampled from the rows (the store's view of slots).
	var peakMu sync.Mutex
	peak := 0
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
			rows, err := f.s.ListManagementTasks(ctx, f.owner, f.computer, types.ManagementTaskRunning)
			if err != nil {
				continue
			}
			peakMu.Lock()
			if len(rows) > peak {
				peak = len(rows)
			}
			peakMu.Unlock()
		}
	}()

	// FM23: every task inbox is woken at once; exactly K start.
	var wg sync.WaitGroup
	for _, task := range queued {
		wg.Add(1)
		go func(inbox string) {
			defer wg.Done()
			if _, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, inbox); err != nil {
				t.Errorf("wake %s: %v", inbox, err)
			}
		}(task.TaskInbox)
	}
	wg.Wait()
	if running := f.tasks(types.ManagementTaskRunning); len(running) != k {
		t.Fatalf("FM23: %d tasks running after a burst of wakes at K=%d", len(running), k)
	}
	if q := f.tasks(types.ManagementTaskQueued); len(q) != 6-k {
		t.Fatalf("queued after the burst = %d, want %d", len(q), 6-k)
	}
	// The K tasks run in parallel: their model calls are in flight together.
	eventually(t, func() bool { return model.inFlight() == k })

	// FM8: release the model and let the outbox alone start the rest.
	f.mu.Lock()
	f.deliverTaskWakes = true
	f.mu.Unlock()
	close(model.gate)
	deadline := time.Now().Add(90 * time.Second)
	for {
		f.rt.sweepActorWakeOutbox(ctx)
		if len(f.tasks(types.ManagementTaskClosed)) == 6 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("FM8: the queue did not drain: queued %d, running %d, closed %d",
				len(f.tasks(types.ManagementTaskQueued)), len(f.tasks(types.ManagementTaskRunning)), len(f.tasks(types.ManagementTaskClosed)))
		}
		time.Sleep(20 * time.Millisecond)
	}
	close(stop)
	<-sampled
	if peak > k {
		t.Fatalf("FM23: %d tasks ran at once at K=%d", peak, k)
	}
	for _, task := range queued {
		report := f.closingReport(task)
		if o := report.TaskOutcome; o == nil || !o.Clean || len(o.Dispositions) != 1 || o.Dispositions[0].Kind != types.DispositionDecline {
			t.Fatalf("task %s outcome = %+v", task.TaskID, report.TaskOutcome)
		}
	}
	// FM14: consecutive task model calls started at least one spacer
	// interval apart, though K tasks called at once.
	starts := model.taskCallStarts()
	if len(starts) < 6 {
		t.Fatalf("task model calls = %d, want at least 6 (one per task; Done ends the run)", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i].Sub(starts[i-1]); gap < interval/2 {
			t.Fatalf("FM14: task calls %d and %d started %s apart; the spacer interval is %s", i-1, i, gap, interval)
		}
	}
}

// TestConcurrentTasksOpenOneStoryForOneEvent (FM3 under real concurrency):
// two tasks at the default K read the library at the same head and are held
// at a barrier, so both are inside a model call at once; then both cast
// texture:new for one event. The store commits one open and refuses the
// other with the first's story named; that task feeds it. One story.
func TestConcurrentTasksOpenOneStoryForOneEvent(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-race")
	ctx := context.Background()
	f.rt.cfg.ManagementTaskSlots = 0 // unset: the default K = 4
	if got := f.rt.managementTaskSlots(ctx, f.owner, f.computer); got != provideriface.DefaultManagementTaskSlots {
		t.Fatalf("default slots = %d", got)
	}
	barrier := newStepBarrier(2)
	model := &concurrencyModel{StubProvider: provider.NewStubProvider(0), parentCell: delegateAllCell}
	model.taskCell = func(step int, last string) (string, bool) {
		leased := "var ids []string\nfor _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }\n"
		switch step {
		case 0:
			return "p, _ := choir.Stories(50, \"\")\nfmt.Println(\"stories\", len(p.Stories))", true
		case 1:
			if !barrier.arrive(20 * time.Second) {
				return "", false
			}
			return leased + `choir.Cast("texture:new", "Harbor strike begins", map[string]any{"captures": ids, "publish": "auto", "hypotheses": []any{map[string]any{"statement": "the strike lasts a week", "probability": 0.5}}})
choir.Done("opened the strike story")`, true
		case 2:
			if !strings.Contains(last, "story library moved") {
				return "", false
			}
			return leased + `p, _ := choir.Stories(50, "")
for _, s := range p.Stories {
	if s.Headline == "Harbor strike begins" { choir.Feed(s.Inbox, ids, "the same strike, opened by another task") }
}
choir.Done("fed the strike story")`, true
		}
		return "", false
	}
	f.rt.provider = model
	headBefore, _ := f.s.StoryLibraryHeadOf(ctx, f.owner, f.computer)
	seedArrival(t, f.s, f.owner, f.computer, "rss:h1", []types.CaptureArrival{{CaptureID: "cap-h1", Title: "Harbor strike begins", URL: "https://h.example/1"}})
	seedArrival(t, f.s, f.owner, f.computer, "rss:h2", []types.CaptureArrival{{CaptureID: "cap-h2", Title: "Dockers walk out at the harbor", URL: "https://h2.example/1"}})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("queued = %+v", queued)
	}
	if rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, queued[0].TaskInbox); err != nil || rec == nil {
		t.Fatalf("start: %v, %v", rec, err)
	}
	if running := f.tasks(types.ManagementTaskRunning); len(running) != 2 {
		t.Fatalf("one wake at K=4 started %d of 2 queued tasks", len(running))
	}
	for _, task := range queued {
		eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
	}
	if !barrier.met() {
		t.Fatal("the two tasks were never inside a model call at once")
	}
	opens, feeds := 0, 0
	var storyInbox string
	for _, task := range queued {
		o := f.closingReport(task).TaskOutcome
		if o == nil || !o.Clean || len(o.Dispositions) != 1 {
			t.Fatalf("task %s outcome = %+v (outputs %v)", task.TaskID, o, model.allOutputs())
		}
		switch d := o.Dispositions[0]; d.Kind {
		case types.DispositionOpen:
			opens++
			storyInbox = d.Inbox
		case types.DispositionFeed:
			feeds++
		}
	}
	if opens != 1 || feeds != 1 {
		t.Fatalf("FM3: %d opens and %d feeds for one event; want one story", opens, feeds)
	}
	for _, c := range []string{"cap-h1", "cap-h2"} {
		if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, c); d.Inbox != storyInbox {
			t.Fatalf("FM3: %s went to %s, the story is %s", c, d.Inbox, storyInbox)
		}
	}
	if head, _ := f.s.StoryLibraryHeadOf(ctx, f.owner, f.computer); head != headBefore+1 {
		t.Fatalf("FM3: the library head moved %d → %d; want one open", headBefore, head)
	}
	refusals := 0
	for _, out := range model.allOutputs() {
		if strings.Contains(out, "story library moved") {
			refusals++
		}
	}
	if refusals != 1 {
		t.Fatalf("refusals = %d, want exactly one (the second open)", refusals)
	}
}

// TestATaskOpensTwoStoriesWithoutRefusingItself (FM28): a task reads the
// library once and opens two stories in two cells; its own first open does
// not make the second stale.
func TestATaskOpensTwoStoriesWithoutRefusingItself(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-two-opens")
	ctx := context.Background()
	model := &concurrencyModel{StubProvider: provider.NewStubProvider(0), parentCell: `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { ids = append(ids, r.RecordID) } }
choir.Delegate("intake", "", ids, []any{ids})`}
	open := func(capture, headline string) string {
		return `choir.Cast("texture:new", "` + headline + `", map[string]any{"captures": []string{"` + capture + `"}, "publish": "auto", "hypotheses": []any{map[string]any{"statement": "it matters", "probability": 0.5}}})`
	}
	model.taskCell = func(step int, _ string) (string, bool) {
		switch step {
		case 0:
			return "p, _ := choir.Stories(50, \"\")\nfmt.Println(\"stories\", len(p.Stories))", true
		case 1:
			return open("cap-t1", "Bridge closes for repairs"), true
		case 2:
			return open("cap-t2", "Election date set") + "\nchoir.Done(\"two stories\")", true
		}
		return "", false
	}
	f.rt.provider = model
	seedArrival(t, f.s, f.owner, f.computer, "rss:t", []types.CaptureArrival{
		{CaptureID: "cap-t1", Title: "Bridge closes for repairs", URL: "https://t.example/1"},
		{CaptureID: "cap-t2", Title: "Election date set", URL: "https://t.example/2"},
	})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 1 {
		t.Fatalf("queued = %+v", queued)
	}
	if rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, queued[0].TaskInbox); err != nil || rec == nil {
		t.Fatalf("start: %v, %v", rec, err)
	}
	eventually(t, func() bool { return f.task(queued[0].TaskID).State == types.ManagementTaskClosed })
	for _, out := range model.allOutputs() {
		if strings.Contains(out, "story library moved") {
			t.Fatalf("FM28: the task's own open refused its next: %s", out)
		}
	}
	o := f.closingReport(queued[0]).TaskOutcome
	if o == nil || !o.Clean || len(o.Dispositions) != 2 || o.Dispositions[0].Kind != types.DispositionOpen || o.Dispositions[1].Kind != types.DispositionOpen {
		t.Fatalf("outcome = %+v (outputs %v)", o, model.allOutputs())
	}
}

// TestSerialTasksSettingIsTheLiveRollback (FM31): the owner's
// management.serial_tasks setting holds K at 1 on the next start, without a
// restart; turning it off lets the starter fill K slots again.
func TestSerialTasksSettingIsTheLiveRollback(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-serial")
	ctx := context.Background()
	f.rt.cfg.ManagementTaskSlots = 4
	setSerial := func(on bool) {
		t.Helper()
		if err := f.s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: store.SettingManagementSerialTasks, Value: on}); err != nil {
			t.Fatal(err)
		}
	}
	if def, known := store.KnownComputerSettings[store.SettingManagementSerialTasks]; !known || def {
		t.Fatalf("management.serial_tasks known %v, default %v; want known, off", known, def)
	}
	setSerial(true)
	if got := f.rt.managementTaskSlots(ctx, f.owner, f.computer); got != 1 {
		t.Fatalf("slots with serial_tasks on = %d, want 1", got)
	}
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	seedArrival(t, f.s, f.owner, f.computer, "rss:s1", []types.CaptureArrival{{CaptureID: "cap-s1", Title: "Listing", URL: "https://s.example/1"}})
	seedArrival(t, f.s, f.owner, f.computer, "rss:s2", []types.CaptureArrival{{CaptureID: "cap-s2", Title: "Listing two", URL: "https://s.example/2"}})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("queued = %+v", queued)
	}
	for _, task := range queued {
		if _, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox); err != nil {
			t.Fatal(err)
		}
	}
	if running := f.tasks(types.ManagementTaskRunning); len(running) != 1 {
		t.Fatalf("FM31: %d tasks running with serial_tasks on", len(running))
	}
	// The probe reads K, the slots in use, the queue from the product
	// route.
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	rec := httptest.NewRecorder()
	(&APIHandler{rt: f.rt}).HandleGuestObservability(rec, req)
	var obs struct {
		ManagementTasks *managementTaskObservability `json:"management_tasks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("observability = %d %s (%v)", rec.Code, rec.Body, err)
	}
	if m := obs.ManagementTasks; m == nil || m.Slots != 1 || m.ConfiguredSlots != 4 || !m.SerialTasks || m.Running != 1 || m.Queued != 1 || m.OldestQueuedAgeS < 0 {
		t.Fatalf("observability management_tasks = %+v", obs.ManagementTasks)
	}
	setSerial(false)
	if rec, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, queued[1].TaskInbox); err != nil || rec == nil {
		t.Fatalf("FM31: with serial_tasks off the second task did not start: %v, %v", rec, err)
	}
	if running := f.tasks(types.ManagementTaskRunning); len(running) != 2 {
		t.Fatalf("running after serial_tasks off = %d, want 2", len(running))
	}
	close(gate)
	for _, task := range queued {
		eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
	}
}

// concurrencyModel scripts the parent (one cell, then end) and each task
// by step: the number of cell results in the task's history. taskCell
// returns the cell for a step and the previous cell's result, or false to
// end the turn. Task calls block on gate while it is open, and the model
// records how many task calls are in flight and when each started.
type concurrencyModel struct {
	*provider.StubProvider
	parentCell string
	taskCell   func(step int, last string) (string, bool)
	gate       chan struct{}

	mu       sync.Mutex
	calls    int
	inflight int
	starts   []time.Time
	outputs  []string
}

func (m *concurrencyModel) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	task := strings.Contains(req.System, "You are a management task")
	m.mu.Lock()
	m.calls++
	callID := fmt.Sprintf("call-%d", m.calls)
	if task {
		m.starts = append(m.starts, time.Now())
		m.inflight++
	}
	gate := m.gate
	m.mu.Unlock()
	if task {
		defer func() {
			m.mu.Lock()
			m.inflight--
			m.mu.Unlock()
		}()
		if gate != nil {
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	step, last := 0, ""
	for _, msg := range req.Messages {
		if strings.Contains(string(msg), "tool_result") {
			step++
			last = string(msg)
		}
	}
	if last != "" {
		m.mu.Lock()
		m.outputs = append(m.outputs, last)
		m.mu.Unlock()
	}
	var cell string
	ok := false
	if task {
		cell, ok = m.taskCell(step, last)
	} else if n := len(req.Messages); n > 0 && !strings.Contains(string(req.Messages[n-1]), "tool_result") {
		// The parent's memory spans its turns: a turn's first call runs
		// its cell, the call after the cell's result ends the turn.
		cell, ok = m.parentCell, true
	}
	if !ok {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
	args, _ := json.Marshal(map[string]any{"source": cell})
	return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "test-model",
		ToolCalls: []types.ToolCall{{ID: callID, Name: "desk_go_eval", Arguments: args}}}, nil
}

func (m *concurrencyModel) inFlight() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inflight
}

func (m *concurrencyModel) taskCallStarts() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]time.Time(nil), m.starts...)
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

func (m *concurrencyModel) allOutputs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.outputs...)
}

// stepBarrier holds n callers until all have arrived.
type stepBarrier struct {
	mu      sync.Mutex
	n       int
	arrived int
	all     chan struct{}
}

func newStepBarrier(n int) *stepBarrier { return &stepBarrier{n: n, all: make(chan struct{})} }

func (b *stepBarrier) arrive(timeout time.Duration) bool {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.n {
		close(b.all)
	}
	b.mu.Unlock()
	select {
	case <-b.all:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (b *stepBarrier) met() bool {
	select {
	case <-b.all:
		return true
	default:
		return false
	}
}
