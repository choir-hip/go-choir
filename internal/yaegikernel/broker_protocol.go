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
	ActionExec                 BrokerAction = "exec"
	ActionReadFile             BrokerAction = "read_file"
	ActionWriteFile            BrokerAction = "write_file"
	ActionListDir              BrokerAction = "list_dir"
	ActionAssign               BrokerAction = "assign"
	ActionMessage              BrokerAction = "message"
	ActionEmit                 BrokerAction = "emit"
	ActionWebSearch            BrokerAction = "web_search"
	ActionFetchURL             BrokerAction = "fetch_url"
	ActionSourceSearch         BrokerAction = "source_search"
	ActionImportDocument       BrokerAction = "import_document_content"
	ActionImportURL            BrokerAction = "import_url_content"
	ActionReadContentItem      BrokerAction = "read_content_item"
	ActionListContentSelectors BrokerAction = "list_content_item_selectors"
	ActionReadContentSelector  BrokerAction = "read_content_item_selector"
	ActionSearchWireCorpus     BrokerAction = "search_wire_corpus"
	ActionSaveEvidence         BrokerAction = "save_evidence"
	ActionReadEvidence         BrokerAction = "read_evidence"
	ActionListEvidence         BrokerAction = "list_evidence"
	ActionGetRunMemoryEntry    BrokerAction = "get_run_memory_entry"
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

// EmitPayload defines the parameters for ActionEmit. Emits bypass the
// cell tray: the host durably mails an rlm "emit" envelope immediately.
type EmitPayload struct {
	ToDesk string `json:"to_desk"`
	Kind   string `json:"kind"`
	Body   string `json:"body"`
}

// EmitResult identifies the durable channel record written by ActionEmit.
type EmitResult struct {
	Seq uint64 `json:"seq"`
	// AdvisorySeq is the advisory piggyback on the s.call response: the newest
	// channel seq of emissions addressed to the *calling* desk at emit time.
	// A cell that already consumed up to its cell-start watermark sees
	// AdvisorySeq > that mark when a new emission landed mid-activation — so
	// delivery does not have to wait for the next injectUserTurns boundary.
	// Zero means no advisory available (non-desk caller, drain read failed).
	AdvisorySeq uint64 `json:"advisory_seq,omitempty"`
}

// WebSearchPayload defines the parameters for ActionWebSearch — a host-
// resolved web search so the cell sees a result, not an open egress socket.
type WebSearchPayload struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// FetchURLPayload defines the parameters for ActionFetchURL — a host-
// resolved fetch; the cell receives bounded content, never a socket.
type FetchURLPayload struct {
	URL string `json:"url"`
}

// The remaining research cell verbs. Each action name equals its registry
// tool name so host egress dispatches into the same deps-bound ToolFunc —
// one source of truth for charging and projection. Results return as opaque
// JSON (the tool's bounded projection) into the cell.
type SourceSearchPayload struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}
type ImportDocumentPayload struct {
	URL      string `json:"url,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Query    string `json:"query,omitempty"`
}
type ImportURLPayload struct {
	URL   string `json:"url"`
	Query string `json:"query,omitempty"`
}
type ReadContentItemPayload struct {
	ContentID    string `json:"content_id"`
	MaxTextChars int    `json:"max_text_chars,omitempty"`
	MaxSegments  int    `json:"max_segments,omitempty"`
}
type ListContentSelectorsPayload struct {
	ContentID string `json:"content_id"`
}
type ReadContentSelectorPayload struct {
	ContentID    string `json:"content_id"`
	SelectorID   string `json:"selector_id"`
	MaxTextChars int    `json:"max_text_chars,omitempty"`
}
type SearchWireCorpusPayload struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// Evidence + run-memory cell verbs. These resolve through rt-bound tools on
// the host (agentcore), not the researchtools deps table — the action name
// equals the tool name there too.
type SaveEvidencePayload struct {
	Kind      string          `json:"kind"`
	SourceURI string          `json:"source_uri,omitempty"`
	Title     string          `json:"title,omitempty"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}
type ReadEvidencePayload struct {
	EvidenceID string `json:"evidence_id"`
}
type ListEvidencePayload struct {
	AgentID string `json:"agent_id,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
type GetRunMemoryEntryPayload struct {
	EntryID string `json:"entry_id"`
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
	switch r.Action {
	case ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionAssign, ActionMessage, ActionEmit,
		ActionWebSearch, ActionFetchURL, ActionSourceSearch, ActionImportDocument, ActionImportURL,
		ActionReadContentItem, ActionListContentSelectors, ActionReadContentSelector, ActionSearchWireCorpus,
		ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry:
		return nil
	default:
		return fmt.Errorf("broker protocol: unsupported action %q", r.Action)
	}
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
