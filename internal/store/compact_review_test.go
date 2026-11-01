package store

// Review 2026-10-31 non-blocking findings N1-N4 for the compact copy.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func init() {
	// The free-space floor is a production headroom (2 GiB); the fixtures
	// here are kilobytes and some stub the free space low on purpose.
	compactFreeFloorBytes = 0
}

// N1: a swapped-in file that will not open is replaced by the original, and a
// staged copy with a damaged header is refused before the swap.
func TestCompactRestorePrecompactAndTornHeader(t *testing.T) {
	t.Run("restore", func(t *testing.T) {
		s, path := compactTestStore(t, 5, 10, 3000)
		if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		compactTestAppend(t, s, 6)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if outcome, err := RebaseOntoCompactCopy(path, true); err != nil || !strings.HasPrefix(outcome, "rebased") {
			t.Fatalf("rebase %q, %v", outcome, err)
		}
		// The swapped-in file is damaged.
		if err := os.WriteFile(StateDBPath(path), []byte("not a database"), 0o640); err != nil {
			t.Fatal(err)
		}
		restored, err := RestorePrecompact(path)
		if err != nil || !restored {
			t.Fatalf("RestorePrecompact = %v, %v", restored, err)
		}
		if _, seq, _ := compactTestRows(t, StateDBPath(path)); seq != 6 {
			t.Fatalf("restored file at sequence %d, want the original's 6", seq)
		}
		if again, _ := RestorePrecompact(path); again {
			t.Fatal("restored twice")
		}
	})
	t.Run("torn header", func(t *testing.T) {
		s, path := compactTestStore(t, 5, 10, 3000)
		if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
			t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(StateDBPath(path)+compactStagedSuffix, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteAt([]byte("XXXXXXXXXXXXXXXX"), 0) // same size, no magic
		_ = f.Close()
		outcome, err := RebaseOntoCompactCopy(path, true)
		if err != nil || !strings.HasPrefix(outcome, "discarded") {
			t.Fatalf("outcome %q, %v; want discarded", outcome, err)
		}
		if exists(StateDBPath(path) + precompactSuffix) {
			t.Fatal("a torn copy was swapped in")
		}
	})
}

// N2: a transient error in the chain check keeps the staged copy; only a
// verdict deletes it.
func TestCompactTransientCheckErrorKeepsTheCopy(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.PrepareCompactCopy(cancelled); err == nil {
		t.Fatal("a cancelled check reported no error")
	}
	if !exists(s.statePath+compactStagedSuffix) || !exists(s.statePath+compactSidecarSuffix) {
		t.Fatal("a transient error deleted the staged copy")
	}
}

// N3: the check of the staged copy is under its own deadline.
func TestCompactIntegrityCheckHasTheCopyBudget(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	if done, err := s.PrepareCompactCopy(context.Background()); err != nil || !done {
		t.Fatalf("PrepareCompactCopy = %v, %v", done, err)
	}
	old := compactCheckBudget
	compactCheckBudget = time.Nanosecond
	defer func() { compactCheckBudget = old }()
	time.Sleep(time.Millisecond)
	if _, err := checkCompactCopy(context.Background(), s.statePath+compactStagedSuffix); err == nil {
		t.Fatal("checkCompactCopy ignored the budget")
	}
}

// N4: free space falling under the floor during the copy aborts it and leaves
// no partial file.
func TestCompactCopyAbortsUnderTheFreeFloor(t *testing.T) {
	s, _ := compactTestStore(t, 5, 10, 3000)
	oldFree, oldFloor := compactFreeBytes, compactFreeFloorBytes
	defer func() { compactFreeBytes, compactFreeFloorBytes = oldFree, oldFloor }()
	compactFreeFloorBytes = 1 << 20
	// Plenty of room until the copy has begun writing.
	compactFreeBytes = func(string) (int64, error) {
		if exists(s.statePath + compactStagingSuffix) {
			return 100, nil
		}
		return 1 << 40, nil
	}
	done, err := s.PrepareCompactCopy(context.Background())
	if done || err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("PrepareCompactCopy = %v, %v; want an abort under the floor", done, err)
	}
	if exists(s.statePath+compactStagingSuffix) || exists(s.statePath+compactStagedSuffix) {
		t.Fatal("a partial copy was left")
	}
}
