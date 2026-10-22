package capsule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Review bunch 91 round 2 (docs/problems/capsule-coldstart-18s-2026-10-22.md).
// Ways it could fail, written before the code:
//   - the spawn refusal drops its cause, so an object-id mismatch, a missing
//     object, a git exit and a deadline all read "source changed";
//   - the refusal drops the expected or copied digest on a mismatch;
//   - wrapping the cause turns the refusal into an acceptance (nil error);
//   - the cache answers one commit with another's digest;
//   - the cache grows without bound, or loses correctness under concurrent
//     readers while it evicts.

func TestSpawnSourceRefusalKeepsTheCauseAndBothDigests(t *testing.T) {
	expected, copied := strings.Repeat("ab", 32), strings.Repeat("cd", 32)
	if err := spawnSourceRefusal(nil, expected, expected); err != nil {
		t.Fatalf("matching copy refused: %v", err)
	}
	mismatch := spawnSourceRefusal(nil, expected, copied)
	if mismatch == nil {
		t.Fatal("mismatched copy accepted")
	}
	for _, want := range []string{"capsule source changed after durable preflight", expected, copied} {
		if !strings.Contains(mismatch.Error(), want) {
			t.Errorf("mismatch refusal %q lacks %q", mismatch, want)
		}
	}
	deadline := spawnSourceRefusal(fmt.Errorf("copy: %w", context.DeadlineExceeded), expected, "")
	if deadline == nil || !errors.Is(deadline, context.DeadlineExceeded) {
		t.Fatalf("deadline cause lost: %v", deadline)
	}
	for _, want := range []string{"capsule source changed after durable preflight", expected} {
		if !strings.Contains(deadline.Error(), want) {
			t.Errorf("deadline refusal %q lacks %q", deadline, want)
		}
	}
	// A cause with a matching digest string is still a refusal.
	if err := spawnSourceRefusal(errors.New("git exited 128"), expected, expected); err == nil {
		t.Fatal("a copy error with a matching digest was accepted")
	}
}

func TestSpawnSourceRefusalNamesACorruptObjectStore(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	expected, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	replaceLooseObject(t, source, blobOIDOf(t, source, commit, "name with spaces"), []byte("other\n"))
	copied, copyErr := copyImmutableCommitTreeTimed(context.Background(), source, commit, filepath.Join(t.TempDir(), "copy"), expected, nil)
	refusal := spawnSourceRefusal(copyErr, expected, copied)
	if refusal == nil || !strings.Contains(refusal.Error(), "does not hash to its id") {
		t.Fatalf("refusal does not say the object store is corrupt: %v", refusal)
	}
}

func TestCopyMismatchNamesExpectedAndCopiedDigests(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	actual, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	wrong := strings.Repeat("ab", 32)
	_, err = copyImmutableCommitTreeTimed(context.Background(), source, commit, filepath.Join(t.TempDir(), "copy"), wrong, nil)
	if err == nil || !strings.Contains(err.Error(), wrong) || !strings.Contains(err.Error(), actual) {
		t.Fatalf("mismatch error %v does not name expected %s and copied %s", err, wrong, actual)
	}
}

// commitChain makes n commits in one repository, each with a different tree,
// and returns their ids in order.
func commitChain(t *testing.T, n int) (string, []string) {
	t.Helper()
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	// No detached auto-maintenance racing the test's own reads (the round 2
	// flake's hypothesis).
	runSourceGit(t, source, "config", "maintenance.auto", "false")
	runSourceGit(t, source, "config", "gc.auto", "0")
	commits := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(source, "counter"), []byte(fmt.Sprint(i)), 0o644); err != nil {
			t.Fatal(err)
		}
		runSourceGit(t, source, "add", ".")
		runSourceGit(t, source, "commit", "-q", "-m", fmt.Sprint(i))
		commit, err := immutableGitCommitIdentity(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		commits = append(commits, commit)
	}
	return source, commits
}

func TestCommitDigestCacheKeepsDifferentCommitsApart(t *testing.T) {
	source, commits := commitChain(t, 2)
	e := &Executor{sourceDir: source}
	for round := 0; round < 2; round++ {
		for _, commit := range commits {
			want, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.commitDigest(context.Background(), commit)
			if err != nil || got != want {
				t.Fatalf("round %d commit %s: cached %s, want %s (%v)", round, commit, got, want, err)
			}
		}
	}
	first, _ := e.commitDigest(context.Background(), commits[0])
	second, _ := e.commitDigest(context.Background(), commits[1])
	if first == second {
		t.Fatal("two commits with different trees share one digest")
	}
}

func TestCommitDigestCacheIsBoundedAndRightUnderConcurrency(t *testing.T) {
	source, commits := commitChain(t, commitDigestCacheLimit+6)
	want := make(map[string]string, len(commits))
	for _, commit := range commits {
		digest, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
		if err != nil {
			t.Fatal(err)
		}
		want[commit] = digest
	}
	e := &Executor{sourceDir: source}
	var wg sync.WaitGroup
	errs := make(chan error, 8*len(commits))
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range commits {
				commit := commits[(i+worker*9)%len(commits)]
				got, err := e.commitDigest(context.Background(), commit)
				if err != nil || got != want[commit] {
					errs <- fmt.Errorf("worker %d commit %s: %s, want %s (%v)", worker, commit, got, want[commit], err)
				}
				e.commitDigestMu.Lock()
				size := len(e.commitDigests)
				e.commitDigestMu.Unlock()
				if size > commitDigestCacheLimit {
					errs <- fmt.Errorf("cache holds %d entries, limit %d", size, commitDigestCacheLimit)
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestInventoryErrorCarriesGitStderr(t *testing.T) {
	source, _ := commitChain(t, 1)
	_, err := immutableCommitInventory(context.Background(), source, strings.Repeat("0", 40))
	if err == nil || !strings.Contains(err.Error(), "exit status 128") || !strings.Contains(strings.ToLower(err.Error()), "fatal") {
		t.Fatalf("inventory error %v does not carry git's exit and stderr", err)
	}
}
