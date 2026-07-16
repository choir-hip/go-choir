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

type projectedFile struct {
	path   string
	size   int64
	digest string
}

// FilesystemProjectionObservationSet derives the deterministic expected output
// of generation. It is not acceptance evidence: acceptance must independently
// return the same schema through ConstructedLauncher.Observe's product path.
func FilesystemProjectionObservationSet(ctx context.Context, name string, version ComputerVersion, root string) (ObservationSet, error) {
	if err := validateProjectionRequest(ctx, version, root); err != nil {
		return ObservationSet{}, err
	}
	files := make([]projectedFile, 0)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := projectRegularFile(path, filepath.ToSlash(relative), entry)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	}); err != nil {
		return ObservationSet{}, err
	}
	return encodeFilesystemProjection(name, version, files)
}

// FilesystemPathProjectionObservationSet observes exactly paths and ignores
// unrelated runtime/cache state sharing the persistent device.
func FilesystemPathProjectionObservationSet(ctx context.Context, name string, version ComputerVersion, root string, paths []string) (ObservationSet, error) {
	if err := validateProjectionRequest(ctx, version, root); err != nil {
		return ObservationSet{}, err
	}
	files := make([]projectedFile, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, relative := range paths {
		if err := ctx.Err(); err != nil {
			return ObservationSet{}, err
		}
		clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return ObservationSet{}, fmt.Errorf("filesystem projection: unsafe path %q", relative)
		}
		key := filepath.ToSlash(clean)
		if _, exists := seen[key]; exists {
			return ObservationSet{}, fmt.Errorf("filesystem projection: duplicate path %q", key)
		}
		seen[key] = struct{}{}
		path, entry, err := secureProjectionPath(root, clean)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("filesystem projection: inspect %q: %w", key, err)
		}
		file, err := projectRegularFile(path, key, fileInfoDirEntry{entry})
		if err != nil {
			return ObservationSet{}, err
		}
		files = append(files, file)
	}
	return encodeFilesystemProjection(name, version, files)
}

func validateProjectionRequest(ctx context.Context, version ComputerVersion, root string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !version.Valid() {
		return fmt.Errorf("filesystem projection: invalid ComputerVersion")
	}
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("filesystem projection: root is required")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("filesystem projection: root is not a directory")
	}
	return nil
}

func secureProjectionPath(root, relative string) (string, os.FileInfo, error) {
	current := root
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("symlink component is not admissible")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", nil, fmt.Errorf("parent component is not a directory")
		}
		if i == len(parts)-1 {
			return current, info, nil
		}
	}
	return "", nil, fmt.Errorf("empty projection path")
}

func projectRegularFile(path, relative string, entry fs.DirEntry) (projectedFile, error) {
	if entry.Type()&os.ModeSymlink != 0 {
		return projectedFile{}, fmt.Errorf("filesystem projection: symlink %q is not admissible", path)
	}
	info, err := entry.Info()
	if err != nil {
		return projectedFile{}, err
	}
	if !info.Mode().IsRegular() {
		return projectedFile{}, fmt.Errorf("filesystem projection: non-regular entry %q is not admissible", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return projectedFile{}, err
	}
	digest := sha256.Sum256(data)
	return projectedFile{path: relative, size: info.Size(), digest: hex.EncodeToString(digest[:])}, nil
}

func encodeFilesystemProjection(name string, version ComputerVersion, files []projectedFile) (ObservationSet, error) {
	if len(files) == 0 {
		return ObservationSet{}, fmt.Errorf("filesystem projection: no files")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	observations := make([]Observation, 0, len(files)*2)
	seenBlobs := make(map[string]struct{}, len(files))
	for _, file := range files {
		blobRef := "sha256:" + file.digest
		manifest, err := json.Marshal(struct {
			Path    string `json:"path"`
			Size    int64  `json:"size"`
			SHA256  string `json:"sha256"`
			BlobRef string `json:"blob_ref"`
		}{Path: file.path, Size: file.size, SHA256: file.digest, BlobRef: blobRef})
		if err != nil {
			return ObservationSet{}, err
		}
		observations = append(observations, Observation{Kind: ObservationFileManifest, Key: file.path, Value: string(manifest)})
		if _, exists := seenBlobs[blobRef]; exists {
			continue
		}
		seenBlobs[blobRef] = struct{}{}
		blobObservation, err := json.Marshal(struct {
			BlobRef   string `json:"blob_ref"`
			SizeBytes int64  `json:"size_bytes"`
			SHA256    string `json:"sha256"`
		}{BlobRef: blobRef, SizeBytes: file.size, SHA256: file.digest})
		if err != nil {
			return ObservationSet{}, err
		}
		observations = append(observations, Observation{Kind: ObservationBlobSet, Key: blobRef, Value: string(blobObservation)})
	}
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Kind != observations[j].Kind {
			return observations[i].Kind < observations[j].Kind
		}
		return observations[i].Key < observations[j].Key
	})
	return ObservationSet{Name: name, Version: version, Required: []ObservationKind{ObservationFileManifest, ObservationBlobSet}, Observations: observations}, nil
}

type fileInfoDirEntry struct{ os.FileInfo }

func (entry fileInfoDirEntry) Type() fs.FileMode          { return entry.Mode().Type() }
func (entry fileInfoDirEntry) Info() (os.FileInfo, error) { return entry.FileInfo, nil }
