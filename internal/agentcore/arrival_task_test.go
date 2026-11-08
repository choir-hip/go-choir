package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Enforce flip 13.5 (F6 S2a): intake runs clean by construction. The failure
// modes A1 to A10 are listed in arrival_task.go; the representative-day cases
// of 13.4 are here too. These are in-process runs through the real commit,
// reduce and task paths; the deployed proof is the S2 re-run on d@e.

var computersLabel = types.OwnerPrivateLabel("computers")

func hasString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// arrive announces captures the way a source poll does.
func (f *delegationFixture) arrive(source string, items ...types.CaptureArrival) string {
	f.t.Helper()
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.CaptureID
	}
	// F6 S3: the arrival commit is the product's path (the host mints the task
	// with the arrival under the management_task custodian), so it runs with the
	// gates enforcing.
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	if err := f.rt.appendCaptureArrival(context.Background(), f.owner, f.computer, source, items, false); err != nil {
		f.t.Fatalf("arrival %s: %v", source, err)
	}
	return types.CaptureArrivalRecordID(source, ids)
}

func (f *delegationFixture) record(id string) *types.CommitmentRecord {
	f.t.Helper()
	rec, err := f.s.GetCommitmentRecord(context.Background(), f.owner, f.computer, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return rec
}

// taintManagement taints the management main run on each of its turns with
// the computers label, as its own product reads of /api/computers do.
func (f *delegationFixture) taintManagement() {
	f.model.mu.Lock()
	f.model.onParent = func(runID string) { f.rt.taintRun(runID, types.NewLabelSet(computersLabel)) }
	f.model.mu.Unlock()
}

// runTask starts a queued task with the given cell and waits for its closure.
func (f *delegationFixture) runTask(task types.ManagementTask, cell string) types.ManagementTask {
	f.t.Helper()
	f.model.mu.Lock()
	f.model.taskCell = cell
	f.model.mu.Unlock()
	rec, err := f.rt.WakeRecordsInbox(context.Background(), f.owner, f.computer, task.TaskInbox)
	if err != nil || rec == nil {
		f.t.Fatalf("start %s: %v, %v", task.TaskID, rec, err)
	}
	waitForStoredRunTerminalState(f.t, f.s, rec.RunID, 60*time.Second)
	eventually(f.t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
	return f.task(task.TaskID)
}

const openStoryTaskCell = `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }
p, _ := choir.Stories(50, "")
fmt.Println("stories", len(p.Stories))
choir.Cast("texture:new", "Harbor strike begins", map[string]any{"captures": ids, "publish": "auto", "hypotheses": []any{map[string]any{"statement": "the strike lasts a week", "probability": 0.5}}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})
choir.Done("opened the strike story")`

const delegateResidualsCell = `var ids []string
for _, r := range choir.Records() { if r.TaskOutcome != nil && len(r.TaskOutcome.Residuals) > 0 { ids = append(ids, r.RecordID) } }
choir.Delegate("intake", ids, "per_record")`

// putOwnerSchedule stores the owner-stamped schedule the publish-auto walk
// ends at.
func (f *delegationFixture) putOwnerSchedule(source string) {
	f.t.Helper()
	sched := store.SourceSchedule{OwnerID: f.owner, ComputerID: f.computer, SourceID: source, IntervalSeconds: 900, State: store.SourceScheduleActive, Generation: 1, CreatedByOwnerAct: "req-owner-" + source}
	if err := f.s.PutSourceSchedule(objectgraph.WithWriter(context.Background(), objectgraph.WriterOwner), sched); err != nil {
		f.t.Fatal(err)
	}
}

// storyOpening returns the Cast that opened the story a capture went to.
func (f *delegationFixture) storyOpening(capture string) (inbox string, cast types.CommitmentRecord) {
	f.t.Helper()
	d, found, err := f.s.CaptureDispositionOf(context.Background(), f.owner, f.computer, capture)
	if err != nil || !found || d.Inbox == "" {
		f.t.Fatalf("capture %s: disposition %+v, %v", capture, d, err)
	}
	recs, err := f.s.ListInboxRecords(context.Background(), f.owner, f.computer, d.Inbox, 0, 0)
	if err != nil || len(recs) == 0 {
		f.t.Fatalf("story inbox %s: %v, %v", d.Inbox, recs, err)
	}
	return d.Inbox, recs[0]
}

// requireStoryPublishesWithoutAsk is the publication sink's decision for the
// story's Texture run: every owner-private label it carries must be admitted.
// The run reads its opening Cast, so it carries that Cast's labels, and its
// frame adds the document and evidence labels by construction.
func (f *delegationFixture) requireStoryPublishesWithoutAsk(inbox string, cast types.CommitmentRecord) {
	f.t.Helper()
	labels := types.NewLabelSet(cast.Provenance.Labels...).Union(types.NewLabelSet(types.OwnerPrivateLabel("document"), types.OwnerPrivateLabel("evidence")))
	if hasString(labels, computersLabel) {
		f.t.Fatalf("the story %s opening Cast carries computers: %v", inbox, cast.Provenance.Labels)
	}
	call := disclosureCall{exec: toolregistry.ExecutionContext{OwnerID: f.owner, ComputerID: f.computer}, deskRole: agentprofile.Texture, target: strings.TrimPrefix(inbox, "texture:")}
	admit := f.rt.publishAutoAdmission(context.Background(), call)
	for _, l := range labels {
		if strings.HasPrefix(l, "owner_private:") && !admit(l) {
			f.t.Errorf("publication of %s would ask on %s (the Cast carries %v)", inbox, l, cast.Provenance.Labels)
		}
	}
}

// A1, A6, A7, A8: the arrival, the task row and its opening Cast commit
// together; the Cast is the host's and carries the arrival's labels; the task
// is woken and management's main run is not.
func TestArrivalMintsTheTaskAndItsOpeningCastInOneBatch(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-task")
	items := []types.CaptureArrival{{CaptureID: "cap-ar1", Title: "Pope recovers", URL: "https://ar.example/1"}, {CaptureID: "cap-ar2", Title: "Newsletter", URL: "https://ar.example/2"}}
	arrivalID := f.arrive("rss:ar", items...)

	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 1 || strings.Join(queued[0].LeasedCaptures, ",") != "cap-ar1,cap-ar2" || queued[0].Objective != managementIntakeObjective || queued[0].ParentRunID != "" {
		t.Fatalf("tasks = %+v", queued)
	}
	task := queued[0]
	arrival := f.record(arrivalID)
	if arrival == nil || arrival.Captures == nil || len(arrival.Captures.StartedTasks) != 1 || arrival.Captures.StartedTasks[0] != task.TaskID {
		t.Fatalf("arrival = %+v", arrival)
	}
	cast := f.record(task.OpeningRecordID)
	if cast == nil || !cast.IsCast() || cast.Inbox != task.TaskInbox || cast.SenderInbox != f.management() || cast.Directive.Objective != managementIntakeObjective {
		t.Fatalf("opening Cast = %+v", cast)
	}
	if cast.Provenance.Origin != types.OriginSystem || cast.Provenance.RunID != "" || cast.SubCall != nil || cast.Prediction.Hypothesis != "" || len(cast.Prediction.Questions) != 0 {
		t.Fatalf("opening Cast provenance = %+v, subcall %+v, prediction %+v", cast.Provenance, cast.SubCall, cast.Prediction)
	}
	want := labelsOfRecord(*arrival)
	if got := types.NewLabelSet(cast.Provenance.Labels...); strings.Join(got, ",") != strings.Join(want, ",") || len(want) == 0 {
		t.Fatalf("opening Cast labels = %v, want the arrival's %v", got, want)
	}
	// A7, A8: the task starts on the Cast's wake; the arrival wakes no one.
	if store.RecordWakesReader(*arrival) || f.outboxWakes(f.management(), arrivalID) != 0 {
		t.Fatal("A7: the arrival that started a task woke management's main run")
	}
	if f.outboxWakes(task.TaskInbox, task.TaskInbox) != 1 {
		t.Fatal("A8: the opening Cast minted no wake for its task")
	}
	if n := f.recheckWakes(time.Now().Add(-time.Hour)); n != 0 {
		t.Fatalf("A7: the post-turn re-check woke management %d times for the arrival", n)
	}
	// The task runs from the Cast alone and disposes of both captures.
	f.model.mu.Lock()
	f.model.taskCell = taskCellFor(f.pope)
	f.model.mu.Unlock()
	closed := f.runTask(task, taskCellFor(f.pope))
	if o := f.closingReport(closed).TaskOutcome; o == nil || !o.Clean || len(o.Dispositions) != 2 {
		t.Fatalf("outcome = %+v", f.closingReport(closed).TaskOutcome)
	}
}

// A2: a replay of the arrival mints no second task. At the runtime the second
// announce finds the captures leased and conflicts on the digest, which
// announceCaptures reads as announced; at the store the identical command
// replays its receipt.
func TestArrivalReplayMintsNoSecondTask(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-replay")
	ctx := context.Background()
	var committed []types.CommitInboxRecordsRequest
	f.rt.commitCaptureArrival = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
		committed = append(committed, req)
		_, err := f.s.CommitInboxRecords(ctx, req)
		return err
	}
	items := []types.CaptureArrival{{CaptureID: "cap-rp1", Title: "Harbor reopens", URL: "https://rp.example/1"}}
	f.arrive("rss:rp", items...)
	err := f.rt.appendCaptureArrival(ctx, f.owner, f.computer, "rss:rp", items, false)
	if !errors.Is(err, store.ErrLifecycleCommandConflict) {
		t.Fatalf("second announce = %v, want a command conflict", err)
	}
	if len(committed) != 2 || len(committed[0].Tasks) != 1 || len(committed[1].Tasks) != 0 {
		t.Fatalf("commands = %d, tasks %d/%d", len(committed), len(committed[0].Tasks), len(committed[1].Tasks))
	}
	receipt, err := f.s.CommitInboxRecords(withKindCustodian(ctx, "choir.management_task"), committed[0])
	if err != nil || !receipt.Replay {
		t.Fatalf("store replay of the identical command = %+v, %v", receipt, err)
	}
	if n := len(f.tasks(types.ManagementTaskQueued)); n != 1 {
		t.Fatalf("tasks after a replay = %d", n)
	}
	// announceCaptures treats the conflict as announced and drops the captures.
	rest, err := f.rt.announceCaptures(ctx, f.owner, f.computer, "rss:rp", nil, nil, 10, false)
	if err != nil || len(rest) != 0 {
		t.Fatalf("announce of nothing = %v, %v", rest, err)
	}
}

// A1: the arrival and its task are one batch. A planned command whose lease
// the store refuses (the capture was leased in between) leaves no arrival, no
// Cast and no task behind.
func TestArrivalAndTaskCommitAtomically(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-atomic")
	ctx := withKindCustodian(context.Background(), "choir.management_task")
	plan := func(source string, ids ...string) types.CommitInboxRecordsRequest {
		var items []types.CaptureArrival
		for _, id := range ids {
			items = append(items, types.CaptureArrival{CaptureID: id, Title: id, URL: "https://at.example/" + id})
		}
		req, err := captureArrivalRequest(f.owner, f.computer, source, items, false)
		if err != nil {
			t.Fatal(err)
		}
		planned, started, err := f.rt.withIntakeTask(ctx, req)
		if err != nil || !started {
			t.Fatalf("plan %s: started %v, %v", source, started, err)
		}
		return planned
	}
	// Both are planned while the capture is free; the first commits.
	first, second := plan("rss:at1", "cap-at-x"), plan("rss:at2", "cap-at-x", "cap-at-y")
	if _, err := f.s.CommitInboxRecords(ctx, first); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.CommitInboxRecords(ctx, second)
	if !errors.Is(err, store.ErrInboxRefused) {
		t.Fatalf("the second command = %v, want the store's refusal of the lease", err)
	}
	if rec := f.record(second.Records[0].RecordID); rec != nil {
		t.Fatalf("the refused command left its arrival behind: %+v", rec)
	}
	if rec := f.record(second.Records[1].RecordID); rec != nil {
		t.Fatalf("the refused command left its Cast behind: %+v", rec)
	}
	if tasks := f.tasks(types.ManagementTaskQueued); len(tasks) != 1 || tasks[0].TaskID != first.Tasks[0].TaskID {
		t.Fatalf("tasks = %+v", tasks)
	}
	if d, _, _ := f.s.CaptureDispositionOf(context.Background(), f.owner, f.computer, "cap-at-y"); d.LeaseTask != "" {
		t.Fatalf("the refused command leased cap-at-y: %+v", d)
	}
}

// A3, A4, A9: held arrivals mint nothing; settled captures are left out; a
// task takes at most MaxTaskCaptures; with intake off the arrival wakes
// management as before.
func TestArrivalTaskSkipsHeldSettledAndCapsTheTask(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-bounds")
	ctx := context.Background()

	// Held: a record, no task, no wake.
	held := []types.CaptureArrival{{CaptureID: "cap-hd1", Title: "Waiting on its event", URL: "https://hd.example/1"}}
	if err := f.rt.appendCaptureArrival(ctx, f.owner, f.computer, "rss:hd", held, true); err != nil {
		t.Fatal(err)
	}
	if n := len(f.tasks(types.ManagementTaskQueued)); n != 0 {
		t.Fatalf("A3: a held arrival minted %d tasks", n)
	}

	// Leased and disposed captures are left out; an arrival with nothing free
	// mints nothing and still wakes management.
	f.arrive("rss:b1", types.CaptureArrival{CaptureID: "cap-b-1", Title: "one", URL: "https://b.example/1"}, types.CaptureArrival{CaptureID: "cap-b-2", Title: "two", URL: "https://b.example/2"})
	second := f.arrive("rss:b2", types.CaptureArrival{CaptureID: "cap-b-2", Title: "two", URL: "https://b.example/2"}, types.CaptureArrival{CaptureID: "cap-b-3", Title: "three", URL: "https://b.example/3"})
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 || strings.Join(queued[1].LeasedCaptures, ",") != "cap-b-3" {
		t.Fatalf("A4: tasks = %+v", queued)
	}
	if a := f.record(second); a == nil || a.Captures == nil || len(a.Captures.StartedTasks) != 1 {
		t.Fatalf("second arrival = %+v", a)
	}
	third := f.arrive("rss:b3", types.CaptureArrival{CaptureID: "cap-b-1", Title: "one", URL: "https://b.example/1"})
	if n := len(f.tasks(types.ManagementTaskQueued)); n != 2 {
		t.Fatalf("A3: an arrival with nothing free minted a task (%d tasks)", n)
	}
	if a := f.record(third); a == nil || !store.RecordWakesReader(*a) || f.outboxWakes(f.management(), third) != 1 {
		t.Fatalf("an arrival with nothing free must wake management as before: %+v", a)
	}

	// The cap: 30 captures are three tasks of at most MaxTaskCaptures.
	var items []types.CaptureArrival
	for i := 0; i < 30; i++ {
		items = append(items, types.CaptureArrival{CaptureID: fmt.Sprintf("cap-big-%02d", i), Title: fmt.Sprintf("item %d", i), URL: fmt.Sprintf("https://big.example/%d", i)})
	}
	f.arrive("rss:big", items...)
	sizes, total := 0, 0
	for _, task := range f.tasks(types.ManagementTaskQueued) {
		if len(task.LeasedCaptures) > 12 {
			t.Fatalf("A4: task %s leases %d captures", task.TaskID, len(task.LeasedCaptures))
		}
		if strings.HasPrefix(task.LeasedCaptures[0], "cap-big-") {
			sizes++
			total += len(task.LeasedCaptures)
		}
	}
	if sizes != 3 || total != 30 {
		t.Fatalf("A4: the 30 captures became %d tasks holding %d", sizes, total)
	}

	// A9: with intake off nothing is minted and management is woken.
	storeIntakeLabelsOff(t, f.s, f.owner, f.computer)
	off := f.arrive("rss:off", types.CaptureArrival{CaptureID: "cap-off", Title: "off", URL: "https://off.example/1"})
	if a := f.record(off); a == nil || len(a.Captures.StartedTasks) != 0 || !store.RecordWakesReader(*a) || f.outboxWakes(f.management(), off) != 1 {
		t.Fatalf("A9: intake off, arrival = %+v", a)
	}
}

// A5 and the representative-day case "two captures of one event open one
// story": the records of one event are one task, and the one task opens one
// story for both.
func TestTwoCapturesOfOneEventOpenOneStory(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-event")
	ctx := context.Background()
	f.putOwnerSchedule("event:ev-harbor")
	event := &types.CaptureEvent{EventID: "ev-harbor", Reason: types.CaptureEventCorroborated, Headline: "Harbor strike begins", Origins: []string{"a.example", "b.example"}}
	items := []types.CaptureArrival{{CaptureID: "cap-ev1", Title: "Harbor strike begins", URL: "https://a.example/1", SourceID: "rss:a"}, {CaptureID: "cap-ev2", Title: "Dockers walk out", URL: "https://b.example/1", SourceID: "rss:b"}}
	req, err := capturesRecordRequest(f.owner, f.computer, "event:ev-harbor", "event-corroborated:ev-harbor", false, event, items)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.rt.commitCapturesRecord(ctx, req); err != nil {
		t.Fatal(err)
	}
	// A second record of the same event in one command joins the same task.
	grew := types.CaptureArrival{CaptureID: "cap-ev3", Title: "Harbor strike enters day two", URL: "https://c.example/1", SourceID: "rss:c"}
	req2, _ := capturesRecordRequest(f.owner, f.computer, "event:ev-harbor", "event-grew-unheld:ev-harbor", false, event, []types.CaptureArrival{grew})
	req3, _ := capturesRecordRequest(f.owner, f.computer, "event:ev-harbor", "event-grew-unheld2:ev-harbor", false, event, []types.CaptureArrival{{CaptureID: "cap-ev4", Title: "Harbor strike: talks", URL: "https://d.example/1", SourceID: "rss:d"}})
	both := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "event-pair", Records: append(req2.Records, req3.Records...)}
	both.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(both)
	if err := f.rt.commitCapturesRecord(ctx, both); err != nil {
		t.Fatal(err)
	}
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 || len(queued[0].LeasedCaptures) != 2 || strings.Join(queued[1].LeasedCaptures, ",") != "cap-ev3,cap-ev4" {
		t.Fatalf("A5: tasks = %+v", queued)
	}
	if got := f.record(both.Records[0].RecordID); got == nil || len(got.Captures.StartedTasks) != 1 || got.Captures.StartedTasks[0] != queued[1].TaskID {
		t.Fatalf("grouped records started %+v", got)
	}
	if got := f.record(both.Records[1].RecordID); got == nil || len(got.Captures.StartedTasks) != 1 || got.Captures.StartedTasks[0] != queued[1].TaskID {
		t.Fatalf("grouped records started %+v", got)
	}

	// A free slot takes the newest task (store.NewestQueuedFirst): close
	// the later one so the first task, holding both captures, runs.
	f.rt.closeManagementTask(ctx, queued[1], types.TaskClosureCancelled, "", "cancelled by the test", "")
	headBefore, _ := f.s.StoryLibraryHeadOf(ctx, f.owner, f.computer)
	f.runTask(queued[0], openStoryTaskCell)
	if head, _ := f.s.StoryLibraryHeadOf(ctx, f.owner, f.computer); head != headBefore+1 {
		t.Fatalf("A5: two captures of one event moved the library head %d -> %d", headBefore, head)
	}
	d1, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-ev1")
	d2, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-ev2")
	if d1.Kind != types.DispositionOpen || d1.Inbox == "" || d2.Inbox != d1.Inbox {
		t.Fatalf("A5: dispositions %+v, %+v: want one story", d1, d2)
	}
}

// A6: no text a model wrote reaches a host-minted or a delegated task Cast,
// and the delegated Cast is stamped with the delegated records' labels
// rather than the tainted management run's.
func TestNoModelTextReachesATaskCast(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-notext")
	ctx := context.Background()
	f.taintManagement()
	const marker = "EXFIL-7f3a"
	f.model.mu.Lock()
	f.model.parentCell = `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { ids = append(ids, r.RecordID) } }
choir.Delegate("intake", ids, "per_record")`
	f.model.mu.Unlock()

	hostID := f.arrive("rss:nt-host", types.CaptureArrival{CaptureID: "cap-nt-host", Title: "Harbor reopens", URL: "https://nt.example/1"})
	// A captures record seeded straight to management's inbox, delegated by
	// the tainted management run with its marker-bearing arguments.
	seedArrival(t, f.s, f.owner, f.computer, "rss:nt-delegate", []types.CaptureArrival{{CaptureID: "cap-nt-del", Title: "Harbor closes", URL: "https://nt.example/2"}})
	delegatedID := types.CaptureArrivalRecordID("rss:nt-delegate", []string{"cap-nt-del"})
	f.parentTurn()

	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 2 {
		t.Fatalf("tasks = %+v (parent output %s)", queued, f.model.lastOutput())
	}
	for _, task := range queued {
		if strings.Contains(task.Objective, marker) || task.Objective != managementIntakeObjective {
			t.Fatalf("task %s objective = %q", task.TaskID, task.Objective)
		}
		cast := f.record(task.OpeningRecordID)
		raw, _ := json.Marshal(cast)
		if strings.Contains(string(raw), marker) {
			t.Fatalf("model text reached task %s's Cast: %s", task.TaskID, raw)
		}
		if cast.Directive.Objective != managementIntakeObjective || cast.Prediction.Hypothesis == marker || len(cast.Prediction.Questions) != 0 {
			t.Fatalf("Cast %s = directive %+v prediction %+v", cast.RecordID, cast.Directive, cast.Prediction)
		}
		// The management run is tainted with computers; the Cast is not.
		if hasString(cast.Provenance.Labels, computersLabel) {
			t.Fatalf("Cast %s carries the management run's taint: %v", cast.RecordID, cast.Provenance.Labels)
		}
		arrival := hostID
		if cast.Provenance.Origin != types.OriginSystem {
			arrival = delegatedID
		}
		want := labelsOfRecord(*f.record(arrival))
		if got := types.NewLabelSet(cast.Provenance.Labels...); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("Cast %s labels = %v, want %v", cast.RecordID, got, want)
		}
	}
	// The delegated Cast keeps its sub-call tag (the judge observation), with
	// a fixed question rather than the model's.
	delegated := f.record(queued[1].OpeningRecordID)
	if delegated == nil || delegated.SubCall == nil || delegated.SubCall.Kind != types.SubCallCast {
		t.Fatalf("the delegated Cast lost its sub-call tag: %+v", delegated)
	}
	_ = ctx
}

// Representative day: the management session is tainted with computers before
// the captures arrive. The host-started intake task's Cast, the story it
// opens and that story's publication show no ask.
func TestRepresentativeDayIntakeUnderATaintedManagementSession(t *testing.T) {
	f := newDelegationFixture(t, "owner-s2a-day")
	f.taintManagement()
	f.putOwnerSchedule("rss:day")
	// Management's session is already tainted: a turn over an earlier
	// arrival it delegates itself.
	seedArrival(t, f.s, f.owner, f.computer, "rss:day-earlier", []types.CaptureArrival{{CaptureID: "cap-day-earlier", Title: "Listing", URL: "https://day.example/0"}})
	f.parentTurn()
	if len(f.tasks(types.ManagementTaskQueued)) != 1 {
		t.Fatalf("the tainted management turn did not delegate: %s", f.model.lastOutput())
	}

	// K is 1: the earlier task runs first (it declines its listing).
	f.runTask(f.tasks(types.ManagementTaskQueued)[0], taskCellFor(f.pope))
	// The captures arrive; the host starts the task.
	f.arrive("rss:day", types.CaptureArrival{CaptureID: "cap-day-1", Title: "Harbor strike begins", URL: "https://day.example/1"})
	var task types.ManagementTask
	for _, q := range f.tasks(types.ManagementTaskQueued) {
		if q.LeasedCaptures[0] == "cap-day-1" {
			task = q
		}
	}
	if task.TaskID == "" {
		t.Fatalf("no host-started task: %+v", f.tasks(types.ManagementTaskQueued))
	}
	f.runTask(task, openStoryTaskCell)
	inbox, cast := f.storyOpening("cap-day-1")
	f.requireStoryPublishesWithoutAsk(inbox, cast)
	// The sink itself: the story's Texture-side labels are all admitted.
	if cast.Provenance.Desk != f.management() && !strings.HasPrefix(cast.SenderInbox, agentprofile.Management) {
		t.Fatalf("story Cast sender = %+v", cast)
	}
}

// A residual re-delegated by a tainted management run carries no ask: the
// task's Report is stamped without the delegating run's labels, and the new
// task's Cast takes the delegated records' labels only.
func TestResidualRedelegatedByATaintedManagementRunAsksNothing(t *testing.T) {
	f := newDelegationFixture(t, "owner-s2a-residual")
	ctx := context.Background()
	f.putOwnerSchedule("rss:res")
	arrivalID := f.arrive("rss:res", types.CaptureArrival{CaptureID: "cap-res-1", Title: "Pope recovers", URL: "https://res.example/1"}, types.CaptureArrival{CaptureID: "cap-res-2", Title: "please leave this", URL: "https://res.example/2"})
	task := f.tasks(types.ManagementTaskQueued)[0]
	first := f.runTask(task, taskCellFor(f.pope))
	report := f.closingReport(first)
	if o := report.TaskOutcome; o == nil || o.Clean || len(o.Residuals) != 1 || o.Residuals[0].CaptureID != "cap-res-2" {
		t.Fatalf("outcome = %+v", report.TaskOutcome)
	}
	if !store.RecordWakesReader(report) {
		t.Fatal("a residual Report must wake management")
	}
	// A tainted management run re-delegates the residual.
	f.taintManagement()
	f.model.mu.Lock()
	f.model.parentCell = delegateResidualsCell
	f.model.mu.Unlock()
	f.parentTurn()
	var again types.ManagementTask
	for _, q := range f.tasks(types.ManagementTaskQueued) {
		if q.LeasedCaptures[0] == "cap-res-2" {
			again = q
		}
	}
	if again.TaskID == "" {
		t.Fatalf("the residual was not re-delegated: %s", f.model.lastOutput())
	}
	redelegated := f.record(again.OpeningRecordID)
	if hasString(redelegated.Provenance.Labels, computersLabel) {
		t.Fatalf("the re-delegation Cast carries the management run's taint: %v", redelegated.Provenance.Labels)
	}
	if want := labelsOfRecord(*f.record(arrivalID)); strings.Join(redelegated.Provenance.Labels, ",") != strings.Join(want, ",") {
		t.Fatalf("re-delegation labels = %v, want the arrival's %v", redelegated.Provenance.Labels, want)
	}
	f.runTask(again, openStoryTaskCell)
	inbox, cast := f.storyOpening("cap-res-2")
	f.requireStoryPublishesWithoutAsk(inbox, cast)
	_ = ctx
}

// A host-started task closed run_passivated by a planned restart returns its
// residual to management, which re-delegates it. Tasks are never resumed.
func TestHostStartedTaskClosedRunPassivatedRedelegates(t *testing.T) {
	owner := "owner-s2a-passivated"
	t.Setenv("CHOIR_OWNER_ID", owner)
	f := newDelegationFixture(t, owner)
	ctx := context.Background()
	f.arrive("rss:ps", types.CaptureArrival{CaptureID: "cap-ps-1", Title: "Harbor strike begins", URL: "https://ps.example/1"})
	task := f.tasks(types.ManagementTaskQueued)[0]
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	f.model.mu.Unlock()
	run, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox)
	if err != nil || run == nil {
		t.Fatalf("start: %v, %v", run, err)
	}
	eventually(t, func() bool {
		rec, err := f.s.GetRunByOwner(ctx, f.owner, run.RunID)
		return err == nil && rec.State == types.RunRunning
	})
	boot := testPeerRuntime(t, f.rt, f.s)
	WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "t"}, true)(boot)
	boot.Start(ctx)
	closed := f.task(task.TaskID)
	if closed.State != types.ManagementTaskClosed || closed.ClosureReason != types.TaskClosureRunPassivated {
		t.Fatalf("the planned boot left the task %+v", closed)
	}
	report := f.closingReport(task)
	if o := report.TaskOutcome; o == nil || len(o.Residuals) != 1 || !store.RecordWakesReader(report) {
		t.Fatalf("report = %+v", report.TaskOutcome)
	}
	f.model.mu.Lock()
	f.model.gate = nil
	f.model.parentCell = delegateResidualsCell
	f.model.mu.Unlock()
	f.parentTurn()
	var again types.ManagementTask
	for _, q := range f.tasks(types.ManagementTaskQueued) {
		if q.LeasedCaptures[0] == "cap-ps-1" {
			again = q
		}
	}
	if again.TaskID == "" || again.TaskID == task.TaskID {
		t.Fatalf("the passivated task's residual was not re-delegated: %s", f.model.lastOutput())
	}
	if closedAgain := f.runTask(again, openStoryTaskCell); closedAgain.ClosureReason != types.TaskClosureDone {
		t.Fatalf("re-delegated task = %+v", closedAgain)
	}
}

// FM11: a management main run tainted with computers that casts a publish-auto
// story itself carries the taint into the story's opening Cast, so the
// publication does ask. The platform-started task above does not.
func TestFM11ATaintedManagementRunOpeningAStoryItselfAsksAtPublication(t *testing.T) {
	f := newDelegationFixture(t, "owner-s2a-fm11")
	f.taintManagement()
	seedArrival(t, f.s, f.owner, f.computer, "rss:fm11", []types.CaptureArrival{{CaptureID: "cap-fm11", Title: "Harbor strike begins", URL: "https://fm11.example/1"}})
	f.model.mu.Lock()
	f.model.parentCell = `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { for _, c := range r.Captures.Items { ids = append(ids, c.CaptureID) } } }
p, _ := choir.Stories(50, "")
fmt.Println("stories", len(p.Stories))
choir.Cast("texture:new", "Harbor strike begins", map[string]any{"captures": ids, "publish": "auto", "hypotheses": []any{map[string]any{"statement": "the strike lasts a week", "probability": 0.5}}}, map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})`
	f.model.mu.Unlock()
	f.parentTurn()
	inbox, cast := f.storyOpening("cap-fm11")
	if !hasString(cast.Provenance.Labels, computersLabel) {
		t.Fatalf("FM11: the story Cast of a tainted main run lacks the taint: %v (output %s)", cast.Provenance.Labels, f.model.lastOutput())
	}
	f.putOwnerSchedule("rss:fm11")
	call := disclosureCall{exec: toolregistry.ExecutionContext{OwnerID: f.owner, ComputerID: f.computer}, deskRole: agentprofile.Texture, target: strings.TrimPrefix(inbox, "texture:")}
	if f.rt.publishAutoAdmission(context.Background(), call)(computersLabel) {
		t.Fatal("FM11: the publication admitted computers on a publish-auto story")
	}
}

// 13.6 item 1: an event-aged record (official_single_outlet) needs a second
// source a task cannot ask research for, so it mints no task and wakes
// management's main run as before.
func TestEventAgedArrivalWakesManagementAndMintsNoTask(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-aged")
	event := &types.CaptureEvent{EventID: "ev-aged", Reason: types.CaptureEventOfficial, Headline: "Ministry statement", Origins: []string{"ministry.example"}}
	items := []types.CaptureArrival{{CaptureID: "cap-aged", Title: "Ministry statement", URL: "https://ministry.example/1", SourceID: "rss:m"}}
	req, err := capturesRecordRequest(f.owner, f.computer, "event:ev-aged", "event-aged:ev-aged", false, event, items)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.rt.commitCapturesRecord(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if n := len(f.tasks(types.ManagementTaskQueued)); n != 0 {
		t.Fatalf("an event-aged record minted %d tasks", n)
	}
	rec := f.record("event-aged:ev-aged")
	if rec == nil || !store.RecordWakesReader(*rec) || f.outboxWakes(f.management(), "event-aged:ev-aged") != 1 {
		t.Fatalf("an event-aged record must wake management: %+v", rec)
	}
}

// 13.6 item 2: a Delegate intent that carries an objective and request
// questions (the host-side path a crafted intent could take) leaves neither
// on the Cast: the objective is the constant and the Cast is weight zero,
// with no questions.
func TestDelegateCastDropsObjectiveAndRequestQuestions(t *testing.T) {
	const marker = "EXFIL-9c1d"
	r := &rlmCallReduction{scope: ReductionScope{CellID: "cell-x", RunID: "run-m", ComputerID: "c1", OwnerID: "o1"}}
	cast := types.CommitmentRecord{RecordID: "cell-x:delegate:L1:0", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: managementIntakeObjective}, Cast: &types.CommitmentCast{},
		Addressee: "management-task:t1", SenderInbox: "management:o1"}
	in := yaegikernel.StagedIntent{Kind: yaegikernel.IntentDelegate, LocalID: "L1", Objective: marker + " objective",
		Req: &types.RequestFields{PSuccess: fp(0.8), Weight: fp(0.5), Settlement: types.SettlementRecipient,
			Questions: []types.TypedQuestion{{Question: marker + " question", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.6, "no": 0.4}, Weight: 0.5}}}}
	if err := r.typeDelegateCast(&cast, in, "t1"); err != nil {
		t.Fatal(err)
	}
	if err := types.ValidateSubCallCast(cast); err != nil || cast.PSuccess != nil || cast.Weight != nil || cast.Settlement != "" {
		t.Fatalf("the Delegate Cast carries a stated weight or fails validation: %v %+v", err, cast)
	}
	raw, _ := json.Marshal(cast)
	if strings.Contains(string(raw), marker) || !cast.IsWeightZero() || cast.SubCall == nil || cast.Prediction.Hypothesis != managementIntakeObjective {
		t.Fatalf("the Delegate Cast carries the cell's text or a stated weight: %s", raw)
	}
}

// 13.7: a held growth record starts its own task. A corroboration followed by
// a growth record ends with the growth captures fed to the story, and
// management's main run is woken by neither.
func TestGrowthRecordStartsATaskThatFeedsTheStory(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-growth")
	ctx := context.Background()
	event := &types.CaptureEvent{EventID: "ev-growth", Reason: types.CaptureEventCorroborated, Headline: "Harbor strike begins", Origins: []string{"a.example", "b.example"}}
	items := []types.CaptureArrival{{CaptureID: "cap-gr1", Title: "Harbor strike begins", URL: "https://a.example/1", SourceID: "rss:a"}, {CaptureID: "cap-gr2", Title: "Dockers walk out", URL: "https://b.example/1", SourceID: "rss:b"}}
	req, _ := capturesRecordRequest(f.owner, f.computer, "event:ev-growth", "event-corroborated:ev-growth", false, event, items)
	if err := f.rt.commitCapturesRecord(ctx, req); err != nil {
		t.Fatal(err)
	}
	f.runTask(f.tasks(types.ManagementTaskQueued)[0], openStoryTaskCell)
	story, _ := f.storyOpening("cap-gr1")

	grew := *event
	grew.Reason = types.CaptureEventGrew
	greq, _ := capturesRecordRequest(f.owner, f.computer, "event:ev-growth", "event-grew:ev-growth:p2", true, &grew,
		[]types.CaptureArrival{{CaptureID: "cap-gr3", Title: "Harbor strike enters day two", URL: "https://c.example/1", SourceID: "rss:c"}})
	if err := f.rt.commitCapturesRecord(ctx, greq); err != nil {
		t.Fatal(err)
	}
	rec := f.record("event-grew:ev-growth:p2")
	queued := f.tasks(types.ManagementTaskQueued)
	if rec == nil || !rec.Captures.Held || len(rec.Captures.StartedTasks) != 1 || len(queued) != 1 || queued[0].LeasedCaptures[0] != "cap-gr3" {
		t.Fatalf("growth record %+v, queued %+v", rec, queued)
	}
	if store.RecordWakesReader(*rec) || f.outboxWakes(f.management(), "event-grew:ev-growth:p2") != 0 {
		t.Fatal("a growth record woke management's main run")
	}
	f.runTask(queued[0], `choir.Feed("`+story+`", []string{"cap-gr3"}, "the same strike, day two", map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"})
choir.Done("fed the strike story")`)
	if d, _, _ := f.s.CaptureDispositionOf(ctx, f.owner, f.computer, "cap-gr3"); d.Kind != types.DispositionFeed || d.Inbox != story {
		t.Fatalf("the growth capture = %+v, want fed to %s", d, story)
	}
}

// 13.7: two groups in one command mint distinct task ids (the chunk counter
// runs across groups).
func TestTwoGroupsInOneCommandMintDistinctTaskIDs(t *testing.T) {
	f := newDelegationFixture(t, "owner-arrival-ids")
	ev := func(id string) *types.CaptureEvent {
		return &types.CaptureEvent{EventID: id, Reason: types.CaptureEventCorroborated, Origins: []string{"a", "b"}}
	}
	r1, _ := capturesRecordRequest(f.owner, f.computer, "event:e1", "event-corroborated:e1", false, ev("e1"), []types.CaptureArrival{{CaptureID: "cap-id1", Title: "one", URL: "https://x.example/1"}})
	r2, _ := capturesRecordRequest(f.owner, f.computer, "event:e2", "event-corroborated:e2", false, ev("e2"), []types.CaptureArrival{{CaptureID: "cap-id2", Title: "two", URL: "https://x.example/2"}})
	both := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "two-groups", Records: append(r1.Records, r2.Records...)}
	both.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(both)
	planned, started, err := f.rt.withIntakeTask(context.Background(), both)
	if err != nil || !started || len(planned.Tasks) != 2 || planned.Tasks[0].TaskID == planned.Tasks[1].TaskID {
		t.Fatalf("planned %+v, started %v, %v", planned.Tasks, started, err)
	}
	if err := f.rt.commitCapturesRecord(context.Background(), both); err != nil {
		t.Fatal(err)
	}
	if n := len(f.tasks(types.ManagementTaskQueued)); n != 2 {
		t.Fatalf("tasks = %d", n)
	}
}
