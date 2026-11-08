package yaegikernel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const ProtocolVersion = 2

// BrokerAction defines the operation verb requested by an interpreted Go activation.
type BrokerAction string

const (
	ActionExec      BrokerAction = "exec"
	ActionReadFile  BrokerAction = "read_file"
	ActionWriteFile BrokerAction = "write_file"
	ActionListDir   BrokerAction = "list_dir"
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
	// ActionTextureSearch is the Texture custodian's guarded corpus search,
	// served only under a texture.search lease: the research desk's Texture
	// child's choir.SearchTexture (plan MA4.6, section 8.6).
	ActionTextureSearch BrokerAction = "texture_search"
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
	// ActionShotStart starts the before/after shots of an accepted operation
	// that owes shots; ActionShotWait waits on that job. Management's
	// choir.ShotStart and choir.ShotWait (desk-shots-and-review §3.3).
	ActionShotStart BrokerAction = "shot_start"
	ActionShotWait  BrokerAction = "shot_wait"
	// ActionInstallPacket reads the packet of an install review management
	// owes: choir.InstallPacket (desk-install-review-2026-11-08 2.3).
	ActionInstallPacket BrokerAction = "install_packet"
	// ActionScreens queues a frozen bundle's screens for management's model:
	// choir.Screens.
	ActionScreens BrokerAction = "screens"
	// ActionSystem1 asks the decision judge closed questions for the research
	// desk's choir.System1 (system1.go).
	ActionSystem1 BrokerAction = "system1"
	// ActionRelateStories records the research desk's judged pair relations
	// for choir.RelateStories (relate_stories.go).
	ActionRelateStories BrokerAction = "relate_stories"
	// ActionRelate records the research desk's entity attachments of a
	// published story for choir.Relate (relate.go).
	ActionRelate BrokerAction = "relate"
	// ActionRequestLease asks the lease service for a lease on kinds or
	// captures an app desk does not keep: choir.RequestLease (MA3.4). The host
	// sets the holder desk and run from the session, never from the cell.
	ActionRequestLease BrokerAction = "request_lease"
	// ActionObjectGet and ActionObjectList read a package desk's own objects:
	// choir.Get and choir.List (MA3.8). The host takes the reading desk from
	// the session, never from the cell.
	ActionObjectGet  BrokerAction = "object_get"
	ActionObjectList BrokerAction = "object_list"
	// ActionReadMail reads the one email a mail.read lease names: choir.ReadMail
	// (modular architecture drill P4). The host takes the reading desk and run
	// from the session and serves the ref only when the run holds the lease.
	ActionReadMail BrokerAction = "read_mail"
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

// ResearchPayload defines the parameters for ActionResearch: the child
// research activations to run, each with its objective and an optional
// model from the catalog (empty means the caller's own model).
type ResearchPayload struct {
	Children []ResearchChild `json:"children"`
}

// ResearchChild is one child research activation's spec.
type ResearchChild struct {
	Objective string `json:"objective"`
	// Role is the child's role (plan MA4.6, section 8.6): ResearchChildWeb
	// (the default when empty) or ResearchChildTexture. Any other value is
	// refused, never run as a web child.
	Role  string `json:"role,omitempty"`
	Model string `json:"model,omitempty"`
	// Questions are the parent's precommitment about this child's result
	// (MA1.5): typed questions with the parent's own probabilities.
	// PSuccess and Weight are the parent's stated chance the child achieves
	// its objective and how much it matters; neither is ever supplied by the
	// host. A child without them is a Cast marked weight zero.
	Questions []types.TypedQuestion `json:"questions,omitempty"`
	PSuccess  *float64              `json:"p_success,omitempty"`
	Weight    *float64              `json:"weight,omitempty"`
}

// The roles a ResearchChild names.
const (
	ResearchChildWeb     = "web"
	ResearchChildTexture = "texture"
)

// ResearchChildResult is what one child returns to the calling cell.
type ResearchChildResult struct {
	Objective string `json:"objective"`
	// Role is the session role the child ran as (research_web or
	// research_texture).
	Role       string `json:"role,omitempty"`
	Model      string `json:"model"`
	Findings   string `json:"findings,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	// CastID and ReportID name the child's records (MA1.5): the Cast the
	// call is, and the Report whose text Findings repeats. WeightZero is why
	// the Cast states no weight, when it does not.
	CastID     string `json:"cast_id,omitempty"`
	ReportID   string `json:"report_id,omitempty"`
	WeightZero string `json:"weight_zero,omitempty"`
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

// ShotStartPayload defines ActionShotStart: the operation whose plan owes
// shots.
type ShotStartPayload struct {
	OperationID string `json:"operation_id"`
}

// ShotStartResult is the started (or already live or finished) shot job.
type ShotStartResult struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
}

// ShotScreen is one shot of a job: its name, content digest and size.
type ShotScreen struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ScreensPayload defines ActionScreens.
type ScreensPayload struct {
	OperationID string `json:"operation_id"`
}

// ScreensResult is the frozen bundle's screens; ImagesQueued says they are
// queued into the run's next turn.
type ScreensResult struct {
	OperationID  string       `json:"operation_id"`
	State        string       `json:"state"`
	Screens      []ShotScreen `json:"screens,omitempty"`
	ImagesQueued bool         `json:"images_queued,omitempty"`
}

// ShotWaitPayload defines ActionShotWait: wait up to MaxSeconds (at most
// 240) on a shot job.
type ShotWaitPayload struct {
	JobID      string `json:"job_id"`
	MaxSeconds int    `json:"max_seconds"`
}

// ShotWaitResult is a shot job's state: running, done or failed (with the
// cause). Screens and RecordedSize are set when done; ImagesQueued says the
// job's images are queued into the run's next turn.
type ShotWaitResult struct {
	JobID        string       `json:"job_id"`
	State        string       `json:"state"`
	Cause        string       `json:"cause,omitempty"`
	RecordedSize string       `json:"recorded_size,omitempty"`
	Screens      []ShotScreen `json:"screens,omitempty"`
	ImagesQueued bool         `json:"images_queued,omitempty"`
}

// ShotFinding is management's finding on one screen pair: the pair, both
// digests and the answers (q1..q6: yes, no or n/a) to the overlay's six
// questions.
type ShotFinding struct {
	Pair         string            `json:"pair"`
	BeforeSHA256 string            `json:"before_sha256"`
	AfterSHA256  string            `json:"after_sha256"`
	Answers      map[string]string `json:"answers"`
	Note         string            `json:"note,omitempty"`
}

// InstallPacketPayload defines ActionInstallPacket: the release whose install
// review management owes.
type InstallPacketPayload struct {
	ReleaseID string `json:"release_id"`
}

// InstallPacketResult is what management reads to judge a widened-ceiling
// install: the subject, the risk, the ceiling entries the install adds, the
// flow sentences the ceiling opens, the mechanical checks, the manifest and the
// built module's source (truncated at 120 KiB, Truncated says so).
type InstallPacketResult struct {
	// OwedRecord is the id of the owed review this packet is of. A cell echoes
	// it to InstallVerdict, which refuses a verdict whose review has since
	// changed (a newer install of the release opened another).
	OwedRecord   string   `json:"owed_record"`
	ReleaseID    string   `json:"release_id"`
	Package      string   `json:"package"`
	Publisher    string   `json:"publisher"`
	Path         string   `json:"path"`
	PatchSHA256  string   `json:"patch_sha256"`
	ManifestSHA  string   `json:"manifest_sha256"`
	Surface      string   `json:"behaviour_surface"`
	Risk         string   `json:"risk"`
	Strong       bool     `json:"publisher_strong"`
	Added        []string `json:"added,omitempty"`
	Flow         []string `json:"flow,omitempty"`
	Checks       []string `json:"checks,omitempty"`
	ManifestJSON string   `json:"manifest_json"`
	Source       string   `json:"source"`
	Truncated    bool     `json:"truncated,omitempty"`
}

// InstallFinding is one concern of management's install review. A cell passes
// findings as plain maps (keys concern, evidence, blocking), never this type.
type InstallFinding struct {
	Concern  string `json:"concern"`
	Evidence string `json:"evidence,omitempty"`
	Blocking bool   `json:"blocking,omitempty"`
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

// RequestLeasePayload defines ActionRequestLease (MA3.4): the scope asked for
// (object kinds, capture ids, owner-private capture labels), a stated purpose,
// an optional duration beyond which the lease ends sooner than its run, and
// the request fields (p_success, weight, settlement) that make the ask a
// scored precommitment. The holder desk and run are not here: the host takes
// them from the session.
//
// Grants and Refs ask a first-party custodian's guarded service instead of a
// kind or a capture: Grants names the grant (mail.read) and Refs the one ref
// it is asked for.
type RequestLeasePayload struct {
	Grants          []string             `json:"grants,omitempty"`
	Refs            []string             `json:"refs,omitempty"`
	Kinds           []string             `json:"kinds,omitempty"`
	CaptureIDs      []string             `json:"capture_ids,omitempty"`
	CaptureLabels   []string             `json:"capture_labels,omitempty"`
	Purpose         string               `json:"purpose"`
	DurationSeconds int                  `json:"duration_seconds,omitempty"`
	Req             *types.RequestFields `json:"req"`
}

// ReadMailPayload defines ActionReadMail: the ref of the one email the run's
// mail.read lease names.
type ReadMailPayload struct {
	Ref string `json:"ref"`
}

// ObjectGetPayload defines ActionObjectGet: one object of the desk's package
// by kind and id (MA3.8).
type ObjectGetPayload struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// ObjectListPayload defines ActionObjectList: the desk's live objects of one of
// its package kinds, newest first (MA3.8). Limit 0 means the default page.
type ObjectListPayload struct {
	Kind  string `json:"kind"`
	Limit int    `json:"limit,omitempty"`
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

// TextureSearchPayload defines ActionTextureSearch: the query and how many
// hits (the custodian clamps it).
type TextureSearchPayload struct {
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
	case ActionExec, ActionReadFile, ActionWriteFile, ActionListDir,
		ActionSearchStart, ActionRead, ActionSearchWireCorpus, ActionTextureSearch,
		ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry,
		ActionProductAPI, ActionResearch, ActionPollSource, ActionReadCapture, ActionStories, ActionShotStart, ActionShotWait, ActionScreens, ActionInstallPacket, ActionSystem1, ActionRelateStories, ActionRelate, ActionSourceAPI, ActionRequestLease, ActionObjectGet, ActionObjectList, ActionReadMail:
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
