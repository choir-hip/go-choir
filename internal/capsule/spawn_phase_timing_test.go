package capsule

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The spawn phase timer is log lines only (docs/problems/capsule-coldstart-18s-2026-10-22.md).
// Ways it could fail, written before the code:
//   - a phase line lacks the capsule id or a duration, so the deployed log
//     cannot attribute the cold start;
//   - the source materialization is logged as one lump, hiding which of the
//     git passes costs the seconds;
//   - the blob pass does not say how many files and git processes it ran;
//   - the timed copy changes the digest or the error the untimed copy gives;
//   - a nil timer (the carried-patch and test callers) panics;
//   - a failed spawn logs no total, so a timeout looks like silence;
//   - a spawn that panics logs ok=true (it returns no error);
//   - git_procs miscounts the git processes a phase ran (the ls-tree).

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(previous); log.SetFlags(flags) })
	return &buf
}

func timedSourceRepo(t *testing.T) (string, string) {
	t.Helper()
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	for name, body := range map[string]string{"a": "one", "nested/b": "two", "nested/deep/c": "three"} {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "tree")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	return source, commit
}

func unlockTree(t *testing.T, root string) {
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
}

func TestSpawnPhaseTimerLogsEachSourcePhaseUnderTheCapsule(t *testing.T) {
	source, commit := timedSourceRepo(t)
	untimedTarget := filepath.Join(t.TempDir(), "untimed")
	unlockTree(t, untimedTarget)
	untimed, err := copyImmutableCommitTree(context.Background(), source, commit, untimedTarget)
	if err != nil {
		t.Fatal(err)
	}

	buf := captureLog(t)
	timer := newSpawnPhaseTimer("capsule-timing-test")
	target := filepath.Join(t.TempDir(), "timed")
	unlockTree(t, target)
	timed, err := copyImmutableCommitTreeTimed(context.Background(), source, commit, target, timer)
	if err != nil {
		t.Fatal(err)
	}
	if timed != untimed {
		t.Fatalf("timed digest %q != untimed %q", timed, untimed)
	}
	timer.markOK()
	timer.done(nil)

	out := buf.String()
	line := regexp.MustCompile(`(?m)^capsule capsule-timing-test spawn phase=(\S+) took=\S+`)
	seen := map[string]bool{}
	for _, m := range line.FindAllStringSubmatch(out, -1) {
		seen[m[1]] = true
	}
	if !regexp.MustCompile(`phase=source_inventory took=\S+ files=3 git_procs=1`).MatchString(out) {
		t.Errorf("inventory phase does not count its ls-tree:\n%s", out)
	}
	for _, phase := range []string{"source_inventory", "source_blobs", "source_digest_copy", "source_digest_commit", "source_read_only"} {
		if !seen[phase] {
			t.Errorf("no %s phase line in:\n%s", phase, out)
		}
	}
	if !regexp.MustCompile(`phase=source_blobs took=\S+ files=3 git_procs=3`).MatchString(out) {
		t.Errorf("blob phase does not count files and git processes:\n%s", out)
	}
	if !regexp.MustCompile(`phase=source_digest_commit took=\S+ files=3 git_procs=4`).MatchString(out) {
		t.Errorf("commit digest phase does not count files and git processes:\n%s", out)
	}
	if !regexp.MustCompile(`(?m)^capsule capsule-timing-test spawn total=\S+ ok=true$`).MatchString(out) {
		t.Errorf("no total line:\n%s", out)
	}
}

func TestSpawnPhaseTimerNilIsSilentAndFailureLogsTotal(t *testing.T) {
	buf := captureLog(t)
	var none *spawnPhaseTimer
	none.phase("anything")
	none.done(errors.New("ignored"))
	if buf.Len() != 0 {
		t.Fatalf("nil timer logged: %q", buf.String())
	}
	timer := newSpawnPhaseTimer("capsule-fail-test")
	timer.phase("preflight", "detail=x")
	timer.done(errors.New("broker readiness timed out"))
	out := buf.String()
	if !strings.Contains(out, "capsule capsule-fail-test spawn phase=preflight took=") || !strings.Contains(out, " detail=x") {
		t.Fatalf("phase line missing: %q", out)
	}
	if !regexp.MustCompile(`capsule capsule-fail-test spawn total=\S+ ok=false err=broker readiness timed out`).MatchString(out) {
		t.Fatalf("failure total missing: %q", out)
	}
}

// A panicking spawn returns no error but never reaches its successful
// return, so it must not log ok=true.
func TestSpawnPhaseTimerWithoutSuccessIsNotOK(t *testing.T) {
	buf := captureLog(t)
	timer := newSpawnPhaseTimer("capsule-panic-test")
	func() {
		defer func() { _ = recover() }()
		defer func() { timer.done(nil) }()
		panic("spawn crashed")
	}()
	if !regexp.MustCompile(`capsule capsule-panic-test spawn total=\S+ ok=false err=spawn did not return \(panic\)`).MatchString(buf.String()) {
		t.Fatalf("panicked spawn total = %q", buf.String())
	}
}

func TestTimedCopyKeepsTheUntimedRefusal(t *testing.T) {
	source, _ := timedSourceRepo(t)
	_ = captureLog(t)
	bogus := strings.Repeat("0", 40)
	_, untimedErr := copyImmutableCommitTree(context.Background(), source, bogus, filepath.Join(t.TempDir(), "u"))
	_, timedErr := copyImmutableCommitTreeTimed(context.Background(), source, bogus, filepath.Join(t.TempDir(), "t"), newSpawnPhaseTimer("capsule-x"))
	if untimedErr == nil || timedErr == nil || untimedErr.Error() != timedErr.Error() {
		t.Fatalf("untimed=%v timed=%v", untimedErr, timedErr)
	}
}
