package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const CapsuleFormatVersion = 1

// RecoveryCapsule is a self-describing, portable disaster recovery archive
// metadata envelope containing all cryptographic commitments required to
// verify and reconstruct a persistent computer across realization engines.
type RecoveryCapsule struct {
	FormatVersion         int                                   `json:"format_version"`
	ComputerID            string                                `json:"computer_id"`
	EventHead             string                                `json:"event_head"`
	EventSequence         uint64                                `json:"event_sequence"`
	FileRoot              string                                `json:"file_root"`
	FileManifest          *filecas.Manifest                     `json:"file_manifest"`
	ProjectionBaseRef     string                                `json:"projection_base_ref,omitempty"`
	KeyEscrowDigest       string                                `json:"key_escrow_digest"`
	VMLocalContentWitness selfdevprotocol.VMLocalContentWitness `json:"vm_local_content_witness"`
	CreatedAt             time.Time                             `json:"created_at"`
	CapsuleDigest         string                                `json:"capsule_digest,omitempty"`
}

// BuildCapsule constructs and validates a new RecoveryCapsule, computing its
// canonical cryptographic digest.
func BuildCapsule(
	computerID string,
	eventHead string,
	eventSequence uint64,
	fileRoot string,
	manifest *filecas.Manifest,
	projectionBaseRef string,
	keyEscrowDigest string,
	witness selfdevprotocol.VMLocalContentWitness,
	createdAt time.Time,
) (*RecoveryCapsule, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("capsule: computer_id is required")
	}
	eventHead = strings.TrimSpace(eventHead)
	if eventHead == "" || !computerevent.IsSHA256(eventHead) {
		return nil, fmt.Errorf("capsule: valid event_head SHA-256 is required")
	}
	fileRoot = strings.TrimSpace(fileRoot)
	if fileRoot == "" || !computerevent.IsSHA256(fileRoot) {
		return nil, fmt.Errorf("capsule: valid file_root SHA-256 is required")
	}
	if manifest == nil {
		return nil, fmt.Errorf("capsule: file_manifest is required")
	}
	if err := manifest.VerifyRoot(); err != nil {
		return nil, fmt.Errorf("capsule: manifest verification failed: %w", err)
	}
	if manifest.Root != fileRoot || manifest.ComputerID != computerID {
		return nil, fmt.Errorf("capsule: manifest does not match file_root or computer_id")
	}
	keyEscrowDigest = strings.TrimSpace(keyEscrowDigest)
	if keyEscrowDigest == "" || !computerevent.IsSHA256(keyEscrowDigest) {
		return nil, fmt.Errorf("capsule: valid key_escrow_digest SHA-256 is required")
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	capsule := &RecoveryCapsule{
		FormatVersion:         CapsuleFormatVersion,
		ComputerID:            computerID,
		EventHead:             eventHead,
		EventSequence:         eventSequence,
		FileRoot:              fileRoot,
		FileManifest:          manifest,
		ProjectionBaseRef:     strings.TrimSpace(projectionBaseRef),
		KeyEscrowDigest:       keyEscrowDigest,
		VMLocalContentWitness: witness,
		CreatedAt:             createdAt.UTC(),
	}

	digest, err := capsule.computeDigest()
	if err != nil {
		return nil, err
	}
	capsule.CapsuleDigest = digest
	return capsule, nil
}

// Verify validates the internal cryptographic consistency of the capsule.
func (c *RecoveryCapsule) Verify() error {
	if c == nil {
		return fmt.Errorf("capsule: nil")
	}
	if c.FormatVersion != CapsuleFormatVersion {
		return fmt.Errorf("capsule: unsupported format version %d", c.FormatVersion)
	}
	if strings.TrimSpace(c.ComputerID) == "" {
		return fmt.Errorf("capsule: computer_id is empty")
	}
	if !computerevent.IsSHA256(c.EventHead) {
		return fmt.Errorf("capsule: invalid event_head")
	}
	if !computerevent.IsSHA256(c.FileRoot) {
		return fmt.Errorf("capsule: invalid file_root")
	}
	if c.FileManifest == nil {
		return fmt.Errorf("capsule: missing file manifest")
	}
	if err := c.FileManifest.VerifyRoot(); err != nil {
		return fmt.Errorf("capsule: invalid file manifest: %w", err)
	}
	if c.FileManifest.Root != c.FileRoot {
		return fmt.Errorf("capsule: file manifest root mismatch")
	}
	if !computerevent.IsSHA256(c.KeyEscrowDigest) {
		return fmt.Errorf("capsule: invalid key_escrow_digest")
	}
	digest, err := c.computeDigest()
	if err != nil {
		return err
	}
	if c.CapsuleDigest != "" && c.CapsuleDigest != digest {
		return fmt.Errorf("capsule: digest mismatch: got %s, want %s", c.CapsuleDigest, digest)
	}
	return nil
}

func (c *RecoveryCapsule) computeDigest() (string, error) {
	unsigned := struct {
		FormatVersion         int                                   `json:"format_version"`
		ComputerID            string                                `json:"computer_id"`
		EventHead             string                                `json:"event_head"`
		EventSequence         uint64                                `json:"event_sequence"`
		FileRoot              string                                `json:"file_root"`
		FileManifest          *filecas.Manifest                     `json:"file_manifest"`
		ProjectionBaseRef     string                                `json:"projection_base_ref,omitempty"`
		KeyEscrowDigest       string                                `json:"key_escrow_digest"`
		VMLocalContentWitness selfdevprotocol.VMLocalContentWitness `json:"vm_local_content_witness"`
		CreatedAt             string                                `json:"created_at"`
	}{
		FormatVersion:         c.FormatVersion,
		ComputerID:            c.ComputerID,
		EventHead:             c.EventHead,
		EventSequence:         c.EventSequence,
		FileRoot:              c.FileRoot,
		FileManifest:          c.FileManifest,
		ProjectionBaseRef:     c.ProjectionBaseRef,
		KeyEscrowDigest:       c.KeyEscrowDigest,
		VMLocalContentWitness: c.VMLocalContentWitness,
		CreatedAt:             c.CreatedAt.Format(time.RFC3339Nano),
	}
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil {
		return "", fmt.Errorf("capsule: canonical json: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
