package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management tasks (management delegation S2a,
// docs/archive/reports/management-delegation-design-2026-10-14.md, round 2).
//
// Management hands capture intake to tasks with choir.Delegate. A task is a
// run of profile management and role management_task, reading its own inbox
// (management-task:<id>) in its own desk session, sending outward as
// management:<owner> with the task named in provenance, and ending with one
// final Report the host mints. The parent never waits on a task: Delegate
// commits a Cast per task and returns.
//
// The task row, the lease (the open round), the disposition index and the
// task's outcome are the store's (internal/store/management_tasks.go). The
// runtime starts tasks (the slot starter), closes them when their run ends,
// and shapes their run: its registry, prompt, model and deadline.

// managementTaskDeadline bounds a task's run from its start (O7, B10): the
// activation budget of a task run.
const managementTaskDeadline = 10 * time.Minute

// managementTaskSlots is K (O7, S2b): how many tasks run at once on this
// computer; the rest queue, never refused. It is the runtime config's
// ManagementTaskSlots (default 4, at most 8), or 1 while the owner's
// management.serial_tasks setting is on: the live rollback to S2a's one at
// a time, read on every start, so it needs no restart. Concurrent tasks
// are kept coherent by the store, not by K: the lease and the disposition
// index (one capture, one disposition) and the story-library head (an open
// from a run that has not seen the stories opened since is refused).
func (rt *Runtime) managementTaskSlots(ctx context.Context, ownerID, computerID string) int {
	if rt.computerSettingOn(ctx, ownerID, computerID, store.SettingManagementSerialTasks) {
		return 1
	}
	return provideriface.NormalizeManagementTaskSlots(rt.cfg.ManagementTaskSlots)
}

// managementTaskCallTimeout bounds one model call of a task (R2-9), so one
// slow call cannot use the task's whole deadline. A var so a test can
// shorten it.
var managementTaskCallTimeout = 3 * time.Minute

// managementTaskPrompt is the first user turn of a task's run.
const managementTaskPrompt = "Management delegated captures to you: read them with choir.Records(). The Cast from management names your objective and r.Cast.Captures, the captures you lease; each captures record (r.Captures.Items, from source:<id>) shows them with Jev's label in r.Labels. Open, feed or decline every one of them, then end with choir.Done(note)."

// isManagementTaskRun reports a run of a management task.
func isManagementTaskRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	_, ok := types.ManagementTaskIDOfInbox(rec.AgentID)
	return ok && agentRoleForRun(rec) == agentprofile.ManagementTask
}

// managementTaskIDOfRun returns the task a run serves.
func managementTaskIDOfRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	id, _ := types.ManagementTaskIDOfInbox(rec.AgentID)
	return id
}

// managementTaskID derives a task's id from the cell that delegated it, so a
// replayed cell opens the same task (and the store refuses a second one).
func managementTaskID(cellID, localID string, group int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("choir:management-task\x00%s\x00%s\x00%d", cellID, localID, group)))
	return "mt-" + hex.EncodeToString(sum[:10])
}

// startQueuedManagementTasks is the slot starter (R2-6): while fewer than K
// tasks run, it claims the oldest queued task and starts its run. A queued
// task is an open task round with no run; its start is the inbox_record
// wake of its opening Cast, or the start wake a task's closure mints for
// the next one. Both are restart work kinds: a crash boot interrupts them
// (and closes the rounds), a planned boot delivers them. One wake fills
// every free slot (S2b, FM8), in one pass over one listing of the queue, so
// a task that cannot start is not retried within the call and a refused
// claim takes no slot (FM29). It returns the first run it started.
func (rt *Runtime) startQueuedManagementTasks(ctx context.Context, ownerID, computerID string) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil {
		return nil, nil
	}
	rt.managementTaskStartMu.Lock()
	defer rt.managementTaskStartMu.Unlock()
	slots := rt.managementTaskSlots(ctx, ownerID, computerID)
	running, err := rt.store.ListManagementTasks(ctx, ownerID, computerID, types.ManagementTaskRunning)
	if err != nil {
		return nil, fmt.Errorf("management tasks: list running: %w", err)
	}
	busy := 0
	for _, task := range running {
		if task.RunID == "" {
			busy++ // claimed, its run starting
			continue
		}
		run, err := rt.getRunForComputer(ctx, ownerID, task.RunID)
		if err != nil || !(run.State.Terminal() || run.State == types.RunPassivated) {
			busy++
			continue
		}
		// The run ended and its closure has not landed: close it now, so
		// the slot frees (the closure is idempotent).
		rt.closeManagementTaskOfEndedRun(ctx, &run)
	}
	if busy >= slots {
		return nil, nil
	}
	queued, err := rt.store.ListManagementTasks(ctx, ownerID, computerID, types.ManagementTaskQueued)
	if err != nil {
		return nil, fmt.Errorf("management tasks: list queued: %w", err)
	}
	var first *types.RunRecord
	for _, task := range queued {
		if busy >= slots {
			break
		}
		rec, err := rt.startManagementTask(ctx, task, slots)
		if err != nil {
			log.Printf("runtime: management task %s did not start: %v", task.TaskID, err)
			continue
		}
		if rec == nil {
			continue // claimed or closed by another path: no slot taken
		}
		busy++
		if first == nil {
			first = rec
		}
	}
	return first, nil
}

// managementTaskObservability is the slot starter's state on
// /api/runtime/observability (S2b): the probe reads K and the queue from a
// product route. Slots is K in force; ConfiguredSlots is the runtime
// config's K, which management.serial_tasks overrides with 1.
type managementTaskObservability struct {
	Slots            int     `json:"slots"`
	ConfiguredSlots  int     `json:"configured_slots"`
	SerialTasks      bool    `json:"serial_tasks"`
	Running          int     `json:"running"`
	Queued           int     `json:"queued"`
	OldestQueuedAgeS float64 `json:"oldest_queued_age_s,omitempty"`
	Error            string  `json:"error,omitempty"`
}

func (rt *Runtime) managementTaskObservability(ctx context.Context, ownerID, computerID string) *managementTaskObservability {
	out := &managementTaskObservability{
		Slots:           rt.managementTaskSlots(ctx, ownerID, computerID),
		ConfiguredSlots: provideriface.NormalizeManagementTaskSlots(rt.cfg.ManagementTaskSlots),
		SerialTasks:     rt.computerSettingOn(ctx, ownerID, computerID, store.SettingManagementSerialTasks),
	}
	running, err := rt.store.ListManagementTasks(ctx, ownerID, computerID, types.ManagementTaskRunning)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	queued, err := rt.store.ListManagementTasks(ctx, ownerID, computerID, types.ManagementTaskQueued)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Running, out.Queued = len(running), len(queued)
	if len(queued) > 0 {
		out.OldestQueuedAgeS = time.Since(queued[0].CreatedAt).Round(100 * time.Millisecond).Seconds()
	}
	return out
}

// startManagementTask claims one queued task's slot and starts its run. A
// task that cannot start is closed with the error, so its captures return
// to the parent as residuals.
func (rt *Runtime) startManagementTask(ctx context.Context, task types.ManagementTask, slots int) (*types.RunRecord, error) {
	claimed, err := rt.store.ClaimManagementTask(ctx, task.OwnerID, task.ComputerID, task.TaskID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrInboxRefused) {
			return nil, nil // started or closed by another path
		}
		return nil, err
	}
	// The parent run is provenance only (I3): it is recorded under its own
	// key, not requested_by_run_id, whose child-run machinery (terminal
	// outcome delivery to a requester) would hang the task on the parent.
	metadata := map[string]any{
		runMetadataAgentProfile:          agentprofile.Management,
		runMetadataAgentRole:             agentprofile.ManagementTask,
		runMetadataAgentID:               claimed.TaskInbox,
		runMetadataChannelID:             claimed.TaskInbox,
		"request_source":                 inboxRecordRequestSource,
		"management_task_id":             claimed.TaskID,
		"management_task_kind":           string(claimed.Kind),
		"management_task_parent_run_id":  claimed.ParentRunID,
		"management_task_parent_inbox":   claimed.ParentInbox,
		"management_task_deadline":       claimed.DeadlineAt.UTC().Format(time.RFC3339Nano),
		"management_task_leased_count":   len(claimed.LeasedCaptures),
		"management_task_opening_record": claimed.OpeningRecordID,
		// K when it started, so the probe reads concurrency against the
		// slots in force (S2b).
		"management_task_slots": slots,
	}
	rec, err := rt.createRunWithMetadata(ctx, managementTaskPrompt, claimed.OwnerID, metadata)
	if err != nil {
		rt.closeManagementTask(ctx, claimed, types.TaskClosureCancelled, "", "the task's run did not start: "+err.Error(), "")
		return nil, err
	}
	if _, err := rt.store.BindManagementTaskRun(ctx, claimed.OwnerID, claimed.ComputerID, claimed.TaskID, rec.RunID); err != nil {
		// The task was cancelled or closed while its run was created.
		_ = rt.CancelRun(context.WithoutCancel(ctx), rec.RunID, rec.OwnerID)
		return nil, err
	}
	rt.activate(rec)
	log.Printf("runtime: started management task %s (%d captures) as run %s", claimed.TaskID, len(claimed.LeasedCaptures), rec.RunID)
	return rec, nil
}

// closeManagementTask mints a task's final Report (R2-8): from the task
// inbox to management, closing the task round. The store fills in what each
// leased capture became; a clean Report waits for the parent's next turn,
// any other wakes it (R2-3). A round already closed (by Done, a crash boot,
// or a cancel) is left as it is.
func (rt *Runtime) closeManagementTask(ctx context.Context, task types.ManagementTask, reason, note, errText, runID string) {
	if rt == nil || rt.store == nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	current, err := rt.store.GetManagementTask(ctx, task.OwnerID, task.ComputerID, task.TaskID)
	if err != nil {
		log.Printf("runtime: close management task %s: %v", task.TaskID, err)
		return
	}
	if current.State == types.ManagementTaskClosed {
		return
	}
	if runID == "" {
		runID = current.RunID
	}
	closing := types.NewTaskClosingReport(current, reason, note, errText, runID, time.Now().UTC().Format(time.RFC3339Nano))
	closing.ComputerID = current.ComputerID
	req := types.CommitInboxRecordsRequest{OwnerID: current.OwnerID, ComputerID: current.ComputerID, CommandID: "task-closure:" + closing.RecordID, Records: []types.CommitmentRecord{closing}}
	if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
		log.Printf("runtime: close management task %s: %v", task.TaskID, err)
		return
	}
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		if !errors.Is(err, store.ErrInboxRefused) {
			log.Printf("runtime: close management task %s (%s): %v", task.TaskID, reason, err)
		}
		return
	}
	log.Printf("runtime: closed management task %s: %s", task.TaskID, reason)
}

// closeManagementTaskOfEndedRun closes the task a run served when the run
// ends without Done (R2-8, R2-12): reason deadline when its activation
// budget fired, else run_<state>. The task's desk session ends with it: a
// task session never takes a slot in the idle cache. A crash-closed run
// closes nothing; the crash boot closed the round.
func (rt *Runtime) closeManagementTaskOfEndedRun(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || !isManagementTaskRun(rec) || rec.Error == runFateInterruptedByRestart {
		return
	}
	if !rec.State.Terminal() && rec.State != types.RunPassivated {
		return
	}
	rt.deskSessionWorkers().release("agent:" + strings.TrimSpace(rec.AgentID))
	task, err := rt.store.GetManagementTask(ctx, rec.OwnerID, rec.ComputerID, managementTaskIDOfRun(rec))
	if err != nil {
		log.Printf("runtime: task of ended run %s: %v", rec.RunID, err)
		return
	}
	if task.RunID != "" && task.RunID != rec.RunID {
		return
	}
	reason := "run_" + string(rec.State)
	errText := ""
	switch {
	case rec.State == types.RunPassivated:
		reason = types.TaskClosureRunPassivated
	case strings.Contains(rec.Error, "activation budget exceeded"):
		reason = types.TaskClosureDeadline
		errText = "the task's 10-minute deadline passed"
	case rec.State != types.RunCompleted:
		errText = rec.Error
	}
	rt.closeManagementTask(ctx, task, reason, "", errText, rec.RunID)
}

// managementTaskOfScope loads the running task a task-scope cell serves.
func (r *rlmCallReduction) managementTaskOfScope(ctx context.Context) (types.ManagementTask, error) {
	taskID := managementTaskIDOfRun(r.rec)
	task, err := r.rt().store.GetManagementTask(ctx, r.scope.OwnerID, r.scope.ComputerID, taskID)
	if err != nil {
		return types.ManagementTask{}, fmt.Errorf("management task %s: %w", taskID, err)
	}
	return task, nil
}

// planDelegation turns one Delegate intent into task rows and the Casts
// that open their rounds (R2-14). A delegated record's captures that are
// already disposed or leased are left out; a record with none left is
// skipped, and a Delegate with nothing left is refused.
func (r *rlmCallReduction) planDelegation(ctx context.Context, in yaegikernel.StagedIntent) ([]types.CommitmentRecord, []types.ManagementTask, error) {
	rt := r.rt()
	var spec yaegikernel.DelegateSpec
	if err := json.Unmarshal([]byte(in.Body), &spec); err != nil {
		return nil, nil, fmt.Errorf("delegate %s: %w", in.LocalID, err)
	}
	if spec.Kind != yaegikernel.TaskKindIntake {
		return nil, nil, fmt.Errorf("delegate %s: kind %q is not open; S2a delegates intake", in.LocalID, spec.Kind)
	}
	if !rt.intakeSurfacesEnabled(ctx, r.scope.OwnerID, r.scope.ComputerID) {
		return nil, nil, fmt.Errorf("delegate %s: intake tasks feed and decline with choir.Feed and choir.Decline, which are off on this computer (the owner's intake.labels and intake.event_wakes settings)", in.LocalID)
	}
	// A captures record delegates its captures; a task's final Report
	// delegates its residuals again, with the records its task was given.
	byID := map[string]yaegikernel.InboxRecord{}
	for _, view := range r.records.view {
		if view.Captures != nil && len(view.Captures.Items) > 0 {
			byID[view.RecordID] = view
		}
		if view.TaskOutcome != nil && len(view.TaskOutcome.Residuals) > 0 && strings.HasPrefix(view.From, types.ManagementTaskDesk+":") {
			byID[view.RecordID] = view
		}
	}
	parent := persistentManagementAgentID(r.scope.OwnerID)
	now := time.Now().UTC()
	var casts []types.CommitmentRecord
	var tasks []types.ManagementTask
	var skipped []string
	seenCapture := map[string]bool{}
	for i, group := range spec.Groups {
		var leased, delegated []string
		var titles []string
		seenSource := map[string]bool{}
		for _, id := range group {
			view, ok := byID[strings.TrimSpace(id)]
			if !ok {
				return nil, nil, fmt.Errorf("delegate %s: %s is not a captures record or a task Report with residuals this turn was shown", in.LocalID, id)
			}
			type candidate struct{ id, title string }
			var candidates []candidate
			sources := []string{view.RecordID}
			if view.Captures != nil {
				for _, item := range view.Captures.Items {
					candidates = append(candidates, candidate{item.CaptureID, item.Title})
				}
			} else {
				for _, residual := range view.TaskOutcome.Residuals {
					candidates = append(candidates, candidate{residual.CaptureID, ""})
				}
				// The new task is shown the captures records the old one
				// was given, cut to what it leases.
				prior, err := rt.store.GetManagementTask(ctx, r.scope.OwnerID, r.scope.ComputerID, view.TaskOutcome.TaskID)
				if err != nil {
					return nil, nil, fmt.Errorf("delegate %s: task %s: %w", in.LocalID, view.TaskOutcome.TaskID, err)
				}
				sources = prior.DelegatedRecordIDs
			}
			kept := 0
			for _, c := range candidates {
				free, err := rt.captureDelegable(ctx, r.scope.OwnerID, r.scope.ComputerID, c.id)
				if err != nil {
					return nil, nil, fmt.Errorf("delegate %s: %w", in.LocalID, err)
				}
				if !free || seenCapture[c.id] {
					continue
				}
				seenCapture[c.id] = true
				leased = append(leased, c.id)
				if c.title != "" {
					titles = append(titles, c.title)
				}
				kept++
			}
			if kept > 0 {
				for _, source := range sources {
					if !seenSource[source] {
						seenSource[source] = true
						delegated = append(delegated, source)
					}
				}
			} else {
				skipped = append(skipped, view.RecordID)
			}
		}
		if len(leased) == 0 {
			continue
		}
		if len(leased) > yaegikernel.MaxTaskCaptures {
			return nil, nil, fmt.Errorf("delegate %s: group %d leases %d captures; a task takes at most %d", in.LocalID, i+1, len(leased), yaegikernel.MaxTaskCaptures)
		}
		taskID := managementTaskID(r.scope.CellID, in.LocalID, i)
		inbox := types.ManagementTaskInbox(taskID)
		body, _ := json.Marshal(map[string]any{"kind": spec.Kind, "records": delegated, "captures": leased, "titles": titles})
		cast := commitmentRecordForIntent(r.scope, yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, ToDesk: inbox, Objective: spec.Objective, Statement: string(body)})
		cast.RecordID = fmt.Sprintf("%s:delegate:%s:%d", r.scope.CellID, in.LocalID, i)
		cast.Inbox, cast.SenderInbox, cast.Addressee = inbox, parent, inbox
		cast.EmitterKind, cast.ComputerID = types.CommitmentEmitterAgent, r.scope.ComputerID
		cast.Cast = &types.CommitmentCast{Captures: leased}
		cast.RelatedIDs = delegated
		casts = append(casts, cast)
		tasks = append(tasks, types.ManagementTask{
			TaskID: taskID, OwnerID: r.scope.OwnerID, ComputerID: r.scope.ComputerID, Kind: types.ManagementTaskIntake,
			Objective: spec.Objective, ParentInbox: parent, ParentRunID: r.scope.RunID, TaskInbox: inbox,
			ConversationID: types.InboxConversationID(parent, inbox), OpeningRecordID: cast.RecordID,
			DelegatedRecordIDs: delegated, LeasedCaptures: leased, DeadlineSeconds: int(managementTaskDeadline / time.Second),
			State: types.ManagementTaskQueued, CreatedAt: now, UpdatedAt: now,
		})
	}
	if len(tasks) == 0 {
		return nil, nil, fmt.Errorf("delegate %s: every capture these records name is already disposed, leased or closed (%s)", in.LocalID, strings.Join(skipped, ", "))
	}
	return casts, tasks, nil
}

// captureDelegable reports whether a capture can be leased: not disposed,
// not closed after two crashes, and not leased to an open task round. The
// store checks the same under the commit lock; this only leaves out what
// is already settled, so one disposed capture does not refuse its record.
func (rt *Runtime) captureDelegable(ctx context.Context, ownerID, computerID, captureID string) (bool, error) {
	d, found, err := rt.store.CaptureDispositionOf(ctx, ownerID, computerID, captureID)
	if err != nil || !found {
		return err == nil, err
	}
	if d.Kind != "" || d.Fate == types.CaptureFateClosedAfterCrashes {
		return false, nil
	}
	if d.LeaseTask == "" {
		return true, nil
	}
	task, err := rt.store.GetManagementTask(ctx, ownerID, computerID, d.LeaseTask)
	if errors.Is(err, store.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return task.State == types.ManagementTaskClosed, nil
}

// cancelManagementTask is CancelTask (R2-20): a running task's run is
// cancelled, and the run-end closure closes its round run_cancelled; a
// queued task's round is closed cancelled. Either Report lists the
// residuals.
func (r *rlmCallReduction) cancelManagementTask(ctx context.Context, in yaegikernel.StagedIntent) error {
	rt := r.rt()
	taskID := strings.TrimPrefix(strings.TrimSpace(in.TargetRef), types.ManagementTaskDesk+":")
	task, err := rt.store.GetManagementTask(ctx, r.scope.OwnerID, r.scope.ComputerID, taskID)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("cancel_task %s: no task %s", in.LocalID, taskID)
	}
	if err != nil {
		return fmt.Errorf("cancel_task %s: %w", in.LocalID, err)
	}
	if task.ParentInbox != persistentManagementAgentID(r.scope.OwnerID) {
		return fmt.Errorf("cancel_task %s: task %s is not this desk's", in.LocalID, taskID)
	}
	switch {
	case task.State == types.ManagementTaskClosed:
		return nil
	case task.State == types.ManagementTaskRunning && task.RunID != "":
		if err := rt.CancelRun(context.WithoutCancel(ctx), task.RunID, r.scope.OwnerID); err != nil && !strings.Contains(err.Error(), "cannot cancel") {
			return fmt.Errorf("cancel_task %s: %w", in.LocalID, err)
		}
		// terminalizeRun's run-end closure closed the round; a run that
		// had already ended is closed here.
		rt.closeManagementTask(ctx, task, "run_cancelled", in.Body, "cancelled by management", task.RunID)
	default:
		rt.closeManagementTask(ctx, task, types.TaskClosureCancelled, in.Body, "cancelled by management", "")
	}
	return nil
}

// noteStoryLibraryShown records the story-library head a run read with
// choir.Stories() (R2-5): its later texture:new commits are stamped with
// it.
func (rt *Runtime) noteStoryLibraryShown(ctx context.Context, runID, ownerID, computerID string) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return
	}
	head, err := rt.store.StoryLibraryHeadOf(ctx, ownerID, computerID)
	if err != nil {
		return
	}
	rt.storyLibraryShown.Store(strings.TrimSpace(runID), head)
}

// storyLibraryShownMark is the head a run last read, or 0.
func (rt *Runtime) storyLibraryShownMark(runID string) int64 {
	if v, ok := rt.storyLibraryShown.Load(strings.TrimSpace(runID)); ok {
		if head, ok := v.(int64); ok {
			return head
		}
	}
	return 0
}
