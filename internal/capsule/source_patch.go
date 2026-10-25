package capsule

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// errSourcePatchSymlink names why a source change is refused when its
// candidate or base path is, or passes through, a symbolic link.
var errSourcePatchSymlink = errors.New("source patches do not read through or change symbolic links")

// sourcePatchFile is one workspace/platform file a capsule changed, with
// its base bytes (from the pinned source snapshot) and candidate bytes.
type sourcePatchFile struct {
	Rel       string
	Old       []byte
	OldExists bool
	New       []byte
	NewExists bool
}

// collectSourcePatchFiles reads, for every workspace/platform file change,
// the base bytes from the pinned source snapshot (<mergedDir>/../source-lower)
// and the candidate bytes from the merged tree. A directory cannot be
// expressed in a unified diff (the files inside it carry it), so directory
// changes are skipped.
//
// Both sides are read beneath their root without following a symbolic link
// in any component (readSourceBeneath), and a change whose recorded mode is
// a symlink is refused by name, whatever its kind. The merged tree is one a
// cell can write: reading it by path would follow a link the cell planted
// and carry bytes from outside the pinned source into source.patch, the
// bundle draft, the verifiers' mounts and the host builder. The patch
// renders every file as a regular file, and the builder refuses any patch
// that changes a symbolic link, so a symlink on either side is refused
// here, with errSourcePatchSymlink, rather than rendered.
//
// Stated limitation: a source file that is a symbolic link in the pinned
// source cannot be deleted or replaced by a capsule (the repository ships
// CLAUDE.md as one), so such a candidate cannot freeze; editing the file
// the link points at still works, because the overlay records that change
// at the target's own path.
func collectSourcePatchFiles(mergedDir string, changes []FileChange) ([]sourcePatchFile, error) {
	var sourceChanges []FileChange
	for _, change := range changes {
		p := change.Path
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		if strings.HasPrefix(p, "/workspace/platform/") && !change.Mode.IsDir() {
			sourceChanges = append(sourceChanges, change)
		}
	}
	lowerRoot := filepath.Join(mergedDir, "..", "source-lower")
	var files []sourcePatchFile
	for _, change := range sourceChanges {
		rel := strings.TrimPrefix(change.Path, "/")
		rel = strings.TrimPrefix(rel, "workspace/platform/")
		if !sourceRelOK(rel) {
			// Refused, never skipped: a skipped change would leave the
			// patch understating the bundle's ordered file effects.
			return nil, fmt.Errorf("capsule source patch refuses %q: %w", change.Path, errUnsafeSourcePath)
		}
		if change.Mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("capsule source patch refuses %s %q: it is a symbolic link: %w", change.Kind, change.Path, errSourcePatchSymlink)
		}
		sourceRel := platformSourcePrefix + "/" + rel
		file := sourcePatchFile{Rel: rel}
		old, oldIsDir, oldErr := readSourceBeneath(lowerRoot, sourceRel)
		switch {
		case oldErr == nil && oldIsDir:
			// A directory replaced by a file: no base file at this path.
		case oldErr == nil:
			file.Old, file.OldExists = old, true
		case errors.Is(oldErr, errSourcePatchSymlink) && change.Kind == ChangeDeleted:
			return nil, fmt.Errorf("capsule source patch cannot delete %q: it is a symbolic link in the pinned source: %w", change.Path, errSourcePatchSymlink)
		case errors.Is(oldErr, errSourcePatchSymlink):
			return nil, fmt.Errorf("capsule source patch cannot change %q: the pinned source path is or passes through a symbolic link: %w", change.Path, errSourcePatchSymlink)
		case errors.Is(oldErr, os.ErrNotExist) || errors.Is(oldErr, syscall.ENOTDIR):
			// Not in the base (ENOTDIR: a base file stands where the
			// candidate has a directory).
		default:
			return nil, fmt.Errorf("capsule source patch reads base %q: %w", change.Path, oldErr)
		}
		if change.Kind != ChangeDeleted {
			data, isDir, err := readSourceBeneath(mergedDir, sourceRel)
			switch {
			case errors.Is(err, errSourcePatchSymlink):
				return nil, fmt.Errorf("capsule source patch refuses %q: the candidate path is or passes through a symbolic link: %w", change.Path, errSourcePatchSymlink)
			case err != nil:
				return nil, fmt.Errorf("capsule source patch reads %q: %w", change.Path, err)
			case isDir:
				return nil, fmt.Errorf("capsule source patch reads %q: a directory where the change names a file", change.Path)
			}
			file.New, file.NewExists = data, true
		}
		files = append(files, file)
	}
	return files, nil
}

// noNewlineMarker is git's marker for a last line without a final newline.
const noNewlineMarker = "\\ No newline at end of file\n"

// patchLines splits content into unified-diff lines. Unlike
// difflib.SplitLines it adds no phantom trailing line: empty content is
// zero lines, and an unterminated last line carries git's marker, so a
// creation hunk reads "@@ -0,0 +1,N @@" and the applied bytes match.
func patchLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(content), "\n")
	if last := lines[len(lines)-1]; last == "" {
		lines = lines[:len(lines)-1]
	} else {
		lines[len(lines)-1] = last + "\n" + noNewlineMarker
	}
	return lines
}

// renderSourcePatch renders a git-applicable unified diff of files, in
// order. Unchanged files are omitted; an empty added or deleted file is
// carried by its git header alone.
func renderSourcePatch(files []sourcePatchFile) ([]byte, error) {
	var buf bytes.Buffer
	for _, f := range files {
		if !f.OldExists && !f.NewExists {
			continue
		}
		if f.OldExists && f.NewExists && bytes.Equal(f.Old, f.New) {
			continue
		}
		fromFile, toFile := "a/"+f.Rel, "b/"+f.Rel
		fmt.Fprintf(&buf, "diff --git %s %s\n", fromFile, toFile)
		switch {
		case !f.OldExists:
			fromFile = "/dev/null"
			buf.WriteString("new file mode 100644\n")
		case !f.NewExists:
			toFile = "/dev/null"
			buf.WriteString("deleted file mode 100644\n")
		}
		var oldBytes, newBytes []byte
		if f.OldExists {
			oldBytes = f.Old
		}
		if f.NewExists {
			newBytes = f.New
		}
		if len(oldBytes) == 0 && len(newBytes) == 0 {
			continue
		}
		ud, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        patchLines(oldBytes),
			B:        patchLines(newBytes),
			FromFile: fromFile,
			ToFile:   toFile,
			Context:  3,
		})
		if err != nil {
			return nil, fmt.Errorf("capsule source patch %q: %w", f.Rel, err)
		}
		buf.WriteString(ud)
	}
	return buf.Bytes(), nil
}

// verifySourcePatch proves the patch reproduces the candidate: it applies
// the patch with git apply (the builder's applier) to a scratch tree of
// the base files and requires every changed file's bytes to match, and
// every deleted file to be gone.
func verifySourcePatch(ctx context.Context, patch []byte, files []sourcePatchFile) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	scratch, err := os.MkdirTemp("", "choir-source-patch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	tree := filepath.Join(scratch, "tree")
	git := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = tree
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], runErr, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := os.MkdirAll(tree, 0o755); err != nil {
		return err
	}
	// A repository root pins git apply's paths to the scratch tree even
	// when the temp dir sits inside another checkout.
	if err := git("init", "-q"); err != nil {
		return err
	}
	for _, f := range files {
		if !f.OldExists {
			continue
		}
		p := filepath.Join(tree, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.Old, 0o644); err != nil {
			return err
		}
	}
	patchPath := filepath.Join(scratch, "source.patch")
	if err := os.WriteFile(patchPath, patch, 0o644); err != nil {
		return err
	}
	if err := git("apply", patchPath); err != nil {
		return fmt.Errorf("capsule source patch does not apply: %w", err)
	}
	for _, f := range files {
		got, readErr := os.ReadFile(filepath.Join(tree, filepath.FromSlash(f.Rel)))
		switch {
		case !f.NewExists:
			if !errors.Is(readErr, os.ErrNotExist) {
				return fmt.Errorf("capsule source patch leaves deleted %q in place", f.Rel)
			}
		case readErr != nil:
			return fmt.Errorf("capsule source patch does not create %q: %w", f.Rel, readErr)
		case !bytes.Equal(got, f.New):
			return fmt.Errorf("capsule source patch does not reproduce %q (%d bytes, candidate %d)", f.Rel, len(got), len(f.New))
		}
	}
	return nil
}
