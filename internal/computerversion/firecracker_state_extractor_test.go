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
	// FirecrackerStateMaterializer names the realization produced from reading
	// a Firecracker VM's persistent directory state.
	FirecrackerStateMaterializer = "firecracker-state-extractor"
	// FirecrackerStateSubstrate names the Firecracker persistent-directory
	// substrate for file-manifest/blob-set extraction.
	FirecrackerStateSubstrate = "firecracker/persistent-dir"
)

// FirecrackerStateExtractor reads file_manifest and blob_set observations from
// a Firecracker VM's persistent directory. It does NOT launch, stop, resume,
// copy, or mutate a VM. It performs read-only filesystem traversal of the
// persistent directory and computes content hashes for blob-set observations.
//
// TEST SCAFFOLDING ONLY. This extractor lives in a _test.go file so it cannot
// link into production code. Its sole role is the round-trip inverse of the
// state generator: proving that Generate wrote to the filesystem exactly what
// the typed tape said. Host-side filesystem extraction is not admissible
// acceptance evidence — production verification must read state back through
// the authenticated product path (see the audited-autoputer Definition,
// not_done_when).
//
// The extractor interprets each file in the persistent directory as a
// file_manifest entry (path, size, content hash) and each unique content hash
// as a blob_set entry. This produces the same observation structure as the
// Base journal/tree extractors, but from a different substrate path.
type FirecrackerStateExtractor struct {
	// PersistentDir is the root directory to read from. Must be non-empty.
	PersistentDir string
}

var _ Extractor = FirecrackerStateExtractor{}

// Extract walks the persistent directory and produces file_manifest and
// blob_set observations for request.Version.
func (e FirecrackerStateExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if !request.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: invalid computer version")
	}
	root := strings.TrimSpace(e.PersistentDir)
	if root == "" {
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: persistent dir is required")
	}
	info, err := os.Stat(root)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: stat %q: %w", root, err)
	}
	if !info.IsDir() {
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: %q is not a directory", root)
	}

	type fileEntry struct {
		relPath  string
		size     int64
		sha256   string
		blobRef  string
	}

	files := make([]fileEntry, 0)
	blobHashes := make(map[string]string) // sha256 -> blobRef
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
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: %w", walkErr)
	}
	if len(files) == 0 {
		return ObservationSet{}, fmt.Errorf("firecracker state extraction: no files in %q", root)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].relPath < files[j].relPath })

	observations := make([]Observation, 0, len(files)+len(blobHashes))

	// file_manifest observations
	for _, f := range files {
		entry := firecrackerFileManifestEntry{
			Path:     f.relPath,
			Size:     f.size,
			SHA256:   f.sha256,
			BlobRef:  f.blobRef,
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("firecracker state extraction: encode %q: %w", f.relPath, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationFileManifest,
			Key:   f.relPath,
			Value: string(encoded),
		})
	}

	// blob_set observations (unique by sha256)
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
			SizeBytes: 0, // will be set from the first file that references this blob
		}
		// find the size from the first file with this hash
		for _, f := range files {
			if f.sha256 == sha {
				entry.SizeBytes = f.size
				break
			}
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("firecracker state extraction: encode blob %q: %w", ref, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationBlobSet,
			Key:   ref,
			Value: string(encoded),
		})
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "firecracker-state"
	}
	return ObservationSet{
		Name:         name,
		Version:      request.Version,
		Required:     []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Observations: observations,
	}, nil
}

// FirecrackerStateCapabilityManifest declares the observation scope for the
// Firecracker persistent-directory extractor.
func FirecrackerStateCapabilityManifest(materializer, substrate string) CapabilityManifest {
	materializer = strings.TrimSpace(materializer)
	if materializer == "" {
		materializer = FirecrackerStateMaterializer
	}
	substrate = strings.TrimSpace(substrate)
	if substrate == "" {
		substrate = FirecrackerStateSubstrate
	}
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    substrate,
		Supported:    []ObservationKind{ObservationFileManifest, ObservationBlobSet},
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationDoltHead, Reason: "firecracker persistent dir does not prove Dolt ledger head equivalence"},
			{Kind: ObservationObjectGraphHead, Reason: "firecracker persistent dir does not prove object graph head equivalence"},
			{Kind: ObservationProvenanceAnswer, Reason: "firecracker persistent dir does not answer provenance queries"},
			{Kind: ObservationLiveProcessContinuity, Reason: "firecracker state extraction does not prove live-process continuity"},
			{Kind: ObservationVMStateManifest, Reason: "firecracker state extraction does not classify VM launch metadata"},
			{Kind: ObservationPromotionCertificate, Reason: "firecracker state extraction does not prove promotion certificate"},
		},
	}
}

type firecrackerFileManifestEntry struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	BlobRef string `json:"blob_ref"`
}

type firecrackerBlobSetEntry struct {
	BlobRef   string `json:"blob_ref"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}
