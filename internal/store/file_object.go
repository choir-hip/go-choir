package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The file object, slice 1 (docs/design/file-object-2026-10-24.md; doctrine
// I27, H038). A file is an object with a stable id, the plaintext digest of
// its bytes and its path; the path view maps a live path to the id, so a
// rename changes the view and keeps the object. The bytes stay in the files
// tree and file CAS in this slice.
const (
	ogKindFile     = objectgraph.ObjectKind("choir.file")
	ogKindFilePath = objectgraph.ObjectKind("choir.file_path")
)

// SettingFilesObject turns on the file object for one computer: the Files API
// and the reconcile pass record choir.file and choir.file_path. Off by
// default and read live on every write; off is the rollback (the objects
// already written stay as inert rows).
const SettingFilesObject = "files.object"

func init() {
	KnownComputerSettings[SettingFilesObject] = false
}

// ErrFileObjectsOff is returned by ApplyFileChanges when the files.object
// setting is not on for the computer.
var ErrFileObjectsOff = errors.New("file object: the files.object setting is off")

// File states. A deleted file keeps its object, so a reference to its id
// resolves to a file that was deleted, never to nothing.
const (
	FileStatePresent = "present"
	FileStateDeleted = "deleted"
)

// FileObject is the body of choir.file.
type FileObject struct {
	FileID     string    `json:"file_id"`
	OwnerID    string    `json:"owner_id"`
	ComputerID string    `json:"computer_id"`
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Digest     string    `json:"digest"`
	Size       int64     `json:"size"`
	Mode       uint32    `json:"mode"`
	MediaType  string    `json:"media_type,omitempty"`
	ModTime    time.Time `json:"mod_time"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DeletedAt  time.Time `json:"deleted_at,omitempty"`
	RecordedBy string    `json:"recorded_by,omitempty"`
}

// filePathView is the body of choir.file_path: one live path's file.
type filePathView struct {
	Path       string `json:"path"`
	FileID     string `json:"file_id"`
	ComputerID string `json:"computer_id"`
}

// FileRecord is what the recorder read from disk for one path.
type FileRecord struct {
	Path      string
	Digest    string
	Size      int64
	Mode      uint32
	ModTime   time.Time
	MediaType string
}

// File change operations.
const (
	FileOpWrite  = "write"
	FileOpDelete = "delete"
	FileOpMove   = "move"
)

// FileChange is one mutation of the files tree to record. Write and move carry
// the destination's Record; delete needs only Record.Path; move names its
// source in FromPath.
type FileChange struct {
	Op         string
	FromPath   string
	Record     FileRecord
	RecordedBy string
}

// FileApplyResult counts what ApplyFileChanges did; FileIDs is the file id of
// each change, in order ("" when the change touched no file).
type FileApplyResult struct {
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Moved     int      `json:"moved"`
	Deleted   int      `json:"deleted"`
	Unchanged int      `json:"unchanged"`
	FileIDs   []string `json:"-"`
}

// NormalizeFilePath cleans a files-root-relative path. It refuses the root
// itself and any path that would leave the root. It changes nothing the
// filesystem keeps literal: a backslash or a space is part of a file name
// (rewriting either made two files share one object path, review bunch 208),
// so the only edits are the slash-separated ones Files API paths already get.
func NormalizeFilePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("file object: path is required")
	}
	rel := strings.TrimLeft(raw, "/")
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("file object: path is the files root")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("file object: path escapes the files root")
	}
	return cleaned, nil
}

func newFileID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("file object: mint id: %w", err)
	}
	return "file_" + hex.EncodeToString(b[:]), nil
}

func fileObjectID(ownerID, fileID string) (string, error) {
	return objectgraph.BuildCanonicalID(ogKindFile, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(strings.TrimSpace(fileID)))
}

func filePathViewID(ownerID, computerID, p string) (string, error) {
	return objectgraph.BuildCanonicalID(ogKindFilePath, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(strings.TrimSpace(computerID)+"\x00"+p))
}

func fileGraphObject(kind objectgraph.ObjectKind, canonicalID, ownerID, computerID string, body any, metadata map[string]any, createdAt, updatedAt time.Time) (objectgraph.Object, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal %s: %w", kind, err)
	}
	metadataJSON, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("metadata %s: %w", kind, err)
	}
	if createdAt.IsZero() {
		createdAt = updatedAt
	}
	return objectgraph.Object{
		CanonicalID: canonicalID,
		ObjectKind:  kind,
		OwnerID:     strings.TrimSpace(ownerID),
		ComputerID:  strings.TrimSpace(computerID),
		ContentHash: objectgraph.ContentHash(kind, bodyJSON, metadataJSON),
		Body:        bodyJSON,
		Metadata:    metadataJSON,
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   updatedAt.UTC(),
	}, nil
}

// fileBatch is one ApplyFileChanges call: the objects to write, keyed by
// canonical id so a later change in the batch replaces an earlier one, read
// through before the store.
type fileBatch struct {
	s          *Store
	ctx        context.Context
	ownerID    string
	computerID string
	now        time.Time
	order      []string
	objects    map[string]objectgraph.Object
}

func (b *fileBatch) put(obj objectgraph.Object) {
	if _, ok := b.objects[obj.CanonicalID]; !ok {
		b.order = append(b.order, obj.CanonicalID)
	}
	b.objects[obj.CanonicalID] = obj
}

func (b *fileBatch) get(id string) (objectgraph.Object, error) {
	if obj, ok := b.objects[id]; ok {
		return obj, nil
	}
	return b.s.ogStore.GetObject(b.ctx, id)
}

// view returns the file id at a live path, or "".
func (b *fileBatch) view(p string) (string, string, error) {
	id, err := filePathViewID(b.ownerID, b.computerID, p)
	if err != nil {
		return "", "", err
	}
	obj, err := b.get(id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return id, "", nil
	}
	if err != nil {
		return id, "", err
	}
	if obj.Tombstone {
		return id, "", nil
	}
	var v filePathView
	if err := ogDecode(obj, &v); err != nil {
		return id, "", err
	}
	return id, strings.TrimSpace(v.FileID), nil
}

func (b *fileBatch) file(fileID string) (FileObject, string, error) {
	id, err := fileObjectID(b.ownerID, fileID)
	if err != nil {
		return FileObject{}, "", err
	}
	obj, err := b.get(id)
	if err != nil {
		return FileObject{}, id, err
	}
	var f FileObject
	if err := ogDecode(obj, &f); err != nil {
		return FileObject{}, id, err
	}
	return f, id, nil
}

func (b *fileBatch) putFile(f FileObject) error {
	id, err := fileObjectID(b.ownerID, f.FileID)
	if err != nil {
		return err
	}
	obj, err := fileGraphObject(ogKindFile, id, b.ownerID, b.computerID, f, map[string]any{
		"file_id":     f.FileID,
		"computer_id": f.ComputerID,
		"path":        f.Path,
		"digest":      f.Digest,
		"state":       f.State,
		"updated_at":  f.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return err
	}
	b.put(obj)
	return nil
}

func (b *fileBatch) putView(p, fileID string) error {
	id, err := filePathViewID(b.ownerID, b.computerID, p)
	if err != nil {
		return err
	}
	obj, err := fileGraphObject(ogKindFilePath, id, b.ownerID, b.computerID, filePathView{Path: p, FileID: fileID, ComputerID: b.computerID}, map[string]any{
		"computer_id": b.computerID,
		"path":        p,
		"file_id":     fileID,
	}, b.now, b.now)
	if err != nil {
		return err
	}
	b.put(obj)
	return nil
}

func (b *fileBatch) dropView(viewID string) error {
	obj, err := b.get(viewID)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if obj.Tombstone {
		return nil
	}
	obj.Tombstone = true
	obj.UpdatedAt = b.now
	b.put(obj)
	return nil
}

// markDeleted moves the file at a live view to deleted and drops the view.
func (b *fileBatch) markDeleted(viewID, fileID, by string) error {
	f, _, err := b.file(fileID)
	if err != nil && !errors.Is(err, objectgraph.ErrNotFound) {
		return err
	}
	if err == nil && f.State != FileStateDeleted {
		f.State = FileStateDeleted
		f.DeletedAt = b.now
		f.UpdatedAt = b.now
		if by != "" {
			f.RecordedBy = by
		}
		if err := b.putFile(f); err != nil {
			return err
		}
	}
	return b.dropView(viewID)
}

func applyRecord(f *FileObject, rec FileRecord, p string) {
	f.Path = p
	f.Name = path.Base(p)
	f.Digest = strings.TrimSpace(rec.Digest)
	f.Size = rec.Size
	f.Mode = rec.Mode
	f.MediaType = strings.TrimSpace(rec.MediaType)
	f.ModTime = rec.ModTime.UTC()
	f.State = FileStatePresent
	f.DeletedAt = time.Time{}
}

// write records the bytes at p: the file at p keeps its id, or a new file is
// minted with its view. It reports whether the file was created.
func (b *fileBatch) write(rec FileRecord, p, by string) (string, bool, error) {
	_, fileID, err := b.view(p)
	if err != nil {
		return "", false, err
	}
	if fileID != "" {
		f, _, err := b.file(fileID)
		if err == nil && f.State == FileStatePresent {
			applyRecord(&f, rec, p)
			f.UpdatedAt = b.now
			if by != "" {
				f.RecordedBy = by
			}
			return fileID, false, b.putFile(f)
		}
		if err != nil && !errors.Is(err, objectgraph.ErrNotFound) {
			return "", false, err
		}
	}
	fileID, err = newFileID()
	if err != nil {
		return "", false, err
	}
	f := FileObject{FileID: fileID, OwnerID: b.ownerID, ComputerID: b.computerID, CreatedAt: b.now, UpdatedAt: b.now, RecordedBy: by}
	applyRecord(&f, rec, p)
	if err := b.putFile(f); err != nil {
		return "", false, err
	}
	return fileID, true, b.putView(p, fileID)
}

// ApplyFileChanges records changes to the files tree in one object graph batch
// (one projection batch on the tape when it is bound). Writer: system:runtime,
// acting for the Files API or the reconcile pass.
func (s *Store) ApplyFileChanges(ctx context.Context, ownerID, computerID string, changes []FileChange, now time.Time) (FileApplyResult, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return FileApplyResult{}, fmt.Errorf("file object: owner_id and computer_id are required")
	}
	if s.ogStore == nil {
		return FileApplyResult{}, fmt.Errorf("store: object graph not initialized")
	}
	// The switch is checked here, at the only writer, not only by the callers:
	// a caller that forgets it, or a switch turned off during a long pass,
	// still cannot write.
	setting, err := s.GetComputerSetting(ctx, ownerID, computerID, SettingFilesObject)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return FileApplyResult{}, fmt.Errorf("file object: read %s: %w", SettingFilesObject, err)
	}
	if err != nil || !setting.Value {
		return FileApplyResult{}, ErrFileObjectsOff
	}
	if now.IsZero() {
		now = time.Now()
	}
	ctx = objectgraph.WithWriter(ctx, objectgraph.WriterRuntime)
	b := &fileBatch{s: s, ctx: ctx, ownerID: ownerID, computerID: computerID, now: now.UTC(), objects: map[string]objectgraph.Object{}}
	var res FileApplyResult
	for _, change := range changes {
		p, err := NormalizeFilePath(change.Record.Path)
		if err != nil {
			return FileApplyResult{}, err
		}
		switch change.Op {
		case FileOpWrite:
			id, created, err := b.write(change.Record, p, change.RecordedBy)
			if err != nil {
				return FileApplyResult{}, err
			}
			if created {
				res.Created++
			} else {
				res.Updated++
			}
			res.FileIDs = append(res.FileIDs, id)
		case FileOpDelete:
			viewID, fileID, err := b.view(p)
			if err != nil {
				return FileApplyResult{}, err
			}
			if fileID == "" {
				res.Unchanged++
				res.FileIDs = append(res.FileIDs, "")
				continue
			}
			if err := b.markDeleted(viewID, fileID, change.RecordedBy); err != nil {
				return FileApplyResult{}, err
			}
			res.Deleted++
			res.FileIDs = append(res.FileIDs, fileID)
		case FileOpMove:
			from, err := NormalizeFilePath(change.FromPath)
			if err != nil {
				return FileApplyResult{}, err
			}
			fromView, fileID, err := b.view(from)
			if err != nil {
				return FileApplyResult{}, err
			}
			var f FileObject
			if fileID != "" {
				f, _, err = b.file(fileID)
				if err != nil && !errors.Is(err, objectgraph.ErrNotFound) {
					return FileApplyResult{}, err
				}
				if err != nil || f.State != FileStatePresent {
					fileID = ""
				}
			}
			if fileID == "" || from == p {
				// Nothing known at the source (or a move onto itself): the
				// destination's bytes are recorded as a write.
				id, created, err := b.write(change.Record, p, change.RecordedBy)
				if err != nil {
					return FileApplyResult{}, err
				}
				if created {
					res.Created++
				} else {
					res.Updated++
				}
				res.FileIDs = append(res.FileIDs, id)
				continue
			}
			// A live file at the destination is replaced: it is deleted.
			toView, targetID, err := b.view(p)
			if err != nil {
				return FileApplyResult{}, err
			}
			if targetID != "" && targetID != fileID {
				if err := b.markDeleted(toView, targetID, change.RecordedBy); err != nil {
					return FileApplyResult{}, err
				}
			}
			rec := change.Record
			if strings.TrimSpace(rec.Digest) == "" {
				rec = FileRecord{Digest: f.Digest, Size: f.Size, Mode: f.Mode, ModTime: f.ModTime, MediaType: f.MediaType}
			}
			applyRecord(&f, rec, p)
			f.UpdatedAt = b.now
			if change.RecordedBy != "" {
				f.RecordedBy = change.RecordedBy
			}
			if err := b.putFile(f); err != nil {
				return FileApplyResult{}, err
			}
			if err := b.dropView(fromView); err != nil {
				return FileApplyResult{}, err
			}
			if err := b.putView(p, fileID); err != nil {
				return FileApplyResult{}, err
			}
			res.Moved++
			res.FileIDs = append(res.FileIDs, fileID)
		default:
			return FileApplyResult{}, fmt.Errorf("file object: unknown change %q", change.Op)
		}
	}
	if len(b.order) == 0 {
		return res, nil
	}
	objects := make([]objectgraph.Object, 0, len(b.order))
	for _, id := range b.order {
		objects = append(objects, b.objects[id])
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		return FileApplyResult{}, fmt.Errorf("file object: write batch: %w", err)
	}
	return res, nil
}

// GetFileObject returns a file of this computer by id, deleted files
// included, or ErrNotFound. Another computer's file id is not found: the id is
// random, but the read path is scoped by computer as the path and list reads are.
func (s *Store) GetFileObject(ctx context.Context, ownerID, computerID, fileID string) (FileObject, error) {
	if s.ogStore == nil {
		return FileObject{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := fileObjectID(ownerID, fileID)
	if err != nil {
		return FileObject{}, err
	}
	obj, err := s.ogStore.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return FileObject{}, ErrNotFound
	}
	if err != nil {
		return FileObject{}, err
	}
	if obj.Tombstone || obj.ObjectKind != ogKindFile {
		return FileObject{}, ErrNotFound
	}
	var f FileObject
	if err := ogDecode(obj, &f); err != nil {
		return FileObject{}, err
	}
	if f.ComputerID != strings.TrimSpace(computerID) {
		return FileObject{}, ErrNotFound
	}
	return f, nil
}

// GetFileObjectByPath returns the live file at a path through the path view,
// or ErrNotFound.
func (s *Store) GetFileObjectByPath(ctx context.Context, ownerID, computerID, rawPath string) (FileObject, error) {
	if s.ogStore == nil {
		return FileObject{}, fmt.Errorf("store: object graph not initialized")
	}
	p, err := NormalizeFilePath(rawPath)
	if err != nil {
		return FileObject{}, err
	}
	b := &fileBatch{s: s, ctx: ctx, ownerID: strings.TrimSpace(ownerID), computerID: strings.TrimSpace(computerID), objects: map[string]objectgraph.Object{}}
	_, fileID, err := b.view(p)
	if err != nil {
		return FileObject{}, err
	}
	if fileID == "" {
		return FileObject{}, ErrNotFound
	}
	f, err := s.GetFileObject(ctx, ownerID, computerID, fileID)
	if err != nil {
		return FileObject{}, err
	}
	if f.State != FileStatePresent {
		return FileObject{}, ErrNotFound
	}
	return f, nil
}

// ListFileObjects returns the present files of one computer, sorted by path.
func (s *Store) ListFileObjects(ctx context.Context, ownerID, computerID string) ([]FileObject, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	objs, err := s.ogListAllByMetadata(ctx, ogKindFile, "state", FileStatePresent)
	if err != nil {
		return nil, err
	}
	out := make([]FileObject, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone || obj.OwnerID != ownerID || obj.ComputerID != computerID {
			continue
		}
		var f FileObject
		if err := ogDecode(obj, &f); err != nil {
			return nil, err
		}
		if f.State == FileStatePresent {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
