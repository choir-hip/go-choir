package capsule

import (
	"os"
	"time"
)

// SpawnSpec describes the desired state of a new capsule.
type SpawnSpec struct {
	CapsuleID    string        // unique capsule identifier (UUID)
	MemoryMax    int64         // total budget: RSS + tmpfs + kmem (bytes)
	CpuQuota     int64         // CPU quota (microseconds per period)
	CpuPeriod    int64         // CPU period (default 100000)
	PidsMax      int64         // max processes in capsule
	DiskMax      int64         // tmpfs sub-budget of MemoryMax (bytes)
	Env          []string      // environment variables for broker/workload
	WorkingDir   string        // initial working directory
	OwnerRunID   string        // agent run that owns this capsule
	Tier         ResourceTier  // resource tier preset
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
type ExecRequest struct {
	SessionID string        // broker-minted random ID (NOT agentRunID)
	Command   string        // command to execute
	Cwd       string        // working directory for command
	Env       []string      // environment overrides
	Stdin     string        // stdin content (empty for no input)
	TimeoutMS int           // timeout in milliseconds (0 = no timeout)
	PTY       bool          // use PTY for command
}

// ExecResult is the result of executing a command in a capsule.
type ExecResult struct {
	ExitCode  int           // process exit code
	SessionID string        // session ID (returned when broker creates new session)
	Stdout    string        // stdout content
	Stderr    string        // stderr content
	Duration  time.Duration // execution duration
}

// ChangeKind describes the type of filesystem change.
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
}

// FileManifest is a snapshot of a file's metadata at commit time.
type FileManifest struct {
	Path  string
	Size  int64
	Mtime time.Time
	Hash  string // content hash (SHA-256 hex)
	Mode  uint32
	Type  string // "file", "dir", "symlink", "device"
}

// CapsuleSummary is a lightweight view of a capsule for listing.
type CapsuleSummary struct {
	ID        string
	State     CapsuleState
	PID       int
	MemoryMax int64
	Pinned    bool
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
