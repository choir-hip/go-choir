package capsule

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalkUpperdir(t *testing.T) {
	tmpDir := t.TempDir()
	upperDir := filepath.Join(tmpDir, "upper")

	// Create test files.
	os.MkdirAll(filepath.Join(upperDir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(upperDir, "file1.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(upperDir, "subdir", "file2.txt"), []byte("world"), 0o644)

	manifests, err := walkUpperdir(context.Background(), upperDir)
	if err != nil {
		t.Fatalf("walkUpperdir failed: %v", err)
	}

	if len(manifests) != 3 { // subdir, file1.txt, subdir/file2.txt
		t.Errorf("expected 3 manifests, got %d", len(manifests))
	}

	// Check sorting (paths should be sorted alphabetically).
	if manifests[0].Path != "file1.txt" {
		t.Errorf("expected first path 'file1.txt', got '%s'", manifests[0].Path)
	}
	if manifests[1].Path != "subdir" {
		t.Errorf("expected second path 'subdir', got '%s'", manifests[1].Path)
	}
	if manifests[2].Path != "subdir/file2.txt" {
		t.Errorf("expected third path 'subdir/file2.txt', got '%s'", manifests[2].Path)
	}

	// Check hash for regular file.
	if manifests[0].Hash == "" {
		t.Error("expected non-empty hash for file1.txt")
	}
	if manifests[2].Hash == "" {
		t.Error("expected non-empty hash for subdir/file2.txt")
	}

	// Check type.
	if manifests[0].Type != "file" {
		t.Errorf("expected type 'file', got '%s'", manifests[0].Type)
	}
	if manifests[1].Type != "dir" {
		t.Errorf("expected type 'dir', got '%s'", manifests[1].Type)
	}
}

func TestDiffManifests(t *testing.T) {
	lastCommit := []FileManifest{
		{Path: "file1.txt", Size: 5, Hash: "hash1", Mode: 0o644, Type: "file"},
		{Path: "file2.txt", Size: 5, Hash: "hash2", Mode: 0o644, Type: "file"},
		{Path: "subdir", Mode: 0o755, Type: "dir"},
	}

	current := []FileManifest{
		{Path: "file1.txt", Size: 11, Hash: "hash1modified", Mode: 0o644, Type: "file"},
		{Path: "file3.txt", Size: 7, Hash: "hash3", Mode: 0o644, Type: "file"},
		{Path: "subdir", Mode: 0o755, Type: "dir"},
	}

	changes := diffManifests(lastCommit, current)

	// Expected: file1.txt modified, file2.txt deleted, file3.txt added.
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}

	// Check sorting.
	changeMap := make(map[string]ChangeKind)
	for _, c := range changes {
		changeMap[c.Path] = c.Kind
	}

	if changeMap["file1.txt"] != ChangeModified {
		t.Errorf("expected file1.txt to be modified, got %s", changeMap["file1.txt"])
	}
	if changeMap["file2.txt"] != ChangeDeleted {
		t.Errorf("expected file2.txt to be deleted, got %s", changeMap["file2.txt"])
	}
	if changeMap["file3.txt"] != ChangeAdded {
		t.Errorf("expected file3.txt to be added, got %s", changeMap["file3.txt"])
	}
}

func TestDiffManifestsNoChanges(t *testing.T) {
	manifest := []FileManifest{
		{Path: "file1.txt", Size: 5, Hash: "hash1", Mode: 0o644, Type: "file"},
	}

	changes := diffManifests(manifest, manifest)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestDiffManifestsEmptyLastCommit(t *testing.T) {
	current := []FileManifest{
		{Path: "file1.txt", Size: 5, Hash: "hash1", Mode: 0o644, Type: "file"},
		{Path: "file2.txt", Size: 5, Hash: "hash2", Mode: 0o644, Type: "file"},
	}

	changes := diffManifests(nil, current)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes (both added), got %d", len(changes))
	}

	for _, c := range changes {
		if c.Kind != ChangeAdded {
			t.Errorf("expected all changes to be 'added', got %s for %s", c.Kind, c.Path)
		}
	}
}

func TestWalkUpperdirHonorsCancellation(t *testing.T) {
	upperDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(upperDir, "large"), make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := walkUpperdir(ctx, upperDir); !errors.Is(err, context.Canceled) {
		t.Fatalf("walkUpperdir error = %v, want context canceled", err)
	}
}

func TestContextReaderHonorsCancellationBetweenChunks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &contextReader{ctx: ctx, reader: strings.NewReader("payload")}
	cancel()
	if _, err := reader.Read(make([]byte, 8)); !errors.Is(err, context.Canceled) {
		t.Fatalf("contextReader error = %v, want context canceled", err)
	}
}

type cancelingEOFReader struct {
	cancel context.CancelFunc
	done   bool
}

func (r *cancelingEOFReader) Read(buffer []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	n := copy(buffer, "payload")
	r.cancel()
	return n, io.EOF
}

func TestContextReaderPrefersCancellationOnFinalRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_, err := io.Copy(io.Discard, &contextReader{
		ctx: ctx, reader: &cancelingEOFReader{cancel: cancel},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("final-read error = %v, want context canceled", err)
	}
}
