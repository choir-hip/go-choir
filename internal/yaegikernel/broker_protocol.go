package yaegikernel

import (
	"encoding/json"
	"fmt"
	"time"
)

const ProtocolVersion = 1

// BrokerAction defines the operation verb requested by an interpreted Go activation.
type BrokerAction string

const (
	ActionExec      BrokerAction = "exec"
	ActionReadFile  BrokerAction = "read_file"
	ActionWriteFile BrokerAction = "write_file"
	ActionListDir   BrokerAction = "list_dir"
	ActionAssign    BrokerAction = "assign"
	ActionMessage   BrokerAction = "message"
)

// BrokerRequest is the flat DTO sent from an untrusted Yaegi activation to the broker.
type BrokerRequest struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	HandleRef       string          `json:"handle_ref"`
	Epoch           uint64          `json:"epoch"`
	Action          BrokerAction    `json:"action"`
	Payload         json.RawMessage `json:"payload"`
	TimeoutMs       int64           `json:"timeout_ms,omitempty"`
}

// BrokerResponse is the flat DTO returned from the broker to the Yaegi activation.
type BrokerResponse struct {
	ProtocolVersion int             `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Success         bool            `json:"success"`
	Result          json.RawMessage `json:"result,omitempty"`
	Error           string          `json:"error,omitempty"`
	ReceiptID       string          `json:"receipt_id,omitempty"`
	DurationMs      int64           `json:"duration_ms"`
}

// ExecPayload defines the parameters for ActionExec.
type ExecPayload struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ExecResult defines the result returned for ActionExec.
type ExecResult struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
}

// ReadFilePayload defines the parameters for ActionReadFile.
type ReadFilePayload struct {
	Path string `json:"path"`
}

// ReadFileResult defines the result for ActionReadFile.
type ReadFileResult struct {
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// WriteFilePayload defines the parameters for ActionWriteFile.
type WriteFilePayload struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Mode    uint32 `json:"mode,omitempty"`
}

// WriteFileResult defines the result for ActionWriteFile.
type WriteFileResult struct {
	BytesWritten int   `json:"bytes_written"`
	ModTime      int64 `json:"mod_time"`
}

// ListDirPayload defines the parameters for ActionListDir.
type ListDirPayload struct {
	Path string `json:"path"`
}

// ListDirResult defines the result for ActionListDir.
type ListDirResult struct {
	Entries []string `json:"entries"`
}

// AssignPayload defines the parameters for ActionAssign (spawning a subagent/worker).
type AssignPayload struct {
	TaskID       string `json:"task_id"`
	ActorProfile string `json:"actor_profile"`
	Instruction  string `json:"instruction"`
}

// AssignResult defines the result for ActionAssign.
type AssignResult struct {
	AssignmentID string `json:"assignment_id"`
	Status       string `json:"status"`
}

// MessagePayload defines the parameters for ActionMessage (inter-agent typed message).
type MessagePayload struct {
	RecipientID string `json:"recipient_id"`
	Kind        string `json:"kind"`
	Body        string `json:"body"`
}

// MessageResult defines the result for ActionMessage.
type MessageResult struct {
	MessageID   string `json:"message_id"`
	DeliveredAt string `json:"delivered_at"`
}

// Validate checks internal consistency of a BrokerRequest.
func (r *BrokerRequest) Validate() error {
	if r.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("broker protocol: unsupported protocol version %d", r.ProtocolVersion)
	}
	if r.RequestID == "" {
		return fmt.Errorf("broker protocol: request_id is required")
	}
	if r.HandleRef == "" {
		return fmt.Errorf("broker protocol: handle_ref is required")
	}
	if r.Epoch == 0 {
		return fmt.Errorf("broker protocol: epoch must be positive")
	}
	if r.Action == "" {
		return fmt.Errorf("broker protocol: action is required")
	}
	return nil
}

// NewSuccessResponse creates a successful response.
func NewSuccessResponse(reqID string, result any, receiptID string, duration time.Duration) (*BrokerResponse, error) {
	var raw json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal result: %w", err)
		}
		raw = b
	}
	return &BrokerResponse{
		ProtocolVersion: ProtocolVersion,
		RequestID:       reqID,
		Success:         true,
		Result:          raw,
		ReceiptID:       receiptID,
		DurationMs:      duration.Milliseconds(),
	}, nil
}

// NewErrorResponse creates an error response.
func NewErrorResponse(reqID, errMsg string, duration time.Duration) *BrokerResponse {
	return &BrokerResponse{
		ProtocolVersion: ProtocolVersion,
		RequestID:       reqID,
		Success:         false,
		Error:           errMsg,
		DurationMs:      duration.Milliseconds(),
	}
}
