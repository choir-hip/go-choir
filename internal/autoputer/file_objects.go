package autoputer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The file object's recorder (docs/design/file-object-2026-10-24.md, slice
// 1). Behind the files.object computer setting, read live: every Files API
// mutation and every reconcile pass records choir.file and its path view.
// The bytes stay where they are; the recorder only reads them to hash them.

// fileObjectStore is the store surface the recorder uses.
type fileObjectStore interface {
	GetComputerSetting(ctx context.Context, ownerID, computerID, key string) (choirstore.ComputerSetting, error)
	ApplyFileChanges(ctx context.Context, ownerID, computerID string, changes []choirstore.FileChange, now time.Time) (choirstore.FileApplyResult, error)
	GetFileObject(ctx context.Context, ownerID, fileID string) (choirstore.FileObject, error)
	GetFileObjectByPath(ctx context.Context, ownerID, computerID, path string) (choirstore.FileObject, error)
	ListFileObjects(ctx context.Context, ownerID, computerID string) ([]choirstore.FileObject, error)
}

const (
	// fileObjectBatchSize bounds one reconcile batch (one tape event).
	fileObjectBatchSize = 64
	// fileObjectExemptDir is the top-level source tree: source and build
	// state are exempt from I27 and Source/platform is large.
	fileObjectExemptDir = "Source"
	// A reconcile that would delete more than half of more than this many
	// present files holds its deletions (an unmounted or emptied volume).
	fileObjectMassDeleteFloor = 8
	defaultFileObjectInterval = 15 * time.Minute
)

type fileObjects struct {
	mu         sync.Mutex
	root       string
	store      fileObjectStore
	ownerID    string
	computerID string
	// recordFailures counts API writes whose object could not be recorded;
	// the next reconcile repairs them.
	recordFailures atomic.Uint64
}

func newFileObjects(root string, st fileObjectStore, ownerID, computerID string) *fileObjects {
	if st == nil {
		return nil
	}
	return &fileObjects{root: filepath.Clean(root), store: st, ownerID: strings.TrimSpace(ownerID), computerID: strings.TrimSpace(computerID)}
}

// enabled reads the switch live. Anything but a stored true is off.
func (f *fileObjects) enabled(ctx context.Context, ownerID string) bool {
	if f == nil || strings.TrimSpace(ownerID) == "" || f.computerID == "" {
		return false
	}
	setting, err := f.store.GetComputerSetting(ctx, ownerID, f.computerID, choirstore.SettingFilesObject)
	if err != nil {
		if !errors.Is(err, choirstore.ErrNotFound) {
			log.Printf("file objects: read %s: %v; treating it as off", choirstore.SettingFilesObject, err)
		}
		return false
	}
	return setting.Value
}

// MoveAllowed gates the Files API move verb on the switch.
func (f *fileObjects) MoveAllowed(r *http.Request) bool {
	if f == nil || r == nil {
		return false
	}
	ownerID, err := authenticatedFilesOwner(r)
	if err != nil {
		return false
	}
	return f.enabled(r.Context(), ownerID)
}

func authenticatedFilesOwner(r *http.Request) (string, error) {
	if err := requireAuth(r); err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Header.Get("X-Authenticated-User")), nil
}

// fileRecordAt reads one regular file's record from disk, hashing it by
// streaming.
func (f *fileObjects) fileRecordAt(rel string) (choirstore.FileRecord, error) {
	abs := filepath.Join(f.root, filepath.FromSlash(rel))
	file, err := os.Open(abs)
	if err != nil {
		return choirstore.FileRecord{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return choirstore.FileRecord{}, err
	}
	if !info.Mode().IsRegular() {
		return choirstore.FileRecord{}, fmt.Errorf("file objects: %s is not a regular file", rel)
	}
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return choirstore.FileRecord{}, err
	}
	return recordFromInfo(rel, info, "sha256:"+hex.EncodeToString(h.Sum(nil))), nil
}

func recordFromInfo(rel string, info fs.FileInfo, digest string) choirstore.FileRecord {
	return choirstore.FileRecord{
		Path:      rel,
		SHA256:    digest,
		Size:      info.Size(),
		Mode:      uint32(info.Mode().Perm()),
		ModTime:   info.ModTime().UTC(),
		MediaType: mime.TypeByExtension(strings.ToLower(filepath.Ext(rel))),
	}
}

func exemptFilePath(rel string) bool {
	return rel == fileObjectExemptDir || strings.HasPrefix(rel, fileObjectExemptDir+"/")
}

// Record records one successful Files API mutation. A failure is logged and
// counted; the file write it follows already stands, and the next reconcile
// repairs the object.
func (f *fileObjects) Record(ctx context.Context, ownerID string, event FileChangeEvent) {
	if f == nil || event.EntryType != "file" || !f.enabled(ctx, ownerID) {
		return
	}
	rel := strings.Trim(event.Path, "/")
	if rel == "" || exemptFilePath(rel) {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var change choirstore.FileChange
	switch event.Operation {
	case "created", "updated", "moved":
		rec, err := f.fileRecordAt(rel)
		if err != nil {
			f.recordFailure(rel, err)
			return
		}
		change = choirstore.FileChange{Op: choirstore.FileOpWrite, Record: rec, RecordedBy: "files_api"}
		if event.Operation == "moved" {
			change.Op = choirstore.FileOpMove
			change.FromPath = strings.Trim(event.FromPath, "/")
			if change.FromPath == "" || exemptFilePath(change.FromPath) {
				change.Op, change.FromPath = choirstore.FileOpWrite, ""
			}
		}
	case "deleted":
		change = choirstore.FileChange{Op: choirstore.FileOpDelete, Record: choirstore.FileRecord{Path: rel}, RecordedBy: "files_api"}
	default:
		return
	}
	if _, err := f.store.ApplyFileChanges(ctx, ownerID, f.computerID, []choirstore.FileChange{change}, time.Now()); err != nil {
		f.recordFailure(rel, err)
	}
}

func (f *fileObjects) recordFailure(rel string, err error) {
	n := f.recordFailures.Add(1)
	log.Printf("file objects: record %s failed (%d so far); the next reconcile repairs it: %v", rel, n, err)
}

// fileReconcileResult is one reconcile pass.
type fileReconcileResult struct {
	Skipped       bool   `json:"skipped,omitempty"`
	Scanned       int    `json:"scanned"`
	Hashed        int    `json:"hashed"`
	Created       int    `json:"created"`
	Updated       int    `json:"updated"`
	Moved         int    `json:"moved"`
	Deleted       int    `json:"deleted"`
	DeletionsHeld int    `json:"deletions_held,omitempty"`
	Diverged      int    `json:"diverged"`
	Note          string `json:"note,omitempty"`
}

// Writes is the number of objects the pass changed.
func (r fileReconcileResult) Writes() int { return r.Created + r.Updated + r.Moved + r.Deleted }

// Reconcile makes the objects state the files tree: it records files written
// outside the Files API, moves found by a unique digest, and deletions. It
// never runs on a fresh volume, and holds a mass deletion.
func (f *fileObjects) Reconcile(ctx context.Context) (fileReconcileResult, error) {
	if f == nil {
		return fileReconcileResult{Skipped: true}, nil
	}
	if f.ownerID == "" {
		return fileReconcileResult{}, fmt.Errorf("file objects: reconcile needs CHOIR_OWNER_ID")
	}
	if !f.enabled(ctx, f.ownerID) {
		return fileReconcileResult{Skipped: true}, nil
	}
	if freshVolumeMarked(f.root) {
		return fileReconcileResult{}, fmt.Errorf("file objects: the files root is a fresh volume awaiting hydration; not reconciling")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	present, err := f.store.ListFileObjects(ctx, f.ownerID, f.computerID)
	if err != nil {
		return fileReconcileResult{}, fmt.Errorf("file objects: list: %w", err)
	}
	known := make(map[string]choirstore.FileObject, len(present))
	for _, obj := range present {
		known[obj.Path] = obj
	}
	var res fileReconcileResult
	var changes []choirstore.FileChange
	var fresh []choirstore.FileRecord
	seen := make(map[string]bool)
	err = filepath.WalkDir(f.root, func(abs string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if abs == f.root {
			return nil
		}
		rel, err := filepath.Rel(f.root, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if rel == fileObjectExemptDir {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		res.Scanned++
		seen[rel] = true
		obj, ok := known[rel]
		if ok && obj.Size == info.Size() && obj.ModTime.Equal(info.ModTime().UTC()) && obj.Mode == uint32(info.Mode().Perm()) {
			return nil
		}
		rec, err := f.fileRecordAt(rel)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				delete(seen, rel)
				return nil
			}
			return err
		}
		res.Hashed++
		if ok {
			if rec.SHA256 != obj.SHA256 {
				res.Diverged++
			}
			changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpWrite, Record: rec, RecordedBy: "reconcile"})
			return nil
		}
		fresh = append(fresh, rec)
		return nil
	})
	if err != nil {
		return fileReconcileResult{}, fmt.Errorf("file objects: walk: %w", err)
	}

	var vanished []choirstore.FileObject
	for _, obj := range present {
		if !seen[obj.Path] && !exemptFilePath(obj.Path) {
			vanished = append(vanished, obj)
		}
	}
	holdDeletions := len(present) > fileObjectMassDeleteFloor && len(vanished)*2 > len(present)
	if holdDeletions {
		res.DeletionsHeld = len(vanished)
		res.Note = "more than half of the present files are gone from the tree; deletions and moves held"
		log.Printf("file objects: reconcile holds %d of %d deletions: %s", len(vanished), len(present), res.Note)
		vanished = nil
	}
	// A vanished file and a new one with the same digest, one of each, is a
	// move: the id is kept.
	vanishedByDigest := map[string][]choirstore.FileObject{}
	for _, obj := range vanished {
		vanishedByDigest[obj.SHA256] = append(vanishedByDigest[obj.SHA256], obj)
	}
	freshByDigest := map[string]int{}
	for _, rec := range fresh {
		freshByDigest[rec.SHA256]++
	}
	movedFrom := map[string]bool{}
	for _, rec := range fresh {
		if from := vanishedByDigest[rec.SHA256]; len(from) == 1 && freshByDigest[rec.SHA256] == 1 {
			changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpMove, FromPath: from[0].Path, Record: rec, RecordedBy: "reconcile"})
			movedFrom[from[0].Path] = true
			continue
		}
		changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpWrite, Record: rec, RecordedBy: "reconcile"})
	}
	for _, obj := range vanished {
		if !movedFrom[obj.Path] {
			changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpDelete, Record: choirstore.FileRecord{Path: obj.Path}, RecordedBy: "reconcile"})
		}
	}
	for start := 0; start < len(changes); start += fileObjectBatchSize {
		end := start + fileObjectBatchSize
		if end > len(changes) {
			end = len(changes)
		}
		applied, err := f.store.ApplyFileChanges(ctx, f.ownerID, f.computerID, changes[start:end], time.Now())
		if err != nil {
			return res, fmt.Errorf("file objects: apply: %w", err)
		}
		res.Created += applied.Created
		res.Updated += applied.Updated
		res.Moved += applied.Moved
		res.Deleted += applied.Deleted
	}
	return res, nil
}

// StartPeriodicFileObjectReconcile runs a reconcile shortly after start and
// then every interval. With the switch off a tick reads one setting.
func StartPeriodicFileObjectReconcile(ctx context.Context, f *fileObjects, interval time.Duration) {
	if f == nil || interval <= 0 {
		return
	}
	go func() {
		first := time.NewTimer(time.Minute)
		defer first.Stop()
		select {
		case <-ctx.Done():
			return
		case <-first.C:
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if res, err := f.Reconcile(ctx); err != nil {
				log.Printf("file objects: reconcile: %v", err)
			} else if !res.Skipped && (res.Writes() > 0 || res.DeletionsHeld > 0) {
				log.Printf("file objects: reconcile scanned=%d hashed=%d created=%d updated=%d moved=%d deleted=%d held=%d diverged=%d",
					res.Scanned, res.Hashed, res.Created, res.Updated, res.Moved, res.Deleted, res.DeletionsHeld, res.Diverged)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func fileObjectIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CHOIR_FILE_OBJECT_RECONCILE_INTERVAL"))
	if raw == "" {
		return defaultFileObjectInterval
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < time.Minute {
		return defaultFileObjectInterval
	}
	return d
}

// HandleObjects serves GET /api/file-objects: ?path= (the live file at a
// path), ?id= (a file by id, deleted included), or the present files paged by
// ?after=<path>&limit=.
func (f *fileObjects) HandleObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ownerID, err := authenticatedFilesOwner(r)
	if err != nil || ownerID == "" {
		writeFileError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	ctx := r.Context()
	enabled := f.enabled(ctx, ownerID)
	q := r.URL.Query()
	if id := strings.TrimSpace(q.Get("id")); id != "" {
		obj, err := f.store.GetFileObject(ctx, ownerID, id)
		f.writeOne(w, obj, err, enabled)
		return
	}
	if p := strings.TrimSpace(q.Get("path")); p != "" {
		obj, err := f.store.GetFileObjectByPath(ctx, ownerID, f.computerID, p)
		f.writeOne(w, obj, err, enabled)
		return
	}
	list, err := f.store.ListFileObjects(ctx, ownerID, f.computerID)
	if err != nil {
		writeFileError(w, http.StatusInternalServerError, "internal error")
		return
	}
	limit := 500
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 && n <= 5000 {
		limit = n
	}
	after := strings.TrimSpace(q.Get("after"))
	page := make([]choirstore.FileObject, 0, limit)
	next := ""
	for _, obj := range list {
		if after != "" && obj.Path <= after {
			continue
		}
		if len(page) == limit {
			next = page[len(page)-1].Path
			break
		}
		page = append(page, obj)
	}
	writeFilesJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "files": page, "next": next})
}

func (f *fileObjects) writeOne(w http.ResponseWriter, obj choirstore.FileObject, err error, enabled bool) {
	if errors.Is(err, choirstore.ErrNotFound) {
		writeFilesJSON(w, http.StatusNotFound, map[string]any{"error": "not found", "enabled": enabled})
		return
	}
	if err != nil {
		writeFileError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeFilesJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "file": obj})
}

// HandleReconcile serves POST /api/file-objects/reconcile: one reconcile now,
// for the computer's owner.
func (f *fileObjects) HandleReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeFileError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ownerID, err := authenticatedFilesOwner(r)
	if err != nil || ownerID == "" {
		writeFileError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if ownerID != f.ownerID {
		writeFileError(w, http.StatusForbidden, "only the computer's owner reconciles its files")
		return
	}
	res, err := f.Reconcile(r.Context())
	if err != nil {
		writeFilesJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeFilesJSON(w, http.StatusOK, map[string]any{"result": res, "record_failures": f.recordFailures.Load()})
}

// RegisterFileObjectRoutes registers the file object read path and reconcile.
func RegisterFileObjectRoutes(s interface {
	HandleFunc(string, http.HandlerFunc)
}, f *fileObjects) {
	if f == nil {
		return
	}
	s.HandleFunc("/api/file-objects", f.HandleObjects)
	s.HandleFunc("/api/file-objects/reconcile", f.HandleReconcile)
}
