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
	ActionMessage   BrokerAction = "message"
	ActionEmit      BrokerAction = "emit"
	// ActionSearchStart starts an async search on the host; its arrivals come
	// back on StreamSearchArrival frames (search_store.go).
	ActionSearchStart BrokerAction = "search_start"
	// ActionRead fetches a page or PDF into a shared capture (Cut 2 unit
	// 2a); it replaced fetch_url, import_url_content,
	// import_document_content and the ContentItem selector reads.
	ActionRead             BrokerAction = "read"
	ActionPollSource       BrokerAction = "poll_source"
	ActionSourceAPI        BrokerAction = "source_api"
	ActionReadCapture      BrokerAction = "read_capture"
	ActionSearchWireCorpus BrokerAction = "search_wire_corpus"
	// ActionResearch runs child research activations on the host and
	// returns their findings to the calling cell (RLM actor recursion).
	ActionResearch          BrokerAction = "research"
	ActionSaveEvidence      BrokerAction = "save_evidence"
	ActionReadEvidence      BrokerAction = "read_evidence"
	ActionListEvidence      BrokerAction = "list_evidence"
	ActionGetRunMemoryEntry BrokerAction = "get_run_memory_entry"
	ActionProductAPI        BrokerAction = "product_api"
	// ActionStories pages the computer's story library, newest first:
	// management's choir.Stories (L2a A2).
	ActionStories BrokerAction = "stories"
	// ActionSystem1 asks the decision judge closed questions for the research
	// desk's choir.System1 (system1.go).
	ActionSystem1 BrokerAction = "system1"
	// ActionRelateStories records the research desk's judged pair relations
	// for choir.RelateStories (relate_stories.go).
	ActionRelateStories BrokerAction = "relate_stories"
	// ActionRelate records the research desk's entity attachments of a
	// published story for choir.Relate (relate.go).
	ActionRelate BrokerAction = "relate"
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
	// SlotToken is the per-worker token the host generated at spawn
	// (ARCH-07). A host-bound frame carries it so the daemon can tell the
	// worker it spawned from a stray or replayed connection.
	SlotToken string `json:"slot_token,omitempty"`
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

// ResearchPayload defines the parameters for ActionResearch: the child
// research activations to run, each with its objective and an optional
// model from the catalog (empty means the caller's own model).
type ResearchPayload struct {
	Children []ResearchChild `json:"children"`
}

// ResearchChild is one child research activation's spec.
type ResearchChild struct {
	Objective string `json:"objective"`
	Model     string `json:"model,omitempty"`
}

// ResearchChildResult is what one child returns to the calling cell.
type ResearchChildResult struct {
	Objective  string `json:"objective"`
	Model      string `json:"model"`
	Findings   string `json:"findings,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	// What the child's model loop spent, tracked and never capped
	// (ARCH-08 slice 1, docs/design/agent-tree-budgets-2026-10-21.md).
	InputTokens       int `json:"input_tokens,omitempty"`
	OutputTokens      int `json:"output_tokens,omitempty"`
	CachedInputTokens int `json:"cached_input_tokens,omitempty"`
}

// ProductAPIPayload defines the parameters for ActionProductAPI — the
// management desk's host-resolved product API call (former
// product_api_request typed tool). The host validates method/route against
// the product-path allowlist and serves the request with the run owner as
// the authenticated user; the cell sees the bounded status/body result.
type ProductAPIPayload struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// StoriesPayload defines ActionStories: a page of at most Limit stories
// updated before Before (RFC 3339; empty for the newest).
type StoriesPayload struct {
	Limit  int    `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
}

// StoryEntry is one story of the library: its inbox (feed it there), its
// headline, the first paragraph of its current version, when it last
// changed and its version number.
type StoryEntry struct {
	DocID     string `json:"doc_id"`
	Inbox     string `json:"inbox"`
	Headline  string `json:"headline"`
	Lede      string `json:"lede,omitempty"`
	UpdatedAt string `json:"updated_at"`
	Version   int    `json:"version"`
}

// StoriesPage is one page of stories, newest first. Next, when set, is the
// Before of the next page.
type StoriesPage struct {
	Stories []StoryEntry `json:"stories"`
	Next    string       `json:"next,omitempty"`
	// Held counts, on the newest page, the captures waiting on their
	// events, as the last sweep found them (event layer §15): they are in
	// no record you read until their event is corroborated.
	Held *HeldCaptures `json:"held_captures,omitempty"`
}

// HeldCaptures is the last held-capture sweep's count.
type HeldCaptures struct {
	Waiting  int    `json:"waiting"`  // with an event that is not corroborated
	Released int    `json:"released"` // sent to you unresolved by that sweep
	SweptAt  string `json:"swept_at"`
}

// String prints the page as JSON, so a cell that prints it reads it.
func (p StoriesPage) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		return fmt.Sprintf("{stories:%d next:%s}", len(p.Stories), p.Next)
	}
	return string(b)
}

// ReadPayload defines ActionRead: a host-resolved fetch into a shared
// capture; the cell receives the capture id and bounded blocks, never a
// socket.
type ReadPayload struct {
	URL string `json:"url"`
}

// PollSourcePayload defines ActionPollSource: poll one registry source into
// web_capture objects (Autopaper W1b).
type PollSourcePayload struct {
	SourceID string `json:"source_id"`
	MaxItems int    `json:"max_items,omitempty"`
}

// SourceAPIPayload defines ActionSourceAPI: one named endpoint of one keyed
// source API through the host gateway, which adds the key
// (docs/problems/keyed-source-apis-2026-10-20.md).
type SourceAPIPayload struct {
	Provider string         `json:"provider"`
	Endpoint string         `json:"endpoint"`
	Params   map[string]any `json:"params,omitempty"`
}

// ReadCapturePayload defines ActionReadCapture: read one shared capture's
// blocks from FromBlock (Autopaper W1; Cut 2 unit 2a).
type ReadCapturePayload struct {
	CaptureID string `json:"capture_id"`
	FromBlock int    `json:"from_block,omitempty"`
	MaxChars  int    `json:"max_chars,omitempty"`
}

// The remaining research cell verbs. Each action name equals its registry
// tool name so host egress dispatches into the same deps-bound ToolFunc —
// one source of truth for charging and projection. Results return as opaque
// JSON (the tool's bounded projection) into the cell.
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
	case ActionExec, ActionReadFile, ActionWriteFile, ActionListDir, ActionMessage, ActionEmit,
		ActionSearchStart, ActionRead, ActionSearchWireCorpus,
		ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry,
		ActionProductAPI, ActionResearch, ActionPollSource, ActionReadCapture, ActionStories, ActionSystem1, ActionRelateStories, ActionRelate, ActionSourceAPI:
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
