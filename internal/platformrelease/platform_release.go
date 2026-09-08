package platformrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidReleaseID       = errors.New("platformrelease: release_id is required")
	ErrInvalidPlatformBaseRef = errors.New("platformrelease: platform_base_ref is required")
	ErrMissingComponents      = errors.New("platformrelease: components must not be empty")
)

// ComputeContentDigest calculates a deterministic SHA-256 digest over the canonical
// fields of a PlatformRelease.
func ComputeContentDigest(pr *PlatformRelease) string {
	if pr == nil {
		return ""
	}

	// Make a shallow copy and clone sorted components for deterministic hashing
	sortedComponents := make([]ComponentRelease, len(pr.Components))
	copy(sortedComponents, pr.Components)
	sort.Slice(sortedComponents, func(i, j int) bool {
		return sortedComponents[i].ComponentID < sortedComponents[j].ComponentID
	})

	canonical := struct {
		SchemaVersion               string             `json:"schema_version"`
		ReleaseID                   string             `json:"release_id"`
		PlatformBaseRef             string             `json:"platform_base_ref"`
		GuestKernelDigest           string             `json:"guest_kernel_digest,omitempty"`
		GuestImageDigest            string             `json:"guest_image_digest,omitempty"`
		AutoputerRuntimeDigest      string             `json:"autoputer_runtime_digest,omitempty"`
		FrontendAssetManifestDigest string             `json:"frontend_asset_manifest_digest,omitempty"`
		FrontendAssetsRef           string             `json:"frontend_assets_ref,omitempty"`
		MaildSchemaVersion          int                `json:"maild_schema_version,omitempty"`
		EventSchemaVersion          uint64             `json:"event_schema_version"`
		ReducerVersion              uint64             `json:"reducer_version"`
		Components                  []ComponentRelease `json:"components"`
	}{
		SchemaVersion:               pr.SchemaVersion,
		ReleaseID:                   pr.ReleaseID,
		PlatformBaseRef:             pr.PlatformBaseRef,
		GuestKernelDigest:           pr.GuestKernelDigest,
		GuestImageDigest:            pr.GuestImageDigest,
		AutoputerRuntimeDigest:      pr.AutoputerRuntimeDigest,
		FrontendAssetManifestDigest: pr.FrontendAssetManifestDigest,
		FrontendAssetsRef:           pr.FrontendAssetsRef,
		MaildSchemaVersion:          pr.MaildSchemaVersion,
		EventSchemaVersion:          pr.EventSchemaVersion,
		ReducerVersion:              pr.ReducerVersion,
		Components:                  sortedComponents,
	}

	raw, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(h[:])
}

// Validate checks that required fields are present and valid.
func Validate(pr *PlatformRelease) error {
	if pr == nil {
		return errors.New("platformrelease: release is nil")
	}
	if strings.TrimSpace(pr.ReleaseID) == "" {
		return ErrInvalidReleaseID
	}
	if strings.TrimSpace(pr.PlatformBaseRef) == "" {
		return ErrInvalidPlatformBaseRef
	}
	if len(pr.Components) == 0 {
		return ErrMissingComponents
	}
	for i, c := range pr.Components {
		if strings.TrimSpace(c.ComponentID) == "" {
			return fmt.Errorf("platformrelease: component %d has empty component_id", i)
		}
		if strings.TrimSpace(c.Version) == "" {
			return fmt.Errorf("platformrelease: component %s has empty version", c.ComponentID)
		}
	}
	return nil
}

// GenerateBaselineRelease creates a default PlatformRelease for a given commit.
func GenerateBaselineRelease(commitSHA string, timestamp time.Time) *PlatformRelease {
	cleanSHA := strings.TrimSpace(commitSHA)
	shortSHA := cleanSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	dateStr := timestamp.Format("20060102")
	releaseID := fmt.Sprintf("pr-%s-%s", dateStr, shortSHA)
	platformRef := fmt.Sprintf("refs/heads/main@%s", cleanSHA)

	defaultComponents := []ComponentRelease{
		{
			ComponentID:        "desktop_shell",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
		{
			ComponentID:        "email",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
		{
			ComponentID:        "texture",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
		{
			ComponentID:        "settings",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
		{
			ComponentID:        "maild",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
		{
			ComponentID:        "autoputer",
			Version:            shortSHA,
			ArtifactRef:        "commit:" + cleanSHA,
			DivergenceTrack:    TrackPlatform,
			CompatibilityRange: ">=0.1.0",
		},
	}

	pr := &PlatformRelease{
		SchemaVersion:      CurrentSchemaVersion,
		ReleaseID:          releaseID,
		PlatformBaseRef:    platformRef,
		CreatedAt:          timestamp,
		FrontendAssetsRef:  cleanSHA,
		MaildSchemaVersion: 1,
		EventSchemaVersion: 1,
		ReducerVersion:     1,
		Components:         defaultComponents,
	}

	pr.ContentDigest = ComputeContentDigest(pr)
	return pr
}

// Save writes the PlatformRelease formatted as JSON to the destination path.
func Save(pr *PlatformRelease, destPath string) error {
	if err := Validate(pr); err != nil {
		return err
	}
	if pr.ContentDigest == "" {
		pr.ContentDigest = ComputeContentDigest(pr)
	}

	raw, err := json.MarshalIndent(pr, "", "  ")
	if err != nil {
		return fmt.Errorf("platformrelease: marshal JSON: %w", err)
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("platformrelease: mkdir %s: %w", dir, err)
	}

	tmpPath := destPath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0644); err != nil {
		return fmt.Errorf("platformrelease: write file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("platformrelease: rename %s -> %s: %w", tmpPath, destPath, err)
	}
	return nil
}

// Load reads and unmarshals a PlatformRelease from a JSON file.
func Load(srcPath string) (*PlatformRelease, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("platformrelease: read file: %w", err)
	}

	var pr PlatformRelease
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("platformrelease: unmarshal JSON: %w", err)
	}

	if err := Validate(&pr); err != nil {
		return nil, err
	}
	return &pr, nil
}
