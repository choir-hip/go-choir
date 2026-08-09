package types

import "time"

const LifecycleReducerVersion = "durable-work/v1"

const DurableWorkSchemaV1 = "choir.durable_work.v1"

type LifecycleCommandKind string

const (
	LifecycleStart                        LifecycleCommandKind = "start"
	LifecycleOpenWork                     LifecycleCommandKind = "open_work"
	LifecycleAmendWork                    LifecycleCommandKind = "amend_work"
	LifecycleRecordRefs                   LifecycleCommandKind = "record_refs"
	LifecycleQueueUpdate                  LifecycleCommandKind = "queue_update"
	LifecycleApplyUpdate                  LifecycleCommandKind = "apply_update"
	LifecycleCommitArtifactHead           LifecycleCommandKind = "commit_artifact_head"
	LifecycleReplaceActivation            LifecycleCommandKind = "replace_activation"
	LifecycleSettleWork                   LifecycleCommandKind = "settle_work"
	LifecycleRefuseWork                   LifecycleCommandKind = "refuse_work"
	LifecycleSettleTrajectory             LifecycleCommandKind = "settle_trajectory"
	LifecycleCancelTrajectory             LifecycleCommandKind = "cancel_trajectory"
	LifecyclePrepareCancelTrajectory      LifecycleCommandKind = "prepare_cancel_trajectory"
	LifecycleArchiveArtifact              LifecycleCommandKind = "archive_artifact"
	LifecycleApplyTextureTurn             LifecycleCommandKind = "apply_texture_turn"
	LifecycleBindControlDelivery          LifecycleCommandKind = "bind_control_delivery"
	LifecycleFailControlActivation        LifecycleCommandKind = "fail_control_activation"
	LifecycleQueueOwnerInstruction        LifecycleCommandKind = "queue_owner_instruction"
	LifecycleOpenCoSuperAssignment        LifecycleCommandKind = "open_co_super_assignment"
	LifecycleBindCoSuperAssignment        LifecycleCommandKind = "bind_co_super_assignment"
	LifecycleRecordCoSuperAssignment      LifecycleCommandKind = "record_co_super_assignment"
	LifecycleCancelCoSuperAssignment      LifecycleCommandKind = "cancel_co_super_assignment"
	LifecycleSetCoSuperCapsuleDisposition LifecycleCommandKind = "set_co_super_capsule_disposition"
)

type LifecycleEventKind string

const (
	LifecycleUpdateLate                      LifecycleEventKind = "update_late"
	LifecycleTrajectoryStarted               LifecycleEventKind = "trajectory_started"
	LifecycleWorkOpened                      LifecycleEventKind = "work_opened"
	LifecycleWorkAmended                     LifecycleEventKind = "work_amended"
	LifecycleRefsRecorded                    LifecycleEventKind = "refs_recorded"
	LifecycleUpdateQueued                    LifecycleEventKind = "update_queued"
	LifecycleActivationReplaced              LifecycleEventKind = "activation_replaced"
	LifecycleUpdateApplied                   LifecycleEventKind = "update_applied"
	LifecycleArtifactHeadAdvanced            LifecycleEventKind = "artifact_head_advanced"
	LifecycleWorkSettled                     LifecycleEventKind = "work_settled"
	LifecycleUpdateRejected                  LifecycleEventKind = "update_rejected"
	LifecycleWorkRefused                     LifecycleEventKind = "work_refused"
	LifecycleTrajectorySettled               LifecycleEventKind = "trajectory_settled"
	LifecycleTrajectoryCancelled             LifecycleEventKind = "trajectory_cancelled"
	LifecycleTrajectoryCancellationRequested LifecycleEventKind = "trajectory_cancellation_requested"
	LifecycleArtifactArchived                LifecycleEventKind = "artifact_archived"
	LifecycleTextureTurnCommitted            LifecycleEventKind = "texture_turn_committed"
	LifecycleControlQueued                   LifecycleEventKind = "control_queued"
	LifecycleControlDelivered                LifecycleEventKind = "control_delivered"
	LifecycleControlActivationFailed         LifecycleEventKind = "control_activation_failed"
	LifecycleOwnerInstructionQueued          LifecycleEventKind = "owner_instruction_queued"
	LifecycleCoSuperAssignmentOpened         LifecycleEventKind = "co_super_assignment_opened"
	LifecycleCoSuperAssignmentBound          LifecycleEventKind = "co_super_assignment_bound"
	LifecycleCoSuperAssignmentReported       LifecycleEventKind = "co_super_assignment_reported"
	LifecycleCoSuperAssignmentCancelled      LifecycleEventKind = "co_super_assignment_cancelled"
	LifecycleCoSuperCapsuleDispositionSet    LifecycleEventKind = "co_super_capsule_disposition_set"
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
	ControlBindingID          string                        `json:"control_binding_id,omitempty"`
	TargetWorkItemID          string                        `json:"target_work_item_id,omitempty"`
	ConsumedDeliveryUpdateIDs []string                      `json:"consumed_delivery_update_ids,omitempty"`
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

// TextureTurnOutcome is the single durable semantic result of one Texture
// controller turn. Only TextureTurnRevision advances the canonical document
// head; the other outcomes remain durable without manufacturing a revision.
type TextureTurnOutcome string

const (
	TextureTurnRevision         TextureTurnOutcome = "revision"
	TextureTurnNoSemanticChange TextureTurnOutcome = "no_semantic_change"
	TextureTurnWait             TextureTurnOutcome = "wait"
	TextureTurnBlock            TextureTurnOutcome = "block"
)

// TextureTurnInboundDisposition explicitly resolves one ordered producer
// report and, when requested, its producer-owned work obligation.
type TextureTurnInboundDisposition struct {
	TargetAgentID      string            `json:"target_agent_id"`
	ProducerAgentID    string            `json:"producer_agent_id"`
	ProducerUpdateID   string            `json:"producer_update_id"`
	UpdateID           string            `json:"update_id"`
	Disposition        UpdateDisposition `json:"disposition"`
	ProducerWorkItemID string            `json:"producer_work_item_id"`
	WorkDisposition    WorkItemStatus    `json:"work_disposition"`
	WorkResultRef      string            `json:"work_result_ref,omitempty"`
	Reason             string            `json:"reason,omitempty"`
}

// TextureTurnControl is one ordered, downward target-control packet. OpenWork
// is populated only for the exact persistent-Super opener. The work and first
// execution_request are committed by the same conditional turn batch.
type TextureTurnControl struct {
	ControlID        string                     `json:"control_id"`
	TargetAgentID    string                     `json:"target_agent_id"`
	TargetWorkItemID string                     `json:"target_work_item_id"`
	OpenAgent        *AgentRecord               `json:"open_agent,omitempty"`
	OpenWork         *WorkItemRecord            `json:"open_work,omitempty"`
	Packet           CoagentSourcePacketPayload `json:"packet"`
	Content          string                     `json:"content"`
	PayloadDigest    string                     `json:"payload_digest"`
}

// ApplyTextureTurnRequest is the sole store mutation for a progressive Texture
// turn. Runtime derives all scope and actor identities before this command.
type ApplyTextureTurnRequest struct {
	OwnerID                        string                          `json:"owner_id"`
	ComputerID                     string                          `json:"computer_id"`
	CommandID                      string                          `json:"command_id"`
	CommandDigest                  string                          `json:"command_digest"`
	DocumentID                     string                          `json:"document_id"`
	TrajectoryID                   string                          `json:"trajectory_id"`
	CallerAgentID                  string                          `json:"caller_agent_id"`
	CallerRunID                    string                          `json:"caller_run_id"`
	OwnerInstructions              []TextureTurnOwnerInstruction   `json:"owner_instructions,omitempty"`
	ExpectedLifecycleVersion       int64                           `json:"expected_lifecycle_version"`
	ExpectedCallerLifecycleVersion int64                           `json:"expected_caller_lifecycle_version"`
	ExpectedHeadRevisionID         string                          `json:"expected_head_revision_id"`
	CallerWorkItemID               string                          `json:"caller_work_item_id"`
	CallerWorkDisposition          WorkItemStatus                  `json:"caller_work_disposition"`
	Outcome                        TextureTurnOutcome              `json:"outcome"`
	Revision                       Revision                        `json:"revision,omitempty"`
	Reason                         string                          `json:"reason,omitempty"`
	SubjectRefs                    map[string]string               `json:"subject_refs,omitempty"`
	Inbound                        []TextureTurnInboundDisposition `json:"inbound"`
	Controls                       []TextureTurnControl            `json:"controls,omitempty"`
}

// TextureTurnRecord is stored in the lifecycle command receipt. It is the
// durable outcome authority for revision and non-revision turns alike.
type TextureTurnRecord struct {
	Outcome               TextureTurnOutcome `json:"outcome"`
	PriorHeadRevisionID   string             `json:"prior_head_revision_id"`
	HeadRevisionID        string             `json:"head_revision_id"`
	InboundUpdateIDs      []string           `json:"inbound_update_ids,omitempty"`
	ControlUpdateIDs      []string           `json:"control_update_ids,omitempty"`
	TargetWorkItemIDs     []string           `json:"target_work_item_ids,omitempty"`
	CallerWorkItemID      string             `json:"caller_work_item_id"`
	CallerWorkDisposition WorkItemStatus     `json:"caller_work_disposition"`
	OwnerInstructionIDs   []string           `json:"owner_instruction_ids,omitempty"`
	CausalRequestIDs      []string           `json:"causal_request_ids,omitempty"`
	Reason                string             `json:"reason,omitempty"`
}

type LifecycleControlActivationVersion struct {
	UpdateID                string `json:"update_id"`
	TargetWorkItemID        string `json:"target_work_item_id"`
	ControlLifecycleVersion int64  `json:"control_lifecycle_version"`
	WorkLifecycleVersion    int64  `json:"work_lifecycle_version"`
}

type BindLifecycleControlDeliveryItem struct {
	UpdateID                        string `json:"update_id"`
	ProducerAgentID                 string `json:"producer_agent_id"`
	ProducerUpdateID                string `json:"producer_update_id"`
	TargetWorkItemID                string `json:"target_work_item_id"`
	ExpectedControlLifecycleVersion int64  `json:"expected_control_lifecycle_version"`
	ExpectedWorkLifecycleVersion    int64  `json:"expected_work_lifecycle_version"`
}

type LifecycleControlActivationRefresh struct {
	Prompt               string                              `json:"prompt"`
	LogicalActivationKey string                              `json:"logical_activation_key"`
	FailedAttemptKey     string                              `json:"failed_attempt_key"`
	BuildCommit          string                              `json:"build_commit"`
	Versions             []LifecycleControlActivationVersion `json:"versions"`
	WorkItemIDs          []string                            `json:"work_item_ids"`
}

type BindLifecycleControlDeliveryRequest struct {
	OwnerID                  string                             `json:"owner_id"`
	ComputerID               string                             `json:"computer_id"`
	CommandID                string                             `json:"command_id"`
	CommandDigest            string                             `json:"command_digest"`
	TrajectoryID             string                             `json:"trajectory_id"`
	TargetAgentID            string                             `json:"target_agent_id"`
	TargetRunID              string                             `json:"target_run_id"`
	ExpectedLifecycleVersion int64                              `json:"expected_lifecycle_version"`
	Controls                 []BindLifecycleControlDeliveryItem `json:"controls"`
	ActivationRefresh        *LifecycleControlActivationRefresh `json:"activation_refresh,omitempty"`
}

type FailLifecycleControlActivationRequest struct {
	OwnerID                  string                             `json:"owner_id"`
	ComputerID               string                             `json:"computer_id"`
	CommandID                string                             `json:"command_id"`
	CommandDigest            string                             `json:"command_digest"`
	TrajectoryID             string                             `json:"trajectory_id"`
	AgentID                  string                             `json:"agent_id"`
	RunID                    string                             `json:"run_id"`
	ExpectedLifecycleVersion int64                              `json:"expected_lifecycle_version"`
	LogicalActivationKey     string                             `json:"logical_activation_key"`
	FailedAttemptKey         string                             `json:"failed_attempt_key"`
	BindCommandID            string                             `json:"bind_command_id"`
	BindCommandDigest        string                             `json:"bind_command_digest"`
	Controls                 []BindLifecycleControlDeliveryItem `json:"controls"`
	ActivationRefresh        *LifecycleControlActivationRefresh `json:"activation_refresh,omitempty"`
	Failure                  string                             `json:"failure"`
}

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
	OwnerID                   string `json:"owner_id"`
	ComputerID                string `json:"computer_id"`
	CommandID                 string `json:"command_id"`
	CommandDigest             string `json:"command_digest"`
	TrajectoryID              string `json:"trajectory_id"`
	ExpectedLifecycleVersion  int64  `json:"expected_lifecycle_version"`
	RequestedLifecycleVersion int64  `json:"requested_lifecycle_version,omitempty"`
	ExpectedHeadRevisionID    string `json:"expected_head_revision_id"`
	Reason                    string `json:"reason"`
}

type LifecycleCancellationIntent struct {
	OwnerID                   string    `json:"owner_id"`
	ComputerID                string    `json:"computer_id"`
	TrajectoryID              string    `json:"trajectory_id"`
	CommandID                 string    `json:"command_id"`
	CommandDigest             string    `json:"command_digest"`
	RequestedLifecycleVersion int64     `json:"requested_lifecycle_version"`
	ExpectedHeadRevisionID    string    `json:"expected_head_revision_id"`
	Reason                    string    `json:"reason"`
	CreatedAt                 time.Time `json:"created_at"`
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

type CommitLifecycleOwnerCorrection struct {
	RequestID        string `json:"request_id"`
	InstructionID    string `json:"instruction_id"`
	TargetAgentID    string `json:"target_agent_id"`
	TargetWorkItemID string `json:"target_work_item_id"`
	Content          string `json:"content"`
}

type CommitLifecycleArtifactHeadRequest struct {
	OwnerID                  string                          `json:"owner_id"`
	ComputerID               string                          `json:"computer_id"`
	CommandID                string                          `json:"command_id"`
	CommandDigest            string                          `json:"command_digest"`
	TrajectoryID             string                          `json:"trajectory_id"`
	ExpectedLifecycleVersion int64                           `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string                          `json:"expected_head_revision_id"`
	Unbound                  bool                            `json:"unbound,omitempty"`
	Revision                 Revision                        `json:"revision"`
	OwnerCorrection          *CommitLifecycleOwnerCorrection `json:"owner_correction,omitempty"`
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
	Trajectory       TrajectoryRecord           `json:"trajectory"`
	Schema           string                     `json:"schema,omitempty"`
	WorkItem         *WorkItemRecord            `json:"work_item,omitempty"`
	Agent            *AgentRecord               `json:"agent,omitempty"`
	Update           *CoagentSourcePacket       `json:"update,omitempty"`
	OwnerInstruction *LifecycleOwnerInstruction `json:"owner_instruction,omitempty"`
	Events           []LifecycleEvent           `json:"events"`
	Document         *Document                  `json:"document,omitempty"`
	Revision         *Revision                  `json:"revision,omitempty"`
	TextureTurn      *TextureTurnRecord         `json:"texture_turn,omitempty"`
	Controls         []CoagentSourcePacket      `json:"controls,omitempty"`
	TargetWorkItems  []WorkItemRecord           `json:"target_work_items,omitempty"`
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
	Schema               string                              `json:"schema,omitempty"`
	EventID              string                              `json:"event_id"`
	OwnerID              string                              `json:"owner_id"`
	ComputerID           string                              `json:"computer_id"`
	TrajectoryID         string                              `json:"trajectory_id"`
	WorkItemID           string                              `json:"work_item_id,omitempty"`
	UpdateID             string                              `json:"update_id,omitempty"`
	RunID                string                              `json:"run_id,omitempty"`
	AgentID              string                              `json:"agent_id,omitempty"`
	LogicalActivationKey string                              `json:"logical_activation_key,omitempty"`
	FailedAttemptKey     string                              `json:"failed_attempt_key,omitempty"`
	BindCommandID        string                              `json:"bind_command_id,omitempty"`
	BindCommandDigest    string                              `json:"bind_command_digest,omitempty"`
	ControlVersions      []LifecycleControlActivationVersion `json:"control_versions,omitempty"`
	Kind                 LifecycleEventKind                  `json:"kind"`
	ReducerVersion       string                              `json:"reducer_version"`
	ReducerSeq           int64                               `json:"reducer_seq"`
	CommandID            string                              `json:"command_id"`
	CommandDigest        string                              `json:"command_digest"`
	RequestID            string                              `json:"request_id,omitempty"`
	RequestIDs           []string                            `json:"request_ids,omitempty"`
	ArtifactRefs         []string                            `json:"artifact_refs,omitempty"`
	EvidenceRefs         []string                            `json:"evidence_refs,omitempty"`
	Reason               string                              `json:"reason,omitempty"`
	CreatedAt            time.Time                           `json:"created_at"`
}

type LifecycleResult struct {
	Receipt          LifecycleCommandReceipt    `json:"receipt"`
	Trajectory       TrajectoryRecord           `json:"trajectory"`
	Schema           string                     `json:"schema,omitempty"`
	WorkItem         *WorkItemRecord            `json:"work_item,omitempty"`
	Agent            *AgentRecord               `json:"agent,omitempty"`
	Update           *CoagentSourcePacket       `json:"update,omitempty"`
	OwnerInstruction *LifecycleOwnerInstruction `json:"owner_instruction,omitempty"`
	Events           []LifecycleEvent           `json:"events"`
	Replay           bool                       `json:"replay"`
	Document         *Document                  `json:"document,omitempty"`
	Revision         *Revision                  `json:"revision,omitempty"`
	TextureTurn      *TextureTurnRecord         `json:"texture_turn,omitempty"`
	Controls         []CoagentSourcePacket      `json:"controls,omitempty"`
	TargetWorkItems  []WorkItemRecord           `json:"target_work_items,omitempty"`
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
	CoSuperAssignments  []CoSuperAssignment           `json:"co_super_assignments,omitempty"`
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
