package autoputer

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	fo := newFileObjects(root, db, foOwner, foComputer, testDigestKey)
	fo.MarkFilesReady()
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

var testFileDigestKey = bytes.Repeat([]byte{0x42}, 32)

func testDigestKey(context.Context) ([]byte, error) { return bytes.Clone(testFileDigestKey), nil }

// digestOf is the keyed digest the recorder must write for these bytes.
func digestOf(b []byte) string {
	mac := hmac.New(sha256.New, testFileDigestKey)
	mac.Write(b)
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
}

func plainDigestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (f *fileObjectFixture) write(t *testing.T, rel, content string) {
	t.Helper()
	abs := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fileObjectFixture) remove(t *testing.T, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func (f *fileObjectFixture) reconcile(t *testing.T) fileReconcileResult {
	t.Helper()
	res, err := f.fo.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return res
}

func (f *fileObjectFixture) byPath(rel string) (choirstore.FileObject, error) {
	return f.db.GetFileObjectByPath(context.Background(), foOwner, foComputer, rel)
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
	if code != http.StatusOK || obj.Digest != digestOf(first) || obj.Size != int64(len(first)) || obj.FileID == "" || obj.MediaType != "text/plain; charset=utf-8" {
		t.Fatalf("after put = %d %+v", code, obj)
	}
	id := obj.FileID

	second := []byte("second, longer bytes")
	f.do(t, http.MethodPut, "/api/files/docs/a.txt", second)
	_, obj = f.object(t, "path=docs/a.txt")
	if obj.FileID != id || obj.Digest != digestOf(second) {
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
	if err != nil || res.Moved != 1 || res.Updated != 1 || res.Deleted != 0 {
		t.Fatalf("third reconcile = %+v, %v; want 1 moved, 1 updated, and the removal only noted", res, err)
	}
	res, err = f.fo.Reconcile(ctx)
	if err != nil || res.Deleted != 1 || res.Writes() != 1 {
		t.Fatalf("fourth reconcile = %+v, %v; want the removal, absent twice, deleted", res, err)
	}
	renamed, err := f.db.GetFileObjectByPath(ctx, foOwner, foComputer, "notes/renamed.md")
	if err != nil || renamed.FileID != two.FileID {
		t.Fatalf("renamed = %+v, %v; want id %s", renamed, err, two.FileID)
	}
	gone, _ := f.db.GetFileObject(ctx, foOwner, foComputer, policy.FileID)
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

// Review round 1 (bunch 208). Ways the fixes can fail, written before them:
//  10. A name the filesystem keeps literal (a backslash) is rewritten, so two
//      files share one object path and reconcile churns on it forever.
//  11. A concurrent PUT to a move's destination is silently replaced.
//  12. A symlinked parent lets a verb read, write or move outside the root.
//  13. A file that is only absent (a slow hydration, an unmounted volume, a
//      rename across a walked directory) is recorded deleted.
//  14. The switch turned off mid-pass still lets the pass write.
//  15. The plaintext digest reaches the tape.
//  16. One unreadable file stops every later pass; an empty file pairs as a
//      move with any other empty file; a file named Source is recorded.
//  17. A reconcile holds the Files API still for the whole walk.

func TestFileObjectDigestIsKeyedNotPlaintext(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	body := []byte("a file whose hash must not be a confirmation oracle")
	f.do(t, http.MethodPut, "/api/files/a.txt", body)
	_, obj := f.object(t, "path=a.txt")
	if !strings.HasPrefix(obj.Digest, "hmac-sha256:") || obj.Digest == plainDigestOf(body) || obj.Digest != digestOf(body) {
		t.Fatalf("digest = %q, want the keyed %q and never the plaintext %q", obj.Digest, digestOf(body), plainDigestOf(body))
	}
	// Reconcile records the same keyed digest for an out-of-band write.
	f.write(t, "b.txt", string(body))
	f.reconcile(t)
	b, err := f.byPath("b.txt")
	if err != nil || b.Digest != obj.Digest {
		t.Fatalf("reconciled digest = %q, %v; want %q", b.Digest, err, obj.Digest)
	}
}

func TestFileObjectWithoutAKeyRecordsNothing(t *testing.T) {
	f := newFileObjectFixture(t)
	f.fo.digestKey = nil
	f.setSwitch(t, true)
	f.do(t, http.MethodPut, "/api/files/a.txt", []byte("x"))
	if _, err := f.byPath("a.txt"); !errors.Is(err, choirstore.ErrNotFound) {
		t.Fatalf("object recorded with no key: %v", err)
	}
	f.write(t, "b.txt", "y")
	if _, err := f.fo.Reconcile(context.Background()); err == nil {
		t.Fatal("reconcile without a digest key ran")
	}
}

func TestFileObjectBackslashNameStaysLiteral(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.do(t, http.MethodPut, "/api/files/a%5Cb.txt", []byte("literal"))
	f.write(t, "a/b.txt", "directory")
	if res := f.reconcile(t); res.Created != 1 {
		t.Fatalf("first reconcile = %+v, want the directory file created", res)
	}
	lit, err := f.byPath(`a\b.txt`)
	if err != nil || lit.Digest != digestOf([]byte("literal")) {
		t.Fatalf(`object at a\b.txt = %+v, %v`, lit, err)
	}
	dir, err := f.byPath("a/b.txt")
	if err != nil || dir.Digest != digestOf([]byte("directory")) || dir.FileID == lit.FileID {
		t.Fatalf("object at a/b.txt = %+v, %v; want its own file", dir, err)
	}
	f.reconcile(t)
	if res := f.reconcile(t); res.Writes() != 0 {
		t.Fatalf("steady-state reconcile wrote %+v", res)
	}
}

func TestFileObjectMoveNeverReplacesADestinationCreatedInTheGap(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.do(t, http.MethodPut, "/api/files/a.txt", []byte("moving"))
	f.fh.beforeMoveRename = func(dest string) {
		if err := os.WriteFile(dest, []byte("concurrent put"), 0o644); err != nil {
			t.Error(err)
		}
	}
	w := f.do(t, http.MethodPatch, "/api/files/a.txt", []byte(`{"move_to":"b.txt"}`))
	if w.Code != http.StatusConflict {
		t.Fatalf("move onto a destination created in the gap = %d %s, want 409", w.Code, w.Body.String())
	}
	if got, _ := os.ReadFile(filepath.Join(f.root, "b.txt")); string(got) != "concurrent put" {
		t.Fatalf("destination was replaced: %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, "a.txt")); err != nil || string(got) != "moving" {
		t.Fatalf("source after a refused move = %q, %v", got, err)
	}
}

func TestFileObjectRefusesSymlinkedParents(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.root, "ln")); err != nil {
		t.Fatal(err)
	}
	f.do(t, http.MethodPut, "/api/files/a.txt", []byte("inside"))
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPatch, "/api/files/ln/secret.txt", `{"move_to":"x.txt"}`},
		{http.MethodPatch, "/api/files/a.txt", `{"move_to":"ln/in.txt"}`},
		{http.MethodPut, "/api/files/ln/new.txt", "written"},
		{http.MethodGet, "/api/files/ln/secret.txt", ""},
		{http.MethodDelete, "/api/files/ln/secret.txt", ""},
	} {
		if w := f.do(t, tc.method, tc.path, []byte(tc.body)); w.Code != http.StatusForbidden {
			t.Errorf("%s %s through a symlinked parent = %d %s, want 403", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.txt")); err != nil {
		t.Errorf("outside file was moved or deleted: %v", err)
	}
	for _, name := range []string{"in.txt", "new.txt"} {
		if _, err := os.Stat(filepath.Join(outside, name)); err == nil {
			t.Errorf("%s was written outside the root", name)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, "x.txt")); err == nil {
		t.Error("outside file was moved into the root")
	}
}

func TestFileObjectReconcileDeletesOnlyAbsenceSeenTwice(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "keep.txt", "keep")
	f.write(t, "gone.txt", "gone")
	f.write(t, "blip.txt", "blip")
	f.reconcile(t)
	gone, _ := f.byPath("gone.txt")
	blip, _ := f.byPath("blip.txt")

	f.remove(t, "gone.txt")
	f.remove(t, "blip.txt")
	if res := f.reconcile(t); res.Deleted != 0 {
		t.Fatalf("first pass that misses two files = %+v, want no deletion yet", res)
	}
	if _, err := f.byPath("gone.txt"); err != nil {
		t.Fatalf("object after one absent pass: %v", err)
	}
	f.write(t, "blip.txt", "blip") // only temporarily absent
	res := f.reconcile(t)
	if res.Deleted != 1 {
		t.Fatalf("second pass = %+v, want only the file absent twice deleted", res)
	}
	if got, err := f.db.GetFileObject(context.Background(), foOwner, foComputer, gone.FileID); err != nil || got.State != choirstore.FileStateDeleted {
		t.Fatalf("gone = %+v, %v", got, err)
	}
	back, err := f.byPath("blip.txt")
	if err != nil || back.FileID != blip.FileID {
		t.Fatalf("blip after reappearing = %+v, %v; want the same id %s", back, err, blip.FileID)
	}
}

func TestFileObjectReconcileRestatsBeforeDeleting(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "keep.txt", "keep")
	f.write(t, "late.txt", "late")
	f.reconcile(t)
	f.remove(t, "late.txt")
	f.reconcile(t) // absent once
	calls := 0
	f.fo.afterWalk = func() {
		calls++
		f.write(t, "late.txt", "late") // back after the walk missed it
	}
	res := f.reconcile(t)
	if res.Deleted != 0 {
		t.Fatalf("reconcile deleted a file that exists at apply time: %+v", res)
	}
	if calls != 1 {
		t.Fatalf("afterWalk calls = %d", calls)
	}
	if _, err := f.byPath("late.txt"); err != nil {
		t.Fatalf("late.txt object: %v", err)
	}
}

func TestFileObjectReconcileHoldsDeletionsWhenTheTreeIsEmpty(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "a.txt", "a")
	f.write(t, "b.txt", "b")
	f.write(t, "c.txt", "c")
	f.reconcile(t)
	f.remove(t, "a.txt")
	f.remove(t, "b.txt")
	f.remove(t, "c.txt")
	for i := 0; i < 3; i++ {
		res := f.reconcile(t)
		if res.Deleted != 0 || res.DeletionsHeld != 3 {
			t.Fatalf("pass %d over an empty tree = %+v, want 3 deletions held however many passes", i, res)
		}
	}
	if list, _ := f.db.ListFileObjects(context.Background(), foOwner, foComputer); len(list) != 3 {
		t.Fatalf("objects after an empty-tree reconcile = %d, want 3", len(list))
	}
}

func TestFileObjectReconcileNeedsTheFilesReady(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "a.txt", "a")
	notReady := newFileObjects(f.root, f.db, foOwner, foComputer, testDigestKey)
	if _, err := notReady.Reconcile(context.Background()); err == nil {
		t.Fatal("reconcile ran before the files were known to be hydrated")
	}
	if list, _ := f.db.ListFileObjects(context.Background(), foOwner, foComputer); len(list) != 0 {
		t.Fatalf("objects written by a reconcile before ready: %+v", list)
	}
	notReady.MarkFilesReady()
	if res, err := notReady.Reconcile(context.Background()); err != nil || res.Created != 1 {
		t.Fatalf("reconcile once ready = %+v, %v", res, err)
	}
}

func TestFileObjectReconcileStopsWhenTheSwitchTurnsOffMidPass(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "a.txt", "a")
	f.write(t, "b.txt", "b")
	f.fo.afterWalk = func() { f.setSwitch(t, false) }
	res, err := f.fo.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Writes() != 0 {
		t.Fatalf("pass wrote %+v after the switch turned off", res)
	}
	if list, _ := f.db.ListFileObjects(context.Background(), foOwner, foComputer); len(list) != 0 {
		t.Fatalf("objects written after the switch turned off: %+v", list)
	}
}

func TestFileObjectReconcileSkipsAnUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "locked.txt", "locked")
	f.write(t, "open.txt", "open")
	if err := os.Chmod(filepath.Join(f.root, "locked.txt"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(f.root, "locked.txt"), 0o644) })
	res := f.reconcile(t)
	if res.Created != 1 || res.Unreadable != 1 {
		t.Fatalf("reconcile = %+v, want open.txt created and locked.txt counted unreadable", res)
	}
}

func TestFileObjectEmptyFilesNeverPairAsAMove(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "e1.txt", "")
	f.write(t, "keep.txt", "keep")
	f.reconcile(t)
	e1, _ := f.byPath("e1.txt")
	f.remove(t, "e1.txt")
	f.write(t, "e2.txt", "")
	res := f.reconcile(t)
	if res.Moved != 0 {
		t.Fatalf("reconcile = %+v, want the empty files not paired", res)
	}
	e2, err := f.byPath("e2.txt")
	if err != nil || e2.FileID == e1.FileID {
		t.Fatalf("e2 = %+v, %v; want its own id, not %s", e2, err, e1.FileID)
	}
}

func TestFileObjectReconcileExemptsAFileNamedSource(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "Source", "a file, not the tree")
	f.write(t, "a.txt", "a")
	if res := f.reconcile(t); res.Created != 1 {
		t.Fatalf("reconcile = %+v, want only a.txt", res)
	}
	if _, err := f.byPath("Source"); !errors.Is(err, choirstore.ErrNotFound) {
		t.Fatalf("a file named Source has an object: %v", err)
	}
}

func TestFileObjectReconcileDoesNotHoldTheFilesAPIDuringTheWalk(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "a.txt", "a")
	recorded := make(chan struct{})
	f.fo.afterWalk = func() {
		go func() {
			f.do(t, http.MethodPut, "/api/files/during.txt", []byte("written during the pass"))
			close(recorded)
		}()
		select {
		case <-recorded:
		case <-time.After(5 * time.Second):
			t.Error("a Files API write was blocked while a reconcile walked the tree")
		}
	}
	f.reconcile(t)
	<-recorded
	if _, err := f.byPath("during.txt"); err != nil {
		t.Fatalf("the write made during the pass: %v", err)
	}
}

func TestFileObjectReconcileDropsARecordThatWentStaleDuringTheWalk(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "x.txt", "first")
	f.fo.afterWalk = func() {
		f.write(t, "x.txt", "rewritten after the hash, with a different size")
		f.fo.afterWalk = nil
	}
	res := f.reconcile(t)
	if res.Created != 0 || res.Raced != 1 {
		t.Fatalf("reconcile = %+v, want the stale record dropped", res)
	}
	if res := f.reconcile(t); res.Created != 1 {
		t.Fatalf("next reconcile = %+v, want x.txt created", res)
	}
	x, _ := f.byPath("x.txt")
	if x.Digest != digestOf([]byte("rewritten after the hash, with a different size")) {
		t.Fatalf("recorded digest is stale: %s", x.Digest)
	}
}

func TestFileObjectByIDIsComputerScoped(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	other := "computer-other"
	if err := f.db.PutComputerSetting(context.Background(), choirstore.ComputerSetting{OwnerID: foOwner, ComputerID: other, Key: choirstore.SettingFilesObject, Value: true}); err != nil {
		t.Fatal(err)
	}
	res, err := f.db.ApplyFileChanges(context.Background(), foOwner, other, []choirstore.FileChange{{Op: choirstore.FileOpWrite, Record: choirstore.FileRecord{Path: "o.txt", Digest: "hmac-sha256:aa", Size: 1}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := f.object(t, "id="+res.FileIDs[0]); code != http.StatusNotFound {
		t.Fatalf("another computer's file by id = %d, want 404", code)
	}
}

// Review round 2 (bunch 217): the symlink refusal reaches the last path
// component. A leaf symlink that resolves outside the root carried GET (serve
// or list the target) and PUT (truncate and write the target) out of the root.

func TestFileObjectLeafSymlinksStayInsideTheRoot(t *testing.T) {
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	outside := t.TempDir()
	outFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(outside, "dir")
	if err := os.Mkdir(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.write(t, "real.txt", "inside")
	for name, target := range map[string]string{
		"leaf-file": outFile,
		"leaf-dir":  outDir,
		"dangling":  filepath.Join(outside, "not-yet.txt"),
		"in-root":   filepath.Join(f.root, "real.txt"),
		"relative":  "real.txt",
	} {
		if err := os.Symlink(target, filepath.Join(f.root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"leaf-file", "leaf-dir", "dangling"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			if w := f.do(t, method, "/api/files/"+name, []byte("overwritten")); w.Code != http.StatusForbidden {
				t.Errorf("%s %s (a leaf symlink out of the root) = %d %s, want 403", method, name, w.Code, w.Body.String())
			}
		}
	}
	if got, _ := os.ReadFile(outFile); string(got) != "outside" {
		t.Errorf("outside file was written through a leaf symlink: %q", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "not-yet.txt")); err == nil {
		t.Error("a dangling leaf symlink created a file outside the root")
	}
	// A link that stays in the root keeps working, like files/Source/platform/CLAUDE.md -> AGENTS.md.
	for _, name := range []string{"in-root", "relative"} {
		w := f.do(t, http.MethodGet, "/api/files/"+name, nil)
		if w.Code != http.StatusOK || w.Body.String() != "inside" {
			t.Errorf("GET %s (in-root link) = %d %q, want 200 inside", name, w.Code, w.Body.String())
		}
	}
	// DELETE of a leaf link removes the link only, wherever it points.
	for _, name := range []string{"leaf-file", "leaf-dir", "dangling", "in-root"} {
		f.do(t, http.MethodDelete, "/api/files/"+name, nil)
		if _, err := os.Lstat(filepath.Join(f.root, name)); err == nil {
			t.Errorf("DELETE %s left the link", name)
		}
	}
	if got, err := os.ReadFile(outFile); err != nil || string(got) != "outside" {
		t.Errorf("DELETE of a link touched its target: %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(f.root, "real.txt")); err != nil || string(got) != "inside" {
		t.Errorf("DELETE of an in-root link touched its target: %q, %v", got, err)
	}
}

func TestFileObjectReconcileSkipsAnUnreadableDirectoryWithoutDeletingWhatIsInIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	f := newFileObjectFixture(t)
	f.setSwitch(t, true)
	f.write(t, "locked/inside.txt", "inside")
	f.write(t, "open.txt", "open")
	f.reconcile(t)
	if err := os.Chmod(filepath.Join(f.root, "locked"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(f.root, "locked"), 0o755) })
	f.write(t, "new.txt", "new")
	for i := 0; i < 3; i++ {
		res, err := f.fo.Reconcile(context.Background())
		if err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
		if res.Deleted != 0 || res.Unreadable != 1 {
			t.Fatalf("pass %d = %+v, want nothing deleted and the directory counted unreadable", i, res)
		}
	}
	if _, err := f.byPath("new.txt"); err != nil {
		t.Fatalf("new.txt next to an unreadable directory: %v", err)
	}
	if _, err := f.byPath("locked/inside.txt"); err != nil {
		t.Fatalf("a file in an unreadable directory was deleted: %v", err)
	}
}
