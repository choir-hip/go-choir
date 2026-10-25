package capsule

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
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

// errOverlayBoundaryNotDirectory: at workspace/platform or a strict
// ancestor, a change other than a whiteout or a directory cannot be
// expanded into deletions.
var errOverlayBoundaryNotDirectory = errors.New("only a whiteout or a directory can stand at or above the pinned source")

// errOverlayUpperSymlink: a probe of the upper met a symlink in a
// non-final component.
var errOverlayUpperSymlink = errors.New("the overlay upper holds a symbolic link above a pinned source path")

// errUnsafeSourcePath: a change path with an empty, "." or ".." component.
var errUnsafeSourcePath = errors.New("source path has an empty, \".\" or \"..\" component")

// maxOverlayDeletions bounds how many deletions one expansion may add. The
// pinned source has far fewer files; a count past this is a runaway.
const maxOverlayDeletions = 100000

// sourceRelOK reports whether rel (slash-separated) has only real name
// components: no empty, "." or ".." component. "..x" is a name like any
// other.
func sourceRelOK(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// upperHolds reports whether the overlay upper holds a non-directory (a
// change of its own, or a whiteout) at workspace/platform/sub. It Lstats
// every component and never follows a symbolic link: a link above sub is
// refused with errOverlayUpperSymlink, because what it serves is not the
// capsule's change, and a whiteout or file above sub hides it (not held).
func upperHolds(upperDir, sub string) (bool, error) {
	parts := strings.Split(platformSourcePrefix+"/"+sub, "/")
	p := upperDir
	for i, part := range parts {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if i == len(parts)-1 {
			return !info.IsDir(), nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("%s: %w", strings.Join(parts[:i+1], "/"), errOverlayUpperSymlink)
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return false, nil
}

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
// source the upper does not hold again. At either, only a whiteout or a
// directory is accepted; anything else (a symlink above all) refuses the
// freeze with errOverlayBoundaryNotDirectory. A change path with a "." or
// ".." component refuses it with errUnsafeSourcePath.
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
			held, err := upperHolds(upperDir, sub)
			if err != nil {
				return err
			}
			if held {
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
		if rel == "." {
			// At the pinned source or above it only a whiteout or a
			// directory is representable. A symlink there serves whatever
			// tree it points at: the walk never descends it, and nothing
			// read through it is the capsule's change. A file or device is
			// refused too rather than guessed at.
			if !(c.Kind == ChangeDeleted && c.Whiteout) && !c.Mode.IsDir() {
				return nil, fmt.Errorf("capsule change %q (%s, mode %s): %w", c.Path, c.Kind, c.Mode, errOverlayBoundaryNotDirectory)
			}
		} else if !sourceRelOK(rel) {
			return nil, fmt.Errorf("capsule change %q: %w", c.Path, errUnsafeSourcePath)
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
