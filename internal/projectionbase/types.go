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

	// DefaultMemoryLimitRSSBytes is the maximum process RSS target during replay (2 GiB).
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

// Config configures the offline projection rebuilder.
type Config struct {
	ComputerID     string
	TargetHead     string
	ArtifactsRoot  string
	ScratchDir     string
	KeyMaterial    []byte
	BatchSize      int
	MemoryLimitRSS int64
}

// Validate checks the configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ComputerID) == "" {
		return fmt.Errorf("projection base: computer ID is required")
	}
	if strings.TrimSpace(c.TargetHead) == "" {
		return fmt.Errorf("projection base: target head is required")
	}
	if strings.TrimSpace(c.ArtifactsRoot) == "" {
		return fmt.Errorf("projection base: artifacts root is required")
	}
	if len(c.KeyMaterial) != 32 {
		return fmt.Errorf("projection base: valid 32-byte key material is required")
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
	if !IsKnownVocabularyVersion(d.VocabularyVersion) {
		return fmt.Errorf("descriptor: unknown vocabulary version %q", d.VocabularyVersion)
	}
	if err := d.VMLocalContentWitness.Validate(); err != nil {
		return fmt.Errorf("descriptor: %w", err)
	}
	return nil
}
