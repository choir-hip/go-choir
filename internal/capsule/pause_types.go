package capsule

import (
	"errors"
	"time"
)

// Pause and resume of a live capsule across a planned restart
// (docs/design/sandbox-pause-resume-2026-11-09.md). The types are shared by
// the Linux executor and the agentcore runtime that drives it.

const (
	// PauseRecordName is the durable pause record in a capsule's state
	// directory. Its presence is the pause's one commit point (Decision 4).
	PauseRecordName = "pause.json"
	// PauseConsumedName is the record after the one resume has taken it.
	PauseConsumedName = "pause.consumed.json"
	// PausedSpareDir is where a boot spares paused capsule directories
	// (inside the new boot's state directory), and where a same-boot
	// resume moves the old directory before the seeded spawn.
	PausedSpareDir = ".paused"
	// PauseRecordVersion is the record schema this binary reads and writes.
	PauseRecordVersion = 1
)

// PauseRecord is pause.json.
type PauseRecord struct {
	Version      int    `json:"version"`
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	CapsuleID    string `json:"capsule_id"`
	RunID        string `json:"run_id,omitempty"`
	// SubjectDigest is the assignment's pinned source subject.
	SubjectDigest string `json:"subject_digest"`
	// Spec is the spawn spec the capsule took, without transient staging
	// directories (inputs, carried patch), which the resume stages again.
	Spec SpawnSpec `json:"spec"`
	// UpperDigest, UpperFiles and UpperBytes are the upper's own digest and
	// counts at the freeze (LastManifest is not populated in production).
	UpperDigest string `json:"upper_digest"`
	UpperFiles  int    `json:"upper_files"`
	UpperBytes  int64  `json:"upper_bytes"`
	// ReleaseCommit is the platform release the capsule ran on (the lower
	// changes under the upper at a platform update).
	ReleaseCommit string    `json:"release_commit,omitempty"`
	PausedAt      time.Time `json:"paused_at"`
	// ExpiresAt is the pre-ack TTL; after Acked the marker bounds the pause.
	ExpiresAt time.Time `json:"expires_at"`
	Acked     bool      `json:"acked,omitempty"`
	AckedAt   time.Time `json:"acked_at,omitempty"`
	// Measures recorded at first use (design, "Measures").
	DrainMS  int64 `json:"drain_ms,omitempty"`
	FreezeMS int64 `json:"freeze_ms,omitempty"`
	DigestMS int64 `json:"digest_ms,omitempty"`
}

// PauseRequest asks the executor to pause one capsule.
type PauseRequest struct {
	CapsuleID     string
	AssignmentID  string
	Attempt       uint64
	SubjectDigest string
	ReleaseCommit string
	// DrainTimeout bounds the wait for in-flight operations (default 10 min).
	DrainTimeout time.Duration
	// TTL is the pre-ack life of the committed record (default 10 min).
	TTL time.Duration
}

// PauseWhere says where a pause record was found.
type PauseWhere string

const (
	PauseNone     PauseWhere = ""
	PauseLive     PauseWhere = "live"     // a capsule of this process, frozen
	PauseSpared   PauseWhere = "spared"   // a directory spared at this boot
	PauseResidual PauseWhere = "residual" // a directory of this boot with no capsule of this process
	PauseConsumed PauseWhere = "consumed" // the resume took it
)

// PauseLookup is the answer of Executor.LookupPause. Err is set when the
// record could not be read or parsed: callers fail closed (read as live).
type PauseLookup struct {
	Where  PauseWhere
	Record PauseRecord
	Err    error
}

var (
	// ErrCapsulePaused refuses a thaw, extract or inspect of a paused capsule.
	ErrCapsulePaused = errors.New("capsule is paused for a platform update; it resumes after the restart")
	// ErrPauseInProgress is returned for a second pause of a capsule that is
	// still pausing (single-flight).
	ErrPauseInProgress = errors.New("capsule pause already in progress")
	// ErrPauseRefused marks a pause the executor will not take (verifier
	// slot, claude token in the upper, state not active).
	ErrPauseRefused = errors.New("capsule pause refused")
)

// PauseBlockMessage is what a refused operation tells the run while its
// capsule is pausing or paused (Decision 3).
const PauseBlockMessage = "pausing for a platform update; this cell will run after the restart"
