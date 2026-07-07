package computerversion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

const (
	defaultDirPerm  os.FileMode = 0o700
	defaultFilePerm os.FileMode = 0o600
)

// TreeToFS writes a Base tree's live items to a target directory, reading
// file content from the blob store. This is the generator function: it
// produces concrete filesystem state from the typed, substrate-independent
// tree representation.
//
// Folders are created as directories. Files are written with content from
// their version's BlobRef via the blob store. Deleted items (tombstones) are
// skipped entirely — their paths are not even resolved.
//
// Security:
//   - Item names are validated to prevent path traversal (../, absolute
//     paths, backslashes, etc.).
//   - All filesystem joins use safeJoin, which rejects paths that escape
//     the target root.
//   - The target directory is checked for symlinks and must be empty if it
//     already exists.
//   - Files are created with O_EXCL (exclusive create) to prevent symlink
//     attacks.
//   - Directory and file permissions are restrictive (0700/0600).
//
// Atomicity:
//   - Generation happens in a uniquely-named temp directory (created via
//     os.MkdirTemp) first. On success, the target is replaced via rename.
//     On any error, the temp directory is cleaned up and the target is
//     left untouched.
//
// This function performs filesystem writes but does NOT launch VMs, touch
// VM lifecycle, mutate product state, or interact with any hypervisor. It is
// the substrate-independent generation step: any substrate that can host a
// filesystem can receive this output.
func TreeToFS(ctx context.Context, tree basetree.Tree, blobs *blob.Store, targetDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if blobs == nil {
		return fmt.Errorf("tree-to-fs: nil blob store")
	}
	trimmed := strings.TrimSpace(targetDir)
	if trimmed == "" {
		return fmt.Errorf("tree-to-fs: target dir is required")
	}
	// Reject backslashes to prevent cross-platform path confusion.
	if strings.Contains(trimmed, "\\") {
		return fmt.Errorf("tree-to-fs: target dir %q cannot contain backslash", trimmed)
	}
	// Reject paths containing ".." components before cleaning, so that
	// inputs like "safe/../target" are rejected rather than silently
	// normalized to a different path than the caller specified.
	for _, c := range strings.Split(filepath.ToSlash(trimmed), "/") {
		if c == ".." {
			return fmt.Errorf("tree-to-fs: target dir %q cannot contain '..'", trimmed)
		}
	}
	targetDir = filepath.Clean(trimmed)
	if targetDir == "" || targetDir == "." {
		return fmt.Errorf("tree-to-fs: target dir is required")
	}

	// Precondition: if targetDir already exists, it must be a real
	// directory (not a symlink) and must be empty. This prevents
	// accidental overwriting of existing user data.
	if info, err := os.Lstat(targetDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("tree-to-fs: target dir %q is a symlink", targetDir)
		}
		if !info.IsDir() {
			return fmt.Errorf("tree-to-fs: target %q is not a directory", targetDir)
		}
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return fmt.Errorf("tree-to-fs: read target dir %q: %w", targetDir, err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("tree-to-fs: target dir %q is not empty", targetDir)
		}
	}

	// Generate into a temp directory first for atomicity. On success we
	// rename it to targetDir; on failure we clean it up.
	//
	// We use os.MkdirTemp with a randomized suffix instead of a fixed
	// ".tmp" suffix. A fixed suffix risks colliding with (and recursively
	// deleting) a real sibling directory left by another process or a
	// crashed previous run. MkdirTemp creates a unique directory that we
	// know we own, so cleaning it up on error is always safe.
	tempDir, err := os.MkdirTemp(filepath.Dir(targetDir), filepath.Base(targetDir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("tree-to-fs: create temp dir: %w", err)
	}

	// Generate into the temp directory. On any error, clean up.
	if genErr := generateIntoDir(ctx, tree, blobs, tempDir); genErr != nil {
		_ = os.RemoveAll(tempDir)
		return genErr
	}

	// Post-hoc symlink detection: walk the entire temp tree and reject
	// any symlink found. This does not prevent TOCTOU races during
	// generation (an attacker who can mutate the temp tree concurrently
	// can swap a component after the per-path Lstat checks), but it
	// detects symlinks before the rename promotes the tree to the target.
	if err := verifyNoSymlinks(tempDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("tree-to-fs: %w", err)
	}

	// Success: remove the (empty) target dir and atomically rename temp
	// into its place. We use os.Remove (non-recursive) instead of
	// os.RemoveAll to defend against a TOCTOU race: we verified earlier
	// that targetDir is empty, and os.Remove only succeeds on an empty
	// directory. If something populated targetDir between the check and
	// now, os.Remove fails and we return an error rather than recursively
	// deleting live data.
	if err := os.Remove(targetDir); err != nil {
		// If the target doesn't exist at all, that's fine — there's
		// nothing to remove before the rename.
		if !os.IsNotExist(err) {
			_ = os.RemoveAll(tempDir)
			return fmt.Errorf("tree-to-fs: remove target dir for rename %q: %w", targetDir, err)
		}
	}
	if err := os.Rename(tempDir, targetDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return fmt.Errorf("tree-to-fs: rename temp to target %q: %w", targetDir, err)
	}

	return nil
}

// generateIntoDir performs the actual generation into rootDir (the temp
// directory). All security checks (path validation, symlink detection,
// O_EXCL writes) are applied here.
func generateIntoDir(ctx context.Context, tree basetree.Tree, blobs *blob.Store, rootDir string) error {
	// Build path for each live item by walking the parent chain.
	paths, err := resolveTreePaths(tree)
	if err != nil {
		return err
	}

	// Process items in a deterministic order: folders first (sorted by path),
	// then files (sorted by path). This ensures parent directories exist
	// before we try to write files into them.
	folderPaths := make([]string, 0)
	fileEntries := make([]struct {
		path string
		ref  model.BlobRef
	}, 0)

	// Track seen paths to detect duplicates.
	seenPaths := make(map[string]model.ItemID)

	for id, item := range tree.Items {
		// Skip tombstones — they are not written and their paths are
		// not resolved.
		if item.DeletedAt != nil || item.CurrentVersion == "" {
			continue
		}
		path, ok := paths[id]
		if !ok {
			continue
		}

		// Detect duplicate paths (two live items resolving to the same
		// filesystem location).
		if existingID, exists := seenPaths[path]; exists {
			return fmt.Errorf("tree-to-fs: duplicate path %q for items %s and %s", path, existingID, id)
		}
		seenPaths[path] = id

		// Validate item identity and structure for ALL live items
		// (mirrors base_tree.go lines 64-110). This ensures TreeToFS
		// rejects trees that BaseTreeObservationSet would reject.
		// Kind validity is deferred to the switch below so that unknown
		// kinds produce a descriptive "unsupported kind" error.
		if id != item.ItemID {
			return fmt.Errorf("tree-to-fs: map key %q does not match item id %q", id, item.ItemID)
		}
		if !item.ItemID.Valid() {
			return fmt.Errorf("tree-to-fs: invalid item id %q", item.ItemID)
		}
		if !item.CurrentVersion.Valid() {
			return fmt.Errorf("tree-to-fs: invalid current version for item %q", item.ItemID)
		}

		switch item.Kind {
		case model.KindFolder:
			// Folders don't carry blobs. A live folder with a
			// CurrentVersion set must have a matching version entry
			// in tree.Versions, mirroring base_tree.go where every
			// live item needs a current version. A folder with an
			// empty CurrentVersion is permitted (folders can exist
			// without versions); this preserves
			// TestTreeToFSOnlyFolders-style cases where folders are
			// created purely structurally.
			if item.CurrentVersion != "" {
				version, hasVersion := tree.Versions[id]
				if !hasVersion {
					return fmt.Errorf("tree-to-fs: folder %q (%s) has current version %q but no version entry", id, path, item.CurrentVersion)
				}
				if version.VersionID == "" {
					return fmt.Errorf("tree-to-fs: folder %q (%s) has version with empty VersionID", id, path)
				}
				if version.ItemID != item.ItemID {
					return fmt.Errorf("tree-to-fs: folder version item %q does not match item %q for path %q", version.ItemID, id, path)
				}
				if version.VersionID != item.CurrentVersion {
					return fmt.Errorf("tree-to-fs: folder version %q does not match current version %q for item %q", version.VersionID, item.CurrentVersion, id)
				}
				if !version.Valid() {
					return fmt.Errorf("tree-to-fs: folder %q (%s) has invalid version", id, path)
				}
				if version.BlobRef != "" {
					return fmt.Errorf("tree-to-fs: folder %q (%s) has non-empty blob ref %q", id, path, version.BlobRef)
				}
			}
			folderPaths = append(folderPaths, path)
		case model.KindFile:
			version, hasVersion := tree.Versions[id]
			if !hasVersion {
				return fmt.Errorf("tree-to-fs: file %q (%s) has no version", id, path)
			}
			// Validate version/item consistency (mirrors base_tree.go
			// lines 93-110).
			if version.VersionID == "" {
				return fmt.Errorf("tree-to-fs: file %q (%s) has version with empty VersionID", id, path)
			}
			if version.ItemID != id {
				return fmt.Errorf("tree-to-fs: version item %q does not match item %q for path %q", version.ItemID, id, path)
			}
			if version.VersionID != item.CurrentVersion {
				return fmt.Errorf("tree-to-fs: version %q does not match current version %q for item %q", version.VersionID, item.CurrentVersion, id)
			}
			if !version.Valid() {
				return fmt.Errorf("tree-to-fs: file %q (%s) has invalid version", id, path)
			}
			if version.BlobRef == "" {
				return fmt.Errorf("tree-to-fs: file %q (%s) has empty blob ref", id, path)
			}
			fileEntries = append(fileEntries, struct {
				path string
				ref  model.BlobRef
			}{path: path, ref: version.BlobRef})
		default:
			return fmt.Errorf("tree-to-fs: item %q has unsupported kind %q", id, item.Kind)
		}
	}

	sort.Strings(folderPaths)
	sort.Slice(fileEntries, func(i, j int) bool { return fileEntries[i].path < fileEntries[j].path })

	// Create folders.
	for _, p := range folderPaths {
		if err := ctx.Err(); err != nil {
			return err
		}
		full, err := safeJoin(rootDir, p)
		if err != nil {
			return fmt.Errorf("tree-to-fs: unsafe folder path %q: %w", p, err)
		}
		// Check no existing component is a symlink before creating.
		if err := verifyPathComponentsNotSymlinks(rootDir, full); err != nil {
			return fmt.Errorf("tree-to-fs: symlink check for %q: %w", p, err)
		}
		if err := os.MkdirAll(full, defaultDirPerm); err != nil {
			return fmt.Errorf("tree-to-fs: mkdir %q: %w", p, err)
		}
	}

	// Write files.
	for _, fe := range fileEntries {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := blobs.Get(fe.ref)
		if err != nil {
			return fmt.Errorf("tree-to-fs: get blob %s for %q: %w", fe.ref, fe.path, err)
		}
		full, err := safeJoin(rootDir, fe.path)
		if err != nil {
			return fmt.Errorf("tree-to-fs: unsafe file path %q: %w", fe.path, err)
		}
		parentDir := filepath.Dir(full)
		// Ensure parent directory exists and is not a symlink.
		if err := verifyPathComponentsNotSymlinks(rootDir, parentDir); err != nil {
			return fmt.Errorf("tree-to-fs: symlink check for %q: %w", fe.path, err)
		}
		if err := os.MkdirAll(parentDir, defaultDirPerm); err != nil {
			return fmt.Errorf("tree-to-fs: mkdir for %q: %w", fe.path, err)
		}
		// Verify parent directory is not a symlink.
		parentInfo, err := os.Lstat(parentDir)
		if err != nil {
			return fmt.Errorf("tree-to-fs: lstat parent for %q: %w", fe.path, err)
		}
		if parentInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("tree-to-fs: parent directory for %q is a symlink", fe.path)
		}
		// Use O_CREATE|O_WRONLY|O_EXCL (exclusive create) to prevent
		// writing through an existing symlink or overwriting an existing
		// file.
		f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_EXCL, defaultFilePerm)
		if err != nil {
			return fmt.Errorf("tree-to-fs: create %q: %w", fe.path, err)
		}
		if _, err := f.Write(data); err != nil {
			_ = f.Close()
			_ = os.Remove(full)
			return fmt.Errorf("tree-to-fs: write %q: %w", fe.path, err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(full)
			return fmt.Errorf("tree-to-fs: close %q: %w", fe.path, err)
		}
	}

	return nil
}

// validateItemName rejects item names that could be used for path traversal
// or that are not clean single-component paths.
func validateItemName(name string) error {
	if name == "" {
		return fmt.Errorf("empty name")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name %q contains path separator", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("name %q is reserved", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("name %q is absolute", name)
	}
	if filepath.Clean(name) != name {
		return fmt.Errorf("name %q is not clean", name)
	}
	return nil
}

// safeJoin cleans both root and rel, joins them, and verifies the result
// does not escape root. It returns an error if the relative path starts
// with ".." after joining.
func safeJoin(root, rel string) (string, error) {
	rootClean := filepath.Clean(root)
	relClean := filepath.Clean(rel)
	full := filepath.Join(rootClean, relClean)
	relCheck, err := filepath.Rel(rootClean, full)
	if err != nil {
		return "", fmt.Errorf("cannot compute relative path: %w", err)
	}
	if filepath.IsAbs(relCheck) {
		return "", fmt.Errorf("tree-to-fs: path %q escapes root directory", rel)
	}
	// Check that no component of the relative path is "..". This is more
	// precise than a prefix check, which would reject valid names like
	// "..foo" or "..." that validateItemName permits.
	components := strings.Split(filepath.ToSlash(relCheck), "/")
	for _, c := range components {
		if c == ".." {
			return "", fmt.Errorf("tree-to-fs: path %q escapes root directory", rel)
		}
	}
	return full, nil
}

// verifyPathComponentsNotSymlinks walks from root to full, checking each
// existing component with Lstat. If any existing component is a symlink,
// it returns an error. Non-existent components are allowed (they will be
// created by MkdirAll).
func verifyPathComponentsNotSymlinks(root, full string) error {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := root
	components := strings.Split(filepath.ToSlash(rel), "/")
	for _, comp := range components {
		if comp == "" || comp == "." {
			continue
		}
		current = filepath.Join(current, comp)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue // doesn't exist yet, fine
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path component %q is a symlink", current)
		}
	}
	return nil
}

// verifyNoSymlinks walks the entire tree under root and rejects any symlink
// found. This is a post-hoc check performed after all files and folders have
// been created but before the rename that promotes the temp directory to the
// target. It does not prevent TOCTOU races during generation (an attacker who
// can mutate the temp tree concurrently can swap a component after the per-path
// Lstat checks), but it detects symlinks before the tree is promoted.
func verifyNoSymlinks(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("verify no symlinks: walk %q: %w", path, err)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("verify no symlinks: info %q: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("verify no symlinks: symlink found at %q", path)
		}
		return nil
	})
}

// resolveTreePaths walks the parent chain for each live item to build its
// filesystem path relative to the tree root. Root items (empty
// ParentItemID) use their Name as the path. Child items are
// parent_path/name.
//
// Only live items (not deleted, has CurrentVersion) are resolved.
// Tombstoned items are ignored entirely — their paths are not computed and
// missing parents on tombstoned items do not cause errors.
//
// For each live item, every ancestor must:
//  1. Exist in tree.Items
//  2. Be live (not deleted, has CurrentVersion)
//  3. Be KindFolder
//
// If any ancestor fails these checks, an error is returned.
func resolveTreePaths(tree basetree.Tree) (map[model.ItemID]string, error) {
	paths := make(map[model.ItemID]string)

	// Collect live item IDs only. Tombstoned items are skipped entirely
	// so that missing parents on deleted items don't cause errors.
	liveIDs := make([]model.ItemID, 0, len(tree.Items))
	for id, item := range tree.Items {
		if item.DeletedAt != nil || item.CurrentVersion == "" {
			continue
		}
		liveIDs = append(liveIDs, id)
	}
	sort.Slice(liveIDs, func(i, j int) bool { return liveIDs[i] < liveIDs[j] })

	// Resolve paths iteratively. A tree may have arbitrary depth, so we
	// loop until all items are resolved or no progress is made.
	resolved := make(map[model.ItemID]bool)
	for iteration := 0; iteration < len(liveIDs)+1; iteration++ {
		progress := false
		for _, id := range liveIDs {
			if resolved[id] {
				continue
			}
			item := tree.Items[id]
			if item.ParentItemID == "" {
				// Root item.
				if err := validateItemName(item.Name); err != nil {
					return nil, fmt.Errorf("tree-to-fs: item %q: %w", id, err)
				}
				paths[id] = item.Name
				resolved[id] = true
				progress = true
			} else {
				// Validate the parent every time we attempt to resolve
				// a child, regardless of whether the parent path is
				// already resolved. This catches the case where a file
				// parent sorts before its child (by ItemID): the file
				// resolves first, then the child would be joined under
				// the file and fail later as a filesystem error instead
				// of a clear tree-invariant error.
				parent, parentExists := tree.Items[item.ParentItemID]
				if !parentExists {
					return nil, fmt.Errorf("tree-to-fs: item %q has missing parent %q", id, item.ParentItemID)
				}
				if parent.DeletedAt != nil || parent.CurrentVersion == "" {
					return nil, fmt.Errorf("tree-to-fs: item %q has deleted parent %q", id, item.ParentItemID)
				}
				if parent.Kind != model.KindFolder {
					return nil, fmt.Errorf("tree-to-fs: item %q has non-folder parent %q", id, item.ParentItemID)
				}
				parentPath, ok := paths[item.ParentItemID]
				if !ok {
					// Parent is valid but not resolved yet; wait for it.
					continue
				}
				if err := validateItemName(item.Name); err != nil {
					return nil, fmt.Errorf("tree-to-fs: item %q: %w", id, err)
				}
				paths[id] = filepath.Join(parentPath, item.Name)
				resolved[id] = true
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	// Check for unresolved items (circular or missing parents).
	for _, id := range liveIDs {
		if !resolved[id] {
			item := tree.Items[id]
			return nil, fmt.Errorf("tree-to-fs: cannot resolve path for item %q (parent %q)", id, item.ParentItemID)
		}
	}

	return paths, nil
}
