package autoputer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The file object's recorder and routes (docs/design/file-object-2026-10-24.md
// section 6). Ways it can fail, written before the code:
//  1. The switch off still records objects, or exposes the move verb.
//  2. A Files API write, rewrite, move or delete leaves the object untrue
//     (wrong digest, a new id on rewrite or move, the old path resolving).
//  3. A move to an existing destination, of a directory, or out of the root
//     goes through.
//  4. Reconcile on a fresh volume (image seed) marks every object deleted.
//  5. Reconcile on an emptied tree mass-deletes the objects.
//  6. Reconcile writes on an unchanged tree (steady-state write volume).
//  7. Reconcile walks Source/ (source and build state are exempt).
//  8. An out-of-band rename mints a new id when the digest pairs uniquely.
//  9. The read path answers for another owner, or hides the switch state.

const (
	foOwner    = "test-user@example.com"
	foComputer = "computer-file-objects"
)

type fileObjectFixture struct {
	db   *choirstore.Store
	root string
	fh   *FilesHandler
	fo   *fileObjects
}

func newFileObjectFixture(t *testing.T) *fileObjectFixture {
	t.Helper()
	db, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	root := t.TempDir()
	fo := newFileObjects(root, db, foOwner, foComputer)
	fh := NewFilesHandlerWithObserver(root, func(r *http.Request, event FileChangeEvent) {
		fo.Record(r.Context(), foOwner, event)
	})
	fh.SetMoveGate(fo.MoveAllowed)
	return &fileObjectFixture{db: db, root: root, fh: fh, fo: fo}
}

func (f *fileObjectFixture) setSwitch(t *testing.T, on bool) {
	t.Helper()
	if err := f.db.PutComputerSetting(context.Background(), choirstore.ComputerSetting{OwnerID: foOwner, ComputerID: foComputer, Key: choirstore.SettingFilesObject, Value: on}); err != nil {
		t.Fatal(err)
	}
}

func (f *fileObjectFixture) do(t *testing.T, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := authBodyRequest(method, path, bytes.NewReader(body))
	w := httptest.NewRecorder()
	f.fh.HandleFileByPath(w, req)
	return w
}

func (f *fileObjectFixture) object(t *testing.T, query string) (int, choirstore.FileObject) {
	t.Helper()
	req := authRequest(http.MethodGet, "/api/file-objects?"+query)
	w := httptest.NewRecorder()
	f.fo.HandleObjects(w, req)
	var resp struct {
		File choirstore.FileObject `json:"file"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp.File
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestFileObjectSwitchOffRecordsNothingAndHidesMove(t *testing.T) {
	f := newFileObjectFixture(t)
	if w := f.do(t, http.MethodPut, "/api/files/a.txt", []byte("hello")); w.Code != http.StatusOK {
		t.Fatalf("put = %d", w.Code)
	}
	if code, _ := f.object(t, "path=a.txt"); code != http.StatusNotFound {
		t.Fatalf("object with the switch off = %d, want 404", code)
	}
	if w := f.do(t, http.MethodPatch, "/api/files/a.txt", []byte(`{"move_to":"b.txt"}`)); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH with the switch off = %d, want 405 as before", w.Code)
	}
	res, err := f.fo.Reconcile(context.Background())
	if err != nil || !res.Skipped {
		t.Fatalf("reconcile with the switch off = %+v, %v; want skipped", res, err)
	}
	list, _ := f.db.ListFileObjects(context.Background(), foOwner, foComputer)
	if len(list) != 0 {
		t.Fatalf("objects with the switch off = %+v", list)
	}
	req := authRequest(http.MethodGet, "/api/file-objects")
	w := httptest.NewRecorder()
	f.fo.HandleObjects(w, req)
	if !bytes.Contains(w.Body.Bytes(), []byte(`"enabled":false`)) {
		t.Fatalf("list = %s, want enabled:false", w.Body.String())
	}
}

func TestFileObjectFollowsTheFilesAPI(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	if err := os.MkdirAll(filepath.Join(f.root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	first := []byte("first bytes")
	if w := f.do(t, http.MethodPut, "/api/files/docs/a.txt", first); w.Code != http.StatusOK {
		t.Fatalf("put = %d", w.Code)
	}
	code, obj := f.object(t, "path=docs/a.txt")
	if code != http.StatusOK || obj.SHA256 != digestOf(first) || obj.Size != int64(len(first)) || obj.FileID == "" || obj.MediaType != "text/plain; charset=utf-8" {
		t.Fatalf("after put = %d %+v", code, obj)
	}
	id := obj.FileID

	second := []byte("second, longer bytes")
	f.do(t, http.MethodPut, "/api/files/docs/a.txt", second)
	_, obj = f.object(t, "path=docs/a.txt")
	if obj.FileID != id || obj.SHA256 != digestOf(second) {
		t.Fatalf("after rewrite = %+v, want id %s and the new digest", obj, id)
	}

	if w := f.do(t, http.MethodPatch, "/api/files/docs/a.txt", []byte(`{"move_to":"b.txt"}`)); w.Code != http.StatusOK {
		t.Fatalf("move = %d %s", w.Code, w.Body.String())
	}
	if got, err := os.ReadFile(filepath.Join(f.root, "b.txt")); err != nil || !bytes.Equal(got, second) {
		t.Fatalf("moved bytes = %q, %v", got, err)
	}
	if code, _ := f.object(t, "path=docs/a.txt"); code != http.StatusNotFound {
		t.Fatalf("old path after move = %d, want 404", code)
	}
	_, obj = f.object(t, "path=b.txt")
	if obj.FileID != id || obj.Path != "b.txt" {
		t.Fatalf("new path after move = %+v, want id %s", obj, id)
	}

	if w := f.do(t, http.MethodDelete, "/api/files/b.txt", nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d", w.Code)
	}
	if code, _ := f.object(t, "path=b.txt"); code != http.StatusNotFound {
		t.Fatalf("path after delete = %d", code)
	}
	code, obj = f.object(t, "id="+id)
	if code != http.StatusOK || obj.State != choirstore.FileStateDeleted {
		t.Fatalf("by id after delete = %d %+v, want state deleted", code, obj)
	}
}

func TestFileObjectMoveRefusals(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.do(t, http.MethodPut, "/api/files/a.txt", []byte("a"))
	f.do(t, http.MethodPut, "/api/files/b.txt", []byte("b"))
	if err := os.Mkdir(filepath.Join(f.root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"/api/files/a.txt", `{"move_to":"b.txt"}`, http.StatusConflict},
		{"/api/files/dir", `{"move_to":"dir2"}`, http.StatusConflict},
		{"/api/files/a.txt", `{"move_to":"../escape.txt"}`, http.StatusForbidden},
		{"/api/files/a.txt", `{"move_to":"missing/parent.txt"}`, http.StatusNotFound},
		{"/api/files/nope.txt", `{"move_to":"c.txt"}`, http.StatusNotFound},
		{"/api/files/a.txt", `{}`, http.StatusBadRequest},
	} {
		if w := f.do(t, http.MethodPatch, tc.path, []byte(tc.body)); w.Code != tc.want {
			t.Errorf("PATCH %s %s = %d %s, want %d", tc.path, tc.body, w.Code, w.Body.String(), tc.want)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(f.root, "b.txt")); string(got) != "b" {
		t.Fatalf("b.txt was overwritten: %q", got)
	}
}

func TestFileObjectReconcile(t *testing.T) {
	f := newFileObjectFixture(t)
	ctx := context.Background()
	write := func(rel, content string) {
		t.Helper()
		abs := filepath.Join(f.root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("notes/one.md", "one")
	write("notes/two.md", "two")
	write("System/model-policy.toml", "policy")
	write("Source/platform/main.go", "package main")
	if err := os.Symlink(filepath.Join(f.root, "notes/one.md"), filepath.Join(f.root, "link.md")); err != nil {
		t.Fatal(err)
	}
	f.setSwitch(t, true)

	res, err := f.fo.Reconcile(ctx)
	if err != nil || res.Created != 3 {
		t.Fatalf("first reconcile = %+v, %v; want 3 created (Source/ and the symlink skipped)", res, err)
	}
	if _, err := f.db.GetFileObjectByPath(ctx, foOwner, foComputer, "Source/platform/main.go"); !errors.Is(err, choirstore.ErrNotFound) {
		t.Fatalf("Source/ file has an object: %v", err)
	}
	two, _ := f.db.GetFileObjectByPath(ctx, foOwner, foComputer, "notes/two.md")

	res, err = f.fo.Reconcile(ctx)
	if err != nil || res.Writes() != 0 {
		t.Fatalf("second reconcile on an unchanged tree = %+v, %v; want no writes", res, err)
	}

	// An out-of-band rename with a unique digest keeps the id; an edit
	// keeps it; a removal marks it deleted.
	if err := os.Rename(filepath.Join(f.root, "notes/two.md"), filepath.Join(f.root, "notes/renamed.md")); err != nil {
		t.Fatal(err)
	}
	write("notes/one.md", "one, edited")
	policy, _ := f.db.GetFileObjectByPath(ctx, foOwner, foComputer, "System/model-policy.toml")
	if err := os.Remove(filepath.Join(f.root, "System/model-policy.toml")); err != nil {
		t.Fatal(err)
	}
	res, err = f.fo.Reconcile(ctx)
	if err != nil || res.Moved != 1 || res.Updated != 1 || res.Deleted != 1 {
		t.Fatalf("third reconcile = %+v, %v; want 1 moved, 1 updated, 1 deleted", res, err)
	}
	renamed, err := f.db.GetFileObjectByPath(ctx, foOwner, foComputer, "notes/renamed.md")
	if err != nil || renamed.FileID != two.FileID {
		t.Fatalf("renamed = %+v, %v; want id %s", renamed, err, two.FileID)
	}
	gone, _ := f.db.GetFileObject(ctx, foOwner, policy.FileID)
	if gone.State != choirstore.FileStateDeleted {
		t.Fatalf("removed file = %+v, want deleted", gone)
	}
}

func TestFileObjectReconcileGuards(t *testing.T) {
	f := newFileObjectFixture(t)
	ctx := context.Background()
	f.setSwitch(t, true)
	for i := 0; i < 10; i++ {
		name := filepath.Join(f.root, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if res, err := f.fo.Reconcile(ctx); err != nil || res.Created != 10 {
		t.Fatalf("seed reconcile = %+v, %v", res, err)
	}

	// A fresh volume (the image seed, hydration pending) is never reconciled.
	if err := MarkFreshVolume(f.root); err != nil {
		t.Fatal(err)
	}
	if _, err := f.fo.Reconcile(ctx); err == nil {
		t.Fatal("reconcile on a fresh volume ran")
	}
	if err := clearFreshVolumeMarker(f.root); err != nil {
		t.Fatal(err)
	}

	// An emptied tree holds its deletions.
	for i := 0; i < 8; i++ {
		_ = os.Remove(filepath.Join(f.root, "f"+string(rune('a'+i))+".txt"))
	}
	res, err := f.fo.Reconcile(ctx)
	if err != nil || res.Deleted != 0 || res.DeletionsHeld != 8 {
		t.Fatalf("reconcile of an emptied tree = %+v, %v; want 8 deletions held", res, err)
	}
	list, _ := f.db.ListFileObjects(ctx, foOwner, foComputer)
	if len(list) != 10 {
		t.Fatalf("objects after a held mass delete = %d, want 10", len(list))
	}
}

func TestFileObjectReadPathIsOwnerScoped(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.do(t, http.MethodPut, "/api/files/a.txt", []byte("a"))
	req := httptest.NewRequest(http.MethodGet, "/api/file-objects?path=a.txt", nil)
	req.Header.Set("X-Authenticated-User", "someone-else@example.com")
	w := httptest.NewRecorder()
	f.fo.HandleObjects(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("another owner = %d %s, want 404", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/file-objects", nil)
	w = httptest.NewRecorder()
	f.fo.HandleObjects(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d, want 401", w.Code)
	}
	// The reconcile route is POST only.
	req = authRequest(http.MethodGet, "/api/file-objects/reconcile")
	w = httptest.NewRecorder()
	f.fo.HandleReconcile(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET reconcile = %d, want 405", w.Code)
	}
	req = authRequest(http.MethodPost, "/api/file-objects/reconcile")
	w = httptest.NewRecorder()
	f.fo.HandleReconcile(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST reconcile = %d %s", w.Code, w.Body.String())
	}
}
