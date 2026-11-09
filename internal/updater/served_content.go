package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// What a pure base import serves out of `current` is its frontend/ tree; the
// binary and the build stamp come from the booted image, and skills and desk
// modules from the baseline root. Two releases that agree on this digest
// serve the same bytes, whatever commit stamp their other files carry.

type servedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func servedDigest(files []servedFile) string {
	if len(files) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	raw, err := json.Marshal(files)
	if err != nil {
		return ""
	}
	return computerevent.DigestBytes(raw)
}

// FrontendDigestOfManifest digests the frontend/ files a manifest names (path
// relative to frontend/, and content hash; the manifest already carries the
// hashes, so nothing is read). "" when the manifest names none.
func FrontendDigestOfManifest(manifest ReleaseManifest) string {
	var files []servedFile
	for _, file := range manifest.Files {
		if rest, ok := strings.CutPrefix(file.Path, "frontend/"); ok && rest != "" {
			files = append(files, servedFile{Path: rest, SHA256: file.SHA256})
		}
	}
	return servedDigest(files)
}

// FrontendDigestOfDir digests a frontend directory the way
// FrontendDigestOfManifest digests a manifest's frontend/ files. "" when the
// directory has no index.html or cannot be read in full.
func FrontendDigestOfDir(dir string) string {
	dir = filepath.Clean(dir)
	if info, err := os.Stat(filepath.Join(dir, "index.html")); err != nil || info.IsDir() {
		return ""
	}
	var files []servedFile
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir || entry.IsDir() {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, servedFile{Path: filepath.ToSlash(relative), SHA256: hex.EncodeToString(hash.Sum(nil))})
		return nil
	})
	if err != nil {
		return ""
	}
	return servedDigest(files)
}

// UnsettledApply reports whether an apply or a settle may be in flight under
// root, and names why: a journal with no result yet (or unreadable), a held
// per-key lock, or an operations/ that exists but cannot be listed (fail
// closed; a missing operations/ is settled).
// HoldApplyKey arbitrates a dead apply by comparing `current` with the
// journal's prior and target digests, so nothing else moves `current` while
// one is open.
func UnsettledApply(root string) (string, bool) {
	dir := operationsDir(root)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		return "operations (" + err.Error() + ")", true
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(name, ".json"):
			journal, found, err := readJournal(filepath.Join(dir, name))
			if err != nil || (found && journal.Result.Outcome == "") {
				return name, true
			}
		case strings.HasSuffix(name, ".lock"):
			// The daemon is another process: it takes this lock at entry
			// and journals only after staging, so a held lock is an apply
			// with no journal yet. Try the daemon's own non-blocking lock.
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_RDWR, 0)
			if err != nil {
				return name + " (" + err.Error() + ")", true
			}
			release, acquired, err := lockFile(f, false)
			if acquired {
				release()
			}
			f.Close()
			if err != nil || !acquired {
				return name + " (held)", true
			}
		}
	}
	return "", false
}
