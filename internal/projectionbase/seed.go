package projectionbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

const (
	// seedMetaFileName records the adopted seed identity in a durable scratch
	// directory so a crash mid-install is recognized and reset, never adopted.
	seedMetaFileName = ".projection-base-seed.json"
	// seedStagingDirName stages a candidate seed before it is adopted.
	seedStagingDirName = ".seed-staging"
)

// seedMeta is the durable scratch seed identity.
type seedMeta struct {
	BlobSHA256    string `json:"blob_sha256"`
	Sequence      uint64 `json:"sequence"`
	CanonicalHead string `json:"canonical_head"`
	InstalledAt   string `json:"installed_at"`
}

// SeedCandidate is a published base descriptor paired with its blob path.
type SeedCandidate struct {
	Descriptor Descriptor
	BlobPath   string
}

// FindLatestCompatibleBase returns the latest published base in artifactsRoot
// that is compatible with this computer and frozen target (sequence at or below
// targetSequence; an equal sequence must match the target head). A zero
// targetSequence skips the sequence bound. The bool is false when no compatible
// base exists. Ancestry is not decidable here; replay refuses non-ancestor
// seeds via VerifyTailHead.
func FindLatestCompatibleBase(artifactsRoot, computerID, targetHead string, targetSequence uint64) (Descriptor, string, bool, error) {
	candidates, err := listCompatibleBases(artifactsRoot, computerID, targetHead, targetSequence)
	if err != nil {
		return Descriptor{}, "", false, err
	}
	if len(candidates) == 0 {
		return Descriptor{}, "", false, nil
	}
	return candidates[0].Descriptor, candidates[0].BlobPath, true, nil
}

// LoadPublishedBase loads exactly the pinned published base: descriptor sidecar
// plus blob, refused when missing, foreign, incompatible, or the blob is gone.
func LoadPublishedBase(artifactsRoot, computerID, baseRef string) (Descriptor, string, error) {
	baseRef = strings.ToLower(strings.TrimSpace(baseRef))
	if !computerevent.IsSHA256(baseRef) {
		return Descriptor{}, "", fmt.Errorf("%w: pinned base ref must be lowercase SHA-256", ErrBaseRefused)
	}
	dir := filepath.Join(filepath.Clean(artifactsRoot), "sha256", Namespace)
	raw, err := os.ReadFile(filepath.Join(dir, DescriptorSidecarName(baseRef)))
	if err != nil {
		return Descriptor{}, "", fmt.Errorf("%w: published base %s has no descriptor sidecar", ErrBaseRefused, baseRef)
	}
	descriptor, err := ParseDescriptor(raw)
	if err != nil {
		return Descriptor{}, "", fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	if descriptor.BlobSHA256 != baseRef {
		return Descriptor{}, "", fmt.Errorf("%w: descriptor blob %s is not the pinned ref %s", ErrBaseRefused, descriptor.BlobSHA256, baseRef)
	}
	if err := descriptor.compatibleForSeeding(computerID); err != nil {
		return Descriptor{}, "", fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	blobPath := filepath.Join(dir, baseRef)
	if info, err := os.Stat(blobPath); err != nil || !info.Mode().IsRegular() {
		return Descriptor{}, "", fmt.Errorf("%w: published blob %s is missing", ErrBaseRefused, baseRef)
	}
	return descriptor, blobPath, nil
}

// listCompatibleBases scans published descriptor sidecars for candidates
// compatible with this computer and frozen target, newest sequence first.
func listCompatibleBases(artifactsRoot, computerID, targetHead string, targetSequence uint64) ([]SeedCandidate, error) {
	dir := filepath.Join(filepath.Clean(artifactsRoot), "sha256", Namespace)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("projection base: scan published bases: %w", err)
	}
	targetHead = strings.ToLower(strings.TrimSpace(targetHead))
	var candidates []SeedCandidate
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".descriptor.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		descriptor, err := ParseDescriptor(raw)
		if err != nil {
			continue
		}
		if err := descriptor.compatibleForSeeding(computerID); err != nil {
			continue
		}
		if targetSequence > 0 {
			if descriptor.Sequence > targetSequence {
				continue
			}
			if descriptor.Sequence == targetSequence && descriptor.CanonicalHead != targetHead {
				continue
			}
		}
		blobPath := filepath.Join(dir, descriptor.BlobSHA256)
		if info, err := os.Stat(blobPath); err != nil || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, SeedCandidate{Descriptor: descriptor, BlobPath: blobPath})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Descriptor.Sequence != candidates[j].Descriptor.Sequence {
			return candidates[i].Descriptor.Sequence > candidates[j].Descriptor.Sequence
		}
		return candidates[i].Descriptor.CreatedAt.After(candidates[j].Descriptor.CreatedAt)
	})
	return candidates, nil
}

// compatibleForSeeding is the offline compatibility gate: the descriptor must
// validate, belong to this computer, and speak the live reducer, schema, and
// vocabulary.
func (d Descriptor) compatibleForSeeding(computerID string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if d.ComputerID != strings.TrimSpace(computerID) {
		return fmt.Errorf("descriptor computer %q is not %q", d.ComputerID, strings.TrimSpace(computerID))
	}
	if d.ReducerVersion != computerevent.ReducerVersionV1 {
		return fmt.Errorf("descriptor reducer version %d is incompatible with live %d", d.ReducerVersion, computerevent.ReducerVersionV1)
	}
	if d.SchemaVersion != computerevent.SchemaVersionV1 {
		return fmt.Errorf("descriptor schema version %d is incompatible with live %d", d.SchemaVersion, computerevent.SchemaVersionV1)
	}
	return nil
}

func readSeedMeta(path string) (*seedMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read seed metadata: %w", err)
	}
	var meta seedMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("decode seed metadata: %w", err)
	}
	if !computerevent.IsSHA256(meta.CanonicalHead) || meta.Sequence == 0 || !computerevent.IsSHA256(meta.BlobSHA256) {
		return nil, fmt.Errorf("seed metadata is incomplete")
	}
	return &meta, nil
}

func writeSeedMeta(path string, meta seedMeta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// installSeedCandidate stages a published base into the scratch seed staging
// directory, proves the blob digest, unpacked head, and content witness, then
// adopts the bytes into the scratch directory and stamps the seed identity for
// durable resume.
func installSeedCandidate(ctx context.Context, candidate SeedCandidate, scratchDir string) error {
	descriptor := candidate.Descriptor
	stagingRoot := filepath.Join(scratchDir, seedStagingDirName)
	if err := os.RemoveAll(stagingRoot); err != nil {
		return fmt.Errorf("%w: clear seed staging: %v", ErrBaseRefused, err)
	}
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return fmt.Errorf("%w: create seed staging: %v", ErrBaseRefused, err)
	}
	defer os.RemoveAll(stagingRoot)

	if err := unpackVerified(candidate.BlobPath, stagingRoot, descriptor.BlobSHA256); err != nil {
		return err
	}
	if err := verifyInstalledHead(stagingRoot, "runtime.db", descriptor); err != nil {
		return err
	}
	workspace := choirstore.TextureWorkspacePath(filepath.Join(stagingRoot, "runtime.db"))
	got, err := witnessForWorkspace(ctx, descriptor.ComputerID, descriptor.CanonicalHead, workspace)
	if err != nil {
		return fmt.Errorf("%w: seed witness: %v", ErrBaseRefused, err)
	}
	if err := selfdevprotocol.WitnessContentMatches(got, descriptor.VMLocalContentWitness); err != nil {
		return fmt.Errorf("%w: seed content witness mismatch: %v", ErrBaseRefused, err)
	}

	// Adopt: clear the scratch entries this tool owns, stamp the seed identity,
	// then move the staged bytes in. A crash at any point leaves the scratch
	// recognizable as a partial seed (metadata without a complete store) and it
	// is reset and reinstalled, never silently adopted.
	if err := resetScratchContent(scratchDir); err != nil {
		return fmt.Errorf("%w: reset scratch: %v", ErrBaseRefused, err)
	}
	meta := seedMeta{
		BlobSHA256:    descriptor.BlobSHA256,
		Sequence:      descriptor.Sequence,
		CanonicalHead: descriptor.CanonicalHead,
		InstalledAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeSeedMeta(filepath.Join(scratchDir, seedMetaFileName), meta); err != nil {
		return fmt.Errorf("%w: stamp seed: %v", ErrBaseRefused, err)
	}
	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		return fmt.Errorf("%w: read seed staging: %v", ErrBaseRefused, err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(stagingRoot, entry.Name()), filepath.Join(scratchDir, entry.Name())); err != nil {
			return fmt.Errorf("%w: adopt seed entry %s: %v", ErrBaseRefused, entry.Name(), err)
		}
	}
	return nil
}
