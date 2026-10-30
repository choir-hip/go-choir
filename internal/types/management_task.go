package types

import (
	"fmt"
	"strings"
	"time"
)

// Management tasks (management delegation S2a,
// docs/archive/reports/management-delegation-design-2026-10-14.md, round 2). A task
// is a bounded sub-RLM of the management desk: its own run, session and
// inbox (management-task:<id>), opened by management's choir.Delegate with
// a Cast into that inbox, holding a lease on the captures the round's
// Casts name while the round is open, and closed by one final Report the
// host mints from the task inbox to management:<owner>.

// ManagementTaskDesk is the desk part of a task's inbox. It must never be
// spelled management:<anything>: WakeRecordsInbox wakes only the persistent
// management inbox under that desk.
const ManagementTaskDesk = "management-task"

// ManagementTaskInbox is the reader inbox of one task.
func ManagementTaskInbox(taskID string) string {
	return ManagementTaskDesk + ":" + strings.TrimSpace(taskID)
}

// ManagementTaskIDOfInbox returns the task id an inbox names, if it is a
// task inbox.
func ManagementTaskIDOfInbox(inbox string) (string, bool) {
	desk, id, ok := strings.Cut(strings.TrimSpace(inbox), ":")
	if !ok || desk != ManagementTaskDesk || strings.TrimSpace(id) == "" {
		return "", false
	}
	return strings.TrimSpace(id), true
}

// ManagementTaskKind is what a task is given (design §3.2). S2a opens
// intake tasks only.
type ManagementTaskKind string

const ManagementTaskIntake ManagementTaskKind = "intake"

// ManagementTaskState is a task row's state. A queued task is an open
// round with no run; a running task has a run; a closed task's round is
// closed.
type ManagementTaskState string

const (
	ManagementTaskQueued  ManagementTaskState = "queued"
	ManagementTaskRunning ManagementTaskState = "running"
	ManagementTaskClosed  ManagementTaskState = "closed"
)

// ManagementTask is the task row (R2-7): the assignment row's patterns, its
// own type. It has no trajectory, capsule or verifier fields. The lease is
// the open round (R2-4, §4.3): LeasedCaptures are written in the commit of
// the round's Casts and never edited apart from them.
type ManagementTask struct {
	TaskID     string             `json:"task_id"`
	OwnerID    string             `json:"owner_id"`
	ComputerID string             `json:"computer_id"`
	Kind       ManagementTaskKind `json:"kind"`
	Objective  string             `json:"objective"`
	// ParentInbox is management:<owner>, the round's caster; ParentRunID
	// is the run that delegated, provenance only (I3: control never hangs
	// on the parent run).
	ParentInbox string `json:"parent_inbox"`
	ParentRunID string `json:"parent_run_id,omitempty"`
	TaskInbox   string `json:"task_inbox"`
	// ConversationID and Round name the task round.
	ConversationID  string `json:"conversation_id"`
	Round           int    `json:"round"`
	OpeningRecordID string `json:"opening_record_id"`
	// DelegatedRecordIDs are the captures records the task was given.
	DelegatedRecordIDs []string `json:"delegated_record_ids"`
	LeasedCaptures     []string `json:"leased_captures"`
	// DeadlineSeconds bounds the task's run from its start.
	DeadlineSeconds int                 `json:"deadline_seconds"`
	State           ManagementTaskState `json:"state"`
	RunID           string              `json:"run_id,omitempty"`
	StartedAt       *time.Time          `json:"started_at,omitempty"`
	DeadlineAt      *time.Time          `json:"deadline_at,omitempty"`
	ClosedAt        *time.Time          `json:"closed_at,omitempty"`
	ClosureReason   string              `json:"closure_reason,omitempty"`
	ClosingRecordID string              `json:"closing_record_id,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// Validate checks a task row's shape.
func (t ManagementTask) Validate() error {
	switch {
	case strings.TrimSpace(t.TaskID) == "", strings.TrimSpace(t.OwnerID) == "", strings.TrimSpace(t.ComputerID) == "":
		return fmt.Errorf("management task needs a task, owner and computer id")
	case t.Kind != ManagementTaskIntake:
		return fmt.Errorf("management task %s: kind %q is not open (S2a opens intake tasks)", t.TaskID, t.Kind)
	case t.ParentInbox != "management:"+t.OwnerID:
		return fmt.Errorf("management task %s: parent inbox %q is not management:%s", t.TaskID, t.ParentInbox, t.OwnerID)
	case t.TaskInbox != ManagementTaskInbox(t.TaskID):
		return fmt.Errorf("management task %s: inbox %q is not %s", t.TaskID, t.TaskInbox, ManagementTaskInbox(t.TaskID))
	case t.ConversationID != InboxConversationID(t.ParentInbox, t.TaskInbox):
		return fmt.Errorf("management task %s: conversation %q does not derive from its inboxes", t.TaskID, t.ConversationID)
	case strings.TrimSpace(t.OpeningRecordID) == "":
		return fmt.Errorf("management task %s needs its opening record", t.TaskID)
	case len(t.LeasedCaptures) == 0 || len(t.DelegatedRecordIDs) == 0:
		return fmt.Errorf("management task %s leases no captures", t.TaskID)
	case t.DeadlineSeconds <= 0:
		return fmt.Errorf("management task %s needs a deadline", t.TaskID)
	}
	switch t.State {
	case ManagementTaskQueued, ManagementTaskRunning, ManagementTaskClosed:
	default:
		return fmt.Errorf("management task %s: state %q is unknown", t.TaskID, t.State)
	}
	seen := map[string]bool{}
	for _, id := range t.LeasedCaptures {
		if strings.TrimSpace(id) == "" || seen[id] {
			return fmt.Errorf("management task %s: leased captures must be distinct ids", t.TaskID)
		}
		seen[id] = true
	}
	return nil
}

// Closure reasons a task's final Report carries (StopReason), beside
// run_<state> and ClosureReasonInterruptedByRestart.
const (
	TaskClosureDone      = "done"
	TaskClosureDeadline  = "deadline"
	TaskClosureCancelled = "cancelled"
	// TaskClosureRunPassivated closes a task whose run a planned restart
	// parked: tasks are not resumed (R2-12).
	TaskClosureRunPassivated = "run_passivated"
)

// ManagementTaskSystemAgentID is the provenance of the task's final Report.
const ManagementTaskSystemAgentID = "system:management-task"

// CommitmentTaskOutcome is the body of a task's final Report (R2-8). The
// store computes Dispositions, Residuals and Clean from the durable
// disposition index under the commit lock; the host supplies the reason,
// the model's note and the error. Clean means every leased capture has a
// disposition, the task ended with Done, and no error: only then does the
// Report leave the parent asleep (R2-3).
type CommitmentTaskOutcome struct {
	TaskID       string                   `json:"task_id"`
	Kind         ManagementTaskKind       `json:"kind"`
	RunID        string                   `json:"run_id,omitempty"`
	Reason       string                   `json:"reason"`
	Note         string                   `json:"note,omitempty"`
	Error        string                   `json:"error,omitempty"`
	Dispositions []TaskCaptureDisposition `json:"dispositions,omitempty"`
	Residuals    []TaskResidual           `json:"residuals,omitempty"`
	Clean        bool                     `json:"clean"`
	InputTokens  int64                    `json:"input_tokens,omitempty"`
	OutputTokens int64                    `json:"output_tokens,omitempty"`
}

// NewTaskClosingReport builds the final Report the host mints for a task
// (R2-8): from the task inbox to management, final, closing the task round,
// with the reason, the model's note and the error. Its id is the round's
// one task closure, so a second closure of the round is refused. The store
// fills the outcome's dispositions, residuals and clean flag at commit.
func NewTaskClosingReport(task ManagementTask, reason, note, errText, runID, committedAt string) CommitmentRecord {
	conversation := InboxConversationID(task.ParentInbox, task.TaskInbox)
	round := task.Round
	if round == 0 {
		round = 1
	}
	return CommitmentRecord{
		SchemaID:    CommitmentRecordSchemaV1,
		RecordID:    RoundClosureRecordID(conversation, round, "task"),
		Kind:        CommitmentKindReport,
		Discrepancy: DiscrepancyUnresolved,
		Provenance:  CommitmentProvenance{AgentID: ManagementTaskSystemAgentID, CommittedAt: committedAt},
		Observation: CommitmentObservation{Excerpt: reason, ObservedAt: committedAt},
		EmitterKind: CommitmentEmitterSystem,
		Inbox:       strings.TrimSpace(task.ParentInbox), SenderInbox: strings.TrimSpace(task.TaskInbox),
		ConversationID: conversation, Round: round, Final: true,
		Report: &CommitmentReport{StopReason: reason},
		TaskOutcome: &CommitmentTaskOutcome{TaskID: task.TaskID, Kind: task.Kind, RunID: strings.TrimSpace(runID),
			Reason: reason, Note: strings.TrimSpace(note), Error: strings.TrimSpace(errText)},
	}
}

// TaskCaptureDisposition is what one leased capture became.
type TaskCaptureDisposition struct {
	CaptureID string `json:"capture_id"`
	Kind      string `json:"kind"`
	RecordID  string `json:"record_id"`
	Inbox     string `json:"inbox,omitempty"`
}

// TaskResidual is a leased capture the task left undisposed.
type TaskResidual struct {
	CaptureID string `json:"capture_id"`
	Reason    string `json:"reason,omitempty"`
}

// Capture disposition kinds in the index: an opening Cast's captures are
// opened, a Cast or feed Note into a story's inbox feeds, a decline Note
// declines.
const DispositionOpen = "open"

// CaptureFateClosedAfterCrashes marks a capture whose task round two crash
// boots closed: it is never delegated again (R2-17).
const CaptureFateClosedAfterCrashes = "closed_after_two_crashes"

// CaptureDisposition is the per-capture index object (R2-4, §6.4), keyed
// by capture id on the computer. It indexes the record that disposed the
// capture (the record stays the content; both commit in one batch) and the
// last task that leased it, whose open round is the lease. A second
// disposition by a task is refused under the commit lock; the parent's
// re-disposition is recorded as an override.
type CaptureDisposition struct {
	CaptureID  string `json:"capture_id"`
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	LeaseTask  string `json:"lease_task,omitempty"`
	LeaseRound int    `json:"lease_round,omitempty"`
	Kind       string `json:"kind,omitempty"`
	RecordID   string `json:"record_id,omitempty"`
	Inbox      string `json:"inbox,omitempty"`
	// ActingTask is the task that disposed it, or "" for the parent.
	ActingTask  string                       `json:"acting_task,omitempty"`
	DisposedAt  *time.Time                   `json:"disposed_at,omitempty"`
	Overrides   []CaptureDispositionOverride `json:"overrides,omitempty"`
	CrashCount  int                          `json:"crash_count,omitempty"`
	Fate        string                       `json:"fate,omitempty"`
	FirstLeased *time.Time                   `json:"first_leased,omitempty"`
}

// CaptureDispositionOverride is a disposition the parent replaced.
type CaptureDispositionOverride struct {
	Kind       string    `json:"kind"`
	RecordID   string    `json:"record_id"`
	ActingTask string    `json:"acting_task,omitempty"`
	ReplacedBy string    `json:"replaced_by"`
	At         time.Time `json:"at"`
}

// StoryLibraryHead is the computer's count of story opens (R2-5), advanced
// in the commit of each Cast to texture:new, with the latest opens for a
// refusal to name.
type StoryLibraryHead struct {
	Head   int64            `json:"head"`
	Recent []StoryOpenEntry `json:"recent,omitempty"`
}

// StoryOpenEntry is one story open the library head recorded.
type StoryOpenEntry struct {
	Seq      int64    `json:"seq"`
	Inbox    string   `json:"inbox"`
	Headline string   `json:"headline"`
	Captures []string `json:"captures,omitempty"`
}
