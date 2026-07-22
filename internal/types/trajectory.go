package types

import "time"

// TrajectoryStatus is the lifecycle state of a trajectory. Trajectories
// settle or are cancelled; they are never "completed" by an agent — agents
// passivate, trajectories settle (docs/glossary.md: trajectory, settlement).
type TrajectoryStatus string

const (
	TrajectoryLive      TrajectoryStatus = "live"
	TrajectorySettled   TrajectoryStatus = "settled"
	TrajectoryCancelled TrajectoryStatus = "cancelled"
)

// TrajectoryKind classifies what a trajectory is about. v1 keeps the
// smallest honest set derived from the spawn surfaces that mint records.
type TrajectoryKind string

const (
	// TrajectoryKindDocument covers prompt-bar/conductor → texture → worker
	// chains and email appagent flows anchored on a document.
	TrajectoryKindDocument TrajectoryKind = "document"
	// TrajectoryKindPublication covers processor/wire publication cycles.
	TrajectoryKindPublication TrajectoryKind = "publication"
	// TrajectoryKindTask is the default for spawns with no more specific
	// subject.
	TrajectoryKindTask TrajectoryKind = "task"
)

// SettlementRule is the trajectory's settlement condition stored as data,
// not as Go control flow. Evaluation is a pure function over the rule, the
// trajectory's open work items, and its subject refs; nothing in v1 acts on
// the verdict (M5 wires reconciliation to it).
type SettlementRule struct {
	// RequireNoOpenWorkItems requires zero open work items on the
	// trajectory before it may settle.
	RequireNoOpenWorkItems bool `json:"require_no_open_work_items"`
	// RequiredSubjectRefs lists subject-ref keys that must be present and
	// non-empty before the trajectory may settle (e.g. "publish_ref").
	RequiredSubjectRefs []string `json:"required_subject_refs,omitempty"`
	// Version names the closed reducer predicate vocabulary.
	Version string `json:"version,omitempty"`
}

// TrajectoryRecord is the durable causality object: the unit that spans
// prompt-bar → conductor → texture → workers → revisions (or a publication
// cycle), replacing parent/child run trees as the control model. The ID is
// the same trajectory_id the runtime already threads through run metadata,
// events, channel messages, and worker updates, so existing surfaces join
// against this record without migration.
type TrajectoryRecord struct {
	TrajectoryID            string            `json:"trajectory_id"`
	OwnerID                 string            `json:"owner_id"`
	ComputerID              string            `json:"computer_id"`
	Kind                    TrajectoryKind    `json:"kind"`
	SubjectRefs             map[string]string `json:"subject_refs,omitempty"`
	Status                  TrajectoryStatus  `json:"status"`
	SettlementRule          SettlementRule    `json:"settlement_rule"`
	LifecycleVersion        int64             `json:"lifecycle_version,omitempty"`
	ReducerSeq              int64             `json:"reducer_seq,omitempty"`
	TerminalArtifactHeadRef string            `json:"terminal_artifact_head_ref,omitempty"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
	SettledAt               *time.Time        `json:"settled_at,omitempty"`
	CancelledAt             *time.Time        `json:"cancelled_at,omitempty"`
}

// WorkItemStatus is the lifecycle state of a work item. Work items complete
// or are cancelled; an open work item is an open obligation on its
// trajectory.
type WorkItemStatus string

const (
	WorkItemOpen      WorkItemStatus = "open"
	WorkItemCompleted WorkItemStatus = "completed"
	WorkItemCancelled WorkItemStatus = "cancelled"
	WorkItemRefused   WorkItemStatus = "refused"
)

// WorkItemRecord is a durable assignment on a trajectory: objective,
// bounded authority, budgets, fingerprint-deduped. It is the ported good
// half of the run-continuation record, re-keyed off the run tree and with
// no lease vocabulary.
type WorkItemRecord struct {
	WorkItemID           string         `json:"work_item_id"`
	TrajectoryID         string         `json:"trajectory_id"`
	OwnerID              string         `json:"owner_id"`
	ComputerID           string         `json:"computer_id"`
	Objective            string         `json:"objective"`
	Reason               string         `json:"reason,omitempty"`
	AuthorityProfile     string         `json:"authority_profile,omitempty"`
	StepBudget           int            `json:"step_budget,omitempty"`
	TokenBudget          int            `json:"token_budget,omitempty"`
	ObjectiveFingerprint string         `json:"objective_fingerprint,omitempty"`
	Status               WorkItemStatus `json:"status"`
	ResultRef            string         `json:"result_ref,omitempty"`
	LifecycleVersion     int64          `json:"lifecycle_version,omitempty"`
	LastReducerSeq       int64          `json:"last_reducer_seq,omitempty"`
	AssignedAgentID      string         `json:"assigned_agent_id,omitempty"`
	CreatedByRunID       string         `json:"created_by_loop_id,omitempty"`
	Details              map[string]any `json:"details,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}
