//go:build linux

package capsule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

// refuseClaudeTokenInChanges is Freeze's check for a claude-mode capsule:
// no changed file may carry the delegate token, or a candidate (and later a
// public commit) would leak it (F6). It scans with the token the capsule
// was spawned with, reads the overlay upper directly and never follows a
// symlink in any component (openBeneath).
func (e *Executor) refuseClaudeTokenInChanges(ctx context.Context, caps *Capsule, changes []FileChange) error {
	secret := caps.claudeToken
	if len(secret) == 0 {
		return fmt.Errorf("claude-mode capsule %s has no delegate token to scan the candidate for; refusing to freeze unchecked", caps.ID)
	}
	rootFD, err := unix.Open(caps.UpperDir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("claude-mode capsule %s upper is unavailable to scan: %w", caps.ID, err)
	}
	defer unix.Close(rootFD)
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if change.Kind == ChangeDeleted || change.Mode.IsDir() {
			continue
		}
		rel := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(change.Path, "/")))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("capsule change has an unsafe path %q", change.Path)
		}
		file, err := openBeneath(rootFD, filepath.ToSlash(rel))
		if err != nil {
			// A symlink (at or above the leaf), whiteout or vanished
			// entry carries no file bytes of the upper's own.
			continue
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = file.Close()
			continue
		}
		found, err := claudedelegate.ContainsSecret(file, secret)
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("scan %s for the delegate token: %w", change.Path, err)
		}
		if found {
			return fmt.Errorf("capsule change %s contains the Claude delegate token; the candidate is refused", change.Path)
		}
	}
	return nil
}
