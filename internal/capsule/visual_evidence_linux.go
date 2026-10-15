//go:build linux

package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// stageVisualEvidence copies the capsule's evidence directory into
// <temporary>/evidence/visual/ (beside release/, never under it: V20) and
// validates it against the staged release. It returns nil when the capsule
// captured nothing. Files are opened beneath the capsule root without
// following symlinks, as release files are.
func (e *Executor) stageVisualEvidence(ctx context.Context, rootFD int, agentRunID string, changes []FileChange, temporary string, release []FrozenReleaseFile) (*VisualEvidence, error) {
	var names []string
	for _, change := range changes {
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), visualcapture.EvidencePrefix)
		if !ok || change.Kind == ChangeDeleted || change.Mode.IsDir() {
			continue
		}
		if rel == "" || strings.Contains(rel, "/") || rel == "." || rel == ".." {
			return nil, fmt.Errorf("capsule freeze: visual evidence path %q is not a file directly in %s", change.Path, visualcapture.OutputDir)
		}
		names = append(names, rel)
	}
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) > visualcapture.MaxRoutes+1 {
		return nil, fmt.Errorf("capsule freeze: the visual evidence directory holds %d files; at most %d captures and the manifest", len(names), visualcapture.MaxRoutes)
	}
	target := filepath.Join(temporary, filepath.FromSlash(visualcapture.BundleDir))
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nil, err
	}
	var files []visualcapture.EvidenceFile
	var manifest []byte
	var total int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fd, err := unix.Openat2(rootFD, visualcapture.EvidencePrefix+name, &unix.OpenHow{
			Flags:   uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK),
			Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
		})
		if err != nil {
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is unavailable: %w", name, err)
		}
		input := os.NewFile(uintptr(fd), name)
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = input.Close()
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is not a regular file", name)
		}
		data, err := io.ReadAll(io.LimitReader(input, visualcapture.MaxCaptureBytes+1))
		_ = input.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > visualcapture.MaxCaptureBytes {
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is larger than %d bytes", name, visualcapture.MaxCaptureBytes)
		}
		total += int64(len(data))
		if total > visualcapture.MaxTotalBytes+visualcapture.MaxManifestBytes {
			return nil, fmt.Errorf("capsule freeze: visual evidence exceeds %d bytes", visualcapture.MaxTotalBytes)
		}
		sum := sha256.Sum256(data)
		files = append(files, visualcapture.EvidenceFile{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
		if name == visualcapture.ManifestFile {
			manifest = data
		}
		if err := os.WriteFile(filepath.Join(target, name), data, 0o444); err != nil {
			return nil, err
		}
	}
	return buildVisualEvidence(manifest, files, release, e.JobViews(agentRunID))
}
