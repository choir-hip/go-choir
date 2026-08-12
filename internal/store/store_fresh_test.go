package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFreshRefusesExistingWorkspaceMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFresh(path); !errors.Is(err, ErrFreshWorkspaceRequiresEmpty) {
		t.Fatalf("OpenFresh error=%v, want ErrFreshWorkspaceRequiresEmpty", err)
	}
}

func TestOpenFreshCreatesThenRefusesReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	workspace, err := OpenFresh(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFresh(path); !errors.Is(err, ErrFreshWorkspaceRequiresEmpty) {
		t.Fatalf("reused OpenFresh error=%v, want ErrFreshWorkspaceRequiresEmpty", err)
	}
}

func TestOpenFreshRefusesExistingWorkspaceWithoutMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	workspacePath := deriveTextureWorkspacePath(path)
	if err := os.Mkdir(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFresh(path); !errors.Is(err, ErrFreshWorkspaceRequiresEmpty) {
		t.Fatalf("OpenFresh error=%v, want ErrFreshWorkspaceRequiresEmpty", err)
	}
	if _, err := os.Stat(workspacePath); err != nil {
		t.Fatalf("OpenFresh removed existing workspace: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("OpenFresh left marker after refusal: err=%v", err)
	}
}

func TestOpenFreshExclusivelyReservesConcurrentCallers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	type result struct {
		store *Store
		err   error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			store, err := OpenFresh(path)
			results <- result{store: store, err: err}
		}()
	}

	var successes int
	for range 2 {
		got := <-results
		if got.err == nil {
			successes++
			if err := got.store.Close(); err != nil {
				t.Fatalf("close fresh store: %v", err)
			}
			continue
		}
		if !errors.Is(got.err, ErrFreshWorkspaceRequiresEmpty) {
			t.Fatalf("concurrent OpenFresh error=%v, want ErrFreshWorkspaceRequiresEmpty", got.err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent OpenFresh successes=%d, want 1", successes)
	}
}
