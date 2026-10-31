package autoputer

import (
	"context"
	"crypto/hmac"
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
	GetFileObject(ctx context.Context, ownerID, computerID, fileID string) (choirstore.FileObject, error)
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
	// passMu serializes reconcile passes (the ticker and the POST route) and
	// guards pendingGone. mu serializes the apply phase of a pass with the
	// Files API's Record, never the walk or the hashing.
	passMu     sync.Mutex
	mu         sync.Mutex
	root       string
	store      fileObjectStore
	ownerID    string
	computerID string
	// recordFailures counts API writes whose object could not be recorded;
	// the next reconcile repairs them.
	recordFailures atomic.Uint64
	// digestKey returns the HMAC key for the file digest, derived in the
	// guest from the computer's DEK (filecas.FileDigestKey). With none, no
	// object is recorded: the plaintext digest is never written instead.
	digestKey func(context.Context) ([]byte, error)
	// readyCheck says the files tree is the owner's: boot hydration settled
	// (or there is none). A reconcile on a tree that is only absent, not
	// gone, would record deletions.
	readyCheck func() bool
	// pendingGone holds the objects the previous pass found absent
	// (path -> file id). A deletion is recorded when a second pass finds the
	// same file still absent, so a file that is only temporarily absent is
	// never deleted. A restart forgets it, which only delays a deletion.
	pendingGone map[string]string
	// afterWalk is a test seam: it runs after the walk, before the apply.
	afterWalk func()
}

func newFileObjects(root string, st fileObjectStore, ownerID, computerID string, digestKey func(context.Context) ([]byte, error)) *fileObjects {
	if st == nil {
		return nil
	}
	return &fileObjects{root: filepath.Clean(root), store: st, ownerID: strings.TrimSpace(ownerID), computerID: strings.TrimSpace(computerID), digestKey: digestKey}
}

// SetReadyCheck sets the function that says the files tree is settled.
func (f *fileObjects) SetReadyCheck(fn func() bool) {
	if f != nil {
		f.readyCheck = fn
	}
}

// MarkFilesReady settles the readiness check for good (tests, and a computer
// with no file sync to wait for).
func (f *fileObjects) MarkFilesReady() { f.SetReadyCheck(func() bool { return true }) }

func (f *fileObjects) filesReady() bool { return f != nil && f.readyCheck != nil && f.readyCheck() }

// key returns the digest key. The caller clears it.
func (f *fileObjects) key(ctx context.Context) ([]byte, error) {
	if f.digestKey == nil {
		return nil, errors.New("file objects: no digest key; nothing is recorded without one")
	}
	key, err := f.digestKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("file objects: digest key: %w", err)
	}
	if len(key) == 0 {
		return nil, errors.New("file objects: empty digest key")
	}
	return key, nil
}

// enabled reads the switch live. A never-stored switch reads its declared
// default; any other read failure is off.
func (f *fileObjects) enabled(ctx context.Context, ownerID string) bool {
	if f == nil || strings.TrimSpace(ownerID) == "" || f.computerID == "" {
		return false
	}
	setting, err := f.store.GetComputerSetting(ctx, ownerID, f.computerID, choirstore.SettingFilesObject)
	if err != nil {
		if errors.Is(err, choirstore.ErrNotFound) {
			return choirstore.KnownComputerSettings[choirstore.SettingFilesObject]
		}
		log.Printf("file objects: read %s: %v; treating it as off", choirstore.SettingFilesObject, err)
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
// streaming with HMAC-SHA-256 under key: the recorded digest is keyed, so it
// is no oracle for a known plaintext.
func (f *fileObjects) fileRecordAt(rel string, key []byte) (choirstore.FileRecord, error) {
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
	h := hmac.New(sha256.New, key)
	if _, err := io.Copy(h, file); err != nil {
		return choirstore.FileRecord{}, err
	}
	return recordFromInfo(rel, info, "hmac-sha256:"+hex.EncodeToString(h.Sum(nil))), nil
}

func recordFromInfo(rel string, info fs.FileInfo, digest string) choirstore.FileRecord {
	return choirstore.FileRecord{
		Path:      rel,
		Digest:    digest,
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
		key, err := f.key(ctx)
		if err != nil {
			f.recordFailure(rel, err)
			return
		}
		defer clear(key)
		rec, err := f.fileRecordAt(rel, key)
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
		if errors.Is(err, choirstore.ErrFileObjectsOff) {
			return // switched off between the check above and the write
		}
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
	Unreadable    int    `json:"unreadable,omitempty"`
	Raced         int    `json:"raced,omitempty"`
	Note          string `json:"note,omitempty"`
}

// Writes is the number of objects the pass changed.
func (r fileReconcileResult) Writes() int { return r.Created + r.Updated + r.Moved + r.Deleted }

// matchesDisk says the file at rec.Path is still the regular file rec was read
// from: a record the walk hashed moments ago may have been rewritten since.
func (f *fileObjects) matchesDisk(rec choirstore.FileRecord) bool {
	info, err := os.Lstat(filepath.Join(f.root, filepath.FromSlash(rec.Path)))
	return err == nil && info.Mode().IsRegular() && info.Size() == rec.Size &&
		info.ModTime().UTC().Equal(rec.ModTime) && uint32(info.Mode().Perm()) == rec.Mode
}

// absentFromDisk says nothing exists at rel (not even a symlink): a stat that
// fails any other way proves nothing and is not absence.
func (f *fileObjects) absentFromDisk(rel string) bool {
	_, err := os.Lstat(filepath.Join(f.root, filepath.FromSlash(rel)))
	return errors.Is(err, fs.ErrNotExist)
}

// stillOn says the pass may write: the switch is on and the volume is not a
// fresh one awaiting hydration. It is read again before every batch.
func (f *fileObjects) stillOn(ctx context.Context) bool {
	return f.enabled(ctx, f.ownerID) && !freshVolumeMarked(f.root)
}

func underAny(rel string, dirs []string) bool {
	for _, d := range dirs {
		if strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}

// Reconcile makes the objects state the files tree: it records files written
// outside the Files API, moves found by a unique digest, and deletions.
//
// It walks and hashes without holding the Files API still, then applies under
// the recorder lock after checking each record against the disk again. A file
// is deleted only when it is absent at apply time in two passes in a row, and
// never from a tree that looks emptied. It never runs on a fresh volume or
// before boot hydration settles, and it stops writing the moment the switch
// goes off.
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
	if !f.filesReady() {
		return fileReconcileResult{}, fmt.Errorf("file objects: the files tree is not settled (boot hydration pending or failed); not reconciling")
	}
	f.passMu.Lock()
	defer f.passMu.Unlock()
	key, err := f.key(ctx)
	if err != nil {
		return fileReconcileResult{}, err
	}
	defer clear(key)

	// The snapshot only decides which files need hashing; the apply below
	// reads the objects again.
	snapshot, err := f.store.ListFileObjects(ctx, f.ownerID, f.computerID)
	if err != nil {
		return fileReconcileResult{}, fmt.Errorf("file objects: list: %w", err)
	}
	known := make(map[string]choirstore.FileObject, len(snapshot))
	for _, obj := range snapshot {
		known[obj.Path] = obj
	}
	var res fileReconcileResult
	var changes []choirstore.FileChange
	var fresh []choirstore.FileRecord
	seen := make(map[string]bool)
	// Directories the walk could not read: what is under them is unknown, not
	// gone, so nothing under one is a deletion candidate.
	var unreadableDirs []string
	err = filepath.WalkDir(f.root, func(abs string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrPermission) && d != nil && d.IsDir() && abs != f.root {
				if rel, err := filepath.Rel(f.root, abs); err == nil {
					unreadableDirs = append(unreadableDirs, filepath.ToSlash(rel)+"/")
					res.Unreadable++
					return filepath.SkipDir
				}
			}
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
		if exemptFilePath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
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
		rec, err := f.fileRecordAt(rel, key)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				delete(seen, rel)
				return nil
			}
			if errors.Is(err, fs.ErrPermission) {
				// Present but unreadable: it is not gone, and it must not
				// stop the pass for every other file.
				res.Unreadable++
				return nil
			}
			return err
		}
		res.Hashed++
		if ok {
			if rec.Digest != obj.Digest {
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
	if f.afterWalk != nil {
		f.afterWalk()
	}

	// The apply phase: the Files API's Record waits only from here.
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.stillOn(ctx) {
		res.Skipped, res.Note = true, "the files.object switch turned off, or the volume went fresh, during the pass; nothing written"
		return res, nil
	}
	present, err := f.store.ListFileObjects(ctx, f.ownerID, f.computerID)
	if err != nil {
		return fileReconcileResult{}, fmt.Errorf("file objects: list: %w", err)
	}

	// Drop records that went stale since they were hashed; the next pass
	// reads them again.
	kept := changes[:0]
	for _, c := range changes {
		if f.matchesDisk(c.Record) {
			kept = append(kept, c)
		} else {
			res.Raced++
		}
	}
	changes = kept
	keptFresh := fresh[:0]
	for _, rec := range fresh {
		if f.matchesDisk(rec) {
			keptFresh = append(keptFresh, rec)
		} else {
			res.Raced++
		}
	}
	fresh = keptFresh

	// Vanished: an object whose path the walk did not see and a fresh stat
	// finds empty. (A file the Files API wrote during the walk, or one that
	// came back, exists and is not vanished.)
	var vanished []choirstore.FileObject
	for _, obj := range present {
		if !seen[obj.Path] && !exemptFilePath(obj.Path) && !underAny(obj.Path, unreadableDirs) && f.absentFromDisk(obj.Path) {
			vanished = append(vanished, obj)
		}
	}
	// A tree that shows no file at all while objects exist, or one that lost
	// more than half of more than a few, is an unmounted or emptied volume,
	// not an owner's deletion spree.
	emptied := len(present) > 0 && len(seen) == 0
	mass := len(present) > fileObjectMassDeleteFloor && len(vanished)*2 > len(present)
	if len(vanished) > 0 && (emptied || mass) {
		res.DeletionsHeld = len(vanished)
		res.Note = "the tree looks emptied (no file, or more than half of the present files, gone); deletions and moves held"
		log.Printf("file objects: reconcile holds %d of %d deletions: %s", len(vanished), len(present), res.Note)
		vanished = nil
		f.pendingGone = nil
	}
	// A vanished file and a new one with the same digest, one of each, is a
	// move: the id is kept. An empty file's digest is every empty file's, so
	// size 0 never pairs.
	vanishedByDigest := map[string][]choirstore.FileObject{}
	for _, obj := range vanished {
		if obj.Size > 0 {
			vanishedByDigest[obj.Digest] = append(vanishedByDigest[obj.Digest], obj)
		}
	}
	freshByDigest := map[string]int{}
	for _, rec := range fresh {
		if rec.Size > 0 {
			freshByDigest[rec.Digest]++
		}
	}
	movedFrom := map[string]bool{}
	for _, rec := range fresh {
		if from := vanishedByDigest[rec.Digest]; rec.Size > 0 && len(from) == 1 && freshByDigest[rec.Digest] == 1 {
			changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpMove, FromPath: from[0].Path, Record: rec, RecordedBy: "reconcile"})
			movedFrom[from[0].Path] = true
			continue
		}
		changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpWrite, Record: rec, RecordedBy: "reconcile"})
	}
	// A deletion needs the same file absent in two passes in a row.
	nextPending := map[string]string{}
	for _, obj := range vanished {
		if movedFrom[obj.Path] {
			continue
		}
		nextPending[obj.Path] = obj.FileID
		if f.pendingGone[obj.Path] == obj.FileID {
			changes = append(changes, choirstore.FileChange{Op: choirstore.FileOpDelete, Record: choirstore.FileRecord{Path: obj.Path}, RecordedBy: "reconcile"})
		}
	}
	f.pendingGone = nextPending

	for start := 0; start < len(changes); start += fileObjectBatchSize {
		end := start + fileObjectBatchSize
		if end > len(changes) {
			end = len(changes)
		}
		if !f.stillOn(ctx) {
			res.Skipped, res.Note = true, "the files.object switch turned off, or the volume went fresh, during the pass; the rest not written"
			return res, nil
		}
		applied, err := f.store.ApplyFileChanges(ctx, f.ownerID, f.computerID, changes[start:end], time.Now())
		if err != nil {
			if errors.Is(err, choirstore.ErrFileObjectsOff) {
				res.Skipped, res.Note = true, "the files.object switch turned off during the pass; the rest not written"
				return res, nil
			}
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
			} else if !res.Skipped && (res.Writes() > 0 || res.DeletionsHeld > 0 || res.Unreadable > 0 || res.Raced > 0) {
				log.Printf("file objects: reconcile scanned=%d hashed=%d created=%d updated=%d moved=%d deleted=%d held=%d diverged=%d unreadable=%d raced=%d",
					res.Scanned, res.Hashed, res.Created, res.Updated, res.Moved, res.Deleted, res.DeletionsHeld, res.Diverged, res.Unreadable, res.Raced)
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
		obj, err := f.store.GetFileObject(ctx, ownerID, f.computerID, id)
		f.writeOne(w, obj, err, enabled)
		return
	}
	if p := q.Get("path"); p != "" {
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
