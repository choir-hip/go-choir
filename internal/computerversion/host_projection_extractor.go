package computerversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// HostProjectionMaterializer names the realization produced from reading
	// a host-process projection directory.
	HostProjectionMaterializer = "host-projection-extractor"
	// HostProjectionSubstrate names the non-Firecracker host-process projection
	// substrate for file-manifest/blob-set extraction.
	HostProjectionSubstrate = "host-process/file-projection"
)

// HostProjectionExtractor reads file_manifest and blob_set observations from a
// host-process projection directory. It is a non-Firecracker substrate: it
// reads the same logical file tree from a different filesystem path, producing
// observations with a different substrate identity.
//
// This extractor proves substrate independence: the same ComputerVersion's
// durable state can be extracted from different substrate paths and produce
// equivalent observations. It does NOT launch a VM, container, or any process.
// It performs read-only filesystem traversal.
type HostProjectionExtractor struct {
	// RootDir is the root directory to read from. Must be non-empty.
	RootDir string
}

var _ Extractor = HostProjectionExtractor{}

// Extract walks the projection directory and produces file_manifest and
// blob_set observations for request.Version.
func (e HostProjectionExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if !request.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("host projection extraction: invalid computer version")
	}
	root := strings.TrimSpace(e.RootDir)
	if root == "" {
		return ObservationSet{}, fmt.Errorf("host projection extraction: root dir is required")
	}
	info, err := os.Stat(root)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("host projection extraction: stat %q: %w", root, err)
	}
	if !info.IsDir() {
		return ObservationSet{}, fmt.Errorf("host projection extraction: %q is not a directory", root)
	}

	type fileEntry struct {
		relPath string
		size    int64
		sha256  string
		blobRef string
	}

	files := make([]fileEntry, 0)
	blobHashes := make(map[string]string)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel path %q: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		hash := sha256.Sum256(data)
		sha := hex.EncodeToString(hash[:])
		blobRef := "sha256:" + sha
		files = append(files, fileEntry{
			relPath: rel,
			size:    int64(len(data)),
			sha256:  sha,
			blobRef: blobRef,
		})
		blobHashes[sha] = blobRef
		return nil
	})
	if walkErr != nil {
		return ObservationSet{}, fmt.Errorf("host projection extraction: %w", walkErr)
	}
	if len(files) == 0 {
		return ObservationSet{}, fmt.Errorf("host projection extraction: no files in %q", root)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })

	observations := make([]Observation, 0, len(files)+len(blobHashes))

	for _, f := range files {
		entry := firecrackerFileManifestEntry{
			Path:    f.relPath,
			Size:    f.size,
			SHA256:  f.sha256,
			BlobRef: f.blobRef,
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("host projection extraction: encode %q: %w", f.relPath, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationFileManifest,
			Key:   f.relPath,
			Value: string(encoded),
		})
	}

	blobKeys := make([]string, 0, len(blobHashes))
	for sha := range blobHashes {
		blobKeys = append(blobKeys, sha)
	}
	sort.Strings(blobKeys)
	for _, sha := range blobKeys {
		ref := blobHashes[sha]
		entry := firecrackerBlobSetEntry{
			BlobRef:   ref,
			SHA256:    sha,
			SizeBytes: 0,
		}
		for _, f := range files {
			if f.sha256 == sha {
				entry.SizeBytes = f.size
				break
			}
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("host projection extraction: encode blob %q: %w", ref, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationBlobSet,
			Key:   ref,
			Value: string(encoded),
		})
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "host-projection"
	}
	return ObservationSet{
		Name:         name,
		Version:      request.Version,
		Required:     []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: observations,
	}, nil
}

// HostProjectionCapabilityManifest declares the observation scope for the
// host-process projection extractor.
func HostProjectionCapabilityManifest(materializer, substrate string) CapabilityManifest {
	materializer = strings.TrimSpace(materializer)
	if materializer == "" {
		materializer = HostProjectionMaterializer
	}
	substrate = strings.TrimSpace(substrate)
	if substrate == "" {
		substrate = HostProjectionSubstrate
	}
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    substrate,
		Supported:    []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationDoltHead, Reason: "host projection does not prove Dolt ledger head equivalence"},
			{Kind: ObservationObjectGraphHead, Reason: "host projection does not prove object graph head equivalence"},
			{Kind: ObservationProvenanceAnswer, Reason: "host projection does not answer provenance queries"},
			{Kind: ObservationLiveProcessContinuity, Reason: "host projection does not prove live-process continuity"},
			{Kind: ObservationVMStateManifest, Reason: "host projection does not classify VM launch metadata"},
			{Kind: ObservationPromotionCertificate, Reason: "host projection does not prove promotion certificate"},
		},
	}
}
