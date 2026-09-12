package projectionbase

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const (
	// Namespace is the artifact subfolder in platform-artifacts.
	Namespace = "projection-base"

	// DefaultBatchSize is the number of events per database transaction during offline replay.
	DefaultBatchSize = 2000

	// DefaultMemoryLimitRSSBytes is the maximum process RSS target during replay (2 GiB).
	DefaultMemoryLimitRSSBytes = 2 * 1024 * 1024 * 1024
)

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
type Descriptor struct {
	ComputerID            string                                `json:"computer_id"`
	Sequence              uint64                                `json:"sequence"`
	CanonicalHead         string                                `json:"canonical_head"`
	BlobSHA256            string                                `json:"blob_sha256"`
	BlobSizeBytes         int64                                 `json:"blob_size_bytes"`
	ReducerVersion        int                                   `json:"reducer_version"`
	SchemaVersion         int                                   `json:"schema_version"`
	VMLocalContentWitness selfdevprotocol.VMLocalContentWitness `json:"vm_local_content_witness"`
	CreatedAt             time.Time                             `json:"created_at"`
}

// Validate verifies the integrity of the descriptor.
func (d Descriptor) Validate() error {
	if strings.TrimSpace(d.ComputerID) == "" {
		return fmt.Errorf("descriptor: computer ID is required")
	}
	if d.Sequence == 0 {
		return fmt.Errorf("descriptor: sequence must be positive")
	}
	if strings.TrimSpace(d.CanonicalHead) == "" {
		return fmt.Errorf("descriptor: canonical head is required")
	}
	if strings.TrimSpace(d.BlobSHA256) == "" {
		return fmt.Errorf("descriptor: blob sha256 is required")
	}
	if d.BlobSizeBytes <= 0 {
		return fmt.Errorf("descriptor: blob size must be positive")
	}
	return nil
}
