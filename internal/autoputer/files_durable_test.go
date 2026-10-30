package autoputer

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file saved through PUT /api/files is acknowledged only once its bytes and
// its directory entry are on disk. The C0 kill -9 drill on d@e (2026-10-30,
// docs/evidence/data-system-cutover-drills-2026-10-30.md) found all 313
// acknowledged saves of the 16 s before a SIGKILL read back as zero-byte
// files: os.WriteFile with no fsync, while the tape already recorded the
// change. Ways the save could still fail:
//  1. the file's bytes are not fsynced before the 200;
//  2. the directory is not fsynced, so a new name can vanish;
//  3. a failed fsync is answered "file saved";
//  4. a failed save leaves a temporary file in the owner's tree, or emits a
//     change event;
//  5. the content or the existing file mode changes.
func TestFilePutIsDurableBeforeItIsAcknowledged(t *testing.T) {
	fh, root := setupFileTest(t)
	dir := filepath.Join(root, "documents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	var synced []string
	restore := filesDurableSync
	t.Cleanup(func() { filesDurableSync = restore })
	filesDurableSync = func(f *os.File) error {
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if info.IsDir() {
			synced = append(synced, "dir:"+f.Name())
		} else {
			synced = append(synced, "file")
		}
		return nil
	}
	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, authBodyRequest(http.MethodPut, "/api/files/documents/note.txt", strings.NewReader("hello texture")))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body)
	}
	if len(synced) != 2 || synced[0] != "file" || synced[1] != "dir:"+dir {
		t.Fatalf("syncs before the 200 = %v, want the file then %s", synced, dir)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "hello texture" {
		t.Fatalf("saved content = %q, %v", got, err)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want the existing 0600", info.Mode().Perm())
	}

	filesDurableSync = func(*os.File) error { return errors.New("disk said no") }
	w = httptest.NewRecorder()
	fh.HandleFileByPath(w, authBodyRequest(http.MethodPut, "/api/files/documents/note.txt", strings.NewReader("lost")))
	if w.Code == http.StatusOK {
		t.Fatal("a save whose fsync failed was acknowledged")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "note.txt" {
			t.Fatalf("a failed save left %q in the owner's tree", e.Name())
		}
	}
	if got, _ := os.ReadFile(target); string(got) != "hello texture" {
		t.Fatalf("a failed save changed the file to %q", got)
	}
}

// Folder creation, deletion and moves change directory entries; each is
// acknowledged only after the directories it changed are fsynced, or an
// acknowledged delete can come back and a new folder or a moved file can
// vanish after a kill. Ways this could fail: the parent of a new folder, of a
// deleted entry, or either side of a move is not fsynced before the 200; a
// failed fsync is acknowledged.
func TestFileTreeChangesAreDurableBeforeTheyAreAcknowledged(t *testing.T) {
	fh, root := setupFileTest(t)
	fh.moveGate = func(*http.Request) bool { return true }
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var synced []string
	restore := filesDurableSync
	t.Cleanup(func() { filesDurableSync = restore })
	filesDurableSync = func(f *os.File) error { synced = append(synced, f.Name()); return nil }
	contains := func(dir string) bool {
		for _, s := range synced {
			if s == dir {
				return true
			}
		}
		return false
	}

	w := httptest.NewRecorder()
	fh.HandleFileByPath(w, authRequest(http.MethodPost, "/api/files/a/new-folder"))
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("mkdir = %d %s", w.Code, w.Body)
	}
	if !contains(a) {
		t.Fatalf("mkdir synced %v, not the parent %s", synced, a)
	}

	synced = nil
	w = httptest.NewRecorder()
	fh.HandleFileByPath(w, authBodyRequest(http.MethodPatch, "/api/files/a/f.txt", strings.NewReader(`{"move_to":"b/f.txt"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("move = %d %s", w.Code, w.Body)
	}
	if !contains(a) || !contains(b) {
		t.Fatalf("move synced %v, want both %s and %s", synced, a, b)
	}

	synced = nil
	w = httptest.NewRecorder()
	fh.HandleFileByPath(w, authRequest(http.MethodDelete, "/api/files/b/f.txt"))
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body)
	}
	if !contains(b) {
		t.Fatalf("delete synced %v, not the parent %s", synced, b)
	}

	filesDurableSync = func(*os.File) error { return errors.New("disk said no") }
	w = httptest.NewRecorder()
	fh.HandleFileByPath(w, authRequest(http.MethodDelete, "/api/files/a/new-folder"))
	if w.Code == http.StatusOK || w.Code == http.StatusNoContent {
		t.Fatal("a delete whose fsync failed was acknowledged")
	}
}
