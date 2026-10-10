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
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// sourcePatchFile is one workspace/platform file a capsule changed, with
// its base bytes (from the pinned source snapshot) and candidate bytes.
type sourcePatchFile struct {
	Rel       string
	Old       []byte
	OldExists bool
	New       []byte
	NewExists bool
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
