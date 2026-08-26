package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScrubReport records the findings of an artifact storage integrity scrub.
type ScrubReport struct {
	ScannedBlobs   int       `json:"scanned_blobs"`
	CorruptedBlobs int       `json:"corrupted_blobs"`
	ScannedBytes   int64     `json:"scanned_bytes"`
	Errors         []string  `json:"errors,omitempty"`
	DurationMs     int64     `json:"duration_ms"`
	Timestamp      time.Time `json:"timestamp"`
}

// IntegrityScrubber continuously or periodically verifies that on-disk blobs
// match their content-addressed SHA-256 digests.
type IntegrityScrubber struct {
	artifactsRoot string
}

// NewIntegrityScrubber creates a new scrubber for the given artifacts root.
func NewIntegrityScrubber(artifactsRoot string) *IntegrityScrubber {
	return &IntegrityScrubber{artifactsRoot: filepath.Clean(artifactsRoot)}
}

// ScrubNamespaces walks the specified namespaces and verifies all blob digests.
func (s *IntegrityScrubber) ScrubNamespaces(ctx context.Context, namespaces []string) (*ScrubReport, error) {
	if s == nil || s.artifactsRoot == "" {
		return nil, fmt.Errorf("scrubber: uninitialized")
	}
	if len(namespaces) == 0 {
		namespaces = []string{"computer-event-payload", "computer-event", "projection-base", "file-cas-chunks", "file-cas-roots"}
	}

	start := time.Now()
	report := &ScrubReport{
		Timestamp: start.UTC(),
	}

	for _, ns := range namespaces {
		nsDir := filepath.Join(s.artifactsRoot, "sha256", ns)
		if _, err := os.Stat(nsDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(nsDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			expectedDigest := filepath.Base(path)
			if strings.HasPrefix(expectedDigest, ".") {
				return nil
			}

			report.ScannedBlobs++
			report.ScannedBytes += info.Size()

			file, err := os.Open(path)
			if err != nil {
				report.CorruptedBlobs++
				report.Errors = append(report.Errors, fmt.Sprintf("%s: open failed: %v", path, err))
				return nil
			}
			defer file.Close()

			hasher := sha256.New()
			if _, err := io.Copy(hasher, file); err != nil {
				report.CorruptedBlobs++
				report.Errors = append(report.Errors, fmt.Sprintf("%s: read failed: %v", path, err))
				return nil
			}

			actualDigest := hex.EncodeToString(hasher.Sum(nil))
			if actualDigest != expectedDigest {
				report.CorruptedBlobs++
				report.Errors = append(report.Errors, fmt.Sprintf("%s: digest mismatch (got %s, want %s)", path, actualDigest, expectedDigest))
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			report.Errors = append(report.Errors, fmt.Sprintf("namespace %s walk error: %v", ns, err))
		}
	}

	report.DurationMs = time.Since(start).Milliseconds()
	return report, nil
}
