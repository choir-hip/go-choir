package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// dropRestoredToBase removes from a diff the files under /workspace/platform
// that the engineer restored to the base: bytes and permission bits equal the
// pinned source's. The overlay upper keeps such a file with a new mtime, so
// it would otherwise count as a source edit for the before/after screen
// rules, and the refusal's advice ("restore every changed file") would
// never come true. Only the screen rules read the result; the frozen patch
// and bundle keep the full diff (a restored file yields no hunk there
// either: renderSourcePatch skips equal bytes). Anything unreadable stays.
func dropRestoredToBase(changes []FileChange, mergedDir, lowerRoot string) []FileChange {
	out := make([]FileChange, 0, len(changes))
	for _, change := range changes {
		if !restoredToBase(change, mergedDir, lowerRoot) {
			out = append(out, change)
		}
	}
	return out
}

func restoredToBase(change FileChange, mergedDir, lowerRoot string) bool {
	if change.Kind == ChangeDeleted || change.Mode.IsDir() || change.Mode&os.ModeSymlink != 0 {
		return false
	}
	p := strings.TrimPrefix(change.Path, "/")
	if rel, ok := strings.CutPrefix(p, platformSourcePrefix+"/"); !ok || rel == "" {
		return false
	}
	old, oldIsDir, err := readSourceBeneath(lowerRoot, p)
	if err != nil || oldIsDir {
		return false
	}
	info, err := os.Lstat(filepath.Join(lowerRoot, filepath.FromSlash(p)))
	if err != nil || info.Mode().Perm() != change.Mode.Perm() {
		return false
	}
	cur, curIsDir, err := readSourceBeneath(mergedDir, p)
	if err != nil || curIsDir {
		return false
	}
	return bytes.Equal(old, cur)
}
