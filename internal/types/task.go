// Package types defines the core runtime types for the go-choir sandbox runtime.
//
// These types represent the foundational vocabulary for Mission 3: run handles,
// lifecycle states, event records, and the minimal type surface needed for stable
// run IDs, persisted state, and later API milestones.
//
// Design decisions:
//   - No adapter-wrapper or native-session model; the runtime loop runs as
//     direct goroutines, not subprocesses.
//   - RunState is a simpler lifecycle than Cogent's JobState because go-choir
//     runs combine session, job, and turn into a single execution handle.
//   - OwnerID links runs to the authenticated user who started them so that
//     status/event surfaces can scope by caller.
//   - Run IDs are UUID strings, generated once at submission and stable across
//     restart, supporting VAL-RUNTIME-003 and VAL-RUNTIME-010.
package types

import (
	"encoding/json"
	"time"
)

// RunState represents the lifecycle state of a runtime run.
type RunState string

const (
	// RunPending means the run was submitted but has not started executing.
	RunPending RunState = "pending"

	// RunRunning means the run is actively executing.
	RunRunning RunState = "running"

	// RunCompleted means the task finished successfully.
	RunCompleted RunState = "completed"

	// RunFailed means the run failed with a structured error outcome.
	// The runtime remains available for later runs (VAL-RUNTIME-008).
	RunFailed RunState = "failed"

	// RunCancelled means the run was cancelled before completion.
	RunCancelled RunState = "cancelled"

	// RunBlocked means the run is blocked (e.g., provider failure)
	// and may be retried or resolved later.
	RunBlocked RunState = "blocked"

	// RunPassivated means the in-process activation ended without a terminal
	// work verdict. The durable agent identity may be re-warmed by backlog or
	// trajectory obligations.
	RunPassivated RunState = "passivated"
)

// Terminal returns true if the state is a terminal state that will not
// transition further.
func (s RunState) Terminal() bool {
	switch s {
	case RunCompleted, RunFailed, RunCancelled:
		return true
	default:
		return false
	}
}

// Active returns true when the run represents current runtime residency or an
// unresolved blocked activation. Passivated runs are non-terminal but no longer
// own live actor slots.
func (s RunState) Active() bool {
	switch s {
	case RunPending, RunRunning, RunBlocked:
		return true
	default:
		return false
	}
}

// Valid returns true if the RunState value is a recognized state.
func (s RunState) Valid() bool {
	switch s {
	case RunPending, RunRunning, RunCompleted, RunFailed, RunCancelled, RunBlocked, RunPassivated:
		return true
	default:
		return false
	}
}

// RunRecord is the persisted representation of a submitted runtime run.
// It carries the stable run ID, owner identity, lifecycle state, and
// enough context for status lookup, event correlation, and restart recovery.
type RunRecord struct {
	// RunID is the stable unique identifier for this run, generated at
	// submission time and persisted for the lifetime of the record.
	// This is the handle used by status/event surfaces (VAL-RUNTIME-003).
	RunID string `json:"loop_id"`

	// AgentID is the durable agent identity that executed this run.
	// Multiple runs may belong to the same agent over time.
	AgentID string `json:"agent_id"`

	// ChannelID is the shared coordination channel for this run family.
	// Related workers and appagents can share a channel without sharing a run.
	ChannelID string `json:"channel_id,omitempty"`

	// ParentRunID links this run to the run that spawned it, if any.
	ParentRunID string `json:"parent_loop_id,omitempty"`

	// TrajectoryID keys this run to its durable trajectory record. It is
	// the same value the runtime threads through run metadata; the column
	// makes trajectory membership queryable without parsing metadata_json.
	TrajectoryID string `json:"trajectory_id,omitempty"`

	// AgentProfile is the profile/tool policy used for this run.
	AgentProfile string `json:"agent_profile,omitempty"`

	// AgentRole is the current role label surfaced to tools and debugging UIs.
	AgentRole string `json:"agent_role,omitempty"`

	// OwnerID is the authenticated user who submitted the run.
	// Status and event surfaces scope by owner (VAL-RUNTIME-006).
	OwnerID string `json:"owner_id"`

	// SandboxID is the sandbox identity that accepted the run.
	SandboxID string `json:"sandbox_id"`

	// State is the current lifecycle state of the run.
	State RunState `json:"state"`

	// Prompt is the user-submitted input that initiated the run.
	Prompt string `json:"prompt"`

	// Result holds the final output text when the run completes.
	// Empty until the run reaches a terminal state with a result.
	Result string `json:"result,omitempty"`

	// Error holds a structured error message when the run fails or is blocked.
	// Empty unless the run is in RunFailed or RunBlocked state.
	Error string `json:"error,omitempty"`

	// CreatedAt is the time the run was submitted.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the time the run state was last changed.
	UpdatedAt time.Time `json:"updated_at"`

	// FinishedAt is the time the run reached a terminal state, or nil.
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Metadata holds extensible key-value data for the run.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EventKind represents the kind of a runtime event emitted during run execution.
type EventKind string

const (
	// EventRunSubmitted is emitted when a run is submitted and accepted.
	EventRunSubmitted EventKind = "loop.submitted"

	// EventRunStarted is emitted when a run begins executing.
	EventRunStarted EventKind = "loop.started"

	// EventRunProgress is emitted for incremental progress updates during
	// run execution.
	EventRunProgress EventKind = "loop.progress"

	// EventRunDelta is emitted for streaming text deltas from the provider
	// response, supporting incremental event streaming (VAL-RUNTIME-005).
	EventRunDelta EventKind = "loop.delta"

	// EventRunCompleted is emitted when a run finishes successfully.
	EventRunCompleted EventKind = "loop.completed"

	// EventRunFailed is emitted when a run fails with a structured error
	// outcome (VAL-RUNTIME-008).
	EventRunFailed EventKind = "loop.failed"

	// EventRunBlocked is emitted when a run is blocked (e.g., provider failure).
	EventRunBlocked EventKind = "loop.blocked"

	// EventRunPassivated is emitted when recovery releases an activation without
	// converting the agent's durable work into failure.
	EventRunPassivated EventKind = "activation.passivated"

	// EventRunCompactionStarted is emitted before the runtime compacts a run's
	// persisted context into an operational memory checkpoint.
	EventRunCompactionStarted EventKind = "loop.compaction.started"

	// EventRunCompactionCompleted is emitted after a run-memory checkpoint has
	// been persisted and the provider context can be rebuilt from it.
	EventRunCompactionCompleted EventKind = "loop.compaction.completed"

	// EventRunRetry is emitted when the runtime retries a provider call after a
	// recoverable context-management action such as overflow compaction.
	EventRunRetry EventKind = "loop.retry"

	// EventRunContinuationSelected is emitted when the runtime records the next
	// objective that should continue a completed run.
	EventRunContinuationSelected EventKind = "loop.continuation.selected"

	// EventRunContinuationStarted is emitted when a selected continuation starts
	// as a child run.
	EventRunContinuationStarted EventKind = "loop.continuation.started"

	// EventRunCancelled is emitted when a run is cancelled.
	EventRunCancelled EventKind = "loop.cancelled"

	// EventRuntimeHealth is emitted when the runtime health state changes.
	EventRuntimeHealth EventKind = "runtime.health"

	// EventRuntimeDegraded is emitted when the runtime enters a degraded state.
	EventRuntimeDegraded EventKind = "runtime.degraded"

	// EventToolInvoked is emitted when the tool-calling loop invokes a
	// registered tool. The payload includes the tool name, call ID, and
	// argument summary (VAL-RUNTIME-005: tool-driven progress is observable).
	EventToolInvoked EventKind = "tool.invoked"

	// EventToolResult is emitted when a tool invocation completes. The
	// payload includes the tool name, call ID, and result summary.
	EventToolResult EventKind = "tool.result"

	// EventChannelMessage is emitted when a message is posted to an agent
	// channel, making inter-agent coordination observable through the
	// event stream.
	EventChannelMessage EventKind = "channel.message"

	// EventEmailDraftApprovalRecorded is emitted when Email appagent records
	// an approval decision for a specific draft version.
	EventEmailDraftApprovalRecorded EventKind = "email.draft.approval_recorded"

	// EventEmailDraftBlocked is emitted when Email appagent blocks a draft
	// before approval because policy detected a risky email artifact.
	EventEmailDraftBlocked EventKind = "email.draft.blocked"

	// EventEmailDraftSent is emitted when an approved Email appagent draft is
	// handed to maild/Resend and stored as sent.
	EventEmailDraftSent EventKind = "email.draft.sent"

	// EventBrowserSessionCreated is emitted when the backend Browser product
	// path creates an owner-scoped browser session.
	EventBrowserSessionCreated EventKind = "browser.session.created"

	// EventBrowserSessionClosed is emitted when the backend Browser product
	// path closes an owner-scoped browser session.
	EventBrowserSessionClosed EventKind = "browser.session.closed"

	// EventBrowserNavigationCompleted is emitted when a backend Browser
	// session successfully captures a server-owned navigation snapshot.
	EventBrowserNavigationCompleted EventKind = "browser.navigation.completed"

	// EventBrowserNavigationFailed is emitted when a backend Browser session
	// cannot navigate or snapshot the requested URL.
	EventBrowserNavigationFailed EventKind = "browser.navigation.failed"

	// EventBrowserControlCompleted is emitted when a backend Browser session
	// completes a bounded input/control action.
	EventBrowserControlCompleted EventKind = "browser.control.completed"

	// EventBrowserControlFailed is emitted when a backend Browser session
	// rejects or fails a bounded input/control action.
	EventBrowserControlFailed EventKind = "browser.control.failed"

	// EventAppChangePackagePublished is emitted when a candidate app change is
	// exported as a product-visible AppChangePackage.
	EventAppChangePackagePublished EventKind = "app_change_package.published"

	// EventAppAdoptionProposed is emitted when a recipient candidate computer
	// starts applying an AppChangePackage.
	EventAppAdoptionProposed EventKind = "app_adoption.proposed"

	// EventAppAdoptionVerificationStarted is emitted before recipient-side
	// verifier contracts run resource-heavy build work.
	EventAppAdoptionVerificationStarted EventKind = "app_adoption.verification_started"

	// EventAppAdoptionVerified is emitted when recipient-side verifier
	// contracts accept the rebuilt app artifacts.
	EventAppAdoptionVerified EventKind = "app_adoption.verified"

	// EventAppAdoptionBlocked is emitted when recipient-side verifier contracts
	// reject or cannot complete an adoption.
	EventAppAdoptionBlocked EventKind = "app_adoption.blocked"

	// EventAppAdoptionOwnerApproved is emitted when the owner approves a
	// verified adoption for promotion. Review authorizes a verified
	// transition; it does not replace verification.
	EventAppAdoptionOwnerApproved EventKind = "app_adoption.owner_approved"

	// EventAppAdoptionPromoted is emitted when an approved adoption advances a
	// target computer source lineage.
	EventAppAdoptionPromoted EventKind = "app_adoption.promoted"

	// EventAppAdoptionRolledBack is emitted when an adoption restores the prior
	// source lineage and route/artifact profile.
	EventAppAdoptionRolledBack EventKind = "app_adoption.rolled_back"

	// EventVTextAgentRevisionStarted is emitted when an appagent-driven
	// document revision starts executing. The payload includes the doc_id
	// so the frontend can correlate the revision to the open document
	// (VAL-ETEXT-004).
	EventVTextAgentRevisionStarted EventKind = "vtext.agent_revision.started"

	// EventVTextAgentRevisionProgress is emitted during appagent revision
	// execution, carrying incremental progress that the open document
	// can display without manual refresh (VAL-ETEXT-004).
	EventVTextAgentRevisionProgress EventKind = "vtext.agent_revision.progress"

	// EventVTextAgentRevisionCompleted is emitted when an appagent-driven
	// revision completes and the canonical revision is created. The payload
	// includes the doc_id and revision_id (VAL-ETEXT-003, VAL-ETEXT-004).
	EventVTextAgentRevisionCompleted EventKind = "vtext.agent_revision.completed"

	// EventVTextAgentRevisionFailed is emitted when an appagent-driven
	// revision fails. The payload includes the doc_id and error message.
	EventVTextAgentRevisionFailed EventKind = "vtext.agent_revision.failed"

	// EventVTextDocumentRevisionCreated is emitted when a canonical document
	// revision is created outside the appagent synthesis loop, such as a direct
	// user-authored save through the document API. The payload includes doc_id,
	// revision_id, and current_revision_id so the editor can follow head changes.
	EventVTextDocumentRevisionCreated EventKind = "vtext.document_revision.created"

	// EventDesktopStateUpdated is emitted when a user's persisted desktop
	// workspace changes.
	EventDesktopStateUpdated EventKind = "desktop.state.updated"

	// EventDesktopDriverLeaseUpdated is emitted when a browser session becomes
	// the current interaction driver for a desktop.
	EventDesktopDriverLeaseUpdated EventKind = "desktop.driver_lease.updated"

	// EventDesktopAppInstancesUpdated is emitted when the shared open app
	// instance roster or semantic stack order changes.
	EventDesktopAppInstancesUpdated EventKind = "desktop.app_instances.updated"

	// EventDesktopWindowPlacementUpdated is emitted when a session-local
	// placement/focus record changes.
	EventDesktopWindowPlacementUpdated EventKind = "desktop.window_placement.updated"

	// EventContentItemCreated is emitted when a durable content item is created.
	EventContentItemCreated EventKind = "content.item.created"

	// EventMediaProgressUpdated is emitted when media playback or reading
	// progress is persisted.
	EventMediaProgressUpdated EventKind = "media.progress.updated"

	// EventMediaRecentUpdated is emitted when a media app records a recently
	// opened source.
	EventMediaRecentUpdated EventKind = "media.recent.updated"

	// EventThemeUpdated is emitted when a user's theme preference changes.
	EventThemeUpdated EventKind = "theme.updated"

	// EventFileChanged is emitted when the authenticated Files surface creates,
	// updates, or deletes a file-system entry.
	EventFileChanged EventKind = "file.changed"

	// EventComputerStatusUpdated is emitted when a user-scoped computer status
	// snapshot changes.
	EventComputerStatusUpdated EventKind = "computer.status.updated"
)

// EventRecord represents a single runtime event emitted during run execution
// or runtime lifecycle changes. Events are ordered by sequence number within
// a run, persisted for restart recovery, and projected through Trace
// (VAL-RUNTIME-005).
type EventRecord struct {
	// EventID is the unique identifier for this event.
	EventID string `json:"event_id"`

	// Seq is the per-run sequence number, assigned monotonically on append.
	// Events for the same run can be fetched incrementally using after-seq
	// cursors.
	Seq int64 `json:"seq"`

	// StreamSeq is the owner/global monotonic sequence used for cross-loop
	// catch-up and streaming.
	StreamSeq int64 `json:"stream_seq,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"ts"`

	// RunID is the run this event is correlated to. For runtime-level events
	// (health, degraded), this may be empty.
	RunID string `json:"loop_id,omitempty"`

	// AgentID is the durable agent identity that emitted or owns this event.
	AgentID string `json:"agent_id,omitempty"`

	// ChannelID is the shared coordination channel correlated to this event.
	ChannelID string `json:"channel_id,omitempty"`

	// OwnerID is the authenticated user who owns the run, used for
	// caller-scoped event streaming (VAL-RUNTIME-006).
	OwnerID string `json:"owner_id,omitempty"`

	// TrajectoryID ties the event to the broader user-visible workflow.
	TrajectoryID string `json:"trajectory_id,omitempty"`

	// Kind is the event kind from the vocabulary above.
	Kind EventKind `json:"kind"`

	// Phase provides additional phase context for the event (e.g., "execution",
	// "translation", "recovery").
	Phase string `json:"phase,omitempty"`

	// Payload carries the event-specific data as a JSON blob.
	Payload json.RawMessage `json:"payload"`
}

// RunMemoryEntryKind describes the durable entry type in a run's context log.
type RunMemoryEntryKind string

const (
	// RunMemoryEntryMessage stores a provider-facing conversation message.
	RunMemoryEntryMessage RunMemoryEntryKind = "message"

	// RunMemoryEntryCompaction stores an operational summary checkpoint and
	// the first raw message retained after that checkpoint.
	RunMemoryEntryCompaction RunMemoryEntryKind = "compaction"
)

// RunMemoryEntry is a durable, ordered context record for a runtime run. The
// provider context is rebuilt from these entries, allowing runs to survive
// process restarts and context-window compaction.
type RunMemoryEntry struct {
	EntryID          string             `json:"entry_id"`
	RunID            string             `json:"loop_id"`
	OwnerID          string             `json:"owner_id"`
	AgentID          string             `json:"agent_id,omitempty"`
	ParentEntryID    string             `json:"parent_entry_id,omitempty"`
	Seq              int64              `json:"seq"`
	Kind             RunMemoryEntryKind `json:"kind"`
	Role             string             `json:"role,omitempty"`
	Message          json.RawMessage    `json:"message,omitempty"`
	Summary          string             `json:"summary,omitempty"`
	FirstKeptEntryID string             `json:"first_kept_entry_id,omitempty"`
	TokensBefore     int                `json:"tokens_before,omitempty"`
	Reason           string             `json:"reason,omitempty"`
	Model            string             `json:"model,omitempty"`
	Details          map[string]any     `json:"details,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
}

// RunContinuationStatus is the lifecycle of a durable next-goal selection.
type RunContinuationStatus string

const (
	RunContinuationSelected RunContinuationStatus = "selected"
	RunContinuationStarted  RunContinuationStatus = "started"
	RunContinuationBlocked  RunContinuationStatus = "blocked"
)

// RunContinuationRecord records the next objective chosen after a run has
// completed and compacted enough context for safe continuation.
type RunContinuationRecord struct {
	ContinuationID   string                `json:"continuation_id"`
	OwnerID          string                `json:"owner_id"`
	SourceRunID      string                `json:"source_loop_id"`
	NextRunID        string                `json:"next_loop_id,omitempty"`
	Objective        string                `json:"objective"`
	Reason           string                `json:"reason,omitempty"`
	AuthorityProfile string                `json:"authority_profile,omitempty"`
	LeaseSeconds     int                   `json:"lease_seconds,omitempty"`
	Status           RunContinuationStatus `json:"status"`
	Details          map[string]any        `json:"details,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// ToolCall represents a single tool invocation request from the LLM provider.
// When the provider returns a tool_use stop reason, each call specifies which
// tool to invoke and with what arguments. The tool-calling loop executes these
// calls and returns the results to the provider for the next turn.
type ToolCall struct {
	// ID is the provider-assigned call identifier, used to correlate the
	// result back to the provider's conversation history.
	ID string `json:"id"`

	// Name is the registered tool name to invoke.
	Name string `json:"name"`

	// Arguments is the raw JSON arguments object from the provider.
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the output of a tool invocation, sent back to the
// provider as a tool_result content block in the conversation history.
type ToolResult struct {
	// CallID is the ID from the originating ToolCall.
	CallID string `json:"call_id"`

	// Output is the text result from the tool execution.
	Output string `json:"output"`

	// IsError is true if the tool execution returned an error.
	IsError bool `json:"is_error,omitempty"`
}

// ChannelMessage represents a message posted to an agent channel for
// inter-agent coordination. Channels support appagent and worker
// communication without going through the LLM provider loop.
type ChannelMessage struct {
	// ChannelID is the shared coordination channel that owns this message.
	ChannelID string `json:"channel_id,omitempty"`

	// Seq is the durable per-channel sequence number for incremental reads.
	Seq int64 `json:"seq,omitempty"`

	// From identifies the sender (e.g., "appagent", "worker-1", "runtime").
	From string `json:"from"`

	// FromAgentID identifies the durable agent that posted the message.
	FromAgentID string `json:"from_agent_id,omitempty"`

	// FromRunID identifies the run that posted the message.
	FromRunID string `json:"from_loop_id,omitempty"`

	// ToAgentID identifies the addressed recipient agent for directed delivery.
	// Empty means the message is broadcast on the channel.
	ToAgentID string `json:"to_agent_id,omitempty"`

	// ToRunID identifies the addressed recipient run for directed delivery when
	// a specific live execution is the target. Empty means no specific run is
	// required.
	ToRunID string `json:"to_loop_id,omitempty"`

	// TrajectoryID ties the message to the broader user-visible workflow.
	TrajectoryID string `json:"trajectory_id,omitempty"`

	// Role classifies the message (e.g., "coordinator", "worker", "status").
	Role string `json:"role"`

	// Content is the message body.
	Content string `json:"content"`

	// Timestamp is when the message was posted.
	Timestamp time.Time `json:"timestamp"`
}

// InboxDelivery is the runtime-owned delivery queue entry for a directed
// message. Unlike ChannelMessage, which is the audit log / trace surface, inbox
// deliveries are consumed by the runtime and threaded back into agent loops as
// user turns.
type InboxDelivery struct {
	DeliveryID        string     `json:"delivery_id"`
	OwnerID           string     `json:"owner_id"`
	ToAgentID         string     `json:"to_agent_id"`
	ToRunID           string     `json:"to_loop_id,omitempty"`
	FromAgentID       string     `json:"from_agent_id,omitempty"`
	FromRunID         string     `json:"from_loop_id,omitempty"`
	ChannelID         string     `json:"channel_id,omitempty"`
	Role              string     `json:"role,omitempty"`
	Content           string     `json:"content"`
	TrajectoryID      string     `json:"trajectory_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	DeliveredToLoopID string     `json:"delivered_to_loop_id,omitempty"`
	DeliveredAt       *time.Time `json:"delivered_at,omitempty"`
}

// AgentRecord is the durable runtime representation of an agent identity.
// Runs are ephemeral executions owned by an agent; channels are the shared
// coordination surface that can outlive any one run.
type AgentRecord struct {
	AgentID   string    `json:"agent_id"`
	OwnerID   string    `json:"owner_id"`
	SandboxID string    `json:"sandbox_id"`
	Profile   string    `json:"profile"`
	Role      string    `json:"role"`
	ChannelID string    `json:"channel_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RuntimeHealthState represents the health state of the runtime.
type RuntimeHealthState string

const (
	// HealthReady means the runtime is ready for task handling.
	HealthReady RuntimeHealthState = "ready"

	// HealthDegraded means the runtime is degraded but partially functional.
	// This is surfaced as degraded rather than hidden behind a generic healthy
	// response (VAL-RUNTIME-001).
	HealthDegraded RuntimeHealthState = "degraded"

	// HealthFailed means the runtime is not functional.
	HealthFailed RuntimeHealthState = "failed"
)
