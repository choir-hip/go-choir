package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

// LocalTreeBuilder scans a local folder and produces a planner.Tree that the
// sync engine can compare against the remote and synced trees. Each file and
// directory becomes a model.Item with a stable ItemID derived from its
// relative path (so rescans produce consistent identity across sync cycles).
//
// Identity is path-derived for the local scan only: the ItemID is
// "base_item_" + sha256(relPath)[:16], which is deterministic for a given
// path. This is a local convention; the remote tree uses server-assigned
// ItemIDs. The sync engine reconciles by matching local and remote items via
// the planner's path-collision detection when IDs differ, and via direct ID
// equality when a previously-synced item's ID was adopted locally.
type LocalTreeBuilder struct {
	root    string
	deviceID string
	now     func() time.Time
}

// NewLocalTreeBuilder returns a scanner rooted at root. The deviceID is
// stamped onto each version's CreatedByDevice.
func NewLocalTreeBuilder(root, deviceID string) *LocalTreeBuilder {
	return &LocalTreeBuilder{
		root:    root,
		deviceID: deviceID,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// SetClock replaces the clock used for version timestamps. Intended for tests.
func (b *LocalTreeBuilder) SetClock(now func() time.Time) {
	if now != nil {
		b.now = now
	}
}

// localItemID derives a deterministic ItemID from a path relative to the
// scan root. The ID is stable across rescans so the planner sees the same
// identity for an unchanged file.
func localItemID(relPath string) model.ItemID {
	h := sha256.Sum256([]byte(filepath.ToSlash(relPath)))
	return model.ItemID("base_item_" + hex.EncodeToString(h[:16]))
}

// Scan walks the root directory and builds a planner.Tree. The root itself is
// not represented as an item (it is the implicit parent). Files and folders
// are keyed by their path relative to root. A folder's version carries no
// blob; a file's version carries a BlobRef computed from the file's content
// hash (the bytes are not read into the tree — the sync engine uploads them
// separately via the blob store).
//
// Hidden files (leading dot) and the Choir internal metadata directory are
// skipped.
func (b *LocalTreeBuilder) Scan() (planner.Tree, error) {
	tree := planner.NewTree()

	info, err := os.Stat(b.root)
	if err != nil {
		return tree, fmt.Errorf("local scan: stat root %s: %w", b.root, err)
	}
	if !info.IsDir() {
		return tree, fmt.Errorf("local scan: root %s is not a directory", b.root)
	}

	// The root folder is represented as the implicit parent (empty
	// ParentItemID). We do not create an item for it.
	err = filepath.WalkDir(b.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == b.root {
			return nil // skip the root itself
		}
		rel, rerr := filepath.Rel(b.root, path)
		if rerr != nil {
			return rerr
		}
		relSlash := filepath.ToSlash(rel)

		// Skip hidden entries and the Choir metadata dir.
		base := filepath.Base(relSlash)
		if strings.HasPrefix(base, ".") && base != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		parentRel := filepath.Dir(relSlash)
		if parentRel == "." {
			parentRel = ""
		}
		var parentID model.ItemID
		if parentRel != "" {
			parentID = localItemID(parentRel)
		}

		id := localItemID(relSlash)
		now := b.now()

		if d.IsDir() {
			// Folders carry a version with no blob.
			verID := model.VersionID("base_ver_folder_" + uuid.NewString())
			item := model.Item{
				ItemID:         id,
				OwnerID:        "", // filled by sync engine from API key identity
				ParentItemID:   parentID,
				Name:           base,
				Kind:           model.KindFolder,
				CurrentVersion: verID,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			tree.Items[id] = item
			tree.Versions[id] = model.Version{
				VersionID:        verID,
				ItemID:           id,
				CreatedByDevice:  b.deviceID,
				CreatedAt:        now,
			}
			return nil
		}

		// File: compute content hash (without loading bytes into the tree).
		finfo, serr := d.Info()
		if serr != nil {
			return serr
		}
		contentHash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", path, err)
		}
		blobRef := model.BlobRef("sha256:" + contentHash)
		verID := model.VersionID("base_ver_local_" + contentHash[:16])
		item := model.Item{
			ItemID:         id,
			OwnerID:        "",
			ParentItemID:   parentID,
			Name:           base,
			Kind:           model.KindFile,
			CurrentVersion: verID,
			CreatedAt:      finfo.ModTime().UTC(),
			UpdatedAt:      finfo.ModTime().UTC(),
		}
		tree.Items[id] = item
		tree.Versions[id] = model.Version{
			VersionID:        verID,
			ItemID:           id,
			BlobRef:          blobRef,
			MediaType:        mediaTypeForPath(base),
			ContentHash:      contentHash,
			CreatedByDevice:  b.deviceID,
			CreatedAt:        now,
		}
		return nil
	})
	if err != nil {
		return tree, fmt.Errorf("local scan: walk %s: %w", b.root, err)
	}
	return tree, nil
}

// hashFile returns the SHA-256 hex digest of the file at path. It streams the
// file in 64 KiB chunks so large files do not consume unbounded memory.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// mediaTypeForPath returns a best-effort media type based on the file
// extension. The blob store is content-addressed and media-type-agnostic;
// this is informational metadata carried on the version.
func mediaTypeForPath(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".go":
		return "text/x-go"
	default:
		return "application/octet-stream"
	}
}

// ReadFile returns the bytes of the file at relPath (relative to the scan
// root). The sync engine uses this to upload blob content for files that the
// planner marked for upload.
func (b *LocalTreeBuilder) ReadFile(relPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(b.root, relPath))
}

// AbsPath returns the absolute path for a relative path within the scan root.
func (b *LocalTreeBuilder) AbsPath(relPath string) string {
	return filepath.Join(b.root, relPath)
}

// RelPathFromID returns the relative path for a local ItemID, or "" if the ID
// is not present in the tree. This is used by the sync engine to map planner
// actions back to filesystem paths.
func RelPathFromID(tree planner.Tree, id model.ItemID) string {
	item, ok := tree.Items[id]
	if !ok {
		return ""
	}
	// Reconstruct the relative path by walking up the parent chain. The local
	// scan stores names; we rebuild the path from names. This works because
	// local ItemIDs are path-derived and the parent chain mirrors the
	// directory structure.
	var parts []string
	cur := item
	for cur.ParentItemID != "" {
		parts = append([]string{cur.Name}, parts...)
		parent, ok := tree.Items[cur.ParentItemID]
		if !ok {
			break
		}
		cur = parent
	}
	parts = append([]string{cur.Name}, parts...)
	return strings.Join(parts, "/")
}
