package types

import "time"

const LifecycleReducerVersion = "durable-work/v1"

const DurableWorkSchemaV1 = "choir.durable_work.v1"

type LifecycleCommandKind string

const (
	LifecycleStart              LifecycleCommandKind = "start"
	LifecycleOpenWork           LifecycleCommandKind = "open_work"
	LifecycleAmendWork          LifecycleCommandKind = "amend_work"
	LifecycleRecordRefs         LifecycleCommandKind = "record_refs"
	LifecycleQueueUpdate        LifecycleCommandKind = "queue_update"
	LifecycleApplyUpdate        LifecycleCommandKind = "apply_update"
	LifecycleCommitArtifactHead LifecycleCommandKind = "commit_artifact_head"
	LifecycleReplaceActivation  LifecycleCommandKind = "replace_activation"
	LifecycleSettleWork         LifecycleCommandKind = "settle_work"
	LifecycleRefuseWork         LifecycleCommandKind = "refuse_work"
	LifecycleSettleTrajectory   LifecycleCommandKind = "settle_trajectory"
	LifecycleCancelTrajectory   LifecycleCommandKind = "cancel_trajectory"
	LifecycleArchiveArtifact    LifecycleCommandKind = "archive_artifact"
)

type LifecycleEventKind string

const (
	LifecycleUpdateLate           LifecycleEventKind = "update_late"
	LifecycleTrajectoryStarted    LifecycleEventKind = "trajectory_started"
	LifecycleWorkOpened           LifecycleEventKind = "work_opened"
	LifecycleWorkAmended          LifecycleEventKind = "work_amended"
	LifecycleRefsRecorded         LifecycleEventKind = "refs_recorded"
	LifecycleUpdateQueued         LifecycleEventKind = "update_queued"
	LifecycleActivationReplaced   LifecycleEventKind = "activation_replaced"
	LifecycleUpdateApplied        LifecycleEventKind = "update_applied"
	LifecycleArtifactHeadAdvanced LifecycleEventKind = "artifact_head_advanced"
	LifecycleWorkSettled          LifecycleEventKind = "work_settled"
	LifecycleUpdateRejected       LifecycleEventKind = "update_rejected"
	LifecycleWorkRefused          LifecycleEventKind = "work_refused"
	LifecycleTrajectorySettled    LifecycleEventKind = "trajectory_settled"
	LifecycleTrajectoryCancelled  LifecycleEventKind = "trajectory_cancelled"
	LifecycleArtifactArchived     LifecycleEventKind = "artifact_archived"
)

type StartLifecycleRequest struct {
	OwnerID            string            `json:"owner_id"`
	ComputerID         string            `json:"computer_id"`
	CommandID          string            `json:"command_id"`
	StartRequestDigest string            `json:"start_request_digest"`
	TrajectoryID       string            `json:"trajectory_id"`
	Kind               TrajectoryKind    `json:"kind,omitempty"`
	SubjectRefs        map[string]string `json:"subject_refs,omitempty"`
	SettlementRule     SettlementRule    `json:"settlement_rule"`
	InitialWork        WorkItemRecord    `json:"initial_work"`
	InitialDocument    Document          `json:"initial_document"`
	InitialRevision    Revision          `json:"initial_revision"`
	Agent              AgentRecord       `json:"agent"`
}

type ApplyLifecycleRelatedUpdate struct {
	TargetAgentID    string            `json:"target_agent_id"`
	ProducerAgentID  string            `json:"producer_agent_id"`
	ProducerUpdateID string            `json:"producer_update_id"`
	UpdateID         string            `json:"update_id"`
	Disposition      UpdateDisposition `json:"disposition"`
	DispositionRef   string            `json:"disposition_ref"`
	WorkDisposition  WorkItemStatus    `json:"work_disposition,omitempty"`
	WorkItemID       string            `json:"work_item_id,omitempty"`
	WorkResultRef    string            `json:"work_result_ref,omitempty"`
	Reason           string            `json:"reason,omitempty"`
}

type ApplyLifecycleUpdateRequest struct {
	OwnerID                   string                        `json:"owner_id"`
	ComputerID                string                        `json:"computer_id"`
	CommandID                 string                        `json:"command_id"`
	CommandDigest             string                        `json:"command_digest"`
	TrajectoryID              string                        `json:"trajectory_id"`
	TargetAgentID             string                        `json:"target_agent_id"`
	ProducerAgentID           string                        `json:"producer_agent_id"`
	ProducerUpdateID          string                        `json:"producer_update_id"`
	UpdateID                  string                        `json:"update_id"`
	MessageSeq                int64                         `json:"message_seq,omitempty"`
	ChannelID                 string                        `json:"channel_id,omitempty"`
	Role                      string                        `json:"role,omitempty"`
	SourceRunID               string                        `json:"source_run_id,omitempty"`
	Packet                    CoagentSourcePacketPayload    `json:"packet"`
	Content                   string                        `json:"content"`
	Disposition               UpdateDisposition             `json:"disposition"`
	Revision                  Revision                      `json:"revision,omitempty"`
	WorkDisposition           WorkItemStatus                `json:"work_disposition,omitempty"`
	WorkItemID                string                        `json:"work_item_id,omitempty"`
	WorkResultRef             string                        `json:"work_result_ref,omitempty"`
	SubjectRefs               map[string]string             `json:"subject_refs,omitempty"`
	Reason                    string                        `json:"reason,omitempty"`
	PayloadDigest             string                        `json:"payload_digest"`
	ReferenceExistingArtifact bool                          `json:"reference_existing_artifact,omitempty"`
	DispositionRef            string                        `json:"disposition_ref"`
	RelatedUpdates            []ApplyLifecycleRelatedUpdate `json:"related_updates,omitempty"`
}

type QueueLifecycleUpdateRequest ApplyLifecycleUpdateRequest

type OpenLifecycleWorkRequest struct {
	OwnerID       string         `json:"owner_id"`
	ComputerID    string         `json:"computer_id"`
	CommandID     string         `json:"command_id"`
	CommandDigest string         `json:"command_digest"`
	TrajectoryID  string         `json:"trajectory_id"`
	WorkItem      WorkItemRecord `json:"work_item"`
}

type AmendLifecycleWorkRequest struct {
	OwnerID                  string         `json:"owner_id"`
	ComputerID               string         `json:"computer_id"`
	CommandID                string         `json:"command_id"`
	CommandDigest            string         `json:"command_digest"`
	TrajectoryID             string         `json:"trajectory_id"`
	WorkItemID               string         `json:"work_item_id"`
	ExpectedLifecycleVersion int64          `json:"expected_lifecycle_version"`
	WorkItem                 WorkItemRecord `json:"work_item"`
}

type ReplaceLifecycleActivationRequest struct {
	OwnerID       string    `json:"owner_id"`
	ComputerID    string    `json:"computer_id"`
	CommandID     string    `json:"command_id"`
	CommandDigest string    `json:"command_digest"`
	TrajectoryID  string    `json:"trajectory_id"`
	AgentID       string    `json:"agent_id"`
	Run           RunRecord `json:"run"`
}

type RecordLifecycleRefsRequest struct {
	OwnerID       string            `json:"owner_id"`
	ComputerID    string            `json:"computer_id"`
	CommandID     string            `json:"command_id"`
	CommandDigest string            `json:"command_digest"`
	TrajectoryID  string            `json:"trajectory_id"`
	WorkItemID    string            `json:"work_item_id,omitempty"`
	ArtifactRefs  []string          `json:"artifact_refs,omitempty"`
	EvidenceRefs  []string          `json:"evidence_refs,omitempty"`
	SubjectRefs   map[string]string `json:"subject_refs,omitempty"`
	Reason        string            `json:"reason,omitempty"`
}

type SettleLifecycleWorkRequest struct {
	OwnerID       string `json:"owner_id"`
	ComputerID    string `json:"computer_id"`
	CommandID     string `json:"command_id"`
	CommandDigest string `json:"command_digest"`
	TrajectoryID  string `json:"trajectory_id"`
	WorkItemID    string `json:"work_item_id"`
	ActingAgentID string `json:"acting_agent_id"`
	ResultRef     string `json:"result_ref"`
}

type RefuseLifecycleWorkRequest struct {
	OwnerID       string `json:"owner_id"`
	ComputerID    string `json:"computer_id"`
	CommandID     string `json:"command_id"`
	CommandDigest string `json:"command_digest"`
	TrajectoryID  string `json:"trajectory_id"`
	WorkItemID    string `json:"work_item_id"`
	ActingAgentID string `json:"acting_agent_id"`
	RefusalRef    string `json:"refusal_ref"`
	Reason        string `json:"reason"`
}

type CancelLifecycleRequest struct {
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	TrajectoryID             string `json:"trajectory_id"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string `json:"expected_head_revision_id"`
	Reason                   string `json:"reason"`
}

type SettleLifecycleTrajectoryRequest struct {
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	TrajectoryID             string `json:"trajectory_id"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string `json:"expected_head_revision_id"`
}

type CommitLifecycleArtifactHeadRequest struct {
	OwnerID                  string   `json:"owner_id"`
	ComputerID               string   `json:"computer_id"`
	CommandID                string   `json:"command_id"`
	CommandDigest            string   `json:"command_digest"`
	TrajectoryID             string   `json:"trajectory_id"`
	ExpectedLifecycleVersion int64    `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string   `json:"expected_head_revision_id"`
	Unbound                  bool     `json:"unbound,omitempty"`
	Revision                 Revision `json:"revision"`
}

type ArchiveLifecycleArtifactRequest struct {
	OwnerID                  string `json:"owner_id"`
	ComputerID               string `json:"computer_id"`
	CommandID                string `json:"command_id"`
	CommandDigest            string `json:"command_digest"`
	TrajectoryID             string `json:"trajectory_id"`
	ExpectedLifecycleVersion int64  `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string `json:"expected_head_revision_id"`
	Reason                   string `json:"reason,omitempty"`
}

type LifecycleStoredResult struct {
	Trajectory TrajectoryRecord     `json:"trajectory"`
	Schema     string               `json:"schema,omitempty"`
	WorkItem   *WorkItemRecord      `json:"work_item,omitempty"`
	Agent      *AgentRecord         `json:"agent,omitempty"`
	Update     *CoagentSourcePacket `json:"update,omitempty"`
	Events     []LifecycleEvent     `json:"events"`
	Document   *Document            `json:"document,omitempty"`
	Revision   *Revision            `json:"revision,omitempty"`
}

type LifecycleCommandReceipt struct {
	CommandID       string                 `json:"command_id"`
	CommandDigest   string                 `json:"command_digest"`
	Kind            LifecycleCommandKind   `json:"kind"`
	OwnerID         string                 `json:"owner_id"`
	ComputerID      string                 `json:"computer_id"`
	TrajectoryID    string                 `json:"trajectory_id"`
	ReducerVersion  string                 `json:"reducer_version"`
	ReducerSeq      int64                  `json:"reducer_seq"`
	ResultEventRefs []string               `json:"result_event_refs"`
	CreatedAt       time.Time              `json:"created_at"`
	StoredResult    *LifecycleStoredResult `json:"stored_result,omitempty"`
}

type LifecycleEvent struct {
	Schema         string             `json:"schema,omitempty"`
	EventID        string             `json:"event_id"`
	OwnerID        string             `json:"owner_id"`
	ComputerID     string             `json:"computer_id"`
	TrajectoryID   string             `json:"trajectory_id"`
	WorkItemID     string             `json:"work_item_id,omitempty"`
	UpdateID       string             `json:"update_id,omitempty"`
	Kind           LifecycleEventKind `json:"kind"`
	ReducerVersion string             `json:"reducer_version"`
	ReducerSeq     int64              `json:"reducer_seq"`
	CommandID      string             `json:"command_id"`
	CommandDigest  string             `json:"command_digest"`
	ArtifactRefs   []string           `json:"artifact_refs,omitempty"`
	EvidenceRefs   []string           `json:"evidence_refs,omitempty"`
	Reason         string             `json:"reason,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

type LifecycleResult struct {
	Receipt    LifecycleCommandReceipt `json:"receipt"`
	Trajectory TrajectoryRecord        `json:"trajectory"`
	Schema     string                  `json:"schema,omitempty"`
	WorkItem   *WorkItemRecord         `json:"work_item,omitempty"`
	Agent      *AgentRecord            `json:"agent,omitempty"`
	Update     *CoagentSourcePacket    `json:"update,omitempty"`
	Events     []LifecycleEvent        `json:"events"`
	Replay     bool                    `json:"replay"`
	Document   *Document               `json:"document,omitempty"`
	Revision   *Revision               `json:"revision,omitempty"`
}

type LifecycleActivationProjection struct {
	AgentID string   `json:"agent_id"`
	RunID   string   `json:"run_id,omitempty"`
	State   RunState `json:"state"`
}

type LifecycleSnapshot struct {
	Trajectory          TrajectoryRecord              `json:"trajectory"`
	WorkItems           []WorkItemRecord              `json:"work_items"`
	Agents              []AgentRecord                 `json:"agents"`
	Activation          LifecycleActivationProjection `json:"activation"`
	Schema              string                        `json:"schema"`
	CurrentDocumentHead *Revision                     `json:"current_document_head,omitempty"`
	Updates             []CoagentSourcePacket         `json:"updates"`
	Document            Document                      `json:"document"`
	HeadRevision        Revision                      `json:"head_revision"`
	Events              []LifecycleEvent              `json:"events"`
	SnapshotCursor      int64                         `json:"snapshot_cursor"`
	Watermark           int64                         `json:"watermark"`
}

type LifecycleEventPage struct {
	Schema         string           `json:"schema"`
	CursorExpired  bool             `json:"cursor_expired,omitempty"`
	ReplayRequired bool             `json:"replay_required,omitempty"`
	Events         []LifecycleEvent `json:"events"`
	NextCursor     int64            `json:"next_cursor"`
	Watermark      int64            `json:"watermark"`
}
