package capsule

import (
	"os"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// ReleaseTreePrefix is where a freeze reads the runtime release a capsule
// built: upperdir-relative, so /var/lib/artifact/release/ at the capsule
// root, never under /workspace/platform (that is source). The engineering
// overlay teaches this path; runtimeprompts tests pin it to this constant.
const ReleaseTreePrefix = "var/lib/artifact/release/"

// SpawnSpec describes the desired state of a new capsule.
type SpawnSpec struct {
	CapsuleID string // unique capsule identifier (UUID)
	MemoryMax int64  // total budget: RSS + tmpfs + kmem (bytes)
	// MemoryRequest is doctrine's request (RES-04, capsule_memory.go): under
	// the doctrine policy memory.high = request, memory.max = 2 x request,
	// admission against the request. 0 keeps the MemoryMax shape.
	MemoryRequest int64
	CpuQuota      int64        // CPU quota (microseconds per period)
	CpuPeriod     int64        // CPU period (default 100000)
	PidsMax       int64        // max processes in capsule
	DiskMax       int64        // tmpfs sub-budget of MemoryMax (bytes)
	Env           []string     // environment variables for broker/workload
	WorkingDir    string       // initial working directory
	OwnerRunID    string       // agent run that owns this capsule
	Tier          ResourceTier // resource tier preset

	// SourceArtifactRef and ExpectedSubjectDigest are trusted-runtime inputs.
	// Spawn materializes exactly this preflighted content-addressed subject and
	// refuses any digest mismatch, closing the durable-open/spawn race.
	SourceArtifactRef     string
	ExpectedSubjectDigest string

	// VerifierBundleDir is a trusted-runtime input: the host-side directory of
	// the exact frozen self-development bundle to install read-only at
	// /selfdev/bundle for a verifier-slot activation. VerifierBinding carries
	// the durable operation identity written to binding.json beside it.
	VerifierBundleDir string
	VerifierBinding   string

	// AdoptionDir is a trusted-runtime input: the host-side directory of a
	// verified release adoption (release.json and source.patch, re-verified
	// by the runtime just before spawn) installed read-only at
	// /selfdev/adoption for the engineering assignment on the adoption's
	// trajectory (P8, docs/archive/reports/p6-p8-release-adopt-2026-10-15.md).
	AdoptionDir string

	// CarriedPatchDir is a trusted-runtime input: the host-side directory of
	// a failed earlier round's carried source patch (source.patch and
	// carried.json, staged content-addressed and re-verified by the runtime
	// just before spawn) installed read-only at /selfdev/carried for the
	// retry's engineering assignment. The capsule's tree stays the clean
	// base; the engineer chooses whether to apply the patch inside it
	// (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
	// failure 15, problem 1, option A).
	CarriedPatchDir string

	// Egress makes this an ecosystem_proxy (L2) capsule: the executor serves
	// an egress proxy on EgressSocketPath and the broker forwards loopback
	// TCP to it. It is a trusted-runtime input read from the durable
	// assignment binding, never from the capsule
	// (docs/design/engineering-network-grants-2026-10-10.md §4.0b).
	Egress bool

	// ClaudeDelegate makes this a claude-mode capsule (requires Egress): the
	// model-API-only egress policy, and the Claude delegate token handed to
	// the broker. Trusted-runtime input from the durable binding; the spawn
	// fails closed when the delegate is unavailable
	// (docs/design/claude-delegate-slice1-2026-10-14.md).
	ClaudeDelegate bool
}

type SourcePreflight struct {
	SubjectDigest string `json:"subject_digest"`
	ArtifactRef   string `json:"artifact_ref"`
}

// ResourceTier is a preset for resource limits.
type ResourceTier string

const (
	TierSmall  ResourceTier = "small"  // 256MB, 0.5 CPU, 128 pids
	TierMedium ResourceTier = "medium" // 1GB, 1 CPU, 256 pids
	TierLarge  ResourceTier = "large"  // 4GB, 2 CPU, 512 pids
)

// CapsuleState tracks the lifecycle state of a capsule.
type CapsuleState int

const (
	StateSpawning CapsuleState = iota
	StateActive
	StateQuiescing
	StateFrozen
	StateDestroying
	StateDestroyed
)

func (s CapsuleState) String() string {
	switch s {
	case StateSpawning:
		return "spawning"
	case StateActive:
		return "active"
	case StateQuiescing:
		return "quiescing"
	case StateFrozen:
		return "frozen"
	case StateDestroying:
		return "destroying"
	case StateDestroyed:
		return "destroyed"
	default:
		return "unknown"
	}
}

// ExecRequest is a request to execute a command in a capsule.
// Direct-argv is canonical: when Args is non-empty, Command names the binary
// and Args carries the argument vector with no shell involved. An empty Args
// selects the frozen legacy shell path (sh -c), reachable only for rollback
// of legacy JSON tools and never in RLM mode.
type ExecRequest struct {
	SessionID string   // broker-minted random ID (NOT agentRunID)
	Command   string   // binary path (direct-argv) or shell string (legacy)
	Args      []string // direct-argv vector; empty = legacy shell
	Cwd       string   // working directory for command
	Env       []string // environment overrides (allowlisted, never credentials)
	Stdin     string   // stdin content (empty for no input)
	TimeoutMS int      // timeout in milliseconds (0 = broker default)
	PTY       bool     // use PTY for command
}

// ExecResult is the result of executing a command in a capsule.
type ExecResult struct {
	ExitCode   int           // process exit code
	SessionID  string        // session ID (returned when broker creates new session)
	Stdout     string        // stdout content
	Stderr     string        // stderr content
	Duration   time.Duration // execution duration
	ReceiptRef string        `json:"receipt_ref,omitempty"`
}

// GoEvalRequest is a request to evaluate model-authored Go source inside the
// capsule's restricted Yaegi interpreter. All package access is constrained by
// the kernel allowlist; the interpreted process runs as a killable child of the
// broker, never in guest core. Inbox carries the cell-start snapshot assembled
// by guest core from the durable mailbox; the worker injects it into the frame
// and the cell reads it through choir.Inbox() without network roundtrips.
type GoEvalRequest struct {
	Source          string                        `json:"source"`           // Go source to evaluate
	Cwd             string                        `json:"cwd"`              // working directory (optional)
	AllowedPackages []string                      `json:"allowed_packages"` // kernel allowlist override (optional)
	TimeoutMS       int                           `json:"timeout_ms"`       // timeout in milliseconds (0 = broker default)
	Inbox           []yaegikernel.IncomingMessage `json:"inbox,omitempty"`
	// Updates carries the pending update_coagent records bound for this
	// cell (RLM prompt-as-variable): the cell reads them via
	// choir.Updates(); the wake turn in chat carries only their ids.
	Updates []yaegikernel.PendingUpdate `json:"updates,omitempty"`
	// Records carries the records addressed to the engineering desk's inbox
	// in (cursor, head] at cell start, read through choir.Records() (Cut 1
	// L3b-2).
	Records []yaegikernel.InboxRecord `json:"records,omitempty"`
	// Jobs carries the run's capsule jobs at cell start, read through
	// choir.Poll; the executor fills it (jobs.go).
	Jobs []yaegikernel.JobView `json:"jobs,omitempty"`
}

// GoEvalResult is the result of evaluating Go source in the capsule. Intents
// carries the cell's staged tray for post-cell reduction by guest core; it is
// populated only for successful cells (failed cells drop their tray and never
// advance the inbox cursor).
type GoEvalResult struct {
	Stdout          string                     `json:"stdout"`
	Stderr          string                     `json:"stderr"`
	Error           string                     `json:"error,omitempty"`
	Duration        time.Duration              `json:"duration,omitempty"`
	ExitCode        int                        `json:"exit_code"`
	ReceiptRef      string                     `json:"receipt_ref,omitempty"`
	Fallback        bool                       `json:"fallback,omitempty"`
	StagedIntentIDs []string                   `json:"staged_intent_ids,omitempty"`
	Intents         []yaegikernel.StagedIntent `json:"intents,omitempty"`
	// FateTerminal is set by guest core when this cell's reduction committed a
	// terminal assignment fate; the run loop ends on it.
	FateTerminal bool `json:"fate_terminal,omitempty"`
	// RecordsShown is the highest record seq a choir.Records() call in the
	// cell showed (C1e B1): the cell marks read only what it was shown.
	RecordsShown int64 `json:"records_shown,omitempty"`
	// JobsAwaited are the jobs a choir.Poll in the cell returned unfinished
	// (the kernel's SessionResult.JobsAwaited, carried by the broker).
	JobsAwaited []string `json:"jobs_awaited,omitempty"`
	// JobPark is the job this turn waits on, set by guest core after
	// reduction (agentcore cellJobPark): the tool loop calls no model until
	// the job ends, then resumes the turn with its outcome.
	JobPark string `json:"job_park,omitempty"`
	// FreezeResult/VerifyResult carry the committed freeze/verify receipts for
	// this cell so the model sees the same result the retired JSON tools
	// returned.
	FreezeResult map[string]any `json:"freeze_result,omitempty"`
	VerifyResult map[string]any `json:"verify_result,omitempty"`
	// JobResults carries what became of the cell's job starts and stops
	// (started, or refused and why), set by guest core after reduction.
	JobResults []map[string]any `json:"job_results,omitempty"`
	// Reuse carries the session reuse disposition for a failed cell
	// (preserve: heap intact, worker alive). DiagKind carries the
	// diagnostic kind with the message verbatim. Settlement-gate item 1.
	Reuse    yaegikernel.ReuseDisposition `json:"reuse,omitempty"`
	DiagKind yaegikernel.DiagnosticKind   `json:"diag_kind,omitempty"`
}

type ExecutionReceipt struct {
	ReceiptRef             string `json:"receipt_ref"`
	GrantedReceiptRef      string `json:"granted_receipt_ref,omitempty"`
	AgentRunID             string `json:"agent_run_id"`
	CapabilityHandleDigest string `json:"capability_handle_digest"`
	CapsuleID              string `json:"capsule_id"`
	Command                string `json:"command"`
	Cwd                    string `json:"cwd"`
	ExitCode               int    `json:"exit_code"`
	StdoutDigest           string `json:"stdout_digest"`
	StderrDigest           string `json:"stderr_digest"`
	WorktreeDigest         string `json:"worktree_digest"`
	SourceTreeDigest       string `json:"source_tree_digest"`
	OccurredAt             string `json:"occurred_at"`
	// Job fields: set only on a capsule job's receipt (jobs.go), omitted
	// otherwise, so a cell or exec receipt keeps its canonical bytes and
	// digest. Argv and EnvDigest bind exactly what ran; JobState says how
	// it ended (done, failed, timeout, stopped); StartWorktreeDigest is the
	// tree the job started on (WorktreeDigest is the tree it ended with);
	// StdoutDigest is the stored log's sha256, LogRef its content address.
	JobID               string   `json:"job_id,omitempty"`
	JobState            string   `json:"job_state,omitempty"`
	Argv                []string `json:"argv,omitempty"`
	EnvDigest           string   `json:"env_digest,omitempty"`
	StartedAt           string   `json:"started_at,omitempty"`
	DurationMS          int64    `json:"duration_ms,omitempty"`
	OutputBytes         int64    `json:"output_bytes,omitempty"`
	OutputTruncated     bool     `json:"output_truncated,omitempty"`
	StartWorktreeDigest string   `json:"start_worktree_digest,omitempty"`
	LogRef              string   `json:"log_ref,omitempty"`
}

// ChangeKind describes the type of filesystem change.
type CapsuleFateReceipt struct {
	ReceiptRef             string `json:"receipt_ref"`
	AgentRunID             string `json:"agent_run_id"`
	CapabilityHandleDigest string `json:"capability_handle_digest"`
	CapsuleID              string `json:"capsule_id"`
	Disposition            string `json:"disposition"`
	SourceSubjectDigest    string `json:"source_subject_digest"`
	FinalSubjectDigest     string `json:"final_subject_digest"`
	OccurredAt             string `json:"occurred_at"`
}

// CapsuleRevocationReceipt is the executor's durable structured
// acknowledgement that the exact run/capability/capsule authority is absent
// after a previously durable revoke intent.
type CapsuleRevocationReceipt struct {
	ReceiptRef                 string `json:"receipt_ref"`
	AgentRunID                 string `json:"agent_run_id"`
	AssignmentCapabilityDigest string `json:"assignment_capability_digest"`
	CapsuleID                  string `json:"capsule_id"`
	IntentRef                  string `json:"intent_ref"`
	Disposition                string `json:"disposition"`
	CapsuleAbsent              bool   `json:"capsule_absent"`
	OccurredAt                 string `json:"occurred_at"`
}

type GrantedExecutionReceipt struct {
	ReceiptRef             string           `json:"receipt_ref"`
	Execution              ExecutionReceipt `json:"execution"`
	AgentRunID             string           `json:"agent_run_id"`
	CapabilityHandleDigest string           `json:"capability_handle_digest"`
	CapsuleID              string           `json:"capsule_id"`
	Frozen                 bool             `json:"frozen"`
	SourceSubjectDigest    string           `json:"source_subject_digest"`
	FinalSubjectDigest     string           `json:"final_subject_digest"`
	// SubjectBinding is "observed" when the receipts were granted for a fate
	// that claims nothing about the subject (failed, blocked): no cited
	// receipt is required to have observed FinalSubjectDigest, and
	// LatestReceiptWorktreeDigest records what the chronologically latest
	// one did observe. Empty means the certified binding. Both are omitted
	// when empty, so a certified receipt keeps its canonical bytes.
	SubjectBinding              string `json:"subject_binding,omitempty"`
	LatestReceiptWorktreeDigest string `json:"latest_receipt_worktree_digest,omitempty"`
}

type ChangeKind int

const (
	ChangeAdded ChangeKind = iota
	ChangeModified
	ChangeDeleted
)

func (c ChangeKind) String() string {
	switch c {
	case ChangeAdded:
		return "added"
	case ChangeModified:
		return "modified"
	case ChangeDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// FileChange represents a single filesystem change in the overlay upperdir.
type FileChange struct {
	Path string
	Kind ChangeKind
	Mode os.FileMode
	// Opaque marks a directory the capsule removed and recreated: the
	// overlay hides every lower entry under it.
	Opaque bool
	// Whiteout marks a deletion the upper records as an overlay whiteout
	// (the walk's FileManifest.Type "whiteout"), as opposed to a deletion
	// derived from a committed manifest entry that is gone.
	Whiteout bool
}

// GrantedWorktree is a read of a granted capsule's live tree that changes
// no state: the digest of /workspace/platform (the receipts' and the frozen
// subject's digest) and the capsule's diff. A second Freeze compares it with
// the candidate the first one froze (R9-1).
type GrantedWorktree struct {
	Digest  string
	Changes []FileChange
	// ReleaseFiles is the live release tree (var/lib/artifact/release/),
	// each regular file hashed and moded as StageGrantedRelease records it
	// in the bundle's RuntimeFiles, sorted by path. The /workspace/platform
	// digest does not cover it.
	ReleaseFiles []FrozenReleaseFile
}

type FrozenReleaseFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// StagedRelease is the freeze's materialized payload: the release/ file set
// plus, when the capsule modified workspace/platform sources, a unified
// source patch the host builder consumes to produce a layered release
// (S2-f — runtime changes are source-carried, not binary payloads).
type StagedRelease struct {
	Files []FrozenReleaseFile
	// Binary records where Files' bin/autoputer came from: built in the
	// capsule, or this computer's running binary reused for a change with
	// no binary input.
	Binary *ReleaseBinaryProvenance
	// SourcePatch is the staged unified-diff artifact (<dir>/source.patch).
	// Empty when the capsule made no source changes.
	SourcePatch      string
	SourcePatchSHA   string
	SourceBaseCommit string
	TemporaryRoot    string
	// Visual is the capsule's visual evidence, staged beside release/ under
	// <TemporaryRoot>/evidence/visual/; nil when the capsule captured none
	// (docs/design/visual-evidence-slice-2026-10-15.md).
	Visual *VisualEvidence
}

// FileManifest is a snapshot of a file's metadata at commit time.
type FileManifest struct {
	Path  string
	Size  int64
	Mtime time.Time
	Hash  string // content hash (SHA-256 hex)
	Mode  uint32
	Type  string // "file", "dir", "symlink", "device", "whiteout"
	// Opaque is set on a directory carrying the overlay opaque xattr.
	Opaque bool
}

// CapsuleSummary is a lightweight view of a capsule for listing.
type CapsuleSummary struct {
	ID         string
	State      CapsuleState
	PID        int
	MemoryMax  int64
	Pinned     bool
	OwnerRunID string
}

// CapsuleDiagnostics is the result of a host-side diagnostic inspection.
type CapsuleDiagnostics struct {
	ID          string
	State       CapsuleState
	PID         int
	MemoryUsage int64
	MemoryMax   int64
	CPUUsage    int64
	PidsCurrent int64
	PidsMax     int64
	Uptime      time.Duration
	UpperDir    string
	MergedDir   string
}

// capKey is the internal key for capability lookup.
type capKey struct {
	AgentRunID string
	Handle     string
}
