package projectionbase

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const (
	// Namespace is the artifact subfolder in platform-artifacts.
	Namespace = "projection-base"

	// DefaultBatchSize is the number of events per database transaction during offline replay.
	DefaultBatchSize = 2000

	// DefaultMemoryLimitRSSBytes is the default process RSS growth budget
	// above the run baseline during replay (2 GiB). The isolated worker also
	// runs under a hard systemd MemoryMax; this is the in-process runaway guard.
	DefaultMemoryLimitRSSBytes = 2 * 1024 * 1024 * 1024

	// CurrentVocabularyVersion is the live protocol vocabulary a base is
	// published and consumed under. Historic tape stays decodable under
	// frozen V1 replay rules; the descriptor seam only records which live
	// vocabulary the materialized base speaks. Mission 2 cutover: writers
	// emit v2 only; v1-stamped bases stay installable via IsKnownVocabularyVersion.
	CurrentVocabularyVersion = "v2"
)

// IsKnownVocabularyVersion reports whether a base speaking v may be installed
// or replayed by this build. Unknown live versions fail closed. Mission 2
// widens the known set to {v1, v2} exactly once and never reverts this commit:
// a v1-stamped base must stay installable after the writer cutover advances
// CurrentVocabularyVersion to v2 in a later revertible commit. Rollback of the
// cutover reverts Current but never this set.
func IsKnownVocabularyVersion(v string) bool {
	switch strings.TrimSpace(v) {
	case "v1", "v2":
		return true
	default:
		return false
	}
}

// SeedPolicy selects how Rebuilder seeds its scratch store before replay.
type SeedPolicy string

const (
	// SeedAuto (the zero value) seeds from the latest compatible verified
	// published base and replays only its tail. A genesis fallback is a
	// routine-path failure: it refuses beyond the first-base bound.
	SeedAuto SeedPolicy = "auto"
	// SeedNone never seeds: genesis replay is an explicit repair or first-base
	// bootstrap, bounded by the caller's MaxTailEvents.
	SeedNone SeedPolicy = "none"
	// SeedPinned seeds exactly SeedBaseRef and refuses when it does not verify.
	SeedPinned SeedPolicy = "pinned"
)

func normalizeSeedPolicy(p SeedPolicy) (SeedPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(string(p))) {
	case "", string(SeedAuto):
		return SeedAuto, nil
	case string(SeedNone):
		return SeedNone, nil
	case string(SeedPinned):
		return SeedPinned, nil
	default:
		return "", fmt.Errorf("projection base: unknown seed policy %q", p)
	}
}

// Config configures the offline projection rebuilder.
type Config struct {
	ComputerID string
	TargetHead string
	// TargetSequence freezes the target event sequence for this job. Zero
	// derives it from the chain head when the chain head equals TargetHead;
	// seeding and MaxTailEvents refuse when the bound cannot be proven.
	TargetSequence uint64
	ArtifactsRoot  string
	ScratchDir     string
	KeyMaterial    []byte
	BatchSize      int
	// MemoryLimitRSS bounds process RSS growth above the run baseline, in
	// bytes (DefaultMemoryLimitRSSBytes when zero). The isolated worker also
	// runs under a hard systemd MemoryMax; this is the in-process runaway guard.
	MemoryLimitRSS int64
	// SeedPolicy selects how the scratch store is seeded (SeedAuto default).
	SeedPolicy SeedPolicy
	// SeedBaseRef pins an exact published base blob digest for SeedPinned.
	SeedBaseRef string
	// SeedRequired refuses genesis fallback entirely (incremental publication).
	SeedRequired bool
	// MaxTailEvents refuses a run whose replay span (frozen target minus seed
	// watermark; the whole chain for genesis) exceeds this many events. Zero
	// means the caller supplied no explicit bound.
	MaxTailEvents uint64
	// PublishParts publishes the base in the parts form (S4): a manifest
	// plus content-addressed parts, so a publish writes only changed files.
	// The caller decides it per computer (PartsSelectorEnv).
	PublishParts bool
}

// Validate checks the configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ComputerID) == "" {
		return fmt.Errorf("projection base: computer ID is required")
	}
	if !computerevent.IsSHA256(strings.TrimSpace(c.TargetHead)) {
		return fmt.Errorf("projection base: target head must be lowercase SHA-256")
	}
	if strings.TrimSpace(c.ArtifactsRoot) == "" {
		return fmt.Errorf("projection base: artifacts root is required")
	}
	if len(c.KeyMaterial) != 32 {
		return fmt.Errorf("projection base: valid 32-byte key material is required")
	}
	policy, err := normalizeSeedPolicy(c.SeedPolicy)
	if err != nil {
		return err
	}
	switch {
	case policy == SeedPinned && !computerevent.IsSHA256(strings.TrimSpace(c.SeedBaseRef)):
		return fmt.Errorf("projection base: SeedPinned requires a lowercase SHA-256 SeedBaseRef")
	case policy != SeedPinned && strings.TrimSpace(c.SeedBaseRef) != "":
		return fmt.Errorf("projection base: SeedBaseRef requires SeedPolicy SeedPinned")
	case policy == SeedNone && c.SeedRequired:
		return fmt.Errorf("projection base: SeedRequired conflicts with SeedPolicy SeedNone")
	}
	return nil
}

// Descriptor records the canonical binding of a published ProjectionBase artifact.
// It is a verified accelerator binding, never a parallel head or authority:
// every field is authenticated before installation and the tail (W,H] still
// replays from immutable tape.
type Descriptor struct {
	ComputerID            string                                `json:"computer_id"`
	Sequence              uint64                                `json:"sequence"`
	CanonicalHead         string                                `json:"canonical_head"`
	BlobSHA256            string                                `json:"blob_sha256"`
	BlobSizeBytes         int64                                 `json:"blob_size_bytes"`
	ReducerVersion        int                                   `json:"reducer_version"`
	SchemaVersion         int                                   `json:"schema_version"`
	VocabularyVersion     string                                `json:"vocabulary_version"`
	VMLocalContentWitness selfdevprotocol.VMLocalContentWitness `json:"vm_local_content_witness"`
	CreatedAt             time.Time                             `json:"created_at"`
	// Origin is how the base's state was made: empty for a replay (from
	// genesis or from a seed), OriginLiveCaptured for a capture of a running
	// computer's store, which is an epoch root.
	Origin string `json:"origin,omitempty"`
	// EpochRoot names the captured base a replayed base descends from, when
	// it does: derivability is measured from that root, and the tape below
	// it is never replayed again.
	EpochRoot *EpochRootRef `json:"epoch_root,omitempty"`
	// Format is how the base's bytes are published: empty for one tar blob
	// at BlobSHA256 (bases from before S4), FormatParts for a manifest at
	// BlobSHA256 naming content-addressed parts. BlobSizeBytes is then the
	// manifest's size. The vocabulary version carries the "+parts" suffix
	// so a build from before S4 refuses the descriptor.
	Format string `json:"format,omitempty"`
}

// OriginLiveCaptured marks an epoch root: a base whose state was captured
// from a running computer's data image rather than rebuilt from the tape (S2
// of docs/problems/paper-never-checkpointed-2026-10-27.md). Its head is
// checked against the tape and its witness is of its own bytes. The owner
// made it the definition of state at its sequence (2026-10-27: "maybe we
// lose some event log data and restart it from here"): bases replayed from
// it are ordinary derived bases that record it (EpochRoot), and nothing
// replays the tape below it. The prefix is not deleted.
const OriginLiveCaptured = "live_captured"

// liveCapturedVocabularySuffix is appended to an epoch root's vocabulary
// version. A build from before S2 ignores the origin field (ParseDescriptor
// allows unknown fields) and refuses any vocabulary outside {v1, v2}, so the
// suffix makes it fail closed on a root instead of reading it as a replay of
// the tape (review 364).
const liveCapturedVocabularySuffix = "+" + OriginLiveCaptured

// EpochRootRef names an epoch root: its blob and the sequence it is at.
type EpochRootRef struct {
	BaseRef  string `json:"base_ref"`
	Sequence uint64 `json:"sequence"`
}

// VocabularyVersionFor is the vocabulary version a base of this origin is
// published under.
func VocabularyVersionFor(origin string) string {
	if origin == OriginLiveCaptured {
		return CurrentVocabularyVersion + liveCapturedVocabularySuffix
	}
	return CurrentVocabularyVersion
}

// EffectiveVocabularyVersion is the protocol vocabulary the base speaks,
// without an epoch root's suffix.
func (d Descriptor) EffectiveVocabularyVersion() string {
	return strings.TrimSuffix(d.formLessVocabularyVersion(), liveCapturedVocabularySuffix)
}

// formLessVocabularyVersion is the vocabulary version without the parts
// form's suffix, the outermost one.
func (d Descriptor) formLessVocabularyVersion() string {
	return strings.TrimSuffix(strings.TrimSpace(d.VocabularyVersion), partsVocabularySuffix)
}

// EpochFloor is the sequence below which no replay onto this base's line may
// start: the root's own sequence for an epoch root, the recorded root's for
// its descendants, zero otherwise.
func (d Descriptor) EpochFloor() uint64 {
	if d.Origin == OriginLiveCaptured {
		return d.Sequence
	}
	if d.EpochRoot != nil {
		return d.EpochRoot.Sequence
	}
	return 0
}

// Validate verifies the integrity of the descriptor. A missing or unknown
// vocabulary version refuses: pre-seam bases are uninstallable, not legacy.
func (d Descriptor) Validate() error {
	if strings.TrimSpace(d.ComputerID) == "" {
		return fmt.Errorf("descriptor: computer ID is required")
	}
	if d.Sequence == 0 {
		return fmt.Errorf("descriptor: sequence must be positive")
	}
	if !computerevent.IsSHA256(d.CanonicalHead) {
		return fmt.Errorf("descriptor: canonical head must be lowercase SHA-256")
	}
	if !computerevent.IsSHA256(d.BlobSHA256) {
		return fmt.Errorf("descriptor: blob sha256 must be lowercase SHA-256")
	}
	if d.BlobSizeBytes <= 0 {
		return fmt.Errorf("descriptor: blob size must be positive")
	}
	if d.ReducerVersion <= 0 {
		return fmt.Errorf("descriptor: reducer version must be positive")
	}
	if d.SchemaVersion <= 0 {
		return fmt.Errorf("descriptor: schema version must be positive")
	}
	if strings.TrimSpace(d.VocabularyVersion) == "" {
		return fmt.Errorf("descriptor: vocabulary version is required")
	}
	if d.Origin != "" && d.Origin != OriginLiveCaptured {
		return fmt.Errorf("descriptor: unknown origin %q", d.Origin)
	}
	// The root's mark and its vocabulary suffix agree both ways.
	if d.Format != "" && d.Format != FormatParts {
		return fmt.Errorf("descriptor: unknown format %q", d.Format)
	}
	// The parts form and its vocabulary suffix agree both ways.
	if tagged := strings.HasSuffix(strings.TrimSpace(d.VocabularyVersion), partsVocabularySuffix); tagged != (d.Format == FormatParts) {
		return fmt.Errorf("descriptor: vocabulary version %q does not match format %q", d.VocabularyVersion, d.Format)
	}
	if tagged := strings.HasSuffix(d.formLessVocabularyVersion(), liveCapturedVocabularySuffix); tagged != (d.Origin == OriginLiveCaptured) {
		return fmt.Errorf("descriptor: vocabulary version %q does not match origin %q", d.VocabularyVersion, d.Origin)
	}
	if !IsKnownVocabularyVersion(d.EffectiveVocabularyVersion()) {
		return fmt.Errorf("descriptor: unknown vocabulary version %q", d.VocabularyVersion)
	}
	if err := d.VMLocalContentWitness.Validate(); err != nil {
		return fmt.Errorf("descriptor: %w", err)
	}
	if d.EpochRoot != nil {
		if d.Origin == OriginLiveCaptured {
			return fmt.Errorf("descriptor: an epoch root records no root of its own")
		}
		if !computerevent.IsSHA256(d.EpochRoot.BaseRef) || d.EpochRoot.Sequence == 0 || d.EpochRoot.Sequence > d.Sequence {
			return fmt.Errorf("descriptor: invalid epoch root %+v", *d.EpochRoot)
		}
	}
	return nil
}
