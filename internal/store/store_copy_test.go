package store

// Tests of the boot sweep of the snapshot scratch directory and of a crashed
// compaction's staging file (the store-copy token these once covered is gone:
// no capture writes a copy any more).

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func leftoverFixture(t *testing.T) (marker string, files map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "store")
	files = map[string][]byte{
		"state.db":                               []byte("the live store"),
		"state.db.compact":                       []byte("the staged copy"),
		"state.db.compact.json":                  []byte(`{"sequence":7}`),
		"state.db.compact-staging":               []byte("never trusted"),
		"state.db.compact-staging-wal":           []byte("w"),
		"state.db.compact-staging-shm":           []byte("s"),
		"state.db.compact-staging-journal":       []byte("j"),
		SnapshotScratchDirName + "/state.db":     []byte("scratch"),
		SnapshotScratchDirName + "/state.db-wal": []byte("w"),
		SnapshotScratchDirName + "/state.db-shm": []byte("s"),
	}
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return marker, files
}

// Review of lane snapshot-capture-s3: a crashed compaction's staging file (up to
// the store's size) with its sidecars is removed at boot, with the scratch
// directory, before the store opens; the staged copy, its sidecar and the store
// are never touched.
func TestTheBootSweepRemovesACrashedCompactionsStagingAndTheScratch(t *testing.T) {
	marker, files := leftoverFixture(t)
	dir := filepath.Dir(marker)
	if err := RemoveBootScratch(marker); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"state.db.compact-staging", "state.db.compact-staging-wal", "state.db.compact-staging-shm", "state.db.compact-staging-journal", SnapshotScratchDirName} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s survived the boot sweep: %v", gone, err)
		}
	}
	for _, kept := range []string{"state.db", "state.db.compact", "state.db.compact.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, kept))
		if err != nil || !bytes.Equal(raw, files[kept]) {
			t.Fatalf("%s changed or vanished: %q %v", kept, raw, err)
		}
	}
	if err := RemoveBootScratch(marker); err != nil {
		t.Fatalf("sweeping what is absent must succeed: %v", err)
	}
}

// Failure 22 (the helper): the boot sweep removes the scratch directory and
// nothing else.
func TestTheBootSweepRemovesOnlyTheScratch(t *testing.T) {
	marker, files := leftoverFixture(t)
	dir := filepath.Dir(marker)
	if err := RemoveSnapshotScratch(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, SnapshotScratchDirName)); !os.IsNotExist(err) {
		t.Fatalf("the scratch survived the boot sweep: %v", err)
	}
	for name, body := range files {
		if filepath.Dir(name) == SnapshotScratchDirName {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(raw, body) {
			t.Fatalf("the boot sweep touched %s: %q %v", name, raw, err)
		}
	}
	if err := RemoveSnapshotScratch(marker); err != nil {
		t.Fatalf("sweeping an absent scratch must succeed: %v", err)
	}
}
