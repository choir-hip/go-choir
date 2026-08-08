package types

import "time"

const LifecycleOwnerInstructionSchemaV1 = "choir.lifecycle_owner_instruction.v1"

type LifecycleOwnerInstructionKind string

const (
	LifecycleOwnerTell    LifecycleOwnerInstructionKind = "tell"
	LifecycleOwnerCorrect LifecycleOwnerInstructionKind = "correct"
)

type LifecycleOwnerInstructionStatus string

const (
	LifecycleOwnerInstructionPending  LifecycleOwnerInstructionStatus = "pending"
	LifecycleOwnerInstructionConsumed LifecycleOwnerInstructionStatus = "consumed"
)

type QueueLifecycleOwnerInstructionRequest struct {
	OwnerID                  string                        `json:"owner_id"`
	ComputerID               string                        `json:"computer_id"`
	CommandID                string                        `json:"command_id"`
	CommandDigest            string                        `json:"command_digest"`
	RequestID                string                        `json:"request_id"`
	InstructionID            string                        `json:"instruction_id"`
	DocumentID               string                        `json:"document_id"`
	TrajectoryID             string                        `json:"trajectory_id"`
	TargetAgentID            string                        `json:"target_agent_id"`
	TargetWorkItemID         string                        `json:"target_work_item_id"`
	ExpectedLifecycleVersion int64                         `json:"expected_lifecycle_version"`
	ExpectedHeadRevisionID   string                        `json:"expected_head_revision_id"`
	Kind                     LifecycleOwnerInstructionKind `json:"kind"`
	Content                  string                        `json:"content"`
}

type LifecycleOwnerInstruction struct {
	Schema           string                          `json:"schema"`
	InstructionID    string                          `json:"instruction_id"`
	RequestID        string                          `json:"request_id"`
	OwnerID          string                          `json:"owner_id"`
	ComputerID       string                          `json:"computer_id"`
	DocumentID       string                          `json:"document_id"`
	TrajectoryID     string                          `json:"trajectory_id"`
	TargetAgentID    string                          `json:"target_agent_id"`
	TargetWorkItemID string                          `json:"target_work_item_id"`
	HeadRevisionID   string                          `json:"head_revision_id"`
	Kind             LifecycleOwnerInstructionKind   `json:"kind"`
	Content          string                          `json:"content"`
	Status           LifecycleOwnerInstructionStatus `json:"status"`
	LifecycleVersion int64                           `json:"lifecycle_version"`
	ReducerSeq       int64                           `json:"reducer_seq"`
	CreatedAt        time.Time                       `json:"created_at"`
	ConsumedAt       *time.Time                      `json:"consumed_at,omitempty"`
}

type TextureTurnOwnerInstruction struct {
	InstructionID string `json:"instruction_id"`
	RequestID     string `json:"request_id"`
}
