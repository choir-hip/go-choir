package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management tasks in the store (management delegation S2a,
// docs/archive/reports/management-delegation-design-2026-10-14.md, round 2).
//
// The store checks what it already holds, under the commit lock, and scores
// nothing:
//
//   - the task row (R2-7) commits with the Cast that opens its round, and
//     the round's open flag is the lease (§4.3): no lease table;
//   - every record a task scope commits names its task (acting_task) and is
//     refused unless the task's round is open and its run is the row's
//     (R2-4, §4.2), so a late commit after a closure cannot land;
//   - a per-capture disposition index (R2-4, §6.4), written in the batch of
//     the record that disposes of the capture, refuses a task's second
//     disposition and any disposition of a capture another task leases; the
//     parent's re-disposition of a free capture is kept as an override;
//   - the task's final Report is minted by the host from the task inbox,
//     and its outcome (dispositions, residuals, clean) is computed here from
//     the index, so "clean" is not anyone's claim (R2-8); a clean Report
//     wakes no one (R2-3), a crash closure spends the capture's crash budget
//     (R2-17), and any other closure mints the durable start of the oldest
//     queued task (R2-6);
//   - a Cast to texture:new is refused when the committing run's last read
//     of the story library is behind its head (R2-5).

const (
	ogKindManagementTask     = objectgraph.ObjectKind("choir.management_task")
	ogKindCaptureDisposition = objectgraph.ObjectKind("choir.capture_disposition")
	ogKindStoryLibraryHead   = objectgraph.ObjectKind("choir.story_library_head")

	storyLibraryKey = "story-library"
	// storyLibraryRecent is how many opens the head keeps for a refusal to
	// name; a refusal names at most storyLibraryRefusalList of them.
	storyLibraryRecent      = 50
	storyLibraryRefusalList = 20
)

// RecordWakesReader is the one wake predicate (R2-3): whether a record wakes
// its recipient. The store mints a wake row exactly when it holds, and the
// runtime's post-turn re-check asks it of each late record, so a record that
// does not wake its reader on commit does not start a turn after one either.
//
// A desk's record wakes its reader. A source inbox has no reader. A system
// record does not wake (D5: a closure is minted at boot, and waking there
// would resume work after a crash), except six minted only by live work:
//   - an engineering progress relay to a Texture (the platform telling the
//     owner's document what engineering reported);
//   - an owner-act record (the owner's decision on a candidate, projected
//     into the document that supervises it);
//   - a stack rebase notice (the runtime's live timer asking the root
//     Texture RLM for a rebase; U4);
//   - a support failure;
//   - a capture arrival that is not held for its events (event layer §15);
//   - a management task's final Report that is not clean and was not
//     written by a crash boot.
//
// A clean task Report waits, unread, for the parent's next turn. This is the
// one list of waking system records (inboxRecordWakes points here).
func RecordWakesReader(rec types.CommitmentRecord) bool {
	if strings.HasPrefix(rec.Inbox, "source:") {
		return false
	}
	if rec.EmitterKind != types.CommitmentEmitterSystem {
		return true
	}
	// An engineering progress relay tells a Texture what engineering
	// reported. It is minted only by the live commit of an agent's Report
	// (never at a crash boot), so waking its reader resumes nothing.
	if types.IsEngineeringProgressRelay(rec) {
		return true
	}
	// An owner-act record tells a Texture that the candidate its document
	// supervises was approved, applied or rejected. It is minted only by
	// the live act (the decision request, or a live apply), never by a
	// crash boot's settlement, so waking its reader resumes nothing.
	if types.IsSelfDevelopmentOwnerAct(rec) {
		return true
	}
	// A rebase notice asks the root Texture RLM to rebase the applied stack.
	// It is minted only by the runtime's live timer, after the stay-up
	// interval since boot, never inside a boot or by a crash boot's
	// settlement; a pre-boot notice's wake is interrupted by the generic
	// crash-boot gate.
	if types.IsStackRebaseNotice(rec) {
		return true
	}
	if rec.Support != nil && len(rec.Support.Failures) > 0 {
		return true
	}
	if rec.Captures != nil && len(rec.Captures.Items) > 0 {
		return !rec.Captures.Held
	}
	if _, fromTask := types.ManagementTaskIDOfInbox(rec.SenderInbox); fromTask && rec.Final && rec.TaskOutcome != nil {
		stop := ""
		if rec.Report != nil {
			stop = rec.Report.StopReason
		}
		return !rec.TaskOutcome.Clean && stop != types.ClosureReasonInterruptedByRestart
	}
	return false
}

func taskConversation(ownerID, taskID string) string {
	return types.InboxConversationID("management:"+ownerID, types.ManagementTaskInbox(taskID))
}

// taskPlan carries one inbox command's task objects: rows, capture
// indexes and the conversation heads it read, each loaded once.
type taskPlan struct {
	s                  *Store
	ctx                context.Context
	ownerID, computer  string
	now                time.Time
	rows               map[string]*inboxCached[types.ManagementTask]
	indexes            map[string]*inboxCached[types.CaptureDisposition]
	heads              map[string]*inboxCached[types.ConversationHead]
	dirtyRows, dirtyIx map[string]bool
	rowOrder, ixOrder  []string
	newTasks           map[string]bool
	wakes              []objectgraph.Object
}

func (p *taskPlan) row(taskID string) (*inboxCached[types.ManagementTask], error) {
	if r, ok := p.rows[taskID]; ok {
		return r, nil
	}
	r, err := loadInboxCached[types.ManagementTask](p.ctx, p.s, ogKindManagementTask, p.ownerID, p.computer, taskID)
	if err == nil {
		p.rows[taskID] = r
	}
	return r, err
}

func (p *taskPlan) index(captureID string) (*inboxCached[types.CaptureDisposition], error) {
	if ix, ok := p.indexes[captureID]; ok {
		return ix, nil
	}
	ix, err := loadInboxCached[types.CaptureDisposition](p.ctx, p.s, ogKindCaptureDisposition, p.ownerID, p.computer, captureID)
	if err == nil {
		ix.value.CaptureID, ix.value.OwnerID, ix.value.ComputerID = captureID, p.ownerID, p.computer
		p.indexes[captureID] = ix
	}
	return ix, err
}

func (p *taskPlan) head(conversationID string) (*inboxCached[types.ConversationHead], error) {
	if h, ok := p.heads[conversationID]; ok {
		return h, nil
	}
	h, err := loadInboxCached[types.ConversationHead](p.ctx, p.s, ogKindConversationHead, p.ownerID, p.computer, conversationID)
	if err == nil {
		p.heads[conversationID] = h
	}
	return h, err
}

// roundOpen reports whether a task's round is open: the lease. A task
// opened in this command holds its round.
func (p *taskPlan) roundOpen(taskID string, round int) (bool, error) {
	if p.newTasks[taskID] {
		return true, nil
	}
	h, err := p.head(taskConversation(p.ownerID, taskID))
	if err != nil {
		return false, err
	}
	return h.exists && h.value.Open && h.value.Round == round, nil
}

func (p *taskPlan) markRow(taskID string) {
	if !p.dirtyRows[taskID] {
		p.dirtyRows[taskID] = true
		p.rowOrder = append(p.rowOrder, taskID)
	}
}

func (p *taskPlan) markIndex(captureID string) {
	if !p.dirtyIx[captureID] {
		p.dirtyIx[captureID] = true
		p.ixOrder = append(p.ixOrder, captureID)
	}
}

// recordDisposition names the captures a management record disposes of
// and how: a feed or decline Note by its disposition, a Cast with captures
// into a story's inbox as an open (it opened the story) or a feed.
func recordDisposition(rec types.CommitmentRecord, opened map[string]bool) (string, []string) {
	switch {
	case rec.Disposition != nil && len(rec.Disposition.Captures) > 0:
		return rec.Disposition.Kind, rec.Disposition.Captures
	case rec.IsCast() && rec.Cast != nil && len(rec.Cast.Captures) > 0 && strings.HasPrefix(rec.Inbox, "texture:"):
		if opened[rec.RecordID] {
			return types.DispositionOpen, rec.Cast.Captures
		}
		return types.DispositionFeed, rec.Cast.Captures
	}
	return "", nil
}

// planManagementTasks checks and plans one command's task rows, task-scope
// preconditions, capture indexes and task closures. It returns the records
// with each task closure's outcome filled in.
func (s *Store) planManagementTasks(ctx context.Context, ownerID, computerID string, tasks []types.ManagementTask, records []types.CommitmentRecord, opened map[string]bool, now time.Time) ([]types.CommitmentRecord, []objectgraph.Object, []objectgraph.ObjectCondition, error) {
	p := &taskPlan{s: s, ctx: ctx, ownerID: ownerID, computer: computerID, now: now,
		rows: map[string]*inboxCached[types.ManagementTask]{}, indexes: map[string]*inboxCached[types.CaptureDisposition]{},
		heads: map[string]*inboxCached[types.ConversationHead]{}, dirtyRows: map[string]bool{}, dirtyIx: map[string]bool{}, newTasks: map[string]bool{}}
	management := "management:" + ownerID
	newRows := map[string]types.ManagementTask{}
	for i, task := range tasks {
		if task.OwnerID != ownerID || task.ComputerID != computerID {
			return nil, nil, nil, inboxRefused("management task %s is scoped to another computer", task.TaskID)
		}
		if task.State != types.ManagementTaskQueued || task.RunID != "" || task.ClosingRecordID != "" {
			return nil, nil, nil, inboxRefused("management task %s opens queued, with no run", task.TaskID)
		}
		task.ConversationID = types.InboxConversationID(task.ParentInbox, task.TaskInbox)
		if err := task.Validate(); err != nil {
			return nil, nil, nil, inboxRefused("%v", err)
		}
		if _, dup := newRows[task.TaskInbox]; dup {
			return nil, nil, nil, inboxRefused("management task %s appears twice", task.TaskID)
		}
		// Rows of one delegation keep its order: the oldest queued task
		// starts first.
		task.CreatedAt, task.UpdatedAt = now.Add(time.Duration(i)*time.Microsecond), now
		newRows[task.TaskInbox] = task
		p.newTasks[task.TaskID] = true
	}
	out := records
	copied := false
	opening := map[string]bool{}
	for i, rec := range records {
		sender, inbox := strings.TrimSpace(rec.SenderInbox), strings.TrimSpace(rec.Inbox)
		// FM6: a task scope's records name their task; the host derives it.
		if id, ok := types.ManagementTaskIDOfInbox(rec.Provenance.AgentID); ok && rec.Provenance.ActingTask != id {
			return nil, nil, nil, inboxRefused("record %s was authored by task %s and does not name it as acting_task", rec.RecordID, id)
		}
		if task := strings.TrimSpace(rec.Provenance.ActingTask); task != "" {
			if err := p.checkTaskScope(rec, management); err != nil {
				return nil, nil, nil, err
			}
		}
		if taskID, ok := types.ManagementTaskIDOfInbox(sender); ok {
			// The one record a task inbox sends is its final Report, which
			// the host mints.
			if rec.EmitterKind != types.CommitmentEmitterSystem || !rec.Final || rec.Report == nil || inbox != management {
				return nil, nil, nil, inboxRefused("record %s: a task inbox sends only the host's final Report to %s", rec.RecordID, management)
			}
			closed, err := p.closeTask(taskID, rec)
			if err != nil {
				return nil, nil, nil, err
			}
			if !copied {
				out = append([]types.CommitmentRecord(nil), records...)
				copied = true
			}
			out[i] = closed
			continue
		}
		if _, ok := types.ManagementTaskIDOfInbox(inbox); ok {
			row, found := newRows[inbox]
			if !found || row.OpeningRecordID != rec.RecordID || opening[inbox] {
				return nil, nil, nil, inboxRefused("record %s: a task inbox takes only its opening Cast, committed with its row by choir.Delegate", rec.RecordID)
			}
			opening[inbox] = true
			if err := p.openTask(row, rec, management); err != nil {
				return nil, nil, nil, err
			}
			continue
		}
		if sender != management {
			continue
		}
		kind, captures := recordDisposition(rec, opened)
		if err := p.dispose(rec, kind, captures); err != nil {
			return nil, nil, nil, err
		}
	}
	for inbox, row := range newRows {
		if !opening[inbox] {
			return nil, nil, nil, inboxRefused("management task %s has no opening Cast in its commit", row.TaskID)
		}
	}
	objects, conditions, err := p.objects(out)
	if err != nil {
		return nil, nil, nil, err
	}
	return out, objects, conditions, nil
}

// checkTaskScope is the store precondition on every record a task scope
// commits (R2-4): sent as management, to a story or a source, while the
// task's round is open and by the task's own run.
func (p *taskPlan) checkTaskScope(rec types.CommitmentRecord, management string) error {
	task := strings.TrimSpace(rec.Provenance.ActingTask)
	if strings.TrimSpace(rec.SenderInbox) != management {
		return inboxRefused("record %s: task %s sends as %s", rec.RecordID, task, management)
	}
	if !strings.HasPrefix(rec.Inbox, "texture:") && !strings.HasPrefix(rec.Inbox, "source:") {
		return inboxRefused("record %s: task %s sends only to a story or a source, not %s", rec.RecordID, task, rec.Inbox)
	}
	row, err := p.row(task)
	if err != nil {
		return err
	}
	if !row.exists {
		return inboxRefused("record %s names task %s, which does not exist", rec.RecordID, task)
	}
	if row.value.State != types.ManagementTaskRunning {
		return inboxRefused("record %s: task %s is %s; nothing it stages commits", rec.RecordID, task, row.value.State)
	}
	if run := strings.TrimSpace(row.value.RunID); run != "" && strings.TrimSpace(rec.Provenance.ActingRunID) != run {
		return inboxRefused("record %s: run %q is not task %s's run", rec.RecordID, rec.Provenance.ActingRunID, task)
	}
	open, err := p.roundOpen(task, row.value.Round)
	if err != nil {
		return err
	}
	if !open {
		return inboxRefused("record %s: task %s's round is closed, so its lease is free; nothing it stages commits", rec.RecordID, task)
	}
	return nil
}

// openTask leases the captures a delegation Cast names to its new task.
func (p *taskPlan) openTask(row types.ManagementTask, cast types.CommitmentRecord, management string) error {
	if !cast.IsCast() || cast.Cast == nil || strings.TrimSpace(cast.SenderInbox) != management || row.ParentInbox != management {
		return inboxRefused("record %s: a task opens with a Cast from %s", cast.RecordID, management)
	}
	named := map[string]bool{}
	for _, c := range cast.Cast.Captures {
		named[strings.TrimSpace(c)] = true
	}
	if len(named) != len(row.LeasedCaptures) {
		return inboxRefused("task %s leases %d captures; its Cast names %d", row.TaskID, len(row.LeasedCaptures), len(named))
	}
	for _, c := range row.LeasedCaptures {
		if !named[c] {
			return inboxRefused("task %s leases %s, which its Cast does not name", row.TaskID, c)
		}
	}
	existing, err := p.row(row.TaskID)
	if err != nil {
		return err
	}
	if existing.exists {
		return inboxRefused("management task %s already exists", row.TaskID)
	}
	h, err := p.head(row.ConversationID)
	if err != nil {
		return err
	}
	if h.exists && h.value.Open {
		return inboxRefused("task %s's conversation already has an open round", row.TaskID)
	}
	row.Round = h.value.Round + 1
	for _, c := range row.LeasedCaptures {
		ix, err := p.index(c)
		if err != nil {
			return err
		}
		v := ix.value
		switch {
		case v.Fate == types.CaptureFateClosedAfterCrashes:
			return inboxRefused("capture %s was closed after two crashes interrupted its tasks; it is not delegated again", c)
		case v.Kind != "":
			return inboxRefused("capture %s is already disposed (%s by record %s); nothing is left to delegate", c, v.Kind, v.RecordID)
		case v.LeaseTask != "" && v.LeaseTask != row.TaskID:
			open, err := p.roundOpen(v.LeaseTask, v.LeaseRound)
			if err != nil {
				return err
			}
			if open {
				return inboxRefused("capture %s is leased to task %s until its final Report", c, v.LeaseTask)
			}
		}
		ix.value.LeaseTask, ix.value.LeaseRound = row.TaskID, row.Round
		if ix.value.FirstLeased == nil {
			at := p.now
			ix.value.FirstLeased = &at
		}
		p.markIndex(c)
	}
	p.rows[row.TaskID] = &inboxCached[types.ManagementTask]{key: row.TaskID, value: row}
	p.markRow(row.TaskID)
	return nil
}

// dispose indexes one management record's disposition of its captures.
func (p *taskPlan) dispose(rec types.CommitmentRecord, kind string, captures []string) error {
	acting := strings.TrimSpace(rec.Provenance.ActingTask)
	seen := map[string]bool{}
	for _, c := range captures {
		if c = strings.TrimSpace(c); c == "" || seen[c] {
			continue
		}
		seen[c] = true
		ix, err := p.index(c)
		if err != nil {
			return err
		}
		v := ix.value
		if acting != "" {
			if v.LeaseTask != acting {
				return inboxRefused("record %s: capture %s is not leased to task %s", rec.RecordID, c, acting)
			}
			if v.Kind != "" {
				return inboxRefused("record %s: capture %s is already disposed (%s by record %s); a task disposes of a capture once", rec.RecordID, c, v.Kind, v.RecordID)
			}
		} else {
			if v.LeaseTask != "" {
				open, err := p.roundOpen(v.LeaseTask, v.LeaseRound)
				if err != nil {
					return err
				}
				if open {
					return inboxRefused("record %s: capture %s is leased to task %s until its final Report; cancel the task with choir.CancelTask first", rec.RecordID, c, v.LeaseTask)
				}
			}
			if v.Kind != "" {
				// The parent's re-disposition is the effective one and the
				// replaced one is kept as an override (§6.4).
				ix.value.Overrides = append(ix.value.Overrides, types.CaptureDispositionOverride{Kind: v.Kind, RecordID: v.RecordID,
					ActingTask: v.ActingTask, ReplacedBy: rec.RecordID, At: p.now})
			}
		}
		at := p.now
		ix.value.Kind, ix.value.RecordID, ix.value.Inbox, ix.value.ActingTask, ix.value.DisposedAt = kind, rec.RecordID, rec.Inbox, acting, &at
		p.markIndex(c)
	}
	return nil
}

// closeTask fills a task's final Report from the index and closes its row.
func (p *taskPlan) closeTask(taskID string, rec types.CommitmentRecord) (types.CommitmentRecord, error) {
	r, err := p.row(taskID)
	if err != nil {
		return rec, err
	}
	if !r.exists {
		return rec, inboxRefused("record %s closes task %s, which does not exist", rec.RecordID, taskID)
	}
	row := r.value
	if row.State == types.ManagementTaskClosed {
		return rec, inboxRefused("record %s: task %s is already closed", rec.RecordID, taskID)
	}
	stop := rec.Report.StopReason
	outcome := types.CommitmentTaskOutcome{}
	if rec.TaskOutcome != nil {
		outcome = *rec.TaskOutcome
	}
	outcome.TaskID, outcome.Kind = taskID, row.Kind
	if outcome.Reason == "" {
		outcome.Reason = stop
	}
	if outcome.RunID == "" {
		outcome.RunID = row.RunID
	}
	outcome.Dispositions, outcome.Residuals = nil, nil
	for _, c := range row.LeasedCaptures {
		ix, err := p.index(c)
		if err != nil {
			return rec, err
		}
		if ix.value.Kind != "" {
			outcome.Dispositions = append(outcome.Dispositions, types.TaskCaptureDisposition{CaptureID: c, Kind: ix.value.Kind, RecordID: ix.value.RecordID, Inbox: ix.value.Inbox})
			continue
		}
		reason := outcome.Note
		if reason == "" {
			reason = outcome.Reason
		}
		outcome.Residuals = append(outcome.Residuals, types.TaskResidual{CaptureID: c, Reason: reason})
		if stop == types.ClosureReasonInterruptedByRestart {
			// R2-17: a capture whose task round two crash boots closed is
			// closed with that cause.
			ix.value.CrashCount++
			if ix.value.CrashCount >= 2 {
				ix.value.Fate = types.CaptureFateClosedAfterCrashes
			}
			p.markIndex(c)
		}
	}
	outcome.Clean = len(outcome.Residuals) == 0 && outcome.Error == "" && outcome.Reason == types.TaskClosureDone
	report := *rec.Report
	summary := fmt.Sprintf("Task %s ended (%s): %d of %d captures disposed.", taskID, outcome.Reason, len(outcome.Dispositions), len(row.LeasedCaptures))
	if len(outcome.Residuals) > 0 {
		var left []string
		for _, residual := range outcome.Residuals {
			left = append(left, residual.CaptureID)
		}
		summary += " Left for you: " + strings.Join(left, ", ") + "."
	}
	if outcome.Error != "" {
		summary += " Error: " + outcome.Error
	}
	report.Claims = []types.ReportClaim{{Text: summary}}
	rec.Report, rec.TaskOutcome = &report, &outcome

	at := p.now
	row.State, row.ClosureReason, row.ClosingRecordID, row.ClosedAt, row.UpdatedAt = types.ManagementTaskClosed, stop, rec.RecordID, &at, p.now
	r.value = row
	p.markRow(taskID)
	if stop != types.ClosureReasonInterruptedByRestart {
		if err := p.startNextQueued(taskID, rec.RecordID); err != nil {
			return rec, err
		}
	}
	return rec, nil
}

// startNextQueued mints the durable start of the oldest queued task (R2-6):
// an inbox_record wake to its inbox, in the commit that freed a slot. A
// crash boot's closures start nothing.
func (p *taskPlan) startNextQueued(closedTask, closingRecordID string) error {
	queued, err := p.s.ListManagementTasks(p.ctx, p.ownerID, p.computer, types.ManagementTaskQueued)
	if err != nil {
		return err
	}
	for _, next := range queued {
		if next.TaskID == closedTask || p.dirtyRows[next.TaskID] {
			continue
		}
		content, err := json.Marshal(map[string]string{"inbox": next.TaskInbox, "start_after": closingRecordID})
		if err != nil {
			return err
		}
		source := objectgraph.Object{OwnerID: p.ownerID, ComputerID: p.computer, CreatedAt: p.now, UpdatedAt: p.now}
		key := "task-start:" + next.TaskID + ":" + closingRecordID
		_, wake, err := actorWakeOutbox(source, key, next.TaskInbox, "", "", InboxRecordWakeKind, string(content), time.Time{}, key)
		if err != nil {
			return err
		}
		p.wakes = append(p.wakes, wake)
		return nil
	}
	return nil
}

// objects renders the plan's writes and the conditions on what it read.
func (p *taskPlan) objects(records []types.CommitmentRecord) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	for _, id := range p.rowOrder {
		r := p.rows[id]
		if err := r.value.Validate(); err != nil {
			return nil, nil, inboxRefused("%v", err)
		}
		obj, condition, err := r.write(ogKindManagementTask, p.ownerID, p.computer, map[string]any{
			"task_id": r.value.TaskID, "state": string(r.value.State), "run_id": r.value.RunID, "owner_id": p.ownerID,
		}, p.now)
		if err != nil {
			return nil, nil, err
		}
		objects, conditions = append(objects, obj), append(conditions, condition)
	}
	for _, c := range p.ixOrder {
		ix := p.indexes[c]
		obj, condition, err := ix.write(ogKindCaptureDisposition, p.ownerID, p.computer, map[string]any{
			"capture_id": c, "lease_task": ix.value.LeaseTask, "kind": ix.value.Kind,
		}, p.now)
		if err != nil {
			return nil, nil, err
		}
		objects, conditions = append(objects, obj), append(conditions, condition)
	}
	for _, wake := range p.wakes {
		objects = append(objects, wake)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: wake.CanonicalID})
	}
	// The task rounds this plan read hold for the commit: a head the
	// command does not itself write is conditioned on what was read.
	written := map[string]bool{}
	for _, rec := range records {
		written[types.InboxConversationID(rec.Inbox, rec.SenderInbox)] = true
	}
	for id, h := range p.heads {
		if written[id] {
			continue
		}
		canonicalID, err := lifecycleCanonicalID(ogKindConversationHead, p.ownerID, p.computer, id)
		if err != nil {
			return nil, nil, err
		}
		condition := objectgraph.ObjectCondition{CanonicalID: canonicalID}
		if h.exists {
			condition.Exists, condition.ExpectedContentHash = true, h.prior.ContentHash
		}
		conditions = append(conditions, condition)
	}
	return objects, conditions, nil
}

// planStoryLibrary advances the story-library head for each story this
// command opens, and refuses the opens when the committing run's last read
// of the library (seen) is behind the head (R2-5). When seen was checked,
// it returns the head after the opens: the run has seen every open up to
// it, its own included, so its next open is not refused by its own (S2b,
// FM28). Otherwise it returns 0.
func (s *Store) planStoryLibrary(ctx context.Context, ownerID, computerID string, seen *int64, records []types.CommitmentRecord, opened map[string]bool, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, int64, error) {
	var opens []types.CommitmentRecord
	for _, rec := range records {
		if opened[strings.TrimSpace(rec.RecordID)] {
			opens = append(opens, rec)
		}
	}
	if len(opens) == 0 {
		return nil, nil, 0, nil
	}
	head, err := loadInboxCached[types.StoryLibraryHead](ctx, s, ogKindStoryLibraryHead, ownerID, computerID, storyLibraryKey)
	if err != nil {
		return nil, nil, 0, err
	}
	if seen != nil && *seen < head.value.Head {
		var since []string
		for i := len(head.value.Recent) - 1; i >= 0 && len(since) < storyLibraryRefusalList; i-- {
			e := head.value.Recent[i]
			if e.Seq <= *seen {
				break
			}
			line := fmt.Sprintf("%s %q", e.Inbox, e.Headline)
			if len(e.Captures) > 0 {
				line += " (captures " + strings.Join(e.Captures, ", ") + ")"
			}
			since = append(since, line)
		}
		more := ""
		if missed := head.value.Head - *seen; missed > int64(len(since)) {
			more = fmt.Sprintf(" and %d more; read choir.Stories() for them", missed-int64(len(since)))
		}
		return nil, nil, 0, fmt.Errorf("%w: %w", ErrStoryLibraryMoved, inboxRefused(
			"the story library moved since this run last read choir.Stories() (head %d, now %d); stories opened since, newest first: %s%s. Feed the story that covers this event, or, if none does, cast texture:new again",
			*seen, head.value.Head, strings.Join(since, "; "), more))
	}
	for _, rec := range opens {
		head.value.Head++
		entry := types.StoryOpenEntry{Seq: head.value.Head, Inbox: rec.Inbox}
		if rec.Directive != nil {
			entry.Headline = rec.Directive.Objective
		}
		if rec.Cast != nil {
			entry.Captures = append([]string(nil), rec.Cast.Captures...)
		}
		head.value.Recent = append(head.value.Recent, entry)
	}
	if n := len(head.value.Recent); n > storyLibraryRecent {
		head.value.Recent = append([]types.StoryOpenEntry(nil), head.value.Recent[n-storyLibraryRecent:]...)
	}
	obj, condition, err := head.write(ogKindStoryLibraryHead, ownerID, computerID, map[string]any{"head": head.value.Head}, now)
	if err != nil {
		return nil, nil, 0, err
	}
	seenAfter := int64(0)
	if seen != nil {
		seenAfter = head.value.Head
	}
	return []objectgraph.Object{obj}, []objectgraph.ObjectCondition{condition}, seenAfter, nil
}

// ErrStoryLibraryMoved marks a texture:new refused by the library head
// (R2-5). It wraps ErrInboxRefused.
var ErrStoryLibraryMoved = errors.New("story library moved")

// StoryLibraryHeadOf returns the computer's story-library head (0 before
// the first open).
func (s *Store) StoryLibraryHeadOf(ctx context.Context, ownerID, computerID string) (int64, error) {
	head, err := loadInboxCached[types.StoryLibraryHead](ctx, s, ogKindStoryLibraryHead, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), storyLibraryKey)
	if err != nil {
		return 0, err
	}
	return head.value.Head, nil
}

// GetManagementTask returns one task row.
func (s *Store) GetManagementTask(ctx context.Context, ownerID, computerID, taskID string) (types.ManagementTask, error) {
	row, err := loadInboxCached[types.ManagementTask](ctx, s, ogKindManagementTask, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(taskID))
	if err != nil {
		return types.ManagementTask{}, err
	}
	if !row.exists {
		return types.ManagementTask{}, ErrNotFound
	}
	return row.value, nil
}

// ListManagementTasks returns a computer's task rows in one state (all
// states for ""), oldest first.
func (s *Store) ListManagementTasks(ctx context.Context, ownerID, computerID string, state types.ManagementTaskState) ([]types.ManagementTask, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	field, value := "owner_id", ownerID
	if state != "" {
		field, value = "state", string(state)
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindManagementTask, field, value)
	if err != nil {
		return nil, err
	}
	var out []types.ManagementTask
	for _, obj := range objs {
		if obj.Tombstone || obj.OwnerID != ownerID || obj.ComputerID != computerID {
			continue
		}
		row, err := decodeLifecycleObject[types.ManagementTask](obj)
		if err != nil {
			return nil, fmt.Errorf("management task decode %s: %w", obj.CanonicalID, err)
		}
		if state == "" || row.State == state {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].TaskID < out[j].TaskID
	})
	return out, nil
}

// CaptureDispositionOf returns a capture's disposition index, if any.
func (s *Store) CaptureDispositionOf(ctx context.Context, ownerID, computerID, captureID string) (types.CaptureDisposition, bool, error) {
	ix, err := loadInboxCached[types.CaptureDisposition](ctx, s, ogKindCaptureDisposition, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(captureID))
	if err != nil {
		return types.CaptureDisposition{}, false, err
	}
	return ix.value, ix.exists, nil
}

// ClaimManagementTask takes a slot for a queued task (R2-6): the row turns
// running, with its start and deadline, while its round is still open.
// The run is bound with BindManagementTaskRun once it exists.
func (s *Store) ClaimManagementTask(ctx context.Context, ownerID, computerID, taskID string, now time.Time) (types.ManagementTask, error) {
	return s.updateManagementTask(ctx, ownerID, computerID, taskID, now, func(row *types.ManagementTask, roundOpen bool) error {
		if row.State != types.ManagementTaskQueued {
			return inboxRefused("task %s is %s, not queued", taskID, row.State)
		}
		if !roundOpen {
			return inboxRefused("task %s's round is closed", taskID)
		}
		started := now.UTC()
		deadline := started.Add(time.Duration(row.DeadlineSeconds) * time.Second)
		row.State, row.StartedAt, row.DeadlineAt = types.ManagementTaskRunning, &started, &deadline
		return nil
	})
}

// BindManagementTaskRun records the run of a claimed task.
func (s *Store) BindManagementTaskRun(ctx context.Context, ownerID, computerID, taskID, runID string) (types.ManagementTask, error) {
	return s.updateManagementTask(ctx, ownerID, computerID, taskID, time.Now().UTC(), func(row *types.ManagementTask, roundOpen bool) error {
		if row.State != types.ManagementTaskRunning || (row.RunID != "" && row.RunID != runID) || !roundOpen {
			return inboxRefused("task %s is %s with run %q; run %s is not bound", taskID, row.State, row.RunID, runID)
		}
		row.RunID = strings.TrimSpace(runID)
		return nil
	})
}

func (s *Store) updateManagementTask(ctx context.Context, ownerID, computerID, taskID string, now time.Time, update func(*types.ManagementTask, bool) error) (types.ManagementTask, error) {
	ownerID, computerID, taskID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(taskID)
	if s.ogStore == nil {
		return types.ManagementTask{}, fmt.Errorf("management task: object graph not initialized")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	row, err := loadInboxCached[types.ManagementTask](ctx, s, ogKindManagementTask, ownerID, computerID, taskID)
	if err != nil {
		return types.ManagementTask{}, err
	}
	if !row.exists {
		return types.ManagementTask{}, ErrNotFound
	}
	head, err := loadInboxCached[types.ConversationHead](ctx, s, ogKindConversationHead, ownerID, computerID, taskConversation(ownerID, taskID))
	if err != nil {
		return types.ManagementTask{}, err
	}
	open := head.exists && head.value.Open && head.value.Round == row.value.Round
	if err := update(&row.value, open); err != nil {
		return types.ManagementTask{}, err
	}
	row.value.UpdatedAt = now.UTC()
	obj, condition, err := row.write(ogKindManagementTask, ownerID, computerID, map[string]any{
		"task_id": row.value.TaskID, "state": string(row.value.State), "run_id": row.value.RunID, "owner_id": ownerID,
	}, now.UTC())
	if err != nil {
		return types.ManagementTask{}, err
	}
	headCondition := objectgraph.ObjectCondition{CanonicalID: ""}
	if head.exists {
		headCondition = objectgraph.ObjectCondition{CanonicalID: head.prior.CanonicalID, Exists: true, ExpectedContentHash: head.prior.ContentHash}
	}
	conditions := []objectgraph.ObjectCondition{condition}
	if headCondition.CanonicalID != "" {
		conditions = append(conditions, headCondition)
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return types.ManagementTask{}, ErrConcurrentStateChange
		}
		return types.ManagementTask{}, err
	}
	return row.value, nil
}
