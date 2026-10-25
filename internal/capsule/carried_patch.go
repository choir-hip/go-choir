package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Failure 15, problem 1, option A
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md): a failed
// round's work reaches the retry as an inert source patch derived from two
// content-addressed trees (the round's base and the candidate it persisted),
// never as a capsule. Nothing here touches a live capsule or the live
// computer.

// CarriedPatchMaxBytes bounds a carried patch. A larger one is refused, not
// truncated.
const CarriedPatchMaxBytes = 2 << 20

var (
	// ErrCarriedPatchEmpty: the candidate's source equals its base's.
	ErrCarriedPatchEmpty = errors.New("carried patch: the candidate changed no source")
	// ErrCarriedPatchTooLarge: the patch would exceed the size bound.
	ErrCarriedPatchTooLarge = errors.New("carried patch: too large")
	// ErrCarriedPatchOutsideSource: a patch path is absolute, climbs out of
	// the tree, or names version-control metadata.
	ErrCarriedPatchOutsideSource = errors.New("carried patch: path outside the source tree")
)

// CarriedPatch is a failed round's source change as inert data: the unified
// diff, its digest, the files it changes, and what was left out.
type CarriedPatch struct {
	Patch   []byte
	SHA256  string
	Files   []string
	Omitted []string
}

// carriedPatchExcludedDirs are directories that are never source: package
// installs, version-control metadata and tool caches.
var carriedPatchExcludedDirs = map[string]bool{"node_modules": true, ".git": true, ".cache": true}

// secretBearingPath reports whether a slash path names a file the capsule
// release refuses as secret-bearing: any element .env or .env.*, .npmrc,
// .netrc, credentials.json, auth.json, id_rsa or id_ed25519, or a .pem, .key,
// .p12 or .pfx extension, ignoring case. The release and the carried patch
// share it, so a patch never carries what the release would refuse
// (failure 16).
func secretBearingPath(path string) bool {
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		base := strings.ToLower(component)
		extension := filepath.Ext(base)
		if base == ".env" || strings.HasPrefix(base, ".env.") || base == ".npmrc" || base == ".netrc" ||
			base == "credentials.json" || base == "auth.json" || base == "id_rsa" || base == "id_ed25519" ||
			extension == ".pem" || extension == ".key" || extension == ".p12" || extension == ".pfx" {
			return true
		}
	}
	return false
}

type carriedTreeFile struct {
	data []byte
}

// readCarriedTree reads the regular text files of a source tree. Excluded
// directories, secret-bearing paths, symlinks and binary files are not read;
// they are named in omitted instead. A file above maxBytes refuses the whole tree.
func readCarriedTree(ctx context.Context, root string, maxBytes int, omitted map[string]string) (map[string]carriedTreeFile, error) {
	files := map[string]carriedTreeFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("%w: %q", ErrCarriedPatchOutsideSource, path)
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if carriedPatchExcludedDirs[entry.Name()] {
				omitted[rel] = "excluded directory"
				return filepath.SkipDir
			}
			return nil
		}
		if secretBearingPath(rel) {
			omitted[rel] = "secret-bearing path"
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			omitted[rel] = "symlink"
		case !info.Mode().IsRegular():
			omitted[rel] = "special file"
		case info.Size() > int64(maxBytes):
			return fmt.Errorf("%w: %s is %d bytes, above the %d byte bound", ErrCarriedPatchTooLarge, rel, info.Size(), maxBytes)
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.IndexByte(data, 0) >= 0 {
				omitted[rel] = "binary file"
				return nil
			}
			files[rel] = carriedTreeFile{data: data}
		}
		return nil
	})
	return files, err
}

// DiffTreesToPatch renders the source change from baseRoot to candidateRoot
// as a git-applicable unified diff, with the freeze path's own renderer, and
// proves with git apply that the patch reproduces the candidate from the
// base. maxBytes bounds the patch; the digest is stable for equal inputs.
func DiffTreesToPatch(ctx context.Context, baseRoot, candidateRoot string, maxBytes int) (CarriedPatch, error) {
	if maxBytes <= 0 {
		maxBytes = CarriedPatchMaxBytes
	}
	omitted := map[string]string{}
	base, err := readCarriedTree(ctx, baseRoot, maxBytes, map[string]string{})
	if err != nil {
		return CarriedPatch{}, fmt.Errorf("carried patch reads the base tree: %w", err)
	}
	candidate, err := readCarriedTree(ctx, candidateRoot, maxBytes, omitted)
	if err != nil {
		return CarriedPatch{}, fmt.Errorf("carried patch reads the candidate tree: %w", err)
	}
	seen := map[string]bool{}
	var rels []string
	for rel := range base {
		seen[rel] = true
		rels = append(rels, rel)
	}
	for rel := range candidate {
		if !seen[rel] {
			rels = append(rels, rel)
		}
	}
	sort.Strings(rels)
	var files []sourcePatchFile
	var changed []string
	for _, rel := range rels {
		old, hadOld := base[rel]
		now, hasNew := candidate[rel]
		if hadOld && hasNew && bytes.Equal(old.data, now.data) {
			continue
		}
		// A new empty file carries no work; the delegate's sandbox leaves
		// them behind as masks (failure 16). A file the candidate emptied is
		// still a change.
		if !hadOld && hasNew && len(now.data) == 0 {
			omitted[rel] = "new empty file"
			continue
		}
		files = append(files, sourcePatchFile{Rel: rel, Old: old.data, OldExists: hadOld, New: now.data, NewExists: hasNew})
		changed = append(changed, rel)
	}
	if len(files) == 0 {
		return CarriedPatch{Omitted: omittedList(omitted)}, ErrCarriedPatchEmpty
	}
	patch, err := renderSourcePatch(files)
	if err != nil {
		return CarriedPatch{}, err
	}
	if len(patch) == 0 {
		return CarriedPatch{Omitted: omittedList(omitted)}, ErrCarriedPatchEmpty
	}
	if len(patch) > maxBytes {
		return CarriedPatch{}, fmt.Errorf("%w: %d bytes, above the %d byte bound", ErrCarriedPatchTooLarge, len(patch), maxBytes)
	}
	if _, err := CarriedPatchPaths(patch); err != nil {
		return CarriedPatch{}, err
	}
	if err := verifySourcePatch(ctx, patch, files); err != nil {
		return CarriedPatch{}, err
	}
	sum := sha256.Sum256(patch)
	return CarriedPatch{Patch: patch, SHA256: hex.EncodeToString(sum[:]), Files: changed, Omitted: omittedList(omitted)}, nil
}

func omittedList(omitted map[string]string) []string {
	out := make([]string, 0, len(omitted))
	for rel, why := range omitted {
		out = append(out, rel+" ("+why+")")
	}
	sort.Strings(out)
	return out
}

// CarriedPatchPaths returns the paths a git unified diff changes, in order,
// and refuses a patch that names one outside the source tree: absolute,
// climbing out, or inside version-control metadata. Only a patch whose
// "diff --git a/P b/P" headers name one path each is understood; a rename or
// a quoted name is refused rather than guessed at.
func CarriedPatchPaths(patch []byte) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(patch), "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		rest := strings.TrimPrefix(line, "diff --git a/")
		if rest == line || len(rest) < 4 {
			return nil, fmt.Errorf("%w: header %q", ErrCarriedPatchOutsideSource, line)
		}
		half := (len(rest) - 3) / 2
		if (len(rest)-3)%2 != 0 || rest[half:half+3] != " b/" || rest[:half] != rest[half+3:] {
			return nil, fmt.Errorf("%w: header %q names more than one path", ErrCarriedPatchOutsideSource, line)
		}
		path := rest[:half]
		if !carriedPatchSourcePath(path) {
			return nil, fmt.Errorf("%w: %q", ErrCarriedPatchOutsideSource, path)
		}
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	return out, nil
}

func carriedPatchSourcePath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\x00\\\"") || !filepath.IsLocal(path) {
		return false
	}
	for _, element := range strings.Split(path, "/") {
		if element == "" || element == "." || element == ".." || element == ".git" {
			return false
		}
	}
	return true
}

// CheckPatchAppliesToTree reports whether patch applies to baseRoot, with
// git apply --check against a scratch copy of only the files the patch
// touches. It writes nothing under baseRoot. The error names what git
// refused.
func CheckPatchAppliesToTree(ctx context.Context, patch []byte, files []string, baseRoot string) error {
	if _, err := CarriedPatchPaths(patch); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	scratch, err := os.MkdirTemp("", "choir-carried-patch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	tree := filepath.Join(scratch, "tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		return err
	}
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = tree
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := run("init", "-q"); err != nil {
		return err
	}
	for _, rel := range files {
		if !carriedPatchSourcePath(rel) {
			return fmt.Errorf("%w: %q", ErrCarriedPatchOutsideSource, rel)
		}
		data, readErr := os.ReadFile(filepath.Join(baseRoot, filepath.FromSlash(rel)))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return readErr
		}
		target := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	patchPath := filepath.Join(scratch, "source.patch")
	if err := os.WriteFile(patchPath, patch, 0o644); err != nil {
		return err
	}
	whole := run("apply", "--check", patchPath)
	if whole == nil {
		return nil
	}
	// Name the files that conflict: check each file's section on its own
	// against the same scratch tree. The patch carries no blob ids, so a
	// 3-way merge has no preimage to merge from and is not attempted.
	conflict := &CarriedPatchConflictError{Detail: whole.Error()}
	for i, section := range splitPatchByFile(patch) {
		sectionPath := filepath.Join(scratch, fmt.Sprintf("section-%d.patch", i))
		if err := os.WriteFile(sectionPath, section.body, 0o644); err != nil {
			return err
		}
		if run("apply", "--check", sectionPath) != nil {
			conflict.Paths = append(conflict.Paths, section.path)
		} else {
			conflict.Clean = append(conflict.Clean, section.path)
		}
	}
	if len(conflict.Paths) == 0 {
		// Every file applies alone but not together: name none rather than
		// guess.
		return fmt.Errorf("the carried patch does not apply to this base: %w", whole)
	}
	return conflict
}

// CarriedPatchConflictError is a carried patch that does not apply to a moved
// base, with the files that conflict and the files that would still apply.
type CarriedPatchConflictError struct {
	Paths  []string
	Clean  []string
	Detail string
}

func (e *CarriedPatchConflictError) Error() string {
	return fmt.Sprintf("the carried patch does not apply to this base: conflict in %s (%s)", strings.Join(e.Paths, ", "), e.Detail)
}

type patchSection struct {
	path string
	body []byte
}

// splitPatchByFile cuts a unified diff at its "diff --git" headers. Only a
// patch CarriedPatchPaths accepted is split, so each header names one path.
func splitPatchByFile(patch []byte) []patchSection {
	var out []patchSection
	lines := strings.SplitAfter(string(patch), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git a/") {
			rest := strings.TrimSuffix(strings.TrimPrefix(line, "diff --git a/"), "\n")
			out = append(out, patchSection{path: rest[:(len(rest)-3)/2]})
		}
		if len(out) > 0 {
			out[len(out)-1].body = append(out[len(out)-1].body, line...)
		}
	}
	return out
}

// unfinishedReadMaxBytes bounds what one unfinished snapshot reads from a
// capsule in all, so a capsule full of large text files cannot fill the
// guest's memory on its way to the patch bound.
const unfinishedReadMaxBytes = 64 << 20

// UnfinishedPatchFromChanges renders the source change of a capsule that is
// being destroyed without a candidate (its round was stopped before it froze)
// from the capsule's own overlay changes, the way Freeze renders source.patch
// (collectSourcePatchFiles, renderSourcePatch, verifySourcePatch), and with
// the carried patch's rules: only workspace/platform, never node_modules, .git
// or a cache, never a secret-named path, a symbolic link, a binary file or a
// new empty file; those are named in Omitted. Both trees are read beneath
// their root without following a link (readSourceBeneath); a change whose
// path passes through a link refuses the whole snapshot. A file or a patch
// above maxBytes is refused, never truncated. The capsule must be frozen
// (its cgroup stopped) so the tree cannot change while it is read.
func UnfinishedPatchFromChanges(ctx context.Context, mergedDir string, changes []FileChange, maxBytes int) (CarriedPatch, error) {
	if maxBytes <= 0 {
		maxBytes = CarriedPatchMaxBytes
	}
	omitted := map[string]string{}
	var kept []FileChange
	var total int64
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return CarriedPatch{}, err
		}
		p := "/" + strings.TrimPrefix(change.Path, "/")
		if !strings.HasPrefix(p, "/"+platformSourcePrefix+"/") || change.Mode.IsDir() {
			continue
		}
		rel := strings.TrimPrefix(p, "/"+platformSourcePrefix+"/")
		if !sourceRelOK(rel) || !carriedPatchSourcePath(rel) {
			if excluded := carriedExcludedPrefix(rel); excluded != "" {
				omitted[excluded] = "excluded directory"
				continue
			}
			return CarriedPatch{}, fmt.Errorf("%w: %q", ErrCarriedPatchOutsideSource, change.Path)
		}
		if excluded := carriedExcludedPrefix(rel); excluded != "" {
			omitted[excluded] = "excluded directory"
			continue
		}
		if secretBearingPath(rel) {
			omitted[rel] = "secret-bearing path"
			continue
		}
		if change.Mode&os.ModeSymlink != 0 {
			omitted[rel] = "symlink"
			continue
		}
		if change.Kind != ChangeDeleted {
			// A bound before any read. The read itself never follows a link
			// (readSourceBeneath); this Lstat only sizes it.
			if info, err := os.Lstat(filepath.Join(mergedDir, platformSourcePrefix, filepath.FromSlash(rel))); err == nil && info.Mode().IsRegular() {
				if info.Size() > int64(maxBytes) {
					return CarriedPatch{}, fmt.Errorf("%w: %s is %d bytes, above the %d byte bound", ErrCarriedPatchTooLarge, rel, info.Size(), maxBytes)
				}
				total += info.Size()
				if total > unfinishedReadMaxBytes {
					return CarriedPatch{}, fmt.Errorf("%w: the changed files exceed %d bytes", ErrCarriedPatchTooLarge, unfinishedReadMaxBytes)
				}
			}
		}
		kept = append(kept, change)
	}
	collected, err := collectSourcePatchFiles(mergedDir, kept)
	if err != nil {
		return CarriedPatch{}, err
	}
	var files []sourcePatchFile
	var changed []string
	for _, file := range collected {
		switch {
		case (file.OldExists && bytes.IndexByte(file.Old, 0) >= 0) || (file.NewExists && bytes.IndexByte(file.New, 0) >= 0):
			omitted[file.Rel] = "binary file"
			continue
		case !file.OldExists && !file.NewExists:
			continue
		case file.OldExists && file.NewExists && bytes.Equal(file.Old, file.New):
			continue
		case !file.OldExists && file.NewExists && len(file.New) == 0:
			omitted[file.Rel] = "new empty file"
			continue
		}
		files = append(files, file)
		changed = append(changed, file.Rel)
	}
	if len(files) == 0 {
		return CarriedPatch{Omitted: omittedList(omitted)}, ErrCarriedPatchEmpty
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	sort.Strings(changed)
	patch, err := renderSourcePatch(files)
	if err != nil {
		return CarriedPatch{}, err
	}
	if len(patch) == 0 {
		return CarriedPatch{Omitted: omittedList(omitted)}, ErrCarriedPatchEmpty
	}
	if len(patch) > maxBytes {
		return CarriedPatch{}, fmt.Errorf("%w: %d bytes, above the %d byte bound", ErrCarriedPatchTooLarge, len(patch), maxBytes)
	}
	if _, err := CarriedPatchPaths(patch); err != nil {
		return CarriedPatch{}, err
	}
	if err := verifySourcePatch(ctx, patch, files); err != nil {
		return CarriedPatch{}, err
	}
	sum := sha256.Sum256(patch)
	return CarriedPatch{Patch: patch, SHA256: hex.EncodeToString(sum[:]), Files: changed, Omitted: omittedList(omitted)}, nil
}

// carriedExcludedPrefix is the leading part of rel up to and including its
// first excluded name (node_modules, .git, .cache), or "".
func carriedExcludedPrefix(rel string) string {
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		if carriedPatchExcludedDirs[part] {
			return strings.Join(parts[:i+1], "/")
		}
	}
	return ""
}
