package computerversion

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const constructionManifestRelativePath = ".choir-construction/manifest.json"

type ConstructionStateManifest struct {
	Version ComputerVersion `json:"computer_version"`
	Paths   []string        `json:"paths"`
}

func WriteConstructionStateManifest(deviceRoot string, version ComputerVersion, expected ObservationSet) error {
	if !version.Valid() || expected.Version != version {
		return fmt.Errorf("construction state manifest: valid matching ComputerVersion is required")
	}
	paths := make([]string, 0)
	for _, observation := range expected.Observations {
		if observation.Kind == ObservationFileManifest {
			paths = append(paths, observation.Key)
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("construction state manifest: file paths are required")
	}
	sort.Strings(paths)
	manifest := ConstructionStateManifest{Version: version, Paths: paths}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("construction state manifest: encode: %w", err)
	}
	manifestPath := filepath.Join(deviceRoot, filepath.FromSlash(constructionManifestRelativePath))
	if _, err := os.Lstat(manifestPath); err == nil {
		return fmt.Errorf("construction state manifest: reserved path already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, encoded, 0o600); err != nil {
		return fmt.Errorf("construction state manifest: write: %w", err)
	}
	return nil
}

// ObserveConstructionState reads only constructor-declared user-file paths.
// Extra runtime/cache files on the persistent device cannot widen or mask the
// semantic readback set.
func ObserveConstructionState(ctx context.Context, deviceRoot, filesRoot string, version ComputerVersion) (ObservationSet, error) {
	manifestPath := filepath.Join(deviceRoot, filepath.FromSlash(constructionManifestRelativePath))
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("construction state manifest: read: %w", err)
	}
	var manifest ConstructionStateManifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return ObservationSet{}, fmt.Errorf("construction state manifest: decode: %w", err)
	}
	if manifest.Version != version || !version.Valid() || len(manifest.Paths) == 0 {
		return ObservationSet{}, fmt.Errorf("construction state manifest: ComputerVersion or paths mismatch")
	}
	root, err := filepath.Abs(strings.TrimSpace(filesRoot))
	if err != nil || strings.TrimSpace(filesRoot) == "" {
		return ObservationSet{}, fmt.Errorf("construction state manifest: files root is required")
	}
	return FilesystemPathProjectionObservationSet(ctx, "constructed-product-readback", version, root, manifest.Paths)

}
