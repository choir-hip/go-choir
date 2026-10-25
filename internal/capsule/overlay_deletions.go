package capsule

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The overlay upper records what the capsule changed, but a deletion is not
// a file: removing a lower-layer file leaves a whiteout (a 0:0 character
// device), removing and recreating a lower directory leaves an opaque
// directory (the overlay opaque xattr), and replacing a lower directory with
// a file leaves just the file. The manifest walk reports the marker; this
// file turns a marker over the pinned workspace/platform source into the
// per-file deletions the source patch, the bundle's ordered file effects and
// the digest need.
//
// Which of those layers is the pinned source is fixed by the mount:
// <state>/source-lower/workspace/platform is the lower layer a capsule
// source path resolves to (executor.go: lowerLayers = sourceLower + ":" +
// e.lowerDir). Only that tree is expanded. The guest root layer below it is
// never walked: the spawn's own opaque release dir lives there and must
// stay invisible to the change list (capsule_release_dir_test.go, failure
// mode 4).

// platformSourcePrefix is where the pinned source appears in a capsule.
const platformSourcePrefix = "workspace/platform"

// maxOverlayDeletions bounds how many deletions one expansion may add. The
// pinned source has far fewer files; a count past this is a runaway.
const maxOverlayDeletions = 100000

// expandOverlayDeletions returns changes plus a ChangeDeleted for every file
// of the pinned source (lowerPlatform) that a change shows the capsule
// removed without naming it:
//
//   - a whiteout over a lower directory: every file under it;
//   - an opaque directory over a lower directory: every lower file under
//     it that the upper does not hold again (a file the upper holds again
//     is already an added or modified change);
//   - an opaque directory over a lower file: that file;
//   - a file or symlink over a lower directory: every file under it.
//
// A marker on workspace itself, a strict ancestor of the pinned source,
// expands as one on workspace/platform would: every file of the pinned
// source the upper does not hold again.
//
// upperDir is the overlay upper (the capsule's own writes), whose
// workspace/platform tree mirrors the source. A path that already has a
// change is never added twice. The result is sorted by path.
func expandOverlayDeletions(ctx context.Context, changes []FileChange, upperDir, lowerPlatform string) ([]FileChange, error) {
	if lowerPlatform == "" || upperDir == "" {
		return changes, nil
	}
	have := make(map[string]bool, len(changes))
	for _, c := range changes {
		// A directory change does not name a file: an opaque directory
		// over a lower file still deletes that file.
		if !c.Mode.IsDir() {
			have[strings.TrimPrefix(c.Path, "/")] = true
		}
	}
	var extra []FileChange
	add := func(rel string, mode os.FileMode) error {
		path := platformSourcePrefix + "/" + rel
		if have[path] {
			return nil
		}
		if len(extra) >= maxOverlayDeletions {
			return fmt.Errorf("capsule deletes more than %d source files; refusing to freeze a change list that large", maxOverlayDeletions)
		}
		have[path] = true
		extra = append(extra, FileChange{Path: path, Kind: ChangeDeleted, Mode: mode})
		return nil
	}
	// expandDir adds a deletion for every non-directory under the lower
	// directory rel that the upper does not hold as a non-directory.
	expandDir := func(rel string) error {
		root := filepath.Join(lowerPlatform, filepath.FromSlash(rel))
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			sub, err := filepath.Rel(lowerPlatform, path)
			if err != nil {
				return err
			}
			sub = filepath.ToSlash(sub)
			if info, statErr := os.Lstat(filepath.Join(upperDir, filepath.FromSlash(platformSourcePrefix), filepath.FromSlash(sub))); statErr == nil && !info.IsDir() {
				// Held again (a change of its own) or already a whiteout.
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return add(sub, info.Mode())
		})
	}
	for _, c := range changes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := strings.TrimPrefix(c.Path, "/")
		var rel string
		switch {
		case path == platformSourcePrefix:
			rel = "."
		case path != "" && strings.HasPrefix(platformSourcePrefix, path+"/"):
			// A marker on a strict ancestor (workspace): a whiteout or a
			// file there, or an opaque workspace, hides the whole pinned
			// source, exactly as the same marker on workspace/platform.
			rel = "."
		case strings.HasPrefix(path, platformSourcePrefix+"/"):
			rel = strings.TrimPrefix(path, platformSourcePrefix+"/")
		default:
			continue
		}
		if rel != "." && (strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../")) {
			continue
		}
		var trigger bool
		switch {
		case c.Kind == ChangeDeleted:
			// The walk's fact, not the mode: a deletion derived from a
			// committed entry that is gone names no lower tree.
			trigger = c.Whiteout
		case c.Mode.IsDir():
			trigger = c.Opaque
		default:
			trigger = true // a file or symlink may stand where a directory was
		}
		if !trigger {
			continue
		}
		info, err := os.Lstat(filepath.Join(lowerPlatform, filepath.FromSlash(rel)))
		if err != nil {
			continue // nothing in the pinned source at this path
		}
		switch {
		case info.IsDir():
			if err := expandDir(rel); err != nil {
				return nil, err
			}
		case c.Kind != ChangeDeleted && c.Mode.IsDir() && c.Opaque:
			// A lower file removed and a directory made in its place.
			if err := add(rel, info.Mode()); err != nil {
				return nil, err
			}
		}
	}
	if len(extra) == 0 {
		return changes, nil
	}
	out := append(append([]FileChange(nil), changes...), extra...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
